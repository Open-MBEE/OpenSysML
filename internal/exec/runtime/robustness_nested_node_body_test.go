package runtime

import (
	"errors"
	"strings"
	"testing"
)

func TestRuntimeRobustnessNestedNodeInBody(t *testing.T) {
	t.Run("state_body_flow_unsequenced_statement", func(t *testing.T) {
		exec := stateExecutorForSource(t, "Machine", `package test {
			private import ScalarValues::*;
			state Machine {
				attribute n : Integer = 0;
				entry; then active;
				state active {
					do action work {
						for i in 1..1 {
							action a { assign n := n + 1; }
							action b { assign n := n + 10; }
							assign n := n + 100;
							first a then b;
							succession a then b;
						}
					}
				}
			}
		}`)
		err := exec.RunToCompletion()
		if !errors.Is(err, ErrInvalidActionFlow) {
			t.Fatalf("error = %v, want ErrInvalidActionFlow", err)
		}
		if !strings.Contains(err.Error(), "assignment written directly in an action body") {
			t.Fatalf("error = %v, want the invalid stated-body flow diagnostic", err)
		}
		if n := exec.stateData["n"]; n.Kind != ValConst || n.Const.Int != 0 {
			t.Fatalf("n = %v, want no action side effects", n)
		}
	})
	t.Run("state_body_flow_valid_nested_actions", func(t *testing.T) {
		exec := stateExecutorForSource(t, "Machine", `package test {
			private import ScalarValues::*;
			state Machine {
				attribute n : Integer = 0;
				entry; then active;
				state active {
					do action work {
						for i in 1..1 {
							action a { assign n := n + 1; }
							action b { assign n := n + 10; }
							first a then b;
						}
					}
				}
			}
		}`)
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("error = %v, want successful state body flow", err)
		}
		if n := exec.stateData["n"]; n.Kind != ValConst || n.Const.Int != 11 {
			t.Fatalf("n = %v, want 11", n)
		}
	})
	t.Run("state_body_flow_result_parameter", func(t *testing.T) {
		exec := stateExecutorForSource(t, "Machine", `package test {
			private import ScalarValues::*;
			state Machine {
				attribute n : Integer = 0;
				entry; then active;
				state active {
					do action work {
						for i in 1..1 {
							action a { assign n := n + 1; }
							action b { return r : Integer = 1; }
							first a then b;
						}
					}
				}
			}
		}`)
		err := exec.RunToCompletion()
		if !errors.Is(err, ErrActionResultParameter) {
			t.Fatalf("error = %v, want ErrActionResultParameter", err)
		}
		if n := exec.stateData["n"]; n.Kind != ValConst || n.Const.Int != 0 {
			t.Fatalf("n = %v, want no action side effects", n)
		}
	})
	t.Run("body_flow_with_no_start", func(t *testing.T) {
		_, err := executeActionSource(t, "host", `package test {
			action host {
				first start;
				then action iterate {
					for i in 1..1 {
						action a;
						action b;
						succession a then b;
						succession b then a;
					}
				}
			}
		}`)
		if !errors.Is(err, ErrInvalidActionFlow) {
			t.Fatalf("error = %v, want ErrInvalidActionFlow", err)
		}
	})
	t.Run("body_flow_two_starts", func(t *testing.T) {
		_, err := executeActionSource(t, "host", `package test {
			action host {
				first start;
				then action iterate {
					for i in 1..1 {
						action a;
						action b;
						first a;
						first b;
					}
				}
			}
		}`)
		if !errors.Is(err, ErrInvalidActionFlow) {
			t.Fatalf("error = %v, want ErrInvalidActionFlow", err)
		}
	})
	t.Run("body_flow_unsequenced_statement", func(t *testing.T) {
		_, err := executeActionSource(t, "host", `package test {
			private import ScalarValues::*;
			action host {
				attribute x : Integer = 0;
				first start;
				then action iterate {
					for i in 1..1 {
						action a;
						action b;
						assign x := 1;
						succession a then b;
					}
				}
			}
		}`)
		if !errors.Is(err, ErrInvalidActionFlow) {
			t.Fatalf("error = %v, want ErrInvalidActionFlow", err)
		}
		if errors.Is(err, ErrStatementNotExecutable) {
			t.Fatalf("error = %v, want initialize-time invalid flow", err)
		}
	})
	t.Run("body_flow_pin_read_before_performed", func(t *testing.T) {
		_, err := executeActionSource(t, "host", `package test {
			private import ScalarValues::*;
			action host {
				attribute result : Integer = 0;
				first start;
				then action iterate {
					for i in 1..1 {
						action a { out v : Integer; assign v := 1; }
						action b { assign result := a.v; }
						first b;
						succession b then a;
					}
				}
			}
		}`)
		if !errors.Is(err, ErrNodeNotPerformed) {
			t.Fatalf("error = %v, want ErrNodeNotPerformed", err)
		}
	})
	t.Run("body_flow_that_never_ends", func(t *testing.T) {
		_, err := executeActionSource(t, "host", `package test {
			private import ScalarValues::*;
			action host {
				attribute n : Integer = 0;
				first start;
				then action iterate {
					for i in 1..1 {
						action a { assign n := n + 1; }
						first a;
						succession a then a;
					}
				}
			}
		}`)
		if !errors.Is(err, ErrStepLimitExceeded) && !errors.Is(err, ErrActionDeadlock) {
			t.Fatalf("error = %v, want a step-limit or deadlock error", err)
		}
	})
	t.Run("body_flow_accept_deadlocks", func(t *testing.T) {
		_, err := executeActionSource(t, "host", `package test {
			attribute def Never;
			action host {
				first start;
				then action iterate {
					for i in 1..1 {
						action a accept Never;
						first a;
					}
				}
			}
		}`)
		if !errors.Is(err, ErrActionDeadlock) && !errors.Is(err, ErrAcceptDeadlock) {
			t.Fatalf("error = %v, want a typed deadlock error", err)
		}
	})
	t.Run("accept_in_declaration_order_body_not_executable", func(t *testing.T) {
		_, err := executeActionSource(t, "host", `package test {
			attribute def Never;
			action host {
				first start;
				then action iterate {
					for i in 1..1 {
						accept Never;
					}
				}
			}
		}`)
		if !errors.Is(err, ErrStatementNotExecutable) {
			t.Fatalf("error = %v, want ErrStatementNotExecutable", err)
		}
	})
	t.Run("calc_body_flow_not_executable", func(t *testing.T) {
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
		if !errors.Is(err, ErrStatementNotExecutable) {
			t.Fatalf("error = %v, want ErrStatementNotExecutable", err)
		}
	})
}
