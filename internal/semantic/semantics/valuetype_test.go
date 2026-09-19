package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// invocationResultConformance judges the call `F()` at the document root
// against the library type fqn.
func invocationResultConformance(t *testing.T, src, fqn string) Conformance {
	t.Helper()
	m, root := stdlibModelWithDoc(t, "result.kerml", src)
	call := &ast.InvocationExpr{
		Type: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "F"}}},
	}
	return m.ExprConformsToLibrary(root, call, fqn)
}

// An untyped result parameter implicitly redefines `Performance::result` (typed
// Anything), which says nothing about it: the call's result is untyped.
func TestExprConformsToUntypedResultParameter(t *testing.T) {
	c := invocationResultConformance(t, "function F { return r; }", fqnNatural)
	if !c.Known || c.Holds || !c.Untyped {
		t.Fatalf("untyped result: got %+v, want known, not holding, untyped", c)
	}
}

// A result typed by a class is a known result: it is neither Natural nor untyped.
func TestExprConformsToClassResultParameter(t *testing.T) {
	c := invocationResultConformance(t, "class C; function F { return : C; }", fqnNatural)
	if !c.Known || c.Holds || c.Untyped || c.Found != "C" {
		t.Fatalf("class result: got %+v, want known, not holding, found C", c)
	}
	c = invocationResultConformance(t, "function F { return : ScalarValues::Natural; }", fqnNatural)
	if !c.Known || !c.Holds {
		t.Fatalf("natural result: got %+v, want it to hold", c)
	}
}
