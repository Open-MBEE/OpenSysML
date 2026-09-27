package view

import (
	"strings"
	"testing"
)

// A pin a route meets sits outside its action, touching the border on one side:
// the side the route's end lies off, the farther one past a corner, and the
// nearest one for an end on or inside the box.
func TestPinBoxTouchesOneSideOutsideTheBox(t *testing.T) {
	box := nodeBox{low: Point{X: 0, Y: 0}, high: Point{X: 100, Y: 50}}
	for name, tc := range map[string]struct {
		end    Point
		centre Point
	}{
		"below":                {Point{X: 40, Y: 60}, Point{X: 40, Y: 56}},
		"right":                {Point{X: 130, Y: 20}, Point{X: 106, Y: 20}},
		"past corner, right":   {Point{X: 120, Y: 60}, Point{X: 106, Y: 50}},
		"past corner, below":   {Point{X: 105, Y: 80}, Point{X: 100, Y: 56}},
		"on the border":        {Point{X: 100, Y: 20}, Point{X: 106, Y: 20}},
		"inside, near the top": {Point{X: 50, Y: 4}, Point{X: 50, Y: -6}},
	} {
		pin := pinBox(box, tc.end)
		if got := pin.centre(); got != tc.centre {
			t.Errorf("%s: pin centre = %v, want %v", name, got, tc.centre)
		}
		inside := pin.low.X < box.high.X && pin.high.X > box.low.X && pin.low.Y < box.high.Y && pin.high.Y > box.low.Y
		if inside {
			t.Errorf("%s: pin %v..%v overlaps the box", name, pin.low, pin.high)
		}
	}
}

// A flow into a pin of an action the drawing leaves unplaced is left undrawn
// with the action, and no edge is written to the pin or to a cell of the
// undeclared node, whether the action would be boxed or, having members,
// drawn as a cluster.
func TestPortEndOfOmittedOwnerIsNoEndpoint(t *testing.T) {
	for name, children := range map[string][]*Node{
		"plain":   nil,
		"cluster": {{ID: "inner", Kind: "action", Name: "inner"}},
	} {
		rendering := &Rendering{
			View: "V", Kind: KindAction,
			Roots: []*Node{
				{ID: "A", Kind: "action", Name: "a", Geometry: &Geometry{X: 10, Y: 10, Width: 100, Height: 40, HasSize: true},
					Ports: []Port{{ID: "A.0", Name: "result", Direction: PortOut}}},
				{ID: "B", Kind: "action", Name: "b", Children: children, Ports: []Port{{ID: "B.0", Name: "input", Direction: PortIn}}},
				{ID: "C", Kind: "action", Name: "c", Geometry: &Geometry{X: 10, Y: 100, Width: 100, Height: 40, HasSize: true}},
			},
			Edges: []Edge{
				{From: "A", To: "B", FromPort: "A.0", ToPort: "B.0", Kind: EdgeFlow},
				{From: "A", To: "C", Kind: EdgeSuccession},
			},
		}
		w := newDOTWriter(rendering, Options{})
		if end := w.portEnd("B", "B.0"); end != "" {
			t.Errorf("%s: port end at the omitted owner is %q, want none", name, end)
		}
		if end := w.portEnd("A", "A.0"); end != `"A.0"` {
			t.Errorf("%s: port end at the drawn owner is %q, want the pin", name, end)
		}
		source, err := rendering.DOTWith(Options{})
		if err != nil {
			t.Fatalf("%s: DOTWith: %v", name, err)
		}
		checkDOTSyntax(t, source)
		if strings.Contains(source, `"B`) || strings.Count(source, " -> ") != 1 {
			t.Errorf("%s: an edge or node is written at the omitted action:\n%s", name, source)
		}
		if !strings.Contains(source, "without a position, left undrawn, and 1 edge(s) at them\n") {
			t.Errorf("%s: the undrawn flow is not noticed:\n%s", name, source)
		}
	}
}
