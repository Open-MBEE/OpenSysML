package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// dimensionOneContext builds a runtime over the quantity libraries with a
// generator whose efficiency is a DimensionOneValue, once written in the
// library's `one` and once as a bare number, a percent unit scaling `one`, and
// a `unity` the model declares as `one` again.
func dimensionOneContext(t *testing.T) (*Context, *symbols.Scope) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			public import ISQ::*;
			public import SI::*;
			public import ScalarValues::*;
			public import QuantityCalculations::*;
			public import MeasurementReferences::*;
			attribute <'%'> percent : DimensionOneUnit { :>> unitConversion : ConversionByConvention { :>> referenceUnit = one; :>> conversionFactor = 0.01; } }
			attribute unity : DimensionOneUnit = one;
			part def Gen {
				attribute power : PowerValue;
				attribute efficiency : DimensionOneValue;
				calc deliveredEnergy { in duration : DurationValue; return : EnergyValue = power * duration * efficiency; }
			}
			part rated : Gen { attribute :>> power = 800.0 [W]; attribute :>> efficiency = 0.7 [one]; }
			part rated2 : Gen { attribute :>> power = 800.0 [W]; attribute :>> efficiency = 0.7; }
		}
	`))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	return ctx, pkg.Scope
}

// TestDimensionOneIdentityInProducts: `MeasurementReferences::one` is the
// identity of the unit product. It leaves any product it shares with another
// unit, whichever side of a multiplication or division it enters from, and
// any power of it alone is itself — so a product with a `one` factor folds to
// the same derived unit the product without it reaches. Once every other unit
// cancels it is the unit again, however the operations are grouped. Units that
// merely have dimension one (`rad`) or scale `one` (`%`) are not the identity
// and stay; a unit the model declares as `one` again is the same identity.
func TestDimensionOneIdentityInProducts(t *testing.T) {
	ctx, scope := dimensionOneContext(t)

	cases := []struct{ src, want string }{
		{"1 [one] * 800 [W] * 120 [s]", "96000 [SI::'kg⋅m²⋅s⁻²']"},
		{"800 [W] * 120 [s] * 1 [one]", "96000 [SI::'kg⋅m²⋅s⁻²']"},
		{"(800 [W] * 120 [s]) / 1 [one]", "96000.0 [SI::'kg⋅m²⋅s⁻²']"},
		{"0.7 [one] * 800 [W]", "560.0 [W]"},
		{"800 [W] * 0.7 [one]", "560.0 [W]"},
		{"800 [W] / 0.5 [one]", "1600.0 [W]"},
		{"2 [one] * 3 [one]", "6 [one]"},
		{"1 [one] / 1 [one]", "1.0 [one]"},
		{"(2 [one] * 3 [m]) / 3 [m]", "2.0 [one]"},
		{"2 [one] * (3 [m] / 3 [m])", "2.0 [one]"},
		{"(2 [one] * 3 [rad]) / 3 [rad]", "2.0 [one]"},
		{"(0.7 [one] * 800 [W] * 120 [s]) / (800 [W] * 120 [s])", "0.7 [one]"},
		{"rated.deliveredEnergy(120.0 [s]) / (800.0 [W] * 120.0 [s])", "0.7 [one]"},
		{"(2 [one] * 3 [J]) / 3 ['N⋅m']", "2.0 [one]"},
		{"2 [one] * (3 [J] / 3 ['N⋅m'])", "2.0 [one]"},
		{"(2 [one] * 3 [km]) / 3 [m]", "2000.0 [one]"},
		{"1 [one] * 1 [unity]", "1 [one]"},
		{"1 [unity] * 1 [one]", "1 [one]"},
		{"(2 [one]) ** 3", "8 [one]"},
		{"sqrt(9.0 [one])", "3.0 [one]"},
		{"0.7 [one]", "0.7 [one]"},
		{"3 [rad] * 1 [one]", "3 [rad]"},
		{"2 [m] * 3 [rad]", "6 [m*rad]"},
		{"50 ['%'] * 800 [W]", "40000 ['%'*W]"},
		{"50 ['%'] + 0.25 [one]", "75.0 ['%']"},
		{"rated.deliveredEnergy(120.0 [s])", "67200.0 [SI::J]"},
		{"rated2.deliveredEnergy(120.0 [s])", "67200.0 [SI::J]"},
		{"rated.power * rated.efficiency", "560.0 [W]"},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			got, err := evalIn(t, ctx, scope, tc.src)
			if err != nil {
				t.Fatalf("%s: %v", tc.src, err)
			}
			if got.Kind != ValQuantity {
				t.Fatalf("%s = %v (%s), want a quantity", tc.src, got, got.Kind)
			}
			if got.Quantity().String() != tc.want {
				t.Errorf("%s = %s, want %s", tc.src, got.Quantity(), tc.want)
			}
		})
	}
}

// TestDimensionOneIdentityIsTheUnitItLeaves: a quantity `one` left carries a
// real reference to the unit that remains, so its measurement reference is
// that library unit and it compares equal to a literal written in it; and the
// identity leaves measurement-reference products the same way.
func TestDimensionOneIdentityIsTheUnitItLeaves(t *testing.T) {
	ctx, scope := dimensionOneContext(t)

	booleans := []struct {
		src  string
		want bool
	}{
		{"(1 [one] * 800 [W]).mRef == W", true},
		{"(800 [W] / 1 [one]).mRef == W", true},
		{"rated.deliveredEnergy(120.0 [s]).mRef == J", true},
		{"2 [W] * 3 [s] * 1 [one] == 6 [J]", true},
		{"rated.deliveredEnergy(120.0 [s]) >= 0.0 [SI::J]", true},
		{"(3 [rad] * 1 [one]).mRef == rad", true},
		{"(50 ['%'] * 2 [m]).mRef == m", false},
		{"((2 [one] * 3 [m]) / 3 [m]).mRef == one", true},
		{"(2 [one] * (3 [m] / 3 [m])).mRef == one", true},
		{"(1 [one] * 1 [unity]).mRef == (1 [unity] * 1 [one]).mRef", true},
		{"(1 [unity]).mRef == one", true},
		{"1 [unity] * 2 [m] == 2 [m] * 1 [one]", true},
	}
	for _, tc := range booleans {
		got, err := evalIn(t, ctx, scope, tc.src)
		if err != nil {
			t.Errorf("%s: %v", tc.src, err)
			continue
		}
		if !got.isBool() || got.Const.Bool != tc.want {
			t.Errorf("%s = %v, want %v", tc.src, got, tc.want)
		}
	}

	refs := []struct{ src, want string }{
		{"one * m", "m"},
		{"m * one", "m"},
		{"m / one", "m"},
		{"one * one", "one"},
		{"one ** 2", "one"},
		{"one * W * s", "W*s"},
		{"one", "one"},
		{"one * rad", "rad"},
		{"'%' * m", "'%'*m"},
		{"one * m / m", "one"},
		{"one * unity", "one"},
		{"unity * one", "one"},
	}
	for _, tc := range refs {
		got, err := evalIn(t, ctx, scope, tc.src)
		if err != nil {
			t.Errorf("%s: %v", tc.src, err)
			continue
		}
		if got.Kind != ValMeasurementRef {
			t.Errorf("%s = %v (%s), want a measurement reference", tc.src, got, got.Kind)
			continue
		}
		if s := got.MeasurementRef().String(); s != tc.want {
			t.Errorf("%s = %s, want %s", tc.src, s, tc.want)
		}
	}
}
