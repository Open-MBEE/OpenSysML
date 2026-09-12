package view

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Draws reports how a rendering of kind shows sym: as a node a Layout positions,
// as an edge a Route steers, or not at all. The answer is the classification the
// renderer of that kind draws by.
func (r *Renderer) Draws(kind Kind, sym *symbols.Symbol) (node, edge bool) {
	if sym == nil {
		return false, false
	}
	switch kind {
	case KindTree:
		return containedKind(sym), false
	case KindInterconnection:
		return featureLike(sym), r.model.IsConnectorUsage(sym) || isFlowUsage(sym)
	case KindState:
		return stateLike(sym), isTransition(sym) || sym.Kind == symbols.SymbolSuccessionUsage
	case KindAction:
		return actionLike(sym), sym.Kind == symbols.SymbolSuccessionUsage || isFlowUsage(sym)
	}
	return false, false
}

// DrawsAnywhere reports whether some rendering kind draws sym as a node and
// whether some draws it as an edge: what an annotation applying in every view
// can position.
func (r *Renderer) DrawsAnywhere(sym *symbols.Symbol) (node, edge bool) {
	for _, kind := range Kinds() {
		n, e := r.Draws(kind, sym)
		node, edge = node || n, edge || e
	}
	return node, edge
}

// stateLike reports whether a state rendering draws sym as a node: a state
// machine, a state or a region.
func stateLike(sym *symbols.Symbol) bool {
	return sym.Kind == symbols.SymbolStateDef || sym.Kind == symbols.SymbolStateUsage
}

// isTransition reports whether sym is a named transition, an action usage a
// state rendering draws as an edge rather than a node.
func isTransition(sym *symbols.Symbol) bool {
	_, ok := sym.Decl.(*ast.TransitionMember)
	return ok
}

// actionLike reports whether an action rendering draws sym as a node: an action
// or one of the control nodes of its flow.
func actionLike(sym *symbols.Symbol) bool {
	switch sym.Kind {
	case symbols.SymbolActionDef:
		return true
	case symbols.SymbolActionUsage:
		return !isTransition(sym)
	}
	return false
}
