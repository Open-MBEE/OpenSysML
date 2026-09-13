package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// emptyAggregateModel declares collections of every kind an aggregate is taken
// over, each without a member, so that what an empty aggregate yields is read.
// They are read on the instantiated rig: at model level a valueless `[*]`
// collection is undetermined, on an object it is empty.
const emptyAggregateModel = `
	package test {
		public import ScalarValues::*;
		public import ISQ::*;
		public import SI::*;
		public import QuantityCalculations::*;
		public import ControlFunctions::*;
		public import CollectionFunctions::*;
		public import SequenceFunctions::size;
		part def Part {
			attribute mass : MassValue;
			attribute spares : MassValue[*];
		}
		part def Rig {
			attribute masses : MassValue[*];
			attribute heavy : MassValue[*] = (1 [kg], 2 [kg]);
			part crew : Part[2];
			attribute lengths : LengthValue[*];
			attribute forces : ForceValue[*];
			attribute speeds : SpeedValue[*];
			attribute temperatures : ThermodynamicTemperatureValue[*];
			attribute counts : Integer[*];
			attribute reals : Real[*];
			part parts : Part[*];
			attribute mass : MassValue = 10 [kg];
			attribute total : MassValue = mass + sum(parts.mass);
			attribute grams : MassValue = 500 [g] + sum(masses);
			attribute inverted : MassValue = sum(masses) + mass;
			attribute area : AreaValue = sum(lengths->collect{in l : LengthValue; l * l});
			attribute mapped : MassValue = sum(parts.{in p : Part; p.mass});
			attribute mappedTotal : MassValue = mass + parts->collect{in p : Part; p.mass}->sum();
			attribute spares : MassValue = sum(crew->collect{in p : Part; p.spares});
		}
		part rig : Rig;
	}
`

func emptyAggregateContext(t *testing.T) (*Context, *symbols.Scope) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, emptyAggregateModel))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	if _, err := ctx.Instantiate(lookupOne(t, idx, "test::rig")); err != nil {
		t.Fatalf("Instantiate rig: %v", err)
	}
	return ctx, pkg.Scope
}

// TestEmptyQuantityAggregateKeepsTheDeclaredKind: the sum of no quantities is
// the zero of the collection's declared quantity kind, in its coherent SI unit,
// so it adds to a quantity of that kind; a product of none is the number one,
// and a count stays a number.
func TestEmptyQuantityAggregateKeepsTheDeclaredKind(t *testing.T) {
	ctx, scope := emptyAggregateContext(t)

	cases := []struct{ src, want string }{
		{"sum(rig.masses)", "0 [kg]"},
		{"rig.masses->sum()", "0 [kg]"},
		{"NumericalFunctions::sum(rig.masses)", "0 [kg]"},
		{"RealFunctions::sum(rig.masses)", "0.0 [kg]"},
		{"sum(rig.lengths)", "0 [m]"},
		{"sum(rig.forces)", "0 [kg*m/s**2]"},
		{"sum(rig.speeds)", "0 [m/s]"},
		{"sum(rig.temperatures)", "0 [K]"},
		{"sum(rig.parts.mass)", "0 [kg]"},
		{"sum(rig.counts)", "0"},
		{"sum(rig.reals)", "0"},
		{"RealFunctions::sum(rig.reals)", "0.0"},
		{"product(rig.masses)", "1"},
		{"rig.masses->product()", "1"},
		{"size(rig.masses)", "0"},
		{"rig.masses->size()", "0"},
		{"NumericalFunctions::sum0(rig.masses, 0 [g])", "0 [g]"},
		{"NumericalFunctions::product1(rig.masses, 1 [kg])", "1 [kg]"},
		{"rig.masses->select{in m; m > 1 [kg]}->sum()", "0 [kg]"},
		{"rig.masses->reject{in m; m > 1 [kg]}->sum()", "0 [kg]"},
		{"rig.mass + sum(rig.masses)", "10 [kg]"},
		{"sum(rig.masses) + rig.mass", "10 [kg]"},
		{"rig.mass - sum(rig.masses)", "10 [kg]"},
		{"3 [kg] + rig.masses->sum()", "3 [kg]"},
		// The zero is in the coherent unit, so a sum in another unit converts it
		// as it would an explicit `0 [kg]`, to a real magnitude.
		{"500 [g] + sum(rig.masses)", "500.0 [g]"},
		{"rig.total", "10 [kg]"},
		{"rig.grams", "500.0 [g]"},
		{"rig.area", "0 [SI::'m²']"},
		{"rig.mapped", "0 [kg]"},
		{"rig.mappedTotal", "10 [kg]"},
		{"rig.inverted", "10 [kg]"},
		{"sum(rig.lengths) == 0 [mm]", "true"},
		{"isZero(sum(rig.lengths))", "true"},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			got, err := evalIn(t, ctx, scope, tc.src)
			if err != nil {
				t.Fatalf("%s: %v", tc.src, err)
			}
			if rendered := FormatValue(got); rendered != tc.want {
				t.Errorf("%s = %s, want %s", tc.src, rendered, tc.want)
			}
		})
	}
}

// TestEmptiedQuantityCollectionKeepsItsKind: a collection operation that leaves
// none of a nonempty quantity collection's elements, or maps each to nothing,
// keeps their kind, so its sum is still a zero of that kind.
func TestEmptiedQuantityCollectionKeepsItsKind(t *testing.T) {
	ctx, scope := emptyAggregateContext(t)

	cases := []struct{ src, want string }{
		{"rig.heavy->select{in m; m > 5 [kg]}->sum()", "0 [kg]"},
		{"rig.heavy->reject{in m; m < 5 [kg]}->sum()", "0 [kg]"},
		{"rig.mass + rig.heavy->select{in m; m > 5 [kg]}->sum()", "10 [kg]"},
		{"sum(SequenceFunctions::excluding(rig.heavy, rig.heavy))", "0 [kg]"},
		{"sum(SequenceFunctions::intersection(rig.heavy, rig.masses))", "0 [kg]"},
		{"sum(SequenceFunctions::subsequence(rig.heavy, 3))", "0 [kg]"},
		{"sum(SequenceFunctions::excludingAt(rig.heavy, 1, 2))", "0 [kg]"},
		{"sum(SequenceFunctions::tail(SequenceFunctions::tail(rig.heavy)))", "0 [kg]"},
		{"sum(SequenceFunctions::union(rig.masses, rig.masses))", "0 [kg]"},
		{"sum(SequenceFunctions::including(rig.masses, rig.masses))", "0 [kg]"},
		{"size(rig.crew)", "2"},
		{"sum(rig.crew->collect{in p : Part; p.spares})", "0 [kg]"},
		{"sum(rig.crew->collect{in p; p.spares})", "0 [kg]"},
		{"rig.spares", "0 [kg]"},
		{"sum(rig.crew.spares)", "0 [kg]"},
		{"sum(rig.crew#(1).spares)", "0 [kg]"},
		{"rig.heavy->select{in m; m > 5 [kg]}->size()", "0"},
		{"product(rig.heavy->select{in m; m > 5 [kg]})", "1"},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			got, err := evalIn(t, ctx, scope, tc.src)
			if err != nil {
				t.Fatalf("%s: %v", tc.src, err)
			}
			if rendered := FormatValue(got); rendered != tc.want {
				t.Errorf("%s = %s, want %s", tc.src, rendered, tc.want)
			}
		})
	}
}

// TestAdoptKeepsTheKindOfAWrittenEmptyQuantityCollection: an empty quantity
// collection written to an object is carried into a re-analysis with its kind,
// so a sum over it there is still a zero of that kind.
func TestAdoptKeepsTheKindOfAWrittenEmptyQuantityCollection(t *testing.T) {
	prev, scope := emptyAggregateContext(t)
	prev.Model().RegisterSource(source.New("<test>", []byte(emptyAggregateModel)))
	obj, err := prev.Instantiate(lookupOne(t, prev.Resolver().Index(), "test::Rig"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	emptied, err := evalIn(t, prev, scope, "rig.heavy->select{in m; m > 5 [kg]}")
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if _, ok := emptied.Sequence().ElementUnit(); !ok {
		t.Fatalf("select yielded %s without an element unit", FormatValue(emptied))
	}
	if err := obj.SetFeatureValue(prev, "masses", emptied); err != nil {
		t.Fatalf("SetFeatureValue(masses): %v", err)
	}
	shapes := prev.ShapesOf(obj)

	ctx, scope := emptyAggregateContext(t)
	ctx.Model().RegisterSource(source.New("<test>", []byte(emptyAggregateModel)))
	if _, err := ctx.Adopt(prev, shapes, obj); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	fv, err := obj.GetFeatureValue(ctx, "masses")
	if err != nil {
		t.Fatalf("GetFeatureValue(masses): %v", err)
	}
	if !fv.Written {
		t.Fatal("the written collection was not carried")
	}
	if unit, ok := fv.Values.Sequence().ElementUnit(); !ok || unit.Text != "kg" {
		t.Fatalf("carried masses = %s with element unit %v, %v; want kg, true", FormatValue(fv.Values), unit, ok)
	}
	for _, tc := range []struct{ src, want string }{
		{"sum(masses)", "0 [kg]"},
		{"mass + sum(masses)", "10 [kg]"},
	} {
		p := parseExpr(t, tc.src)
		got, err := ctx.EvalWithScopeOn(p, scope, obj)
		if err != nil {
			t.Fatalf("%s: %v", tc.src, err)
		}
		if rendered := FormatValue(got); rendered != tc.want {
			t.Errorf("%s = %s, want %s", tc.src, rendered, tc.want)
		}
	}
}

// TestEmptyQuantitySumMagnitudeKind: the zero an empty sum yields is an integer
// magnitude, as sum's is, and a real one from the real-valued sum.
func TestEmptyQuantitySumMagnitudeKind(t *testing.T) {
	ctx, scope := emptyAggregateContext(t)
	for _, tc := range []struct {
		src  string
		kind semantics.ValueKind
	}{
		{"sum(rig.masses)", semantics.ValInt},
		{"RealFunctions::sum(rig.masses)", semantics.ValReal},
	} {
		got, err := evalIn(t, ctx, scope, tc.src)
		if err != nil {
			t.Fatalf("%s: %v", tc.src, err)
		}
		if got.Kind != ValQuantity {
			t.Fatalf("%s is a %v, want a quantity", tc.src, got.Kind)
		}
		if kind := got.Quantity().Num.Kind; kind != tc.kind {
			t.Errorf("%s has magnitude kind %v, want %v", tc.src, kind, tc.kind)
		}
	}
}

// TestEmptyAggregateIdentityIsNotLenientAddition: only the typed zero of an
// empty aggregate adds to a quantity of its kind; a number, a zero of another
// kind, and an empty sum of another kind or of numbers still do not.
func TestEmptyAggregateIdentityIsNotLenientAddition(t *testing.T) {
	ctx, scope := emptyAggregateContext(t)
	for _, src := range []string{
		"10 [kg] + 0 [m]",
		"10 [kg] + 5",
		"10 [kg] + 0",
		"10 [kg] + 0.0",
		"10 [kg] + sum(rig.lengths)",
		"sum(rig.lengths) + 10 [kg]",
		"10 [kg] + sum(rig.counts)",
		"10 [kg] + sum(rig.reals)",
		"10 [kg] + product(rig.masses)",
	} {
		t.Run(src, func(t *testing.T) {
			got, err := evalIn(t, ctx, scope, src)
			if err == nil {
				t.Fatalf("%s = %s, want an error", src, FormatValue(got))
			}
			if !errors.Is(err, ErrIncommensurableUnits) {
				t.Fatalf("%s: %v, want ErrIncommensurableUnits", src, err)
			}
		})
	}
}

// TestEmptySequenceKeepsItsElementUnitOnlyWhileEmpty: the unit an empty typed
// sequence carries describes elements it does not have, so it is gone once
// an element is appended.
func TestEmptySequenceKeepsItsElementUnitOnlyWhileEmpty(t *testing.T) {
	unit := Unit{Text: "kg"}
	val := NewEmptySequenceOf(unit)
	if got, ok := val.Sequence().ElementUnit(); !ok || got.Text != "kg" {
		t.Fatalf("ElementUnit() = %v, %v; want kg, true", got, ok)
	}
	if val.Sequence().Size() != 0 {
		t.Fatalf("Size() = %d, want 0", val.Sequence().Size())
	}
	val.Sequence().Append(Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}})
	if _, ok := val.Sequence().ElementUnit(); ok {
		t.Fatal("ElementUnit() reported a unit for a sequence with an element")
	}
	if _, ok := NewSequence().ElementUnit(); ok {
		t.Fatal("ElementUnit() reported a unit for an untyped sequence")
	}
}
