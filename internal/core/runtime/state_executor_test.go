package runtime

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

func TestStateExecutor_Creation(t *testing.T) {
	ctx := NewContext(semantics.NewModel(nil), nil, 1000)
	
	// Create minimal state machine symbol
	stateMachine := &symbols.Symbol{
		Name: "TestStateMachine",
		Kind: symbols.SymbolStateUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageState,
			Ident:   ast.Identification{Name: "TestStateMachine"},
			Members: []ast.Node{},
		},
	}
	
	exec, err := newStateExecutor(ctx, stateMachine)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	
	if exec.ctx != ctx {
		t.Error("expected context to be set")
	}
	
	if exec.stateMachine != stateMachine {
		t.Error("expected stateMachine symbol to be set")
	}
	
	if exec.state != StateReady {
		t.Errorf("expected StateReady, got %v", exec.state)
	}
	
	if exec.eventQueue == nil {
		t.Error("expected event queue to be initialized")
	}
}

func TestStateExecutor_Initialize(t *testing.T) {
	ctx := NewContext(semantics.NewModel(nil), nil, 1000)
	
	// Build state machine: initialState → finalState
	initialState := &ast.StateNode{Name: "initial", IsInitial: true}
	finalState := &ast.StateNode{Name: "final", IsFinal: true}
	
	stateMachine := &symbols.Symbol{
		Name: "SimpleStateMachine",
		Kind: symbols.SymbolStateUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageState,
			Ident:   ast.Identification{Name: "SimpleStateMachine"},
			Members: []ast.Node{
				initialState,
				finalState,
			},
		},
	}
	
	exec, err := newStateExecutor(ctx, stateMachine)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	
	err = exec.initialize()
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	
	// Verify current state set to initial
	if exec.currentState != initialState {
		t.Errorf("expected current state to be initialState, got %v", exec.currentState)
	}
	
	if exec.state != StateRunning {
		t.Errorf("expected StateRunning, got %v", exec.state)
	}
	
	if exec.currentTime != 0.0 {
		t.Errorf("expected time 0.0, got %f", exec.currentTime)
	}
}

func TestStateExecutor_Initialize_NoInitialState(t *testing.T) {
	ctx := NewContext(semantics.NewModel(nil), nil, 1000)
	
	// State machine without initial state
	stateMachine := &symbols.Symbol{
		Name: "NoInitialStateMachine",
		Kind: symbols.SymbolStateUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageState,
			Ident:   ast.Identification{Name: "NoInitialStateMachine"},
			Members: []ast.Node{
				&ast.StateNode{Name: "someState"},
			},
		},
	}
	
	exec, err := newStateExecutor(ctx, stateMachine)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	
	err = exec.initialize()
	if err == nil {
		t.Fatal("expected error for missing initial state")
	}
	
	if !containsText(err.Error(), "no initial state") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStateExecutor_EntryBehavior(t *testing.T) {
	ctx := NewContext(semantics.NewModel(nil), nil, 1000)
	
	// State with entry action: entry { x = 42 }
	initialState := &ast.StateNode{
		Name:      "initial",
		IsInitial: true,
		Entry: []ast.Node{
			&ast.ActionExecutionNode{
				Name:       "entryAction",
				Expression: &ast.LiteralInteger{Value: "42"},
			},
		},
	}
	
	stateMachine := &symbols.Symbol{
		Name: "EntryBehaviorMachine",
		Kind: symbols.SymbolStateUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageState,
			Ident:   ast.Identification{Name: "EntryBehaviorMachine"},
			Members: []ast.Node{initialState},
		},
	}
	
	exec, err := newStateExecutor(ctx, stateMachine)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	
	err = exec.initialize()
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	
	// Entry behavior should have executed
	// Check execution context or token data (depends on implementation)
	// For now, just verify no error
}

func TestStateExecutor_ExitBehavior(t *testing.T) {
	ctx := NewContext(semantics.NewModel(nil), nil, 1000)
	
	// Two states with exit behavior on first
	stateA := &ast.StateNode{
		Name:      "stateA",
		IsInitial: true,
		Exit: []ast.Node{
			&ast.ActionExecutionNode{
				Name:       "exitAction",
				Expression: &ast.LiteralInteger{Value: "99"},
			},
		},
	}
	stateB := &ast.StateNode{
		Name: "stateB",
	}
	
	// Transition A → B
	transition := &ast.TransitionEdge{
		Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "stateA"}}},
		Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "stateB"}}},
	}
	
	stateMachine := &symbols.Symbol{
		Name: "ExitBehaviorMachine",
		Kind: symbols.SymbolStateUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageState,
			Ident:   ast.Identification{Name: "ExitBehaviorMachine"},
			Members: []ast.Node{stateA, stateB, transition},
		},
	}
	
	exec, err := newStateExecutor(ctx, stateMachine)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	
	err = exec.initialize()
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	
	// Execute transition (will test in later tasks)
	// For now just verify structure
	if len(exec.transitions[stateA]) != 1 {
		t.Errorf("expected 1 transition from stateA, got %d", len(exec.transitions[stateA]))
	}
}

func TestStateExecutor_TimeEvent(t *testing.T) {
	ctx := NewContext(semantics.NewModel(nil), nil, 1000)
	
	// State machine: stateA --[after 10]-> stateB
	stateA := &ast.StateNode{
		Name:      "stateA",
		IsInitial: true,
	}
	stateB := &ast.StateNode{
		Name:    "stateB",
		IsFinal: true,
	}
	
	// Transition with TimeEvent trigger (after 10 time units)
	transition := &ast.TransitionEdge{
		Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "stateA"}}},
		Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "stateB"}}},
		Trigger: &ast.TimeEvent{
			Duration: &ast.LiteralInteger{Value: "10"},
		},
	}
	
	stateMachine := &symbols.Symbol{
		Name: "TimeEventMachine",
		Kind: symbols.SymbolStateUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageState,
			Ident:   ast.Identification{Name: "TimeEventMachine"},
			Members: []ast.Node{stateA, stateB, transition},
		},
	}
	
	exec, err := newStateExecutor(ctx, stateMachine)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	
	err = exec.initialize()
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	
	// Current state should be stateA
	if exec.currentState != stateA {
		t.Errorf("expected current state stateA, got %v", exec.currentState)
	}
	
	// TimeEvent should be scheduled
	if exec.eventQueue.Len() != 1 {
		t.Fatalf("expected 1 pending event, got %d", exec.eventQueue.Len())
	}
	
	nextEvent := exec.eventQueue.Peek()
	if nextEvent.Type != EventTime {
		t.Errorf("expected EventTime, got %v", nextEvent.Type)
	}
	
	if nextEvent.Timestamp != 10.0 {
		t.Errorf("expected timestamp 10.0, got %f", nextEvent.Timestamp)
	}
	
	// Process event (advance time and fire transition)
	err = exec.processNextEvent()
	if err != nil {
		t.Fatalf("process event: %v", err)
	}
	
	// Should transition to stateB
	if exec.currentState != stateB {
		t.Errorf("expected current state stateB, got %v", exec.currentState)
	}
	
	// Time should advance
	if exec.currentTime != 10.0 {
		t.Errorf("expected time 10.0, got %f", exec.currentTime)
	}
}

func TestStateExecutor_ChangeEvent(t *testing.T) {
	ctx := NewContext(semantics.NewModel(nil), nil, 1000)
	
	// State machine: stateA --[when x > 5]-> stateB
	stateA := &ast.StateNode{
		Name:      "stateA",
		IsInitial: true,
	}
	stateB := &ast.StateNode{
		Name:    "stateB",
		IsFinal: true,
	}
	
	// Transition with ChangeEvent trigger
	transition := &ast.TransitionEdge{
		Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "stateA"}}},
		Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "stateB"}}},
		Trigger: &ast.ChangeEvent{
			Condition: &ast.OperatorExpr{
				Operator: ast.OpGt,
				Operands: []ast.Node{
					&ast.FeatureReference{Name: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "x"}}}},
					&ast.LiteralInteger{Value: "5"},
				},
			},
		},
	}
	
	stateMachine := &symbols.Symbol{
		Name: "ChangeEventMachine",
		Kind: symbols.SymbolStateUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageState,
			Ident:   ast.Identification{Name: "ChangeEventMachine"},
			Members: []ast.Node{stateA, stateB, transition},
		},
	}
	
	exec, err := newStateExecutor(ctx, stateMachine)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	
	// Set x = 3 (condition false)
	exec.stateData["x"] = Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}
	
	err = exec.initialize()
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	
	// Should remain in stateA (condition false)
	if exec.currentState != stateA {
		t.Errorf("expected current state stateA, got %v", exec.currentState)
	}
	
	// Change x = 10 (condition true)
	exec.stateData["x"] = Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 10}}
	
	// Poll for change events
	err = exec.pollChangeEvents()
	if err != nil {
		t.Fatalf("poll change events: %v", err)
	}
	
	// Should transition to stateB
	if exec.currentState != stateB {
		t.Errorf("expected current state stateB after condition true, got %v", exec.currentState)
	}
}

func TestStateExecutor_GuardCondition(t *testing.T) {
	ctx := NewContext(semantics.NewModel(nil), nil, 1000)
	
	// State machine: stateA --[after 1][x > 5]-> stateB
	stateA := &ast.StateNode{
		Name:      "stateA",
		IsInitial: true,
	}
	stateB := &ast.StateNode{
		Name:    "stateB",
		IsFinal: true,
	}
	
	// Transition with TimeEvent and guard
	transition := &ast.TransitionEdge{
		Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "stateA"}}},
		Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "stateB"}}},
		Trigger: &ast.TimeEvent{
			Duration: &ast.LiteralInteger{Value: "1"},
		},
		Guard: &ast.OperatorExpr{
			Operator: ast.OpGt,
			Operands: []ast.Node{
				&ast.FeatureReference{Name: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "x"}}}},
				&ast.LiteralInteger{Value: "5"},
			},
		},
	}
	
	stateMachine := &symbols.Symbol{
		Name: "GuardMachine",
		Kind: symbols.SymbolStateUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageState,
			Ident:   ast.Identification{Name: "GuardMachine"},
			Members: []ast.Node{stateA, stateB, transition},
		},
	}
	
	exec, err := newStateExecutor(ctx, stateMachine)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	
	// Set x = 3 (guard false)
	exec.stateData["x"] = Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}
	
	err = exec.initialize()
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	
	// Process TimeEvent (guard should fail)
	err = exec.processNextEvent()
	if err != nil {
		t.Fatalf("process event: %v", err)
	}
	
	// Should remain in stateA (guard blocked transition)
	if exec.currentState != stateA {
		t.Errorf("expected current state stateA (guard false), got %v", exec.currentState)
	}
	
	// Set x = 10 (guard true), schedule new event
	exec.stateData["x"] = Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 10}}
	exec.eventQueue.Push(Event{
		ID:        2,
		Type:      EventTime,
		Timestamp: exec.currentTime + 1.0,
		Payload:   transition,
	})
	
	// Process second event (guard should pass)
	err = exec.processNextEvent()
	if err != nil {
		t.Fatalf("process second event: %v", err)
	}
	
	// Should transition to stateB
	if exec.currentState != stateB {
		t.Errorf("expected current state stateB (guard true), got %v", exec.currentState)
	}
}

// Integration Tests

func TestStateExecutor_Integration_SimpleTransitions(t *testing.T) {
	ctx := NewContext(semantics.NewModel(nil), nil, 1000)
	
	// State machine: idle → working → done
	idle := &ast.StateNode{Name: "idle", IsInitial: true}
	working := &ast.StateNode{Name: "working"}
	done := &ast.StateNode{Name: "done", IsFinal: true}
	
	trans1 := &ast.TransitionEdge{
		Source:  &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "idle"}}},
		Target:  &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "working"}}},
		Trigger: &ast.TimeEvent{Duration: &ast.LiteralInteger{Value: "5"}},
	}
	trans2 := &ast.TransitionEdge{
		Source:  &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "working"}}},
		Target:  &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "done"}}},
		Trigger: &ast.TimeEvent{Duration: &ast.LiteralInteger{Value: "10"}},
	}
	
	stateMachine := &symbols.Symbol{
		Name: "WorkflowMachine",
		Kind: symbols.SymbolStateUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageState,
			Ident:   ast.Identification{Name: "WorkflowMachine"},
			Members: []ast.Node{idle, working, done, trans1, trans2},
		},
	}
	
	exec, err := newStateExecutor(ctx, stateMachine)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	
	err = exec.initialize()
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	
	// Should start in idle
	if exec.currentState != idle {
		t.Errorf("expected idle, got %v", exec.currentState)
	}
	
	// Process first event (idle → working at t=5)
	err = exec.processNextEvent()
	if err != nil {
		t.Fatalf("process event 1: %v", err)
	}
	
	if exec.currentState != working {
		t.Errorf("expected working, got %v", exec.currentState)
	}
	if exec.currentTime != 5.0 {
		t.Errorf("expected time 5.0, got %f", exec.currentTime)
	}
	
	// Process second event (working → done at t=15)
	err = exec.processNextEvent()
	if err != nil {
		t.Fatalf("process event 2: %v", err)
	}
	
	if exec.currentState != done {
		t.Errorf("expected done, got %v", exec.currentState)
	}
	if exec.currentTime != 15.0 {
		t.Errorf("expected time 15.0, got %f", exec.currentTime)
	}
	if exec.state != StateCompleted {
		t.Errorf("expected StateCompleted, got %v", exec.state)
	}
}

func TestStateExecutor_Integration_TransitionEffects(t *testing.T) {
	ctx := NewContext(semantics.NewModel(nil), nil, 1000)
	
	// State machine with transition effect: stateA --[after 1 / counter++]-> stateB
	stateA := &ast.StateNode{Name: "stateA", IsInitial: true}
	stateB := &ast.StateNode{Name: "stateB", IsFinal: true}
	
	// Transition with effect that increments counter
	transition := &ast.TransitionEdge{
		Source:  &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "stateA"}}},
		Target:  &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "stateB"}}},
		Trigger: &ast.TimeEvent{Duration: &ast.LiteralInteger{Value: "1"}},
		Effect: []ast.Node{
			&ast.ActionExecutionNode{
				Name:       "incrementCounter",
				Expression: &ast.LiteralInteger{Value: "42"}, // Simplified - just set counter
			},
		},
	}
	
	stateMachine := &symbols.Symbol{
		Name: "EffectMachine",
		Kind: symbols.SymbolStateUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageState,
			Ident:   ast.Identification{Name: "EffectMachine"},
			Members: []ast.Node{stateA, stateB, transition},
		},
	}
	
	exec, err := newStateExecutor(ctx, stateMachine)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	
	err = exec.initialize()
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	
	// Process transition
	err = exec.processNextEvent()
	if err != nil {
		t.Fatalf("process event: %v", err)
	}
	
	// Verify effect executed
	if val, ok := exec.stateData["incrementCounter"]; !ok {
		t.Error("expected incrementCounter in stateData")
	} else if val.Kind != ValConst || val.Const.Kind != semantics.ValInt || val.Const.Int != 42 {
		t.Errorf("expected incrementCounter = 42, got %v", val)
	}
	
	if exec.currentState != stateB {
		t.Errorf("expected stateB, got %v", exec.currentState)
	}
}
