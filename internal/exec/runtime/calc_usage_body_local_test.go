package runtime

import (
	"strings"
	"testing"
)

// bodyLocalLoopModel declares the usage inside the loop body, so each iteration
// declares it anew: its inputs are bound from the state that iteration starts
// with, and its outputs are read from that iteration's evaluation.
const bodyLocalLoopModel = `
package test {
	private import ScalarValues::*;
	calc def Rates {
		in px : Real;
		in pv : Real;
		out dx = pv;
		out dv = 0.0 - px;
	}
	calc def Propagate {
		in n : Integer;
		attribute x : Real = 1.0;
		attribute v : Real = 0.0;
		attribute i : Integer = 0;
		while i < n {
			calc r : Rates { in px = x; in pv = v; }
			assign x := x + 0.5 * r.dx;
			assign v := v + 0.5 * r.dv;
			assign i := i + 1;
		}
		x * 1000.0 + v
	}
	calc def Branch {
		in k : Real;
		attribute out1 : Real = 0.0;
		if k > 0.0 {
			calc r : Rates { in px = k; in pv = k * 2.0; }
			assign out1 := r.dx + r.dv;
		} else {
			calc r : Rates { in px = 0.0 - k; in pv = 1.0; }
			assign out1 := r.dx;
		}
		out1
	}
	calc def NestedBodies {
		in n : Integer;
		attribute acc : Real = 0.0;
		attribute i : Integer = 0;
		while i < n {
			if i > 0 {
				calc r : Rates { in px = acc; in pv = 2.0; }
				assign acc := acc + r.dx + r.dv;
			}
			assign i := i + 1;
		}
		acc
	}
}
`

// TestBodyLocalCalcUsageInLoopBindsPerIteration requires a usage declared in a
// loop body to be executable and to have per-iteration lifetime.
func TestBodyLocalCalcUsageInLoopBindsPerIteration(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, bodyLocalLoopModel))
	root := idx.DocumentRoot("<test>")
	propagate, scope := calcByName(t, root, "test", "Propagate")

	// Iteration 1 reads x = 1, v = 0 and leaves x = 1, v = -0.5; iteration 2 must
	// read those, leaving x = 0.75, v = -1.
	for _, want := range []struct {
		n     int64
		value float64
	}{
		{1, 1.0*1000.0 - 0.5},
		{2, 0.75*1000.0 - 1.0},
	} {
		got, err := ctx.InvokeCalc(propagate, []Value{constInt(want.n)}, scope)
		if err != nil {
			t.Fatalf("Propagate(%d): %v", want.n, err)
		}
		if got.Const.Real != want.value {
			t.Errorf("Propagate(%d) = %v, want %v", want.n, got.Const.Real, want.value)
		}
	}
}

// TestBodyLocalCalcUsageInBranch requires a usage declared in one branch of a
// conditional to run in that branch's scope, the other branch's declaration of
// the same name being a different usage.
func TestBodyLocalCalcUsageInBranch(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, bodyLocalLoopModel))
	root := idx.DocumentRoot("<test>")
	branch, scope := calcByName(t, root, "test", "Branch")

	// k = 3: dx = 6, dv = -3.
	taken, err := ctx.InvokeCalc(branch, []Value{constReal(3.0)}, scope)
	if err != nil {
		t.Fatalf("Branch(3): %v", err)
	}
	if taken.Const.Real != 3.0 {
		t.Errorf("Branch(3) = %v, want 3", taken.Const.Real)
	}

	// k = -2: the else branch's usage binds pv = 1, so dx = 1.
	other, err := ctx.InvokeCalc(branch, []Value{constReal(-2.0)}, scope)
	if err != nil {
		t.Fatalf("Branch(-2): %v", err)
	}
	if other.Const.Real != 1.0 {
		t.Errorf("Branch(-2) = %v, want 1", other.Const.Real)
	}
}

// TestBodyLocalCalcUsageInNestedBodies requires a usage declared in a conditional
// inside a loop to bind from the iteration it runs in.
func TestBodyLocalCalcUsageInNestedBodies(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, bodyLocalLoopModel))
	root := idx.DocumentRoot("<test>")
	nested, scope := calcByName(t, root, "test", "NestedBodies")

	// The branch runs on iterations 1 and 2 only: acc = 0 -> 2 -> 2 + 2 - 2 = 2.
	got, err := ctx.InvokeCalc(nested, []Value{constInt(3)}, scope)
	if err != nil {
		t.Fatalf("NestedBodies(3): %v", err)
	}
	if got.Const.Real != 2.0 {
		t.Errorf("NestedBodies(3) = %v, want 2", got.Const.Real)
	}
}

// TestBodyLocalUnsupportedDeclarationIsReported requires a body-local
// declaration the runtime cannot execute to be named rather than skipped.
func TestBodyLocalUnsupportedDeclarationIsReported(t *testing.T) {
	src := `
		package test {
			calc def Bad {
				in n : Integer;
				attribute i : Integer = 0;
				while i < n {
					part broken;
					assign i := i + 1;
				}
				i
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	root := idx.DocumentRoot("<test>")
	bad, scope := calcByName(t, root, "test", "Bad")

	_, err := ctx.InvokeCalc(bad, []Value{constInt(1)}, scope)
	if err == nil {
		t.Fatal("want an error naming the declaration, got a value")
	}
	if !strings.Contains(err.Error(), "broken") || !strings.Contains(err.Error(), "not executable") {
		t.Errorf("error = %v, want it to name the unsupported declaration", err)
	}
}
