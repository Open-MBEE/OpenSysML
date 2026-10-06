package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Two relationships written on one recorded owner share its declaration span,
// so their ordinals break the tie.
func TestCompareSymbolsImplicitOrdinal(t *testing.T) {
	owner := &symbols.Symbol{Name: "p", DocName: "pkg.sysml", DeclSpan: source.Span{Offset: 5, Len: 10}}
	first := &symbols.Symbol{Kind: symbols.SymbolRelationship, DocName: "pkg.sysml",
		DeclSpan: owner.DeclSpan,
		Implicit: &symbols.ImplicitRelationship{Owner: owner, Ordinal: 0}}
	second := &symbols.Symbol{Kind: symbols.SymbolRelationship, DocName: "pkg.sysml",
		DeclSpan: owner.DeclSpan,
		Implicit: &symbols.ImplicitRelationship{Owner: owner, Ordinal: 1}}
	if compareSymbols(first, first) != 0 {
		t.Error("one relationship does not compare equal to itself")
	}
	if compareSymbols(first, second) == 0 || compareSymbols(second, first) == 0 {
		t.Error("distinct relationships of one owner compare equal")
	}
	if compareSymbols(first, second) != -compareSymbols(second, first) {
		t.Error("relationship ordering is not antisymmetric")
	}
}
