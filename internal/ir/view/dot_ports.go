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

// dotPinPts is the type a port's name is set in beside its square.
const dotPinPts = 8

// dotPortEnds is where the routes of the edges meet each port, by port ID: a
// route's first waypoint at its source port, its last at its target port, and
// the mean of those ends for a port several routes meet, since they all touch
// the one pin.
func dotPortEnds(edges []Edge) map[string]Point {
	sums := map[string]Point{}
	counts := map[string]int{}
	meet := func(port string, p Point) {
		if port == "" {
			return
		}
		sums[port] = Point{X: sums[port].X + p.X, Y: sums[port].Y + p.Y}
		counts[port]++
	}
	for _, edge := range edges {
		if n := len(edge.Route); n > 1 {
			meet(edge.FromPort, edge.Route[0])
			meet(edge.ToPort, edge.Route[n-1])
		}
	}
	ends := make(map[string]Point, len(sums))
	for port, sum := range sums {
		n := float64(counts[port])
		ends[port] = Point{X: sum.X / n, Y: sum.Y / n}
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
	c := b.centre()
	dx, dy := next.X-c.X, next.Y-c.Y
	if b.holds(next) || dx == 0 && dy == 0 {
		return c
	}
	t := math.Min((b.high.X-b.low.X)/2/math.Abs(dx), (b.high.Y-b.low.Y)/2/math.Abs(dy))
	return Point{X: halfPixel(c.X + dx*t), Y: halfPixel(c.Y + dy*t)}
}

// holds reports whether p lies in the box, its border included.
func (b nodeBox) holds(p Point) bool {
	return p.X >= b.low.X && p.X <= b.high.X && p.Y >= b.low.Y && p.Y <= b.high.Y
}

// placePorts finds the box of every port of a boxed node, under it: a port a
// route meets sits outside the border where the route ends, touching it; the
// rest are spread along the top edge (the inputs) and the bottom (the outputs).
func (w *dotWriter) placePorts(node *Node, ends map[string]Point) {
	for _, child := range node.Children {
		w.placePorts(child, ends)
	}
	box, ok := w.boxes[node.ID]
	if !ok || len(node.Ports) == 0 {
		return
	}
	var free [2][]Port
	for _, port := range node.Ports {
		end, ok := ends[port.ID]
		if !ok {
			side := 0
			if port.Direction == PortOut {
				side = 1
			}
			free[side] = append(free[side], port)
			continue
		}
		w.pins[port.ID] = pinBox(box, end)
	}
	for side, ports := range free {
		y := box.low.Y - dotPinSize/2
		if side == 1 {
			y = box.high.Y + dotPinSize/2
		}
		for i, port := range ports {
			x := box.low.X + (box.high.X-box.low.X)*float64(i+1)/float64(len(ports)+1)
			w.pins[port.ID] = pinAround(Point{X: halfPixel(x), Y: y})
		}
	}
}

// pinBox is the box of a pin a route meets at end: the pin sits just outside
// the node's border, touching it, on the side the end lies off — past a corner,
// the side it lies farther off; an end on or inside the border puts the pin
// outside its nearest side.
func pinBox(box nodeBox, end Point) nodeBox {
	border := Point{X: math.Min(math.Max(end.X, box.low.X), box.high.X), Y: math.Min(math.Max(end.Y, box.low.Y), box.high.Y)}
	dx, dy := end.X-border.X, end.Y-border.Y
	switch {
	case dx == 0 && dy == 0:
		dx, dy = outwardNormal(box, border)
		border = Point{X: sideAlong(dx, box.low.X, box.high.X, border.X), Y: sideAlong(dy, box.low.Y, box.high.Y, border.Y)}
	case math.Abs(dx) >= math.Abs(dy):
		dx, dy = math.Copysign(1, dx), 0
	default:
		dx, dy = 0, math.Copysign(1, dy)
	}
	return pinAround(Point{X: halfPixel(border.X + dx*dotPinSize/2), Y: halfPixel(border.Y + dy*dotPinSize/2)})
}

// sideAlong is the coordinate of the side an outward normal component points
// at, low or high, and at for a component of zero.
func sideAlong(normal, low, high, at float64) float64 {
	switch {
	case normal < 0:
		return low
	case normal > 0:
		return high
	}
	return at
}

// outwardNormal is the unit normal of the side of box nearest p, pointing out.
func outwardNormal(box nodeBox, p Point) (dx, dy float64) {
	sides := []struct {
		distance float64
		dx, dy   float64
	}{
		{p.X - box.low.X, -1, 0},
		{box.high.X - p.X, 1, 0},
		{p.Y - box.low.Y, 0, -1},
		{box.high.Y - p.Y, 0, 1},
	}
	nearest := sides[0]
	for _, side := range sides[1:] {
		if side.distance < nearest.distance {
			nearest = side
		}
	}
	return nearest.dx, nearest.dy
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
	for _, port := range node.Ports {
		attrs := []string{"shape=box", `label=""`, "xlabel=" + dotQuote(port.Name), "fontsize=" + strconv.Itoa(dotPinPts),
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

// dotPortedLabel wraps a plain node's label in a table whose top row holds a
// cell for each input port and whose bottom row one for each output, each a
// small bordered square named in small type, so an edge can end at the cell.
func (w *dotWriter) dotPortedLabel(node *Node, label string) string {
	var in, out []Port
	for _, port := range node.Ports {
		if port.Direction == PortOut {
			out = append(out, port)
		} else {
			in = append(in, port)
		}
	}
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

// dotPortColumns is the columns a ported label's table has: two per port on
// its wider row, the square and its name, at the least one for the body.
func dotPortColumns(in, out int) int {
	return max(1, 2*max(in, out))
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
			dotQuote(port.ID), dotPinSize-2, dotPinSize-2, w.labels.sized(dotPinPts, dotEscape(port.Name))))
	}
	return "<tr>" + strings.Join(cells, "") + "</tr>"
}

// portEnd is the DOT endpoint of an edge at a node's port: the pin node when the
// port is written as one, else the node's label cell; "" for no port.
func (w *dotWriter) portEnd(node, port string) string {
	if port == "" {
		return ""
	}
	if owner, ok := w.ported[port]; ok && w.pinNode(owner) {
		return dotQuote(port)
	}
	return dotQuote(node) + ":" + dotQuote(port)
}

// collectPorts records the node each port belongs to, by port ID.
func (w *dotWriter) collectPorts(nodes []*Node) {
	for _, node := range nodes {
		for _, port := range node.Ports {
			w.ported[port.ID] = node
		}
		w.collectPorts(node.Children)
	}
}
