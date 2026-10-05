package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// Replacing a document drops the relationship metaobjects synthesized for its
// declarations, so an edit never reads a relationship of the previous tree.
func TestOwnedRelationshipsFollowTheirDocument(t *testing.T) {
	const src = `package P {
		part def B { attribute x; }
		part def D :> B { attribute :>> x; }
	}`
	m, r, replace := trackedModel(t, stdlibIndex(t), "rels.sysml")
	for i := 0; i < 3; i++ {
		scope := replace(src)
		x := sym(t, sym(t, sym(t, scope, "P").Scope, "D").Scope, "x")
		var rels []*symbols.Symbol
		r.InDocument("rels.sysml", func() { rels = m.OwnedRelationshipSymbols(x) })
		if len(rels) != 1 || m.relationshipInfo[rels[0]].kind != ast.RelRedefines {
			t.Fatalf("edit %d: owned relationships of D::x = %v, want one redefinition", i, rels)
		}
		if len(m.ownedRelationships) != 1 || len(m.relationshipInfo) != 1 {
			t.Fatalf("edit %d: %d owners and %d relationships memoized, want 1 and 1",
				i, len(m.ownedRelationships), len(m.relationshipInfo))
		}
	}
}
