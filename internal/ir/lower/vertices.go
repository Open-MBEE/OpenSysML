package lower

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// VertexDecls returns every declaration a transition of this state machine may
// name as an endpoint, collected by the pass a lowered graph's vertices are
// collected by, so a checker reads the same ownership the executor does.
// scope is the scope the machine's body was declared in.
func VertexDecls(stateMachineDecl ast.Node, scope *symbols.Scope) (map[ast.Node]bool, error) {
	members, err := machineMembers(stateMachineDecl)
	if err != nil {
		return nil, err
	}
	// Collecting vertices resolves no endpoint, so the graph needs none.
	graph := newStateGraph(scope, nil)
	inherited, owners, err := graph.inheritedContent(stateMachineDecl, outerScope(scope, stateMachineDecl))
	if err != nil {
		return nil, err
	}
	parallel := stateMachineIsParallel(stateMachineDecl)
	for _, owner := range owners {
		parallel = parallel || stateMachineIsParallel(owner)
	}
	body := append(append([]inheritedMember{}, inherited...), ownMembers(members, scope)...)
	for _, group := range groupMembers(body) {
		if err := collectVertices(graph, group.nodes, group.scope, parallel); err != nil {
			return nil, err
		}
	}
	vertices := make(map[ast.Node]bool, len(graph.vertexOf))
	for decl := range graph.vertexOf {
		vertices[decl] = true
	}
	// A declaration a state inherits is a vertex of the machine too: it is named
	// through the state that inherits it (`nested.i1`).
	for _, inst := range graph.instanceOf {
		for decl := range inst.vertexOf {
			vertices[decl] = true
		}
	}
	return vertices, nil
}
