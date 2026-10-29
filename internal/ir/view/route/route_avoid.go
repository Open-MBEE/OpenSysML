//go:build !js && !wasip1

package route

import (
	"context"
	"fmt"
	"math"
)

func routeAll(shapes []Shape, edges []Edge, bounds *Box) ([][]Point, error) {
	routes := make([][]Point, len(edges))
	if len(edges) == 0 {
		return routes, nil
	}
	e, err := sharedEngine()
	if err != nil {
		return nil, err
	}
	s, err := e.acquire(context.Background())
	if err != nil {
		return nil, err
	}
	defer e.release(s)

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
		size := s.method("Polygon", poly, "size")
		if size > math.MaxInt32 {
			return nil, avoidError{fmt.Sprintf("route of %d points", size)}
		}
		n := int(size)
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
func decodeF64(v uint64) float64 { return math.Float64frombits(v) }
