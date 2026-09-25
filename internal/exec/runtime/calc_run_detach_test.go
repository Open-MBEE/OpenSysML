package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// detachModel yields bodies that name outputs of the calc they escape from:
// Window binds its result to a body naming two other outputs, and Cyclic's
// limit is worked out by applying the very body that names it.
const detachModel = `
package test {
	private import ScalarValues::*;
	private import ControlFunctions::*;
	private import CollectionFunctions::*;

	calc def Window {
		in n : Integer;
		out threshold : Integer = n;
		out limit : Integer = threshold * 10;
		out pred : expr = { in x; x > threshold and x < limit };
		bind result = pred;
	}
	calc def Cyclic {
		in n : Integer;
		out pred : expr = { in x; x > limit };
		out limit : Integer = size((n)->select pred);
		bind result = pred;
	}
	calc def Double { in n : Integer; return : Integer = n * 2; }
}
`

func detachRuntime(t *testing.T) (*symbols.Scope, *Context) {
	t.Helper()

	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, detachModel))
	ctx.SetCalcCompile(false)
	return idx.DocumentRoot("<test>"), ctx
}

func escapedRun(t *testing.T, body Value) *calcRun {
	t.Helper()

	closure, ok := body.ref.(*exprValue)
	if !ok || body.Kind != ValExpr || closure.env == nil {
		t.Fatalf("got %s, want a body closing over its environment", describeValue(body))
	}
	if closure.env.calcRun == nil {
		t.Fatal("the escaped body holds no calc evaluation to read outputs from")
	}
	return closure.env.calcRun
}

// A body escaping the calc that bound it to its result holds that evaluation
// over storage of its own: applied after other invocations have reused the frame
// the calc ran in, it works its outputs out from the parameter it was invoked
// with, and memoizes them for the next application.
func TestEscapedBodyReadsCalcOutputsAfterFrameReuse(t *testing.T) {
	scope, ctx := detachRuntime(t)
	body, err := invokeInts(t, ctx, scope, "Window", 2)
	if err != nil {
		t.Fatalf("Window(2): %v", err)
	}
	run := escapedRun(t, body)
	if run.env.slots != nil {
		t.Fatal("the escaped body's evaluation still reads the invocation's pooled slots")
	}
	if _, ok := run.outputs["threshold"]; ok {
		t.Fatal("threshold was worked out before any application named it")
	}
	wantInvokedInt(t, ctx, scope, "Double", 2000, 1000)
	wantInvokedInt(t, ctx, scope, "Double", 14, 7)

	ec := NewEvalContext(ctx, scope)
	for _, tc := range []struct {
		x    int64
		want bool
	}{{2, false}, {3, true}, {19, true}, {25, false}} {
		got, err := ec.applyPredicate("select", body, Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: tc.x}})
		if err != nil {
			t.Fatalf("Window(2)(%d): %v", tc.x, err)
		}
		if got != tc.want {
			t.Errorf("Window(2)(%d) = %v, want %v", tc.x, got, tc.want)
		}
	}
	for _, name := range []string{"threshold", "limit"} {
		if _, ok := run.outputs[name]; !ok {
			t.Errorf("%s is not memoized in the evaluation the body holds", name)
		}
	}
	wantFramesReleased(t, ctx)
}

// An output worked out by applying a body that names that same output is a
// cycle, reported as one through the evaluation the body holds.
func TestEscapedBodyReportsOutputCycle(t *testing.T) {
	scope, ctx := detachRuntime(t)
	body, err := invokeInts(t, ctx, scope, "Cyclic", 2)
	if err != nil {
		t.Fatalf("Cyclic(2): %v", err)
	}
	_, err = NewEvalContext(ctx, scope).applyPredicate("select", body, Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 5}})
	if !errors.Is(err, ErrCyclicOutput) {
		t.Fatalf("Cyclic(2)(5): %v, want %v", err, ErrCyclicOutput)
	}
	wantFramesReleased(t, ctx)
}
