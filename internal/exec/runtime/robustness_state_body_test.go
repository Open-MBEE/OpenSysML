package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessStateBody covers entry, do and exit bodies of states: dangling and
// unstarted flows, unbound typed-action inputs and bodies that wait or never end.
func TestRuntimeRobustnessStateBody(t *testing.T) {
	t.Run("state_entry_action_input_bound_by_nothing", testStateEntryActionInputBoundByNothing)
	t.Run("state_block_typed_node_input_bound_by_nothing", testStateBlockTypedNodeInputBoundByNothing)
	t.Run("state_block_node_unvalued_pin_write_checked", testStateBlockNodeUnvaluedPinWriteChecked)
	t.Run("state_block_node_pin_read_before_performed", testStateBlockNodePinReadBeforePerformed)
	t.Run("state_block_node_bound_at_no_pin", testStateBlockNodeBoundAtNoPin)
	t.Run("state_block_node_own_flow_runs", testStateBlockNodeOwnFlowRuns)
	t.Run("state_do_body_dangling_succession", testStateDoBodyDanglingSuccession)
	t.Run("state_do_body_first_then_undefined", testStateDoBodyFirstThenUndefined)
	t.Run("state_do_body_flow_without_start", testStateDoBodyFlowWithoutStart)
	t.Run("state_do_body_starts_at_its_unpreceded_step", testStateDoBodyStartsAtItsUnprecededStep)
	t.Run("state_do_body_nested_node_dangling_succession", testStateDoBodyNestedNodeDanglingSuccession)
	t.Run("state_do_body_nested_node_starts_at_its_unpreceded_step", testStateDoBodyNestedNodeStartsAtItsUnprecededStep)
	t.Run("state_entry_body_dangling_succession", testStateEntryBodyDanglingSuccession)
	t.Run("state_do_typed_action_input_unbound", testStateDoTypedActionInputUnbound)
	t.Run("state_do_typed_action_pin_bound_to_missing_feature", testStateDoTypedActionPinBoundToMissingFeature)
	t.Run("state_do_typed_action_inout_valued_by_an_imported_literal", testStateDoTypedActionInoutValuedByAnImportedLiteral)
	t.Run("state_entry_body_waits_for_the_clock", testStateEntryBodyWaitsForTheClock)
	t.Run("state_exit_body_waits_for_the_clock", testStateExitBodyWaitsForTheClock)
	t.Run("transition_effect_body_waits_for_the_clock", testTransitionEffectBodyWaitsForTheClock)
	t.Run("state_do_body_flow_that_never_ends", testStateDoBodyFlowThatNeverEnds)
	t.Run("state_do_body_node_return_parameter", testStateDoBodyNodeReturnParameter)
	t.Run("state_do_body_return_parameter", testStateDoBodyReturnParameter)
	t.Run("non_terminating_do_behavior", testNonTerminatingDoBehavior)
	t.Run("empty_anonymous_action_body", testEmptyAnonymousActionBody)
	t.Run("non_terminating_anonymous_do_body", testNonTerminatingAnonymousDoBody)
	t.Run("qualified_assignment_target_in_a_state_effect", testQualifiedAssignmentTargetInAStateEffect)
}

// testNonTerminatingDoBehavior: a do behavior whose state is re-entered every
// round never ends, so the run is bounded and reports instead of hanging. The
// self-transition restarts the do behavior, so either bound may report first.
func testNonTerminatingDoBehavior(t *testing.T) {
	spin := &ast.StateNode{
		Name: "spin",
		Do: []ast.Node{&ast.AssignmentActionNode{
			Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "ticks"}}},
			Value:  &ast.LiteralInteger{Value: "1"},
		}},
	}
	exec := stateExecutorFor(t, &ast.Usage{
		Kind:  ast.UsageState,
		Ident: ast.Identification{Name: "Machine"},
		Members: []ast.Node{
			entryStart("init"),
			&ast.StateNode{Name: "init"},
			spin,
			transitionMember("init", "spin"),
			transitionMember("spin", "spin"),
		},
	})
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	err := exec.RunToCompletion()
	if err == nil {
		t.Fatal("expected a budget error for a machine that never settles")
	}
	if !strings.Contains(err.Error(), "exceeded max") && !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("expected a budget error, got: %v", err)
	}
}

// testEmptyAnonymousActionBody: entry, do and exit bodies stating no statement
// run the machine to completion rather than reporting an unexecutable behavior.
func testEmptyAnonymousActionBody(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			entry; then start;
			state start;
			state quiet {
				entry action { }
				do action { }
				exit action { }
			}
			state done;
			succession first start then quiet;
			succession first quiet then done;
		}
	}`)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run to completion: %v", err)
	}
	current, ok := exec.CurrentState().(*ast.StateNode)
	if !ok || current.Name != "done" {
		t.Errorf("expected the empty bodies to leave the machine in done, got %v", exec.CurrentState())
	}
}

// testNonTerminatingAnonymousDoBody: a do body that never finishes spends the
// step budget instead of hanging the machine.
func testNonTerminatingAnonymousDoBody(t *testing.T) {
	err := stateRunErrorForSource(t, "Machine", `package test {
		state Machine {
			attribute c : Integer = 0;
			entry; then start;
			state start;
			state spin {
				do action {
					while true {
						assign c := c + 1;
					}
				}
			}
			state done;
			succession first start then spin;
			succession first spin then done;
		}
	}`)
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("expected ErrStepLimitExceeded, got: %v", err)
	}
}

// testQualifiedAssignmentTargetInAStateEffect: an assignment naming more than
// one segment is reported rather than writing the last segment.
func testQualifiedAssignmentTargetInAStateEffect(t *testing.T) {
	err := stateRunErrorForSource(t, "Machine", `package test {
		package other { attribute c : Integer = 0; }
		state Machine {
			attribute c : Integer = 0;
			entry; then init;
			state init;
			state active;
			succession first init then active;
			transition first active do assign other::c := 1 then done;
		}
	}`)
	if err == nil {
		t.Fatal("expected a qualified assignment target to be reported")
	}
	if !strings.Contains(err.Error(), "assignment to a qualified target") {
		t.Errorf("expected the report to name the unsupported target, got: %v", err)
	}
}

// testStateEntryActionInputBoundByNothing: a state's entry action is an
// invocation too, so an input it leaves unbound is reported before it runs.
func testStateEntryActionInputBoundByNothing(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {`+adderActionDef+`
		action adder : Adder;
		state Machine {
			attribute a : Integer = 1;
			entry; then init;
			state init;
			state active {
				entry adder;
			}
			succession first init then active;
			succession first active then done;
		}
	}`)
	err := exec.RunToCompletion()
	if !errors.Is(err, errUnboundParameter) {
		t.Fatalf("error = %v, want errUnboundParameter", err)
	}
	if !strings.Contains(err.Error(), "b") {
		t.Errorf("error %q does not name the unbound parameter", err)
	}
}

// testStateBlockTypedNodeInputBoundByNothing: a typed node in a branch of a
// state's body binds the callee's inputs from the pins it declares and the
// values in scope, so one it leaves unbound is reported before its body runs.
func testStateBlockTypedNodeInputBoundByNothing(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {`+adderActionDef+`
		state Machine {
			attribute total : Integer = 0;
			entry; then init;
			state init;
			state active {
				entry action {
					if total == 0 {
						action adding : Adder {
							in a = 1;
							assign total := sum;
						}
					}
				}
			}
			succession first init then active;
			succession first active then done;
		}
	}`)
	err := exec.RunToCompletion()
	if !errors.Is(err, errUnboundParameter) {
		t.Fatalf("error = %v, want errUnboundParameter", err)
	}
	if !strings.Contains(err.Error(), "b") {
		t.Errorf("error %q does not name the unbound parameter", err)
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(0)) {
		t.Errorf("total = %v, want 0: the node's body must not run", total)
	}
}

// testStateBlockNodePinReadBeforePerformed: a state body's node has a frame of its
// own, so a sibling reading its pin before it performs is errNodeNotPerformed.
func testStateBlockNodePinReadBeforePerformed(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		private import ScalarValues::*;
		state Machine {
			attribute total : Integer = 0;
			entry; then init;
			state init;
			state active {
				entry action {
					if total == 0 {
						action early { assign total := late.v; }
						action late { out v : Integer; assign v := 1; }
					}
				}
			}
			succession first init then active;
			succession first active then done;
		}
	}`)
	err := exec.RunToCompletion()
	if !errors.Is(err, errNodeNotPerformed) {
		t.Fatalf("error = %v, want errNodeNotPerformed", err)
	}
	if !strings.Contains(err.Error(), "late") {
		t.Errorf("error %q does not name the node", err)
	}
}

// testStateBlockNodeBoundAtNoPin: a binding in a state body at a pin the node does
// not declare is errBindingEnd, as it is in an action's flow.
func testStateBlockNodeBoundAtNoPin(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {`+adderActionDef+`
		state Machine {
			attribute total : Integer = 0;
			entry; then init;
			state init;
			state active {
				entry action {
					if total == 0 {
						bind adding.c = total;
						action adding : Adder { in a = 1; in b = 2; }
						assign total := adding.sum;
					}
				}
			}
			succession first init then active;
			succession first active then done;
		}
	}`)
	err := exec.RunToCompletion()
	if !errors.Is(err, errBindingEnd) {
		t.Fatalf("error = %v, want errBindingEnd", err)
	}
	if !strings.Contains(err.Error(), "c") {
		t.Errorf("error %q does not name the pin", err)
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(0)) {
		t.Errorf("total = %v, want 0: the node must not run", total)
	}
}

// testStateBlockNodeOwnFlowRuns: a node of a state behavior's body stating a
// flow of its own runs that flow to its end when the block reaches it, as a node
// of an action body does.
func testStateBlockNodeOwnFlowRuns(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		private import ScalarValues::*;
		state Machine {
			attribute total : Integer = 0;
			entry; then init;
			state init;
			state active {
				entry action {
					if total == 0 {
						action step {
							first start;
							then action one { assign total := total + 1; }
							then action two { assign total := total * 10; }
							then done;
						}
					}
				}
			}
			succession first init then active;
			succession first active then done;
		}
	}`)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(10)) {
		t.Errorf("total = %v, want 10: the node's flow runs one then two", total)
	}
}

// testStateDoBodyDanglingSuccession: a succession of an inline do body naming a
// node the body does not declare is reported as a flow that cannot be built,
// naming the target, and no node of the body runs.
func testStateDoBodyDanglingSuccession(t *testing.T) {
	exec := stateWithDoBody(t, `
		first start;
		then action a { assign total := total + 1; }
		succession a then missing;
	`)
	err := exec.RunToCompletion()
	if !errors.Is(err, errStatementNotExecutable) {
		t.Fatalf("expected errStatementNotExecutable, got: %v", err)
	}
	if !strings.Contains(err.Error(), "state behavior ops") || !strings.Contains(err.Error(), `"missing"`) {
		t.Errorf("error %q does not name the behavior and the undefined target", err)
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(0)) {
		t.Errorf("total = %v, want 0: no node of the body must run", total)
	}
}

// testStateDoBodyFirstThenUndefined: `first a then b` in an inline do body where
// b is not declared is reported the same way.
func testStateDoBodyFirstThenUndefined(t *testing.T) {
	exec := stateWithDoBody(t, `
		action a { assign total := total + 1; }
		first a then b;
	`)
	err := exec.RunToCompletion()
	if !errors.Is(err, errStatementNotExecutable) {
		t.Fatalf("expected errStatementNotExecutable, got: %v", err)
	}
	if !strings.Contains(err.Error(), `"b"`) {
		t.Errorf("error %q does not name the undefined target", err)
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(0)) {
		t.Errorf("total = %v, want 0: no node of the body must run", total)
	}
}

// testStateDoBodyFlowWithoutStart: successions that give the flow no node to
// start at — a cycle and nothing first — are an invalid flow, not a hang.
func testStateDoBodyFlowWithoutStart(t *testing.T) {
	exec := stateWithDoBody(t, `
		action a { assign total := total + 1; }
		action b { assign total := total + 1; }
		succession first a then b;
		succession first b then a;
	`)
	err := exec.RunToCompletion()
	if !errors.Is(err, errInvalidActionFlow) {
		t.Fatalf("expected errInvalidActionFlow, got: %v", err)
	}
	if !strings.Contains(err.Error(), "no node starts the flow") {
		t.Errorf("error %q does not say what the flow lacks", err)
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(0)) {
		t.Errorf("total = %v, want 0: no node of the body must run", total)
	}
}

// testStateDoBodyStartsAtItsUnprecededStep: a do body written in declaration
// order, with no `first`, starts at the one node no succession leads to.
func testStateDoBodyStartsAtItsUnprecededStep(t *testing.T) {
	exec := stateWithDoBody(t, `
		action a { assign total := total + 1; }
		then action b { assign total := total * 10; }
	`)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(10)) {
		t.Errorf("total = %v, want 10: a then b", total)
	}
}

// testStateDoBodyNestedNodeDanglingSuccession: the flow a node of an inline do
// body states of its own is validated with the body's before any node runs, so
// a dangling succession in it is reported naming the node and the target.
func testStateDoBodyNestedNodeDanglingSuccession(t *testing.T) {
	exec := stateWithDoBody(t, `
		first start;
		then action a { assign total := total + 1; }
		then action step {
			first start;
			then action one { assign total := total + 1; }
			succession one then missing;
		}
		then done;
	`)
	err := exec.RunToCompletion()
	if !errors.Is(err, errInvalidActionFlow) {
		t.Fatalf("expected errInvalidActionFlow, got: %v", err)
	}
	if !strings.Contains(err.Error(), "action node step") || !strings.Contains(err.Error(), `"missing"`) {
		t.Errorf("error %q does not name the node and the undefined target", err)
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(0)) {
		t.Errorf("total = %v, want 0: no node of the body must run", total)
	}
}

// testStateDoBodyNestedNodeStartsAtItsUnprecededStep: a node of an inline do
// body stating its flow in declaration order, with no `first`, starts at the one
// node no succession leads to, as the body itself does.
func testStateDoBodyNestedNodeStartsAtItsUnprecededStep(t *testing.T) {
	exec := stateWithDoBody(t, `
		action inner {
			action a { assign total := total + 1; }
			action b { assign total := total * 10; }
			succession first a then b;
		}
	`)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(10)) {
		t.Errorf("total = %v, want 10: a then b", total)
	}
}

// testStateEntryBodyDanglingSuccession: an inline entry body's flow is built the
// same way, so its dangling succession is reported on entering the state.
func testStateEntryBodyDanglingSuccession(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		private import ScalarValues::*;
		state Machine {
			attribute total : Integer = 0;
			entry; then init;
			state init;
			state active {
				entry action prep {
					first start;
					then action a { assign total := total + 1; }
					then missing;
				}
			}
			succession first init then active;
			succession first active then done;
		}
	}`)
	err := exec.RunToCompletion()
	if !errors.Is(err, errStatementNotExecutable) {
		t.Fatalf("expected errStatementNotExecutable, got: %v", err)
	}
	if !strings.Contains(err.Error(), "state behavior prep") || !strings.Contains(err.Error(), `"missing"`) {
		t.Errorf("error %q does not name the behavior and the undefined target", err)
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(0)) {
		t.Errorf("total = %v, want 0: no node of the body must run", total)
	}
}

// testStateDoBodyNodeReturnParameter: an action node of an inline do body's flow
// declaring `return` is refused before any node runs, as in a standalone action.
func testStateDoBodyNodeReturnParameter(t *testing.T) {
	exec := stateWithDoBody(t, `
		first start;
		then action a { assign total := total + 1; }
		then action b { return r : Integer = 1; }
		then done;
	`)
	err := exec.RunToCompletion()
	if !errors.Is(err, errActionResultParameter) {
		t.Fatalf("expected errActionResultParameter, got: %v", err)
	}
	if !strings.Contains(err.Error(), "state behavior ops") || !strings.Contains(err.Error(), "action node b") {
		t.Errorf("error %q does not name the behavior and the node", err)
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(0)) {
		t.Errorf("total = %v, want 0: no node of the body must run", total)
	}
}

// testStateDoBodyReturnParameter: an inline do body stating a flow and declaring
// `return` itself is refused before any node runs, as a standalone action is.
func testStateDoBodyReturnParameter(t *testing.T) {
	exec := stateWithDoBody(t, `
		return r : Integer = 1;
		first start;
		then action a { assign total := total + 1; }
		then done;
	`)
	err := exec.RunToCompletion()
	if !errors.Is(err, errActionResultParameter) {
		t.Fatalf("expected errActionResultParameter, got: %v", err)
	}
	if !strings.Contains(err.Error(), "action ops declares `return r`") {
		t.Errorf("error %q does not name the body's parameter", err)
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(0)) {
		t.Errorf("total = %v, want 0: no node of the body must run", total)
	}
}

// testStateDoBodyFlowThatNeverEnds: a cycle of successions in an inline do body
// spends the step budget and is reported, rather than running forever.
func testStateDoBodyFlowThatNeverEnds(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		private import ScalarValues::*;
		state Machine {
			attribute total : Integer = 0;
			entry; then active;
			state active {
				do action ops {
					first start;
					then action a { assign total := total + 1; }
					then action b { assign total := total + 1; }
					succession first b then a;
				}
			}
			succession first active then done;
		}
	}`))
	ctx.maxActionSteps = 50
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("state machine Machine not found")
	}
	exec, err := newStateExecutor(ctx, sym, nil)
	if err != nil {
		t.Fatalf("newStateExecutor: %v", err)
	}
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	err = exec.RunToCompletion()
	if !errors.Is(err, ErrActionStepLimitExceeded) {
		t.Fatalf("expected ErrActionStepLimitExceeded, got: %v", err)
	}
	if !strings.Contains(err.Error(), "possible infinite loop") {
		t.Errorf("error %q does not point at the loop", err)
	}
}

// testStateBlockNodeUnvaluedPinWriteChecked: a write to a pin a node in a state's
// body declares without a value is checked against that pin's declaration, and the
// machine's same-named attribute is left as it was.
func testStateBlockNodeUnvaluedPinWriteChecked(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package test {
		private import ScalarValues::*;
		state Machine {
			attribute v : Integer = 100;
			entry; then init;
			state init;
			state active {
				entry action {
					if v == 100 {
						action p {
							out v : Integer;
							assign v := "one";
						}
					}
				}
			}
			succession first init then active;
			succession first active then done;
		}
	}`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("state machine Machine not found")
	}
	exec, err := newStateExecutor(ctx, sym, nil)
	if err != nil {
		t.Fatalf("newStateExecutor: %v", err)
	}
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	err = exec.RunToCompletion()
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("error = %v, want ErrTypeMismatch", err)
	}
	if !strings.Contains(err.Error(), "v") {
		t.Errorf("error %q does not name the pin written", err)
	}
	if v := exec.StateData()["v"]; !valueEqual(v, integerValue(100)) {
		t.Errorf("v = %v, want the machine's 100: the node's pin is not the machine's attribute", v)
	}
}

// testStateDoTypedActionInputUnbound: a typed do usage that binds none of the
// action's input parameters is refused when the state's do behavior starts,
// naming the action and the parameter, and the action does not run.
func testStateDoTypedActionInputUnbound(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		private import ScalarValues::*;
		action def Poll { in n : Integer[1]; assign n := n + 1; }
		state Machine {
			attribute total : Integer = 0;
			entry; then active;
			state active { do action poll : Poll { } }
			succession first active then done;
		}
	}`)
	err := exec.RunToCompletion()
	if !errors.Is(err, errUnboundParameter) {
		t.Fatalf("expected errUnboundParameter, got: %v", err)
	}
	for _, want := range []string{"do action in state active", "action Poll", "input parameter n"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not say %q", err, want)
		}
	}
	assertCurrentState(t, exec, "active")
}

// testStateDoTypedActionPinBoundToMissingFeature: a typed do usage binding a pin
// to a feature the state does not declare is refused naming the pin, and the
// action does not run.
func testStateDoTypedActionPinBoundToMissingFeature(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		private import ScalarValues::*;
		action def Poll { in n : Integer; assign n := n + 1; }
		state Machine {
			attribute total : Integer = 0;
			entry; then active;
			state active { do action poll : Poll { in n = nothing; } }
			succession first active then done;
		}
	}`)
	err := exec.RunToCompletion()
	if !errors.Is(err, ErrUnresolvedReference) {
		t.Fatalf("expected ErrUnresolvedReference, got: %v", err)
	}
	for _, want := range []string{"do action in state active", "n of node poll", "nothing"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not say %q", err, want)
		}
	}
	assertCurrentState(t, exec, "active")
}

// testStateDoTypedActionInoutValuedByAnImportedLiteral: an `inout` pin valued by
// an enumeration literal reached through an import holds the literal as its
// initial value; the performance ends without writing back to it, while the
// pin valued by a feature writes back to that feature.
func testStateDoTypedActionInoutValuedByAnImportedLiteral(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		private import ScalarValues::*;
		enum def Mode { idle; busy; }
		private import Mode::*;
		action def Poll {
			inout n : Integer;
			inout mode : Mode;
			first start;
			then action count assign n := if mode == idle ? n + 1 else n + 100;
			then action flip assign mode := busy;
			then done;
		}
		state Machine {
			attribute total : Integer = 0;
			entry; then active;
			state active { do action poll : Poll { inout n = total; inout mode = idle; } }
			succession first active then done;
		}
	}`)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(1)) {
		t.Errorf("total = %v, want 1: n counted from mode idle and wrote back", total)
	}
	assertCurrentState(t, exec, "done")
}

// testStateEntryBodyWaitsForTheClock: an entry body whose flow waits for the
// clock is refused when the state is entered, naming the wait; a state is
// entered at one instant, and only its do behavior may wait.
func testStateEntryBodyWaitsForTheClock(t *testing.T) {
	exec := stateMachineWithLibraries(t, `package test {
		private import ScalarValues::*;
		private import SI::*;
		state Machine {
			attribute total : Integer = 0;
			entry; then active;
			state active {
				entry action {
					action w accept after 1 [s];
					then action c assign total := 1;
				}
			}
			succession first active then done;
		}
	}`)
	err := exec.initialize()
	if !errors.Is(err, errStateBehaviorWaits) {
		t.Fatalf("expected errStateBehaviorWaits, got: %v", err)
	}
	for _, want := range []string{"enter state active", "entry action", "t=1.0"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not say %q", err, want)
		}
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(0)) {
		t.Errorf("total = %v, want 0: nothing after the wait must run", total)
	}
	if got := len(exec.ctx.Clock().Waits()); got != 0 {
		t.Errorf("%d wait(s) left on the clock by the refused entry body", got)
	}
}

// testStateExitBodyWaitsForTheClock: an exit body whose flow waits for the clock
// is refused when the state is left, naming the wait, and time does not advance.
func testStateExitBodyWaitsForTheClock(t *testing.T) {
	exec := stateMachineWithLibraries(t, `package test {
		private import ScalarValues::*;
		private import SI::*;
		state Machine {
			attribute total : Integer = 0;
			entry; then active;
			state active {
				exit action {
					action w accept after 1 [s];
					then action c assign total := 1;
				}
			}
			succession first active then done;
		}
	}`)
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	err := exec.RunToCompletion()
	if !errors.Is(err, errStateBehaviorWaits) {
		t.Fatalf("expected errStateBehaviorWaits, got: %v", err)
	}
	for _, want := range []string{"exit state", "exit action", "t=1.0"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not say %q", err, want)
		}
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(0)) {
		t.Errorf("total = %v, want 0: nothing after the wait must run", total)
	}
	if now := exec.ctx.Clock().Now(); now != 0 {
		t.Errorf("clock at %v, want 0: a refused exit body must not advance time", now)
	}
}

// testTransitionEffectBodyWaitsForTheClock: a transition effect whose flow waits
// for the clock is refused when the transition fires, naming the wait.
func testTransitionEffectBodyWaitsForTheClock(t *testing.T) {
	exec := stateMachineWithLibraries(t, `package test {
		private import ScalarValues::*;
		private import SI::*;
		state Machine {
			attribute total : Integer = 0;
			entry; then active;
			state active;
			transition first active then done do action {
				action w accept after 1 [s];
				then action c assign total := 1;
			}
		}
	}`)
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	err := exec.RunToCompletion()
	if !errors.Is(err, errStateBehaviorWaits) {
		t.Fatalf("expected errStateBehaviorWaits, got: %v", err)
	}
	for _, want := range []string{"transition effect", "t=1.0"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not say %q", err, want)
		}
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(0)) {
		t.Errorf("total = %v, want 0: nothing after the wait must run", total)
	}
}
