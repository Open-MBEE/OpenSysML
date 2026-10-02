package runtime

import (
	"errors"
	"testing"
)

// TestRuntimeRobustnessUnorderedSubactions covers the failure modes of
// subactions no succession reaches, which start with their owner's performance.
func TestRuntimeRobustnessUnorderedSubactions(t *testing.T) {
	t.Run("successions_cycle_over_every_step", func(t *testing.T) {
		_, err := executeActionSource(t, "loop", `package test {
			action loop {
				action a;
				action b;
				succession a then b;
				succession b then a;
			}
		}`)
		if !errors.Is(err, ErrInvalidActionFlow) {
			t.Fatalf("error = %v, want ErrInvalidActionFlow: no step is unpreceded", err)
		}
	})
	t.Run("unordered_accept_never_satisfied", func(t *testing.T) {
		_, err := executeActionSource(t, "waiting", `package test {
			private import ScalarValues::*;
			action waiting {
				attribute n : Integer := 0;
				action wait accept x : Integer;
				action b { assign n := 1; }
			}
		}`)
		if !errors.Is(err, ErrAcceptDeadlock) {
			t.Fatalf("error = %v, want ErrAcceptDeadlock: the owner cannot end before its accept", err)
		}
	})
	t.Run("nested_unordered_accept_holds_successor", func(t *testing.T) {
		outputs, err := executeActionSource(t, "host", `package test {
			private import ScalarValues::*;
			action host {
				attribute later : Integer := 0;
				first start;
				then action outer {
					action wait accept x : Integer;
					action b;
				}
				then action after { assign later := 1; }
				then done;
			}
		}`)
		if !errors.Is(err, ErrAcceptDeadlock) {
			t.Fatalf("error = %v, want ErrAcceptDeadlock", err)
		}
		if v, ok := outputs["later"]; ok && v.Kind == ValConst && v.Const.Int != 0 {
			t.Fatalf("later = %v, want the successor of outer not performed", v)
		}
	})
	t.Run("abstract_and_reference_usages_not_performed", func(t *testing.T) {
		outputs, err := executeActionSource(t, "host", `package test {
			private import ScalarValues::*;
			action host {
				attribute c : Integer := 0;
				abstract action x { assign c := c + 100; }
				ref action r { assign c := c + 1000; }
			}
		}`)
		if err != nil {
			t.Fatalf("error = %v, want the action to end with nothing to perform", err)
		}
		if c := outputs["c"]; c.Kind != ValConst || c.Const.Int != 0 {
			t.Fatalf("c = %v, want 0: neither usage is a composite subaction", c)
		}
	})
	for name, decl := range map[string]string{
		"reference": "ref action r { assign x := 1; }",
		"abstract":  "abstract action r { assign x := 1; }",
	} {
		t.Run("sole_"+name+"_usage_not_performed", func(t *testing.T) {
			outputs, err := executeActionSource(t, "host", `package test {
				private import ScalarValues::*;
				action host {
					attribute x : Integer := 0;
					`+decl+`
				}
			}`)
			if err != nil {
				t.Fatalf("error = %v, want the action to end with nothing to perform", err)
			}
			assertIntOutput(t, outputs, "x", 0)
		})
	}
	t.Run("two_unpreceded_steps_both_start", func(t *testing.T) {
		outputs, err := executeActionSource(t, "Count", `package P {
			private import ScalarValues::*;
			action def Count {
				attribute total : Integer = 0;
				action a { assign total := total + 1; }
				action b { assign total := total + 10; }
				action c { assign total := total + 100; }
				succession first a then c;
				succession first b then c;
			}
		}`)
		if err != nil {
			t.Fatalf("error = %v, want a and b started with the owner", err)
		}
		assertIntOutput(t, outputs, "total", 111)
	})
	t.Run("nested_node_two_unpreceded_steps_both_start", func(t *testing.T) {
		outputs, err := executeActionSource(t, "Count", `package P {
			private import ScalarValues::*;
			action def Count {
				attribute total : Integer = 0;
				action inner {
					action a { assign total := total + 1; }
					action b { assign total := total + 10; }
					action c { assign total := total + 100; }
					succession first a then c;
					succession first b then c;
				}
			}
		}`)
		if err != nil {
			t.Fatalf("error = %v, want inner's a and b started with inner", err)
		}
		assertIntOutput(t, outputs, "total", 111)
	})
	t.Run("state_do_body_two_unpreceded_steps_both_start", func(t *testing.T) {
		exec := stateWithDoBody(t, `
			action a { assign total := total + 1; }
			action b { assign total := total + 10; }
			action c { assign total := total + 100; }
			succession first a then c;
			succession first b then c;
		`)
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("error = %v, want the do body's a and b started together", err)
		}
		if total := exec.StateData()["total"]; !valueEqual(total, integerValue(111)) {
			t.Errorf("total = %v, want 111", total)
		}
	})
	for name, stmt := range map[string]string{
		"while":      "while total < 5 { assign total := total + 1; }",
		"if":         "if total < 5 { assign total := total + 5; }",
		"assignment": "assign total := total + 5;",
	} {
		t.Run("statement_directly_in_an_action_body_"+name, func(t *testing.T) {
			outputs, err := executeActionSource(t, "counter", `package test {
				private import ScalarValues::*;
				action counter {
					attribute total : Integer = 0;
					first start;
					`+stmt+`
					done;
					succession first start then done;
				}
			}`)
			if err != nil {
				t.Fatalf("error = %v, want the %s performed as a subaction", err, name)
			}
			assertIntOutput(t, outputs, "total", 5)
		})
	}
}
