package view

import (
	"fmt"
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
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
	return append(anchors{root.Origin}, root.Inherited...)
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

// StateLocator finds the nodes and edges of a rendering that draw the vertices
// and transitions of a lowered state machine.
type StateLocator struct {
	root  string
	at    anchors
	doc   string
	graph *lower.StateGraph
	// drawn are the states the rendering draws: graph-only owners are not.
	drawn map[*ast.StateNode]bool
	// keys are the located keys of the graph's vertices and regions, memoized;
	// seen numbers same-span children per parent key as the rendering does.
	keys  map[ast.Node]locatorKey
	seen  map[locatorKey]map[string]int
	nodes map[locatorKey]string
	edges map[edgeKey][]int
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
		root:  root.ID,
		at:    graphAnchors(machine, graph.Inherited()),
		doc:   machine.DocName,
		graph: graph,
		drawn: make(map[*ast.StateNode]bool, len(graph.States)),
		keys:  make(map[ast.Node]locatorKey),
		seen:  make(map[locatorKey]map[string]int),
		nodes: renderedKeys(root, true),
		edges: renderedEdges(rendering, root),
	}
	for _, state := range graph.States {
		l.drawn[state] = true
	}
	for _, region := range graph.TopRegions {
		l.key(region)
	}
	for _, state := range graph.States {
		l.key(state)
		for _, region := range graph.CompositeStates[state] {
			l.key(region)
		}
	}
	for _, pseudo := range graph.Pseudostates {
		l.key(pseudo)
	}
	return l, nil
}

// key is the located key of a vertex or region, "" with false for one the
// rendering does not draw, such as a graph-only region owner.
func (l *StateLocator) key(v ast.Node) (locatorKey, bool) {
	if key, ok := l.keys[v]; ok {
		return key, key != ""
	}
	var parent ast.Node
	var kind string
	switch n := v.(type) {
	case *ast.StateNode:
		if !l.drawn[n] {
			l.keys[v] = ""
			return "", false
		}
		kind, parent = "state", l.parentOfState(n)
	case *ast.StateRegion:
		owner := l.graph.RegionOwner[n]
		if owner != nil && !l.drawn[owner] {
			l.keys[v] = ""
			return "", false
		}
		kind, parent = "region", owner
	case *ast.PseudostateNode:
		kind = n.Kind.String()
		if owner := l.graph.PseudostateOwner[n]; owner != nil && l.drawn[owner] {
			parent = owner
		}
	default:
		return "", false
	}
	parentKey := locatorKey("")
	if parent != nil {
		if key, ok := l.key(parent); ok {
			parentKey = key
		}
	}
	seen := l.seen[parentKey]
	if seen == nil {
		seen = map[string]int{}
		l.seen[parentKey] = seen
	}
	key := childKey(parentKey, kind, l.place(v), seen)
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
// drawn, else its parent state when that is, else the machine itself (nil).
func (l *StateLocator) parentOfState(state *ast.StateNode) ast.Node {
	if region := l.graph.RegionOf[state]; region != nil {
		if owner := l.graph.RegionOwner[region]; owner == nil || l.drawn[owner] {
			return region
		}
	}
	if owner := l.graph.ParentState[state]; owner != nil && l.drawn[owner] {
		return owner
	}
	return nil
}

// Root is the ID of the node drawing the machine itself.
func (l *StateLocator) Root() string { return l.root }

// Node is the ID of the node drawing vertex — a state, region or pseudostate of
// the graph — and false for one the rendering does not draw.
func (l *StateLocator) Node(vertex ast.Node) (string, bool) {
	key, ok := l.key(vertex)
	if !ok {
		return "", false
	}
	id, ok := l.nodes[key]
	return id, ok
}

// Start is the ID of the start marker of the body owner owns — a state, a
// region, or nil for the machine's own body — and false when none is drawn.
func (l *StateLocator) Start(owner ast.Node) (string, bool) {
	parentKey := locatorKey("")
	if owner != nil {
		key, ok := l.key(owner)
		if !ok {
			return "", false
		}
		parentKey = key
	}
	id, ok := l.nodes[parentKey+"/"+startKind+"@0:0"]
	return id, ok
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
