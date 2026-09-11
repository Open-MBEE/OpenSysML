package runtime

import (
	"errors"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// TestQuantityCalculationsAreAllDispatchable: every Quantities and Units calculation is
// registered with its parameters' effective names in declared order, or is a builtin.
// An `in : Type` parameter takes the name of the parameter it implicitly redefines
// in the calc's general (KerML 7.4.7.2); with no general it is anonymous, "".
func TestQuantityCalculationsAreAllDispatchable(t *testing.T) {
	packages := map[string]string{
		"QuantityCalculations":       "Domain Libraries/Quantities and Units/QuantityCalculations.sysml",
		"MeasurementRefCalculations": "Domain Libraries/Quantities and Units/MeasurementRefCalculations.sysml",
		"VectorCalculations":         "Domain Libraries/Quantities and Units/VectorCalculations.sysml",
		"TensorCalculations":         "Domain Libraries/Quantities and Units/TensorCalculations.sysml",
	}
	idx := libs.NewModelIndex()
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	ctx := NewContext(NewModel(model, resolver), 10000)

	for pkg, path := range packages {
		t.Run(pkg, func(t *testing.T) {
			declared := 0
			for _, sym := range idx.LookupDirectChildren(pkg) {
				if !isCalcSymbol(sym) {
					continue
				}
				declared++
				fqn := ctx.qualifiedSymbolName(sym)
				fn, ok := libraryFunctionByName(fqn)
				if !ok {
					if _, isBuiltin := builtins[fqn]; !isBuiltin {
						t.Errorf("%s is declared in %s and is not dispatchable", fqn, path)
					}
					continue
				}
				params := effectiveInputs(t, ctx, sym)
				if !slices.Equal(params, fn.params) {
					t.Errorf("%s declares %+v, implementation takes %+v", fqn, params, fn.params)
				}
			}
			if declared == 0 {
				t.Fatalf("%s declares no calculations; the gate found nothing to check", pkg)
			}
		})
	}
}

// effectiveInputs lists a calc's input parameters in declared order by
// effective name, an anonymous one as "", with the optionality each declares.
func effectiveInputs(t *testing.T, ctx *Context, sym *symbols.Symbol) []declaredParam {
	t.Helper()
	var params []declaredParam
	for _, param := range ctx.model.semantics.BehaviorParametersOf(sym) {
		if param.IsResult || (param.Direction != ast.DirIn && param.Direction != ast.DirInOut) {
			continue
		}
		usage, ok := param.Symbol.Decl.(*ast.Usage)
		if !ok {
			t.Fatalf("%s: parameter %s is declared by a %T, not a usage", ctx.qualifiedSymbolName(sym), param.Symbol.Name, param.Symbol.Decl)
		}
		params = append(params, declaredParam{
			name:     ctx.model.semantics.EffectiveNameOf(param.Symbol),
			optional: usage.Value != nil || ctx.model.semantics.IsOptionalParameter(usage),
		})
	}
	return params
}

// An anonymous library parameter binds by position only: a call cannot name it,
// and LibraryFunctionParams publishes no name for it, while a parameter that
// takes its name from the redefined general parameter binds by that name.
func TestAnonymousLibraryParametersBindByPositionOnly(t *testing.T) {
	unit := realVec(0.6, 0.8)

	params, ok := LibraryFunctionParams("VectorCalculations::isUnitVectorQuantity")
	if !ok || !slices.Equal(params, []string{""}) {
		t.Fatalf("LibraryFunctionParams(isUnitVectorQuantity) = %q, %v; want [\"\"], true", params, ok)
	}
	params, ok = LibraryFunctionParams("VectorCalculations::angle")
	if !ok || !slices.Equal(params, []string{"v", "w"}) {
		t.Fatalf("LibraryFunctionParams(angle) = %q, %v; want [v w], true", params, ok)
	}

	fn, ok := libraryFunctionByName("VectorCalculations::isUnitVectorQuantity")
	if !ok {
		t.Fatal("VectorCalculations::isUnitVectorQuantity is not registered")
	}
	got, err := fn.invoke(libCtx(t), calcArgs{positional: []Value{unit}})
	if err != nil || !valueEqual(got, boolValue(true)) {
		t.Fatalf("isUnitVectorQuantity((0.6, 0.8)) = %+v, %v; want true", got, err)
	}
	for _, name := range []string{"v", "x", ""} {
		_, err = fn.invoke(libCtx(t), calcArgs{named: map[string]Value{name: unit}})
		if !errors.Is(err, ErrUnknownParameter) {
			t.Fatalf("isUnitVectorQuantity(%s = ...) error = %v, want %v", name, err, ErrUnknownParameter)
		}
		if !strings.Contains(err.Error(), "(expected #1)") {
			t.Fatalf("isUnitVectorQuantity(%s = ...) error %q does not list the parameter by position", name, err)
		}
	}
	_, err = fn.invoke(libCtx(t), calcArgs{positional: []Value{vec(strValue("a"))}})
	if !errors.Is(err, ErrTypeMismatch) || !strings.Contains(err.Error(), `parameter #1 requires`) {
		t.Fatalf("isUnitVectorQuantity((\"a\")) error = %v, want %v naming parameter #1", err, ErrTypeMismatch)
	}

	outer, ok := libraryFunctionByName("VectorCalculations::outer")
	if !ok {
		t.Fatal("VectorCalculations::outer is not registered")
	}
	_, err = outer.invoke(libCtx(t), calcArgs{named: map[string]Value{"v": unit, "w": unit}})
	if !errors.Is(err, ErrUnknownParameter) || !strings.Contains(err.Error(), "(expected #1, #2)") {
		t.Fatalf("outer(v = ..., w = ...) error = %v, want %v listing #1, #2", err, ErrUnknownParameter)
	}

	angle, ok := libraryFunctionByName("VectorCalculations::angle")
	if !ok {
		t.Fatal("VectorCalculations::angle is not registered")
	}
	got, err = angle.invoke(libCtx(t), calcArgs{named: map[string]Value{"w": realVec(0, 1), "v": realVec(1, 0)}})
	if err != nil || asReal(got.Const) != math.Pi/2 {
		t.Fatalf("angle(w = (0, 1), v = (1, 0)) = %+v, %v; want π/2", got, err)
	}
}

// quantityCalculationsContext evaluates expressions in a package that imports
// ISQ, SI and QuantityCalculations, as the ISQ examples do.
func quantityCalculationsContext(t *testing.T) (*Context, *symbols.Scope) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			public import ISQ::*;
			public import SI::*;
			public import QuantityCalculations::*;
			public import TrigFunctions::*;
			attribute side : LengthValue = 3 [m];
			attribute area : AreaValue = side * side;
			attribute none : LengthValue[0..*] = ();
		}
	`))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	return ctx, pkg.Scope
}

// TestQuantityCalculations evaluates each QuantityCalculations function the
// runtime computes, over quantities written in SI units.
func TestQuantityCalculations(t *testing.T) {
	ctx, scope := quantityCalculationsContext(t)

	cases := []struct {
		src  string
		want string
	}{
		{"sqrt(area)", "3.0 [m]"},
		{"sqrt(9 [m**2])", "3.0 [m]"},
		{"sqrt(16 [m*m])", "4.0 [m]"},
		{"sqrt(4 [m**2/s**2])", "2.0 [SI::'m/s']"},
		{"sqrt(2.25 [km**2])", "1.5 [km]"},
		{"sqrt(4 [N**2])", "2.0 [N]"},
		{"sqrt(9 [rad**2])", "3.0 [rad]"},
		{"sqrt(9 [m/m])", "3.0"},
		{"sqrt(9 [m*m/m**2])", "3.0"},
		{"sqrt(4 [km*m])", "63.245553203367585 [m]"},
		{"sqrt(16.0)", "4.0"},
		{"abs(-3 [m])", "3 [m]"},
		{"abs(-2.5 [m/s])", "2.5 [m/s]"},
		{"abs(-3 ['°'])", "3 ['°']"},
		{"abs(-3)", "3"},
		{"abs(ToDimensionOneValue(-2.5))", "2.5"},
		{"floor(2.7)", "2"},
		{"round(2.5)", "3"},
		{"max(1, 2)", "2"},
		{"QuantityCalculations::'+'(2)", "2"},
		{"QuantityCalculations::'+'(2, 1 [rad])", "3"},
		{"QuantityCalculations::'-'(2)", "-2"},
		{"QuantityCalculations::'*'(2, 3 [m])", "6 [m]"},
		{"QuantityCalculations::'*'(2 [m], 3)", "6 [m]"},
		{"QuantityCalculations::'/'(6 [m], 3)", "2.0 [m]"},
		{"QuantityCalculations::'=='(2, 2)", "true"},
		{"ToString(2)", `"2"`},
		{"abs(2 [kg])", "2 [kg]"},
		{"floor(2.7 [m])", "2 [m]"},
		{"floor(-2.2 [m])", "-3 [m]"},
		{"round(2.5 [m])", "3 [m]"},
		{"round(2.4 [m])", "2 [m]"},
		{"max(1 [m], 200 [cm])", "200 [cm]"},
		{"max(300 [cm], 2 [m])", "300 [cm]"},
		{"max(1 [m], 100 [cm])", "1 [m]"},
		{"QuantityCalculations::min(1 [m], 200 [cm])", "1 [m]"},
		{"QuantityCalculations::min(300 [cm], 2 [m])", "2 [m]"},
		{"QuantityCalculations::min(1 [m], 100 [cm])", "1 [m]"},
		{"(1 [m], 2 [m])->sum()", "3 [m]"},
		{"(1 [km], 500 [m])->sum()", "1.5 [km]"},
		{"(side, side, side)->sum()", "9 [m]"},
		{"sum((2 [m], 3 [m]))", "5 [m]"},
		{"(2 [m], 3 [m])->product()", "6 [SI::'m²']"},
		{"(2 [m], 3 [s])->product()", "6 [m*s]"},
		{"(2 [m])->sum()", "2 [m]"},
		{"none->sum()", "0 [m]"},
		{"none->product()", "1"},
		{"QuantityCalculations::'+'(1 [m], 2 [m])", "3 [m]"},
		{"QuantityCalculations::'+'(2 [m])", "2 [m]"},
		{"QuantityCalculations::'-'(5 [m], 2 [m])", "3 [m]"},
		{"QuantityCalculations::'-'(2 [m])", "-2 [m]"},
		{"QuantityCalculations::'*'(2 [m], 3 [m])", "6 [SI::'m²']"},
		{"QuantityCalculations::'/'(6 [m], 3 [s])", "2.0 [SI::'m/s']"},
		{"QuantityCalculations::'/'(6 [m], 3 [m])", "2.0"},
		{"QuantityCalculations::'**'(3 [m], 2)", "9 [SI::'m²']"},
		{"QuantityCalculations::'^'(2 [m], 3)", "8 [SI::'m³']"},
		{"QuantityCalculations::'<'(1 [m], 200 [cm])", "true"},
		{"QuantityCalculations::'>'(1 [m], 200 [cm])", "false"},
		{"QuantityCalculations::'<='(1 [m], 100 [cm])", "true"},
		{"QuantityCalculations::'>='(1 [m], 100 [cm])", "true"},
		{"QuantityCalculations::'=='(1 [m], 100 [cm])", "true"},
		{"QuantityCalculations::'=='(1 [m], 101 [cm])", "false"},
		{"QuantityCalculations::'<'(0, 1 [m])", "true"},
		{"QuantityCalculations::'>='(1 [m], 0.0)", "true"},
		{"QuantityCalculations::'=='(0 [m], 0)", "true"},
		{"isZero(0 [m])", "true"},
		{"isZero(0.0 [m/s])", "true"},
		{"isZero(1 [m])", "false"},
		{"isUnit(1 [m])", "true"},
		{"isUnit(2 [m])", "false"},
		{"ToString(1.5 [m/s])", `"1.5 [m/s]"`},
		{"ToString(2 [m] * 3 [m])", `"6 [SI::'m²']"`},
		{"ToInteger(3 [m])", "3"},
		{"ToInteger(3.0 [m])", "3"},
		{"ToInteger(2.5 [m])", "2"},
		{"ToInteger(-2.5 [m])", "-2"},
		{"ToReal(3 [m])", "3.0"},
		{"ToRational(1.5 [m])", "1.5"},
		{"ToDimensionOneValue(2.5)", "2.5"},
		{"sin(90 ['°']) > 0.99999", "true"},
		{"cos(0 [rad])", "1.0"},
		{"sin(0.0 [rad])", "0.0"},
		{"tan(45 ['°']) < 1.0001 and tan(45 ['°']) > 0.9999", "true"},
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

// TestQuantityCalculationsReport: each failure a QuantityCalculations function
// has is a typed error naming the function.
func TestQuantityCalculationsReport(t *testing.T) {
	ctx, scope := quantityCalculationsContext(t)

	cases := []struct {
		src  string
		want error
		text string
	}{
		{"sqrt(9 [m])", ErrUnitRoot, "function QuantityCalculations::sqrt: unit has no root: m (metre) raises metre to the odd power 1"},
		{"sqrt(8 [m**3])", ErrUnitRoot, "raises metre to the odd power 3"},
		{"sqrt(4 [m**2/s])", ErrUnitRoot, "raises second to the odd power -1"},
		{"sqrt(9 [rad])", ErrUnitRoot, "function QuantityCalculations::sqrt: unit has no root: rad raises the dimension-one unit rad to the odd power 1"},
		{"sqrt(9 ['°'])", ErrUnitRoot, "'°' raises the dimension-one unit '°' to the odd power 1"},
		{"sqrt(9 [rad**3])", ErrUnitRoot, "raises the dimension-one unit rad to the odd power 3"},
		{"sqrt(9 [rad*m**2])", ErrUnitRoot, "raises the dimension-one unit rad to the odd power 1"},
		{"sqrt(-4 [m**2])", semantics.ErrArithmeticDomain, "sqrt of a negative quantity -4 [m**2]"},
		{"sqrt(\"9\")", ErrTypeMismatch, `function QuantityCalculations::sqrt parameter "x" requires a quantity, got string`},
		{"abs(true)", ErrTypeMismatch, `function QuantityCalculations::abs parameter "x" requires a quantity`},
		{"max(1 [m], 1 [s])", ErrIncommensurableUnits, "function QuantityCalculations::max"},
		{"QuantityCalculations::min(1 [kg], 1 [m])", ErrIncommensurableUnits, "function QuantityCalculations::min"},
		{"max(1 [m], 0)", ErrIncommensurableUnits, "function QuantityCalculations::max"},
		{"max(0, -1 [m])", ErrIncommensurableUnits, "function QuantityCalculations::max"},
		{"QuantityCalculations::min(1 [m], 0)", ErrIncommensurableUnits, "function QuantityCalculations::min"},
		{"QuantityCalculations::min(0.0, 1 [m])", ErrIncommensurableUnits, "function QuantityCalculations::min"},
		{"(1 [m], 1 [s])->sum()", ErrIncommensurableUnits, ""},
		{"QuantityCalculations::'+'(1 [m], 1 [s])", ErrIncommensurableUnits, "function QuantityCalculations::'+'"},
		{"QuantityCalculations::'+'(1 [m], 1)", ErrIncommensurableUnits, "function QuantityCalculations::'+'"},
		{"QuantityCalculations::'<'(1 [m], 1)", ErrIncommensurableUnits, "function QuantityCalculations::'<'"},
		{"max(1 [m], 1)", ErrIncommensurableUnits, "function QuantityCalculations::max"},
		{"QuantityCalculations::'<'(1 [m], 1 [s])", ErrIncommensurableUnits, "function QuantityCalculations::'<'"},
		{"QuantityCalculations::'/'(1 [m], 0 [s])", ErrDivisionByZero, "function QuantityCalculations::'/'"},
		{"sin(90 [m])", ErrTypeMismatch, `function TrigFunctions::sin parameter "theta" requires a number of radians or an angle quantity, got a quantity in m`},
		{"arcsin(1 [rad])", ErrTypeMismatch, `function TrigFunctions::arcsin parameter "x" requires a numeric value, got a quantity in rad`},
		{"IntegerFunctions::abs(1 [rad])", ErrTypeMismatch, `function IntegerFunctions::abs parameter "x" requires a numeric value, got a quantity in rad`},
		{"RealFunctions::sqrt(4 ['°'])", ErrTypeMismatch, `function RealFunctions::sqrt parameter "x" requires a numeric value, got a quantity in '°'`},
		{"NaturalFunctions::max(1 [rad], 2)", ErrTypeMismatch, `function NaturalFunctions::max parameter "x" requires a numeric value`},
		{"OpenSysMLMathFunctions::exp(1 [rad])", ErrTypeMismatch, `function OpenSysMLMathFunctions::exp parameter "x" requires a numeric value`},
		{"ConvertQuantity(1 [m], 2 [cm])", ErrTypeMismatch, `function QuantityCalculations::ConvertQuantity parameter "targetMRef" requires a measurement reference such as SI::m, got a quantity in cm`},
		{"ConvertQuantity(1 [m], s)", ErrIncommensurableUnits, "function QuantityCalculations::ConvertQuantity: incommensurable units: cannot express m (metre) in s (second)"},
		{"QuantityCalculations::'['(1, 2)", ErrTypeMismatch, `function QuantityCalculations::'[' parameter "mRef" requires a measurement reference such as SI::m, got an Integer`},
		{"QuantityCalculations::'['(1 [m], m)", ErrTypeMismatch, `function QuantityCalculations::'[' parameter "num" requires a numeric value`},
		{"MeasurementRefCalculations::'*'(1, 2)", ErrTypeMismatch, `function MeasurementRefCalculations::'*' parameter "x" requires a measurement reference such as SI::m, got an Integer`},
		{"MeasurementRefCalculations::'**'(m, s)", ErrTypeMismatch, `function MeasurementRefCalculations::'**' parameter "y" requires a numeric value`},
		{"MeasurementRefCalculations::ToString(1)", ErrTypeMismatch, `function MeasurementRefCalculations::ToString parameter "x" requires a measurement reference such as SI::m, got an Integer`},
		{"MeasurementRefCalculations::'CoordinateFrame*'(m, s)", ErrTypeMismatch, `function MeasurementRefCalculations::'CoordinateFrame*' parameter "x" requires a coordinate frame`},
		{"VectorCalculations::'['((1.0, 2.0), m)", ErrTypeMismatch, `function VectorCalculations::'[' parameter "mRef" requires a coordinate frame`},
		{"VectorCalculations::outer((1.0, 2.0), (3.0, 4.0))", ErrUnevaluableLibraryFunction, "VectorCalculations::outer: the declaration `calc def outer { in : VectorQuantityValue[1]; in : VectorQuantityValue[1]; return : VectorQuantityValue[1]; }` returns a VectorQuantityValue, and the outer product of two vectors is a tensor of order two"},
		{"VectorCalculations::vectorScalarQuantityDiv(VectorFunctions::VectorOf((1.0, 2.0)) [m], 0 [s])", ErrDivisionByZero, "function VectorCalculations::vectorScalarQuantityDiv"},
		{"VectorCalculations::'+'(VectorFunctions::VectorOf((1.0, 2.0)) [m], VectorFunctions::VectorOf((1.0, 2.0)) [s])", ErrIncommensurableUnits, "function VectorCalculations::'+'"},
		{"VectorCalculations::'+'(VectorFunctions::VectorOf((1.0, 2.0)) [m], VectorFunctions::VectorOf((1.0, 2.0, 3.0)) [m])", ErrTypeMismatch, "requires vectors of equal dimension, got 2 and 3"},
		{"VectorCalculations::transform(1, (1.0, 2.0))", ErrTypeMismatch, `function VectorCalculations::transform parameter "transformation" requires a coordinate transformation, got an Integer`},
		{"VectorCalculations::'+'((1 [m], 2 [m]), (3 [m], 4 [m]))", ErrTypeMismatch, `function VectorCalculations::'+' parameter "v" requires a vector of numeric values, element 1 is a quantity in m`},
		{"VectorCalculations::norm((3 [m], 4 [m]))", ErrTypeMismatch, `function VectorCalculations::norm parameter "v" requires a vector of numeric values, element 1 is a quantity in m`},
		{"VectorCalculations::vectorScalarMult((1.0, 2.0), 2 [m])", ErrTypeMismatch, `function VectorCalculations::vectorScalarMult parameter "x" requires a numeric value`},
		{"TensorCalculations::'+'(1, 2)", ErrTypeMismatch, `function TensorCalculations::'+' parameter "x" requires a TensorQuantityValue (a tensor, vector or scalar quantity), got an Integer`},
		{"TensorCalculations::tensorTensorMult(1, 2)", ErrUnevaluableLibraryFunction, "TensorCalculations::tensorTensorMult: the declaration `calc def tensorTensorMult { in : TensorQuantityValue[1]; in : TensorQuantityValue[1]; return : TensorQuantityValue[1]; }` states only the parameter and result types; neither TensorCalculations nor the Kernel says which indices contract"},
		{"TensorCalculations::tensorVectorMult(1, 2)", ErrUnevaluableLibraryFunction, "TensorCalculations::tensorVectorMult: the declaration `calc def tensorVectorMult { in : TensorQuantityValue[1]; in : VectorQuantityValue[1]; return : VectorQuantityValue[1]; }` states only the parameter and result types"},
		{"TensorCalculations::vectorTensorMult(1, 2)", ErrUnevaluableLibraryFunction, "TensorCalculations::vectorTensorMult: the declaration `calc def vectorTensorMult { in : VectorQuantityValue[1]; in : TensorQuantityValue[1]; return : VectorQuantityValue[1]; }` states only the parameter and result types"},
		{"TensorCalculations::transform(1, 2)", ErrUnevaluableLibraryFunction, "TensorCalculations::transform: the declaration `calc def transform { in transformation : CoordinateTransformation; in sourceTensor : TensorQuantityValue; return targetTensor : TensorQuantityValue; }` has no body, and neither TensorCalculations nor the Kernel says how a CoordinateTransformation acts on a tensor's contravariant and covariant indices: the runtime transforms a vector quantity (VectorCalculations::transform), and a tensor quantity carries a unit per component and no source frame"},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			got, err := evalIn(t, ctx, scope, tc.src)
			if err == nil {
				t.Fatalf("%s = %s, want error %v", tc.src, FormatValue(got), tc.want)
			}
			if !errors.Is(err, tc.want) {
				t.Errorf("%s: error %v, want %v", tc.src, err, tc.want)
			}
			if !strings.Contains(err.Error(), tc.text) {
				t.Errorf("%s: error %q does not mention %q", tc.src, err, tc.text)
			}
		})
	}
}

// TestTrigFunctionsTakeAngles: a trigonometric function takes a number of radians
// or a quantity in an angular unit; a dimension-one quantity in any other unit
// (bit, sr, one) or an angle raised to a power is rejected, not read as radians.
func TestTrigFunctionsTakeAngles(t *testing.T) {
	ctx, scope := quantityCalculationsContext(t)

	accepted := []struct {
		src  string
		want float64
	}{
		{"sin(0.5)", math.Sin(0.5)},
		{"sin(0.5 [rad])", math.Sin(0.5)},
		{"cos(0.5 [rad])", math.Cos(0.5)},
		{"tan(0.5 [rad])", math.Tan(0.5)},
		{"cot(0.5 [rad])", 1 / math.Tan(0.5)},
		{"sin(30 ['°'])", math.Sin(30 * 1.745329e-02)},
		{"cos(60 ['°'])", math.Cos(60 * 1.745329e-02)},
		{"tan(45 ['°'])", math.Tan(45 * 1.745329e-02)},
		{"cot(45 ['°'])", 1 / math.Tan(45*1.745329e-02)},
		{"sin(60 [arcmin])", math.Sin(60 * 2.908882e-04)},
		{"sin(0.5 [rad] * 2 [m] / 1 [m])", math.Sin(1)},
		{"sin(1 [m] / 2 [m])", math.Sin(0.5)},
	}
	for _, tc := range accepted {
		t.Run(tc.src, func(t *testing.T) {
			got, err := evalIn(t, ctx, scope, tc.src)
			if err != nil {
				t.Fatalf("%s: %v", tc.src, err)
			}
			if got.Kind != ValConst || got.Const.Kind != semantics.ValReal || math.Abs(got.Const.Real-tc.want) > 1e-12 {
				t.Errorf("%s = %s, want %v", tc.src, FormatValue(got), tc.want)
			}
		})
	}

	rejected := []struct {
		src string
		got string
	}{
		{"sin(1 [bit])", "a quantity in bit"},
		{"cos(1 [B])", "a quantity in B"},
		{"tan(1 [sr])", "a quantity in sr"},
		{"cot(1 [MeasurementReferences::one])", "a quantity in MeasurementReferences::one"},
		{"sin(1 [rad**2])", "a quantity in rad**2"},
		{"sin(1 [rad] * 1 ['°'])", "a quantity in '°'*rad"},
		{"sin(1 [rad*bit])", "a quantity in rad*bit"},
		{"sin(1 [m])", "a quantity in m"},
	}
	for _, tc := range rejected {
		t.Run(tc.src, func(t *testing.T) {
			got, err := evalIn(t, ctx, scope, tc.src)
			if err == nil {
				t.Fatalf("%s = %s, want %v", tc.src, FormatValue(got), ErrTypeMismatch)
			}
			if !errors.Is(err, ErrTypeMismatch) {
				t.Errorf("%s: error %v, want %v", tc.src, err, ErrTypeMismatch)
			}
			want := `requires a number of radians or an angle quantity, got ` + tc.got
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: error %q does not mention %q", tc.src, err, want)
			}
		})
	}
}

// testQuantityCalculationThatHasNoValue: a calculation with no answer (a root no unit
// has, a value no representation holds) is a typed error, never a made-up unit or a NaN.
func testQuantityCalculationThatHasNoValue(t *testing.T) {
	ctx, scope := quantityCalculationsContext(t)
	for _, tt := range []struct {
		expr string
		want error
	}{
		{"sqrt(side)", ErrUnitRoot},
		{"sqrt(-1 * area)", semantics.ErrArithmeticDomain},
		{"sqrt(side, side)", ErrCalcArity},
		{"max(side, 1 [s])", ErrIncommensurableUnits},
		{"QuantityCalculations::'/'(side, 0 [s])", ErrDivisionByZero},
		{"ConvertQuantity(side, side)", ErrTypeMismatch},
		{"ConvertQuantity(side, s)", ErrIncommensurableUnits},
		{"VectorCalculations::outer((1.0, 2.0), (3.0, 4.0))", ErrUnevaluableLibraryFunction},
		{"TensorCalculations::tensorTensorMult(side, side)", ErrUnevaluableLibraryFunction},
	} {
		got, err := evalIn(t, ctx, scope, tt.expr)
		if !errors.Is(err, tt.want) {
			t.Errorf("%s = (%v, %v), want %v", tt.expr, got, err, tt.want)
		}
	}
}

// TestVectorCalculations: the VectorCalculations functions compute as their
// VectorFunctions counterparts over plain vectors, and over vector quantities
// with each axis composed as the scalar QuantityCalculations compose it; a
// sequence of unit-bearing numbers is no vector (TestQuantityCalculationsReport).
func TestVectorCalculations(t *testing.T) {
	ctx, scope := quantityCalculationsContext(t)

	cases := []struct {
		src  string
		want string
	}{
		{"VectorCalculations::isZeroVectorQuantity((0.0, 0.0))", "true"},
		{"VectorCalculations::isZeroVectorQuantity((0.0, 1.0))", "false"},
		{"VectorCalculations::isUnitVectorQuantity((0.0, 1.0))", "true"},
		{"VectorCalculations::isUnitVectorQuantity((3.0, 4.0))", "false"},
		{"VectorCalculations::'+'((1.0, 2.0), (3.0, 4.0))", "⟨4.0, 6.0⟩"},
		{"VectorCalculations::'-'((3.0, 4.0), (1.0, 2.0))", "⟨2.0, 2.0⟩"},
		{"VectorCalculations::scalarVectorMult(2.0, (1.0, 2.0))", "⟨2.0, 4.0⟩"},
		{"VectorCalculations::vectorScalarMult((1.0, 2.0), 2.0)", "⟨2.0, 4.0⟩"},
		{"VectorCalculations::vectorScalarDiv((2.0, 4.0), 2.0)", "⟨1.0, 2.0⟩"},
		{"VectorCalculations::inner((1.0, 2.0), (3.0, 4.0))", "11.0"},
		{"VectorCalculations::norm((3.0, 4.0))", "5.0"},
		{"VectorCalculations::angle((1.0, 0.0), (0.0, 1.0)) > 1.5707", "true"},
		{"VectorCalculations::scalarQuantityVectorMult(2 [m], (1.0, 2.0))", "⟨2.0, 4.0⟩ [m]"},
		{"VectorCalculations::vectorScalarQuantityMult(VectorFunctions::VectorOf((1.0, 2.0)) [m], 2 [m])", "⟨2.0, 4.0⟩ [SI::'m²']"},
		{"VectorCalculations::vectorScalarQuantityDiv(VectorFunctions::VectorOf((2.0, 4.0)) [m], 2 [s])", "⟨1.0, 2.0⟩ [SI::'m/s']"},
		{"VectorCalculations::'+'(VectorFunctions::VectorOf((1.0, 2.0)) [m], VectorFunctions::VectorOf((100.0, 200.0)) [cm])", "⟨2.0, 4.0⟩ [m]"},
		// inner and norm are declared `return : Number[1]`: the magnitude in the
		// unit the axes compose ([m**2], [cm*m], [m]), the unit itself not carried.
		{"VectorCalculations::inner(VectorFunctions::VectorOf((1.0, 2.0)) [m], VectorFunctions::VectorOf((3.0, 4.0)) [m])", "11.0"},
		{"VectorCalculations::inner(VectorFunctions::VectorOf((1.0, 2.0)) [m], VectorFunctions::VectorOf((300.0, 400.0)) [cm])", "1100.0"},
		{"VectorCalculations::norm(VectorFunctions::VectorOf((3.0, 4.0)) [m])", "5.0"},
		{"VectorCalculations::norm(VectorFunctions::VectorOf((300.0, 400.0)) [cm])", "500.0"},
		{"VectorCalculations::isZeroVectorQuantity(VectorFunctions::VectorOf((0.0, 0.0)) [m])", "true"},
		{"2 [m] * VectorFunctions::VectorOf((1.0, 2.0))", "⟨2.0, 4.0⟩ [m]"},
		{"VectorFunctions::VectorOf((2.0, 4.0)) [m] / 2 [s]", "⟨1.0, 2.0⟩ [SI::'m/s']"},
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

// TestQuantitySumWithAndWithoutImport: `sum` over quantities computes the same whether
// it resolves to NumericalFunctions::sum or, with the import, QuantityCalculations::sum.
func TestQuantitySumWithAndWithoutImport(t *testing.T) {
	for _, imported := range []bool{false, true} {
		name := "without import"
		importLine := "public import NumericalFunctions::*;"
		if imported {
			name = "with import"
			importLine = "public import QuantityCalculations::*;"
		}
		t.Run(name, func(t *testing.T) {
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
				package test {
					public import ISQ::*;
					public import SI::*;
					`+importLine+`
				}
			`))
			pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
			if !ok || pkg.Scope == nil {
				t.Fatal("test package not indexed")
			}
			got, err := evalIn(t, ctx, pkg.Scope, "(1 [m], 2 [m])->sum()")
			if err != nil {
				t.Fatalf("(1 [m], 2 [m])->sum(): %v", err)
			}
			if rendered := FormatValue(got); rendered != "3 [m]" {
				t.Errorf("(1 [m], 2 [m])->sum() = %s, want 3 [m]", rendered)
			}
		})
	}
}

// TestQuantityPowerAndProductAgree: l*l and l**2 are the same quantity in the
// same rendering, magnitude kind included.
func TestQuantityPowerAndProductAgree(t *testing.T) {
	ctx, scope := quantityCalculationsContext(t)

	cases := []struct{ product, power, want string }{
		{"side * side", "side ** 2", "9 [SI::'m²']"},
		{"side * side / side", "side ** 2 / side", "3.0 [SI::m]"},
		{"2.5 [m] * 2.5 [m]", "2.5 [m] ** 2", "6.25 [SI::'m²']"},
		{"2 [km/h] * 2 [km/h]", "2 [km/h] ** 2", "0.30864197530864196 [SI::'m²⋅s⁻²']"},
	}
	for _, tc := range cases {
		t.Run(tc.power, func(t *testing.T) {
			product, err := evalIn(t, ctx, scope, tc.product)
			if err != nil {
				t.Fatalf("%s: %v", tc.product, err)
			}
			power, err := evalIn(t, ctx, scope, tc.power)
			if err != nil {
				t.Fatalf("%s: %v", tc.power, err)
			}
			if got := FormatValue(product); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.product, got, tc.want)
			}
			if got := FormatValue(power); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.power, got, tc.want)
			}
			if product.Quantity().Num.Kind != power.Quantity().Num.Kind {
				t.Errorf("%s has magnitude kind %v, %s has %v", tc.product, product.Quantity().Num.Kind, tc.power, power.Quantity().Num.Kind)
			}
		})
	}
}
