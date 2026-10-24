package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessNodePins covers action-node pins: reads before the node performs,
// pins the node does not declare, invocation arity and pin bindings.
func TestRuntimeRobustnessNodePins(t *testing.T) {
	t.Run("node_pin_of_a_node_not_yet_performed", testNodePinOfANodeNotYetPerformed)
	t.Run("node_pin_the_node_does_not_declare", testNodePinTheNodeDoesNotDeclare)
	t.Run("node_read_as_a_value_without_a_result", testNodeReadAsAValueWithoutAResult)
	t.Run("settled_node_read_as_a_value_by_another_candidates_result", testSettledNodeReadAsAValueByAnotherCandidatesResult)
	t.Run("node_pin_member_through_a_scalar_pin", testNodePinMemberThroughAScalarPin)
	t.Run("block_node_pin_of_a_node_not_yet_performed", testBlockNodePinOfANodeNotYetPerformed)
	t.Run("block_node_pin_the_node_does_not_declare", testBlockNodePinTheNodeDoesNotDeclare)
	t.Run("else_branch_node_read_before_it_performs", testElseBranchNodeReadBeforeItPerforms)
	t.Run("typed_node_pin_of_a_node_the_callee_does_not_declare", testTypedNodePinOfANodeTheCalleeDoesNotDeclare)
	t.Run("node_invocation_too_many_arguments", testNodeInvocationTooManyArguments)
	t.Run("node_invocation_too_few_arguments", testNodeInvocationTooFewArguments)
	t.Run("node_invocation_unknown_named_argument", testNodeInvocationUnknownNamedArgument)
	t.Run("node_invocation_repeated_named_argument", testNodeInvocationRepeatedNamedArgument)
	t.Run("node_invocation_argument_fails_before_defaults", testNodeInvocationArgumentFailsBeforeDefaults)
	t.Run("calc_block_node_unvalued_pin_write_checked", testCalcBlockNodeUnvaluedPinWriteChecked)
	t.Run("node_binding_to_a_non_parameter", testNodeBindingToANonParameter)
	t.Run("node_undirected_binding_carried_to_a_non_parameter", testNodeUndirectedBindingCarriedToANonParameter)
	t.Run("node_pin_bound_to_unequal_values", testNodePinBoundToUnequalValues)
	t.Run("node_output_bound_to_a_nested_node_that_never_runs", testNodeOutputBoundToANestedNodeThatNeverRuns)
	t.Run("block_node_binding_to_a_non_parameter", testBlockNodeBindingToANonParameter)
	t.Run("block_node_binding_names_a_node_without_a_pin", testBlockNodeBindingNamesANodeWithoutAPin)
	t.Run("block_node_pin_bound_where_nodes_are_not_performed", testBlockNodePinBoundWhereNodesAreNotPerformed)
	t.Run("block_node_own_flow_malformed", testBlockNodeOwnFlowMalformed)
	t.Run("block_node_own_flow_where_nodes_are_not_performed", testBlockNodeOwnFlowWhereNodesAreNotPerformed)
	t.Run("block_node_own_flow_that_never_ends", testBlockNodeOwnFlowThatNeverEnds)
	t.Run("inherited_binding_names_a_node_without_a_pin", testInheritedBindingNamesANodeWithoutAPin)
	t.Run("inherited_binding_does_not_reach_a_masking_node", testInheritedBindingDoesNotReachAMaskingNode)
	t.Run("inherited_binding_does_not_reach_through_a_replaced_other_end", testInheritedBindingDoesNotReachThroughAReplacedOtherEnd)
	t.Run("node_inherited_default_that_cannot_be_evaluated", testNodeInheritedDefaultThatCannotBeEvaluated)
	t.Run("node_binding_output_to_an_unknown_feature", testNodeBindingOutputToAnUnknownFeature)
	t.Run("node_binding_output_through_a_scalar_chain", testNodeBindingOutputThroughAScalarChain)
	t.Run("node_binding_output_through_a_chain_violates_target_type", testNodeBindingOutputThroughAChainViolatesTargetType)
	t.Run("nested_pin_binding_into_a_node_performing_another_action", testNestedPinBindingIntoANodePerformingAnotherAction)
	t.Run("nested_pin_binding_at_an_undeclared_pin", testNestedPinBindingAtAnUndeclaredPin)
	t.Run("flow_reaching_into_a_nodes_own_flow", testFlowReachingIntoANodesOwnFlow)
	t.Run("node_flow_into_a_pin_the_target_does_not_declare", testNodeFlowIntoAPinTheTargetDoesNotDeclare)
}

// testNodePinOfANodeNotYetPerformed: a pin holds a value only once its node has
// been performed, so reading it earlier is reported rather than answered.
func testNodePinOfANodeNotYetPerformed(t *testing.T) {
	src := `
		package test {
			action outer {
				attribute total : Integer = 0;
				first start;
				then action early { assign total := late.v; }
				then action late { out v : Integer; assign v := 1; }
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, errNodeNotPerformed) {
		t.Fatalf("error = %v, want errNodeNotPerformed", err)
	}
	if !strings.Contains(err.Error(), "late") {
		t.Errorf("error %q does not name the node", err)
	}
}

// testNodeOutputBoundToANestedNodeThatNeverRuns: a node's output bound to a pin of one
// of its own nested nodes takes its value as that node ends, so where the nested node
// never runs — a reference no succession leads to — the output is unvalued when its
// node ends, and reported so.
func testNodeOutputBoundToANestedNodeThatNeverRuns(t *testing.T) {
	src := `
		package test {
			action outer {
				out attribute legV : Integer;
				bind leg.inner.v = leg.v;
				first start;
				then action leg {
					out v : Integer[1];
					ref action inner { out v : Integer[1]; assign v := 1; }
					first start;
					then action own { assign legV := 0; }
					then done;
				}
				then action fin { assign legV := leg.v; }
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, errBindingEnd) {
		t.Fatalf("error = %v, want errBindingEnd", err)
	}
	if !strings.Contains(err.Error(), "leg.inner.v") {
		t.Errorf("error %q does not name the other end", err)
	}
}

// testNodePinTheNodeDoesNotDeclare: a chain through a node names one of its
// pins; a name it does not declare is reported with the node it was read from.
func testNodePinTheNodeDoesNotDeclare(t *testing.T) {
	src := `
		package test {
			action outer {
				attribute total : Integer = 0;
				first start;
				then action p { out v : Integer; assign v := 1; }
				then action fin { assign total := p.w; }
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, errNodePin) {
		t.Fatalf("error = %v, want errNodePin", err)
	}
	if !strings.Contains(err.Error(), "p") || !strings.Contains(err.Error(), "w") {
		t.Errorf("error %q does not name the node and the pin", err)
	}
}

// testBlockNodePinOfANodeNotYetPerformed: a node declared in a branch is a
// performance of its own like any other, so reading its pin from a sibling that
// runs before it is reported the same way.
func testBlockNodePinOfANodeNotYetPerformed(t *testing.T) {
	src := `
		package test {
			action outer {
				attribute total : Integer = 0;
				first start;
				then action run {
					if true {
						action early { assign total := late.v; }
						action late { out v : Integer; assign v := 1; }
					}
				}
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, errNodeNotPerformed) {
		t.Fatalf("error = %v, want errNodeNotPerformed", err)
	}
	if !strings.Contains(err.Error(), "late") {
		t.Errorf("error %q does not name the node", err)
	}
}

// testBlockNodePinTheNodeDoesNotDeclare: a pin read through a node declared in
// a loop body that the node does not declare is reported with the node.
func testBlockNodePinTheNodeDoesNotDeclare(t *testing.T) {
	src := `
		package test {
			action outer {
				attribute total : Integer = 0;
				first start;
				then action run {
					for i in 1..2 {
						action p { out v : Integer; assign v := i; }
						assign total := total + p.w;
					}
				}
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, errNodePin) {
		t.Fatalf("error = %v, want errNodePin", err)
	}
	if !strings.Contains(err.Error(), "p") || !strings.Contains(err.Error(), "w") {
		t.Errorf("error %q does not name the node and the pin", err)
	}
}

// testElseBranchNodeReadBeforeItPerforms: an else branch's `p` is the one its
// own reads name even where the then branch declares a `p` too, so a read ahead
// of it is not-yet-performed rather than a read of the other branch's node.
func testElseBranchNodeReadBeforeItPerforms(t *testing.T) {
	src := `
		package test {
			action outer {
				attribute total : Integer = 0;
				first start;
				then action run {
					if false {
						action p { out v : Integer; assign v := 1; }
						assign total := p.v;
					} else {
						assign total := p.v;
						action p { out v : Integer; assign v := 2; }
					}
				}
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, errNodeNotPerformed) {
		t.Fatalf("error = %v, want errNodeNotPerformed", err)
	}
	if !strings.Contains(err.Error(), "p") {
		t.Errorf("error %q does not name the node", err)
	}
}

// testTypedNodePinOfANodeTheCalleeDoesNotDeclare: a typed node's subactions are
// those of the action it performed; a path through one it declares not is reported.
func testTypedNodePinOfANodeTheCalleeDoesNotDeclare(t *testing.T) {
	src := `
		package test {
			action def Seven {
				out result : Integer;
				first start;
				then action inner { out v : Integer; assign v := 7; }
				then action publish { assign result := inner.v; }
				then done;
			}
			action outer {
				attribute total : Integer = 0;
				first start;
				then action call : Seven;
				then action read { assign total := call.other.v; }
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, errNodePin) {
		t.Fatalf("error = %v, want errNodePin", err)
	}
	if !strings.Contains(err.Error(), "call") || !strings.Contains(err.Error(), "other") {
		t.Errorf("error %q does not name the node and the missing one", err)
	}
}

// testNodeReadAsAValueWithoutAResult: a node read as a value stands for its
// performance's `result`; a node whose callee declares none is reported.
func testNodeReadAsAValueWithoutAResult(t *testing.T) {
	src := `
		package test {` + adderActionDef + `
			action outer {
				attribute total : Integer = 0;
				first start;
				then action add = Adder(1, 2);
				then action fin { assign total := add; }
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, errNodePin) {
		t.Fatalf("error = %v, want errNodePin", err)
	}
	if !strings.Contains(err.Error(), "result") {
		t.Errorf("error %q does not name the missing result", err)
	}
}

// testSettledNodeReadAsAValueByAnotherCandidatesResult: a call its values settle holds
// the pins of the action performed alone, so it is not read as a value by a `result` only
// another candidate declares, which would leave the read waiting for a value never written.
func testSettledNodeReadAsAValueByAnotherCandidatesResult(t *testing.T) {
	src := `
		package Tags { private import ScalarValues::*;
			action def tag { in x : Integer; in y : Real; out mark : Integer; first step; action step { assign mark := 1; } }
			action def tag { in x : Real; in y : Integer; out result : Integer; first step; action step { assign result := 2; } } }
		package test {
			private import ScalarValues::*;
			private import Tags::*;
			calc def same { in v; v }
			action outer {
				attribute p = same(1);
				attribute q = same(2.5);
				attribute total : Integer = 0;
				first start;
				then action call = tag(x = p, y = q);
				then action fin { assign total := call; }
				then done;
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	outer := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
	if outer == nil {
		t.Fatal("action outer not found")
	}
	_, err := ctx.ExecuteAction(outer)
	if !errors.Is(err, errNodePin) {
		t.Fatalf("error = %v, want errNodePin", err)
	}
	if !strings.Contains(err.Error(), "call") || !strings.Contains(err.Error(), "result") {
		t.Errorf("error %q does not name the node and the missing result", err)
	}
}

// testNodePinMemberThroughAScalarPin: `node.pin.member` chains through the pin's
// value like any feature chain, so a pin holding no object cannot be read through.
func testNodePinMemberThroughAScalarPin(t *testing.T) {
	src := `
		package test {
			action outer {
				attribute total : Integer = 0;
				first start;
				then action p { out v : Integer; assign v := 7; }
				then action read { assign total := p.v.mark; }
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if err == nil {
		t.Fatal("expected an error chaining a member through a scalar pin")
	}
	if !strings.Contains(err.Error(), "p.v") || !strings.Contains(err.Error(), "non-instance") {
		t.Errorf("error %q does not name the pin read through", err)
	}
}

// testNodeInheritedDefaultThatCannotBeEvaluated: an inherited default is seeded
// when the node starts, so its failure is reported there, naming the parameter.
func testNodeInheritedDefaultThatCannotBeEvaluated(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			action def Base {
				in divisor : Integer = 0;
				in share : Integer = 6 / divisor;
				out r : Integer;
			}
			action def Derived :> Base {
				first start;
				then assign r := 1;
				then done;
			}
			action outer {
				first start;
				then action d : Derived;
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("error = %v, want ErrDivisionByZero", err)
	}
	if !strings.Contains(err.Error(), "share") {
		t.Errorf("error %q does not name the parameter whose default failed", err)
	}
}

// testNodeInvocationTooManyArguments: positional arguments beyond the callee's
// input parameters are reported rather than dropped.
func testNodeInvocationTooManyArguments(t *testing.T) {
	src := `
		package test {` + adderActionDef + `
			action outer {
				first start;
				then action add = Adder(1, 2, 3);
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, errActionArity) {
		t.Fatalf("error = %v, want errActionArity", err)
	}
}

// testNodeInvocationTooFewArguments: an input parameter that no argument and no
// default binds is reported before the callee runs, not when its body reads it.
func testNodeInvocationTooFewArguments(t *testing.T) {
	src := `
		package test {` + adderActionDef + `
			action outer {
				attribute b : Integer = 40;
				first start;
				then action add = Adder(1);
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, errUnboundParameter) {
		t.Fatalf("error = %v, want errUnboundParameter", err)
	}
	if !strings.Contains(err.Error(), "b") {
		t.Errorf("error %q does not name the unbound parameter", err)
	}
}

// testNodeInvocationArgumentFailsBeforeDefaults: an argument that fails to evaluate
// is the error reported, not the default it replaces (which is never evaluated) nor
// a default reading the pin it would have bound.
func testNodeInvocationArgumentFailsBeforeDefaults(t *testing.T) {
	src := `
		package test {` + adderActionDef + `
			action outer {
				attribute zero : Integer = 0;
				first start;
				then action add = Adder(a = 1 / zero) {
					in a = 5;
					in b = a + 1;
				}
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("error = %v, want ErrDivisionByZero", err)
	}
	if !strings.Contains(err.Error(), `argument "a" of Adder`) {
		t.Errorf("error %q does not name the argument that failed", err)
	}
}

// testNodeInvocationUnknownNamedArgument: a named argument must name an input
// parameter of the callee.
func testNodeInvocationUnknownNamedArgument(t *testing.T) {
	src := `
		package test {` + adderActionDef + `
			action outer {
				first start;
				then action add = Adder(a = 1, c = 2);
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, errUnknownParameter) {
		t.Fatalf("error = %v, want errUnknownParameter", err)
	}
}

// testNodeInvocationRepeatedNamedArgument: a parameter named twice is rejected, not
// bound to whichever argument comes last.
func testNodeInvocationRepeatedNamedArgument(t *testing.T) {
	src := `
		package test {` + adderActionDef + `
			action outer {
				first start;
				then action add = Adder(a = 1, b = 2, a = 3);
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, errDuplicateArgument) {
		t.Fatalf("error = %v, want errDuplicateArgument", err)
	}
	if !strings.Contains(err.Error(), `"a"`) {
		t.Errorf("error %q does not name the parameter bound twice", err)
	}
}

// testCalcBlockNodeUnvaluedPinWriteChecked: a pin a node in a calc's loop body
// declares without a value is the node's to write, so the write is judged against
// the pin's declaration, not refused as a name the calc never declared.
func testCalcBlockNodeUnvaluedPinWriteChecked(t *testing.T) {
	err := calcErrorWithLibraries(t, `package test {
		private import ScalarValues::*;
		calc noted {
			attribute v : Integer = 100;
			for i in 1..1 {
				action p {
					out w : Integer;
					assign w := "one";
				}
			}
			return : Integer = v;
		}
	}`, "noted", nil, 1000)
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("error = %v, want ErrTypeMismatch", err)
	}
	if !strings.Contains(err.Error(), "w") {
		t.Errorf("error %q does not name the pin written", err)
	}
}

// testNodeBindingToANonParameter: a binding end at a node's pin must name a
// parameter or attribute the node's performance holds.
func testNodeBindingToANonParameter(t *testing.T) {
	src := `
		package test {` + adderActionDef + `
			action outer {
				attribute x : Integer = 1;
				bind add.nope = x;
				first start;
				then action add : Adder { in a = 1; in b = 2; }
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, errBindingEnd) {
		t.Fatalf("error = %v, want errBindingEnd", err)
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("error %q does not name the pin", err)
	}
}

// testNodeUndirectedBindingCarriedToANonParameter: a changed undirected attribute carried
// to a pin the downstream node does not declare is reported, not dropped.
func testNodeUndirectedBindingCarriedToANonParameter(t *testing.T) {
	src := `
		package test {` + adderActionDef + `
			action outer {
				bind acc.total = add.nope;
				first start;
				then action acc { attribute total : Integer; assign total := 3; }
				then action add : Adder { in a = 1; in b = 2; }
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, errNodePin) {
		t.Fatalf("error = %v, want errNodePin", err)
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("error %q does not name the pin", err)
	}
}

// testBlockNodeBindingToANonParameter: a binding in a branch at a pin of the
// branch's node is checked against that node's performance like any other.
func testBlockNodeBindingToANonParameter(t *testing.T) {
	src := `
		package test {` + adderActionDef + `
			action outer {
				attribute x : Integer = 1;
				first start;
				then action choose {
					if x > 0 {
						bind add.nope = x;
						action add : Adder { in a = 1; in b = 2; }
					}
				}
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, errBindingEnd) {
		t.Fatalf("error = %v, want errBindingEnd", err)
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("error %q does not name the pin", err)
	}
}

// testBlockNodeBindingNamesANodeWithoutAPin: a binding end in a branch that names
// one of the branch's nodes but no pin of it is reported, not run as a statement.
func testBlockNodeBindingNamesANodeWithoutAPin(t *testing.T) {
	src := `
		package test {` + adderActionDef + `
			action outer {
				attribute x : Integer = 1;
				first start;
				then action choose {
					if x > 0 {
						bind add = x;
						action add : Adder { in a = 1; in b = 2; }
					}
				}
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if err == nil {
		t.Fatal("expected the binding at the node itself to be reported")
	}
	if !strings.Contains(err.Error(), "names an action node but no pin of it") {
		t.Errorf("error %q does not explain the binding end", err)
	}
}

// testBlockNodePinBoundWhereNodesAreNotPerformed: a calc body keeps no
// performances of its nested actions, so a binding at one of their pins is reported.
func testBlockNodePinBoundWhereNodesAreNotPerformed(t *testing.T) {
	src := `
		package test {
			calc c {
				attribute x : Integer = 3;
				attribute seen : Integer = 0;
				if x > 0 {
					action p { out v : Integer; assign v := x * 2; }
					bind p.v = x;
					assign seen := 1;
				}
				return : Integer = seen;
			}
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "result", 10000)
	if err == nil || !strings.Contains(err.Error(), "a binding or flow at a pin of node p in a body is not executable") {
		t.Errorf("expected the binding at p's pin to be reported, got: %v", err)
	}
}

// testBlockNodeOwnFlowMalformed: the flow an action in a loop body states of its
// own is validated with the action's, so a dangling succession in it is reported
// at initialize() as an invalid flow rather than when the loop reaches it.
func testBlockNodeOwnFlowMalformed(t *testing.T) {
	src := `
		package test {
			action outer {
				attribute i : Integer = 0;
				first start;
				then action iterate {
					while i < 2 {
						action step {
							first start;
							then action one { assign i := i + 1; }
							succession one then missing;
						}
					}
				}
				then done;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
	if sym == nil {
		t.Fatal("action outer not found")
	}
	_, err := ctx.CreateActionExecutor(sym)
	if !errors.Is(err, errInvalidActionFlow) {
		t.Fatalf("expected errInvalidActionFlow for step's dangling succession, got: %v", err)
	}
	if !strings.Contains(err.Error(), "action node step") || !strings.Contains(err.Error(), "missing") {
		t.Errorf("error %q does not name the node and the undefined target", err)
	}
}

// testInheritedBindingNamesANodeWithoutAPin: a binding a base action states at an
// inherited node itself is reported when the derived action's flow is built.
func testInheritedBindingNamesANodeWithoutAPin(t *testing.T) {
	src := `
		package test {
			action def Base {
				attribute x : Integer = 5;
				action add { in a : Integer; }
				bind add = x;
			}
			action def Derived :> Base {
				first start then add;
				succession add then done;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Derived", ast.DefAction)
	if sym == nil {
		t.Fatal("action Derived not found")
	}
	_, err := ctx.CreateActionExecutor(sym)
	if err == nil {
		t.Fatal("expected the inherited binding at the node itself to be reported")
	}
	if !strings.Contains(err.Error(), `binding end "add" names an action node but no pin of it`) {
		t.Errorf("error %q does not explain the binding end", err)
	}
}

// testInheritedBindingDoesNotReachAMaskingNode: a base's binding at a node it
// declares does not bind the same-named pin of a node the derived action
// declares in its place, so that pin stays unvalued.
func testInheritedBindingDoesNotReachAMaskingNode(t *testing.T) {
	src := `
		package test {
			action def Base {
				attribute x : Integer = 5;
				action add { in a : Integer; out sum : Integer; assign sum := a; }
				bind add.a = x;
			}
			action def Derived :> Base {
				action add { in a : Integer[1]; out sum : Integer; assign sum := a + 1; }
				first start then add;
				succession add then done;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Derived", ast.DefAction)
	if sym == nil {
		t.Fatal("action Derived not found")
	}
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	err = exec.RunToCompletion()
	if err == nil {
		t.Fatal("the masking node's pin took the base's binding")
	}
	var noValue *NoValueError
	if !errors.As(err, &noValue) || noValue.Feature != "a" {
		t.Errorf("error %v, want %T for a", err, noValue)
	}
	if v, ok := exec.Results()["add.sum"]; ok {
		t.Errorf("add.sum = %v, want no value", v)
	}
}

// testInheritedBindingDoesNotReachThroughAReplacedOtherEnd: a base binding whose
// other end names a node the derived action replaced holds at neither end, so
// the inherited node's input is bound by nothing rather than by the replacement.
func testInheritedBindingDoesNotReachThroughAReplacedOtherEnd(t *testing.T) {
	src := `
		package test {
			action def Adder {
				in a : Integer[1];
				out sum : Integer;
				first step;
				action step { assign sum := a; }
			}
			action def Base {
				action src { out n : Integer; assign n := 5; }
				action add : Adder;
				bind add.a = src.n;
			}
			action def Derived :> Base {
				action src { out n : Integer; assign n := 1000; }
				first start then src;
				succession src then add;
				succession add then done;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Derived", ast.DefAction)
	if sym == nil {
		t.Fatal("action Derived not found")
	}
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	err = exec.RunToCompletion()
	if err == nil {
		t.Fatal("the inherited node's pin took the replacement node's value")
	}
	if !errors.Is(err, errUnboundParameter) || !strings.Contains(err.Error(), "parameter a") {
		t.Errorf("error %v, want errUnboundParameter for a", err)
	}
	if v, ok := exec.Results()["add.sum"]; ok {
		t.Errorf("add.sum = %v, want no value", v)
	}
}

// testBlockNodeOwnFlowWhereNodesAreNotPerformed: a calc body keeps no performances
// of its nested actions, so one stating a flow of its own is reported, not run.
func testBlockNodeOwnFlowWhereNodesAreNotPerformed(t *testing.T) {
	src := `
		package test {
			calc c {
				attribute x : Integer = 3;
				attribute seen : Integer = 0;
				if x > 0 {
					action p {
						first start;
						then action bump { assign seen := 1; }
						then done;
					}
				}
				return : Integer = seen;
			}
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "result", 10000)
	if err == nil || !strings.Contains(err.Error(), "the flow node p states of its own in a body is not executable") {
		t.Errorf("expected p's own flow to be reported, got: %v", err)
	}
}

// testBlockNodeOwnFlowThatNeverEnds: a cycle in the flow a block-declared node
// states of its own spends the action's token-flow budget, run by a body statement
// as it is, and reports that budget's error.
func testBlockNodeOwnFlowThatNeverEnds(t *testing.T) {
	src := `
		package test {
			action outer {
				attribute x : Integer = 3;
				first start;
				then action pick {
					if x > 0 {
						action leg {
							first a;
							action a;
							action b;
							succession first a then b;
							succession first b then a;
						}
					}
				}
				then done;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	ctx.maxActionSteps = 50
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
	if sym == nil {
		t.Fatal("action outer not found")
	}
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	err = exec.RunToCompletion()
	if !errors.Is(err, ErrActionStepLimitExceeded) {
		t.Fatalf("error = %v, want ErrActionStepLimitExceeded", err)
	}
	if !strings.Contains(err.Error(), MaxActionStepsEnvVar) {
		t.Errorf("error %q does not name %s", err, MaxActionStepsEnvVar)
	}
}

// testNodePinBoundToUnequalValues: two bindings at one input pin are two equalities,
// so unequal other ends are a binding conflict, not a declaration-order choice.
func testNodePinBoundToUnequalValues(t *testing.T) {
	src := `
		package test {` + adderActionDef + `
			action outer {
				attribute x : Integer = 1;
				attribute y : Integer = 2;
				bind add.a = x;
				bind add.a = y;
				first start;
				then action add : Adder { in b = 2; }
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, errBindingConflict) {
		t.Fatalf("error = %v, want errBindingConflict", err)
	}
	if got, want := err.Error(), "binding conflict at add.a: x = 1, y = 2"; got != want {
		t.Errorf("conflict error = %q, want %q", got, want)
	}
}

// testNestedPinBindingIntoANodePerformingAnotherAction: a node typed by an action def
// performs that action, whose nodes are its own, so a binding reaching into it is
// reported when the flow is built rather than routed to the node's like-named pin.
func testNestedPinBindingIntoANodePerformingAnotherAction(t *testing.T) {
	src := `
		package test {
			action def Leg {
				in w : Integer;
				first start;
				then action inner { in w : Integer; }
				then done;
			}
			action outer {
				attribute x : Integer = 5;
				bind leg.inner.w = x;
				first start;
				then action leg : Leg;
				then done;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
	if sym == nil {
		t.Fatal("action outer not found")
	}
	_, err := ctx.CreateActionExecutor(sym)
	want := `binding end "leg.inner.w" reaches into leg, which performs an action of its own rather than declaring inner; bind at a pin of leg itself`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want %q", err, want)
	}
}

// testNestedPinBindingAtAnUndeclaredPin: a binding reaching a nested node must name a
// pin that node declares; the inner node reports it as it begins.
func testNestedPinBindingAtAnUndeclaredPin(t *testing.T) {
	src := `
		package test {
			action outer {
				attribute x : Integer = 5;
				bind leg.inner.nope = x;
				first start;
				then action leg {
					first start;
					then action inner { in w : Integer; }
					then done;
				}
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, errBindingEnd) {
		t.Fatalf("error = %v, want errBindingEnd", err)
	}
	if !strings.Contains(err.Error(), "leg.inner.nope names no parameter or attribute of") {
		t.Errorf("error %q does not name the nested pin path", err)
	}
}

// testFlowReachingIntoANodesOwnFlow: a flow joins pins of the nodes of one flow, so an
// end reaching into a node's own flow is reported when the flow is built.
func testFlowReachingIntoANodesOwnFlow(t *testing.T) {
	src := `
		package test {
			action outer {
				first start;
				then action leg {
					first start;
					then action inner { out v : Integer; assign v := 1; }
					then done;
				}
				then action q { in n : Integer; }
				then done;
				flow leg.inner.v to q.n;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
	if sym == nil {
		t.Fatal("action outer not found")
	}
	_, err := ctx.CreateActionExecutor(sym)
	want := `end "leg.inner.v" reaches into a node's own flow`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want %q", err, want)
	}
}

// testNodeBindingOutputToAnUnknownFeature: what a node's output pin is bound to
// must be a feature the action holds, so the value has somewhere to go.
func testNodeBindingOutputToAnUnknownFeature(t *testing.T) {
	src := `
		package test {` + adderActionDef + `
			action outer {
				bind add.sum = nowhere;
				first start;
				then action add : Adder { in a = 1; in b = 2; }
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, errBindingEnd) {
		t.Fatalf("error = %v, want errBindingEnd", err)
	}
}

// testNodeBindingOutputThroughAScalarChain: a chained other end must reach an
// object whose feature the output can be written to.
func testNodeBindingOutputThroughAScalarChain(t *testing.T) {
	src := `
		package test {` + adderActionDef + `
			action outer {
				attribute x : Integer = 1;
				bind add.sum = x.value;
				first start;
				then action add : Adder { in a = 1; in b = 2; }
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, errBindingEnd) || !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("error = %v, want errBindingEnd wrapping ErrTypeMismatch", err)
	}
	if !strings.Contains(err.Error(), "x.value") {
		t.Errorf("error %q does not name the chained end x.value", err)
	}
}

// testNodeBindingOutputThroughAChainViolatesTargetType: an output written through a
// chain answers to the reached feature's declared type as a direct write does.
func testNodeBindingOutputThroughAChainViolatesTargetType(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			part def Holder { attribute label : String = "none"; }
			action outer {
				part holder : Holder;
				bind num.v = holder.label;
				first start;
				then action num { out v : Integer; assign v := 3; }
				then done;
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
	if sym == nil {
		t.Fatal("action outer not found")
	}
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	err = exec.RunToCompletion()
	if !errors.Is(err, errBindingEnd) || !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("error = %v, want errBindingEnd wrapping ErrTypeMismatch", err)
	}
}

// testNodeFlowIntoAPinTheTargetDoesNotDeclare: a flow's target pin must be a
// feature of the node it reaches.
func testNodeFlowIntoAPinTheTargetDoesNotDeclare(t *testing.T) {
	src := `
		package test {
			action outer {
				first start;
				then action p { out v : Integer; assign v := 1; }
				then action q { in w : Integer; }
				then done;
				flow p.v to q.nope;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, errNodePin) {
		t.Fatalf("error = %v, want errNodePin", err)
	}
}
