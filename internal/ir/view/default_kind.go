package view

import "github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"

// DefaultKind chooses the rendering kind a set of elements is drawn in when
// none is asked for: the kind each element calls for, and KindMixed when they
// disagree. An element asks for a state rendering when it is a state def or
// usage, an action rendering when it is an action def or usage, a case
// rendering when it is a case def or usage of any kind, an interconnection
// rendering when it is a structural usage holding a connector, binding or flow,
// and a tree rendering otherwise — a definition, a package or a usage with no
// connections to draw. An empty set is a tree.
func (r *Renderer) DefaultKind(exposed []*symbols.Symbol) Kind {
	chosen := Kind("")
	for _, sym := range exposed {
		kind := r.ElementKind(sym)
		switch {
		case chosen == "":
			chosen = kind
		case chosen != kind:
			return KindMixed
		}
	}
	if chosen == "" {
		return KindTree
	}
	return chosen
}

// ElementKind is the rendering kind one element calls for; see DefaultKind.
func (r *Renderer) ElementKind(sym *symbols.Symbol) Kind {
	if sym == nil {
		return KindTree
	}
	switch sym.Kind {
	case symbols.SymbolStateDef, symbols.SymbolStateUsage:
		return KindState
	case symbols.SymbolActionDef, symbols.SymbolActionUsage:
		return KindAction
	case symbols.SymbolCaseDef, symbols.SymbolCaseUsage,
		symbols.SymbolAnalysisCaseDef, symbols.SymbolAnalysisCaseUsage,
		symbols.SymbolVerificationCaseDef, symbols.SymbolVerificationCaseUsage,
		symbols.SymbolUseCaseDef, symbols.SymbolUseCaseUsage:
		return KindCase
	}
	if sym.Kind.IsFeature() && featureLike(sym) && r.holdsConnections(sym) {
		return KindInterconnection
	}
	return KindTree
}

// holdsConnections reports whether an element contains a connection an
// interconnection rendering draws as an edge: a connector, binding or flow
// usage, or a feature bound by its value to another feature.
func (r *Renderer) holdsConnections(sym *symbols.Symbol) bool {
	for _, member := range r.containedMembers(sym) {
		if r.drawsConnector(member) || r.bindsByValue(member) {
			return true
		}
	}
	return false
}

// bindsByValue reports whether a feature's value names another feature that
// resolves — the binding valueBindingEdges draws.
func (r *Renderer) bindsByValue(sym *symbols.Symbol) bool {
	value := featureValue(sym)
	if value == nil {
		return false
	}
	_, resolved := r.resolver.ResolveTarget(sym.OwnerScope, value)
	return resolved
}
