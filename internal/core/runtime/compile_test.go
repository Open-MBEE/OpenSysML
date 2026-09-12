package runtime

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// compileModel holds calcs across the eligible subset and its borders.
const compileModel = `
package test {
	private import ScalarValues::*;
	calc def Fib {
		in k : Integer;
		return : Integer = if k <= 1 ? k else Fib(k - 1) + Fib(k - 2);
	}
	calc def SumTo {
		in n : Integer;
		return : Integer = if n <= 0 ? 0 else n + SumTo(n - 1);
	}
	calc def Add { in a : Integer; in b : Integer; return : Integer = a + b; }
	calc def Sub { in a : Integer; in b : Integer; return : Integer = a - b; }
	calc def Mul { in a : Integer; in b : Integer; return : Integer = a * b; }
	calc def Div { in a : Integer; in b : Integer; return : Real = a / b; }
	calc def Mod { in a : Integer; in b : Integer; return : Integer = a % b; }
	calc def Pow { in a : Integer; in b : Integer; return r = a ** b; }
	calc def Neg { in a : Integer; return : Integer = -a; }
	calc def Mixed { in a : Integer; in b : Real; return : Real = a * b + a / 2 - b % 2; }
	calc def Less { in a : Integer; in b : Real; return : Boolean = a < b and a <= b or a > b and a >= b; }
	calc def Same { in a : Integer; in b : Real; return : Boolean = a == b xor a === b; }
	calc def Differs { in a : Integer; in b : Real; return : Boolean = a != b implies a !== b; }
	calc def Guarded { in k : Integer; return : Boolean = k == 0 or 7 % k == 0; }
	calc def Implied { in k : Integer; return : Boolean = k != 0 implies 7 % k == 0; }
	calc def Choose { in k : Integer; return : Integer = if k > 0 ? 100 % k else 100 % (k - 1); }
	calc def Not { in b : Boolean; return : Boolean = not b; }
	calc def Least { return : Integer = -9223372036854775808; }
	calc def Dflt { in a : Integer; in b : Integer = 10; in c : Real = 2.5; return : Real = a + b * c; }
	calc def SameReal { in r : Real; return : Real = r; }
	calc def SameRational { in q : Rational; return : Rational = q; }
	calc def Natural1 { in n : Natural; return : Natural = n - 1; }
	calc def Positive1 { in n : Positive; return : Positive = n - 1; }
	calc def TailPos { in a : Integer; return : Rank; a - 1 }
	attribute def Rank :> Positive;
	calc def IsEven { in n : Integer; return : Boolean = if n == 0 ? true else IsOdd(n - 1); }
	calc def IsOdd { in n : Integer; return : Boolean = if n == 0 ? false else IsEven(n - 1); }
	calc def Deep { in n : Integer; return : Integer = if n <= 0 ? 0 else 1 + Deep(n - 1); }
	calc def Nested { in a : Integer; in b : Integer; return : Integer = Add(Mul(a, b), Sub(a, Fib(b))); }
	calc def Tail { in a : Integer; in b : Integer; a * b + Fib(b) }
	calc def TailNat { in a : Integer; return : Natural; a - 1 }

	calc def Twice { in n : Integer; out a = n * 2; }
	calc def UsesUsage { in k : Integer; calc tw : Twice { in n = k; } return : Integer = tw.a; }
	calc def Local { in k : Integer; attribute m = k * 2; return : Integer = m; }
	calc def LocalPos { in k : Integer; attribute p : Positive = k - 1; return : Integer = p; }
	enum def Level :> Integer { low = 1; high = 3; }
	calc def LocalEnum { in k : Integer; attribute l : Level = k; return : Integer = l + 0; }
	calc def Stringy { in k : Integer; return : String = "x"; }
	calc def NonLiteralDefault { in a : Integer; in b : Integer = a + 1; return : Integer = a + b; }
	calc def Collects { in k : Integer; return r = (1, 2, k)->size(); }
	calc def CallsIneligible { in k : Integer; return : Integer = k + UsesUsage(k); }
	calc def CycleA { in k : Integer; return : Integer = if k <= 0 ? 0 else CycleB(k - 1); }
	calc def CycleB { in k : Integer; return : Integer = if k <= 0 ? 0 else CycleA(k - 1) + UsesUsage(k); }
	calc def NamedCall { in k : Integer; return : Integer = Add(a = k, b = 1); }
	calc def UnknownName { in k : Integer; return : Integer = Add(a = k, c = 1); }
	calc def ReceiverNamed { in k : Integer; return : Integer = k->Add(b = 1); }
	calc def Untyped { in k; k + 1 }
	calc def WithOut { in k : Integer; out o : Integer = k; return : Integer = k + 1; }
	calc def Inherits :> Add;
	calc def Redeclares :> Dflt { in b : Integer = 3; }
}
`

// compileRuntime builds compileModel with the compiled tier on or off.
func compileRuntime(t *testing.T, compile bool) (*symbols.Scope, *Context) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, compileModel))
	ctx.maxSteps = DefaultMaxSteps
	ctx.SetCalcCompile(compile)
	return idx.DocumentRoot("<test>"), ctx
}

func intArg(i int64) Value {
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: i}}
}

func realArg(f float64) Value {
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: f}}
}

func boolArg(b bool) Value {
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: b}}
}

// calcOutcome is what one invocation observably did: its value or error, and
// the steps it spent.
type calcOutcome struct {
	value Value
	err   error
	steps int64
}

func invokeOutcome(t *testing.T, ctx *Context, scope *symbols.Scope, name string, args ...Value) calcOutcome {
	t.Helper()
	sym := findSymbolByName(scope, name, ast.DefCalc)
	if sym == nil {
		t.Fatalf("calc %s not found", name)
	}
	value, err := ctx.InvokeCalc(sym, args, scope)
	return calcOutcome{value: value, err: err, steps: ctx.run.steps}
}

// wantSameOutcome invokes name on both tiers and requires the same value or the
// same error text, and the same step count.
func wantSameOutcome(t *testing.T, name string, args ...Value) calcOutcome {
	t.Helper()
	scope, ctx := compileRuntime(t, true)
	compiled := invokeOutcome(t, ctx, scope, name, args...)
	scope, ctx = compileRuntime(t, false)
	reference := invokeOutcome(t, ctx, scope, name, args...)
	wantOutcomesEqual(t, name, compiled, reference)
	return compiled
}

func wantOutcomesEqual(t *testing.T, name string, compiled, reference calcOutcome) {
	t.Helper()
	switch {
	case (compiled.err == nil) != (reference.err == nil):
		t.Errorf("%s: compiled error %v, reference error %v", name, compiled.err, reference.err)
	case compiled.err != nil:
		if compiled.err.Error() != reference.err.Error() {
			t.Errorf("%s: compiled error %q, reference error %q", name, compiled.err, reference.err)
		}
	default:
		if !outcomeValuesIdentical(compiled.value, reference.value) {
			t.Errorf("%s: compiled %s, reference %s", name, FormatTraceValue(compiled.value), FormatTraceValue(reference.value))
		}
	}
	if compiled.steps != reference.steps {
		t.Errorf("%s: compiled spent %d steps, reference %d", name, compiled.steps, reference.steps)
	}
}

// outcomeValuesIdentical is identity down to the bits of a Real, so a NaN is
// identical to the same NaN and a negative zero only to a negative zero.
func outcomeValuesIdentical(compiled, reference Value) bool {
	if compiled.Kind == ValConst && reference.Kind == ValConst &&
		compiled.Const.Kind == semantics.ValReal && reference.Const.Kind == semantics.ValReal {
		return math.Float64bits(compiled.Const.Real) == math.Float64bits(reference.Const.Real)
	}
	return valueIdentical(compiled, reference)
}

func wantOutcomeInt(t *testing.T, name string, got calcOutcome, want int64) {
	t.Helper()
	if got.err != nil {
		t.Fatalf("%s: %v", name, got.err)
	}
	if got.value.Kind != ValConst || got.value.Const.Kind != semantics.ValInt || got.value.Const.Int != want {
		t.Fatalf("%s = %s, want %d", name, FormatTraceValue(got.value), want)
	}
}

func wantErrorIs(t *testing.T, name string, got calcOutcome, target error) {
	t.Helper()
	if !errors.Is(got.err, target) {
		t.Fatalf("%s: error %v, want %v", name, got.err, target)
	}
}

// The compiled tier spends the steps the evaluator would, so the budget trips
// at the same point either way.
func TestCompiledCalcStepParity(t *testing.T) {
	for _, k := range []int64{0, 1, 2, 10, 15} {
		wantSameOutcome(t, "Fib", intArg(k))
	}
	for _, n := range []int64{0, 1, 50, 500} {
		wantSameOutcome(t, "SumTo", intArg(n))
	}
	got := wantSameOutcome(t, "Fib", intArg(20))
	wantOutcomeInt(t, "Fib(20)", got, 6765)
	if got.steps == 0 {
		t.Fatal("Fib(20) spent no steps")
	}
}

// Every eligible operator answers as the evaluator does, at the edges too.
func TestCompiledCalcOperatorParity(t *testing.T) {
	extremes := []int64{0, 1, -1, 2, -2, 7, math.MaxInt64, math.MinInt64, math.MaxInt64 - 1, math.MinInt64 + 1}
	for _, name := range []string{"Add", "Sub", "Mul", "Div", "Mod", "Pow"} {
		for _, a := range extremes {
			for _, b := range extremes {
				wantSameOutcome(t, name, intArg(a), intArg(b))
			}
		}
	}
	for _, a := range extremes {
		wantSameOutcome(t, "Neg", intArg(a))
		wantSameOutcome(t, "Guarded", intArg(a))
		wantSameOutcome(t, "Implied", intArg(a))
		wantSameOutcome(t, "Choose", intArg(a))
		for _, b := range []float64{0, 1, -1, 0.5, 2, 1e308, -1e308, 3} {
			wantSameOutcome(t, "Mixed", intArg(a), realArg(b))
			wantSameOutcome(t, "Less", intArg(a), realArg(b))
			wantSameOutcome(t, "Same", intArg(a), realArg(b))
			wantSameOutcome(t, "Differs", intArg(a), realArg(b))
		}
	}
	wantSameOutcome(t, "Not", boolArg(true))
	wantSameOutcome(t, "Not", boolArg(false))
	wantOutcomeInt(t, "Least", wantSameOutcome(t, "Least"), math.MinInt64)
	wantOutcomeInt(t, "Untyped(3)", wantSameOutcome(t, "Untyped", intArg(3)), 4)
	wantSameOutcome(t, "Untyped", realArg(0.5))
	wantSameOutcome(t, "Untyped", boolArg(true))
	wantOutcomeInt(t, "Tail(3, 5)", wantSameOutcome(t, "Tail", intArg(3), intArg(5)), 20)
	wantOutcomeInt(t, "TailNat(4)", wantSameOutcome(t, "TailNat", intArg(4)), 3)
	if refused := wantSameOutcome(t, "TailNat", intArg(0)); refused.err == nil {
		t.Fatal("TailNat(0) yielded a negative Natural")
	}
}

// The errors a body raises keep their kind, their text and their calc frames.
func TestCompiledCalcErrorParity(t *testing.T) {
	overflow := wantSameOutcome(t, "Add", intArg(math.MaxInt64), intArg(1))
	wantErrorIs(t, "Add", overflow, semantics.ErrArithmeticOverflow)
	byZero := wantSameOutcome(t, "Mod", intArg(1), intArg(0))
	wantErrorIs(t, "Mod", byZero, ErrDivisionByZero)
	nested := wantSameOutcome(t, "Nested", intArg(math.MaxInt64), intArg(3))
	wantErrorIs(t, "Nested", nested, semantics.ErrArithmeticOverflow)
	if !strings.Contains(nested.err.Error(), "Mul") || !strings.Contains(nested.err.Error(), "Nested") {
		t.Errorf("Nested error names neither frame: %v", nested.err)
	}
	negative := wantSameOutcome(t, "Natural1", intArg(0))
	wantErrorIs(t, "Natural1", negative, ErrTypeMismatch)
	tailNegative := wantSameOutcome(t, "TailNat", intArg(0))
	wantErrorIs(t, "TailNat", tailNegative, ErrTypeMismatch)
	tailOverflow := wantSameOutcome(t, "Tail", intArg(math.MaxInt64), intArg(2))
	wantErrorIs(t, "Tail", tailOverflow, semantics.ErrArithmeticOverflow)
	badArg := wantSameOutcome(t, "Natural1", intArg(-1))
	wantErrorIs(t, "Natural1", badArg, ErrTypeMismatch)
	wantOutcomeInt(t, "Positive1(2)", wantSameOutcome(t, "Positive1", intArg(2)), 1)
	wantErrorIs(t, "Positive1", wantSameOutcome(t, "Positive1", intArg(0)), ErrTypeMismatch)
	wantErrorIs(t, "Positive1", wantSameOutcome(t, "Positive1", intArg(1)), ErrTypeMismatch)
	wantOutcomeInt(t, "TailPos(2)", wantSameOutcome(t, "TailPos", intArg(2)), 1)
	wantErrorIs(t, "TailPos", wantSameOutcome(t, "TailPos", intArg(1)), ErrTypeMismatch)
	wantOutcomeInt(t, "LocalPos(3)", wantSameOutcome(t, "LocalPos", intArg(3)), 2)
	wantErrorIs(t, "LocalPos", wantSameOutcome(t, "LocalPos", intArg(1)), ErrTypeMismatch)
	wantOutcomeInt(t, "LocalEnum(3)", wantSameOutcome(t, "LocalEnum", intArg(3)), 3)
	wantErrorIs(t, "LocalEnum", wantSameOutcome(t, "LocalEnum", intArg(2)), ErrTypeMismatch)
	realForInt := wantSameOutcome(t, "Add", realArg(1.5), intArg(1))
	wantErrorIs(t, "Add", realForInt, ErrTypeMismatch)
	wantOutcomeReal(t, "SameReal(Inf)", wantSameOutcome(t, "SameReal", realArg(math.Inf(1))), math.Inf(1))
	wantOutcomeReal(t, "SameRational(1.5)", wantSameOutcome(t, "SameRational", realArg(1.5)), 1.5)
	wantErrorIs(t, "SameReal", wantSameOutcome(t, "SameReal", realArg(math.NaN())), ErrTypeMismatch)
	wantErrorIs(t, "SameRational", wantSameOutcome(t, "SameRational", realArg(math.NaN())), ErrTypeMismatch)
	boolForInt := wantSameOutcome(t, "Add", boolArg(true), intArg(1))
	wantErrorIs(t, "Add", boolForInt, ErrTypeMismatch)
	intForBool := wantSameOutcome(t, "Not", intArg(1))
	wantErrorIs(t, "Not", intForBool, ErrTypeMismatch)
	tooMany := wantSameOutcome(t, "Neg", intArg(1), intArg(2))
	if tooMany.err == nil {
		t.Fatal("Neg(1, 2) succeeded")
	}
	tooFew := wantSameOutcome(t, "Add", intArg(1))
	if tooFew.err == nil {
		t.Fatal("Add(1) succeeded")
	}
}

// Defaults literal in the declaration bind as the evaluator binds them.
func TestCompiledCalcDefaults(t *testing.T) {
	full := wantSameOutcome(t, "Dflt", intArg(1), intArg(2), realArg(3))
	if full.err != nil || full.value.Const.Real != 7 {
		t.Fatalf("Dflt(1, 2, 3) = %s, %v", FormatTraceValue(full.value), full.err)
	}
	partial := wantSameOutcome(t, "Dflt", intArg(1))
	if partial.err != nil || partial.value.Const.Real != 26 {
		t.Fatalf("Dflt(1) = %s, %v", FormatTraceValue(partial.value), partial.err)
	}
	// An inherited body binds the flattened parameters, a redeclared one in the
	// slot of the parameter it redefines.
	inherited := wantSameOutcome(t, "Inherits", intArg(2), intArg(3))
	wantOutcomeInt(t, "Inherits(2, 3)", inherited, 5)
	redeclared := wantSameOutcome(t, "Redeclares", intArg(1))
	if redeclared.err != nil || redeclared.value.Const.Real != 8.5 {
		t.Fatalf("Redeclares(1) = %s, %v", FormatTraceValue(redeclared.value), redeclared.err)
	}
}

// Recursion through a cycle resolves its targets lazily and still recurses in
// bounded depth.
func TestCompiledCalcCyclesAndLimits(t *testing.T) {
	even := wantSameOutcome(t, "IsEven", intArg(101))
	if even.err != nil || even.value.Const.Bool {
		t.Fatalf("IsEven(101) = %s, %v", FormatTraceValue(even.value), even.err)
	}
	deep := wantSameOutcome(t, "Deep", intArg(100000))
	wantErrorIs(t, "Deep", deep, ErrCalcRecursionLimit)

	for _, compile := range []bool{true, false} {
		scope, ctx := compileRuntime(t, compile)
		ctx.maxSteps = 1000
		got := invokeOutcome(t, ctx, scope, "Fib", intArg(20))
		wantErrorIs(t, "Fib under a small budget", got, ErrStepLimitExceeded)
		if got.steps != ctx.maxSteps+1 {
			t.Errorf("compile=%v: stopped at step %d, want %d", compile, got.steps, ctx.maxSteps+1)
		}
	}
}

// eligibility answers whether the calc named compiled, asking once.
func eligibility(t *testing.T, ctx *Context, scope *symbols.Scope, name string) bool {
	t.Helper()
	sym := findSymbolByName(scope, name, ast.DefCalc)
	if sym == nil {
		t.Fatalf("calc %s not found", name)
	}
	shape, err := ctx.calcShapeOf(sym)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	compiled := ctx.compiledCalcOf(shape)
	if (compiled != nil) != (shape.compileState == compileEligible) {
		t.Fatalf("%s: compiled %v but state %d", name, compiled != nil, shape.compileState)
	}
	return compiled != nil
}

// Only the scalar subset compiles — untyped and redeclared parameters, scalar
// locals, named calls included; a caller of an ineligible calc is ineligible too.
func TestCompiledCalcEligibility(t *testing.T) {
	scope, ctx := compileRuntime(t, true)
	eligible := []string{"Fib", "SumTo", "Add", "Div", "Pow", "Mixed", "Less", "Same", "Not", "Least", "Dflt", "Natural1", "Positive1", "TailPos", "IsEven", "IsOdd", "Nested", "Tail", "TailNat", "Untyped", "Inherits", "Local", "LocalPos", "NamedCall", "Redeclares"}
	for _, name := range eligible {
		if !eligibility(t, ctx, scope, name) {
			t.Errorf("%s is ineligible, want eligible", name)
		}
	}
	ineligible := []string{"UsesUsage", "Stringy", "NonLiteralDefault", "Collects", "CallsIneligible", "CycleA", "CycleB", "Twice", "WithOut", "UnknownName", "ReceiverNamed", "LocalEnum"}
	for _, name := range ineligible {
		if eligibility(t, ctx, scope, name) {
			t.Errorf("%s is eligible, want ineligible", name)
		}
		if sym := findSymbolByName(scope, name, ast.DefCalc); ctx.model.calcShapes[sym].ineligibleWhy == "" {
			t.Errorf("%s records no reason for its ineligibility", name)
		}
	}
	for _, name := range ineligible {
		wantSameOutcome(t, name, intArg(3))
	}
	wantErrorIs(t, "UnknownName", wantSameOutcome(t, "UnknownName", intArg(3)), ErrUnknownParameter)
	wantErrorIs(t, "ReceiverNamed", wantSameOutcome(t, "ReceiverNamed", intArg(3)), ErrReceiverWithNamedArgs)
}

// A traced run takes the evaluator, so the trace records every sub-expression.
func TestCompiledCalcTraceFallsBack(t *testing.T) {
	var traces [2]string
	for i, compile := range []bool{true, false} {
		scope, ctx := compileRuntime(t, compile)
		tr := NewTraceRecorder()
		ctx.SetTrace(tr)
		got := invokeOutcome(t, ctx, scope, "Fib", intArg(5))
		wantOutcomeInt(t, "Fib(5)", got, 5)
		traces[i] = tr.String()
		sym := findSymbolByName(scope, "Fib", ast.DefCalc)
		shape, err := ctx.calcShapeOf(sym)
		if err != nil {
			t.Fatal(err)
		}
		if shape.compileState != compileUndecided {
			t.Errorf("compile=%v: a traced run compiled Fib", compile)
		}
	}
	if traces[0] != traces[1] {
		t.Errorf("traces differ:\n%s\n---\n%s", traces[0], traces[1])
	}
	if !strings.Contains(traces[0], "eval operator -") {
		t.Errorf("trace records no sub-expression:\n%s", traces[0])
	}
}

// Named arguments and non-scalar values take the evaluator.
func TestCompiledCalcDeclines(t *testing.T) {
	scope, ctx := compileRuntime(t, true)
	sym := findSymbolByName(scope, "Add", ast.DefCalc)
	named, err := ctx.InvokeCalcNamed(sym, map[string]Value{"b": intArg(2), "a": intArg(1)}, scope)
	if err != nil || named.Const.Int != 3 {
		t.Fatalf("Add(a = 1, b = 2) = %s, %v", FormatTraceValue(named), err)
	}
	str := wantSameOutcome(t, "Add", NewStringValue("x"), intArg(1))
	wantErrorIs(t, "Add", str, ErrTypeMismatch)
	null := wantSameOutcome(t, "Add", Value{Kind: ValNull}, intArg(1))
	if null.err == nil {
		t.Fatal("Add(null, 1) succeeded")
	}
}

// OPENSYSML_CALC_COMPILE=0 turns the tier off; anything else leaves it on.
func TestCalcCompileFromEnv(t *testing.T) {
	for raw, want := range map[string]bool{"": true, "1": true, "yes": true, "0": false, " false ": false, "OFF": false, "no": false} {
		if got := calcCompileFromValue(raw); got != want {
			t.Errorf("calcCompileFromValue(%q) = %v, want %v", raw, got, want)
		}
	}
	t.Setenv(CalcCompileEnvVar, "0")
	_, ctx := compileRuntimeFromEnv(t)
	if ctx.CalcCompile() {
		t.Fatalf("%s=0 left the compiled tier on", CalcCompileEnvVar)
	}
	t.Setenv(CalcCompileEnvVar, "")
	t.Setenv("SYSML_CALC_COMPILE", "0")
	_, ctx = compileRuntimeFromEnv(t)
	if ctx.CalcCompile() {
		t.Fatal("SYSML_CALC_COMPILE=0 left the compiled tier on")
	}
	t.Setenv("SYSML_CALC_COMPILE", "")
	scope, ctx := compileRuntimeFromEnv(t)
	if !ctx.CalcCompile() {
		t.Fatal("the compiled tier is off by default")
	}
	if !eligibility(t, ctx, scope, "Fib") {
		t.Fatal("Fib did not compile with the tier on")
	}
}

func compileRuntimeFromEnv(t *testing.T) (*symbols.Scope, *Context) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, compileModel))
	return idx.DocumentRoot("<test>"), ctx
}
