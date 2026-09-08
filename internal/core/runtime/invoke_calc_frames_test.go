package runtime

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// framesModel exercises the frames a recursion runs in: fib reads its parameter
// again after each recursive call returns, and F binds an input from a nested
// usage that reads another parameter, so a frame reused across invocations
// would show as a wrong answer.
const framesModel = `
package test {
	private import ScalarValues::*;
	calc def Fib {
		in k : Integer;
		return : Integer = if k <= 1 ? k else Fib(k - 1) + Fib(k - 2);
	}
	calc def Twice { in n : Integer; out a = n * 2; }
	calc def F {
		in k : Integer;
		in x : Integer = tw.a;
		calc tw : Twice { in n = k; }
		return : Integer = x + 1;
	}
	calc def Both { in p : Integer; in q : Integer; return : Integer = F(p) * 100 + F(q); }
	calc def Fail {
		in k : Integer;
		return : Integer = if k <= 0 ? 1 / k else Fail(k - 1) + k;
	}
}
`

// wideCalcModel adds Wide, a recursion binding more names per frame than a free
// frame keeps storage for.
func wideCalcModel() string {
	var params strings.Builder
	for i := 0; i <= maxPooledBindings; i++ {
		fmt.Fprintf(&params, "\t\tin p%d : Integer = k;\n", i)
	}
	wide := fmt.Sprintf(`
	calc def Wide {
		in k : Integer;
%s		return : Integer = if k <= 0 ? p0 else Wide(k - 1) + p%d;
	}
`, params.String(), maxPooledBindings)
	return strings.Replace(framesModel, "\tcalc def Fail {", wide+"\tcalc def Fail {", 1)
}

func framesRuntime(t *testing.T) (*symbols.Scope, *Context) {
	t.Helper()

	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, wideCalcModel()))
	ctx.maxSteps = DefaultMaxSteps
	// The frames under test are the evaluator's; a compiled body runs in none.
	ctx.SetCalcCompile(false)
	return idx.DocumentRoot("<test>"), ctx
}

func invokeInts(t *testing.T, ctx *Context, scope *symbols.Scope, name string, args ...int64) (Value, error) {
	t.Helper()

	sym := findSymbolByName(scope, name, ast.DefCalc)
	if sym == nil {
		t.Fatalf("calc %s not found", name)
	}
	values := make([]Value, len(args))
	for i, arg := range args {
		values[i] = Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: arg}}
	}
	return ctx.InvokeCalc(sym, values, scope)
}

func wantInvokedInt(t *testing.T, ctx *Context, scope *symbols.Scope, name string, want int64, args ...int64) {
	t.Helper()

	result, err := invokeInts(t, ctx, scope, name, args...)
	if err != nil {
		t.Fatalf("%s(%v): %v", name, args, err)
	}
	if result.Kind != ValConst || result.Const.Kind != semantics.ValInt || result.Const.Int != want {
		t.Fatalf("%s(%v) = %s, want %d", name, args, FormatTraceValue(result), want)
	}
}

// wantFramesReleased requires every invocation to have given its frame back
// emptied: nothing bound in it is held past its return, and no run it memoized
// outlives it.
func wantFramesReleased(t *testing.T, ctx *Context) {
	t.Helper()

	if ctx.calcDepth != 0 {
		t.Errorf("calc depth %d after every invocation returned, want 0", ctx.calcDepth)
	}
	if len(ctx.run.calcUsageRuns) != 0 {
		t.Errorf("%d activations still hold calc usage runs after every invocation returned", len(ctx.run.calcUsageRuns))
	}
	if len(ctx.freeInvocationFrames) > maxFreeInvocationFrames {
		t.Errorf("%d frames kept, want at most %d", len(ctx.freeInvocationFrames), maxFreeInvocationFrames)
	}
	for i, frame := range ctx.freeInvocationFrames {
		if len(frame.bindings) != 0 {
			t.Errorf("free frame %d still binds %d names", i, len(frame.bindings))
		}
		if frame.ec.ctx != nil || frame.ec.frames != nil || frame.engine.env != nil || frame.host.ctx != nil {
			t.Errorf("free frame %d still refers to the invocation that released it", i)
		}
	}
}

// A recursion reads each frame's parameter after the calls under it returned,
// so the frames of active invocations are distinct and each is bound once.
func TestRecursiveCalcFramesDoNotAlias(t *testing.T) {
	scope, ctx := framesRuntime(t)
	wantInvokedInt(t, ctx, scope, "Fib", 610, 15)
	wantFramesReleased(t, ctx)
	if len(ctx.freeInvocationFrames) == 0 {
		t.Fatal("no frame kept after the recursion returned")
	}

	// A second recursion runs in the frames the first left, to the same answer.
	kept := len(ctx.freeInvocationFrames)
	wantInvokedInt(t, ctx, scope, "Fib", 6765, 20)
	wantFramesReleased(t, ctx)
	if len(ctx.freeInvocationFrames) < kept {
		t.Errorf("%d frames kept after the second recursion, fewer than the %d before it", len(ctx.freeInvocationFrames), kept)
	}
}

// An input default reading a nested usage answers within its own invocation:
// the next invocation with another argument binds the usage again rather than
// reading what the first one computed.
func TestCalcDefaultReadsNestedUsagePerInvocation(t *testing.T) {
	scope, ctx := framesRuntime(t)
	wantInvokedInt(t, ctx, scope, "Both", 711, 3, 5)
	wantInvokedInt(t, ctx, scope, "F", 7, 3)
	wantInvokedInt(t, ctx, scope, "F", 11, 5)
	wantFramesReleased(t, ctx)
}

// A recursion binding many names per frame gives its frames back without their
// maps, which clearing would have kept at that width; later invocations bind
// into fresh ones.
func TestWideRecursionDropsOversizedBindings(t *testing.T) {
	scope, ctx := framesRuntime(t)
	wantInvokedInt(t, ctx, scope, "Wide", 55, 10)
	wantFramesReleased(t, ctx)
	if len(ctx.freeInvocationFrames) == 0 {
		t.Fatal("no frame kept after the recursion returned")
	}
	for i, frame := range ctx.freeInvocationFrames {
		if frame.bindings != nil {
			t.Errorf("free frame %d keeps a map that bound %d names", i, maxPooledBindings+2)
		}
	}

	// Deeper than Wide went, so every kept frame runs a narrow invocation.
	wantInvokedInt(t, ctx, scope, "Fib", 144, 12)
	wantFramesReleased(t, ctx)
	for i, frame := range ctx.freeInvocationFrames {
		if frame.bindings == nil {
			t.Errorf("free frame %d has no map after a narrow recursion ran in it", i)
		}
	}
}

// A recursion failing at its deepest frame unwinds every frame above it, and the
// context evaluates the next invocation as if it had not failed.
func TestFailingRecursionReleasesFrames(t *testing.T) {
	scope, ctx := framesRuntime(t)
	if _, err := invokeInts(t, ctx, scope, "Fail", 12); err == nil {
		t.Fatal("Fail(12) evaluated, want a division by zero")
	}
	wantFramesReleased(t, ctx)
	wantInvokedInt(t, ctx, scope, "Fib", 55, 10)
	wantFramesReleased(t, ctx)
}

// A recursion beyond the depth budget reports the budget, and the context is
// left with every frame released and the depth budget whole.
func TestRecursionBeyondDepthBudgetReleasesFrames(t *testing.T) {
	scope, ctx := framesRuntime(t)
	ctx.maxCalcDepth = 16
	_, err := invokeInts(t, ctx, scope, "Fib", 40)
	if !errors.Is(err, ErrCalcRecursionLimit) {
		t.Fatalf("Fib(40) under a depth budget of 16: %v, want %v", err, ErrCalcRecursionLimit)
	}
	wantFramesReleased(t, ctx)
	wantInvokedInt(t, ctx, scope, "Fib", 55, 10)
	wantFramesReleased(t, ctx)
}

// The frames a recursion ran in are recorded as they nest, whether or not the
// frames were reused: each invocation enters, binds and exits in order.
func TestRecursiveCalcTraceNestsPerInvocation(t *testing.T) {
	scope, ctx := framesRuntime(t)
	wantInvokedInt(t, ctx, scope, "Fib", 5, 5)

	tr := NewTraceRecorder()
	ctx.SetTrace(tr)
	wantInvokedInt(t, ctx, scope, "Fib", 3, 4)
	ctx.SetTrace(nil)

	trace := tr.String()
	if enters, exits := strings.Count(trace, "enter calc test::Fib"), strings.Count(trace, "exit calc test::Fib ->"); enters != 9 || exits != 9 {
		t.Errorf("Fib(4) traced %d enters and %d exits, want 9 of each:\n%s", enters, exits, trace)
	}
	if !strings.Contains(trace, "bind k = 4 [argument]") || !strings.Contains(trace, "exit calc test::Fib -> 3") {
		t.Errorf("trace does not bind the outermost argument and yield its result:\n%s", trace)
	}
	wantFramesReleased(t, ctx)
}
