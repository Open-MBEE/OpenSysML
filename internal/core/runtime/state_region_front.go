package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// unitGate is what a unit may wait for; it opens when a shared unit has run or
// when a group of queues is spent.
type unitGate struct {
	open bool
}

// sharedUnit is a unit several queues hold — a state on a fork's way down — which
// the first queue to reach it performs and the others then drop.
type sharedUnit struct {
	unitGate
}

// queueGroup is the queues of one state's regions: its gate opens once every one
// of them is spent, and then runs once, if set.
type queueGroup struct {
	unitGate
	left int
	then func()
}

// orderUnit is one performance the library orders against the others of its
// region and a trace logs separately: a state's entry or exit, a segment's effect.
type orderUnit struct {
	label  string
	state  *ast.StateNode // the state whose entry or exit the unit logs first, if any
	shared *sharedUnit    // set on a unit several queues hold
	after  *unitGate      // set on a unit that waits for a gate to open
	run    func(q *orderQueue) error
}

// orderQueue is what one region has left to do, in the library's order; a unit
// may queue what it opens up ahead of the rest.
type orderQueue struct {
	region *ast.StateRegion
	group  *queueGroup // the group the queue is one of, until it is spent
	wrap   func(error) error
	units  []orderUnit
}

// ahead puts unit ahead of everything else q has left.
func (q *orderQueue) ahead(unit orderUnit) {
	q.units = append([]orderUnit{unit}, q.units...)
}

// orderFront is the queues of one site — a composite's regions being entered or
// left, a fork's branches — drawn from one unit at a time while two or more have
// a unit ready; each draw is a choice of the front's kind.
type orderFront struct {
	exec   *StateExecutor
	kind   ChoiceKind
	where  string
	span   source.Span
	queues []*orderQueue
}

// insert puts queues on the front ahead of before, or last when before is not on
// it, so a run taking the first alternative at every draw runs them next.
func (f *orderFront) insert(before *orderQueue, queues []*orderQueue) {
	at := len(f.queues)
	for i, q := range f.queues {
		if q == before {
			at = i
			break
		}
	}
	f.queues = append(f.queues[:at], append(queues, f.queues[at:]...)...)
}

// drain runs the front to the end: while two or more queues have a unit ready,
// the policy draws the queue that advances, and the draw is recorded.
func (f *orderFront) drain() error {
	e := f.exec
	for {
		live := f.live()
		if len(live) == 0 {
			for _, q := range f.queues {
				if len(q.units) > 0 {
					return fmt.Errorf("%s: region %s waits for a unit no queue performs", f.where, regionName(q.region))
				}
			}
			return nil
		}
		pick := 0
		if len(live) > 1 {
			var err error
			choice := ChoicePoint{Kind: f.kind, Where: f.where, Alternatives: f.labels(live), File: e.stateMachine.DocName, Span: f.span}
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
			unit.shared.open = true
		}
		f.advance()
	}
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

// advance drops the shared units a sibling queue performed from the head of every
// queue and settles the queues that are spent.
func (f *orderFront) advance() {
	for _, q := range f.queues {
		for len(q.units) > 0 && q.units[0].shared != nil && q.units[0].shared.open {
			q.units = q.units[1:]
		}
	}
	for _, q := range f.queues {
		f.settle(q)
	}
}

// live is every queue with a unit ready to run, in the front's order: a shared
// unit several queues reach is offered once, and a unit waiting for a gate is
// held back until it opens.
func (f *orderFront) live() []*orderQueue {
	var live []*orderQueue
	offered := make(map[*sharedUnit]bool)
	for _, q := range f.queues {
		if len(q.units) == 0 {
			continue
		}
		head := q.units[0]
		if head.after != nil && !head.after.open {
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

// settle takes a spent queue out of its group, opening the group once it was
// the last.
func (f *orderFront) settle(q *orderQueue) {
	if len(q.units) > 0 || q.group == nil {
		return
	}
	group := q.group
	q.group = nil
	if group.left--; group.left > 0 {
		return
	}
	group.open = true
	if group.then != nil {
		group.then()
	}
}

// labels spells the head of each queue, qualified by its region where two heads
// name states of one name, as typed regions' states are.
func (f *orderFront) labels(live []*orderQueue) []string {
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

// wrapping composes how an error is reported: inner first, then outer.
func wrapping(outer, inner func(error) error) func(error) error {
	return func(err error) error { return outer(inner(err)) }
}
