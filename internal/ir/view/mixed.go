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
		cases:           &caseWalk{r: r, view: view, ids: nil, drawn: map[*symbols.Symbol]*Node{}, out: out, deferIncludes: true},
		structureOwners: map[*symbols.Symbol]bool{}, deferredMembers: map[*symbols.Symbol][]*symbols.Symbol{}}
	w.cases.ids = w.ids
	for _, elem := range exposed {
		w.render(elem, true, nil)
	}
	w.renderStructures()
	w.cases.resolveIncludedCases()
	for _, caseSym := range w.cases.order {
		w.remember(caseSym, w.cases.drawn[caseSym])
	}
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
	if sym == nil || w.seen[sym] {
		return
	}
	w.seen[sym] = true

	if sym.Kind == symbols.SymbolPackage || sym.Kind == symbols.SymbolNamespace {
		node := w.packageNode(sym, exposed)
		w.remember(sym, node)
		w.append(node, parent)
		for _, member := range w.r.containedMembers(sym) {
			w.render(member, false, node)
		}
		return
	}

	if caseFamily(sym) {
		w.cases.container = parent
		w.cases.render(sym, exposed, nil)
		w.cases.container = nil
		for _, caseSym := range w.cases.order {
			w.remember(caseSym, w.cases.drawn[caseSym])
		}
		for _, member := range w.r.containedMembers(sym) {
			if semantics.IsActorUsage(member) || semantics.IsSubjectUsage(member) || semantics.IsObjectiveUsage(member) {
				w.seen[member] = true
				continue
			}
			if isReferencedIncludedCase(member) {
				w.seen[member] = true
				continue
			}
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
	for _, member := range w.r.containedMembers(sym) {
		w.render(member, false, node)
	}
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

// renderStructures builds queued feature subtrees with the shared node IDs.
func (w *mixedWalk) renderStructures() {
	if len(w.structures) == 0 {
		return
	}
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
	w.renderDeferredStructureMembers(members)
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
func (w *mixedWalk) renderDeferredStructureMembers(members map[*symbols.Symbol]*Node) {
	for _, sym := range w.structureOrder {
		owner := members[sym]
		if owner == nil {
			continue
		}
		for _, member := range w.deferredMembers[sym] {
			w.render(member, false, owner)
		}
		var children []*Node
		for _, member := range w.r.containedMembers(sym) {
			if w.r.drawsConnector(member) {
				continue
			}
			node := members[member]
			if node == nil {
				node = w.nodes[member]
			}
			if node != nil {
				children = append(children, node)
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

// remember indexes one semantic symbol and its shared node.
func (w *mixedWalk) remember(sym *symbols.Symbol, node *Node) {
	if sym == nil || node == nil || w.nodes[sym] != nil {
		return
	}
	w.nodes[sym] = node
	w.order = append(w.order, sym)
}

// referenceEdges adds typing, specialization, perform and exhibit links between drawn nodes.
func (w *mixedWalk) referenceEdges() {
	seen := map[string]bool{}
	add := func(from, to *symbols.Symbol, kind EdgeKind, label string, decl *symbols.Symbol) {
		fromNode, toNode := w.nodes[from], w.nodes[to]
		if fromNode == nil || toNode == nil {
			return
		}
		key := fromNode.ID + "\x00" + toNode.ID + "\x00" + kind.String() + "\x00" + label
		if seen[key] {
			return
		}
		seen[key] = true
		w.out.Edges = append(w.out.Edges, Edge{From: fromNode.ID, To: toNode.ID, Kind: kind,
			Label: label, Origin: symbolOrigin(decl), Route: w.r.routeOf(w.view, decl, w.out),
			Style: w.r.edgeDress(w.view, decl, fromNode.ID, toNode.ID, w.out)})
		if w.out.drawn != nil {
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
					add(sym, target, EdgeTyping, "", sym)
				}
			}
		}
		for _, rel := range viewRelationshipsOf(sym) {
			if rel == nil || rel.Kind != ast.RelSpecializes {
				continue
			}
			if target := w.r.model.RelationshipTarget(sym, rel); target != nil {
				add(sym, target, EdgeSpecialization, "«specializes»", sym)
			}
		}
		if specialLabel != "" {
			target := w.r.model.ReferencedFeature(sym)
			if target != nil {
				add(sym, target, EdgeReference, specialLabel, sym)
				continue
			}
			for _, rel := range viewRelationshipsOf(sym) {
				if rel == nil || rel.Kind != ast.RelTyping && rel.Kind != ast.RelReferences {
					continue
				}
				if target := w.r.model.RelationshipTarget(sym, rel); target != nil {
					add(sym, target, EdgeReference, specialLabel, sym)
					break
				}
			}
		}
	}
}
