package view

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// shownOf is the optional notation elements the rendering view states shows:
// what its `show…` Boolean attributes say, starting from DefaultShown. It is
// nil, the default, for a view stating no rendering or one declaring none.
func (r *Renderer) shownOf(view *symbols.Symbol) Shown {
	renderings, err := r.model.ViewRenderings(view)
	if err != nil {
		return nil
	}
	for _, rendering := range renderings {
		if _, ok := r.renderingKind(rendering); !ok {
			continue
		}
		return r.shownBy(rendering)
	}
	return nil
}

// shownBy reads the `showEdgeLabels : Boolean = true` attributes a rendering
// member, the rendering it names and the renderings that one specializes
// declare, the nearest declaration of each element deciding: a rendering
// `:> Views::asInterconnectionDiagram` turns elements on or off over the
// default, and a `render` of it may turn them again.
func (r *Renderer) shownBy(rendering semantics.ViewRendering) Shown {
	chain := []*symbols.Symbol{rendering.Member}
	if rendering.Rendering != nil {
		chain = append(chain, rendering.Rendering)
		chain = append(chain, r.model.AllSupertypes(rendering.Rendering)...)
	}
	var shown Shown
	decided := map[Element]bool{}
	for _, sym := range chain {
		if sym == nil {
			continue
		}
		for _, member := range r.model.MembersOf(sym) {
			element, ok := ElementOfAttribute(member.Name)
			if !ok || decided[element] {
				continue
			}
			value, ok := booleanValue(member)
			if !ok {
				continue
			}
			if shown == nil {
				shown = DefaultShown.Union(nil)
			}
			decided[element] = true
			shown[element] = value
		}
	}
	return shown
}

// booleanValue is the Boolean literal an attribute usage is bound to, false
// when it is bound to none or to something else.
func booleanValue(sym *symbols.Symbol) (bool, bool) {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || usage.Kind != ast.UsageAttribute {
		return false, false
	}
	literal, ok := usage.Value.(*ast.LiteralBool)
	if !ok {
		return false, false
	}
	return literal.Value, true
}
