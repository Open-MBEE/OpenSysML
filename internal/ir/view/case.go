package view

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// renderCase walks exposed elements and writes case nodes, roles and relationships.
func (r *Renderer) renderCase(view *symbols.Symbol, exposed []*symbols.Symbol, out *Rendering) {
	w := &caseWalk{r: r, view: view, ids: &nodeIDs{}, drawn: map[*symbols.Symbol]*Node{}, out: out}
	for _, elem := range exposed {
		if !w.collect(elem, true, nil, map[*symbols.Symbol]bool{}) {
			out.Notices = append(out.Notices, fmt.Sprintf("%s %s holds no case; a case rendering does not show it",
				declKind(elem), r.notationName(elem)))
		}
	}
}

// caseWalk tracks case nodes and their relationships during one rendering.
type caseWalk struct {
	r             *Renderer
	view          *symbols.Symbol
	ids           *nodeIDs
	drawn         map[*symbols.Symbol]*Node
	order         []*symbols.Symbol
	out           *Rendering
	container     *Node
	deferIncludes bool
	includes      []pendingCaseInclude
}

// pendingCaseInclude records a target and edge for resolution after traversal.
type pendingCaseInclude struct {
	from    *Node
	target  *symbols.Symbol
	include *symbols.Symbol
}

// collect descends transparent containers and renders case-family members.
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

// render writes one case node, its roles, includes and nested cases.
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
			if w.deferIncludes {
				w.includes = append(w.includes, pendingCaseInclude{from: owner, target: target, include: sym})
				return nil
			}
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

	type rolePresentation struct {
		kind     string
		edgeKind EdgeKind
		label    string
	}
	ownedRoles := map[*symbols.Symbol]rolePresentation{}
	actorsOwned, actorsInherited := w.r.model.ActorsOf(sym)
	subjectsOwned, subjectsInherited := w.r.model.SubjectsOf(sym)
	objectivesOwned, objectivesInherited := w.r.model.ObjectivesOf(sym)
	for _, role := range actorsOwned {
		ownedRoles[role] = rolePresentation{kind: "actor", edgeKind: EdgeAssociation}
	}
	for _, role := range subjectsOwned {
		ownedRoles[role] = rolePresentation{kind: "subject", edgeKind: EdgeAssociation, label: "«subject»"}
	}
	for _, role := range objectivesOwned {
		ownedRoles[role] = rolePresentation{kind: "objective", edgeKind: EdgeAnchor}
	}
	drawRole := func(role *symbols.Symbol, presentation rolePresentation) {
		if presentation.kind == "objective" {
			w.objectiveNode(sym, node, role)
		} else {
			w.roleNode(sym, node, role, presentation.kind, presentation.edgeKind, presentation.label)
		}
	}

	for _, member := range w.r.containedMembers(sym) {
		if presentation, ok := ownedRoles[member]; ok {
			drawRole(member, presentation)
			continue
		}
		switch {
		case semantics.IsActorUsage(member) || semantics.IsSubjectUsage(member) || semantics.IsObjectiveUsage(member):
			continue
		case caseFamily(member):
			w.render(member, false, node)
		default:
			w.collect(member, false, node, map[*symbols.Symbol]bool{sym: true})
		}
	}
	for _, role := range actorsInherited {
		if w.libraryInheritedRole(role) {
			continue
		}
		drawRole(role, rolePresentation{kind: "actor", edgeKind: EdgeAssociation})
	}
	for _, role := range subjectsInherited {
		if w.libraryInheritedRole(role) {
			continue
		}
		drawRole(role, rolePresentation{kind: "subject", edgeKind: EdgeAssociation, label: "«subject»"})
	}
	for _, role := range objectivesInherited {
		if w.libraryInheritedRole(role) {
			continue
		}
		drawRole(role, rolePresentation{kind: "objective", edgeKind: EdgeAnchor})
	}
	return node
}

// libraryInheritedRole reports whether sym is declared in bundled library content.
func (w *caseWalk) libraryInheritedRole(sym *symbols.Symbol) bool {
	return w.r.resolver != nil && w.r.resolver.Index().IsLibraryDocument(sym.DocName)
}

// resolveIncludedCases adds include edges after their targets have been placed.
func (w *caseWalk) resolveIncludedCases() {
	for i := 0; i < len(w.includes); i++ {
		include := w.includes[i]
		target := w.drawn[include.target]
		if target == nil {
			w.render(include.target, true, nil)
			target = w.drawn[include.target]
		}
		if include.from != nil && target != nil {
			w.edge(include.from, target, EdgeInclude, "«include»", include.include)
		}
	}
	w.includes = nil
}

// roleNode adds an actor or subject node and its association.
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

// objectiveNode adds an objective, its documentation and anchor edge.
func (w *caseWalk) objectiveNode(owner *symbols.Symbol, caseNode *Node, sym *symbols.Symbol) {
	detail := strings.Join(w.r.documentation(sym), "\n")
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

// append attaches a case node to its current container or rendering roots.
func (w *caseWalk) append(node *Node) {
	if w.container != nil {
		w.container.Children = append(w.container.Children, node)
		return
	}
	w.out.Roots = append(w.out.Roots, node)
}

// edge records a relationship between rendered nodes.
func (w *caseWalk) edge(from, to *Node, kind EdgeKind, label string, sym *symbols.Symbol) {
	w.edgeIDs(sym, from.ID, to.ID, kind, label)
}

// edgeIDs appends an edge and records its source symbol for layout.
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

// caseNotation reports whether a rendering of kind draws its typing, reference
// and composition edges in the case diagram's notation rather than a graph's.
func caseNotation(kind Kind) bool {
	return kind == KindCase || kind == KindMixed
}

// caseFamily reports whether sym belongs to the case definition and usage families.
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

// isIncludedCase reports whether sym is an included-use-case usage.
func isIncludedCase(sym *symbols.Symbol) bool {
	usage := caseUsage(sym)
	return usage != nil && usage.IsIncludedUseCase()
}

// isInlineIncludedCase reports whether an include declares its own case body.
func isInlineIncludedCase(sym *symbols.Symbol) bool {
	usage := caseUsage(sym)
	return usage != nil && usage.IsIncludedUseCase() && usage.Keyword == "use case" && usage.HasBody
}

// caseUsage returns sym's usage declaration, unwrapping its membership when needed.
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

// viewRelationshipsOf returns declared relationships, unwrapping membership for view traversal.
func viewRelationshipsOf(sym *symbols.Symbol) []*ast.Relationship {
	if sym == nil {
		return nil
	}
	membership, ok := sym.Decl.(*ast.Membership)
	if !ok {
		return semantics.RelationshipsOf(sym)
	}
	member := *sym
	member.Decl = membership.Member
	return semantics.RelationshipsOf(&member)
}

// documentation reads doc bodies through this renderer's source lookup.
func (r *Renderer) documentation(sym *symbols.Symbol) []string {
	if sym == nil || sym.Scope == nil || r.text == nil {
		return nil
	}
	if target, ok := r.resolver.ResolveAliasTarget(sym); ok {
		sym = target
	}
	var bodies []string
	sym.Scope.ForEachMember(func(member *symbols.Symbol) bool {
		if member.Kind != symbols.SymbolDocumentation {
			return true
		}
		doc, ok := member.Decl.(*ast.Documentation)
		if !ok {
			return true
		}
		if body := source.CommentProse(r.text(member.DocName, doc.BodySpan)); body != "" {
			bodies = append(bodies, body)
		}
		return true
	})
	return bodies
}

// includedCaseTarget finds the case referenced by an include usage.
func (w *caseWalk) includedCaseTarget(sym *symbols.Symbol) *symbols.Symbol {
	for _, rel := range viewRelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelIncludes {
			continue
		}
		if target := w.r.model.RelationshipTarget(sym, rel); caseFamily(target) {
			return target
		}
	}
	return nil
}

// isReferencedIncludedCase reports includes that point at an existing case.
func isReferencedIncludedCase(sym *symbols.Symbol) bool {
	return isIncludedCase(sym) && !isInlineIncludedCase(sym)
}

// caseNodeKind reports whether a notation kind names a case-family member.
func caseNodeKind(kind string) bool {
	return strings.Contains(kind, "case") || strings.HasPrefix(kind, "analysis ") || strings.HasPrefix(kind, "verification ")
}
