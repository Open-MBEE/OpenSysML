package runtime

import (
	"errors"
	"testing"
)

func TestRuntimeRobustnessSequencedAssertion(t *testing.T) {
	t.Run("violated_assertion_stops_the_flow", func(t *testing.T) {
		outputs, err := executeActionSource(t, "check", `package test {
			private import ScalarValues::*;
			action check {
				attribute level : Integer = 0;
				assert constraint raised { level == 3 }
				action raise { assign level := 3; }
				first start then raised;
				first raised then raise;
				first raise then done;
			}
		}`)
		var violation *ViolationError
		if !errors.As(err, &violation) || violation.Element != "raised" || !errors.Is(err, ErrViolated) {
			t.Fatalf("error = %v, want the assertion raised violated", err)
		}
		if outputs != nil {
			t.Fatalf("outputs = %v, want none from a failed run", outputs)
		}
	})
	t.Run("violated_negated_assertion", func(t *testing.T) {
		_, err := executeActionSource(t, "check", `package test {
			private import ScalarValues::*;
			action check {
				attribute level : Integer = 3;
				assert not constraint high { level > 2 }
				first start then high;
				first high then done;
			}
		}`)
		var violation *ViolationError
		if !errors.As(err, &violation) || violation.Element != "high" {
			t.Fatalf("error = %v, want the negated assertion high violated", err)
		}
	})
	t.Run("unsequenced_assertion_is_not_a_step", func(t *testing.T) {
		outputs, err := executeActionSource(t, "check", `package test {
			private import ScalarValues::*;
			action check {
				attribute level : Integer = 0;
				assert constraint never { level == 3 }
				action raise { assign level := 1; }
				first start then raise;
				first raise then done;
			}
		}`)
		if err != nil {
			t.Fatalf("error = %v, want an assertion no succession orders left unchecked", err)
		}
		if v := outputs["level"]; v.Kind != ValConst || v.Const.Int != 1 {
			t.Fatalf("level = %v, want 1", v)
		}
	})
	t.Run("unresolved_name_in_condition", func(t *testing.T) {
		_, err := executeActionSource(t, "check", `package test {
			action check {
				assert constraint lost { missing > 1 }
				first start then lost;
				first lost then done;
			}
		}`)
		if err == nil {
			t.Fatal("error = nil, want the unresolved name reported")
		}
		var violation *ViolationError
		if errors.As(err, &violation) {
			t.Fatalf("error = %v, want an evaluation failure rather than a violation", err)
		}
	})
	t.Run("assertion_in_a_loop_checked_each_pass", func(t *testing.T) {
		_, err := executeActionSource(t, "check", `package test {
			private import ScalarValues::*;
			action check {
				attribute level : Integer = 0;
				action body {
					for i in 1..3 {
						action raise { assign level := level + 1; }
						assert constraint low { level < 3 }
						first raise then low;
					}
				}
				first start then body;
				first body then done;
			}
		}`)
		var violation *ViolationError
		if !errors.As(err, &violation) || violation.Element != "low" {
			t.Fatalf("error = %v, want the assertion low violated on the third pass", err)
		}
	})
	t.Run("inherited_assertion_sequenced_by_the_specialization", func(t *testing.T) {
		_, err := executeActionSource(t, "Derived", `package test {
			private import ScalarValues::*;
			action def Base {
				attribute level : Integer = 0;
				assert constraint ready { level == 0 }
			}
			action def Derived :> Base {
				action raise { assign level := 3; }
				first start then raise;
				first raise then ready;
				first ready then done;
			}
		}`)
		var violation *ViolationError
		if !errors.As(err, &violation) || violation.Element != "ready" {
			t.Fatalf("error = %v, want the inherited assertion ready violated", err)
		}
	})
}
