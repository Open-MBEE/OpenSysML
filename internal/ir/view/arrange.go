package view

import (
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view/route"
)

// arranger lays a rendering's boxes out and routes its edges: the step the
// draw.io form and the orthogonal DOT form share. A box is settled where a
// Layout states it, where a layered layout put it (positions), or on a
// deterministic grid; the edges left unrouted are then routed by libavoid
// around the settled boxes, inside the container both ends lie in.
type arranger struct {
	labels     labeller
	ports      portView
	boxes      map[string]drawioBox  // node ID -> its box in canvas coordinates
	positions  map[string]drawioBox  // node ID -> the box a layered layout gave it, taken ahead of the grid
	side       func(Port) route.Side // the border a drawn port sits on; nil is the draw.io form's
	portRoom   func(Port) float64    // the border length a drawn port and its name take; nil takes none
	pixelPins  bool                  // pins sit on half pixels, where the DOT form centres its pin squares
	unrouted   bool                  // this build has no router; the form routes on opening
	routed     map[int]bool          // the edges (by index) the router gave their route
	unroutable []int                 // the edges (by index) the router found no path for
	inward     map[int]drawioEnds    // per edge, the ends that lead into the port's owner
	grid       int                   // nodes and notes laid on the grid for want of a position
	extent     drawioBox             // the bounds of everything placed
}

// newArranger is an arranger with nothing settled yet, sizing boxes by labels
// and drawing the ports of ports.
func newArranger(labels labeller, ports portView) arranger {
	return arranger{labels: labels, ports: ports, boxes: map[string]drawioBox{}, routed: map[int]bool{}, inward: map[int]drawioEnds{}}
}

// layout settles a box for every node: the stated one, or one fitted to the
// label and the children, on a deterministic grid when nothing places it.
func (a *arranger) layout(r *Rendering) {
	var placed, unplaced []*Node
	for _, root := range r.Roots {
		if root.Geometry.placed() {
			placed = append(placed, root)
		} else {
			unplaced = append(unplaced, root)
		}
	}
	for _, root := range placed {
		a.settle(root, drawioBox{X: root.Geometry.X, Y: root.Geometry.Y})
		a.extend(a.boxes[root.ID])
	}
	for _, picture := range r.Pictures {
		a.extend(drawioBox{X: picture.X, Y: picture.Y, Width: picture.Width, Height: picture.Height})
	}
	for _, note := range r.Notes {
		if note.HasSize {
			a.extend(drawioBox{X: note.X, Y: note.Y, Width: note.Width, Height: note.Height})
		}
	}
	for _, edge := range r.Edges {
		for _, p := range edge.Route {
			a.extend(drawioBox{X: p.X, Y: p.Y})
		}
	}
	gridded := a.settlePositioned(unplaced)
	sizes := make([]drawioBox, len(gridded))
	for i, root := range gridded {
		sizes[i] = a.fitted(root)
	}
	origin := drawioBox{X: drawioGridGap, Y: drawioGridGap}
	if a.grid = len(gridded); len(placed) > 0 || a.extent.Width > 0 {
		origin.Y = a.extent.bottom() + drawioGridGap
		origin.X = math.Min(drawioGridGap, a.extent.X)
	}
	for i, box := range drawioGrid(sizes, origin, drawioGridGap) {
		a.settle(gridded[i], box)
		a.extend(box)
	}
}

// settlePositioned settles the nodes the layered layout placed where it put
// them and returns the rest, for the grid.
func (a *arranger) settlePositioned(nodes []*Node) []*Node {
	var left []*Node
	for _, node := range nodes {
		if at, ok := a.positions[node.ID]; ok {
			a.settle(node, at)
			a.extend(a.boxes[node.ID])
		} else {
			left = append(left, node)
		}
	}
	return left
}

// extend grows the placed extent to cover a box.
func (a *arranger) extend(box drawioBox) {
	if a.extent.Width == 0 && a.extent.Height == 0 && a.extent.X == 0 && a.extent.Y == 0 {
		a.extent = box
		return
	}
	x, y := math.Min(a.extent.X, box.X), math.Min(a.extent.Y, box.Y)
	right, bottom := math.Max(a.extent.right(), box.right()), math.Max(a.extent.bottom(), box.bottom())
	a.extent = drawioBox{X: x, Y: y, Width: right - x, Height: bottom - y}
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
func (a *arranger) settle(node *Node, at drawioBox) {
	size := a.fitted(node)
	box := drawioBox{X: at.X, Y: at.Y, Width: size.Width, Height: size.Height}
	a.boxes[node.ID] = box
	a.settleChildren(node, box)
}

// settleChildren places a container's children: placed ones keep their stated offset,
// the rest fill a grid under the header, compartments and placed children.
func (a *arranger) settleChildren(node *Node, box drawioBox) {
	if len(node.Children) == 0 {
		return
	}
	shift := a.childShift(node, box)
	var unplaced []*Node
	for _, child := range node.Children {
		if child.Geometry.placed() {
			a.settle(child, drawioBox{X: child.Geometry.X + shift.X, Y: child.Geometry.Y + shift.Y})
		} else {
			unplaced = append(unplaced, child)
		}
	}
	unplaced = a.settlePositioned(unplaced)
	sizes := make([]drawioBox, len(unplaced))
	for i, child := range unplaced {
		sizes[i] = a.fitted(child)
	}
	// Under a header the router needs a channel; a container of stated size
	// keeps the pad, since its children must fit the size it states.
	gap := drawioNestedGap
	if g := node.Geometry; g != nil && g.HasSize {
		gap = drawioNestedPad
	}
	origin := drawioBox{X: box.X + drawioNestedPad, Y: box.Y + a.headroom(node) + gap}
	for _, child := range node.Children {
		if child.Geometry.placed() {
			origin.Y = math.Max(origin.Y, a.boxes[child.ID].bottom()+gap)
		}
	}
	for i, child := range drawioGrid(sizes, origin, gap) {
		a.settle(unplaced[i], child)
	}
}

// childShift is what a container's placed children move by: nothing when the
// container is placed too, else the offset putting their bounds under its header.
func (a *arranger) childShift(node *Node, box drawioBox) drawioBox {
	if node.Geometry.placed() {
		return drawioBox{}
	}
	minX, minY, found := math.Inf(1), math.Inf(1), false
	for _, child := range node.Children {
		if child.Geometry.placed() {
			minX, minY, found = math.Min(minX, child.Geometry.X), math.Min(minY, child.Geometry.Y), true
		}
	}
	if !found {
		return drawioBox{}
	}
	return drawioBox{X: box.X + drawioNestedPad - minX, Y: box.Y + a.headroom(node) + drawioNestedGap - minY}
}

// fitted is the size a node is drawn at: the stated one, else the one the
// layered layout grew a container to, else a glyph's fixed size, else the
// label's box grown to enclose the children settled under it.
func (a *arranger) fitted(node *Node) drawioBox {
	if g := node.Geometry; g != nil && g.HasSize {
		return drawioBox{Width: g.Width, Height: g.Height}
	}
	if at, ok := a.positions[node.ID]; ok && len(node.Children) > 0 {
		return drawioBox{Width: at.Width, Height: at.Height}
	}
	if width, height, ok := drawioGlyphSizeOf(node); ok {
		return drawioBox{Width: width, Height: height}
	}
	width, height := a.labels.dotBox(node)
	if len(a.labels.compartments(node)) > 0 || len(node.Children) > 0 {
		width = math.Max(width, drawioMinWidth)
	}
	width, height = math.Max(width, a.portsRoom(node, route.North, route.South)), math.Max(height, a.portsRoom(node, route.West, route.East))
	if len(node.Children) == 0 {
		return drawioBox{Width: width, Height: height}
	}
	// Children are settled from the container's corner, placed ones sitting at
	// their absolute Layouts, so their extent is known; settle moves them after.
	corner := drawioBox{Width: width, Height: height}
	if g := node.Geometry; g.placed() {
		corner.X, corner.Y = g.X, g.Y
	}
	probe := a.probe()
	probe.settleChildren(node, corner)
	right, bottom := 0.0, 0.0
	for _, child := range node.Children {
		right = math.Max(right, probe.boxes[child.ID].right()-corner.X)
		bottom = math.Max(bottom, probe.boxes[child.ID].bottom()-corner.Y)
	}
	return drawioBox{Width: math.Max(width, right+drawioNestedPad), Height: math.Max(height, bottom+drawioNestedPad)}
}

// probe is an arranger sizing and placing as a does, with nothing settled,
// for measuring a container's children before they are settled under it.
func (a *arranger) probe() *arranger {
	return &arranger{labels: a.labels, ports: a.ports, positions: a.positions, side: a.side, portRoom: a.portRoom, pixelPins: a.pixelPins, boxes: map[string]drawioBox{}}
}

// headroom is the height of a node's header and compartments, under which its
// nested children sit.
func (a *arranger) headroom(node *Node) float64 {
	height := a.headerHeight(node)
	for _, compartment := range a.compartments(node) {
		height += a.compartmentHeight(node, compartment)
	}
	return height
}

// headerHeight is the height of a node's title strip: its keyword, head and
// detail lines with a margin above and below.
func (a *arranger) headerHeight(node *Node) float64 {
	lines := a.headerLines(node)
	if len(lines) == 0 {
		return 2 * dotMarginHeight
	}
	_, height := dotTextExtent(lines, a.labels.sizeOf(node))
	return math.Ceil(height + 2*dotMarginHeight)
}

// compartmentHeight is the height of a compartment's cell: its name and rows.
func (a *arranger) compartmentHeight(node *Node, compartment Compartment) float64 {
	_, height := dotTextExtent(a.compartmentLines(node, compartment), a.labels.sizeOf(node))
	return math.Ceil(height + 2*dotMarginHeight)
}

// headerLines are the lines of a node's title: keyword, head and details.
func (a *arranger) headerLines(node *Node) []string {
	var lines []string
	if keyword := a.labels.keyword(node); keyword != "" {
		lines = append(lines, keyword)
	}
	lines = append(lines, a.labels.headLines(node)...)
	lines = append(lines, a.labels.details(node)...)
	return append(lines, a.labels.parameterLines(node)...)
}

// compartments are the compartments a node's cell stacks: those shown, less
// any a Layout hides and any a Layout left without a row. A compartment that
// heads the node's children (`action flow`, `state transition`) has no rows and stays.
func (a *arranger) compartments(node *Node) []Compartment {
	var out []Compartment
	for _, compartment := range a.labels.compartments(node) {
		var rows []*Node
		for _, row := range compartment.Rows {
			if !hiddenRow(node, row) {
				rows = append(rows, row)
			}
		}
		if len(rows) > 0 || len(compartment.Rows) == 0 {
			out = append(out, Compartment{Name: compartment.Name, Rows: rows})
		}
	}
	return out
}

// compartmentLines are a compartment cell's label lines: its name, then its rows.
func (a *arranger) compartmentLines(node *Node, compartment Compartment) []string {
	lines := []string{compartment.Name}
	for _, row := range compartment.Rows {
		lines = append(lines, strings.Split(a.labels.rowText(row), "\n")...)
	}
	return lines
}

// routeEdges gives every edge the rendering leaves unrouted the path libavoid
// finds around the settled boxes, as the diagram panel routes it: edges are
// routed in groups by the container both ends lie in, kept inside it, with the
// containers an end lies in reduced to their headers so routes may cross them.
func (a *arranger) routeEdges(r *Rendering) ([]Edge, error) {
	edges := append([]Edge(nil), r.Edges...)
	tree := a.drawioTree(r)
	groups := a.routeGroups(edges, tree)
	for _, key := range slices.Sorted(maps.Keys(groups)) {
		g := routeGroup{key: key, edges: groups[key], all: edges, tree: tree}
		routes, err := a.routeOneGroup(g)
		if errors.Is(err, route.ErrUnavailable) {
			a.unrouted = true
			return edges, nil
		}
		if err != nil {
			return nil, err
		}
		a.routeGroupFound(g, routes)
	}
	return edges, nil
}

// routeGroups indexes the unrouted edges with settled ends by the container
// both ends lie in, "" for the canvas.
func (a *arranger) routeGroups(edges []Edge, tree drawioTree) map[string][]int {
	groups := map[string][]int{}
	for i, edge := range edges {
		_, from := a.boxes[edge.From]
		_, to := a.boxes[edge.To]
		if len(edge.Route) > 0 || !from || !to {
			continue
		}
		key := tree.commonContainer(edge.From, edge.To)
		// A member drawn outside its container is not held by it.
		for key != "" && !(a.boxes[key].contains(a.boxes[edge.From]) && a.boxes[key].contains(a.boxes[edge.To])) {
			key = tree.parents[key]
		}
		groups[key] = append(groups[key], i)
	}
	return groups
}

// routeGroup is one routing pass: the edges, by index into all, both of whose
// ends lie in the container key, "" for the canvas.
type routeGroup struct {
	key   string
	edges []int
	all   []Edge
	tree  drawioTree
}

// headersOnly are the containers an end of the group lies in: routes may
// cross them, so the router sees their headers alone.
func (g routeGroup) headersOnly() map[string]bool {
	exempt := map[string]bool{}
	for _, i := range g.edges {
		for _, end := range []string{g.all[i].From, g.all[i].To} {
			for p := g.tree.parents[end]; p != ""; p = g.tree.parents[p] {
				exempt[p] = true
			}
		}
	}
	return exempt
}

// routeOneGroup routes the group's edges inside their container.
func (a *arranger) routeOneGroup(g routeGroup) ([]route.Route, error) {
	header := a.boxes[g.key]
	if g.key != "" {
		header.Height = a.headroom(g.tree.nodes[g.key])
	}
	asked, memberPins, portShapes := a.routeGroupEnds(g, header)
	shapes := a.routeGroupShapes(g, header, memberPins, portShapes)
	// An end at a port that is a shape of its own attaches there.
	for j := range asked {
		if _, ok := portShapes[asked[j].From+":"+asked[j].FromPort]; ok && asked[j].FromPort != "" {
			asked[j].From += ":" + asked[j].FromPort
		}
		if _, ok := portShapes[asked[j].To+":"+asked[j].ToPort]; ok && asked[j].ToPort != "" {
			asked[j].To += ":" + asked[j].ToPort
		}
	}
	var bounds *route.Box
	if g.key != "" {
		b := route.Box(a.boxes[g.key])
		bounds = &b
	}
	return route.Routes(shapes, asked, bounds)
}

// routeGroupEnds is the group's edges as asked of the router: container-to-member
// edges leave a header pin (memberPins), those at a named port its cell (portShapes).
func (a *arranger) routeGroupEnds(g routeGroup, header drawioBox) (asked []route.Edge, memberPins []route.Port, portShapes map[string]route.Shape) {
	portShapes = map[string]route.Shape{}
	asked = make([]route.Edge, len(g.edges))
	for j, i := range g.edges {
		edge := g.all[i]
		asked[j] = route.Edge{Index: j, From: edge.From, To: edge.To, FromPort: edge.FromPort, ToPort: edge.ToPort}
		if g.key == "" || (edge.From != g.key && edge.To != g.key) || edge.From == edge.To {
			continue
		}
		member, port := edge.To, edge.FromPort
		if member == g.key {
			member, port = edge.From, edge.ToPort
		}
		// The container's own named port is a cell of its own: the edge
		// leaves it through its inner face, into the container.
		if shape, ok := a.portShape(g.tree.nodes[g.key], a.boxes[g.key], port, true); ok {
			portShapes[shape.ID] = shape
			if edge.From == g.key {
				a.inward[i] |= drawioFrom
			} else {
				a.inward[i] |= drawioTo
			}
			continue
		}
		pin := route.Port{ID: fmt.Sprintf("\x00member/%d", i), Side: route.South, Flush: true}
		pin.Offset = min(1, max(0, (a.boxes[member].X+a.boxes[member].Width/2-header.X)/header.Width))
		memberPins = append(memberPins, pin)
		if edge.From == g.key {
			asked[j].FromPort = pin.ID
		} else {
			asked[j].ToPort = pin.ID
		}
	}
	return asked, memberPins, portShapes
}

// routeGroupShapes are the router's obstacles: the settled boxes (crossed ones
// as headers, the container's with memberPins) and the port cells, added to portShapes.
func (a *arranger) routeGroupShapes(g routeGroup, header drawioBox, memberPins []route.Port, portShapes map[string]route.Shape) []route.Shape {
	exempt := g.headersOnly()
	shapes := make([]route.Shape, 0, len(g.tree.order))
	for _, id := range g.tree.order {
		node := g.tree.nodes[id]
		box := a.boxes[id]
		if exempt[id] {
			box.Height = a.headroom(node)
		}
		ports, placed := a.routePorts(node, box)
		for _, shape := range placed {
			// The container's port an edge enters it through keeps its inner face.
			if _, inward := portShapes[shape.ID]; !inward {
				portShapes[shape.ID] = shape
			}
		}
		shape := route.Shape{ID: id, Box: route.Box(box), Ports: ports}
		if id == g.key {
			shape.Box = route.Box(header)
			shape.Ports = append(shape.Ports, memberPins...)
		}
		shapes = append(shapes, shape)
	}
	for _, id := range slices.Sorted(maps.Keys(portShapes)) {
		shapes = append(shapes, portShapes[id])
	}
	return shapes
}

// routeGroupFound copies the routes found onto the group's edges and notes
// the edges the router found no path for.
func (a *arranger) routeGroupFound(g routeGroup, routes []route.Route) {
	for _, found := range routes {
		i := g.edges[found.Index]
		// Routed points carry float noise a declared route never has.
		for _, p := range found.Points {
			g.all[i].Route = append(g.all[i].Route, Point{X: drawioRound(p.X), Y: drawioRound(p.Y)})
		}
		a.routed[i] = true
	}
	for _, i := range g.edges {
		if !a.routed[i] {
			a.unroutable = append(a.unroutable, i)
		}
	}
}

// routePorts is where a node's drawn ports sit for the router: a port drawn
// on the border at a fraction is a pin of the node's shape; a positioned
// port, wherever it was put, is a shape of its own with a pin on its outer face.
func (a *arranger) routePorts(node *Node, box drawioBox) ([]route.Port, []route.Shape) {
	ports := a.ports.of(node)
	if len(ports) == 0 {
		return nil, nil
	}
	var pins []route.Port
	var shapes []route.Shape
	for _, port := range ports {
		if port.Geometry.placed() {
			shape, _ := a.portShape(node, box, port.ID, false)
			shapes = append(shapes, shape)
			continue
		}
		side, fraction := a.portPlace(node, port)
		if length := box.Width; a.pixelPins {
			if !side.Vertical() {
				length = box.Height
			}
			fraction = halfPixel(fraction*length) / length
		}
		pins = append(pins, route.Port{ID: port.ID, Side: side, Offset: fraction})
	}
	return pins, shapes
}

// portsRoom is the border length the drawn ports on the fuller of two
// opposite sides of a node take, so the node is drawn long enough for them.
func (a *arranger) portsRoom(node *Node, sides ...route.Side) float64 {
	if a.portRoom == nil {
		return 0
	}
	room := map[route.Side]float64{}
	for _, port := range a.ports.of(node) {
		if !port.Geometry.placed() {
			room[a.portSide(port)] += a.portRoom(port)
		}
	}
	longest := 0.0
	for _, side := range sides {
		longest = math.Max(longest, room[side])
	}
	return longest
}

// portSide is the border a drawn port sits on: the form's choice, else the
// draw.io form's, inputs left, outputs right and the rest along the top.
func (a *arranger) portSide(port Port) route.Side {
	if a.side != nil {
		return a.side(port)
	}
	return routeSide(drawioPortSide(port))
}

// portPlace is where a node's drawn port sits on its border: its side and the
// fraction along it, the ports of a side spaced evenly in their order.
func (a *arranger) portPlace(node *Node, port Port) (route.Side, float64) {
	side := a.portSide(port)
	var peers []Port
	for _, p := range a.ports.of(node) {
		if a.portSide(p) == side {
			peers = append(peers, p)
		}
	}
	return side, float64(drawioPortIndex(peers, port)+1) / float64(len(peers)+1)
}

// portCellBox is a port cell's box on the page: its stated geometry at the
// port's size when it has one, else the square straddling its border.
func portCellBox(box drawioBox, side route.Side, fraction float64, g *Geometry) drawioBox {
	if g.placed() {
		size := drawioBox{Width: drawioPortSize, Height: drawioPortSize}
		if g.HasSize {
			size = drawioBox{Width: g.Width, Height: g.Height}
		}
		return drawioBox{X: g.X, Y: g.Y, Width: size.Width, Height: size.Height}
	}
	x, y := fraction, 0.0
	switch side {
	case route.West:
		x, y = 0, fraction
	case route.East:
		x, y = 1, fraction
	case route.South:
		y = 1
	}
	return drawioBox{
		X:      box.X + x*box.Width - drawioPortSize/2,
		Y:      box.Y + y*box.Height - drawioPortSize/2,
		Width:  drawioPortSize,
		Height: drawioPortSize,
	}
}

// portShape is a port cell of node as a shape of its own, named like the
// cell, with one flush pin at the middle of the face the edge leaves through:
// the face away from the owner, or the inner one for an edge into the owner.
func (a *arranger) portShape(node *Node, box drawioBox, portID string, inward bool) (route.Shape, bool) {
	if node == nil || portID == "" {
		return route.Shape{}, false
	}
	for _, port := range a.ports.of(node) {
		if port.ID != portID {
			continue
		}
		face, fraction := a.portPlace(node, port)
		cell := portCellBox(box, face, fraction, port.Geometry)
		if port.Geometry.placed() {
			face, _ = placedPortSide(box, port.Geometry)
		}
		if inward {
			face = opposite(face)
		}
		pin := route.Port{ID: port.ID, Side: face, Offset: 0.5, Flush: true}
		return route.Shape{ID: node.ID + ":" + port.ID, Box: route.Box(cell), Ports: []route.Port{pin}}, true
	}
	return route.Shape{}, false
}

func opposite(side route.Side) route.Side {
	switch side {
	case route.North:
		return route.South
	case route.South:
		return route.North
	case route.East:
		return route.West
	}
	return route.East
}

// routeSide is the router's side for the border a port direction puts it on.
func routeSide(side PortDirection) route.Side {
	switch side {
	case PortIn:
		return route.West
	case PortOut:
		return route.East
	}
	return route.North
}

// placedPortSide is the side a positioned port's centre is nearest and the
// fraction along it, as draw.io shows the port.
func placedPortSide(box drawioBox, g *Geometry) (route.Side, float64) {
	size := drawioBox{Width: drawioPortSize, Height: drawioPortSize}
	if g.HasSize {
		size = drawioBox{Width: g.Width, Height: g.Height}
	}
	cx, cy := g.X+size.Width/2, g.Y+size.Height/2
	fraction := func(v, from, span float64) float64 {
		if span <= 0 {
			return 0.5
		}
		return min(1, max(0, (v-from)/span))
	}
	side, best := route.North, cy-box.Y
	if d := box.right() - cx; d < best {
		side, best = route.East, d
	}
	if d := box.bottom() - cy; d < best {
		side, best = route.South, d
	}
	if cx-box.X < best {
		side = route.West
	}
	switch side {
	case route.North, route.South:
		return side, fraction(cx, box.X, box.Width)
	}
	return side, fraction(cy, box.Y, box.Height)
}

// drawioTree is the settled nodes by ID, each one's container, and the order
// they were settled in.
type drawioTree struct {
	nodes   map[string]*Node
	parents map[string]string
	order   []string
}

func (a *arranger) drawioTree(r *Rendering) drawioTree {
	tree := drawioTree{nodes: map[string]*Node{}, parents: map[string]string{}}
	var walk func(node *Node, parent string)
	walk = func(node *Node, parent string) {
		if _, ok := a.boxes[node.ID]; !ok {
			return
		}
		// A node shown in two places is one shape, where it first appears.
		if _, seen := tree.nodes[node.ID]; seen {
			return
		}
		tree.nodes[node.ID] = node
		tree.parents[node.ID] = parent
		tree.order = append(tree.order, node.ID)
		for _, child := range node.Children {
			walk(child, node.ID)
		}
	}
	for _, root := range r.Roots {
		walk(root, "")
	}
	return tree
}

// commonContainer is the innermost node holding both ends, one of them
// included when it holds the other; "" when they share none.
func (t drawioTree) commonContainer(a, b string) string {
	above := map[string]bool{}
	for p := a; p != ""; p = t.parents[p] {
		above[p] = true
	}
	for p := b; p != ""; p = t.parents[p] {
		if above[p] {
			return p
		}
	}
	return ""
}

// drawioRound trims a routed coordinate to a thousandth of a pixel.
func drawioRound(v float64) float64 { return math.Round(v*1000) / 1000 }
