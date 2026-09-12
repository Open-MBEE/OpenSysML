package view

import (
	"fmt"
	"slices"
	"strings"
)

// DOT is the Graphviz form of a graph-shaped rendering: a `digraph`, written
// as text like the Mermaid form, with no Graphviz installed. A node with
// children is a `subgraph "cluster_<id>"` (a tree draws containment as edges,
// as its Mermaid form does) holding an invisible anchor node `"<id>"`, so an
// edge keeps the rendering's endpoints and is clipped at the cluster with
// `lhead`/`ltail` — except at an end that encloses the other, where the edge
// starts or ends inside it. Edge kinds parallel the Mermaid arrows:
//
//	EdgeKind        Mermaid  DOT
//	EdgeConnection  ---      arrowhead=none
//	EdgeTransition  -->      solid, default arrowhead
//	EdgeSuccession  -->      solid, default arrowhead
//	EdgeFlow        -.->     style=dashed
//	(containment)   ---      arrowhead=none
//
// The DiagramLayout geometry of the rendering is written as Graphviz reads it,
// one pixel to one point, y up: a positioned node is pinned with `pos="x,y!"`
// at its centre, sized in inches with `width`/`height` and `fixedsize`; a
// positioned cluster pins its anchor and states its `bb`; a routed edge has
// its waypoints as a `pos` spline; a sized canvas is the graph's `size`. The
// `// layout:` header names the engine to lay the text out with: `neato -n2`
// when every node is positioned and every edge routed, `neato -n` when every
// node is positioned, `neato` when some are (the rest are placed around
// them), `dot` when none is.
//
// A kind with no DOT counterpart — sequence, table — is a *WrongFormError.
func (r *Rendering) DOT() (string, error) {
	return r.DOTDirected("")
}

// DOTDirected is the DOT form laid out in the stated direction, as `rankdir`;
// the empty direction leaves the engine's default and writes no `rankdir`.
func (r *Rendering) DOTDirected(direction Direction) (string, error) {
	if !r.Kind.SupportsForm(FormDot) {
		return "", &WrongFormError{Form: FormDot, Kind: r.Kind, View: r.View}
	}
	w := &dotWriter{tree: r.Kind == KindTree, clusters: map[string]bool{}, enclosing: map[string][]string{}, canvas: r.Canvas}
	for _, root := range r.Roots {
		if !w.tree {
			w.collectClusters(root, nil)
		}
		w.countPlaced(root)
	}
	for _, edge := range r.Edges {
		if w.clipped(edge.From, edge.To) || w.clipped(edge.To, edge.From) {
			w.compound = true
		}
		if len(edge.Route) < 2 {
			w.unrouted++
		}
	}
	b := &w.b
	if r.View != "" {
		fmt.Fprintf(b, "// view: %s\n", r.View)
	}
	fmt.Fprintf(b, "// kind: %s\n", r.Kind)
	if r.Stated != "" {
		fmt.Fprintf(b, "// stated: %s\n", r.Stated)
	}
	for _, notice := range r.Notices {
		fmt.Fprintf(b, "// not represented: %s\n", notice)
	}
	if c := r.Canvas; c != nil {
		b.WriteString("// canvas:")
		if c.Unit != "" {
			b.WriteString(" unit=" + c.Unit)
		}
		if c.HasSize {
			fmt.Fprintf(b, " w=%s h=%s", formatCoord(c.Width), formatCoord(c.Height))
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(b, "// layout: %s\n", w.engine())
	if r.View == "" {
		b.WriteString("digraph {\n")
	} else {
		fmt.Fprintf(b, "digraph %s {\n", dotQuote(r.View))
	}
	if attrs := w.graphAttributes(direction); len(attrs) > 0 {
		fmt.Fprintf(b, "  graph [%s];\n", strings.Join(attrs, ", "))
	}
	b.WriteString("  node [shape=box];\n")
	if r.Empty() {
		fmt.Fprintf(b, "  \"empty\" [shape=plaintext, label=%s];\n", dotQuote(r.EmptyReason()))
		b.WriteString("}\n")
		return b.String(), nil
	}
	for _, root := range r.Roots {
		w.writeNode(root, 1)
	}
	for _, edge := range r.Edges {
		w.writeEdge(edge.From, edge.To, w.dotEdgeAttributes(edge))
	}
	b.WriteString("}\n")
	return b.String(), nil
}

// dotWriter holds what one rendering's DOT form needs across nodes and edges.
type dotWriter struct {
	b         strings.Builder
	tree      bool                // containment as edges, not clusters
	clusters  map[string]bool     // node IDs drawn as clusters
	enclosing map[string][]string // node ID -> the cluster IDs around it
	compound  bool                // an edge is clipped at a cluster
	canvas    *Canvas             // the surface positions are flipped against
	nodes     int                 // nodes written, and how many are positioned
	placed    int
	unrouted  int // edges with no route to write
}

// countPlaced counts the nodes under node and those a Geometry positions.
func (w *dotWriter) countPlaced(node *Node) {
	w.nodes++
	if node.Geometry != nil {
		w.placed++
	}
	for _, child := range node.Children {
		w.countPlaced(child)
	}
}

// engine is the Graphviz command the text is written for: the `neato` variant
// that keeps what is positioned and routed, or `dot` for an unpositioned graph.
func (w *dotWriter) engine() string {
	switch {
	case w.placed == 0:
		return "dot"
	case w.placed < w.nodes:
		return "neato"
	case w.unrouted > 0:
		return "neato -n"
	}
	return "neato -n2"
}

// collectClusters records every node under node that is drawn as a cluster
// and the clusters each node sits in, outermost first.
func (w *dotWriter) collectClusters(node *Node, around []string) {
	w.enclosing[node.ID] = around
	if len(node.Children) == 0 {
		return
	}
	w.clusters[node.ID] = true
	around = append(around[:len(around):len(around)], node.ID)
	for _, child := range node.Children {
		w.collectClusters(child, around)
	}
}

// clipped reports whether an edge's end at node is clipped at node's cluster:
// node is a cluster that does not enclose the other end.
func (w *dotWriter) clipped(node, other string) bool {
	return w.clusters[node] && !slices.Contains(w.enclosing[other], node)
}

// dotClusterName is the subgraph name of the cluster drawn for a node.
func dotClusterName(id string) string { return "cluster_" + id }

// graphAttributes is the graph attribute list: the layout direction when one is
// stated, `compound` when an edge is clipped at a cluster, the pixel scale when
// a node is positioned, and the canvas extent as the drawing's size.
func (w *dotWriter) graphAttributes(direction Direction) []string {
	var attrs []string
	if direction != "" {
		attrs = append(attrs, "rankdir="+string(direction))
	}
	if w.compound {
		attrs = append(attrs, "compound=true")
	}
	if w.placed > 0 {
		attrs = append(attrs, "inputscale=72", "dpi=72")
	}
	if c := w.canvas; c != nil && c.HasSize {
		attrs = append(attrs, "size="+dotQuote(dotInches(c.Width)+","+dotInches(c.Height)))
	}
	return attrs
}

// flipY turns a y-down pixel coordinate into Graphviz's y-up point: measured
// from the canvas's bottom edge when it has a height, negated otherwise.
func (w *dotWriter) flipY(y float64) float64 {
	if w.canvas != nil && w.canvas.HasSize {
		return w.canvas.Height - y
	}
	return -y
}

// dotPoint is a pixel point as a Graphviz point, `x,y`.
func (w *dotWriter) dotPoint(p Point) string {
	return formatCoord(p.X) + "," + formatCoord(w.flipY(p.Y))
}

// dotInches is a pixel length in inches, at Graphviz's 72 points to the inch.
func dotInches(px float64) string {
	return formatCoord(px / 72)
}

// writeNode writes one node: a cluster holding its children, a plain node
// otherwise. A tree writes the node and an edge to each child instead.
func (w *dotWriter) writeNode(node *Node, depth int) {
	indent := strings.Repeat("  ", depth)
	if len(node.Children) == 0 || w.tree {
		fmt.Fprintf(&w.b, "%s%s [%s];\n", indent, dotQuote(node.ID), strings.Join(w.dotNodeAttributes(node), ", "))
		for _, child := range node.Children {
			w.writeNode(child, depth)
			w.writeEdge(node.ID, child.ID, dotContainmentAttributes())
		}
		return
	}
	fmt.Fprintf(&w.b, "%ssubgraph %s {\n", indent, dotQuote(dotClusterName(node.ID)))
	for _, attr := range w.dotClusterAttributes(node) {
		fmt.Fprintf(&w.b, "%s  %s;\n", indent, attr)
	}
	fmt.Fprintf(&w.b, "%s  %s [%s];\n", indent, dotQuote(node.ID), strings.Join(w.dotAnchorAttributes(node), ", "))
	for _, child := range node.Children {
		w.writeNode(child, depth+1)
	}
	fmt.Fprintf(&w.b, "%s}\n", indent)
}

// writeEdge writes one edge between the rendering's endpoints; an end that is
// a cluster is its anchor node, clipped at the cluster with `ltail`/`lhead`.
func (w *dotWriter) writeEdge(from, to string, attrs []string) {
	if w.clipped(from, to) {
		attrs = append(attrs, "ltail="+dotQuote(dotClusterName(from)))
	}
	if w.clipped(to, from) {
		attrs = append(attrs, "lhead="+dotQuote(dotClusterName(to)))
	}
	if len(attrs) == 0 {
		fmt.Fprintf(&w.b, "  %s -> %s;\n", dotQuote(from), dotQuote(to))
		return
	}
	fmt.Fprintf(&w.b, "  %s -> %s [%s];\n", dotQuote(from), dotQuote(to), strings.Join(attrs, ", "))
}

// dotNodeAttributes is a plain node's attribute list: its label, the shape its
// Kind chooses, and its position and size when a Geometry places it.
func (w *dotWriter) dotNodeAttributes(node *Node) []string {
	var attrs []string
	switch node.Kind {
	case startKind:
		attrs = []string{"shape=point", `label=""`}
	case "initial":
		attrs = []string{"shape=circle", "label=" + dotLabel(node)}
	case "final":
		attrs = []string{"shape=doublecircle", "label=" + dotLabel(node)}
	case "state":
		attrs = []string{"shape=box", "style=rounded", "label=" + dotLabel(node)}
	default:
		attrs = []string{"label=" + dotLabel(node)}
	}
	if g := node.Geometry; g != nil {
		attrs = append(attrs, w.dotPosition(node))
		if g.HasSize {
			attrs = append(attrs, "width="+dotInches(g.Width), "height="+dotInches(g.Height), "fixedsize=true")
		}
		if g.Collapsed {
			attrs = append(attrs, `comment="collapsed"`)
		}
	}
	return attrs
}

// dotPosition pins a positioned node at the centre of its box: `pos="x,y!"`
// and `pin=true`. A box of no stated size is Graphviz's default node, 0.75 by
// 0.5 inches, a point 0.05 across and a circle 0.75.
func (w *dotWriter) dotPosition(node *Node) string {
	g := node.Geometry
	width, height := g.Width, g.Height
	if !g.HasSize {
		switch node.Kind {
		case startKind:
			width, height = 3.6, 3.6
		case "initial", "final":
			width, height = 54, 54
		default:
			width, height = 54, 36
		}
	}
	centre := Point{X: g.X + width/2, Y: g.Y + height/2}
	return fmt.Sprintf("pos=%s, pin=true", dotQuote(w.dotPoint(centre)+"!"))
}

// dotAnchorAttributes is a cluster's anchor node: invisible and sizeless, the
// node an edge to or from the cluster names, pinned where the cluster is placed.
func (w *dotWriter) dotAnchorAttributes(node *Node) []string {
	attrs := []string{"shape=point", "style=invis", "width=0", "height=0", `label=""`}
	if node.Geometry != nil {
		attrs = append(attrs, w.dotPosition(node))
	}
	return attrs
}

// dotClusterAttributes is a cluster's attribute statements: its label, a dashed
// border for an orthogonal region, and its box as `bb` when a Geometry sizes it.
func (w *dotWriter) dotClusterAttributes(node *Node) []string {
	attrs := []string{"label=" + dotLabel(node)}
	if node.Kind == "region" {
		attrs = append(attrs, "style=dashed")
	}
	if g := node.Geometry; g != nil {
		if g.HasSize {
			corners := w.dotPoint(Point{X: g.X, Y: g.Y + g.Height}) + "," + w.dotPoint(Point{X: g.X + g.Width, Y: g.Y})
			attrs = append(attrs, "bb="+dotQuote(corners))
		}
		if g.Collapsed {
			attrs = append(attrs, `comment="collapsed"`)
		}
	}
	return attrs
}

// dotEdgeAttributes is an edge's attribute list: its label, its kind's style,
// and its route as a `pos` spline when a Route gives waypoints.
func (w *dotWriter) dotEdgeAttributes(edge Edge) []string {
	var attrs []string
	if edge.Label != "" {
		attrs = append(attrs, "label="+dotQuote(edge.Label))
	}
	switch edge.Kind {
	case EdgeConnection:
		attrs = append(attrs, "arrowhead=none")
	case EdgeFlow:
		attrs = append(attrs, "style=dashed")
	}
	if len(edge.Route) > 1 {
		attrs = append(attrs, "pos="+dotQuote(w.dotSpline(edge.Route)))
	}
	return attrs
}

// dotSpline is a polyline of waypoints as Graphviz's cubic B-spline: each
// segment's ends are its own control points, so the curve is the polyline.
func (w *dotWriter) dotSpline(route []Point) string {
	points := []string{w.dotPoint(route[0])}
	for i := 1; i < len(route); i++ {
		from, to := w.dotPoint(route[i-1]), w.dotPoint(route[i])
		points = append(points, from, to, to)
	}
	return strings.Join(points, " ")
}

// dotContainmentAttributes is a tree's containment edge, Mermaid's `---`.
func dotContainmentAttributes() []string {
	return []string{"arrowhead=none"}
}

// dotLabel is a node's quoted label: kind and name, then its detail on a
// second line.
func dotLabel(node *Node) string {
	head := node.Kind
	if node.Name != "" {
		head += " " + node.Name
	}
	if node.Detail == "" {
		return dotQuote(head)
	}
	return dotQuote(head + "\n" + node.Detail)
}

// dotQuote writes text as a double-quoted DOT string; every ID and label goes
// through it, so none is written bare. A newline becomes a `\n` line break.
func dotQuote(text string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)
	return `"` + replacer.Replace(text) + `"`
}
