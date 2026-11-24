package view

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// identityOf reads the declared element's EffectiveID, the id a written form's {id} fills.
// Both are empty when no identified declaration sits at the origin.
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
	if info, ok := identity.Of(r.model, r.resolver, sym); ok && info != nil {
		id = info.EffectiveID
	}
	return name, id
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
	var walk func([]*Node)
	walk = func(nodes []*Node) {
		for _, node := range nodes {
			if node == nil {
				continue
			}
			node.ElementID = id(node.Origin)
			for i := range node.Ports {
				node.Ports[i].ElementID = id(node.Ports[i].Origin)
			}
			for _, compartment := range node.Compartments {
				walk(compartment.Rows)
			}
			walk(node.Children)
		}
	}
	walk(out.Roots)
	for i := range out.Edges {
		out.Edges[i].ElementID = id(out.Edges[i].Origin)
	}
}
