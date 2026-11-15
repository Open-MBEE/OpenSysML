package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Expose is an alternative of ViewBodyItem alone (SysML v2 8.2.2.26.2), so only a
// view usage owns one (8.3.26.2 validateExposeOwningNamespace):
// ViewDefinitionBodyItem offers a filter and a rendering, and no Expose.
func TestExposeIsAViewUsageBodyItemOnly(t *testing.T) {
	admitted := []struct{ name, src string }{
		{"view usage", "view v { expose P::*; }"},
		{"typed view usage", "view def V; view v : V { expose P::**; }"},
		{"filtered", "view v { expose P::**[@M]; }"},
		{"membership", "view v { expose P::x; }"},
		{"view usage in a view def", "view def V { view v { expose P::*; } }"},
		{"view usage in a part def", "part def D { view v { expose P::*; } }"},
		{"beside a filter and a rendering", "view v { filter @M; expose P::*; render asTable; }"},
	}
	for _, c := range admitted {
		t.Run(c.name, func(t *testing.T) {
			p := New(source.New("t.sysml", []byte(c.src)))
			root := p.ParseFile()
			if len(p.Diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
			}
			if !exposeParsed(root) {
				t.Fatalf("no expose in the tree:\n%s", ast.Dump(root))
			}
		})
	}

	rejected := []struct{ name, src string }{
		{"view def", "view def V { expose P::*; }"},
		{"view def, recursive", "view def V { expose P::**; }"},
		{"view def, filtered", "view def V { expose P::**[@M]; }"},
		{"view def, membership", "view def V { expose P::x; }"},
		{"view def with visibility", "view def V { protected expose P::*; }"},
		{"view def beside its own items", "view def V { filter @M; expose P::*; render asTable; }"},
		{"view def in a view usage", "view v { view def V { expose P::*; } }"},
		{"specializing view def", "view def V; view def W :> V { expose P::*; }"},
		{"part usage in a view usage", "view v { part p { expose P::*; } }"},
		{"rendering in a view def", "view def V { render r { expose P::*; } }"},
		{"viewpoint def", "viewpoint def V { expose P::*; }"},
		{"part def", "part def D { expose P::*; }"},
	}
	for _, c := range rejected {
		t.Run(c.name, func(t *testing.T) {
			p := New(source.New("t.sysml", []byte(c.src)))
			root := p.ParseFile()
			if len(p.Diagnostics) != 1 {
				t.Fatalf("want the owning-body diagnostic alone, got %v", p.Diagnostics)
			}
			d := p.Diagnostics[0]
			if d.Code != "expose-owning-namespace" || !strings.Contains(d.Message, "only allowed in a view usage body") {
				t.Fatalf("diagnostic = [%s] %s", d.Code, d.Message)
			}
			if exposeParsed(root) {
				t.Fatalf("misplaced expose kept as an import:\n%s", ast.Dump(root))
			}
			if !strings.Contains(ast.Dump(root), "ErrorNode") {
				t.Fatalf("expected an ErrorNode:\n%s", ast.Dump(root))
			}
		})
	}
}

func exposeParsed(root ast.Node) bool {
	found := false
	ast.Inspect(root, func(n ast.Node) bool {
		if imp, ok := n.(*ast.Import); ok && imp.IsExpose {
			found = true
		}
		return !found
	})
	return found
}
