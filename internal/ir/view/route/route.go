// Package route routes a rendering's edges orthogonally around its boxes with
// libavoid, the router the diagram panel bundles, run as WebAssembly in-process.
package route

import (
	"context"
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
const (
	clearance         = 16
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
	routes := make([][]Point, len(edges))
	if len(edges) == 0 {
		return routes, nil
	}
	e, err := sharedEngine()
	if err != nil {
		return nil, err
	}
	s, err := e.newSession(context.Background())
	if err != nil {
		return nil, err
	}
	defer s.close()

	byID := make(map[string]*Shape, len(shapes))
	for i := range shapes {
		byID[shapes[i].ID] = &shapes[i]
	}
	counts := map[string]int{}
	hasPortEnds := false
	for _, edge := range edges {
		from, to := byID[edge.From], byID[edge.To]
		if from == nil || to == nil {
			continue
		}
		counts[edge.From]++
		counts[edge.To]++
		if from.port(edge.FromPort) != nil || to.port(edge.ToPort) != nil {
			hasPortEnds = true
		}
	}

	router := s.bind("Router", "Router", s.enum("RouterFlag_OrthogonalRouting"))
	s.method("Router", router, "setRoutingParameter", s.enum("RoutingParameter_idealNudgingDistance"), f64(nudging))
	s.method("Router", router, "setRoutingParameter", s.enum("RoutingParameter_segmentPenalty"), f64(segmentPenalty))
	s.method("Router", router, "setRoutingParameter", s.enum("RoutingParameter_shapeBufferDistance"), f64(clearance))
	s.method("Router", router, "setRoutingOption", s.enum("RoutingOption_nudgeSharedPathsWithCommonEndPoint"), 1)
	s.method("Router", router, "setRoutingOption", s.enum("RoutingOption_performUnifyingNudgingPreprocessingStep"), 1)
	// Shape-connected nudging moves crowded port endpoints away from their faces.
	s.method("Router", router, "setRoutingOption", s.enum("RoutingOption_nudgeOrthogonalSegmentsConnectedToShapes"), boolArg(!hasPortEnds))

	refs := map[string]shapeRef{}
	nextClass := uint64(pinClass + 1)
	for _, shape := range shapes {
		refs[shape.ID] = s.addShape(router, shape, counts[shape.ID] <= exclusivePinLimit, &nextClass)
	}
	if bounds != nil {
		s.addFrames(router, *bounds)
	}

	type connector struct {
		index int
		ref   uint64
	}
	var connectors []connector
	orthogonal := s.enum("ConnType_ConnType_Orthogonal")
	for i, edge := range edges {
		from, to := refs[edge.From], refs[edge.To]
		if from.shape == 0 || to.shape == 0 {
			continue
		}
		source := s.bind("ConnEnd", "ConnEnd", from.shape, from.class(edge.FromPort))
		target := s.bind("ConnEnd", "ConnEnd", to.shape, to.class(edge.ToPort))
		ref := s.bind("ConnRef", "ConnRef", router, source, target)
		s.destroy("ConnEnd", source)
		s.destroy("ConnEnd", target)
		s.method("ConnRef", ref, "setRoutingType", orthogonal)
		connectors = append(connectors, connector{index: i, ref: ref})
	}
	s.method("Router", router, "processTransaction")

	for _, c := range connectors {
		edge := edges[c.index]
		poly := s.method("ConnRef", c.ref, "displayRoute")
		n := int(s.method("Polygon", poly, "size"))
		points := make([]Point, 0, n)
		for i := 0; i < n; i++ {
			p := s.method("Polygon", poly, "get_ps", uint64(i))
			points = append(points, Point{
				X: decodeF64(s.method("Point", p, "get_x")),
				Y: decodeF64(s.method("Point", p, "get_y")),
			})
		}
		if s.err != nil {
			break
		}
		from, to := byID[edge.From], byID[edge.To]
		route := cleanRoute(points)
		if route == nil || !isOrthogonal(route) {
			continue
		}
		if port := from.port(edge.FromPort); port != nil {
			alignPortFace(route, 0, 1, from.Box, *port)
		}
		if port := to.port(edge.ToPort); port != nil {
			alignPortFace(route, len(route)-1, len(route)-2, to.Box, *port)
		}
		routes[c.index] = route
	}
	s.destroy("Router", router)
	if s.err != nil {
		return nil, s.err
	}
	return routes, nil
}

// shapeRef is a shape in the router, with the pin class of each named port.
type shapeRef struct {
	shape   uint64
	classes map[string]uint64
}

// class is the pin class an edge end attaches at: the named port's, else the generic one.
func (r shapeRef) class(port string) uint64 {
	if c, ok := r.classes[port]; ok && port != "" {
		return c
	}
	return pinClass
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

// addShape puts a shape in the router as an obstacle with its pins; the
// generic pins are exclusive when the shape has few edges, a named port's never.
func (s *session) addShape(router uint64, shape Shape, exclusive bool, nextClass *uint64) shapeRef {
	box := shape.routingBox()
	ref := shapeRef{shape: s.rectangle(router, box), classes: map[string]uint64{}}
	directions := map[Side]uint64{
		North: s.enum("ConnDirFlag_ConnDirUp"),
		East:  s.enum("ConnDirFlag_ConnDirRight"),
		South: s.enum("ConnDirFlag_ConnDirDown"),
		West:  s.enum("ConnDirFlag_ConnDirLeft"),
	}
	for _, p := range shape.pins() {
		class := uint64(pinClass)
		if p.id != "" {
			class = *nextClass
			*nextClass++
			ref.classes[p.id] = class
		}
		pinRef := s.bind("ShapeConnectionPin", "ShapeConnectionPin", ref.shape, class, f64(p.x), f64(p.y), 1, f64(0), directions[p.side])
		s.method("ShapeConnectionPin", pinRef, "setExclusive", boolArg(p.id == "" && exclusive))
	}
	return ref
}

// rectangle adds a box to the router as a shape and returns it.
func (s *session) rectangle(router uint64, box Box) uint64 {
	center := s.bind("Point", "Point", f64(box.X+box.Width/2), f64(box.Y+box.Height/2))
	rect := s.bind("Rectangle", "Rectangle", center, f64(box.Width), f64(box.Height))
	shape := s.bind("ShapeRef", "ShapeRef", router, rect)
	s.destroy("Rectangle", rect)
	s.destroy("Point", center)
	return shape
}

// addFrames fences the routing in with four thin obstacles a clearance outside bounds.
func (s *session) addFrames(router uint64, bounds Box) {
	outer := Box{
		X:      bounds.X - clearance - frameThickness,
		Y:      bounds.Y - clearance - frameThickness,
		Width:  bounds.Width + 2*(clearance+frameThickness),
		Height: bounds.Height + 2*(clearance+frameThickness),
	}
	for _, frame := range []Box{
		{X: outer.X, Y: outer.Y, Width: outer.Width, Height: frameThickness},
		{X: outer.X, Y: outer.Y + outer.Height - frameThickness, Width: outer.Width, Height: frameThickness},
		{X: outer.X, Y: outer.Y, Width: frameThickness, Height: outer.Height},
		{X: outer.X + outer.Width - frameThickness, Y: outer.Y, Width: frameThickness, Height: outer.Height},
	} {
		s.rectangle(router, frame)
	}
}

// cleanRoute drops repeated points and mid-segment bends; fewer than two
// points left is no route.
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

func decodeF64(v uint64) float64 { return math.Float64frombits(v) }
