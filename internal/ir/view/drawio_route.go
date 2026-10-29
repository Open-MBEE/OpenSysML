package view

import (
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view/route"
)

// routeEdges gives every edge the rendering leaves unrouted the path libavoid
// finds around the settled boxes, as the diagram panel routes it: edges are
// routed in groups by the container both ends lie in, kept inside it, with the
// containers an end lies in reduced to their headers so routes may cross them.
func (w *drawioWriter) routeEdges(r *Rendering) ([]Edge, error) {
	edges := append([]Edge(nil), r.Edges...)
	tree := w.drawioTree(r)
	groups := map[string][]int{}
	for i, edge := range edges {
		_, from := w.boxes[edge.From]
		_, to := w.boxes[edge.To]
		if len(edge.Route) > 0 || !from || !to {
			continue
		}
		key := tree.commonContainer(edge.From, edge.To)
		// A member drawn outside its container is not held by it.
		for key != "" && !(w.boxes[key].contains(w.boxes[edge.From]) && w.boxes[key].contains(w.boxes[edge.To])) {
			key = tree.parents[key]
		}
		groups[key] = append(groups[key], i)
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		group := groups[key]
		exempt := map[string]bool{}
		for _, i := range group {
			for _, end := range []string{edges[i].From, edges[i].To} {
				for p := tree.parents[end]; p != ""; p = tree.parents[p] {
					exempt[p] = true
				}
			}
		}
		// An edge from the container to a member leaves its header above the
		// member, through a pin on the header's underside at the member's centre.
		header := w.boxes[key]
		if key != "" {
			header.Height = w.headroom(tree.nodes[key])
		}
		var memberPins []route.Port
		portShapes := map[string]route.Shape{}
		asked := make([]route.Edge, len(group))
		for j, i := range group {
			asked[j] = route.Edge{Index: j, From: edges[i].From, To: edges[i].To, FromPort: edges[i].FromPort, ToPort: edges[i].ToPort}
			if key == "" || (edges[i].From != key && edges[i].To != key) || edges[i].From == edges[i].To {
				continue
			}
			member, port := edges[i].To, edges[i].FromPort
			if member == key {
				member, port = edges[i].From, edges[i].ToPort
			}
			// The container's own named port is a cell of its own: the edge
			// leaves it through its inner face, into the container.
			if shape, ok := w.portShape(tree.nodes[key], w.boxes[key], port, true); ok {
				portShapes[shape.ID] = shape
				if edges[i].From == key {
					w.inward[i] |= drawioFrom
				} else {
					w.inward[i] |= drawioTo
				}
				continue
			}
			pin := route.Port{ID: fmt.Sprintf("\x00member/%d", i), Side: route.South, Flush: true}
			pin.Offset = min(1, max(0, (w.boxes[member].X+w.boxes[member].Width/2-header.X)/header.Width))
			memberPins = append(memberPins, pin)
			if edges[i].From == key {
				asked[j].FromPort = pin.ID
			} else {
				asked[j].ToPort = pin.ID
			}
		}
		shapes := make([]route.Shape, 0, len(tree.order))
		for _, id := range tree.order {
			node := tree.nodes[id]
			box := w.boxes[id]
			if exempt[id] {
				box.Height = w.headroom(node)
			}
			ports, placed := w.routePorts(node, box)
			for _, shape := range placed {
				// The container's port an edge enters it through keeps its inner face.
				if _, inward := portShapes[shape.ID]; !inward {
					portShapes[shape.ID] = shape
				}
			}
			shape := route.Shape{ID: id, Box: route.Box(box), Ports: ports}
			if id == key {
				shape.Box = route.Box(header)
				shape.Ports = append(shape.Ports, memberPins...)
			}
			shapes = append(shapes, shape)
		}
		for _, id := range slices.Sorted(maps.Keys(portShapes)) {
			shapes = append(shapes, portShapes[id])
		}
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
		if key != "" {
			b := route.Box(w.boxes[key])
			bounds = &b
		}
		routes, err := route.Routes(shapes, asked, bounds)
		if errors.Is(err, route.ErrUnavailable) {
			w.unrouted = true
			return edges, nil
		}
		if err != nil {
			return nil, err
		}
		for _, found := range routes {
			i := group[found.Index]
			// Routed points carry float noise a declared route never has.
			for _, p := range found.Points {
				edges[i].Route = append(edges[i].Route, Point{X: drawioRound(p.X), Y: drawioRound(p.Y)})
			}
			w.routed[i] = true
		}
		for _, i := range group {
			if !w.routed[i] {
				w.unroutable = append(w.unroutable, i)
			}
		}
	}
	return edges, nil
}

// routePorts is where a node's drawn ports sit for the router: a port drawn
// on the border at a fraction is a pin of the node's shape; a positioned
// port, wherever it was put, is a shape of its own with a pin on its outer face.
func (w *drawioWriter) routePorts(node *Node, box drawioBox) ([]route.Port, []route.Shape) {
	ports := w.ports.of(node)
	if len(ports) == 0 {
		return nil, nil
	}
	sides := map[PortDirection][]Port{}
	for _, port := range ports {
		sides[drawioPortSide(port)] = append(sides[drawioPortSide(port)], port)
	}
	var pins []route.Port
	var shapes []route.Shape
	for _, port := range ports {
		if port.Geometry.placed() {
			shape, _ := w.portShape(node, box, port.ID, false)
			shapes = append(shapes, shape)
			continue
		}
		side := drawioPortSide(port)
		pins = append(pins, route.Port{ID: port.ID, Side: routeSide(side), Offset: float64(drawioPortIndex(sides[side], port)+1) / float64(len(sides[side])+1)})
	}
	return pins, shapes
}

// portShape is a port cell of node as a shape of its own, named like the
// cell, with one flush pin at the middle of the face the edge leaves through:
// the face away from the owner, or the inner one for an edge into the owner.
func (w *drawioWriter) portShape(node *Node, box drawioBox, portID string, inward bool) (route.Shape, bool) {
	if node == nil || portID == "" {
		return route.Shape{}, false
	}
	ports := w.ports.of(node)
	sides := map[PortDirection][]Port{}
	for _, port := range ports {
		sides[drawioPortSide(port)] = append(sides[drawioPortSide(port)], port)
	}
	for _, port := range ports {
		if port.ID != portID {
			continue
		}
		side := drawioPortSide(port)
		fraction := float64(drawioPortIndex(sides[side], port)+1) / float64(len(sides[side])+1)
		cell := drawioPortCellBox(box, side, fraction, port.Geometry)
		face := routeSide(side)
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
	if d := cx - box.X; d < best {
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

func (w *drawioWriter) drawioTree(r *Rendering) drawioTree {
	tree := drawioTree{nodes: map[string]*Node{}, parents: map[string]string{}}
	var walk func(node *Node, parent string)
	walk = func(node *Node, parent string) {
		if _, ok := w.boxes[node.ID]; !ok {
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
