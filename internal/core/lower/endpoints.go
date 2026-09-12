package lower

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// connectorEndReference returns the feature a connector end attaches to.
func connectorEndReference(end *ast.ConnectorEnd) ast.Node {
	if end == nil {
		return nil
	}
	return end.AttachedTarget()
}

// EndpointResolver resolves the vertex a transition endpoint names, implemented
// by the name-resolution tier (*resolve.Resolver) so lowering matches no names
// itself.
type EndpointResolver interface {
	// Endpoint returns the declaration qn names, written in scope, and whether it
	// names a vertex at all.
	Endpoint(scope *symbols.Scope, qn *ast.QualifiedName) (decl ast.Node, ok bool)
}

// machineEndpoints resolves the endpoints of one machine through a resolver over
// an index of that machine, from the scope each endpoint was written in.
type machineEndpoints struct {
	resolver *resolve.Resolver
	scope    *symbols.Scope
}

func (m machineEndpoints) Endpoint(scope *symbols.Scope, qn *ast.QualifiedName) (ast.Node, bool) {
	if scope == nil {
		scope = m.scope
	}
	return m.resolver.Endpoint(scope, qn)
}

func (m machineEndpoints) RedefinitionTarget(scope *symbols.Scope, decl ast.Node, target ast.Node) (*symbols.Symbol, bool) {
	if scope == nil {
		scope = m.scope
	}
	return m.resolver.RedefinitionTarget(scope, decl, target)
}

// scopeEndpoints resolves an endpoint from the caller's own scope tree, for a
// machine lowered with that tree but without the resolver over its document.
type scopeEndpoints struct{ machine *symbols.Scope }

func (s scopeEndpoints) Endpoint(scope *symbols.Scope, qn *ast.QualifiedName) (ast.Node, bool) {
	if scope == nil {
		scope = s.machine
	}
	return resolve.VertexInScope(scope, qn)
}

// localEndpoints indexes a machine no scope tree holds — a hand-built one in a
// unit test — and returns its endpoints plus the machine body scope of that
// index, which lowering descends so a region names its own vertices.
func localEndpoints(decl ast.Node) (EndpointResolver, *symbols.Scope) {
	root := &ast.RootNamespace{Members: []ast.Node{decl}}
	idx := symbols.NewIndexFromDoc("<lowered>", root)
	scope := idx.DocumentRoot("<lowered>")
	if scope != nil {
		if body := scope.ChildFor(decl); body != nil {
			scope = body
		}
	}
	return machineEndpoints{resolver: resolve.New(idx), scope: scope}, scope
}
