package runtime

import (
	"errors"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
)

// The labels of a firing's units: a state's exit or entry, or a segment's effect
// (`effectLabel`). They tell a per-unit region-order draw from one among states.
const (
	exitUnitPrefix   = "exit "
	enterUnitPrefix  = "enter "
	effectUnitSuffix = "(effect)"
)

// firingUnits reports whether every label names a unit of a firing.
func firingUnits(labels []string) bool {
	if len(labels) == 0 {
		return false
	}
	for _, label := range labels {
		if !strings.HasPrefix(label, exitUnitPrefix) && !strings.HasPrefix(label, enterUnitPrefix) && !strings.HasSuffix(label, effectUnitSuffix) {
			return false
		}
	}
	return true
}

// errFiringAbandoned unwinds a firing the dispatch does not let finish: its
// leaf was left by a sibling, a sibling ended the machine, or a move was refused.
var errFiringAbandoned = errors.New("firing abandoned")

// firingScope is what the executor holds for the firing under way alone, swapped
// out while the firing waits for its turn and back in when it resumes.
type firingScope struct {
	notes        []RunNote
	event        *Event
	change       *lower.Transition
	leftAhead    map[*ast.StateNode]bool
	exitingAhead bool
	enteredAhead map[*ast.StateNode]bool
	moving       *moveMark
	// bound are the state data the firing's trigger binds, `accept msg : Sig`'s
	// payload or a call's parameters, held apart from what its siblings read.
	bound []boundDatum
}

// boundDatum is one name a trigger binds, with the value the other side holds.
type boundDatum struct {
	name  string
	value Value
	held  bool
}

// swapFiringScope exchanges the executor's per-firing fields with s, the
// trigger's bindings included.
func (e *StateExecutor) swapFiringScope(s *firingScope) {
	for i := range s.bound {
		d := &s.bound[i]
		value, held := e.stateData[d.name]
		if d.held {
			e.stateData[d.name] = d.value
		} else {
			delete(e.stateData, d.name)
		}
		d.value, d.held = value, held
	}
	s.notes, e.firingNotes = e.firingNotes, s.notes
	s.event, e.firingEvent = e.firingEvent, s.event
	s.change, e.firingChange = e.firingChange, s.change
	s.leftAhead, e.leftAhead = e.leftAhead, s.leftAhead
	s.exitingAhead, e.exitingAhead = e.exitingAhead, s.exitingAhead
	s.enteredAhead, e.enteredAhead = e.enteredAhead, s.enteredAhead
	s.moving, e.moving = e.moving, s.moving
}

// triggerBindings is what a transition's trigger binds into the state data.
func (e *StateExecutor) triggerBindings(trans *lower.Transition) []boundDatum {
	var names []string
	switch trigger := trans.Trigger.(type) {
	case *ast.AcceptEvent:
		if trigger.Payload != nil && trigger.Payload.Ident.Name != "" {
			names = append(names, trigger.Payload.Ident.Name)
		}
	case *ast.CallEvent:
		for _, param := range trigger.Parameters {
			names = append(names, param.Text)
		}
	}
	bound := make([]boundDatum, len(names))
	for i, name := range names {
		value, held := e.stateData[name]
		bound[i] = boundDatum{name: name, value: value, held: held}
	}
	return bound
}

// firingUnit is one unit a firing announces before performing it: what it is
// labelled, the state it exits or enters (nil for an effect), and whether it
// performs the rest of the firing at once.
type firingUnit struct {
	label string
	state *ast.StateNode
	exits bool
	whole bool
}

// firing is one candidate's transition performed unit by unit — its source's
// exit, its segments' effects, its target's entry — each announced to the
// dispatch before it runs, so the units of sibling firings interleave. It runs
// on its own goroutine, which the dispatch and the firing hand control to and
// from, one of them running at a time.
type firing struct {
	exec      *StateExecutor
	candidate dispatchCandidate
	perform   func(dispatchCandidate, *lower.Transition, []RunNote) (bool, error)
	// next is the unit announced and not yet performed: the exit of the source's
	// leaf before the firing starts, nothing once it is done.
	next          firingUnit
	started, done bool
	// performed counts the units the firing ran; a firing that ran none is dropped
	// as an unstarted one is when a sibling leaves its leaf.
	performed int
	fired     bool
	err       error
	panicked  any
	turn      chan bool     // to the firing: perform the unit announced, or unwind
	yielded   chan struct{} // to the dispatch: a unit announced, or the firing done
	scope     firingScope
}

// advance performs the firing's next unit and runs on to its next announcement or
// its end; a firing not started first starts, reading its guard, and performs
// the first unit it announces. A panic on the firing's goroutine is raised again
// on the caller's.
func (f *firing) advance() {
	if !f.started {
		f.resume(true)
		if f.done {
			return
		}
	}
	f.resume(true)
}

// abandon unwinds a firing with a unit announced, so nothing of it runs on; a
// firing not started or done is left as it is.
func (f *firing) abandon() {
	if f.started && !f.done {
		f.resume(false)
	}
}

func (f *firing) resume(perform bool) {
	if !f.started {
		f.started = true
		f.scope.bound = f.exec.triggerBindings(f.candidate.chosen)
		f.turn, f.yielded = make(chan bool), make(chan struct{})
		go f.run()
	} else {
		f.turn <- perform
	}
	<-f.yielded
	if f.panicked != nil {
		panic(f.panicked)
	}
}

// run performs the firing on its own goroutine, handing control back when done.
func (f *firing) run() {
	e := f.exec
	defer func() {
		if p := recover(); p != nil {
			f.panicked = p
		}
		f.done, f.next = true, firingUnit{}
		e.performing = nil
		f.scope.bound = nil
		e.swapFiringScope(&f.scope)
		f.yielded <- struct{}{}
	}()
	e.swapFiringScope(&f.scope)
	e.performing = f
	f.fired, f.err = f.perform(f.candidate, f.candidate.chosen, f.candidate.notes)
	if errors.Is(f.err, errFiringAbandoned) {
		f.err, f.fired = nil, false
	}
}

// await announces the firing's next unit and waits for the dispatch's turn to
// perform it; the firing unwinds with errFiringAbandoned when it gets none.
func (f *firing) await(unit firingUnit) error {
	e := f.exec
	f.next = unit
	e.performing = nil
	e.swapFiringScope(&f.scope)
	f.yielded <- struct{}{}
	perform := <-f.turn
	e.swapFiringScope(&f.scope)
	e.performing = f
	if !perform {
		return errFiringAbandoned
	}
	f.performed++
	return nil
}

// unitExit announces the exit of state as the next unit of the firing under way
// and waits for its turn; outside such a firing it runs on.
func (e *StateExecutor) unitExit(state *ast.StateNode) error {
	if e.performing == nil || e.graph.HiddenStates[state] {
		return nil
	}
	return e.performing.await(firingUnit{label: exitUnitPrefix + state.Name, state: state, exits: true})
}

// unitEnter announces the entry of state as the next unit of the firing under way.
func (e *StateExecutor) unitEnter(state *ast.StateNode) error {
	if e.performing == nil || e.graph.HiddenStates[state] {
		return nil
	}
	return e.performing.await(firingUnit{label: enterUnitPrefix + state.Name, state: state})
}

// unitEffect announces the effect of segment as the next unit of the firing under way.
func (e *StateExecutor) unitEffect(segment *lower.Transition) error {
	if e.performing == nil {
		return nil
	}
	return e.performing.await(firingUnit{label: effectLabel(segment)})
}

// performWhole announces the rest of the firing under way as one unit and performs
// run without announcing what it does: a fork, join or history reshapes the
// configuration of sibling regions rather than moving within its own.
func (e *StateExecutor) performWhole(trans *lower.Transition, run func() error) error {
	f := e.performing
	if f == nil {
		return run()
	}
	unit := firingUnit{label: exitUnitPrefix + StateVertexName(trans.Source), whole: true}
	if source, ok := trans.Source.(*ast.StateNode); ok {
		unit = e.firstUnit(source)
		unit.whole = true
	}
	if err := f.await(unit); err != nil {
		return err
	}
	e.performing = nil
	err := run()
	e.performing = f
	return err
}

// firstUnit is the unit a firing out of source performs first: the exit of the
// one active leaf below it, or of source itself when its regions hold several and
// the exit front draws which leaves first.
func (e *StateExecutor) firstUnit(source *ast.StateNode) firingUnit {
	var leaf *ast.StateNode
	for _, state := range e.activeLeaves() {
		if !e.isBelowOrEqual(state, source) || e.graph.HiddenStates[state] {
			continue
		}
		if leaf != nil {
			leaf = source
			break
		}
		leaf = state
	}
	if leaf == nil {
		leaf = source
	}
	return firingUnit{label: exitUnitPrefix + leaf.Name, state: leaf, exits: true}
}

// unitLabels spells the next unit of each firing; the units of firings whose
// sources share a name, as typed regions' states do, are qualified by region.
func (e *StateExecutor) unitLabels(firings []*firing) []string {
	labels := make([]string, len(firings))
	shared := make(map[string]int, len(firings))
	for i, f := range firings {
		labels[i] = f.next.label
		shared[f.candidate.source.Name]++
	}
	for i, f := range firings {
		state := f.next.state
		if state == nil || shared[f.candidate.source.Name] < 2 {
			continue
		}
		if region := e.graph.RegionOf[state]; region != nil && region.Name != "" {
			prefix := enterUnitPrefix
			if f.next.exits {
				prefix = exitUnitPrefix
			}
			labels[i] = prefix + region.Name + "." + state.Name
		}
	}
	return labels
}

// firesWhole reports whether the dispatch performs each firing whole, drawn
// among the candidates by source state: the fixed policies do, as does a replay
// whose witness records the dispatch's next region order among states.
func (e *StateExecutor) firesWhole() bool {
	s := e.ctx.scheduling()
	switch s.policy.kind {
	case scheduleReverse, scheduleDeclared:
		return true
	case scheduleReplay:
		if !s.replaying() {
			return true
		}
		for _, c := range s.replay.choices[s.replay.next:] {
			if c.Kind == ChoiceRegionOrder {
				return !firingUnits(c.Among)
			}
		}
		return true
	}
	return false
}

// blocked reports a unit that would leave a state a started sibling firing is
// still moving under, which the sibling's remaining units are drawn before.
func (f *firing) blocked(siblings []*firing) bool {
	if !f.next.whole && !f.next.exits {
		return false
	}
	for _, s := range siblings {
		if s == f || !s.started || s.done {
			continue
		}
		if f.next.whole || f.exec.isBelowOrEqual(s.candidate.source, f.next.state) {
			return true
		}
	}
	return false
}

// dispatchByUnit fires the candidates one unit at a time: the policy draws which
// firing performs its next unit among those with one left, and a firing whose
// leaf a sibling left before it performed anything is dropped. Two firings that
// leave one state each conflict: the first drawn leaves it and the other is
// dropped where it stands. A refused draw undoes the dispatch whole, under a
// mark of its own.
func (e *StateExecutor) dispatchByUnit(
	where string,
	candidates []dispatchCandidate,
	fire func(dispatchCandidate, *lower.Transition, []RunNote) (bool, error),
) (acted bool, err error) {
	var mark *moveMark
	if e.ctx.scheduling().replaying() {
		mark = e.markMove()
	}
	firings := make([]*firing, len(candidates))
	for i, candidate := range candidates {
		firings[i] = &firing{exec: e, candidate: candidate, perform: fire, next: e.firstUnit(candidate.source), scope: firingScope{moving: e.moving}}
	}
	defer func() {
		for _, f := range firings {
			f.abandon()
			acted = acted || f.fired
		}
		if mark == nil {
			return
		}
		if e.ctx.scheduling().refusal() != nil {
			mark.undo()
		} else {
			mark.keep()
		}
	}()
	for {
		live := make([]*firing, 0, len(firings))
		for _, f := range firings {
			if f.done {
				continue
			}
			if f.performed == 0 && !e.isActive(f.candidate.leaf) {
				f.abandon()
				continue
			}
			live = append(live, f)
		}
		if len(live) == 0 {
			return acted, nil
		}
		offered := slices.DeleteFunc(slices.Clone(live), func(f *firing) bool { return f.blocked(live) })
		conflict := len(offered) == 0
		if conflict {
			offered = live
		}
		pick := 0
		if len(offered) > 1 {
			choice := ChoicePoint{
				Kind:         ChoiceRegionOrder,
				Where:        where,
				Alternatives: e.unitLabels(offered),
				File:         e.stateMachine.DocName,
			}
			scheduling := e.ctx.scheduling()
			choice.Taken = scheduling.choose(choice, nil)
			if err := scheduling.refusal(); err != nil {
				return acted, err
			}
			choice.Span = offered[choice.Taken].candidate.source.Span()
			e.ctx.noteFrom(choice, e.self, e.stateMachine)
			pick = choice.Taken
		}
		f := offered[pick]
		if conflict {
			for _, s := range live {
				if s != f {
					s.abandon()
				}
			}
		}
		f.advance()
		if f.err != nil {
			return acted, f.err
		}
		if e.state.Ended() {
			return acted, nil
		}
	}
}
