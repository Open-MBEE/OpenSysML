package view

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/naming"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// identityOf reads the declared element's EffectiveID, falling back to the
// positional identity of an unnamed declaration the index names by its owner.
func (r *Renderer) identityOf(origin Origin) (name, id string) {
	if r == nil || !origin.Located() || r.model == nil || r.resolver == nil {
		return "", ""
	}
	index := r.resolver.Index()
	if index == nil {
		return "", ""
	}
	scope := index.DocumentRoot(origin.Doc)
	if scope == nil {
		return "", ""
	}
	sym := scope.DeclaredAt(origin.Span)
	if sym == nil {
		return "", ""
	}
	if sym.Named() {
		name = source.QualifiedNameOf(symbols.NameChain(sym))
	}
	var info *identity.Info
	if found, ok := identity.Of(r.model, r.resolver, sym); ok {
		info = found
		id = info.EffectiveID
	}
	if ownerNamedOnly(index, sym, info) {
		// The derived id names every unnamed member of one owner alike; use the
		// declaration's position so a property lookup resolves the id.
		if positional := r.positionalOf(origin.Doc, sym); positional != "" {
			id = identity.DerivedElementID(positional)
		}
	}
	return name, id
}

// ownerNamedOnly reports whether the identity builder derived no name of its
// own, leaving the index's owner-only fallback id.
func ownerNamedOnly(index *symbols.Index, sym *symbols.Symbol, info *identity.Info) bool {
	if sym == nil || sym.Named() || index == nil {
		return false
	}
	if info == nil {
		return true
	}
	return info.Source == identity.SourceDerived && info.FQN == index.GetFQN(sym)
}

// positionalOf names sym by its position in doc, the way the element form does.
// It derives the document at most once and keeps the result for every origin in it.
func (r *Renderer) positionalOf(doc string, sym *symbols.Symbol) string {
	if r == nil || sym == nil || doc == "" || r.resolver == nil || r.text == nil {
		return ""
	}
	index := r.resolver.Index()
	if index == nil {
		return ""
	}
	r.positionalMu.Lock()
	defer r.positionalMu.Unlock()
	bySym, ok := r.positional[doc]
	if !ok {
		bySym = r.derivePositional(doc)
		if r.positional == nil {
			r.positional = make(map[string]map[*symbols.Symbol]string)
		}
		r.positional[doc] = bySym
	}
	return bySym[sym]
}

// derivePositional derives the positional identities of one document from the
// tree the index holds and the notation the renderer reads, using the resolver
// and model this renderer already has. Re-resolving the document here would
// change what later renderings of the same model draw, breaking drawn-query
// parity; PositionalIdentitiesResolved derives names without that side effect.
func (r *Renderer) derivePositional(doc string) map[*symbols.Symbol]string {
	index := r.resolver.Index()
	scope := index.DocumentRoot(doc)
	if scope == nil {
		return nil
	}
	root, ok := scope.Node().(*ast.RootNamespace)
	if !ok || root == nil {
		return nil
	}
	file := source.New(doc, []byte(r.text(doc, root.Span())))
	bySym, _ := naming.PositionalIdentitiesResolved(index, []naming.Document{{File: file, Root: root, Scope: scope}},
		r.resolver, r.model, func(sym *symbols.Symbol) bool { return index.QualifiedIdentity(sym) != "" })
	return bySym
}

// walkNodes visits every node once, roots, compartments and children included;
// shared node pointers make a plain recursion re-enter them exponentially.
func walkNodes(roots []*Node, visit func(*Node)) {
	seen := make(map[*Node]bool)
	var walk func([]*Node)
	walk = func(nodes []*Node) {
		for _, node := range nodes {
			if node == nil || seen[node] {
				continue
			}
			seen[node] = true
			visit(node)
			for _, compartment := range node.Compartments {
				walk(compartment.Rows)
			}
			walk(node.Children)
		}
	}
	walk(roots)
}

// resolveElementIDs fills nodes, ports and edges with the identity their origins link by.
// The ids stay as data; nothing in the rendering points back into the model.
func (r *Renderer) resolveElementIDs(out *Rendering) {
	if r == nil || out == nil {
		return
	}
	cache := make(map[Origin]string)
	id := func(origin Origin) string {
		if !origin.Located() {
			return ""
		}
		if found, ok := cache[origin]; ok {
			return found
		}
		_, found := r.identityOf(origin)
		cache[origin] = found
		return found
	}
	walkNodes(out.Roots, func(node *Node) {
		node.ElementID = id(node.Origin)
		for i := range node.Ports {
			node.Ports[i].ElementID = id(node.Ports[i].Origin)
		}
	})
	for i := range out.Edges {
		out.Edges[i].ElementID = id(out.Edges[i].Origin)
	}
}
