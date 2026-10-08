package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// runObjectFlowCase runs the action named run in src under a small token-flow budget.
func runObjectFlowCase(t *testing.T, src string) error {
	t.Helper()
	index, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	ctx.maxActionSteps = 1000
	symbol := findSymbolByName(index.DocumentRoot("<test>"), "run", ast.DefAction)
	if symbol == nil {
		t.Fatal("action run not found")
	}
	_, err := ctx.ExecuteAction(symbol)
	return err
}

func TestRuntimeRobustnessObjectFlow(t *testing.T) {
	t.Run("join_waiting_on_a_flow_that_never_arrives_deadlocks", func(t *testing.T) {
		err := runObjectFlowCase(t, `package test {
			private import ScalarValues::*;
			action run {
				first start then which;
				decide which;
				if false then produce;
				else hold;
				action produce { out y : Real; assign y := 1.0; }
				action hold;
				first hold then pair;
				join pair {
					in ref inputObject1 : Real;
					out ref outputObject1 : Real = inputObject1;
				}
				action keep { in v : Real; }
				succession flow of Real from produce.y to pair.inputObject1;
				succession flow of Real from pair.outputObject1 to keep.v;
			}
		}`)
		if !errors.Is(err, ErrActionDeadlock) {
			t.Fatalf("run error = %v, want ErrActionDeadlock", err)
		}
	})

	t.Run("guarded_branches_with_object_flows_both_follow", func(t *testing.T) {
		err := runObjectFlowCase(t, `package test {
			private import ScalarValues::*;
			action run {
				first start then produce;
				action produce { out y : Real; assign y := 1.0; }
				first produce if true then left;
				first produce if true then right;
				action left;
				action right;
			}
		}`)
		if err != nil {
			t.Fatalf("run error = %v, want both guarded targets to follow", err)
		}
	})

	t.Run("merge_reached_by_control_with_values_at_two_inputs_is_ambiguous", func(t *testing.T) {
		err := runObjectFlowCase(t, `package test {
			private import ScalarValues::*;
			action run {
				first start then split;
				fork split;
				first split then a;
				first split then b;
				action a { out y : Integer; assign y := 1; }
				action b { out y : Integer; assign y := 2; }
				first a then both;
				first b then both;
				join both;
				first both then m;
				merge m {
					in ref inputObject1 : Integer;
					in ref inputObject2 : Integer;
					out ref outputObject1 : Integer = (inputObject1, inputObject2);
				}
				flow a.y to m.inputObject1;
				flow b.y to m.inputObject2;
				action keep { in v : Integer; }
				succession flow of Integer from m.outputObject1 to keep.v;
			}
		}`)
		if !errors.Is(err, ErrAmbiguousMergeInput) {
			t.Fatalf("run error = %v, want ErrAmbiguousMergeInput", err)
		}
	})

	t.Run("empty_succession_flow_beside_plain_flows_into_two_merge_inputs_is_ambiguous", func(t *testing.T) {
		err := runObjectFlowCase(t, `package test {
			private import ScalarValues::*;
			action run {
				first start then p;
				action p {
					out x : Integer [0..1];
					out y : Integer;
					out z : Integer;
					assign y := 1;
					assign z := 2;
				}
				merge m {
					in ref inputObject1 : Integer;
					in ref inputObject2 : Integer;
					out ref outputObject1 : Integer = (inputObject1, inputObject2);
				}
				succession flow of Integer from p.x to m.inputObject1;
				flow p.y to m.inputObject1;
				flow p.z to m.inputObject2;
				action keep { in v : Integer; }
				succession flow of Integer from m.outputObject1 to keep.v;
			}
		}`)
		if !errors.Is(err, ErrAmbiguousMergeInput) {
			t.Fatalf("run error = %v, want ErrAmbiguousMergeInput", err)
		}
	})

	t.Run("merge_loop_without_exit_hits_the_step_budget", func(t *testing.T) {
		err := runObjectFlowCase(t, `package test {
			private import ScalarValues::*;
			action run {
				first start then zero;
				action zero { out y : Integer; assign y := 0; }
				merge again {
					in ref inputObject1 : Integer;
					in ref inputObject2 : Integer;
					out ref outputObject1 : Integer = (inputObject1, inputObject2);
				}
				action inc { in x : Integer; out y : Integer; assign y := x + 1; }
				first inc then again;
				succession flow of Integer from zero.y to again.inputObject1;
				succession flow of Integer from again.outputObject1 to inc.x;
				flow inc.y to again.inputObject2;
			}
		}`)
		if !errors.Is(err, ErrActionStepLimitExceeded) {
			t.Fatalf("run error = %v, want ErrActionStepLimitExceeded", err)
		}
	})

	t.Run("failing_control_node_output_returns_typed_error", func(t *testing.T) {
		err := runObjectFlowCase(t, `package test {
			private import ScalarValues::*;
			action run {
				first start then produce;
				action produce { out y : Integer; assign y := 0; }
				fork split {
					in ref inputObject1 : Integer;
					out ref outputObject1 : Integer = 1 / inputObject1;
				}
				action keep { in v : Integer; }
				succession flow of Integer from produce.y to split.inputObject1;
				succession flow of Integer from split.outputObject1 to keep.v;
			}
		}`)
		if !errors.Is(err, ErrDivisionByZero) {
			t.Fatalf("run error = %v, want ErrDivisionByZero", err)
		}
	})
	t.Run("rejected_gated_flow_into_a_required_pin_reached_by_control_deadlocks", func(t *testing.T) {
		err := runObjectFlowCase(t, `package test {
			private import ScalarValues::*;
			action run {
				first start then split;
				fork split;
				first split then a;
				first split then c;
				action a { out y : Integer; assign y := 1; }
				action c;
				first c then b;
				first a if false then f;
				succession flow f of Integer from a.y to b.v;
				action b { in v : Integer [1]; }
			}
		}`)
		if !errors.Is(err, ErrActionDeadlock) {
			t.Fatalf("run error = %v, want ErrActionDeadlock", err)
		}
	})

	for name, c := range map[string]struct{ flow, want string }{
		"succession_leading_to_a_plain_flow_is_invalid": {
			"flow f of Integer from a.y to b.v;", "leads to flow f, which is no succession flow"},
		"succession_leading_to_a_flow_from_another_node_is_invalid": {
			"succession flow f of Integer from c.y to b.v;", "leads to flow f, which leaves c, not a"},
	} {
		t.Run(name, func(t *testing.T) {
			err := runObjectFlowCase(t, `package test {
				private import ScalarValues::*;
				action run {
					first start then a;
					action a { out y : Integer; assign y := 1; }
					action c { out y : Integer; assign y := 2; }
					first a if true then f;
					`+c.flow+`
					action b { in v : Integer; }
				}
			}`)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("run error = %v, want one reading %q", err, c.want)
			}
		})
	}
}
