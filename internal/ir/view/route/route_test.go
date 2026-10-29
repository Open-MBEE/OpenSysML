//go:build !js && !wasip1

package route

import (
	"reflect"
	"testing"
)

func routeOrFail(t *testing.T, shapes []Shape, edges []Edge, bounds *Box) [][]Point {
	t.Helper()
	routes, err := Routes(shapes, edges, bounds)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != len(edges) {
		t.Fatalf("%d routes for %d edges", len(routes), len(edges))
	}
	return routes
}

// An edge between two boxes with a third between them bends around it, every
// segment horizontal or vertical, from the source's border to the target's.
func TestRoutesDetourAroundAnObstacle(t *testing.T) {
	shapes := []Shape{
		{ID: "a", Box: Box{X: 40, Y: 70, Width: 120, Height: 60}},
		{ID: "b", Box: Box{X: 340, Y: 270, Width: 120, Height: 60}},
		{ID: "wall", Box: Box{X: 210, Y: 160, Width: 80, Height: 80}},
	}
	routes := routeOrFail(t, shapes, []Edge{{From: "a", To: "b"}}, nil)
	route := routes[0]
	if len(route) < 3 {
		t.Fatalf("route %v has no bend", route)
	}
	if !isOrthogonal(route) {
		t.Errorf("route %v is not orthogonal", route)
	}
	wall := shapes[2].Box
	for i := 1; i < len(route); i++ {
		if crossesInterior(route[i-1], route[i], wall) {
			t.Errorf("segment %v-%v crosses the obstacle", route[i-1], route[i])
		}
	}
	if !onBorder(route[0], shapes[0].Box) || !onBorder(route[len(route)-1], shapes[1].Box) {
		t.Errorf("route %v does not run border to border", route)
	}
}

// An edge naming ports leaves and arrives at their faces.
func TestRoutesAttachAtNamedPorts(t *testing.T) {
	shapes := []Shape{
		{ID: "a", Box: Box{X: 0, Y: 0, Width: 100, Height: 50}, Ports: []Port{{ID: "out", Side: East, Offset: 0.5}}},
		{ID: "b", Box: Box{X: 300, Y: 200, Width: 100, Height: 50}, Ports: []Port{{ID: "in", Side: West, Offset: 0.5}}},
	}
	routes := routeOrFail(t, shapes, []Edge{{From: "a", To: "b", FromPort: "out", ToPort: "in"}}, nil)
	route := routes[0]
	if want := (Point{X: 105, Y: 25}); route[0] != want {
		t.Errorf("route starts at %v, want the port face %v", route[0], want)
	}
	if want := (Point{X: 295, Y: 225}); route[len(route)-1] != want {
		t.Errorf("route ends at %v, want the port face %v", route[len(route)-1], want)
	}
}

// The same input routes the same way every time, and an edge whose end is not
// among the shapes gets no route.
func TestRoutesAreDeterministicAndSkipUnknownEnds(t *testing.T) {
	shapes := []Shape{
		{ID: "a", Box: Box{X: 0, Y: 0, Width: 80, Height: 40}},
		{ID: "b", Box: Box{X: 200, Y: 0, Width: 80, Height: 40}},
		{ID: "c", Box: Box{X: 100, Y: 150, Width: 80, Height: 40}},
	}
	edges := []Edge{{From: "a", To: "b"}, {From: "b", To: "c"}, {From: "c", To: "a"}, {From: "a", To: "ghost"}}
	first := routeOrFail(t, shapes, edges, nil)
	second := routeOrFail(t, shapes, edges, nil)
	if !reflect.DeepEqual(first, second) {
		t.Errorf("routes differ between runs:\n%v\n%v", first, second)
	}
	for i := 0; i < 3; i++ {
		if len(first[i]) < 2 {
			t.Errorf("edge %d has route %v", i, first[i])
		}
	}
	if first[3] != nil {
		t.Errorf("the edge to an unknown shape got route %v", first[3])
	}
}

// Routes keep inside the bounds they are given.
func TestRoutesStayInsideBounds(t *testing.T) {
	bounds := Box{X: 0, Y: 0, Width: 400, Height: 200}
	shapes := []Shape{
		{ID: "a", Box: Box{X: 20, Y: 80, Width: 80, Height: 40}},
		{ID: "b", Box: Box{X: 300, Y: 80, Width: 80, Height: 40}},
	}
	routes := routeOrFail(t, shapes, []Edge{{From: "a", To: "b"}}, &bounds)
	for _, p := range routes[0] {
		if p.X < bounds.X || p.Y < bounds.Y || p.X > bounds.X+bounds.Width || p.Y > bounds.Y+bounds.Height {
			t.Errorf("point %v lies outside %v", p, bounds)
		}
	}
}

func TestCleanRouteDropsDoubledAndCollinearPoints(t *testing.T) {
	got := cleanRoute([]Point{{0, 0}, {0, 0}, {10, 0}, {20, 0}, {20, 5}, {20, 10}})
	want := []Point{{0, 0}, {20, 0}, {20, 10}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("cleanRoute = %v, want %v", got, want)
	}
	if cleanRoute([]Point{{1, 1}, {1, 1}}) != nil {
		t.Error("a single point is a route")
	}
}

func onBorder(p Point, box Box) bool {
	const eps = 1e-6
	onX := p.X >= box.X-portSize && p.X <= box.X+box.Width+portSize
	onY := p.Y >= box.Y-portSize && p.Y <= box.Y+box.Height+portSize
	atLeft := abs(p.X-box.X) < eps || abs(p.X-(box.X+box.Width)) < eps
	atTop := abs(p.Y-box.Y) < eps || abs(p.Y-(box.Y+box.Height)) < eps
	return (atLeft && onY) || (atTop && onX)
}

func crossesInterior(a, b Point, box Box) bool {
	minX, maxX := min(a.X, b.X), max(a.X, b.X)
	minY, maxY := min(a.Y, b.Y), max(a.Y, b.Y)
	return maxX > box.X && minX < box.X+box.Width && maxY > box.Y && minY < box.Y+box.Height
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
