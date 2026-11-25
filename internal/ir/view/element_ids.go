package view

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/naming"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// identityOf reads the declared element's EffectiveID, the id a written form's {id} fills.
// An unnamed connector the index names only by its owner falls back to the positional
// identity the element form gives the declaration, so distinct anonymous connectors
// under one owner keep distinct ids. Both are empty when no identified declaration sits
// at the origin.
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
	if anonymousConnector(sym) && (info == nil || info.Source == identity.SourceDerived) {
		// The derived id encodes the index's name, which every unnamed member of
		// one owner shares. Name the declaration by its position instead, as the
		// element form does, so a canvas can tell the connectors apart.
		if positional := r.positionalOf(origin.Doc, sym); positional != "" {
			id = identity.DerivedElementID(positional)
		}
	}
	return name, id
}

// anonymousConnector reports whether sym is an unnamed connector usage whose id
// would otherwise derive from its owner's name alone.
func anonymousConnector(sym *symbols.Symbol) bool {
	if sym == nil || sym.Named() {
		return false
	}
	switch sym.Kind {
	case symbols.SymbolConnectionUsage, symbols.SymbolBindingUsage,
		symbols.SymbolSuccessionUsage, symbols.SymbolFlowUsage,
		symbols.SymbolSuccessionFlowUsage, symbols.SymbolInterfaceUsage,
		symbols.SymbolAllocationUsage:
		return true
	}
	return false
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
		bySym = derivePositional(index, r.text, doc)
		if r.positional == nil {
			r.positional = make(map[string]map[*symbols.Symbol]string)
		}
		r.positional[doc] = bySym
	}
	return bySym[sym]
}

// derivePositional derives the positional identities of one document from the
// tree the index holds and the notation the renderer reads. The elements the
// index names already are not candidates, the same test the element form uses.
func derivePositional(index *symbols.Index, text source.Lookup, doc string) map[*symbols.Symbol]string {
	scope := index.DocumentRoot(doc)
	if scope == nil {
		return nil
	}
	root, ok := scope.Node().(*ast.RootNamespace)
	if !ok || root == nil {
		return nil
	}
	file := source.New(doc, []byte(text(doc, root.Span())))
	bySym, _ := naming.PositionalIdentities(index, []naming.Document{{File: file, Root: root, Scope: scope}},
		func(sym *symbols.Symbol) bool { return index.QualifiedIdentity(sym) != "" })
	return bySym
}

// walkNodes visits every node of a rendering once, roots, compartment rows and
// children included. A rendering can share one node between several parents —
// folded rows and expanded type trees clone pointers rather than subtrees — so
// a plain recursive walk would re-enter shared nodes exponentially.
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
