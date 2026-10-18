package view

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
)

// dotPinSize is the side, in pixels, of the square a node's port is drawn as on
// its border, Cameo's pin.
const dotPinSize = 12

// dotEmptyCell pads a port strip so the pins sit between the corners.
const dotEmptyCell = "<td></td>"

// dotPinPts is the type a port's name is set in beside its square.
const dotPinPts = 8

// dotPortEnds is where the routes of the edges meet each port, by port ID, and
// the waypoint they go on to: a route's first waypoint at its source port, its
// last at its target port, and the mean of those for a port several routes meet.
func dotPortEnds(edges []Edge) map[string]routeEnd {
	sums := map[string]routeEnd{}
	counts := map[string]int{}
	meet := func(port string, at, next Point) {
		if port == "" {
			return
		}
		sum := sums[port]
		sums[port] = routeEnd{at: Point{X: sum.at.X + at.X, Y: sum.at.Y + at.Y}, next: Point{X: sum.next.X + next.X, Y: sum.next.Y + next.Y}}
		counts[port]++
	}
	for _, edge := range edges {
		if n := len(edge.Route); n > 1 {
			meet(edge.FromPort, edge.Route[0], edge.Route[1])
			meet(edge.ToPort, edge.Route[n-1], edge.Route[n-2])
		}
	}
	ends := make(map[string]routeEnd, len(sums))
	for port, sum := range sums {
		n := float64(counts[port])
		ends[port] = routeEnd{node: port, at: Point{X: sum.at.X / n, Y: sum.at.Y / n}, next: Point{X: sum.next.X / n, Y: sum.next.Y / n}}
	}
	return ends
}

// pinnedRoute is an edge's route with each end at a drawn pin brought onto the
// pin: an end the pin's box holds stays, one off it — as of a pin several
// routes meet at different points — moves to where the pin's border faces the
// next waypoint, so every route touches the one square.
func (w *dotWriter) pinnedRoute(edge Edge) []Point {
	route := edge.Route
	if pin, ok := w.pins[edge.FromPort]; ok && edge.FromPort != "" {
		route = append([]Point{pin.faces(route[0], route[1])}, route[1:]...)
	}
	if pin, ok := w.pins[edge.ToPort]; ok && edge.ToPort != "" {
		n := len(route)
		route = append(slices.Clone(route[:n-1]), pin.faces(route[n-1], route[n-2]))
	}
	return route
}

// faces is end when the box holds it, else the point of the box's border on
// the way from its centre toward next; the centre when next is the centre.
func (b nodeBox) faces(end, next Point) Point {
	if b.holds(end) {
		return end
	}
	return b.toward(next)
}

// holds reports whether p lies in the box, its border included.
func (b nodeBox) holds(p Point) bool {
	return p.X >= b.low.X && p.X <= b.high.X && p.Y >= b.low.Y && p.Y <= b.high.Y
}

// placePorts boxes every port of a boxed node: by its Layout when the node has one too, else
// where its route meets the border, else spread along the top or bottom; overlapping pins are moved apart.
func (w *dotWriter) placePorts(node *Node, ends map[string]routeEnd) {
	for _, child := range node.Children {
		w.placePorts(child, ends)
	}
	box, ok := w.boxes[node.ID]
	ports := w.ports.of(node)
	if !ok || len(ports) == 0 {
		return
	}
	var free [2][]Port
	var sides [4][]boundarySymbol
	for _, port := range ports {
		end, ok := ends[port.ID]
		switch {
		case port.Geometry.placed() && node.Geometry.placed():
			at := port.Geometry.centre()
			centre := box.centre()
			side := box.nearestSide(at, Point{X: at.X - centre.X, Y: at.Y - centre.Y})
			w.pins[port.ID] = pinBox(box, side, at)
			sides[side] = append(sides[side], boundarySymbol{id: port.ID, box: w.pins[port.ID], at: at, from: at})
		case ok:
			side := box.pinSide(end.at, end.next)
			w.pins[port.ID] = pinBox(box, side, end.at)
			sides[side] = append(sides[side], boundarySymbol{id: port.ID, box: w.pins[port.ID], at: end.at, from: end.next})
		default:
			free[w.portSide(port)] = append(free[w.portSide(port)], port)
		}
	}
	for i, ports := range free {
		side, y := topSide, box.low.Y-dotPinSize/2
		if i == 1 {
			side, y = bottomSide, box.high.Y+dotPinSize/2
		}
		for i, port := range ports {
			x := box.low.X + (box.high.X-box.low.X)*float64(i+1)/float64(len(ports)+1)
			w.pins[port.ID] = pinAround(Point{X: halfPixel(x), Y: y})
			at := w.pins[port.ID].centre()
			sides[side] = append(sides[side], boundarySymbol{id: port.ID, box: w.pins[port.ID], at: at, from: at})
		}
	}
	w.spreadPins(box, sides)
}

// pinBox is the box of a pin on side of box that a route meets at end: just
// outside the border, touching it, level with end along the side.
func pinBox(box nodeBox, side boundarySide, end Point) nodeBox {
	border := Point{X: math.Min(math.Max(end.X, box.low.X), box.high.X), Y: math.Min(math.Max(end.Y, box.low.Y), box.high.Y)}
	return box.sideBox(side, pinAround(Point{}), side.along(border), true)
}

// pinLayoutPorts boxes every port member a Layout sizes as the notation's square, on the side of
// its owner's stated box nearest the Layout's centre: the source rectangle gives a place, not a size.
func (w *dotWriter) pinLayoutPorts(node *Node, owner *nodeBox) {
	box, boxed := w.boxes[node.ID]
	if boxed && w.layoutPinned(node) {
		box = w.layoutPinBox(node, owner)
		w.boxes[node.ID] = box
	}
	var stated *nodeBox
	if boxed && box.stated {
		stated = &box
	}
	for _, child := range node.Children {
		w.pinLayoutPorts(child, stated)
	}
}

// layoutPinned reports whether node is a port drawn as a square where its sized Layout puts it:
// not a cluster, which is boxed round its drawn members instead.
func (w *dotWriter) layoutPinned(node *Node) bool {
	cluster := len(node.Children) > 0 && !w.tree
	return !cluster && isPortKind(node.Kind) && node.Geometry.placed() && node.Geometry.HasSize
}

// layoutPinBox is the square of a port a Layout sizes: on the side of owner nearest the Layout's
// centre, level with it, or centred on the Layout when no stated box bounds the owner.
func (w *dotWriter) layoutPinBox(node *Node, owner *nodeBox) nodeBox {
	at := node.Geometry.centre()
	if owner == nil {
		return pinAround(Point{X: halfPixel(at.X), Y: halfPixel(at.Y)})
	}
	centre := owner.centre()
	side := owner.nearestSide(at, Point{X: at.X - centre.X, Y: at.Y - centre.Y})
	w.spread[node.ID] = side
	return pinBox(*owner, side, at)
}

// pinAround is a pin's box centred on a point.
func pinAround(centre Point) nodeBox {
	half := float64(dotPinSize) / 2
	return nodeBox{low: Point{X: centre.X - half, Y: centre.Y - half}, high: Point{X: centre.X + half, Y: centre.Y + half}, stated: true}
}

// pinNode reports whether a port is written as a node of its own: one of a
// boxed node, drawn on its border, or of a cluster, drawn inside it. A port of
// an unplaced plain node is a cell of the node's label instead.
func (w *dotWriter) pinNode(node *Node) bool {
	_, boxed := w.boxes[node.ID]
	return boxed || w.clusters[node.ID]
}

// writePins writes the port nodes of a node: small squares, each named beside
// itself in small type, pinned on the node's border where the node is boxed.
func (w *dotWriter) writePins(node *Node, indent string) {
	if !w.pinNode(node) {
		return
	}
	for _, port := range w.ports.of(node) {
		attrs := []string{"shape=box", `label=""`, "xlabel=" + dotQuote(w.ports.pinLabel(port)), "fontsize=" + strconv.Itoa(dotPinPts),
			"width=" + dotInches(dotPinSize), "height=" + dotInches(dotPinSize), "fixedsize=true"}
		if w.skin.cameo {
			attrs = append(attrs, "fillcolor="+dotQuote(cameoNoteFill), dotColorAttr(cameoActionLine))
		}
		if box, ok := w.pins[port.ID]; ok {
			attrs = append(attrs, w.dotPin(box.centre()))
		}
		fmt.Fprintf(&w.b, "%s%s [%s];\n", indent, dotQuote(port.ID), strings.Join(attrs, ", "))
	}
}

// portSide is the side of its node a port no route meets sits on: 0 the top,
// 1 the bottom. A pin's direction places it, an input above and an output
// below. An interconnection's port has no direction, so its edges place it: a
// port a connector leaves sits below and one it reaches above, the way DOT
// ranks the ends of the edge, so the line runs between the parts and not
// through one.
func (w *dotWriter) portSide(port Port) int {
	if port.Direction == PortOut || port.Direction == PortUndirected && w.sources[port.ID] {
		return 1
	}
	return 0
}

// dotPortedLabel wraps a plain node's label in a table whose top row holds a
// cell for each port on the top side and whose bottom row one for each on the
// bottom (portSide), each a small bordered square named in small type, so an
// edge can end at the cell.
func (w *dotWriter) dotPortedLabel(node *Node, label string) string {
	in, out := w.portSides(node)
	body := strings.TrimSuffix(strings.TrimPrefix(label, "label=<"), ">")
	rows := []string{}
	if row := w.dotPortRow(in); row != "" {
		rows = append(rows, row)
	}
	rows = append(rows, `<tr><td colspan="`+fmt.Sprint(dotPortColumns(len(in), len(out)))+`">`+body+`</td></tr>`)
	if row := w.dotPortRow(out); row != "" {
		rows = append(rows, row)
	}
	return `label=<<table border="0" cellborder="0" cellspacing="0" cellpadding="2">` + strings.Join(rows, "") + `</table>>`
}

// portSides splits the drawn ports of a node by portSide: those on the top,
// then those on the bottom.
func (w *dotWriter) portSides(node *Node) (in, out []Port) {
	for _, port := range w.ports.of(node) {
		if w.portSide(port) == 1 {
			out = append(out, port)
		} else {
			in = append(in, port)
		}
	}
	return in, out
}

// dotPortColumns is the columns a ported label's table has: two per port on
// its wider row, the square and its name, at the least one for the body.
func dotPortColumns(in, out int) int {
	return max(1, 2*max(in, out))
}

// dotPinnedAttributes is a plain node's shape and label under the minimal
// display: a borderless table whose body cell draws the node's box — a
// bordered table, rounded as the skin rounds, filled and penned in the node's
// colours, which an HTML table takes from its node — and whose row above or
// below it sets each drawn port as a small square on the box's outer edge,
// named beside it in small type, so an edge ends at the square
// (`"n1":"n1.0"`) and the box itself stays the size its label needs. Graphviz
// draws a node as one shape, so the square touches the border rather than
// straddling it.
func (w *dotWriter) dotPinnedAttributes(node *Node, label string) []string {
	in, out := w.portSides(node)
	body := strings.TrimSuffix(strings.TrimPrefix(label, "label=<"), ">")
	columns := 3*max(len(in), len(out)) + 1
	box := []string{`border="1"`, `cellborder="0"`, `cellspacing="0"`, `cellpadding="6"`}
	if w.skin.rounded(node.Kind) {
		box = append(box, `style="rounded"`)
	}
	rows := []string{}
	if row := w.dotPinRow(in, columns); row != "" {
		rows = append(rows, row)
	}
	rows = append(rows, fmt.Sprintf(`<tr><td colspan="%d"><table %s><tr><td>%s</td></tr></table></td></tr>`, columns, strings.Join(box, " "), body))
	if row := w.dotPinRow(out, columns); row != "" {
		rows = append(rows, row)
	}
	attrs := append([]string{"shape=plain"}, w.fillAttributes(node)...)
	return append(attrs, `label=<<table border="0" cellborder="0" cellspacing="0" cellpadding="0">`+strings.Join(rows, "")+`</table>>`)
}

// dotPinRow is one row of pins on a box's outer edge: a spacer cell, then for
// each port its bordered square, its name and a spacer, the spacers sharing
// the width the box leaves over; "" for no ports.
func (w *dotWriter) dotPinRow(ports []Port, columns int) string {
	if len(ports) == 0 {
		return ""
	}
	cells := []string{dotEmptyCell}
	for _, port := range ports {
		cells = append(cells,
			fmt.Sprintf(`<td port=%s border="1" fixedsize="true" width="%d" height="%d" bgcolor=%s></td>`, dotQuote(port.ID), dotPinSize-2, dotPinSize-2, dotQuote(w.pinFill())),
			fmt.Sprintf(`<td align="left">%s</td>`, w.labels.sized(w.labels.size(), dotPinPts, dotEscape(w.ports.pinLabel(port)))),
			dotEmptyCell)
	}
	for len(cells) < columns {
		cells = append(cells, dotEmptyCell)
	}
	return "<tr>" + strings.Join(cells, "") + "</tr>"
}

// pinFill is the fill of a pin's square: white, Cameo's note fill in its skin.
func (w *dotWriter) pinFill() string {
	if w.skin.cameo {
		return cameoNoteFill
	}
	return "white"
}

// dotPortRow is one row of port cells: each port a bordered square cell named
// after it, or "" for no ports.
func (w *dotWriter) dotPortRow(ports []Port) string {
	if len(ports) == 0 {
		return ""
	}
	var cells []string
	for _, port := range ports {
		cells = append(cells, fmt.Sprintf(`<td port=%s border="1" fixedsize="true" width="%d" height="%d"></td><td align="left">%s</td>`,
			dotQuote(port.ID), dotPinSize-2, dotPinSize-2, w.labels.sized(w.labels.size(), dotPinPts, dotEscape(w.ports.pinLabel(port)))))
	}
	return "<tr>" + strings.Join(cells, "") + "</tr>"
}

// portEnd is the DOT endpoint of an edge at a node's port: the pin node when the
// port is written as one, else the node's label cell; "" for no port, and for a
// port of a node the DOT declares none for, whose pins and cells it writes neither.
func (w *dotWriter) portEnd(node, port string) string {
	owner, ok := w.ported[port]
	if port == "" || !ok || !w.draws(owner.ID) {
		return ""
	}
	if w.pinNode(owner) {
		return dotQuote(port)
	}
	return dotQuote(node) + ":" + dotQuote(port)
}

// collectPorts records the node each port belongs to, by port ID.
func (w *dotWriter) collectPorts(nodes []*Node) {
	for _, node := range nodes {
		for _, port := range w.ports.of(node) {
			w.ported[port.ID] = node
		}
		w.collectPorts(node.Children)
	}
}
