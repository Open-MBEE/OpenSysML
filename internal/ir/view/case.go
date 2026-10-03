package view

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func (r *Renderer) renderCase(view *symbols.Symbol, exposed []*symbols.Symbol, out *Rendering) {
	w := &caseWalk{r: r, view: view, ids: &nodeIDs{}, drawn: map[*symbols.Symbol]*Node{}, out: out}
	for _, elem := range exposed {
		if !w.collect(elem, true, nil, map[*symbols.Symbol]bool{}) {
			out.Notices = append(out.Notices, fmt.Sprintf("%s %s holds no case; a case rendering does not show it",
				declKind(elem), r.notationName(elem)))
		}
	}
}

type caseWalk struct {
	r         *Renderer
	view      *symbols.Symbol
	ids       *nodeIDs
	drawn     map[*symbols.Symbol]*Node
	order     []*symbols.Symbol
	out       *Rendering
	container *Node
}

func (w *caseWalk) collect(sym *symbols.Symbol, exposed bool, owner *Node, seen map[*symbols.Symbol]bool) bool {
	if sym == nil || seen[sym] {
		return false
	}
	seen[sym] = true
	if caseFamily(sym) {
		w.render(sym, exposed, owner)
		return true
	}
	found := false
	for _, member := range w.r.containedMembers(sym) {
		found = w.collect(member, false, owner, seen) || found
	}
	return found
}

func (w *caseWalk) render(sym *symbols.Symbol, exposed bool, owner *Node) *Node {
	if existing := w.drawn[sym]; existing != nil {
		if owner != nil {
			kind, label := EdgeComposition, ""
			if isInlineIncludedCase(sym) {
				kind, label = EdgeInclude, "«include»"
			}
			w.edge(owner, existing, kind, label, sym)
		}
		return existing
	}
	if isReferencedIncludedCase(sym) {
		if target := w.includedCaseTarget(sym); target != nil {
			node := w.render(target, true, nil)
			if owner != nil && node != nil {
				w.edge(owner, node, EdgeInclude, "«include»", sym)
			}
			return node
		}
	}

	name := localName(sym)
	if exposed {
		name = w.r.notationName(sym)
	}
	node := &Node{ID: w.ids.take(), Kind: declKind(sym), Name: name,
		NameSynthesized: w.r.model.NameSynthesized(sym), Type: declType(sym),
		Typings: w.r.declTypings(sym), Origin: symbolOrigin(sym),
		Geometry: w.r.geometryOf(w.view, sym, w.out)}
	w.r.dress(w.view, sym, node, w.out)
	w.drawn[sym] = node
	w.order = append(w.order, sym)
	w.append(node)
	if w.out.drawn != nil {
		w.out.drawn.note(sym, false)
	}
	if owner != nil {
		kind, label := EdgeComposition, ""
		if isInlineIncludedCase(sym) {
			kind, label = EdgeInclude, "«include»"
		}
		w.edge(owner, node, kind, label, sym)
	}

	for _, member := range w.r.containedMembers(sym) {
		switch {
		case semantics.IsActorUsage(member):
			w.roleNode(sym, node, member, "actor", EdgeAssociation, "")
		case semantics.IsSubjectUsage(member):
			w.roleNode(sym, node, member, "subject", EdgeAssociation, "«subject»")
		case semantics.IsObjectiveUsage(member):
			w.objectiveNode(sym, node, member)
		case caseFamily(member):
			w.render(member, false, node)
		default:
			w.collect(member, false, node, map[*symbols.Symbol]bool{sym: true})
		}
	}
	return node
}

func (w *caseWalk) roleNode(owner *symbols.Symbol, caseNode *Node, sym *symbols.Symbol, kind string, edgeKind EdgeKind, label string) {
	node := &Node{ID: w.ids.take(), Kind: kind, Name: localName(sym), NameSynthesized: w.r.model.NameSynthesized(sym),
		Type: declType(sym), Typings: w.r.declTypings(sym), Origin: symbolOrigin(sym),
		Geometry: w.r.geometryOf(w.view, sym, w.out)}
	w.r.dress(w.view, sym, node, w.out)
	w.append(node)
	w.drawn[sym] = node
	w.order = append(w.order, sym)
	if w.out.drawn != nil {
		w.out.drawn.noteMember(owner, sym)
	}
	from, to := caseNode.ID, node.ID
	if kind == "actor" {
		from, to = node.ID, caseNode.ID
	}
	w.edgeIDs(sym, from, to, edgeKind, label)
}

func (w *caseWalk) objectiveNode(owner *symbols.Symbol, caseNode *Node, sym *symbols.Symbol) {
	detail := strings.Join(w.r.model.DocumentationOf(sym), "\n")
	node := &Node{ID: w.ids.take(), Kind: "objective", Name: localName(sym), NameSynthesized: w.r.model.NameSynthesized(sym),
		Type: declType(sym), Detail: detail, Typings: w.r.declTypings(sym), Origin: symbolOrigin(sym),
		Geometry: w.r.geometryOf(w.view, sym, w.out)}
	w.r.dress(w.view, sym, node, w.out)
	w.append(node)
	w.drawn[sym] = node
	w.order = append(w.order, sym)
	if w.out.drawn != nil {
		w.out.drawn.noteMember(owner, sym)
	}
	w.edgeIDs(sym, caseNode.ID, node.ID, EdgeAnchor, "")
}

func (w *caseWalk) append(node *Node) {
	if w.container != nil {
		w.container.Children = append(w.container.Children, node)
		return
	}
	w.out.Roots = append(w.out.Roots, node)
}

func (w *caseWalk) edge(from, to *Node, kind EdgeKind, label string, sym *symbols.Symbol) {
	w.edgeIDs(sym, from.ID, to.ID, kind, label)
}

func (w *caseWalk) edgeIDs(sym *symbols.Symbol, from, to string, kind EdgeKind, label string) {
	w.out.Edges = append(w.out.Edges, Edge{
		From: from, To: to, Kind: kind, Label: label, Origin: symbolOrigin(sym),
		Route: w.r.routeOf(w.view, sym, w.out),
		Style: w.r.edgeDress(w.view, sym, from, to, w.out),
	})
	if w.out.drawn != nil {
		w.out.drawn.note(sym, true)
	}
}

func caseFamily(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	switch sym.Kind {
	case symbols.SymbolCaseDef, symbols.SymbolCaseUsage,
		symbols.SymbolAnalysisCaseDef, symbols.SymbolAnalysisCaseUsage,
		symbols.SymbolVerificationCaseDef, symbols.SymbolVerificationCaseUsage,
		symbols.SymbolUseCaseDef, symbols.SymbolUseCaseUsage:
		return true
	}
	return false
}

func isIncludedCase(sym *symbols.Symbol) bool {
	usage := caseUsage(sym)
	return usage != nil && usage.IsIncludedUseCase()
}

func isInlineIncludedCase(sym *symbols.Symbol) bool {
	usage := caseUsage(sym)
	return usage != nil && usage.IsIncludedUseCase() && usage.Keyword == "use case" && usage.HasBody
}

func caseUsage(sym *symbols.Symbol) *ast.Usage {
	if sym == nil {
		return nil
	}
	decl := sym.Decl
	if membership, ok := decl.(*ast.Membership); ok {
		decl = membership.Member
	}
	usage, _ := decl.(*ast.Usage)
	return usage
}

func (w *caseWalk) includedCaseTarget(sym *symbols.Symbol) *symbols.Symbol {
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelIncludes {
			continue
		}
		if target := w.r.model.RelationshipTarget(sym, rel); caseFamily(target) {
			return target
		}
	}
	return nil
}

func isReferencedIncludedCase(sym *symbols.Symbol) bool {
	return isIncludedCase(sym) && !isInlineIncludedCase(sym)
}

func caseNodeKind(kind string) bool {
	return strings.Contains(kind, "case") || strings.HasPrefix(kind, "analysis ") || strings.HasPrefix(kind, "verification ")
}
