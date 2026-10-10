package runtime

import (
	"errors"
	"reflect"
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

	t.Run("typed_usage_bodyless_inherited_pin_binding", func(t *testing.T) {
		outputs, err := executeInheritedAction(t, `package test {
			private import ScalarValues::*;
			action def T {
				in source : Integer = 1;
				out y : Integer = source;
			}
			action def Bodyless {
				out attribute observed : Integer = 0;
				action m : T;
				action read { assign observed := m.y; }
				first start then m;
				first m then read;
				first read then done;
			}
		}`, "Bodyless")
		if err != nil {
			t.Fatalf("ExecuteAction(Bodyless): %v", err)
		}
		assertIntOutput(t, outputs, "observed", 1)
		assertIntOutput(t, outputs, "m.y", 1)
	})

	t.Run("typed_usage_body_preserves_inherited_pin_binding", func(t *testing.T) {
		outputs, err := executeInheritedAction(t, `package test {
			private import ScalarValues::*;
			action def T {
				in source : Integer = 1;
				out y : Integer = source;
			}
			action def WithBody {
				out attribute observed : Integer = 0;
				action m : T { assign source := 2; }
				action read { assign observed := m.y; }
				first start then m;
				first m then read;
				first read then done;
			}
		}`, "WithBody")
		if err != nil {
			t.Fatalf("ExecuteAction(WithBody): %v", err)
		}
		assertIntOutput(t, outputs, "m.source", 2)
		assertIntOutput(t, outputs, "m.y", 2)
		assertIntOutput(t, outputs, "observed", 2)
	})

	t.Run("inherited_succession_plain_restatement_is_refused", func(t *testing.T) {
		_, err := executeInheritedAction(t, `package test {
			private import ScalarValues::*;
			action def Base {
				out attribute c : Integer = 0;
				action p;
				action a[3] { assign c := c + 1; }
				succession first [1] p then [3] a;
			}
			action def Derived :> Base {
				succession first p then a;
			}
		}`, "Derived")
		if !errors.Is(err, ErrActionStepMultiplicity) {
			t.Fatalf("ExecuteAction(Derived) error = %v, want ErrActionStepMultiplicity", err)
		}
		var stepErr *lower.StepMultiplicityError
		if !errors.As(err, &stepErr) || stepErr.Code != lower.StepOrderUnsatisfiableCode {
			t.Fatalf("ExecuteAction(Derived) error = %v, want %s", err, lower.StepOrderUnsatisfiableCode)
		}
	})

	t.Run("inherited_succession_matching_restatement_runs_repeated_step", func(t *testing.T) {
		src := `package test {
			private import ScalarValues::*;
			action def Base {
				out attribute c : Integer = 0;
				action p;
				action a[3] { assign c := c + 1; }
				succession first [1] p then [3] a;
			}
			action def Derived :> Base {
				succession first [1] p then [3] a;
			}
		}`
		for _, action := range []string{"Base", "Derived"} {
			outputs, err := executeInheritedAction(t, src, action)
			if err != nil {
				t.Fatalf("ExecuteAction(%s): %v", action, err)
			}
			assertIntOutput(t, outputs, "c", 3)
		}
	})

	t.Run("composite_redefinition_keeps_inherited_statement_and_owned_subaction", func(t *testing.T) {
		outputs, err := executeInheritedAction(t, `package test {
			private import ScalarValues::*;
			action def Base {
				out attribute c : Integer = 0;
				action a { assign c := c + 1; }
			}
			action def Derived :> Base {
				action a :>> a {
					action b { assign c := c + 10; }
				}
			}
		}`, "Derived")
		if err != nil {
			t.Fatalf("ExecuteAction(Derived): %v", err)
		}
		assertIntOutput(t, outputs, "c", 11)
	})

	t.Run("composite_redefinition_keeps_inherited_subaction_and_owned_statement", func(t *testing.T) {
		outputs, err := executeInheritedAction(t, `package test {
			private import ScalarValues::*;
			action def Base {
				out attribute c : Integer = 0;
				action a {
					action b { assign c := c + 10; }
				}
			}
			action def Derived :> Base {
				action a :>> a { assign c := c + 1; }
			}
		}`, "Derived")
		if err != nil {
			t.Fatalf("ExecuteAction(Derived): %v", err)
		}
		assertIntOutput(t, outputs, "c", 11)
	})

	t.Run("specialized_body_binding_replaces_inherited_binding", func(t *testing.T) {
		outputs, err := executeInheritedAction(t, `package test {
			private import ScalarValues::*;
			action def Adder {
				in a : Integer;
				in b : Integer;
				out sum : Integer;
				first step;
				action step { assign sum := a + b; }
			}
			action def Base {
				attribute x : Integer = 5;
				out result : Integer = 0;
				action add : Adder { in b = 2; }
				bind add.a = x;
				flow add.sum to fin.n;
				action fin { in n : Integer; assign result := n; }
			}
			action def Redefined :> Base {
				action add :>> add : Adder { in b = 100; }
				first start then add;
				succession add then fin;
				then done;
			}
		}`, "Redefined")
		if err != nil {
			t.Fatalf("ExecuteAction(Redefined): %v", err)
		}
		assertIntOutput(t, outputs, "add.a", 5)
		assertIntOutput(t, outputs, "add.b", 100)
		assertIntOutput(t, outputs, "add.sum", 105)
		assertIntOutput(t, outputs, "fin.n", 105)
		assertIntOutput(t, outputs, "result", 105)
	})

	t.Run("terminate_usage_body_flow_is_inherited", func(t *testing.T) {
		src := `package test {
			private import ScalarValues::*;
			action def G {
				out attribute x : Integer = 0;
				out attribute later : Integer = 0;
				first start then stop;
				action stop terminate {
					first start;
					then action inner { assign x := 1; }
					then done;
				}
				then action tail { assign later := 1; }
				then done;
			}
			action def S :> G;
		}`
		for _, name := range []string{"G", "S"} {
			t.Run(name, func(t *testing.T) {
				outputs, err := executeInheritedAction(t, src, name)
				if err != nil {
					t.Fatalf("ExecuteAction(%s): %v", name, err)
				}
				assertIntOutput(t, outputs, "x", 1)
				assertIntOutput(t, outputs, "later", 0)
			})
		}
	})

	t.Run("inherited_case_keeps_case_step_ordering", func(t *testing.T) {
		src := `package test {
			private import ScalarValues::*;
			action def G {
				out attribute r : Integer = 0;
				analysis nested {
					return : Integer;
					attribute x : Integer := 1;
					action multiply { assign x := x * 10; }
					action add { assign x := x + 2; }
					x
				}
				first start then nested;
				first nested then read;
				action read { assign r := nested.result; }
				first read then done;
			}
			action def S :> G;
		}`
		for _, name := range []string{"G", "S"} {
			t.Run(name, func(t *testing.T) {
				outputs, err := executeInheritedAction(t, src, name)
				if err != nil {
					t.Fatalf("ExecuteAction(%s): %v", name, err)
				}
				assertIntOutput(t, outputs, "r", 12)
			})
		}
	})

	t.Run("specialization_flow_connects_inherited_steps", func(t *testing.T) {
		outputs, err := executeInheritedAction(t, `package test {
			private import ScalarValues::*;
			action def G {
				action a { out y : Integer; assign y := 7; }
				action b { in v : Integer; }
				first start then a;
				first a then b;
				first b then done;
			}
			action def S :> G {
				flow f2 from a.y to b.v;
			}
		}`, "S")
		if err != nil {
			t.Fatalf("ExecuteAction(S): %v", err)
		}
		if got, ok := outputs["b.v"]; !ok || got.Kind != ValConst || got.Const.Int != 7 {
			t.Fatalf("S b.v = %v, want 7", got)
		}
	})

	t.Run("inherited_gate_resolves_same_named_flow_in_declaring_scope", func(t *testing.T) {
		for _, tc := range []struct {
			guard string
			wantB bool
		}{
			{guard: "false"},
			{guard: "true", wantB: true},
		} {
			t.Run(tc.guard, func(t *testing.T) {
				outputs, err := executeInheritedAction(t, `package test {
					private import ScalarValues::*;
					action def G {
						attribute go : Boolean = `+tc.guard+`;
						action a { out y : Integer; assign y := 7; }
						action b { in v : Integer; }
						first start then a;
						first a if go then f;
						succession flow f of Integer from a.y to b.v;
					}
					action def S :> G {
						action c { in v : Integer; }
						succession flow f of Integer from a.y to c.v;
					}
				}`, "S")
				if err != nil {
					t.Fatalf("ExecuteAction(S): %v", err)
				}
				if got, ok := outputs["c.v"]; !ok || got.Kind != ValConst || got.Const.Int != 7 {
					t.Fatalf("S c.v = %v, want 7", got)
				}
				b, hasB := outputs["b.v"]
				if tc.wantB {
					if !hasB || b.Kind != ValConst || b.Const.Int != 7 {
						t.Fatalf("S b.v = %v, want 7", b)
					}
				} else if hasB {
					t.Fatalf("S b.v = %v, want no value from a.y", b)
				}
			})
		}
	})

	t.Run("inherited_gated_succession_flow", func(t *testing.T) {
		for _, tc := range []struct {
			guard string
			value int64
		}{
			{guard: "true", value: 1},
			{guard: "false", value: 2},
		} {
			t.Run(tc.guard, func(t *testing.T) {
				src := `package test {
					private import ScalarValues::*;
					action def G {
						attribute go : Boolean = ` + tc.guard + `;
						first start then a;
						action a { out y : Integer; assign y := 1; }
						action c { out y : Integer; assign y := 2; }
						action b { in v : Integer; }
						first a if go then f;
						first a if not go then c;
						succession flow f of Integer from a.y to b.v;
						succession flow of Integer from c.y to b.v;
					}
					action def S :> G;
				}`
				general, err := executeInheritedAction(t, src, "G")
				if err != nil {
					t.Fatalf("ExecuteAction(G): %v", err)
				}
				specialized, err := executeInheritedAction(t, src, "S")
				if err != nil {
					t.Fatalf("ExecuteAction(S): %v", err)
				}
				if !reflect.DeepEqual(specialized, general) {
					t.Fatalf("ExecuteAction(S) outputs = %v, want G outputs %v", specialized, general)
				}
				got, ok := specialized["b.v"]
				if !ok || got.Kind != ValConst || got.Const.Int != tc.value {
					t.Fatalf("S b.v = %v, want %d", got, tc.value)
				}
			})
		}
	})

	t.Run("inherited_guard_survives_differently_guarded_owned_succession", func(t *testing.T) {
		outputs, err := executeInheritedAction(t, `package test {
			private import ScalarValues::*;
			action def Base {
				attribute x : Integer = 1;
				out attribute c : Integer = 0;
				action a;
				action b { assign c := 1; }
				first a if x < 3 then b;
			}
			action def S :> Base {
				first a if x > 5 then b;
			}
		}`, "S")
		if err != nil {
			t.Fatalf("ExecuteAction(S): %v", err)
		}
		assertIntOutput(t, outputs, "c", 1)
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

	t.Run("assertion_ordered_by_intermediate_general", func(t *testing.T) {
		_, err := executeInheritedAction(t, `package test {
			action def G { assert constraint check { true } }
			action def M :> G {
				first start then check;
				first check then done;
			}
			action def S :> M;
		}`, "S")
		if err != nil {
			t.Fatalf("ExecuteAction(S): %v, want the inherited assertion flow to complete", err)
		}
	})

	t.Run("intermediate_general_assertion_violation", func(t *testing.T) {
		src := `package test {
			action def G { assert constraint check { false } }
			action def M :> G {
				first start then check;
				first check then done;
			}
			action def S :> M;
		}`
		for _, name := range []string{"M", "S"} {
			t.Run(name, func(t *testing.T) {
				_, err := executeInheritedAction(t, src, name)
				var violation *ViolationError
				if !errors.As(err, &violation) || violation.Element != "check" || !errors.Is(err, ErrViolated) {
					t.Fatalf("ExecuteAction(%s) error = %v, want assertion check violated", name, err)
				}
			})
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

	t.Run("restated_end_multiplicity_succession_performs_once", func(t *testing.T) {
		outputs, err := executeInheritedAction(t, `package test {
			private import ScalarValues::*;
			action def Base {
				attribute n : Integer = 0;
				action a[1];
				action b { assign n := n + 1; }
				first start then a;
				succession first [1] a then [1] b;
				first b then done;
			}
			action def D :> Base {
				succession first [1] a then [1] b;
			}
		}`, "D")
		if err != nil {
			t.Fatalf("ExecuteAction: %v", err)
		}
		if got := outputs["n"]; got.Kind != ValConst || got.Const.Int != 1 {
			t.Fatalf("n = %v, want 1", got)
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
