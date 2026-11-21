package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// SuccessionMintsEnds reports whether a `then` succession owns two end features, as the conversion mints
// them and the pilot exports them: its source is the member before it, named or reached by position (TargetSuccession).
func SuccessionMintsEnds(edge *ast.SuccessionEdge) bool {
	if edge == nil {
		return false
	}
	implied := edge.Source != nil && edge.Source.Span().Offset < edge.Span().Offset
	positional := edge.Source == nil && edge.SourceMember != nil
	return implied || positional
}

// SuccessionEndMembers are the source and target end features a `then x;` or `first a then b;` succession owns
// through an EndFeatureMembership each (successionOwnsMintedEnds), nameless and named by the end they redefine.
func (m *Model) SuccessionEndMembers(sym *symbols.Symbol) []*symbols.Symbol {
	if m == nil || sym == nil || sym.Implicit != nil || sym.Recorded() || sym.Scope == nil {
		return nil
	}
	forms := []symbols.ImpliedForm{symbols.ImpliedSuccessionSourceEnd, symbols.ImpliedSuccessionTargetEnd}
	if transitionSuccessionPart(sym) {
		// The succession a transition owns mints its two ends likewise (transitionSuccession), under the transition.
		forms = []symbols.ImpliedForm{symbols.ImpliedTransitionSuccessionSourceEnd, symbols.ImpliedTransitionSuccessionTargetEnd}
	} else if !successionOwnsMintedEnds(sym) {
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

// successionOwnsMintedEnds reports whether sym is a succession whose two ends the notation leaves unnamed: a
// `then x;` (TargetSuccession) or an action body's `first a then b;`.
func successionOwnsMintedEnds(sym *symbols.Symbol) bool {
	switch decl := sym.Decl.(type) {
	case *ast.SuccessionEdge:
		// A state body's `then x;` after a state declares a TransitionUsage, no connector: it owns no ends itself.
		return SuccessionMintsEnds(decl) && !sym.DeclaresTransition()
	case *ast.InitialNode:
		return sym.Kind == symbols.SymbolSuccessionUsage && symbols.FirstNamesSource(decl)
	}
	return false
}

// successionEndReference is the feature a succession's minted end references; stated reports a ReferenceSubsetting
// the end owns, which the EmptySourceEndMember of a `then` (its source the occurrence before it, §7.17.2) does not.
func (m *Model) successionEndReference(end *symbols.Symbol) (target *symbols.Symbol, stated bool) {
	i, ok := symbols.ImpliedSuccessionEndIndex(end)
	if !ok || end.OwnerScope == nil {
		return nil, false
	}
	succession := end.OwnerScope.Owner()
	if succession == nil {
		return nil, false
	}
	decl, _ := symbols.ImpliedDeclOf(end)
	switch edge := decl.Of.(type) {
	case *ast.SuccessionEdge:
		stated = i == 1 && (edge.Target != nil || edge.TargetMember != nil)
	case *ast.TransitionMember:
		stated = i == 1 && edge.Target != nil
	case *ast.InitialNode:
		stated = true
	}
	if related, _ := m.connectorSuccessionRelatedFeatures(succession); i < len(related) {
		target = related[i]
	}
	return target, stated
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

// successionEndTarget is the reference a `then` succession's target end names, nil for its source end.
func successionEndTarget(end *symbols.Symbol) *ast.QualifiedName {
	if i, ok := symbols.ImpliedSuccessionEndIndex(end); !ok || i != 1 {
		return nil
	}
	decl, _ := symbols.ImpliedDeclOf(end)
	switch edge := decl.Of.(type) {
	case *ast.SuccessionEdge:
		return edge.Target
	case *ast.TransitionMember:
		return edge.Target
	case *ast.ControlFlowEdge:
		return edge.Target
	}
	return nil
}
