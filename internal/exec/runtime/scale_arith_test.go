package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
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
			if !errors.Is(err, errScalePoint) {
				t.Errorf("%s: err = %v, want errScalePoint", tc.src, err)
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
		if !errors.Is(err, errScalePoint) {
			t.Errorf("%s: err = %v, want errScalePoint", src, err)
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

// TestUniquenessJudgedInContext: a unique sequence-held feature refuses a point
// beside the magnitude it equals on another scale, as a set in the runtime would drop it.
func TestUniquenessJudgedInContext(t *testing.T) {
	ctx, idx := libraryModelContext(t, `package test {
		private import SI::*;
		private import ISQ::*;
		attribute repeated : TemperatureValue[*] ordered = (293.15 [K], 20.0 [SI::'°C_abs']);
		attribute distinct : TemperatureValue[*] ordered = (293.15 [K], 0.0 [SI::'°C_abs']);
	}`)
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok {
		t.Fatal("package test not found")
	}
	if _, err := evalIn(t, ctx, pkg.Scope, "repeated"); !errors.Is(err, ErrUniquenessViolation) {
		t.Errorf("repeated = %v, want ErrUniquenessViolation: 20.0 °C_abs is the point 293.15 K", err)
	}
	if val, err := evalIn(t, ctx, pkg.Scope, "distinct"); err != nil || len(elementsOf(val)) != 2 {
		t.Errorf("distinct = %s, %v; want two points", FormatValue(val), err)
	}
}

// TestAnchoredOrdinalPointsInSets: a set keeps an ordinal point apart from the magnitude
// an affine reading of its mapping would carry it to, as `==` and ConvertQuantity do.
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
	if ratio, err := ctx.toRatioReference(*evalQuantity(t, ctx, scope, "7 [mohs]")); !errors.Is(err, errScalePoint) {
		t.Fatalf("7 [mohs] carried to %v, %v; want errScalePoint: one mapped value gives an ordinal scale no interval", ratio, err)
	}
	if _, err := evalIn(t, ctx, scope, "7 [mohs] == 279.16 [K]"); !errors.Is(err, errScalePoint) {
		t.Errorf("7 [mohs] == 279.16 [K]: err = %v, want errScalePoint", err)
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
	if _, err := ctx.addQuantities(ast.OpSub, kelvin, warm); !errors.Is(err, errScalePoint) {
		t.Errorf("10 K - warm: err = %v, want errScalePoint", err)
	}
	if got, err := ctx.compareQuantities(ast.OpLt, quantity("300.0 [K]"), warm); err != nil || got.Const.Bool {
		t.Errorf("300 K < warm = %v, %v; want false", got, err)
	}
	if got, err := ctx.equalQuantities(ast.OpEq, quantity("300.0 [K]"), warm); err != nil || !got.Const.Bool {
		t.Errorf("300 K == warm = %v, %v; want true", got, err)
	}
	if _, err := ctx.negateQuantity(warm); !errors.Is(err, errScalePoint) {
		t.Errorf("-warm: err = %v, want errScalePoint", err)
	}
}

// scaleKindsModel declares one scale of each kind that is no interval scale.
const scaleKindsModel = `package test {
	public import SI::*;
	private import Collections::*;
	private import SequenceFunctions::*;
	private import MeasurementReferences::*;
	private import QuantityCalculations::*;
	attribute bearing : CyclicRatioScale { :>> unit = '°'; :>> modulus = 360; }
	attribute course : CyclicRatioScale { :>> unit = '°'; :>> modulus = 360.0; }
	attribute dBm : LogarithmicScale { :>> unit = one; :>> logarithmBase = 10; :>> factor = 10; :>> exponent = 1; :>> referenceQuantity = 0.001 [W]; }
	attribute field : LogarithmicScale { :>> unit = one; :>> logarithmBase = 10; :>> factor = 10; :>> exponent = 2; :>> referenceQuantity = 1.0 [V]; }
	attribute acidity : LogarithmicScale { :>> unit = one; :>> logarithmBase = 10; :>> factor = -1; :>> exponent = 1; :>> referenceQuantity = 1.0 [mol/L]; }
	attribute dayClock : CyclicRatioScale { :>> unit = s; :>> modulus = 6.283185307179586; }
	attribute halfTurn : CyclicRatioScale { :>> unit = '°'; :>> modulus = 180; }
	attribute bearings [2] = (10 [bearing], 20 [bearing]);
	attribute headings : Set { :>> elements = (450 [bearing], 90 [bearing], -270 [bearing], 91 [bearing], 360.1 [bearing], 0.1 [bearing], 90 ['°']); }
	attribute levels : Set { :>> elements = (30.0 [dBm], 1.0 [W], 20.0 [dBm]); }
}`

// TestCyclicRatioScaleValues: a value on a cyclic ratio scale is a magnitude in the
// scale's unit, equivalent to every value a whole number of cycles away.
func TestCyclicRatioScaleValues(t *testing.T) {
	ctx, idx := libraryModelContext(t, scaleKindsModel)
	pkg, _ := idx.DocumentRoot("<test>").LookupLocal("test")
	scope := pkg.Scope
	for src, want := range map[string]string{
		"450 [bearing] + 30 ['°']":             "480 [bearing]",
		"30 ['°'] + 450 [bearing]":             "480 ['°']",
		"450 [bearing] - 100 [bearing]":        "350 [bearing]",
		"-(450 [bearing])":                     "-450 [bearing]",
		"3 * 450 [bearing]":                    "1350 [bearing]",
		"450 [bearing] / 2":                    "225.0 [bearing]",
		"450 [bearing] / 9 [s]":                "50.0 ['°'/s]",
		"ConvertQuantity(450 [bearing], '°')":  "450 ['°']",
		"ConvertQuantity(90.0 ['°'], bearing)": "90.0 [bearing]",
		"max(10 [bearing], 20 [bearing])":      "20 [bearing]",
		"sum((450 [bearing], 30 [bearing]))":   "480 [bearing]",
	} {
		if got, err := evalIn(t, ctx, scope, src); err != nil || FormatValue(got) != want {
			t.Errorf("%s = %s, %v; want %s", src, FormatValue(got), err, want)
		}
	}
	for src, want := range map[string]bool{
		"450 [bearing] == 90 [bearing]":                        true,
		"-60 [bearing] == 300 [bearing]":                       true,
		"450 [bearing] != 90 [bearing]":                        false,
		"450 [bearing] > 90 [bearing]":                         false,
		"450 [bearing] >= 90 [bearing]":                        true,
		"451 [bearing] > 90 [bearing]":                         true,
		"-60 [bearing] < 10 [bearing]":                         true,
		"450 [bearing] == 90 ['°']":                            true,
		"90 ['°'] == 450 [bearing]":                            true,
		"360.1 [bearing] == 0.1 [bearing]":                     true,
		"720.25 [bearing] == 0.25 ['°']":                       true,
		"360.1 [bearing] > 0.1 [bearing]":                      false,
		"headings.elements->includes(720.1 [bearing])":         true,
		"headings.elements->includes(450 ['°'])":               false,
		"450 [bearing] == 0":                                   false,
		"720 [bearing] == 0":                                   true,
		"0 == 720 [bearing]":                                   true,
		"90 [course] == 450 [bearing]":                         true,
		"450 [bearing] <= ConvertQuantity(90.0 ['°'], course)": true,
		"headings.elements->includes(810 [bearing])":           true,
		"headings.elements->includes(92 [bearing])":            false,
	} {
		got, err := evalIn(t, ctx, scope, src)
		if err != nil || got.Kind != ValConst || got.Const.Bool != want {
			t.Errorf("%s = %v, %v; want %v", src, got, err, want)
		}
	}
	// Distinct values keep distinct keys, so a collection subsetting them keeps both.
	if got, err := evalIn(t, ctx, scope, "bearings"); err != nil || len(elementsOf(got)) != 2 {
		t.Errorf("bearings = %s, %v; want two distinct bearings", FormatValue(got), err)
	}
	// Two scales of one cycle hold one member; two of one period in other dimensions do not.
	for src, want := range map[string]bool{
		"(450 [bearing], 1 [m])->includes(90 [course])":                   true,
		"(450 [bearing], 1 [m])->includes(90 [dayClock])":                 false,
		"(450 [bearing], 90 [bearing]) == (90 [course], 90 [course])":     true,
		"(450 [bearing], 90 [bearing]) == (90 [dayClock], 90 [dayClock])": false,
	} {
		if got, err := evalIn(t, ctx, scope, src); err != nil || got.Const.Bool != want {
			t.Errorf("%s = %v, %v; want %v", src, got, err, want)
		}
	}
	if valueKeyOf(t, ctx, scope, "450 [bearing]") != valueKeyOf(t, ctx, scope, "90 [course]") {
		t.Error("equivalent values on two scales of one cycle have two keys")
	}
	if valueKeyOf(t, ctx, scope, "0 [bearing]") == valueKeyOf(t, ctx, scope, "0 [halfTurn]") {
		t.Error("zero on two cycles shares a key; they are different members")
	}
	if got, err := evalIn(t, ctx, scope, "headings.elements"); err != nil || len(got.Set().Elements()) != 4 {
		t.Errorf("headings = %s, %v; want one bearing per cycle class, apart from the angle in a unit", FormatValue(got), err)
	}
}

// TestLogarithmicScaleValues: a value on a logarithmic scale converts and compares through
// `factor * log_base((x / referenceQuantity) ** exponent)`, and has no interval or multiple.
func TestLogarithmicScaleValues(t *testing.T) {
	ctx, idx := libraryModelContext(t, scaleKindsModel)
	pkg, _ := idx.DocumentRoot("<test>").LookupLocal("test")
	scope := pkg.Scope
	for src, want := range map[string]string{
		"ConvertQuantity(30.0 [dBm], W)":   "1.0 [W]",
		"ConvertQuantity(0.1 [W], dBm)":    "20.0 [dBm]",
		"ConvertQuantity(10.0 [V], field)": "20.0 [field]",
		"ConvertQuantity(40.0 [field], V)": "100.0 [V]",
		"max(20.0 [dBm], 2.0 [W])":         "2.0 [W]",
	} {
		if got, err := evalIn(t, ctx, scope, src); err != nil || FormatValue(got) != want {
			t.Errorf("%s = %s, %v; want %s", src, FormatValue(got), err, want)
		}
	}
	for src, want := range map[string]bool{
		"30.0 [dBm] == 1.0 [W]":   true,
		"1.0 [W] == 30.0 [dBm]":   true,
		"20.0 [dBm] < 1.0 [W]":    true,
		"1.0 [W] > 20.0 [dBm]":    true,
		"20.0 [dBm] < 30.0 [dBm]": true,
		// A decreasing scale orders as its values do where it leads, as the quantities where a unit does.
		"3.0 [acidity] < 7.0 [acidity]":                         true,
		"3.0 [acidity] < ConvertQuantity(7.0 [acidity], mol/L)": true,
		"ConvertQuantity(3.0 [acidity], mol/L) < 7.0 [acidity]": false,
		"levels.elements->includes(0.1 [W])":                    true,
		"levels.elements->includes(10.0 [dBm])":                 false,
	} {
		got, err := evalIn(t, ctx, scope, src)
		if err != nil || got.Kind != ValConst || got.Const.Bool != want {
			t.Errorf("%s = %v, %v; want %v", src, got, err, want)
		}
	}
	if got, err := evalIn(t, ctx, scope, "levels.elements"); err != nil || len(got.Set().Elements()) != 2 {
		t.Errorf("levels = %s, %v; want 30 dBm and 1 W as one member", FormatValue(got), err)
	}
	// An even exponent would give a negative quantity a value no conversion returns it from.
	for _, src := range []string{"40.0 [field] == -100.0 [V]", "-100.0 [V] == 40.0 [field]"} {
		if got, err := evalIn(t, ctx, scope, src); !errors.Is(err, semantics.ErrArithmeticDomain) && (err != nil || got.Const.Bool) {
			t.Errorf("%s = %v, %v; want false or a domain error, the same either way round", src, got, err)
		}
	}
	for _, src := range []string{"30.0 [dBm] - 20.0 [dBm]", "30.0 [dBm] + 1.0 [W]", "2 * 30.0 [dBm]", "-(30.0 [dBm])", "sum((30.0 [dBm], 20.0 [dBm]))"} {
		if _, err := evalIn(t, ctx, scope, src); !errors.Is(err, errScalePoint) {
			t.Errorf("%s: err = %v, want errScalePoint", src, err)
		}
	}
}

// TestCyclicRatioScaleZero: a cyclic ratio scale's zero is its unit's, so a
// quantityValueMapping placing it elsewhere is refused rather than read as an offset.
func TestCyclicRatioScaleZero(t *testing.T) {
	ctx, idx := libraryModelContext(t, `package test {
		public import SI::*;
		private import MeasurementReferences::*;
		private import QuantityCalculations::*;
		attribute shifted : CyclicRatioScale {
			:>> unit = K; :>> modulus = 100;
			private attribute zero : DefinitionalQuantityValue { :>> num = 0; :>> definition = "zero"; }
			private attribute zeroInKelvin : QuantityValueMapping {
				:>> mappedQuantityValue = zero;
				:>> referenceQuantityValue = K.temperatureOfWaterAtTriplePointInK;
			}
			attribute :>> definitionalQuantityValues = (zero);
			attribute :>> quantityValueMapping = zeroInKelvin;
		}
	}`)
	pkg, _ := idx.DocumentRoot("<test>").LookupLocal("test")
	for _, src := range []string{"ConvertQuantity(3 [shifted], K)", "2 * 3 [shifted]", "3 [shifted] == 3 [K]"} {
		if _, err := evalIn(t, ctx, pkg.Scope, src); !errors.Is(err, errUnevaluableLibraryFunction) || !strings.Contains(err.Error(), "a CyclicRatioScale is a ratio scale") {
			t.Errorf("%s: err = %v, want the mapping refused", src, err)
		}
	}
}

// valueKeyOf is the key a collection tells the value of src apart by.
func valueKeyOf(t *testing.T, ctx *Context, scope *symbols.Scope, src string) any {
	t.Helper()
	v, err := evalIn(t, ctx, scope, src)
	if err != nil {
		t.Fatalf("%s: %v", src, err)
	}
	return ctx.valueKey(v)
}
