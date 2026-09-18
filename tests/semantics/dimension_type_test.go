package semantics_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// dimensionModel indexes the bundled libraries, which is what makes quantity
// value types and their measurement references resolve.
func dimensionModel(t *testing.T) (*semantics.Model, *symbols.Index) {
	t.Helper()
	idx := libs.NewModelIndex()
	idx.AddDocument("<t>", parser.New(source.New("<t>", []byte(
		"package T { private import ISQ::*; attribute t : DurationValue; }"))).ParseFile())
	idx.ExpandWildcardImports()
	return semantics.NewModel(resolve.New(idx)), idx
}

func dimensionSymbol(t *testing.T, idx *symbols.Index, fqn string) *symbols.Symbol {
	t.Helper()
	matches := idx.LookupQualified(fqn)
	if len(matches) != 1 {
		t.Fatalf("%s matched %d symbols, want 1", fqn, len(matches))
	}
	return matches[0]
}

// TestDimensionOfTypeFixesTheDeclaredKind: a quantity value type states the
// dimension of the values it types, which is what a write target declared by it
// must answer to.
func TestDimensionOfTypeFixesTheDeclaredKind(t *testing.T) {
	m, idx := dimensionModel(t)
	cases := []struct {
		fqn  string
		want string
	}{
		{"ISQBase::DurationValue", "T"},
		{"ISQSpaceTime::SpeedValue", "L·T^-1"},
		{"MeasurementReferences::DimensionOneValue", "1"},
	}
	for _, tc := range cases {
		t.Run(tc.fqn, func(t *testing.T) {
			got, ok := m.DimensionOfType(dimensionSymbol(t, idx, tc.fqn))
			if !ok {
				t.Fatalf("%s states no dimension", tc.fqn)
			}
			if got.String() != tc.want {
				t.Fatalf("dimension of %s = %s, want %s", tc.fqn, got, tc.want)
			}
		})
	}
}

// TestDimensionOfTypeIsSilentWhereNothingIsFixed: a type outside the quantity
// hierarchy, and one whose measurement reference is any unit, determine no
// dimension rather than a dimensionless one.
func TestDimensionOfTypeIsSilentWhereNothingIsFixed(t *testing.T) {
	m, idx := dimensionModel(t)
	for _, fqn := range []string{
		"ScalarValues::Real",
		"ScalarValues::Boolean",
		"Quantities::ScalarQuantityValue",
	} {
		if dim, ok := m.DimensionOfType(dimensionSymbol(t, idx, fqn)); ok {
			t.Errorf("%s reports dimension %s, want none", fqn, dim)
		}
	}
	if _, ok := m.DimensionOfType(nil); ok {
		t.Error("a missing type reports a dimension")
	}
}

// TestDimensionOfTypeThroughAnAlias: an alias fixes the dimension of the type it
// names.
func TestDimensionOfTypeThroughAnAlias(t *testing.T) {
	m, idx := dimensionModel(t)
	got, ok := m.DimensionOfType(dimensionSymbol(t, idx, "ISQ::TemperatureValue"))
	if !ok {
		t.Fatal("ISQ::TemperatureValue states no dimension")
	}
	if got.String() != "Θ" {
		t.Fatalf("dimension of ISQ::TemperatureValue = %s, want Θ", got)
	}
}

// TestDimensionOfTypeMatchesTheFeatureItTypes: a feature and its declared type
// answer with one dimension, so the runtime's target and the static one agree.
func TestDimensionOfTypeMatchesTheFeatureItTypes(t *testing.T) {
	m, idx := dimensionModel(t)
	feature := dimensionSymbol(t, idx, "T::t")
	ofFeature, ok := m.DimensionOfFeature(feature)
	if !ok {
		t.Fatal("the feature states no dimension")
	}
	ofType, ok := m.DimensionOfType(dimensionSymbol(t, idx, "ISQBase::DurationValue"))
	if !ok {
		t.Fatal("the type states no dimension")
	}
	if !ofFeature.Term.Commensurable(ofType.Term) {
		t.Fatalf("feature dimension %s and type dimension %s disagree", ofFeature, ofType)
	}
}
