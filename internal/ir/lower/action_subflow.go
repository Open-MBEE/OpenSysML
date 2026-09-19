package lower

import (
	"errors"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
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
	if node.IsTerminate {
		lowerTerminateNode(graph, node, scope)
		return
	}
	if !statesOwnFlow(node.Members) {
		lowerBody(graph, node, scope)
		return
	}
	lowerAccept(graph, node, scope)
	if graph.Subflows == nil {
		graph.Subflows = make(map[ast.Node]*Subflow)
	}
	sub, err := ToActionGraphWith(node, scope, graph.resolver)
	if err == nil {
		StartFlow(sub)
	}
	graph.Subflows[node] = &Subflow{Graph: sub, Err: err}
}

// lowerTerminateNode records what a terminate action usage runs: the statements of
// its body as a leaf's, then the terminate it stands for. A body stating a flow of
// its own has no place to end the performance from, so it is refused at initialize.
func lowerTerminateNode(graph *ActionGraph, node *ast.Usage, scope *symbols.Scope) {
	if statesOwnFlow(node.Members) {
		if graph.Subflows == nil {
			graph.Subflows = make(map[ast.Node]*Subflow)
		}
		graph.Subflows[node] = &Subflow{Err: errors.New("a terminate action usage states no flow of its own")}
		return
	}
	lowerBody(graph, node, scope)
	graph.Bodies[node] = append(graph.Bodies[node], lowerStatement(node, scope))
}

// TerminateUsage returns the terminate a terminate action usage stands for, the last of
// its node's body after the statements it declares; false for a node that is none.
func (g *ActionGraph) TerminateUsage(node ast.Node) (Effect, bool) {
	body := g.Bodies[node]
	if len(body) == 0 {
		return Effect{}, false
	}
	last, ok := body[len(body)-1].(Effect)
	if !ok || last.Kind != EffectTerminate || last.Terminates != TerminateEnclosing || last.Node != node {
		return Effect{}, false
	}
	return last, true
}

// lowerAccept records the message a nested action node waits for, which a node
// owning a flow still does before that flow starts.
func lowerAccept(graph *ActionGraph, node *ast.Usage, scope *symbols.Scope) {
	for _, member := range node.Members {
		m, ok := unwrapMembership(member).(*ast.Usage)
		if !ok || !m.IsAccept {
			continue
		}
		port, viaSelf := acceptPort(node)
		graph.Accepts[node] = Accept{
			ParamName:    m.Ident.Name,
			SignalType:   typingTarget(m),
			ViaPort:      port,
			ViaSelf:      viaSelf,
			SubsetsEvent: subsettingTarget(m),
			Trigger:      m.Value,
			Scope:        scope,
		}
	}
}
