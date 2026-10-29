package route

import (
	"context"
	"fmt"
	"math"
)

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

// PortExitReach is how far a connector leaves a port face when sharing connectors fan out from it.
func PortExitReach(sharing int) float64 {
	return Clearance + Nudging*math.Max(0, float64(sharing-1))
}

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

// Edge is a connector to route between two shapes, optionally at named ports.
type Edge struct {
	Index    int
	From, To string
	FromPort string
	ToPort   string
}

// Route is the orthogonal polyline found for the edge at Index.
type Route struct {
	Index  int
	Points []Point
}

// Compute routes the edges around the shapes with the process-wide router;
// routes stay inside bounds when it is given.
func Compute(shapes []Shape, edges []Edge, bounds *Box) ([]Route, error) {
	router, err := Default()
	if err != nil {
		return nil, err
	}
	return router.Route(shapes, edges, bounds)
}

// Route routes the edges around the shapes. Edges whose ends are not both among
// the shapes, and edges libavoid finds no orthogonal path for, have no route.
func (r *Router) Route(shapes []Shape, edges []Edge, bounds *Box) ([]Route, error) {
	byID := make(map[string]*Shape, len(shapes))
	for i := range shapes {
		if _, dup := byID[shapes[i].ID]; dup {
			return nil, fmt.Errorf("route: duplicate shape %q", shapes[i].ID)
		}
		byID[shapes[i].ID] = &shapes[i]
	}
	in, err := r.instantiate(context.Background())
	if err != nil {
		return nil, err
	}
	defer in.close()
	a := &avoid{in: in}
	a.loadEnums()

	router := in.call("emscripten_bind_Router_Router_1", i32(a.orthogonalRouting))
	in.call("emscripten_bind_Router_setRoutingParameter_2", router, i32(a.idealNudgingDistance), f64(Nudging))
	in.call("emscripten_bind_Router_setRoutingParameter_2", router, i32(a.segmentPenalty), f64(segmentPenalty))
	in.call("emscripten_bind_Router_setRoutingOption_2", router, i32(a.nudgeSharedPathsWithCommonEndPoint), boolean(true))
	in.call("emscripten_bind_Router_setRoutingOption_2", router, i32(a.performUnifyingNudgingPreprocessingStep), boolean(true))

	var usable []Edge
	for _, edge := range edges {
		if byID[edge.From] != nil && byID[edge.To] != nil {
			usable = append(usable, edge)
		}
	}
	hasPortEnds := false
	for _, edge := range usable {
		if byID[edge.From].port(edge.FromPort) != nil || byID[edge.To].port(edge.ToPort) != nil {
			hasPortEnds = true
			break
		}
	}
	in.call("emscripten_bind_Router_setRoutingParameter_2", router, i32(a.shapeBufferDistance), f64(Clearance))
	// Shape-connected nudging moves crowded port endpoints away from their faces.
	in.call("emscripten_bind_Router_setRoutingOption_2", router, i32(a.nudgeOrthogonalSegmentsConnectedToShapes), boolean(!hasPortEnds))

	counts := make(map[string]int, len(shapes))
	for _, edge := range usable {
		counts[edge.From]++
		counts[edge.To]++
	}
	refs := make(map[string]*shapePins, len(shapes))
	nextPortClass := uint32(pinClass + 1)
	for i := range shapes {
		shape := &shapes[i]
		refs[shape.ID] = a.addShape(router, shape, counts[shape.ID] <= ExclusivePinLimit, &nextPortClass)
	}
	if bounds != nil {
		a.addBoundsFrames(router, *bounds)
	}

	type connector struct {
		ref  uint64
		edge Edge
	}
	connectors := make([]connector, 0, len(usable))
	for _, edge := range usable {
		source := refs[edge.From]
		target := refs[edge.To]
		sourceEnd := in.call("emscripten_bind_ConnEnd_ConnEnd_2", source.shape, i32(source.class(edge.FromPort)))
		targetEnd := in.call("emscripten_bind_ConnEnd_ConnEnd_2", target.shape, i32(target.class(edge.ToPort)))
		ref := in.call("emscripten_bind_ConnRef_ConnRef_3", router, sourceEnd, targetEnd)
		in.call("emscripten_bind_ConnEnd___destroy___0", sourceEnd)
		in.call("emscripten_bind_ConnEnd___destroy___0", targetEnd)
		in.call("emscripten_bind_ConnRef_setRoutingType_1", ref, i32(a.connTypeOrthogonal))
		connectors = append(connectors, connector{ref: ref, edge: edge})
	}

	in.call("emscripten_bind_Router_processTransaction_0", router)

	routes := make([]Route, 0, len(connectors))
	for _, c := range connectors {
		// The display route's points are borrowed from the module's heap, never destroyed.
		polygon := in.call("emscripten_bind_ConnRef_displayRoute_0", c.ref)
		size := int(in.call("emscripten_bind_Polygon_size_0", polygon))
		points := make([]Point, 0, size)
		for i := 0; i < size; i++ {
			point := in.call("emscripten_bind_Polygon_get_ps_1", polygon, i32(uint32(i)))
			points = append(points, Point{
				X: in.callF64("emscripten_bind_Point_get_x_0", point),
				Y: in.callF64("emscripten_bind_Point_get_y_0", point),
			})
		}
		if in.err != nil {
			break
		}
		edge := c.edge
		source := byID[edge.From]
		target := byID[edge.To]
		hasPort := refs[edge.From].has(edge.FromPort) || refs[edge.To].has(edge.ToPort)
		// With no path between the ends libavoid draws a straight line between them; that is no route.
		route := cleanRoute(points, hasPort)
		if route == nil || !orthogonal(route) {
			continue
		}
		if port := source.port(edge.FromPort); port != nil {
			alignPortFace(route, 0, 1, source.Box, *port)
		}
		if port := target.port(edge.ToPort); port != nil {
			alignPortFace(route, len(route)-1, len(route)-2, target.Box, *port)
		}
		routes = append(routes, Route{Index: edge.Index, Points: route})
	}
	in.call("emscripten_bind_Router___destroy___0", router)
	if in.err != nil {
		return nil, &ModuleError{Source: r.source, Err: in.err}
	}

	obstacles := make([]obstacle, 0, len(shapes))
	for i := range shapes {
		obstacles = append(obstacles, obstacle{id: shapes[i].ID, Obstacle: RoutingObstacle(shapes[i])})
	}
	ends := make(map[int][2]string, len(usable))
	for _, edge := range usable {
		ends[edge.Index] = [2]string{edge.From, edge.To}
	}
	return StraightenJogs(routes, obstacles, ends, bounds), nil
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

// shapePins is a routed shape with the pin class of each of its named ports.
type shapePins struct {
	shape uint64
	ports map[string]uint32
}

func (p *shapePins) class(port string) uint32 {
	if class, ok := p.ports[port]; ok && port != "" {
		return class
	}
	return pinClass
}

func (p *shapePins) has(port string) bool {
	_, ok := p.ports[port]
	return ok && port != ""
}

// avoid binds the libavoid enumeration values of one module instance.
type avoid struct {
	in                                       *instance
	orthogonalRouting, connTypeOrthogonal    uint32
	dirUp, dirDown, dirLeft, dirRight        uint32
	segmentPenalty, idealNudgingDistance     uint32
	shapeBufferDistance                      uint32
	nudgeOrthogonalSegmentsConnectedToShapes uint32
	performUnifyingNudgingPreprocessingStep  uint32
	nudgeSharedPathsWithCommonEndPoint       uint32
}

func (a *avoid) loadEnums() {
	enum := func(name string) uint32 { return uint32(a.in.call("emscripten_enum_Avoid_" + name)) }
	a.orthogonalRouting = enum("RouterFlag_OrthogonalRouting")
	a.connTypeOrthogonal = enum("ConnType_ConnType_Orthogonal")
	a.dirUp = enum("ConnDirFlag_ConnDirUp")
	a.dirDown = enum("ConnDirFlag_ConnDirDown")
	a.dirLeft = enum("ConnDirFlag_ConnDirLeft")
	a.dirRight = enum("ConnDirFlag_ConnDirRight")
	a.segmentPenalty = enum("RoutingParameter_segmentPenalty")
	a.idealNudgingDistance = enum("RoutingParameter_idealNudgingDistance")
	a.shapeBufferDistance = enum("RoutingParameter_shapeBufferDistance")
	a.nudgeOrthogonalSegmentsConnectedToShapes = enum("RoutingOption_nudgeOrthogonalSegmentsConnectedToShapes")
	a.performUnifyingNudgingPreprocessingStep = enum("RoutingOption_performUnifyingNudgingPreprocessingStep")
	a.nudgeSharedPathsWithCommonEndPoint = enum("RoutingOption_nudgeSharedPathsWithCommonEndPoint")
}

func (a *avoid) direction(side Side) uint32 {
	switch side {
	case North:
		return a.dirUp
	case East:
		return a.dirRight
	case South:
		return a.dirDown
	default:
		return a.dirLeft
	}
}

// shapeRef adds a rectangular obstacle to the router and returns its ShapeRef.
func (a *avoid) shapeRef(router uint64, box Box) uint64 {
	in := a.in
	center := in.call("emscripten_bind_Point_Point_2", f64(box.X+box.Width/2), f64(box.Y+box.Height/2))
	rectangle := in.call("emscripten_bind_Rectangle_Rectangle_3", center, f64(box.Width), f64(box.Height))
	shape := in.call("emscripten_bind_ShapeRef_ShapeRef_2", router, rectangle)
	in.call("emscripten_bind_Rectangle___destroy___0", rectangle)
	in.call("emscripten_bind_Point___destroy___0", center)
	return shape
}

// addBoundsFrames walls the canvas in with four obstacles so routes stay inside bounds.
func (a *avoid) addBoundsFrames(router uint64, bounds Box) {
	thickness := math.Max(math.Max(bounds.Width, bounds.Height), Clearance)
	horizontalWidth := bounds.Width + 2*thickness
	verticalHeight := bounds.Height + 2*thickness
	frames := []Box{
		{X: bounds.X - thickness, Y: bounds.Y - Clearance - thickness, Width: horizontalWidth, Height: thickness},
		{X: bounds.X - thickness, Y: bounds.Y + bounds.Height + Clearance, Width: horizontalWidth, Height: thickness},
		{X: bounds.X - Clearance - thickness, Y: bounds.Y - thickness, Width: thickness, Height: verticalHeight},
		{X: bounds.X + bounds.Width + Clearance, Y: bounds.Y - thickness, Width: thickness, Height: verticalHeight},
	}
	for _, frame := range frames {
		a.shapeRef(router, frame)
	}
}

// addShape is a box as a routing obstacle, with twelve proportional pins spread
// over its sides for connectors to attach at, plus one pin per named port.
func (a *avoid) addShape(router uint64, shape *Shape, exclusive bool, nextPortClass *uint32) *shapePins {
	in := a.in
	ref := a.shapeRef(router, shapeRoutingBox(shape))
	pins := &shapePins{shape: ref, ports: map[string]uint32{}}
	for _, position := range RoutingPinPositions(shape) {
		class := uint32(pinClass)
		if position.Named {
			class = *nextPortClass
			*nextPortClass++
		}
		pin := in.call("emscripten_bind_ShapeConnectionPin_ShapeConnectionPin_7",
			ref, i32(class), f64(position.X), f64(position.Y), boolean(true), f64(0), i32(a.direction(position.Side)))
		in.call("emscripten_bind_ShapeConnectionPin_setExclusive_1", pin, boolean(!position.Named && exclusive))
		if position.Named {
			pins.ports[position.ID] = class
		}
	}
	return pins
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
