package runtime

import (
	"errors"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// libCtx returns a runtime context over an empty model, enough to apply a
// library function, which needs no scope.
func libCtx(t *testing.T) *Context {
	t.Helper()
	idx := symbols.NewIndex()
	resolver := resolve.New(idx)
	return NewContext(NewModel(semantics.NewModel(resolver), resolver), 10000)
}

func constInt(i int64) Value {
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: i}}
}

func emptySequence() Value { return NewSequenceValue(NewSequence()) }

func constReal(f float64) Value {
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: f}}
}

// applyLibrary applies the function of that fully-qualified name to positional
// arguments.
func applyLibrary(t *testing.T, name string, args ...Value) (Value, error) {
	t.Helper()
	fn, ok := libraryFunctionByName(name)
	if !ok {
		t.Fatalf("no library function %s registered", name)
	}
	return fn.invoke(libCtx(t), calcArgs{positional: args})
}

func TestLibraryFunctionValues(t *testing.T) {
	cases := []struct {
		name string
		args []Value
		want semantics.Value
	}{
		{"RealFunctions::sqrt", []Value{constReal(16)}, semantics.Value{Kind: semantics.ValReal, Real: 4}},
		{"RealFunctions::sqrt", []Value{constInt(9)}, semantics.Value{Kind: semantics.ValReal, Real: 3}},
		{"RealFunctions::abs", []Value{constReal(-2.5)}, semantics.Value{Kind: semantics.ValReal, Real: 2.5}},
		{"RealFunctions::floor", []Value{constReal(2.7)}, semantics.Value{Kind: semantics.ValInt, Int: 2}},
		{"RealFunctions::floor", []Value{constReal(-2.1)}, semantics.Value{Kind: semantics.ValInt, Int: -3}},
		{"RealFunctions::round", []Value{constReal(2.5)}, semantics.Value{Kind: semantics.ValInt, Int: 3}},
		{"RealFunctions::round", []Value{constReal(-2.5)}, semantics.Value{Kind: semantics.ValInt, Int: -3}},
		{"RealFunctions::floor", []Value{constReal(math.MinInt64)}, semantics.Value{Kind: semantics.ValInt, Int: math.MinInt64}},
		{"RealFunctions::max", []Value{constReal(2), constInt(7)}, semantics.Value{Kind: semantics.ValReal, Real: 7}},
		{"RealFunctions::min", []Value{constReal(2), constInt(7)}, semantics.Value{Kind: semantics.ValReal, Real: 2}},
		{"RationalFunctions::abs", []Value{constReal(-0.5)}, semantics.Value{Kind: semantics.ValReal, Real: 0.5}},
		{"RationalFunctions::max", []Value{constReal(0.5), constReal(0.25)}, semantics.Value{Kind: semantics.ValReal, Real: 0.5}},
		{"RationalFunctions::min", []Value{constReal(0.5), constReal(0.25)}, semantics.Value{Kind: semantics.ValReal, Real: 0.25}},
		{"NumericalFunctions::abs", []Value{constInt(-3)}, semantics.Value{Kind: semantics.ValInt, Int: 3}},
		{"NumericalFunctions::abs", []Value{constReal(-3.5)}, semantics.Value{Kind: semantics.ValReal, Real: 3.5}},
		{"NumericalFunctions::max", []Value{constInt(2), constInt(7)}, semantics.Value{Kind: semantics.ValInt, Int: 7}},
		{"NumericalFunctions::max", []Value{constInt(2), constReal(7.5)}, semantics.Value{Kind: semantics.ValReal, Real: 7.5}},
		{"NumericalFunctions::min", []Value{constInt(2), constInt(7)}, semantics.Value{Kind: semantics.ValInt, Int: 2}},
		{"NumericalFunctions::isZero", []Value{constInt(0)}, semantics.Value{Kind: semantics.ValBool, Bool: true}},
		{"NumericalFunctions::isZero", []Value{constReal(0.5)}, semantics.Value{Kind: semantics.ValBool, Bool: false}},
		{"NumericalFunctions::isUnit", []Value{constReal(1)}, semantics.Value{Kind: semantics.ValBool, Bool: true}},
		{"NumericalFunctions::isUnit", []Value{constInt(2)}, semantics.Value{Kind: semantics.ValBool, Bool: false}},
		{"IntegerFunctions::abs", []Value{constInt(-3)}, semantics.Value{Kind: semantics.ValInt, Int: 3}},
		{"IntegerFunctions::max", []Value{constInt(2), constInt(7)}, semantics.Value{Kind: semantics.ValInt, Int: 7}},
		{"IntegerFunctions::min", []Value{constInt(2), constInt(7)}, semantics.Value{Kind: semantics.ValInt, Int: 2}},
		{"NaturalFunctions::max", []Value{constInt(2), constInt(7)}, semantics.Value{Kind: semantics.ValInt, Int: 7}},
		{"NaturalFunctions::min", []Value{constInt(2), constInt(7)}, semantics.Value{Kind: semantics.ValInt, Int: 2}},
		{"TrigFunctions::sin", []Value{constReal(0)}, semantics.Value{Kind: semantics.ValReal, Real: 0}},
		{"TrigFunctions::sin", []Value{constReal(math.Pi / 2)}, semantics.Value{Kind: semantics.ValReal, Real: 1}},
		{"TrigFunctions::cos", []Value{constReal(0)}, semantics.Value{Kind: semantics.ValReal, Real: 1}},
		{"TrigFunctions::tan", []Value{constReal(0)}, semantics.Value{Kind: semantics.ValReal, Real: 0}},
		{"TrigFunctions::cot", []Value{constReal(math.Pi / 2)}, semantics.Value{Kind: semantics.ValReal, Real: math.Cos(math.Pi / 2)}},
		{"TrigFunctions::arcsin", []Value{constReal(1)}, semantics.Value{Kind: semantics.ValReal, Real: math.Pi / 2}},
		{"TrigFunctions::arccos", []Value{constReal(1)}, semantics.Value{Kind: semantics.ValReal, Real: 0}},
		{"TrigFunctions::arctan", []Value{constReal(1)}, semantics.Value{Kind: semantics.ValReal, Real: math.Pi / 4}},
		{"OpenSysMLMathFunctions::exp", []Value{constReal(0)}, semantics.Value{Kind: semantics.ValReal, Real: 1}},
		{"OpenSysMLMathFunctions::exp", []Value{constReal(1)}, semantics.Value{Kind: semantics.ValReal, Real: math.E}},
		{"OpenSysMLMathFunctions::exp", []Value{constInt(0)}, semantics.Value{Kind: semantics.ValReal, Real: 1}},
		{"OpenSysMLMathFunctions::exp", []Value{constReal(-1)}, semantics.Value{Kind: semantics.ValReal, Real: 1 / math.E}},
		{"OpenSysMLMathFunctions::ln", []Value{constReal(1)}, semantics.Value{Kind: semantics.ValReal, Real: 0}},
		{"OpenSysMLMathFunctions::ln", []Value{constReal(math.E)}, semantics.Value{Kind: semantics.ValReal, Real: 1}},
		{"OpenSysMLMathFunctions::ln", []Value{constInt(1)}, semantics.Value{Kind: semantics.ValReal, Real: 0}},
		{"OpenSysMLMathFunctions::log", []Value{constReal(8), constReal(2)}, semantics.Value{Kind: semantics.ValReal, Real: 3}},
		{"OpenSysMLMathFunctions::log", []Value{constReal(1000), constReal(10)}, semantics.Value{Kind: semantics.ValReal, Real: 3}},
		{"OpenSysMLMathFunctions::log", []Value{constInt(8), constInt(2)}, semantics.Value{Kind: semantics.ValReal, Real: 3}},
		// A base below 1 gives a decreasing logarithm: log(0.5, 0.5) is 1.
		{"OpenSysMLMathFunctions::log", []Value{constReal(0.5), constReal(0.5)}, semantics.Value{Kind: semantics.ValReal, Real: 1}},
		{"OpenSysMLMathFunctions::atan2", []Value{constReal(1), constReal(1)}, semantics.Value{Kind: semantics.ValReal, Real: math.Pi / 4}},
		// The quadrant arctan(y/x) loses: (1, -1) and (-1, 1) share the ratio -1.
		{"OpenSysMLMathFunctions::atan2", []Value{constReal(1), constReal(-1)}, semantics.Value{Kind: semantics.ValReal, Real: 3 * math.Pi / 4}},
		{"OpenSysMLMathFunctions::atan2", []Value{constReal(-1), constReal(1)}, semantics.Value{Kind: semantics.ValReal, Real: -math.Pi / 4}},
		{"OpenSysMLMathFunctions::atan2", []Value{constReal(-1), constReal(-1)}, semantics.Value{Kind: semantics.ValReal, Real: -3 * math.Pi / 4}},
		{"OpenSysMLMathFunctions::atan2", []Value{constInt(1), constInt(0)}, semantics.Value{Kind: semantics.ValReal, Real: math.Pi / 2}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := applyLibrary(t, tc.name, tc.args...)
			if err != nil {
				t.Fatalf("%s%v = error %v", tc.name, tc.args, err)
			}
			if got.Kind != ValConst || got.Const != tc.want {
				t.Fatalf("%s%v = %+v, want %+v", tc.name, tc.args, got, tc.want)
			}
		})
	}
}

func TestLibraryFunctionErrors(t *testing.T) {
	cases := []struct {
		name string
		fn   string
		args []Value
		want error
	}{
		{"square root of a negative", "RealFunctions::sqrt", []Value{constReal(-1)}, semantics.ErrArithmeticDomain},
		{"arcsin outside the unit bound", "TrigFunctions::arcsin", []Value{constReal(2)}, semantics.ErrArithmeticDomain},
		{"arccos outside the unit bound", "TrigFunctions::arccos", []Value{constReal(-2)}, semantics.ErrArithmeticDomain},
		// No Real is exactly pi/2, so tan has no infinite argument to report;
		// cot does, at zero.
		{"cotangent of a zero sine", "TrigFunctions::cot", []Value{constReal(0)}, semantics.ErrArithmeticOverflow},
		{"floor beyond the Integer range", "RealFunctions::floor", []Value{constReal(1e300)}, semantics.ErrArithmeticOverflow},
		// 2^63 is the least Real above the Integer range, and the only one the
		// int64 conversion would silently wrap.
		{"floor at the Integer boundary", "RealFunctions::floor", []Value{constReal(-float64(math.MinInt64))}, semantics.ErrArithmeticOverflow},
		{"round at the Integer boundary", "RealFunctions::round", []Value{constReal(-float64(math.MinInt64))}, semantics.ErrArithmeticOverflow},
		{"absolute value of the least Integer", "IntegerFunctions::abs", []Value{constInt(math.MinInt64)}, semantics.ErrArithmeticOverflow},
		{"too few arguments", "RealFunctions::max", []Value{constReal(1)}, ErrCalcArity},
		{"too many arguments", "RealFunctions::sqrt", []Value{constReal(1), constReal(2)}, ErrCalcArity},
		{"no arguments", "RealFunctions::sqrt", nil, ErrCalcArity},
		{"boolean argument", "RealFunctions::sqrt", []Value{boolValue(true)}, ErrTypeMismatch},
		{"string argument", "RealFunctions::sqrt", []Value{NewStringValue("4")}, ErrTypeMismatch},
		{"Real argument to an Integer parameter", "IntegerFunctions::abs", []Value{constReal(1.5)}, ErrTypeMismatch},
		{"negative argument to a Natural parameter", "NaturalFunctions::max", []Value{constInt(-1), constInt(2)}, ErrTypeMismatch},
		{"logarithm of zero", "OpenSysMLMathFunctions::ln", []Value{constReal(0)}, semantics.ErrArithmeticDomain},
		{"logarithm of a negative", "OpenSysMLMathFunctions::ln", []Value{constReal(-1)}, semantics.ErrArithmeticDomain},
		{"logarithm to base 1", "OpenSysMLMathFunctions::log", []Value{constReal(8), constReal(1)}, semantics.ErrArithmeticDomain},
		{"logarithm of a negative to a base", "OpenSysMLMathFunctions::log", []Value{constReal(-1), constReal(10)}, semantics.ErrArithmeticDomain},
		{"logarithm of zero to a base", "OpenSysMLMathFunctions::log", []Value{constReal(0), constReal(10)}, semantics.ErrArithmeticDomain},
		{"logarithm to a negative base", "OpenSysMLMathFunctions::log", []Value{constReal(8), constReal(-2)}, semantics.ErrArithmeticDomain},
		{"logarithm to base zero", "OpenSysMLMathFunctions::log", []Value{constReal(8), constReal(0)}, semantics.ErrArithmeticDomain},
		{"angle at the origin", "OpenSysMLMathFunctions::atan2", []Value{constReal(0), constReal(0)}, semantics.ErrArithmeticDomain},
		{"exponential beyond the Real range", "OpenSysMLMathFunctions::exp", []Value{constReal(1000)}, semantics.ErrArithmeticOverflow},
		{"exponential with no argument", "OpenSysMLMathFunctions::exp", nil, ErrCalcArity},
		{"logarithm with one argument", "OpenSysMLMathFunctions::log", []Value{constReal(8)}, ErrCalcArity},
		{"angle with three arguments", "OpenSysMLMathFunctions::atan2", []Value{constReal(1), constReal(1), constReal(1)}, ErrCalcArity},
		{"boolean argument to the exponential", "OpenSysMLMathFunctions::exp", []Value{boolValue(true)}, ErrTypeMismatch},
		{"string argument to the logarithm", "OpenSysMLMathFunctions::ln", []Value{NewStringValue("1")}, ErrTypeMismatch},
		{"string base", "OpenSysMLMathFunctions::log", []Value{constReal(8), NewStringValue("2")}, ErrTypeMismatch},
		{"boolean argument to the angle", "OpenSysMLMathFunctions::atan2", []Value{constReal(1), boolValue(false)}, ErrTypeMismatch},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := applyLibrary(t, tc.fn, tc.args...)
			if !errors.Is(err, tc.want) {
				t.Fatalf("%s%v = %+v, %v; want error %v", tc.fn, tc.args, got, err, tc.want)
			}
		})
	}
}

// A named argument binds to the parameter name the vendored signature declares.
func TestLibraryFunctionNamedArguments(t *testing.T) {
	fn, _ := libraryFunctionByName("TrigFunctions::sin")
	got, err := fn.invoke(libCtx(t), calcArgs{named: map[string]Value{"theta": constReal(0)}})
	if err != nil || got.Const.Real != 0 {
		t.Fatalf("sin(theta = 0.0) = %+v, %v", got, err)
	}

	if _, err := fn.invoke(libCtx(t), calcArgs{named: map[string]Value{"x": constReal(0)}}); !errors.Is(err, ErrUnknownParameter) {
		t.Fatalf("sin(x = 0.0) error = %v, want %v", err, ErrUnknownParameter)
	}
}

// atan2 takes its arguments in the order y then x, so a named argument that
// names them the other way round still computes the angle to (x, y).
func TestLibraryFunctionAtan2NamedArguments(t *testing.T) {
	fn, _ := libraryFunctionByName("OpenSysMLMathFunctions::atan2")
	got, err := fn.invoke(libCtx(t), calcArgs{named: map[string]Value{"x": constReal(-1), "y": constReal(1)}})
	if err != nil || got.Const.Real != 3*math.Pi/4 {
		t.Fatalf("atan2(x = -1.0, y = 1.0) = %+v, %v; want 3pi/4", got, err)
	}
}

// The OpenSysML extension library ships the declarations these implementations
// are registered against, so the shipped signatures and the registry cannot
// drift: every function the file declares has an implementation whose parameters
// carry the declared names in the declared order.
func TestOpenSysMLMathFunctionsMatchTheShippedDeclarations(t *testing.T) {
	const path = "OpenSysML Libraries/OpenSysMLMathFunctions.kerml"
	data, err := libs.DefaultSource().Read(path)
	if err != nil {
		t.Fatalf("Read(%q): %v", path, err)
	}
	p := parser.New(source.New(path, data))
	file := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("%s has %d parse diagnostics, want 0: %v", path, len(p.Diagnostics), p.Diagnostics)
	}

	idx := symbols.NewIndex()
	idx.AddDocument(path, file)
	idx.MarkLibrary(path)
	resolver := resolve.New(idx)
	ctx := NewContext(NewModel(semantics.NewModel(resolver), resolver), 10000)

	declared := 0
	for _, sym := range idx.LookupDirectChildren("OpenSysMLMathFunctions") {
		if !isCalcSymbol(sym) {
			continue
		}
		declared++
		fqn := ctx.qualifiedSymbolName(sym)
		fn, ok := ctx.libraryFunctionFor(sym)
		if !ok {
			t.Errorf("%s is declared in %s but has no built-in implementation", fqn, path)
			continue
		}
		checkLibrarySignature(t, ctx, fqn, sym, fn)
	}
	if declared != 4 {
		t.Errorf("%s declares %d functions, want 4 (exp, ln, log, atan2)", path, declared)
	}
}

// An unqualified call denotes the library function of that name exactly where
// the validator resolves it to one: under an import of the package that declares
// it. Without the import the call fails with a typed error carrying the same
// hint the validator's diagnostic does — the qualified names the model may have
// meant, whose package an import would make visible — rather than being answered
// by a declaration the model never made visible.
func TestLibraryFunctionUnqualifiedNames(t *testing.T) {
	cases := []struct {
		name     string
		call     string
		imported string
		want     string
		hints    []string
	}{
		{"sqrt", "sqrt(x)", "RealFunctions", "2.0", []string{"RealFunctions::sqrt"}},
		{"abs", "abs(-x)", "RealFunctions", "4.0", []string{"RealFunctions::abs", "IntegerFunctions::abs"}},
		{"floor", "floor(x)", "RealFunctions", "4", []string{"RealFunctions::floor"}},
		{"sin", "sin(0.0 * x)", "TrigFunctions", "0.0", []string{"TrigFunctions::sin"}},
		{"exp", "exp(0.0 * x)", "OpenSysMLMathFunctions", "1.0", []string{"OpenSysMLMathFunctions::exp"}},
		{"size", "(x, x)->size()", "SequenceFunctions", "2", []string{"SequenceFunctions::size", "CollectionFunctions::size"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model := func(imports string) string {
				return "package test {\n" + imports + "\n\tcalc f { in x : Real; " + tc.call + " }\n}"
			}

			ctx, idx := libraryModelContext(t, model(""))
			sym := lookupOne(t, idx, "test::f")
			_, err := ctx.InvokeCalc(sym, []Value{constReal(4)}, nil)
			if !errors.Is(err, ErrUnresolvedReference) {
				t.Fatalf("%s without an import: error = %v, want %v", tc.call, err, ErrUnresolvedReference)
			}
			if !strings.Contains(err.Error(), ": unresolved reference: "+tc.name+" — did you mean ") {
				t.Errorf("error %q does not read as the validator's diagnostic", err)
			}
			for _, hint := range tc.hints {
				if !strings.Contains(err.Error(), hint) {
					t.Errorf("error %q does not offer %s", err, hint)
				}
			}

			ctx, idx = libraryModelContext(t, model("\tprivate import "+tc.imported+"::*;"))
			sym = lookupOne(t, idx, "test::f")
			got, err := ctx.InvokeCalc(sym, []Value{constReal(4)}, nil)
			if err != nil {
				t.Fatalf("%s under import %s::*: %v", tc.call, tc.imported, err)
			}
			if FormatValue(got) != tc.want {
				t.Errorf("%s under import %s::* = %s, want %s", tc.call, tc.imported, FormatValue(got), tc.want)
			}
		})
	}
}

// A qualified name reaches a library function whatever the model imports: the
// library packages are members of the root namespace, so the qualified spelling
// is the model's own reading of the declaration.
func TestLibraryFunctionQualifiedCallNeedsNoImport(t *testing.T) {
	ctx, idx := libraryModelContext(t, `package test {
	calc f { in x : Real; OpenSysMLMathFunctions::exp(0.0 * x) + RealFunctions::sqrt(x) + (x, x)->SequenceFunctions::size() }
}`)
	got, err := ctx.InvokeCalc(lookupOne(t, idx, "test::f"), []Value{constReal(4)}, nil)
	if err != nil || FormatValue(got) != "5.0" {
		t.Fatalf("qualified calls = %s, %v; want 5.0", FormatValue(got), err)
	}
}

// A call that resolves to the library's declaration is applied by the built-in
// implementation, because the library gives that declaration no body.
func TestLibraryFunctionDispatchByResolvedSymbol(t *testing.T) {
	ctx, idx := libraryModelContext(t, `package test {}`)
	sym := lookupOne(t, idx, "RealFunctions::sqrt")

	if _, ok := ctx.libraryFunctionFor(sym); !ok {
		t.Fatalf("RealFunctions::sqrt did not dispatch to its built-in implementation")
	}
	got, err := ctx.InvokeCalc(sym, []Value{constReal(25)}, nil)
	if err != nil || got.Const.Real != 5 {
		t.Fatalf("InvokeCalc(sqrt, 25.0) = %+v, %v", got, err)
	}
}

// A declaration the model makes under a library function's qualified name is
// the model's own, so it is never answered by the built-in: without a body it
// has no result, whether the library is loaded beside it or not.
func TestLibraryFunctionNeverAnswersAModelDeclaration(t *testing.T) {
	const src = `package RealFunctions {
	function sqrt { in x : Real[1]; return : Real[1]; }
}`
	for name, build := range map[string]func(*testing.T, string) (*Context, *symbols.Index){
		"alone": contextForSource, "beside the library": libraryModelContext,
	} {
		ctx, idx := build(t, src)
		var sym *symbols.Symbol
		for _, s := range idx.LookupQualified("RealFunctions::sqrt") {
			if !ctx.libraryDeclared(s) {
				sym = s
			}
		}
		if sym == nil {
			t.Fatalf("%s: the model's RealFunctions::sqrt was not indexed", name)
		}
		if _, ok := ctx.libraryFunctionFor(sym); ok {
			t.Errorf("%s: the model's RealFunctions::sqrt dispatched to the built-in", name)
		}
		if _, err := ctx.InvokeCalc(sym, []Value{constReal(25)}, nil); !errors.Is(err, ErrNoResultExpression) {
			t.Errorf("%s: InvokeCalc(model sqrt, 25.0) = %v, want %v", name, err, ErrNoResultExpression)
		}
	}
}

// A declaration that carries a body is evaluated from that body, so a model's
// own calc is never hijacked by a built-in of the same name.
func TestLibraryFunctionDoesNotHijackADeclaredBody(t *testing.T) {
	ctx, idx := contextForSource(t, `package RealFunctions {
	calc sqrt { in x : Real; return : Real = x; }
}
package mine {
	calc sqrt { in x : Real; return : Real = 42.0; }
}`)

	libSym := lookupOne(t, idx, "RealFunctions::sqrt")
	if _, ok := ctx.libraryFunctionFor(libSym); ok {
		t.Fatalf("a declaration with a body dispatched to the built-in implementation")
	}
	got, err := ctx.InvokeCalc(libSym, []Value{constReal(25)}, nil)
	if err != nil || got.Const.Real != 25 {
		t.Fatalf("InvokeCalc(RealFunctions::sqrt, 25.0) = %+v, %v; want the declared body", got, err)
	}

	ownSym := lookupOne(t, idx, "mine::sqrt")
	got, err = ctx.InvokeCalc(ownSym, []Value{constReal(25)}, nil)
	if err != nil || got.Const.Real != 42 {
		t.Fatalf("InvokeCalc(mine::sqrt, 25.0) = %+v, %v; want the declared body", got, err)
	}
}

// A calc computing its result by assigning its output states a body too, so the
// built-in of the same name does not answer in its place.
func TestLibraryFunctionDoesNotHijackAnOutputAssignedInABody(t *testing.T) {
	ctx, idx := contextForSource(t, `package RealFunctions {
	calc sqrt { in x : Real; out r : Real; assign r := 42.0; }
}`)

	sym := lookupOne(t, idx, "RealFunctions::sqrt")
	if _, ok := ctx.libraryFunctionFor(sym); ok {
		t.Fatalf("a declaration assigning its output dispatched to the built-in implementation")
	}
	got, err := ctx.InvokeCalc(sym, []Value{constReal(25)}, nil)
	if err != nil || got.Const.Real != 42 {
		t.Fatalf("InvokeCalc(RealFunctions::sqrt, 25.0) = %+v, %v; want the declared body", got, err)
	}
}

// A library function is implemented by its built-in even where the library
// declares a body for it, which library content does on every load path.
func TestLibraryFunctionDispatchByLibraryDeclaration(t *testing.T) {
	ctx, idx := libraryContextForSource(t, `package RealFunctions {
	calc sqrt { in x : Real; return : Real = 42.0; }
	attribute tolerance = 0.5;
}`)

	fnSym := lookupOne(t, idx, "RealFunctions::sqrt")
	if _, ok := ctx.libraryFunctionFor(fnSym); !ok {
		t.Fatalf("the library RealFunctions::sqrt did not dispatch to its built-in implementation")
	}
	got, err := ctx.InvokeCalc(fnSym, []Value{constReal(25)}, nil)
	if err != nil || got.Const.Real != 5 {
		t.Fatalf("InvokeCalc(RealFunctions::sqrt, 25.0) = %+v, %v; want the built-in", got, err)
	}
	attrSym := lookupOne(t, idx, "RealFunctions::tolerance")
	if _, ok := ctx.libraryFunctionFor(attrSym); ok {
		t.Fatalf("a library attribute dispatched to a built-in implementation")
	}
}

// A declaration that is not a calc keeps the not-a-calc diagnostic, however it
// is named: only a function is a library function.
func TestLibraryFunctionDoesNotClaimANonCalcDeclaration(t *testing.T) {
	ctx, idx := contextForSource(t, `package RealFunctions {
	attribute sqrt = 3.0;
}`)
	sym := lookupOne(t, idx, "RealFunctions::sqrt")

	if _, ok := ctx.libraryFunctionFor(sym); ok {
		t.Fatalf("an attribute named sqrt dispatched to the built-in implementation")
	}
	if _, err := ctx.InvokeCalc(sym, []Value{constReal(2)}, nil); !errors.Is(err, ErrNotACalc) {
		t.Fatalf("InvokeCalc(attribute sqrt) error = %v; want ErrNotACalc", err)
	}
}

// libraryModelContext indexes src as a model over the standard library and
// returns a runtime context over it.
func libraryModelContext(t *testing.T, src string) (*Context, *symbols.Index) {
	t.Helper()
	file := parser.New(source.New("<test>", []byte(src))).ParseFile()
	idx := libs.NewModelIndex()
	idx.AddDocument("<test>", file)
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	return NewContext(NewModel(semantics.NewModel(resolver), resolver), 10000), idx
}

// contextForSource indexes src as one document and returns a runtime context
// over it.
func contextForSource(t *testing.T, src string) (*Context, *symbols.Index) {
	t.Helper()
	file := parser.New(source.New("<test>", []byte(src))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("<test>", file)
	resolver := resolve.New(idx)
	return NewContext(NewModel(semantics.NewModel(resolver), resolver), 10000), idx
}

// lookupOne returns the single symbol with that fully-qualified name.
func lookupOne(t *testing.T, idx *symbols.Index, fqn string) *symbols.Symbol {
	t.Helper()
	syms := idx.LookupQualified(fqn)
	if len(syms) != 1 {
		t.Fatalf("LookupQualified(%q) returned %d symbols, want 1", fqn, len(syms))
	}
	return syms[0]
}

// vectorValues is the elements of a vector result, which is a vector value.
func vectorValues(t *testing.T, val Value) []semantics.Value {
	t.Helper()
	if val.Kind != ValVector {
		t.Fatalf("result is %s, want a vector", val.Kind)
	}
	return val.Vector().Elements
}

func realConsts(reals ...float64) []semantics.Value {
	out := make([]semantics.Value, len(reals))
	for i, x := range reals {
		out[i] = semantics.Value{Kind: semantics.ValReal, Real: x}
	}
	return out
}

func intConsts(ints ...int64) []semantics.Value {
	out := make([]semantics.Value, len(ints))
	for i, n := range ints {
		out[i] = semantics.Value{Kind: semantics.ValInt, Int: n}
	}
	return out
}

// vec is a vector argument: the sequence of its elements.
func vec(elements ...Value) Value { return sequenceOf(elements) }

func realVec(reals ...float64) Value {
	elements := make([]Value, len(reals))
	for i, x := range reals {
		elements[i] = constReal(x)
	}
	return vec(elements...)
}

// A vector is the sequence of its elements, and the operations VectorFunctions
// declares abstractly over VectorValue compute for every vector this runtime
// represents, so the abstract name and its Cartesian specialization agree.
func TestVectorFunctionValues(t *testing.T) {
	cases := []struct {
		fn   string
		args []Value
		want []semantics.Value
	}{
		{"VectorFunctions::VectorOf", []Value{vec(constInt(1), constInt(2))}, intConsts(1, 2)},
		{"VectorFunctions::VectorOf", []Value{constInt(3)}, intConsts(3)},
		{"VectorFunctions::CartesianVectorOf", []Value{vec(constInt(1), constReal(2.5))}, realConsts(1, 2.5)},
		{"VectorFunctions::CartesianVectorOf", []Value{nullValue()}, nil},
		{"VectorFunctions::CartesianThreeVectorOf", []Value{realVec(1, 2, 3)}, realConsts(1, 2, 3)},
		// '+' and '-' keep the elements' kind, as the declaration over
		// NumericalValue does; the Cartesian specializations are Real.
		{"VectorFunctions::+", []Value{vec(constInt(1), constInt(2)), vec(constInt(3), constInt(4))}, intConsts(4, 6)},
		{"VectorFunctions::cartesian+", []Value{realVec(1, 2), realVec(3, 4)}, realConsts(4, 6)},
		{"VectorFunctions::-", []Value{vec(constInt(1), constInt(2)), vec(constInt(4), constInt(4))}, intConsts(-3, -2)},
		{"VectorFunctions::cartesian-", []Value{realVec(1, 2), realVec(0.5, 0.5)}, realConsts(0.5, 1.5)},
		{"VectorFunctions::scalarVectorMult", []Value{constInt(2), vec(constInt(1), constInt(2))}, intConsts(2, 4)},
		{"VectorFunctions::*", []Value{constReal(0.5), realVec(1, 2)}, realConsts(0.5, 1)},
		{"VectorFunctions::vectorScalarMult", []Value{realVec(1, 2), constReal(3)}, realConsts(3, 6)},
		{"VectorFunctions::cartesianVectorScalarMult", []Value{realVec(1, 2), constReal(3)}, realConsts(3, 6)},
		// The library defines the quotient as the product with 1.0 / x, whose
		// rounding a division does not carry: 1.0 / 3.0 * 3.0 is 0.9999999999999998.
		{"VectorFunctions::vectorScalarDiv", []Value{realVec(3, 6), constReal(3)}, realConsts(1, 2)},
	}

	for _, tc := range cases {
		t.Run(tc.fn, func(t *testing.T) {
			got, err := applyLibrary(t, tc.fn, tc.args...)
			if err != nil {
				t.Fatalf("%s = error %v", tc.fn, err)
			}
			elements := vectorValues(t, got)
			if len(elements) != len(tc.want) {
				t.Fatalf("%s = %v, want %v", tc.fn, elements, tc.want)
			}
			for i := range elements {
				if elements[i] != tc.want[i] {
					t.Fatalf("%s = %v, want %v", tc.fn, elements, tc.want)
				}
			}
		})
	}
}

// The scalar-valued vector operations: the inner product keeps the elements'
// kind, the norm and the angle are Reals.
func TestVectorFunctionScalarValues(t *testing.T) {
	cases := []struct {
		fn   string
		args []Value
		want semantics.Value
	}{
		{"VectorFunctions::inner", []Value{vec(constInt(1), constInt(2)), vec(constInt(3), constInt(4))}, semantics.Value{Kind: semantics.ValInt, Int: 11}},
		{"VectorFunctions::cartesianInner", []Value{realVec(1, 2), realVec(3, 4)}, semantics.Value{Kind: semantics.ValReal, Real: 11}},
		{"VectorFunctions::norm", []Value{realVec(3, 4)}, semantics.Value{Kind: semantics.ValReal, Real: 5}},
		{"VectorFunctions::cartesianNorm", []Value{realVec(3, 4)}, semantics.Value{Kind: semantics.ValReal, Real: 5}},
		{"VectorFunctions::angle", []Value{realVec(1, 0), realVec(0, 1)}, semantics.Value{Kind: semantics.ValReal, Real: math.Pi / 2}},
		// Two parallel vectors: the cosine rounds to just above 1.0, where the
		// arc cosine has no value, and the angle is 0.
		{"VectorFunctions::cartesianAngle", []Value{realVec(1, 2, 3), realVec(2, 4, 6)}, semantics.Value{Kind: semantics.ValReal, Real: 0}},
		{"VectorFunctions::isZeroVector", []Value{realVec(0, 0)}, semantics.Value{Kind: semantics.ValBool, Bool: true}},
		{"VectorFunctions::isZeroVector", []Value{realVec(0, 1)}, semantics.Value{Kind: semantics.ValBool, Bool: false}},
		{"VectorFunctions::isCartesianZeroVector", []Value{realVec(0, 0, 0)}, semantics.Value{Kind: semantics.ValBool, Bool: true}},
	}

	for _, tc := range cases {
		t.Run(tc.fn, func(t *testing.T) {
			got, err := applyLibrary(t, tc.fn, tc.args...)
			if err != nil {
				t.Fatalf("%s = error %v", tc.fn, err)
			}
			if got.Kind != ValConst || got.Const != tc.want {
				t.Fatalf("%s = %+v, want %+v", tc.fn, got, tc.want)
			}
		})
	}
}

// A norm is finite whenever it is in the Real range, even where the squares of
// the components are not, and the angle is defined for any two non-zero vectors
// of finite components, even ones whose norms leave the Real range.
func TestVectorNormAndAngleOfLargeComponents(t *testing.T) {
	norm, err := applyLibrary(t, "VectorFunctions::norm", realVec(3e200, 4e200))
	if err != nil {
		t.Fatalf("norm = error %v", err)
	}
	if got := norm.Const.Real; math.Abs(got-5e200) > 1e185 {
		t.Fatalf("norm = %g, want 5e200", got)
	}
	cases := []struct {
		name string
		v, w Value
		want float64
	}{
		{"orthogonal", realVec(1e200, 0), realVec(0, 1e200), math.Pi / 2},
		{"parallel", realVec(1e200, 2e200), realVec(2e200, 4e200), 0},
		{"opposite", realVec(1e200, 0), realVec(-1e200, 0), math.Pi},
		{"parallel, norms beyond the Real range", realVec(1.5e308, 1.5e308), realVec(1e308, 1e308), 0},
		{"orthogonal, norms beyond the Real range", realVec(1.5e308, 1.5e308), realVec(1.5e308, -1.5e308), math.Pi / 2},
		{"opposite, norms beyond the Real range", realVec(1.5e308, 1.5e308), realVec(-1.5e308, -1.5e308), math.Pi},
		{"one norm beyond the Real range", realVec(1.5e308, 1.5e308), realVec(1, 1), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := applyLibrary(t, "VectorFunctions::angle", tc.v, tc.w)
			if err != nil {
				t.Fatalf("angle = error %v", err)
			}
			// The arc cosine magnifies a rounding of the cosine near ±1 to ~1e-8.
			if got.Kind != ValConst || math.Abs(got.Const.Real-tc.want) > 1e-7 {
				t.Fatalf("angle = %s, want %g", FormatValue(got), tc.want)
			}
		})
	}
}

// cx is a Complex runtime value with the given parts.
func cx(re, im float64) Value { return NewComplex(complex(re, im)) }

// complexValue reads a result as the one Complex value it must be.
func complexValue(t *testing.T, val Value) complex128 {
	t.Helper()
	if val.Kind != ValComplex {
		t.Fatalf("result is %s (%s), want a Complex", val.Kind, FormatValue(val))
	}
	return val.Complex()
}

// A Complex is one ValComplex value, and a Real is a Complex with a zero
// imaginary part (ScalarValues declares Real :> Complex), so both bind to a
// Complex parameter.
func TestComplexFunctionValues(t *testing.T) {
	cases := []struct {
		fn   string
		args []Value
		want complex128
	}{
		{"ComplexFunctions::rect", []Value{constReal(1), constReal(2)}, complex(1, 2)},
		{"ComplexFunctions::rect", []Value{constInt(1), constInt(2)}, complex(1, 2)},
		{"ComplexFunctions::polar", []Value{constReal(2), constReal(0)}, complex(2, 0)},
		{"ComplexFunctions::+", []Value{cx(1, 2), cx(3, 4)}, complex(4, 6)},
		{"ComplexFunctions::+", []Value{cx(1, 2), constReal(3)}, complex(4, 2)},
		{"ComplexFunctions::-", []Value{cx(1, 2), cx(3, 4)}, complex(-2, -2)},
		{"ComplexFunctions::*", []Value{cx(0, 1), cx(0, 1)}, complex(-1, 0)},
		{"ComplexFunctions::*", []Value{cx(1, 2), constReal(2)}, complex(2, 4)},
		{"ComplexFunctions::/", []Value{cx(-1, 0), cx(0, 1)}, complex(0, 1)},
		{"ComplexFunctions::**", []Value{cx(0, 1), constInt(2)}, complex(-1, 0)},
		{"ComplexFunctions::^", []Value{constReal(2), constInt(3)}, complex(8, 0)},
		{"ComplexFunctions::sum", []Value{vec(cx(1, 2), cx(3, 4), constReal(1))}, complex(5, 6)},
		{"ComplexFunctions::sum", []Value{vec()}, 0},
		{"ComplexFunctions::product", []Value{vec(cx(0, 1), cx(0, 1))}, complex(-1, 0)},
		{"ComplexFunctions::product", []Value{nullValue()}, 1},
	}

	for _, tc := range cases {
		t.Run(tc.fn, func(t *testing.T) {
			got, err := applyLibrary(t, tc.fn, tc.args...)
			if err != nil {
				t.Fatalf("%s = error %v", tc.fn, err)
			}
			z := complexValue(t, got)
			// The products and quotients of a Complex round, so compare the parts
			// as Reals rather than bit for bit.
			if math.Abs(real(z)-real(tc.want)) > 1e-12 || math.Abs(imag(z)-imag(tc.want)) > 1e-12 {
				t.Fatalf("%s = %s, want %s", tc.fn, FormatValue(got), FormatComplex(tc.want))
			}
		})
	}
}

func TestComplexFunctionScalarValues(t *testing.T) {
	cases := []struct {
		fn   string
		args []Value
		want semantics.Value
	}{
		{"ComplexFunctions::re", []Value{cx(1, 2)}, semantics.Value{Kind: semantics.ValReal, Real: 1}},
		{"ComplexFunctions::im", []Value{cx(1, 2)}, semantics.Value{Kind: semantics.ValReal, Real: 2}},
		{"ComplexFunctions::im", []Value{constReal(1)}, semantics.Value{Kind: semantics.ValReal, Real: 0}},
		{"ComplexFunctions::abs", []Value{cx(3, 4)}, semantics.Value{Kind: semantics.ValReal, Real: 5}},
		{"ComplexFunctions::arg", []Value{cx(0, 1)}, semantics.Value{Kind: semantics.ValReal, Real: math.Pi / 2}},
		{"ComplexFunctions::isZero", []Value{cx(0, 0)}, semantics.Value{Kind: semantics.ValBool, Bool: true}},
		{"ComplexFunctions::isZero", []Value{constInt(0)}, semantics.Value{Kind: semantics.ValBool, Bool: true}},
		{"ComplexFunctions::isUnit", []Value{cx(1, 0)}, semantics.Value{Kind: semantics.ValBool, Bool: true}},
		{"ComplexFunctions::isUnit", []Value{cx(1, 1)}, semantics.Value{Kind: semantics.ValBool, Bool: false}},
		// A Real and the Complex with a zero imaginary part are the same number.
		{"ComplexFunctions::==", []Value{constReal(2), cx(2, 0)}, semantics.Value{Kind: semantics.ValBool, Bool: true}},
		{"ComplexFunctions::==", []Value{cx(2, 1), cx(2, 0)}, semantics.Value{Kind: semantics.ValBool, Bool: false}},
		// Both operands are declared [0..1]: two empty operands are equal.
		{"ComplexFunctions::==", []Value{nullValue(), nullValue()}, semantics.Value{Kind: semantics.ValBool, Bool: true}},
		{"ComplexFunctions::==", []Value{constReal(0), nullValue()}, semantics.Value{Kind: semantics.ValBool, Bool: false}},
	}

	for _, tc := range cases {
		t.Run(tc.fn, func(t *testing.T) {
			got, err := applyLibrary(t, tc.fn, tc.args...)
			if err != nil {
				t.Fatalf("%s = error %v", tc.fn, err)
			}
			if got.Kind != ValConst || got.Const != tc.want {
				t.Fatalf("%s = %+v, want %+v", tc.fn, got, tc.want)
			}
		})
	}
}

// The library's `reduce '+'` over Reals stays on the real axis, so the Complex
// aggregations keep the elements' kind unless one is a Complex.
func TestComplexAggregationsKeepRealElementsReal(t *testing.T) {
	cases := []struct {
		fn   string
		args []Value
		want semantics.Value
	}{
		{"ComplexFunctions::sum", []Value{vec(constReal(1), constReal(2))}, semantics.Value{Kind: semantics.ValReal, Real: 3}},
		{"ComplexFunctions::sum", []Value{vec(constInt(1), constInt(2))}, semantics.Value{Kind: semantics.ValInt, Int: 3}},
		{"ComplexFunctions::product", []Value{vec(constInt(2), constReal(1.5))}, semantics.Value{Kind: semantics.ValReal, Real: 3}},
		{"ComplexFunctions::product", []Value{constInt(4)}, semantics.Value{Kind: semantics.ValInt, Int: 4}},
	}
	for _, tc := range cases {
		t.Run(tc.fn, func(t *testing.T) {
			got, err := applyLibrary(t, tc.fn, tc.args...)
			if err != nil {
				t.Fatalf("%s = error %v", tc.fn, err)
			}
			if got.Kind != ValConst || got.Const != tc.want {
				t.Fatalf("%s = %s, want %+v", tc.fn, FormatValue(got), tc.want)
			}
		})
	}
}

// A numeric pair is a vector, not a Complex: only rect, polar, i and the
// Complex operations make one, so a pair never binds to a Complex parameter.
func TestComplexFunctionsRejectNumericPairs(t *testing.T) {
	for _, fn := range []string{
		"ComplexFunctions::re", "ComplexFunctions::im", "ComplexFunctions::abs",
		"ComplexFunctions::arg", "ComplexFunctions::isZero", "ComplexFunctions::isUnit",
	} {
		got, err := applyLibrary(t, fn, realVec(1, 2))
		if !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("%s((1.0, 2.0)) = (%v, %v), want %v", fn, got, err, ErrTypeMismatch)
		}
	}
	for _, fn := range []string{"ComplexFunctions::+", "ComplexFunctions::*", "ComplexFunctions::=="} {
		got, err := applyLibrary(t, fn, cx(1, 2), realVec(1, 2))
		if !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("%s(1.0 + 2.0i, (1.0, 2.0)) = (%v, %v), want %v", fn, got, err, ErrTypeMismatch)
		}
	}
	// A one-element collection is the scalar it holds, here a Real on the real axis.
	got, err := applyLibrary(t, "ComplexFunctions::re", realVec(1))
	if err != nil || got.Kind != ValConst || !got.Const.IsNumeric() || got.Const.Real != 1 {
		t.Errorf("re((1.0)) = (%v, %v), want 1.0", got, err)
	}
}

// TrigFunctions::deg and ::rad convert with the pi the feature seam supplies,
// which is what their library bodies read.
func TestTrigDegreesAndRadians(t *testing.T) {
	got, err := applyLibrary(t, "TrigFunctions::deg", constReal(math.Pi))
	if err != nil || got.Const.Real != 180 {
		t.Fatalf("deg(pi) = %+v, %v; want 180.0", got, err)
	}
	got, err = applyLibrary(t, "TrigFunctions::rad", constReal(180))
	if err != nil || got.Const.Real != math.Pi {
		t.Fatalf("rad(180.0) = %+v, %v; want pi", got, err)
	}
	got, err = applyLibrary(t, "TrigFunctions::rad", constInt(0))
	if err != nil || got.Const.Real != 0 {
		t.Fatalf("rad(0) = %+v, %v; want 0.0", got, err)
	}
}

// The library declares the second operand of '+' and '-' [0..1]: with one
// argument, '+' is that value and '-' is its inverse.
func TestLibraryFunctionOptionalOperand(t *testing.T) {
	got, err := applyLibrary(t, "VectorFunctions::cartesian+", realVec(1, 2))
	if err != nil {
		t.Fatalf("cartesian+((1.0, 2.0)) = error %v", err)
	}
	if elements := vectorValues(t, got); len(elements) != 2 || elements[0].Real != 1 || elements[1].Real != 2 {
		t.Fatalf("cartesian+((1.0, 2.0)) = %v, want (1.0, 2.0)", got)
	}

	got, err = applyLibrary(t, "VectorFunctions::-", vec(constInt(1), constInt(-2)))
	if err != nil {
		t.Fatalf("-((1, -2)) = error %v", err)
	}
	if elements := vectorValues(t, got); len(elements) != 2 || elements[0].Int != -1 || elements[1].Int != 2 {
		t.Fatalf("-((1, -2)) = %v, want (-1, 2)", got)
	}

	got, err = applyLibrary(t, "ComplexFunctions::-", cx(1, 2))
	if err != nil {
		t.Fatalf("-(1.0 + 2.0i) = error %v", err)
	}
	if complexValue(t, got) != complex(-1, -2) {
		t.Fatalf("-(1.0 + 2.0i) = %v, want -1.0 - 2.0i", got)
	}

	// A required operand is still required: '*' declares both [1].
	if _, err := applyLibrary(t, "ComplexFunctions::*", cx(1, 2)); !errors.Is(err, ErrCalcArity) {
		t.Fatalf("*(1.0 + 2.0i) error = %v, want %v", err, ErrCalcArity)
	}
}

// An empty collection written for the optional operand is the same no value as
// null, so the call answers as the one-argument form rather than reporting.
func TestLibraryFunctionEmptyOptionalOperand(t *testing.T) {
	got, err := applyLibrary(t, "VectorFunctions::cartesian+", realVec(1, 2), vec())
	if err != nil {
		t.Fatalf("cartesian+((1.0, 2.0), ()) = error %v", err)
	}
	if elements := vectorValues(t, got); len(elements) != 2 || elements[0].Real != 1 || elements[1].Real != 2 {
		t.Fatalf("cartesian+((1.0, 2.0), ()) = %v, want (1.0, 2.0)", got)
	}

	got, err = applyLibrary(t, "VectorFunctions::cartesian-", realVec(1, 2), vec())
	if err != nil {
		t.Fatalf("cartesian-((1.0, 2.0), ()) = error %v", err)
	}
	if elements := vectorValues(t, got); len(elements) != 2 || elements[0].Real != -1 || elements[1].Real != -2 {
		t.Fatalf("cartesian-((1.0, 2.0), ()) = %v, want (-1.0, -2.0)", got)
	}

	got, err = applyLibrary(t, "ComplexFunctions::+", cx(1, 2), vec())
	if err != nil {
		t.Fatalf("+(1.0 + 2.0i, ()) = error %v", err)
	}
	if complexValue(t, got) != complex(1, 2) {
		t.Fatalf("+(1.0 + 2.0i, ()) = %v, want 1.0 + 2.0i", got)
	}

	for _, tc := range []struct {
		args []Value
		want bool
	}{
		{[]Value{vec(), vec()}, true},
		{[]Value{vec(), nullValue()}, true},
		{[]Value{cx(1, 2), vec()}, false},
	} {
		got, err := applyLibrary(t, "ComplexFunctions::==", tc.args...)
		if err != nil || got.Kind != ValConst || got.Const.Bool != tc.want {
			t.Errorf("==(%v) = (%v, %v), want %v", tc.args, got, err, tc.want)
		}
	}
}

// A named argument binds to the parameter name the vendored vector and Complex
// signatures declare, whichever order the call names them in.
func TestVectorAndComplexNamedArguments(t *testing.T) {
	fn, _ := libraryFunctionByName("VectorFunctions::scalarVectorMult")
	got, err := fn.invoke(libCtx(t), calcArgs{named: map[string]Value{"v": realVec(1, 2), "x": constReal(3)}})
	if err != nil {
		t.Fatalf("scalarVectorMult(v = (1.0, 2.0), x = 3.0) = error %v", err)
	}
	if elements := vectorValues(t, got); len(elements) != 2 || elements[0].Real != 3 || elements[1].Real != 6 {
		t.Fatalf("scalarVectorMult(v = (1.0, 2.0), x = 3.0) = %v, want (3.0, 6.0)", got)
	}

	fn, _ = libraryFunctionByName("ComplexFunctions::rect")
	got, err = fn.invoke(libCtx(t), calcArgs{named: map[string]Value{"im": constReal(2), "re": constReal(1)}})
	if err != nil {
		t.Fatalf("rect(im = 2.0, re = 1.0) = error %v", err)
	}
	if complexValue(t, got) != complex(1, 2) {
		t.Fatalf("rect(im = 2.0, re = 1.0) = %v, want 1.0 + 2.0i", got)
	}

	// An omitted optional parameter binds empty, so naming only `v` is a call
	// with one argument rather than an unknown-parameter error.
	fn, _ = libraryFunctionByName("VectorFunctions::+")
	if _, err := fn.invoke(libCtx(t), calcArgs{named: map[string]Value{"v": realVec(1, 2)}}); err != nil {
		t.Fatalf("+(v = (1.0, 2.0)) = error %v", err)
	}
	if _, err := fn.invoke(libCtx(t), calcArgs{named: map[string]Value{"u": realVec(1, 2)}}); !errors.Is(err, ErrUnknownParameter) {
		t.Fatalf("+(u = (1.0, 2.0)) error = %v, want %v", err, ErrUnknownParameter)
	}
}

// A name no parameter of the signature carries is reported, rather than absorbed
// by an optional parameter the call then leaves empty — which would answer the
// call as if the argument had not been written.
func TestVectorAndComplexUnknownNamedArgument(t *testing.T) {
	for _, tt := range []struct {
		fn    string
		named map[string]Value
	}{
		{"VectorFunctions::+", map[string]Value{"v": realVec(1, 2), "zz": realVec(3, 4)}},
		{"VectorFunctions::cartesian+", map[string]Value{"v": realVec(1, 2), "zz": realVec(3, 4)}},
		{"VectorFunctions::-", map[string]Value{"v": realVec(1, 2), "zz": realVec(3, 4)}},
		{"ComplexFunctions::+", map[string]Value{"x": cx(1, 2), "zz": cx(3, 4)}},
		{"ComplexFunctions::-", map[string]Value{"x": cx(1, 2), "zz": cx(3, 4)}},
		{"ComplexFunctions::==", map[string]Value{"zz": constReal(1)}},
		{"ComplexFunctions::==", map[string]Value{"x": cx(1, 2), "zz": cx(1, 2)}},
	} {
		fn, ok := libraryFunctionByName(tt.fn)
		if !ok {
			t.Fatalf("%s is not registered", tt.fn)
		}
		got, err := fn.invoke(libCtx(t), calcArgs{named: tt.named})
		if !errors.Is(err, ErrUnknownParameter) {
			t.Errorf("%s with an unknown name = (%v, %v), want %v", tt.fn, got, err, ErrUnknownParameter)
		}
	}
}

func TestVectorAndComplexFunctionErrors(t *testing.T) {
	cases := []struct {
		name string
		fn   string
		args []Value
		want error
	}{
		{"vectors of unequal dimension", "VectorFunctions::cartesian+", []Value{realVec(1, 2), realVec(1, 2, 3)}, ErrTypeMismatch},
		{"inner product of unequal dimensions", "VectorFunctions::inner", []Value{realVec(1), realVec(1, 2)}, ErrTypeMismatch},
		{"angle of unequal dimensions", "VectorFunctions::angle", []Value{realVec(1), realVec(1, 2)}, ErrTypeMismatch},
		{"a string element", "VectorFunctions::norm", []Value{vec(constReal(1), NewStringValue("2"))}, ErrTypeMismatch},
		{"a boolean element", "VectorFunctions::isZeroVector", []Value{vec(boolValue(true))}, ErrTypeMismatch},
		{"a vector where a scalar is declared", "VectorFunctions::scalarVectorMult", []Value{realVec(1, 2), realVec(1, 2)}, ErrTypeMismatch},
		{"no components", "VectorFunctions::VectorOf", []Value{nullValue()}, ErrMultiplicityViolation},
		{"two components of a three-vector", "VectorFunctions::CartesianThreeVectorOf", []Value{realVec(1, 2)}, ErrMultiplicityViolation},
		{"division of a vector by zero", "VectorFunctions::vectorScalarDiv", []Value{realVec(1, 2), constReal(0)}, ErrDivisionByZero},
		{"the angle to a zero vector", "VectorFunctions::angle", []Value{realVec(0, 0), realVec(1, 0)}, semantics.ErrArithmeticDomain},
		{"a norm beyond the Real range", "VectorFunctions::norm", []Value{realVec(1.5e308, 1.5e308)}, semantics.ErrArithmeticOverflow},
		{"a sum beyond the Real range", "VectorFunctions::cartesian+", []Value{realVec(1e308, 1), realVec(1e308, 1)}, semantics.ErrArithmeticOverflow},
		{"a difference beyond the Real range", "VectorFunctions::cartesian-", []Value{realVec(1e308, 1), realVec(-1e308, 1)}, semantics.ErrArithmeticOverflow},
		{"a scaled element beyond the Real range", "VectorFunctions::scalarVectorMult", []Value{constReal(1e300), realVec(1e300)}, semantics.ErrArithmeticOverflow},
		{"an inner product beyond the Real range", "VectorFunctions::cartesianInner", []Value{realVec(1e200), realVec(1e200)}, semantics.ErrArithmeticOverflow},
		{"an Integer sum beyond the Integer range", "VectorFunctions::+", []Value{vec(constInt(math.MaxInt64)), vec(constInt(1))}, semantics.ErrArithmeticOverflow},
		{"an Integer difference beyond the Integer range", "VectorFunctions::-", []Value{vec(constInt(math.MinInt64)), vec(constInt(1))}, semantics.ErrArithmeticOverflow},
		{"the negation of the least Integer", "VectorFunctions::-", []Value{vec(constInt(math.MinInt64))}, semantics.ErrArithmeticOverflow},
		{"an Integer scaling beyond the Integer range", "VectorFunctions::scalarVectorMult", []Value{constInt(2), vec(constInt(math.MaxInt64))}, semantics.ErrArithmeticOverflow},
		{"an Integer inner product beyond the Integer range", "VectorFunctions::inner", []Value{vec(constInt(math.MaxInt64), constInt(1)), vec(constInt(1), constInt(1))}, semantics.ErrArithmeticOverflow},
		{"a vector where a Complex is declared", "ComplexFunctions::re", []Value{realVec(1, 2, 3)}, ErrTypeMismatch},
		{"an empty Complex", "ComplexFunctions::abs", []Value{nullValue()}, ErrTypeMismatch},
		{"a string where a Complex is declared", "ComplexFunctions::im", []Value{strValue("2")}, ErrTypeMismatch},
		{"a Boolean where a Complex is declared", "ComplexFunctions::isZero", []Value{boolValue(true)}, ErrTypeMismatch},
		{"a Complex where rect declares a Real", "ComplexFunctions::rect", []Value{cx(1, 2), constReal(0)}, ErrTypeMismatch},
		{"a vector where rect declares a Real", "ComplexFunctions::rect", []Value{realVec(1, 2), constReal(0)}, ErrTypeMismatch},
		{"a string element of a Complex collection", "ComplexFunctions::sum", []Value{vec(cx(1, 2), strValue("2"))}, ErrTypeMismatch},
		{"the argument of zero", "ComplexFunctions::arg", []Value{cx(0, 0)}, semantics.ErrArithmeticDomain},
		{"division of a Complex by zero", "ComplexFunctions::/", []Value{cx(1, 2), cx(0, 0)}, ErrDivisionByZero},
		{"a Complex sum beyond the Real range", "ComplexFunctions::+", []Value{cx(1e308, 0), cx(1e308, 0)}, semantics.ErrArithmeticOverflow},
		{"zero to a negative power", "ComplexFunctions::**", []Value{constReal(0), constReal(-1)}, semantics.ErrArithmeticDomain},
		{"a power beyond the Real range", "ComplexFunctions::**", []Value{constReal(1e200), constReal(2)}, semantics.ErrArithmeticOverflow},
		{"too many arguments to a vector constructor", "VectorFunctions::CartesianVectorOf", []Value{realVec(1), realVec(2)}, ErrCalcArity},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := applyLibrary(t, tc.fn, tc.args...)
			if !errors.Is(err, tc.want) {
				t.Fatalf("%s = %+v, %v; want error %v", tc.fn, got, err, tc.want)
			}
		})
	}
}

// TestStringFunctionValues pins StringFunctions: Length counts characters (one
// per Unicode code point, not per byte) and Substring takes 1-based inclusive
// character positions.
func TestStringFunctionValues(t *testing.T) {
	cases := []struct {
		fn   string
		args []Value
		want Value
	}{
		{"StringFunctions::+", []Value{strValue("ab"), strValue("cd")}, strValue("abcd")},
		{"StringFunctions::+", []Value{strValue(""), strValue("")}, strValue("")},
		{"StringFunctions::Length", []Value{strValue("abc")}, integerValue(3)},
		{"StringFunctions::Length", []Value{strValue("")}, integerValue(0)},
		// "héllo" is 6 bytes and 5 characters; Length answers characters.
		{"StringFunctions::Length", []Value{strValue("héllo")}, integerValue(5)},
		{"StringFunctions::Length", []Value{strValue("日本語")}, integerValue(3)},
		{"StringFunctions::Substring", []Value{strValue("abc"), constInt(1), constInt(1)}, strValue("a")},
		{"StringFunctions::Substring", []Value{strValue("abc"), constInt(2), constInt(3)}, strValue("bc")},
		{"StringFunctions::Substring", []Value{strValue("abc"), constInt(1), constInt(3)}, strValue("abc")},
		// An upper below lower selects no character, as
		// SequenceFunctions::subsequence answers nothing for such a range.
		{"StringFunctions::Substring", []Value{strValue("abc"), constInt(2), constInt(1)}, strValue("")},
		{"StringFunctions::Substring", []Value{strValue(""), constInt(1), constInt(0)}, strValue("")},
		{"StringFunctions::Substring", []Value{strValue("héllo"), constInt(2), constInt(3)}, strValue("él")},
		{"StringFunctions::<", []Value{strValue("abc"), strValue("b")}, boolValue(true)},
		{"StringFunctions::<", []Value{strValue("abc"), strValue("abc")}, boolValue(false)},
		{"StringFunctions::<=", []Value{strValue("abc"), strValue("abc")}, boolValue(true)},
		{"StringFunctions::>", []Value{strValue("abc"), strValue("abb")}, boolValue(true)},
		{"StringFunctions::>=", []Value{strValue("abc"), strValue("abd")}, boolValue(false)},
		{"StringFunctions::==", []Value{strValue("abc"), strValue("abc")}, boolValue(true)},
		{"StringFunctions::==", []Value{strValue("abc"), strValue("abd")}, boolValue(false)},
		// '==' declares String[0..1], so an omitted operand is a value it has an
		// answer for: two of them are equal, one of them is not equal to a string.
		{"StringFunctions::==", []Value{nullValue(), nullValue()}, boolValue(true)},
		{"StringFunctions::==", []Value{strValue(""), nullValue()}, boolValue(false)},
		{"StringFunctions::ToString", []Value{strValue("héllo")}, strValue("héllo")},
	}

	for _, tc := range cases {
		got, err := applyLibrary(t, tc.fn, tc.args...)
		if err != nil {
			t.Errorf("%s%v = error %v", tc.fn, tc.args, err)
			continue
		}
		if !valueEqual(got, tc.want) {
			t.Errorf("%s%v = %+v, want %+v", tc.fn, tc.args, got, tc.want)
		}
	}
}

// TestStringFunctionErrors pins the reports of StringFunctions: a position
// naming no character, and an argument of another type, which is reported rather
// than rendered as a string.
func TestStringFunctionErrors(t *testing.T) {
	cases := []struct {
		name string
		fn   string
		args []Value
		want error
	}{
		{"substring below the first character", "StringFunctions::Substring",
			[]Value{strValue("abc"), constInt(0), constInt(2)}, ErrIndexOutOfRange},
		{"substring past the last character", "StringFunctions::Substring",
			[]Value{strValue("abc"), constInt(1), constInt(9)}, ErrIndexOutOfRange},
		// The bound is in characters: "héllo" has 5, though it is 6 bytes.
		{"substring past the last character of a multi-byte string", "StringFunctions::Substring",
			[]Value{strValue("héllo"), constInt(1), constInt(6)}, ErrIndexOutOfRange},
		{"substring of an empty string", "StringFunctions::Substring",
			[]Value{strValue(""), constInt(1), constInt(1)}, ErrIndexOutOfRange},
		{"length of a number", "StringFunctions::Length",
			[]Value{constInt(3)}, ErrTypeMismatch},
		{"concatenation with a number", "StringFunctions::+",
			[]Value{strValue("abc"), constInt(3)}, ErrTypeMismatch},
		{"comparison against a number", "StringFunctions::<",
			[]Value{strValue("abc"), constInt(3)}, ErrTypeMismatch},
		{"equality against a number", "StringFunctions::==",
			[]Value{strValue("abc"), constInt(3)}, ErrTypeMismatch},
		{"substring at a non-integer position", "StringFunctions::Substring",
			[]Value{strValue("abc"), strValue("a"), constInt(2)}, ErrTypeMismatch},
		{"substring of a collection", "StringFunctions::Substring",
			[]Value{vec(strValue("a"), strValue("b")), constInt(1), constInt(1)}, ErrTypeMismatch},
		{"length without an argument", "StringFunctions::Length", nil, ErrCalcArity},
		{"concatenation of three strings", "StringFunctions::+",
			[]Value{strValue("a"), strValue("b"), strValue("c")}, ErrCalcArity},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := applyLibrary(t, tc.fn, tc.args...)
			if !errors.Is(err, tc.want) {
				t.Fatalf("%s%v = %+v, %v; want error %v", tc.fn, tc.args, got, err, tc.want)
			}
			if !strings.Contains(err.Error(), writtenName(tc.fn)) {
				t.Fatalf("%s error %q does not name the function", tc.fn, err)
			}
		})
	}
}

// A named argument binds to the parameter names StringFunctions declares.
func TestStringFunctionNamedArguments(t *testing.T) {
	fn, ok := libraryFunctionByName("StringFunctions::Substring")
	if !ok {
		t.Fatal("StringFunctions::Substring is not registered")
	}
	got, err := fn.invoke(libCtx(t), calcArgs{named: map[string]Value{
		"upper": constInt(3),
		"x":     NewStringValue("abcd"),
		"lower": constInt(2),
	}})
	if err != nil {
		t.Fatalf("Substring(x = \"abcd\", lower = 2, upper = 3) = error %v", err)
	}
	if !valueEqual(got, NewStringValue("bc")) {
		t.Fatalf("Substring(x = \"abcd\", lower = 2, upper = 3) = %+v, want \"bc\"", got)
	}
}

// A declaration this runtime has no representation for the values of reports
// itself by name rather than computing something else.
func TestUnevaluableLibraryFunctionsNameThemselves(t *testing.T) {
	unevaluable := []struct {
		fn   string
		args []Value
	}{
		{"ComplexFunctions::ToString", []Value{cx(1, 2)}},
		{"ComplexFunctions::ToComplex", []Value{NewStringValue("1.0")}},
		{"BaseFunctions::ToString", []Value{cx(1, 2)}},
		{"BaseFunctions::[", []Value{constInt(1), constInt(1)}},
		{"BaseFunctions::all", nil},
		{"BaseFunctions::as", []Value{constInt(1)}},
		{"BaseFunctions::meta", []Value{constInt(1)}},
		{"BaseFunctions::istype", []Value{constInt(1), constInt(1)}},
		{"BaseFunctions::hastype", []Value{constInt(1), constInt(1)}},
		{"BaseFunctions::@", []Value{constInt(1), constInt(1)}},
		{"BaseFunctions::@@", []Value{constInt(1), constInt(1)}},
		{"ControlFunctions::.", []Value{constInt(1)}},
		{"DataFunctions::~", []Value{constInt(1)}},
		{"ScalarFunctions::~", []Value{constInt(1)}},
	}

	for _, tc := range unevaluable {
		t.Run(tc.fn, func(t *testing.T) {
			_, err := applyLibrary(t, tc.fn, tc.args...)
			if !errors.Is(err, ErrUnevaluableLibraryFunction) {
				t.Fatalf("%s error = %v, want %v", tc.fn, err, ErrUnevaluableLibraryFunction)
			}
			if !strings.Contains(err.Error(), writtenName(tc.fn)) {
				t.Fatalf("%s error %q does not name the function", tc.fn, err)
			}
		})
	}
}

// applyBuiltin binds args to a built-in's declared parameters as a direct
// invocation does and applies it in a context with no caller.
func applyBuiltin(t *testing.T, name string, args calcArgs) (Value, error) {
	t.Helper()
	fn, ok := builtins[name]
	if !ok {
		t.Fatalf("%s is not registered", name)
	}
	bound, err := bindBuiltinValues(name, args)
	if err != nil {
		return Value{}, err
	}
	return fn(NewEvalContext(libCtx(t), nil), bound)
}

// TestOccurrenceFunctionArity pins arity before binding: `group [0..*]` may be
// omitted, a missing `occ`/`index` is an arity error, a data value is no occurrence.
func TestOccurrenceFunctionArity(t *testing.T) {
	one := constInt(1)
	for _, tc := range []struct {
		fn   string
		args calcArgs
		want error
	}{
		{"OccurrenceFunctions::addNew", calcArgs{positional: []Value{one}}, ErrCalcArity},
		{"OccurrenceFunctions::addNew", calcArgs{named: map[string]Value{"group": one}}, ErrCalcArity},
		{"OccurrenceFunctions::addNew", calcArgs{named: map[string]Value{"occ": one}}, ErrNotAnOccurrence},
		{"OccurrenceFunctions::addNew", calcArgs{positional: []Value{one, one}}, ErrNotAnOccurrence},
		{"OccurrenceFunctions::addNewAt", calcArgs{positional: []Value{one, one}}, ErrCalcArity},
		{"OccurrenceFunctions::addNewAt", calcArgs{named: map[string]Value{"occ": one}}, ErrCalcArity},
		{"OccurrenceFunctions::addNewAt", calcArgs{named: map[string]Value{"group": one, "index": one}}, ErrCalcArity},
		{"OccurrenceFunctions::addNewAt", calcArgs{named: map[string]Value{"occ": one, "index": one}}, ErrNotAnOccurrence},
		{"OccurrenceFunctions::addNewAt", calcArgs{positional: []Value{one, one, one}}, ErrNotAnOccurrence},
		{"OccurrenceFunctions::create", calcArgs{}, ErrCalcArity},
		{"OccurrenceFunctions::create", calcArgs{positional: []Value{one}}, ErrNotAnOccurrence},
		{"OccurrenceFunctions::isDuring", calcArgs{}, ErrCalcArity},
		{"OccurrenceFunctions::isDuring", calcArgs{positional: []Value{one}}, ErrNotAnOccurrence},
		{"OccurrenceFunctions::destroy", calcArgs{positional: []Value{one}}, ErrNotAnOccurrence},
		{"OccurrenceFunctions::===", calcArgs{positional: []Value{one, one}}, ErrNotAnOccurrence},
		{"OccurrenceFunctions::===", calcArgs{positional: []Value{one, one, one}}, ErrCalcArity},
	} {
		_, err := applyBuiltin(t, tc.fn, tc.args)
		if !errors.Is(err, tc.want) {
			t.Errorf("%s(%+v) error = %v, want %v", tc.fn, tc.args, err, tc.want)
		}
		if errors.Is(err, ErrNotAnOccurrence) && !strings.Contains(err.Error(), writtenName(tc.fn)) {
			t.Errorf("%s(%+v) error %q does not name the function", tc.fn, tc.args, err)
		}
	}

	// `destroy` of nothing destroys nothing; `'==='` of nothing with nothing holds.
	if got, err := applyBuiltin(t, "OccurrenceFunctions::destroy", calcArgs{}); err != nil || !isEmptyValue(got) {
		t.Errorf("destroy() = %v, %v; want nothing", got, err)
	}
	nothing := calcArgs{positional: []Value{emptySequence(), emptySequence()}}
	if got, err := applyBuiltin(t, "OccurrenceFunctions::===", nothing); err != nil || got.Kind != ValConst || !got.Const.Bool {
		t.Errorf("===((), ()) = %v, %v; want true", got, err)
	}
}

// vendoredFunctionPackages lists every function library package the runtime
// vendors. The declarations the implementations are registered against cannot
// drift from the registry: every function these packages declare is either
// implemented here with the declared parameter names in the declared order, or
// implemented as a built-in over collections (builtins.go) whose signature
// (builtins_signature.go) states the declared names, optionality and `expr`.
var vendoredFunctionPackages = map[string]string{
	"BaseFunctions":          "Kernel Libraries/Kernel Function Library/BaseFunctions.kerml",
	"BooleanFunctions":       "Kernel Libraries/Kernel Function Library/BooleanFunctions.kerml",
	"CollectionFunctions":    "Kernel Libraries/Kernel Function Library/CollectionFunctions.kerml",
	"ComplexFunctions":       "Kernel Libraries/Kernel Function Library/ComplexFunctions.kerml",
	"ControlFunctions":       "Kernel Libraries/Kernel Function Library/ControlFunctions.kerml",
	"DataFunctions":          "Kernel Libraries/Kernel Function Library/DataFunctions.kerml",
	"IntegerFunctions":       "Kernel Libraries/Kernel Function Library/IntegerFunctions.kerml",
	"NaturalFunctions":       "Kernel Libraries/Kernel Function Library/NaturalFunctions.kerml",
	"NumericalFunctions":     "Kernel Libraries/Kernel Function Library/NumericalFunctions.kerml",
	"OccurrenceFunctions":    "Kernel Libraries/Kernel Function Library/OccurrenceFunctions.kerml",
	"RationalFunctions":      "Kernel Libraries/Kernel Function Library/RationalFunctions.kerml",
	"RealFunctions":          "Kernel Libraries/Kernel Function Library/RealFunctions.kerml",
	"ScalarFunctions":        "Kernel Libraries/Kernel Function Library/ScalarFunctions.kerml",
	"SequenceFunctions":      "Kernel Libraries/Kernel Function Library/SequenceFunctions.kerml",
	"StringFunctions":        "Kernel Libraries/Kernel Function Library/StringFunctions.kerml",
	"TrigFunctions":          "Kernel Libraries/Kernel Function Library/TrigFunctions.kerml",
	"VectorFunctions":        "Kernel Libraries/Kernel Function Library/VectorFunctions.kerml",
	"OpenSysMLMathFunctions": "OpenSysML Libraries/OpenSysMLMathFunctions.kerml",
}

// Every function each vendored package declares, operator-named ones included,
// is registered: computed, a builtin, or unevaluable by name with a reason. No
// declaration is exempt.
func TestVendoredFunctionsAreAllDispatchable(t *testing.T) {
	for pkg, path := range vendoredFunctionPackages {
		t.Run(pkg, func(t *testing.T) {
			data, err := libs.DefaultSource().Read(path)
			if err != nil {
				t.Fatalf("Read(%q): %v", path, err)
			}
			p := parser.New(source.New(path, data))
			file := p.ParseFile()
			if len(p.Diagnostics) > 0 {
				t.Fatalf("%s has %d parse diagnostics, want 0: %v", path, len(p.Diagnostics), p.Diagnostics)
			}
			idx := symbols.NewIndex()
			idx.AddDocument(path, file)
			resolver := resolve.New(idx)
			ctx := NewContext(NewModel(semantics.NewModel(resolver), resolver), 10000)

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
						continue
					}
					checkBuiltinSignature(t, ctx, fqn, sym)
					continue
				}
				checkLibrarySignature(t, ctx, fqn, sym, fn)
			}
			if declared == 0 {
				t.Fatalf("%s declares no function in %s; the package name or path is wrong", pkg, path)
			}
		})
	}
}

// checkLibrarySignature holds the registered signature of the library function
// fqn to the parameters sym declares: name, order and optionality.
func checkLibrarySignature(t *testing.T, ctx *Context, fqn string, sym *symbols.Symbol, fn *libraryFunction) {
	t.Helper()
	var declared []declaredParam
	for _, param := range ctx.calcParameters(ctx.calcChain(sym), new(map[string]string)) {
		declared = append(declared, declaredParam{
			name:     param.Name,
			optional: param.Default != nil || param.optional(),
		})
	}
	if !slices.Equal(declared, fn.params) {
		t.Errorf("%s declares %+v, implementation takes %+v", fqn, declared, fn.params)
	}
}

// checkBuiltinSignature holds the registered signature of the built-in fqn to
// the parameters sym declares: name, order, optionality and `expr`.
func checkBuiltinSignature(t *testing.T, ctx *Context, fqn string, sym *symbols.Symbol) {
	t.Helper()
	sig, ok := builtinSignatures[fqn]
	if !ok {
		t.Errorf("%s is a built-in with no signature in builtinSignatures", fqn)
		return
	}
	var declared []declaredParam
	for _, member := range declMembers(sym.Decl) {
		usage, ok := member.(*ast.Usage)
		if !ok || (usage.Direction != ast.DirIn && usage.Direction != ast.DirInOut) {
			continue
		}
		name, _ := ast.EffectiveName(usage)
		declared = append(declared, declaredParam{
			name:     name,
			optional: usage.Value != nil || ctx.model.semantics.IsOptionalParameter(usage),
			deferred: usage.Kind == ast.UsageExpr,
		})
	}
	if !slices.Equal(declared, sig) {
		t.Errorf("%s declares %+v, builtinSignatures has %+v", fqn, declared, sig)
	}
}

// Every registered signature belongs to a registered built-in.
func TestBuiltinSignaturesNameBuiltins(t *testing.T) {
	for name := range builtinSignatures {
		if _, ok := builtins[name]; !ok {
			t.Errorf("builtinSignatures has %s, which is no built-in", name)
		}
	}
	for name := range builtins {
		if _, ok := builtinSignatures[name]; !ok {
			t.Errorf("built-in %s has no signature", name)
		}
	}
}

// A model reads a library feature by name through the value seam, which answers
// for a library declaration the library gives no foldable value.
func TestLibraryFeatureNameReadFromItsLibraryDeclaration(t *testing.T) {
	root := parser.New(source.New("test.sysml", []byte(`package test {
	attribute twoPi = 2 * TrigFunctions::pi;
}`))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("test.sysml", root)
	lib := parser.New(source.New("lib.kerml", []byte(`package TrigFunctions {
	feature pi : Real;
}`))).ParseFile()
	idx.AddDocument("lib.kerml", lib)
	idx.MarkLibrary("lib.kerml")
	resolver := resolve.New(idx)

	pkg, ok := idx.DocumentRoot("test.sysml").LookupLocal("test")
	if !ok || pkg == nil || pkg.Scope == nil {
		t.Fatal("package test not found")
	}
	sym, ok := pkg.Scope.LookupLocal("twoPi")
	if !ok || sym == nil {
		t.Fatal("attribute twoPi not found")
	}
	decl, ok := sym.Decl.(*ast.Usage)
	if !ok || decl.Value == nil {
		t.Fatalf("twoPi declares %T with no value", sym.Decl)
	}

	ctx := NewContext(NewModel(semantics.NewModel(resolver), resolver), 10000)
	got, err := NewEvalContext(ctx, pkg.Scope).Eval(decl.Value)
	if err != nil {
		t.Fatalf("2 * TrigFunctions::pi = error %v", err)
	}
	if got.Kind != ValConst || got.Const.Real != 2*math.Pi {
		t.Fatalf("2 * TrigFunctions::pi = %+v, want %v", got, 2*math.Pi)
	}
}

// libraryContextForSource indexes src as a library document, which is what the
// feature-value seam answers for.
func libraryContextForSource(t *testing.T, src string) (*Context, *symbols.Index) {
	t.Helper()
	file := parser.New(source.New("lib.kerml", []byte(src))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("lib.kerml", file)
	idx.MarkLibrary("lib.kerml")
	resolver := resolve.New(idx)
	return NewContext(NewModel(semantics.NewModel(resolver), resolver), 10000), idx
}

// The library declares `feature pi : Real` with no value, so its value comes from
// the seam.
func TestLibraryFeatureValue(t *testing.T) {
	ctx, idx := libraryContextForSource(t, `package TrigFunctions {
	feature pi : Real;
}`)
	sym := lookupOne(t, idx, "TrigFunctions::pi")

	got, ok, err := ctx.libraryFeatureValue(sym)
	if err != nil || !ok {
		t.Fatalf("libraryFeatureValue(TrigFunctions::pi) = %+v, %v, %v", got, ok, err)
	}
	if got.Kind != ValConst || got.Const.Real != math.Pi {
		t.Fatalf("TrigFunctions::pi = %+v, want %v", got, math.Pi)
	}
}

// The seam answers for a library declaration that carries a value of its own,
// which library content does on every load path: the value is this runtime's.
func TestLibraryFeatureValueOverridesADeclaredLibraryValue(t *testing.T) {
	ctx, idx := libraryContextForSource(t, `package TrigFunctions {
	feature pi : Real = 3.0;
}`)

	got, ok, err := ctx.libraryFeatureValue(lookupOne(t, idx, "TrigFunctions::pi"))
	if err != nil || !ok || got.Const.Real != math.Pi {
		t.Fatalf("TrigFunctions::pi = %+v, %v, %v; want pi", got, ok, err)
	}
}

// The seam is keyed by the resolved symbol and answers for a library declaration
// only, so a model that declares its own feature of the same name keeps its own
// value — name resolution decides, as it does for every other name.
func TestLibraryFeatureValueLeavesAModelsOwnFeatureAlone(t *testing.T) {
	ctx, idx := contextForSource(t, `package TrigFunctions {
	feature pi : Real = 3.0;
}`)
	if _, ok, _ := ctx.libraryFeatureValue(lookupOne(t, idx, "TrigFunctions::pi")); ok {
		t.Fatalf("the seam supplied a value for a model's own feature")
	}
}

// The zero-vector features are vectors, not flat sequences of Reals:
// cartesianZeroVector groups the 1-, 2- and 3-dimensional zero vectors, and
// every read builds its own value, so no reader can change another's.
func TestLibraryFeatureZeroVectors(t *testing.T) {
	ctx, idx := libraryContextForSource(t, `package VectorFunctions {
	feature cartesianZeroVector : CartesianVectorValue[3];
	feature cartesian3DZeroVector : CartesianThreeVectorValue;
}`)

	grouped, ok, err := ctx.libraryFeatureValue(lookupOne(t, idx, "VectorFunctions::cartesianZeroVector"))
	if err != nil || !ok || grouped.Kind != ValSequence {
		t.Fatalf("cartesianZeroVector = %+v, %v, %v; want a sequence of vectors", grouped, ok, err)
	}
	if rendered := FormatValue(grouped); rendered != "[⟨0.0⟩, ⟨0.0, 0.0⟩, ⟨0.0, 0.0, 0.0⟩]" {
		t.Fatalf("cartesianZeroVector = %s, want the zero vectors of dimension 1, 2 and 3", rendered)
	}
	for i, zero := range grouped.Sequence().Elements() {
		if elements := vectorValues(t, zero); len(elements) != i+1 {
			t.Fatalf("cartesianZeroVector#(%d) has dimension %d, want %d", i+1, len(elements), i+1)
		}
	}

	first, ok, err := ctx.libraryFeatureValue(lookupOne(t, idx, "VectorFunctions::cartesian3DZeroVector"))
	if err != nil || !ok {
		t.Fatalf("cartesian3DZeroVector = %+v, %v, %v", first, ok, err)
	}
	if elements := vectorValues(t, first); len(elements) != 3 || elements[0].Real != 0 {
		t.Fatalf("cartesian3DZeroVector = %v, want ⟨0.0, 0.0, 0.0⟩", first)
	}
	second, _, _ := ctx.libraryFeatureValue(lookupOne(t, idx, "VectorFunctions::cartesian3DZeroVector"))
	if first.Vector() == second.Vector() {
		t.Fatalf("two reads of cartesian3DZeroVector share one vector")
	}
}

// Every feature the seam supplies is named by its fully-qualified name, which is
// how a model that imports no part of the libraries reads one.
func TestLibraryFeatureByName(t *testing.T) {
	for _, fqn := range []string{"TrigFunctions::pi", "ComplexFunctions::i", "VectorFunctions::cartesian3DZeroVector"} {
		feature, ok := libraryFeatureByName(fqn)
		if !ok {
			t.Fatalf("no library feature %s registered", fqn)
		}
		if _, err := feature.value(libCtx(t)); err != nil {
			t.Fatalf("%s = error %v", fqn, err)
		}
	}
	if _, ok := libraryFeatureByName("TrigFunctions::tau"); ok {
		t.Fatalf("TrigFunctions::tau is not a library feature")
	}
}

// ComplexFunctions declares `feature i: Complex[1] = rect(0.0, 1.0)`, which the
// seam supplies as that one Complex value.
func TestLibraryFeatureImaginaryUnit(t *testing.T) {
	feature, ok := libraryFeatureByName("ComplexFunctions::i")
	if !ok {
		t.Fatal("no library feature ComplexFunctions::i registered")
	}
	got, err := feature.value(libCtx(t))
	if err != nil {
		t.Fatalf("ComplexFunctions::i = error %v", err)
	}
	if complexValue(t, got) != complex(0, 1) {
		t.Fatalf("ComplexFunctions::i = %v, want 0.0 + 1.0i", got)
	}
}

// A library function is answered by its built-in even where the vendored
// declaration carries a body: the body is not there to evaluate when the library
// index cache is warm, so dispatch must not depend on it.
func TestLibraryFunctionAnswersALibraryDeclarationWithABody(t *testing.T) {
	ctx, idx := libraryContextForSource(t, `package TrigFunctions {
	feature pi : Real;
	function deg { in theta_rad : Real[1]; return : Real[1] = theta_rad * 180 / pi; }
}`)
	sym := lookupOne(t, idx, "TrigFunctions::deg")

	if _, ok := ctx.libraryFunctionFor(sym); !ok {
		t.Fatalf("the library's own deg did not dispatch to its built-in implementation")
	}
	got, err := ctx.InvokeCalc(sym, []Value{constReal(math.Pi)}, nil)
	if err != nil || got.Const.Real != 180 {
		t.Fatalf("InvokeCalc(TrigFunctions::deg, pi) = %+v, %v; want 180.0", got, err)
	}
}

// The listing covers every implemented function a model calls by name, each
// with the package an import must name for the unqualified call to resolve, so
// a working function is never advertised as unsupported nor as callable bare.
func TestBuiltinsListEveryFunctionWithItsPackage(t *testing.T) {
	listed := make(map[string]Builtin)
	for _, b := range Builtins() {
		listed[b.FQN] = b
		if b.FQN != b.Package+"::"+b.Name {
			t.Errorf("%q is listed under package %q and name %q", b.FQN, b.Package, b.Name)
		}
	}
	for fqn, fn := range libraryFunctions {
		if _, operator := builtinNamed(fqn); !operator || fn.unevaluable {
			if _, ok := listed[fqn]; ok {
				t.Errorf("%q is listed, want operators and unevaluable declarations left out", fqn)
			}
			continue
		}
		b, ok := listed[fqn]
		if !ok {
			t.Errorf("%q is implemented but not listed", fqn)
			continue
		}
		if b.Collection || !slices.Equal(b.Params, fn.paramNames()) {
			t.Errorf("%q is listed as %+v, want a scalar function with parameters %v", fqn, b, fn.paramNames())
		}
	}
	for fqn := range builtins {
		if _, named := builtinNamed(fqn); !named {
			continue
		}
		if b, ok := listed[fqn]; !ok || !b.Collection {
			t.Errorf("%q is listed as %+v, want a collection function", fqn, b)
		}
	}
	for _, want := range []struct{ fqn, pkg string }{
		{"RealFunctions::sqrt", "RealFunctions"},
		{"OpenSysMLMathFunctions::exp", "OpenSysMLMathFunctions"},
		{"SequenceFunctions::size", "SequenceFunctions"},
	} {
		if b := listed[want.fqn]; b.Package != want.pkg {
			t.Errorf("%s is listed as %+v, want package %s", want.fqn, b, want.pkg)
		}
	}
	for _, absent := range []string{"SequenceFunctions::#", "IntegerFunctions::..", "TensorCalculations::transform", "ComplexFunctions::ToString"} {
		if b, ok := listed[absent]; ok {
			t.Errorf("%s is listed as %+v, want it left out", absent, b)
		}
	}
}

// TestArraySpecializationKeepsOwnMembers: an attribute def specializing Array
// answers Array's features from its shape and its own members from the object,
// directly and after the value passed through a calc, a parameter or an attribute.
func TestArraySpecializationKeepsOwnMembers(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			private import ScalarValues::*;
			private import Collections::*;
			attribute def LabeledGrid :> Array {
				attribute label : String;
				attribute scale : Real = 0.5;
			}
			attribute def OtherGrid :> Array;
			attribute grid : LabeledGrid {
				:>> dimensions = (2, 2);
				:>> elements = (1, 2, 3, 4);
				:>> label = "grid";
			}
			calc def pick { return : LabeledGrid = grid; }
			calc def labelOf { in g : LabeledGrid; return : String = g.label; }
			attribute copy : LabeledGrid = grid;
			attribute plain : Array = grid;
			attribute other : OtherGrid = grid;
		}
	`))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	for _, tc := range []struct{ src, want string }{
		{"grid.rank", "2"},
		{"grid.flattenedSize", "4"},
		{"grid.dimensions", "[2, 2]"},
		{"grid.elements#(3)", "3"},
		{"grid.label", `"grid"`},
		{"grid.scale", "0.5"},
		{"pick()", "Array(2, 2)[1, 2, 3, 4]"},
		{"pick().label", `"grid"`},
		{"pick().scale", "0.5"},
		{"pick().rank", "2"},
		{"CollectionFunctions::'array#'(pick(), (2, 1))", "3"},
		{"labelOf(grid)", `"grid"`},
		{"copy.label", `"grid"`},
		{"plain.label", `"grid"`},
	} {
		got, err := evalIn(t, ctx, pkg.Scope, tc.src)
		if err != nil {
			t.Errorf("%s: %v", tc.src, err)
			continue
		}
		if rendered := FormatValue(got); rendered != tc.want {
			t.Errorf("%s = %s, want %s", tc.src, rendered, tc.want)
		}
	}
	if _, err := evalIn(t, ctx, pkg.Scope, "grid.missing"); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Errorf("grid.missing = %v, want an error naming the member", err)
	}
	// The value stays a LabeledGrid, which no OtherGrid feature can hold.
	if got, err := evalIn(t, ctx, pkg.Scope, "other"); !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("other = (%s, %v), want %v", FormatValue(got), err, ErrTypeMismatch)
	}
}
