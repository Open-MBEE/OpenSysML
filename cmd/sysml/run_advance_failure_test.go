package main

import "testing"

// advanceFailureModel is a worker whose machine indexes an empty sequence in the
// effect of a transition a sibling's timed send triggers through a port.
const advanceFailureModel = `package E {
	private import ScalarValues::*;
	private import SI::*;
	item def Ask;
	port def P { out item ask : Ask; }
	part def W {
		port p : ~P;
		attribute xs : Integer[0..*];
		attribute n : Integer = 0;
		exhibit state run {
			entry; then idle;
			state idle;
			transition first idle accept Ask via p do action { assign n := n + xs#(1); } then idle;
		}
	}
	part def S {
		port a : P;
		exhibit state run {
			entry; then s0;
			state s0;
			transition first s0 accept after 1 [SI::s] do action { send new Ask() via a; } then s0;
		}
	}
	part sys { part s : S; part w : W; connect s.a to w.p; }
}
`

// TestAdvanceReportsAMachineFailingOnTheClock checks that a machine failing as the clock
// advances is a failing outcome of an exploration (status 1) and a failed advance (status 2).
func TestAdvanceReportsAMachineFailingOnTheClock(t *testing.T) {
	binary := buildCLI(t)
	const failure = "fire transition out of idle: transition effect: eval assignment RHS: index out of range"

	got := check(t, binary, advanceFailureModel, "-schedule", "explore:runs=2,depth=200", "-state", "E::W::run E::sys.w", "-advance", "3")
	wantReport(t, got, 1, "✗ explored E::W::run: 1 outcome (0 values, 1 error), 1 of 1 linearization failing", "error: ", failure)

	got = check(t, binary, advanceFailureModel, "-instantiate", "E::sys", "-state", "E::W::run E::sys.w", "-advance", "3")
	wantReport(t, got, 2, "advance failed: ", failure)
}
