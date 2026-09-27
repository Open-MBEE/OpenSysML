package view

import "fmt"

// withoutStandIns is a positioned rendering without the control nodes a
// migration made up — a fork, join or merge it marked as standing for no
// source element, which nothing positions — for a drawing that leaves unplaced
// nodes undrawn, since no diagram symbol stands at one. Each edge into such a
// node meets each edge out of it: the pair is redrawn as one edge between the
// nodes the stand-in was written between, along whichever route the two had,
// and a pair with no route is left undrawn, as the migration's own wiring
// rather than the diagram's. It is r itself when there is nothing to elide,
// else a copy; r is not changed.
func withoutStandIns(r *Rendering) *Rendering {
	if !positionedRendering(r) || findStandIn(r.Roots) == nil {
		return r
	}
	out := r.Clone()
	geometry := map[string]*Geometry{}
	collectGeometry(out.Roots, geometry)
	elided, dropped := 0, 0
	for {
		standIn := findStandIn(out.Roots)
		if standIn == nil {
			break
		}
		dropped += elideNode(out, standIn.ID, geometry)
		elided++
	}
	notice := fmt.Sprintf("%d control node(s) a migration made up, which no diagram positions, elided", elided)
	if dropped > 0 {
		notice += fmt.Sprintf(", and %d edge(s) through them without a route", dropped)
	}
	out.Notices = append(out.Notices, notice)
	return out
}

// collectGeometry maps every node under nodes, by ID, to its stated geometry.
func collectGeometry(nodes []*Node, into map[string]*Geometry) {
	for _, node := range nodes {
		into[node.ID] = node.Geometry
		collectGeometry(node.Children, into)
	}
}

// positionedRendering reports whether any node of r has stated geometry or any
// edge a route: a rendering a diagram places, in which unplaced nodes are undrawn.
func positionedRendering(r *Rendering) bool {
	for _, edge := range r.Edges {
		if len(edge.Route) > 1 {
			return true
		}
	}
	var positioned func(nodes []*Node) bool
	positioned = func(nodes []*Node) bool {
		for _, node := range nodes {
			if node.Geometry != nil || positioned(node.Children) {
				return true
			}
		}
		return false
	}
	return positioned(r.Roots)
}

// findStandIn is the first made-up control node under nodes, nil for none: one
// the model marks as a stand-in, not merely one whose name was made up, since
// a source's own unnamed fork is named the same way yet has a symbol.
func findStandIn(nodes []*Node) *Node {
	for _, node := range nodes {
		if standInKind(node.Kind) && node.StandIn && node.Geometry == nil && len(node.Children) == 0 {
			return node
		}
		if found := findStandIn(node.Children); found != nil {
			return found
		}
	}
	return nil
}

// standInKind reports whether a node of kind is a control node a migration may
// make up to thread edges through: one with no text of its own.
func standInKind(kind string) bool {
	return kind == "fork" || kind == "join" || kind == "merge"
}

// elideNode removes node id from out, joining its edges pairwise; it returns
// how many pairs had no route to be drawn along.
func elideNode(out *Rendering, id string, geometry map[string]*Geometry) int {
	var into, from, kept []Edge
	for _, edge := range out.Edges {
		switch {
		case edge.To == id && edge.From == id:
		case edge.To == id:
			into = append(into, edge)
		case edge.From == id:
			from = append(from, edge)
		default:
			kept = append(kept, edge)
		}
	}
	dropped := 0
	var joins [][2]string
	for _, in := range into {
		for _, o := range from {
			joined, ok := joinEdges(in, o, geometry)
			if !ok {
				dropped++
				continue
			}
			kept = append(kept, joined)
			joins = append(joins, [2]string{in.From, o.To})
		}
	}
	out.Edges = kept
	out.Roots = removeNode(out.Roots, id)
	out.Notes = rejoinNotes(out.Notes, id, joins)
	return dropped
}

// rejoinNotes moves the notes on the edges into and out of the elided node id
// onto the joined edges, given as (from, to) pairs, continuing each: a note on
// `a -> id` goes on every `a -> c` joined, one on `id -> c` on every `a -> c`.
// Notes on the node itself, and on edges no join continued, are dropped.
func rejoinNotes(notes []Note, id string, joins [][2]string) []Note {
	kept := make([]Note, 0, len(notes))
	for _, note := range notes {
		switch {
		case note.Anchor == id:
		case note.EdgeTo == id:
			for _, join := range joins {
				if join[0] == note.EdgeFrom {
					moved := note
					moved.EdgeTo = join[1]
					kept = append(kept, moved)
				}
			}
		case note.EdgeFrom == id:
			for _, join := range joins {
				if join[1] == note.EdgeTo {
					moved := note
					moved.EdgeFrom = join[0]
					kept = append(kept, moved)
				}
			}
		default:
			kept = append(kept, note)
		}
	}
	return kept
}

// joinEdges is the edge in and out draw as one, along in's route continued by
// out's; false when neither has a route. Where one alone has a route, it ends
// at the elided node, so the way on to the other's end is added: straight to
// the border of that node's box, when its geometry gives one.
func joinEdges(in, out Edge, geometry map[string]*Geometry) (Edge, bool) {
	route := append([]Point(nil), in.Route...)
	switch {
	case len(route) > 1 && len(out.Route) > 1:
		route = append(route, out.Route[1:]...)
	case len(out.Route) > 1:
		route = reachFrom(geometry[in.From], out.Route)
	case len(route) < 2:
		return Edge{}, false
	default:
		route = reversed(reachFrom(geometry[out.To], reversed(route)))
	}
	joined := out
	joined.From, joined.FromPort = in.From, in.FromPort
	joined.Label = joinNonEmpty(in.Label, out.Label)
	joined.Name = joinNonEmpty(in.Name, out.Name)
	joined.Route = route
	if joined.Style == nil {
		joined.Style = in.Style
	}
	return joined, true
}

// reachFrom is route led out of the box g states, when it states one and the
// route starts off it: from the point of the border facing the route's start.
func reachFrom(g *Geometry, route []Point) []Point {
	if g == nil || !g.HasSize {
		return route
	}
	box := nodeBox{low: Point{X: g.X, Y: g.Y}, high: Point{X: g.X + g.Width, Y: g.Y + g.Height}}
	if box.holds(route[0]) {
		return route
	}
	return append([]Point{box.faces(route[0], route[0])}, route...)
}

// removeNode is nodes without the node id, wherever it is nested.
func removeNode(nodes []*Node, id string) []*Node {
	kept := make([]*Node, 0, len(nodes))
	for _, node := range nodes {
		if node.ID == id {
			continue
		}
		node.Children = removeNode(node.Children, id)
		kept = append(kept, node)
	}
	return kept
}
