package runtime

import (
	"errors"
	"fmt"
	"iter"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A front performs the units of several regions one at a time, drawing which advances while two
// or more have one; each region is a coroutine yielding before every unit. It lives within one move.

// errFrontClosed is what a queue's unit returns once its front closed under it.
var errFrontClosed = errors.New("front closed")

// An entry order's Where names the composite or fork entered, an exit order's the composite
// exited, and a region order among firing units the occurrence they react to.
const (
	firingWherePrefix   = "on "
	enteringWherePrefix = "entering "
	forkWherePrefix     = "fork "
	exitingWherePrefix  = "exiting "
)

// entryLabel, exitLabel and effectLabel spell a unit as a PSSM trace does; a state sharing
// its name with another region's is told apart by its region, as stateNames does.
func (e *StateExecutor) entryLabel(state *ast.StateNode) string {
	return e.stateName(state) + "(entry)"
}
func (e *StateExecutor) exitLabel(state *ast.StateNode) string { return e.stateName(state) + "(exit)" }
func (e *StateExecutor) effectLabel(trans *lower.Transition) string {
	return e.transitionLabel(trans) + "(effect)"
}

// stateName is the state's name, qualified by its region where another region's state shares it.
func (e *StateExecutor) stateName(state *ast.StateNode) string {
	if region := e.graph.RegionOf[state]; region != nil && region.Name != "" && e.nameShared(state) {
		return region.Name + "." + state.Name
	}
	return state.Name
}

// vertexName is StateVertexName with a state's name qualified as stateName does.
func (e *StateExecutor) vertexName(node ast.Node) string {
	if state, isState := node.(*ast.StateNode); isState {
		return e.stateName(state)
	}
	return StateVertexName(node)
}

// entryIsUnit: every visible state's entry is a unit; a hidden owner's only when it performs.
func (e *StateExecutor) entryIsUnit(state *ast.StateNode) bool {
	return !e.graph.HiddenStates[state] || len(e.behaviorsOf(state).Entry) > 0
}

// exitIsUnit is entryIsUnit for leaving state.
func (e *StateExecutor) exitIsUnit(state *ast.StateNode) bool {
	return !e.graph.HiddenStates[state] || len(e.behaviorsOf(state).Exit) > 0
}

// silentEntry reports whether entering state performs no behavior: nothing a
// sibling region's unit could observe or be observed by.
func (e *StateExecutor) silentEntry(state *ast.StateNode) bool {
	behaviors := e.behaviorsOf(state)
	return len(behaviors.Entry) == 0 && len(behaviors.Do) == 0
}

// entryHead is the unit entering state; one queuing a completion (leaf) is observable through
// the pool's order, so it is drawn even when the state performs nothing.
func (e *StateExecutor) entryHead(state *ast.StateNode, leaf bool) unitHead {
	var body ast.Node
	if parent := e.graph.ParentState[state]; parent != nil {
		body = parent
	}
	return unitHead{label: e.entryLabel(state), at: state, site: e.bodySite(body), silent: !leaf && e.silentEntry(state)}
}

// bodySite names the entry site of a body's units as a front entering it would: the state
// whose body it is, or the machine.
func (e *StateExecutor) bodySite(body ast.Node) string {
	switch body := body.(type) {
	case *ast.StateNode:
		return enteringWherePrefix + body.Name
	case *ast.StateRegion:
		if owner := e.graph.RegionOwner[body]; owner != nil {
			return enteringWherePrefix + owner.Name
		}
	}
	return enteringWherePrefix + e.stateMachine.Name
}

// silentExit is silentEntry for leaving state.
func (e *StateExecutor) silentExit(state *ast.StateNode) bool {
	behaviors := e.behaviorsOf(state)
	return len(behaviors.Exit) == 0 && len(behaviors.Do) == 0
}

// transitionLabel names a transition by its own name, or by its ends when it has none.
func (e *StateExecutor) transitionLabel(trans *lower.Transition) string {
	if trans.Name != "" {
		return trans.Name
	}
	return e.vertexName(trans.Source) + "->" + e.vertexName(trans.Target)
}

// unitFront is one such site under way: its queues in canonical order, the queue
// whose coroutine is running, and the front it is nested in.
type unitFront struct {
	exec    *StateExecutor
	kind    ChoiceKind
	where   string
	queues  []*unitQueue
	current *unitQueue
	outer   *unitFront
}

// unitQueue is one region's remaining units: the coroutine performing them and the head it
// yielded. A queue drawn for a unit it has yet to name is prepaid; one spawned at a head starts when drawn.
type unitQueue struct {
	front   *unitFront
	head    unitHead
	done    bool
	closed  bool
	err     error
	body    func() error
	next    func() (unitHead, bool)
	stop    func()
	yield   func(unitHead) bool
	perform bool
	prepaid bool
	firing  firingScope
}

// unitHead is a queue's next unit: shared (performed once, by the first drawn), waiting (until
// holds), void (a disabled firing, run last performing nothing), silent (rides with the next
// performing unit) or a do step (one token move, offered while a sibling has a unit left).
// site names the body the unit enters, the Where of its draw when no front orders it.
type unitHead struct {
	label   string
	at      ast.Node
	site    string
	shared  *ast.StateNode
	dropped func() bool
	until   func() bool
	void    func() bool
	silent  bool
	doStep  bool
}

// pathDraw is the draw along an entry path no front orders: once the path is drawn over the
// due do steps it rides its silent units, as a front's queue does, until it has performed one.
type pathDraw struct {
	drawn     bool
	performed bool
	prepaid   bool
	due       []*doAction
}

// firingScope is the state of a queue's firing (occurrence, what it left and entered ahead,
// mark, bound trigger arguments), kept aside while another queue's units run.
type firingScope struct {
	event        *Event
	taken        *lower.Transition
	change       *lower.Transition
	notes        []RunNote
	leftAhead    map[*ast.StateNode]bool
	exitingAhead bool
	enteredAhead map[*ast.StateNode]bool
	moving       *moveMark
	// bound are the machine's data the firing bound, installed while its units
	// run; shadowed what the data held under each while installed.
	bound    map[string]Value
	shadowed map[string]dataSlot
}

// dataSlot is one entry of the machine's data as it was, or that it was absent.
type dataSlot struct {
	value Value
	held  bool
}

// firing captures the firing the executor is in the middle of.
func (e *StateExecutor) firing() firingScope {
	return firingScope{
		event: e.firingEvent, taken: e.firingTrans, change: e.firingChange, notes: e.firingNotes,
		leftAhead: e.leftAhead, exitingAhead: e.exitingAhead, enteredAhead: e.enteredAhead,
		moving: e.moving,
	}
}

// setFiring puts the executor back in the middle of the firing, keeping what the
// firing had bound as it is.
func (e *StateExecutor) setFiring(s *firingScope) {
	e.firingEvent, e.firingTrans, e.firingChange, e.firingNotes = s.event, s.taken, s.change, s.notes
	e.leftAhead, e.exitingAhead, e.enteredAhead = s.leftAhead, s.exitingAhead, s.enteredAhead
	e.moving = s.moving
}

// install puts the data the firing bound in place, remembering what was under it.
func (s *firingScope) install(e *StateExecutor) {
	for name, value := range s.bound {
		prior, held := e.stateData[name]
		s.shadowed[name] = dataSlot{value: prior, held: held}
		e.stateData[name] = value
	}
}

// uninstall takes the data the firing bound out, putting back what was under it.
func (s *firingScope) uninstall(e *StateExecutor) {
	for name := range s.bound {
		s.bound[name] = e.stateData[name]
		s.restore(e, name)
	}
}

// restore puts back what the data held under the firing's binding of name.
func (s *firingScope) restore(e *StateExecutor, name string) {
	if slot := s.shadowed[name]; slot.held {
		e.stateData[name] = slot.value
	} else {
		delete(e.stateData, name)
	}
}

// runningQueue is the queue whose units are running, nil outside a front.
func (e *StateExecutor) runningQueue() *unitQueue {
	if e.front == nil {
		return nil
	}
	return e.front.current
}

// bindData binds one of the machine's data for the firing under way to read.
func (e *StateExecutor) bindData(name string, value Value) {
	e.stateData[name] = value
	if q := e.runningQueue(); q != nil {
		q.firing.bound[name] = value
	}
}

// restoreData snapshots the named data entries and returns the function putting them back,
// deleting the ones that were absent; a front swaps a firing's bindings in and out this way.
func (e *StateExecutor) restoreData(names []string) func() {
	q := e.runningQueue()
	if q == nil {
		return e.restoreSharedData(names)
	}
	s := &q.firing
	for _, name := range names {
		if _, bound := s.bound[name]; bound {
			continue
		}
		prior, held := e.stateData[name]
		s.shadowed[name] = dataSlot{value: prior, held: held}
		s.bound[name] = prior
	}
	return func() {
		for _, name := range names {
			if _, bound := s.bound[name]; !bound {
				continue
			}
			s.restore(e, name)
			delete(s.bound, name)
			delete(s.shadowed, name)
		}
	}
}

// accepts reports whether a front of the kind orders units of the given kind: a
// firing's units are entries, exits and effects alike.
func (f *unitFront) accepts(kind ChoiceKind) bool {
	return f.kind == kind || f.kind == ChoiceRegionOrder
}

// openFront begins a site nested in whatever front is under way.
func (e *StateExecutor) openFront(kind ChoiceKind, where string) *unitFront {
	f := &unitFront{exec: e, kind: kind, where: where, outer: e.front}
	e.front = f
	return f
}

// performUnits performs the bodies as queues of a front of the kind (the one under way, waited
// for when wait says, or a front of their own at where); one body alone runs as it stands.
func (e *StateExecutor) performUnits(kind ChoiceKind, where string, bodies []func() error, wait bool) error {
	if e.inFront(kind) {
		return e.spawnUnits(kind, bodies, wait)
	}
	if len(bodies) < 2 {
		for _, body := range bodies {
			if err := body(); err != nil {
				return err
			}
		}
		return nil
	}
	return e.drawUnits(kind, where, bodies)
}

// drawUnits performs the bodies as the queues of a front of their own; an entry front also
// offers the due steps of the do behaviors the move began before it opened.
func (e *StateExecutor) drawUnits(kind ChoiceKind, where string, bodies []func() error) error {
	f := e.openFront(kind, where)
	if f.accepts(ChoiceEntryOrder) {
		e.path = pathDraw{}
		for _, act := range e.dueAmong(e.began) {
			f.offer(act)
		}
	}
	for _, body := range bodies {
		f.spawn(body)
	}
	return f.drain()
}

// spawnUnits adds the bodies as queues of the front under way, and parks the
// running queue until they are done when wait says.
func (e *StateExecutor) spawnUnits(kind ChoiceKind, bodies []func() error, wait bool) error {
	queues := make([]*unitQueue, 0, len(bodies))
	for _, body := range bodies {
		queues = append(queues, e.front.spawn(body))
	}
	if !wait {
		return nil
	}
	return e.await(kind, func() bool {
		for _, q := range queues {
			if !q.done {
				return false
			}
		}
		return true
	})
}

// close ends the front, stopping whatever its queues had left.
func (f *unitFront) close() {
	for _, q := range f.queues {
		q.closed = true
		if !q.done && q.stop != nil {
			q.stop()
		}
	}
	f.exec.front = f.outer
}

// spawn adds a queue performing body and runs it to its first head; a queue spawned by a
// running one goes right before it, as the nested region was entered before its spawner went on.
func (f *unitFront) spawn(body func() error) *unitQueue {
	q := f.add(body)
	q.start()
	f.resume(q, false)
	return q
}

// spawnAt adds a queue whose first unit head names; body runs when the queue is first drawn,
// performing that unit without a draw of its own.
func (f *unitFront) spawnAt(head unitHead, body func() error) *unitQueue {
	q := f.add(body)
	q.head = head
	return q
}

// offer adds a queue drawing each due step of a do behavior begun in the move against the
// other queues' units; a step is never silent, and only the one-move engines step this fine.
func (f *unitFront) offer(act *doAction) {
	if !f.exec.ctx.scheduling().oneMove() {
		return
	}
	var q *unitQueue
	q = f.spawnAt(f.doStepHead(act), func() error { return f.offerDoSteps(q, act) })
	q.head.void = func() bool { return !f.offering(q, act) }
}

// doStepHead is the unit moving one token of the do behavior.
func (f *unitFront) doStepHead(act *doAction) unitHead {
	return unitHead{label: doStepLabel([]string{f.exec.stateName(act.state)}), at: act.state, doStep: true}
}

// add places a queue for body in the front, before the running queue.
func (f *unitFront) add(body func() error) *unitQueue {
	q := &unitQueue{front: f, body: body, firing: f.exec.firing()}
	q.firing.bound, q.firing.shadowed = make(map[string]Value), make(map[string]dataSlot)
	at := len(f.queues)
	if i := slices.Index(f.queues, f.current); i >= 0 {
		at = i
	}
	f.queues = slices.Insert(f.queues, at, q)
	return q
}

// start begins the queue's coroutine.
func (q *unitQueue) start() {
	q.next, q.stop = iter.Pull(func(yield func(unitHead) bool) {
		q.yield = yield
		q.err = q.body()
	})
}

// resume runs the queue to its next head, performing the unit at its head or
// dropping it as perform says, in the firing the queue's units belong to.
func (f *unitFront) resume(q *unitQueue, perform bool) {
	prev := f.current
	f.current, q.perform = q, perform
	if q.next == nil {
		q.start()
		q.prepaid = true
	}
	outer := f.exec.firing()
	f.exec.setFiring(&q.firing)
	q.firing.install(f.exec)
	head, ok := q.next()
	q.firing.uninstall(f.exec)
	bound, shadowed := q.firing.bound, q.firing.shadowed
	q.firing = f.exec.firing()
	q.firing.bound, q.firing.shadowed = bound, shadowed
	f.exec.setFiring(&outer)
	f.current = prev
	if !ok {
		q.done = true
		return
	}
	q.head = head
}

// unit yields before a unit of the running queue and reports whether to perform it; a unit
// no front of this kind orders is performed at once, an entry drawn against the due do steps.
func (e *StateExecutor) unit(kind ChoiceKind, head unitHead) (bool, error) {
	f := e.front
	if f == nil || !f.accepts(kind) || f.current == nil {
		if kind == ChoiceEntryOrder {
			return true, e.drawOnPath(head)
		}
		return true, nil
	}
	q := f.current
	if q.closed {
		return false, errFrontClosed
	}
	if q.prepaid {
		q.prepaid = false
		return q.perform, nil
	}
	if !q.yield(head) {
		return false, errFrontClosed
	}
	return q.perform, nil
}

// unitAhead draws a unit whose name the run learns only by performing it: the
// draw is made under the given label and pays for the unit's own draw.
func (e *StateExecutor) unitAhead(kind ChoiceKind, head unitHead) error {
	f := e.front
	if f == nil || !f.accepts(kind) || f.current == nil {
		if kind != ChoiceEntryOrder {
			return nil
		}
		if err := e.drawOnPath(head); err != nil {
			return err
		}
		e.path.prepaid = true
		return nil
	}
	if _, err := e.unit(kind, head); err != nil {
		return err
	}
	f.current.prepaid = true
	return nil
}

// drawOnPath draws a unit of an entry path no front orders against the due do steps the move
// began, moving the steps drawn until the unit is; a unit riding a drawn one is not drawn.
func (e *StateExecutor) drawOnPath(head unitHead) error {
	p := &e.path
	if p.prepaid {
		p.prepaid = false
		return nil
	}
	if !e.ctx.scheduling().oneMove() {
		return nil
	}
	for {
		due := e.dueAmong(e.began)
		if len(due) == 0 {
			*p = pathDraw{}
			return nil
		}
		if p.drawn && slices.Equal(due, p.due) && !(p.performed && !head.silent) {
			p.performed = p.performed || !head.silent
			return nil
		}
		alternatives := make([]string, 0, len(due)+1)
		for _, act := range due {
			alternatives = append(alternatives, doStepLabel([]string{e.stateName(act.state)}))
		}
		alternatives = append(alternatives, head.label)
		choice := ChoicePoint{Kind: ChoiceEntryOrder, Where: head.site, Alternatives: alternatives, File: e.stateMachine.DocName}
		pick := e.ctx.scheduling().choose(choice, nil)
		if err := e.ctx.scheduling().refusal(); err != nil {
			return err
		}
		choice.Taken = pick
		at := head.at
		if pick < len(due) {
			at = due[pick].state
		}
		if at != nil {
			choice.Span = at.Span()
		}
		e.noteChoice(choice)
		if pick == len(due) {
			*p = pathDraw{drawn: true, performed: !head.silent, due: due}
			return nil
		}
		if err := e.moveDoStep(due[pick]); err != nil {
			return err
		}
	}
}

// inFront reports whether a queue of a front of the given kind is running, so a
// site nested in it adds its queues to that front rather than opening one.
func (e *StateExecutor) inFront(kind ChoiceKind) bool {
	return e.front != nil && e.front.accepts(kind) && e.front.current != nil
}

// await parks the running queue until the condition holds; no unit of its own
// is performed until then.
func (e *StateExecutor) await(kind ChoiceKind, until func() bool) error {
	f := e.front
	if f == nil || !f.accepts(kind) || f.current == nil || until() {
		return nil
	}
	if f.current.closed {
		return errFrontClosed
	}
	if !f.current.yield(unitHead{until: until}) {
		return errFrontClosed
	}
	return nil
}

// drain performs the units of every queue, one at a time, drawing the queue that
// advances while two or more have a unit, and closes the front.
func (f *unitFront) drain() (err error) {
	defer f.close()
	for {
		f.settle()
		for _, q := range f.queues {
			if q.done && q.err != nil {
				return q.err
			}
		}
		ready := f.ready()
		if len(ready) == 0 {
			if f.drainVoid() {
				continue
			}
			if f.finished() {
				return nil
			}
			return fmt.Errorf("%s: every region waits on another", f.where)
		}
		pick := 0
		if len(ready) >= 2 {
			choice := ChoicePoint{Kind: f.kind, Where: f.where, Alternatives: f.labels(ready), File: f.exec.stateMachine.DocName}
			pick = f.exec.ctx.scheduling().choose(choice, nil)
			if err := f.exec.ctx.scheduling().refusal(); err != nil {
				return err
			}
			choice.Taken = pick
			if at := ready[pick].head.at; at != nil {
				choice.Span = at.Span()
			}
			f.exec.noteChoice(choice)
		}
		f.advance(ready[pick])
	}
}

// advance performs the drawn queue's next performing unit and the silent units around it,
// stopping early where a sibling's readiness changes so `declared` keeps its sequence.
func (f *unitFront) advance(q *unitQueue) {
	others := f.readyExcept(q)
	performed := false
	for {
		silent := q.head.silent
		f.resume(q, true)
		performed = performed || !silent
		f.settle()
		if q.done || !slices.Contains(f.ready(), q) || !slices.Equal(others, f.readyExcept(q)) {
			return
		}
		if performed && !q.head.silent {
			return
		}
	}
}

// offerDoSteps yields each due token move of the do behavior as a unit of q while a sibling has a
// unit left.
func (f *unitFront) offerDoSteps(q *unitQueue, act *doAction) error {
	e := f.exec
	for f.offering(q, act) {
		head := f.doStepHead(act)
		head.void = func() bool { return !f.offering(q, act) }
		perform, err := e.unit(f.kind, head)
		if err != nil || !perform {
			return err
		}
		if err := e.moveDoStep(act); err != nil {
			return err
		}
	}
	return nil
}

// offering reports whether q still has a step of the do behavior to offer: the behavior is
// running and due, and a sibling has a unit left.
func (f *unitFront) offering(q *unitQueue, act *doAction) bool {
	return len(f.exec.dueAmong([]*doAction{act})) > 0 && f.unitsPending(q)
}

// moveDoStep moves one token of the do behavior as a unit of an entry site does.
func (e *StateExecutor) moveDoStep(act *doAction) error {
	if err := e.stepDoAction(act, func(run *doRun) (*doRun, error) { return run.resume(e.ctx) }); err != nil {
		return err
	}
	if err := e.countDoStep(); err != nil {
		return err
	}
	return e.settleDoActions()
}

// unitsPending reports a queue other than q with a unit of its own left: neither done, waiting,
// void, nor offering a do step.
func (f *unitFront) unitsPending(q *unitQueue) bool {
	for _, r := range f.queues {
		if r == q || r.done || r.head.until != nil || r.head.doStep {
			continue
		}
		if r.head.void != nil && r.head.void() {
			continue
		}
		return true
	}
	return false
}

// readyExcept lists the ready queues other than q.
func (f *unitFront) readyExcept(q *unitQueue) []*unitQueue {
	return slices.DeleteFunc(f.ready(), func(r *unitQueue) bool { return r == q })
}

// settle resumes the queues whose wait is over and drops the shared units a
// sibling performed, until every head is a unit to perform or a wait still on.
func (f *unitFront) settle() {
	for moved := true; moved; {
		moved = false
		for _, q := range f.queues {
			switch {
			case q.done:
			case q.head.until != nil && q.head.until():
				f.resume(q, true)
				moved = true
			case q.head.dropped != nil && q.head.dropped():
				f.resume(q, false)
				moved = true
			}
		}
	}
}

// ready lists the queues with a unit to perform, a shared unit once, by the
// first queue heading it.
func (f *unitFront) ready() []*unitQueue {
	var ready []*unitQueue
	for _, q := range f.queues {
		if q.done || q.head.until != nil || (q.head.void != nil && q.head.void()) {
			continue
		}
		if q.head.shared != nil && slices.ContainsFunc(ready, func(r *unitQueue) bool { return r.head.shared == q.head.shared }) {
			continue
		}
		ready = append(ready, q)
	}
	return ready
}

// drainVoid runs the queues void of a unit to their ends, performing nothing,
// and reports whether there were any.
func (f *unitFront) drainVoid() bool {
	drained := false
	for _, q := range f.queues {
		if !q.done && q.head.void != nil && q.head.void() {
			f.resume(q, false)
			drained = true
		}
	}
	return drained
}

// finished reports whether every queue ran to its end.
func (f *unitFront) finished() bool {
	for _, q := range f.queues {
		if !q.done {
			return false
		}
	}
	return true
}

// labels spells the ready queues' next units, the draw's alternatives.
func (f *unitFront) labels(ready []*unitQueue) []string {
	labels := make([]string, len(ready))
	for i, q := range ready {
		labels[i] = q.head.label
	}
	return labels
}

// nameShared reports whether a state of another region bears this state's name.
func (e *StateExecutor) nameShared(state *ast.StateNode) bool {
	for other, region := range e.graph.RegionOf {
		if other != state && other.Name == state.Name && region != e.graph.RegionOf[state] {
			return true
		}
	}
	return false
}
