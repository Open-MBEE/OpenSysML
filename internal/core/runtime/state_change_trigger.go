package runtime

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// changeWait is one change condition the active configuration is waiting on: the
// state watching it, the trigger as written, and why it did not fire.
type changeWait struct {
	state   string
	trigger string
	reason  string
}

// String renders one wait for a report of a machine that cannot progress.
func (w changeWait) String() string {
	return fmt.Sprintf("%s: %s (%s)", w.state, w.trigger, w.reason)
}

// changePoll is one poll of the change conditions the active configuration
// watches: a transition's condition and guard are evaluated once per poll,
// however many active leaves the state watching it encloses.
type changePoll struct {
	condition   map[*lower.Transition]bool
	guard       map[*lower.Transition]bool
	blocked     map[*lower.Transition]bool
	unevaluable map[*lower.Transition]UnevaluableGuard
	waits       []changeWait
	waited      map[*lower.Transition]bool
}

// pollChangeEvents re-tests the ChangeEvent conditions the active configuration
// watches and takes the transitions they enable, reporting whether any fired. It
// walks outward from each active leaf, and selects and resolves conflicts exactly
// as an event dispatch does.
//
// A trigger fires on the condition rising: one that stays true does not take the
// same edge again, and only a condition observed false re-arms it.
func (e *StateExecutor) pollChangeEvents() (bool, error) {
	poll := &changePoll{
		condition:   make(map[*lower.Transition]bool),
		guard:       make(map[*lower.Transition]bool),
		blocked:     make(map[*lower.Transition]bool),
		unevaluable: make(map[*lower.Transition]UnevaluableGuard),
		waited:      make(map[*lower.Transition]bool),
	}
	e.changeRearmed = make(map[*lower.Transition]bool)
	defer func() { e.changeRearmed = nil }()

	if err := e.observeChangeConditions(poll); err != nil {
		return false, err
	}
	if len(poll.condition) == 0 {
		e.changeWaits = nil
		return false, nil
	}

	selected, err := e.selectCandidates(func(state *ast.StateNode) ([]int, []RunNote, error) {
		enabled, notes := e.risenChangeTransitions(state, poll)
		return enabled, notes, nil
	})
	if err != nil {
		e.changeWaits = poll.waits
		return false, err
	}

	candidates, err := e.chooseTransitions(selected, nil)
	if err != nil {
		e.changeWaits = poll.waits
		return false, err
	}
	fired, err := e.dispatchInOrder("on change", candidates, func(candidate dispatchCandidate, trans *lower.Transition, notes []RunNote) (bool, error) {
		// An earlier candidate's effect may have blocked this guard since the poll
		// read it, and the fire path re-tests it: a transition that would not move
		// the machine must stay armed rather than latch as fired.
		pass, err := e.passesGuard(trans)
		if err != nil {
			return false, fmt.Errorf("eval change guard: %w", err)
		}
		if !pass {
			poll.blocked[trans] = true
			poll.wait(trans, candidate.source.Name, "guard is false")
			return false, nil
		}
		// The edge is latched before it is taken: an effect that leaves the
		// condition true must not enable the same edge again, and the exit this
		// firing causes must not re-arm the edge that caused it.
		e.changeFired[trans] = true
		e.firingChange = trans
		e.moved = true
		_, err = e.fireFrom(candidate.source, trans, notes, candidate.route)
		e.firingChange = nil
		if err != nil {
			return true, fmt.Errorf("fire transition out of %s: %w", candidate.source.Name, err)
		}
		return true, nil
	})
	if err != nil {
		return fired, err
	}
	if fired {
		// The configuration changed, so a state left by this step no longer holds
		// back the events it deferred.
		e.recallDeferredEvents()
	}
	e.consumeRise(poll)
	e.changeWaits = poll.waits
	return fired, nil
}

// consumeRise latches every enabled transition whose condition was observed
// risen, not only the ones taken: one rise is one occurrence, so a transition
// that lost conflict resolution waits for the next rise instead of firing on the
// next poll. A transition its guard blocked consumes nothing and stays armed, and
// so does one a state entry re-armed during this poll: that watch belongs to an
// activation later than the observation.
func (e *StateExecutor) consumeRise(poll *changePoll) {
	for trans, holds := range poll.condition {
		if holds && poll.guard[trans] && !poll.blocked[trans] && !e.changeRearmed[trans] {
			e.changeFired[trans] = true
		}
	}
}

// observeChangeConditions evaluates, once each, the change conditions of the
// transitions out of the active configuration. A false one re-arms its
// transition and is recorded as a wait.
func (e *StateExecutor) observeChangeConditions(poll *changePoll) error {
	for _, leaf := range e.activeLeaves() {
		for _, source := range e.getParentChain(leaf) {
			transitions := e.graph.Transitions[source]
			decided := false
			for i, trans := range transitions {
				changeEvent, ok := trans.Trigger.(*ast.ChangeEvent)
				if !ok {
					continue
				}
				if _, seen := poll.condition[trans]; seen {
					continue
				}
				holds, err := e.changeConditionHolds(changeEvent, trans)
				if err != nil {
					return fmt.Errorf("state %s: %w", source.Name, err)
				}
				poll.condition[trans] = holds
				if !holds {
					delete(e.changeFired, trans)
					poll.wait(trans, source.Name, "condition is false")
					continue
				}
				if e.changeFired[trans] {
					poll.wait(trans, source.Name, "condition has not changed since it fired")
					continue
				}
				// The guard is read once per poll, alongside the condition, so which
				// transitions this rise enables does not depend on selection order.
				if decided {
					poll.guard[trans] = e.probeChangeGuard(poll, source, transitions, i)
				} else if poll.guard[trans], err = e.passesGuard(trans); err != nil {
					return fmt.Errorf("state %s: eval change guard: %w", source.Name, err)
				}
				if poll.guard[trans] {
					decided = true
				} else if _, unevaluable := poll.unevaluable[trans]; !unevaluable {
					poll.blocked[trans] = true
					poll.wait(trans, source.Name, "guard is false")
				}
			}
		}
	}
	return nil
}

// probeChangeGuard reads the guard of the transition at position i out of state
// once an earlier one is enabled, as a probe the context undoes whole. One that
// cannot be evaluated is not enabled, consumes nothing and is noted on the poll.
func (e *StateExecutor) probeChangeGuard(poll *changePoll, state *ast.StateNode, transitions []*lower.Transition, i int) bool {
	var pass bool
	var err error
	e.preview(func() { pass, err = e.passesGuard(transitions[i]) })
	if err != nil {
		poll.unevaluable[transitions[i]] = e.unevaluableTransition(state, transitions, i, fmt.Errorf("eval change guard: %w", err))
		poll.wait(transitions[i], state.Name, "guard is not evaluable")
		return false
	}
	return pass
}

// changeConditionHolds evaluates one change condition in the scope the
// transition was written in, the machine's data shadowing it.
func (e *StateExecutor) changeConditionHolds(changeEvent *ast.ChangeEvent, trans *lower.Transition) (bool, error) {
	condVal, err := e.evalStepOf(trans.Source, changeEvent.Condition, trans.Scope)
	if err != nil {
		return false, fmt.Errorf("eval change condition: %w", err)
	}
	if condVal.Kind != ValConst || condVal.Const.Kind != semantics.ValBool {
		return false, fmt.Errorf("change condition must be boolean, got %v", condVal.Kind)
	}
	return condVal.Const.Bool, nil
}

// risenChangeTransitions returns the positions of the state's change-triggered
// transitions whose condition has risen and whose guard does not block them,
// several enabled at once being a choice point. A blocked one stays armed for the
// next poll.
func (e *StateExecutor) risenChangeTransitions(state *ast.StateNode, poll *changePoll) ([]int, []RunNote) {
	var enabled []int
	var notes []RunNote
	transitions := e.graph.Transitions[state]
	for i, trans := range transitions {
		if _, ok := trans.Trigger.(*ast.ChangeEvent); !ok {
			continue
		}
		if poll.condition[trans] && !e.changeFired[trans] && poll.guard[trans] {
			enabled = append(enabled, i)
		}
		if unevaluable, ok := poll.unevaluable[trans]; ok {
			notes = append(notes, unevaluable)
		}
	}
	if len(enabled) == 0 {
		return nil, nil
	}
	return enabled, notes
}

// wait records, once per transition, a change condition the configuration is
// waiting on.
func (p *changePoll) wait(trans *lower.Transition, state, reason string) {
	if p.waited[trans] {
		return
	}
	p.waited[trans] = true
	p.waits = append(p.waits, changeWait{state: state, trigger: triggerDescription(trans.Trigger), reason: reason})
}

// PollChangeEvents re-tests the change conditions the active configuration
// watches and takes the transitions they enable, reporting whether any fired:
// the step RunToCompletion takes, for a driver that steps the machine itself.
func (e *StateExecutor) PollChangeEvents() (bool, error) {
	defer e.ctx.beginExecutorRun(&e.driven)()

	fired, err := e.pollChangeEvents()
	if err != nil {
		return fired, err
	}
	return fired, e.completedRun()
}

// ChangeWaits describes the change conditions the active configuration was
// waiting on as of the last poll, and nothing when it watches none.
func (e *StateExecutor) ChangeWaits() []string {
	waits := make([]string, 0, len(e.changeWaits))
	for _, wait := range e.changeWaits {
		waits = append(waits, wait.String())
	}
	return waits
}

// WatchesChangeCondition reports whether the active configuration watches a
// change condition, which data written outside the machine can make true.
func (e *StateExecutor) WatchesChangeCondition() bool {
	for _, leaf := range e.activeLeaves() {
		for _, source := range e.getParentChain(leaf) {
			for _, trans := range e.graph.Transitions[source] {
				if _, ok := trans.Trigger.(*ast.ChangeEvent); ok {
					return true
				}
			}
		}
	}
	return false
}

// canStillProgress reports whether a step is left to take. A suspended machine's
// do activities are registered but exhausted, so what is left there is a queued
// event, a signal in flight or a do action still to run.
func (e *StateExecutor) canStillProgress() bool {
	if e.state != StateSuspended {
		return e.HasPendingWork()
	}
	return e.eventQueue.Len() > 0 || e.hasPendingSignal() || e.HasPendingDoWork()
}

// SuspendReason says why a machine that cannot progress cannot: the change
// conditions it waits on, or that nothing is left that could fire.
func (e *StateExecutor) SuspendReason() string {
	if e.state == StateCompleted || e.canStillProgress() {
		return ""
	}
	reason := "quiesced: nothing left can fire (no queued event, signal in flight, running do behavior or watched change condition)"
	if waits := e.ChangeWaits(); len(waits) > 0 {
		reason = "waiting on change condition: " + strings.Join(waits, "; ")
	}
	if due, waiting := e.NextWait(); waiting {
		reason = fmt.Sprintf("waiting on the clock: the next timer is due at t=%s (advance the clock to reach it)", semantics.FormatReal(due))
	}
	// An event the active states still defer cannot be dispatched here, but it is
	// not gone either, so a stalled machine reports it rather than losing it.
	if held := len(e.deferred); held > 0 {
		reason += fmt.Sprintf("; %d event(s) still deferred by the active states", held)
	}
	return reason
}
