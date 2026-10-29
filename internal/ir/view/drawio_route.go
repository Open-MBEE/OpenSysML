package view

import (
	"fmt"
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
		asked := make([]route.Edge, len(group))
		for j, i := range group {
			asked[j] = route.Edge{From: edges[i].From, To: edges[i].To, FromPort: edges[i].FromPort, ToPort: edges[i].ToPort}
			if key == "" || (edges[i].From != key && edges[i].To != key) || edges[i].From == edges[i].To {
				continue
			}
			member := edges[i].To
			if member == key {
				member = edges[i].From
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
			shape := route.Shape{ID: id, Box: route.Box(box), Ports: w.routePorts(node, box)}
			if id == key {
				shape.Box = route.Box(header)
				shape.Ports = append(shape.Ports, memberPins...)
			}
			shapes = append(shapes, shape)
		}
		var bounds *route.Box
		if key != "" {
			b := route.Box(w.boxes[key])
			bounds = &b
		}
		routes, err := route.Routes(shapes, asked, bounds)
		if err != nil {
			return nil, err
		}
		for j, i := range group {
			for _, p := range routes[j] {
				edges[i].Route = append(edges[i].Route, Point{X: p.X, Y: p.Y})
			}
		}
	}
	return edges, nil
}

// routePorts is where a node's drawn ports sit for the router: on the side
// and at the fraction the port cell is written at.
func (w *drawioWriter) routePorts(node *Node, box drawioBox) []route.Port {
	ports := w.ports.of(node)
	if len(ports) == 0 {
		return nil
	}
	sides := map[PortDirection][]Port{}
	for _, port := range ports {
		sides[drawioPortSide(port)] = append(sides[drawioPortSide(port)], port)
	}
	out := make([]route.Port, 0, len(ports))
	for _, port := range ports {
		side := drawioPortSide(port)
		p := route.Port{ID: port.ID, Side: routeSide(side), Offset: float64(drawioPortIndex(sides[side], port)+1) / float64(len(sides[side])+1)}
		if g := port.Geometry; g.placed() {
			p.Side, p.Offset = placedPortSide(box, g)
		}
		out = append(out, p)
	}
	return out
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
