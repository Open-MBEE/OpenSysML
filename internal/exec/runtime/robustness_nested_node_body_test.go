package runtime

import (
	"errors"
	"testing"
)

func TestRuntimeRobustnessNestedNodeInBody(t *testing.T) {
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
