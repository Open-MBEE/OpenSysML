package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessBracedBlock exercises the edges of a braced `entry { … }`,
// `do { … }`, `exit { … }` or transition `do { … }`, each one anonymous action:
// an empty block, a block of one `terminate`, a name the block does not bind, and
// a block-local attribute reached from outside the block. Each is a typed error
// or a clean run, never a panic or a hang.
func TestRuntimeRobustnessBracedBlock(t *testing.T) {
	t.Run("empty_blocks_run_to_completion", testEmptyBracedBlocksRunToCompletion)
	t.Run("block_of_only_terminate_ends_the_block", testBlockOfOnlyTerminateEndsTheBlock)
	t.Run("unbound_name_in_a_block", testUnboundNameInABracedBlock)
	t.Run("block_local_attribute_reached_from_outside", testBlockLocalAttributeReachedFromOutside)
}

// testEmptyBracedBlocksRunToCompletion: `entry { }`, `do { }`, `exit { }` and a
// transition's `do { }` declare an action of no statements; the state completes.
func testEmptyBracedBlocksRunToCompletion(t *testing.T) {
	_, visited, err := executeStateSource(t, "Machine", `package test {
		state def Machine {
			entry; then s;
			state s {
				entry { }
				do { }
				exit { }
			}
			state t;
			transition first s do { } then t;
		}
	}`)
	if err != nil {
		t.Fatalf("error = %v, want none", err)
	}
	if got := strings.Join(visited, ","); got != "s,t" {
		t.Fatalf("visited = %q, want s,t", got)
	}
}

// testBlockOfOnlyTerminateEndsTheBlock: a block holding a lone `terminate;` ends
// itself and nothing else; the machine goes on to the next state.
func testBlockOfOnlyTerminateEndsTheBlock(t *testing.T) {
	outputs, visited, err := executeStateSource(t, "Machine", `package test {
		private import ScalarValues::*;
		state def Machine {
			attribute n : Integer = 0;
			entry; then s;
			state s {
				entry { terminate; }
				do { terminate; }
				exit { terminate; }
			}
			state t { entry { assign n := 1; } }
			transition first s do { terminate; } then t;
		}
	}`)
	if err != nil {
		t.Fatalf("error = %v, want none", err)
	}
	if got := strings.Join(visited, ","); got != "s,t" {
		t.Fatalf("visited = %q, want s,t", got)
	}
	if got := outputs["n"]; got.Const.Int != 1 {
		t.Fatalf("n = %v, want 1", got)
	}
}

// testUnboundNameInABracedBlock: a statement reading a name nothing declares is
// refused as an unresolved reference, the error naming the block's state.
func testUnboundNameInABracedBlock(t *testing.T) {
	_, _, err := executeStateSource(t, "Machine", `package test {
		private import ScalarValues::*;
		state def Machine {
			attribute n : Integer = 0;
			entry; then s;
			state s {
				entry { assign n := nowhere + 1; }
			}
		}
	}`)
	if !errors.Is(err, ErrUnresolvedReference) {
		t.Fatalf("error = %v, want ErrUnresolvedReference", err)
	}
	if !strings.Contains(err.Error(), "s") {
		t.Fatalf("error = %v, want it to name the state", err)
	}
}

// testBlockLocalAttributeReachedFromOutside: an attribute a braced block declares
// is the anonymous action's, so a statement outside the block does not see it.
func testBlockLocalAttributeReachedFromOutside(t *testing.T) {
	_, _, err := executeStateSource(t, "Machine", `package test {
		private import ScalarValues::*;
		state def Machine {
			attribute n : Integer = 0;
			entry; then s;
			state s {
				entry { attribute k : Integer = 2; }
				exit { assign n := k; }
			}
			state t;
			transition first s then t;
		}
	}`)
	if !errors.Is(err, ErrUnresolvedReference) {
		t.Fatalf("error = %v, want ErrUnresolvedReference", err)
	}
}
