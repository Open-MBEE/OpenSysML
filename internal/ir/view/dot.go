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
// has the rules. A drawing some Layout positions leaves the nodes none does
// undrawn, or sets them in a strip below it when asked (Options.Unplaced).
//
// The drawing is in the Standard B&W style of the SysML v2 Pilot visualizer
// (docs/project/view-rendering-forms.md#style): Helvetica, white fills, thin
// #181818 lines, square definitions and rounded usages, the keyword line in
// italics. Defaults are written once as `graph`, `node` and `edge` statements;
// a node or edge states only what it deviates in, its style before its geometry.
// Options.Style draws the Cameo look instead (StyleCameo); a node's or edge's
// own Style annotation wins over either, and notes are `shape=note` nodes with
// a dashed anchor edge.
//
// A kind with no DOT counterpart — sequence, table — is a *WrongFormError.
func (r *Rendering) DOT() (string, error) {
	return r.DOTWith(Options{})
}

// DOTWith is the DOT form written with options: laid out in the stated
// direction, as `rankdir` (the empty direction leaves the engine's default and
// writes no `rankdir`), filled from the stated palette by keyword family (the
// empty palette draws in black and white), and with the nodes a positioned
// drawing leaves unplaced undrawn or set in a strip below it. The layout is
// the same whatever the palette: it changes fills and borders alone.
func (r *Rendering) DOTWith(options Options) (string, error) {
	if !r.Kind.SupportsForm(FormDot) {
		return "", &WrongFormError{Form: FormDot, Kind: r.Kind, View: r.View}
	}
	if err := options.Palette.check(); err != nil {
		return "", err
	}
	if err := options.Unplaced.check(); err != nil {
		return "", err
	}
	if err := options.Style.check(); err != nil {
		return "", err
	}
	direction := options.Direction
	w := newDOTWriter(r, options)
	for _, note := range r.Notes {
		if note.Anchor != "" && w.draws(note.Anchor) && w.clipped(note.Anchor, "") {
			w.compound = true
		}
	}
	edges := r.Edges
	if w.placement.partial() {
		edges = w.settleUnplaced(r.Roots, r.Edges, options.Unplaced)
	}
	edges = w.drawnEdges(edges)
	for _, edge := range edges {
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
	fmt.Fprintf(b, "  node [%s];\n", strings.Join(w.skin.nodeDefaults(), ", "))
	fmt.Fprintf(b, "  edge [%s];\n", strings.Join(w.skin.edgeDefaults(), ", "))
	w.writeCanvas()
	if r.Empty() && len(r.Notes) == 0 {
		fmt.Fprintf(b, "  \"empty\" [shape=plaintext, label=%s];\n", dotQuote(r.EmptyReason()))
		b.WriteString("}\n")
		return b.String(), nil
	}
	depth := 1
	if w.skin.cameo {
		w.openFrame(r, edges)
		depth = 2
	}
	for _, root := range w.drawOrder(r.Roots) {
		w.writeNode(root, depth)
	}
	w.writeNotes(r.Notes, depth)
	if w.skin.cameo {
		b.WriteString("  }\n")
	}
	for _, edge := range edges {
		w.writeEdge(edge.From, edge.To, w.dotEdgeAttributes(edge))
	}
	w.writeAnchors(r.Notes, edges)
	b.WriteString("}\n")
	return b.String(), nil
}

// openFrame opens the Cameo diagram frame: a cluster round the whole drawing,
// headed `kind [Type] Owner [ Name ]` at its top left, on the canvas when one
// is sized, else round everything placed.
func (w *dotWriter) openFrame(r *Rendering, edges []Edge) {
	fmt.Fprintf(&w.b, "  subgraph %s {\n", dotQuote(dotFrameCluster))
	fmt.Fprintf(&w.b, "    label=<%s>;\n", w.frameHeader(r))
	for _, attr := range []string{"labeljust=l", "labelloc=t", "fontsize=" + formatCoord(cameoFontSize), dotColorAttr(cameoFrameColor), dotPenWidthOne, "margin=" + formatCoord(dotFrameMargin)} {
		fmt.Fprintf(&w.b, "    %s;\n", attr)
	}
	if w.placement.count > 0 {
		box := w.extent(edges)
		if c := w.canvas; c == nil || !c.HasSize {
			box = nodeBox{low: Point{X: box.low.X - dotFrameMargin, Y: box.low.Y - dotFrameMargin - dotFrameHeader},
				high: Point{X: box.high.X + dotFrameMargin, Y: box.high.Y + dotFrameMargin}}
		}
		fmt.Fprintf(&w.b, "    bb=%s;\n", dotQuote(w.dotBB(box)))
	}
}

// The frame cluster's name, the margin it keeps round the drawing and the
// height of its header line, in points.
const (
	dotFrameCluster = "cluster_frame"
	dotFrameMargin  = 8
	dotFrameHeader  = 20
)

// frameHeader is the Cameo frame's header text as HTML-like label content:
// the diagram kind in bold, the context element's type in brackets and its
// name, then the diagram's name in brackets.
func (w *dotWriter) frameHeader(r *Rendering) string {
	parts := []string{"<b>" + cameoFrameKind(r.Kind) + "</b>"}
	if len(r.Roots) > 0 {
		root := r.Roots[0]
		if typ := cameoFrameType(root.Kind); typ != "" {
			parts = append(parts, "["+dotEscape(typ)+"]")
		}
		if name := shown(root); name != "" {
			parts = append(parts, dotEscape(w.labels.name(root)))
		}
	}
	if r.View != "" {
		parts = append(parts, "[ "+dotEscape(lastName(r.View))+" ]")
	}
	return strings.Join(parts, " ")
}

// lastName is the last segment of a qualified name, the name itself otherwise.
func lastName(qualified string) string {
	if i := strings.LastIndex(qualified, "::"); i >= 0 {
		return qualified[i+2:]
	}
	return qualified
}

// dotBB is a pixel box as a Graphviz `bb`, lower-left then upper-right.
func (w *dotWriter) dotBB(box nodeBox) string {
	return w.dotPoint(Point{X: box.low.X, Y: box.high.Y}) + "," + w.dotPoint(Point{X: box.high.X, Y: box.low.Y})
}

// newDOTWriter is the writer for r with every node that has a box placed in
// it, the clusters and the palette's families collected.
func newDOTWriter(r *Rendering, options Options) *dotWriter {
	skin := skinOf(options.Style)
	w := &dotWriter{tree: r.Kind == KindTree, clusters: map[string]bool{}, enclosing: map[string][]string{}, canvas: r.Canvas,
		placement: placeRendering(r), drawn: map[string]bool{}, boxes: map[string]nodeBox{}, omitted: map[string]bool{},
		fills: familyFills{palette: options.Palette, tree: r.Kind == KindTree}, labels: labelsOf(r.Roots), skin: skin}
	w.collectDrawn(r.Roots)
	w.labels.skin = skin
	w.placeNodes(r.Roots, r.Edges)
	w.placeNotes(r.Notes)
	for _, root := range r.Roots {
		if !w.tree {
			w.collectClusters(root, nil)
		}
		w.fills.collect(root)
	}
	return w
}

// settleUnplaced settles the nodes a positioned drawing leaves unplaced, as
// asked: boxed in a strip below the drawing, or left undrawn with the edges
// at them; either is noticed. The edges left to write are returned.
func (w *dotWriter) settleUnplaced(roots []*Node, edges []Edge, unplaced Unplaced) []Edge {
	count := w.placement.unplaced()
	if unplaced == UnplacedStrip {
		w.stripUnplaced(roots, edges)
		w.notices = append(w.notices, fmt.Sprintf("%d node(s) without a position, drawn in a strip below the drawing", count))
		return edges
	}
	w.omitUnplaced(roots)
	kept, dropped := w.placement.keptEdges(edges)
	w.notices = append(w.notices, w.placement.omitNotice(dropped))
	return kept
}

// omitUnplaced marks every node under nodes that has no place as undrawn.
func (w *dotWriter) omitUnplaced(nodes []*Node) {
	for _, node := range nodes {
		if !w.placement.placed[node.ID] {
			w.omitted[node.ID] = true
		}
		w.omitUnplaced(node.Children)
	}
}

// dotStripGap is the space, in pixels, between the drawing and the strip of
// unplaced nodes below it, and between the boxes in the strip.
const dotStripGap = 24

// stripUnplaced boxes the unplaced nodes in rows in a strip below everything
// the drawing places — the canvas, every box, every route — as wide as that.
func (w *dotWriter) stripUnplaced(roots []*Node, edges []Edge) {
	w.stated = make(map[string]bool, len(w.boxes))
	for id := range w.boxes {
		w.stated[id] = true
	}
	extent := w.extent(edges)
	corner := Point{X: extent.low.X, Y: extent.high.Y + dotStripGap}
	w.packRows(w.unplacedTops(roots), corner, extent.high.X-extent.low.X)
}

// extent is the box round everything placed: the sized canvas, every box and
// every route's waypoints.
func (w *dotWriter) extent(edges []Edge) nodeBox {
	var extent *nodeBox
	add := func(box nodeBox) {
		if extent == nil {
			extent = &box
			return
		}
		*extent = extent.union(box)
	}
	if c := w.canvas; c != nil && c.HasSize {
		add(nodeBox{high: Point{X: c.Width, Y: c.Height}})
	}
	for _, box := range w.boxes {
		add(box)
	}
	for _, box := range w.noteBoxes {
		add(box)
	}
	for _, edge := range edges {
		for _, p := range edge.Route {
			add(nodeBox{low: p, high: p})
		}
	}
	return *extent
}

// unplacedTops are the unplaced nodes the strip packs as items, in walk order:
// an unplaced cluster packs its own members, a tree's nodes are each an item.
func (w *dotWriter) unplacedTops(nodes []*Node) []*Node {
	var tops []*Node
	for _, node := range nodes {
		if !w.stated[node.ID] {
			tops = append(tops, node)
			if !w.tree {
				continue
			}
		}
		tops = append(tops, w.unplacedTops(node.Children)...)
	}
	return tops
}

// packRows boxes nodes in rows from corner, a gap apart, wrapping before a box
// would overrun across (a box wider than that has a row to itself); the box
// round them all. Corners fall on whole pixels.
func (w *dotWriter) packRows(nodes []*Node, corner Point, across float64) nodeBox {
	corner = Point{X: math.Ceil(corner.X), Y: math.Ceil(corner.Y)}
	packed := nodeBox{low: corner, high: corner}
	at, rowHeight := corner, 0.0
	for _, node := range nodes {
		box := w.stripBox(node, at, across)
		if box.high.X > corner.X+across && at.X > corner.X {
			at, rowHeight = Point{X: corner.X, Y: math.Ceil(at.Y + rowHeight + dotStripGap)}, 0
			box = w.stripBox(node, at, across)
		}
		packed = packed.union(box)
		rowHeight = math.Max(rowHeight, box.high.Y-box.low.Y)
		at.X = math.Ceil(box.high.X + dotStripGap)
	}
	return packed
}

// stripBox boxes an unplaced node with its top-left corner at corner: a plain
// node in its label's box, a cluster round its title and its members packed in
// rows below it, a margin about them.
func (w *dotWriter) stripBox(node *Node, corner Point, across float64) nodeBox {
	if len(node.Children) == 0 || w.tree {
		width, height := w.labels.dotBox(node)
		box := nodeBox{low: corner, high: Point{X: corner.X + width, Y: corner.Y + height}}
		w.boxes[node.ID] = box
		return box
	}
	_, title := w.labels.dotLabelExtent(node)
	inner := Point{X: corner.X + dotClusterMargin, Y: corner.Y + title + dotClusterMargin}
	members := w.packRows(w.unplacedTops(node.Children), inner, math.Max(across-2*dotClusterMargin, 1))
	box := nodeBox{low: corner, high: Point{X: members.high.X + dotClusterMargin, Y: members.high.Y + dotClusterMargin}}
	w.boxes[node.ID] = box
	return box
}

// dotWriter holds what one rendering's DOT form needs across nodes and edges.
type dotWriter struct {
	b         strings.Builder
	tree      bool                // containment as edges, not clusters
	clusters  map[string]bool     // node IDs drawn as clusters
	enclosing map[string][]string // node ID -> the cluster IDs around it
	compound  bool                // an edge is clipped at a cluster
	canvas    *Canvas             // the surface positions are flipped against
	placement *placement          // which nodes have a place, shared with every form
	drawn     map[string]bool     // node ID -> in the rendering's trees
	boxes     map[string]nodeBox  // node ID -> the box it is drawn in, for every node that has one
	stated    map[string]bool     // node IDs the drawing itself boxes, once a strip adds boxes of its own
	omitted   map[string]bool     // node IDs left undrawn for want of a box
	routed    int                 // edges with a route to write
	notices   []string            // geometry the form cannot draw
	fills     familyFills         // the palette fills, by keyword family
	labels    labeller            // the node labels, headed relative to the roots' namespace
	skin      dotSkin             // the drawing style's defaults
	noteBoxes []nodeBox           // where each positioned note is drawn, by index in Rendering.Notes
}

// The Standard B&W style, after the sysmlbw PlantUML skin: Helvetica text,
// white fills, thin #181818 lines, edge text a point smaller than node text.
const (
	dotFontName      = "Helvetica"
	dotLineColor     = "#181818"
	dotEdgeFontPts   = 13
	dotPenWidthOne   = "penwidth=1"
	dotFillBlack     = "fillcolor=black"
	dotArrowheadNone = "arrowhead=none"
)

// dotSkin is a drawing style's defaults: what the `node` and `edge` statements
// open the digraph with, and what a node or edge is measured against.
type dotSkin struct {
	cameo    bool    // the Cameo look; false is the Pilot Standard B&W
	font     string  // the text font
	fontSize float64 // node text, in points
	edgePts  float64 // edge text, in points
	line     string  // the default pen
	text     string  // the default text colour; empty is black
}

// skinOf is the skin of a drawing style; the empty style is the Pilot's.
func skinOf(style DrawingStyle) dotSkin {
	if style == StyleCameo {
		return dotSkin{cameo: true, font: cameoFontName, fontSize: cameoFontSize, edgePts: cameoSmallPts, line: cameoLineColor, text: cameoTextColor}
	}
	return dotSkin{font: dotFontName, fontSize: dotFontSize, edgePts: dotEdgeFontPts, line: dotLineColor}
}

// nodeDefaults is the `node` statement the digraph opens with; a node lists only
// what it deviates in. The Cameo skin fills with a block's orange gradient.
func (s dotSkin) nodeDefaults() []string {
	if s.cameo {
		return []string{"shape=box", "style=filled", "fillcolor=" + dotQuote(cameoBlockFill), "gradientangle=0", dotColorAttr(cameoBlockLine),
			dotFontAttr(s.font), "fontsize=" + formatCoord(s.fontSize), "fontcolor=" + dotQuote(s.text), dotPenWidthOne}
	}
	return []string{"shape=box", "style=filled", "fillcolor=white", dotColorAttr(s.line),
		dotFontAttr(s.font), "fontsize=" + formatCoord(s.fontSize), "penwidth=0.5"}
}

// edgeDefaults is the `edge` statement the digraph opens with. Cameo draws
// open arrowheads, which a connection or containment edge then takes off.
func (s dotSkin) edgeDefaults() []string {
	if s.cameo {
		return []string{dotColorAttr(cameoEdgeColor), dotFontAttr(s.font), "fontsize=" + formatCoord(s.edgePts),
			"fontcolor=" + dotQuote(s.text), dotPenWidthOne, "arrowhead=open"}
	}
	return []string{dotColorAttr(s.line), dotFontAttr(s.font), "fontsize=" + formatCoord(s.edgePts), dotPenWidthOne}
}

// fill is the skin's fill and pen for a plain node of the kind, beyond the
// node defaults: Cameo's state and action gradients; the Pilot's are the defaults.
func (s dotSkin) fill(kind string) []string {
	if !s.cameo {
		return nil
	}
	fill, pen := cameoFill(kind)
	if fill == cameoBlockFill {
		return nil
	}
	return []string{"fillcolor=" + dotQuote(fill), dotColorAttr(pen)}
}

// rounded reports whether the skin rounds a plain node of the kind: the Pilot
// rounds usages, Cameo states and actions.
func (s dotSkin) rounded(kind string) bool {
	if controlKinds[kind] {
		return false
	}
	if s.cameo {
		return cameoRounded(kind)
	}
	return !isDefinitionKind(kind)
}

// dotStyleAttributes is what a Style annotation sets on a node or edge, after
// the skin: a solid fill, the pen, the text colour, the font and its size. The
// fill and pen are left out when the palette branch has written them already.
func dotStyleAttributes(style *Style, coloured bool) []string {
	if style == nil {
		return nil
	}
	var attrs []string
	if style.Fill != "" && !coloured {
		attrs = append(attrs, "fillcolor="+dotQuote(style.Fill))
	}
	if style.Line != "" && !coloured {
		attrs = append(attrs, dotColorAttr(style.Line))
	}
	if style.Text != "" {
		attrs = append(attrs, "fontcolor="+dotQuote(style.Text))
	}
	if style.Font != "" {
		attrs = append(attrs, dotFontAttr(style.Font))
	}
	if style.FontSize > 0 {
		attrs = append(attrs, "fontsize="+formatCoord(style.FontSize))
	}
	return attrs
}

// dotStyledLabel wraps an HTML-like `label=<…>` attribute in `<b>` or `<i>`
// as a Style asks; a quoted label and a label of no style are left alone.
func dotStyledLabel(attr string, style *Style) string {
	if style == nil || !(style.Bold || style.Italic) || !strings.HasPrefix(attr, "label=<") {
		return attr
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(attr, "label=<"), ">")
	if style.Italic && !dotWrapped(inner, "i") {
		inner = "<i>" + inner + "</i>"
	}
	if style.Bold && !dotWrapped(inner, "b") {
		inner = "<b>" + inner + "</b>"
	}
	return "label=<" + inner + ">"
}

// dotWrapped reports whether an HTML-like label is one element of the tag.
func dotWrapped(inner, tag string) bool {
	open, close := "<"+tag+">", "</"+tag+">"
	return strings.HasPrefix(inner, open) && strings.HasSuffix(inner, close) &&
		!strings.Contains(inner[len(open):len(inner)-len(close)], close)
}

// dotOverridden drops every attribute a later one of the same name replaces,
// so a Style's fill or pen stands alone rather than after the skin's.
func dotOverridden(attrs []string) []string {
	kept := attrs[:0:0]
	for i, attr := range attrs {
		name, _, _ := strings.Cut(attr, "=")
		later := false
		for _, other := range attrs[i+1:] {
			if otherName, _, _ := strings.Cut(other, "="); otherName == name {
				later = true
				break
			}
		}
		if !later {
			kept = append(kept, attr)
		}
	}
	return kept
}

// dotColorAttr and dotFontAttr are the quoted `color` and `fontname` attributes.
func dotColorAttr(color string) string { return "color=" + dotQuote(color) }
func dotFontAttr(name string) string   { return "fontname=" + dotQuote(name) }

// nodeBox is where a node is drawn, top-left to bottom-right in pixels; stated
// when the Layout gives its size and not only its corner.
type nodeBox struct {
	low, high Point
	stated    bool
}

// centre is the middle of the box, the point Graphviz pins a node at.
func (b nodeBox) centre() Point {
	return Point{X: (b.low.X + b.high.X) / 2, Y: (b.low.Y + b.high.Y) / 2}
}

// encloses reports whether other lies within b and is the smaller of the two.
func (b nodeBox) encloses(other nodeBox) bool {
	return other != b && other.low.X >= b.low.X && other.low.Y >= b.low.Y &&
		other.high.X <= b.high.X && other.high.Y <= b.high.Y
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
	var derived []*Node
	for _, root := range roots {
		w.placeNode(root, ends, &derived)
	}
	if len(derived) == 0 {
		return
	}
	for _, node := range derived {
		w.boxes[node.ID] = w.derivedBox(node, edges)
	}
	for _, root := range roots {
		w.reboxClusters(root)
	}
}

// reboxClusters recomputes, members first, the box of every cluster under node
// boxed round its members, once nodes placed by their neighbours have theirs.
func (w *dotWriter) reboxClusters(node *Node) {
	for _, child := range node.Children {
		w.reboxClusters(child)
	}
	if _, ok := w.boxes[node.ID]; !ok || len(node.Children) == 0 || w.tree {
		return
	}
	switch {
	case node.Geometry != nil && !node.Geometry.HasSize:
		w.boxes[node.ID] = w.clusterBox(node)
	case node.Geometry == nil:
		if members := w.membersBox(node); members != nil {
			w.boxes[node.ID] = *members
		}
	}
}

// placeNode records the box of every placed node under and including node,
// members first so a cluster can be boxed round them; a node placed by its
// neighbours is collected for boxing once every neighbour has a box.
func (w *dotWriter) placeNode(node *Node, ends map[string][]routeEnd, derived *[]*Node) {
	for _, child := range node.Children {
		w.placeNode(child, ends, derived)
	}
	if !w.placement.placed[node.ID] {
		return
	}
	cluster := len(node.Children) > 0 && !w.tree
	switch {
	case w.placement.derived[node.ID]:
		*derived = append(*derived, node)
	case node.Geometry != nil && cluster:
		w.boxes[node.ID] = w.clusterBox(node)
	case node.Geometry != nil:
		w.boxes[node.ID] = w.statedBox(node)
	case cluster && w.membersBox(node) != nil:
		w.boxes[node.ID] = *w.membersBox(node)
	default:
		w.boxes[node.ID] = w.routedBox(node, ends[node.ID])
	}
}

// statedBox is the box a Layout places a plain node in: from its top-left
// corner, the stated size or the one fitted to its label.
func (w *dotWriter) statedBox(node *Node) nodeBox {
	g := node.Geometry
	width, height := w.labels.dotBox(node)
	return nodeBox{low: Point{X: g.X, Y: g.Y}, high: Point{X: g.X + width, Y: g.Y + height}, stated: g.HasSize}
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

// dotEndGap is the space, in pixels, between a start or final node placed by
// its neighbours and the nearest of them.
const dotEndGap = 20

// derivedBox is the box of a start or final node its neighbours place: centred
// over the nodes its edges go to and dotEndGap above the topmost, or under the
// nodes its edges come from and dotEndGap below the lowest.
func (w *dotWriter) derivedBox(node *Node, edges []Edge) nodeBox {
	width, height := w.labels.dotBox(node)
	start := node.Kind == startKind || node.Kind == "initial"
	var sum, n float64
	edge := math.Inf(1)
	if !start {
		edge = math.Inf(-1)
	}
	for _, e := range edges {
		other, ok := edgeOtherEnd(e, node.ID)
		if !ok {
			continue
		}
		box, ok := w.boxes[other]
		if !ok {
			continue
		}
		sum += box.centre().X
		n++
		if start {
			edge = math.Min(edge, box.low.Y)
		} else {
			edge = math.Max(edge, box.high.Y)
		}
	}
	cx := sum / n
	if start {
		return nodeBox{low: Point{X: cx - width/2, Y: edge - dotEndGap - height}, high: Point{X: cx + width/2, Y: edge - dotEndGap}}
	}
	return nodeBox{low: Point{X: cx - width/2, Y: edge + dotEndGap}, high: Point{X: cx + width/2, Y: edge + dotEndGap + height}}
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
// Every node drawn in a positioned drawing is pinned, so plain `neato` is never named.
func (w *dotWriter) engine() string {
	switch {
	case w.placement.count == 0:
		return "dot"
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

// collectDrawn records the ID of every node in the rendering's trees.
func (w *dotWriter) collectDrawn(nodes []*Node) {
	for _, node := range nodes {
		w.drawn[node.ID] = true
		w.collectDrawn(node.Children)
	}
}

// draws reports whether the DOT declares a node for id: it is in the
// rendering's trees and not left undrawn for want of a place.
func (w *dotWriter) draws(id string) bool { return w.drawn[id] && !w.omitted[id] }

// drawnEdges is edges without one an end the drawing holds no node for leaves
// undrawable — an anchor of a note on a node the rendering dropped, say — each
// noticed rather than written, DOT having no node of that ID to end it at.
func (w *dotWriter) drawnEdges(edges []Edge) []Edge {
	kept := make([]Edge, 0, len(edges))
	for _, edge := range edges {
		if w.draws(edge.From) && w.draws(edge.To) {
			kept = append(kept, edge)
			continue
		}
		w.notices = append(w.notices, fmt.Sprintf("edge %s->%s has an end the rendering draws no node for; no edge is drawn", edge.From, edge.To))
	}
	return kept
}

// dotClusterName is the subgraph name of the cluster drawn for a node.
func dotClusterName(id string) string { return "cluster_" + id }

// graphAttributes is the graph attribute list: the font, the layout direction
// when one is stated, `compound` when an edge is clipped at a cluster, and the
// pixel scale when a node is positioned.
func (w *dotWriter) graphAttributes(direction Direction) []string {
	attrs := []string{dotFontAttr(w.skin.font)}
	if w.skin.text != "" {
		attrs = append(attrs, "fontcolor="+dotQuote(w.skin.text))
	}
	if direction != "" {
		attrs = append(attrs, "rankdir="+string(direction))
	}
	if w.compound {
		attrs = append(attrs, "compound=true")
	}
	if w.placement.count > 0 {
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
	if c == nil || !c.HasSize || w.placement.count == 0 {
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

// drawOrder moves a placed sibling ahead of the siblings its box encloses, which
// Graphviz would otherwise paint it over, as it paints in file order.
func (w *dotWriter) drawOrder(nodes []*Node) []*Node {
	ordered := make([]*Node, 0, len(nodes))
	for _, node := range nodes {
		at := len(ordered)
		if box, ok := w.boxes[node.ID]; ok {
			at = slices.IndexFunc(ordered, func(other *Node) bool {
				inner, ok := w.boxes[other.ID]
				return ok && box.encloses(inner)
			})
			if at < 0 {
				at = len(ordered)
			}
		}
		ordered = slices.Insert(ordered, at, node)
	}
	return ordered
}

// writeNode writes one node: a cluster holding its children, a plain node
// otherwise. A tree writes the node and an edge to each child instead. An
// omitted node writes nothing but the nodes under it, in its place.
func (w *dotWriter) writeNode(node *Node, depth int) {
	indent := strings.Repeat("  ", depth)
	if w.omitted[node.ID] {
		for _, child := range w.drawOrder(node.Children) {
			w.writeNode(child, depth)
		}
		return
	}
	if len(node.Children) == 0 || w.tree {
		fmt.Fprintf(&w.b, "%s%s [%s];\n", indent, dotQuote(node.ID), strings.Join(w.dotNodeAttributes(node), ", "))
		for _, child := range w.drawOrder(node.Children) {
			w.writeNode(child, depth)
			if !w.omitted[child.ID] {
				w.writeEdge(node.ID, child.ID, w.skin.containmentAttributes())
			}
		}
		return
	}
	fmt.Fprintf(&w.b, "%ssubgraph %s {\n", indent, dotQuote(dotClusterName(node.ID)))
	for _, attr := range w.dotClusterAttributes(node) {
		fmt.Fprintf(&w.b, "%s  %s;\n", indent, attr)
	}
	fmt.Fprintf(&w.b, "%s  %s [%s];\n", indent, dotQuote(node.ID), strings.Join(w.dotAnchorAttributes(node), ", "))
	for _, child := range w.drawOrder(node.Children) {
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
		attrs = []string{"shape=point", dotFillBlack, `label=""`}
	case isSymbolKind(node.Kind) && (stated || w.skin.cameo && !isPortKind(node.Kind)):
		attrs = w.dotSymbolAttributes(node)
	case node.Kind == "initial" || node.Kind == "final":
		attrs = w.dotPseudostateAttributes(node)
	default:
		if w.skin.rounded(node.Kind) {
			attrs = append(attrs, `style="rounded,filled"`)
		}
		attrs = append(attrs, w.fillAttributes(node)...)
		if stated {
			attrs = append(attrs, w.dotStatedLabel(node)...)
		} else {
			attrs = append(attrs, w.labels.dotLabel(node))
		}
	}
	for i, attr := range attrs {
		attrs[i] = dotStyledLabel(attr, node.Style)
	}
	attrs = dotOverridden(append(attrs, dotStyleAttributes(node.Style, w.fills.filled(node))...))
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

// dotStatedLabel is a stated box's label attributes: the label fitted to the
// box's width and its headroom with no margin taken off them, set at the top
// when that is a header strip; when the room holds no line even at the floor,
// the head is set outside as `xlabel`.
func (w *dotWriter) dotStatedLabel(node *Node) []string {
	width := node.Geometry.Width
	height, header := w.headroom(node)
	if !dotHoldsALine(width, height) {
		attrs := []string{`label=""`}
		if keyworded(node) {
			attrs = append(attrs, "xlabel="+dotQuote(w.labels.head(node)))
		}
		return attrs
	}
	attrs := []string{w.labels.dotFittedLabel(node, width, height), "margin=0"}
	if header {
		attrs = append(attrs, "labelloc=t")
	}
	return attrs
}

// dotHoldsALine reports whether a box has room for one line of one glyph at the floor size.
func dotHoldsALine(width, height float64) bool {
	return width >= dotFitFloor*dotBoldGlyphEm && height >= dotFitFloor*dotLineEm
}

// headroom is the height a stated box has for its title: the strip above the
// topmost stated box it encloses (header), or the whole box when it encloses none.
func (w *dotWriter) headroom(node *Node) (height float64, header bool) {
	box := w.boxes[node.ID]
	top := box.high.Y
	for _, inner := range w.boxes {
		if inner.stated && box.encloses(inner) && inner.low.Y < top {
			top, header = inner.low.Y, true
		}
	}
	return top - box.low.Y, header
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

// dotSymbolAttributes draws a symbol kind as the notation's symbol: a diamond,
// a filled bar or a port's square (the default box at the stated size), the
// filled dot or double ring. A given name or a type is set outside as `xlabel`;
// a synthesized name is not drawn. The Pilot look draws them only in a stated
// box; the Cameo look always, a bar with no box at Cameo's default size.
func (w *dotWriter) dotSymbolAttributes(node *Node) []string {
	var attrs []string
	_, boxed := w.boxes[node.ID]
	switch node.Kind {
	case "decision", "merge", "choice":
		attrs = append([]string{"shape=diamond"}, w.skin.fill(node.Kind)...)
	case "fork", "join":
		attrs = []string{dotFillBlack}
		if !boxed {
			attrs = append(attrs, "width="+dotInches(cameoBarWidth), "height="+dotInches(cameoBarHeight), "fixedsize=true")
		}
	case "initial", "junction":
		attrs = []string{"shape=circle", dotFillBlack}
	case "final", terminateKind:
		attrs = []string{"shape=doublecircle", dotFillBlack}
	default:
		attrs = append(attrs, w.fillAttributes(node)...)
	}
	switch node.Kind {
	case "initial", "junction", "final", terminateKind:
		if !boxed {
			attrs = append(attrs, "width="+dotInches(dotPseudostateSize))
		}
	}
	attrs = append(attrs, `label=""`)
	if keyworded(node) {
		attrs = append(attrs, "xlabel="+dotQuote(w.labels.head(node)))
	}
	return attrs
}

// fillAttributes is a plain node's fill beyond the node defaults: the palette's
// family colours when one fills it, else the skin's fill for its kind.
func (w *dotWriter) fillAttributes(node *Node) []string {
	if w.fills.filled(node) {
		attrs := []string{"fillcolor=" + dotQuote(w.fills.fill(node)), dotColorAttr(w.fills.color(node))}
		if !w.skin.cameo {
			attrs = append(attrs, dotPenWidthOne)
		}
		return attrs
	}
	return w.skin.fill(node.Kind)
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
	if shown(node) != "" {
		return []string{shape, w.labels.dotLabel(node)}
	}
	attrs := []string{shape, dotFillBlack, `label=""`}
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
	if (node.Kind == "initial" || node.Kind == "final") && shown(node) == "" {
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
		size, glyph := l.size(), dotGlyphEm
		switch {
		case i == 0:
			glyph = dotBoldGlyphEm
		case i == 1 && keyworded(node):
			size = l.keywordSize()
		}
		width = math.Max(width, float64(utf8.RuneCountInString(line))*size*glyph)
		height += size * dotLineEm
	}
	return width, height
}

// dotTextBox is the box round lines of plain text at the label size, in points,
// with a node's margin about them: what a note with no stated size takes.
func (l labeller) dotTextBox(lines []string) (width, height float64) {
	width, height = dotTextExtent(lines, l.size())
	return math.Max(dotNodeWidth, math.Ceil(width+2*dotMarginWidth)), math.Max(dotNodeHeight, math.Ceil(height+2*dotMarginHeight))
}

// dotTextExtent is the extent, in points, of lines of plain text at a font size.
func dotTextExtent(lines []string, size float64) (width, height float64) {
	for _, line := range lines {
		width = math.Max(width, float64(utf8.RuneCountInString(line))*size*dotGlyphEm)
		height += size * dotLineEm
	}
	return width, height
}

// size is the label font size the skin draws in, the Pilot's 14pt by default.
func (l labeller) size() float64 {
	if l.skin.fontSize == 0 {
		return dotFontSize
	}
	return l.skin.fontSize
}

// keywordSize is the guillemet keyword line's font size under the skin.
func (l labeller) keywordSize() float64 {
	if l.skin.cameo {
		return cameoSmallPts
	}
	return dotKeywordPointSize
}

// dotFitFloor is the smallest font size, in points, a stated box's label shrinks to.
const dotFitFloor = 8

// dotFittedLabel is a node's label composed to fit a stated box: the head wrapped
// at the box's width and shrunk from the default size to the largest at which it
// fits, the keyword and detail lines after it while height remains. A head too
// tall even at the floor is cut to the lines that fit and ellipsized.
func (l labeller) dotFittedLabel(node *Node, width, height float64) string {
	lines := l.lines(node)
	size, head, fits := dotFitHead(lines[0], width, height, l.size())
	var parts labelParts
	parts.head = l.sized(size, "<b>"+dotEscapeLines(head)+"</b>")
	left := height - float64(len(head))*size*dotLineEm
	for i := 1; fits && i < len(lines); i++ {
		keyword := i == 1 && keyworded(node)
		lineSize := size
		if keyword {
			lineSize = math.Round(size * l.keywordSize() / l.size())
		}
		wrapped := dotWrap(lines[i], dotRunesAcross(width, lineSize, dotGlyphEm))
		used := float64(len(wrapped)) * lineSize * dotLineEm
		if used > left {
			break
		}
		left -= used
		text := dotEscapeLines(wrapped)
		if keyword {
			parts.keyword = l.sized(lineSize, l.keywordText(text))
			continue
		}
		parts.details = append(parts.details, l.sized(lineSize, text))
	}
	return dotLabelAttribute(l.assemble(node, parts))
}

// labelParts is a node's label as HTML-like content, by part: the bold head,
// the keyword line when the node has one, then its detail lines.
type labelParts struct {
	head, keyword string
	details       []string
}

// keywordText is the keyword line's markup: italic for the Pilot, plain for Cameo.
func (l labeller) keywordText(text string) string {
	if l.skin.cameo {
		return text
	}
	return "<i>" + text + "</i>"
}

// assemble is the HTML-like label of the parts. The Pilot stacks head, keyword
// and details; Cameo sets the keyword above the name (none for a state or an
// action, as Cameo shows none) and the details in a compartment under a rule.
func (l labeller) assemble(node *Node, parts labelParts) string {
	if !l.skin.cameo {
		lines := []string{parts.head}
		if parts.keyword != "" {
			lines = append(lines, parts.keyword)
		}
		return "<" + strings.Join(append(lines, parts.details...), "<br/>") + ">"
	}
	title := parts.head
	if parts.keyword != "" && node.Kind != "state" && node.Kind != "action" {
		title = parts.keyword + "<br/>" + title
	}
	if len(parts.details) == 0 {
		return "<" + title + ">"
	}
	return fmt.Sprintf(`<<table border="0" cellborder="0" cellspacing="0" cellpadding="2"><tr><td>%s</td></tr><hr/><tr><td align="left">%s</td></tr></table>>`,
		title, strings.Join(parts.details, "<br/>"))
}

// dotFitHead wraps a head line into a box at the largest font size, from the
// default down to the floor, at which it fits with its words whole, else at the
// largest at which it fits with a word broken; when none does, the floor's
// wrapping is cut to the lines the height holds, the last ellipsized.
func dotFitHead(head string, width, height, from float64) (size float64, lines []string, fits bool) {
	return dotFitText([]string{head}, dotBoldGlyphEm, width, height, from)
}

// dotFitText fits several lines of text into a box as dotFitHead fits one: each
// entry is wrapped separately at the runes a glyph of the size holds and the
// wrappings concatenated; a whole-word pass fails when any entry must break a
// word.
func dotFitText(text []string, glyph, width, height, from float64) (size float64, lines []string, fits bool) {
	for _, whole := range []bool{true, false} {
		for size = from; size >= dotFitFloor; size-- {
			across := dotRunesAcross(width, size, glyph)
			lines = lines[:0]
			broken := false
			for _, entry := range text {
				if whole && dotBreaksAWord(entry, across) {
					broken = true
					break
				}
				lines = append(lines, dotWrap(entry, across)...)
			}
			if broken {
				continue
			}
			if float64(len(lines))*size*dotLineEm <= height {
				return size, lines, true
			}
		}
	}
	size = dotFitFloor
	across := dotRunesAcross(width, size, glyph)
	lines = lines[:0]
	for _, entry := range text {
		lines = append(lines, dotWrap(entry, across)...)
	}
	down := max(1, int(height/(size*dotLineEm)))
	if len(lines) > down {
		lines = lines[:down]
		last := []rune(lines[down-1])
		lines[down-1] = string(last[:max(0, min(len(last), across-1))]) + "…"
	}
	return size, lines, false
}

// dotBreaksAWord reports whether wrapping text to across runes a line must break
// a word: one longer than the line.
func dotBreaksAWord(text string, across int) bool {
	for _, word := range strings.Fields(text) {
		if utf8.RuneCountInString(word) > across {
			return true
		}
	}
	return false
}

// dotRunesAcross is how many glyphs of a font size fit across a width, one at
// least so a line can be written at all.
func dotRunesAcross(width, size, glyph float64) int {
	return max(1, int(width/(size*glyph)))
}

// dotWrap word-wraps text to at most across runes a line, breaking a word longer
// than that at the rune it overruns. A ":" that would end a line instead leads
// the type name after it onto the next, where the two fit together.
func dotWrap(text string, across int) []string {
	fits := func(line, word string) bool { // word fits on line, joined by a space when line is not empty
		n := utf8.RuneCountInString(word)
		if line != "" {
			n += utf8.RuneCountInString(line) + 1
		}
		return n <= across
	}
	var lines []string
	line := ""
	words := strings.Fields(text)
	for i := 0; i < len(words); i++ {
		word := words[i]
		if word == ":" && i+1 < len(words) && !fits(line, ": "+words[i+1]) && fits("", ": "+words[i+1]) {
			word, i = ": "+words[i+1], i+1
		}
		if line != "" && fits(line, word) {
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

// sized wraps label text in a `<font point-size>` when its size is not the
// skin's node default.
func (l labeller) sized(size float64, text string) string {
	if size == l.size() {
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

// dotClusterAttributes is a cluster's attribute statements: its label (fitted to
// its headroom when its box is stated, at the least one line at the floor, since a
// cluster's label has no outside to go to), a dashed border for an orthogonal
// region, its black border at the skin's thickness (a package's heavier than an
// element's), and its box as `bb` when it has an extent.
func (w *dotWriter) dotClusterAttributes(node *Node) []string {
	label := w.labels.dotLabel(node)
	if g := node.Geometry; g != nil && g.HasSize {
		height, _ := w.headroom(node)
		label = w.labels.dotFittedLabel(node, g.Width, math.Max(height, dotFitFloor*dotLineEm))
	}
	attrs := []string{dotStyledLabel(label, node.Style)}
	attrs = append(attrs, w.clusterStyle(node)...)
	attrs = append(attrs, dotStyleAttributes(node.Style, false)...)
	if box, ok := w.boxes[node.ID]; ok && box.low != box.high {
		attrs = append(attrs, "bb="+dotQuote(w.dotBB(box)))
	}
	if g := node.Geometry; g != nil && g.Collapsed {
		attrs = append(attrs, `comment="collapsed"`)
	}
	return attrs
}

// clusterStyle is a cluster's border and fill under the skin: the Pilot's black
// border, dashed for a region; Cameo's rounded, gradient-filled state or action.
func (w *dotWriter) clusterStyle(node *Node) []string {
	if !w.skin.cameo {
		attrs := []string{}
		if node.Kind == "region" {
			attrs = append(attrs, "style=dashed")
		}
		return append(attrs, "color=black", "penwidth="+dotClusterPenwidth(node))
	}
	if node.Kind == "region" {
		return []string{`style="rounded,dashed"`, dotColorAttr(cameoLineColor), dotPenWidthOne}
	}
	fill, pen := cameoFill(node.Kind)
	style := "style=filled"
	if cameoRounded(node.Kind) {
		style = `style="rounded,filled"`
	}
	return []string{style, "fillcolor=" + dotQuote(fill), "gradientangle=0", dotColorAttr(pen), dotPenWidthOne}
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
		return nodeBox{low: corner, high: Point{X: g.X + g.Width, Y: g.Y + g.Height}, stated: true}
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
		if !ok || !w.placement.extent[child.ID] {
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
		attrs = append(attrs, dotArrowheadNone)
		if !w.skin.cameo {
			attrs = append(attrs, "penwidth=3")
		}
	case EdgeFlow:
		attrs = append(attrs, "style=dashed")
	case EdgeBinding:
		attrs = append(attrs, dotArrowheadNone)
	}
	attrs = append(attrs, dotStyleAttributes(edge.Style, false)...)
	if len(edge.Route) > 1 {
		attrs = append(attrs, "pos="+dotQuote(w.dotSpline(edge.Route, dotArrowheaded(attrs))))
		if edge.Label != "" {
			attrs = append(attrs, "lp="+dotQuote(w.dotPoint(w.dotLabelPoint(edge))))
		}
	}
	return attrs
}

// dotArrowheaded reports whether an edge with these attributes draws a head at
// its end: every edge but one that takes the head off.
func dotArrowheaded(attrs []string) bool {
	return !slices.Contains(attrs, "arrowhead=none")
}

// dotArrowLength is the length, in points, of Graphviz's default arrowhead.
const dotArrowLength = 10

// dotSpline is a polyline of waypoints as Graphviz's cubic B-spline: each
// segment's ends are its own control points, so the curve is the polyline. An
// arrowheaded edge ends `e,x,y` at its last waypoint, the curve stopping an
// arrow's length short of it, which is what Graphviz draws the head between.
func (w *dotWriter) dotSpline(route []Point, arrowheaded bool) string {
	var points []string
	if arrowheaded {
		tip := route[len(route)-1]
		route = append(slices.Clone(route[:len(route)-1]), dotArrowBase(route))
		points = append(points, "e,"+w.dotPoint(tip))
	}
	points = append(points, w.dotPoint(route[0]))
	for i := 1; i < len(route); i++ {
		from, to := w.dotPoint(route[i-1]), w.dotPoint(route[i])
		points = append(points, from, to, to)
	}
	return strings.Join(points, " ")
}

// dotArrowBase is where a route's curve stops for its arrowhead: an arrow's
// length back from the last waypoint along the last segment, at the segment's
// midpoint at most so a short segment keeps its direction.
func dotArrowBase(route []Point) Point {
	n := len(route)
	from, tip := route[n-2], route[n-1]
	d := math.Hypot(tip.X-from.X, tip.Y-from.Y)
	if d == 0 {
		return tip
	}
	back := math.Min(dotArrowLength, d/2)
	return Point{X: halfPixel(tip.X - (tip.X-from.X)/d*back), Y: halfPixel(tip.Y - (tip.Y-from.Y)/d*back)}
}

// dotLabelGap is the space, in pixels, between a routed edge and its label.
const dotLabelGap = 4

// dotLabelPoint is where a routed edge's label is centred: beside the midpoint
// of the route's longest segment, clear of it by the label's half-extent and a
// gap — right of a segment going down, above one going right — not on a box.
func (w *dotWriter) dotLabelPoint(edge Edge) Point {
	from, to := longestSegment(edge.Route)
	mid := Point{X: (from.X + to.X) / 2, Y: (from.Y + to.Y) / 2}
	length := math.Hypot(to.X-from.X, to.Y-from.Y)
	if length == 0 {
		return mid
	}
	nx, ny := (to.Y-from.Y)/length, -(to.X-from.X)/length
	width, height := dotTextExtent([]string{edge.Label}, w.skin.edgePts)
	off := math.Abs(nx)*width/2 + math.Abs(ny)*height/2 + dotLabelGap
	return Point{X: halfPixel(mid.X + nx*off), Y: halfPixel(mid.Y + ny*off)}
}

// halfPixel rounds a coordinate to the nearest half pixel.
func halfPixel(v float64) float64 { return math.Round(v*2) / 2 }

// containmentAttributes is a tree's containment edge, Mermaid's `---`; Cameo
// draws it as UML composition, a filled diamond at the owner.
func (s dotSkin) containmentAttributes() []string {
	if s.cameo {
		return []string{dotArrowheadNone, "dir=back", "arrowtail=diamond"}
	}
	return []string{dotArrowheadNone}
}

// dotNoteID is the node ID of the i-th note of a rendering.
func dotNoteID(i int) string { return fmt.Sprintf("note:%d", i) }

// placeNotes boxes every note in a positioned drawing, at its corner in its
// stated size or one fitted to its text; an unpositioned drawing lays notes out.
func (w *dotWriter) placeNotes(notes []Note) {
	if w.placement.count == 0 {
		return
	}
	w.noteBoxes = make([]nodeBox, len(notes))
	for i, note := range notes {
		width, height := note.Width, note.Height
		if !note.HasSize {
			width, height = w.labels.dotTextBox(w.noteLines(note))
		}
		w.noteBoxes[i] = nodeBox{low: Point{X: note.X, Y: note.Y}, high: Point{X: note.X + width, Y: note.Y + height}, stated: note.HasSize}
	}
}

// noteLines is a note's label text by line: Cameo heads it with «comment».
func (w *dotWriter) noteLines(note Note) []string {
	lines := strings.Split(note.Text, "\n")
	if w.skin.cameo {
		return append([]string{"«comment»"}, lines...)
	}
	return lines
}

// writeNotes writes each note as a `shape=note` node, white under either skin,
// pinned in its box when the drawing is positioned.
func (w *dotWriter) writeNotes(notes []Note, depth int) {
	indent := strings.Repeat("  ", depth)
	for i, note := range notes {
		attrs := []string{"shape=note"}
		if w.skin.cameo {
			attrs = append(attrs, "fillcolor="+dotQuote(cameoNoteFill), dotColorAttr(cameoLineColor))
		}
		var box *nodeBox
		if w.noteBoxes != nil {
			box = &w.noteBoxes[i]
		}
		attrs = append(attrs, w.dotNoteLabel(note, box)...)
		if box != nil {
			attrs = append(attrs, w.dotPin(box.centre()), "width="+dotInches(box.high.X-box.low.X), "height="+dotInches(box.high.Y-box.low.Y))
			if box.stated {
				attrs = append(attrs, "fixedsize=true")
			}
		}
		fmt.Fprintf(&w.b, "%s%s [%s];\n", indent, dotQuote(dotNoteID(i)), strings.Join(attrs, ", "))
	}
}

// dotNoteLabel is a note's label attributes: its text at the label size, or fitted
// to a stated box as a node's label is, set outside as `xlabel` when the box holds
// no line.
func (w *dotWriter) dotNoteLabel(note Note, box *nodeBox) []string {
	lines := strings.Split(note.Text, "\n")
	header := ""
	if w.skin.cameo {
		header = fmt.Sprintf(`<font point-size="%d">«comment»</font><br/>`, cameoSmallPts)
	}
	if box == nil || !box.stated {
		if w.skin.cameo {
			return []string{dotLabelAttribute("<" + header + dotEscapeLines(lines) + ">")}
		}
		return []string{dotLabelAttribute(dotQuote(note.Text))}
	}
	width, height := box.high.X-box.low.X, box.high.Y-box.low.Y
	if !dotHoldsALine(width, height) {
		return []string{`label=""`, "xlabel=" + dotQuote(note.Text)}
	}
	if w.skin.cameo {
		height = math.Max(height-cameoSmallPts*dotLineEm, dotFitFloor*dotLineEm)
	}
	size, fitted, _ := dotFitText(lines, dotGlyphEm, width, height, w.labels.size())
	return []string{"margin=0", dotLabelAttribute("<" + header + w.labels.sized(size, dotEscapeLines(fitted)) + ">")}
}

// writeAnchors writes each anchored note's anchor: a dashed line with no
// arrowhead, clipped at the anchor's cluster when that is one. A note on an
// edge anchors to a sizeless point on the edge's route; on an unrouted edge, to
// the edge's tail end, the nearest Graphviz can draw.
func (w *dotWriter) writeAnchors(notes []Note, edges []Edge) {
	attrs := []string{"style=dashed", dotArrowheadNone}
	for i, note := range notes {
		switch {
		case note.Anchor != "":
			if w.draws(note.Anchor) {
				w.writeEdge(dotNoteID(i), note.Anchor, attrs)
			}
		case note.EdgeFrom != "":
			if !w.draws(note.EdgeFrom) || !w.draws(note.EdgeTo) {
				continue
			}
			route := edgeRoute(edges, note.EdgeFrom, note.EdgeTo)
			if len(route) < 2 || w.placement.count == 0 {
				w.writeEdge(dotNoteID(i), note.EdgeFrom, attrs)
				continue
			}
			point := dotNoteID(i) + ":on"
			fmt.Fprintf(&w.b, "  %s [shape=point, width=0, height=0, style=invis, %s];\n", dotQuote(point), w.dotPin(routeMidpoint(route)))
			w.writeEdge(dotNoteID(i), point, attrs)
		}
	}
}

// edgeRoute is the stated route of the first edge from one node to another,
// nil when none is routed.
func edgeRoute(edges []Edge, from, to string) []Point {
	for _, edge := range edges {
		if edge.From == from && edge.To == to && len(edge.Route) >= 2 {
			return edge.Route
		}
	}
	return nil
}

// longestSegment is the ends of a route's longest segment.
func longestSegment(route []Point) (from, to Point) {
	best, length := 1, -1.0
	for i := 1; i < len(route); i++ {
		if d := math.Hypot(route[i].X-route[i-1].X, route[i].Y-route[i-1].Y); d > length {
			best, length = i, d
		}
	}
	return route[best-1], route[best]
}

// routeMidpoint is the middle of a route's longest segment.
func routeMidpoint(route []Point) Point {
	from, to := longestSegment(route)
	return Point{X: (from.X + to.X) / 2, Y: (from.Y + to.Y) / 2}
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
	parts := labelParts{head: "<b>" + dotEscape(lines[0]) + "</b>"}
	rest := lines[1:]
	if keyworded(node) {
		parts.keyword = l.sized(l.keywordSize(), l.keywordText(dotEscape(rest[0])))
		rest = rest[1:]
	}
	for _, line := range rest {
		parts.details = append(parts.details, dotEscape(line))
	}
	return dotLabelAttribute(l.assemble(node, parts))
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
