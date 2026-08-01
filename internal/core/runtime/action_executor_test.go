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
