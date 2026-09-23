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
//	EdgeBinding     ---      arrowhead=none
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
		boxes: map[string]nodeBox{}, fills: familyFills{palette: options.Palette, tree: r.Kind == KindTree}, labels: labelsOf(r.Roots)}
	w.placeNodes(r.Roots, r.Edges)
	for _, root := range r.Roots {
		if !w.tree {
			w.collectClusters(root, nil)
		}
		w.countPlaced(root)
		w.fills.collect(root)
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
	boxes     map[string]nodeBox  // node ID -> the box it is drawn in, for every node that has one
	nodes     int                 // nodes written, and how many are positioned
	placed    int
	routed    int         // edges with a route to write
	notices   []string    // geometry the form cannot draw
	fills     familyFills // the palette fills, by keyword family
	labels    labeller    // the node labels, headed relative to the roots' namespace
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
	dotNodeDefaults = []string{"shape=box", "style=filled", "fillcolor=white", dotColorAttr(dotLineColor),
		dotFontAttr(dotFontName), fmt.Sprintf("fontsize=%d", dotFontSize), "penwidth=0.5"}
	dotEdgeDefaults = []string{dotColorAttr(dotLineColor), dotFontAttr(dotFontName),
		fmt.Sprintf("fontsize=%d", dotEdgeFontPts), "penwidth=1"}
)

// dotColorAttr and dotFontAttr are the quoted `color` and `fontname` attributes.
func dotColorAttr(color string) string { return "color=" + dotQuote(color) }
func dotFontAttr(name string) string   { return "fontname=" + dotQuote(name) }

// countPlaced counts the nodes under node and those with a box to pin them in.
func (w *dotWriter) countPlaced(node *Node) {
	w.nodes++
	if _, ok := w.boxes[node.ID]; ok {
		w.placed++
	}
	for _, child := range node.Children {
		w.countPlaced(child)
	}
}

// nodeBox is where a node is drawn: its box in pixels, top-left to bottom-right.
type nodeBox struct {
	low, high Point
}

// centre is the middle of the box, the point Graphviz pins a node at.
func (b nodeBox) centre() Point {
	return Point{X: (b.low.X + b.high.X) / 2, Y: (b.low.Y + b.high.Y) / 2}
}

// routeEnd is where a route meets a node, and the waypoint it goes on to.
type routeEnd struct {
	at, next Point
}

// placeNodes finds the box of every node that has one: the one its Layout
// states; for a cluster with none, the one round its placed members; else the
// one the routes of its edges meet. Only a node with none of these is unplaced.
func (w *dotWriter) placeNodes(roots []*Node, edges []Edge) {
	ends := map[string][]routeEnd{}
	for _, edge := range edges {
		if n := len(edge.Route); n > 1 {
			ends[edge.From] = append(ends[edge.From], routeEnd{at: edge.Route[0], next: edge.Route[1]})
			ends[edge.To] = append(ends[edge.To], routeEnd{at: edge.Route[n-1], next: edge.Route[n-2]})
		}
	}
	for _, root := range roots {
		w.placeNode(root, ends)
	}
}

// placeNode records the box of node and of the nodes under it, members first
// so a cluster can be boxed round them.
func (w *dotWriter) placeNode(node *Node, ends map[string][]routeEnd) {
	for _, child := range node.Children {
		w.placeNode(child, ends)
	}
	cluster := len(node.Children) > 0 && !w.tree
	switch {
	case node.Geometry != nil && cluster:
		w.boxes[node.ID] = w.clusterBox(node)
	case node.Geometry != nil:
		w.boxes[node.ID] = w.statedBox(node)
	case cluster && w.membersBox(node) != nil:
		w.boxes[node.ID] = *w.membersBox(node)
	case len(ends[node.ID]) > 0:
		w.boxes[node.ID] = w.routedBox(node, ends[node.ID])
	}
}

// statedBox is the box a Layout places a plain node in: from its top-left
// corner, the stated size or the one fitted to its label.
func (w *dotWriter) statedBox(node *Node) nodeBox {
	g := node.Geometry
	width, height := w.labels.dotBox(node)
	return nodeBox{low: Point{X: g.X, Y: g.Y}, high: Point{X: g.X + width, Y: g.Y + height}}
}

// routedBox is the box a node with no Layout takes from the routes that meet
// it, sized to its label: centred one reach back from each route's end along
// its end segment, so the route meets the border, at their mean under several.
func (w *dotWriter) routedBox(node *Node, ends []routeEnd) nodeBox {
	width, height := w.labels.dotBox(node)
	var sum Point
	for _, end := range ends {
		centre := end.at
		if d := math.Hypot(end.next.X-end.at.X, end.next.Y-end.at.Y); d > 0 {
			ux, uy := (end.next.X-end.at.X)/d, (end.next.Y-end.at.Y)/d
			reach := dotReach(node, width, height, ux, uy)
			centre = Point{X: end.at.X - ux*reach, Y: end.at.Y - uy*reach}
		}
		sum.X += centre.X
		sum.Y += centre.Y
	}
	n := float64(len(ends))
	c := Point{X: sum.X / n, Y: sum.Y / n}
	return nodeBox{low: Point{X: c.X - width/2, Y: c.Y - height/2}, high: Point{X: c.X + width/2, Y: c.Y + height/2}}
}

// dotReach is the distance from the centre of a node's shape to its border along
// a unit direction: a round pseudo-state's radius, the edge of a box otherwise.
func dotReach(node *Node, width, height, ux, uy float64) float64 {
	if dotRound(node) {
		return width / 2
	}
	reach := math.Inf(1)
	if ux != 0 {
		reach = math.Min(reach, width/2/math.Abs(ux))
	}
	if uy != 0 {
		reach = math.Min(reach, height/2/math.Abs(uy))
	}
	return reach
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
	attrs := []string{dotFontAttr(dotFontName)}
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
// position and size when a box places it.
func (w *dotWriter) dotNodeAttributes(node *Node) []string {
	var attrs []string
	stated := node.Geometry != nil && node.Geometry.HasSize
	switch {
	case node.Kind == startKind:
		attrs = []string{"shape=point", "fillcolor=black", `label=""`}
	case stated && isSymbolKind(node.Kind):
		attrs = w.dotSymbolAttributes(node)
	case node.Kind == "initial" || node.Kind == "final":
		attrs = w.dotPseudostateAttributes(node)
	default:
		if !controlKinds[node.Kind] && !isDefinitionKind(node.Kind) {
			attrs = append(attrs, `style="rounded,filled"`)
		}
		if w.fills.filled(node) {
			attrs = append(attrs, "fillcolor="+dotQuote(w.fills.fill(node)), dotColorAttr(w.fills.color(node)), "penwidth=1")
		}
		if stated {
			attrs = append(attrs, w.labels.dotFittedLabel(node, node.Geometry.Width, node.Geometry.Height))
		} else {
			attrs = append(attrs, w.labels.dotLabel(node))
		}
	}
	if box, ok := w.boxes[node.ID]; ok {
		width, height := w.labels.dotBox(node)
		attrs = append(attrs, w.dotPin(box.centre()))
		if node.Kind != startKind {
			attrs = append(attrs, "width="+dotInches(width), "height="+dotInches(height))
		}
		if stated {
			attrs = append(attrs, "fixedsize=true")
		}
		if g := node.Geometry; g != nil && g.Collapsed {
			attrs = append(attrs, `comment="collapsed"`)
		}
	}
	return attrs
}

// isSymbolKind reports whether a kind has a notation symbol a stated box is drawn
// as, with no text inside it: the control and pseudo-state nodes and a port.
func isSymbolKind(kind string) bool {
	switch kind {
	case "initial", "final", terminateKind, "fork", "join", "merge", "decision", "choice", "junction":
		return true
	}
	return isPortKind(kind)
}

// dotRound reports whether a node is drawn round, so an edge reaches its border
// at its radius: the start point, the pseudo-states, and a stated junction or
// terminate action.
func dotRound(node *Node) bool {
	switch node.Kind {
	case startKind, "initial", "final":
		return true
	case "junction", terminateKind:
		return node.Geometry != nil && node.Geometry.HasSize
	}
	return false
}

// isPortKind reports whether a kind is a port usage: `port`, `ref port`, but no
// `port def`.
func isPortKind(kind string) bool {
	return slices.Contains(strings.Fields(kind), "port") && !isDefinitionKind(kind)
}

// dotSymbolAttributes draws a symbol kind in its stated box as the notation's symbol: a
// diamond, a filled bar or a port's square (the default box at the stated size), the
// filled dot or double ring. A given name is set outside as `xlabel`; a synthesized one is not drawn.
func (w *dotWriter) dotSymbolAttributes(node *Node) []string {
	var attrs []string
	switch node.Kind {
	case "decision", "merge", "choice":
		attrs = []string{"shape=diamond"}
	case "fork", "join":
		attrs = []string{"fillcolor=black"}
	case "initial", "junction":
		attrs = []string{"shape=circle", "fillcolor=black"}
	case "final", terminateKind:
		attrs = []string{"shape=doublecircle", "fillcolor=black"}
	default:
		if w.fills.filled(node) {
			attrs = append(attrs, "fillcolor="+dotQuote(w.fills.fill(node)), dotColorAttr(w.fills.color(node)), "penwidth=1")
		}
	}
	attrs = append(attrs, `label=""`)
	if node.Name != "" && !node.NameSynthesized {
		attrs = append(attrs, "xlabel="+dotQuote(w.labels.head(node)))
	}
	return attrs
}

// dotPseudostateSize is the diameter, in points, of an initial or final
// pseudo-state drawn as the UML filled dot, with no name to show: 0.2in.
const dotPseudostateSize = 14.4

// dotPseudostateAttributes is an initial or final node's shape and label: the
// UML filled black dot, or double ring, when it has no given name to show, a
// labelled circle when the rendering names it. A placed one keeps its placed size.
func (w *dotWriter) dotPseudostateAttributes(node *Node) []string {
	shape := "shape=circle"
	if node.Kind == "final" {
		shape = "shape=doublecircle"
	}
	if node.Name != "" && !node.NameSynthesized {
		return []string{shape, w.labels.dotLabel(node)}
	}
	attrs := []string{shape, "fillcolor=black", `label=""`}
	if _, ok := w.boxes[node.ID]; !ok {
		attrs = append(attrs, "width="+dotInches(dotPseudostateSize))
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
func (l labeller) dotBox(node *Node) (width, height float64) {
	if g := node.Geometry; g != nil && g.HasSize {
		return g.Width, g.Height
	}
	if node.Kind == startKind {
		return dotPointSize, dotPointSize
	}
	if (node.Kind == "initial" || node.Kind == "final") && node.Name == "" {
		return dotPseudostateSize, dotPseudostateSize
	}
	width, height = l.dotLabelExtent(node)
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
func (l labeller) dotLabelExtent(node *Node) (width, height float64) {
	for i, line := range l.lines(node) {
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

// dotFitFloor is the smallest font size, in points, a stated box's label shrinks to.
const dotFitFloor = 8

// dotFittedLabel is a node's label composed to fit a stated box: the head wrapped
// at the box's width and shrunk from the default size to the largest at which it
// fits, the keyword and detail lines after it while height remains. A head too
// tall even at the floor is cut to the lines that fit and ellipsized.
func (l labeller) dotFittedLabel(node *Node, width, height float64) string {
	lines := l.lines(node)
	size, head, fits := dotFitHead(lines[0], width, height)
	parts := []string{dotSized(size, "<b>"+dotEscapeLines(head)+"</b>")}
	left := height - float64(len(head))*size*dotLineEm
	for i := 1; fits && i < len(lines); i++ {
		keyword := i == 1 && node.Name != ""
		lineSize := size
		if keyword {
			lineSize = math.Round(size * dotKeywordPointSize / dotFontSize)
		}
		wrapped := dotWrap(lines[i], dotRunesAcross(width, lineSize, dotGlyphEm))
		used := float64(len(wrapped)) * lineSize * dotLineEm
		if used > left {
			break
		}
		left -= used
		text := dotEscapeLines(wrapped)
		if keyword {
			text = "<i>" + text + "</i>"
		}
		parts = append(parts, dotSized(lineSize, text))
	}
	return dotLabelAttribute("<" + strings.Join(parts, "<br/>") + ">")
}

// dotFitHead wraps a head line into a box at the largest font size, from the
// default down to the floor, at which it fits; when none does, the floor's
// wrapping is cut to the lines the height holds, the last ellipsized.
func dotFitHead(head string, width, height float64) (size float64, lines []string, fits bool) {
	for size = dotFontSize; size >= dotFitFloor; size-- {
		lines = dotWrap(head, dotRunesAcross(width, size, dotBoldGlyphEm))
		if float64(len(lines))*size*dotLineEm <= height {
			return size, lines, true
		}
	}
	size = dotFitFloor
	across := dotRunesAcross(width, size, dotBoldGlyphEm)
	lines = dotWrap(head, across)
	down := max(1, int(height/(size*dotLineEm)))
	if len(lines) > down {
		lines = lines[:down]
		last := []rune(lines[down-1])
		lines[down-1] = string(last[:max(0, min(len(last), across-1))]) + "…"
	}
	return size, lines, false
}

// dotRunesAcross is how many glyphs of a font size fit across a width, one at
// least so a line can be written at all.
func dotRunesAcross(width, size, glyph float64) int {
	return max(1, int(width/(size*glyph)))
}

// dotWrap word-wraps text to at most across runes a line, breaking a word longer
// than that at the rune it overruns.
func dotWrap(text string, across int) []string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		if line != "" && utf8.RuneCountInString(line)+1+utf8.RuneCountInString(word) <= across {
			line += " " + word
			continue
		}
		if line != "" {
			lines = append(lines, line)
		}
		runes := []rune(word)
		for len(runes) > across {
			lines = append(lines, string(runes[:across]))
			runes = runes[across:]
		}
		line = string(runes)
	}
	if line != "" || len(lines) == 0 {
		lines = append(lines, line)
	}
	return lines
}

// dotEscapeLines joins lines as HTML-like label text, each escaped.
func dotEscapeLines(lines []string) string {
	escaped := make([]string, len(lines))
	for i, line := range lines {
		escaped[i] = dotEscape(line)
	}
	return strings.Join(escaped, "<br/>")
}

// dotSized wraps label text in a `<font point-size>` when its size is not the
// node's 14pt default.
func dotSized(size float64, text string) string {
	if size == dotFontSize {
		return text
	}
	return fmt.Sprintf(`<font point-size="%s">%s</font>`, formatCoord(size), text)
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
	if box, ok := w.boxes[node.ID]; ok {
		attrs = append(attrs, w.dotPin(box.centre()))
	}
	return attrs
}

// dotInvisibleAttributes draw a node as nothing: a cluster's anchor, a canvas corner.
var dotInvisibleAttributes = []string{"shape=point", "style=invis", "width=0", "height=0", `label=""`}

// dotClusterAttributes is a cluster's attribute statements: its label, a dashed
// border for an orthogonal region, its black border at the skin's thickness (a
// package's heavier than an element's), and its box as `bb` when it has an extent.
func (w *dotWriter) dotClusterAttributes(node *Node) []string {
	attrs := []string{w.labels.dotLabel(node)}
	if node.Kind == "region" {
		attrs = append(attrs, "style=dashed")
	}
	attrs = append(attrs, "color=black", "penwidth="+dotClusterPenwidth(node))
	if box, ok := w.boxes[node.ID]; ok && box.low != box.high {
		attrs = append(attrs, "bb="+dotQuote(w.dotPoint(Point{X: box.low.X, Y: box.high.Y})+","+w.dotPoint(Point{X: box.high.X, Y: box.low.Y})))
	}
	if g := node.Geometry; g != nil && g.Collapsed {
		attrs = append(attrs, `comment="collapsed"`)
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

// clusterBox is a Layout-positioned cluster's box: the stated one, or its
// corner grown round its placed members (the corner alone with none).
func (w *dotWriter) clusterBox(node *Node) nodeBox {
	g := node.Geometry
	corner := Point{X: g.X, Y: g.Y}
	if g.HasSize {
		return nodeBox{low: corner, high: Point{X: g.X + g.Width, Y: g.Y + g.Height}}
	}
	box := nodeBox{low: corner, high: corner}
	if members := w.membersBox(node); members != nil {
		box = box.union(*members)
	}
	return box
}

// membersBox is the box round a cluster's placed members, a margin out from
// them; nil while no member has a box with an extent.
func (w *dotWriter) membersBox(node *Node) *nodeBox {
	var box *nodeBox
	for _, child := range node.Children {
		member, ok := w.boxes[child.ID]
		if !ok || member.low == member.high {
			continue
		}
		grown := nodeBox{low: Point{X: member.low.X - dotClusterMargin, Y: member.low.Y - dotClusterMargin},
			high: Point{X: member.high.X + dotClusterMargin, Y: member.high.Y + dotClusterMargin}}
		if box == nil {
			box = &grown
			continue
		}
		*box = box.union(grown)
	}
	return box
}

// union is the smallest box holding both b and other.
func (b nodeBox) union(other nodeBox) nodeBox {
	return nodeBox{low: Point{X: math.Min(b.low.X, other.low.X), Y: math.Min(b.low.Y, other.low.Y)},
		high: Point{X: math.Max(b.high.X, other.high.X), Y: math.Max(b.high.Y, other.high.Y)}}
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
	case EdgeBinding:
		attrs = append(attrs, "arrowhead=none")
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
func (l labeller) dotLabel(node *Node) string {
	lines := l.lines(node)
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
