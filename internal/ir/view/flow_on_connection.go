package view

import (
	"math"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// flowSite is a flow edge an interconnection drew: its index among the edges,
// the flow, and the node it was found in.
type flowSite struct {
	index  int
	sym    *symbols.Symbol
	within *Node
}

// lineEnds are the two ends of a connection line, as a flow's ends match them
// in either order.
type lineEnds struct {
	from, fromPort, to, toPort string
}

func edgeLineEnds(edge Edge) lineEnds {
	return lineEnds{edge.From, edge.FromPort, edge.To, edge.ToPort}
}

func (l lineEnds) reversed() lineEnds { return lineEnds{l.to, l.toPort, l.from, l.fromPort} }

// flowsOnConnections redraws each flow whose ends are a connection's as §8.2.3.16's item box
// splitting the line (label on the first segment); a flow with no such connection stays an edge.
func (w *featureWalk) flowsOnConnections() {
	out := w.out
	onLine := map[int][]flowSite{}
	drop := map[int]bool{}
	for _, flow := range w.flows {
		line := w.lineOf(out.Edges[flow.index])
		if line < 0 {
			continue
		}
		onLine[line] = append(onLine[line], flow)
		drop[flow.index] = true
	}
	if len(onLine) == 0 {
		return
	}
	edges := make([]Edge, 0, len(out.Edges)+len(drop))
	for i, edge := range out.Edges {
		switch flows := onLine[i]; {
		case drop[i]:
		case len(flows) > 0:
			edges = append(edges, w.splitLine(edge, flows)...)
		default:
			edges = append(edges, edge)
		}
	}
	out.Edges = edges
}

// lineOf is the index of the connection line the flow edge lies on — the one
// connection joining the flow's two ends — or -1 for none.
func (w *featureWalk) lineOf(flow Edge) int {
	ends := edgeLineEnds(flow)
	for i, edge := range w.out.Edges {
		if edge.Kind != EdgeConnection || w.dotSegment(edge) {
			continue
		}
		if line := edgeLineEnds(edge); line == ends || line == ends.reversed() {
			return i
		}
	}
	return -1
}

// dotSegment reports whether a connection edge is a segment of an n-ary dot.
func (w *featureWalk) dotSegment(edge Edge) bool {
	for _, nodes := range w.nodes {
		for _, node := range nodes {
			if node.ID == edge.From && isDotKind(node.Kind) {
				return true
			}
		}
	}
	return false
}

// splitLine is the connection line redrawn as segments between its ends and the item boxes;
// a stated route is cut at even intervals, each unplaced box standing where it is cut.
func (w *featureWalk) splitLine(line Edge, flows []flowSite) []Edge {
	boxes := make([]*Node, len(flows))
	for i, flow := range flows {
		boxes[i] = w.itemBox(w.out.Edges[flow.index], flow, line)
		w.moveFlowNotes(flow, boxes[i], line)
	}
	routes := splitRoute(line.Route, len(boxes))
	atCut := make([]bool, len(boxes))
	for i, box := range boxes {
		if box.Geometry == nil && routes != nil && len(routes[i+1]) > 0 {
			cut := routes[i+1][0]
			box.Geometry, atCut[i] = &Geometry{X: cut.X, Y: cut.Y}, true
		}
	}
	stops := make([]edgeEnd, 0, len(boxes)+2)
	stops = append(stops, edgeEnd{port: line.FromPort, node: &Node{ID: line.From}})
	for _, box := range boxes {
		stops = append(stops, edgeEnd{node: box})
	}
	stops = append(stops, edgeEnd{port: line.ToPort, node: &Node{ID: line.To}})
	segments := make([]Edge, 0, len(stops)-1)
	for i := 1; i < len(stops); i++ {
		segment := Edge{From: stops[i-1].node.ID, FromPort: stops[i-1].port, To: stops[i].node.ID, ToPort: stops[i].port,
			Kind: EdgeConnection, Origin: line.Origin, Style: line.Style}
		if routes != nil {
			segment.Route = segmentRoute(routes[i-1], i > 1 && atCut[i-2], i < len(stops)-1 && atCut[i-1])
		}
		if i == 1 {
			segment.Label, segment.Name, segment.FromLabel = line.Label, line.Name, line.FromLabel
		}
		if i == len(stops)-1 {
			segment.ToLabel = line.ToLabel
		}
		segments = append(segments, segment)
	}
	// A note on the line, or on a flow drawn on it, stays with the first segment.
	for i := range w.out.Notes {
		note := &w.out.Notes[i]
		if ends := [2]string{note.EdgeFrom, note.EdgeTo}; ends == [2]string{line.From, line.To} || ends == [2]string{line.To, line.From} {
			note.EdgeFrom, note.EdgeTo = line.From, boxes[0].ID
		}
	}
	return segments
}

// itemBox is a flow's box on the connection line: keyword its kind, text the flow's label,
// arrow the way the item goes (right toward the line's second end, left toward its first).
func (w *featureWalk) itemBox(flow Edge, site flowSite, line Edge) *Node {
	r, out := w.r, w.out
	kind, text := flowBoxKind(site.sym, flow.Kind), flow.Label
	if flow.Kind == EdgeSuccessionFlow {
		text = strings.TrimSpace(strings.TrimPrefix(text, successionFlowKeyword))
	}
	arrow := arrowRight
	if flow.From != line.From || flow.FromPort != line.FromPort {
		arrow = arrowLeft
	}
	box := &Node{ID: w.ids.take(), Kind: kind, Text: text, Arrow: arrow, Origin: flow.Origin,
		Geometry: r.geometryOf(w.view, site.sym, out), Style: flow.Style}
	if name := r.connectorName(site.sym); name != "" && flowPayload(site.sym) == "" {
		box.Name, box.Text = name, ""
	}
	w.nodes[site.sym] = append(w.nodes[site.sym], box)
	if site.within == nil {
		out.Roots = append(out.Roots, box)
	} else {
		w.parent[box] = site.within
		site.within.Children = append(site.within.Children, box)
	}
	return box
}

// flowBoxKind is the keyword of an item box: `message` for a message, else the
// flow's edge kind — `flow` or `succession flow`.
func flowBoxKind(sym *symbols.Symbol, kind EdgeKind) string {
	if usage, ok := sym.Decl.(*ast.Usage); ok && usage.Keyword == "message" {
		return "message"
	}
	return kind.String()
}

// moveFlowNotes anchors the notes of a flow drawn as an item box on the box: the ones
// its edge got, anchored on the line's ends like the connection's own, are dropped.
func (w *featureWalk) moveFlowNotes(flow flowSite, box *Node, line Edge) {
	out := w.out
	before, notices := len(out.Notes), len(out.Notices)
	w.r.notesOf(w.view, flow.sym, box.ID, out)
	out.Notices = out.Notices[:notices]
	moved := map[Origin]bool{}
	for _, note := range out.Notes[before:] {
		moved[note.Origin] = true
	}
	kept := make([]Note, 0, len(out.Notes))
	for i, note := range out.Notes {
		ends := [2]string{note.EdgeFrom, note.EdgeTo}
		onLine := ends == [2]string{line.From, line.To} || ends == [2]string{line.To, line.From}
		if i < before && onLine && moved[note.Origin] {
			continue
		}
		kept = append(kept, note)
	}
	out.Notes = kept
}

// segmentRoute is a piece of a split route less the cut points a box stands
// on: the segment ends at the box, so such a cut adds no bend.
func segmentRoute(piece []Point, headCut, tailCut bool) []Point {
	if headCut {
		piece = piece[1:]
	}
	if tailCut && len(piece) > 0 {
		piece = piece[:len(piece)-1]
	}
	if len(piece) == 0 {
		return nil
	}
	return piece
}

// splitRoute cuts a stated route into n+1 pieces of equal length sharing their cut points;
// nil for no route, and a lone waypoint, with no length to cut, stays on the first piece.
func splitRoute(route []Point, n int) [][]Point {
	if len(route) == 0 {
		return nil
	}
	if len(route) == 1 {
		pieces := make([][]Point, n+1)
		pieces[0] = route
		return pieces
	}
	total := 0.0
	for i := 1; i < len(route); i++ {
		total += math.Hypot(route[i].X-route[i-1].X, route[i].Y-route[i-1].Y)
	}
	pieces := make([][]Point, 0, n+1)
	piece := []Point{route[0]}
	walked, next, cuts := 0.0, total/float64(n+1), 0
	for i := 1; i < len(route); i++ {
		a, b := route[i-1], route[i]
		length := math.Hypot(b.X-a.X, b.Y-a.Y)
		for cuts < n && length > 0 && walked+length >= next {
			t := (next - walked) / length
			cut := Point{X: a.X + t*(b.X-a.X), Y: a.Y + t*(b.Y-a.Y)}
			pieces = append(pieces, append(piece, cut))
			piece = []Point{cut}
			cuts++
			next = total * float64(cuts+1) / float64(n+1)
		}
		walked += length
		piece = append(piece, b)
	}
	for len(pieces) < n+1 {
		pieces = append(pieces, piece)
		piece = []Point{route[len(route)-1]}
	}
	return pieces
}
