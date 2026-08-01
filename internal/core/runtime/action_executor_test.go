package runtime

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

func TestActionExecutor_Creation(t *testing.T) {
	ctx := NewContext(semantics.NewModel(nil), nil, 1000)
	
	// Create minimal action symbol
	action := &symbols.Symbol{
		Name: "TestAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageAction,
			Ident:   ast.Identification{Name: "TestAction"},
			Members: []ast.Node{},
		},
	}
	
	exec, err := newActionExecutor(ctx, action)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	
	if exec.ctx != ctx {
		t.Error("expected context to be set")
	}
	
	if exec.action != action {
		t.Error("expected action symbol to be set")
	}
	
	if exec.state != StateReady {
		t.Errorf("expected StateReady, got %v", exec.state)
	}
}

func TestActionExecutor_GraphExtraction(t *testing.T) {
	ctx := NewContext(semantics.NewModel(nil), nil, 1000)
	
	initial := &ast.InitialNode{Name: "start"}
	final := &ast.FinalNode{Name: "end"}
	edge := &ast.SuccessionEdge{
		Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}},
		Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "end"}}},
	}
	
	action := &symbols.Symbol{
		Name: "TestAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageAction,
			Ident:   ast.Identification{Name: "TestAction"},
			Members: []ast.Node{initial, final, edge},
		},
	}
	
	exec, err := newActionExecutor(ctx, action)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	
	if len(exec.nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(exec.nodes))
	}
	
	successors, ok := exec.edges[initial]
	if !ok || len(successors) != 1 || successors[0] != final {
		t.Error("expected edge from initial to final")
	}
}

func TestActionExecutor_InitialNode(t *testing.T) {
	ctx := NewContext(semantics.NewModel(nil), nil, 1000)
	
	// Create action: initial → final
	initial := &ast.InitialNode{Name: "start"}
	final := &ast.FinalNode{Name: "end"}
	
	action := &symbols.Symbol{
		Name: "SimpleAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageAction,
			Ident:   ast.Identification{Name: "SimpleAction"},
			Members: []ast.Node{
				initial,
				final,
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "end"}}},
				},
			},
		},
	}
	
	exec, err := newActionExecutor(ctx, action)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	
	// Initialize should spawn token at initial node
	err = exec.initialize()
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	
	if len(exec.tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(exec.tokens))
	}
	
	token := exec.tokens[0]
	if token.Location != initial {
		t.Error("expected token at initial node")
	}
	
	if exec.state != StateRunning {
		t.Errorf("expected StateRunning, got %v", exec.state)
	}
}

func TestActionExecutor_FinalNode(t *testing.T) {
	ctx := NewContext(semantics.NewModel(nil), nil, 1000)
	
	initial := &ast.InitialNode{Name: "start"}
	final := &ast.FinalNode{Name: "end"}
	
	action := &symbols.Symbol{
		Name: "SimpleAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageAction,
			Ident:   ast.Identification{Name: "SimpleAction"},
			Members: []ast.Node{
				initial,
				final,
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "end"}}},
				},
			},
		},
	}
	
	exec, err := newActionExecutor(ctx, action)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	
	exec.initialize()
	
	// Step from initial to final
	err = exec.stepToken(0)
	if err != nil {
		t.Fatalf("step initial: %v", err)
	}
	
	// Token should move to final
	if len(exec.tokens) != 1 {
		t.Fatalf("expected 1 token after step, got %d", len(exec.tokens))
	}
	
	if exec.tokens[0].Location != final {
		t.Error("expected token at final node")
	}
	
	// Step final node (should consume token and complete)
	err = exec.stepToken(0)
	if err != nil {
		t.Fatalf("step final: %v", err)
	}
	
	if len(exec.tokens) != 0 {
		t.Errorf("expected 0 tokens after final, got %d", len(exec.tokens))
	}
	
	if exec.state != StateCompleted {
		t.Errorf("expected StateCompleted, got %v", exec.state)
	}
}

func TestActionExecutor_ActionExecutionNode(t *testing.T) {
	ctx := NewContext(semantics.NewModel(nil), nil, 1000)
	
	initial := &ast.InitialNode{Name: "start"}
	action := &ast.ActionExecutionNode{
		Name:       "compute",
		Expression: &ast.LiteralInteger{Value: "42"},
	}
	final := &ast.FinalNode{Name: "end"}
	
	// Build action: initial → compute → final
	actionSym := &symbols.Symbol{
		Name: "ComputeAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageAction,
			Ident:   ast.Identification{Name: "ComputeAction"},
			Members: []ast.Node{
				initial,
				action,
				final,
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "compute"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "compute"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "end"}}},
				},
			},
		},
	}
	
	exec, err := newActionExecutor(ctx, actionSym)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	
	exec.initialize()
	exec.stepToken(0) // start → compute
	
	// Step compute node
	err = exec.stepToken(0)
	if err != nil {
		t.Fatalf("step compute: %v", err)
	}
	
	// Token should have result in data
	token := exec.tokens[0]
	result, ok := token.Data["result"]
	if !ok {
		t.Error("expected result in token data")
	}
	
	// Check result value
	if result.Kind != ValConst {
		t.Errorf("expected ValConst, got %v", result.Kind)
	}
	
	if result.Const.Kind != semantics.ValInt || result.Const.Int != 42 {
		t.Errorf("expected integer 42, got %v", result.Const)
	}
	
	// Token should move to final node
	if token.Location != final {
		t.Error("expected token at final node")
	}
}

func TestActionExecutor_ForkNode(t *testing.T) {
	ctx := NewContext(semantics.NewModel(nil), nil, 1000)
	
	initial := &ast.InitialNode{Name: "start"}
	fork := &ast.ForkNode{Name: "split"}
	action1 := &ast.ActionExecutionNode{Name: "task1"}
	action2 := &ast.ActionExecutionNode{Name: "task2"}
	action3 := &ast.ActionExecutionNode{Name: "task3"}
	
	// Build action: initial → fork → [task1, task2, task3]
	actionSym := &symbols.Symbol{
		Name: "ForkAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageAction,
			Ident:   ast.Identification{Name: "ForkAction"},
			Members: []ast.Node{
				initial, fork, action1, action2, action3,
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}},
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task1"}}}},
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task2"}}}},
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task3"}}}},
			},
		},
	}
	
	exec, err := newActionExecutor(ctx, actionSym)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	
	exec.initialize()
	exec.stepToken(0) // start → fork
	
	// Step fork (1 token → 3 tokens)
	err = exec.stepToken(0)
	if err != nil {
		t.Fatalf("step fork: %v", err)
	}
	
	if len(exec.tokens) != 3 {
		t.Fatalf("expected 3 tokens after fork, got %d", len(exec.tokens))
	}
	
	// Verify all tokens at correct locations
	locations := make(map[ast.Node]bool)
	for _, token := range exec.tokens {
		locations[token.Location] = true
	}
	
	if !locations[action1] || !locations[action2] || !locations[action3] {
		t.Error("expected tokens at task1, task2, task3")
	}
}
