package view

import (
	"fmt"
	"html"
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
// `pos` splines, pinned canvas corners); docs/project/view-rendering-forms.md#geometry
// has the rules.
//
// The drawing is in the Standard B&W style of the SysML v2 Pilot visualizer
// (docs/project/view-rendering-forms.md#style): Helvetica, white fills, thin
// #181818 lines, square definitions and rounded usages, the keyword line in
// italics. Defaults are written once as `graph`, `node` and `edge` statements;
// a node or edge states only what it deviates in, its style before its geometry.
//
// A kind with no DOT counterpart — sequence, table — is a *WrongFormError.
func (r *Rendering) DOT() (string, error) {
	return r.DOTWith(Options{})
}

// DOTWith is the DOT form written with options: laid out in the stated
// direction, as `rankdir` (the empty direction leaves the engine's default and
// writes no `rankdir`), and filled from the stated palette by keyword family
// (the empty palette draws in black and white). The layout is the same
// whatever the palette: it changes fills and borders alone.
func (r *Rendering) DOTWith(options Options) (string, error) {
	if !r.Kind.SupportsForm(FormDot) {
		return "", &WrongFormError{Form: FormDot, Kind: r.Kind, View: r.View}
	}
	if err := options.Palette.check(); err != nil {
		return "", err
	}
	direction := options.Direction
	w := &dotWriter{tree: r.Kind == KindTree, clusters: map[string]bool{}, enclosing: map[string][]string{}, canvas: r.Canvas,
		palette: options.Palette}
	for _, root := range r.Roots {
		if !w.tree {
			w.collectClusters(root, nil)
		}
		w.countPlaced(root)
		w.collectFamilies(root)
	}
	for _, edge := range r.Edges {
		if w.clipped(edge.From, edge.To) || w.clipped(edge.To, edge.From) {
			w.compound = true
		}
		if len(edge.Route) > 1 {
			w.routed++
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
	fmt.Fprintf(b, "  graph [%s];\n", strings.Join(w.graphAttributes(direction), ", "))
	fmt.Fprintf(b, "  node [%s];\n", strings.Join(dotNodeDefaults, ", "))
	fmt.Fprintf(b, "  edge [%s];\n", strings.Join(dotEdgeDefaults, ", "))
	w.writeCanvas()
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
	routed    int      // edges with a route to write
	notices   []string // geometry the form cannot draw
	palette   Palette  // the fills, by keyword family; empty is black and white
	families  []string // the keyword families of the nodes filled, in palette order
}

// The Standard B&W style, after the sysmlbw PlantUML skin: Helvetica text,
// white fills, thin #181818 lines, edge text a point smaller than node text.
const (
	dotFontName    = "Helvetica"
	dotLineColor   = "#181818"
	dotEdgeFontPts = 13
)

// dotNodeDefaults and dotEdgeDefaults are the `node` and `edge` statements the
// digraph opens with; a node or edge lists only what it deviates in.
var (
	dotNodeDefaults = []string{"shape=box", "style=filled", "fillcolor=white", "color=" + dotQuote(dotLineColor),
		"fontname=" + dotQuote(dotFontName), fmt.Sprintf("fontsize=%d", dotFontSize), "penwidth=0.5"}
	dotEdgeDefaults = []string{"color=" + dotQuote(dotLineColor), "fontname=" + dotQuote(dotFontName),
		fmt.Sprintf("fontsize=%d", dotEdgeFontPts), "penwidth=1"}
)

// dotControlKinds are the kinds drawn as control and pseudo-state nodes: they
// keep the black-and-white rules and a square shape under every palette.
var dotControlKinds = map[string]bool{startKind: true, "initial": true, "final": true, "fork": true, "join": true,
	"merge": true, "decision": true, "choice": true, "junction": true, "shallow history": true, "deep history": true}

// dotFilled reports whether a node takes a family colour under a palette: a
// plain node (a cluster keeps its black border) that is no control node.
func (w *dotWriter) dotFilled(node *Node) bool {
	return w.palette != "" && !dotControlKinds[node.Kind] && (len(node.Children) == 0 || w.tree)
}

// collectFamilies records the keyword families of the nodes under node that a
// palette fills, in palette order, so a sequential palette spans those present.
func (w *dotWriter) collectFamilies(node *Node) {
	if w.dotFilled(node) {
		family := paletteFamily(node.Kind)
		if !slices.Contains(w.families, family) {
			w.families = append(w.families, family)
			slices.SortFunc(w.families, func(a, b string) int { return familyRank(a) - familyRank(b) })
		}
	}
	for _, child := range node.Children {
		w.collectFamilies(child)
	}
}

// dotFamilyColor is the palette colour of a node's keyword family: a qualitative
// palette's colour at the family's fixed rank, a sequential palette's at the
// family's place among those present.
func (w *dotWriter) dotFamilyColor(node *Node) string {
	family := paletteFamily(node.Kind)
	if w.palette.Sequential() {
		return w.palette.Color(slices.Index(w.families, family), len(w.families))
	}
	return w.palette.Color(familyRank(family), len(paletteFamilies)+1)
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

// engine is the Graphviz command the text is written for: `neato -n` keeps every
// node where it is pinned, `-n2` keeps the routes too and draws the other edges.
func (w *dotWriter) engine() string {
	switch {
	case w.placed == 0:
		return "dot"
	case w.placed < w.nodes:
		return "neato"
	case w.routed > 0:
		return "neato -n2"
	}
	return "neato -n"
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

// graphAttributes is the graph attribute list: the font, the layout direction
// when one is stated, `compound` when an edge is clipped at a cluster, and the
// pixel scale when a node is positioned.
func (w *dotWriter) graphAttributes(direction Direction) []string {
	attrs := []string{"fontname=" + dotQuote(dotFontName)}
	if direction != "" {
		attrs = append(attrs, "rankdir="+string(direction))
	}
	if w.compound {
		attrs = append(attrs, "compound=true")
	}
	if w.placed > 0 {
		attrs = append(attrs, "inputscale=72", "dpi=72")
	}
	return attrs
}

// dotCanvasCorners are the nodes that hold a positioned drawing to its canvas.
var dotCanvasCorners = [2]string{"canvas:0", "canvas:1"}

// writeCanvas pins an invisible, sizeless point at each corner of a sized
// canvas, so the drawing's `bb` is the canvas; it needs an engine that keeps pins.
func (w *dotWriter) writeCanvas() {
	c := w.canvas
	if c == nil || !c.HasSize || w.placed == 0 {
		return
	}
	for i, corner := range [2]Point{{}, {X: c.Width, Y: c.Height}} {
		fmt.Fprintf(&w.b, "  %s [%s, %s];\n", dotQuote(dotCanvasCorners[i]), strings.Join(dotInvisibleAttributes, ", "), w.dotPin(corner))
	}
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

// dotNodeAttributes is a plain node's attribute list: the shape and style its
// Kind chooses, its family colours under a palette, its label, then its
// position and size when a Geometry places it.
func (w *dotWriter) dotNodeAttributes(node *Node) []string {
	var attrs []string
	switch node.Kind {
	case startKind:
		attrs = []string{"shape=point", "fillcolor=black", `label=""`}
	case "initial", "final":
		attrs = w.dotPseudostateAttributes(node)
	default:
		if !dotControlKinds[node.Kind] && !isDefinitionKind(node.Kind) {
			attrs = append(attrs, `style="rounded,filled"`)
		}
		if w.dotFilled(node) {
			color := w.dotFamilyColor(node)
			attrs = append(attrs, "fillcolor="+dotQuote(paletteFill(color, !isDefinitionKind(node.Kind))),
				"color="+dotQuote(color), "penwidth=1")
		}
		attrs = append(attrs, dotLabel(node))
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

// dotPseudostateWidth is the diameter, in inches, of an initial or final
// pseudo-state drawn as the UML filled dot, with no name to show.
const dotPseudostateWidth = "0.2"

// dotPseudostateAttributes is an initial or final node's shape and label: the
// UML filled black dot, or double ring, when it has no name to show, a labelled
// circle when the rendering names it. A placed one keeps its placed size.
func (w *dotWriter) dotPseudostateAttributes(node *Node) []string {
	shape := "shape=circle"
	if node.Kind == "final" {
		shape = "shape=doublecircle"
	}
	if node.Name != "" {
		return []string{shape, dotLabel(node)}
	}
	attrs := []string{shape, "fillcolor=black", `label=""`}
	if node.Geometry == nil {
		attrs = append(attrs, "width="+dotPseudostateWidth)
	}
	return attrs
}

// Graphviz's defaults a label-fitted box is estimated with: 14pt text at 0.6em
// a glyph (0.66em in bold) and 1.2em a line, a 0.11in by 0.055in margin, a
// 0.75in by 0.5in node.
const (
	dotFontSize     = 14
	dotGlyphEm      = 0.6
	dotBoldGlyphEm  = 0.66
	dotLineEm       = 1.2
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
	width, height = dotLabelExtent(node)
	width = math.Ceil(width + 2*dotMarginWidth)
	height = math.Ceil(height + 2*dotMarginHeight)
	if node.Kind == "initial" || node.Kind == "final" {
		side := math.Max(dotNodeHeight, math.Ceil(math.Hypot(width, height)))
		return side, side
	}
	return math.Max(dotNodeWidth, width), math.Max(dotNodeHeight, height)
}

// dotLabelExtent is a label's text extent in points: its widest line by its
// lines' summed heights, the head in bold glyphs and the keyword line at 10pt.
func dotLabelExtent(node *Node) (width, height float64) {
	for i, line := range labelLines(node) {
		size, glyph := float64(dotFontSize), dotGlyphEm
		switch {
		case i == 0:
			glyph = dotBoldGlyphEm
		case i == 1 && node.Name != "":
			size = dotKeywordPointSize
		}
		width = math.Max(width, float64(utf8.RuneCountInString(line))*size*glyph)
		height += size * dotLineEm
	}
	return width, height
}

// dotPin pins a node at a pixel point: `pos="x,y!"` and `pin=true`.
func (w *dotWriter) dotPin(centre Point) string {
	return fmt.Sprintf("pos=%s, pin=true", dotQuote(w.dotPoint(centre)+"!"))
}

// dotAnchorAttributes is a cluster's anchor node: invisible and sizeless, the
// node an edge to or from the cluster names. A placed cluster pins it at the
// centre of its box, or at its corner while the box has no extent.
func (w *dotWriter) dotAnchorAttributes(node *Node) []string {
	attrs := slices.Clone(dotInvisibleAttributes)
	if node.Geometry != nil {
		low, high := clusterBox(node)
		attrs = append(attrs, w.dotPin(Point{X: (low.X + high.X) / 2, Y: (low.Y + high.Y) / 2}))
	}
	return attrs
}

// dotInvisibleAttributes draw a node as nothing: a cluster's anchor, a canvas corner.
var dotInvisibleAttributes = []string{"shape=point", "style=invis", "width=0", "height=0", `label=""`}

// dotClusterAttributes is a cluster's attribute statements: its label, a dashed
// border for an orthogonal region, its black border at the skin's thickness (a
// package's heavier than an element's), and its box as `bb` when it has an extent.
func (w *dotWriter) dotClusterAttributes(node *Node) []string {
	attrs := []string{dotLabel(node)}
	if node.Kind == "region" {
		attrs = append(attrs, "style=dashed")
	}
	attrs = append(attrs, "color=black", "penwidth="+dotClusterPenwidth(node))
	if g := node.Geometry; g != nil {
		if low, high := clusterBox(node); low != high {
			attrs = append(attrs, "bb="+dotQuote(w.dotPoint(Point{X: low.X, Y: high.Y})+","+w.dotPoint(Point{X: high.X, Y: low.Y})))
		}
		if g.Collapsed {
			attrs = append(attrs, `comment="collapsed"`)
		}
	}
	return attrs
}

// dotClusterPenwidth is a cluster's border thickness: the skin's package
// thickness for a package, its element thickness for every other cluster,
// regions included.
func dotClusterPenwidth(node *Node) string {
	if slices.Contains(strings.Fields(node.Kind), "package") {
		return "1.5"
	}
	return "0.5"
}

// dotClusterMargin is the space Graphviz keeps between a cluster's border and
// its members, in points.
const dotClusterMargin = 8

// clusterBox is a positioned cluster's box, top-left to bottom-right: the stated
// one, or its corner grown round its positioned members (the corner alone with none).
func clusterBox(node *Node) (topLeft, bottomRight Point) {
	g := node.Geometry
	topLeft = Point{X: g.X, Y: g.Y}
	if g.HasSize {
		return topLeft, Point{X: g.X + g.Width, Y: g.Y + g.Height}
	}
	bottomRight = topLeft
	for _, child := range node.Children {
		if child.Geometry == nil {
			continue
		}
		low, high := memberBox(child)
		if low == high {
			continue
		}
		topLeft = Point{X: math.Min(topLeft.X, low.X-dotClusterMargin), Y: math.Min(topLeft.Y, low.Y-dotClusterMargin)}
		bottomRight = Point{X: math.Max(bottomRight.X, high.X+dotClusterMargin), Y: math.Max(bottomRight.Y, high.Y+dotClusterMargin)}
	}
	return topLeft, bottomRight
}

// memberBox is the box a positioned member of a cluster takes up: a cluster's
// own box, a node's stated or label-fitted box.
func memberBox(node *Node) (topLeft, bottomRight Point) {
	if len(node.Children) > 0 {
		return clusterBox(node)
	}
	g := node.Geometry
	width, height := dotBox(node)
	return Point{X: g.X, Y: g.Y}, Point{X: g.X + width, Y: g.Y + height}
}

// dotEdgeAttributes is an edge's attribute list: its label, its kind's style (a
// connection drawn heavy, as the Pilot draws connectors), and its route as a
// `pos` spline when a Route gives waypoints.
func (w *dotWriter) dotEdgeAttributes(edge Edge) []string {
	var attrs []string
	if edge.Label != "" {
		attrs = append(attrs, dotLabelAttribute(dotQuote(edge.Label)))
	}
	switch edge.Kind {
	case EdgeConnection:
		attrs = append(attrs, "arrowhead=none", "penwidth=3")
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

// dotKeywordPointSize is the font size of the guillemet keyword line, under the
// 14pt Graphviz draws the rest of a label in.
const dotKeywordPointSize = 10

// dotLabel is a node's label attribute, an HTML-like label for nodes and clusters
// alike: the head in bold, the keyword line smaller and in italics, then the
// notes, one line each. A state's name is bold too, where the Pilot's is plain:
// the label's extent estimate (dotLabelExtent) and the other forms are kept to.
func dotLabel(node *Node) string {
	lines := labelLines(node)
	parts := []string{"<b>" + dotEscape(lines[0]) + "</b>"}
	for _, line := range lines[1:] {
		parts = append(parts, dotEscape(line))
	}
	if node.Name != "" {
		parts[1] = fmt.Sprintf(`<font point-size="%d"><i>%s</i></font>`, dotKeywordPointSize, parts[1])
	}
	return dotLabelAttribute("<" + strings.Join(parts, "<br/>") + ">")
}

// dotLabelAttribute is the `label=` attribute holding a quoted or HTML-like label.
func dotLabelAttribute(label string) string {
	return "label=" + label
}

// dotEscape writes text as HTML-like label content: `&`, `<`, `>`, `"` and `'`
// become entities so no name reads as markup, and a newline becomes `<br/>`.
func dotEscape(text string) string {
	return strings.ReplaceAll(html.EscapeString(text), "\n", "<br/>")
}

// dotQuote writes text as a double-quoted DOT string; every ID and label goes
// through it, so none is written bare. A newline becomes a `\n` line break.
func dotQuote(text string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)
	return `"` + replacer.Replace(text) + `"`
}
