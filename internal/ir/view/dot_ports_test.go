package view

import "testing"

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
