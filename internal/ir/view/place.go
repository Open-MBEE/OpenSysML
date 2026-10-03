package view

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// Drawn is what one rendering of a view draws: the elements it positions as
// nodes and those it steers as edges, by their declarations; a member drawn as
// an owner's — the start or done a body inherits — by the owner as well.
type Drawn struct {
	nodes, edges map[ast.Node]bool
	members      map[ast.Node]map[ast.Node]bool // member decl -> owner decls it is drawn as a member of
}

// Node reports whether the rendering draws sym as a node a Layout positions.
func (d *Drawn) Node(sym *symbols.Symbol) bool {
	return d != nil && sym != nil && d.nodes[sym.Decl]
}

// MemberNode reports whether the rendering draws sym as a node of owner's: one
// element inherited by many bodies is drawn once per body, and a Layout naming
// it as one inheriting owner's positions that body's alone. One naming it as
// the declaring owner's positions every body's, as a member the rendering draws
// as no owner's in particular is drawn for every owner.
func (d *Drawn) MemberNode(owner, sym *symbols.Symbol) bool {
	if d == nil || sym == nil {
		return false
	}
	owners, ok := d.members[sym.Decl]
	if !ok || owner != nil && sym.Owner() != nil && sym.Owner().Decl == owner.Decl {
		return d.Node(sym)
	}
	return owner != nil && owners[owner.Decl]
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

// noteMember records that the rendering drew member as owner's.
func (d *Drawn) noteMember(owner, member *symbols.Symbol) {
	if d == nil || owner == nil || member == nil || owner.Decl == nil || member.Decl == nil {
		return
	}
	d.note(member, false)
	owners, ok := d.members[member.Decl]
	if !ok {
		owners = map[ast.Node]bool{}
		d.members[member.Decl] = owners
	}
	owners[owner.Decl] = true
}

// DrawnIn renders view and reports what the rendering draws: the elements a
// Layout or Route stated in the view's body can apply to. A view that does
// not render is the error Render gives.
func (r *Renderer) DrawnIn(view *symbols.Symbol) (*Drawn, error) {
	drawn := &Drawn{nodes: map[ast.Node]bool{}, edges: map[ast.Node]bool{}, members: map[ast.Node]map[ast.Node]bool{}}
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
		return r.contentKind(sym), false
	case KindInterconnection:
		return featureLike(sym), r.drawsConnector(sym)
	case KindState:
		return stateLike(sym), isTransition(sym) || sym.Kind == symbols.SymbolSuccessionUsage
	case KindAction:
		return actionLike(sym), sym.Kind == symbols.SymbolSuccessionUsage || isFlowUsage(sym)
	case KindCase:
		if caseFamily(sym) {
			return !isReferencedIncludedCase(sym), isIncludedCase(sym) || hasCaseOwner(sym)
		}
		role := semantics.IsActorUsage(sym) || semantics.IsSubjectUsage(sym) || semantics.IsObjectiveUsage(sym)
		return role, role
	case KindMixed:
		caseRole := semantics.IsActorUsage(sym) || semantics.IsSubjectUsage(sym) || semantics.IsObjectiveUsage(sym)
		return featureLike(sym) || stateLike(sym) || actionLike(sym) ||
				(caseFamily(sym) && !isReferencedIncludedCase(sym)) || caseRole || r.contentKind(sym),
			r.drawsConnector(sym) || isTransition(sym) || sym.Kind == symbols.SymbolSuccessionUsage || isFlowUsage(sym) ||
				isIncludedCase(sym) || (caseFamily(sym) && hasCaseOwner(sym)) || caseRole || r.mixedReferenceEdge(sym)
	}
	return false, false
}

func hasCaseOwner(sym *symbols.Symbol) bool {
	for owner := sym.Owner(); owner != nil; owner = owner.Owner() {
		if caseFamily(owner) {
			return true
		}
	}
	return false
}

func (r *Renderer) mixedReferenceEdge(sym *symbols.Symbol) bool {
	if sym.Kind.IsFeature() {
		for _, target := range r.model.DeclaredTypes(sym) {
			if target.Kind.IsDefinition() {
				return true
			}
		}
	}
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel != nil && rel.Kind == ast.RelSpecializes && r.model.RelationshipTarget(sym, rel) != nil {
			return true
		}
	}
	if usage := caseUsage(sym); usage != nil && (usage.IsPerformedAction() || usage.IsExhibitedState()) {
		if r.model.ReferencedFeature(sym) != nil {
			return true
		}
		for _, rel := range semantics.RelationshipsOf(sym) {
			if rel == nil || rel.Kind != ast.RelTyping && rel.Kind != ast.RelReferences {
				continue
			}
			if r.model.RelationshipTarget(sym, rel) != nil {
				return true
			}
		}
	}
	return false
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
