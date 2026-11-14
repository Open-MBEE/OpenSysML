package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// SuccessionMintsEnds reports whether a `then` succession owns two end features, as the conversion mints
// them: its source is the member before it, or a positional source carries a multiplicity (TargetSuccession).
func SuccessionMintsEnds(edge *ast.SuccessionEdge) bool {
	if edge == nil {
		return false
	}
	implied := edge.Source != nil && edge.Source.Span().Offset < edge.Span().Offset
	positional := edge.Source == nil && edge.SourceMember != nil
	return implied || positional && (edge.SourceMultiplicity != nil || edge.TargetMultiplicity != nil)
}

// SuccessionEndMembers are the source and target end features a `then x;` succession owns through
// an EndFeatureMembership each (SuccessionMintsEnds), nameless and named by the end they redefine.
func (m *Model) SuccessionEndMembers(sym *symbols.Symbol) []*symbols.Symbol {
	if m == nil || sym == nil || sym.Implicit != nil || sym.Recorded() || sym.Scope == nil {
		return nil
	}
	forms := []symbols.ImpliedForm{symbols.ImpliedSuccessionSourceEnd, symbols.ImpliedSuccessionTargetEnd}
	if transitionSuccessionPart(sym) {
		// The succession a transition owns mints its two ends likewise (transitionSuccession), under the transition.
		forms = []symbols.ImpliedForm{symbols.ImpliedTransitionSuccessionSourceEnd, symbols.ImpliedTransitionSuccessionTargetEnd}
	} else if edge, ok := sym.Decl.(*ast.SuccessionEdge); !ok || !SuccessionMintsEnds(edge) || sym.DeclaresTransition() {
		// A state body's `then x;` after a state declares a TransitionUsage, no connector: it owns no ends itself.
		return nil
	}
	defer m.own(sym).LeaveDoc()
	if cached, ok := m.successionEnds[sym]; ok {
		return cached
	}
	journal(m, m.successionEnds, sym, sym.Decl)
	out := make([]*symbols.Symbol, len(forms))
	for i, form := range forms {
		out[i] = &symbols.Symbol{
			Kind:       symbols.SymbolConnectorEnd,
			DocName:    sym.DocName,
			DeclSpan:   source.Span{Offset: sym.Decl.Span().Offset},
			Visibility: ast.VisibilityDefault,
			OwnerScope: sym.Scope,
			Decl:       &symbols.ImpliedDecl{Of: sym.Decl, Form: form},
		}
	}
	m.successionEnds[sym] = out
	return out
}

// transitionSuccessionPart reports whether sym is the succession a transition owns (transitionPartsOf).
func transitionSuccessionPart(sym *symbols.Symbol) bool {
	return sym != nil && sym.Expr != nil && sym.Expr.Role == symbols.ExpressionTransitionSuccession
}

// successionEndReference is the feature a `then` succession's target end reference-subsets (nil when the model
// resolves none); stated is false for the source end, which the conversion mints empty (encodeSuccessionEdge).
func (m *Model) successionEndReference(end *symbols.Symbol) (target *symbols.Symbol, stated bool) {
	i, ok := symbols.ImpliedSuccessionEndIndex(end)
	if !ok || i != 1 || end.OwnerScope == nil {
		return nil, false
	}
	succession := end.OwnerScope.Owner()
	if succession == nil {
		return nil, false
	}
	decl, _ := symbols.ImpliedDeclOf(end)
	switch edge := decl.Of.(type) {
	case *ast.SuccessionEdge:
		if edge.Target == nil && edge.TargetMember == nil {
			return nil, false
		}
	case *ast.TransitionMember:
		if edge.Target == nil {
			return nil, false
		}
	}
	if related, _ := m.connectorSuccessionRelatedFeatures(succession); len(related) == 2 {
		target = related[1]
	}
	return target, true
}

// successionEndMultiplicity is the range a `then` succession writes for the generated end sym, as the
// conversion gives it: the source multiplicity to the source end, the crossing one to the target end.
func successionEndMultiplicity(sym *symbols.Symbol) *ast.Multiplicity {
	i, ok := symbols.ImpliedSuccessionEndIndex(sym)
	if !ok {
		return nil
	}
	decl, _ := symbols.ImpliedDeclOf(sym)
	switch edge := decl.Of.(type) {
	case *ast.SuccessionEdge:
		if i == 0 {
			return edge.SourceMultiplicity
		}
		return edge.TargetMultiplicity
	case *ast.TransitionMember:
		// A transition's succession writes only the target's range (`then [0..1] busy`).
		if i == 1 {
			return edge.TargetMultiplicity
		}
	}
	return nil
}
