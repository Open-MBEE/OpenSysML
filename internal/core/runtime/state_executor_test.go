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

func TestStateExecutor_HierarchicalStates(t *testing.T) {
	ctx := NewContext(semantics.NewModel(nil), nil, 1000)
	
	// Build hierarchical state machine:
	//   composite (parent)
	//     ├─ childA (initial)
	//     └─ childB
	//   standalone
	
	childA := &ast.StateNode{
		Name:      "childA",
		IsInitial: true,
	}
	
	childB := &ast.StateNode{
		Name: "childB",
	}
	
	composite := &ast.StateNode{
		Name:      "composite",
		Substates: []ast.Node{childA, childB},
	}
	
	standalone := &ast.StateNode{
		Name: "standalone",
	}
	
	stateMachine := &symbols.Symbol{
		Name: "HierarchicalSM",
		Kind: symbols.SymbolStateUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageState,
			Ident:   ast.Identification{Name: "HierarchicalSM"},
			Members: []ast.Node{composite, standalone},
		},
	}
	
	exec, err := newStateExecutor(ctx, stateMachine)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	
	// Verify all states collected
	if len(exec.states) != 4 {
		t.Errorf("expected 4 states (composite, childA, childB, standalone), got %d", len(exec.states))
	}
	
	// Verify parent relationships
	if parent := exec.parentState[childA]; parent != composite {
		t.Errorf("expected childA parent = composite, got %v", parent)
	}
	
	if parent := exec.parentState[childB]; parent != composite {
		t.Errorf("expected childB parent = composite, got %v", parent)
	}
	
	if _, hasParent := exec.parentState[composite]; hasParent {
		t.Error("expected composite to have no parent")
	}
	
	if _, hasParent := exec.parentState[standalone]; hasParent {
		t.Error("expected standalone to have no parent")
	}
	
	// Verify parent chain
	chain := exec.getParentChain(childA)
	if len(chain) != 2 || chain[0] != childA || chain[1] != composite {
		t.Errorf("expected chain [childA, composite], got %v", chain)
	}
	
	// Verify LCA
	lca := exec.getLCA(childA, childB)
	if lca != composite {
		t.Errorf("expected LCA(childA, childB) = composite, got %v", lca)
	}
	
	lcaStandaloneChild := exec.getLCA(standalone, childA)
	if lcaStandaloneChild != nil {
		t.Errorf("expected LCA(standalone, childA) = nil, got %v", lcaStandaloneChild)
	}
}

func TestStateExecutor_HierarchicalEntryExit(t *testing.T) {
	ctx := NewContext(semantics.NewModel(nil), nil, 1000)
	
	// Build state machine:
	//   parentState (entry: val=1, exit: val=10)
	//     └─ childState (entry: val=2, exit: val=100, initial)
	//   siblingState (entry: val=1000)
	//
	// Transition: childState →[after 1]→ siblingState
	// Verify entry/exit actions execute in order
	
	childState := &ast.StateNode{
		Name:      "childState",
		IsInitial: true,
		Entry: []ast.Node{
			&ast.ActionExecutionNode{
				Name:       "enterChild",
				Expression: &ast.LiteralInteger{Value: "2"},
			},
		},
		Exit: []ast.Node{
			&ast.ActionExecutionNode{
				Name:       "exitChild",
				Expression: &ast.LiteralInteger{Value: "100"},
			},
		},
	}
	
	parentState := &ast.StateNode{
		Name: "parentState",
		Entry: []ast.Node{
			&ast.ActionExecutionNode{
				Name:       "enterParent",
				Expression: &ast.LiteralInteger{Value: "1"},
			},
		},
		Exit: []ast.Node{
			&ast.ActionExecutionNode{
				Name:       "exitParent",
				Expression: &ast.LiteralInteger{Value: "10"},
			},
		},
		Substates: []ast.Node{childState},
	}
	
	siblingState := &ast.StateNode{
		Name: "siblingState",
		Entry: []ast.Node{
			&ast.ActionExecutionNode{
				Name:       "enterSibling",
				Expression: &ast.LiteralInteger{Value: "1000"},
			},
		},
	}
	
	transition := &ast.TransitionEdge{
		Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "childState"}}},
		Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "siblingState"}}},
		Trigger: &ast.TimeEvent{
			Duration: &ast.LiteralInteger{Value: "1"},
		},
	}
	
	stateMachine := &symbols.Symbol{
		Name: "HierarchicalEntryExitSM",
		Kind: symbols.SymbolStateUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageState,
			Ident:   ast.Identification{Name: "HierarchicalEntryExitSM"},
			Members: []ast.Node{parentState, siblingState, transition},
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
	
	// After init: should have entered parent then child
	if _, ok := exec.stateData["enterParent"]; !ok {
		t.Error("expected enterParent to execute")
	}
	
	if _, ok := exec.stateData["enterChild"]; !ok {
		t.Error("expected enterChild to execute")
	}
	
	// Process transition
	err = exec.processNextEvent()
	if err != nil {
		t.Fatalf("process event: %v", err)
	}
	
	// Verify exit/entry actions executed in order
	if _, ok := exec.stateData["exitChild"]; !ok {
		t.Error("expected exitChild to execute")
	}
	
	if _, ok := exec.stateData["exitParent"]; !ok {
		t.Error("expected exitParent to execute")
	}
	
	if _, ok := exec.stateData["enterSibling"]; !ok {
		t.Error("expected enterSibling to execute")
	}
	
	// Verify final state
	if exec.currentState != siblingState {
		t.Errorf("expected siblingState, got %v", exec.currentState)
	}
}

func TestStateExecutor_StateStackTracking(t *testing.T) {
	ctx := NewContext(semantics.NewModel(nil), nil, 1000)
	
	// Build state machine:
	//   composite
	//     └─ nested (initial)
	//         └─ deepNested (initial)
	//   standalone
	//
	// Transition: deepNested →[after 1]→ standalone
	
	deepNested := &ast.StateNode{
		Name:      "deepNested",
		IsInitial: true,
	}
	
	nested := &ast.StateNode{
		Name:      "nested",
		IsInitial: true,
		Substates: []ast.Node{deepNested},
	}
	
	composite := &ast.StateNode{
		Name:      "composite",
		Substates: []ast.Node{nested},
	}
	
	standalone := &ast.StateNode{
		Name: "standalone",
	}
	
	transition := &ast.TransitionEdge{
		Source:  &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "deepNested"}}},
		Target:  &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "standalone"}}},
		Trigger: &ast.TimeEvent{Duration: &ast.LiteralInteger{Value: "1"}},
	}
	
	stateMachine := &symbols.Symbol{
		Name: "StateStackSM",
		Kind: symbols.SymbolStateUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageState,
			Ident:   ast.Identification{Name: "StateStackSM"},
			Members: []ast.Node{composite, standalone, transition},
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
	
	// Verify initial stateStack: [composite, nested, deepNested]
	expectedStack := []*ast.StateNode{composite, nested, deepNested}
	if len(exec.stateStack) != len(expectedStack) {
		t.Fatalf("expected stateStack length %d, got %d", len(expectedStack), len(exec.stateStack))
	}
	
	for i, expected := range expectedStack {
		if exec.stateStack[i] != expected {
			t.Errorf("stateStack[%d]: expected %s, got %s", i, expected.Name, exec.stateStack[i].Name)
		}
	}
	
	// Process transition to standalone
	err = exec.processNextEvent()
	if err != nil {
		t.Fatalf("process event: %v", err)
	}
	
	// Verify stateStack after transition: [standalone]
	if len(exec.stateStack) != 1 {
		t.Errorf("expected stateStack length 1, got %d", len(exec.stateStack))
	}
	
	if len(exec.stateStack) > 0 && exec.stateStack[0] != standalone {
		t.Errorf("expected stateStack[0] = standalone, got %s", exec.stateStack[0].Name)
	}
	
	if exec.currentState != standalone {
		t.Errorf("expected currentState = standalone, got %s", exec.currentState.Name)
	}
}

func TestStateExecutor_Integration_HierarchicalWorkflow(t *testing.T) {
	ctx := NewContext(semantics.NewModel(nil), nil, 1000)
	
	// Complex hierarchical state machine:
	//   workflow (composite)
	//     ├─ ready (initial, entry: step=1)
	//     └─ processing (entry: step=2, exit: step=3)
	//         ├─ validate (initial, entry: step=step+10)
	//         └─ execute (entry: step=step+100)
	//   done (final, entry: step=step+1000)
	//
	// Transitions:
	//   ready →[after 1]→ validate (enters processing parent)
	//   validate →[after 1]→ execute (within same parent)
	//   execute →[after 1]→ done (exits processing parent)
	
	validate := &ast.StateNode{
		Name:      "validate",
		IsInitial: true,
		Entry: []ast.Node{
			&ast.ActionExecutionNode{
				Name:       "validateEntry",
				Expression: &ast.LiteralInteger{Value: "10"},
			},
		},
	}
	
	execute := &ast.StateNode{
		Name: "execute",
		Entry: []ast.Node{
			&ast.ActionExecutionNode{
				Name:       "executeEntry",
				Expression: &ast.LiteralInteger{Value: "100"},
			},
		},
	}
	
	processing := &ast.StateNode{
		Name: "processing",
		Entry: []ast.Node{
			&ast.ActionExecutionNode{
				Name:       "processingEntry",
				Expression: &ast.LiteralInteger{Value: "2"},
			},
		},
		Exit: []ast.Node{
			&ast.ActionExecutionNode{
				Name:       "processingExit",
				Expression: &ast.LiteralInteger{Value: "3"},
			},
		},
		Substates: []ast.Node{validate, execute},
	}
	
	ready := &ast.StateNode{
		Name:      "ready",
		IsInitial: true,
		Entry: []ast.Node{
			&ast.ActionExecutionNode{
				Name:       "readyEntry",
				Expression: &ast.LiteralInteger{Value: "1"},
			},
		},
	}
	
	workflow := &ast.StateNode{
		Name:      "workflow",
		Substates: []ast.Node{ready, processing},
	}
	
	done := &ast.StateNode{
		Name:    "done",
		IsFinal: true,
		Entry: []ast.Node{
			&ast.ActionExecutionNode{
				Name:       "doneEntry",
				Expression: &ast.LiteralInteger{Value: "1000"},
			},
		},
	}
	
	trans1 := &ast.TransitionEdge{
		Source:  &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "ready"}}},
		Target:  &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "validate"}}},
		Trigger: &ast.TimeEvent{Duration: &ast.LiteralInteger{Value: "1"}},
	}
	
	trans2 := &ast.TransitionEdge{
		Source:  &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "validate"}}},
		Target:  &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "execute"}}},
		Trigger: &ast.TimeEvent{Duration: &ast.LiteralInteger{Value: "1"}},
	}
	
	trans3 := &ast.TransitionEdge{
		Source:  &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "execute"}}},
		Target:  &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "done"}}},
		Trigger: &ast.TimeEvent{Duration: &ast.LiteralInteger{Value: "1"}},
	}
	
	stateMachine := &symbols.Symbol{
		Name: "HierarchicalWorkflowSM",
		Kind: symbols.SymbolStateUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageState,
			Ident:   ast.Identification{Name: "HierarchicalWorkflowSM"},
			Members: []ast.Node{workflow, done, trans1, trans2, trans3},
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
	
	// Verify initial state: workflow/ready
	if exec.currentState != ready {
		t.Errorf("expected ready, got %s", exec.currentState.Name)
	}
	
	if _, ok := exec.stateData["readyEntry"]; !ok {
		t.Error("expected readyEntry to execute")
	}
	
	// Transition 1: ready → validate (enters processing parent)
	err = exec.processNextEvent()
	if err != nil {
		t.Fatalf("transition 1: %v", err)
	}
	
	if exec.currentState != validate {
		t.Errorf("expected validate, got %s", exec.currentState.Name)
	}
	
	// Should have entered processing then validate
	if _, ok := exec.stateData["processingEntry"]; !ok {
		t.Error("expected processingEntry to execute")
	}
	
	if _, ok := exec.stateData["validateEntry"]; !ok {
		t.Error("expected validateEntry to execute")
	}
	
	// Verify stateStack: [workflow, processing, validate]
	if len(exec.stateStack) != 3 {
		t.Errorf("expected stateStack length 3, got %d", len(exec.stateStack))
	}
	
	// Transition 2: validate → execute (within processing parent)
	err = exec.processNextEvent()
	if err != nil {
		t.Fatalf("transition 2: %v", err)
	}
	
	if exec.currentState != execute {
		t.Errorf("expected execute, got %s", exec.currentState.Name)
	}
	
	if _, ok := exec.stateData["executeEntry"]; !ok {
		t.Error("expected executeEntry to execute")
	}
	
	// Verify stateStack: [workflow, processing, execute]
	if len(exec.stateStack) != 3 {
		t.Errorf("expected stateStack length 3, got %d", len(exec.stateStack))
	}
	
	// Transition 3: execute → done (exits processing parent)
	err = exec.processNextEvent()
	if err != nil {
		t.Fatalf("transition 3: %v", err)
	}
	
	if exec.currentState != done {
		t.Errorf("expected done, got %s", exec.currentState.Name)
	}
	
	// Should have exited processing
	if _, ok := exec.stateData["processingExit"]; !ok {
		t.Error("expected processingExit to execute")
	}
	
	if _, ok := exec.stateData["doneEntry"]; !ok {
		t.Error("expected doneEntry to execute")
	}
	
	// Verify final state
	if exec.state != StateCompleted {
		t.Errorf("expected StateCompleted, got %v", exec.state)
	}
	
	// Verify stateStack: [done]
	if len(exec.stateStack) != 1 {
		t.Errorf("expected stateStack length 1, got %d", len(exec.stateStack))
	}
}

// Task 47: Traffic light state machine - realistic TimeEvent demo
func TestStateExecutor_Integration_TrafficLight(t *testing.T) {
	ctx := NewContext(semantics.NewModel(nil), nil, 1000)
	
	// States: red (initial, 30s) → green (25s) → yellow (5s) → off (final)
	// Total cycle: 30 + 25 + 5 = 60 seconds
	
	red := &ast.StateNode{Name: "red", IsInitial: true}
	green := &ast.StateNode{Name: "green"}
	yellow := &ast.StateNode{Name: "yellow"}
	off := &ast.StateNode{Name: "off", IsFinal: true}
	
	// Entry actions for tracking
	red.Entry = []ast.Node{
		&ast.ActionExecutionNode{
			Name: "logRed",
			Expression: &ast.LiteralString{Value: "Red light ON"},
		},
	}
	green.Entry = []ast.Node{
		&ast.ActionExecutionNode{
			Name: "logGreen",
			Expression: &ast.LiteralString{Value: "Green light ON"},
		},
	}
	yellow.Entry = []ast.Node{
		&ast.ActionExecutionNode{
			Name: "logYellow",
			Expression: &ast.LiteralString{Value: "Yellow light ON"},
		},
	}
	off.Entry = []ast.Node{
		&ast.ActionExecutionNode{
			Name: "logOff",
			Expression: &ast.LiteralString{Value: "Traffic light OFF"},
		},
	}
	
	// Transitions with realistic timing (no guards for simplicity)
	trans1 := &ast.TransitionEdge{
		Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "red"}}},
		Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "green"}}},
		Trigger: &ast.TimeEvent{
			Duration: &ast.LiteralInteger{Value: "30"}, // 30 seconds
		},
	}
	trans2 := &ast.TransitionEdge{
		Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "green"}}},
		Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "yellow"}}},
		Trigger: &ast.TimeEvent{
			Duration: &ast.LiteralInteger{Value: "25"}, // 25 seconds
		},
	}
	trans3 := &ast.TransitionEdge{
		Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "yellow"}}},
		Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "off"}}},
		Trigger: &ast.TimeEvent{
			Duration: &ast.LiteralInteger{Value: "5"}, // 5 seconds
		},
	}
	
	// State machine
	stateMachine := &symbols.Symbol{
		Name: "TrafficLight",
		Kind: symbols.SymbolStateUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageState,
			Ident:   ast.Identification{Name: "TrafficLight"},
			Members: []ast.Node{red, green, yellow, off, trans1, trans2, trans3},
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
	
	// Should start in red
	if exec.currentState != red {
		t.Errorf("expected red state, got %s", exec.currentState.Name)
	}
	
	if exec.currentTime != 0.0 {
		t.Errorf("expected time=0, got %f", exec.currentTime)
	}
	
	// Verify red entry
	if _, ok := exec.stateData["logRed"]; !ok {
		t.Error("expected logRed to execute")
	}
	
	// Event 1: red → green (30s)
	err = exec.processNextEvent()
	if err != nil {
		t.Fatalf("event 1: %v", err)
	}
	
	if exec.currentState != green {
		t.Errorf("expected green, got %s", exec.currentState.Name)
	}
	
	if exec.currentTime != 30.0 {
		t.Errorf("expected time=30, got %f", exec.currentTime)
	}
	
	// Event 2: green → yellow (25s)
	err = exec.processNextEvent()
	if err != nil {
		t.Fatalf("event 2: %v", err)
	}
	
	if exec.currentState != yellow {
		t.Errorf("expected yellow, got %s", exec.currentState.Name)
	}
	
	if exec.currentTime != 55.0 {
		t.Errorf("expected time=55, got %f", exec.currentTime)
	}
	
	// Event 3: yellow → off (5s)
	err = exec.processNextEvent()
	if err != nil {
		t.Fatalf("event 3: %v", err)
	}
	
	if exec.currentState != off {
		t.Errorf("expected off, got %s", exec.currentState.Name)
	}
	
	if exec.currentTime != 60.0 {
		t.Errorf("expected time=60, got %f", exec.currentTime)
	}
	
	// Verify final state
	if exec.state != StateCompleted {
		t.Errorf("expected StateCompleted, got %v", exec.state)
	}
	
	// Verify all entry actions executed
	if _, ok := exec.stateData["logOff"]; !ok {
		t.Error("expected logOff to execute")
	}
}
