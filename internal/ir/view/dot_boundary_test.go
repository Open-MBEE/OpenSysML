package view

import (
	"maps"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

var (
	dotPinnedNode = regexp.MustCompile(`(?m)^\s*"(n[\d.]+)" \[(.*pos="[-0-9.]+,[-0-9.]+!".*)\]`)
	dotPinnedPos  = regexp.MustCompile(`pos="([-0-9.]+),([-0-9.]+)!"`)
	dotPinnedSize = regexp.MustCompile(`width=([0-9.]+), height=([0-9.]+)`)
	dotRoutedEdge = regexp.MustCompile(`(?m)^\s*"(n[\d.]+)" -> "(n[\d.]+)" \[.*pos="([^"]+)"`)
)

// pinBoxes renders the view and returns the DOT text's pinned boxes and route
// ends with the pin IDs of the named ports of node.
func pinBoxes(t *testing.T, view, node string, names ...string) (boxes map[string]nodeBox, ends map[string][]Point, pins []string) {
	t.Helper()
	return pinBoxesWith(t, Options{}, view, node, names...)
}

// pinBoxesWith is pinBoxes with the DOT text written under options.
func pinBoxesWith(t *testing.T, options Options, view, node string, names ...string) (boxes map[string]nodeBox, ends map[string][]Point, pins []string) {
	t.Helper()
	rendering := render(t, "boundary-parameters.sysml", view)
	dot, err := rendering.DOTWith(options)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		pins = append(pins, pinOf(t, rendering, node, name))
	}
	return pinnedBoxes(t, dot), routeEndsAt(t, dot), pins
}

// pinnedBoxes is the box of every node the DOT text pins, by ID, in the
// rendering's pixels (y down).
func pinnedBoxes(t *testing.T, dot string) map[string]nodeBox {
	t.Helper()
	boxes := map[string]nodeBox{}
	for _, m := range dotPinnedNode.FindAllStringSubmatch(dot, -1) {
		pos, size := dotPinnedPos.FindStringSubmatch(m[2]), dotPinnedSize.FindStringSubmatch(m[2])
		if pos == nil || size == nil {
			t.Fatalf("pinned node %s without pos or size: %s", m[1], m[2])
		}
		x, y := dotNumber(t, pos[1]), -dotNumber(t, pos[2])
		width, height := dotNumber(t, size[1])*72, dotNumber(t, size[2])*72
		boxes[m[1]] = nodeBox{low: Point{X: x - width/2, Y: y - height/2}, high: Point{X: x + width/2, Y: y + height/2}}
	}
	return boxes
}

// routeEndsAt is where the routed edges of the DOT text end at each head node,
// in the rendering's pixels.
func routeEndsAt(t *testing.T, dot string) map[string][]Point {
	t.Helper()
	ends := map[string][]Point{}
	for _, m := range dotRoutedEdge.FindAllStringSubmatch(dot, -1) {
		points := strings.Fields(m[3])
		xy := strings.Split(points[len(points)-1], ",")
		ends[m[2]] = append(ends[m[2]], Point{X: dotNumber(t, xy[0]), Y: -dotNumber(t, xy[1])})
	}
	return ends
}

func dotNumber(t *testing.T, s string) float64 {
	t.Helper()
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		t.Fatalf("number %q: %v", s, err)
	}
	return v
}

// onBorder reports whether p lies on the box's border, to half a pixel.
func (b nodeBox) onBorder(p Point) bool {
	const tolerance = 0.5
	within := p.X >= b.low.X-tolerance && p.X <= b.high.X+tolerance && p.Y >= b.low.Y-tolerance && p.Y <= b.high.Y+tolerance
	onEdge := math.Abs(p.X-b.low.X) <= tolerance || math.Abs(p.X-b.high.X) <= tolerance ||
		math.Abs(p.Y-b.low.Y) <= tolerance || math.Abs(p.Y-b.high.Y) <= tolerance
	return within && onEdge
}

// overlaps reports whether the two boxes share any area.
func (b nodeBox) overlaps(other nodeBox) bool {
	return b.low.X < other.high.X && other.low.X < b.high.X && b.low.Y < other.high.Y && other.low.Y < b.high.Y
}

// pinOf is the ID of the port of the node named node that is named name.
func pinOf(t *testing.T, rendering *Rendering, node, name string) string {
	t.Helper()
	for _, port := range findNode(t, rendering.Roots, node).Ports {
		if port.Name == name {
			return port.ID
		}
	}
	t.Fatalf("no port %q of %q", name, node)
	return ""
}

// routesEndOnPins checks that exactly one route ends at each pin, on its border.
func routesEndOnPins(t *testing.T, boxes map[string]nodeBox, ends map[string][]Point, pins ...string) {
	t.Helper()
	for _, pin := range pins {
		if routeEnds := ends[pin]; len(routeEnds) != 1 || !boxes[pin].onBorder(routeEnds[0]) {
			t.Errorf("%s: route ends at %v, off its pin %v", pin, routeEnds, boxes[pin])
		}
	}
}

// Two actor parameters whose routes end at one point of the left border are drawn apart along
// it, the one from above on top, each route ending on its own pin; a lone parameter stays put.
func TestDOTSpreadsBoundaryParametersAlongTheSide(t *testing.T) {
	rendering := render(t, "boundary-parameters.sysml", "BoundaryParameters::lab")
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatal(err)
	}
	boxes, ends := pinnedBoxes(t, dot), routeEndsAt(t, dot)
	useCase := findNode(t, rendering.Roots, "BoundaryParameters::'Configure Instrument'").Geometry
	operator := pinOf(t, rendering, "BoundaryParameters::'Configure Instrument'", "operator")
	administrator := pinOf(t, rendering, "BoundaryParameters::'Configure Instrument'", "administrator")
	op, admin := boxes[operator], boxes[administrator]
	if op.overlaps(admin) {
		t.Errorf("actor pins overlap: operator %v, administrator %v", op, admin)
	}
	for name, box := range map[string]nodeBox{"operator": op, "administrator": admin} {
		if math.Abs(box.high.X-useCase.X) > 0.5 {
			t.Errorf("%s pin does not touch the left border x=%v of the use case: %v", name, useCase.X, box)
		}
	}
	if op.centre().Y >= admin.centre().Y {
		t.Errorf("operator, whose route comes from above, is drawn at y=%v, below administrator at y=%v", op.centre().Y, admin.centre().Y)
	}
	routesEndOnPins(t, boxes, ends, operator, administrator)
	if ends[operator][0] == ends[administrator][0] {
		t.Errorf("both routes end at %v", ends[operator][0])
	}

	single := pinOf(t, rendering, "BoundaryParameters::'Acquire Image'", "operator")
	if got, want := ends[single], []Point{{X: 320, Y: 45}}; !slices.Equal(got, want) {
		t.Errorf("single parameter's route ends at %v, want the stated %v", got, want)
	}
	if !boxes[single].onBorder(Point{X: 320, Y: 45}) {
		t.Errorf("single parameter's pin %v does not meet its route at (320, 45)", boxes[single])
	}
}

// Pins on the right border spread along it too, in the order their routes
// come from.
func TestDOTSpreadsBoundaryParametersOnTheRight(t *testing.T) {
	boxes, ends, pins := pinBoxes(t, "BoundaryParameters::mirroredLab", "BoundaryParameters::'Configure Instrument'", "operator", "administrator")
	op, admin := boxes[pins[0]], boxes[pins[1]]
	if op.overlaps(admin) {
		t.Errorf("actor pins overlap: operator %v, administrator %v", op, admin)
	}
	for name, box := range map[string]nodeBox{"operator": op, "administrator": admin} {
		if math.Abs(box.low.X-480) > 0.5 {
			t.Errorf("%s pin does not touch the right border x=480 of the use case: %v", name, box)
		}
	}
	if admin.centre().Y >= op.centre().Y {
		t.Errorf("administrator, whose route comes from above, is drawn at y=%v, below operator at y=%v", admin.centre().Y, op.centre().Y)
	}
	routesEndOnPins(t, boxes, ends, pins...)
}

// A connection's name a migration made up does not label its edge; a written
// name does.
func TestSynthesizedConnectionNameIsNoEdgeLabel(t *testing.T) {
	rendering := render(t, "boundary-parameters.sysml", "BoundaryParameters::lab")
	labels := edgeLabels(rendering)
	if labels["'Operator to Acquire Image'"] {
		t.Errorf("synthesized name labels an edge; labels: %v", slices.Sorted(maps.Keys(labels)))
	}
	for _, written := range []string{"'Operator to Configure Instrument'", "'Administrator to Configure Instrument'"} {
		if !labels[written] {
			t.Errorf("written name %s labels no edge; labels: %v", written, slices.Sorted(maps.Keys(labels)))
		}
	}
}

// Two actor parameters whose routes reach one corner from different sides are drawn apart,
// each on the side its route comes to, each route ending on its own pin.
func TestDOTSpreadsBoundaryParametersAtACorner(t *testing.T) {
	boxes, ends, pins := pinBoxes(t, "BoundaryParameters::corner", "BoundaryParameters::'Load Sample'", "operator", "technician")
	op, tech := boxes[pins[0]], boxes[pins[1]]
	if op.overlaps(tech) {
		t.Errorf("actor pins overlap: operator %v, technician %v", op, tech)
	}
	if math.Abs(op.high.X-320) > 0.5 {
		t.Errorf("operator, reached from the left, does not touch the left border x=320: %v", op)
	}
	if math.Abs(tech.high.Y-120) > 0.5 {
		t.Errorf("technician, reached from above, does not touch the top border y=120: %v", tech)
	}
	routesEndOnPins(t, boxes, ends, pins...)
}

// Two pins from above, together wider than the top side leaves beyond the pin from the left,
// run from that pin past the far corner rather than back over it.
func TestDOTRunsCrowdedCornerSymbolsPastTheFarCorner(t *testing.T) {
	boxes, ends, pins := pinBoxes(t, "BoundaryParameters::crowdedCorner", "BoundaryParameters::'Load Sample'", "operator", "technician", "administrator")
	op, tech, admin := boxes[pins[0]], boxes[pins[1]], boxes[pins[2]]
	for _, pair := range [][2]nodeBox{{op, tech}, {op, admin}, {tech, admin}} {
		if pair[0].overlaps(pair[1]) {
			t.Errorf("pins overlap: %v and %v", pair[0], pair[1])
		}
	}
	if math.Abs(op.high.X-320) > 0.5 {
		t.Errorf("operator does not touch the left border x=320: %v", op)
	}
	for name, box := range map[string]nodeBox{"technician": tech, "administrator": admin} {
		if math.Abs(box.high.Y-120) > 0.5 {
			t.Errorf("%s does not touch the top border y=120: %v", name, box)
		}
		if box.low.X < op.high.X+dotBoundaryGap-0.5 {
			t.Errorf("%s %v is not clear of the operator %v", name, box, op)
		}
	}
	if tech.centre().X >= admin.centre().X {
		t.Errorf("technician, whose route comes from the left of the administrator's, is drawn right of it: %v, %v", tech, admin)
	}
	if admin.high.X <= 350 {
		t.Errorf("administrator %v does not run past the far corner x=350 of a side too short for both pins", admin)
	}
	routesEndOnPins(t, boxes, ends, pins...)
}

// Two pins from above that fit between neither corner's pin are centred between the two, the
// overlap split evenly, each still touching the top with its route ending on it.
func TestDOTSplitsTheOverlapOfACrowdedSideBetweenItsCorners(t *testing.T) {
	boxes, ends, pins := pinBoxes(t, "BoundaryParameters::crowdedCorners", "BoundaryParameters::'Load Sample'", "operator", "supervisor", "technician", "administrator")
	op, sup, tech, admin := boxes[pins[0]], boxes[pins[1]], boxes[pins[2]], boxes[pins[3]]
	if math.Abs(op.high.X-320) > 0.5 || math.Abs(sup.low.X-350) > 0.5 {
		t.Fatalf("corner pins off their sides: operator %v, supervisor %v", op, sup)
	}
	if tech.overlaps(admin) {
		t.Errorf("pins from above overlap: %v and %v", tech, admin)
	}
	low, high := op.high.X+dotBoundaryGap, sup.low.X-dotBoundaryGap
	if tech.high.X-tech.low.X+dotBoundaryGap+admin.high.X-admin.low.X <= high-low {
		t.Fatalf("the pins from above fit between the corners' (%v..%v): %v, %v", low, high, tech, admin)
	}
	if math.Abs((tech.low.X-low)-(high-admin.high.X)) > 0.5 {
		t.Errorf("overlap not split between the corners (%v..%v): technician %v, administrator %v", low, high, tech, admin)
	}
	for name, box := range map[string]nodeBox{"technician": tech, "administrator": admin} {
		if math.Abs(box.high.Y-120) > 0.5 {
			t.Errorf("%s does not touch the top border y=120: %v", name, box)
		}
	}
	routesEndOnPins(t, boxes, ends, pins...)
}

var dotClusterBB = regexp.MustCompile(`(?s)subgraph "cluster_(n\d+)" \{.*?bb="([-0-9.]+),([-0-9.]+),([-0-9.]+),([-0-9.]+)"`)

// clusterBoxes is the box of every cluster the DOT text gives a `bb`, by ID,
// in the rendering's pixels (y down).
func clusterBoxes(t *testing.T, dot string) map[string]nodeBox {
	t.Helper()
	boxes := map[string]nodeBox{}
	for _, m := range dotClusterBB.FindAllStringSubmatch(dot, -1) {
		boxes[m[1]] = nodeBox{low: Point{X: dotNumber(t, m[2]), Y: -dotNumber(t, m[5])}, high: Point{X: dotNumber(t, m[4]), Y: -dotNumber(t, m[3])}}
	}
	return boxes
}

// memberBoxes renders the view and returns the DOT text's pinned boxes, cluster boxes and
// route ends with the IDs of the named members of node.
func memberBoxes(t *testing.T, view, node string, names ...string) (boxes, clusters map[string]nodeBox, ends map[string][]Point, members []string) {
	t.Helper()
	rendering := render(t, "boundary-parameters.sysml", view)
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatal(err)
	}
	members = append(members, findNode(t, rendering.Roots, node).ID)
	for _, name := range names {
		members = append(members, child(t, rendering, node, name).ID)
	}
	return pinnedBoxes(t, dot), clusterBoxes(t, dot), routeEndsAt(t, dot), members
}

// child is the member of the node named parent that is named name.
func child(t *testing.T, rendering *Rendering, parent, name string) *Node {
	t.Helper()
	for _, node := range findNode(t, rendering.Roots, parent).Children {
		if node.Name == name {
			return node
		}
	}
	t.Fatalf("no member %q of %q", name, parent)
	return nil
}

// Two declared ports, drawn as members of their part, whose routes end at one point of the left
// border are drawn apart along it inside the border, each route ending on its own symbol.
func TestDOTSpreadsDeclaredPortsAlongTheSide(t *testing.T) {
	boxes, _, ends, ports := memberBoxes(t, "BoundaryParameters::ports", "BoundaryParameters::Instrument", "sensor", "heater")
	ports = ports[1:]
	sensor, heater := boxes[ports[0]], boxes[ports[1]]
	if sensor.overlaps(heater) {
		t.Errorf("port symbols overlap: sensor %v, heater %v", sensor, heater)
	}
	for name, box := range map[string]nodeBox{"sensor": sensor, "heater": heater} {
		if math.Abs(box.low.X-320) > 0.5 {
			t.Errorf("%s does not touch the left border x=320 from inside: %v", name, box)
		}
	}
	routesEndOnPins(t, boxes, ends, ports...)
}

// A positioned part with no stated size has its coincident ports drawn apart and its box
// refitted round them: the margin round the ports except on the side they touch, stretched to
// the stated corner; the ports touch the refitted border and the routes end on them.
func TestDOTReboxesAnUnsizedNodeRoundItsSpreadPorts(t *testing.T) {
	boxes, clusters, ends, members := memberBoxes(t, "BoundaryParameters::unsized", "BoundaryParameters::Instrument", "sensor", "heater")
	sensor, heater := boxes[members[1]], boxes[members[2]]
	if sensor.overlaps(heater) {
		t.Errorf("port symbols overlap: sensor %v, heater %v", sensor, heater)
	}
	cluster, ok := clusters[members[0]]
	if !ok {
		t.Fatalf("no bb for the instrument %s; clusters: %v", members[0], clusters)
	}
	ports := sensor.union(heater)
	want := nodeBox{low: Point{X: math.Min(320, ports.low.X), Y: math.Min(120, ports.low.Y-dotClusterMargin)},
		high: Point{X: ports.high.X + dotClusterMargin, Y: ports.high.Y + dotClusterMargin}}
	for _, corner := range [][2]Point{{cluster.low, want.low}, {cluster.high, want.high}} {
		if math.Abs(corner[0].X-corner[1].X) > 0.5 || math.Abs(corner[0].Y-corner[1].Y) > 0.5 {
			t.Fatalf("instrument boxed at %v, want the margin round its spread ports %v", cluster, want)
		}
	}
	for name, box := range map[string]nodeBox{"sensor": sensor, "heater": heater} {
		if math.Abs(box.low.X-cluster.low.X) > 0.5 {
			t.Errorf("%s %v does not touch the refitted left border x=%v", name, box, cluster.low.X)
		}
	}
	routesEndOnPins(t, boxes, ends, members[1:]...)
}

// A part with no Layout at all is boxed round its members; its coincident ports are drawn
// apart all the same, touching the border fitted round them, their routes ending on them.
func TestDOTSpreadsThePortsOfAnUnpositionedNode(t *testing.T) {
	boxes, clusters, ends, members := memberBoxes(t, "BoundaryParameters::unpositioned", "BoundaryParameters::Instrument", "sensor", "heater")
	cluster, sensor, heater := clusters[members[0]], boxes[members[1]], boxes[members[2]]
	if sensor.overlaps(heater) {
		t.Fatalf("port symbols overlap: sensor %v, heater %v", sensor, heater)
	}
	for name, box := range map[string]nodeBox{"sensor": sensor, "heater": heater} {
		if math.Abs(box.low.X-cluster.low.X) > 0.5 {
			t.Errorf("%s %v does not touch the fitted left border x=%v", name, box, cluster.low.X)
		}
	}
	routesEndOnPins(t, boxes, ends, members[1:]...)
}

// Declared ports whose routes end inside their owner are no boundary symbols: they stay where
// the routes end, off the owner's border, as nested parts do.
func TestDOTLeavesInteriorRoutedPortsWhereTheirRoutesEnd(t *testing.T) {
	boxes, _, ends, members := memberBoxes(t, "BoundaryParameters::interiorPorts", "BoundaryParameters::Instrument", "sensor", "heater")
	owner := nodeBox{low: Point{X: 320, Y: 120}, high: Point{X: 480, Y: 220}}
	for name, box := range map[string]nodeBox{"sensor": boxes[members[1]], "heater": boxes[members[2]]} {
		if !owner.enclosesClear(box) {
			t.Errorf("%s %v was moved to the border of %v", name, box, owner)
		}
		if !box.holds(Point{X: 370, Y: 170}) && !box.onBorder(Point{X: 370, Y: 170}) {
			t.Errorf("%s %v does not meet its route's end (370, 170)", name, box)
		}
	}
	routesEndOnPins(t, boxes, ends, members[1:]...)
}

// Nested parts are no boundary symbols: the routes that reach them inside their
// owner leave them where they end, off the owner's border.
func TestDOTLeavesInteriorMembersWhereRoutesPlaceThem(t *testing.T) {
	boxes, _, _, parts := memberBoxes(t, "BoundaryParameters::interior", "BoundaryParameters::Instrument", "detector", "filament")
	parts = parts[1:]
	owner := nodeBox{low: Point{X: 320, Y: 120}, high: Point{X: 480, Y: 220}}
	for name, box := range map[string]nodeBox{"detector": boxes[parts[0]], "filament": boxes[parts[1]]} {
		inside := box.low.X > owner.low.X+1 && box.high.X < owner.high.X-1 && box.low.Y > owner.low.Y+1 && box.high.Y < owner.high.Y-1
		if !inside {
			t.Errorf("%s %v was moved to the border of %v", name, box, owner)
		}
		if !box.holds(Point{X: 370, Y: 170}) && !box.onBorder(Point{X: 370, Y: 170}) {
			t.Errorf("%s %v does not meet its route's end (370, 170)", name, box)
		}
	}
}

// Under the full ports display, a routed pin whose route ends where an unconnected parameter's
// pin is placed is drawn apart from it along the top, every pin still touching the top.
func TestDOTKeepsRoutedPinsOffUnconnectedPins(t *testing.T) {
	boxes, ends, pins := pinBoxesWith(t, Options{Ports: PortsFull}, "BoundaryParameters::freePin", "BoundaryParameters::'Load Sample'", "operator", "technician", "administrator", "supervisor")
	for i, a := range pins {
		if math.Abs(boxes[a].high.Y-120) > 0.5 {
			t.Errorf("%s does not touch the top border y=120: %v", a, boxes[a])
		}
		for _, b := range pins[i+1:] {
			if boxes[a].overlaps(boxes[b]) {
				t.Errorf("pins overlap: %s %v and %s %v", a, boxes[a], b, boxes[b])
			}
		}
	}
	routesEndOnPins(t, boxes, ends, pins[2])
}
