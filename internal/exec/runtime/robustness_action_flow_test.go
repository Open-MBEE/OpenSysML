package runtime

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessActionFlow covers malformed action control flow: starved joins,
// flows without a start, dangling successions, firsts, merges and decisions.
func TestRuntimeRobustnessActionFlow(t *testing.T) {
	t.Run("deadlock_join_starvation", testDeadlockJoinStarvation)
	t.Run("deadlock_join_same_succession_twice", testDeadlockJoinSameSuccessionTwice)
	t.Run("nested_flow_without_an_initial_node", testNestedFlowWithoutAnInitialNode)
	t.Run("nested_flow_with_a_dangling_succession", testNestedFlowWithADanglingSuccession)
	t.Run("nested_flow_that_cannot_progress", testNestedFlowThatCannotProgress)
	t.Run("nested_flow_that_never_ends", testNestedFlowThatNeverEnds)
	t.Run("action_flow_starts_at_its_unpreceded_step", testActionFlowStartsAtItsUnprecededStep)
	t.Run("action_flow_cycle_without_start", testActionFlowCycleWithoutStart)
	t.Run("action_nested_node_starts_at_its_unpreceded_step", testActionNestedNodeStartsAtItsUnprecededStep)
	t.Run("fork_without_a_successor", testForkWithoutASuccessor)
	t.Run("explicit_succession_missing_endpoint", testExplicitSuccessionMissingEndpoint)
	t.Run("control_flow_missing_endpoint", testControlFlowMissingEndpoint)
	t.Run("merge_without_a_successor", testMergeWithoutASuccessor)
	t.Run("unguarded_loop_through_a_merge", testUnguardedLoopThroughAMerge)
	t.Run("action_whose_last_node_has_no_succession", testActionWhoseLastNodeHasNoSuccession)
	t.Run("first_node_with_a_second_succession", testFirstNodeWithASecondSuccession)
	t.Run("first_beside_an_initial_node", testFirstBesideAnInitialNode)
	t.Run("two_one_ended_firsts", testTwoOneEndedFirsts)
	t.Run("first_naming_a_final_node", testFirstNamingAFinalNode)
	t.Run("fork_branches_assigning_the_same_feature", testForkBranchesAssigningTheSameFeature)
	t.Run("decision_no_satisfied_guard", testDecisionNoSatisfiedGuard)
	t.Run("decision_all_guards_false", testDecisionAllGuardsFalse)
	t.Run("flow_end_naming_no_node", testFlowEndNamingNoNode)
	t.Run("flow_naming_no_pin", testFlowNamingNoPin)
	t.Run("flow_from_a_node_that_produced_nothing", testFlowFromANodeThatProducedNothing)
	t.Run("succession_guard_failure_modes", testSuccessionGuardFailureModes)
}

// testSuccessionGuardFailureModes: a guard on a succession leaving an ordinary
// action node is evaluated, so its failure modes — a value that is not Boolean and
// a guard nothing supplies a name for — are each reported as a typed error rather
// than a panic or a hang. Two guards holding at once are no failure: each
// succession is its own HappensBefore link, so both targets are performed.
func testSuccessionGuardFailureModes(t *testing.T) {
	for _, tc := range []struct {
		name    string
		body    string
		want    error
		results map[string]int64
	}{
		{
			name: "guard is not a boolean",
			body: `
				attribute x : Integer = 1;
				attribute y : Integer = 0;
				action s1 assign x := 7;
				action s2 assign y := 9;
				first s1 if x + 1 then s2;
			`,
			want: ErrTypeMismatch,
		},
		{
			name: "guard reads a name nothing supplies",
			body: `
				attribute y : Integer = 0;
				action s1;
				action s2 assign y := 9;
				first s1 if missing > 5 then s2;
			`,
			want: ErrUnresolvedReference,
		},
		{
			name: "two guards hold at once",
			body: `
				attribute level : Integer = 12;
				attribute low : Integer = 0;
				attribute high : Integer = 0;
				action check assign level := level;
				action alert assign high := 1;
				action idle assign low := 1;
				first check;
				succession first check if level > 10 then alert;
				succession first check if level > 5 then idle;
			`,
			results: map[string]int64{"high": 1, "low": 1},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := "package test {\n private import ScalarValues::*;\n action guarded {" + tc.body + "}\n}"
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
			sym := findSymbolByName(idx.DocumentRoot("<test>"), "guarded", ast.DefAction)
			if sym == nil {
				t.Fatal("action guarded not found")
			}

			done := make(chan error, 1)
			go func() {
				defer func() {
					if r := recover(); r != nil {
						done <- fmt.Errorf("panic: %v", r)
					}
				}()
				results, err := ctx.ExecuteAction(sym)
				for name, want := range tc.results {
					if got, ok := results[name]; err == nil && (!ok || got.Const.Int != want) {
						err = fmt.Errorf("%s = %v, want %d", name, got, want)
					}
				}
				done <- err
			}()
			select {
			case err := <-done:
				if !errors.Is(err, tc.want) {
					t.Errorf("ExecuteAction err = %v, want %v", err, tc.want)
				}
			case <-watchdog(5 * time.Second):
				t.Fatal("executing the guarded action did not terminate")
			}
		})
	}
}

// testDeadlockJoinStarvation: join awaiting token that never arrives. `stranded`
// is a reference no succession leads to, so nothing performs it: the join has two
// incoming edges but can only ever be reached by one token.
func testDeadlockJoinStarvation(t *testing.T) {
	src := `
		package test {
			action starve {
				first start;
				ref action stranded;
				join sync;
				done;
				succession first start then sync;
				succession first stranded then sync;
				succession first sync then done;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	sym := findSymbolByName(idx.DocumentRoot("<test>"), "starve", ast.DefAction)
	if sym == nil {
		t.Fatal("action starve not found")
	}

	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}

	err = exec.RunToCompletion()
	if err == nil {
		t.Fatal("expected a deadlock error, the starved join completed")
	}
	if !errors.Is(err, ErrActionDeadlock) {
		t.Errorf("expected ErrActionDeadlock, got: %v", err)
	}
}

// testDeadlockJoinSameSuccessionTwice: two tokens reach the join over the one
// succession from the merge; they do not stand in for the succession from
// `stranded`, a reference nothing performs, so the join never fires.
func testDeadlockJoinSameSuccessionTwice(t *testing.T) {
	src := `
		package test {
			action starve {
				first start;
				fork split;
				action a;
				action b;
				merge m;
				ref action stranded;
				join sync;
				done;
				succession first start then split;
				succession first split then a;
				succession first split then b;
				succession first a then m;
				succession first b then m;
				succession first m then sync;
				succession first stranded then sync;
				succession first sync then done;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	sym := findSymbolByName(idx.DocumentRoot("<test>"), "starve", ast.DefAction)
	if sym == nil {
		t.Fatal("action starve not found")
	}

	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	if err := exec.RunToCompletion(); !errors.Is(err, ErrActionDeadlock) {
		t.Fatalf("error = %v, want ErrActionDeadlock", err)
	}
	for _, token := range exec.Tokens() {
		if awaiting := exec.Awaiting(token); len(awaiting) != 1 {
			t.Errorf("token %d awaits %d successions, want the one from stranded", token.ID, len(awaiting))
		}
	}
}

func testForkWithoutASuccessor(t *testing.T) {
	src := `
		package test {
			action broken {
				first start;
				fork split;
				succession first start then split;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "broken", ast.DefAction)
	if sym == nil {
		t.Fatal("action broken not found")
	}

	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	err = exec.RunToCompletion()
	if !errors.Is(err, errInvalidActionFlow) {
		t.Fatalf("error = %v, want errInvalidActionFlow", err)
	}
}

func testExplicitSuccessionMissingEndpoint(t *testing.T) {
	src := `
		package test {
			action broken {
				action compute;
				succession first missing then compute;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "broken", ast.DefAction)
	if sym == nil {
		t.Fatal("action broken not found")
	}

	_, err := ctx.CreateActionExecutor(sym)
	if err == nil || !strings.Contains(err.Error(), "action succession references undefined source node") {
		t.Fatalf("error = %v, want an explicit succession source diagnostic", err)
	}
}

func testControlFlowMissingEndpoint(t *testing.T) {
	src := `
		package test {
			action broken {
				first start;
				decide check;
				succession first start then check;
				if true then missing;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "broken", ast.DefAction)
	if sym == nil {
		t.Fatal("action broken not found")
	}

	_, err := ctx.CreateActionExecutor(sym)
	if err == nil || !strings.Contains(err.Error(), `control flow edge references undefined target "missing"`) {
		t.Fatalf("error = %v, want an undefined control-flow target diagnostic", err)
	}
}

func testMergeWithoutASuccessor(t *testing.T) {
	src := `
		package test {
			action broken {
				first start;
				merge converge;
				succession first start then converge;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "broken", ast.DefAction)
	if sym == nil {
		t.Fatal("action broken not found")
	}

	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	err = exec.RunToCompletion()
	if !errors.Is(err, errInvalidActionFlow) {
		t.Fatalf("error = %v, want errInvalidActionFlow", err)
	}
}

// testUnguardedLoopThroughAMerge: a merge passes every arrival, so a loop with no
// exit spins through it until the step budget stops the run with its typed error.
func testUnguardedLoopThroughAMerge(t *testing.T) {
	src := `
		package test {
			action spin {
				first start;
				merge m;
				action a;
				succession first start then m;
				succession first m then a;
				succession first a then m;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	ctx.maxActionSteps = 1000
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "spin", ast.DefAction)
	if sym == nil {
		t.Fatal("action spin not found")
	}

	// RunToCompletion spends the whole budget on the loop and reports it.
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	err = exec.RunToCompletion()
	if !errors.Is(err, ErrActionStepLimitExceeded) {
		t.Fatalf("RunToCompletion() = %v, want ErrActionStepLimitExceeded", err)
	}
	if !strings.Contains(err.Error(), MaxActionStepsEnvVar) {
		t.Errorf("error %q does not name %s", err, MaxActionStepsEnvVar)
	}

	// A debugger steps the loop one node at a time: each step is one bounded unit
	// of work, the token keeps circling m and a, and continuing from there hits the budget.
	ctx.maxActionSteps = 20
	stepped, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	for i := 0; i < 2*int(ctx.maxActionSteps); i++ {
		if err := stepped.Step(); err != nil {
			t.Fatalf("Step() %d = %v, want nil", i, err)
		}
		tokens := stepped.Tokens()
		if len(tokens) != 1 {
			t.Fatalf("after step %d: %d tokens, want 1 circling the loop", i, len(tokens))
		}
		_, atMerge := tokens[0].Location.(*ast.MergeNode)
		if atMerge != (i%2 == 0) {
			t.Fatalf("after step %d: token at %T, want it alternating between m and a", i, tokens[0].Location)
		}
	}
	err = stepped.RunToCompletion()
	if !errors.Is(err, ErrActionStepLimitExceeded) {
		t.Fatalf("RunToCompletion() after stepping = %v, want ErrActionStepLimitExceeded", err)
	}
}

// testActionWhoseLastNodeHasNoSuccession: a node the flow leads no further from
// ends the flow, so an action declaring no `done` node completes instead of
// failing, and stepping past the end neither errors nor spins.
func testActionWhoseLastNodeHasNoSuccession(t *testing.T) {
	src := `
		package test {
			action ends {
				first start;
				then action a;
				then action b;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	sym := findSymbolByName(idx.DocumentRoot("<test>"), "ends", ast.DefAction)
	if sym == nil {
		t.Fatal("action ends not found")
	}

	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}

	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run action whose last node has no succession: %v", err)
	}
	if exec.State() != StateCompleted {
		t.Fatalf("expected the action to complete, got state %s", exec.State())
	}

	for i := 0; i < 3; i++ {
		if err := exec.Step(); err != nil {
			t.Fatalf("step %d past the end: %v", i+1, err)
		}
		if exec.State() != StateCompleted {
			t.Fatalf("step %d past the end left state %s", i+1, exec.State())
		}
		if len(exec.Tokens()) != 0 {
			t.Fatalf("step %d past the end revived %d token(s)", i+1, len(exec.Tokens()))
		}
	}
}

// testFirstNodeWithASecondSuccession: `first s1 then s2;` is a succession out of
// s1, so a second succession out of that node is a second HappensBefore link
// from it, and both s2 and s3 are performed after s1.
func testFirstNodeWithASecondSuccession(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			action seq {
				attribute two : Integer = 0;
				attribute three : Integer = 0;
				action s1;
				action s2 { assign two := 2; }
				action s3 { assign three := 3; }
				first s1 then s2;
				succession first s1 then s3;
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))

	sym := findSymbolByName(idx.DocumentRoot("<test>"), "seq", ast.DefAction)
	if sym == nil {
		t.Fatal("action seq not found")
	}

	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}

	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("a first node with two successions: %v", err)
	}
	results := exec.Results()
	if results["two"].Const.Int != 2 || results["three"].Const.Int != 3 {
		t.Fatalf("two = %v, three = %v, want both successors performed", results["two"], results["three"])
	}
}

// testFirstBesideAnInitialNode: `first s1 then s2;` beside `first start;` is the
// succession s1 -> s2, not a second start: start is the initial node and has no
// edge of its own.
func testFirstBesideAnInitialNode(t *testing.T) {
	src := `
		package test {
			action seq {
				action s1;
				action s2;
				first start;
				first s1 then s2;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	sym := findSymbolByName(idx.DocumentRoot("<test>"), "seq", ast.DefAction)
	if sym == nil {
		t.Fatal("action seq not found")
	}

	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	initial, ok := exec.graph.Initial.(*ast.InitialNode)
	if !ok || initial.Name() != "start" {
		t.Fatalf("initial node = %T, want the `first start;` marker", exec.graph.Initial)
	}
	if edges := exec.graph.Edges[initial]; len(edges) != 0 {
		t.Fatalf("start has %d successions, want none", len(edges))
	}
	var s1, s2 ast.Node
	for _, node := range exec.graph.Nodes {
		if u, ok := node.(*ast.Usage); ok {
			switch u.Ident.Name {
			case "s1":
				s1 = node
			case "s2":
				s2 = node
			}
		}
	}
	if s1 == nil || s2 == nil {
		t.Fatal("s1 and s2 are not both nodes of the graph")
	}
	if edges := exec.graph.Edges[s1]; len(edges) != 1 || edges[0].Target != s2 {
		t.Fatalf("successions out of s1 = %d, want exactly s1 -> s2", len(edges))
	}
}

// testTwoOneEndedFirsts: two one-ended `first` ends each mark a start, and a
// body has one, so lowering rejects them.
func testTwoOneEndedFirsts(t *testing.T) {
	src := `
		package test {
			action seq {
				action a;
				action b;
				first a;
				first b;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	sym := findSymbolByName(idx.DocumentRoot("<test>"), "seq", ast.DefAction)
	if sym == nil {
		t.Fatal("action seq not found")
	}

	if _, err := ctx.CreateActionExecutor(sym); err == nil {
		t.Fatal("two starts lowered without an error")
	} else if !strings.Contains(err.Error(), "multiple initial nodes") {
		t.Fatalf("error = %q, want it to report multiple initial nodes", err)
	}
}

// testFirstNamingAFinalNode: a flow cannot start where it ends, so lowering
// rejects it rather than completing with the declared node never run.
func testFirstNamingAFinalNode(t *testing.T) {
	src := `
		package test {
			action seq {
				attribute x = 0;
				action s1 { assign x := 7; }
				done;
				first done;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	sym := findSymbolByName(idx.DocumentRoot("<test>"), "seq", ast.DefAction)
	if sym == nil {
		t.Fatal("action seq not found")
	}

	if _, err := ctx.CreateActionExecutor(sym); err == nil {
		t.Fatal("a first end naming a final node lowered without an error")
	} else if !strings.Contains(err.Error(), "final node done") {
		t.Fatalf("error = %q, want it to name the final node", err)
	}
}

// testForkBranchesAssigningTheSameFeature: concurrent branches writing one feature
// are unordered by the spec; the runtime resolves them by its own step order.
func testForkBranchesAssigningTheSameFeature(t *testing.T) {
	src := `
		package test {
			action clash {
				attribute x : Integer = 0;

				first start;
				fork split;
				action left { assign x := 1; }
				action right { assign x := 2; }
				join sync;
				done;

				succession first start then split;
				succession first split then left;
				succession first split then right;
				succession first left then sync;
				succession first right then sync;
				succession first sync then done;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	sym := findSymbolByName(idx.DocumentRoot("<test>"), "clash", ast.DefAction)
	if sym == nil {
		t.Fatal("action clash not found")
	}

	var first semantics.Value
	for run := 0; run < 3; run++ {
		exec, err := ctx.CreateActionExecutor(sym)
		if err != nil {
			t.Fatalf("run %d create action executor: %v", run+1, err)
		}
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("run %d conflicting writes: %v", run+1, err)
		}
		if exec.State() != StateCompleted {
			t.Fatalf("run %d left state %s", run+1, exec.State())
		}

		got, ok := exec.Results()["x"]
		if !ok {
			t.Fatalf("run %d lost the contested feature x", run+1)
		}
		if got.Kind != ValConst || got.Const.Kind != semantics.ValInt {
			t.Fatalf("run %d gave x a non-integer value: %+v", run+1, got)
		}
		if got.Const.Int != 1 && got.Const.Int != 2 {
			t.Fatalf("run %d gave x %d, which neither branch assigned", run+1, got.Const.Int)
		}
		if run == 0 {
			first = got.Const
			continue
		}
		if got.Const.Int != first.Int {
			t.Fatalf("run %d gave x %d after run 1 gave %d: execution is not deterministic",
				run+1, got.Const.Int, first.Int)
		}
	}
}

// testDecisionNoSatisfiedGuard: a decision must select one outgoing succession.
func testDecisionNoSatisfiedGuard(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;

			action noGuard {
				attribute enabled : Boolean = false;

				first start;
				action selected;
				done;

				succession first start then choose;
				succession first selected then done;

				decide choose;
				if enabled then selected;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "noGuard", ast.DefAction)
	if sym == nil {
		t.Fatal("action noGuard not found")
	}

	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	err = exec.RunToCompletion()
	if !errors.Is(err, errNoEnabledSuccession) {
		t.Fatalf("error = %v, want errNoEnabledSuccession", err)
	}
}

// testDecisionAllGuardsFalse: every guard of a decision is evaluated so that
// several holding at once can be reported; none holding is still the same
// error, recorded as no choice at all.
func testDecisionAllGuardsFalse(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;

			action pick {
				attribute level : Integer = 5;

				first start;
				action low;
				action high;
				done;

				succession first start then choose;
				succession first low then done;
				succession first high then done;

				decide choose;
				if level > 10 then low;
				if level > 20 then high;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "pick", ast.DefAction)
	if sym == nil {
		t.Fatal("action pick not found")
	}

	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	err = exec.RunToCompletion()
	if !errors.Is(err, errNoEnabledSuccession) {
		t.Fatalf("error = %v, want errNoEnabledSuccession", err)
	}
	if !strings.Contains(err.Error(), "decision node choose has no true guard") {
		t.Fatalf("error = %v, want the decision named", err)
	}
	if choices := ctx.Choices(); len(choices) != 0 {
		t.Fatalf("choices = %v, want none when no guard holds", choices)
	}
}

// testFlowEndNamingNoNode: a flow moves a value from one action node's output
// to another's input, so an end naming something that is not a node of the
// action is reported rather than dropped, which would leave the flow declared
// and carrying nothing.
func testFlowEndNamingNoNode(t *testing.T) {
	cases := map[string]string{
		"source": "flow bad from missing.engineTorque to amplify.torqueIn;",
		"target": "flow bad from generate.engineTorque to missing.torqueIn;",
	}

	for name, flow := range cases {
		t.Run(name, func(t *testing.T) {
			src := `
				package test {
					action driveTrain {
						first start;
						action generate { out engineTorque : Integer; assign engineTorque := 1; }
						action amplify { in torqueIn : Integer[1]; }
						done;
						succession first start then generate;
						succession first generate then amplify;
						succession first amplify then done;
						` + flow + `
					}
				}
			`
			idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
			sym := findSymbolByName(idx.DocumentRoot("<test>"), "driveTrain", ast.DefAction)
			if sym == nil {
				t.Fatal("action driveTrain not found")
			}

			_, err := ctx.ExecuteAction(sym)
			if err == nil {
				t.Fatalf("expected the flow's %s end to be reported", name)
			}
			if !strings.Contains(err.Error(), "flow bad") ||
				!strings.Contains(err.Error(), name) {
				t.Errorf("error does not name the flow and the end at fault: %v", err)
			}
		})
	}
}

// testFlowNamingNoPin: a flow carries the value of a feature, so a flow whose
// ends name nodes alone and which declares no payload identifies nothing to
// move, and is reported when the graph is built rather than mid-run.
func testFlowNamingNoPin(t *testing.T) {
	_, err := executeActionSource(t, "driveTrain", `package test {
		action driveTrain {
			first start;
			action generate { out engineTorque : Integer; assign engineTorque := 1; }
			action amplify { in engineTorque : Integer; }
			done;
			succession first start then generate;
			succession first generate then amplify;
			succession first amplify then done;
			flow generateToAmplify from generate to amplify;
		}
	}`)
	if err == nil {
		t.Fatal("expected the flow naming no feature to be reported")
	}
	if !strings.Contains(err.Error(), "generateToAmplify") ||
		!strings.Contains(err.Error(), "names no feature to carry") {
		t.Errorf("error does not say the flow names no feature: %v", err)
	}
}

// testFlowFromANodeThatProducedNothing: a flow out of a node that left its
// source pin empty carries nothing, which is reported rather than silently
// leaving the target pin unwritten.
func testFlowFromANodeThatProducedNothing(t *testing.T) {
	src := `
		package test {
			action driveTrain {
				first start;
				action generate { out engineTorque : Integer[1]; }
				action amplify { in torqueIn : Integer[1]; }
				done;
				succession first start then generate;
				succession first generate then amplify;
				succession first amplify then done;
				flow generateToAmplify from generate.engineTorque to amplify.torqueIn;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "driveTrain", ast.DefAction)
	if sym == nil {
		t.Fatal("action driveTrain not found")
	}

	_, err := ctx.ExecuteAction(sym)
	if err == nil {
		t.Fatal("expected the empty source pin to be reported")
	}
	if !strings.Contains(err.Error(), "generateToAmplify") ||
		!strings.Contains(err.Error(), "engineTorque") {
		t.Errorf("error does not name the flow and the pin that stayed empty: %v", err)
	}
}

// testNestedFlowWithoutAnInitialNode: a node stating a flow with no node to
// start at — a cycle, every node preceded — is reported at initialize(), not
// run as a leaf.
func testNestedFlowWithoutAnInitialNode(t *testing.T) {
	src := `
		package test {
			action outer {
				first leg;
				action leg {
					action a;
					action b;
					succession first a then b;
					succession first b then a;
				}
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
	if sym == nil {
		t.Fatal("action outer not found")
	}

	// The constructor is permissive; the flow's failure surfaces at initialize().
	exec, err := newActionExecutor(ctx, sym, nil)
	if err != nil {
		t.Fatalf("newActionExecutor: %v", err)
	}
	err = exec.initialize()
	if !errors.Is(err, errInvalidActionFlow) {
		t.Fatalf("error = %v, want errInvalidActionFlow", err)
	}
	if !strings.Contains(err.Error(), "leg") {
		t.Errorf("error = %v, want it to name the node whose flow cannot be built", err)
	}
}

// testNestedFlowWithADanglingSuccession: a succession inside a node's own flow
// naming nothing is that node's error, reported rather than dropped.
func testNestedFlowWithADanglingSuccession(t *testing.T) {
	src := `
		package test {
			action outer {
				first leg;
				action leg {
					first a;
					action a;
					succession first a then missing;
				}
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
	if sym == nil {
		t.Fatal("action outer not found")
	}

	exec, err := newActionExecutor(ctx, sym, nil)
	if err != nil {
		t.Fatalf("newActionExecutor: %v", err)
	}
	if err := exec.initialize(); !errors.Is(err, errInvalidActionFlow) {
		t.Fatalf("error = %v, want errInvalidActionFlow", err)
	}
}

// testNestedFlowThatCannotProgress: a join inside a node's own flow that can
// never be reached by both its tokens deadlocks the run, reported rather than hung.
func testNestedFlowThatCannotProgress(t *testing.T) {
	src := `
		package test {
			action outer {
				first leg;
				action leg {
					first s;
					action s;
					ref action stranded;
					join sync;
					done;
					succession first s then sync;
					succession first stranded then sync;
					succession first sync then done;
				}
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
	if sym == nil {
		t.Fatal("action outer not found")
	}

	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	if err := exec.RunToCompletion(); !errors.Is(err, ErrActionDeadlock) {
		t.Fatalf("error = %v, want ErrActionDeadlock", err)
	}
}

// testNestedFlowThatNeverEnds: a cycle inside a node's own flow spends the
// action's step budget rather than running forever.
func testNestedFlowThatNeverEnds(t *testing.T) {
	src := `
		package test {
			action outer {
				first leg;
				action leg {
					first a;
					action a;
					action b;
					succession first a then b;
					succession first b then a;
				}
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	ctx.maxActionSteps = 1000
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
	if sym == nil {
		t.Fatal("action outer not found")
	}

	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	if err := exec.RunToCompletion(); !errors.Is(err, ErrActionStepLimitExceeded) {
		t.Fatalf("error = %v, want ErrActionStepLimitExceeded", err)
	}
}

// testActionFlowStartsAtItsUnprecededStep: an action definition performed whole
// starts at its one unpreceded node the same way.
func testActionFlowStartsAtItsUnprecededStep(t *testing.T) {
	outputs, err := executeActionSource(t, "Count", `package P {
		private import ScalarValues::*;
		action def Count {
			attribute total : Integer = 1;
			action a { assign total := total + 1; }
			then action b { assign total := total * 10; }
		}
	}`)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertIntOutput(t, outputs, "total", 20)
}

// testActionFlowCycleWithoutStart: successions closing a cycle over every node
// leave nothing to start at; the error says so rather than the run hanging.
func testActionFlowCycleWithoutStart(t *testing.T) {
	_, err := executeActionSource(t, "Loop", `package P {
		private import ScalarValues::*;
		action def Loop {
			attribute total : Integer = 0;
			action a { assign total := total + 1; }
			action b { assign total := total + 1; }
			succession first a then b;
			succession first b then a;
		}
	}`)
	if !errors.Is(err, errInvalidActionFlow) {
		t.Fatalf("expected errInvalidActionFlow, got: %v", err)
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error %q does not name the cycle", err)
	}
}

// testActionNestedNodeStartsAtItsUnprecededStep: a node of an action definition
// stating its flow the same way starts there too.
func testActionNestedNodeStartsAtItsUnprecededStep(t *testing.T) {
	outputs, err := executeActionSource(t, "Count", `package P {
		private import ScalarValues::*;
		action def Count {
			attribute total : Integer = 1;
			action inner {
				action a { assign total := total + 1; }
				action b { assign total := total * 10; }
				succession first a then b;
			}
		}
	}`)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertIntOutput(t, outputs, "total", 20)
}
