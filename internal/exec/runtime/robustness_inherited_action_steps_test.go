package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func TestRuntimeRobustnessInheritedActionSteps(t *testing.T) {
	t.Run("body_stating_typed_usage_of_general", func(t *testing.T) {
		_, err := executeInheritedAction(t, `package test {
			private import ScalarValues::*;
			action def Base {
				attribute c : Integer = 0;
			}
			action def P :> Base {
				attribute d : Integer = 0;
				first start then x;
				action x : Base {
					assign c := c + 1;
				}
				then done;
			}
		}`, "P")
		if err != nil {
			t.Fatalf("ExecuteAction: %v", err)
		}
	})

	t.Run("unsequenced_inherited_assertion_stays_unchecked", func(t *testing.T) {
		src := `package test {
			action def G { assert constraint check { false } }
			action def S :> G;
		}`
		for _, name := range []string{"G", "S"} {
			t.Run(name, func(t *testing.T) {
				if _, err := executeInheritedAction(t, src, name); err != nil {
					t.Fatalf("ExecuteAction(%s): %v, want an unsequenced assertion to remain unchecked", name, err)
				}
			})
		}
	})

	t.Run("inherited_assertion_sequenced_by_specialization", func(t *testing.T) {
		_, err := executeInheritedAction(t, `package test {
			action def G { assert constraint check { false } }
			action def S :> G {
				first start then check;
				first check then done;
			}
		}`, "S")
		var violation *ViolationError
		if !errors.As(err, &violation) || violation.Element != "check" || !errors.Is(err, ErrViolated) {
			t.Fatalf("ExecuteAction error = %v, want inherited assertion check violated", err)
		}
	})

	t.Run("inherited_sequenced_assertion_violation", func(t *testing.T) {
		outputs, err := executeInheritedAction(t, `package test {
			private import ScalarValues::*;
			action def G {
				attribute level : Integer = 0;
				action raise { assign level := 3; }
				assert constraint raised { level == 4 }
				action lower { assign level := 1; }
				first start then raise;
				first raise then raised;
				first raised then lower;
				first lower then done;
			}
			action def S :> G;
		}`, "S")
		var violation *ViolationError
		if !errors.As(err, &violation) || violation.Element != "raised" ||
			violation.Condition != "level == 4" || !errors.Is(err, ErrViolated) {
			t.Fatalf("ExecuteAction error = %v, want inherited assertion raised violated", err)
		}
		if outputs != nil {
			t.Fatalf("outputs = %v, want none from a failed run", outputs)
		}
	})

	t.Run("recursive_typed_action_body", func(t *testing.T) {
		_, err := executeInheritedAction(t, `package test {
			private import ScalarValues::*;
			action def A {
				attribute c : Integer = 0;
				first start then x;
				action x : A {
					assign c := c + 1;
				}
				then done;
			}
		}`, "A")
		if !errors.Is(err, lower.ErrRecursiveActionTyping) {
			t.Fatalf("ExecuteAction error = %v, want ErrRecursiveActionTyping", err)
		}
	})

	t.Run("cyclic_specialization", func(t *testing.T) {
		_, err := executeInheritedAction(t, `package test {
			action def A :> B;
			action def B :> A;
		}`, "A")
		if !errors.Is(err, lower.ErrCyclicSpecialization) {
			t.Fatalf("ExecuteAction error = %v, want ErrCyclicSpecialization", err)
		}
	})

	t.Run("redefinition_naming_missing_step", func(t *testing.T) {
		_, err := executeInheritedAction(t, `package test {
			action def Base { action a; }
			action def Bad :> Base { action :>> nosuch; }
		}`, "Bad")
		if !errors.Is(err, lower.ErrRedefinedStepMissing) {
			t.Fatalf("ExecuteAction error = %v, want ErrRedefinedStepMissing", err)
		}
	})

	t.Run("inherited_succession_to_incompatible_replacement", func(t *testing.T) {
		_, err := executeInheritedAction(t, `package test {
			action def Base {
				first start then a;
				action a;
				then done;
			}
			action def Inc :> Base { attribute :>> a : Integer; }
		}`, "Inc")
		if !errors.Is(err, lower.ErrIncompatibleRedefinedStep) {
			t.Fatalf("ExecuteAction error = %v, want ErrIncompatibleRedefinedStep", err)
		}
	})

	t.Run("entry_body_redefines_missing_performed_step", func(t *testing.T) {
		err := stateRunErrorForSource(t, "Machine", `package test {
			action def Bump;
			state def Machine {
				entry; then active;
				state active {
					entry action run : Bump { action :>> missing; }
				}
			}
		}`)
		if !errors.Is(err, lower.ErrRedefinedStepMissing) {
			t.Fatalf("state execution error = %v, want ErrRedefinedStepMissing", err)
		}
	})

	t.Run("entry_body_redefines_step_incompatibly", func(t *testing.T) {
		err := stateRunErrorForSource(t, "Machine", `package test {
			private import ScalarValues::*;
			action def BumpBase {
				first start then a;
				action a;
				then done;
			}
			action def Bump :> BumpBase;
			state def Machine {
				entry; then active;
				state active {
					entry action run : Bump {
						attribute :>> a : Integer;
					}
				}
			}
		}`)
		if !errors.Is(err, lower.ErrIncompatibleRedefinedStep) {
			t.Fatalf("state execution error = %v, want ErrIncompatibleRedefinedStep", err)
		}
	})

	t.Run("typed_entry_usage_specializes_back_to_itself", func(t *testing.T) {
		err := stateRunErrorForSource(t, "Machine", `package test {
			state def Machine {
				attribute hits : Integer = 0;
				entry; then active;
				state active {
					action def A :> run;
					entry action run : A { assign hits := hits + 1; }
				}
			}
		}`)
		if !errors.Is(err, lower.ErrCyclicSpecialization) {
			t.Fatalf("state execution error = %v, want ErrCyclicSpecialization", err)
		}
	})

	t.Run("ambiguous_inherited_redefinitions", func(t *testing.T) {
		_, err := executeInheritedAction(t, `package test {
			action def Base { action a; }
			action def Left :> Base { action :>> a; }
			action def Right :> Base { action :>> a; }
			action def Diamond :> Left, Right;
		}`, "Diamond")
		if !errors.Is(err, lower.ErrAmbiguousInheritedStep) {
			t.Fatalf("ExecuteAction error = %v, want ErrAmbiguousInheritedStep", err)
		}
	})

	t.Run("diamond_performs_once", func(t *testing.T) {
		outputs, err := executeInheritedAction(t, `package test {
			private import ScalarValues::*;
			action def Base {
				attribute c : Integer = 0;
				first start then a;
				action a { assign c := c + 1; }
				then done;
			}
			action def Left :> Base { action l { assign c := c + 10; } }
			action def Right :> Base { action r { assign c := c + 100; } }
			action def Diamond :> Left, Right;
		}`, "Diamond")
		if err != nil {
			t.Fatalf("ExecuteAction: %v", err)
		}
		if got := outputs["c"]; got.Kind != ValConst || got.Const.Int != 111 {
			t.Fatalf("c = %v, want 111", got)
		}
	})

	t.Run("narrow_zero", func(t *testing.T) {
		outputs, err := executeInheritedAction(t, `package test {
			private import ScalarValues::*;
			action def Base {
				attribute c : Integer = 0;
				first start then a;
				action a { assign c := c + 1; }
				then done;
			}
			action def Narrow :> Base { action :>> a[0]; }
		}`, "Narrow")
		if err != nil {
			t.Fatalf("ExecuteAction: %v", err)
		}
		if got := outputs["c"]; got.Kind != ValConst || got.Const.Int != 0 {
			t.Fatalf("c = %v, want 0", got)
		}
	})
}

func executeInheritedAction(t *testing.T, src, name string) (map[string]Value, error) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<inherited-action-steps>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<inherited-action-steps>"), name, ast.DefAction)
	if sym == nil {
		t.Fatalf("action %s not found", name)
	}
	return ctx.ExecuteAction(sym)
}
