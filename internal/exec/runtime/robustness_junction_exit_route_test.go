package runtime

import (
	"strings"
	"testing"
	"time"
)

// TestRuntimeRobustnessJunctionExitRoute: a compound transition that leaves a
// composite state through a junction of its own fails with a typed error, never
// a hang or a panic, when the route past the junction cannot be taken.
func TestRuntimeRobustnessJunctionExitRoute(t *testing.T) {
	t.Run("junction_of_the_left_state_with_no_outgoing_transition", testJunctionOfTheLeftStateWithNoOutgoingTransition)
	t.Run("junction_of_the_left_state_leading_into_history", testJunctionOfTheLeftStateLeadingIntoHistory)
	t.Run("junction_of_the_left_state_whose_every_branch_is_closed", testJunctionOfTheLeftStateWhoseEveryBranchIsClosed)
}

// runRouteToError drives one Go through the machine and returns the error the
// run ends with, failing the test if the run hangs or succeeds.
func runRouteToError(t *testing.T, exec *StateExecutor) error {
	t.Helper()
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	exec.SendSignal("Go", nil)
	done := make(chan error, 1)
	go func() { done <- exec.RunToCompletion() }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected the run to fail at the junction the composite state is left through")
		}
		return err
	case <-watchdog(10 * time.Second):
		t.Fatal("RunToCompletion hung on a junction the composite state is left through")
	}
	return nil
}

// testJunctionOfTheLeftStateWithNoOutgoingTransition: the nested source's exit
// leads to a junction of its owner that no transition leaves; the error names it.
func testJunctionOfTheLeftStateWithNoOutgoingTransition(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		attribute def Go;
		state Machine {
			entry; then S1;
			state S1 {
				entry; then A;
				state A;
				junction XP;
				transition first A accept Go then XP;
			}
			state S2;
		}
	}`)
	err := runRouteToError(t, exec)
	if !strings.Contains(err.Error(), "junction XP has no outgoing transitions") {
		t.Errorf("expected the error to name the junction, got %v", err)
	}
}

// testJunctionOfTheLeftStateLeadingIntoHistory: a junction the owner is left
// through cannot lead on into a sibling's history; the refusal names both.
func testJunctionOfTheLeftStateLeadingIntoHistory(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		attribute def Go;
		state Machine {
			entry; then S1;
			state S1 {
				entry; then A;
				state A;
				junction XP;
				transition first A accept Go then XP;
			}
			state S2 {
				entry; then B;
				state B;
				history H;
			}
			transition first S1::XP then S2::H;
		}
	}`)
	err := runRouteToError(t, exec)
	if !strings.Contains(err.Error(), "junction XP: a transition into shallow history H is not supported") {
		t.Errorf("expected the refusal to name the junction and the history, got %v", err)
	}
}

// testJunctionOfTheLeftStateWhoseEveryBranchIsClosed: a junction's guards are
// read before the compound transition fires; when none holds the run fails at
// the junction before any exit or effect runs.
func testJunctionOfTheLeftStateWhoseEveryBranchIsClosed(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		private import ScalarValues::*;
		attribute def Go;
		state Machine {
			attribute log : String = "";
			attribute x : Integer = 0;
			entry; then S1;
			state S1 {
				exit action { assign log := log + "S1(exit);"; }
				entry; then A;
				state A { exit action { assign log := log + "A(exit);"; } }
				junction XP;
				transition first A accept Go do assign x := 2 then XP;
			}
			state S2;
			transition first S1::XP if x == 1 then S2;
		}
	}`)
	err := runRouteToError(t, exec)
	if !strings.Contains(err.Error(), "junction XP: no guard evaluated to true") {
		t.Errorf("expected the error to name the junction, got %v", err)
	}
	if log := FormatValue(exec.StateData()["log"]); log != `""` {
		t.Errorf("log is %s, want empty: nothing is left when no branch out of the junction holds", log)
	}
}
