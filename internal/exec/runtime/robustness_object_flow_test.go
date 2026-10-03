package runtime

import (
	"errors"
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

	t.Run("guarded_branches_with_object_flows_stay_ambiguous", func(t *testing.T) {
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
		if !errors.Is(err, ErrAmbiguousSuccession) {
			t.Fatalf("run error = %v, want ErrAmbiguousSuccession", err)
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
}
