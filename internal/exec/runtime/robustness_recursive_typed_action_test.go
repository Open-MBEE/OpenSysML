package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessRecursiveTypedAction covers typed action bodies nested in
// their own definition: bounded recursion runs, unbounded ends with the budget.
func TestRuntimeRobustnessRecursiveTypedAction(t *testing.T) {
	t.Run("unbounded_self_recursion_exhausts_action_steps", func(t *testing.T) {
		_, err := executeRecursiveTypedAction(t, `package test {
			private import ScalarValues::*;
			action def A {
				attribute c : Integer = 0;
				first start then x;
				action x : A { assign c := c + 1; }
				then done;
			}
		}`, "A", 200)
		if !errors.Is(err, ErrActionStepLimitExceeded) {
			t.Fatalf("ExecuteAction error = %v, want ErrActionStepLimitExceeded", err)
		}
	})

	t.Run("unbounded_recursion_in_loop_body_exhausts_action_steps", func(t *testing.T) {
		_, err := executeRecursiveTypedAction(t, `package test {
			private import ScalarValues::*;
			action def Walk {
				in depth : Integer;
				attribute visited : Integer = 1;
				if depth >= 0 {
					action step : Walk { in depth = depth + 1; assign visited := visited + 1; }
				}
			}
			action run {
				action w : Walk { in depth = 0; }
			}
		}`, "run", 200)
		if !errors.Is(err, ErrActionStepLimitExceeded) {
			t.Fatalf("ExecuteAction error = %v, want ErrActionStepLimitExceeded", err)
		}
	})

	t.Run("unbounded_mutual_recursion_exhausts_action_steps", func(t *testing.T) {
		_, err := executeRecursiveTypedAction(t, `package test {
			private import ScalarValues::*;
			action def Ping {
				attribute c : Integer = 0;
				action pong : Pong { assign c := c + 1; }
			}
			action def Pong {
				attribute c : Integer = 0;
				action ping : Ping { assign c := c + 1; }
			}
		}`, "Ping", 200)
		if !errors.Is(err, ErrActionStepLimitExceeded) {
			t.Fatalf("ExecuteAction error = %v, want ErrActionStepLimitExceeded", err)
		}
	})

	t.Run("deferred_body_result_parameter_is_refused_when_a_body_unfolds_it", func(t *testing.T) {
		_, err := executeRecursiveTypedAction(t, `package test {
			private import ScalarValues::*;
			action def A {
				in depth : Integer;
				if depth > 0 {
					action x : A {
						in depth = depth - 1;
						action bad { return r : Integer; }
					}
				}
			}
			action run {
				action w : A { in depth = 1; }
			}
		}`, "run", 200)
		if !errors.Is(err, ErrActionResultParameter) {
			t.Fatalf("ExecuteAction error = %v, want ErrActionResultParameter", err)
		}
	})

	t.Run("deferred_body_result_parameter_is_refused_when_a_token_unfolds_it", func(t *testing.T) {
		_, err := executeRecursiveTypedAction(t, `package test {
			private import ScalarValues::*;
			action def A {
				first start then x;
				action x : A { action bad { return r : Integer; } }
				then done;
			}
		}`, "A", 200)
		if !errors.Is(err, ErrActionResultParameter) {
			t.Fatalf("ExecuteAction error = %v, want ErrActionResultParameter", err)
		}
	})

	t.Run("bounded_inherited_recursion_runs_to_depth", func(t *testing.T) {
		outputs, err := executeRecursiveTypedAction(t, `package test {
			private import ScalarValues::*;
			action def Base {
				in depth : Integer;
				out reached : Integer = 0;
				if depth > 0 {
					action deeper : Derived { in depth = depth - 1; assign reached := reached + 0; }
					then assign reached := deeper.reached + 1;
				} else {
					assign reached := 1;
				}
			}
			action def Derived :> Base;
			action run {
				attribute total : Integer = 0;
				action d : Derived { in depth = 2; }
				then assign total := d.reached;
			}
		}`, "run", DefaultMaxActionSteps)
		if err != nil {
			t.Fatalf("ExecuteAction error = %v, want bounded recursion to run", err)
		}
		if got := outputs["total"]; got.Kind != ValConst || got.Const.Int != 3 {
			t.Fatalf("total = %v, want 3 (one performance per level)", got)
		}
	})
}

// executeRecursiveTypedAction runs the named action with an action-step budget
// of maxSteps, under the watchdog so a hang fails rather than stalls the suite.
func executeRecursiveTypedAction(t *testing.T, src, name string, maxSteps int64) (map[string]Value, error) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<recursive-typed-action>", parseAndBuild(t, src))
	ctx.maxActionSteps = maxSteps
	sym := findSymbolByName(idx.DocumentRoot("<recursive-typed-action>"), name, ast.DefAction)
	if sym == nil {
		t.Fatalf("action %s not found", name)
	}
	var outputs map[string]Value
	err := runWithWatchdog(t, func() error {
		var err error
		outputs, err = ctx.ExecuteAction(sym)
		return err
	})
	return outputs, err
}
