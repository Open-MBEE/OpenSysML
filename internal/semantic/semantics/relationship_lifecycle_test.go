package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// Replacing a document drops the relationship metaobjects synthesized for its
// declarations and the chaining features they own, so an edit never reads a
// relationship of the previous tree.
func TestOwnedRelationshipsFollowTheirDocument(t *testing.T) {
	const src = `package P {
		part def B { attribute x; part sub { attribute w; } }
		part def D :> B { attribute :>> x; attribute deep :>> sub.w; }
	}`
	m, r, replace := trackedModel(t, stdlibIndex(t), "rels.sysml")
	for i := 0; i < 3; i++ {
		scope := replace(src)
		d := sym(t, sym(t, scope, "P").Scope, "D").Scope
		x, deep := sym(t, d, "x"), sym(t, d, "deep")
		var rels, deepRels []*symbols.Symbol
		var chain *symbols.Symbol
		r.InDocument("rels.sysml", func() {
			rels = m.ImplicitRelationships(x)
			deepRels = m.ImplicitRelationships(deep)
			if len(deepRels) == 1 {
				chain = m.chainTargetFeature(deepRels[0])
			}
		})
		if len(rels) != 1 || rels[0].Implicit.Kind != ast.RelRedefines {
			t.Fatalf("edit %d: relationships of D::x = %v, want one redefinition", i, rels)
		}
		if chain == nil {
			t.Fatalf("edit %d: relationships of D::deep = %v, want one redefinition targeting a chain", i, deepRels)
		}
		if len(m.implicitRels) != 2 || len(m.chainTargets) != 1 {
			t.Fatalf("edit %d: %d owners and %d chain targets memoized, want 2 and 1",
				i, len(m.implicitRels), len(m.chainTargets))
		}
	}
}
