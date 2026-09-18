package runtime

import (
	"fmt"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// regionEntry is one orthogonal region's share of entering its container.
type regionEntry struct {
	region    *ast.StateRegion
	container *ast.StateNode
	branches  map[*ast.StateRegion]*ast.StateNode
	target    *ast.StateNode    // where the region starts instead of its own start, if anywhere
	branch    *lower.Transition // the fork branch into the region, whose effect runs first, if any
}

// lazyEntry is the chain of states a fork's branches enter on their way down to
// owner, the composite whose regions they enter; the first entered, as far as the
// state holding the fork, are active before the branches run.
type lazyEntry struct {
	chain    []*ast.StateNode
	entered  int
	owner    *ast.StateNode
	branches map[*ast.StateRegion]*ast.StateNode // region the chain passes through → the state it passes into
}

// sharedEntry is a unit several queues hold — a state on a fork's way down — which
// the first queue to reach it performs and the others then drop.
type sharedEntry struct {
	done bool
}

// entryUnit is one step of entering a region, ordered by the library against the
// others of that region: a branch's effect, or one state's entry.
type entryUnit struct {
	label  string
	state  *ast.StateNode // the state whose entry the unit logs first, if any
	shared *sharedEntry   // set on a unit several queues hold
	after  *sharedEntry   // set on a unit that waits for a shared one to have run
	run    func(q *entryQueue) error
}

// entryQueue is what one region has left to enter, in the library's order; a unit
// entering a state queues what that state starts ahead of the rest.
type entryQueue struct {
	region *ast.StateRegion
	owner  *ast.StateNode // the orthogonal state whose do behavior starts once its queues are spent
	wrap   func(error) error
	units  []entryUnit
}

// entryWay is the way a queue's units are on: the region they enter, where the
// regions below start, and how an error on the way reads.
type entryWay struct {
	region *ast.StateRegion
	plan   map[*ast.StateRegion]*ast.StateNode
	wrap   func(error) error
}

// entryFront is the queues entering one composite's regions or one fork's
// branches, drawn from one unit at a time while two or more have a unit ready.
type entryFront struct {
	exec   *StateExecutor
	where  string
	span   source.Span
	queues []*entryQueue
}

// forkEntry describes the way from boundary, which stays active, down to the
// composite whose regions a fork's branches enter.
func (e *StateExecutor) forkEntry(boundary, owner *ast.StateNode) *lazyEntry {
	return &lazyEntry{
		chain:    e.descendantChain(boundary, owner),
		owner:    owner,
		branches: e.branchesTo(boundary, owner),
	}
}

// enterAboveFork enters the way down as far as the state holding the fork, ahead
// of its branches: each state is activated without its do behavior, and its
// regions off the way start as usual.
func (e *StateExecutor) enterAboveFork(fork *ast.PseudostateNode, l *lazyEntry) error {
	upto := slices.Index(l.chain, e.graph.PseudostateOwner[fork]) + 1
	for ; l.entered < upto; l.entered++ {
		state := l.chain[l.entered]
		if err := e.activateState(state); err != nil {
			return fmt.Errorf("enter state %s: %w", state.Name, err)
		}
		if state == l.owner {
			continue
		}
		if err := e.enterRegionsInto(state, e.offWay(state, l), l.branches); err != nil {
			return fmt.Errorf("enter state %s: %w", state.Name, err)
		}
	}
	return nil
}

// enterRegionsInto enters every orthogonal region of container — at the state
// branches names for it, or at its own start — one unit at a time in an order
// the policy draws. container is nil for the machine's own regions.
func (e *StateExecutor) enterRegionsInto(container *ast.StateNode, regions []*ast.StateRegion, branches map[*ast.StateRegion]*ast.StateNode) error {
	front := &entryFront{exec: e, where: enteringWherePrefix + e.stateMachine.Name, span: e.stateMachine.DeclSpan}
	if container != nil {
		front.where, front.span = enteringWherePrefix+container.Name, container.Span()
	}
	front.addRegions(nil, container, nil, regions, branches, unwrapped)
	return front.drain()
}

// enterForkBranches enters the owner's regions through the fork's branches, one
// queue per region: a branch's effect, the way down to the owner as far as no
// sibling entered it, then its target; a region no branch enters starts as usual
// once the owner is entered. The do behaviors of the states entered on the way
// down start once all have, innermost first, as after an ordinary entry.
func (e *StateExecutor) enterForkBranches(fork *ast.PseudostateNode, plan *lower.ForkPlan, above *lazyEntry) error {
	targets := plan.Targets()
	front := &entryFront{exec: e, where: forkWherePrefix + fork.Name, span: fork.Span()}
	way := front.wayDown(above)
	for _, region := range e.graph.CompositeStates[plan.Owner] {
		entry := &regionEntry{region: region, container: plan.Owner, branches: targets}
		if branch := plan.Branches[region]; branch != nil {
			entry.target, entry.branch = branch.Target.(*ast.StateNode), branch
		}
		front.queues = append(front.queues, front.branchQueue(entry, way))
	}
	if err := front.drain(); err != nil {
		return err
	}
	for i := len(above.chain) - 1; i >= 0; i-- {
		e.startDoAction(above.chain[i])
	}
	return nil
}

// offWay is the regions of state a fork's way down does not pass through.
func (e *StateExecutor) offWay(state *ast.StateNode, l *lazyEntry) []*ast.StateRegion {
	var off []*ast.StateRegion
	for _, region := range e.graph.CompositeStates[state] {
		if _, onWay := l.branches[region]; !onWay {
			off = append(off, region)
		}
	}
	return off
}

// runBranchEffect executes a fork branch's effect, if any.
func (e *StateExecutor) runBranchEffect(branch *lower.Transition) error {
	if branch == nil {
		return nil
	}
	for _, behavior := range branch.Effect {
		if err := e.executeBehavior(behavior); err != nil {
			return fmt.Errorf("fork branch effect: %w", err)
		}
	}
	return nil
}

// regionStart is the state a region starts in: the target it was given, the
// substate of a parallel state it stands for, or its own entry transitions' choice.
func (e *StateExecutor) regionStart(w *regionEntry) (*ast.StateNode, error) {
	if w.target != nil {
		return w.target, nil
	}
	if owner := e.graph.RegionState[w.region]; owner != nil {
		return owner, nil
	}
	entry, err := e.startIn(w.region)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, fmt.Errorf("region %s has no initial state", w.region.Name)
	}
	return entry, nil
}

// unguardedStart is the state owner's body starts in when its entry declares one
// unconditional transition, and nil when a guard, read as it starts, decides it.
func (e *StateExecutor) unguardedStart(owner ast.Node) *ast.StateNode {
	transitions := e.graph.StartOf(owner)
	if len(transitions) == 1 && transitions[0].Guard == nil {
		return transitions[0].Target
	}
	return nil
}

// drawOrder resolves an order draw among sibling regions: the policy picks, a
// refused witness or checker move stops the run, and the pick is recorded unless
// the policy keeps the canonical order.
func (e *StateExecutor) drawOrder(choice ChoicePoint) (int, error) {
	scheduling := e.ctx.scheduling()
	if scheduling.keepsOrder(choice.Kind) {
		return 0, nil
	}
	choice.Taken = scheduling.choose(choice, nil)
	if err := scheduling.refusal(); err != nil {
		return 0, err
	}
	e.ctx.noteChoice(choice)
	return choice.Taken, nil
}

func unwrapped(err error) error { return err }

// mayDrawEntering reports whether entering target may draw an entry order: an
// orthogonal state of two or more regions is target, above it or below it.
func (e *StateExecutor) mayDrawEntering(target *ast.StateNode) bool {
	if target == nil {
		return false
	}
	for _, state := range e.graph.CompositeStateOrder {
		if len(e.graph.CompositeStates[state]) < 2 {
			continue
		}
		if slices.Contains(e.getParentChain(target), state) || slices.Contains(e.getParentChain(state), target) {
			return true
		}
	}
	return false
}

// wrapping composes how an error is reported: inner first, then outer.
func wrapping(outer, inner func(error) error) func(error) error {
	return func(err error) error { return outer(inner(err)) }
}

func enterStateWrap(err error) error { return fmt.Errorf("enter state: %w", err) }

func enterNamedWrap(state *ast.StateNode) func(error) error {
	return func(err error) error { return fmt.Errorf("enter state %s: %w", state.Name, err) }
}

func regionStartWrap(region *ast.StateRegion) func(error) error {
	return func(err error) error { return fmt.Errorf("enter starting state in region %s: %w", region.Name, err) }
}

// wayDown is the units entering the states of a fork's way down not yet active,
// shared by every branch's queue; nil when the owner is active already.
func (f *entryFront) wayDown(above *lazyEntry) []entryUnit {
	var way []entryUnit
	for _, group := range f.grouped(above.chain[above.entered:]) {
		way = append(way, f.wayUnit(above, group))
	}
	return way
}

// wayUnit enters a group of states on a fork's way down, once for every branch:
// each is activated without its do behavior, and its regions off the way start
// as queues of the front ahead of the queue that reached it.
func (f *entryFront) wayUnit(above *lazyEntry, group []*ast.StateNode) entryUnit {
	e := f.exec
	unit := entryUnit{label: f.chainLabel(group, nil), state: f.firstShown(group), shared: &sharedEntry{}}
	unit.run = func(q *entryQueue) error {
		for _, state := range group {
			if err := e.activateState(state); err != nil {
				return enterNamedWrap(state)(err)
			}
			if state != above.owner {
				f.addRegions(q, state, nil, e.offWay(state, above), above.branches, enterNamedWrap(state))
			}
		}
		return nil
	}
	return unit
}

// branchQueue is the queue entering w through a fork: its branch's effect if it
// has one, the way down to its container, then its target; a region no branch
// enters waits for the way down to be walked and then starts as usual.
func (f *entryFront) branchQueue(w *regionEntry, way []entryUnit) *entryQueue {
	e := f.exec
	q := &entryQueue{region: w.region, wrap: unwrapped}
	start := f.startUnit(w)
	if w.branch != nil {
		branch := w.branch
		if len(branch.Effect) > 0 {
			q.units = append(q.units, entryUnit{label: effectLabel(branch), run: func(*entryQueue) error { return e.runBranchEffect(branch) }})
		}
		q.units = append(q.units, way...)
	} else if len(way) > 0 {
		start.after = way[len(way)-1].shared
	}
	q.units = append(q.units, start)
	return q
}

// addRegions puts the queues entering container's regions on the front ahead of
// the queue that entered it, so a run taking the first alternative at every draw
// enters them before it goes on. owner's do behavior starts once they are spent.
func (f *entryFront) addRegions(before *entryQueue, container, owner *ast.StateNode, regions []*ast.StateRegion, branches map[*ast.StateRegion]*ast.StateNode, wrap func(error) error) {
	if len(regions) == 0 {
		if owner != nil {
			f.exec.startDoAction(owner)
		}
		return
	}
	added := make([]*entryQueue, 0, len(regions))
	for _, region := range regions {
		entry := &regionEntry{region: region, container: container, branches: branches, target: branches[region]}
		q := &entryQueue{region: region, owner: owner, wrap: wrap}
		q.units = append(q.units, f.startUnit(entry))
		added = append(added, q)
	}
	at := len(f.queues)
	for i, q := range f.queues {
		if q == before {
			at = i
			break
		}
	}
	f.queues = append(f.queues[:at], append(added, f.queues[at:]...)...)
}

// startUnit resolves where the region starts, as the region's first entry, and
// enters the first state on the way there; the rest of the way follows in the queue.
func (f *entryFront) startUnit(w *regionEntry) entryUnit {
	e := f.exec
	return entryUnit{label: f.regionLabel(w), state: f.regionStartShown(w), run: func(q *entryQueue) error {
		entry, err := e.regionStart(w)
		if err != nil {
			return err
		}
		e.activeConfig.regionStates[w.region] = entry
		enter, plan := e.entryPlan(w.container, entry)
		if plan == nil {
			plan = w.branches
		} else {
			for region, state := range w.branches {
				plan[region] = state
			}
		}
		way := &entryWay{region: w.region, plan: plan, wrap: regionStartWrap(w.region)}
		return f.enterChain(q, way, e.descendantChain(w.container, enter), true)
	}}
}

// enterChain enters the first state of chain that logs its entry, along with the
// hidden ones ahead of it, then queues the rest ahead of q's other units: the
// states left, or what the last one starts. onWay tells the states a transition's
// way down, whose errors read alike, from the ones a body starts in.
func (f *entryFront) enterChain(q *entryQueue, way *entryWay, chain []*ast.StateNode, onWay bool) error {
	e := f.exec
	for len(chain) > 0 {
		i := 0
		for ; i < len(chain); i++ {
			state := chain[i]
			stepWrap := enterNamedWrap(state)
			if onWay {
				stepWrap = enterStateWrap
			}
			if err := f.enterState(q, way, state, onWay, wrapping(way.wrap, stepWrap)); err != nil {
				return way.wrap(stepWrap(err))
			}
			if !f.hidden(state) {
				i++
				break
			}
		}
		if rest := chain[i:]; len(rest) > 0 {
			f.queueAhead(q, f.chainUnit(way, rest, onWay))
			return nil
		}
		leaf := chain[len(chain)-1]
		if _, orthogonal := e.graph.CompositeStates[leaf]; orthogonal || len(e.graph.StartOf(leaf)) == 0 {
			return nil
		}
		if !f.hidden(leaf) {
			f.queueAhead(q, f.startOfUnit(way, leaf))
			return nil
		}
		// A hidden state stands for its region and is entered along with what it starts.
		start, err := e.startIn(leaf)
		if err != nil {
			return way.wrap(err)
		}
		chain, onWay = e.descendantChain(leaf, start), false
	}
	return nil
}

// queueAhead puts unit ahead of everything else q has left.
func (f *entryFront) queueAhead(q *entryQueue, unit entryUnit) {
	q.units = append([]entryUnit{unit}, q.units...)
}

// chainUnit enters the next state of a way down already resolved.
func (f *entryFront) chainUnit(way *entryWay, chain []*ast.StateNode, onWay bool) entryUnit {
	return entryUnit{
		label: f.chainLabel(chain, way.region),
		state: f.firstShown(chain),
		run:   func(q *entryQueue) error { return f.enterChain(q, way, chain, onWay) },
	}
}

// startOfUnit chooses the state leaf's body starts in, as leaf's own entry does,
// and enters the first state on the way there.
func (f *entryFront) startOfUnit(way *entryWay, leaf *ast.StateNode) entryUnit {
	e := f.exec
	unit := entryUnit{label: regionName(way.region)}
	if start := e.unguardedStart(leaf); start != nil {
		chain := e.descendantChain(leaf, start)
		unit.label, unit.state = f.chainLabel(chain, way.region), f.firstShown(chain)
	}
	unit.run = func(q *entryQueue) error {
		start, err := e.startIn(leaf)
		if err != nil {
			return way.wrap(err)
		}
		return f.enterChain(q, way, e.descendantChain(leaf, start), false)
	}
	return unit
}

// enterState performs one state's entry as a unit of the queue: its activation,
// then its do behavior, or its own regions as new queues of the front, started
// where the way's plan says when the state lies on the way.
func (f *entryFront) enterState(q *entryQueue, way *entryWay, state *ast.StateNode, onWay bool, wrap func(error) error) error {
	e := f.exec
	if err := e.activateState(state); err != nil {
		return err
	}
	e.activeConfig.regionStates[way.region] = state
	if regions, orthogonal := e.graph.CompositeStates[state]; orthogonal {
		var plan map[*ast.StateRegion]*ast.StateNode
		if onWay {
			plan = way.plan
		}
		f.addRegions(q, state, state, regions, plan, wrapping(q.wrap, wrap))
		return nil
	}
	e.startDoAction(state)
	return nil
}

// drain runs the front to the end: while two or more queues have a unit ready,
// the policy draws the queue that advances, and the draw is recorded.
func (f *entryFront) drain() error {
	e := f.exec
	for {
		live := f.live()
		if len(live) == 0 {
			for _, q := range f.queues {
				if len(q.units) > 0 {
					return fmt.Errorf("%s: region %s waits for a state no branch enters", f.where, q.region.Name)
				}
			}
			return nil
		}
		pick := 0
		if len(live) > 1 {
			var err error
			choice := ChoicePoint{Kind: ChoiceEntryOrder, Where: f.where, Alternatives: f.labels(live), File: e.stateMachine.DocName, Span: f.span}
			if pick, err = e.drawOrder(choice); err != nil {
				return err
			}
		}
		q := live[pick]
		unit := q.units[0]
		q.units = q.units[1:]
		if err := unit.run(q); err != nil {
			return q.wrap(err)
		}
		if unit.shared != nil {
			unit.shared.done = true
		}
		f.advance()
	}
}

// advance drops the shared units a sibling queue performed from the head of every
// queue and starts the do behavior of each owner whose queues are spent.
func (f *entryFront) advance() {
	for _, q := range f.queues {
		for len(q.units) > 0 && q.units[0].shared != nil && q.units[0].shared.done {
			q.units = q.units[1:]
		}
	}
	for _, q := range f.queues {
		f.settle(q)
	}
}

// live is every queue with a unit ready to run, in the front's order: a shared
// unit several queues reach is offered once, and a unit waiting for a shared one
// is held back until it has run.
func (f *entryFront) live() []*entryQueue {
	var live []*entryQueue
	offered := make(map[*sharedEntry]bool)
	for _, q := range f.queues {
		if len(q.units) == 0 {
			continue
		}
		head := q.units[0]
		if head.after != nil && !head.after.done {
			continue
		}
		if head.shared != nil {
			if offered[head.shared] {
				continue
			}
			offered[head.shared] = true
		}
		live = append(live, q)
	}
	return live
}

// settle starts the do behavior of the state whose regions q was among once
// every one of them is spent.
func (f *entryFront) settle(q *entryQueue) {
	if len(q.units) > 0 || q.owner == nil {
		return
	}
	for _, other := range f.queues {
		if other.owner == q.owner && len(other.units) > 0 {
			return
		}
	}
	f.exec.startDoAction(q.owner)
	for _, other := range f.queues {
		if other.owner == q.owner {
			other.owner = nil
		}
	}
}

// labels spells the head of each queue, qualified by its region where two heads
// enter states of one name, as typed regions' states are.
func (f *entryFront) labels(live []*entryQueue) []string {
	labels := make([]string, len(live))
	shared := make(map[string]int, len(live))
	for i, q := range live {
		labels[i] = q.units[0].label
		shared[labels[i]]++
	}
	for i, q := range live {
		state := q.units[0].state
		if state == nil || shared[labels[i]] < 2 {
			continue
		}
		if region := f.exec.graph.RegionOf[state]; region != nil && region.Name != "" {
			labels[i] = region.Name + "." + labels[i]
		}
	}
	return labels
}

// hidden reports a state that stands for a region of a parallel state and logs
// nothing on entry, so it is entered along with the state its body starts in.
func (f *entryFront) hidden(state *ast.StateNode) bool {
	return f.exec.graph.HiddenStates[state] && len(f.exec.behaviorsOf(state).Entry) == 0
}

// grouped cuts chain into the runs a trace tells apart: the hidden states ahead
// of each state that logs its entry go with it, and trailing hidden ones together.
func (f *entryFront) grouped(chain []*ast.StateNode) [][]*ast.StateNode {
	var groups [][]*ast.StateNode
	from := 0
	for i, state := range chain {
		if !f.hidden(state) {
			groups = append(groups, chain[from:i+1])
			from = i + 1
		}
	}
	if from < len(chain) {
		groups = append(groups, chain[from:])
	}
	return groups
}

// firstShown is the first state of chain whose entry a trace records.
func (f *entryFront) firstShown(chain []*ast.StateNode) *ast.StateNode {
	for _, state := range chain {
		if !f.hidden(state) {
			return state
		}
	}
	return nil
}

// effectLabel names a transition's effect as a unit: `T2.1(effect)`.
func effectLabel(trans *lower.Transition) string {
	name := trans.Name
	if name == "" {
		name = orAny(StateVertexName(trans.Source)) + " -> " + orAny(StateVertexName(trans.Target))
	}
	return name + "(effect)"
}

// stateEntryLabel names a state's entry as a unit: `left(entry)`.
func stateEntryLabel(state *ast.StateNode) string {
	return state.Name + "(entry)"
}

// regionName labels a unit that logs nothing by its region.
func regionName(region *ast.StateRegion) string {
	if region == nil || region.Name == "" {
		return "region"
	}
	return "region " + region.Name
}

// chainLabel names the first entry of chain a trace records, or the region when
// none is.
func (f *entryFront) chainLabel(chain []*ast.StateNode, region *ast.StateRegion) string {
	if state := f.firstShown(chain); state != nil {
		return stateEntryLabel(state)
	}
	return regionName(region)
}

// regionLabel names the first entry a region logs, when the graph fixes it, and
// the region otherwise: where it starts may turn on a guard read as it starts.
func (f *entryFront) regionLabel(w *regionEntry) string {
	if state := f.regionStartShown(w); state != nil {
		return stateEntryLabel(state)
	}
	return regionName(w.region)
}

// regionStartShown is the first state whose entry a region logs when it starts,
// when the graph fixes it without a guard, and nil otherwise.
func (f *entryFront) regionStartShown(w *regionEntry) *ast.StateNode {
	e := f.exec
	start := w.target
	if start == nil {
		start = e.graph.RegionState[w.region]
	}
	if start == nil {
		start = e.unguardedStart(w.region)
	}
	if start == nil {
		return nil
	}
	chain := e.descendantChain(w.container, start)
	// A hidden start is entered along with the state its body starts in.
	for len(chain) > 0 && f.hidden(chain[len(chain)-1]) {
		next := e.unguardedStart(chain[len(chain)-1])
		if next == nil {
			break
		}
		chain = append(chain, e.descendantChain(chain[len(chain)-1], next)...)
	}
	return f.firstShown(chain)
}
