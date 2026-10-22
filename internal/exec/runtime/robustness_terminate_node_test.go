package runtime

import (
	"errors"
	"testing"
)

// TestRuntimeRobustnessTerminateNode exercises the shapes a named terminate node
// (`action halt terminate <occurrence>;`) refuses, as the bare statement does:
// each is a typed error, never a silent no-op or a panic.
func TestRuntimeRobustnessTerminateNode(t *testing.T) {
	t.Run("terminate_node_of_an_unknown_name", testTerminateNodeOfAnUnknownName)
	t.Run("terminate_node_of_an_ended_performance", testTerminateNodeOfAnEndedPerformance)
	t.Run("state_transition_to_a_terminate_node_of_an_unknown_name", testStateTransitionToTerminateNodeOfAnUnknownName)
}

// testTerminateNodeOfAnUnknownName: a terminate node naming nothing declared
// around it has no performance to end.
func testTerminateNodeOfAnUnknownName(t *testing.T) {
	_, err := executeActionSource(t, "host", `package test {
		action host {
			first start;
			then action c1 { action halt terminate nowhere; }
			then done;
		}
	}`)
	if !errors.Is(err, errTerminateTarget) {
		t.Fatalf("error = %v, want errTerminateTarget", err)
	}
}

// testTerminateNodeOfAnEndedPerformance: `action halt terminate c1;` after c1
// completed names a performance that has already ended, which is reported
// rather than ended twice.
func testTerminateNodeOfAnEndedPerformance(t *testing.T) {
	_, err := executeActionSource(t, "host", `package test {
		private import ScalarValues::*;
		action host {
			out attribute x : Integer = 0;
			first start;
			then action c1 { assign x := 1; }
			then action c2 { action halt terminate c1; }
			then done;
		}
	}`)
	if !errors.Is(err, errPerformanceEnded) {
		t.Fatalf("error = %v, want errPerformanceEnded", err)
	}
}

// testStateTransitionToTerminateNodeOfAnUnknownName: a transition ending at a
// terminate node naming nothing declared around the machine has no occurrence
// to end, which the machine reports rather than ending itself.
func testStateTransitionToTerminateNodeOfAnUnknownName(t *testing.T) {
	_, _, err := executeStateSource(t, "M", `package test {
		state def M {
			entry; then s;
			state s;
			transition first s then halt;
			action halt terminate nowhere;
		}
	}`)
	if !errors.Is(err, errTerminateTarget) {
		t.Fatalf("error = %v, want errTerminateTarget", err)
	}
}
