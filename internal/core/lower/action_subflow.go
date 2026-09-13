package lower

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// An action node whose own members state a flow — `first`, a succession, a fork
// — owns that flow rather than being a leaf: KerML makes its steps
// subperformances of it (`Actions::Action::subactions :> actions,
// subperformances`, `Performances::subperformances subsets suboccurrences`), and
// a suboccurrence is time-enclosed by the occurrence owning it. Such a node is
// lowered to a subgraph the executor runs to completion before the node's own
// succession fires.

// Subflow is the flow a nested action node's own members state. Graph is nil
// when those members state a flow that could not be built, which Err says; the
// executor reports it at initialize() rather than treating the node as a leaf.
type Subflow struct {
	Graph *ActionGraph
	Err   error
}

// statesOwnFlow reports whether an action node's members state a flow of its
// own: a start, an end, an edge or a control node. A node whose members are only
// statements, parameters or undirected declarations states none and stays a leaf.
func statesOwnFlow(members []ast.Node) bool {
	for _, member := range members {
		if outsideBlockFlow(unwrapMembership(member)) {
			return true
		}
	}
	return false
}

// lowerActionNode records what a nested action node runs: the flow its own
// members state (its start inferred as a whole action's is), or — where they
// state none — the statements and accept of a leaf. scope is the node's own namespace.
func lowerActionNode(graph *ActionGraph, node *ast.Usage, scope *symbols.Scope) {
	lowerFeatures(graph, node, scope)
	if !statesOwnFlow(node.Members) {
		lowerBody(graph, node, scope)
		return
	}
	lowerAccept(graph, node, scope)
	if graph.Subflows == nil {
		graph.Subflows = make(map[ast.Node]*Subflow)
	}
	sub, err := ToActionGraph(node, scope)
	if err == nil {
		StartFlow(sub)
	}
	graph.Subflows[node] = &Subflow{Graph: sub, Err: err}
}

// lowerAccept records the message a nested action node waits for, which a node
// owning a flow still does before that flow starts.
func lowerAccept(graph *ActionGraph, node *ast.Usage, scope *symbols.Scope) {
	for _, member := range node.Members {
		m, ok := unwrapMembership(member).(*ast.Usage)
		if !ok || !m.IsAccept {
			continue
		}
		graph.Accepts[node] = Accept{
			ParamName:    m.Ident.Name,
			SignalType:   typingTarget(m),
			ViaPort:      acceptPort(node),
			SubsetsEvent: subsettingTarget(m),
			Trigger:      m.Value,
			Scope:        scope,
		}
	}
}
