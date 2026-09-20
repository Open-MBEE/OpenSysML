package runtime

import (
	"errors"
	"testing"
)

// TestRuntimeRobustnessTerminateBlock exercises the edges of `terminate` in a
// braced entry/do/exit block, which the terminate ends as one performance.
func TestRuntimeRobustnessTerminateBlock(t *testing.T) {
	t.Run("terminate_as_the_only_statement_of_a_block", testTerminateAsTheOnlyStatementOfABlock)
	t.Run("second_terminate_of_an_ended_block_never_runs", testSecondTerminateOfAnEndedBlockNeverRuns)
	t.Run("terminate_in_a_loop_ends_the_block_once", testTerminateInALoopEndsTheBlockOnce)
	t.Run("terminate_of_a_state_name_in_a_block", testTerminateOfAStateNameInABlock)
}

// A block made of one `terminate;` has nothing else to cut; the state stays
// active and its do block still runs.
func testTerminateAsTheOnlyStatementOfABlock(t *testing.T) {
	outputs, visits, err := executeStateSource(t, "Machine", `package test {
		private import ScalarValues::*;
		state def Machine {
			attribute d : Integer = 0;
			entry; then s;
			state s {
				entry { terminate; }
				do { assign d := 1; }
				exit { terminate; }
			}
		}
	}`)
	if err != nil {
		t.Fatalf("error = %v, want none", err)
	}
	if got := outputs["d"].Const.Int; got != 1 {
		t.Fatalf("d = %d, want 1", got)
	}
	if len(visits) != 1 || visits[0] != "s" {
		t.Fatalf("visits = %v, want [s]", visits)
	}
}

// The first `terminate;` ends the block, so a second one written after it is
// never reached: the block's performance is not ended twice.
func testSecondTerminateOfAnEndedBlockNeverRuns(t *testing.T) {
	outputs, _, err := executeStateSource(t, "Machine", `package test {
		private import ScalarValues::*;
		state def Machine {
			attribute e : Integer = 0;
			entry; then s;
			state s {
				entry { assign e := 1; terminate; assign e := 5; terminate; assign e := 9; }
			}
		}
	}`)
	if err != nil {
		t.Fatalf("error = %v, want none", err)
	}
	if got := outputs["e"].Const.Int; got != 1 {
		t.Fatalf("e = %d, want 1", got)
	}
}

// A `terminate;` in a loop of a braced block ends the block at its first
// iteration; later iterations and the statements after the loop do not run.
func testTerminateInALoopEndsTheBlockOnce(t *testing.T) {
	outputs, _, err := executeStateSource(t, "Machine", `package test {
		private import ScalarValues::*;
		state def Machine {
			attribute e : Integer = 0;
			entry; then s;
			state s {
				entry {
					assign e := 1;
					while e < 3 { assign e := e + 1; terminate; }
					assign e := 9;
				}
			}
		}
	}`)
	if err != nil {
		t.Fatalf("error = %v, want none", err)
	}
	if got := outputs["e"].Const.Int; got != 2 {
		t.Fatalf("e = %d, want 2", got)
	}
}

// `terminate s;` naming the state whose block it stands in names no performance
// a terminate can end, in a braced block as in a named action.
func testTerminateOfAStateNameInABlock(t *testing.T) {
	_, _, err := executeStateSource(t, "Machine", `package test {
		private import ScalarValues::*;
		state def Machine {
			attribute e : Integer = 0;
			entry; then s;
			state s {
				entry { assign e := 1; terminate s; assign e := 9; }
			}
		}
	}`)
	if !errors.Is(err, ErrTerminateTarget) {
		t.Fatalf("error = %v, want ErrTerminateTarget", err)
	}
}
