package view

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// renderMixed combines structural, behavioral, case and tree nodes on one canvas.
func (r *Renderer) renderMixed(view *symbols.Symbol, exposed []*symbols.Symbol, out *Rendering) {
	w := &mixedWalk{r: r, view: view, ids: &nodeIDs{}, out: out,
		nodes: map[*symbols.Symbol]*Node{}, seen: map[*symbols.Symbol]bool{},
		cases:           &caseWalk{r: r, view: view, ids: nil, drawn: map[*symbols.Symbol]*Node{}, occurrences: map[*symbols.Symbol][]*Node{}, roleCase: map[*Node]*Node{}, out: out, deferIncludes: true},
		structureOwners: map[*symbols.Symbol]bool{}, deferredMembers: map[*symbols.Symbol][]*symbols.Symbol{}}
	w.cases.ids = w.ids
	for _, elem := range exposed {
		w.render(elem, true, nil)
	}
	w.renderStructures()
	w.cases.resolveIncludedCases()
	w.rememberCaseNodes()
	w.referenceEdges()
}

// mixedWalk tracks the nodes and deferred structural work of one mixed rendering.
type mixedWalk struct {
	r               *Renderer
	view            *symbols.Symbol
	ids             *nodeIDs
	out             *Rendering
	nodes           map[*symbols.Symbol]*Node
	order           []*symbols.Symbol
	seen            map[*symbols.Symbol]bool
	cases           *caseWalk
	structures      []*mixedStructure
	structureOwners map[*symbols.Symbol]bool
	structureOrder  []*symbols.Symbol
	deferredMembers map[*symbols.Symbol][]*symbols.Symbol
}

// mixedStructure records a structure queued for the interconnection builder.
type mixedStructure struct {
	sym    *symbols.Symbol
	parent *Node
	index  int
	placed bool
}

// render dispatches one symbol to its mixed-view node builder.
func (w *mixedWalk) render(sym *symbols.Symbol, exposed bool, parent *Node) {
	if sym == nil {
		return
	}
	if mixedStructural(sym) {
		if node := w.nodes[sym]; node != nil {
			w.seen[sym] = true
			w.attachBuiltStructure(node, parent)
			return
		}
	}
	if w.seen[sym] {
		return
	}
	w.seen[sym] = true

	if sym.Kind == symbols.SymbolPackage || sym.Kind == symbols.SymbolNamespace {
		node := w.packageNode(sym, exposed)
		w.remember(sym, node)
		w.append(node, parent)
		for _, member := range w.traversedMembers(sym) {
			w.render(member, false, node)
		}
		return
	}

	if caseFamily(sym) {
		w.cases.container = parent
		w.cases.render(sym, exposed, nil)
		w.cases.container = nil
		w.rememberCaseNodes()
		for _, member := range w.r.containedMembers(sym) {
			if semantics.IsActorUsage(member) || semantics.IsSubjectUsage(member) || semantics.IsObjectiveUsage(member) {
				w.seen[member] = true
			} else if isReferencedIncludedCase(member) {
				w.seen[member] = true
			}
		}
		for _, member := range w.traversedMembers(sym) {
			w.render(member, false, parent)
		}
		return
	}

	if stateLike(sym) {
		if node := w.r.renderState(w.view, sym, w.ids, w.out, w.remember); node != nil {
			w.remember(sym, node)
			w.append(node, parent)
		}
		return
	}
	if sym.Kind == symbols.SymbolActionDef || sym.Kind == symbols.SymbolActionUsage {
		if node, ok := w.r.renderAction(w.view, sym, w.ids, w.out, w.remember); ok {
			w.remember(sym, node)
			w.append(node, parent)
		}
		return
	}
	if w.r.drawsConnector(sym) || mixedStructural(sym) {
		w.queueStructure(sym, parent)
		return
	}

	node := w.r.treeNodeShallow(w.view, sym, w.ids, map[*symbols.Symbol]bool{}, 0, exposed, w.out)
	w.remember(sym, node)
	w.append(node, parent)
	for _, member := range w.traversedMembers(sym) {
		w.render(member, false, node)
	}
}

// traversedMembers lists the members mixedWalk.render visits under sym.
func (w *mixedWalk) traversedMembers(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil || stateLike(sym) || sym.Kind == symbols.SymbolActionDef || sym.Kind == symbols.SymbolActionUsage {
		return nil
	}
	members := w.r.containedMembers(sym)
	if !caseFamily(sym) {
		return members
	}
	traversed := make([]*symbols.Symbol, 0, len(members))
	for _, member := range members {
		if semantics.IsActorUsage(member) || semantics.IsSubjectUsage(member) ||
			semantics.IsObjectiveUsage(member) || isReferencedIncludedCase(member) {
			continue
		}
		traversed = append(traversed, member)
	}
	return traversed
}

// packageNode creates the shared package cluster for a mixed view.
func (w *mixedWalk) packageNode(sym *symbols.Symbol, exposed bool) *Node {
	name := localName(sym)
	if exposed {
		name = w.r.notationName(sym)
	}
	node := &Node{ID: w.ids.take(), Kind: declKind(sym), Name: name,
		NameSynthesized: w.r.model.NameSynthesized(sym), Type: declType(sym),
		Typings: w.r.declTypings(sym), Origin: symbolOrigin(sym),
		Geometry: w.r.geometryOf(w.view, sym, w.out)}
	w.r.dress(w.view, sym, node, w.out)
	return node
}

// queueStructure defers a feature subtree to the interconnection builder.
func (w *mixedWalk) queueStructure(sym *symbols.Symbol, parent *Node) {
	entry := &mixedStructure{sym: sym, parent: parent}
	if !w.r.drawsConnector(sym) {
		entry.placed = true
		if parent == nil {
			entry.index = len(w.out.Roots)
			w.out.Roots = append(w.out.Roots, nil)
		} else {
			entry.index = len(parent.Children)
			parent.Children = append(parent.Children, nil)
		}
	}
	w.structures = append(w.structures, entry)
	w.walkStructureMembers(sym, parent)
}

// walkStructureMembers collects structural descendants into the shared node map.
func (w *mixedWalk) walkStructureMembers(sym *symbols.Symbol, parent *Node) {
	if !w.structureOwners[sym] {
		w.structureOwners[sym] = true
		w.structureOrder = append(w.structureOrder, sym)
	}
	for _, member := range w.r.containedMembers(sym) {
		if w.seen[member] {
			continue
		}
		switch {
		case w.r.drawsConnector(member):
			w.seen[member] = true
		case mixedStructural(member):
			w.seen[member] = true
			w.walkStructureMembers(member, parent)
		default:
			w.deferredMembers[sym] = append(w.deferredMembers[sym], member)
		}
	}
}

// discoverDeferredStructureMembers finds structures and connectors behind deferred members.
func (w *mixedWalk) discoverDeferredStructureMembers() {
	queued := map[*symbols.Symbol]bool{}
	for _, entry := range w.structures {
		queued[entry.sym] = true
	}
	visited := map[*symbols.Symbol]bool{}
	var discover func(*symbols.Symbol)
	discover = func(sym *symbols.Symbol) {
		if sym == nil || visited[sym] {
			return
		}
		visited[sym] = true
		switch {
		case w.r.drawsConnector(sym):
			if !queued[sym] {
				w.structures = append(w.structures, &mixedStructure{sym: sym})
				queued[sym] = true
			}
			w.seen[sym] = true
		case mixedStructural(sym):
			if !w.structureOwners[sym] {
				if !queued[sym] {
					w.structures = append(w.structures, &mixedStructure{sym: sym})
					queued[sym] = true
				}
				w.walkStructureMembers(sym, nil)
			}
			for _, member := range w.deferredMembers[sym] {
				discover(member)
			}
		default:
			for _, member := range w.traversedMembers(sym) {
				discover(member)
			}
		}
	}
	for i := 0; i < len(w.structureOrder); i++ {
		for _, member := range w.deferredMembers[w.structureOrder[i]] {
			discover(member)
		}
	}
}

// renderStructures builds all queued feature subtrees in one shared-ID pass.
func (w *mixedWalk) renderStructures() {
	if len(w.structures) == 0 {
		return
	}
	w.discoverDeferredStructureMembers()
	exposed := make([]*symbols.Symbol, len(w.structures))
	for i, entry := range w.structures {
		exposed[i] = entry.sym
	}
	temp := &Rendering{drawn: w.out.drawn}
	members := w.r.renderInterconnectionWithIDs(w.view, exposed, temp, w.ids, true)
	var index func(*symbols.Symbol)
	index = func(sym *symbols.Symbol) {
		node := members[sym]
		if node == nil || w.nodes[sym] != nil {
			return
		}
		w.remember(sym, node)
		w.seen[sym] = true
		for _, member := range w.r.containedMembers(sym) {
			index(member)
		}
	}
	for _, entry := range w.structures {
		index(entry.sym)
	}
	w.renderDeferredStructureMembers(members, w.structureOrder)
	roots := map[*Node]bool{}
	for _, root := range temp.Roots {
		roots[root] = true
	}
	for _, entry := range w.structures {
		node := members[entry.sym]
		if !entry.placed || node == nil || !roots[node] {
			continue
		}
		if entry.parent == nil {
			w.out.Roots[entry.index] = node
		} else {
			entry.parent.Children[entry.index] = node
		}
	}
	w.removeStructuralSlots()
	w.out.Edges = append(w.out.Edges, temp.Edges...)
	w.out.Notes = append(w.out.Notes, temp.Notes...)
	w.out.Notices = append(w.out.Notices, temp.Notices...)
}

// renderDeferredStructureMembers attaches members after their structural owners are indexed.
func (w *mixedWalk) renderDeferredStructureMembers(members map[*symbols.Symbol]*Node, owners []*symbols.Symbol) {
	for _, sym := range owners {
		owner := members[sym]
		if owner == nil {
			continue
		}
		for _, member := range w.deferredMembers[sym] {
			w.render(member, false, owner)
		}
		existingChildren := append([]*Node(nil), owner.Children...)
		var children []*Node
		directMembers := map[*Node]bool{}
		for _, member := range w.r.containedMembers(sym) {
			if w.r.drawsConnector(member) {
				continue
			}
			node := members[member]
			if node == nil {
				node = w.nodes[member]
			}
			if node != nil && !directMembers[node] {
				children = append(children, node)
				directMembers[node] = true
			}
		}
		for _, child := range existingChildren {
			if !directMembers[child] {
				children = append(children, child)
			}
		}
		owner.Children = children
	}
}

// removeStructuralSlots replaces temporary structural placeholders with built nodes.
func (w *mixedWalk) removeStructuralSlots() {
	compact := func(nodes []*Node) []*Node {
		out := nodes[:0]
		for _, node := range nodes {
			if node != nil {
				out = append(out, node)
			}
		}
		return out
	}
	var walk func(*Node)
	walk = func(node *Node) {
		node.Children = compact(node.Children)
		for _, child := range node.Children {
			walk(child)
		}
	}
	w.out.Roots = compact(w.out.Roots)
	for _, root := range w.out.Roots {
		walk(root)
	}
}

// mixedStructural reports symbols rendered through the interconnection builder.
func mixedStructural(sym *symbols.Symbol) bool {
	switch sym.Kind {
	case symbols.SymbolPartDef, symbols.SymbolPartUsage, symbols.SymbolItemDef, symbols.SymbolItemUsage,
		symbols.SymbolPortDef, symbols.SymbolPortUsage, symbols.SymbolConnectionDef, symbols.SymbolConnectionUsage,
		symbols.SymbolInterfaceDef, symbols.SymbolInterfaceUsage, symbols.SymbolAllocationDef, symbols.SymbolAllocationUsage:
		return true
	}
	return false
}

// append attaches a shared node beneath its parent or to the rendering roots.
func (w *mixedWalk) append(node, parent *Node) {
	if parent == nil {
		w.out.Roots = append(w.out.Roots, node)
		return
	}
	parent.Children = append(parent.Children, node)
}

// attachBuiltStructure places an indexed feature without duplicating it.
func (w *mixedWalk) attachBuiltStructure(node, parent *Node) {
	siblings := w.out.Roots
	if parent != nil {
		siblings = parent.Children
	}
	for _, sibling := range siblings {
		if sibling == node {
			return
		}
	}
	w.append(node, parent)
}

// remember indexes one semantic symbol and its shared node.
func (w *mixedWalk) remember(sym *symbols.Symbol, node *Node) {
	if sym == nil || node == nil || w.nodes[sym] != nil {
		return
	}
	w.nodes[sym] = node
	w.order = append(w.order, sym)
}

// rememberCaseNodes indexes case-specific nodes over generic feature nodes.
func (w *mixedWalk) rememberCaseNodes() {
	for _, sym := range w.cases.order {
		node := w.cases.drawn[sym]
		if node == nil {
			continue
		}
		if w.nodes[sym] == nil {
			w.remember(sym, node)
		} else {
			w.nodes[sym] = node
		}
	}
}

func (w *mixedWalk) nodesFor(sym *symbols.Symbol) []*Node {
	if sym == nil {
		return nil
	}
	if occurrences := w.cases.occurrences[sym]; len(occurrences) > 0 {
		return occurrences
	}
	if node := w.nodes[sym]; node != nil {
		return []*Node{node}
	}
	return nil
}

func referenceTargetNode(sym *symbols.Symbol) ast.Node {
	if usage := caseUsage(sym); usage != nil {
		if reference := usage.ReferenceSubsetting(); reference != nil {
			return reference.Target
		}
	}
	return nil
}

func referenceQualifiedName(target ast.Node) *ast.QualifiedName {
	switch target := target.(type) {
	case *ast.QualifiedName:
		return target
	case *ast.FeatureReference:
		return target.Name
	}
	return nil
}

func (w *mixedWalk) qualifyingReferenceCase(from, to *symbols.Symbol, reference ast.Node) *Node {
	qn := referenceQualifiedName(reference)
	if from == nil || qn == nil || len(qn.Parts) < 2 || w.r.resolver == nil {
		return nil
	}
	prefix := &ast.QualifiedName{Global: qn.Global, Parts: qn.Parts[:len(qn.Parts)-1]}
	sym, ok := w.r.resolver.ResolveQualified(from.OwnerScope, prefix)
	if !ok || !caseFamily(sym) {
		return nil
	}
	caseNode := w.cases.drawn[sym]
	if caseNode == nil {
		return nil
	}
	for _, occurrence := range w.cases.occurrences[to] {
		if w.cases.roleCase[occurrence] == caseNode {
			return caseNode
		}
	}
	return nil
}

// referenceEdges adds typing, specialization, perform and exhibit links between drawn nodes.
func (w *mixedWalk) referenceEdges() {
	seen := map[string]bool{}
	add := func(from, to *symbols.Symbol, kind EdgeKind, label string, decl *symbols.Symbol, reference ast.Node) {
		sources, targets := w.nodesFor(from), w.nodesFor(to)
		if len(sources) == 0 || len(targets) == 0 {
			return
		}
		perCaseTargets := 0
		for _, target := range targets {
			if w.cases.roleCase[target] != nil {
				perCaseTargets++
			}
		}
		var qualifier *Node
		if perCaseTargets > 1 {
			qualifier = w.qualifyingReferenceCase(from, to, reference)
		}
		drawnSources := map[string]bool{}
		emitted := false
		for _, source := range sources {
			if source == nil || drawnSources[source.ID] {
				continue
			}
			drawnSources[source.ID] = true
			targetsForSource := targets
			if perCaseTargets <= 1 {
				targetsForSource = []*Node{w.nodes[to]}
			} else {
				owner := w.cases.roleCase[source]
				if owner == nil {
					owner = qualifier
				}
				if owner != nil {
					targetsForSource = nil
					for _, target := range targets {
						if w.cases.roleCase[target] == owner {
							targetsForSource = append(targetsForSource, target)
						}
					}
				}
			}
			for _, target := range targetsForSource {
				if target == nil {
					continue
				}
				key := source.ID + "\x00" + target.ID + "\x00" + kind.String() + "\x00" + label
				if seen[key] {
					continue
				}
				seen[key] = true
				w.out.Edges = append(w.out.Edges, Edge{From: source.ID, To: target.ID, Kind: kind,
					Label: label, Origin: symbolOrigin(decl), Route: w.r.routeOf(w.view, decl, w.out),
					Style: w.r.edgeDress(w.view, decl, source.ID, target.ID, w.out)})
				emitted = true
			}
		}
		if emitted && w.out.drawn != nil {
			w.out.drawn.note(decl, true)
		}
	}

	for _, sym := range w.order {
		specialLabel := ""
		if usage := caseUsage(sym); usage != nil {
			switch {
			case usage.IsPerformedAction():
				specialLabel = "«perform»"
			case usage.IsExhibitedState():
				specialLabel = "«exhibit»"
			}
		}
		if sym.Kind.IsFeature() && specialLabel == "" {
			for _, target := range w.r.model.DeclaredTypes(sym) {
				if target.Kind.IsDefinition() {
					add(sym, target, EdgeTyping, "", sym, nil)
				}
			}
		}
		for _, rel := range viewRelationshipsOf(sym) {
			if rel == nil || rel.Kind != ast.RelSpecializes {
				continue
			}
			if target := w.r.model.RelationshipTarget(sym, rel); target != nil {
				add(sym, target, EdgeSpecialization, "«specializes»", sym, nil)
			}
		}
		if specialLabel != "" {
			target := w.r.model.ReferencedFeature(sym)
			if target != nil {
				add(sym, target, EdgeReference, specialLabel, sym, referenceTargetNode(sym))
				continue
			}
			for _, rel := range viewRelationshipsOf(sym) {
				if rel == nil || rel.Kind != ast.RelTyping && rel.Kind != ast.RelReferences {
					continue
				}
				if target := w.r.model.RelationshipTarget(sym, rel); target != nil {
					add(sym, target, EdgeReference, specialLabel, sym, rel.Target)
					break
				}
			}
		}
	}
}
