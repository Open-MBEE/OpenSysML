package runtime

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// breakpointAt is the breakpoint on node in the flow of within, which exec must resolve.
func breakpointAt(t *testing.T, exec *ActionExecutor, within []ast.Node, node ast.Node) NodeBreakpoint {
	t.Helper()
	bp, ok := exec.BreakpointAt(within, node)
	if !ok {
		t.Fatalf("BreakpointAt(%v, %v) resolves no node", within, node)
	}
	return bp
}

// A breakpoint set by identity is keyed by the ids of the nodes it names, each in the flow
// the node before it performs them in; a node named under a path not leading to it resolves to nothing.
func TestBreakpointAtResolvesNodesInTheirFlows(t *testing.T) {
	exec := blockDebugExecutor(t)
	choose := actionNodeNamed(t, exec, "choose")
	var q ast.Node
	for _, node := range exec.Graph().BlockNodes[choose] {
		if ActionNodeName(node) == "q" {
			q = node
		}
	}
	at := breakpointAt(t, exec, []ast.Node{choose}, q)
	if want := (NodeRef{Graph: exec.Graph(), ID: exec.Graph().ID[choose]}); len(at.Within) != 1 || at.Within[0] != want {
		t.Errorf("Within = %v, want choose by id in the action's own flow", at.Within)
	}
	if at.Node.Graph == exec.Graph() || at.Node.Graph == nil || at.Node.Node() != q || at.Node.Graph.ID[q] != at.Node.ID {
		t.Errorf("Node = %+v, want q by id in the block flow of choose's body", at.Node)
	}
	if bp, ok := exec.BreakpointAt(nil, q); ok {
		t.Errorf("BreakpointAt(nil, q) = %+v, want q unresolved in the action's own flow", bp)
	}
	if bp, ok := exec.BreakpointAt([]ast.Node{q}, choose); ok {
		t.Errorf("BreakpointAt([q], choose) = %+v, want choose unresolved under q", bp)
	}

	exec.ReplaceBreakpointsAt([]NodeBreakpoint{at})
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	if bp, ok := exec.PausedBreakpoint(); !ok || bp.Node != at.Node || !slices.Equal(bp.Within, at.Within) {
		t.Errorf("PausedBreakpoint() = %+v, %v, want the breakpoint set, by the same ids", bp, ok)
	}
	if tokens := exec.Tokens(); len(tokens) != 1 || tokens[0].Location != at.Within[0] {
		t.Errorf("tokens = %v, want the one at choose, keyed by its id", tokens)
	}
}

// A node of a deferred flow, the typed body of a step nested in its own type, is one a
// breakpoint can name: the path unfolds the flow, as the performance running it does.
func TestBreakpointAtUnfoldsADeferredFlow(t *testing.T) {
	ctx, sym := loadAction(t, `package test {
	action def Walk {
		in depth = 3;
		out reached = 0;
		attribute visited = 1;
		if depth > 0 {
			action step : Walk { in depth = depth - 1; assign visited := visited + 1; }
			then assign reached := step.reached + 1;
		} else {
			assign reached := 1;
		}
	}
}`, "Walk")
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("CreateActionExecutor: %v", err)
	}
	graph := exec.Graph()
	var step ast.Node
	for _, node := range graph.Nodes {
		for _, nested := range graph.BlockNodes[node] {
			if ActionNodeName(nested) == "step" {
				step = nested
			}
		}
	}
	if step == nil {
		t.Fatalf("Walk's flow declares no step: %v", exec.NodeNames())
	}
	block, ok := locateNode(graph, step)
	if !ok {
		t.Fatalf("step is not located in a block flow of Walk's")
	}
	if inner := block.Graph.Subflows[step]; inner == nil || inner.Deferred == nil || inner.Graph != nil {
		t.Fatalf("step's flow is not deferred: %+v", inner)
	}

	at := breakpointAt(t, exec, []ast.Node{step}, step)
	if len(at.Within) != 1 || at.Within[0].Node() != step || at.Node.Node() != step || at.Node.Graph == at.Within[0].Graph {
		t.Fatalf("BreakpointAt = %+v, want step by id in the block flow of the unfolded body, under step in Walk's", at)
	}
	exec.ReplaceBreakpointsAt([]NodeBreakpoint{at})
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	if got := exec.PausedAt(); got != "step" {
		t.Fatalf("PausedAt() = %q, want step one level down", got)
	}
	if bp, ok := exec.PausedBreakpoint(); !ok || bp.Node != at.Node || !slices.Equal(bp.Within, at.Within) {
		t.Errorf("PausedBreakpoint() = %+v, %v, want the breakpoint set, by the same ids", bp, ok)
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion after the pause: %v", err)
	}
	if got := exec.PausedAt(); got != "" || !exec.State().Ended() {
		t.Errorf("after resuming: PausedAt() = %q, state %v; want the run ended, the breakpoint firing once", got, exec.State())
	}
}
