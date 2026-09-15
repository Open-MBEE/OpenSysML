package runtime

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

const debugActionSrc = `package test {
	action tally {
		attribute total = 0;

		first start;

		action accumulate {
			assign total := total + 5;
		}

		done;

		succession first start then accumulate;
		succession first accumulate then done;
	}
}`

const debugStateSrc = `package test {
	state Cycle {
		entry; then init;
		state init;
		state waiting;
		accept after 10 then working;
		state working;
		accept after 5 then done;

		succession first init then waiting;
	}
}`

// debugActionExecutor builds an initialized executor for the tally action.
func debugActionExecutor(t *testing.T) *ActionExecutor {
	t.Helper()
	ctx, sym := loadAction(t, debugActionSrc, "tally")
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("CreateActionExecutor: %v", err)
	}
	return exec
}

// The accessors a debugger reads between steps report the executor's state.
func TestActionExecutorDebugAccessors(t *testing.T) {
	exec := debugActionExecutor(t)

	if got := exec.ActionSymbol(); got == nil || got.Name != "tally" {
		t.Fatalf("ActionSymbol() = %v, want tally", got)
	}
	if got := exec.State(); got != StateRunning {
		t.Errorf("State() = %v, want %v", got, StateRunning)
	}

	tokens := exec.Tokens()
	if len(tokens) != 1 {
		t.Fatalf("Tokens() = %d tokens, want 1", len(tokens))
	}
	if name := ActionNodeName(tokens[0].Location); name != "start" {
		t.Errorf("token sits at %q, want start", name)
	}

	// Tokens is a copy: mutating it must not disturb the executor.
	tokens[0].ID = -1
	if exec.Tokens()[0].ID == -1 {
		t.Error("Tokens() exposed the executor's own slice")
	}

	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	if got := exec.State(); got != StateCompleted {
		t.Errorf("State() = %v, want %v", got, StateCompleted)
	}
}

func TestActionExecutorNodeNames(t *testing.T) {
	names := strings.Join(debugActionExecutor(t).NodeNames(), ",")
	for _, want := range []string{"start", "accumulate", "done"} {
		if !strings.Contains(names, want) {
			t.Errorf("NodeNames() = %s, want it to contain %q", names, want)
		}
	}
}

// A breakpoint stops a run when a token reaches the node, with the tokens left
// in place so the run can resume.
func TestSetBreakpointStopsRun(t *testing.T) {
	exec := debugActionExecutor(t)
	exec.SetBreakpoint("accumulate")

	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}

	if got := exec.PausedAt(); got != "accumulate" {
		t.Fatalf("PausedAt() = %q, want accumulate", got)
	}
	if got := exec.State(); got != StateSuspended {
		t.Errorf("State() = %v, want %v", got, StateSuspended)
	}
	tokens := exec.Tokens()
	if len(tokens) != 1 || ActionNodeName(tokens[0].Location) != "accumulate" {
		t.Fatalf("expected one token at accumulate, got %v", tokens)
	}

	// Resuming past the breakpoint completes the action.
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if got := exec.State(); got != StateCompleted {
		t.Errorf("State() = %v, want %v", got, StateCompleted)
	}
	if got := exec.PausedAt(); got != "" {
		t.Errorf("PausedAt() = %q after completing, want empty", got)
	}
	if total, ok := exec.Results()["total"]; !ok || total.Const.Int != 5 {
		t.Errorf("results = %v, want total 5", exec.Results())
	}
}

// A step stating a short name and a redefinition answers to its short name
// alone: a declared short name is a name, so the step takes none from what it
// redefines (KerML 7.3.4.5), and a breakpoint on that name never fires.
func TestBreakpointNamesAShortNamedStepByItsShortName(t *testing.T) {
	const src = `package test {
	action def Base { action accumulate; }
	action tally : Base {
		attribute total = 0;
		first start;
		action <acc> :>> accumulate {
			assign total := total + 5;
		}
		done;
		succession first start then acc;
		succession first acc then done;
	}
}`

	for breakpoint, want := range map[string]string{"acc": "acc", "accumulate": ""} {
		ctx, sym := loadAction(t, src, "tally")
		exec, err := ctx.CreateActionExecutor(sym)
		if err != nil {
			t.Fatalf("CreateActionExecutor: %v", err)
		}
		exec.SetBreakpoint(breakpoint)
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("RunToCompletion: %v", err)
		}
		if got := exec.PausedAt(); got != want {
			t.Errorf("breakpoint %s: PausedAt() = %q, want %q", breakpoint, got, want)
		}
	}
}

func TestClearBreakpointsResumesUnconditionally(t *testing.T) {
	exec := debugActionExecutor(t)
	exec.SetBreakpoint("accumulate")
	exec.ClearBreakpoints()

	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	if got := exec.PausedAt(); got != "" {
		t.Errorf("PausedAt() = %q, want empty after clearing breakpoints", got)
	}
	if got := exec.State(); got != StateCompleted {
		t.Errorf("State() = %v, want %v", got, StateCompleted)
	}
}

// A breakpoint on a node no token reaches leaves the run unaffected.
func TestBreakpointOnUnreachedNode(t *testing.T) {
	exec := debugActionExecutor(t)
	exec.SetBreakpoint("nowhere")

	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	if got := exec.State(); got != StateCompleted {
		t.Errorf("State() = %v, want %v", got, StateCompleted)
	}
}

// Stepping resumes a run a breakpoint suspended.
func TestStepResumesFromBreakpoint(t *testing.T) {
	exec := debugActionExecutor(t)
	exec.SetBreakpoint("accumulate")
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}

	if err := exec.Step(); err != nil {
		t.Fatalf("Step: %v", err)
	}
	if exec.State() == StateSuspended {
		t.Error("Step() left the executor suspended")
	}
}

// A step landing a token on a breakpoint suspends the run there, as a run to
// completion would; the step after resumes past it, and a breakpoint set on
// the node a token already sits on stops the next step before it moves.
func TestStepToBreakpointPausesWhereARunWould(t *testing.T) {
	exec := debugActionExecutor(t)
	exec.SetBreakpoint("accumulate")

	if err := exec.StepToBreakpoint(); err != nil {
		t.Fatalf("StepToBreakpoint: %v", err)
	}
	if got := exec.PausedAt(); got != "accumulate" {
		t.Fatalf("PausedAt() after the step onto it = %q, want accumulate", got)
	}
	if got := exec.State(); got != StateSuspended {
		t.Errorf("State() = %v, want %v", got, StateSuspended)
	}
	if total := exec.Results()["total"]; total.Const.Int != 0 {
		t.Errorf("total = %v before accumulate performs, want 0", total)
	}

	if err := exec.StepToBreakpoint(); err != nil {
		t.Fatalf("resuming StepToBreakpoint: %v", err)
	}
	if got := exec.PausedAt(); got != "" {
		t.Errorf("PausedAt() after resuming = %q, want empty", got)
	}
	if total := exec.Results()["total"]; total.Const.Int != 5 {
		t.Errorf("total = %v after accumulate performed, want 5", total)
	}
	if err := exec.StepToBreakpoint(); err != nil {
		t.Fatalf("final StepToBreakpoint: %v", err)
	}
	if got := exec.State(); got != StateCompleted {
		t.Errorf("State() = %v, want %v", got, StateCompleted)
	}

	// Set on the node the token sits on, the breakpoint holds the next step.
	exec = debugActionExecutor(t)
	exec.SetBreakpoint("start")
	if err := exec.StepToBreakpoint(); err != nil {
		t.Fatalf("StepToBreakpoint at start: %v", err)
	}
	if got := exec.PausedAt(); got != "start" {
		t.Errorf("PausedAt() = %q, want start", got)
	}
	if tokens := exec.Tokens(); len(tokens) != 1 || ActionNodeName(tokens[0].Location) != "start" {
		t.Errorf("tokens = %v, want the one still at start", tokens)
	}
	if err := exec.StepToBreakpoint(); err != nil {
		t.Fatalf("resuming from start: %v", err)
	}
	if tokens := exec.Tokens(); len(tokens) != 1 || ActionNodeName(tokens[0].Location) != "accumulate" {
		t.Errorf("tokens = %v, want the one moved on to accumulate", tokens)
	}
}

// actionNodeNamed is the node of the executor's own flow with the given name.
func actionNodeNamed(t *testing.T, exec *ActionExecutor, name string) ast.Node {
	t.Helper()
	for _, node := range exec.Graph().Nodes {
		if ActionNodeName(node) == name {
			return node
		}
	}
	t.Fatalf("no node named %s among %v", name, exec.NodeNames())
	return nil
}

// Replacing the breakpoints set by identity keeps a stop already made at one kept,
// so the next step resumes past it; one removed stops the run again once re-set.
func TestReplaceBreakpointsAtKeepsAStopAlreadyMade(t *testing.T) {
	exec := debugActionExecutor(t)
	at := []NodeBreakpoint{{Node: actionNodeNamed(t, exec, "accumulate")}}
	exec.ReplaceBreakpointsAt(at)
	if err := exec.StepToBreakpoint(); err != nil {
		t.Fatalf("StepToBreakpoint: %v", err)
	}
	if got := exec.PausedAt(); got != "accumulate" {
		t.Fatalf("PausedAt() = %q, want accumulate", got)
	}

	exec.ReplaceBreakpointsAt(at)
	if err := exec.StepToBreakpoint(); err != nil {
		t.Fatalf("StepToBreakpoint after setting the same breakpoints again: %v", err)
	}
	if got := exec.PausedAt(); got != "" {
		t.Errorf("PausedAt() = %q after setting the same breakpoints again, want the run resumed", got)
	}
	if total := exec.Results()["total"]; total.Const.Int != 5 {
		t.Errorf("total = %v after the resumed step, want accumulate performed (5)", total)
	}

	exec = debugActionExecutor(t)
	at = []NodeBreakpoint{{Node: actionNodeNamed(t, exec, "accumulate")}}
	exec.ReplaceBreakpointsAt(at)
	if err := exec.StepToBreakpoint(); err != nil {
		t.Fatalf("StepToBreakpoint: %v", err)
	}
	exec.ReplaceBreakpointsAt(nil)
	exec.ReplaceBreakpointsAt(at)
	if err := exec.StepToBreakpoint(); err != nil {
		t.Fatalf("StepToBreakpoint after re-setting the breakpoint: %v", err)
	}
	if got := exec.PausedAt(); got != "accumulate" {
		t.Errorf("PausedAt() = %q after re-setting the breakpoint, want accumulate stopped at again", got)
	}
	if total := exec.Results()["total"]; total.Const.Int != 0 {
		t.Errorf("total = %v while stopped again, want 0", total)
	}
}

// A stop made at a node a body performs follows the same rule: kept, the resumed
// body passes it; removed and re-set while it stands, the body stops there again.
func TestReplaceBreakpointsAtResetsABodyStopRemoved(t *testing.T) {
	stopAtQ := func(t *testing.T) (*ActionExecutor, []NodeBreakpoint) {
		t.Helper()
		exec := blockDebugExecutor(t)
		choose := actionNodeNamed(t, exec, "choose")
		var q ast.Node
		for _, node := range exec.Graph().BlockNodes[choose] {
			if ActionNodeName(node) == "q" {
				q = node
			}
		}
		if q == nil {
			t.Fatalf("choose's blocks declare no q: %v", exec.Graph().BlockNodes[choose])
		}
		at := []NodeBreakpoint{{Within: []ast.Node{choose}, Node: q}}
		exec.ReplaceBreakpointsAt(at)
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("RunToCompletion: %v", err)
		}
		if got := exec.PausedAt(); got != "q" {
			t.Fatalf("PausedAt() = %q, want q", got)
		}
		return exec, at
	}

	exec, at := stopAtQ(t)
	exec.ReplaceBreakpointsAt(at)
	if err := exec.StepToBreakpoint(); err != nil {
		t.Fatalf("StepToBreakpoint after setting the same breakpoints again: %v", err)
	}
	if got := exec.PausedAt(); got != "" {
		t.Errorf("PausedAt() = %q after setting the same breakpoints again, want the body resumed past q", got)
	}
	if _, ok := exec.Results()["choose.q.n"]; !ok {
		t.Errorf("results = %v, want q performed by the resumed step", exec.Results())
	}

	exec, at = stopAtQ(t)
	exec.ReplaceBreakpointsAt(nil)
	exec.ReplaceBreakpointsAt(at)
	if err := exec.StepToBreakpoint(); err != nil {
		t.Fatalf("StepToBreakpoint after re-setting the breakpoint: %v", err)
	}
	if got := exec.PausedAt(); got != "q" {
		t.Errorf("PausedAt() = %q after re-setting the breakpoint, want q stopped at again", got)
	}
	if _, ok := exec.Results()["choose.q.n"]; ok {
		t.Errorf("results = %v while stopped again, q must not have performed", exec.Results())
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if total := exec.Results()["total"]; total.Const.Int != 13 {
		t.Errorf("total = %v after resuming, want 13", total)
	}

	exec, _ = stopAtQ(t)
	exec.ClearBreakpoints()
	exec.SetBreakpoint("q")
	if err := exec.StepToBreakpoint(); err != nil {
		t.Fatalf("StepToBreakpoint after clearing and naming the breakpoint again: %v", err)
	}
	if got := exec.PausedAt(); got != "q" {
		t.Errorf("PausedAt() = %q after clearing and naming the breakpoint again, want q stopped at again", got)
	}
}

// Resume returns a run a breakpoint suspended to the clock, which then runs it past
// the breakpoint; a run in any other state is left alone.
func TestResumeReturnsAPausedRunToTheClock(t *testing.T) {
	ctx, sym := loadAction(t, debugActionSrc, "tally")
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("CreateActionExecutor: %v", err)
	}
	if exec.Resume() {
		t.Error("Resume() = true for a run no breakpoint suspended")
	}
	exec.ReplaceBreakpointsAt([]NodeBreakpoint{{Node: actionNodeNamed(t, exec, "accumulate")}})
	if err := exec.StepToBreakpoint(); err != nil {
		t.Fatalf("StepToBreakpoint: %v", err)
	}
	if _, err := ctx.Advance(0); err != nil {
		t.Fatalf("Advance while suspended: %v", err)
	}
	if got := exec.State(); got != StateSuspended {
		t.Fatalf("State() = %v after an advance while suspended, want the run still %v", got, StateSuspended)
	}

	if !exec.Resume() {
		t.Fatal("Resume() = false for a run a breakpoint suspended")
	}
	if got, paused := exec.State(), exec.PausedAt(); got != StateRunning || paused != "" {
		t.Fatalf("State(), PausedAt() = %v, %q after Resume, want %v and none", got, paused, StateRunning)
	}
	if _, err := ctx.Advance(0); err != nil {
		t.Fatalf("Advance after Resume: %v", err)
	}
	if got := exec.State(); got != StateCompleted {
		t.Errorf("State() = %v after the advance, want %v", got, StateCompleted)
	}
	if total := exec.Results()["total"]; total.Const.Int != 5 {
		t.Errorf("total = %v after the advance, want 5", total)
	}
}

// reusedFlowSrc runs one inherited action declaration in two nested flows.
const reusedFlowSrc = `package test {
	action def Check {
		action look;
	}
	action twice {
		first start;
		action a : Check { first begin; then look; }
		action b : Check { first begin; then look; }
		succession first start then a;
		succession first a then b;
	}
}`

// A breakpoint set by identity is on one occurrence of a node: the one in the flow
// of the nested node named, not the same declaration another nested flow runs.
func TestBreakpointsAtDistinguishReusedNestedFlows(t *testing.T) {
	ctx, sym := loadAction(t, reusedFlowSrc, "twice")
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("CreateActionExecutor: %v", err)
	}
	a, b := actionNodeNamed(t, exec, "a"), actionNodeNamed(t, exec, "b")
	sub := exec.Graph().Subflows[a]
	if sub == nil || sub.Graph == nil || exec.Graph().Subflows[b] == nil || exec.Graph().Subflows[b].Graph == nil {
		t.Fatalf("a and b own no flows: %v", exec.Graph().Subflows)
	}
	var look ast.Node
	for _, node := range sub.Graph.Nodes {
		if ActionNodeName(node) == "look" {
			look = node
		}
	}
	if look == nil || !slices.Contains(exec.Graph().Subflows[b].Graph.Nodes, look) {
		t.Fatalf("look is not one node both flows run: %v", look)
	}

	exec.ReplaceBreakpointsAt([]NodeBreakpoint{{Within: []ast.Node{b}, Node: look}})
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	if got := exec.PausedAt(); got != "look" {
		t.Fatalf("PausedAt() = %q, want look", got)
	}
	tokens := exec.Tokens()
	if len(tokens) != 1 || tokens[0].Location != look || !slices.Equal(tokens[0].Within(), []ast.Node{b}) {
		t.Fatalf("tokens = %v, want the one at look within b", tokens)
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("resuming RunToCompletion: %v", err)
	}
	if got := exec.State(); got != StateCompleted {
		t.Errorf("State() = %v after resuming, want %v", got, StateCompleted)
	}
}

// blockDebugSrc declares action nodes inside an `if` branch and a loop body.
const blockDebugSrc = `package test {
	private import ScalarValues::*;
	action outer {
		attribute total : Integer = 0;
		first start;
		then action choose {
			if total == 0 {
				action p { out v : Integer = 7; }
				action q { in n : Integer = p.v; assign total := total + n; }
			}
		}
		then action iterate {
			for i in 1..3 {
				action add { in n : Integer = i; assign total := total + n; }
			}
		}
		then done;
	}
}`

// blockDebugExecutor builds an initialized executor for the outer action.
func blockDebugExecutor(t *testing.T) *ActionExecutor {
	t.Helper()
	ctx, sym := loadAction(t, blockDebugSrc, "outer")
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("CreateActionExecutor: %v", err)
	}
	return exec
}

// A node a block declares is one a breakpoint can name.
func TestNodeNamesIncludeBlockFlowNodes(t *testing.T) {
	names := strings.Join(blockDebugExecutor(t).NodeNames(), ",")
	for _, want := range []string{"choose", "p", "q", "iterate", "add"} {
		if !strings.Contains(","+names+",", ","+want+",") {
			t.Errorf("NodeNames() = %s, want it to contain %q", names, want)
		}
	}
}

// A breakpoint on a node an `if` branch declares pauses the run before that node
// performs, with the branch's token left at the node running the block.
func TestBreakpointPausesBeforeABranchNode(t *testing.T) {
	exec := blockDebugExecutor(t)
	exec.SetBreakpoint("q")

	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	if got := exec.PausedAt(); got != "q" {
		t.Fatalf("PausedAt() = %q, want q", got)
	}
	if got := exec.State(); got != StateSuspended {
		t.Errorf("State() = %v, want %v", got, StateSuspended)
	}
	if tokens := exec.Tokens(); len(tokens) != 1 || ActionNodeName(tokens[0].Location) != "choose" {
		t.Fatalf("expected one token at choose, got %v", tokens)
	}
	results := exec.Results()
	if v, ok := results["choose.p.v"]; !ok || v.Const.Int != 7 {
		t.Errorf("results = %v, want choose.p.v 7 (p performed before the pause)", results)
	}
	if _, ok := results["choose.q.n"]; ok {
		t.Errorf("results = %v, q must not have performed yet", results)
	}

	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if got := exec.State(); got != StateCompleted {
		t.Errorf("State() = %v, want %v", got, StateCompleted)
	}
	if total := exec.Results()["total"]; total.Const.Int != 13 {
		t.Errorf("total = %v, want 13", total)
	}
}

// The breakpoint paused at is identified by the node and its flow, not by where the
// tokens are: a branch node pauses the run while its token stays on the enclosing node.
func TestPausedBreakpointIdentifiesABranchNodeInItsFlow(t *testing.T) {
	exec := blockDebugExecutor(t)
	if _, ok := exec.PausedBreakpoint(); ok {
		t.Fatal("PausedBreakpoint() reports a stop before any run")
	}
	choose := actionNodeNamed(t, exec, "choose")
	var q ast.Node
	for _, node := range exec.Graph().BlockNodes[choose] {
		if ActionNodeName(node) == "q" {
			q = node
		}
	}
	if q == nil {
		t.Fatalf("choose's blocks declare no q: %v", exec.Graph().BlockNodes[choose])
	}
	exec.ReplaceBreakpointsAt([]NodeBreakpoint{{Within: []ast.Node{choose}, Node: q}})

	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	if got := exec.PausedAt(); got != "q" {
		t.Fatalf("PausedAt() = %q, want q", got)
	}
	bp, ok := exec.PausedBreakpoint()
	if !ok || bp.Node != q || !slices.Equal(bp.Within, []ast.Node{choose}) {
		t.Errorf("PausedBreakpoint() = %+v, %v, want q within choose", bp, ok)
	}
	if tokens := exec.Tokens(); len(tokens) != 1 || tokens[0].Location != choose {
		t.Errorf("tokens = %v, want the one still on choose", tokens)
	}
	// The path reported is the caller's own: writing to it leaves the stop where it was.
	bp.Within[0] = q
	if again, ok := exec.PausedBreakpoint(); !ok || !slices.Equal(again.Within, []ast.Node{choose}) {
		t.Errorf("PausedBreakpoint() = %+v, %v after writing to the path reported, want q still within choose", again, ok)
	}

	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if bp, ok := exec.PausedBreakpoint(); ok {
		t.Errorf("PausedBreakpoint() = %+v after resuming, want none", bp)
	}
}

// A breakpoint on a node a loop body declares pauses the run once per iteration,
// resuming once per pause, as a breakpoint on a node of the action's own flow does.
func TestBreakpointPausesOnEachLoopIteration(t *testing.T) {
	exec := blockDebugExecutor(t)
	exec.SetBreakpoint("add")

	for iteration, wantTotal := range []int64{7, 8, 10} {
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("iteration %d: RunToCompletion: %v", iteration, err)
		}
		if got := exec.PausedAt(); got != "add" {
			t.Fatalf("iteration %d: PausedAt() = %q, want add", iteration, got)
		}
		if total := exec.Results()["total"]; total.Const.Int != wantTotal {
			t.Errorf("iteration %d: total = %v, want %d", iteration, total, wantTotal)
		}
	}

	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("final run: %v", err)
	}
	if got := exec.PausedAt(); got != "" {
		t.Errorf("PausedAt() = %q after completing, want empty", got)
	}
	if total := exec.Results()["total"]; total.Const.Int != 13 {
		t.Errorf("total = %v, want 13", total)
	}
}

// TestStepSpendsTheActionBudgetOnABlockNodesOwnFlow: one Step performs the flow a
// block-declared node states of its own to its end, so a cycle in it spends the
// action's token-flow budget within that step.
func TestStepSpendsTheActionBudgetOnABlockNodesOwnFlow(t *testing.T) {
	ctx, sym := loadAction(t, `package test {
		private import ScalarValues::*;
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
	}`, "outer")
	ctx.maxActionSteps = 50
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("CreateActionExecutor: %v", err)
	}
	for i := 0; i < 10; i++ {
		err = exec.Step()
		if err != nil {
			break
		}
	}
	if !errors.Is(err, ErrActionStepLimitExceeded) {
		t.Fatalf("Step() = %v, want ErrActionStepLimitExceeded", err)
	}
}

// A breakpoint on a step of the flow a block node owns pauses the run when a
// token of that flow reaches it, with the tokens of both flows in view.
func TestBreakpointPausesInsideABlockNodesOwnFlow(t *testing.T) {
	ctx, sym := loadAction(t, `package test {
		private import ScalarValues::*;
		action outer {
			out attribute total : Integer = 0;
			first start;
			then action choose {
				if total == 0 {
					action split {
						out sum : Integer;
						first start;
						then action left { out a : Integer; assign a := 10; }
						then action gather { assign sum := left.a + 1; }
						then done;
					}
					action report { assign total := split.sum; }
				}
			}
			then done;
		}
	}`, "outer")
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("CreateActionExecutor: %v", err)
	}
	exec.SetBreakpoint("gather")

	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	if got := exec.PausedAt(); got != "gather" {
		t.Fatalf("PausedAt() = %q, want gather", got)
	}
	var at []string
	for _, token := range exec.Tokens() {
		at = append(at, ActionNodeName(token.Location))
	}
	if got := strings.Join(at, ","); got != "choose,gather" {
		t.Errorf("tokens at %s, want choose,gather", got)
	}
	if a, ok := exec.Results()["choose.split.left.a"]; !ok || a.Const.Int != 10 {
		t.Errorf("results = %v, want choose.split.left.a 10", exec.Results())
	}

	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if total := exec.Results()["total"]; total.Const.Int != 11 {
		t.Errorf("total = %v, want 11", total)
	}
}

// Stepping resumes the paused block node and pauses again at the next one.
func TestStepResumesAPausedBlockNode(t *testing.T) {
	exec := blockDebugExecutor(t)
	exec.SetBreakpoint("add")
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}

	if err := exec.Step(); err != nil {
		t.Fatalf("Step: %v", err)
	}
	if got := exec.PausedAt(); got != "add" {
		t.Errorf("PausedAt() = %q after a step, want the next iteration's add", got)
	}
	if total := exec.Results()["total"]; total.Const.Int != 8 {
		t.Errorf("total = %v after a step, want 8", total)
	}

	exec.ClearBreakpoints()
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if got := exec.State(); got != StateCompleted {
		t.Errorf("State() = %v, want %v", got, StateCompleted)
	}
}

// forkedDebugSrc forks into two nodes whose loop bodies each declare a node, so
// two tokens meet body breakpoints of their own.
const forkedDebugSrc = `package test {
	private import ScalarValues::*;
	action outer {
		out attribute left : Integer = 0;
		out attribute right : Integer = 0;
		fork split;
		action l {
			assign left := 100;
			for i in 1..2 { action addL { in n : Integer = i; assign left := left + n; } }
		}
		action r {
			assign right := 100;
			for i in 1..2 { action addR { in n : Integer = i; assign right := right + n; } }
		}
		join sync;
		succession first start then split;
		succession first split then l;
		succession first split then r;
		succession first l then sync;
		succession first r then sync;
		succession first sync then done;
	}
}`

// stepUntilPaused steps exec until a breakpoint suspends it, returning the
// number of steps taken.
func stepUntilPaused(t *testing.T, exec *ActionExecutor) int {
	t.Helper()
	for steps := 1; steps <= 20; steps++ {
		if err := exec.Step(); err != nil {
			t.Fatalf("Step %d: %v", steps, err)
		}
		if exec.State() == StateSuspended {
			return steps
		}
	}
	t.Fatal("no breakpoint paused the run")
	return 0
}

// A body breakpoint one token meets ends the step: its sibling out of the fork
// stays put, its body not begun, and resumes on its own coroutine next step.
func TestBodyBreakpointFreezesSiblingTokens(t *testing.T) {
	ctx, sym := loadAction(t, forkedDebugSrc, "outer")
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("CreateActionExecutor: %v", err)
	}
	exec.SetBreakpoint("addL")
	exec.SetBreakpoint("addR")

	stepUntilPaused(t, exec)
	first := exec.PausedAt()
	other := map[string]string{"addL": "right", "addR": "left"}[first]
	if other == "" {
		t.Fatalf("PausedAt() = %q, want addL or addR", first)
	}
	if v := exec.Results()[other]; v.Const.Int != 0 {
		t.Errorf("%s = %v while paused at %s, want 0: the sibling's body ran", other, v, first)
	}
	if n := len(exec.Tokens()); n != 2 {
		t.Errorf("%d tokens while paused, want the fork's 2", n)
	}

	// The sibling steps next and pauses in a body of its own; the two then take
	// turns, each pausing once per iteration.
	pauses := []string{first}
	for len(pauses) < 4 {
		stepUntilPaused(t, exec)
		pauses = append(pauses, exec.PausedAt())
	}
	if pauses[0] == pauses[1] || pauses[1] == pauses[2] || pauses[2] == pauses[3] {
		t.Errorf("pauses %v, want the two bodies taking turns", pauses)
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("final run: %v", err)
	}
	if got := exec.PausedAt(); got != "" {
		t.Errorf("PausedAt() = %q after four pauses, want a run to the end", got)
	}
	results := exec.Results()
	if results["left"].Const.Int != 103 || results["right"].Const.Int != 103 {
		t.Errorf("left, right = %v, %v, want 103, 103", results["left"], results["right"])
	}
}

// convergingDebugSrc forks into two nodes whose successions both lead to the node
// declared by decl, named node: it is performed once, after both have arrived.
func convergingDebugSrc(node, decl string) string {
	return `package test {
	private import ScalarValues::*;
	action outer {
		out attribute hits : Integer = 0;
		fork split;
		action l { assign hits := hits + 1; }
		action r { assign hits := hits + 1; }
		` + decl + `
		succession first start then split;
		succession first split then l;
		succession first split then r;
		succession first l then ` + node + `;
		succession first r then ` + node + `;
		succession first ` + node + ` then done;
	}
}`
}

// A breakpoint on a node two fork branches converge on — a join or a plain node —
// pauses once, before its one performance, with both arrivals in; not once per token.
func TestBreakpointPausesOncePerSynchronizedPerformance(t *testing.T) {
	cases := []struct {
		node, decl string
		hits       int64
	}{
		{"sync", "join sync;", 2},
		{"scale", "action scale { assign hits := hits * 10; }", 20},
	}
	for _, tc := range cases {
		t.Run(tc.node, func(t *testing.T) {
			ctx, sym := loadAction(t, convergingDebugSrc(tc.node, tc.decl), "outer")
			exec, err := ctx.CreateActionExecutor(sym)
			if err != nil {
				t.Fatalf("CreateActionExecutor: %v", err)
			}
			exec.SetBreakpoint(tc.node)

			if err := exec.RunToCompletion(); err != nil {
				t.Fatalf("RunToCompletion: %v", err)
			}
			if got := exec.PausedAt(); got != tc.node {
				t.Fatalf("PausedAt() = %q, want %s", got, tc.node)
			}
			if got := len(exec.Tokens()); got != 2 {
				t.Fatalf("%d tokens while paused, want the 2 arrivals at %s", got, tc.node)
			}
			for _, tok := range exec.Tokens() {
				if ActionNodeName(tok.Location) != tc.node {
					t.Errorf("token %d @ %s while paused, want %s", tok.ID, ActionNodeName(tok.Location), tc.node)
				}
				if awaiting := exec.Awaiting(tok); len(awaiting) > 0 {
					t.Errorf("paused at %s while token %d still awaits %d successions", tc.node, tok.ID, len(awaiting))
				}
			}
			if hits := exec.Results()["hits"]; hits.Const.Int != 2 {
				t.Errorf("hits = %v while paused, want 2: both branches ran, %s did not", hits, tc.node)
			}

			if err := exec.RunToCompletion(); err != nil {
				t.Fatalf("resume: %v", err)
			}
			if got := exec.PausedAt(); got != "" {
				t.Fatalf("PausedAt() = %q after resuming, want a run to the end", got)
			}
			if got := exec.State(); got != StateCompleted {
				t.Errorf("State() = %v, want %v", got, StateCompleted)
			}
			if hits := exec.Results()["hits"]; hits.Const.Int != tc.hits {
				t.Errorf("hits = %v, want %d", hits, tc.hits)
			}
		})
	}
}

// A token entering a nested flow in the step a synchronization drops tokens below it
// takes no second step: the breakpoint on the nested flow's first node still pauses.
func TestBreakpointOnANestedFirstNodePausesWhileOthersSynchronize(t *testing.T) {
	ctx, sym := loadAction(t, `package test {
	private import ScalarValues::*;
	action outer {
		out attribute hits : Integer = 0;
		out attribute n : Integer = 0;
		fork split;
		action l { assign hits := hits + 1; }
		action r { assign hits := hits + 1; }
		join sync;
		action tally { assign hits := hits * 10; }
		action pre { assign n := n + 1; }
		action nested {
			action inner { assign n := n * 10; }
			first inner;
		}
		succession first start then split;
		succession first split then l;
		succession first split then r;
		succession first split then pre;
		succession first l then sync;
		succession first r then sync;
		succession first sync then tally;
		succession first pre then nested;
	}
}`, "outer")
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("CreateActionExecutor: %v", err)
	}
	exec.SetBreakpoint("inner")

	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	if got := exec.PausedAt(); got != "inner" {
		t.Fatalf("PausedAt() = %q, want inner", got)
	}
	if n := exec.Results()["n"]; n.Const.Int != 1 {
		t.Errorf("n = %v while paused, want 1: pre ran, inner did not", n)
	}
	var atInner int
	for _, tok := range exec.Tokens() {
		if ActionNodeName(tok.Location) == "inner" {
			atInner++
		}
	}
	if atInner != 1 {
		t.Errorf("%d tokens at inner while paused, want 1", atInner)
	}

	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if got := exec.State(); got != StateCompleted {
		t.Errorf("State() = %v, want %v", got, StateCompleted)
	}
	if n := exec.Results()["n"]; n.Const.Int != 10 {
		t.Errorf("n = %v, want 10", n)
	}
	if hits := exec.Results()["hits"]; hits.Const.Int != 20 {
		t.Errorf("hits = %v, want 20", hits)
	}
}

// A breakpoint on a node reached from the flow's start and back around a loop pauses
// before each pass: once per performance, for as many performances as the loop makes.
func TestBreakpointOnALoopedNodePausesEachPass(t *testing.T) {
	ctx, sym := loadAction(t, `package test {
	private import ScalarValues::*;
	action count {
		out attribute n : Integer = 0;
		action bump { assign n := n + 1; }
		decide again;
		succession first start then bump;
		succession first bump then again;
		succession first again if n < 3 then bump;
		succession first again if n >= 3 then done;
	}
}`, "count")
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("CreateActionExecutor: %v", err)
	}
	exec.SetBreakpoint("bump")

	for pass := int64(0); pass < 3; pass++ {
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("pass %d: RunToCompletion: %v", pass, err)
		}
		if got := exec.PausedAt(); got != "bump" {
			t.Fatalf("pass %d: PausedAt() = %q, want bump", pass, got)
		}
		if n := exec.Results()["n"]; n.Const.Int != pass {
			t.Errorf("pass %d: n = %v while paused, want %d", pass, n, pass)
		}
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("final run: %v", err)
	}
	if got := exec.State(); got != StateCompleted {
		t.Errorf("State() = %v, want %v", got, StateCompleted)
	}
	if n := exec.Results()["n"]; n.Const.Int != 3 {
		t.Errorf("n = %v, want 3", n)
	}
}

// Releasing an executor paused in a body ends the paused work, so nothing of the
// run stays suspended; a later step is refused, and releasing again is harmless.
func TestReleaseEndsAPausedBody(t *testing.T) {
	exec := blockDebugExecutor(t)
	exec.SetBreakpoint("add")
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	var run *bodyRun
	for _, token := range exec.tokens {
		if token.body != nil {
			run = token.body
		}
	}
	if run == nil {
		t.Fatal("no token holds paused work while paused at add")
	}

	exec.Release()
	if !run.ended {
		t.Error("the paused work is still paused after Release")
	}
	if !errors.Is(run.err, ErrActionDeadlock) {
		t.Errorf("the ended work reports %v, want ErrActionDeadlock (abandoned)", run.err)
	}
	for _, token := range exec.tokens {
		if token.body != nil {
			t.Errorf("token %d still holds paused work after Release", token.ID)
		}
	}
	exec.Release()
	if err := exec.Step(); !errors.Is(err, ErrExecutorReleased) {
		t.Errorf("Step after Release = %v, want ErrExecutorReleased", err)
	}
}

// A run that fails its token-flow budget while a body is paused ends the paused
// work as a failing step does, so nothing of the run stays suspended.
func TestBudgetFailureEndsAPausedBody(t *testing.T) {
	exec := blockDebugExecutor(t)
	exec.SetBreakpoint("add")
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	var run *bodyRun
	for _, token := range exec.tokens {
		if token.body != nil {
			run = token.body
		}
	}
	if run == nil {
		t.Fatal("no token holds paused work while paused at add")
	}

	exec.ctx.maxActionSteps = 0
	if err := exec.RunToCompletion(); !errors.Is(err, ErrActionStepLimitExceeded) {
		t.Fatalf("resume = %v, want ErrActionStepLimitExceeded", err)
	}
	if !run.ended {
		t.Error("the paused work is still paused after the budget failure")
	}
	for _, token := range exec.tokens {
		if token.body != nil {
			t.Errorf("token %d still holds paused work after the budget failure", token.ID)
		}
	}
}

// A released executor refuses to step whatever state its run ended in.
func TestReleasedExecutorRefusesToStep(t *testing.T) {
	exec := blockDebugExecutor(t)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	if exec.State() != StateCompleted {
		t.Fatalf("State() = %v after the run, want StateCompleted", exec.State())
	}
	exec.Release()
	if err := exec.Step(); !errors.Is(err, ErrExecutorReleased) {
		t.Errorf("Step of a released, completed executor = %v, want ErrExecutorReleased", err)
	}
	if err := exec.RunToCompletion(); !errors.Is(err, ErrExecutorReleased) {
		t.Errorf("RunToCompletion of a released, completed executor = %v, want ErrExecutorReleased", err)
	}
}

// A step that fails ends the work another token had paused.
func TestFailedStepEndsPausedBodies(t *testing.T) {
	ctx, sym := loadAction(t, `package test {
		private import ScalarValues::*;
		action outer {
			out attribute total : Integer = 0;
			fork split;
			action l { for i in 1..2 { action add { assign total := total + i; } } }
			action wait {}
			action r { assign total := total / 0; }
			join sync;
			succession first start then split;
			succession first split then l;
			succession first split then wait;
			succession first wait then r;
			succession first l then sync;
			succession first r then sync;
			succession first sync then done;
		}
	}`, "outer")
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("CreateActionExecutor: %v", err)
	}
	exec.SetBreakpoint("add")

	var run *bodyRun
	for steps := 0; run == nil && steps < 20; steps++ {
		if err := exec.Step(); err != nil {
			t.Fatalf("Step: %v", err)
		}
		for _, token := range exec.tokens {
			if token.body != nil {
				run = token.body
			}
		}
	}
	if run == nil {
		t.Fatal("no token paused in its body")
	}
	if err := exec.RunToCompletion(); !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("RunToCompletion = %v, want ErrDivisionByZero from the sibling", err)
	}
	if !run.ended {
		t.Error("the paused work is still paused after the run failed")
	}
}

func TestStateExecutorDebugAccessors(t *testing.T) {
	ctx, sym := loadState(t, debugStateSrc, "Cycle")
	exec, err := ctx.CreateStateExecutor(sym)
	if err != nil {
		t.Fatalf("CreateStateExecutor: %v", err)
	}

	if got := exec.StateMachineSymbol(); got == nil || got.Name != "Cycle" {
		t.Fatalf("StateMachineSymbol() = %v, want Cycle", got)
	}
	if got := exec.State(); got != StateRunning {
		t.Errorf("State() = %v, want %v", got, StateRunning)
	}
	if got := exec.CurrentTime(); got != 0 {
		t.Errorf("CurrentTime() = %v, want 0", got)
	}
	if exec.EventQueue().Len() == 0 {
		t.Error("EventQueue() is empty, want the initial completion event")
	}
	if got := activeStateNames(exec); got != "init" {
		t.Errorf("ActiveStates() = %s, want init", got)
	}
	if exec.StateData() == nil {
		t.Error("StateData() = nil")
	}

	// Drain the queue: time follows the events' timestamps.
	for exec.HasPendingWork() && exec.State() == StateRunning {
		if err := exec.ProcessNextEvent(); err != nil {
			t.Fatalf("ProcessNextEvent: %v", err)
		}
	}

	if got := exec.CurrentTime(); got != 15 {
		t.Errorf("CurrentTime() = %v, want 15", got)
	}
	if got := activeStateNames(exec); got != "done" {
		t.Errorf("ActiveStates() = %s, want done", got)
	}
	if len(exec.StateStack()) == 0 {
		t.Error("StateStack() is empty")
	}
	if got := exec.State(); got != StateCompleted {
		t.Errorf("State() = %v, want %v", got, StateCompleted)
	}
}

// RunDoRound advances a state's do behavior without dispatching an event, so a
// debugger can run work that is due now while leaving a future event queued.
func TestRunDoRoundRunsDoWorkOnly(t *testing.T) {
	exec := stateExecutorForSource(t, "Slow", `package test {
		state Slow {
			attribute count = 0;
			entry; then init;
			state init;
			state working {
				do { assign count := count + 1; }
			}
			accept after 100 then done;
			succession first init then working;
		}
	}`)

	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent: %v", err)
	}
	if !exec.HasPendingDoWork() {
		t.Fatal("HasPendingDoWork() = false, want the do behavior of working pending")
	}

	queued := exec.EventQueue().Len()
	ran, err := exec.RunDoRound()
	if err != nil {
		t.Fatalf("RunDoRound: %v", err)
	}
	if ran != 1 {
		t.Errorf("RunDoRound() ran %d actions, want 1", ran)
	}
	if got := exec.EventQueue().Len(); got != queued {
		t.Errorf("event queue length = %d, want %d (no event dispatched)", got, queued)
	}
	if got := exec.CurrentTime(); got != 0 {
		t.Errorf("CurrentTime() = %v, want 0 (the future event is untouched)", got)
	}
	if exec.HasPendingDoWork() {
		t.Error("HasPendingDoWork() = true after the behavior's only action ran")
	}
}

// ActiveStates reports every region's state for an orthogonal machine, where
// CurrentState has no single answer to give.
func TestActiveStatesCoversOrthogonalRegions(t *testing.T) {
	exec := stateExecutorForSource(t, "TrafficLight", `package test {
		state def TrafficLight parallel {
			state pedestrian {
				entry; then start;
				state start;
				state Walk;
				succession first start then Walk;
			}
			state vehicle {
				entry; then begin;
				state begin;
				state Green;
				succession first begin then Green;
			}
		}
	}`)

	if exec.CurrentState() != nil {
		t.Error("CurrentState() should have no single answer for an orthogonal machine")
	}
	if got := len(exec.ActiveStates()); got != 2 {
		t.Fatalf("ActiveStates() returned %d states, want one per region", got)
	}
}

// activeStateNames joins the machine's active configuration for comparison.
func activeStateNames(exec *StateExecutor) string {
	names := make([]string, 0, 2)
	for _, state := range exec.ActiveStates() {
		names = append(names, state.Name)
	}
	return strings.Join(names, "|")
}

// firedNames spells the transitions an executor logged as source->target, an
// entry transition as ->target, so a test compares the log against the model.
func firedNames(exec *StateExecutor) []string {
	out := make([]string, 0, exec.FiredCount())
	for _, fired := range exec.FiredTransitions() {
		source := ""
		if fired.Source != nil {
			source = StateVertexName(fired.Source)
		}
		out = append(out, source+"->"+StateVertexName(fired.Target))
	}
	return out
}

// The fired-transition log names the entry transition, then every transition
// taken in order; a mark read before a step delimits what that step fired.
func TestFiredTransitionsLogsEachTransitionInOrder(t *testing.T) {
	ctx, sym := loadState(t, debugStateSrc, "Cycle")
	exec, err := ctx.CreateStateExecutor(sym)
	if err != nil {
		t.Fatalf("CreateStateExecutor: %v", err)
	}
	if got := firedNames(exec); !slices.Equal(got, []string{"->init"}) {
		t.Fatalf("after start fired = %v, want the entry transition only", got)
	}
	mark := exec.FiredCount()
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent: %v", err)
	}
	if got := firedNames(exec)[mark:]; !slices.Equal(got, []string{"init->waiting"}) {
		t.Fatalf("first step fired = %v, want init->waiting", got)
	}
	if got := exec.FiredSince(mark); !slices.Equal(got, exec.FiredTransitions()[mark:]) {
		t.Errorf("FiredSince(%d) = %v, want the firings past the mark", mark, got)
	}
	if got := exec.FiredSince(exec.FiredCount()); got != nil {
		t.Errorf("FiredSince(FiredCount()) = %v, want nil", got)
	}
	if got := exec.FiredSince(exec.FiredCount() + 3); got != nil {
		t.Errorf("FiredSince past the count = %v, want nil", got)
	}
	for exec.HasPendingWork() && exec.State() == StateRunning {
		if err := exec.ProcessNextEvent(); err != nil {
			t.Fatalf("ProcessNextEvent: %v", err)
		}
	}
	want := []string{"->init", "init->waiting", "waiting->working", "working->done"}
	if got := firedNames(exec); !slices.Equal(got, want) {
		t.Fatalf("fired = %v, want %v", got, want)
	}
}

// A compound transition through a junction logs each segment; a fork logs the
// transition into it and then its branches; a join logs every branch into it in
// the order they fire.
func TestFiredTransitionsLogsCompoundAndForkSegments(t *testing.T) {
	src := `package test {
		state Machine {
			attribute priority : Integer = 2;
			entry; then init;
			state init;
			junction route;
			state low;
			state high;
			state working parallel {
				state left {
					entry; then leftStart;
					state leftStart;
					state building;
					succession first leftStart then building;
				}
				state right {
					entry; then rightStart;
					state rightStart;
					state checking;
					succession first rightStart then checking;
				}
			}
			fork split;
			join sync;
			transition first init then route;
			transition first route if priority > 5 then high;
			transition first route then low;
			transition first low then split;
			transition first high then split;
			transition first split then building;
			transition first split then checking;
			transition first building then sync;
			transition first checking then sync;
			transition first sync then done;
		}
	}`
	ctx, sym := loadState(t, src, "Machine")
	exec, err := ctx.CreateStateExecutor(sym)
	if err != nil {
		t.Fatalf("CreateStateExecutor: %v", err)
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	want := []string{
		"->init", "init->route", "route->low", "low->split",
		"split->building", "split->checking",
		"building->sync", "checking->sync", "sync->done",
	}
	if got := firedNames(exec); !slices.Equal(got, want) {
		t.Fatalf("fired = %v, want %v", got, want)
	}
}

// A compound transition through a choice logs the segments into the choice as
// well as the branch it takes; one through two choices logs every segment.
func TestFiredTransitionsLogsSegmentsIntoAChoice(t *testing.T) {
	src := `package test {
		state Machine {
			attribute priority : Integer = 2;
			entry; then init;
			state init;
			choice route;
			choice again;
			state low;
			state high;
			transition first init do assign priority := priority + 1 then route;
			transition first route if priority > 5 then high;
			transition first route then again;
			transition first again if priority > 5 then high;
			transition first again then low;
		}
	}`
	ctx, sym := loadState(t, src, "Machine")
	exec, err := ctx.CreateStateExecutor(sym)
	if err != nil {
		t.Fatalf("CreateStateExecutor: %v", err)
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	want := []string{"->init", "init->route", "route->again", "again->low"}
	if got := firedNames(exec); !slices.Equal(got, want) {
		t.Fatalf("fired = %v, want %v", got, want)
	}
}

// A history entered before its owner has run takes its default transition, and
// that route is logged after the transition into the history: straight to the
// default state, or through a choice with each segment on the way.
func TestFiredTransitionsLogsDefaultHistoryRoute(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		want      []string
	}{
		{"direct", `package test {
			state Machine {
				entry; then init;
				state init;
				state running {
					history previous;
					state idle;
					state busy;
					transition first previous then idle;
					transition first idle accept work then busy;
				}
				transition first init accept go then previous;
			}
		}`, []string{"->init", "init->previous", "previous->idle"}},
		{"choice", `package test {
			state Machine {
				attribute priority : Integer = 2;
				entry; then init;
				state init;
				state running {
					history previous;
					choice route;
					state idle;
					state busy;
					transition first previous then route;
					transition first route if priority > 5 then busy;
					transition first route then idle;
				}
				transition first init accept go then previous;
			}
		}`, []string{"->init", "init->previous", "previous->route", "route->idle"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, sym := loadState(t, tc.src, "Machine")
			exec, err := ctx.CreateStateExecutor(sym)
			if err != nil {
				t.Fatalf("CreateStateExecutor: %v", err)
			}
			exec.SendSignal("go", nil)
			if err := exec.ProcessNextEvent(); err != nil {
				t.Fatalf("ProcessNextEvent: %v", err)
			}
			if got := activeStateNames(exec); !strings.Contains(got, "idle") {
				t.Fatalf("active = %s, want idle", got)
			}
			if got := firedNames(exec); !slices.Equal(got, tc.want) {
				t.Fatalf("fired = %v, want %v", got, tc.want)
			}
		})
	}
}

// A firing that fails midway logs nothing: not the fork and its branches, nor
// the segments of a compound transition whose last effect fails, nor those
// into a choice when the branch out of it fails.
func TestFiredTransitionsOmitsAFailedFiring(t *testing.T) {
	for _, tc := range []struct {
		name, src string
	}{
		{"fork", `package test {
			state Machine {
				attribute counter : Integer = 0;
				entry; then init;
				state init;
				state working parallel {
					state left { entry; then leftStart; state leftStart; state building; }
					state right { entry; then rightStart; state rightStart; state checking; }
				}
				fork split;
				transition first init accept go do assign counter := missingName + 1 then split;
				transition first split then building;
				transition first split then checking;
			}
		}`},
		{"compound", `package test {
			state Machine {
				attribute counter : Integer = 0;
				entry; then init;
				state init;
				junction route;
				state low;
				transition first init accept go then route;
				transition first route do assign counter := missingName + 1 then low;
			}
		}`},
		{"choice", `package test {
			state Machine {
				attribute counter : Integer = 0;
				entry; then init;
				state init;
				choice route;
				state low;
				transition first init accept go then route;
				transition first route do assign counter := missingName + 1 then low;
			}
		}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, sym := loadState(t, tc.src, "Machine")
			exec, err := ctx.CreateStateExecutor(sym)
			if err != nil {
				t.Fatalf("CreateStateExecutor: %v", err)
			}
			if got := firedNames(exec); !slices.Equal(got, []string{"->init"}) {
				t.Fatalf("after start fired = %v, want the entry transition only", got)
			}
			exec.SendSignal("go", nil)
			if err := exec.ProcessNextEvent(); !errors.Is(err, ErrUnresolvedReference) {
				t.Fatalf("ProcessNextEvent: %v, want ErrUnresolvedReference from the effect", err)
			}
			if got := firedNames(exec); !slices.Equal(got, []string{"->init"}) {
				t.Errorf("fired = %v after the failed firing, want the entry transition only", got)
			}
		})
	}
}

// A breakpoint pauses the machine as the dispatch entering the state completes,
// even one leaving it again; the clock skips the paused machine until resumed.
func TestStateBreakpointPausesOnATransientState(t *testing.T) {
	src := `package test {
		state Vehicle {
			entry; then cruising;
			state cruising;
			accept after 5 then braking;
			state braking;
			then stopped;
			state stopped;
			accept after 5 then done;
		}
	}`
	ctx, sym := loadState(t, src, "Vehicle")
	exec, err := ctx.CreateStateExecutor(sym)
	if err != nil {
		t.Fatalf("CreateStateExecutor: %v", err)
	}
	exec.SetBreakpointAt(stateNamed(t, exec, "braking"))

	if _, err := ctx.Advance(20); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if got := exec.PausedAt(); got == nil || StateVertexName(got) != "braking" {
		t.Fatalf("PausedAt() = %v, want braking", got)
	}
	if got := exec.State(); got != StateSuspended {
		t.Errorf("State() = %v, want %v", got, StateSuspended)
	}
	if got := activeStateNames(exec); got != "braking" {
		t.Errorf("ActiveStates() = %s, want braking", got)
	}
	if got := ctx.Clock().Now(); got != 20 {
		t.Errorf("clock = %v, want 20: the clock moves on past a paused machine", got)
	}
	if !exec.HasDueEvent() {
		t.Error("HasDueEvent() = false, want the completion event held for the resumed run")
	}

	if err := exec.RunToQuiescence(); err != nil {
		t.Fatalf("RunToQuiescence: %v", err)
	}
	if exec.PausedAt() != nil {
		t.Errorf("PausedAt() = %v after resuming, want nil", exec.PausedAt())
	}
	if got := activeStateNames(exec); got != "stopped" {
		t.Errorf("ActiveStates() = %s, want stopped", got)
	}
	if got := exec.CurrentTime(); got != 20 {
		t.Errorf("CurrentTime() = %v, want 20", got)
	}
}

// A dispatch that fails entering a breakpoint state pauses on nothing, then and
// later: the next dispatch to succeed does not pause on the state it entered.
func TestStateBreakpointStagedByAFailedEntryIsDropped(t *testing.T) {
	src := `package test {
		state Machine {
			attribute counter : Integer = 0;
			entry; then init;
			state init;
			state arming {
				entry assign counter := missingName + 1;
			}
			state idle;
			transition first init accept go then arming;
			transition first init accept rest then idle;
			transition first arming accept rest then idle;
		}
	}`
	ctx, sym := loadState(t, src, "Machine")
	exec, err := ctx.CreateStateExecutor(sym)
	if err != nil {
		t.Fatalf("CreateStateExecutor: %v", err)
	}
	exec.SetBreakpointAt(stateNamed(t, exec, "arming"))
	exec.SendSignal("go", nil)
	if err := exec.ProcessNextEvent(); !errors.Is(err, ErrUnresolvedReference) {
		t.Fatalf("ProcessNextEvent: %v, want ErrUnresolvedReference from the entry", err)
	}
	if exec.PausedAt() != nil || exec.State() == StateSuspended {
		t.Fatalf("paused at %v in state %v after the failed entry, want no pause", exec.PausedAt(), exec.State())
	}

	exec.ClearBreakpoints()
	exec.SendSignal("rest", nil)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent: %v", err)
	}
	if exec.PausedAt() != nil || exec.State() == StateSuspended {
		t.Fatalf("paused at %v in state %v with no breakpoint set, want none", exec.PausedAt(), exec.State())
	}
}

// Breakpoints are cleared as a set; a pause already reached stands until resumed.
func TestClearStateBreakpointsRunsThrough(t *testing.T) {
	ctx, sym := loadState(t, debugStateSrc, "Cycle")
	exec, err := ctx.CreateStateExecutor(sym)
	if err != nil {
		t.Fatalf("CreateStateExecutor: %v", err)
	}
	exec.SetBreakpointAt(stateNamed(t, exec, "waiting"))
	exec.ClearBreakpoints()
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	if exec.PausedAt() != nil || exec.State() != StateCompleted {
		t.Fatalf("paused at %v in state %v, want a completed run", exec.PausedAt(), exec.State())
	}
}

// A breakpoint on a pseudostate pauses the machine as the dispatch passing
// through it completes, in the state the route reached; a dispatch that fails on
// the way through pauses on nothing.
func TestPseudostateBreakpointPausesAfterTheRouteThroughIt(t *testing.T) {
	src := `package test {
		state Machine {
			attribute counter : Integer = 0;
			entry; then init;
			state init;
			junction route;
			state low;
			state high;
			transition first init accept go then route;
			transition first route if counter > 5 then high;
			transition first route then low;
			transition first low accept go then route;
			transition first low accept bad do assign counter := missingName then route;
		}
	}`
	ctx, sym := loadState(t, src, "Machine")
	exec, err := ctx.CreateStateExecutor(sym)
	if err != nil {
		t.Fatalf("CreateStateExecutor: %v", err)
	}
	var route *ast.PseudostateNode
	for _, ps := range exec.graph.Pseudostates {
		if ps.Name == "route" {
			route = ps
		}
	}
	if route == nil {
		t.Fatal("the graph lowers no pseudostate route")
	}
	exec.SetBreakpointAt(route)

	exec.SendSignal("go", nil)
	if err := exec.RunToQuiescence(); err != nil {
		t.Fatalf("RunToQuiescence: %v", err)
	}
	if got := exec.PausedAt(); got != ast.Node(route) {
		t.Fatalf("PausedAt() = %v, want the junction route", got)
	}
	if got := exec.State(); got != StateSuspended {
		t.Errorf("State() = %v, want %v", got, StateSuspended)
	}
	if got := activeStateNames(exec); got != "low" {
		t.Errorf("ActiveStates() = %s, want low, where the route through the junction ended", got)
	}

	exec.Resume()
	exec.SendSignal("bad", nil)
	if err := exec.ProcessNextEvent(); !errors.Is(err, ErrUnresolvedReference) {
		t.Fatalf("ProcessNextEvent: %v, want ErrUnresolvedReference from the effect", err)
	}
	if got := exec.PausedAt(); got != nil {
		t.Errorf("PausedAt() = %v after a failed dispatch, want nil", got)
	}
}

// traversalNames spells the traversal log as source->target, prefixing an edge
// of a nested action's own flow with that action's name.
func traversalNames(exec *ActionExecutor) []string {
	out := make([]string, 0, exec.TraversalCount())
	for _, tr := range exec.Traversals() {
		prefix := ""
		for _, owner := range tr.Within {
			prefix += ActionNodeName(owner) + "/"
		}
		out = append(out, prefix+ActionNodeName(tr.Edge.Source)+"->"+ActionNodeName(tr.Edge.Target))
	}
	return out
}

// The traversal log records every succession taken, fork branches, nested flows
// and join branches included; a mark read before a step delimits what it took.
func TestTraversalsLogEachSuccessionInOrder(t *testing.T) {
	src := `package test {
		action Drive {
			attribute speed : Integer = 0;
			first start;
			fork split;
			action prep { first begin; action warm; succession first begin then warm; }
			action tally { assign speed := speed + 1; }
			join sync;
			done;
			succession first start then split;
			succession first split then prep;
			succession first split then tally;
			succession first prep then sync;
			succession first tally then sync;
			succession first sync then done;
		}
	}`
	ctx, sym := loadAction(t, src, "Drive")
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("CreateActionExecutor: %v", err)
	}
	if got := traversalNames(exec); len(got) != 0 {
		t.Fatalf("before any step traversals = %v, want none", got)
	}
	if err := exec.Step(); err != nil {
		t.Fatalf("Step: %v", err)
	}
	if got := traversalNames(exec); !slices.Equal(got, []string{"start->split"}) {
		t.Fatalf("first step traversals = %v, want start->split", got)
	}
	mark := exec.TraversalCount()
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	got := traversalNames(exec)[mark:]
	want := []string{
		"split->prep", "split->tally", "tally->sync",
		"prep/begin->warm", "prep->sync", "sync->done",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("run traversals = %v, want %v", got, want)
	}
	if since := exec.TraversalsSince(mark); len(since) != len(want) || since[0].Edge != exec.Traversals()[mark].Edge {
		t.Errorf("TraversalsSince(%d) = %v, want the %d successions past the mark", mark, since, len(want))
	}
	if since := exec.TraversalsSince(exec.TraversalCount()); since != nil {
		t.Errorf("TraversalsSince(TraversalCount()) = %v, want nil", since)
	}
	if since := exec.TraversalsSince(-1); len(since) != exec.TraversalCount() {
		t.Errorf("TraversalsSince(-1) = %d successions, want all %d", len(since), exec.TraversalCount())
	}
	tokens := make(map[int64]bool)
	for _, tr := range exec.Traversals() {
		tokens[tr.Token] = true
	}
	if len(tokens) < 2 {
		t.Errorf("traversals name %d token(s), want the fork's branches to be distinct", len(tokens))
	}

	// The copies are the caller's: a path overwritten in one leaves the record as it was.
	nested := slices.IndexFunc(exec.Traversals(), func(tr Traversal) bool { return len(tr.Within) > 0 })
	if nested < 0 {
		t.Fatal("no traversal ran in a nested flow")
	}
	exec.Traversals()[nested].Within[0] = nil
	exec.TraversalsSince(nested)[0].Within[0] = nil
	if got := traversalNames(exec); !slices.Equal(got[mark:], want) {
		t.Errorf("after writing into copies, traversals = %v, want %v unchanged", got[mark:], want)
	}
}

// An advance stops as the debugged machine pauses at a breakpoint: a sibling due
// at the same instant does not run before the advance returns, and runs once the
// clock is driven again. The default policy runs the last registered waiter
// first, so the debugged bulb is instantiated after its sibling.
func TestAdvanceUntilHaltsBeforeSiblingsRun(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "lamp.sysml", parseAndBuild(t, lampSource))
	root := idx.DocumentRoot("lamp.sysml")
	bulbDef := resolveSymbol(t, root, "Bulb")
	sibling, err := ctx.Instantiate(bulbDef)
	if err != nil {
		t.Fatalf("Instantiate sibling: %v", err)
	}
	debugged, err := ctx.Instantiate(bulbDef)
	if err != nil {
		t.Fatalf("Instantiate debugged: %v", err)
	}
	level := map[string]Value{"level": integerValue(7)}
	for _, bulb := range []*Instance{sibling, debugged} {
		runTo(t, root, ctx, bulb, "go", nil)
		runTo(t, root, ctx, bulb, "Dim", level)
		if leaf := lampLeaf(t, bulb); leaf != "dimmed" {
			t.Fatalf("bulb #%d is in %s, want dimmed", bulb.ID, leaf)
		}
	}
	machine := lampMachine(t, debugged)
	machine.SetBreakpointAt(stateNamed(t, machine, "off"))

	report, err := ctx.AdvanceUntil(10, func() bool { return machine.PausedAt() != nil })
	if err != nil {
		t.Fatalf("AdvanceUntil: %v", err)
	}
	if report.To != 5 || machine.PausedAt() == nil {
		t.Fatalf("advance reached t=%v paused at %v, want held at t=5 on the breakpoint", report.To, machine.PausedAt())
	}
	if leaf := lampLeaf(t, sibling); leaf != "dimmed" {
		t.Errorf("the sibling is in %s after the debugged machine paused, want still dimmed: it ran after the halt", leaf)
	}

	if !machine.Resume() {
		t.Fatal("Resume: the machine was not paused")
	}
	if _, err := ctx.Advance(0); err != nil {
		t.Fatalf("Advance(0): %v", err)
	}
	if leaf := lampLeaf(t, sibling); leaf != "off" {
		t.Errorf("the sibling is in %s once the clock is driven again, want off", leaf)
	}
	if leaf := lampLeaf(t, debugged); leaf != "off" {
		t.Errorf("the debugged machine is in %s after resuming, want off", leaf)
	}
}
