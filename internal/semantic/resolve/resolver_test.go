package resolve

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
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
