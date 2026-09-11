package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// scaleContext is a runtime over the standard library whose scope imports the
// quantity calculations and the collection functions beside SI.
func scaleContext(t *testing.T) (*Context, *symbols.Scope) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			public import SI::*;
			private import QuantityCalculations::*;
			private import CollectionFunctions::*;
			private import SequenceFunctions::*;
		}
	`))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	return ctx, pkg.Scope
}

// TestPointArithmetic: a value on an interval scale is a point. A point moved by
// a magnitude commensurable with the scale's unit is a point on the same scale;
// two points differ by a magnitude in the left scale's unit, the right point
// converted onto the left scale first, across scales and through a scale placed
// on another scale.
func TestPointArithmetic(t *testing.T) {
	cases := []struct {
		src  string
		want string
	}{
		{"26.85 [SI::'°C_abs'] + 10.0 [SI::'°C']", "36.85 ['°C_abs']"},
		{"10.0 [SI::'°C'] + 26.85 [SI::'°C_abs']", "36.85 ['°C_abs']"},
		{"26.85 [SI::'°C_abs'] + 10.0 [K]", "36.85 ['°C_abs']"},
		{"26.85 [SI::'°C_abs'] - 10.0 [K]", "16.85 ['°C_abs']"},
		{"26.85 [SI::'°C_abs'] - 10.0 [SI::'°C_abs']", "16.85 ['°C']"},
		{"20.0 [SI::'°C_abs'] - 283.15 [K]", "-263.15 ['°C_abs']"},
		{"ConvertQuantity(293.15 [K], SI::'°C_abs') + 10.0 [SI::'°C']", "30.0 ['°C_abs']"},
		{"ConvertQuantity(20.0 [SI::'°C_abs'], K) - 273.15 [K]", "20.0 [K]"},
		{"5.0 [Time::UTC] + 3.0 [s]", "8.0 [UTC]"},
		{"5.0 [Time::UTC] - 3.0 [s]", "2.0 [UTC]"},
		{"5.0 [Time::UTC] - 3.0 [Time::UTC]", "2.0 [s]"},
		{"5.0 [Time::UTC] + 3.0 [min]", "185.0 [UTC]"},
	}
	ctx, scope := scaleContext(t)
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			got, err := evalIn(t, ctx, scope, tc.src)
			if err != nil {
				t.Fatalf("%s: %v", tc.src, err)
			}
			if got.Kind != ValQuantity {
				t.Fatalf("%s = %v (%s), want a quantity", tc.src, got, got.Kind)
			}
			if s := got.Quantity().String(); s != tc.want {
				t.Errorf("%s = %s, want %s", tc.src, s, tc.want)
			}
		})
	}
}

// TestPointComparison: a point compares with a magnitude, or with a point on
// another scale, by converting the right operand onto the left reference —
// equality agrees with ConvertQuantity about what is the same temperature.
func TestPointComparison(t *testing.T) {
	cases := []struct {
		src  string
		want bool
	}{
		{"300.0 [K] == ConvertQuantity(300.0 [K], SI::'°C_abs')", true},
		{"ConvertQuantity(300.0 [K], SI::'°C_abs') == 300.0 [K]", true},
		{"300.0 [K] != 26.85 [SI::'°C_abs']", false},
		{"300.0 [K] < 30.0 [SI::'°C_abs']", true},
		{"30.0 [SI::'°C_abs'] > 300.0 [K]", true},
		{"30.0 [SI::'°C_abs'] <= 300.0 [K]", false},
		{"20.0 [SI::'°C_abs'] >= 293.15 [K]", true},
		{"20.0 [SI::'°C_abs'] == 293.15 [K]", true},
		{"0.0 [SI::'°C_abs'] == 273.15 [K]", true},
		{"26.85 [SI::'°C_abs'] > 20.0 [SI::'°C_abs']", true},
		{"5.0 [Time::UTC] < 8.0 [Time::UTC]", true},
		{"5.0 [Time::UTC] == 5.0 [Time::UTC]", true},
	}
	ctx, scope := scaleContext(t)
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			got, err := evalIn(t, ctx, scope, tc.src)
			if err != nil {
				t.Fatalf("%s: %v", tc.src, err)
			}
			if got.Kind != ValConst || got.Const.Kind != semantics.ValBool {
				t.Fatalf("%s = %v, want a boolean", tc.src, got)
			}
			if got.Const.Bool != tc.want {
				t.Errorf("%s = %v, want %v", tc.src, got.Const.Bool, tc.want)
			}
		})
	}
}

// TestPointRefusals: what a point has no meaning for — a sum of points, a point
// taken from a magnitude, a multiple, a quotient, a power, a root, a negative —
// is a typed error naming the scale and the operation, never a silent result.
func TestPointRefusals(t *testing.T) {
	cases := []struct {
		src  string
		want string
	}{
		{"26.85 [SI::'°C_abs'] + 10.0 [SI::'°C_abs']", "operator '+' on a point of the interval scale SI::'°C_abs'"},
		{"300.0 [K] - 26.85 [SI::'°C_abs']", "subtracting a point from a magnitude on a point of the interval scale SI::'°C_abs'"},
		{"2 * 26.85 [SI::'°C_abs']", "operator '*' on a point of the interval scale SI::'°C_abs'"},
		{"26.85 [SI::'°C_abs'] * 2", "operator '*' on a point of the interval scale SI::'°C_abs'"},
		{"26.85 [SI::'°C_abs'] * 2.0 [s]", "operator '*' on a point of the interval scale SI::'°C_abs'"},
		{"26.85 [SI::'°C_abs'] / 2", "operator '/' on a point of the interval scale SI::'°C_abs'"},
		{"2.0 [s] / 26.85 [SI::'°C_abs']", "operator '/' on a point of the interval scale SI::'°C_abs'"},
		{"26.85 [SI::'°C_abs'] ** 2", "operator '**' on a point of the interval scale SI::'°C_abs'"},
		{"QuantityCalculations::sqrt(26.85 [SI::'°C_abs'])", "sqrt on a point of the interval scale SI::'°C_abs'"},
		{"-(26.85 [SI::'°C_abs'])", "negation on a point of the interval scale SI::'°C_abs'"},
		{"5.0 [Time::UTC] + 3.0 [Time::UTC]", "operator '+' on a point of the interval scale Time::UTC"},
		{"2 * 5.0 [Time::UTC]", "operator '*' on a point of the interval scale Time::UTC"},
		{"-(5.0 [Time::UTC])", "negation on a point of the interval scale Time::UTC"},
	}
	ctx, scope := scaleContext(t)
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			got, err := evalIn(t, ctx, scope, tc.src)
			if err == nil {
				t.Fatalf("%s = %v, want an error", tc.src, got)
			}
			if !errors.Is(err, ErrScalePoint) {
				t.Errorf("%s: err = %v, want ErrScalePoint", tc.src, err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("%s: err = %q, want it to contain %q", tc.src, err, tc.want)
			}
		})
	}
}

// TestPointIncommensurable: a point moved by, or compared with, a magnitude of
// another dimension is the incommensurable-units error the units path reports.
func TestPointIncommensurable(t *testing.T) {
	ctx, scope := scaleContext(t)
	for _, src := range []string{
		"26.85 [SI::'°C_abs'] + 1.0 [m]",
		"1.0 [m] + 26.85 [SI::'°C_abs']",
		"26.85 [SI::'°C_abs'] - 1.0 [m]",
		"26.85 [SI::'°C_abs'] < 1.0 [m]",
		"26.85 [SI::'°C_abs'] == 1.0 [m]",
		"5.0 [Time::UTC] - 26.85 [SI::'°C_abs']",
		"5.0 [Time::UTC] == 26.85 [SI::'°C_abs']",
	} {
		if _, err := evalIn(t, ctx, scope, src); !errors.Is(err, semantics.ErrIncommensurableUnits) {
			t.Errorf("%s: err = %v, want ErrIncommensurableUnits", src, err)
		}
	}
}

// TestPointCollections: min and max over points are comparisons and answer a
// point; a sum or a product over points is refused, while a sum of differences
// between points is a magnitude.
func TestPointCollections(t *testing.T) {
	ctx, scope := scaleContext(t)
	for src, want := range map[string]string{
		"max(26.85 [SI::'°C_abs'], 300.5 [K])":                           "300.5 [K]",
		"min(26.85 [SI::'°C_abs'], 300.5 [K])":                           "26.85 ['°C_abs']",
		"max(26.85 [SI::'°C_abs'], 20.0 [SI::'°C_abs'])":                 "26.85 ['°C_abs']",
		"max(max(26.85 [SI::'°C_abs'], 20.0 [SI::'°C_abs']), 305.0 [K])": "305.0 [K]",
		"sum((30.0 [SI::'°C_abs'] - 20.0 [SI::'°C_abs'], 1.0 [K]))":      "11.0 ['°C']",
		"sum((5.0 [Time::UTC] - 3.0 [Time::UTC], 1.0 [s]))":              "3.0 [s]",
	} {
		got, err := evalIn(t, ctx, scope, src)
		if err != nil {
			t.Errorf("%s: %v", src, err)
			continue
		}
		if got.Kind != ValQuantity {
			t.Errorf("%s = %v (%s), want a quantity", src, got, got.Kind)
		} else if s := got.Quantity().String(); s != want {
			t.Errorf("%s = %s, want %s", src, s, want)
		}
	}
	for _, src := range []string{
		"sum((26.85 [SI::'°C_abs'], 20.0 [SI::'°C_abs']))",
		"sum((26.85 [SI::'°C_abs'], 1.0 [K]))",
		"product((26.85 [SI::'°C_abs'], 20.0 [SI::'°C_abs']))",
		"sum((5.0 [Time::UTC], 3.0 [Time::UTC]))",
	} {
		_, err := evalIn(t, ctx, scope, src)
		if !errors.Is(err, ErrScalePoint) {
			t.Errorf("%s: err = %v, want ErrScalePoint", src, err)
		}
	}
}

// TestPointsInSets: a set deduplicates a point and the magnitude it equals, and
// orders points among magnitudes by their common reference.
func TestPointsInSets(t *testing.T) {
	ctx, idx := libraryModelContext(t, `package test {
		public import SI::*;
		private import Collections::*;
		private import SequenceFunctions::*;
		attribute temperatures : Set { :>> elements = (300.0 [K], 20.0 [SI::'°C_abs'], 0.0 [SI::'°C_abs'], 273.15 [K], 293.15 [K]); }
	}`)
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok {
		t.Fatal("package test not found")
	}
	scope := pkg.Scope
	got, err := evalIn(t, ctx, scope, "temperatures.elements")
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != ValSet {
		t.Fatalf("got %v (%s), want a set", got, got.Kind)
	}
	elements := got.Set().Elements()
	if len(elements) != 3 {
		t.Fatalf("set has %d elements, want 3: %v", len(elements), FormatTraceValue(got))
	}
	for i := 1; i < len(elements); i++ {
		colder, warmer := ctx.canonicalQuantity(*elements[i-1].Quantity()), ctx.canonicalQuantity(*elements[i].Quantity())
		if colder.BaseMagnitude() >= warmer.BaseMagnitude() {
			t.Errorf("set orders %s before %s, want the colder first", elements[i-1].Quantity(), elements[i].Quantity())
		}
	}
	for _, src := range []string{
		"(293.15 [K], 0.0 [SI::'°C_abs'])->includes(20.0 [SI::'°C_abs'])",
		"(300.0 [K], 0.0 [SI::'°C_abs'])->includes(273.15 [K])",
		"(300.0 [K], 20.0 [SI::'°C_abs'])->excludes(200.0 [K])",
	} {
		got, err := evalIn(t, ctx, scope, src)
		if err != nil || got.Kind != ValConst || !got.Const.Bool {
			t.Errorf("%s = %v, %v; want true", src, got, err)
		}
	}
}

// TestSetsJudgedAcrossContexts: a set built with no context may hold a point and
// the magnitude it equals as two members; judged in a runtime they are one, so
// it equals the runtime's set of that one member, and neither equals a set of two.
func TestSetsJudgedAcrossContexts(t *testing.T) {
	ctx, scope := scaleContext(t)
	kelvin := evalQuantity(t, ctx, scope, "293.15 [K]")
	celsius := evalQuantity(t, ctx, scope, "20.0 [SI::'°C_abs']")
	cold := evalQuantity(t, ctx, scope, "0.0 [SI::'°C_abs']")
	both := NewSet()
	both.Add(NewQuantityValue(kelvin))
	both.Add(NewQuantityValue(celsius))
	if both.Size() != 2 {
		t.Fatalf("with no context the set holds %d members, want 2", both.Size())
	}
	one := ctx.setOf([]Value{NewQuantityValue(celsius)}).Set()
	if one.Size() != 1 {
		t.Fatalf("in the runtime the set holds %d members, want 1", one.Size())
	}
	if !ctx.setsEqual(one, both) || !ctx.setsEqual(both, one) || !one.Equal(both) {
		t.Errorf("{20.0 °C_abs} judged in the runtime != {293.15 K, 20.0 °C_abs} built with none")
	}
	if !ctx.valueEqual(NewSetValue(one), NewSetValue(both)) || !ctx.valueEqual(NewSetValue(both), NewSetValue(one)) {
		t.Errorf("the set values compare unequal in the runtime")
	}
	two := ctx.setOf([]Value{NewQuantityValue(kelvin), NewQuantityValue(cold)}).Set()
	if ctx.setsEqual(two, both) || ctx.setsEqual(both, two) {
		t.Errorf("{293.15 K, 0.0 °C_abs} judged in the runtime == {293.15 K, 20.0 °C_abs} built with none")
	}
	if both.Equal(one) {
		t.Errorf("{293.15 K, 20.0 °C_abs} judged with no context == {20.0 °C_abs}")
	}
}

// TestAnchoredOrdinalPointsInSets: a set keeps an ordinal point apart from the
// magnitude its mapping would carry it to, as `==` does.
func TestAnchoredOrdinalPointsInSets(t *testing.T) {
	ctx, idx := libraryModelContext(t, `package test {
		public import SI::*;
		private import Collections::*;
		private import SequenceFunctions::*;
		private import MeasurementReferences::*;
		attribute mohs : OrdinalScale {
			:>> unit = SI::K;
			private attribute talc : DefinitionalQuantityValue { :>> num = 1; :>> definition = "talc"; }
			private attribute talcInKelvin : QuantityValueMapping {
				:>> mappedQuantityValue = talc;
				:>> referenceQuantityValue = K.temperatureOfWaterAtTriplePointInK;
			}
			attribute :>> definitionalQuantityValues = (talc);
			attribute :>> quantityValueMapping = talcInKelvin;
		}
		attribute hardnesses : Set { :>> elements = (7 [mohs], 279.16 [K], 7 [mohs]); }
	}`)
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok {
		t.Fatal("package test not found")
	}
	scope := pkg.Scope
	if ratio, err := ctx.toRatioReference(*evalQuantity(t, ctx, scope, "7 [mohs]")); err != nil || ratio.String() != "279.16 [K]" {
		t.Fatalf("the mapping does not anchor mohs on K: %v, %v", ratio, err)
	}
	if _, err := evalIn(t, ctx, scope, "7 [mohs] == 279.16 [K]"); !errors.Is(err, ErrScalePoint) {
		t.Errorf("7 [mohs] == 279.16 [K]: err = %v, want ErrScalePoint", err)
	}
	got, err := evalIn(t, ctx, scope, "hardnesses.elements")
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != ValSet || len(got.Set().Elements()) != 2 {
		t.Fatalf("set is %s, want the ordinal point and the kelvin magnitude apart", FormatTraceValue(got))
	}
	for src, want := range map[string]bool{
		"(7 [mohs], 1 [mohs])->includes(279.16 [K])": false,
		"(7 [mohs], 1 [mohs])->includes(7 [mohs])":   true,
		"(279.16 [K], 1 [mohs])->includes(7 [mohs])": false,
	} {
		got, err := evalIn(t, ctx, scope, src)
		if err != nil || got.Kind != ValConst || got.Const.Bool != want {
			t.Errorf("%s = %v, %v; want %v", src, got, err, want)
		}
	}
}

// evalQuantity evaluates src in scope and requires a quantity.
func evalQuantity(t *testing.T, ctx *Context, scope *symbols.Scope, src string) *Quantity {
	t.Helper()
	v, err := evalIn(t, ctx, scope, src)
	if err != nil || v.Kind != ValQuantity {
		t.Fatalf("%s = %v, %v; want a quantity", src, v, err)
	}
	return v.Quantity()
}

// TestPointsInQuantityOperators exercises the operator table directly, so a
// caller of the operators with an operator kind gets the same verdicts as the
// expression path.
func TestPointsInQuantityOperators(t *testing.T) {
	ctx, scope := scaleContext(t)
	quantity := func(src string) *Quantity {
		v, err := evalIn(t, ctx, scope, src)
		if err != nil || v.Kind != ValQuantity {
			t.Fatalf("%s = %v, %v; want a quantity", src, v, err)
		}
		return v.Quantity()
	}
	warm, kelvin := quantity("26.85 [SI::'°C_abs']"), quantity("10.0 [K]")
	if got, err := ctx.addQuantities(ast.OpAdd, warm, kelvin); err != nil || got.Quantity().String() != "36.85 ['°C_abs']" {
		t.Errorf("warm + 10 K = %s, %v", FormatTraceValue(got), err)
	}
	if got, err := ctx.addQuantities(ast.OpSub, quantity("30.0 [SI::'°C_abs']"), quantity("20.0 [SI::'°C_abs']")); err != nil || got.Quantity().String() != "10.0 ['°C']" {
		t.Errorf("30 °C_abs - 20 °C_abs = %s, %v", FormatTraceValue(got), err)
	}
	if _, err := ctx.addQuantities(ast.OpSub, kelvin, warm); !errors.Is(err, ErrScalePoint) {
		t.Errorf("10 K - warm: err = %v, want ErrScalePoint", err)
	}
	if got, err := ctx.compareQuantities(ast.OpLt, quantity("300.0 [K]"), warm); err != nil || got.Const.Bool {
		t.Errorf("300 K < warm = %v, %v; want false", got, err)
	}
	if got, err := ctx.equalQuantities(ast.OpEq, quantity("300.0 [K]"), warm); err != nil || !got.Const.Bool {
		t.Errorf("300 K == warm = %v, %v; want true", got, err)
	}
	if _, err := ctx.negateQuantity(warm); !errors.Is(err, ErrScalePoint) {
		t.Errorf("-warm: err = %v, want ErrScalePoint", err)
	}
}
