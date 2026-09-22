package runtime

import (
	"fmt"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// entryStepWherePrefix names entry-step choices at an instant.
const entryStepWherePrefix = "entry at t=" // #nosec G101 -- a trace label, not a credential

// HoldsEntry reports whether an entry cascade is waiting for its next step.
func (e *StateExecutor) HoldsEntry() bool { return len(e.held) > 0 }

// heldEntry is the unfinished portion of a state entry cascade.
type heldEntry struct {
	// owner is the composite body whose entry is paused.
	owner *ast.StateNode
	// regions are orthogonal regions still awaiting entry.
	regions []*ast.StateRegion
	// branches maps each held region to its selected branch.
	branches map[*ast.StateRegion]*ast.StateNode
	// chain lists states entered by this cascade, outermost first.
	chain []*ast.StateNode
	// scopes are the RTC scopes guarding dispatch while this cascade is held.
	scopes []*ast.StateNode
	// machine reports that the machine body is part of this cascade.
	machine bool
}

// RunToCompletionValueError reports an unevaluable or non-Boolean RTC value.
type RunToCompletionValueError struct {
	State *ast.StateNode
	Value ast.Node
	Err   error
}

func (e *RunToCompletionValueError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("evaluate run-to-completion value of %s: %v", StateVertexName(e.State), e.Err)
	}
	return fmt.Sprintf("run-to-completion value of %s must be boolean", StateVertexName(e.State))
}

func (e *RunToCompletionValueError) Unwrap() error { return e.Err }

// rtcOf evaluates the effective isRunToCompletion value for state.
func (e *StateExecutor) rtcOf(state *ast.StateNode) (bool, error) {
	config := e.graph.RunToCompletionOf(state)
	if config.Value == nil {
		return true, nil
	}
	if literal, ok := config.Value.(*ast.LiteralBool); ok {
		return literal.Value, nil
	}
	value, err := e.evalStepOf(ast.Node(config.ValueOwner), config.Value, config.ValueScope)
	if err != nil {
		return false, &RunToCompletionValueError{State: state, Value: config.Value, Err: err}
	}
	if value.Kind != ValConst || value.Const.Kind != semantics.ValBool {
		return false, &RunToCompletionValueError{State: state, Value: config.Value}
	}
	return value.Const.Bool, nil
}

// enteringChain returns the active entry chain, outermost state first.
func (e *StateExecutor) enteringChain(owner *ast.StateNode) []*ast.StateNode {
	if owner == nil {
		return nil
	}
	var chain []*ast.StateNode
	for _, state := range e.rootToLeaf(owner) {
		if e.entering[state] {
			chain = append(chain, state)
		}
	}
	return chain
}

// holdEntry records an entry cascade when no whole-machine RTC member is entering.
func (e *StateExecutor) holdEntry(owner *ast.StateNode, regions []*ast.StateRegion, branches map[*ast.StateRegion]*ast.StateNode) (bool, error) {
	chain := e.enteringChain(owner)
	machine := e.enteringMachine
	if !machine && len(chain) == 0 {
		return false, nil
	}
	if owner != nil && e.activeAtEntry[owner] && e.entering[owner] {
		return false, nil
	}
	members := slices.Clone(chain)
	if !slices.Contains(members, owner) {
		members = append(members, owner)
	}
	if machine {
		members = append(members, nil)
	}
	var scopes []*ast.StateNode
	for _, state := range members {
		rtc, err := e.rtcOf(state)
		if err != nil {
			return false, err
		}
		if rtc && e.graph.RunToCompletionOf(state).Scope == nil {
			return false, nil
		}
		if rtc {
			scopes = append(scopes, e.graph.RunToCompletionOf(state).Scope)
		}
	}
	e.held = append(e.held, heldEntry{
		owner: owner, regions: regions, branches: branches,
		chain: chain, scopes: scopes, machine: machine,
	})
	return true, nil
}

func (e *StateExecutor) heldOwner(state *ast.StateNode) *heldEntry {
	for i := range e.held {
		if e.held[i].owner == state {
			return &e.held[i]
		}
	}
	return nil
}

// performHeld resumes one held entry cascade.
func (e *StateExecutor) performHeld(item heldEntry) (err error) {
	clear(e.entering)
	for _, state := range item.chain {
		e.entering[state] = true
	}
	e.enteringMachine = item.machine
	begun := append(slices.Clone(item.chain), item.owner)
	if item.machine {
		begun = append(begun, e.graph.Machine)
	}
	e.resumeEntering(begun)
	defer e.unfireOnError(len(e.fired), &err)
	if item.regions != nil {
		if e.activeConfig.simpleState == item.owner {
			e.activeConfig.simpleState = nil
		}
		if err = e.enterRegionsInto(item.owner, item.regions, item.branches); err == nil {
			err = e.settleEntered(item.owner)
		}
	} else {
		var leaf *ast.StateNode
		leaf, err = e.enterStartOf(item.owner)
		if err == nil {
			err = e.settleEntered(leaf)
		}
	}
	return err
}

// settleEntered records the active configuration and schedules its transitions.
func (e *StateExecutor) settleEntered(leaf *ast.StateNode) error {
	if leaf == nil {
		return nil
	}
	onPath := e.branchesTo(nil, leaf)
	for region, state := range onPath {
		e.activeConfig.regionStates[region] = state
	}
	if len(onPath) == 0 && len(e.activeConfig.regionStates) == 0 {
		e.activeConfig.simpleState = leaf
	}
	e.stateStack = e.rootToLeaf(leaf)
	if err := e.scheduleTransitionEvents(); err != nil {
		return fmt.Errorf("schedule events: %w", err)
	}
	return e.completeIfDone(leaf)
}

// entryStep chooses between free dispatch and held entry work.
func (e *StateExecutor) entryStep(progress *dueProgress) (bool, error) {
	dispatch, free := e.dispatchFree(e.dueDispatch())
	if !free && len(e.held) == 1 {
		item := e.held[0]
		e.held = slices.Delete(e.held, 0, 1)
		return true, e.performHeld(item)
	}
	alternatives := make([]string, 0, len(e.held)+1)
	if free {
		alternatives = append(alternatives, dispatch.label)
	}
	for _, item := range e.held {
		alternatives = append(alternatives, e.entryLabel(item.owner))
	}
	if len(alternatives) == 1 {
		if free {
			e.dispatchAmong = dispatch.among
			defer func() { e.dispatchAmong = nil }()
			return e.dispatchOne(progress)
		}
		item := e.held[0]
		e.held = slices.Delete(e.held, 0, 1)
		return true, e.performHeld(item)
	}
	choice := ChoicePoint{
		Kind:         ChoiceEntryStep,
		Where:        entryStepWherePrefix + semantics.FormatReal(e.ctx.clock.now),
		Alternatives: alternatives,
		File:         e.stateMachine.DocName,
		Span:         e.stateMachine.DeclSpan,
	}
	choice.Taken = e.ctx.scheduling().choose(choice, nil)
	if err := e.ctx.scheduling().refusal(); err != nil {
		return false, err
	}
	e.noteChoice(choice)
	if free && choice.Taken == 0 {
		e.dispatchAmong = dispatch.among
		defer func() { e.dispatchAmong = nil }()
		return e.dispatchOne(progress)
	}
	index := choice.Taken
	if free {
		index--
	}
	item := e.held[index]
	e.held = slices.Delete(e.held, index, index+1)
	return true, e.performHeld(item)
}

// heldScopes returns RTC scopes currently guarding dispatch.
func (e *StateExecutor) heldScopes() []*ast.StateNode {
	var scopes []*ast.StateNode
	for _, item := range e.held {
		scopes = append(scopes, item.scopes...)
	}
	return scopes
}

// dispatchFree filters a due dispatch to transitions outside held RTC scopes.
func (e *StateExecutor) dispatchFree(d dueDispatch) (dueDispatch, bool) {
	if !d.due || !d.acts {
		return d, false
	}
	scopes := e.heldScopes()
	if len(scopes) == 0 {
		return d, true
	}
	held := func(event Event) bool {
		if trans, ok := event.Payload.(*lower.Transition); ok {
			return scopeContains(e.graph, scopes, e.transitionOwner(trans))
		}
		var candidates []dispatchCandidate
		var err error
		e.preview(func() {
			candidates, err = e.selectTransitions(&event)
		})
		if err != nil {
			return true
		}
		for _, candidate := range candidates {
			for _, index := range candidate.enabled {
				if scopeContains(e.graph, scopes, e.graph.Transitions[candidate.source][index].Owner) {
					return true
				}
			}
		}
		return false
	}
	if len(d.among) == 0 {
		if d.event != nil && held(*d.event) {
			return d, false
		}
		if risen, ok := e.risenChanges(); ok {
			for _, trans := range risen {
				if trans != nil && held(Event{Payload: trans}) {
					return d, false
				}
			}
		}
		return d, true
	}
	free := make([]Event, 0, len(d.among))
	for _, event := range d.among {
		if !held(event) {
			free = append(free, event)
		}
	}
	if len(free) == 0 {
		return d, false
	}
	d.among = free
	if len(free) == 1 {
		d.label = dispatchPrefix + e.eventLabel(free[0])
		d.step = d.label
	}
	return d, true
}

func (e *StateExecutor) transitionOwner(trans *lower.Transition) *ast.StateNode {
	return trans.Owner
}

func scopeContains(graph *lower.StateGraph, scopes []*ast.StateNode, owner *ast.StateNode) bool {
	for _, scope := range scopes {
		if scope == nil {
			return true
		}
		for current := owner; current != nil; current = graph.ParentState[current] {
			if current == scope {
				return true
			}
		}
	}
	return false
}
