package view

import (
	"fmt"
	"math"
	"slices"
	"strings"
)

// DOT is the Graphviz form of a graph-shaped rendering: a `digraph` for the
// `dot` engine, written as text like the Mermaid form, with no Graphviz
// installed. A node with children is a `subgraph "cluster_<id>"` (a tree draws
// containment as edges, as its Mermaid form does) holding an invisible anchor
// node `"<id>"`, so an edge keeps the rendering's endpoints and is clipped at
// the cluster with `lhead`/`ltail` — except at an end that encloses the other,
// where the edge starts or ends inside it. Edge kinds parallel the Mermaid
// arrows:
//
//	EdgeKind        Mermaid  DOT
//	EdgeConnection  ---      arrowhead=none
//	EdgeTransition  -->      solid, default arrowhead
//	EdgeSuccession  -->      solid, default arrowhead
//	EdgeFlow        -.->     style=dashed
//	(containment)   ---      arrowhead=none
//
// Geometry is written for `neato -n`: a placed node pinned at its centre in
// points (1 px = 0.75 pt, y up), a route as a B-spline, a collapsed node a leaf.
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
	w := &dotWriter{tree: r.Kind == KindTree, clusters: map[string]bool{}, enclosing: map[string][]string{}, drawnAs: map[string]string{}}
	for _, root := range r.Roots {
		w.collectClusters(root, nil, "")
	}
	w.geometry = newDOTGeometry(r, w.drawnAs)
	for _, edge := range r.Edges {
		from, to := w.drawn(edge.From), w.drawn(edge.To)
		if w.clipped(from, to) || w.clipped(to, from) {
			w.compound = true
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
	for _, notice := range w.geometry.notices {
		fmt.Fprintf(b, "// not represented: %s\n", notice)
	}
	fmt.Fprintf(b, "// layout: %s\n", w.geometry.engine())
	if r.View == "" {
		b.WriteString("digraph {\n")
	} else {
		fmt.Fprintf(b, "digraph %s {\n", dotQuote(r.View))
	}
	if attrs := w.graphAttributes(direction, r.Canvas); len(attrs) > 0 {
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
		from, to := w.drawn(edge.From), w.drawn(edge.To)
		if from == to && edge.From != edge.To {
			continue // both ends folded into one collapsed node
		}
		w.writeEdge(from, to, dotEdgeAttributes(edge, w.geometry))
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
	drawnAs   map[string]string   // node ID -> the ID it is drawn as: its own, or a collapsed ancestor's
	compound  bool                // an edge is clipped at a cluster
	geometry  *dotGeometry
}

// collectClusters records, for node and everything under it, the node each is
// drawn as, which are clusters, and the clusters each sits in, outermost first.
func (w *dotWriter) collectClusters(node *Node, around []string, folded string) {
	if folded != "" {
		w.drawnAs[node.ID] = folded
	} else {
		w.drawnAs[node.ID] = node.ID
		w.enclosing[node.ID] = around
	}
	if len(node.Children) == 0 {
		return
	}
	if folded == "" && dotCollapsed(node) {
		folded = node.ID
	}
	if folded == "" && !w.tree {
		w.clusters[node.ID] = true
		around = append(around[:len(around):len(around)], node.ID)
	}
	for _, child := range node.Children {
		w.collectClusters(child, around, folded)
	}
}

// drawn is the node an edge end is drawn at: a node folded into a collapsed
// ancestor is drawn as that ancestor.
func (w *dotWriter) drawn(id string) string {
	if drawn, ok := w.drawnAs[id]; ok {
		return drawn
	}
	return id
}

// dotCollapsed reports whether node's body is folded: it is drawn as a leaf
// and its children not at all.
func dotCollapsed(node *Node) bool {
	return node.Geometry != nil && node.Geometry.Collapsed && len(node.Children) > 0
}

// clipped reports whether an edge's end at node is clipped at node's cluster:
// node is a cluster that does not enclose the other end.
func (w *dotWriter) clipped(node, other string) bool {
	return w.clusters[node] && !slices.Contains(w.enclosing[other], node)
}

// dotClusterName is the subgraph name of the cluster drawn for a node.
func dotClusterName(id string) string { return "cluster_" + id }

// graphAttributes is the graph attribute list: `rankdir` when a direction is
// stated, `compound` when an edge is clipped at a cluster, `bb` for a sized canvas.
func (w *dotWriter) graphAttributes(direction Direction, canvas *Canvas) []string {
	var attrs []string
	if direction != "" {
		attrs = append(attrs, "rankdir="+string(direction))
	}
	if w.compound {
		attrs = append(attrs, "compound=true")
	}
	if canvas != nil && canvas.HasSize {
		attrs = append(attrs, w.geometry.boundingBox(0, 0, canvas.Width, canvas.Height))
	}
	return attrs
}

// writeNode writes one node: a cluster holding its children, a plain node
// otherwise — a collapsed node is plain and its children are not written. A
// tree writes the node and an edge to each child instead.
func (w *dotWriter) writeNode(node *Node, depth int) {
	indent := strings.Repeat("  ", depth)
	if dotCollapsed(node) {
		fmt.Fprintf(&w.b, "%s%s [%s];\n", indent, dotQuote(node.ID), strings.Join(dotNodeAttributes(node, w.geometry), ", "))
		return
	}
	if len(node.Children) == 0 || w.tree {
		fmt.Fprintf(&w.b, "%s%s [%s];\n", indent, dotQuote(node.ID), strings.Join(dotNodeAttributes(node, w.geometry), ", "))
		for _, child := range node.Children {
			w.writeNode(child, depth)
			w.writeEdge(node.ID, child.ID, dotContainmentAttributes())
		}
		return
	}
	fmt.Fprintf(&w.b, "%ssubgraph %s {\n", indent, dotQuote(dotClusterName(node.ID)))
	for _, attr := range dotClusterAttributes(node, w.geometry) {
		fmt.Fprintf(&w.b, "%s  %s;\n", indent, attr)
	}
	fmt.Fprintf(&w.b, "%s  %s [%s];\n", indent, dotQuote(node.ID), strings.Join(dotAnchorAttributes(node, w.geometry), ", "))
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

// dotNodeAttributes is a plain node's attribute list: its label and the shape
// its Kind chooses, then its pinned position and fixed size when it is placed.
func dotNodeAttributes(node *Node, g *dotGeometry) []string {
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
	if geo := node.Geometry; geo != nil {
		attrs = append(attrs, g.position(geo)...)
		if geo.HasSize {
			attrs = append(attrs, "width="+g.inches(geo.Width), "height="+g.inches(geo.Height), "fixedsize=true")
		}
	}
	return attrs
}

// dotAnchorAttributes is a cluster's invisible, sizeless anchor node, the node an
// edge to or from the cluster names; pinned at the cluster's centre when placed,
// else set amid its placed members so `neato -n` has a position for it.
func dotAnchorAttributes(node *Node, g *dotGeometry) []string {
	attrs := []string{"shape=point", "style=invis", "width=0", "height=0", `label=""`}
	if node.Geometry != nil {
		return append(attrs, g.position(node.Geometry)...)
	}
	if at, ok := g.anchors[node.ID]; ok {
		attrs = append(attrs, "pos="+dotQuote(g.point(at)))
	}
	return attrs
}

// dotClusterAttributes is a cluster's attribute statements: its label, a
// dashed border for an orthogonal region, and its box as `bb` when it is sized.
func dotClusterAttributes(node *Node, g *dotGeometry) []string {
	attrs := []string{"label=" + dotLabel(node)}
	if node.Kind == "region" {
		attrs = append(attrs, "style=dashed")
	}
	if geo := node.Geometry; geo != nil && geo.HasSize {
		attrs = append(attrs, g.boundingBox(geo.X, geo.Y, geo.Width, geo.Height))
	}
	return attrs
}

// dotEdgeAttributes is an edge's attribute list: its label, its kind's style,
// and its route as a spline when it has waypoints to draw.
func dotEdgeAttributes(edge Edge, g *dotGeometry) []string {
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
		attrs = append(attrs, "pos="+dotQuote(g.spline(edge.Route)))
	}
	return attrs
}

// dotContainmentAttributes is a tree's containment edge, Mermaid's `---`.
func dotContainmentAttributes() []string {
	return []string{"arrowhead=none"}
}

// dotGeometry converts one rendering's geometry (units from the top left, y down)
// to Graphviz's (points from the bottom left, y up); read once per rendering.
type dotGeometry struct {
	pointsPerUnit float64          // 0.75 for px (96 dpi), 1 for pt
	height        float64          // in rendering units; y flips against it
	placed        bool             // a drawn node is positioned
	routed        bool             // an edge has a route with a line to draw
	anchors       map[string]Point // an unplaced cluster -> the centre of its placed members' extent
	notices       []string         // what the DOT does not represent
}

// dotExtent is the box around placed nodes, in rendering units.
type dotExtent struct {
	minX, minY, maxX, maxY float64
	any                    bool
}

func (e *dotExtent) add(geo *Geometry) {
	if !e.any {
		*e = dotExtent{minX: geo.X, minY: geo.Y, maxX: geo.X + geo.Width, maxY: geo.Y + geo.Height, any: true}
		return
	}
	e.minX, e.minY = math.Min(e.minX, geo.X), math.Min(e.minY, geo.Y)
	e.maxX, e.maxY = math.Max(e.maxX, geo.X+geo.Width), math.Max(e.maxY, geo.Y+geo.Height)
}

func (e *dotExtent) join(o dotExtent) {
	if o.any {
		e.add(&Geometry{X: o.minX, Y: o.minY, Width: o.maxX - o.minX, Height: o.maxY - o.minY})
	}
}

// dotPointsPerUnit is the Graphviz points in one unit of each Canvas unit the
// writer converts; an unnamed unit is px.
var dotPointsPerUnit = map[string]float64{"": 0.75, "px": 0.75, "pt": 1}

// newDOTGeometry reads the conversion off the rendering: the canvas unit and
// extent, the positions of the nodes drawn (as drawnAs says), and the routes.
func newDOTGeometry(r *Rendering, drawnAs map[string]string) *dotGeometry {
	g := &dotGeometry{pointsPerUnit: dotPointsPerUnit[""], anchors: map[string]Point{}}
	unitKnown := true
	if r.Canvas != nil {
		if f, ok := dotPointsPerUnit[r.Canvas.Unit]; ok {
			g.pointsPerUnit = f
		} else {
			unitKnown = false
		}
	}
	drawn, unplaced := 0, 0
	var unsizedClusters []string
	cluster := func(node *Node) bool { return r.Kind != KindTree && len(node.Children) > 0 && !dotCollapsed(node) }
	var walk func(node *Node) dotExtent
	walk = func(node *Node) (extent dotExtent) {
		if drawnAs[node.ID] != node.ID {
			return
		}
		drawn++
		if cluster(node) && (node.Geometry == nil || !node.Geometry.HasSize) {
			unsizedClusters = append(unsizedClusters, node.ID)
		}
		for _, child := range node.Children {
			extent.join(walk(child))
		}
		geo := node.Geometry
		switch {
		case geo != nil:
			g.placed = true
			g.height = math.Max(g.height, geo.Y+geo.Height)
			extent.add(geo)
		case cluster(node) && extent.any:
			g.anchors[node.ID] = Point{X: (extent.minX + extent.maxX) / 2, Y: (extent.minY + extent.maxY) / 2}
		default:
			unplaced++
		}
		return extent
	}
	for _, root := range r.Roots {
		walk(root)
	}
	sized := r.Canvas != nil && r.Canvas.HasSize
	if sized {
		g.height = r.Canvas.Height
	}
	for _, edge := range r.Edges {
		switch len(edge.Route) {
		case 0:
		case 1:
			g.notices = append(g.notices, fmt.Sprintf("the route of %s -> %s: one waypoint draws no line", edge.From, edge.To))
		default:
			g.routed = true
		}
	}
	if !unitKnown && (g.placed || g.routed || sized) {
		g.notices = append(g.notices, fmt.Sprintf("canvas unit %q; positions are converted as px (1 px = 0.75 pt)", r.Canvas.Unit))
	}
	if (g.placed || g.routed) && unplaced > 0 {
		g.notices = append(g.notices, fmt.Sprintf("no position for %d of %d nodes (neato -n needs one on every node; neato -s72 keeps the pinned ones and places the rest)", unplaced, drawn))
	}
	if g.placed {
		for _, id := range unsizedClusters {
			g.notices = append(g.notices, fmt.Sprintf("the box of %s, which has no size (neato -n draws a cluster from its bb alone)", id))
		}
	}
	return g
}

// engine is the Graphviz command the `// layout:` header names: `dot` when
// nothing is placed, `neato -n` to keep node positions, `-n2` to keep routes too.
func (g *dotGeometry) engine() string {
	switch {
	case g.routed:
		return "neato -n2"
	case g.placed:
		return "neato -n"
	}
	return "dot"
}

// position is a placed node's `pos` and `pin`: the centre of its box, or the
// point itself when it has no size.
func (g *dotGeometry) position(geo *Geometry) []string {
	x, y := geo.X, geo.Y
	if geo.HasSize {
		x += geo.Width / 2
		y += geo.Height / 2
	}
	return []string{fmt.Sprintf("pos=\"%s!\"", g.point(Point{X: x, Y: y})), "pin=true"}
}

// boundingBox is the `bb` of the box whose top-left corner is (x, y): its
// lower-left and upper-right corners in points.
func (g *dotGeometry) boundingBox(x, y, width, height float64) string {
	return fmt.Sprintf("bb=\"%s,%s,%s,%s\"", g.points(x), g.points(g.height-y-height), g.points(x+width), g.points(g.height-y))
}

// spline is the `pos` of an edge drawn straight through its waypoints: each
// segment P→Q as the cubic P, P+(Q-P)/3, P+2(Q-P)/3, Q, sharing Q with the next.
func (g *dotGeometry) spline(route []Point) string {
	points := []string{g.point(route[0])}
	for i := 1; i < len(route); i++ {
		p, q := route[i-1], route[i]
		dx, dy := q.X-p.X, q.Y-p.Y
		points = append(points,
			g.point(Point{X: p.X + dx/3, Y: p.Y + dy/3}),
			g.point(Point{X: p.X + 2*dx/3, Y: p.Y + 2*dy/3}),
			g.point(q))
	}
	return strings.Join(points, " ")
}

// point is a rendering point as Graphviz writes one: `x,y` in points, y up.
func (g *dotGeometry) point(p Point) string {
	return g.points(p.X) + "," + g.points(g.height-p.Y)
}

// points converts a length to points.
func (g *dotGeometry) points(v float64) string {
	return dotNumber(v * g.pointsPerUnit)
}

// inches converts a length to inches, the unit of a node's width and height.
func (g *dotGeometry) inches(v float64) string {
	return dotNumber(v * g.pointsPerUnit / 72)
}

// dotNumber writes a converted length to at most four decimals in its shortest
// form, so a third of a pixel does not print as sixteen digits.
func dotNumber(v float64) string {
	v = math.Round(v*1e4) / 1e4
	if v == 0 {
		v = 0 // never -0
	}
	return formatCoord(v)
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
