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
// fork's branches, the regions of a composite it enters or exits — one at a
// time, drawing which region's next unit runs while two or more have one. Each
// region's units run in the order the executor always performed them, as a
// coroutine that yields before every unit; the front resumes the one drawn.
// Under `declared` the lowest queue with a unit is drawn, so the front performs
// the very sequence the executor did before it drew anything.

// errFrontClosed is what a queue's unit returns once its front closed under it.
var errFrontClosed = errors.New("front closed")

// The Where of an entry order names the composite whose regions are entered, or
// the fork whose branches are; an exit order names the composite exited. A step
// order's alternatives are units, `do <state>` and `dispatch <event>`.
const (
	enteringWherePrefix = "entering "
	forkWherePrefix     = "fork "
	exitingWherePrefix  = "exiting "
	stepDoPrefix        = "do "
	stepDispatchPrefix  = "dispatch "
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
type unitQueue struct {
	front   *unitFront
	head    unitHead
	done    bool
	closed  bool
	err     error
	next    func() (unitHead, bool)
	stop    func()
	yield   func(unitHead) bool
	perform bool
	prepaid bool
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
		if !q.done {
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
	q := &unitQueue{front: f}
	q.next, q.stop = iter.Pull(func(yield func(unitHead) bool) {
		q.yield = yield
		q.err = body()
	})
	at := len(f.queues)
	if i := slices.Index(f.queues, f.current); i >= 0 {
		at = i
	}
	f.queues = slices.Insert(f.queues, at, q)
	f.resume(q, true)
	return q
}

// resume runs the queue to its next head, performing the unit at its head or
// dropping it as perform says.
func (f *unitFront) resume(q *unitQueue, perform bool) {
	prev := f.current
	f.current, q.perform = q, perform
	head, ok := q.next()
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
	if f == nil || f.kind != kind || f.current == nil {
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
	if f == nil || f.kind != kind || f.current == nil {
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
	return e.front != nil && e.front.kind == kind && e.front.current != nil
}

// await parks the running queue until the condition holds; no unit of its own
// is performed until then.
func (e *StateExecutor) await(kind ChoiceKind, until func() bool) error {
	f := e.front
	if f == nil || f.kind != kind || f.current == nil || until() {
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

// labels spells the ready queues' next units, the alternatives of the draw; two
// states of one name are told apart by their regions, as stateNames tells them.
func (f *unitFront) labels(ready []*unitQueue) []string {
	shared := make(map[string]int, len(ready))
	for _, q := range ready {
		shared[q.head.label]++
	}
	labels := make([]string, len(ready))
	for i, q := range ready {
		labels[i] = q.head.label
		state, isState := q.head.at.(*ast.StateNode)
		if shared[labels[i]] < 2 || !isState {
			continue
		}
		if region := f.exec.graph.RegionOf[state]; region != nil && region.Name != "" {
			labels[i] = region.Name + "." + labels[i]
		}
	}
	return labels
}
