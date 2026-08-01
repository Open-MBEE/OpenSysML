package runtime

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

func TestContextIDAllocation(t *testing.T) {
	idx := symbols.NewIndex()
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	ctx := NewContext(model, resolver, 100000)
	
	id1 := ctx.allocateID()
	id2 := ctx.allocateID()
	
	if id1 == id2 {
		t.Error("expected unique IDs, got duplicates")
	}
	if id1 != 1 || id2 != 2 {
		t.Errorf("expected sequential IDs 1,2; got %d,%d", id1, id2)
	}
}

func TestContextStepCounter(t *testing.T) {
	idx := symbols.NewIndex()
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	ctx := NewContext(model, resolver, 10)
	
	for i := 0; i < 10; i++ {
		if err := ctx.incrementStep(); err != nil {
			t.Fatalf("step %d failed: %v", i, err)
		}
	}
	
	// 11th step should error
	if err := ctx.incrementStep(); err == nil {
		t.Error("expected step limit error, got nil")
	}
}

func TestContext_ExecuteAction(t *testing.T) {
	idx := symbols.NewIndex()
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	ctx := NewContext(model, resolver, 100000)
	
	// Create simple action: initial → action(x=42) → final
	initial := &ast.InitialNode{Name: "start"}
	actionNode := &ast.ActionExecutionNode{
		Name: "compute",
		Expression: &ast.LiteralInteger{
			Value: "42",
		},
	}
	final := &ast.FinalNode{Name: "end"}
	
	edge1 := &ast.SuccessionEdge{
		Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}},
		Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "compute"}}},
	}
	edge2 := &ast.SuccessionEdge{
		Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "compute"}}},
		Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "end"}}},
	}
	
	actionSym := &symbols.Symbol{
		Name: "TestAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageAction,
			Ident:   ast.Identification{Name: "TestAction"},
			Members: []ast.Node{initial, actionNode, final, edge1, edge2},
		},
	}
	
	// Execute
	result, err := ctx.ExecuteAction(actionSym)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	
	// Verify result contains action output
	val, ok := result["result"]
	if !ok {
		t.Fatal("expected 'result' key in output")
	}
	
	if val.Kind != ValConst {
		t.Errorf("expected ValConst, got %v", val.Kind)
	}
	if val.Const.Kind != semantics.ValInt {
		t.Errorf("expected ValInt, got %v", val.Const.Kind)
	}
	if val.Const.Int != 42 {
		t.Errorf("expected 42, got %d", val.Const.Int)
	}
}

func TestContext_ExecuteAction_InvalidSymbol(t *testing.T) {
	idx := symbols.NewIndex()
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	ctx := NewContext(model, resolver, 100000)
	
	// Pass non-action symbol
	notAction := &symbols.Symbol{
		Name: "NotAction",
		Kind: symbols.SymbolPartUsage,
		Decl: &ast.Usage{Kind: ast.UsagePart},
	}
	
	_, err := ctx.ExecuteAction(notAction)
	if err == nil {
		t.Error("expected error for non-action symbol, got nil")
	}
}

func TestContext_ExecuteState(t *testing.T) {
	idx := symbols.NewIndex()
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	ctx := NewContext(model, resolver, 100000)
	
	// Create simple state machine: idle →[after 5]→ done (final)
	idle := &ast.StateNode{
		Name:      "idle",
		IsInitial: true,
	}
	done := &ast.StateNode{
		Name:    "done",
		IsFinal: true,
	}
	
	trans := &ast.TransitionEdge{
		Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "idle"}}},
		Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "done"}}},
		Trigger: &ast.TimeEvent{
			Duration: &ast.LiteralReal{Value: "5.0"},
		},
	}
	
	stateMachineSym := &symbols.Symbol{
		Name: "TestStateMachine",
		Kind: symbols.SymbolStateUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageState,
			Ident:   ast.Identification{Name: "TestStateMachine"},
			Members: []ast.Node{idle, done, trans},
		},
	}
	
	// Execute
	result, err := ctx.ExecuteState(stateMachineSym)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	
	// Verify result is stateData map (may be empty for this simple machine)
	if result == nil {
		t.Error("expected non-nil result")
	}
}

func TestContext_ExecuteState_InvalidSymbol(t *testing.T) {
	idx := symbols.NewIndex()
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	ctx := NewContext(model, resolver, 100000)
	
	// Pass non-state symbol
	notState := &symbols.Symbol{
		Name: "NotState",
		Kind: symbols.SymbolPartUsage,
		Decl: &ast.Usage{Kind: ast.UsagePart},
	}
	
	_, err := ctx.ExecuteState(notState)
	if err == nil {
		t.Error("expected error for non-state symbol, got nil")
	}
}

// Task 49: Integration test - combined action + state machine
// State machine entry action invokes an action execution
func TestContext_Integration_ActionWithinState(t *testing.T) {
	idx := symbols.NewIndex()
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	ctx := NewContext(model, resolver, 100000)
	
	// Create a simple action: compute = 10 + 20
	initial := &ast.InitialNode{
		Name: "initial",
	}
	compute := &ast.ActionExecutionNode{
		Name: "compute",
		Expression: &ast.OperatorExpr{
			Operator: ast.OpAdd,
			Operands: []ast.Node{
				&ast.LiteralInteger{Value: "10"},
				&ast.LiteralInteger{Value: "20"},
			},
		},
	}
	final := &ast.FinalNode{
		Name: "final",
	}
	edge1 := &ast.ControlFlowEdge{
		Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "initial"}}},
		Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "compute"}}},
	}
	edge2 := &ast.ControlFlowEdge{
		Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "compute"}}},
		Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "final"}}},
	}
	
	actionSym := &symbols.Symbol{
		Name: "ComputeAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageAction,
			Ident:   ast.Identification{Name: "ComputeAction"},
			Members: []ast.Node{initial, compute, final, edge1, edge2},
		},
	}
	
	// Execute action standalone first
	actionResult, err := ctx.ExecuteAction(actionSym)
	if err != nil {
		t.Fatalf("action execution failed: %v", err)
	}
	
	// Verify action result
	resultVal, ok := actionResult["result"]
	if !ok {
		t.Fatal("expected 'result' in action output")
	}
	if resultVal.Kind != ValConst {
		t.Fatalf("expected const value, got %v", resultVal.Kind)
	}
	if resultVal.Const.Kind != semantics.ValInt || resultVal.Const.Int != 30 {
		t.Errorf("expected result=30, got %v", resultVal.Const)
	}
	
	// Create state machine with entry action
	// State 'processing' executes the action on entry
	processing := &ast.StateNode{
		Name:      "processing",
		IsInitial: true,
		Entry: []ast.Node{
			&ast.ActionExecutionNode{
				Name: "entryAction",
				Expression: &ast.OperatorExpr{
					Operator: ast.OpMul,
					Operands: []ast.Node{
						&ast.LiteralInteger{Value: "3"},
						&ast.LiteralInteger{Value: "7"},
					},
				},
			},
		},
	}
	done := &ast.StateNode{
		Name:    "done",
		IsFinal: true,
	}
	trans := &ast.TransitionEdge{
		Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "processing"}}},
		Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "done"}}},
		Trigger: &ast.TimeEvent{
			Duration: &ast.LiteralReal{Value: "1.0"},
		},
	}
	
	stateSym := &symbols.Symbol{
		Name: "ProcessingStateMachine",
		Kind: symbols.SymbolStateUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageState,
			Ident:   ast.Identification{Name: "ProcessingStateMachine"},
			Members: []ast.Node{processing, done, trans},
		},
	}
	
	// Execute state machine
	stateResult, err := ctx.ExecuteState(stateSym)
	if err != nil {
		t.Fatalf("state machine execution failed: %v", err)
	}
	
	// Verify entry action executed (stored in stateData)
	entryVal, ok := stateResult["entryAction"]
	if !ok {
		t.Fatal("expected 'entryAction' in stateData")
	}
	if entryVal.Kind != ValConst {
		t.Fatalf("expected const value, got %v", entryVal.Kind)
	}
	if entryVal.Const.Kind != semantics.ValInt || entryVal.Const.Int != 21 {
		t.Errorf("expected entryAction=21 (3*7), got %v", entryVal.Const)
	}
}
