package view

import (
	"fmt"
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A locator finds the render node or edge drawing a vertex of a graph lowered
// separately, by declaration-relative position rather than identity.

// locatorKey is a node's path within the drawn root: kind, declaration place and
// an ordinal among same-place siblings, one segment per level of nesting.
type locatorKey = string

// edgeKey is how an edge is found: where it was written and the nodes it joins.
type edgeKey struct {
	at       string
	from, to string
}

// anchors are the declarations a graph took content from, the behavior drawn and
// those it inherits, against which the spans of their documents are measured.
type anchors []Origin

// renderedAnchors are the anchors of a drawn root: itself and what it inherits.
func renderedAnchors(root *Node) anchors {
	return append(anchors{root.Origin}, root.InheritedFrom...)
}

// graphAnchors are the anchors of a graph lowered from sym's declaration.
func graphAnchors(sym *symbols.Symbol, inherited []lower.Inherited) anchors {
	return append(anchors{symbolOrigin(sym)}, inheritedOrigins(inherited)...)
}

// place spells where a span of doc was written: doc and the span's position within
// the innermost anchor of doc enclosing it. An unlocated span spells as 0:0.
func (a anchors) place(doc string, span source.Span) string {
	if span.Len <= 0 {
		return "0:0"
	}
	offset := span.Offset
	if base, ok := a.enclosing(doc, span); ok {
		offset -= base.Offset
	}
	return doc + "@" + strconv.Itoa(offset) + ":" + strconv.Itoa(span.Len)
}

// enclosing is the span of the innermost anchor of doc enclosing span.
func (a anchors) enclosing(doc string, span source.Span) (source.Span, bool) {
	var found source.Span
	ok := false
	for _, anchor := range a {
		if anchor.Doc != doc || !anchor.Span.Contains(span) {
			continue
		}
		if !ok || found.Contains(anchor.Span) {
			found, ok = anchor.Span, true
		}
	}
	return found, ok
}

// childKey is the key of a child of parent drawn as kind at place, numbered
// among the parent's children of the same kind and place so far.
func childKey(parent locatorKey, kind string, place string, seen map[string]int) locatorKey {
	base := parent + "/" + kind + "@" + place
	n := seen[base]
	seen[base]++
	if n > 0 {
		return base + "#" + strconv.Itoa(n)
	}
	return base
}

// renderedKeys keys every node nested in root, the root itself under "", by
// kind and place, or by place alone when kinded is false.
func renderedKeys(root *Node, kinded bool) map[locatorKey]string {
	at := renderedAnchors(root)
	keys := map[locatorKey]string{"": root.ID}
	var walk func(node *Node, parent locatorKey)
	walk = func(node *Node, parent locatorKey) {
		seen := map[string]int{}
		for _, child := range node.Children {
			kind := ""
			if kinded {
				kind = child.Kind
			}
			key := childKey(parent, kind, at.place(child.Origin.Doc, child.Origin.Span), seen)
			keys[key] = child.ID
			walk(child, key)
		}
	}
	walk(root, "")
	return keys
}

// renderedEdges indexes the rendering's edges by where they were written and
// their endpoints, each to the positions it holds in rendering.Edges.
func renderedEdges(rendering *Rendering, root *Node) map[edgeKey][]int {
	at := renderedAnchors(root)
	edges := map[edgeKey][]int{}
	for i, edge := range rendering.Edges {
		key := edgeKey{at.place(edge.Origin.Doc, edge.Origin.Span), edge.From, edge.To}
		edges[key] = append(edges[key], i)
	}
	return edges
}

// RootDrawing is the root of rendering that draws the exposed element sym, and an
// error when the rendering draws no such element.
func RootDrawing(rendering *Rendering, sym *symbols.Symbol) (*Node, error) {
	if rendering == nil || sym == nil {
		return nil, fmt.Errorf("%w: nothing to locate in", ErrNotDrawn)
	}
	// A root drawn from the lowered graph carries no name span, so only the
	// declaration itself is compared.
	want := symbolOrigin(sym)
	for _, root := range rendering.Roots {
		if root.Origin.Doc == want.Doc && root.Origin.Span == want.Span {
			return root, nil
		}
	}
	return nil, fmt.Errorf("%w: %s is not among the elements rendering %s draws", ErrNotDrawn, sym.Name, rendering.View)
}

// ErrNotDrawn reports a behavior a rendering does not draw, so nothing of its
// execution can be located in it.
var ErrNotDrawn = fmt.Errorf("behavior not drawn")

// stateItem names a vertex or a region of a lowered state machine by id.
type stateItem struct {
	vertex program.VertexRef
	region program.RegionID
}

// StateLocator finds the nodes and edges of a rendering that draw the vertices
// and transitions of a lowered state machine.
type StateLocator struct {
	root  string
	at    anchors
	doc   string
	graph *lower.StateGraph
	// keys are the located keys of the graph's vertices and regions, memoized;
	// seen numbers same-span children per parent key as the rendering does.
	// states are the states the rendering draws: graph-only owners are not.
	states map[program.StateID]bool
	keys   map[stateItem]locatorKey
	seen   map[locatorKey]map[string]int
	nodes  map[locatorKey]string
	edges  map[edgeKey][]int
}

// LocateStates matches graph, lowered from machine's declaration, to the root of
// rendering drawing drawn: the same declaration as the rendering's documents have it.
func LocateStates(rendering *Rendering, drawn, machine *symbols.Symbol, graph *lower.StateGraph) (*StateLocator, error) {
	root, err := RootDrawing(rendering, drawn)
	if err != nil {
		return nil, err
	}
	if machine == nil {
		return nil, fmt.Errorf("%w: no declaration was lowered", ErrNotDrawn)
	}
	if graph == nil {
		return nil, fmt.Errorf("%w: %s has no lowered state graph", ErrNotDrawn, machine.Name)
	}
	l := &StateLocator{
		root:   root.ID,
		at:     graphAnchors(machine, graph.Inherited()),
		doc:    machine.DocName,
		graph:  graph,
		states: make(map[program.StateID]bool),
		keys:   make(map[stateItem]locatorKey),
		seen:   make(map[locatorKey]map[string]int),
		nodes:  renderedKeys(root, true),
		edges:  renderedEdges(rendering, root),
	}
	for _, state := range graph.StateIDs() {
		l.states[state] = true
	}
	for _, region := range graph.TopRegionIDs() {
		l.key(stateItem{region: region})
	}
	for _, state := range graph.StateIDs() {
		l.key(stateItem{vertex: program.StateVertex(state)})
		for _, region := range graph.RegionsOf(state) {
			l.key(stateItem{region: region})
		}
	}
	for _, pseudo := range graph.PseudoIDs() {
		l.key(stateItem{vertex: program.PseudoVertex(pseudo)})
	}
	return l, nil
}

// drawn reports whether the rendering draws the state: the machine itself and
// the graph-only owners of a parallel body's usages are not.
func (l *StateLocator) drawn(state program.StateID) bool { return l.states[state] }

// key is the located key of a vertex or region, "" with false for one the
// rendering does not draw, such as a graph-only region owner.
func (l *StateLocator) key(v stateItem) (locatorKey, bool) {
	if key, ok := l.keys[v]; ok {
		return key, key != ""
	}
	var parent stateItem
	var kind, place string
	switch {
	case v.region != 0:
		region := l.graph.Provenance.Region(v.region)
		if region == nil {
			return "", false
		}
		owner := l.graph.RegionOwnerOf(v.region)
		if owner != 0 && !l.drawn(owner) {
			l.keys[v] = ""
			return "", false
		}
		kind, parent.vertex = "region", program.StateVertex(owner)
		place = l.at.place(docOr(l.graph.RegionDoc(v.region), l.doc), region.Span())
	case v.vertex.Kind == program.VertexState:
		id := v.vertex.State()
		state := l.graph.Provenance.State(id)
		if state == nil || !l.drawn(id) {
			l.keys[v] = ""
			return "", false
		}
		kind, parent = "state", l.parentOfState(id)
		if l.graph.Completing(id) {
			kind = "final"
		}
		place = l.at.place(docOr(l.graph.VertexDoc(v.vertex), l.doc), state.Span())
	case v.vertex.Kind == program.VertexPseudostate:
		id := v.vertex.Pseudo()
		pseudo := l.graph.Provenance.Pseudostate(id)
		if pseudo == nil {
			return "", false
		}
		kind = pseudo.Kind.String()
		if owner := l.graph.PseudostateOwnerOf(id); l.drawn(owner) {
			parent.vertex = program.StateVertex(owner)
		}
		place = l.at.place(docOr(l.graph.VertexDoc(v.vertex), l.doc), pseudo.Span())
	default:
		return "", false
	}
	parentKey := locatorKey("")
	if parent != (stateItem{}) {
		if key, ok := l.key(parent); ok {
			parentKey = key
		}
	}
	seen := l.seen[parentKey]
	if seen == nil {
		seen = map[string]int{}
		l.seen[parentKey] = seen
	}
	key := childKey(parentKey, kind, place, seen)
	l.keys[v] = key
	return key, true
}

// place spells where a declaration of the graph was written, as the rendering does.
func (l *StateLocator) place(decl ast.Node) string {
	if decl == nil {
		return l.at.place("", source.Span{})
	}
	return l.at.place(docOf(l.graph, decl, l.doc), decl.Span())
}

// parentOfState is what the rendering nests a state in: its region when that is
// drawn, else its parent state when that is, else the machine itself (the zero item).
func (l *StateLocator) parentOfState(state program.StateID) stateItem {
	if region := l.graph.StateRegion(state); region != 0 {
		if owner := l.graph.RegionOwnerOf(region); owner == 0 || l.drawn(owner) {
			return stateItem{region: region}
		}
	}
	if owner := l.graph.ParentOf(state); l.drawn(owner) {
		return stateItem{vertex: program.StateVertex(owner)}
	}
	return stateItem{}
}

// Root is the ID of the node drawing the machine itself.
func (l *StateLocator) Root() string { return l.root }

// item names a declaration of the graph by id, the zero item for any other.
func (l *StateLocator) item(decl ast.Node) stateItem {
	if region, ok := decl.(*ast.StateRegion); ok {
		return stateItem{region: l.graph.RegionID[region]}
	}
	return stateItem{vertex: l.graph.VertexOf(decl)}
}

// located is the ID of the node drawing a vertex or region, false for one not drawn.
func (l *StateLocator) located(v stateItem) (string, bool) {
	key, ok := l.key(v)
	if !ok {
		return "", false
	}
	id, ok := l.nodes[key]
	return id, ok
}

// Vertex is the ID of the node drawing the state or pseudostate ref names, and
// false for one the rendering does not draw.
func (l *StateLocator) Vertex(ref program.VertexRef) (string, bool) {
	return l.located(stateItem{vertex: ref})
}

// Region is the ID of the node drawing the region, and false when none does.
func (l *StateLocator) Region(id program.RegionID) (string, bool) {
	return l.located(stateItem{region: id})
}

// Node is the ID of the node drawing vertex — a state, region or pseudostate of
// the graph — and false for one the rendering does not draw.
func (l *StateLocator) Node(vertex ast.Node) (string, bool) {
	return l.located(l.item(vertex))
}

// start is the ID of the start marker of the body owner names, the zero item
// for the machine's own body, and false when none is drawn.
func (l *StateLocator) start(owner stateItem) (string, bool) {
	parentKey := locatorKey("")
	if owner != (stateItem{}) {
		key, ok := l.key(owner)
		if !ok {
			return "", false
		}
		parentKey = key
	}
	id, ok := l.nodes[parentKey+"/"+startKind+"@0:0"]
	return id, ok
}

// StateStart is the ID of the start marker of the state's body, 0 for the
// machine's own, and false when none is drawn.
func (l *StateLocator) StateStart(owner program.StateID) (string, bool) {
	return l.start(stateItem{vertex: program.StateVertex(owner)})
}

// RegionStart is the ID of the start marker of the region's body, and false when none is drawn.
func (l *StateLocator) RegionStart(owner program.RegionID) (string, bool) {
	return l.start(stateItem{region: owner})
}

// Start is the ID of the start marker of the body owner owns — a state, a
// region, or nil for the machine's own body — and false when none is drawn.
func (l *StateLocator) Start(owner ast.Node) (string, bool) {
	if owner == nil {
		return l.start(stateItem{})
	}
	item := l.item(owner)
	if item == (stateItem{}) {
		return "", false
	}
	return l.start(item)
}

// TransitionEdge is the edge position drawing the transition; false when none does.
func (l *StateLocator) TransitionEdge(transition *lower.Transition) (int, bool) {
	to, ok := l.Vertex(transition.TargetRef)
	if !ok {
		return 0, false
	}
	from, ok := l.Vertex(transition.SourceRef)
	if !ok {
		return 0, false
	}
	return firstEdge(l.edges, l.place(transition.Declared()), from, to)
}

// EntryEdge is the edge position drawing the entry transition from the start
// marker from to the vertex it reaches first; false when none does.
func (l *StateLocator) EntryEdge(entry *lower.EntryTransition, from string) (int, bool) {
	ref, _ := entry.TargetVertex()
	to, ok := l.Vertex(ref)
	if !ok {
		return 0, false
	}
	return firstEdge(l.edges, l.place(entry.Declared()), from, to)
}

// Transition is the edge position of decl from source to target; false when
// none draws it.
func (l *StateLocator) Transition(decl, source, target ast.Node) (int, bool) {
	to, ok := l.Node(target)
	if !ok {
		return 0, false
	}
	from, ok := l.Node(source)
	if !ok {
		return 0, false
	}
	return firstEdge(l.edges, l.place(decl), from, to)
}

// EntryTransition is the edge position of the entry transition decl written in
// owner's body (nil for the machine's own) to target; false when none draws it.
func (l *StateLocator) EntryTransition(decl, owner, target ast.Node) (int, bool) {
	to, ok := l.Node(target)
	if !ok {
		return 0, false
	}
	from, ok := l.Start(owner)
	if !ok {
		return 0, false
	}
	return firstEdge(l.edges, l.place(decl), from, to)
}

// firstEdge is the first edge written at place joining from and to; an edge with
// no declaration of its own is matched by its endpoints alone.
func firstEdge(edges map[edgeKey][]int, at string, from, to string) (int, bool) {
	if found := edges[edgeKey{at, from, to}]; len(found) > 0 {
		return found[0], true
	}
	return 0, false
}

// ActionLocator finds the nodes and edges of a rendering that draw the nodes
// and successions of a lowered action, the flows of its nested actions included.
type ActionLocator struct {
	root  string
	at    anchors
	doc   string
	graph *lower.ActionGraph
	nodes map[locatorKey]string
	edges map[edgeKey][]int
}

// LocateActions matches the nodes of graph, lowered from action's declaration, to
// the root of rendering drawing drawn; keys carry no kind, as an executor's node has none.
func LocateActions(rendering *Rendering, drawn, action *symbols.Symbol, graph *lower.ActionGraph) (*ActionLocator, error) {
	root, err := RootDrawing(rendering, drawn)
	if err != nil {
		return nil, err
	}
	if action == nil {
		return nil, fmt.Errorf("%w: no declaration was lowered", ErrNotDrawn)
	}
	if graph == nil {
		return nil, fmt.Errorf("%w: %s has no lowered action graph", ErrNotDrawn, action.Name)
	}
	return &ActionLocator{
		root:  root.ID,
		at:    graphAnchors(action, graph.Inherited()),
		doc:   action.DocName,
		graph: graph,
		nodes: renderedKeys(root, false),
		edges: renderedEdges(rendering, root),
	}, nil
}

// Node is the ID drawing node in the flow of the nested actions within (outermost
// first); an undrawn node is placed at the innermost drawn node around it, with false.
func (l *ActionLocator) Node(within []ast.Node, node ast.Node) (string, bool) {
	key := locatorKey("")
	graph := l.graph
	for _, outer := range within {
		next := key + "/@" + l.place(graph, outer)
		if _, ok := l.nodes[next]; !ok {
			return l.nodes[key], false
		}
		key, graph = next, l.flowOf(graph, outer)
	}
	if node != nil {
		if id, ok := l.nodes[key+"/@"+l.place(graph, node)]; ok {
			return id, true
		}
	}
	return l.nodes[key], false
}

// NodeAt is Node by id: within are the nested actions around the node, outermost
// first, each a node of the flow holding it; the zero FlowNode names no node.
func (l *ActionLocator) NodeAt(within []lower.FlowNode, node lower.FlowNode) (string, bool) {
	key := locatorKey("")
	for _, outer := range within {
		next := key + "/@" + l.placeOf(outer)
		if _, ok := l.nodes[next]; !ok {
			return l.nodes[key], false
		}
		key = next
	}
	if node != (lower.FlowNode{}) {
		if id, ok := l.nodes[key+"/@"+l.placeOf(node)]; ok {
			return id, true
		}
	}
	return l.nodes[key], false
}

// EdgeAt is Edge by id: the position of the succession edge of graph, the flow
// of the innermost of the nested actions within, false when no edge draws it.
func (l *ActionLocator) EdgeAt(within []lower.FlowNode, graph *lower.ActionGraph, edge lower.ActionEdge) (int, bool) {
	from, ok := l.NodeAt(within, lower.FlowNode{Graph: graph, ID: edge.SourceID})
	if !ok {
		return 0, false
	}
	to, ok := l.NodeAt(within, lower.FlowNode{Graph: graph, ID: edge.TargetID})
	if !ok {
		return 0, false
	}
	return firstEdge(l.edges, l.place(graph, edge.Succession()), from, to)
}

// placeOf spells where a node was written, as the rendering does.
func (l *ActionLocator) placeOf(node lower.FlowNode) string {
	if node.Graph == nil {
		return l.at.place("", source.Span{})
	}
	return l.place(node.Graph, node.Graph.Provenance.Node(node.ID))
}

// flowOf is the graph of the flow node owns within graph, nil where it owns none.
func (l *ActionLocator) flowOf(graph *lower.ActionGraph, node ast.Node) *lower.ActionGraph {
	if graph == nil {
		return nil
	}
	if sub := graph.Subflows[node]; sub != nil {
		return sub.Graph
	}
	return nil
}

// place spells where a declaration of graph was written, as the rendering does; a
// declaration of a flow the graph does not hold is placed in the action's document.
func (l *ActionLocator) place(graph *lower.ActionGraph, decl ast.Node) string {
	if decl == nil {
		return l.at.place("", source.Span{})
	}
	doc := l.doc
	if graph != nil {
		doc = docOf(graph, decl, l.doc)
	}
	return l.at.place(doc, decl.Span())
}

// Root is the ID of the node drawing the action itself.
func (l *ActionLocator) Root() string { return l.root }

// Edge is the position in the rendering's edges of the succession edge taken in
// the flow of the nested actions within, false when no edge draws it.
func (l *ActionLocator) Edge(within []ast.Node, edge lower.ActionEdge) (int, bool) {
	from, ok := l.Node(within, edge.Source)
	if !ok {
		return 0, false
	}
	to, ok := l.Node(within, edge.Target)
	if !ok {
		return 0, false
	}
	graph := l.graph
	for _, outer := range within {
		graph = l.flowOf(graph, outer)
	}
	return firstEdge(l.edges, l.place(graph, edge.Decl), from, to)
}
