package view

import (
	"fmt"
	"math"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view/layout"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view/route"
)

// arranged is r laid out and routed for the orthogonal DOT form, with the
// notices to write: every node is given the box the layout settled it in and
// every edge the route libavoid found, so the writer's positioned path writes
// pin nodes and stated edge positions and Graphviz only draws. The layout is
// Graphviz's layered one when a dot is found, the grid otherwise. A rendering
// the model positions, a tree (whose edges meet no port) and a build with no
// router keep Graphviz routing them, as the other routings do.
func (r *Rendering) arranged(options Options, direction Direction) (*Rendering, []string, error) {
	if r.Kind == KindTree || len(r.Edges) == 0 || placeRendering(r).positioned() {
		return r, nil, nil
	}
	skin := skinOf(options.Style)
	labels := labelsOf(r.Roots, skin.cameo, nil, r.Shown)
	labels.skin = skin
	a := newArranger(labels, r.portViewOver(options.Ports, r.Edges))
	sources := map[string]bool{}
	for _, edge := range r.Edges {
		if edge.FromPort != "" {
			sources[edge.FromPort] = true
		}
	}
	a.side = func(port Port) route.Side { return dotPortSide(port, sources) }
	a.portRoom = func(port Port) float64 {
		width, _ := dotTextExtent([]string{a.ports.pinLabel(port)}, dotPinPts)
		return dotPinSize + width + 2*dotMarginWidth
	}
	var notices []string
	positions, notice := r.layered(&a, direction)
	if notice != "" {
		notices = append(notices, notice)
	}
	a.positions = positions
	a.layout(r)
	edges, err := a.routeEdges(r)
	if err != nil {
		return nil, nil, err
	}
	if a.unrouted {
		return r, []string{"lines ortho routed by Graphviz, which ignores port positions: this build has no router"}, nil
	}
	out := r.Clone()
	out.Edges = edges
	var place func(nodes []*Node)
	place = func(nodes []*Node) {
		for _, node := range nodes {
			if box, ok := a.boxes[node.ID]; ok {
				node.Geometry = &Geometry{X: box.X, Y: box.Y, Width: box.Width, Height: box.Height, HasSize: true}
			}
			place(node.Children)
		}
	}
	place(out.Roots)
	return out, notices, nil
}

// dotPortSide is the border a port sits on in the DOT form, as portSide ranks
// it: an output, or an undirected port a connector leaves, below, the rest above.
func dotPortSide(port Port, sources map[string]bool) route.Side {
	if port.Direction == PortOut || port.Direction == PortUndirected && sources[port.ID] {
		return route.South
	}
	return route.North
}

// layered is the box Graphviz's layered layout settles every node in, or the
// notice saying why the grid lays the drawing out instead.
func (r *Rendering) layered(a *arranger, direction Direction) (map[string]drawioBox, string) {
	path, err := layout.Locate()
	if err != nil {
		return nil, fmt.Sprintf("lines ortho laid out on a grid, not layered: %v", err)
	}
	g := layout.Graph{Direction: string(direction), Gap: drawioGridGap, Pad: drawioNestedPad}
	for _, root := range r.Roots {
		g.Nodes = append(g.Nodes, a.layoutNode(root))
	}
	for _, edge := range r.Edges {
		g.Edges = append(g.Edges, layout.Edge{From: edge.From, To: edge.To})
	}
	placed, err := layout.Place(path, g)
	if err != nil {
		return nil, fmt.Sprintf("lines ortho laid out on a grid, not layered: %v", err)
	}
	boxes := make(map[string]drawioBox, len(placed))
	for id, box := range placed {
		boxes[id] = drawioBox(box)
	}
	return boxes, ""
}

// layoutNode is a node as the layered layout sizes it: a leaf at its fitted
// size, a container by its header, its members nested under it.
func (a *arranger) layoutNode(node *Node) layout.Node {
	out := layout.Node{ID: node.ID}
	if len(node.Children) == 0 {
		size := a.fitted(node)
		out.Width, out.Height = size.Width, size.Height
		return out
	}
	width, _ := a.labels.dotBox(node)
	out.Width, out.Height = math.Max(width, drawioMinWidth), a.headroom(node)
	for _, child := range node.Children {
		out.Children = append(out.Children, a.layoutNode(child))
	}
	return out
}
