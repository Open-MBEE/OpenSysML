package resolve

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func scopeOf(t *testing.T, parent *symbols.Scope, name string) *symbols.Scope {
	t.Helper()
	sym, ok := parent.LookupLocal(name)
	if !ok || sym.Scope == nil {
		t.Fatalf("child scope %q not found", name)
	}
	return sym.Scope
}

func TestResolveNameOutward(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package P { namespace Inner; namespace N { } }",
	})
	r := New(idx)
	root := idx.DocumentRoot("a.sysml")
	pScope := scopeOf(t, root, "P")
	nScope := scopeOf(t, pScope, "N")
	// From inside N, unqualified "Inner" resolves to P::Inner (ancestor scope).
	sym, ok := r.ResolveName(nScope, "Inner", &ast.FeatureReference{})
	if !ok {
		t.Fatalf("Inner unresolved from N; diagnostics=%v", r.Diagnostics)
	}
	if sym.Name != "Inner" {
		t.Fatalf("resolved name = %q, want Inner", sym.Name)
	}
}

func TestResolveNameUnresolved(t *testing.T) {
	idx := indexOf(t, map[string]string{"a.sysml": "package P { }"})
	r := New(idx)
	root := idx.DocumentRoot("a.sysml")
	pScope := scopeOf(t, root, "P")
	if _, ok := r.ResolveName(pScope, "Missing", &ast.FeatureReference{}); ok {
		t.Fatalf("Missing should be unresolved")
	}
	if len(r.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %d, want 1", len(r.Diagnostics))
	}
}

func TestResolveNameMemoizes(t *testing.T) {
	idx := indexOf(t, map[string]string{"a.sysml": "package P { }"})
	r := New(idx)
	root := idx.DocumentRoot("a.sysml")
	pScope := scopeOf(t, root, "P")
	at := &ast.FeatureReference{}
	r.ResolveName(pScope, "Missing", at)
	r.ResolveName(pScope, "Missing", at)
	if len(r.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %d, want 1 (memoized)", len(r.Diagnostics))
	}
}
