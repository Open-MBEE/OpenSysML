package view

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"unicode/utf8"
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
// DiagramLayout geometry is written as Graphviz reads it (pinned `pos`, `bb`,
// `pos` splines, `size`); docs/project/view-rendering-forms.md#geometry has the rules.
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
		if len(edge.Route) > 1 {
			w.routed++
		} else {
			w.unrouted++
		}
		if len(edge.Route) == 1 {
			p := edge.Route[0]
			w.notices = append(w.notices, fmt.Sprintf("route of %s->%s is one waypoint, (%s, %s); a line needs two", edge.From, edge.To, formatCoord(p.X), formatCoord(p.Y)))
		}
	}
	if engine := w.engine(); w.routed > 0 && engine != "neato -n2" {
		w.notices = append(w.notices, fmt.Sprintf("%d route(s) written as pos; %s redraws every edge, only neato -n2 keeps them", w.routed, engine))
	}
	b := &w.b
	if r.View != "" {
		fmt.Fprintf(b, "// view: %s\n", r.View)
	}
	fmt.Fprintf(b, "// kind: %s\n", r.Kind)
	if r.Stated != "" {
		fmt.Fprintf(b, "// stated: %s\n", r.Stated)
	}
	for _, notice := range slices.Concat(r.Notices, w.notices) {
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
	routed    int // edges with a route to write, and those without
	unrouted  int
	notices   []string // geometry the form cannot draw
}

// countPlaced counts the nodes under node, those a Geometry positions, and a
// tree's containment edges, which no Route covers.
func (w *dotWriter) countPlaced(node *Node) {
	w.nodes++
	if node.Geometry != nil {
		w.placed++
	}
	if w.tree {
		w.unrouted += len(node.Children)
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
		width, height := dotBox(node)
		attrs = append(attrs, w.dotPin(Point{X: g.X + width/2, Y: g.Y + height/2}))
		if node.Kind != startKind {
			attrs = append(attrs, "width="+dotInches(width), "height="+dotInches(height))
		}
		if g.HasSize {
			attrs = append(attrs, "fixedsize=true")
		}
		if g.Collapsed {
			attrs = append(attrs, `comment="collapsed"`)
		}
	}
	return attrs
}

// Graphviz's defaults a label-fitted box is estimated with: 14pt text at 0.6em
// a glyph and 1.2em a line, a 0.11in by 0.055in margin, a 0.75in by 0.5in node.
const (
	dotGlyphWidth   = 8.4
	dotLineHeight   = 16.8
	dotMarginWidth  = 7.92
	dotMarginHeight = 3.96
	dotNodeWidth    = 54
	dotNodeHeight   = 36
	dotPointSize    = 3.6
)

// dotBox is the box a positioned node is centred in, in points: the stated
// size, or one fitted to its label so Graphviz has no cause to grow it.
func dotBox(node *Node) (width, height float64) {
	if g := node.Geometry; g.HasSize {
		return g.Width, g.Height
	}
	if node.Kind == startKind {
		return dotPointSize, dotPointSize
	}
	lines, longest := dotLabelExtent(node)
	width = math.Ceil(float64(longest)*dotGlyphWidth + 2*dotMarginWidth)
	height = math.Ceil(float64(lines)*dotLineHeight + 2*dotMarginHeight)
	if node.Kind == "initial" || node.Kind == "final" {
		side := math.Max(dotNodeHeight, math.Ceil(math.Hypot(width, height)))
		return side, side
	}
	return math.Max(dotNodeWidth, width), math.Max(dotNodeHeight, height)
}

// dotLabelExtent is the line count of a node's label and the glyphs on its
// longest line.
func dotLabelExtent(node *Node) (lines, longest int) {
	for _, line := range strings.Split(dotLabelText(node), "\n") {
		lines++
		longest = max(longest, utf8.RuneCountInString(line))
	}
	return lines, longest
}

// dotPin pins a node at a pixel point: `pos="x,y!"` and `pin=true`.
func (w *dotWriter) dotPin(centre Point) string {
	return fmt.Sprintf("pos=%s, pin=true", dotQuote(w.dotPoint(centre)+"!"))
}

// dotAnchorAttributes is a cluster's anchor node: invisible and sizeless, the
// node an edge to or from the cluster names. A placed cluster pins it at the
// centre of its box, or at its corner while the box has no extent.
func (w *dotWriter) dotAnchorAttributes(node *Node) []string {
	attrs := []string{"shape=point", "style=invis", "width=0", "height=0", `label=""`}
	if node.Geometry != nil {
		min, max := clusterBox(node)
		attrs = append(attrs, w.dotPin(Point{X: (min.X + max.X) / 2, Y: (min.Y + max.Y) / 2}))
	}
	return attrs
}

// dotClusterAttributes is a cluster's attribute statements: its label, a dashed
// border for an orthogonal region, and its box as `bb` when it has an extent.
func (w *dotWriter) dotClusterAttributes(node *Node) []string {
	attrs := []string{"label=" + dotLabel(node)}
	if node.Kind == "region" {
		attrs = append(attrs, "style=dashed")
	}
	if g := node.Geometry; g != nil {
		if min, max := clusterBox(node); min != max {
			attrs = append(attrs, "bb="+dotQuote(w.dotPoint(Point{X: min.X, Y: max.Y})+","+w.dotPoint(Point{X: max.X, Y: min.Y})))
		}
		if g.Collapsed {
			attrs = append(attrs, `comment="collapsed"`)
		}
	}
	return attrs
}

// dotClusterMargin is the space Graphviz keeps between a cluster's border and
// its members, in points.
const dotClusterMargin = 8

// clusterBox is a positioned cluster's box, top-left to bottom-right: the stated
// one, or its corner grown round its positioned members (the corner alone with none).
func clusterBox(node *Node) (min, max Point) {
	g := node.Geometry
	min = Point{X: g.X, Y: g.Y}
	if g.HasSize {
		return min, Point{X: g.X + g.Width, Y: g.Y + g.Height}
	}
	max = min
	for _, child := range node.Children {
		if child.Geometry == nil {
			continue
		}
		low, high := memberBox(child)
		if low == high {
			continue
		}
		min = Point{X: math.Min(min.X, low.X-dotClusterMargin), Y: math.Min(min.Y, low.Y-dotClusterMargin)}
		max = Point{X: math.Max(max.X, high.X+dotClusterMargin), Y: math.Max(max.Y, high.Y+dotClusterMargin)}
	}
	return min, max
}

// memberBox is the box a positioned member of a cluster takes up: a cluster's
// own box, a node's stated or label-fitted box.
func memberBox(node *Node) (min, max Point) {
	if len(node.Children) > 0 {
		return clusterBox(node)
	}
	g := node.Geometry
	width, height := dotBox(node)
	return Point{X: g.X, Y: g.Y}, Point{X: g.X + width, Y: g.Y + height}
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
	return dotQuote(dotLabelText(node))
}

// dotLabelText is the text of a node's label before quoting.
func dotLabelText(node *Node) string {
	head := node.Kind
	if node.Name != "" {
		head += " " + node.Name
	}
	if node.Detail == "" {
		return head
	}
	return head + "\n" + node.Detail
}

// dotQuote writes text as a double-quoted DOT string; every ID and label goes
// through it, so none is written bare. A newline becomes a `\n` line break.
func dotQuote(text string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)
	return `"` + replacer.Replace(text) + `"`
}
