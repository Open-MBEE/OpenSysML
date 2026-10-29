package view

import (
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view/route"
	"math"
	"os"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/imagefile"
)

// Drawio writes the rendering as an uncompressed diagrams.net (draw.io) file:
// one mxGraphModel whose vertices and edges carry the rendering's geometry.
func (r *Rendering) Drawio() (string, error) { return r.DrawioWith(Options{}) }

// DrawioWith writes the rendering as a draw.io file under options; every semantic
// cell is an <object> carrying the element's qualified name, kind, view and declaration.
func (r *Rendering) DrawioWith(options Options) (string, error) {
	if !r.supportsForm(FormDrawio) {
		return "", r.wrongFormError(FormDrawio)
	}
	if err := options.Palette.check(); err != nil {
		return "", err
	}
	if err := options.Unplaced.check(); err != nil {
		return "", err
	}
	if err := options.Ports.check(); err != nil {
		return "", err
	}
	r = r.showing(options.Show)
	if options.Unplaced != UnplacedStrip || options.Compartments.automatic() {
		r = foldCompartments(withoutStandIns(r), options.Compartments)
	}
	if p := placeRendering(r); p.partial() && options.Unplaced != UnplacedStrip {
		r = p.omit(r)
	}
	w := &drawioWriter{
		fills:  familyFills{palette: options.Palette, tree: r.Kind == KindTree},
		labels: labelsOf(r.Roots, false, nil, r.Shown),
		ports:  r.portView(options.Ports),
		links:  options.Links,
		view:   r.View,
		boxes:  map[string]drawioBox{},
		cells:  map[string]bool{},
	}
	if r.Kind == KindAction {
		w.used = r.usedPorts(w.ports)
		w.labels.listedPin = r.listedPin(func(node *Node, port Port) bool { return w.used[node.ID][port.ID] })
	}
	for _, root := range r.Roots {
		w.fills.collect(root)
	}
	w.layout(r)
	edges, err := w.routeEdges(r)
	if err != nil {
		return "", err
	}
	routed := *r
	routed.Edges = edges
	r = &routed
	w.edgeIDs = drawioEdgeIDs(r.Edges)
	return w.document(r, options), nil
}

// drawioWriter composes one draw.io file from a rendering.
type drawioWriter struct {
	fills    familyFills
	labels   labeller
	ports    portView
	used     map[string]map[string]bool
	links    Links
	view     string
	boxes    map[string]drawioBox // node ID -> its box in canvas coordinates
	unrouted bool                 // this build has no router; draw.io routes on opening
	cells    map[string]bool      // the cell IDs written, the ends an edge may attach to
	edgeIDs  map[[2]string]string // edge ends -> its cell ID, the anchor an edge note joins
	grid     int                  // nodes and notes laid on the grid for want of a position
	extent   drawioBox            // the bounds of everything placed
	b        strings.Builder
}

// drawioBox is an axis-aligned box in canvas coordinates: the top-left corner
// and the size, as DiagramLayout states them and draw.io stores them.
type drawioBox struct {
	X, Y, Width, Height float64
}

// relativeTo is the box in a parent's coordinates: draw.io stores a child
// vertex's geometry relative to its parent's top-left corner.
func (b drawioBox) relativeTo(parent drawioBox) drawioBox {
	return drawioBox{X: b.X - parent.X, Y: b.Y - parent.Y, Width: b.Width, Height: b.Height}
}

// right and bottom are the box's far edges.
func (b drawioBox) right() float64  { return b.X + b.Width }
func (b drawioBox) bottom() float64 { return b.Y + b.Height }

// The spacing the grid lays unplaced nodes out with, and the padding a
// container keeps about its nested children, in draw.io pixels.
const (
	drawioGridGap = 40
	// drawioNestedGap also parts a header from its members: more than twice
	// the router's clearance, so a path can pass between neighbours.
	drawioNestedGap   = 2*route.Clearance + 8
	drawioNestedPad   = 16
	drawioPortSize    = 12
	drawioMinWidth    = 100
	drawioPageWidth   = 827
	drawioPageHeight  = 1169
	drawioGlyphSize   = 20
	drawioFinalSize   = 30
	drawioChoiceSize  = 40
	drawioBarLength   = 120
	drawioBarWidth    = 10
	drawioJunctionDim = 12
)

// layout settles a box for every node: the stated one, or one fitted to the
// label and the children, on a deterministic grid when nothing places it.
func (w *drawioWriter) layout(r *Rendering) {
	var placed, unplaced []*Node
	for _, root := range r.Roots {
		if root.Geometry.placed() {
			placed = append(placed, root)
		} else {
			unplaced = append(unplaced, root)
		}
	}
	for _, root := range placed {
		w.settle(root, drawioBox{X: root.Geometry.X, Y: root.Geometry.Y})
		w.extend(w.boxes[root.ID])
	}
	for _, picture := range r.Pictures {
		w.extend(drawioBox{X: picture.X, Y: picture.Y, Width: picture.Width, Height: picture.Height})
	}
	for _, note := range r.Notes {
		if note.HasSize {
			w.extend(drawioBox{X: note.X, Y: note.Y, Width: note.Width, Height: note.Height})
		}
	}
	for _, edge := range r.Edges {
		for _, p := range edge.Route {
			w.extend(drawioBox{X: p.X, Y: p.Y})
		}
	}
	sizes := make([]drawioBox, len(unplaced))
	for i, root := range unplaced {
		sizes[i] = w.fitted(root)
	}
	origin := drawioBox{X: drawioGridGap, Y: drawioGridGap}
	if w.grid = len(unplaced); len(placed) > 0 || w.extent.Width > 0 {
		origin.Y = w.extent.bottom() + drawioGridGap
		origin.X = math.Min(drawioGridGap, w.extent.X)
	}
	for i, box := range drawioGrid(sizes, origin, drawioGridGap) {
		w.settle(unplaced[i], box)
		w.extend(box)
	}
}

// extend grows the placed extent to cover a box.
func (w *drawioWriter) extend(box drawioBox) {
	if w.extent.Width == 0 && w.extent.Height == 0 && w.extent.X == 0 && w.extent.Y == 0 {
		w.extent = box
		return
	}
	x, y := math.Min(w.extent.X, box.X), math.Min(w.extent.Y, box.Y)
	right, bottom := math.Max(w.extent.right(), box.right()), math.Max(w.extent.bottom(), box.bottom())
	w.extent = drawioBox{X: x, Y: y, Width: right - x, Height: bottom - y}
}

// drawioGrid lays boxes of the given sizes in rows from origin, as many a row
// as fits a square, each row as tall as its tallest box; the order is kept.
func drawioGrid(sizes []drawioBox, origin drawioBox, gap float64) []drawioBox {
	boxes := make([]drawioBox, len(sizes))
	if len(sizes) == 0 {
		return boxes
	}
	columns := int(math.Ceil(math.Sqrt(float64(len(sizes)))))
	x, y, rowHeight := origin.X, origin.Y, 0.0
	for i, size := range sizes {
		if i > 0 && i%columns == 0 {
			x, y, rowHeight = origin.X, y+rowHeight+gap, 0
		}
		boxes[i] = drawioBox{X: x, Y: y, Width: size.Width, Height: size.Height}
		x += size.Width + gap
		rowHeight = math.Max(rowHeight, size.Height)
	}
	return boxes
}

// settle fixes a node's box at a corner, sized as fitted sizes it, and settles
// the nodes under it.
func (w *drawioWriter) settle(node *Node, at drawioBox) {
	size := w.fitted(node)
	box := drawioBox{X: at.X, Y: at.Y, Width: size.Width, Height: size.Height}
	w.boxes[node.ID] = box
	w.settleChildren(node, box)
}

// settleChildren places a container's children: placed ones keep their stated offset,
// the rest fill a grid under the header, compartments and placed children.
func (w *drawioWriter) settleChildren(node *Node, box drawioBox) {
	if len(node.Children) == 0 {
		return
	}
	shift := w.childShift(node, box)
	var unplaced []*Node
	for _, child := range node.Children {
		if child.Geometry.placed() {
			w.settle(child, drawioBox{X: child.Geometry.X + shift.X, Y: child.Geometry.Y + shift.Y})
		} else {
			unplaced = append(unplaced, child)
		}
	}
	sizes := make([]drawioBox, len(unplaced))
	for i, child := range unplaced {
		sizes[i] = w.fitted(child)
	}
	origin := drawioBox{X: box.X + drawioNestedPad, Y: box.Y + w.headroom(node) + drawioNestedGap}
	for _, child := range node.Children {
		if child.Geometry.placed() {
			origin.Y = math.Max(origin.Y, w.boxes[child.ID].bottom()+drawioNestedGap)
		}
	}
	for i, child := range drawioGrid(sizes, origin, drawioNestedGap) {
		w.settle(unplaced[i], child)
	}
}

// childShift is what a container's placed children move by: nothing when the
// container is placed too, else the offset putting their bounds under its header.
func (w *drawioWriter) childShift(node *Node, box drawioBox) drawioBox {
	if node.Geometry.placed() {
		return drawioBox{}
	}
	minX, minY, any := math.Inf(1), math.Inf(1), false
	for _, child := range node.Children {
		if child.Geometry.placed() {
			minX, minY, any = math.Min(minX, child.Geometry.X), math.Min(minY, child.Geometry.Y), true
		}
	}
	if !any {
		return drawioBox{}
	}
	return drawioBox{X: box.X + drawioNestedPad - minX, Y: box.Y + w.headroom(node) + drawioNestedGap - minY}
}

// fitted is the size a node is drawn at: the stated one, else a glyph's fixed
// size, else the label's box grown to enclose the children settled under it.
func (w *drawioWriter) fitted(node *Node) drawioBox {
	if g := node.Geometry; g != nil && g.HasSize {
		return drawioBox{Width: g.Width, Height: g.Height}
	}
	if width, height, ok := drawioGlyphSizeOf(node); ok {
		return drawioBox{Width: width, Height: height}
	}
	width, height := w.labels.dotBox(node)
	if len(w.labels.compartments(node)) > 0 || len(node.Children) > 0 {
		width = math.Max(width, drawioMinWidth)
	}
	if len(node.Children) == 0 {
		return drawioBox{Width: width, Height: height}
	}
	// Children are settled from a provisional corner so their extent is known;
	// settle moves them to the final one afterwards.
	probe := &drawioWriter{labels: w.labels, ports: w.ports, boxes: map[string]drawioBox{}}
	probe.settleChildren(node, drawioBox{Width: width, Height: height})
	right, bottom := 0.0, 0.0
	for _, child := range node.Children {
		right = math.Max(right, probe.boxes[child.ID].right())
		bottom = math.Max(bottom, probe.boxes[child.ID].bottom())
	}
	return drawioBox{Width: math.Max(width, right+drawioNestedPad), Height: math.Max(height, bottom+drawioNestedPad)}
}

// headroom is the height of a node's header and compartments, under which its
// nested children sit.
func (w *drawioWriter) headroom(node *Node) float64 {
	height := w.headerHeight(node)
	for _, compartment := range w.compartments(node) {
		height += w.compartmentHeight(node, compartment)
	}
	return height
}

// headerHeight is the height of a node's title strip: its keyword, head and
// detail lines with a margin above and below.
func (w *drawioWriter) headerHeight(node *Node) float64 {
	lines := w.headerLines(node)
	if len(lines) == 0 {
		return 2 * dotMarginHeight
	}
	_, height := dotTextExtent(lines, w.labels.sizeOf(node))
	return math.Ceil(height + 2*dotMarginHeight)
}

// compartmentHeight is the height of a compartment's cell: its name and rows.
func (w *drawioWriter) compartmentHeight(node *Node, compartment Compartment) float64 {
	_, height := dotTextExtent(w.compartmentLines(node, compartment), w.labels.sizeOf(node))
	return math.Ceil(height + 2*dotMarginHeight)
}

// headerLines are the lines of a node's title: keyword, head and details.
func (w *drawioWriter) headerLines(node *Node) []string {
	var lines []string
	if keyword := w.labels.keyword(node); keyword != "" {
		lines = append(lines, keyword)
	}
	lines = append(lines, w.labels.headLines(node)...)
	lines = append(lines, w.labels.details(node)...)
	return append(lines, w.labels.parameterLines(node)...)
}

// compartments are the compartments a node's cell stacks: those shown, less
// any a Layout hides and any left without a row.
func (w *drawioWriter) compartments(node *Node) []Compartment {
	var out []Compartment
	for _, compartment := range w.labels.compartments(node) {
		var rows []*Node
		for _, row := range compartment.Rows {
			if !hiddenRow(node, row) {
				rows = append(rows, row)
			}
		}
		if len(rows) > 0 {
			out = append(out, Compartment{Name: compartment.Name, Rows: rows})
		}
	}
	return out
}

// compartmentLines are a compartment cell's label lines: its name, then its rows.
func (w *drawioWriter) compartmentLines(node *Node, compartment Compartment) []string {
	lines := []string{compartment.Name}
	for _, row := range compartment.Rows {
		lines = append(lines, strings.Split(w.labels.rowText(row), "\n")...)
	}
	return lines
}

// drawioGlyphSizeOf is the fixed size a notation symbol is drawn at, when the
// node is one: a dot, a ring, a bar, a diamond.
func drawioGlyphSizeOf(node *Node) (width, height float64, ok bool) {
	switch {
	case !isSymbolKind(node.Kind) || isPortKind(node.Kind):
		return 0, 0, false
	case node.Kind == "final":
		return drawioFinalSize, drawioFinalSize, true
	case node.Kind == "junction":
		return drawioJunctionDim, drawioJunctionDim, true
	case isBarKind(node.Kind):
		return drawioBarLength, drawioBarWidth, true
	case isHistoryKind(node.Kind):
		return drawioFinalSize, drawioFinalSize, true
	case node.Kind == "decision" || node.Kind == "choice" || node.Kind == "merge":
		return drawioChoiceSize, drawioChoiceSize, true
	default:
		return drawioGlyphSize, drawioGlyphSize, true
	}
}

// document writes the whole file: the leading comments, the model and its cells.
func (w *drawioWriter) document(r *Rendering, options Options) string {
	w.b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	title := r.View + " — " + string(r.Kind) + " rendering"
	if r.Stated != "" {
		title += " (" + r.Stated + ")"
	}
	w.comment(title)
	if r.blank() {
		w.comment(r.blankReason(FormDrawio))
	}
	if w.grid > 0 {
		w.comment(fmt.Sprintf("%d node(s) were not laid out and sit on a grid: the draw.io desktop command line (32 or later) arranges them with its normalize and layout elkLayered options, the editor with Arrange › Layout", w.grid))
	}
	for _, notice := range w.notices(r, options) {
		w.comment("notice: " + notice)
	}
	w.b.WriteString(`<mxfile host="OpenSysML" type="device" compressed="false">` + "\n")
	w.b.WriteString("  <diagram")
	w.attr("id", drawioDiagramID(r.View))
	w.attr("name", r.View)
	w.b.WriteString(">\n")
	w.b.WriteString(`    <mxGraphModel dx="0" dy="0" grid="1" gridSize="10" guides="1" tooltips="1" connect="1" arrows="1" fold="1" page="1" pageScale="1"`)
	width, height := drawioPage(r.Canvas, w.extent)
	w.attr("pageWidth", formatCoord(width))
	w.attr("pageHeight", formatCoord(height))
	if r.Canvas != nil && r.Canvas.Unit != "" {
		w.attr("unit", r.Canvas.Unit)
	}
	w.b.WriteString(` math="0" shadow="0">` + "\n")
	w.b.WriteString("      <root>\n        <mxCell id=\"0\"/>\n        <mxCell id=\"1\" parent=\"0\"/>\n")
	refusals := r.pictureRefusals()
	for i, picture := range r.Pictures {
		if !picture.Above && refusals[i] == nil {
			w.writePicture(i, picture)
		}
	}
	for _, root := range r.Roots {
		w.writeNode(root, "1", "")
	}
	for i, edge := range r.Edges {
		w.writeEdge(i, edge)
	}
	w.writeNotes(r)
	for i, picture := range r.Pictures {
		if picture.Above && refusals[i] == nil {
			w.writePicture(i, picture)
		}
	}
	w.b.WriteString("      </root>\n    </mxGraphModel>\n  </diagram>\n</mxfile>\n")
	return w.b.String()
}

// drawioPage is the page the model states: the canvas's size when it has one,
// else A4 portrait grown to hold the drawing.
func drawioPage(canvas *Canvas, extent drawioBox) (width, height float64) {
	if canvas != nil && canvas.HasSize {
		return canvas.Width, canvas.Height
	}
	return math.Max(drawioPageWidth, math.Ceil(extent.right()+drawioGridGap)), math.Max(drawioPageHeight, math.Ceil(extent.bottom()+drawioGridGap))
}

// drawioDiagramID is the diagram's stable identifier, the view's name made safe
// for an XML ID.
func drawioDiagramID(view string) string {
	return strings.NewReplacer("::", "-", " ", "_").Replace(view)
}

// notices are what the file does not represent, written as leading comments.
func (w *drawioWriter) notices(r *Rendering, options Options) []string {
	notices := append([]string(nil), r.Notices...)
	if w.unrouted {
		notices = append(notices, "edges not routed: this build runs no router; draw.io routes them when the file opens")
	}
	if options.Style != "" && options.Style != StylePilot {
		notices = append(notices, styleNotice(options.Style))
	}
	if options.Lines.routed() {
		notices = append(notices, linesNotice(options.Lines, FormDrawio))
	}
	if options.Direction != "" {
		notices = append(notices, fmt.Sprintf("direction %s not applied; draw.io keeps the stated positions and the grid", options.Direction))
	}
	if r.Kind == KindAction {
		if pins := r.undrawnPins(func(node *Node, port Port) bool { return w.used[node.ID][port.ID] || w.labels.lists(node, port) }); len(pins) > 0 {
			notices = append(notices, fmt.Sprintf("%d pin(s) not drawn (%s); draw.io draws the pins an edge ends at", len(pins), strings.Join(pins, ", ")))
		}
	}
	if len(r.Pictures) > 0 {
		notices = append(notices, refusedPictureNotices(r.Pictures, r.pictureRefusals())...)
	}
	return notices
}

// comment writes one leading XML comment; a comment holds no `--`.
func (w *drawioWriter) comment(text string) {
	w.b.WriteString("<!-- " + strings.ReplaceAll(text, "--", "- -") + " -->\n")
}

// attr writes one attribute, its value escaped by encoding/xml.
func (w *drawioWriter) attr(name, value string) {
	w.b.WriteString(" " + name + `="`)
	_ = xml.EscapeText(&w.b, []byte(value))
	w.b.WriteString(`"`)
}

// drawioObject are the attributes a semantic cell's <object> carries.
type drawioObject struct {
	id, label, fqn, kind, link string
}

// openObject writes the <object> wrapper of a semantic cell; the view is every
// cell's, the fqn and link those with one.
func (w *drawioWriter) openObject(indent string, o drawioObject) {
	w.b.WriteString(indent + "<object")
	w.attr("id", o.id)
	w.attr("label", o.label)
	if o.fqn != "" {
		w.attr("fqn", o.fqn)
	}
	w.attr("kind", o.kind)
	w.attr("view", w.view)
	if o.link != "" {
		w.attr("link", o.link)
	}
	w.b.WriteString(">\n")
}

// link is a declaration's site, `file:line`, or the link template's URL when
// one is set; nothing for an origin with no source.
func (w *drawioWriter) link(origin Origin) string {
	if url, ok := w.links.URL(origin); ok {
		return url
	}
	if !origin.Located() || w.links.Sites == nil {
		return ""
	}
	site, ok := w.links.Sites(origin)
	if !ok || site.File == "" {
		return ""
	}
	return fmt.Sprintf("%s:%d", site.File, site.Line)
}

// fqn is a node's qualified name: the declaration's when its site is known, else the
// exposed name under its container's; a synthesized name is none.
func (w *drawioWriter) fqn(node *Node, parent string) string {
	if name := w.siteName(node.Origin); name != "" {
		return name
	}
	return fqnOf(node, parent)
}

// siteName is the qualified name a declaration's site states, when Sites knows it.
func (w *drawioWriter) siteName(origin Origin) string {
	if w.links.Sites == nil || !origin.Located() {
		return ""
	}
	site, ok := w.links.Sites(origin)
	if !ok {
		return ""
	}
	return site.QualifiedName
}

// fqnOf is a node's qualified name from its label: a root's name as the view
// exposes it, a nested node's under its container's; a synthesized name is none.
func fqnOf(node *Node, parent string) string {
	if node.Name == "" || node.NameSynthesized {
		return ""
	}
	if parent == "" || strings.Contains(node.Name, "::") {
		return node.Name
	}
	return parent + "::" + node.Name
}

// writeNode writes a node's cell under a parent cell, then its compartments,
// ports and children; a node the rendering lists twice is drawn where first met.
func (w *drawioWriter) writeNode(node *Node, parent, parentFQN string) {
	if w.cells[node.ID] {
		return
	}
	box := w.boxes[node.ID]
	rel := box
	if parent != "1" {
		rel = box.relativeTo(w.boxes[parent])
	}
	fqn := w.fqn(node, parentFQN)
	nested := fqn
	if nested == "" {
		nested = parentFQN
	}
	compartments := w.compartments(node)
	header := w.headerHeight(node)
	w.openObject("        ", drawioObject{id: node.ID, label: strings.Join(w.headerLines(node), "\n"), fqn: fqn, kind: node.Kind, link: w.link(node.Origin)})
	w.b.WriteString("          <mxCell")
	w.attr("style", w.nodeStyle(node, header, len(compartments) > 0))
	w.b.WriteString(` vertex="1"`)
	if collapsed(node) {
		w.b.WriteString(` collapsed="1"`)
	}
	w.attr("parent", parent)
	w.b.WriteString(">\n")
	if collapsed(node) {
		w.geometry("            ", drawioBox{X: rel.X, Y: rel.Y, Width: rel.Width, Height: header}, nil, &rel)
	} else {
		w.geometry("            ", rel, nil, nil)
	}
	w.b.WriteString("          </mxCell>\n        </object>\n")
	w.cells[node.ID] = true
	y := header
	for i, compartment := range compartments {
		height := w.compartmentHeight(node, compartment)
		w.writeCompartment(fmt.Sprintf("%s:c%d", node.ID, i), node, compartment, drawioBox{Y: y, Width: box.Width, Height: height})
		y += height
	}
	w.writePorts(node, nested)
	for _, child := range node.Children {
		w.writeNode(child, node.ID, nested)
	}
}

// writeCompartment writes one compartment as a text cell stacked in its node.
func (w *drawioWriter) writeCompartment(id string, node *Node, compartment Compartment, box drawioBox) {
	w.b.WriteString("        <mxCell")
	w.attr("id", id)
	w.attr("value", strings.Join(w.compartmentLines(node, compartment), "\n"))
	w.attr("style", "text;html=0;whiteSpace=wrap;strokeColor=inherit;fillColor=none;align=left;verticalAlign=top;spacingLeft=4;spacingRight=4;overflow=hidden;rotatable=0;portConstraint=eastwest;"+drawioFontStyle(node.Style))
	w.b.WriteString(` vertex="1"`)
	w.attr("parent", node.ID)
	w.b.WriteString(">\n")
	w.geometry("          ", box, nil, nil)
	w.b.WriteString("        </mxCell>\n")
}

// writePorts writes a node's drawn ports as small border cells, inputs left, outputs
// right, the rest along the top, each at a fraction of the edge unless its Layout places it.
func (w *drawioWriter) writePorts(node *Node, fqn string) {
	ports := w.ports.of(node)
	if len(ports) == 0 {
		return
	}
	sides := map[PortDirection][]Port{}
	for _, port := range ports {
		sides[drawioPortSide(port)] = append(sides[drawioPortSide(port)], port)
	}
	box := w.boxes[node.ID]
	for _, port := range ports {
		side := drawioPortSide(port)
		index, count := drawioPortIndex(sides[side], port), len(sides[side])
		fraction := float64(index+1) / float64(count+1)
		id := node.ID + ":" + port.ID
		portFQN := w.siteName(port.Origin)
		if portFQN == "" && fqn != "" && port.Name != "" && !port.NameSynthesized {
			portFQN = fqn + "::" + port.Name
		}
		w.openObject("        ", drawioObject{id: id, label: w.ports.pinLabel(port), fqn: portFQN, kind: drawioPortKind(port), link: w.link(port.Origin)})
		w.b.WriteString("          <mxCell")
		w.attr("style", drawioPortStyle(side)+drawioStyleOf(nil, ""))
		w.b.WriteString(` vertex="1"`)
		w.attr("parent", node.ID)
		w.b.WriteString(">\n")
		if g := port.Geometry; g.placed() {
			size := drawioBox{Width: drawioPortSize, Height: drawioPortSize}
			if g.HasSize {
				size = drawioBox{Width: g.Width, Height: g.Height}
			}
			w.geometry("            ", drawioBox{X: g.X - box.X, Y: g.Y - box.Y, Width: size.Width, Height: size.Height}, nil, nil)
		} else {
			w.relativeGeometry("            ", side, fraction)
		}
		w.b.WriteString("          </mxCell>\n        </object>\n")
		w.cells[id] = true
	}
}

// drawioPortKind names a port cell's kind: a pin's direction, else `port`.
func drawioPortKind(port Port) string {
	if port.Parameter {
		return "parameter"
	}
	return "port"
}

// drawioPortSide is the border a port sits on: PortIn the left, PortOut the
// right, every other the top.
func drawioPortSide(port Port) PortDirection {
	switch port.Direction {
	case PortIn, PortOut:
		return port.Direction
	}
	return PortUndirected
}

func drawioPortIndex(ports []Port, port Port) int {
	for i, p := range ports {
		if p.ID == port.ID {
			return i
		}
	}
	return 0
}

// drawioPortStyle is a port square's style: its label beside it, inside the owner.
func drawioPortStyle(side PortDirection) string {
	base := "rounded=0;whiteSpace=wrap;html=0;fontSize=10;"
	switch side {
	case PortIn:
		return base + "labelPosition=right;verticalLabelPosition=middle;align=left;verticalAlign=middle;"
	case PortOut:
		return base + "labelPosition=left;verticalLabelPosition=middle;align=right;verticalAlign=middle;"
	}
	return base + "labelPosition=center;verticalLabelPosition=top;align=center;verticalAlign=bottom;"
}

// relativeGeometry writes a port's geometry as a fraction of its owner's
// border, offset to centre the square on the edge.
func (w *drawioWriter) relativeGeometry(indent string, side PortDirection, fraction float64) {
	x, y := fraction, 0.0
	switch side {
	case PortIn:
		x, y = 0, fraction
	case PortOut:
		x, y = 1, fraction
	}
	w.b.WriteString(indent + "<mxGeometry")
	w.attr("x", formatCoord(x))
	w.attr("y", formatCoord(y))
	w.attr("width", formatCoord(drawioPortSize))
	w.attr("height", formatCoord(drawioPortSize))
	w.b.WriteString(` relative="1" as="geometry">` + "\n")
	w.b.WriteString(indent + `  <mxPoint x="-6" y="-6" as="offset"/>` + "\n")
	w.b.WriteString(indent + "</mxGeometry>\n")
}

// geometry writes an mxGeometry: the box, any route points, and the full box a
// collapsed cell expands to.
func (w *drawioWriter) geometry(indent string, box drawioBox, points []Point, alternate *drawioBox) {
	w.b.WriteString(indent + "<mxGeometry")
	if box.X != 0 {
		w.attr("x", formatCoord(box.X))
	}
	if box.Y != 0 {
		w.attr("y", formatCoord(box.Y))
	}
	w.attr("width", formatCoord(box.Width))
	w.attr("height", formatCoord(box.Height))
	w.b.WriteString(` as="geometry"`)
	if len(points) == 0 && alternate == nil {
		w.b.WriteString("/>\n")
		return
	}
	w.b.WriteString(">\n")
	if alternate != nil {
		w.b.WriteString(indent + "  <mxRectangle")
		w.attr("width", formatCoord(alternate.Width))
		w.attr("height", formatCoord(alternate.Height))
		w.b.WriteString(` as="alternateBounds"/>` + "\n")
	}
	w.writePoints(indent+"  ", points)
	w.b.WriteString(indent + "</mxGeometry>\n")
}

// writePoints writes a route's waypoints.
func (w *drawioWriter) writePoints(indent string, points []Point) {
	if len(points) == 0 {
		return
	}
	w.b.WriteString(indent + `<Array as="points">` + "\n")
	for _, p := range points {
		w.b.WriteString(indent + "  <mxPoint")
		w.attr("x", formatCoord(p.X))
		w.attr("y", formatCoord(p.Y))
		w.b.WriteString("/>\n")
	}
	w.b.WriteString(indent + "</Array>\n")
}

// nodeStyle is a node cell's style: a swimlane stacking compartments and
// children under its header, a glyph's shape, a rounded or square box.
func (w *drawioWriter) nodeStyle(node *Node, header float64, compartmented bool) string {
	var style string
	switch {
	case isSymbolKind(node.Kind) && !isPortKind(node.Kind):
		style = drawioGlyphStyle(node)
	case compartmented && len(node.Children) == 0:
		style = fmt.Sprintf("swimlane;html=0;whiteSpace=wrap;fontStyle=1;align=center;verticalAlign=top;childLayout=stackLayout;horizontal=1;startSize=%s;horizontalStack=0;resizeParent=1;resizeParentMax=0;resizeLast=0;collapsible=1;marginBottom=0;", formatCoord(header))
	case compartmented || len(node.Children) > 0:
		style = fmt.Sprintf("swimlane;html=0;whiteSpace=wrap;fontStyle=1;align=center;verticalAlign=top;startSize=%s;collapsible=1;", formatCoord(header))
		if node.Kind == "region" {
			style = fmt.Sprintf("swimlane;html=0;whiteSpace=wrap;dashed=1;align=left;verticalAlign=top;startSize=%s;collapsible=0;", formatCoord(header))
		}
	default:
		style = w.plainStyle(node)
	}
	if w.labels.skin.rounded(node.Kind) && !strings.HasPrefix(style, "ellipse") && !strings.HasPrefix(style, "shape=") && !strings.HasPrefix(style, "rhombus") {
		style = strings.Replace(style, "rounded=0;", "rounded=1;arcSize=10;", 1)
		if strings.HasPrefix(style, "swimlane;") {
			style += "rounded=1;arcSize=10;"
		}
	}
	return style + drawioStyleOf(node.Style, w.fillOf(node))
}

// plainStyle is the shape of a node without compartments or children: a box,
// or the case notation's ellipse, note and actor.
func (w *drawioWriter) plainStyle(node *Node) string {
	switch {
	case caseNodeKind(node.Kind):
		return "ellipse;html=0;whiteSpace=wrap;align=center;verticalAlign=middle;"
	case node.Kind == "objective":
		return "shape=note;html=0;whiteSpace=wrap;size=14;align=left;verticalAlign=top;spacing=4;"
	case node.Kind == "actor":
		return "shape=umlActor;html=0;whiteSpace=wrap;verticalLabelPosition=bottom;verticalAlign=top;"
	}
	return "rounded=0;html=0;whiteSpace=wrap;align=center;verticalAlign=middle;"
}

// drawioGlyphStyle is a notation symbol's shape as the UML template draws it.
func drawioGlyphStyle(node *Node) string {
	labelled := ""
	if shown(node) != "" {
		labelled = "verticalLabelPosition=bottom;verticalAlign=top;"
	}
	switch {
	case node.Kind == "final":
		return "ellipse;shape=endState;html=0;fillColor=#000000;strokeColor=#000000;" + labelled
	case node.Kind == terminateKind:
		return "shape=umlDestroy;html=0;strokeWidth=2;" + labelled
	case isBarKind(node.Kind):
		return "shape=line;html=0;strokeWidth=6;strokeColor=#000000;fillColor=#000000;" + labelled
	case isHistoryKind(node.Kind):
		return "ellipse;html=0;whiteSpace=wrap;align=center;verticalAlign=middle;"
	case node.Kind == "decision" || node.Kind == "choice" || node.Kind == "merge":
		return "rhombus;html=0;whiteSpace=wrap;" + labelled
	}
	return "ellipse;html=0;fillColor=#000000;strokeColor=#000000;" + labelled
}

// fillOf is the palette's fill for a node, when the palette fills it.
func (w *drawioWriter) fillOf(node *Node) string {
	if w.fills.filled(node) {
		return w.fills.fill(node)
	}
	return ""
}

// drawioStyleOf maps a Style to draw.io style keys; fill is the palette's
// colour when the Style sets none.
func drawioStyleOf(style *Style, fill string) string {
	var parts []string
	if style != nil && style.Fill != "" {
		fill = style.Fill
	}
	if fill != "" {
		parts = append(parts, "fillColor="+fill)
	}
	if style != nil {
		if style.Line != "" {
			parts = append(parts, "strokeColor="+style.Line)
		}
		if style.Text != "" {
			parts = append(parts, "fontColor="+style.Text)
		}
	}
	if font := drawioFontStyle(style); font != "" {
		parts = append(parts, strings.TrimSuffix(font, ";"))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ";") + ";"
}

// drawioFontStyle maps a Style's font to draw.io keys: family, size, and
// fontStyle's bit set (bold 1, italic 2).
func drawioFontStyle(style *Style) string {
	if style == nil {
		return ""
	}
	var parts []string
	if style.Font != "" {
		parts = append(parts, "fontFamily="+style.Font)
	}
	if style.FontSize > 0 {
		parts = append(parts, "fontSize="+formatCoord(style.FontSize))
	}
	if bits := drawioFontBits(style); bits > 0 {
		parts = append(parts, fmt.Sprintf("fontStyle=%d", bits))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ";") + ";"
}

func drawioFontBits(style *Style) int {
	bits := 0
	if style.Bold {
		bits |= 1
	}
	if style.Italic {
		bits |= 2
	}
	return bits
}

// drawioEdgeIDs are the edges' cell IDs by their ends, for a note anchored to an edge.
func drawioEdgeIDs(edges []Edge) map[[2]string]string {
	ids := map[[2]string]string{}
	for i, edge := range edges {
		key := [2]string{edge.From, edge.To}
		if _, ok := ids[key]; !ok {
			ids[key] = fmt.Sprintf("e%d", i)
		}
	}
	return ids
}

// writeEdge writes an edge between two cells, a port's when the edge names one,
// its route as waypoints and its end labels as child label cells.
func (w *drawioWriter) writeEdge(i int, edge Edge) {
	source, target := w.end(edge.From, edge.FromPort), w.end(edge.To, edge.ToPort)
	if !w.cells[source] || !w.cells[target] {
		return
	}
	id := fmt.Sprintf("e%d", i)
	w.openObject("        ", drawioObject{id: id, label: edge.Label, kind: edge.Kind.String(), link: w.link(edge.Origin)})
	w.b.WriteString("          <mxCell")
	w.attr("style", drawioEdgeFullStyle(edge))
	w.b.WriteString(` edge="1"`)
	w.attr("parent", "1")
	w.attr("source", source)
	w.attr("target", target)
	w.b.WriteString(">\n")
	w.b.WriteString(`            <mxGeometry relative="1" as="geometry"`)
	if len(edge.Route) == 0 {
		w.b.WriteString("/>\n")
	} else {
		w.b.WriteString(">\n")
		w.writePoints("              ", edge.Route)
		w.b.WriteString("            </mxGeometry>\n")
	}
	w.b.WriteString("          </mxCell>\n        </object>\n")
	w.endLabel(id+":from", id, edge.FromLabel, -0.8, edge.Style)
	w.endLabel(id+":to", id, edge.ToLabel, 0.8, edge.Style)
}

// end is the cell an edge end attaches to: the port's when the end names a
// drawn port, else the node's.
func (w *drawioWriter) end(node, port string) string {
	if port != "" && w.cells[node+":"+port] {
		return node + ":" + port
	}
	return node
}

// endLabel writes an end label as a label cell riding the edge near one end.
func (w *drawioWriter) endLabel(id, edge, text string, position float64, style *Style) {
	if text == "" {
		return
	}
	w.b.WriteString("        <mxCell")
	w.attr("id", id)
	w.attr("value", text)
	w.attr("style", "edgeLabel;html=0;align=center;verticalAlign=middle;resizable=0;"+drawioFontStyle(style))
	w.b.WriteString(` vertex="1" connectable="0"`)
	w.attr("parent", edge)
	w.b.WriteString(">\n          <mxGeometry")
	w.attr("x", formatCoord(position))
	w.b.WriteString(` relative="1" as="geometry"/>` + "\n        </mxCell>\n")
}

// drawioEdgeFullStyle is an edge's whole style: orthogonal routing unless a
// Route states the path, the kind's arrows, and its Style.
func drawioEdgeFullStyle(edge Edge) string {
	routing := "edgeStyle=orthogonalEdgeStyle;"
	if len(edge.Route) > 0 {
		routing = "edgeStyle=none;"
	}
	style := routing + "rounded=0;html=0;" + drawioEdgeStyle(edge.Kind) + ";"
	if edge.Style != nil {
		if edge.Style.Line != "" {
			style += "strokeColor=" + edge.Style.Line + ";"
		}
		if edge.Style.Text != "" {
			style += "fontColor=" + edge.Style.Text + ";"
		}
		style += drawioFontStyle(edge.Style)
	}
	return style
}

// drawioEdgeStyle is the notation table's draw.io column for an edge kind: the
// source end's marker as startArrow, the target end's as endArrow.
func drawioEdgeStyle(kind EdgeKind) string {
	switch kind {
	case EdgeConnection:
		return "endArrow=none;strokeWidth=3"
	case EdgeBinding, EdgeAssociation:
		return "endArrow=none"
	case EdgeFlow:
		return "endArrow=block;endFill=1"
	case EdgeSuccession:
		return "dashed=1;endArrow=open"
	case EdgeTransition, EdgeRequire, EdgeAssume, EdgeAssert, EdgeFrame, EdgeEvent:
		return "endArrow=open"
	case EdgeSpecialization, EdgeSubsetting, EdgeRedefinition, EdgeReferenceSubsetting:
		return "endArrow=block;endFill=0"
	case EdgeTyping:
		return "dashed=1;endArrow=block;endFill=0"
	case EdgeComposition:
		return "startArrow=diamondThin;startFill=1;endArrow=none"
	case EdgeReference:
		return "startArrow=diamondThin;startFill=0;endArrow=none"
	case EdgeContainment:
		return "startArrow=circlePlus;startFill=0;endArrow=none"
	case EdgeImport, EdgeDependency, EdgeDerive, EdgeRefine, EdgeExpose, EdgeSatisfy, EdgeVerify, EdgeAllocate, EdgeInclude, EdgePerform, EdgeExhibit:
		return "dashed=1;endArrow=open"
	case EdgeAnchor:
		return "dashed=1;endArrow=none"
	case EdgeSuccessionFlow:
		return "dashed=1;endArrow=block;endFill=1"
	case EdgePortion:
		return "startArrow=oval;startFill=1;endArrow=none"
	case EdgeUnownedMembership:
		return "startArrow=oval;startFill=0;endArrow=none"
	}
	return "endArrow=open"
}

// writeNotes writes one note cell per annotation (its notes share an Origin), anchored
// to each subject; an unplaced note sits beside its first subject, else on the grid.
func (w *drawioWriter) writeNotes(r *Rendering) {
	groups := drawioNoteGroups(r.Notes)
	var loose [][]int
	sizes := []drawioBox{}
	for _, group := range groups {
		note := r.Notes[group[0]]
		if note.HasSize {
			w.writeNote(group, r.Notes, drawioBox{X: note.X, Y: note.Y, Width: note.Width, Height: note.Height})
			continue
		}
		width, height := w.labels.dotTextBox(strings.Split(note.Text, "\n"))
		if box, ok := w.boxes[note.Anchor]; ok && note.Anchor != "" {
			w.writeNote(group, r.Notes, drawioBox{X: box.right() + drawioNestedGap, Y: box.Y, Width: width, Height: height})
			continue
		}
		loose = append(loose, group)
		sizes = append(sizes, drawioBox{Width: width, Height: height})
	}
	origin := drawioBox{X: drawioGridGap, Y: w.extent.bottom() + drawioGridGap}
	if len(loose) > 0 && w.extent.Width > 0 {
		origin.X = math.Min(drawioGridGap, w.extent.X)
	}
	for j, box := range drawioGrid(sizes, origin, drawioGridGap) {
		w.writeNote(loose[j], r.Notes, box)
	}
}

// drawioNoteGroups groups the notes of one annotation, those sharing a located
// Origin, in the order their first note comes; a note with no origin is its own.
func drawioNoteGroups(notes []Note) [][]int {
	var groups [][]int
	at := map[Origin]int{}
	for i, note := range notes {
		if !note.Origin.Located() {
			groups = append(groups, []int{i})
			continue
		}
		if g, ok := at[note.Origin]; ok {
			groups[g] = append(groups[g], i)
			continue
		}
		at[note.Origin] = len(groups)
		groups = append(groups, []int{i})
	}
	return groups
}

// writeNote writes one note cell for a group of notes and an anchor edge to
// each subject the group names.
func (w *drawioWriter) writeNote(group []int, notes []Note, box drawioBox) {
	id := fmt.Sprintf("note%d", group[0])
	w.b.WriteString("        <mxCell")
	w.attr("id", id)
	w.attr("value", notes[group[0]].Text)
	w.attr("style", "shape=note;html=0;whiteSpace=wrap;size=14;align=left;verticalAlign=top;spacing=4;")
	w.b.WriteString(` vertex="1" parent="1">` + "\n")
	w.geometry("          ", box, nil, nil)
	w.b.WriteString("        </mxCell>\n")
	anchored := map[string]bool{}
	for _, i := range group {
		note := notes[i]
		target := ""
		switch {
		case note.Anchor != "" && w.cells[note.Anchor]:
			target = note.Anchor
		case note.EdgeFrom != "":
			target = w.edgeIDs[[2]string{note.EdgeFrom, note.EdgeTo}]
		}
		if target == "" || anchored[target] {
			continue
		}
		anchored[target] = true
		w.b.WriteString("        <mxCell")
		w.attr("id", fmt.Sprintf("%s:anchor%d", id, len(anchored)-1))
		w.attr("style", "edgeStyle=none;html=0;dashed=1;endArrow=none;")
		w.b.WriteString(` edge="1" parent="1"`)
		w.attr("source", id)
		w.attr("target", target)
		w.b.WriteString(">\n          <mxGeometry relative=\"1\" as=\"geometry\"/>\n        </mxCell>\n")
	}
}

// writePicture writes a picture as an image cell, the image embedded in its style.
func (w *drawioWriter) writePicture(i int, picture Picture) {
	src, ok := drawioPictureSource(picture)
	if !ok {
		return
	}
	w.b.WriteString("        <mxCell")
	w.attr("id", fmt.Sprintf("picture%d", i))
	w.attr("value", picture.Alt)
	w.attr("style", "shape=image;html=0;imageAspect=0;verticalLabelPosition=bottom;verticalAlign=top;image="+src+";")
	w.b.WriteString(` vertex="1" parent="1">` + "\n")
	w.geometry("          ", drawioBox{X: picture.X, Y: picture.Y, Width: picture.Width, Height: picture.Height}, nil, nil)
	w.b.WriteString("        </mxCell>\n")
}

// drawioPictureSource is a picture's image as draw.io keeps one in a style,
// `data:<type>,<base64>`; a remote or unsafe picture is none, as pictureRefusal reported.
func drawioPictureSource(picture Picture) (string, bool) {
	src := strings.TrimSpace(picture.Location)
	if strings.HasPrefix(strings.ToLower(src), "data:") {
		if CheckPicture(src, nil) != nil {
			return "", false
		}
		contentType, data, err := imagefile.DecodeDataURL(src)
		if err != nil {
			return "", false
		}
		return drawioImageData(contentType, data), true
	}
	if RemotePictureLocation(src) {
		return "", false
	}
	data, err := os.ReadFile(picture.Path()) // #nosec G304 -- pictures are intentionally loaded from their declared path.
	if err != nil {
		return "", false
	}
	contentType := imagefile.ContentType(data)
	if contentType == "" || CheckPicture(picture.Location, data) != nil {
		return "", false
	}
	return drawioImageData(contentType, data), true
}

// drawioImageData is image data as a draw.io style's image value.
func drawioImageData(contentType string, data []byte) string {
	return "data:" + contentType + "," + base64.StdEncoding.EncodeToString(data)
}
