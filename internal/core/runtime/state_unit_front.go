package runtime

import (
	"errors"
	"fmt"
	"iter"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
)

// A front is where the executor performs the units of several regions — a
// fork's branches, the regions of a composite it enters or exits, the firings
// one occurrence selected — one at a time, drawing which region's next unit runs
// while two or more have one. Each region's units run in the order the executor
// always performed them, as a coroutine that yields before every unit; the front
// resumes the one drawn. Under `declared` the lowest queue with a unit is drawn,
// so the front performs the very sequence the executor did before it drew anything.
// A front lives within one move: a refused draw fails the move, whose mark undoes
// every unit performed, so no snapshot ever holds a front.

// errFrontClosed is what a queue's unit returns once its front closed under it.
var errFrontClosed = errors.New("front closed")

// The Where of an entry order names the composite whose regions are entered, or
// the fork whose branches are; an exit order names the composite exited; a
// region order among the units of firings names the occurrence they react to.
const (
	firingWherePrefix   = "on "
	enteringWherePrefix = "entering "
	forkWherePrefix     = "fork "
	exitingWherePrefix  = "exiting "
)

// entryLabel, exitLabel and effectLabel spell a unit as a PSSM trace does.
func entryLabel(state *ast.StateNode) string     { return state.Name + "(entry)" }
func exitLabel(state *ast.StateNode) string      { return state.Name + "(exit)" }
func effectLabel(trans *lower.Transition) string { return transitionLabel(trans) + "(effect)" }

// transitionLabel names a transition by its own name, or by its ends when it has none.
func transitionLabel(trans *lower.Transition) string {
	if trans.Name != "" {
		return trans.Name
	}
	return StateVertexName(trans.Source) + "->" + StateVertexName(trans.Target)
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

// unitQueue is one region's remaining units: the coroutine performing them and
// the head it yielded, the unit it performs next when resumed. A queue drawn for
// a unit it has yet to name (a start a guard decides) is prepaid for that unit.
// A queue spawned at a head starts its coroutine when first drawn, for that head.
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

// unitHead describes a queue's next unit. A shared unit is one several queues
// may perform, entered once by whichever is drawn first: the others drop it as
// dropped reports. A queue waiting on something a sibling does (its owner's
// entry, the queues it spawned) has no unit until until holds.
type unitHead struct {
	label   string
	at      ast.Node
	shared  *ast.StateNode
	dropped func() bool
	until   func() bool
}

// firingScope is the state of the firing a queue's units belong to — the
// occurrence taken, what a compound transition under way left and entered ahead,
// its mark, the trigger arguments it bound — kept with the queue while another
// queue's units run, so two firings interleaved read each their own.
type firingScope struct {
	event        *Event
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
		event: e.firingEvent, change: e.firingChange, notes: e.firingNotes,
		leftAhead: e.leftAhead, exitingAhead: e.exitingAhead, enteredAhead: e.enteredAhead,
		moving: e.moving,
	}
}

// setFiring puts the executor back in the middle of the firing, keeping what the
// firing had bound as it is.
func (e *StateExecutor) setFiring(s *firingScope) {
	e.firingEvent, e.firingChange, e.firingNotes = s.event, s.change, s.notes
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

// restoreData snapshots the named entries of the machine's data and returns the
// function putting them back, deleting the ones that were not there before. In
// a front the entries are the running queue's firing's, put back when it unbinds
// them and kept out of the way while another queue's units run.
func (e *StateExecutor) restoreData(names []ast.NameSegment) func() {
	q := e.runningQueue()
	if q == nil {
		return e.restoreSharedData(names)
	}
	s := &q.firing
	for _, name := range names {
		if _, bound := s.bound[name.Text]; bound {
			continue
		}
		prior, held := e.stateData[name.Text]
		s.shadowed[name.Text] = dataSlot{value: prior, held: held}
		s.bound[name.Text] = prior
	}
	return func() {
		for _, name := range names {
			if _, bound := s.bound[name.Text]; !bound {
				continue
			}
			s.restore(e, name.Text)
			delete(s.bound, name.Text)
			delete(s.shadowed, name.Text)
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

// performUnits performs the bodies, each one region's units in their order, as
// queues of a front of the kind: the front under way when a queue of it is
// running, which waits for them when wait says, or a front of their own at where,
// drawn one unit at a time. One body alone is performed as it stands.
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

// drawUnits performs the bodies as the queues of a front of their own.
func (e *StateExecutor) drawUnits(kind ChoiceKind, where string, bodies []func() error) error {
	f := e.openFront(kind, where)
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

// spawn adds a queue performing body and runs it to its first head. A queue a
// running queue spawns, for a region nested in its own, goes right before it:
// the lowest queue is drawn first under `declared`, and the nested region was
// entered before the spawning region went on.
func (f *unitFront) spawn(body func() error) *unitQueue {
	q := f.add(body)
	q.start()
	f.resume(q, false)
	return q
}

// spawnAt adds a queue performing body whose first unit head names; the body
// runs when the queue is first drawn, performing that unit without a draw of
// its own. A body deciding first whether it has a unit at all is spawned so.
func (f *unitFront) spawnAt(head unitHead, body func() error) *unitQueue {
	q := f.add(body)
	q.head = head
	return q
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

// unit yields before a unit of the running queue and reports whether to perform
// it; a unit no front orders is performed at once. A front of another kind
// orders coarser units this one is part of.
func (e *StateExecutor) unit(kind ChoiceKind, head unitHead) (bool, error) {
	f := e.front
	if f == nil || !f.accepts(kind) || f.current == nil {
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
		return nil
	}
	if _, err := e.unit(kind, head); err != nil {
		return err
	}
	f.current.prepaid = true
	return nil
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
			f.exec.ctx.noteChoice(choice)
		}
		f.resume(ready[pick], true)
	}
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
		if q.done || q.head.until != nil {
			continue
		}
		if q.head.shared != nil && slices.ContainsFunc(ready, func(r *unitQueue) bool { return r.head.shared == q.head.shared }) {
			continue
		}
		ready = append(ready, q)
	}
	return ready
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

// labels spells the ready queues' next units, the alternatives of the draw; a
// state whose name another region's state shares, as typed regions' states do,
// is told apart by its region, as stateNames tells states of one name apart.
func (f *unitFront) labels(ready []*unitQueue) []string {
	labels := make([]string, len(ready))
	for i, q := range ready {
		labels[i] = q.head.label
		if state, isState := q.head.at.(*ast.StateNode); isState && f.exec.nameShared(state) {
			if region := f.exec.graph.RegionOf[state]; region != nil && region.Name != "" {
				labels[i] = region.Name + "." + labels[i]
			}
		}
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
