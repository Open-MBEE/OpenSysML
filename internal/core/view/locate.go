package view

import (
	"fmt"
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// A locator finds the render node or edge drawing a vertex of a graph lowered
// separately, by declaration-relative position rather than identity.

// locatorKey is a node's path within the drawn root: kind, declaration span and
// an ordinal among same-span siblings, one segment per level of nesting.
type locatorKey = string

// edgeKey is how an edge is found: what it was written as and the nodes it joins.
type edgeKey struct {
	span     source.Span
	from, to string
}

// relativeTo places span against the declaration drawn at base; an unlocated
// span is the zero span.
func relativeTo(span source.Span, base int) source.Span {
	if span.Len <= 0 {
		return source.Span{}
	}
	return source.Span{Offset: span.Offset - base, Len: span.Len}
}

// spanKey spells a span for a locator key; an unlocated node spells as 0:0.
func spanKey(span source.Span) string {
	if span.Len <= 0 {
		return "0:0"
	}
	return strconv.Itoa(span.Offset) + ":" + strconv.Itoa(span.Len)
}

// childKey is the key of a child of parent drawn as kind from span, numbered
// among the parent's children of the same kind and span so far.
func childKey(parent locatorKey, kind string, span source.Span, seen map[string]int) locatorKey {
	base := parent + "/" + kind + "@" + spanKey(span)
	n := seen[base]
	seen[base]++
	if n > 0 {
		return base + "#" + strconv.Itoa(n)
	}
	return base
}

// renderedKeys keys every node nested in root, the root itself under "".
func renderedKeys(root *Node) map[locatorKey]string {
	base := root.Origin.Span.Offset
	keys := map[locatorKey]string{"": root.ID}
	var walk func(node *Node, parent locatorKey)
	walk = func(node *Node, parent locatorKey) {
		seen := map[string]int{}
		for _, child := range node.Children {
			key := childKey(parent, child.Kind, relativeTo(child.Origin.Span, base), seen)
			keys[key] = child.ID
			walk(child, key)
		}
	}
	walk(root, "")
	return keys
}

// renderedEdges indexes the rendering's edges by declaration, relative to root,
// and endpoints, each to the positions it holds in rendering.Edges.
func renderedEdges(rendering *Rendering, root *Node) map[edgeKey][]int {
	base := root.Origin.Span.Offset
	edges := map[edgeKey][]int{}
	for i, edge := range rendering.Edges {
		key := edgeKey{relativeTo(edge.Origin.Span, base), edge.From, edge.To}
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
	base  int
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
		base:  machine.DeclSpan.Offset,
		graph: graph,
		drawn: make(map[*ast.StateNode]bool, len(graph.States)),
		keys:  make(map[ast.Node]locatorKey),
		seen:  make(map[locatorKey]map[string]int),
		nodes: renderedKeys(root),
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
	key := childKey(parentKey, kind, relativeTo(v.Span(), l.base), seen)
	l.keys[v] = key
	return key, true
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

// Transition is the edge position of decl from source to target, source nil for
// an entry transition leaving target's body start; false when none draws it.
func (l *StateLocator) Transition(decl, source, target ast.Node) (int, bool) {
	to, ok := l.Node(target)
	if !ok {
		return 0, false
	}
	var from string
	if source == nil {
		state, isState := target.(*ast.StateNode)
		if !isState {
			return 0, false
		}
		if from, ok = l.Start(l.parentOfState(state)); !ok {
			return 0, false
		}
	} else if from, ok = l.Node(source); !ok {
		return 0, false
	}
	return firstEdge(l.edges, decl, l.base, from, to)
}

// firstEdge is the first edge written as decl, relative to base, joining from
// and to; an edge with no declaration of its own is matched by its endpoints alone.
func firstEdge(edges map[edgeKey][]int, decl ast.Node, base int, from, to string) (int, bool) {
	var span source.Span
	if decl != nil {
		span = relativeTo(decl.Span(), base)
	}
	if found := edges[edgeKey{span, from, to}]; len(found) > 0 {
		return found[0], true
	}
	return 0, false
}

// ActionLocator finds the nodes and edges of a rendering that draw the nodes
// and successions of a lowered action, the flows of its nested actions included.
type ActionLocator struct {
	root  string
	base  int
	nodes map[locatorKey]string
	edges map[edgeKey][]int
}

// LocateActions matches the nodes of action's lowered graph, nested flows
// included, to the root of rendering drawing drawn: the same declaration, as is.
func LocateActions(rendering *Rendering, drawn, action *symbols.Symbol) (*ActionLocator, error) {
	root, err := RootDrawing(rendering, drawn)
	if err != nil {
		return nil, err
	}
	if action == nil {
		return nil, fmt.Errorf("%w: no declaration was lowered", ErrNotDrawn)
	}
	return &ActionLocator{root: root.ID, base: action.DeclSpan.Offset, nodes: actionKeys(root), edges: renderedEdges(rendering, root)}, nil
}

// actionKeys keys every node nested in root by the spans of the nodes it is
// nested in, kinds left out: an executor's node carries a span, not a kind.
func actionKeys(root *Node) map[locatorKey]string {
	base := root.Origin.Span.Offset
	keys := map[locatorKey]string{"": root.ID}
	var walk func(node *Node, parent locatorKey)
	walk = func(node *Node, parent locatorKey) {
		seen := map[string]int{}
		for _, child := range node.Children {
			key := childKey(parent, "", relativeTo(child.Origin.Span, base), seen)
			keys[key] = child.ID
			walk(child, key)
		}
	}
	walk(root, "")
	return keys
}

// Node is the ID drawing node in the flow of the nested actions within (outermost
// first); an undrawn node is placed at the innermost drawn node around it, with false.
func (l *ActionLocator) Node(within []ast.Node, node ast.Node) (string, bool) {
	key := locatorKey("")
	for _, outer := range within {
		next := key + "/@" + spanKey(relativeTo(outer.Span(), l.base))
		if _, ok := l.nodes[next]; !ok {
			return l.nodes[key], false
		}
		key = next
	}
	if node != nil {
		if id, ok := l.nodes[key+"/@"+spanKey(relativeTo(node.Span(), l.base))]; ok {
			return id, true
		}
	}
	return l.nodes[key], false
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
	return firstEdge(l.edges, edge.Decl, l.base, from, to)
}
