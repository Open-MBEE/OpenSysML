package view

import "fmt"

// elideStandIns takes out of a positioned rendering the control nodes a
// migration made up — a fork, join or merge with a synthesized name that
// nothing positions — since no diagram symbol stands at one. Each edge into
// such a node meets each edge out of it: the pair is redrawn as one edge
// between the nodes the stand-in was written between, along whichever route
// the two had, and a pair with no route is left undrawn, as the migration's
// own wiring rather than the diagram's.
func elideStandIns(out *Rendering) {
	if !positionedRendering(out) {
		return
	}
	elided, dropped := 0, 0
	for {
		standIn := findStandIn(out.Roots)
		if standIn == nil {
			break
		}
		dropped += elideNode(out, standIn.ID)
		elided++
	}
	if elided == 0 {
		return
	}
	notice := fmt.Sprintf("%d control node(s) a migration made up, which no diagram positions, elided", elided)
	if dropped > 0 {
		notice += fmt.Sprintf(", and %d edge(s) through them without a route", dropped)
	}
	out.Notices = append(out.Notices, notice)
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

// findStandIn is the first made-up control node under nodes, nil for none.
func findStandIn(nodes []*Node) *Node {
	for _, node := range nodes {
		if standInKind(node.Kind) && node.NameSynthesized && node.Geometry == nil && len(node.Children) == 0 {
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
func elideNode(out *Rendering, id string) int {
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
	for _, in := range into {
		for _, o := range from {
			joined, ok := joinEdges(in, o)
			if !ok {
				dropped++
				continue
			}
			kept = append(kept, joined)
		}
	}
	out.Edges = kept
	out.Roots = removeNode(out.Roots, id)
	notes := make([]Note, 0, len(out.Notes))
	for _, note := range out.Notes {
		if note.Anchor != id && note.EdgeFrom != id && note.EdgeTo != id {
			notes = append(notes, note)
		}
	}
	out.Notes = notes
	return dropped
}

// joinEdges is the edge in and out draw as one, along in's route continued by
// out's; false when neither has a route.
func joinEdges(in, out Edge) (Edge, bool) {
	route := append([]Point(nil), in.Route...)
	switch {
	case len(route) > 1 && len(out.Route) > 1:
		route = append(route, out.Route[1:]...)
	case len(out.Route) > 1:
		route = append([]Point(nil), out.Route...)
	case len(route) < 2:
		return Edge{}, false
	}
	joined := out
	joined.From, joined.FromPort = in.From, in.FromPort
	joined.Label = joinNonEmpty(in.Label, out.Label)
	joined.Route = route
	if joined.Style == nil {
		joined.Style = in.Style
	}
	return joined, true
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
