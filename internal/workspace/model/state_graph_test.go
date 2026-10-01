package model

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
)

// TestWorkspaceStateGraphResolvesMetadata: StateGraph lowers through the
// workspace's resolver, so the StateMachines metadata spellings resolve to
// their library definitions — the resolver-free ToStateGraph drops them.
func TestWorkspaceStateGraphResolvesMetadata(t *testing.T) {
	ws := NewWorkspace()
	src := `package P {
	private import StateMachines::*;
	item def Go;
	state M {
		entry; then idle;
		state idle;
		state a;
		transition first idle accept Go then pick;
		#StateMachines::choice state pick;
		transition first pick if true then busy;
		state busy;
	}
}`
	ws.Open("m.sysml", []byte(src), 1)
	syms := ws.LookupQualified("P::M")
	if len(syms) != 1 {
		t.Fatalf("got %d symbols named P::M, want 1", len(syms))
	}
	graph, err := ws.StateGraph(syms[0])
	if err != nil {
		t.Fatalf("StateGraph: %v", err)
	}
	if !hasPseudostate(graph, "pick") {
		t.Errorf("StateGraph lost the #choice pseudostate: %v", graph.Pseudostates)
	}

	// The resolver-free entry point drops it, which is why Validate goes
	// through the workspace.
	scope := syms[0].Scope
	if scope == nil {
		scope = syms[0].OwnerScope
	}
	plain, err := lower.ToStateGraph(syms[0].Decl, scope)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	if hasPseudostate(plain, "pick") {
		t.Error("ToStateGraph kept the #choice pseudostate without a resolver")
	}
}

func hasPseudostate(g *lower.StateGraph, name string) bool {
	for _, ps := range g.Pseudostates {
		if ps.Name == name {
			return true
		}
	}
	return false
}
