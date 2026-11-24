package view

import (
	"fmt"
	"math"
	"slices"

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
	a.pixelPins = true
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
				// A Layout that only folds or hides keeps doing so where the box puts the
				// node. A port drawn as a node of its own keeps the box its label fits, as
				// the unpositioned form draws it: a sized one would be a pin on its owner.
				placed := Geometry{X: box.X, Y: box.Y, Width: box.Width, Height: box.Height, HasSize: !isPortKind(node.Kind)}
				if g := node.Geometry; g != nil {
					placed.Collapsed, placed.Hidden = g.Collapsed, g.Hidden
				}
				node.Geometry = &placed
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

// dotAnchorClearance is how far, in pixels, an orthogonal drawing keeps a
// cluster's anchor from the nodes and pins drawn in the cluster.
const dotAnchorClearance = 4

// anchorPoint is where a cluster's anchor is pinned: its box's centre, or under the orthogonal routing the nearest point clear
// of its nodes, the pins and the notes (the clearest where they leave less room), since Graphviz routes none once two nodes touch.
func (w *dotWriter) anchorPoint(node *Node, box nodeBox) Point {
	centre := box.centre()
	if w.lines != LinesOrtho {
		return centre
	}
	var obstacles []nodeBox
	var members func(nodes []*Node)
	members = func(nodes []*Node) {
		for _, member := range nodes {
			if len(member.Children) > 0 || w.omitted[member.ID] {
				members(member.Children)
			} else if drawn, ok := w.boxes[member.ID]; ok {
				obstacles = append(obstacles, drawn)
			}
		}
	}
	members(node.Children)
	for _, pin := range w.pins {
		obstacles = append(obstacles, pin)
	}
	for _, note := range w.noteBoxes {
		if !note.encloses(box) {
			obstacles = append(obstacles, note)
		}
	}
	free := func(p Point) bool {
		for _, o := range obstacles {
			if p.X > o.low.X-dotAnchorClearance && p.X < o.high.X+dotAnchorClearance &&
				p.Y > o.low.Y-dotAnchorClearance && p.Y < o.high.Y+dotAnchorClearance {
				return false
			}
		}
		return true
	}
	if free(centre) {
		return centre
	}
	// Beside an obstacle the nearest clear point lies on a line one clearance off a box's side.
	xs, ys := []float64{centre.X}, []float64{centre.Y}
	for _, o := range obstacles {
		xs = append(xs, o.low.X-dotAnchorClearance, o.high.X+dotAnchorClearance)
		ys = append(ys, o.low.Y-dotAnchorClearance, o.high.Y+dotAnchorClearance)
	}
	best, found := centre, false
	for _, x := range xs {
		for _, y := range ys {
			p := Point{X: x, Y: y}
			if x <= box.low.X || x >= box.high.X || y <= box.low.Y || y >= box.high.Y || !free(p) {
				continue
			}
			if !found || nearer(p, best, centre) {
				best, found = p, true
			}
		}
	}
	if found {
		return best
	}
	return clearestPoint(box, obstacles, centre)
}

// clearestPoint is the point in the box farthest from the obstacles when none is a clearance off them all:
// the middle of a cell their sides cut the box into, so one touching nothing is found wherever there is one.
func clearestPoint(box nodeBox, obstacles []nodeBox, centre Point) Point {
	xs, ys := []float64{box.low.X, box.high.X}, []float64{box.low.Y, box.high.Y}
	for _, o := range obstacles {
		xs = append(xs, o.low.X, o.high.X)
		ys = append(ys, o.low.Y, o.high.Y)
	}
	xs, ys = middlesWithin(xs, box.low.X, box.high.X), middlesWithin(ys, box.low.Y, box.high.Y)
	best, room := centre, 0.0
	for _, x := range xs {
		for _, y := range ys {
			p, clear := Point{X: x, Y: y}, math.Inf(1)
			for _, o := range obstacles {
				clear = math.Min(clear, max(o.low.X-x, x-o.high.X, o.low.Y-y, y-o.high.Y))
			}
			if clear > room || clear == room && room > 0 && nearer(p, best, centre) {
				best, room = p, clear
			}
		}
	}
	return best
}

// middlesWithin is the midpoint of every two neighbouring values that lie from low to high.
func middlesWithin(values []float64, low, high float64) []float64 {
	slices.Sort(values)
	values = slices.Compact(values)
	var middles []float64
	for i := 1; i < len(values); i++ {
		if values[i-1] >= low && values[i] <= high {
			middles = append(middles, (values[i-1]+values[i])/2)
		}
	}
	return middles
}

// nearer reports whether p lies closer to the target than q, the upper and
// then the left one of two as close.
func nearer(p, q, target Point) bool {
	dp, dq := math.Hypot(p.X-target.X, p.Y-target.Y), math.Hypot(q.X-target.X, q.Y-target.Y)
	if dp != dq {
		return dp < dq
	}
	if p.Y != q.Y {
		return p.Y < q.Y
	}
	return p.X < q.X
}
