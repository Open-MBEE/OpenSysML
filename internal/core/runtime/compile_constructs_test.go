package runtime

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"sort"
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

// compiledFixtures are the models under testdata/compiled, each exercising one
// construct the tier compiles, with the calcs expected on each tier.
var compiledFixtures = []struct {
	file       string
	eligible   []string
	ineligible []string
}{
	{
		file:       "body_locals.sysml",
		eligible:   []string{"Chain", "MasksParameter", "Branches", "Clamp", "Shadow", "Sign", "FailsInLocal", "NaturalResult", "BadCondition", "CallsChain"},
		ineligible: []string{"ReadsEarly", "NoValue", "MayFall", "StringLocal", "Assigns"},
	},
	{
		file:       "specialization.sysml",
		eligible:   []string{"Base", "Mid", "Top", "Qualified", "Narrow", "Own", "Extended", "Statements", "CallsTop"},
		ineligible: []string{"ComputedDefault"},
	},
	{
		file: "intrinsics.sysml",
		eligible: []string{
			"Sqrt", "Abs", "Floor", "Round", "Max", "Min", "IsZero", "IsUnit",
			"Sin", "Cos", "Tan", "Cot", "Arcsin", "Arccos", "Arctan", "Deg", "Rad",
			"Exp", "Ln", "Log", "Atan2", "Pi", "Sum", "Product",
			"IntAbs", "IntMax", "IntMin", "NumAbs", "NumMax", "IntSum", "IntProduct", "RealSqrtOfInt", "IntegerAbsOfReal",
			"Qualified", "Aliased", "Receiver", "Hypotenuse", "Circumference",
			"OwnSqrt", "sqrt", "LibraryStill",
		},
		ineligible: []string{"Length", "SumOfSequence", "OwnPi"},
	},
	{
		file:     "named_arguments.sysml",
		eligible: []string{"Weighted", "InOrder", "Reordered", "Defaulted", "OnlyRequired", "Failing", "LibraryNamed", "Fib", "Scaled"},
	},
}

// compiledFixture is one fixture built once, with a context on each tier.
type compiledFixture struct {
	root      *symbols.Scope
	compiled  *Context
	reference *Context
}

// loadCompiledFixture builds testdata/compiled/<file> with the libraries.
func loadCompiledFixture(t *testing.T, file string) *compiledFixture {
	t.Helper()
	path := filepath.Join("testdata", "compiled", file)
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return buildCompiledFixture(path, src)
}

// buildCompiledFixture builds the model in src with the libraries.
func buildCompiledFixture(path string, src []byte) *compiledFixture {
	idx := libs.NewModelIndex()
	idx.AddDocument(path, parser.New(source.New(path, src)).ParseFile())
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	f := &compiledFixture{root: idx.DocumentRoot(path)}
	f.compiled = NewContext(NewModel(model, resolver), differentialMaxSteps)
	f.compiled.SetCalcCompile(true)
	f.reference = NewContext(NewModel(model, resolver), differentialMaxSteps)
	f.reference.SetCalcCompile(false)
	return f
}

// calc finds the calc definition named in the fixture, in any of its packages.
func (f *compiledFixture) calc(t *testing.T, name string) *symbols.Symbol {
	t.Helper()
	for _, sym := range calcSymbolsUnder(f.root) {
		if sym.Name == name && sym.Decl != nil {
			if def, ok := sym.Decl.(*ast.Definition); ok && def.Kind == ast.DefCalc {
				return sym
			}
		}
	}
	t.Fatalf("calc %s not found", name)
	return nil
}

// eligible reports whether the calc compiled, and the reason where it did not.
func (f *compiledFixture) eligible(t *testing.T, name string) (bool, string) {
	t.Helper()
	shape, err := f.compiled.calcShapeOf(f.calc(t, name))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return f.compiled.compiledCalcOf(shape) != nil, shape.ineligibleWhy
}

// outcomes invokes the calc on both tiers with the same arguments.
func (f *compiledFixture) outcomes(t *testing.T, name string, args calcArgs) (compiled, reference calcOutcome) {
	t.Helper()
	sym := f.calc(t, name)
	invoke := func(ctx *Context) calcOutcome {
		var out calcOutcome
		if args.named != nil {
			out.value, out.err = ctx.InvokeCalcNamed(sym, args.named, f.root)
		} else {
			out.value, out.err = ctx.InvokeCalc(sym, args.positional, f.root)
		}
		out.steps = ctx.run.steps
		return out
	}
	return invoke(f.compiled), invoke(f.reference)
}

// same invokes the calc on both tiers and requires the same outcome.
func (f *compiledFixture) same(t *testing.T, name string, args ...Value) calcOutcome {
	t.Helper()
	compiled, reference := f.outcomes(t, name, calcArgs{positional: args})
	wantOutcomesEqual(t, name+describeArgs(args), compiled, reference)
	return compiled
}

// sameNamed is same for arguments bound by name.
func (f *compiledFixture) sameNamed(t *testing.T, name string, args map[string]Value) calcOutcome {
	t.Helper()
	compiled, reference := f.outcomes(t, name, calcArgs{named: args})
	wantOutcomesEqual(t, name+describeNamedArgs(args), compiled, reference)
	return compiled
}

func describeNamedArgs(args map[string]Value) string {
	names := make([]string, 0, len(args))
	for name := range args {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, len(names))
	for i, name := range names {
		parts[i] = name + " = " + FormatTraceValue(args[name])
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// Every fixture calc lands on the tier expected of it, and answers on the
// compiled tier exactly as on the evaluator over the differential vectors.
func TestCompiledConstructs(t *testing.T) {
	for _, fixture := range compiledFixtures {
		t.Run(strings.TrimSuffix(fixture.file, ".sysml"), func(t *testing.T) {
			f := loadCompiledFixture(t, fixture.file)
			for _, name := range fixture.eligible {
				if ok, why := f.eligible(t, name); !ok {
					t.Errorf("%s is ineligible: %s", name, why)
				}
			}
			for _, name := range fixture.ineligible {
				ok, why := f.eligible(t, name)
				if ok {
					t.Errorf("%s is eligible, want ineligible", name)
				} else if why == "" {
					t.Errorf("%s records no reason for its ineligibility", name)
				}
			}
			for _, name := range append(append([]string{}, fixture.eligible...), fixture.ineligible...) {
				shape, err := f.compiled.calcShapeOf(f.calc(t, name))
				if err != nil {
					t.Fatalf("%s: %v", name, err)
				}
				for _, args := range differentialVectors(len(shape.Params)) {
					f.same(t, name, args...)
				}
			}
		})
	}
}

func wantOutcomeReal(t *testing.T, name string, got calcOutcome, want float64) {
	t.Helper()
	if got.err != nil {
		t.Fatalf("%s: %v", name, got.err)
	}
	if got.value.Kind != ValConst || got.value.Const.Kind != semantics.ValReal || got.value.Const.Real != want {
		t.Fatalf("%s = %s, want %g", name, FormatTraceValue(got.value), want)
	}
}

// Body locals compute in declaration order, mask what they are named like, and
// fail as the evaluator's declarations fail.
func TestCompiledBodyLocals(t *testing.T) {
	f := loadCompiledFixture(t, "body_locals.sysml")
	wantOutcomeReal(t, "Chain(3)", f.same(t, "Chain", realArg(3)), 42)
	wantOutcomeInt(t, "MasksParameter(5)", f.same(t, "MasksParameter", intArg(5)), 18)
	wantOutcomeReal(t, "Branches(-2)", f.same(t, "Branches", realArg(-2)), 4)
	wantOutcomeReal(t, "Branches(2)", f.same(t, "Branches", realArg(2)), 2)
	wantOutcomeInt(t, "Clamp(50)", f.same(t, "Clamp", intArg(50)), 10)
	wantOutcomeInt(t, "Clamp(-50)", f.same(t, "Clamp", intArg(-50)), -10)
	wantOutcomeInt(t, "Clamp(5)", f.same(t, "Clamp", intArg(5)), 5)
	wantOutcomeInt(t, "Shadow(3)", f.same(t, "Shadow", intArg(3)), 30)
	wantOutcomeInt(t, "Shadow(-3)", f.same(t, "Shadow", intArg(-3)), -3)
	wantOutcomeInt(t, "Sign(-0.5)", f.same(t, "Sign", realArg(-0.5)), -1)
	wantOutcomeInt(t, "Sign(0)", f.same(t, "Sign", realArg(0)), 0)
	wantOutcomeReal(t, "CallsChain(1)", f.same(t, "CallsChain", realArg(1)), 6+12*13)

	failed := f.same(t, "FailsInLocal", intArg(0))
	wantErrorIs(t, "FailsInLocal(0)", failed, ErrDivisionByZero)
	if !strings.Contains(failed.err.Error(), "eval declaration a") {
		t.Errorf("FailsInLocal(0) is not worded as a declaration: %v", failed.err)
	}
	negative := f.same(t, "NaturalResult", intArg(0))
	wantErrorIs(t, "NaturalResult(0)", negative, ErrTypeMismatch)
	bad := f.same(t, "BadCondition", realArg(1))
	if bad.err == nil || !strings.Contains(bad.err.Error(), "condition of 'if' must evaluate to a Boolean") {
		t.Errorf("BadCondition(1) = %v", bad.err)
	}
	// The evaluator reads a local declared later through its declaration; the
	// tier leaves that body to it.
	wantOutcomeInt(t, "ReadsEarly(1)", f.same(t, "ReadsEarly", intArg(1)), 2)
}

// Parameters redeclared along a chain bind in the slots of the parameters they
// redefine, with the defaults and the types the redeclaration states.
func TestCompiledSpecializationChain(t *testing.T) {
	f := loadCompiledFixture(t, "specialization.sysml")
	wantOutcomeReal(t, "Base(2)", f.same(t, "Base", realArg(2)), 2)
	wantOutcomeReal(t, "Mid(2)", f.same(t, "Mid", realArg(2)), 4)
	wantOutcomeReal(t, "Top()", f.same(t, "Top"), 6)
	wantOutcomeReal(t, "Top(5)", f.same(t, "Top", realArg(5)), 10)
	wantOutcomeReal(t, "Top(5, 5)", f.same(t, "Top", realArg(5), realArg(5)), 25)
	wantOutcomeReal(t, "Qualified()", f.same(t, "Qualified"), 10)
	wantOutcomeReal(t, "Own(5, 2)", f.same(t, "Own", realArg(5), realArg(2)), 3)
	wantOutcomeReal(t, "Extended(2, 3)", f.same(t, "Extended", realArg(2), realArg(3)), 6.5)
	wantOutcomeReal(t, "Statements(1)", f.same(t, "Statements", realArg(1)), 9)
	wantOutcomeReal(t, "CallsTop(1)", f.same(t, "CallsTop", realArg(1)), 2+6+2)
	wantOutcomeReal(t, "ComputedDefault(2)", f.same(t, "ComputedDefault", realArg(2)), 6)

	// Narrowing Real to Integer: an Integer computes, a Real is refused.
	wantOutcomeReal(t, "Narrow(3)", f.same(t, "Narrow", intArg(3)), 3)
	refused := f.same(t, "Narrow", realArg(1.5))
	wantErrorIs(t, "Narrow(1.5)", refused, ErrTypeMismatch)
	if !strings.Contains(refused.err.Error(), `parameter "x"`) {
		t.Errorf("Narrow(1.5) names no parameter: %v", refused.err)
	}
	wantOutcomeReal(t, "Narrow(x = 4)", f.sameNamed(t, "Narrow", map[string]Value{"x": intArg(4)}), 4)
	wantOutcomeReal(t, "Top(y = 4)", f.sameNamed(t, "Top", map[string]Value{"y": realArg(4)}), 12)
}

// realEdges are the Reals a library function is tried with beyond the
// differential vectors: the domain edges of logarithms, inverse trig, and angles.
var realEdges = []float64{
	0, math.Copysign(0, -1), 1, -1, 0.5, -0.5, 2, 10, math.E, math.Pi, math.Pi / 2, 180, 360, -270,
	1e-300, 1e300, -1e300, math.MaxFloat64, math.SmallestNonzeroFloat64,
	math.Inf(1), math.Inf(-1), math.NaN(),
}

// Every intrinsic answers bit for bit as the evaluator's library seam does,
// values and errors alike, however the declaration is reached.
func TestCompiledIntrinsics(t *testing.T) {
	f := loadCompiledFixture(t, "intrinsics.sysml")
	unary := []string{"Sqrt", "Abs", "Floor", "Round", "IsZero", "IsUnit", "Sin", "Cos", "Tan", "Cot",
		"Arcsin", "Arccos", "Arctan", "Deg", "Rad", "Exp", "Ln", "Sum", "Product", "Aliased", "Circumference",
		"OwnSqrt", "sqrt", "LibraryStill", "IntegerAbsOfReal"}
	for _, name := range unary {
		for _, x := range realEdges {
			f.same(t, name, realArg(x))
		}
		for _, k := range []int64{0, 1, -1, 7, math.MaxInt64, math.MinInt64} {
			f.same(t, name, intArg(k))
		}
	}
	binary := []string{"Max", "Min", "Log", "Atan2", "Receiver", "Hypotenuse"}
	for _, name := range binary {
		for _, x := range realEdges {
			for _, y := range []float64{0, math.Copysign(0, -1), 1, -1, 2, 10, math.Inf(1), math.NaN()} {
				f.same(t, name, realArg(x), realArg(y))
			}
		}
	}
	for _, k := range []int64{0, 1, -1, 7, math.MaxInt64, math.MinInt64} {
		f.same(t, "IntAbs", intArg(k))
		f.same(t, "NumAbs", intArg(k))
		f.same(t, "IntSum", intArg(k))
		f.same(t, "IntProduct", intArg(k))
		f.same(t, "RealSqrtOfInt", intArg(k))
		for _, j := range []int64{0, 3, -3, math.MaxInt64, math.MinInt64} {
			f.same(t, "IntMax", intArg(k), intArg(j))
			f.same(t, "IntMin", intArg(k), intArg(j))
			f.same(t, "NumMax", intArg(k), intArg(j))
		}
	}

	wantOutcomeReal(t, "Sqrt(9)", f.same(t, "Sqrt", realArg(9)), 3)
	wantOutcomeReal(t, "Pi()", f.same(t, "Pi"), math.Pi)
	wantOutcomeReal(t, "Qualified(0)", f.same(t, "Qualified", realArg(0)), 1)
	wantOutcomeReal(t, "Aliased(16)", f.same(t, "Aliased", realArg(16)), 4)
	wantOutcomeReal(t, "Receiver(4, 1)", f.same(t, "Receiver", realArg(4), realArg(1)), 6)
	wantOutcomeReal(t, "Hypotenuse(3, 4)", f.same(t, "Hypotenuse", realArg(3), realArg(4)), 5)
	wantOutcomeReal(t, "Circumference(1)", f.same(t, "Circumference", realArg(1)), 2*math.Pi)
	wantOutcomeInt(t, "Floor(-1.5)", f.same(t, "Floor", realArg(-1.5)), -2)
	wantOutcomeInt(t, "IntMax(3, 7)", f.same(t, "IntMax", intArg(3), intArg(7)), 7)
	wantOutcomeInt(t, "NumMax(3, 7)", f.same(t, "NumMax", intArg(3), intArg(7)), 7)
	wantOutcomeInt(t, "IntAbs(-3)", f.same(t, "IntAbs", intArg(-3)), 3)
	wantOutcomeInt(t, "IntSum(7)", f.same(t, "IntSum", intArg(7)), 7)
	wantOutcomeReal(t, "Sum(2.5)", f.same(t, "Sum", realArg(2.5)), 2.5)

	// The model's own sqrt and pi answer, never the library's.
	wantOutcomeReal(t, "OwnSqrt(16)", f.same(t, "OwnSqrt", realArg(16)), 8)
	wantOutcomeReal(t, "OwnPi()", f.same(t, "OwnPi"), 3)
	wantOutcomeReal(t, "LibraryStill(16)", f.same(t, "LibraryStill", realArg(16)), 4+math.Pi)
	if ok, why := f.eligible(t, "OwnPi"); ok || !strings.Contains(why, `"pi"`) {
		t.Errorf("OwnPi: eligible %v, reason %q", ok, why)
	}

	if str := f.same(t, "Length", NewStringValue("abc")); str.err != nil || str.value.Const.Int != 3 {
		t.Errorf("Length(\"abc\") = %s, %v", FormatTraceValue(str.value), str.err)
	}
}

// Arguments bound by name land in the parameters' slots whatever their order,
// and every refusal is worded as the evaluator words it.
func TestCompiledNamedArguments(t *testing.T) {
	f := loadCompiledFixture(t, "named_arguments.sysml")
	wantOutcomeReal(t, "InOrder(2)", f.same(t, "InOrder", realArg(2)), 5)
	wantOutcomeReal(t, "Reordered(2)", f.same(t, "Reordered", realArg(2)), 5)
	wantOutcomeReal(t, "Defaulted(2)", f.same(t, "Defaulted", realArg(2)), 6)
	wantOutcomeReal(t, "OnlyRequired(2)", f.same(t, "OnlyRequired", realArg(2)), 2)
	wantOutcomeReal(t, "LibraryNamed(1, 2)", f.same(t, "LibraryNamed", realArg(1), realArg(2)), 2)
	wantOutcomeInt(t, "Fib(10)", f.same(t, "Fib", intArg(10)), 55)
	wantOutcomeReal(t, "Scaled(2)", f.same(t, "Scaled", realArg(2)), 6)

	failing := f.same(t, "Failing", intArg(0))
	wantErrorIs(t, "Failing(0)", failing, ErrDivisionByZero)

	// The entry binding by name reaches the compiled tier too.
	wantOutcomeReal(t, "Weighted(offset = 1, value = 2)",
		f.sameNamed(t, "Weighted", map[string]Value{"offset": realArg(1), "value": realArg(2)}), 3)
	wantOutcomeReal(t, "Weighted(weight = 3, value = 2)",
		f.sameNamed(t, "Weighted", map[string]Value{"weight": realArg(3), "value": realArg(2)}), 6)
	unknownEntry := f.sameNamed(t, "Weighted", map[string]Value{"value": realArg(2), "scale": realArg(1)})
	wantErrorIs(t, "Weighted(scale = 1)", unknownEntry, ErrUnknownParameter)
	missingEntry := f.sameNamed(t, "Weighted", map[string]Value{"weight": realArg(2)})
	wantErrorIs(t, "Weighted(weight = 2)", missingEntry, ErrUnboundParameter)
	mistyped := f.sameNamed(t, "Weighted", map[string]Value{"value": boolArg(true)})
	wantErrorIs(t, "Weighted(value = true)", mistyped, ErrTypeMismatch)
	if stringy := f.sameNamed(t, "Weighted", map[string]Value{"value": NewStringValue("x")}); !errors.Is(stringy.err, ErrTypeMismatch) {
		t.Errorf("Weighted(value = \"x\") = %v", stringy.err)
	}
}

// A label spelled as a short name, an alias, or a qualified inherited name binds
// the parameter it resolves to on both tiers, and two spellings of one parameter
// are refused as the checker refuses them.
func TestCompiledNamedArgumentSpellings(t *testing.T) {
	f := buildCompiledFixture("named_spellings.sysml", []byte(`package test {
		private import ScalarValues::*;
		calc def F { in <xs> x : Integer; return : Integer = x + 1; }
		calc def G { in b : Integer; alias bs for b; return : Integer = b * 2; }
		calc def Base { in n : Integer; return : Integer; }
		calc def Derived :> Base { return : Integer = n + 10; }
		calc def ViaShort { in v : Integer; return : Integer = F(xs = v); }
		calc def ViaAlias { in v : Integer; return : Integer = G(bs = v); }
		calc def ViaQualified { in v : Integer; return : Integer = F(F::x = v); }
		calc def ViaInherited { in v : Integer; return : Integer = Derived(Base::n = v); }
		calc def Twice { in v : Integer; return : Integer = F(x = v, xs = v); }
	}`))
	for name, want := range map[string]int64{"ViaShort": 3, "ViaAlias": 4, "ViaQualified": 3, "ViaInherited": 12} {
		if ok, why := f.eligible(t, name); !ok {
			t.Errorf("%s: ineligible: %s", name, why)
		}
		wantOutcomeInt(t, name+"(2)", f.same(t, name, intArg(2)), want)
	}
	if ok, why := f.eligible(t, "Twice"); ok || why == "" {
		t.Errorf("Twice: eligible %v, reason %q", ok, why)
	}
	twice := f.same(t, "Twice", intArg(2))
	wantErrorIs(t, "Twice(2)", twice, ErrCalcArity)
	if !strings.Contains(twice.err.Error(), `binds parameter "x" twice`) {
		t.Errorf("Twice(2) = %v", twice.err)
	}
}

// Calls the type checker refuses (a required parameter left unbound, a name
// bound twice) are ineligible for the compiled tier and fail on the evaluator
// as the checker says they do.
func TestCompiledNamedArgumentsIllFormed(t *testing.T) {
	f := buildCompiledFixture("ill_formed_named.sysml", []byte(`package test {
		private import ScalarValues::*;
		calc def Weighted {
			in value : Real;
			in weight : Real = 1.0;
			return : Real = value * weight;
		}
		calc def Missing { in v : Real; return : Real = Weighted(weight = v); }
		calc def Duplicate { in v : Real; return : Real = Weighted(value = v, value = 2.0); }
		calc def Other { in value : Real; return : Real = value; }
		calc def Elsewhere { in v : Real; return : Real = Weighted(Other::value = v); }
	}`))
	for _, name := range []string{"Missing", "Duplicate", "Elsewhere"} {
		if ok, why := f.eligible(t, name); ok || why == "" {
			t.Errorf("%s: eligible %v, reason %q", name, ok, why)
		}
	}
	wantErrorIs(t, "Missing(1)", f.same(t, "Missing", realArg(1)), ErrUnboundParameter)
	elsewhere := f.same(t, "Elsewhere", realArg(1))
	wantErrorIs(t, "Elsewhere(1)", elsewhere, ErrUnknownParameter)
	if !strings.Contains(elsewhere.err.Error(), `"Other::value"`) {
		t.Errorf("Elsewhere(1) = %v", elsewhere.err)
	}
	duplicate := f.same(t, "Duplicate", realArg(1))
	wantErrorIs(t, "Duplicate(1)", duplicate, ErrCalcArity)
	if !strings.Contains(duplicate.err.Error(), `binds parameter "value" twice`) {
		t.Errorf("Duplicate(1) = %v", duplicate.err)
	}
}
