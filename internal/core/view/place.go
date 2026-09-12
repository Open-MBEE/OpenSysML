package view

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Drawn is what one rendering of a view draws: the elements it positions as
// nodes and those it steers as edges, by their declarations.
type Drawn struct {
	nodes, edges map[ast.Node]bool
}

// Node reports whether the rendering draws sym as a node a Layout positions.
func (d *Drawn) Node(sym *symbols.Symbol) bool {
	return d != nil && sym != nil && d.nodes[sym.Decl]
}

// Edge reports whether the rendering draws sym as an edge a Route steers.
func (d *Drawn) Edge(sym *symbols.Symbol) bool {
	return d != nil && sym != nil && d.edges[sym.Decl]
}

// note records that the rendering drew elem; a nil Drawn collects nothing.
func (d *Drawn) note(elem *symbols.Symbol, asEdge bool) {
	if d == nil || elem == nil || elem.Decl == nil {
		return
	}
	if asEdge {
		d.edges[elem.Decl] = true
	} else {
		d.nodes[elem.Decl] = true
	}
}

// DrawnIn renders view and reports what the rendering draws: the elements a
// Layout or Route stated in the view's body can apply to. A view that does
// not render is the error Render gives.
func (r *Renderer) DrawnIn(view *symbols.Symbol) (*Drawn, error) {
	drawn := &Drawn{nodes: map[ast.Node]bool{}, edges: map[ast.Node]bool{}}
	if _, err := r.render(view, drawn); err != nil {
		return nil, err
	}
	return drawn, nil
}

// draws reports how a rendering of kind shows sym: as a node a Layout positions,
// as an edge a Route steers, or not at all. The answer is the classification the
// renderer of that kind draws by.
func (r *Renderer) draws(kind Kind, sym *symbols.Symbol) (node, edge bool) {
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
		n, e := r.draws(kind, sym)
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
