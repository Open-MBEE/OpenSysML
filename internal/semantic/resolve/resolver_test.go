package resolve

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestResolverNewAndMemoEmpty(t *testing.T) {
	idx := symbols.NewIndex()
	r := New(idx)
	if r == nil {
		t.Fatalf("New returned nil")
	}
	if len(r.Diagnostics) != 0 {
		t.Fatalf("fresh resolver has diagnostics: %v", r.Diagnostics)
	}
}

func TestResolverMemoizes(t *testing.T) {
	idx := symbols.NewIndex()
	r := New(idx)
	qn := &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "X"}}}

	// First call records the result in the memo.
	_, ok1 := r.ResolveQualified(nil, qn)
	// Second call must return the memoized result without appending a
	// duplicate diagnostic.
	_, ok2 := r.ResolveQualified(nil, qn)

	if ok1 != ok2 {
		t.Fatalf("memoized result differs: %v vs %v", ok1, ok2)
	}
	if got := len(r.Diagnostics); got != 1 {
		t.Fatalf("diagnostics recorded %d times, want 1 (memoized)", got)
	}
}

// TestResolveSegments pins that names given as segments resolve as the qualified
// name they spell does.
func TestResolveSegments(t *testing.T) {
	src := `package P { part def Q { attribute x; } part q : Q; }`
	root := parser.New(source.New("s.sysml", []byte(src))).ParseFile()
	idx := symbols.NewIndexFromDoc("s.sysml", root)
	r := New(idx)
	scope := idx.DocumentRoot("s.sysml")
	want := idx.LookupQualified("P::Q::x")
	if len(want) != 1 {
		t.Fatalf("P::Q::x matched %d symbols", len(want))
	}
	if got, ok := r.ResolveSegments(scope, "P", "Q", "x"); !ok || got != want[0] {
		t.Errorf("ResolveSegments(P, Q, x) = %v, %v; want %v", got, ok, want[0])
	}
	if got, ok := r.ResolveSegments(scope, "P", "missing"); ok {
		t.Errorf("ResolveSegments(P, missing) resolved to %v", got)
	}
}
