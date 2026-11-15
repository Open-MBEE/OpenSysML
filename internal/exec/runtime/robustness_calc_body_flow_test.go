package runtime

import (
	"errors"
	"testing"
)

// TestRuntimeRobustnessCalcBodyFlow covers the failure modes of a token flow a
// calculation body, or a loop or branch block of it, states of its own: each
// ends in a typed error rather than a hang or a silently dropped flow.
func TestRuntimeRobustnessCalcBodyFlow(t *testing.T) {
	t.Run("start_without_successor_is_an_invalid_flow", func(t *testing.T) {
		err := invokeCalcInSource(t, `package test {
			private import ScalarValues::*;
			calc host {
				in n : Integer;
				first start;
				return : Integer = n;
			}
		}`, "host", 1, 100)
		if !errors.Is(err, errInvalidActionFlow) {
			t.Fatalf("error = %v, want errInvalidActionFlow", err)
		}
	})
	t.Run("join_missing_a_branch_deadlocks", func(t *testing.T) {
		err := invokeCalcInSource(t, `package test {
			private import ScalarValues::*;
			calc host {
				in n : Integer;
				first start then d;
				decide d;
					if n > 0 then a;
					else b;
				action a;
				action b;
				join j;
				first a then j;
				first b then j;
				first j then done;
				return : Integer = n;
			}
		}`, "host", 1, 100)
		if !errors.Is(err, ErrActionDeadlock) {
			t.Fatalf("error = %v, want ErrActionDeadlock", err)
		}
	})
	t.Run("loop_block_flow_exhausts_the_step_budget", func(t *testing.T) {
		err := invokeCalcInSource(t, `package test {
			private import ScalarValues::*;
			calc host {
				in n : Integer;
				while n > 0 {
					action a;
					first a;
				}
				return : Integer = n;
			}
		}`, "host", 1, 100)
		if !errors.Is(err, ErrStepLimitExceeded) {
			t.Fatalf("error = %v, want ErrStepLimitExceeded", err)
		}
	})
	t.Run("accept_in_block_flow_deadlocks", func(t *testing.T) {
		err := invokeCalcInSource(t, `package test {
			private import ScalarValues::*;
			attribute def Never;
			calc host {
				in n : Integer;
				if n > 0 {
					action wait { accept Never; }
					action after;
					first wait then after;
				}
				return : Integer = n;
			}
		}`, "host", 1, 100)
		if !errors.Is(err, ErrAcceptDeadlock) {
			t.Fatalf("error = %v, want ErrAcceptDeadlock", err)
		}
	})
}
