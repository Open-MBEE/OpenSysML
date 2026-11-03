package semantics

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

func fqnsOf(m *Model, syms []*symbols.Symbol) string {
	names := make([]string, 0, len(syms))
	for _, s := range syms {
		names = append(names, m.fqnOf(s))
	}
	return strings.Join(names, " ")
}

// FeatureTypeSetWithImplied is KerML §8.3.3.3.4 deriveFeatureType over the
// implied specializations too: a part typed by an item definition is also typed
// by Parts::Part through the implied Parts::parts subsetting (SysML v2 §7.10).
func TestFeatureTypeSetWithImpliedReachesTheKindsBaseDefinition(t *testing.T) {
	const src = `package P {
	item def I;
	part def PD;
	part p : I;
	part r : PD;
	part s;
	item def J { part q : I; }
	attribute a : ScalarValues::Real;
}`
	m, root := libraryModelWithDoc(t, "implied-types.sysml", src)
	cases := []struct {
		path        string
		declared    string
		withImplied string
	}{
		{path: "P::p", declared: "P::I", withImplied: "P::I Parts::Part"},
		{path: "P::J::q", declared: "P::I", withImplied: "P::I Parts::Part"},
		{path: "P::r", declared: "P::PD", withImplied: "P::PD"},
		{path: "P::s", declared: "Parts::Part", withImplied: "Parts::Part"},
		{path: "P::a", declared: "ScalarValues::Real", withImplied: "ScalarValues::Real"},
	}
	for _, tc := range cases {
		sym := nestedSym(t, root, tc.path)
		if got := fqnsOf(m, m.FeatureTypeSet(sym)); got != tc.declared {
			t.Errorf("FeatureTypeSet(%s) = %q, want %q", tc.path, got, tc.declared)
		}
		if got := fqnsOf(m, m.FeatureTypeSetWithImplied(sym)); got != tc.withImplied {
			t.Errorf("FeatureTypeSetWithImplied(%s) = %q, want %q", tc.path, got, tc.withImplied)
		}
	}
}
