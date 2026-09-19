package semantics_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// measurementRefModel indexes the libraries with a package whose features are
// bound to unit expressions, so both a unit and an expression over units resolve.
func measurementRefModel(t *testing.T) (*semantics.Model, *symbols.Index) {
	t.Helper()
	idx := libs.NewModelIndex()
	idx.AddDocument("<t>", parser.New(source.New("<t>", []byte(`package T {
		private import ISQ::*;
		private import SI::*;
		private import Time::*;
		attribute area = m * m;
		attribute duration = s * s / s;
		attribute speed = m / s;
		attribute cubed = m ** 3;
		attribute ratio = km / m;
		attribute sum = m + m;
		attribute one = 1 * 1;
		attribute unity = 1 / 1;
		attribute square = 1 ** 2;
		attribute twice = 2 * m;
		attribute whole = m / 1;
		attribute e : ScalarValues::Real = 2.0;
		attribute power = m ** e;
		attribute cubedPower = (m ** 3) ** e;
		attribute wrongPower = m ** "2";
		attribute numberPower = 2 ** e;
		attribute inferred = area;
		attribute inferredSum = sum;
		alias metre for SI::m;
		attribute viaAlias = metre * m;
		attribute notAUnit = e * m;
		attribute unresolved = nothing * m;
	}`))).ParseFile())
	idx.ExpandWildcardImports()
	return semantics.NewModel(resolve.New(idx)), idx
}

func boundOperatorExpr(t *testing.T, idx *symbols.Index, fqn string) (*symbols.Scope, *ast.OperatorExpr) {
	t.Helper()
	sym := dimensionSymbol(t, idx, fqn)
	u, ok := sym.Decl.(*ast.Usage)
	if !ok || u.Value == nil {
		t.Fatalf("%s is not a usage with a value", fqn)
	}
	e, ok := u.Value.(*ast.OperatorExpr)
	if !ok {
		t.Fatalf("%s is bound to a %T, want an operator expression", fqn, u.Value)
	}
	return sym.OwnerScope, e
}

// TestMeasurementRefConforms: a unit conforms to the unit definition typing it and
// to every measurement-reference supertype; a composed unit, typed DerivedUnit,
// conforms to a unit definition of its dimension and to no other; a quantity value
// type, or a measurement scale, is never the type of a unit, whatever the dimension.
func TestMeasurementRefConforms(t *testing.T) {
	m, idx := measurementRefModel(t)
	metre, err := m.UnitTermOf(dimensionSymbol(t, idx, "SI::m"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := m.UnitTermOf(dimensionSymbol(t, idx, "SI::s"))
	if err != nil {
		t.Fatal(err)
	}
	lengthUnit := dimensionSymbol(t, idx, "ISQBase::LengthUnit")
	derivedUnit := dimensionSymbol(t, idx, "MeasurementReferences::DerivedUnit")
	cases := []struct {
		name  string
		typ   *symbols.Symbol
		unit  semantics.UnitTerm
		want  string
		holds bool
		found string
	}{
		{"m : LengthUnit", lengthUnit, metre, "ISQBase::LengthUnit", true, ""},
		{"m : ScalarMeasurementReference", lengthUnit, metre, "MeasurementReferences::ScalarMeasurementReference", true, ""},
		{"m : TensorMeasurementReference", lengthUnit, metre, "MeasurementReferences::TensorMeasurementReference", true, ""},
		{"s : LengthUnit", dimensionSymbol(t, idx, "ISQBase::DurationUnit"), second, "ISQBase::LengthUnit", false, "a measurement reference of dimension T"},
		{"m*m : AreaUnit", derivedUnit, metre.Times(metre), "ISQSpaceTime::AreaUnit", true, ""},
		{"m*m : LengthUnit", derivedUnit, metre.Times(metre), "ISQBase::LengthUnit", false, "a measurement reference of dimension L^2"},
		{"m*m : MeasurementUnit", derivedUnit, metre.Times(metre), "MeasurementReferences::MeasurementUnit", true, ""},
		{"m*m : DerivedUnit", derivedUnit, metre.Times(metre), "MeasurementReferences::DerivedUnit", true, ""},
		{"m : DerivedUnit", lengthUnit, metre, "MeasurementReferences::DerivedUnit", false, "a measurement reference typed LengthUnit"},
		{"m : LengthValue", lengthUnit, metre, "ISQBase::LengthValue", false, "a measurement reference typed LengthUnit"},
		{"m*m : AreaValue", derivedUnit, metre.Times(metre), "ISQSpaceTime::AreaValue", false, "a measurement reference typed DerivedUnit"},
		{"m : Real", lengthUnit, metre, "ScalarValues::Real", false, "a measurement reference typed LengthUnit"},
		{"s : TimeScale", dimensionSymbol(t, idx, "ISQBase::DurationUnit"), second, "Time::TimeScale", false, "a measurement reference typed DurationUnit"},
		{"s*s/s : TimeScale", derivedUnit, second.Times(second).DividedBy(second), "Time::TimeScale", false, "a measurement reference typed DerivedUnit"},
		{"s*s/s : IntervalScale", derivedUnit, second.Times(second).DividedBy(second), "MeasurementReferences::IntervalScale", false, "a measurement reference typed DerivedUnit"},
		{"s*s/s : MeasurementScale", derivedUnit, second.Times(second).DividedBy(second), "MeasurementReferences::MeasurementScale", false, "a measurement reference typed DerivedUnit"},
		{"s*s/s : DurationUnit", derivedUnit, second.Times(second).DividedBy(second), "ISQBase::DurationUnit", true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := m.MeasurementRefConforms(tc.typ, tc.unit, dimensionSymbol(t, idx, tc.want))
			if !c.Known {
				t.Fatalf("conformance of %s is unknown", tc.name)
			}
			if c.Holds != tc.holds {
				t.Fatalf("conformance of %s holds = %v, want %v (found %q)", tc.name, c.Holds, tc.holds, c.Found)
			}
			if !strings.Contains(c.Found, tc.found) {
				t.Fatalf("conformance of %s found %q, want it to mention %q", tc.name, c.Found, tc.found)
			}
		})
	}
	if c := m.MeasurementRefConforms(nil, metre, lengthUnit); c.Known {
		t.Error("a reference of no type judged known")
	}
}

// TestMeasurementRefExpr: `*`, `/` and `**` over units are a DerivedUnit of the
// composed dimension; `+` over units, and `*`, `/` or `**` with a number for an
// operand (`1 * 1`, `2 * m`), are not references, though unit notation reads `1`.
func TestMeasurementRefExpr(t *testing.T) {
	m, idx := measurementRefModel(t)
	derivedUnit := dimensionSymbol(t, idx, "MeasurementReferences::DerivedUnit")
	for _, tc := range []struct {
		feature string
		want    string
		holds   bool
	}{
		{"T::area", "ISQSpaceTime::AreaUnit", true},
		{"T::area", "ISQBase::LengthUnit", false},
		{"T::speed", "ISQSpaceTime::SpeedUnit", true},
		{"T::speed", "ISQBase::LengthUnit", false},
		{"T::cubed", "ISQSpaceTime::VolumeUnit", true},
		{"T::ratio", "MeasurementReferences::DerivedUnit", true},
		{"T::ratio", "ISQBase::LengthUnit", false},
		{"T::duration", "ISQBase::DurationUnit", true},
		{"T::duration", "Time::TimeScale", false},
		{"T::duration", "MeasurementReferences::IntervalScale", false},
		{"T::viaAlias", "ISQSpaceTime::AreaUnit", true},
	} {
		t.Run(tc.feature+" : "+tc.want, func(t *testing.T) {
			scope, e := boundOperatorExpr(t, idx, tc.feature)
			if got := m.MeasurementRefExprType(scope, e); got != derivedUnit {
				t.Fatalf("type of %s = %v, want DerivedUnit", tc.feature, got)
			}
			c, ok := m.MeasurementRefExprConformance(scope, e, dimensionSymbol(t, idx, tc.want))
			if !ok || !c.Known {
				t.Fatalf("%s against %s is not judged (%v, %v)", tc.feature, tc.want, ok, c.Known)
			}
			if c.Holds != tc.holds {
				t.Fatalf("%s conforms to %s = %v, want %v (found %q)", tc.feature, tc.want, c.Holds, tc.holds, c.Found)
			}
		})
	}
	for _, feature := range []string{"T::sum", "T::one", "T::unity", "T::square", "T::twice", "T::whole", "T::wrongPower", "T::numberPower", "T::notAUnit", "T::unresolved"} {
		scope, e := boundOperatorExpr(t, idx, feature)
		if got := m.MeasurementRefExprType(scope, e); got != nil {
			t.Errorf("%s typed %v, want no measurement reference", feature, got)
		}
		if _, ok := m.MeasurementRefExprConformance(scope, e, derivedUnit); ok {
			t.Errorf("%s judged as a measurement reference", feature)
		}
	}
}

// TestMeasurementRefPowerOfUnknownExponent: a unit raised to a Real the checker
// cannot fold is still a DerivedUnit, though of no dimension it can judge; the
// runtime reduces it once the exponent is known.
func TestMeasurementRefPowerOfUnknownExponent(t *testing.T) {
	m, idx := measurementRefModel(t)
	derivedUnit := dimensionSymbol(t, idx, "MeasurementReferences::DerivedUnit")
	for _, feature := range []string{"T::power", "T::cubedPower"} {
		scope, e := boundOperatorExpr(t, idx, feature)
		if got := m.MeasurementRefExprType(scope, e); got != derivedUnit {
			t.Errorf("type of %s = %v, want DerivedUnit", feature, got)
		}
		if got := m.ExprResultType(scope, e); got != derivedUnit {
			t.Errorf("result type of %s = %v, want DerivedUnit", feature, got)
		}
		if _, ok := m.MeasurementRefExprConformance(scope, e, dimensionSymbol(t, idx, "ISQSpaceTime::AreaUnit")); ok {
			t.Errorf("%s judged against a dimension its exponent does not fix", feature)
		}
	}
}

// TestMeasurementRefExprResultType: the result type of a unit expression, and so
// of an untyped feature bound to one or to such a feature, is DerivedUnit; a number
// composed with a number keeps the DataValue the Kernel function declares.
func TestMeasurementRefExprResultType(t *testing.T) {
	m, idx := measurementRefModel(t)
	derivedUnit := dimensionSymbol(t, idx, "MeasurementReferences::DerivedUnit")
	dataValue := dimensionSymbol(t, idx, "Base::DataValue")
	for _, tc := range []struct {
		feature string
		want    *symbols.Symbol
	}{
		{"T::area", derivedUnit},
		{"T::speed", derivedUnit},
		{"T::cubed", derivedUnit},
		{"T::power", derivedUnit},
		{"T::inferred", derivedUnit},
		{"T::inferredSum", dataValue},
		{"T::sum", dataValue},
		{"T::one", dataValue},
		{"T::twice", dataValue},
		{"T::numberPower", dataValue},
	} {
		sym := dimensionSymbol(t, idx, tc.feature)
		u, ok := sym.Decl.(*ast.Usage)
		if !ok || u.Value == nil {
			t.Fatalf("%s is not a usage with a value", tc.feature)
		}
		if got := m.ExprResultType(sym.OwnerScope, u.Value); got != tc.want {
			t.Errorf("result type of %s = %v, want %v", tc.feature, got, tc.want)
		}
		if got := m.ExprResultTypes(sym.OwnerScope, u.Value); len(got) != 1 || got[0] != tc.want {
			t.Errorf("result types of %s = %v, want %v", tc.feature, got, tc.want)
		}
	}
}
