package view

import (
	"math"
	"strings"
)

// labelEnd is the part of an edge a label names: the edge itself, drawn along
// it, or one of its ends, drawn beside that end.
type labelEnd int

const (
	labelMid labelEnd = iota
	labelFrom
	labelTo
)

// text is an edge's texts folded into one, for a form that places no text at
// an end: the end names in their ends' order about the edge's own text.
func (e Edge) text() string {
	return foldEdgeText(e.FromLabel, e.Label, e.ToLabel)
}

// Text is the edge's texts folded into one, as Edge.text folds them.
func (e EdgeData) Text() string {
	return foldEdgeText(e.FromLabel, e.Label, e.ToLabel)
}

func foldEdgeText(from, label, to string) string {
	parts := make([]string, 0, 3)
	for _, s := range []string{from, label, to} {
		if s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, " ")
}

// labelGap is the space, in pixels, between a routed edge and its label.
const labelGap = 4

// labelEndClearance is how far, in pixels, an end label keeps from the end
// itself: the length of the arrowhead or diamond drawn there.
const labelEndClearance = dotArrowLength

// labelPlacer places the labels of one positioned drawing, in pixels y-down,
// so that none sits on a node, a note or a label placed before it, nor, where
// it can be helped, across a drawn line. An end name goes beside the last
// segment before its end, the edge's own text beside the midpoint of its
// longest segment, each clear of the line by the label's half-extent and a
// gap. The candidate spots are tried in a fixed order — the nearest first, the
// other side of the line next, then farther along — and the first clear of
// everything wins, else the one covering the least box, and of those the one
// crossing the least line. An end name leaves its last segment
// for the ones before only when every spot beside it lies on a box, a note, a
// label or off the canvas: a line across it is borne rather than the name
// straying from its end.
type labelPlacer struct {
	// extent is the box, in pixels, a text takes at a font size in points.
	extent func(text string, pts float64) (width, height float64)
	// cover is how much of a box lies on the drawing's nodes and notes or off
	// its canvas, as an area, and how much of the drawing's routed lines run
	// through it, as a length.
	cover func(box nodeBox) (area, lines float64)
	// placed are the boxes of the labels placed so far, in order.
	placed []nodeBox
}

// place is where the text of the edge following route, drawn at pts points,
// is centred, at the end it names; the point is rounded to the half pixel.
func (p *labelPlacer) place(route []Point, text string, pts float64, end labelEnd) Point {
	width, height := p.extent(text, pts)
	var groups [][]Point
	if end == labelMid {
		groups = [][]Point{midCandidates(route, width, height)}
	} else {
		groups = endCandidates(route, width, height, end)
	}
	best := groups[0][0]
	area, lines := math.Inf(1), math.Inf(1)
	for _, group := range groups {
		for _, at := range group {
			box := labelBox(at, width, height)
			a, l := p.cover(box)
			for _, other := range p.placed {
				a += box.overlap(other)
			}
			if a < area || a == area && l < lines {
				best, area, lines = at, a, l
				if area == 0 && lines == 0 {
					break
				}
			}
		}
		if area == 0 {
			break
		}
	}
	p.placed = append(p.placed, labelBox(best, width, height))
	return best
}

// labelBox is the box of a label of the given extent centred at p.
func labelBox(p Point, width, height float64) nodeBox {
	return nodeBox{low: Point{X: p.X - width/2, Y: p.Y - height/2}, high: Point{X: p.X + width/2, Y: p.Y + height/2}}
}

// beside is the centre of a label of the given extent set beside the point at
// of a segment running in the direction (ux, uy): off the line by the label's
// half-extent across it and the gap, on the side given — right of a segment
// going down, above one going right, for side 1; the other for -1.
func beside(at Point, ux, uy float64, width, height float64, side float64) Point {
	nx, ny := uy, -ux
	off := math.Abs(nx)*width/2 + math.Abs(ny)*height/2 + labelGap
	return Point{X: halfPixel(at.X + side*nx*off), Y: halfPixel(at.Y + side*ny*off)}
}

// midCandidates are the spots an edge's own text may take, beside its longest
// segment: at the midpoint, the quarter points, then stepping away from the
// middle by the label's extent along the segment, each on both sides.
func midCandidates(route []Point, width, height float64) []Point {
	from, to := longestSegment(route)
	length := math.Hypot(to.X-from.X, to.Y-from.Y)
	if length == 0 {
		return []Point{from}
	}
	ux, uy := (to.X-from.X)/length, (to.Y-from.Y)/length
	step := (math.Abs(ux)*width + math.Abs(uy)*height + labelGap) / length
	alongs := []float64{0.5, 0.25, 0.75}
	for k := 1; 0.5-float64(k)*step >= 0 || 0.5+float64(k)*step <= 1; k++ {
		if along := 0.5 - float64(k)*step; along >= 0 {
			alongs = append(alongs, along)
		}
		if along := 0.5 + float64(k)*step; along <= 1 {
			alongs = append(alongs, along)
		}
	}
	var candidates []Point
	for _, along := range alongs {
		at := Point{X: from.X + (to.X-from.X)*along, Y: from.Y + (to.Y-from.Y)*along}
		for _, side := range []float64{1, -1} {
			candidates = append(candidates, beside(at, ux, uy, width, height, side))
		}
	}
	return candidates
}

// endCandidates are the spots an end name may take, nearest first: beside the
// last segment before its end at increasing distances from the end — clear of
// the arrowhead there and stepping back by the label's extent along the line —
// on both sides of the segment, then the same beside the segments before, never
// past the route's middle. The two are returned as separate groups, the last
// segment's first, either left out when empty; a route too short to hold the
// label beside any segment gets the spots beside the middle of its last
// segment, the label overhanging the segment's ends.
func endCandidates(route []Point, width, height float64, end labelEnd) [][]Point {
	if end == labelFrom {
		reversed := make([]Point, len(route))
		for i, p := range route {
			reversed[len(route)-1-i] = p
		}
		route = reversed
	}
	total := 0.0
	for i := 1; i < len(route); i++ {
		total += math.Hypot(route[i].X-route[i-1].X, route[i].Y-route[i-1].Y)
	}
	if total == 0 {
		return [][]Point{{route[len(route)-1]}}
	}
	var onLast, beyond []Point
	for d := 0.0; ; {
		at, ux, uy, last, ok := alongRoute(route, d+labelEndClearance, width, height)
		if !ok {
			break
		}
		for _, side := range []float64{1, -1} {
			if p := beside(at, ux, uy, width, height, side); last {
				onLast = append(onLast, p)
			} else {
				beyond = append(beyond, p)
			}
		}
		d += math.Abs(ux)*width + math.Abs(uy)*height + labelGap
		if d+labelEndClearance > total/2 {
			break
		}
	}
	var groups [][]Point
	for _, group := range [][]Point{onLast, beyond} {
		if len(group) > 0 {
			groups = append(groups, group)
		}
	}
	if len(groups) > 0 {
		return groups
	}
	at, ux, uy := lastSegmentMiddle(route)
	return [][]Point{{beside(at, ux, uy, width, height, 1), beside(at, ux, uy, width, height, -1)}}
}

// lastSegmentMiddle is the midpoint of the route's last segment of any length
// and that segment's direction, in route order.
func lastSegmentMiddle(route []Point) (at Point, ux, uy float64) {
	for i := len(route) - 1; i > 0; i-- {
		from, to := route[i-1], route[i]
		length := math.Hypot(to.X-from.X, to.Y-from.Y)
		if length == 0 {
			continue
		}
		return Point{X: (from.X + to.X) / 2, Y: (from.Y + to.Y) / 2}, (to.X - from.X) / length, (to.Y - from.Y) / length
	}
	return route[len(route)-1], 0, 0
}

// alongRoute is the centre of a label of the given extent whose near edge is
// back from the route's last point by d along the route, the direction, in
// route order, of the segment it lies on, and whether that is the route's
// last segment; ok is false when the route is too short to hold it on one of
// its segments.
func alongRoute(route []Point, d float64, width, height float64) (at Point, ux, uy float64, last, ok bool) {
	remaining := d
	last = true
	for i := len(route) - 1; i > 0; i-- {
		from, to := route[i-1], route[i]
		length := math.Hypot(to.X-from.X, to.Y-from.Y)
		if length == 0 {
			continue
		}
		ux, uy = (to.X-from.X)/length, (to.Y-from.Y)/length
		half := (math.Abs(ux)*width + math.Abs(uy)*height) / 2
		if remaining+2*half <= length {
			back := remaining + half
			return Point{X: to.X - ux*back, Y: to.Y - uy*back}, ux, uy, last, true
		}
		remaining = math.Max(remaining-length, 0)
		last = false
	}
	return Point{}, 0, 0, false, false
}
