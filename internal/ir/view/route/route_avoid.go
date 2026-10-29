//go:build !js && !wasip1

package route

import (
	"context"
	"fmt"
	"math"
)

// Route routes the edges around the shapes. Edges whose ends are not both among
// the shapes, and edges libavoid finds no orthogonal path for, have no route.
func routeAll(shapes []Shape, edges []Edge, bounds *Box) ([]Route, error) {
	byID := make(map[string]*Shape, len(shapes))
	for i := range shapes {
		if _, dup := byID[shapes[i].ID]; dup {
			return nil, fmt.Errorf("route: duplicate shape %q", shapes[i].ID)
		}
		byID[shapes[i].ID] = &shapes[i]
	}
	e, err := sharedEngine()
	if err != nil {
		return nil, err
	}
	in, err := e.acquire(context.Background())
	if err != nil {
		return nil, err
	}
	defer e.release(in)
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
		count := in.call("emscripten_bind_Polygon_size_0", polygon)
		if count > math.MaxInt32 {
			return nil, avoidError{fmt.Sprintf("route of %d points", count)}
		}
		size := int(count)
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
		return nil, in.err
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
	in                                       *session
	orthogonalRouting, connTypeOrthogonal    uint32
	dirUp, dirDown, dirLeft, dirRight        uint32
	segmentPenalty, idealNudgingDistance     uint32
	shapeBufferDistance                      uint32
	nudgeOrthogonalSegmentsConnectedToShapes uint32
	performUnifyingNudgingPreprocessingStep  uint32
	nudgeSharedPathsWithCommonEndPoint       uint32
}

func (a *avoid) loadEnums() {
	enum := func(name string) uint32 {
		v := a.in.call("emscripten_enum_Avoid_" + name)
		if v > math.MaxUint32 {
			a.in.err = avoidError{fmt.Sprintf("enumerator %s is %d", name, v)}
			return 0
		}
		return uint32(v)
	}
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
