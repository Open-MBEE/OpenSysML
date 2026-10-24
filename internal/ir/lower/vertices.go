package lower

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// VertexDeclsResolving returns every declaration a transition of this state machine may name
// as an endpoint, as the lowered graph collects them; a nil resolver reads no metadata vertices.
func VertexDeclsResolving(stateMachineDecl ast.Node, scope *symbols.Scope, resolver *resolve.Resolver) (map[ast.Node]bool, error) {
	members, err := machineMembers(stateMachineDecl)
	if err != nil {
		return nil, err
	}
	// Collecting vertices resolves no endpoint, so the graph needs none.
	graph := newStateGraph(scope, nil)
	graph.resolver = resolver
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
		if err := collectVertices(graph, group.nodes, group.scope, group.owner, parallel); err != nil {
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
