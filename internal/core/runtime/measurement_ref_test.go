package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// measurementRefContext evaluates expressions in a package that imports the
// units, the quantity calculations and the measurement-reference calculations.
func measurementRefContext(t *testing.T) (*Context, *symbols.Scope) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			public import ISQ::*;
			public import SI::*;
			public import ScalarValues::*;
			public import QuantityCalculations::*;
			public import MeasurementRefCalculations::*;
			private import MeasurementReferences::*;
			attribute halfMetre : LengthUnit { :>> unitConversion : ConversionByConvention { :>> referenceUnit = m; :>> conversionFactor = 0.5; } }
			attribute demiMetre : LengthUnit { :>> unitConversion : ConversionByConvention { :>> referenceUnit = m; :>> conversionFactor = 1/2; } }
			attribute side : LengthValue = 3 [km];
			attribute unit : LengthUnit = m;
			attribute area : AreaUnit = m * m;
			attribute speed : SpeedUnit = km / h;
			attribute wrongUnit : LengthUnit = s;
			attribute wrongDimension : AreaUnit = m * s;
			attribute notAUnit : LengthValue = m;
			attribute notAnArea : AreaValue = m * m;
			attribute notAScale : Time::TimeScale = h * s / min;
			attribute notAnInterval : IntervalScale = h * s / min;
			attribute aDuration : DurationUnit = h * s / min;
			attribute epoch : Time::TimeScale = Time::UTC;
			attribute exponent : Real = 2.0;
			attribute powered : AreaUnit = m ** exponent;
			attribute misPowered : LengthUnit = m ** exponent;
			attribute inferred = m * m;
			attribute inferredAgain = inferred;
			attribute celsius : IntervalScale = SI::'°C_abs';
			attribute vq : Quantities::VectorQuantityValue = VectorFunctions::VectorOf((1.0, 2.0, 3.0)) [m];
			attribute scaled = VectorFunctions::VectorOf((1.0, 2.0)) [m] * (2 [s]);
			package Imperial {
				attribute <m> mile : LengthUnit { :>> unitConversion : ConversionByConvention { :>> referenceUnit = SI::m; :>> conversionFactor = 1609.344; } }
			}
		}
	`))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	return ctx, pkg.Scope
}

// TestMeasurementRefValues: a unit declaration evaluates to the measurement
// reference it names, references compose as MeasurementRefCalculations declares,
// and a quantity is built from or converted to one.
func TestMeasurementRefValues(t *testing.T) {
	ctx, scope := measurementRefContext(t)

	cases := []struct {
		src  string
		want string
	}{
		{"m", "m"},
		{"SI::m", "m"},
		{"SI::km", "km"},
		{"SI::'m/s'", "'m/s'"},
		{"MeasurementReferences::one", "one"},
		{"unit", "m"},
		{"m * s", "m*s"},
		{"m / s", "m/s"},
		{"m ** 2", "m**2"},
		{"m ** 0.5", "m**0.5"},
		{"km / m", "km/m"},
		{"m / m", "1"},
		{"area", "m**2"},
		{"speed", "km/h"},
		{"aDuration", "h*s/min"},
		{"m ** exponent", "m**2"},
		{"powered", "m**2"},
		{"MeasurementRefCalculations::'**'(m, exponent)", "m**2"},
		{"ToString(m ** exponent)", `"m**2"`},
		{"ConvertQuantity(1 [m*m], m ** exponent)", "1 [m**2]"},
		{"inferred", "m**2"},
		{"ToString(inferred)", `"m**2"`},
		{"'['(2, inferred)", "2 [m**2]"},
		{"ConvertQuantity(1 [km * km], inferred)", "1000000.0 [m**2]"},
		{"ToString(inferredAgain)", `"m**2"`},
		{"MeasurementRefCalculations::'*'(m, s)", "m*s"},
		{"MeasurementRefCalculations::'/'(m, s)", "m/s"},
		{"MeasurementRefCalculations::'**'(m, 2)", "m**2"},
		{"MeasurementRefCalculations::'^'(m, 3)", "m**3"},
		{"MeasurementRefCalculations::ToString(km)", `"km"`},
		{"ToString(m / s)", `"m/s"`},
		{"ToString(SI::'m/s')", `"'m/s'"`},
		{"QuantityCalculations::'['(3, m)", "3 [m]"},
		{"'['(2.5, km)", "2.5 [km]"},
		{"'['(2, m / s)", "2 [m/s]"},
		{"ConvertQuantity(3 [km], m)", "3000.0 [m]"},
		{"ConvertQuantity(300 [cm], m)", "3.0 [m]"},
		{"ConvertQuantity(3 [m], cm)", "300.0 [cm]"},
		{"ConvertQuantity(3 [km], km)", "3 [km]"},
		{"ConvertQuantity(side, m)", "3000.0 [m]"},
		{"ConvertQuantity(2 [m/s], SI::'km/h')", "7.2 ['km/h']"},
		{"ConvertQuantity(1 [h], s)", "3600.0 [s]"},
		{"ConvertQuantity(1 [m*m], m ** 2)", "1 [m**2]"},
		{"side.num", "3"},
		{"side.mRef", "km"},
		{"(2.5 [m/s]).mRef", "m/s"},
		{"(2.5 [m/s]).num", "2.5"},
		{"side.rank", "0"},
		{"side.dimensions", "[]"},
		{"side.elements", "[3]"},
		{"side.order", "0"},
		{"vq.mRef", "m"},
		{"vq.num", "[1.0, 2.0, 3.0]"},
		{"scaled.mRef", "m*s"},
		{"scaled.mRef == m * s", "true"},
		{"m.rank", "0"},
		{"m.dimensions", "[]"},
		{"m.flattenedSize", "1"},
		{"m.elements", "[m]"},
		{"m.mRefs", "[m]"},
		{"m.isBound", "false"},
		{"m.order", "0"},
		{"m.isOrthogonal", "true"},
		{"side.mRef.isBound", "false"},
		{"Time::UTC", "UTC [s]"},
		{"epoch", "UTC [s]"},
		{"Time::UTC.unit", "s"},
		{"SI::'°C_abs'", "'°C_abs' ['°C']"},
		{"celsius", "'°C_abs' ['°C']"},
		{"celsius.unit", "'°C'"},
		{"'['(3, Time::UTC)", "3 [UTC]"},
		{"3 [Time::UTC]", "3 [UTC]"},
		{"(3 [Time::UTC]).mRef", "UTC [s]"},
		{"MeasurementRefCalculations::ToString(Time::UTC)", `"UTC"`},
		{"ConvertQuantity(273.15 [K], SI::'°C_abs')", "0.0 ['°C_abs']"},
		{"ConvertQuantity(0.0 ['°C_abs'], K)", "273.15 [K]"},
		{"ConvertQuantity(100.0 ['°C_abs'], K)", "373.15 [K]"},
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

// TestMeasurementRefDeclarationMembers: a reference naming a declaration answers
// that declaration's members from its materialized object, following the
// record's redefinitions (`conversionFactor = prefix.conversionFactor`) and
// defaults (`isExact default true`); a member the library states no value for
// is empty, as the pilot leaves it, and a member the value carries (isBound,
// mRefs) is answered by the value whether or not the declaration restates it.
func TestMeasurementRefDeclarationMembers(t *testing.T) {
	ctx, scope := measurementRefContext(t)

	cases := []struct{ src, want string }{
		{"km.unitConversion.conversionFactor", "1000.0"},
		{"km.unitConversion.referenceUnit", "m"},
		{"km.unitConversion.isExact", "true"},
		{"km.unitConversion.prefix.longName", `"kilo"`},
		{"km.unitConversion.prefix.conversionFactor", "1000.0"},
		{"side.mRef.unitConversion.conversionFactor", "1000.0"},
		{"ConvertQuantity(1 [km], km.unitConversion.referenceUnit)", "1000.0 [m]"},
		{"halfMetre.unitConversion.conversionFactor", "0.5"},
		{"demiMetre.unitConversion.conversionFactor", "0.5"},
		{"halfMetre.unitConversion.isExact", "true"},
		{"Imperial::m.unitConversion.conversionFactor", "1609.344"},
		{"m.unitConversion", "[]"},
		{"m.quantityDimension.quantityPowerFactors#(1).exponent", "1"},
		{"SI::'m/s'.quantityDimension.quantityPowerFactors#(2).exponent", "-1"},
		{"m.unitPowerFactors#(1).unit", "m"},
		{"m.unitPowerFactors#(1).exponent", "1"},
		{"km.unitPowerFactors#(1).unit", "km"},
		{"SI::'m/s'.unitPowerFactors", "[]"},
		{"K.definitionalQuantityValues#(1).num", "[273.16]"},
		{"K.definitionalQuantityValues#(1).definition", `"temperature in kelvin of pure water at the triple point"`},
		{"K.temperatureOfWaterAtTriplePointInK.num", "[273.16]"},
		{"m.definitionalQuantityValues", "[]"},
		{"SI::'m/s'.isBound", "false"},
		{"SI::'m/s'.mRefs", "['m/s']"},
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

	first, err := evalIn(t, ctx, scope, "km.unitConversion")
	if err != nil {
		t.Fatal(err)
	}
	again, err := evalIn(t, ctx, scope, "km.unitConversion")
	if err != nil {
		t.Fatal(err)
	}
	if first.Kind != ValInstance || first.Instance != again.Instance {
		t.Errorf("km.unitConversion = %s then %s, want one object", FormatValue(first), FormatValue(again))
	}
}

// TestMeasurementRefEquality: references are equal at one reduction and one
// scale, a ratio that cancels included; a named dimension-one unit is only
// itself; and a set keys them the same way.
func TestMeasurementRefEquality(t *testing.T) {
	ctx, scope := measurementRefContext(t)

	cases := []struct {
		left, right string
		want        bool
	}{
		{"m", "SI::m", true},
		{"m", "unit", true},
		{"side.mRef", "km", true},
		{"side.mRef", "m", false},
		{"km", "m", false},
		{"SI::'m/s'", "m / s", true},
		{"SI::'km/h'", "km / h", true},
		{"SI::'km/h'", "m / s", false},
		{"m ** 2", "m * m", true},
		{"area", "m ** 2", true},
		{"m / m", "s / s", true},
		{"km / m", "m / mm", true},
		{"km / m", "m / m", false},
		{"m / m", "MeasurementReferences::one", false},
		{"km / m", "MeasurementReferences::one", false},
		{"rad", "rad", true},
		{"rad", "sr", false},
		{"rad", "m / m", false},
		{"rad / rad", "m / m", true},
		{"m", "3", false},
		{"m", `"m"`, false},
		{"m", "1 [m]", false},
		{"vq.mRef", "m", true},
		{"(1 [m]).mRef", "(2 [m]).mRef", true},
		{"halfMetre", "demiMetre", true},
		{"halfMetre", "m", false},
		{"halfMetre / s", "demiMetre / s", true},
	}
	for _, tc := range cases {
		t.Run(tc.left+" == "+tc.right, func(t *testing.T) {
			left, err := evalIn(t, ctx, scope, tc.left)
			if err != nil {
				t.Fatalf("%s: %v", tc.left, err)
			}
			right, err := evalIn(t, ctx, scope, tc.right)
			if err != nil {
				t.Fatalf("%s: %v", tc.right, err)
			}
			if got := valueEqual(left, right); got != tc.want {
				t.Errorf("valueEqual(%s, %s) = %v, want %v", tc.left, tc.right, got, tc.want)
			}
			if got := valueEqual(right, left); got != tc.want {
				t.Errorf("valueEqual(%s, %s) = %v, want %v", tc.right, tc.left, got, tc.want)
			}
			if sameKey := valueKeyFunc(left) == valueKeyFunc(right); sameKey != tc.want {
				t.Errorf("valueKey(%s) == valueKey(%s) is %v, want %v", tc.left, tc.right, sameKey, tc.want)
			}
			got, err := evalIn(t, ctx, scope, tc.left+" == "+tc.right)
			if err != nil {
				t.Fatalf("%s == %s: %v", tc.left, tc.right, err)
			}
			if FormatValue(got) != FormatValue(boolValue(tc.want)) {
				t.Errorf("%s == %s = %s, want %v", tc.left, tc.right, FormatValue(got), tc.want)
			}
		})
	}
}

// TestMeasurementRefClassification: a reference is of its declaration's type; a
// composed unit is a DerivedUnit, and a unit definition fixing its dimension
// (AreaUnit's quantityDimension) classifies the composed unit of that dimension,
// as a feature of that type holds it.
func TestMeasurementRefClassification(t *testing.T) {
	ctx, scope := measurementRefContext(t)

	cases := []struct {
		src  string
		want string
	}{
		{"m istype LengthUnit", "true"},
		{"m istype DurationUnit", "false"},
		{"m istype MeasurementReferences::ScalarMeasurementReference", "true"},
		{"m istype MeasurementReferences::MeasurementUnit", "true"},
		{"m istype MeasurementReferences::TensorMeasurementReference", "true"},
		{"m hastype LengthUnit", "true"},
		{"m hastype MeasurementReferences::TensorMeasurementReference", "false"},
		{"side.mRef istype LengthUnit", "true"},
		{"(m * s) istype MeasurementReferences::MeasurementUnit", "true"},
		{"(m * s) istype MeasurementReferences::DerivedUnit", "true"},
		{"m istype MeasurementReferences::DerivedUnit", "false"},
		{"(m * s) istype LengthUnit", "false"},
		{"(m * m) istype AreaUnit", "true"},
		{"(m * s) istype AreaUnit", "false"},
		{"(m * m) hastype AreaUnit", "false"},
		{"(m * m) @ MeasurementReferences::MeasurementUnit", "true"},
		{"m istype ScalarValues::Real", "false"},
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

// TestMeasurementRefReport: what a reference cannot answer is a typed error
// naming the declaration or the operator, never a number or a made-up member.
func TestMeasurementRefReport(t *testing.T) {
	ctx, scope := measurementRefContext(t)

	cases := []struct {
		src  string
		want error
		text string
	}{
		{"wrongUnit", ErrTypeMismatch, "cannot write the measurement reference s, a measurement reference of dimension T, to a feature typed by LengthUnit"},
		{"wrongDimension", ErrTypeMismatch, "cannot write the measurement reference m*s, a measurement reference of dimension L·T, to a feature typed by AreaUnit"},
		{"notAUnit", ErrTypeMismatch, "cannot write the measurement reference m, a measurement reference typed LengthUnit, to a feature typed by LengthValue"},
		{"notAnArea", ErrTypeMismatch, "cannot write the measurement reference m**2, a measurement reference typed DerivedUnit, to a feature typed by AreaValue"},
		{"misPowered", ErrTypeMismatch, "cannot write the measurement reference m**2, a measurement reference of dimension L^2, to a feature typed by LengthUnit"},
		{"notAScale", ErrTypeMismatch, "cannot write the measurement reference h*s/min, a measurement reference typed DerivedUnit, to a feature typed by TimeScale"},
		{"notAnInterval", ErrTypeMismatch, "cannot write the measurement reference h*s/min, a measurement reference typed DerivedUnit, to a feature typed by IntervalScale"},
		{"ConvertQuantity(3 [s], Time::UTC)", ErrUnevaluableLibraryFunction, "Time::UTC"},
		{"ConvertQuantity(3 [Time::UTC], s)", ErrUnevaluableLibraryFunction, "Time::UTC"},
		{"ConvertQuantity(3 [K], Time::UTC)", ErrIncommensurableUnits, ""},
		{"Time::UTC / s", ErrUnevaluableLibraryFunction, "UTC is a measurement scale"},
		{"m + m", ErrTypeMismatch, "operator '+' is not defined for a measurement reference and a measurement reference"},
		{"m - s", ErrTypeMismatch, "operator '-' is not defined for a measurement reference and a measurement reference"},
		{"m * 3", ErrTypeMismatch, "operator '*' is not defined for a measurement reference and an Integer"},
		{"3 * m", ErrTypeMismatch, "operator '*' is not defined for an Integer and a measurement reference"},
		{"m / 2.0", ErrTypeMismatch, "operator '/' is not defined for a measurement reference and a Real"},
		{"m ** s", ErrTypeMismatch, "operator '**' is not defined for a measurement reference and a measurement reference"},
		{"-m", ErrTypeMismatch, "unary '-' requires numeric operand, got measurement reference"},
		{"m < s", nil, "comparison operands must be constants, got measurement reference and measurement reference"},
		{"(m / s).quantityDimension", ErrUnevaluableLibraryFunction, "MeasurementReferences::DerivedUnit::quantityDimension: m/s is a MeasurementReferences::DerivedUnit reducing to metre·second^-1, which names no declaration whose member quantityDimension could be read"},
		{"(m / s).unitConversion", ErrUnevaluableLibraryFunction, "MeasurementReferences::DerivedUnit::unitConversion: m/s is a MeasurementReferences::DerivedUnit reducing to metre·second^-1"},
		{"(m / s).unitPowerFactors", ErrUnevaluableLibraryFunction, "MeasurementReferences::DerivedUnit::unitPowerFactors: m/s is a MeasurementReferences::DerivedUnit reducing to metre·second^-1"},
		{"(m ** exponent).definitionalQuantityValues", ErrUnevaluableLibraryFunction, "MeasurementReferences::DerivedUnit::definitionalQuantityValues: m**2 is a MeasurementReferences::DerivedUnit reducing to metre^2"},
		{"m.hasValidUnitPowerFactors", ErrUnevaluableLibraryFunction, "MeasurementReferences::ScalarMeasurementReference::hasValidUnitPowerFactors: a measurement reference value holds the unit m and its reduction metre, not the declaration's member hasValidUnitPowerFactors"},
		{"(m / s).foo", ErrTypeMismatch, "measurement reference has no feature foo"},
		{"m.foo", ErrTypeMismatch, "measurement reference has no feature foo"},
		{"side.isBound", ErrUnevaluableLibraryFunction, "Quantities::ScalarQuantityValue::isBound: a scalar quantity value holds its num and mRef, not whether the quantity is bound"},
		{"side.quantityDimension", ErrTypeMismatch, "a quantity in km has no feature quantityDimension"},
		{"ConvertQuantity(side, s)", ErrIncommensurableUnits, "function QuantityCalculations::ConvertQuantity: incommensurable units: cannot express km (1000·metre) in s (second)"},
		{"ConvertQuantity(1 [m], SI::'m/s')", ErrIncommensurableUnits, "cannot express m (metre) in 'm/s' (metre·second^-1)"},
		{"ConvertQuantity(3, m)", ErrIncommensurableUnits, "cannot express 1 (1) in m (metre)"},
		{"ConvertQuantity(m, m)", ErrTypeMismatch, `function QuantityCalculations::ConvertQuantity parameter "x" requires a quantity`},
		{"'['(3, 1 [m])", ErrTypeMismatch, `function QuantityCalculations::'[' parameter "mRef" requires a measurement reference such as SI::m, got a quantity in m`},
		{"'['(\"3\", m)", ErrTypeMismatch, `function QuantityCalculations::'[' parameter "num" requires a numeric value`},
		{"MeasurementRefCalculations::'/'(m, 2)", ErrTypeMismatch, `function MeasurementRefCalculations::'/' parameter "y" requires a measurement reference such as SI::m, got an Integer`},
		{"MeasurementRefCalculations::'^'(m, s)", ErrTypeMismatch, `function MeasurementRefCalculations::'^' parameter "y" requires a numeric value`},
		{"MeasurementRefCalculations::ToString(1 [m])", ErrTypeMismatch, `function MeasurementRefCalculations::ToString parameter "x" requires a measurement reference such as SI::m, got a quantity in m`},
		{"MeasurementRefCalculations::'CoordinateFrame*'(m, s)", ErrTypeMismatch, `function MeasurementRefCalculations::'CoordinateFrame*' parameter "x" requires a coordinate frame`},
		{"MeasurementRefCalculations::'CoordinateFrame/'(m, s)", ErrTypeMismatch, `function MeasurementRefCalculations::'CoordinateFrame/' parameter "x" requires a coordinate frame`},
		{"VectorCalculations::'['((1.0, 2.0), m)", ErrTypeMismatch, `function VectorCalculations::'[' parameter "mRef" requires a coordinate frame`},
		{"VectorCalculations::transform(m, vq)", ErrTypeMismatch, `function VectorCalculations::transform parameter "transformation" requires a coordinate transformation`},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			got, err := evalIn(t, ctx, scope, tc.src)
			if err == nil {
				t.Fatalf("%s = %s, want error %v", tc.src, FormatValue(got), tc.want)
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Errorf("%s: error %v, want %v", tc.src, err, tc.want)
			}
			if !strings.Contains(err.Error(), tc.text) {
				t.Errorf("%s: error %q does not mention %q", tc.src, err, tc.text)
			}
		})
	}
}

// TestVectorQuantityMRefNeedsOneUnit: a vector quantity's mRef is the one
// reference its axes share, however each is spelt; axes measuring differently,
// even under one spelling, have no one scalar reference, so mRef is a typed error.
func TestVectorQuantityMRefNeedsOneUnit(t *testing.T) {
	ctx, scope := measurementRefContext(t)
	unitOf := func(src string) Unit {
		val, err := evalIn(t, ctx, scope, src)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		if val.Kind != ValMeasurementRef {
			t.Fatalf("%s = %s, want a measurement reference", src, FormatValue(val))
		}
		return val.MeasurementRef().Unit
	}
	metre, qualified, second, mile := unitOf("m"), unitOf("(1 [SI::m]).mRef"), unitOf("s"), unitOf("Imperial::mile")
	if qualified.String() == metre.String() || mile.String() != metre.String() {
		t.Fatalf("spellings %q, %q and %q do not exercise the cases", metre, qualified, mile)
	}
	num := []semantics.Value{{Kind: semantics.ValReal, Real: 1}, {Kind: semantics.ValReal, Real: 2}}
	wantMRef := func(units []Unit, want Unit) {
		t.Helper()
		vq := NewVectorQuantityValue(num, units)
		got, ok, err := ctx.structuredFeature(vq, "mRef")
		if !ok || err != nil || !valueEqual(got, measurementRefOf(want)) || FormatValue(got) != want.String() {
			t.Fatalf("mRef of %s = %s, %v, %v; want %s", FormatValue(vq), FormatValue(got), ok, err, want)
		}
	}
	wantNoMRef := func(units []Unit, axes string) {
		t.Helper()
		vq := NewVectorQuantityValue(num, units)
		_, ok, err := ctx.structuredFeature(vq, "mRef")
		if !ok || !errors.Is(err, ErrUnevaluableLibraryFunction) {
			t.Fatalf("mRef of %s = %v, %v; want %v", FormatValue(vq), ok, err, ErrUnevaluableLibraryFunction)
		}
		want := "Quantities::VectorQuantityValue::mRef: the axes of " + axes + " carry different units, and no one measurement reference names them all"
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}

	wantMRef([]Unit{metre, metre}, metre)
	wantMRef([]Unit{metre, qualified}, metre)
	wantMRef([]Unit{qualified, metre}, qualified)
	wantNoMRef([]Unit{metre, second}, "⟨1.0 [m], 2.0 [s]⟩")
	wantNoMRef([]Unit{metre, mile}, "⟨1.0 [m], 2.0 [m]⟩")
}

// TestMeasurementRefDescribed: the kind describes, renders and traces itself.
func TestMeasurementRefDescribed(t *testing.T) {
	ctx, scope := measurementRefContext(t)
	ref, err := evalIn(t, ctx, scope, "km / h")
	if err != nil {
		t.Fatalf("km / h: %v", err)
	}
	if got := describeOperand(ref); got != "a measurement reference" {
		t.Errorf("describeOperand(km/h) = %q", got)
	}
	if got := describeValue(ref); got != "measurement reference" {
		t.Errorf("describeValue(km/h) = %q", got)
	}
	if got := FormatValue(ref); got != "km/h" {
		t.Errorf("FormatValue(km/h) = %q", got)
	}
	if got := ref.Kind.String(); got != "measurement reference" {
		t.Errorf("Kind.String() = %q", got)
	}
	if ref.MeasurementRef().Declaration() != nil {
		t.Error("km/h names a single declaration")
	}
	single, err := evalIn(t, ctx, scope, "SI::km")
	if err != nil {
		t.Fatalf("SI::km: %v", err)
	}
	if decl := single.MeasurementRef().Declaration(); decl == nil || unitSymbolName(decl) != "km" {
		t.Errorf("SI::km names the declaration %v, want km", decl)
	}
	var nilRef *MeasurementRef
	if got := nilRef.String(); got != "<unknown>" {
		t.Errorf("nil reference renders %q", got)
	}
	if nilRef.equal(nilRef) != true || nilRef.equal(ref.MeasurementRef()) {
		t.Error("nil reference equality is not identity")
	}
}
