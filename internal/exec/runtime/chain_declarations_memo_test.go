package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

const chainDeclarationsModel = `package test {
	part def Panel { attribute area : ScalarValues::Real; }
	part def Rig { part panel : Panel; }
	part rig : Rig;
}`

// chainNoteAllocs is what noting the declarations of one nested chain n times
// allocates beyond flattening it, as a loop re-evaluating the chain does both.
func chainNoteAllocs(t *testing.T, n int) (float64, int) {
	t.Helper()
	ctx, idx := libraryShapeContext(t, chainDeclarationsModel)
	ec := newEvalContext(ctx, lookupOne(t, idx, "test").Scope)
	chain, ok := parseExpr(t, "rig.panel.area").(*ast.FeatureChainExpr)
	if !ok {
		t.Fatal("rig.panel.area did not parse as a feature chain")
	}
	if _, nested := chain.Operand.(*ast.FeatureChainExpr); !nested {
		t.Fatal("rig.panel.area did not parse as a nested chain")
	}
	base, parts := ast.FeatureChainBase(chain)
	if decls := ec.chainDeclarations(chain, base, parts); len(decls) != 2 || decls[0] != lookupOne(t, idx, "test::Rig::panel") || decls[1] != lookupOne(t, idx, "test::Panel::area") {
		t.Fatalf("chainDeclarations(rig, panel.area) = %v, want Rig::panel and Panel::area", decls)
	}
	flatten := testing.AllocsPerRun(20, func() {
		for i := 0; i < n; i++ {
			ast.FeatureChainBase(chain)
		}
	})
	note := testing.AllocsPerRun(20, func() {
		for i := 0; i < n; i++ {
			base, parts := ast.FeatureChainBase(chain)
			ec.restoreChain(ec.noteChainMembers(chain, base, parts))
		}
	})
	return note - flatten, len(ctx.model.chainDeclarations)
}

// The declarations a chain's segments name are resolved once per model and chain
// expression, however often the nested chain is flattened afresh: noting them for
// twice the evaluations allocates no more, and the memo holds one entry.
func TestChainDeclarationsAreResolvedOncePerModel(t *testing.T) {
	small, _ := chainNoteAllocs(t, 50)
	large, entries := chainNoteAllocs(t, 100)
	t.Logf("50 notes allocate %.0f objects beyond flattening, 100 notes %.0f; %d memo entries", small, large, entries)
	if large > small {
		t.Errorf("100 chain evaluations allocate %.0f objects beyond flattening, 50 allocate %.0f: the segments are re-resolved per evaluation", large, small)
	}
	if entries != 1 {
		t.Errorf("memo holds %d entries for one chain expression, want 1", entries)
	}
}
