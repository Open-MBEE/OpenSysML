package route

import "math"

type axis int

const (
	horizontal axis = iota
	vertical
)

// segment is an orthogonal segment's axis, direction (-1 or 1) and length.
type segment struct {
	axis      axis
	direction float64
	length    float64
}

// jogShift moves the two points of a short jog onto the neighbouring segment.
type jogShift struct {
	points         [2]int
	movedSegment   int
	changedSegment int
	mergedSegment  [2]int
	delta          Point
}

// obstacle is a shape's routing geometry under its ID.
type obstacle struct {
	id string
	Obstacle
}

// scene is what a jog must respect: the other routes, the obstacles, each route's ends and the bounds.
type scene struct {
	routes    []Route
	obstacles []obstacle
	ends      map[int][2]string
	bounds    *Box
}

// StraightenJogs straightens short Z-jogs without losing clearance or adding
// wire crossings; obstacles and ends are keyed by shape ID and edge index.
func StraightenJogs(routes []Route, obstacles []obstacle, ends map[int][2]string, bounds *Box) []Route {
	straightened := make([]Route, len(routes))
	for i, route := range routes {
		straightened[i] = Route{Index: route.Index, Points: append([]Point(nil), route.Points...)}
	}
	sc := &scene{routes: straightened, obstacles: obstacles, ends: ends, bounds: bounds}
	for {
		changed := false
		for i := range straightened {
			route := straightened[i].Points
			for jog := 1; jog+2 < len(route); jog++ {
				shifts := jogShifts(route, jog)
				sortStable(shifts, func(first, second jogShift) bool {
					return segmentLength(route[first.movedSegment], route[first.movedSegment+1]) <
						segmentLength(route[second.movedSegment], route[second.movedSegment+1])
				})
				for _, shift := range shifts {
					candidate := shiftedRoute(route, shift)
					if !legalShift(straightened[i].Index, route, candidate, shift, sc) {
						continue
					}
					straightened[i].Points = CompactRoute(candidate)
					changed = true
					break
				}
				if changed {
					break
				}
			}
			if changed {
				break
			}
		}
		if !changed {
			return straightened
		}
	}
}

// sortStable is an insertion sort, stable like Array.prototype.sort, for the two shifts a jog can have.
func sortStable(shifts []jogShift, less func(a, b jogShift) bool) {
	for i := 1; i < len(shifts); i++ {
		for j := i; j > 0 && less(shifts[j], shifts[j-1]); j-- {
			shifts[j], shifts[j-1] = shifts[j-1], shifts[j]
		}
	}
}

func jogShifts(route []Point, jog int) []jogShift {
	previous, okPrevious := orthogonalSegment(route[jog-1], route[jog])
	short, okShort := orthogonalSegment(route[jog], route[jog+1])
	next, okNext := orthogonalSegment(route[jog+1], route[jog+2])
	if !okPrevious || !okShort || !okNext ||
		short.length >= MinJog ||
		previous.axis != next.axis ||
		previous.axis == short.axis ||
		previous.direction != next.direction {
		return nil
	}
	delta := Point{X: route[jog+1].X - route[jog].X, Y: route[jog+1].Y - route[jog].Y}
	var shifts []jogShift
	if jog > 1 {
		shifts = append(shifts, jogShift{
			points:         [2]int{jog - 1, jog},
			movedSegment:   jog - 1,
			changedSegment: jog - 2,
			mergedSegment:  [2]int{jog - 1, jog + 2},
			delta:          delta,
		})
	}
	if jog+2 < len(route)-1 {
		shifts = append(shifts, jogShift{
			points:         [2]int{jog + 1, jog + 2},
			movedSegment:   jog + 1,
			changedSegment: jog + 2,
			mergedSegment:  [2]int{jog - 1, jog + 2},
			delta:          Point{X: -delta.X, Y: -delta.Y},
		})
	}
	return shifts
}

func shiftedRoute(route []Point, shift jogShift) []Point {
	shifted := make([]Point, len(route))
	for i, point := range route {
		if i == shift.points[0] || i == shift.points[1] {
			shifted[i] = Point{X: point.X + shift.delta.X, Y: point.Y + shift.delta.Y}
		} else {
			shifted[i] = point
		}
	}
	return shifted
}

func legalShift(index int, route, candidate []Point, shift jogShift, sc *scene) bool {
	bounds := sc.bounds
	if bounds != nil && (!inside(candidate[shift.points[0]], *bounds) || !inside(candidate[shift.points[1]], *bounds)) {
		return false
	}
	for _, segmentIndex := range []int{shift.movedSegment, shift.changedSegment} {
		before, okBefore := orthogonalSegment(route[segmentIndex], route[segmentIndex+1])
		after, okAfter := orthogonalSegment(candidate[segmentIndex], candidate[segmentIndex+1])
		if !okBefore || !okAfter || before.axis != after.axis || before.direction != after.direction {
			return false
		}
		if (segmentIndex == 0 || segmentIndex == len(route)-2) && after.length < math.Min(before.length, Clearance) {
			return false
		}
		if crossesObstacle(index, candidate[segmentIndex], candidate[segmentIndex+1], sc) {
			return false
		}
		if tooCloseToOthers(index, candidate[segmentIndex], candidate[segmentIndex+1], sc.routes) {
			return false
		}
	}
	mergeStart, mergeEnd := shift.mergedSegment[0], shift.mergedSegment[1]
	merged, ok := orthogonalSegment(candidate[mergeStart], candidate[mergeEnd])
	if !ok {
		return false
	}
	if bounds != nil && (!inside(candidate[mergeStart], *bounds) || !inside(candidate[mergeEnd], *bounds)) {
		return false
	}
	if crossesObstacle(index, candidate[mergeStart], candidate[mergeEnd], sc) {
		return false
	}
	if (mergeStart == 0 && merged.length < math.Min(segmentLength(route[0], route[1]), Clearance)) ||
		(mergeEnd == len(route)-1 && merged.length < math.Min(segmentLength(route[len(route)-2], route[len(route)-1]), Clearance)) {
		return false
	}
	if tooCloseToOthers(index, candidate[mergeStart], candidate[mergeEnd], sc.routes) {
		return false
	}
	return !addsCrossings(index, route, candidate, sc.routes)
}

func tooCloseToOthers(index int, a, b Point, routes []Route) bool {
	for _, other := range routes {
		if other.Index == index {
			continue
		}
		for i := 1; i < len(other.Points); i++ {
			if lanesTooClose(a, b, other.Points[i-1], other.Points[i]) {
				return true
			}
		}
	}
	return false
}

func crossesObstacle(route int, a, b Point, sc *scene) bool {
	routeEnds := sc.ends[route]
	for _, obstacle := range sc.obstacles {
		box := obstacle.Routing
		if obstacle.id == routeEnds[0] || obstacle.id == routeEnds[1] {
			box = obstacle.Raw
		}
		if crossesInterior(a, b, box) {
			return true
		}
	}
	return false
}

func addsCrossings(index int, route, candidate []Point, routes []Route) bool {
	for _, other := range routes {
		if other.Index != index && properCrossings(candidate, other.Points) > properCrossings(route, other.Points) {
			return true
		}
	}
	return false
}

func properCrossings(route, other []Point) int {
	crossings := 0
	for first := 1; first < len(route); first++ {
		for second := 1; second < len(other); second++ {
			if properlyCrosses(route[first-1], route[first], other[second-1], other[second]) {
				crossings++
			}
		}
	}
	return crossings
}

func properlyCrosses(a, b, c, d Point) bool {
	first, okFirst := orthogonalSegment(a, b)
	second, okSecond := orthogonalSegment(c, d)
	if !okFirst || !okSecond || first.axis == second.axis {
		return false
	}
	horizontalStart, horizontalEnd, verticalStart, verticalEnd := c, d, a, b
	if first.axis == horizontal {
		horizontalStart, horizontalEnd, verticalStart, verticalEnd = a, b, c, d
	}
	x := verticalStart.X
	y := horizontalStart.Y
	return x > math.Min(horizontalStart.X, horizontalEnd.X) &&
		x < math.Max(horizontalStart.X, horizontalEnd.X) &&
		y > math.Min(verticalStart.Y, verticalEnd.Y) &&
		y < math.Max(verticalStart.Y, verticalEnd.Y)
}

func orthogonalSegment(a, b Point) (segment, bool) {
	if a.X == b.X && a.Y != b.Y {
		return segment{axis: vertical, direction: sign(b.Y - a.Y), length: math.Abs(b.Y - a.Y)}, true
	}
	if a.Y == b.Y && a.X != b.X {
		return segment{axis: horizontal, direction: sign(b.X - a.X), length: math.Abs(b.X - a.X)}, true
	}
	return segment{}, false
}

func sign(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}

func segmentLength(a, b Point) float64 {
	return math.Abs(b.X-a.X) + math.Abs(b.Y-a.Y)
}

func inside(point Point, bounds Box) bool {
	return point.X >= bounds.X && point.Y >= bounds.Y &&
		point.X <= bounds.X+bounds.Width && point.Y <= bounds.Y+bounds.Height
}

func crossesInterior(a, b Point, box Box) bool {
	if a.Y == b.Y {
		return a.Y > box.Y && a.Y < box.Y+box.Height &&
			math.Max(math.Min(a.X, b.X), box.X) < math.Min(math.Max(a.X, b.X), box.X+box.Width)
	}
	return a.X > box.X && a.X < box.X+box.Width &&
		math.Max(math.Min(a.Y, b.Y), box.Y) < math.Min(math.Max(a.Y, b.Y), box.Y+box.Height)
}

func lanesTooClose(a, b, c, d Point) bool {
	first, okFirst := orthogonalSegment(a, b)
	second, okSecond := orthogonalSegment(c, d)
	if !okFirst || !okSecond || first.axis != second.axis {
		return false
	}
	if first.axis == horizontal {
		return math.Abs(a.Y-c.Y) < Nudging &&
			math.Min(math.Max(a.X, b.X), math.Max(c.X, d.X)) > math.Max(math.Min(a.X, b.X), math.Min(c.X, d.X))
	}
	return math.Abs(a.X-c.X) < Nudging &&
		math.Min(math.Max(a.Y, b.Y), math.Max(c.Y, d.Y)) > math.Max(math.Min(a.Y, b.Y), math.Min(c.Y, d.Y))
}

// CompactRoute removes zero-length and same-direction collinear points from an orthogonal route.
func CompactRoute(route []Point) []Point {
	compacted := make([]Point, 0, len(route))
	for _, point := range route {
		if len(compacted) > 0 && compacted[len(compacted)-1] == point {
			continue
		}
		for len(compacted) >= 2 {
			previous, okPrevious := orthogonalSegment(compacted[len(compacted)-2], compacted[len(compacted)-1])
			next, okNext := orthogonalSegment(compacted[len(compacted)-1], point)
			if !okPrevious || !okNext || previous.axis != next.axis || previous.direction != next.direction {
				break
			}
			compacted = compacted[:len(compacted)-1]
		}
		compacted = append(compacted, point)
	}
	return compacted
}
