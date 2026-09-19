package runtime

import (
	"errors"
	"testing"
)

// snapshotModel reads one output of a usage, changes what the usage's input
// named, then reads another output. Both readings must answer from the binding
// the first read made, so the two calculations agree.
const snapshotModel = `
package test {
	private import ScalarValues::*;
	calc def Pair {
		in k : Real;
		out a = k;
		out b = k * 10.0;
	}
	calc def Interleaved {
		attribute v : Real = 1.0;
		calc p : Pair { in k = v; }
		attribute ra : Real = p.a;
		assign v := 2.0;
		attribute rb : Real = p.b;
		ra * 1000.0 + rb
	}
	calc def Snapshotted {
		attribute v : Real = 1.0;
		calc p : Pair { in k = v; }
		attribute ra : Real = p.a;
		attribute rb : Real = p.b;
		assign v := 2.0;
		ra * 1000.0 + rb
	}
}
`

// TestCalcUsageOutputsShareOneInputBinding requires the order the outputs of a
// usage are read in to be unobservable: a change to a feature the usage's inputs
// named does not retroactively rebind them, so `a` and `b` cannot come from two
// states of the same feature.
func TestCalcUsageOutputsShareOneInputBinding(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, snapshotModel))
	root := idx.DocumentRoot("<test>")

	interleaved, scope := calcByName(t, root, "test", "Interleaved")
	snapshotted, _ := calcByName(t, root, "test", "Snapshotted")

	got, err := ctx.InvokeCalc(interleaved, nil, scope)
	if err != nil {
		t.Fatalf("Interleaved: %v", err)
	}
	want, err := ctx.InvokeCalc(snapshotted, nil, scope)
	if err != nil {
		t.Fatalf("Snapshotted: %v", err)
	}
	if got.Const.Real != want.Const.Real {
		t.Errorf("Interleaved = %v, Snapshotted = %v; reading order must not be observable",
			got.Const.Real, want.Const.Real)
	}
	if want.Const.Real != 1010.0 {
		t.Errorf("Snapshotted = %v, want 1010 (both outputs from k = 1.0)", want.Const.Real)
	}
}

// loopStepModel assigns four features from the four outputs of one usage inside a
// loop, then advances the state the usage's inputs read. Each iteration must bind
// that iteration's values, which is the fixed-step integration pattern.
const loopStepModel = `
package test {
	private import ScalarValues::*;
	calc def Rates {
		in px : Real;
		in pv : Real;
		out dx = pv;
		out dv = 0.0 - px;
		out dsum = px + pv;
		out dcount = 1.0;
	}
	calc def Step {
		in n : Integer;
		attribute x : Real = 1.0;
		attribute v : Real = 0.0;
		attribute sum : Real = 0.0;
		attribute count : Real = 0.0;
		attribute i : Integer = 0;
		calc r : Rates { in px = x; in pv = v; }
		while i < n {
			assign x := x + 0.5 * r.dx;
			assign v := v + 0.5 * r.dv;
			assign sum := sum + r.dsum;
			assign count := count + r.dcount;
			assign i := i + 1;
		}
		x * 1000000.0 + v * 10000.0 + sum * 100.0 + count
	}
}
`

// TestCalcUsageOutputsInAssignmentLoop requires the four outputs assigned in one
// iteration to come from one evaluation of the usage, and a later iteration to
// see the state that iteration starts with.
func TestCalcUsageOutputsInAssignmentLoop(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, loopStepModel))
	root := idx.DocumentRoot("<test>")
	step, scope := calcByName(t, root, "test", "Step")

	// One iteration: x = 1 + 0.5*0 = 1, v = 0 + 0.5*(-1) = -0.5, sum = 1, count = 1.
	one, err := ctx.InvokeCalc(step, []Value{constInt(1)}, scope)
	if err != nil {
		t.Fatalf("Step(1): %v", err)
	}
	if one.Const.Real != 1000000.0-5000.0+100.0+1.0 {
		t.Errorf("Step(1) = %v, want %v", one.Const.Real, 1000000.0-5000.0+100.0+1.0)
	}

	// Two iterations: the second reads x = 1, v = -0.5 — the values the first
	// iteration left — so x = 0.75, v = -1.0, sum = 1.5, count = 2.
	two, err := ctx.InvokeCalc(step, []Value{constInt(2)}, scope)
	if err != nil {
		t.Fatalf("Step(2): %v", err)
	}
	want := 0.75*1000000.0 + -1.0*10000.0 + 1.5*100.0 + 2.0
	if two.Const.Real != want {
		t.Errorf("Step(2) = %v, want %v (each iteration binds its own state)", two.Const.Real, want)
	}
}

// distinctArgumentsModel invokes the same calc, whose body reads a nested usage,
// with two different arguments: the memo of the nested usage belongs to the
// enclosing invocation, so the second invocation must not answer the first's.
const distinctArgumentsModel = `
package test {
	private import ScalarValues::*;
	calc def Double {
		in n : Real;
		out twice = n * 2.0;
	}
	calc def Outer {
		in m : Real;
		calc d : Double { in n = m; }
		d.twice
	}
	calc def Both {
		Outer(3.0) * 1000.0 + Outer(5.0)
	}
}
`

// TestCalcUsageMemoDistinguishesEnclosingArguments requires the nested usage's
// evaluation to belong to the invocation that bound its inputs.
func TestCalcUsageMemoDistinguishesEnclosingArguments(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, distinctArgumentsModel))
	root := idx.DocumentRoot("<test>")
	both, scope := calcByName(t, root, "test", "Both")

	got, err := ctx.InvokeCalc(both, nil, scope)
	if err != nil {
		t.Fatalf("Both: %v", err)
	}
	if got.Const.Real != 6010.0 {
		t.Errorf("Both = %v, want 6010 (Outer(3) = 6, Outer(5) = 10)", got.Const.Real)
	}
}

// cyclicOutputModel declares outputs valued from each other, read through a
// usage in a calc body: binding the inputs once gives the cycle no value.
const cyclicOutputModel = `
package test {
	private import ScalarValues::*;
	calc def Knot {
		in n : Real;
		out a = b + 1.0;
		out b = a + n;
	}
	calc def Cycle {
		calc p : Knot { in n = 1.0; }
		p.a
	}
}
`

// TestCyclicCalcUsageOutputStillDiagnosed requires a genuine cycle to fail with
// ErrCyclicOutput rather than to be answered from a stale evaluation.
func TestCyclicCalcUsageOutputStillDiagnosed(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, cyclicOutputModel))
	root := idx.DocumentRoot("<test>")
	cycle, scope := calcByName(t, root, "test", "Cycle")

	_, err := ctx.InvokeCalc(cycle, nil, scope)
	if err == nil {
		t.Fatal("Cycle: want an error, got a value")
	}
	if !errors.Is(err, ErrCyclicOutput) {
		t.Errorf("Cycle: error = %v, want ErrCyclicOutput", err)
	}
}
