// Package route routes a rendering's edges orthogonally around its boxes with
// libavoid, the router the diagram panel bundles, run as WebAssembly in-process.
package route

import (
	"errors"
	"math"
)

// Point is a coordinate in the rendering's plane.
type Point struct{ X, Y float64 }

// Box is an axis-aligned rectangle by its top-left corner and size.
type Box struct{ X, Y, Width, Height float64 }

// Side is a border of a box.
type Side int

// The four sides, clockwise from the top.
const (
	North Side = iota
	East
	South
	West
)

// Port is a named attachment point on a shape's border: Offset is the
// fraction along the side, and Flush marks a point on the border itself rather
// than a port square straddling it.
type Port struct {
	ID     string
	Side   Side
	Offset float64
	Flush  bool
}

// Shape is a box edges route around and attach to.
type Shape struct {
	ID    string
	Box   Box
	Ports []Port
}

// Edge joins two shapes, at named ports when FromPort or ToPort names one the
// shape has, else at whichever of the shape's sides the router picks.
type Edge struct {
	From, To         string
	FromPort, ToPort string
}

// The panel's routing constants: the clearance kept from boxes, the distance
// parallel routes are nudged apart, the size of a port square, and the number
// of edges at a shape above which its generic pins are shared.
// Clearance is how far the router keeps a path from every shape; two shapes
// closer than twice it leave no channel between them.
const Clearance = 16

const (
	clearance         = Clearance
	nudging           = 8
	portSize          = 10
	exclusivePinLimit = 12
	segmentPenalty    = 50
	pinClass          = 1
	frameThickness    = 2
)

// Routes is the orthogonal route of each edge, the points from its source to
// its target with every bend between, nil for an edge the router found no
// path for or whose ends are not among the shapes. Routes stay inside bounds
// when given. Shapes and edges are taken in order, so the result is deterministic.
func Routes(shapes []Shape, edges []Edge, bounds *Box) ([][]Point, error) {
	return routeAll(shapes, edges, bounds)
}

// ErrUnavailable is what Routes returns in a build without the router: the
// js and wasip1 builds, which leave routing to the drawing program.
var ErrUnavailable = errors.New("libavoid: no router in this build")

func (s *Shape) port(id string) *Port {
	if id == "" {
		return nil
	}
	for i := range s.Ports {
		if s.Ports[i].ID == id {
			return &s.Ports[i]
		}
	}
	return nil
}

// squaresOn reports a side with a port square on it, which the routing box grows over.
func (s Shape) squaresOn(side Side) bool {
	for _, port := range s.Ports {
		if port.Side == side && !port.Flush {
			return true
		}
	}
	return false
}

// routingBox is the box the router keeps clear: the shape's, grown over the
// port squares straddling its sides.
func (s Shape) routingBox() Box {
	box := s.Box
	var west, east, north, south float64
	if s.squaresOn(West) {
		west = portSize / 2
	}
	if s.squaresOn(East) {
		east = portSize / 2
	}
	if s.squaresOn(North) {
		north = portSize / 2
	}
	if s.squaresOn(South) {
		south = portSize / 2
	}
	return Box{X: box.X - west, Y: box.Y - north, Width: box.Width + west + east, Height: box.Height + north + south}
}

// portCenter is the point on the border a port sits at.
func portCenter(box Box, port Port) Point {
	switch port.Side {
	case North:
		return Point{X: box.X + box.Width*port.Offset, Y: box.Y}
	case East:
		return Point{X: box.X + box.Width, Y: box.Y + box.Height*port.Offset}
	case South:
		return Point{X: box.X + box.Width*port.Offset, Y: box.Y + box.Height}
	}
	return Point{X: box.X, Y: box.Y + box.Height*port.Offset}
}

// portFace is the outer face of a port's square, where an edge meets it.
func portFace(box Box, port Port) Point {
	p := portCenter(box, port)
	switch port.Side {
	case North:
		p.Y -= portSize / 2
	case East:
		p.X += portSize / 2
	case South:
		p.Y += portSize / 2
	case West:
		p.X -= portSize / 2
	}
	return p
}

// portPoint is where an edge reaches a port: the border itself for a flush one.
func portPoint(box Box, port Port) Point {
	if port.Flush {
		return portCenter(box, port)
	}
	return portFace(box, port)
}

// pin is a connection pin on a shape, at proportional coordinates of its routing box.
type pin struct {
	id   string
	side Side
	x, y float64
}

// pins is a shape's pins: three generic ones spread along each side, then one
// per named port at its face.
func (s Shape) pins() []pin {
	box, routing := s.Box, s.routingBox()
	portion := func(v float64) float64 { return math.Max(0, math.Min(1, v)) }
	var pins []pin
	for _, offset := range []float64{0.25, 0.5, 0.75} {
		p := pin{side: North, x: portion((box.X + box.Width*offset - routing.X) / routing.Width)}
		if s.squaresOn(North) {
			p.y = portion((box.Y - routing.Y) / routing.Height)
		}
		pins = append(pins, p)
	}
	for _, offset := range []float64{0.25, 0.5, 0.75} {
		p := pin{side: East, x: 1, y: portion((box.Y + box.Height*offset - routing.Y) / routing.Height)}
		if s.squaresOn(East) {
			p.x = portion((box.X + box.Width - routing.X) / routing.Width)
		}
		pins = append(pins, p)
	}
	for _, offset := range []float64{0.25, 0.5, 0.75} {
		p := pin{side: South, x: portion((box.X + box.Width*offset - routing.X) / routing.Width), y: 1}
		if s.squaresOn(South) {
			p.y = portion((box.Y + box.Height - routing.Y) / routing.Height)
		}
		pins = append(pins, p)
	}
	for _, offset := range []float64{0.25, 0.5, 0.75} {
		p := pin{side: West, y: portion((box.Y + box.Height*offset - routing.Y) / routing.Height)}
		if s.squaresOn(West) {
			p.x = portion((box.X - routing.X) / routing.Width)
		}
		pins = append(pins, p)
	}
	for _, port := range s.Ports {
		at := portPoint(box, port)
		p := pin{id: port.ID, side: port.Side}
		switch port.Side {
		case North:
			p.x = portion((at.X - routing.X) / routing.Width)
		case East:
			p.x, p.y = 1, portion((at.Y-routing.Y)/routing.Height)
		case South:
			p.x, p.y = portion((at.X-routing.X)/routing.Width), 1
		case West:
			p.y = portion((at.Y - routing.Y) / routing.Height)
		}
		pins = append(pins, p)
	}
	return pins
}

func cleanRoute(points []Point) []Point {
	var clean []Point
	for _, p := range points {
		if n := len(clean); n > 0 && clean[n-1] == p {
			continue
		}
		for len(clean) >= 2 && collinear(clean[len(clean)-2], clean[len(clean)-1], p) {
			clean = clean[:len(clean)-1]
		}
		if n := len(clean); n > 0 && clean[n-1] == p {
			continue
		}
		clean = append(clean, p)
	}
	if len(clean) < 2 {
		return nil
	}
	return clean
}

// alignPortFace puts a route's end exactly on the port's face and squares the
// adjacent point to it when the router left it within rounding of that.
func alignPortFace(route []Point, end, adjacent int, box Box, port Port) {
	face := portPoint(box, port)
	route[end] = face
	vertical := port.Side == North || port.Side == South
	actual, expected := route[adjacent].Y, face.Y
	if vertical {
		actual, expected = route[adjacent].X, face.X
	}
	tolerance := 4 * math.SmallestNonzeroFloat64 * math.Max(1, math.Max(math.Abs(actual), math.Abs(expected)))
	tolerance = math.Max(tolerance, 1e-9)
	if math.Abs(actual-expected) <= tolerance {
		if vertical {
			route[adjacent].X = expected
		} else {
			route[adjacent].Y = expected
		}
	}
}

func isOrthogonal(points []Point) bool {
	for i := 1; i < len(points); i++ {
		if points[i].X != points[i-1].X && points[i].Y != points[i-1].Y {
			return false
		}
	}
	return true
}

func collinear(a, b, c Point) bool {
	return (a.X == b.X && b.X == c.X) || (a.Y == b.Y && b.Y == c.Y)
}
