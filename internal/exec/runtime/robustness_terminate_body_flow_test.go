package runtime

import (
	"errors"
	"slices"
	"testing"
)

// TestRuntimeRobustnessTerminateBodyFlow exercises a terminate action usage whose
// body states a flow: the flow runs, a `terminate` in its chain ends it early, and
// a flow the successions leave no start to is a typed error at initialize.
func TestRuntimeRobustnessTerminateBodyFlow(t *testing.T) {
	t.Run("stated_flow_runs_and_ends_the_performance", testTerminateBodyFlowRuns)
	t.Run("then_chain_reaches_the_terminate_early", testTerminateBodyFlowEndsEarly)
	t.Run("succession_cycle_leaves_no_start", testTerminateBodyFlowCycle)
	t.Run("succession_to_an_unknown_step", testTerminateBodyFlowUnknownStep)
}

func testTerminateBodyFlowRuns(t *testing.T) {
	outputs, err := executeActionSource(t, "host", `package test {
		private import ScalarValues::*;
		action host {
			out attribute x : Integer = 0;
			out attribute later : Integer = 0;
			first start;
			then stop;
			action stop terminate {
				first start;
				then action inner { assign x := 1; }
				then done;
			}
			then action tail { assign later := 1; }
			then done;
		}
	}`)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	assertIntOutput(t, outputs, "x", 1)
	assertIntOutput(t, outputs, "later", 0)
}

func testTerminateBodyFlowEndsEarly(t *testing.T) {
	m := parseExploreModel(t, `package test {
		private import ScalarValues::*;
		action early {
			out attribute x : Integer = 0;
			out attribute y : Integer = 0;
			first start;
			then stop;
			action stop terminate {
				assign x := 1;
				then terminate;
				then assign y := 5;
			}
			then done;
		}
	}`)
	x := m.exploreAction(t, "explore", "early")
	if !x.Complete() {
		t.Fatalf("exploration %s, want complete", x.Status())
	}
	xs := featureValues(t, x, "x")
	slices.Sort(xs)
	if xs = slices.Compact(xs); !slices.Equal(xs, []string{"0", "1"}) {
		t.Errorf("x over every schedule = %v, want [0 1]: the implicit terminate before or after the assign", xs)
	}
	if ys := slices.Compact(featureValues(t, x, "y")); !slices.Equal(ys, []string{"0"}) {
		t.Errorf("y over every schedule = %v, want [0]: no statement after the terminate is performed", ys)
	}
}

func testTerminateBodyFlowCycle(t *testing.T) {
	_, err := executeActionSource(t, "cycle", `package test {
		private import ScalarValues::*;
		action cycle {
			out attribute x : Integer = 0;
			first start;
			then stop;
			action stop terminate {
				action a { assign x := 1; }
				action b { assign x := 2; }
				succession a then b;
				succession b then a;
			}
			then done;
		}
	}`)
	if !errors.Is(err, ErrInvalidActionFlow) {
		t.Fatalf("error = %v, want ErrInvalidActionFlow", err)
	}
}

func testTerminateBodyFlowUnknownStep(t *testing.T) {
	_, err := executeActionSource(t, "missing", `package test {
		private import ScalarValues::*;
		action missing {
			out attribute x : Integer = 0;
			first start;
			then stop;
			action stop terminate {
				first start;
				then nowhere;
				then done;
			}
			then done;
		}
	}`)
	if !errors.Is(err, ErrInvalidActionFlow) {
		t.Fatalf("error = %v, want ErrInvalidActionFlow", err)
	}
}
