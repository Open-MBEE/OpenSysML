package runtime

import (
	"slices"
	"testing"
)

// TestRuntimeRobustnessHeldHomonym: a feature a frame holds without a value is read as
// unset, never falling through to a same-named feature an enclosing frame holds.
func TestRuntimeRobustnessHeldHomonym(t *testing.T) {
	t.Run("unset_state_attribute_is_no_value_beside_a_machine_homonym", testHeldHomonymUnsetStateAttributeIsNoValue)
}

// Both guards read idle's unset x, so neither holds and the machine rests in wait; a read
// falling through to the machine's x = 7 would take high.
func testHeldHomonymUnsetStateAttributeIsNoValue(t *testing.T) {
	_, states, err := executeStateSource(t, "Modes", `package test {
		private import ScalarValues::*;
		state def Modes {
			attribute x : Integer = 7;
			entry; then idle;
			state idle {
				attribute x : Integer;
				entry; then wait;
				state wait;
				state high;
				state low;
				transition first wait if x > 5 then high;
				transition first wait if x <= 5 then low;
			}
		}
	}`)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !slices.Contains(states, "wait") || slices.Contains(states, "high") || slices.Contains(states, "low") {
		t.Fatalf("states = %v, want wait held and neither high nor low entered", states)
	}
}
