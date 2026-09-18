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

// entryWay is the way a queue's units are on: the region they enter, where the
// regions below start, and how an error on the way reads.
type entryWay struct {
	region *ast.StateRegion
	plan   map[*ast.StateRegion]*ast.StateNode
	wrap   func(error) error
}

// entryFront is the front entering one composite's regions or one fork's branches.
func (e *StateExecutor) entryFront(where string, span source.Span) *orderFront {
	return &orderFront{exec: e, kind: ChoiceEntryOrder, where: where, span: span}
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
	front := e.entryFront(enteringWherePrefix+e.stateMachine.Name, e.stateMachine.DeclSpan)
	if container != nil {
		front.where, front.span = enteringWherePrefix+container.Name, container.Span()
	}
	e.addRegions(front, nil, container, nil, regions, branches, unwrapped)
	return front.drain()
}

// enterForkBranches enters the owner's regions through the fork's branches, one
// queue per region: a branch's effect, the way down to the owner as far as no
// sibling entered it, then its target; a region no branch enters starts as usual
// once the owner is entered. The do behaviors of the states entered on the way
// down start once all have, innermost first, as after an ordinary entry.
func (e *StateExecutor) enterForkBranches(fork *ast.PseudostateNode, plan *lower.ForkPlan, above *lazyEntry) error {
	targets := plan.Targets()
	front := e.entryFront(forkWherePrefix+fork.Name, fork.Span())
	way := e.wayDown(front, above)
	for _, region := range e.graph.CompositeStates[plan.Owner] {
		entry := &regionEntry{region: region, container: plan.Owner, branches: targets}
		if branch := plan.Branches[region]; branch != nil {
			entry.target, entry.branch = branch.Target.(*ast.StateNode), branch
		}
		front.queues = append(front.queues, e.branchQueue(front, entry, way))
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

func unwrapped(err error) error { return err }

// mayDrawOrder reports whether entering or leaving target may draw an order: an
// orthogonal state of two or more regions is target, above it or below it, or
// the machine's own regions are orthogonal.
func (e *StateExecutor) mayDrawOrder(target *ast.StateNode) bool {
	if target == nil {
		return false
	}
	if len(e.graph.TopRegions) >= 2 {
		return true
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

func enterStateWrap(err error) error { return fmt.Errorf("enter state: %w", err) }

func enterNamedWrap(state *ast.StateNode) func(error) error {
	return func(err error) error { return fmt.Errorf("enter state %s: %w", state.Name, err) }
}

func regionStartWrap(region *ast.StateRegion) func(error) error {
	return func(err error) error { return fmt.Errorf("enter starting state in region %s: %w", region.Name, err) }
}

// wayDown is the units entering the states of a fork's way down not yet active,
// shared by every branch's queue; nil when the owner is active already.
func (e *StateExecutor) wayDown(f *orderFront, above *lazyEntry) []orderUnit {
	var way []orderUnit
	for _, group := range e.groupedEntries(above.chain[above.entered:]) {
		way = append(way, e.wayUnit(f, above, group))
	}
	return way
}

// wayUnit enters a group of states on a fork's way down, once for every branch:
// each is activated without its do behavior, and its regions off the way start
// as queues of the front ahead of the queue that reached it.
func (e *StateExecutor) wayUnit(f *orderFront, above *lazyEntry, group []*ast.StateNode) orderUnit {
	unit := orderUnit{label: e.chainLabel(group, nil), state: e.firstEntryShown(group), shared: &sharedUnit{}}
	unit.run = func(q *orderQueue) error {
		for _, state := range group {
			if err := e.activateState(state); err != nil {
				return enterNamedWrap(state)(err)
			}
			if state != above.owner {
				e.addRegions(f, q, state, nil, e.offWay(state, above), above.branches, enterNamedWrap(state))
			}
		}
		return nil
	}
	return unit
}

// branchQueue is the queue entering w through a fork: its branch's effect if it
// has one, the way down to its container, then its target; a region no branch
// enters waits for the way down to be walked and then starts as usual.
func (e *StateExecutor) branchQueue(f *orderFront, w *regionEntry, way []orderUnit) *orderQueue {
	q := &orderQueue{region: w.region, wrap: unwrapped}
	start := e.startUnit(f, w)
	if w.branch != nil {
		branch := w.branch
		if len(branch.Effect) > 0 {
			q.units = append(q.units, orderUnit{label: effectLabel(branch), run: func(*orderQueue) error { return e.runBranchEffect(branch) }})
		}
		q.units = append(q.units, way...)
	} else if len(way) > 0 {
		start.after = &way[len(way)-1].shared.unitGate
	}
	q.units = append(q.units, start)
	return q
}

// addRegions puts the queues entering container's regions on the front ahead of
// the queue that entered it, so a run taking the first alternative at every draw
// enters them before it goes on. owner's do behavior starts once they are spent.
func (e *StateExecutor) addRegions(f *orderFront, before *orderQueue, container, owner *ast.StateNode, regions []*ast.StateRegion, branches map[*ast.StateRegion]*ast.StateNode, wrap func(error) error) {
	if len(regions) == 0 {
		if owner != nil {
			e.startDoAction(owner)
		}
		return
	}
	var group *queueGroup
	if owner != nil {
		group = &queueGroup{left: len(regions), then: func() { e.startDoAction(owner) }}
	}
	added := make([]*orderQueue, 0, len(regions))
	for _, region := range regions {
		entry := &regionEntry{region: region, container: container, branches: branches, target: branches[region]}
		q := &orderQueue{region: region, group: group, wrap: wrap}
		q.units = append(q.units, e.startUnit(f, entry))
		added = append(added, q)
	}
	f.insert(before, added)
}

// startUnit resolves where the region starts, as the region's first entry, and
// enters the first state on the way there; the rest of the way follows in the queue.
func (e *StateExecutor) startUnit(f *orderFront, w *regionEntry) orderUnit {
	return orderUnit{label: e.regionLabel(w), state: e.regionStartShown(w), run: func(q *orderQueue) error {
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
		return e.enterChain(f, q, way, e.descendantChain(w.container, enter), true)
	}}
}

// enterChain enters the first state of chain that logs its entry, along with the
// hidden ones ahead of it, then queues the rest ahead of q's other units: the
// states left, or what the last one starts. onWay tells the states a transition's
// way down, whose errors read alike, from the ones a body starts in.
func (e *StateExecutor) enterChain(f *orderFront, q *orderQueue, way *entryWay, chain []*ast.StateNode, onWay bool) error {
	for len(chain) > 0 {
		i := 0
		for ; i < len(chain); i++ {
			state := chain[i]
			stepWrap := enterNamedWrap(state)
			if onWay {
				stepWrap = enterStateWrap
			}
			if err := e.enterUnit(f, q, way, state, onWay, wrapping(way.wrap, stepWrap)); err != nil {
				return way.wrap(stepWrap(err))
			}
			if !e.hiddenEntry(state) {
				i++
				break
			}
		}
		if rest := chain[i:]; len(rest) > 0 {
			q.ahead(e.chainUnit(f, way, rest, onWay))
			return nil
		}
		leaf := chain[len(chain)-1]
		if _, orthogonal := e.graph.CompositeStates[leaf]; orthogonal || len(e.graph.StartOf(leaf)) == 0 {
			return nil
		}
		if !e.hiddenEntry(leaf) {
			q.ahead(e.startOfUnit(f, way, leaf))
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

// chainUnit enters the next state of a way down already resolved.
func (e *StateExecutor) chainUnit(f *orderFront, way *entryWay, chain []*ast.StateNode, onWay bool) orderUnit {
	return orderUnit{
		label: e.chainLabel(chain, way.region),
		state: e.firstEntryShown(chain),
		run:   func(q *orderQueue) error { return e.enterChain(f, q, way, chain, onWay) },
	}
}

// startOfUnit chooses the state leaf's body starts in, as leaf's own entry does,
// and enters the first state on the way there.
func (e *StateExecutor) startOfUnit(f *orderFront, way *entryWay, leaf *ast.StateNode) orderUnit {
	unit := orderUnit{label: regionName(way.region)}
	if start := e.unguardedStart(leaf); start != nil {
		chain := e.descendantChain(leaf, start)
		unit.label, unit.state = e.chainLabel(chain, way.region), e.firstEntryShown(chain)
	}
	unit.run = func(q *orderQueue) error {
		start, err := e.startIn(leaf)
		if err != nil {
			return way.wrap(err)
		}
		return e.enterChain(f, q, way, e.descendantChain(leaf, start), false)
	}
	return unit
}

// enterUnit performs one state's entry as a unit of the queue: its activation,
// then its do behavior, or its own regions as new queues of the front, started
// where the way's plan says when the state lies on the way.
func (e *StateExecutor) enterUnit(f *orderFront, q *orderQueue, way *entryWay, state *ast.StateNode, onWay bool, wrap func(error) error) error {
	if err := e.activateState(state); err != nil {
		return err
	}
	e.activeConfig.regionStates[way.region] = state
	if regions, orthogonal := e.graph.CompositeStates[state]; orthogonal {
		var plan map[*ast.StateRegion]*ast.StateNode
		if onWay {
			plan = way.plan
		}
		e.addRegions(f, q, state, state, regions, plan, wrapping(q.wrap, wrap))
		return nil
	}
	e.startDoAction(state)
	return nil
}

// hiddenEntry reports a state that stands for a region of a parallel state and
// logs nothing on entry, so it is entered along with the state its body starts in.
func (e *StateExecutor) hiddenEntry(state *ast.StateNode) bool {
	return e.graph.HiddenStates[state] && len(e.behaviorsOf(state).Entry) == 0
}

// groupedEntries cuts a way down into the units a trace tells apart: the hidden
// states ahead of each state that logs its entry go with it, trailing hidden ones together.
func (e *StateExecutor) groupedEntries(chain []*ast.StateNode) [][]*ast.StateNode {
	var groups [][]*ast.StateNode
	from := 0
	for i, state := range chain {
		if !e.hiddenEntry(state) {
			groups = append(groups, chain[from:i+1])
			from = i + 1
		}
	}
	if from < len(chain) {
		groups = append(groups, chain[from:])
	}
	return groups
}

// firstEntryShown is the first state of chain whose entry a trace records.
func (e *StateExecutor) firstEntryShown(chain []*ast.StateNode) *ast.StateNode {
	for _, state := range chain {
		if !e.hiddenEntry(state) {
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
func (e *StateExecutor) chainLabel(chain []*ast.StateNode, region *ast.StateRegion) string {
	if state := e.firstEntryShown(chain); state != nil {
		return stateEntryLabel(state)
	}
	return regionName(region)
}

// regionLabel names the first entry a region logs, when the graph fixes it, and
// the region otherwise: where it starts may turn on a guard read as it starts.
func (e *StateExecutor) regionLabel(w *regionEntry) string {
	if state := e.regionStartShown(w); state != nil {
		return stateEntryLabel(state)
	}
	return regionName(w.region)
}

// regionStartShown is the first state whose entry a region logs when it starts,
// when the graph fixes it without a guard, and nil otherwise.
func (e *StateExecutor) regionStartShown(w *regionEntry) *ast.StateNode {
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
	for len(chain) > 0 && e.hiddenEntry(chain[len(chain)-1]) {
		next := e.unguardedStart(chain[len(chain)-1])
		if next == nil {
			break
		}
		chain = append(chain, e.descendantChain(chain[len(chain)-1], next)...)
	}
	return e.firstEntryShown(chain)
}
