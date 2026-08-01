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
