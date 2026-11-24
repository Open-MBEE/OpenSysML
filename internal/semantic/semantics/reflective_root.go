package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// RootNamespace is the unnamed, unowned Namespace a document's text is (KerML 8.2.3.2), reflected once
// per document, owning its top-level elements through OwningMemberships; nil for a document not indexed.
func (m *Model) RootNamespace(doc string) *symbols.Symbol {
	if m == nil || m.resolver == nil || m.resolver.Index() == nil {
		return nil
	}
	if root, ok := m.rootNamespaces[doc]; ok {
		return root
	}
	scope := m.resolver.Index().DocumentRoot(doc)
	var root *symbols.Symbol
	if scope != nil {
		// A recorded document (a cached or library index) keeps its root scope
		// without the parse, so the root is reflected from the scope alone.
		root = &symbols.Symbol{Kind: symbols.SymbolNamespace, Visibility: ast.VisibilityPublic, Scope: scope, DocName: doc}
		if node, ok := scope.Node().(*ast.RootNamespace); ok {
			root.Decl, root.DeclSpan = node, node.Span()
		}
	}
	var node ast.Node
	if root != nil {
		node = root.Decl
	}
	journal(m, m.rootNamespaces, doc, node)
	m.rootNamespaces[doc] = root
	return root
}

// IsRootNamespace reports whether sym is a document's reflected RootNamespace.
func (m *Model) IsRootNamespace(sym *symbols.Symbol) bool {
	if m == nil || sym == nil || sym.Kind != symbols.SymbolNamespace || sym.OwnerScope != nil {
		return false
	}
	return m.rootNamespaces[sym.DocName] == sym
}

// rootNamespaceOwner is the RootNamespace owning sym when sym is a top-level
// declaration of its document, nil for every other symbol.
func (m *Model) rootNamespaceOwner(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil || sym.Implicit != nil || sym.Expr != nil || sym.Chain != nil || sym.OwnerScope == nil ||
		sym.OwnerScope.Parent() != nil || m.resolver == nil || m.resolver.Index() == nil {
		return nil
	}
	if m.resolver.Index().DocumentRoot(sym.DocName) != sym.OwnerScope {
		return nil
	}
	return m.RootNamespace(sym.DocName)
}

// StatedReflectiveElements is ReflectiveElements without a document's RootNamespace, an element no
// model states: a root-level element reads no owning namespace, membership or relationship.
func (m *Model) StatedReflectiveElements(sym *symbols.Symbol, feature string) ([]*symbols.Symbol, bool) {
	elements, ok := m.ReflectiveElements(sym, feature)
	if !ok {
		return nil, false
	}
	owning := feature == "owningRelationship" || feature == "owningMembership"
	stated := func(element *symbols.Symbol) bool {
		if m.IsRootNamespace(element) {
			return false
		}
		return !owning || element.Implicit == nil || !m.IsRootNamespace(element.Implicit.Owner)
	}
	for at, element := range elements {
		if stated(element) {
			continue
		}
		// The model's answer is cached, so the kept elements are copied rather than moved.
		kept := append(make([]*symbols.Symbol, 0, len(elements)-1), elements[:at]...)
		for _, rest := range elements[at+1:] {
			if stated(rest) {
				kept = append(kept, rest)
			}
		}
		return kept, true
	}
	return elements, true
}
