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

// chainNoteAllocs is what noting the declarations of one chain n times allocates
// once the chain was resolved, as a loop re-evaluating the chain notes them.
func chainNoteAllocs(t *testing.T, n int) float64 {
	t.Helper()
	ctx, idx := libraryShapeContext(t, chainDeclarationsModel)
	ec := newEvalContext(ctx, lookupOne(t, idx, "test").Scope)
	chain, ok := parseExpr(t, "rig.panel.area").(*ast.FeatureChainExpr)
	if !ok {
		t.Fatal("rig.panel.area did not parse as a feature chain")
	}
	base, parts := ast.FeatureChainBase(chain)
	ec.restoreChain(ec.noteChainMembers(base, parts))
	if decls := ec.chainDeclarations(base, parts); len(decls) != 2 || decls[0] != lookupOne(t, idx, "test::Rig::panel") || decls[1] != lookupOne(t, idx, "test::Panel::area") {
		t.Fatalf("chainDeclarations(rig, panel.area) = %v, want Rig::panel and Panel::area", decls)
	}
	return testing.AllocsPerRun(20, func() {
		for i := 0; i < n; i++ {
			ec.restoreChain(ec.noteChainMembers(base, parts))
		}
	})
}

// The declarations a chain's segments name are resolved once per model: noting
// them for twice the evaluations allocates no more than for half of them.
func TestChainDeclarationsAreResolvedOncePerModel(t *testing.T) {
	small, large := chainNoteAllocs(t, 50), chainNoteAllocs(t, 100)
	t.Logf("50 notes allocate %.0f objects, 100 notes %.0f", small, large)
	if large > small {
		t.Errorf("100 chain evaluations allocate %.0f objects, 50 allocate %.0f: the segments are re-resolved per evaluation", large, small)
	}
}
