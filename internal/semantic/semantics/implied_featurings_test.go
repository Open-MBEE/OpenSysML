package semantics

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func featuringFQNs(m *Model, types []FeaturingType) []string {
	var out []string
	for _, t := range types {
		out = append(out, m.resolver.Index().GetFQN(t.Type))
	}
	slices.Sort(out)
	return out
}

// A connector that names itself as both related features has no featuring type to derive, and stops.
func TestImpliedTypeFeaturingsOfASelfRelatedConnector(t *testing.T) {
	m, root := buildModelWithLibraryNamedKind(t, "self.kerml", source.KindKerML, `package P { connector c from c to c; }`)
	if got := m.DerivedFeaturingTypes(auditLibraryBases(t, root, "P::c")); len(got) != 0 {
		t.Errorf("P::c: featuring types %v, want none", featuringFQNs(m, got))
	}
}

// KerML checkConnectorTypeFeaturing holds for a connector no namespace but the root owns.
func TestImpliedTypeFeaturingsOfARootConnector(t *testing.T) {
	m, root := buildModelWithLibraryNamedKind(t, "root.kerml", source.KindKerML,
		`class C; assoc A { end a : C; end b : C; } class O { feature x : C; feature y : C; } connector top : A from O::x to O::y;`)
	if got, want := featuringFQNs(m, m.ImpliedTypeFeaturings(auditLibraryBases(t, root, "top"))), []string{"O"}; !slices.Equal(got, want) {
		t.Errorf("top: implied featurings %v, want %v", got, want)
	}
}

// KerML checkFeatureOwnedCrossFeatureTypeFeaturing reads the other end an association inherits too.
func TestImpliedTypeFeaturingsOfACrossFeatureOppositeAnInheritedEnd(t *testing.T) {
	m, root := buildModelWithLibraryNamedKind(t, "inherited.kerml", source.KindKerML, `package P {
		class C; class D;
		assoc A { end a : C; end b : D; }
		assoc B specializes A { end [1] feature a2 : C redefines a; }
	}`)
	cross := m.OwnedCrossFeature(auditLibraryBases(t, root, "P::B::a2"))
	if cross == nil {
		t.Fatal("P::B::a2 owns no cross feature")
	}
	if got, want := featuringFQNs(m, m.ImpliedTypeFeaturings(cross)), []string{"P::D"}; !slices.Equal(got, want) {
		t.Errorf("the cross feature of P::B::a2: implied featurings %v, want %v", got, want)
	}
}
