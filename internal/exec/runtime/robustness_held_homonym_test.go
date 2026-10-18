package runtime

import (
	"errors"
	"testing"
)

// TestRuntimeRobustnessHeldHomonym: a feature a frame holds without a value is read as
// unset, never falling through to a same-named feature an enclosing frame holds.
func TestRuntimeRobustnessHeldHomonym(t *testing.T) {
	t.Run("unset_state_attribute_is_no_value_beside_a_machine_homonym", testHeldHomonymUnsetStateAttributeIsNoValue)
}

func testHeldHomonymUnsetStateAttributeIsNoValue(t *testing.T) {
	_, _, err := executeStateSource(t, "Modes", `package test {
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
	var noValue *NoValueError
	if !errors.Is(err, ErrNoValue) || !errors.As(err, &noValue) {
		t.Fatalf("error = %v, want ErrNoValue for the state's unset attribute x", err)
	}
	if noValue.Feature != "x" {
		t.Errorf("NoValueError.Feature = %q, want x", noValue.Feature)
	}
}
