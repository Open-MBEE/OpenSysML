package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// A satisfy or verify is typed by the requirement it reference-subsets (KerML §8.3.3.3.9: a
// ReferenceSubsetting has a Subsetting's semantics) and stays composite; a `references` clause is no such edge.
func TestSatisfyReferenceSubsettingTypesTheUsage(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
		requirement def R;
		requirement r : R;
		requirement group { requirement nested : R; }
		part v;
		part context {
			satisfy r by v;
			satisfy group.nested by v;
			requirement q references r;
		}
		verification def V { objective o { verify r; } }
	}`)
	def := nestedSym(t, root, "P::R")
	var satisfies, verifies []*symbols.Symbol
	nestedSym(t, root, "P::context").Scope.WalkDeclarations(func(s *symbols.Symbol) {
		if s.Kind == symbols.SymbolSatisfyRequirementUsage {
			satisfies = append(satisfies, s)
		}
	})
	nestedSym(t, root, "P::V::o").Scope.WalkDeclarations(func(s *symbols.Symbol) { verifies = append(verifies, s) })
	if len(satisfies) != 2 || len(verifies) != 1 {
		t.Fatalf("found %d satisfy and %d verify usages, want 2 and 1", len(satisfies), len(verifies))
	}
	for _, usage := range append(satisfies, verifies...) {
		if got := typeNames(m.FeatureTypes(usage)); got != "R" {
			t.Errorf("FeatureTypes = %q, want R", got)
		}
		if !m.Conforms(usage, def) {
			t.Error("the usage does not conform to the definition of the requirement it names")
		}
		if !m.UsageIsComposite(usage) {
			t.Error("the usage is referential; only an EventOccurrenceUsage's reference makes it so (SysML v2 §8.3.9.3)")
		}
	}
	if q := nestedSym(t, root, "P::context::q"); m.Conforms(q, def) {
		t.Error("a `references` clause made its feature conform to the referenced feature's type")
	}
}
