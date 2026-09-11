package semantics_test

import (
	"errors"
	"math"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// coherentModel declares a derived unit of the model's own, so a result may be
// spelt by a library unit or fall back to the base units it reduces to.
const coherentModel = `private import SI::*;
private import ISQ::*;
private import MeasurementReferences::*;

package C {
	attribute 'm³⋅s⁻²' : DerivedUnit {
		private attribute m3 : UnitPowerFactor[1] { :>> unit = SI::m; :>> exponent = 3; }
		private attribute s_2 : UnitPowerFactor[1] { :>> unit = SI::s; :>> exponent = -2; }
		attribute :>> unitPowerFactors = (m3, s_2);
	}
	attribute 'kph' : DerivedUnit {
		private attribute km1 : UnitPowerFactor[1] { :>> unit = SI::km; :>> exponent = 1; }
		private attribute h_1 : UnitPowerFactor[1] { :>> unit = SI::h; :>> exponent = -1; }
		attribute :>> unitPowerFactors = (km1, h_1);
	}
	attribute energy :> ISQ::energy;
	attribute torque :> ISQ::torque;
	attribute frequency :> ISQ::frequency;
	attribute area :> ISQ::area;
}`

func coherentFixture(t *testing.T) (*semantics.Model, *symbols.Index) {
	t.Helper()
	idx := libs.NewModelIndex()
	p := parser.New(source.New("c.sysml", []byte(coherentModel)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx.AddDocument("c.sysml", root)
	idx.ExpandWildcardImports()
	return semantics.NewModel(resolve.New(idx)), idx
}

// coherentFold parses a quantity expression at the fixture's root and folds it.
func coherentFold(t *testing.T, m *semantics.Model, idx *symbols.Index, expr string) semantics.Quantity {
	t.Helper()
	p := parser.New(source.New("<expr>", []byte(expr)))
	node := p.ParseExpression()
	if node == nil || len(p.Diagnostics) != 0 {
		t.Fatalf("parse %q: %v", expr, p.Diagnostics)
	}
	q, ok := m.EvalQuantity(idx.DocumentRoot("c.sysml"), node)
	if !ok {
		t.Fatalf("%q does not fold to a quantity", expr)
	}
	return q
}

func lookupOne(t *testing.T, idx *symbols.Index, fqn string) *symbols.Symbol {
	t.Helper()
	matches := symbols.PreferDeclared(idx.LookupQualified(fqn))
	if len(matches) != 1 {
		t.Fatalf("%s matched %d symbols, want 1", fqn, len(matches))
	}
	return matches[0]
}

// TestCoherentQuantitySpellsTheLibraryUnit: a coherent composed unit is spelt by the
// library's shortest unit for its dimension; the magnitude is untouched.
func TestCoherentQuantitySpellsTheLibraryUnit(t *testing.T) {
	m, idx := coherentFixture(t)
	cases := []struct {
		expr, unit string
		num        float64
	}{
		{"9.80665 ['m⋅s⁻²'] * 311 [s]", "SI::'m/s'", 3049.86815},
		{"10 [N] / 2 [kg]", "SI::'m⋅s⁻²'", 5},
		{"6 [m] / 2 [s]", "SI::'m/s'", 3},
		{"3 [km] / 2 [s]", "SI::'m/s'", 1500},
		{"3 [C::'kph'] * 2 [s]", "SI::m", 6 / 3.6},
		{"(3.986E14 [C::'m³⋅s⁻²'] / 6563000 [m]) ** 0.5", "SI::'m/s'", 7793.229127559948},
		{"(3.986E14 [C::'m³⋅s⁻²'] / 6563 [km]) ** 0.5", "SI::'m/s'", 7793.229127559948},
		{"3 [m] * 2 [m]", "SI::'m²'", 6},
		{"3 [m] * 2 [m] * 2 [m]", "SI::'m³'", 12},
	}
	for _, tc := range cases {
		q, err := m.CoherentQuantity(coherentFold(t, m, idx, tc.expr), nil)
		if err != nil {
			t.Fatalf("%s: %v", tc.expr, err)
		}
		if q.Unit.Text != tc.unit {
			t.Errorf("%s: unit %s, want %s", tc.expr, q.Unit.Text, tc.unit)
		}
		if got := q.Num.AsReal(); math.Abs(got-tc.num) > 1e-9*math.Abs(tc.num) {
			t.Errorf("%s: magnitude %v, want %v", tc.expr, got, tc.num)
		}
	}
}

// TestCoherentQuantityFallsBackToBaseUnits: where a dimension's coherent units measure
// different kinds (`J`, `N⋅m`), the base-unit-defined one or the base units spell it; the declared type picks.
func TestCoherentQuantityFallsBackToBaseUnits(t *testing.T) {
	m, idx := coherentFixture(t)
	cases := []struct {
		expr, declared, unit string
	}{
		{"3.986E14 [C::'m³⋅s⁻²'] / 6563 [km]", "", "SI::'m²⋅s⁻²'"},
		{"3 [m] * 2 [m]", "C::area", "SI::'m²'"},
		{"3 [N] * 2 [m]", "", "SI::'kg⋅m²⋅s⁻²'"},
		{"1 [m/s] * 1 [kg/s]", "", "SI::N"},
		{"6 [m] / 2 [s] / 3 [kg]", "", "m/(kg*s)"},
		{"3 [N] * 2 [m]", "C::energy", "SI::J"},
		{"3 [N] * 2 [m]", "C::torque", "SI::'N⋅m'"},
		{"1 / 2 [s]", "", "1/s"},
		{"1 / 2 [s]", "C::frequency", "SI::Hz"},
	}
	for _, tc := range cases {
		var declared *symbols.Symbol
		if tc.declared != "" {
			declared = lookupOne(t, idx, tc.declared)
		}
		q, err := m.CoherentQuantity(coherentFold(t, m, idx, tc.expr), declared)
		if err != nil {
			t.Fatalf("%s: %v", tc.expr, err)
		}
		if q.Unit.Text != tc.unit {
			t.Errorf("%s (%s): unit %s, want %s", tc.expr, tc.declared, q.Unit.Text, tc.unit)
		}
	}
}

// TestCoherentQuantityKeepsWhatItCannotReduce: a named unit, a dimensionless one, a
// dimension-one factor and an unreducible fractional power are returned as written.
func TestCoherentQuantityKeepsWhatItCannotReduce(t *testing.T) {
	m, idx := coherentFixture(t)
	for _, expr := range []string{"3 [km]", "3 [C::'kph'] * 2", "4 [m] / 2 [m]", "2 [rad] * 3 [m]", "4 [m] ** 0.5"} {
		before := coherentFold(t, m, idx, expr)
		after, err := m.CoherentQuantity(before, nil)
		if err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		if after.Unit.Text != before.Unit.Text || after.Num != before.Num {
			t.Errorf("%s: %s changed to %s", expr, &before, &after)
		}
	}
}

// TestCoherentSpellingFollowsTheDeclaredType: a quantity bound to a typed feature is
// spelt by a unit its measurement reference admits; an admitted or scaled unit is left as written.
func TestCoherentSpellingFollowsTheDeclaredType(t *testing.T) {
	m, idx := coherentFixture(t)
	cases := []struct {
		expr, declared, unit string
	}{
		{"3 [J]", "C::torque", "SI::'N⋅m'"},
		{"3 ['N⋅m']", "C::energy", "SI::J"},
		{"3 ['kg⋅m²⋅s⁻²']", "C::energy", "SI::J"},
		{"3 [J]", "C::energy", "J"},
		{"3 [kJ]", "C::torque", "kJ"},
		{"3 [m] * 2 [m]", "C::area", "SI::'m²'"},
		{"3 [km] * 2 [km]", "C::area", "km**2"},
		{"1 / 2 [s]", "C::frequency", "SI::Hz"},
	}
	for _, tc := range cases {
		before := coherentFold(t, m, idx, tc.expr)
		after := m.CoherentSpelling(before, lookupOne(t, idx, tc.declared))
		if after.Unit.Text != tc.unit {
			t.Errorf("%s (%s): unit %s, want %s", tc.expr, tc.declared, after.Unit.Text, tc.unit)
		}
		if after.Num != before.Num {
			t.Errorf("%s (%s): magnitude %v changed to %v", tc.expr, tc.declared, before.Num, after.Num)
		}
	}
}

// TestUnitOfExprReadsAUnitAsAQuantityCarriesIt: the text, the product of the named units
// and the reduction agree with folding a quantity in that unit; a name that is no unit
// is ErrNotAUnit; a spelling by short names drops the qualifiers.
func TestUnitOfExprReadsAUnitAsAQuantityCarriesIt(t *testing.T) {
	m, idx := coherentFixture(t)
	scope := idx.DocumentRoot("c.sysml")
	parse := func(text string) ast.Node {
		p := parser.New(source.New("<unit>", []byte(text)))
		node := p.ParseExpression()
		if node == nil || len(p.Diagnostics) != 0 {
			t.Fatalf("parse %q: %v", text, p.Diagnostics)
		}
		return node
	}
	for _, text := range []string{"SI::km/SI::h", "m/s**2", "kW", "C::kph"} {
		unit, err := m.UnitOfExpr(scope, parse(text))
		if err != nil {
			t.Fatalf("UnitOfExpr(%q): %v", text, err)
		}
		q := coherentFold(t, m, idx, "1 ["+text+"]")
		if unit.Text != text || unit.Product.String() != q.Unit.Product.String() || !unit.Term.Same(q.Unit.Term) {
			t.Errorf("UnitOfExpr(%q) = %v %v %v, want the unit of %v", text, unit.Text, unit.Product, unit.Term, q)
		}
	}
	if _, err := m.UnitOfExpr(scope, parse("C::energy")); !errors.Is(err, semantics.ErrNotAUnit) {
		t.Errorf("UnitOfExpr(C::energy) = %v, want ErrNotAUnit", err)
	}
	if _, err := m.UnitOfExpr(scope, parse("nothing")); !errors.Is(err, semantics.ErrNotAUnit) {
		t.Errorf("UnitOfExpr(nothing) = %v, want ErrNotAUnit", err)
	}
	for text, want := range map[string]string{"SI::km/SI::h": "km/h", "SI::kg*SI::m**2": "kg*m**2", "C::kph": "kph", "SI::'m/s'": "'m/s'"} {
		unit, err := m.UnitOfExpr(scope, parse(text))
		if err != nil {
			t.Fatalf("UnitOfExpr(%q): %v", text, err)
		}
		if got := unit.Product.ShortSpelling().String(); got != want {
			t.Errorf("%q spelt short = %q, want %q", text, got, want)
		}
	}
}
