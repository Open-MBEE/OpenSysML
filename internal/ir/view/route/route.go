// Package route routes a rendering's edges orthogonally around its boxes with
// libavoid, the router the diagram panel bundles, run as WebAssembly in-process,
// and straightens the result as the panel does, so both draw the same routes.
package route

import (
	"errors"
	"math"
)

// Routes is the orthogonal route of each edge the router found a path for,
// from its source to its target with every bend between, under the edge's
// Index; an edge whose ends are not among the shapes has none. Routes stay
// inside bounds when given. The js and wasip1 builds return ErrUnavailable.
func Routes(shapes []Shape, edges []Edge, bounds *Box) ([]Route, error) {
	if len(edges) == 0 {
		return nil, nil
	}
	return routeAll(shapes, edges, bounds)
}

// ErrUnavailable is what Routes returns in a build without the router: the
// js and wasip1 builds, which leave routing to the drawing program.
var ErrUnavailable = errors.New("libavoid: no router in this build")

// The panel's routing constants: a GAP between slots, PORT_SIZE for a port square.
const (
	Gap               = 32.0
	PortSize          = 10.0
	Clearance         = Gap / 2
	Nudging           = 8.0
	MinJog            = 16.0
	ExclusivePinLimit = 12
	segmentPenalty    = 50.0
	pinClass          = 1
)

// Point is a position on the canvas.
type Point struct {
	X, Y float64
}

// Box is an axis-aligned rectangle given by its top-left corner and size.
type Box struct {
	X, Y, Width, Height float64
}

// Side is a border of a box.
type Side string

// The four sides of a box.
const (
	North Side = "north"
	East  Side = "east"
	South Side = "south"
	West  Side = "west"
)

func (s Side) vertical() bool { return s == North || s == South }

// Port is a connection point on a shape's border at Offset (0..1) along Side;
// a Flush port sits on the border itself with no square straddling it.
type Port struct {
	ID     string
	Side   Side
	Offset float64
	Flush  bool
}

// Shape is an obstacle connectors route around and attach to.
type Shape struct {
	ID    string
	Box   Box
	Ports []Port
}

// Route is the orthogonal polyline found for the edge at Index.
type Route struct {
	Index  int
	Points []Point
}

// Edge is a connector to route between two shapes, optionally at named ports.
type Edge struct {
	Index    int
	From, To string
	FromPort string
	ToPort   string
}

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

// PortCenter is the point on the border where a port sits.
func PortCenter(box Box, port Port) Point {
	if port.Side.vertical() {
		y := box.Y
		if port.Side == South {
			y = box.Y + box.Height
		}
		return Point{X: box.X + box.Width*port.Offset, Y: y}
	}
	x := box.X
	if port.Side == East {
		x = box.X + box.Width
	}
	return Point{X: x, Y: box.Y + box.Height*port.Offset}
}

// PortFace is the outer face of the port square straddling the border.
func PortFace(box Box, port Port) Point {
	center := PortCenter(box, port)
	switch port.Side {
	case North:
		return Point{X: center.X, Y: center.Y - PortSize/2}
	case East:
		return Point{X: center.X + PortSize/2, Y: center.Y}
	case South:
		return Point{X: center.X, Y: center.Y + PortSize/2}
	default:
		return Point{X: center.X - PortSize/2, Y: center.Y}
	}
}

// portPoint is where a connector meets a port: the square's outer face, or the border itself at a flush pin.
func portPoint(box Box, port Port) Point {
	if port.Flush {
		return PortCenter(box, port)
	}
	return PortFace(box, port)
}

// squaresOn reports a side with a port square on it, which the routing box grows over.
func squaresOn(shape *Shape, side Side) bool {
	for _, port := range shape.Ports {
		if port.Side == side && !port.Flush {
			return true
		}
	}
	return false
}

func shapeRoutingBox(shape *Shape) Box {
	growth := func(side Side) float64 {
		if squaresOn(shape, side) {
			return PortSize / 2
		}
		return 0
	}
	west, east, north, south := growth(West), growth(East), growth(North), growth(South)
	box := shape.Box
	return Box{
		X:      box.X - west,
		Y:      box.Y - north,
		Width:  box.Width + west + east,
		Height: box.Height + north + south,
	}
}

// Obstacle holds a shape's routing-clearance bounds and its raw interior.
type Obstacle struct {
	Routing Box
	Raw     Box
}

// RoutingObstacle builds the routing geometry shared by libavoid and route simplification.
func RoutingObstacle(shape Shape) Obstacle {
	box := shapeRoutingBox(&shape)
	return Obstacle{
		Routing: Box{
			X:      box.X - Clearance,
			Y:      box.Y - Clearance,
			Width:  box.Width + 2*Clearance,
			Height: box.Height + 2*Clearance,
		},
		Raw: shape.Box,
	}
}

// PinPosition is a libavoid pin as proportions (0..1) of the shape's routing box.
type PinPosition struct {
	ID    string
	Named bool
	Side  Side
	X, Y  float64
}

// RoutingPinPositions computes bounded libavoid portions for a shape's generic and named pins.
func RoutingPinPositions(shape *Shape) []PinPosition {
	box := shape.Box
	routingBox := shapeRoutingBox(shape)
	portion := func(value float64) float64 { return math.Max(0, math.Min(1, value)) }
	offsets := []float64{0.25, 0.5, 0.75}
	var pins []PinPosition
	for _, side := range []Side{North, East, South, West} {
		for _, offset := range offsets {
			var pin PinPosition
			pin.Side = side
			switch side {
			case North:
				pin.X = portion((box.X + box.Width*offset - routingBox.X) / routingBox.Width)
				if squaresOn(shape, side) {
					pin.Y = portion((box.Y - routingBox.Y) / routingBox.Height)
				}
			case East:
				pin.X = 1
				if squaresOn(shape, side) {
					pin.X = portion((box.X + box.Width - routingBox.X) / routingBox.Width)
				}
				pin.Y = portion((box.Y + box.Height*offset - routingBox.Y) / routingBox.Height)
			case South:
				pin.X = portion((box.X + box.Width*offset - routingBox.X) / routingBox.Width)
				pin.Y = 1
				if squaresOn(shape, side) {
					pin.Y = portion((box.Y + box.Height - routingBox.Y) / routingBox.Height)
				}
			case West:
				if squaresOn(shape, side) {
					pin.X = portion((box.X - routingBox.X) / routingBox.Width)
				}
				pin.Y = portion((box.Y + box.Height*offset - routingBox.Y) / routingBox.Height)
			}
			pins = append(pins, pin)
		}
	}
	for _, port := range shape.Ports {
		point := portPoint(box, port)
		pin := PinPosition{ID: port.ID, Named: true, Side: port.Side}
		switch port.Side {
		case North:
			pin.X = portion((point.X - routingBox.X) / routingBox.Width)
		case East:
			pin.X = 1
			pin.Y = portion((point.Y - routingBox.Y) / routingBox.Height)
		case South:
			pin.X = portion((point.X - routingBox.X) / routingBox.Width)
			pin.Y = 1
		case West:
			pin.Y = portion((point.Y - routingBox.Y) / routingBox.Height)
		}
		pins = append(pins, pin)
	}
	return pins
}

// snap rounds to the pixel grid the way JavaScript's Math.round does (halves go up).
func snap(value float64) float64 {
	return math.Floor(value + 0.5)
}

// cleanRoute keeps port-face coordinates exact and drops doubled points and mid-segment bends.
func cleanRoute(points []Point, preservePortFaces bool) []Point {
	normalized := make([]Point, 0, len(points))
	for _, point := range points {
		next := point
		if !preservePortFaces {
			next = Point{X: snap(point.X), Y: snap(point.Y)}
		}
		if len(normalized) == 0 || normalized[len(normalized)-1] != next {
			normalized = append(normalized, next)
		}
	}
	clean := make([]Point, 0, len(normalized))
	for _, point := range normalized {
		for len(clean) >= 2 && collinear(clean[len(clean)-2], clean[len(clean)-1], point) {
			clean = clean[:len(clean)-1]
		}
		if len(clean) == 0 || clean[len(clean)-1] != point {
			clean = append(clean, point)
		}
	}
	if len(clean) < 2 {
		return nil
	}
	return clean
}

func alignPortFace(route []Point, pointIndex, adjacentIndex int, box Box, port Port) {
	face := portPoint(box, port)
	route[pointIndex] = face
	adjacent := route[adjacentIndex]
	vertical := port.Side.vertical()
	actual, expected := adjacent.Y, face.Y
	if vertical {
		actual, expected = adjacent.X, face.X
	}
	tolerance := epsilon * math.Max(1, math.Max(math.Abs(actual), math.Abs(expected))) * 4
	if math.Abs(actual-expected) <= tolerance {
		if vertical {
			route[adjacentIndex].X = expected
		} else {
			route[adjacentIndex].Y = expected
		}
	}
}

// epsilon is JavaScript's Number.EPSILON.
const epsilon = 2.220446049250313e-16

func orthogonal(points []Point) bool {
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
