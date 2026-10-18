package view

import (
	"math"
	"strings"
	"testing"
)

// portSquare checks that the DOT text pins id as the notation's port square, dotPinSize on a
// side, centred where the Layout's rectangle puts it on its owner's border.
func portSquare(t *testing.T, boxes map[string]nodeBox, id string, centre Point) {
	t.Helper()
	box, ok := boxes[id]
	if !ok {
		t.Fatalf("no pinned box for %s; boxes: %v", id, boxes)
	}
	if width, height := box.size(); math.Abs(width-dotPinSize) > 0.5 || math.Abs(height-dotPinSize) > 0.5 {
		t.Errorf("%s drawn %vx%v, want the port square %dx%d: %v", id, width, height, dotPinSize, dotPinSize, box)
	}
	if got := box.centre(); math.Abs(got.X-centre.X) > 0.5 || math.Abs(got.Y-centre.Y) > 0.5 {
		t.Errorf("%s centred at %v, want %v", id, got, centre)
	}
}

// Ports a part has from its definition, exposed beside the part, whose Layouts are the source
// tool's rectangles sized to the names: each is drawn once, as the pin of the part, the
// notation's square on the side of the part the rectangle's centre lies off, level with it; the
// connector between them joins the two pins, and neither port is a root of its own.
func TestLayoutSizedTypedPortsAreThePartsPins(t *testing.T) {
	rendering := render(t, "layout-ports.sysml", "LayoutPorts::typed")
	if len(rendering.Roots) != 2 {
		t.Fatalf("roots = %v, want the two parts alone", rendering.Roots)
	}
	detector, amplifier := findNode(t, rendering.Roots, "LayoutPorts::Chain::detector"), findNode(t, rendering.Roots, "LayoutPorts::Chain::amplifier")
	if len(detector.Ports) != 1 || len(amplifier.Ports) != 1 {
		t.Fatalf("pins: detector %v, amplifier %v, want one each", detector.Ports, amplifier.Ports)
	}
	out, in := detector.Ports[0].ID, amplifier.Ports[0].ID
	if len(rendering.Edges) != 1 || rendering.Edges[0].FromPort != out || rendering.Edges[0].ToPort != in {
		t.Fatalf("edges = %+v, want one from %s to %s", rendering.Edges, out, in)
	}
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatal(err)
	}
	boxes := pinnedBoxes(t, dot)
	portSquare(t, boxes, out, Point{X: 265, Y: 180 + dotPinSize/2})
	portSquare(t, boxes, in, Point{X: 265, Y: 240 - dotPinSize/2})
	for _, name := range []string{`"tx"`, `"rx"`} {
		if got := strings.Count(dot, "xlabel="+name); got != 1 {
			t.Errorf("DOT names the port %s %d times, want once beside its square:\n%s", name, got, dot)
		}
	}
	if strings.Contains(dot, "width=1.8") {
		t.Errorf("DOT draws a port at the source rectangle's width:\n%s", dot)
	}
}

// Ports a part declares, drawn as its members, whose Layouts are the source tool's rectangles:
// each is the square on the side of its part nearest the rectangle's centre — the right border
// it straddles, the left border, and the bottom border nearest a centre inside the nested part
// — and the connector's route is led on to the squares.
func TestLayoutSizedDeclaredPortsArePinsOnTheirOwnersBorder(t *testing.T) {
	rendering := render(t, "layout-ports.sysml", "LayoutPorts::declared")
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatal(err)
	}
	boxes, ends := pinnedBoxes(t, dot), routeEndsAt(t, dot)
	out := child(t, rendering, "LayoutPorts::Bench::supply", "outlet").ID
	in := child(t, rendering, "LayoutPorts::Bench::instrument", "inlet").ID
	probe := findNode(t, rendering.Roots, "probe").ID
	portSquare(t, boxes, out, Point{X: 240 + dotPinSize/2, Y: 78})
	portSquare(t, boxes, in, Point{X: 400 - dotPinSize/2, Y: 60})
	portSquare(t, boxes, probe, Point{X: 565, Y: 180 + dotPinSize/2})
	clusters := clusterBoxes(t, dot)
	supply, instrument := clusters[findNode(t, rendering.Roots, "LayoutPorts::Bench::supply").ID], clusters[findNode(t, rendering.Roots, "LayoutPorts::Bench::instrument").ID]
	if supply.low.X != 40 || supply.high.X != 240 || instrument.low.X != 400 || instrument.high.X != 700 {
		t.Errorf("owners moved: supply %v, instrument %v", supply, instrument)
	}
	routesEndOnPins(t, boxes, ends, in)
	if routeEnds := ends[in]; len(routeEnds) != 1 {
		t.Fatalf("feed ends %v, want one at the instrument's pin", routeEnds)
	}
}

// A definition's port in a tree rendering follows the same rule: the square on the border the
// rectangle's centre lies on, not the rectangle, though a member nested in the port is left
// undrawn beneath it.
func TestLayoutSizedPortOfADefinitionIsAPinInATree(t *testing.T) {
	rendering := render(t, "layout-ports.sysml", "LayoutPorts::tree")
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatal(err)
	}
	out := child(t, rendering, "LayoutPorts::Detector", "tx").ID
	portSquare(t, pinnedBoxes(t, dot), out, Point{X: 240 + dotPinSize/2, Y: 118})
}

// A connection a definition declares between its port, named bare, and a nested part, drawn
// beside the one usage of the definition the view exposes: the port is the usage's pin, shown
// though the display is minimal, the part is nested in the usage, and the connection joins them
// there.
func TestABareConnectionToADefinitionsPortEndsAtItsOnlyUsagesPin(t *testing.T) {
	rendering := render(t, "layout-ports.sysml", "LayoutPorts::soloWire")
	if len(rendering.Roots) != 1 {
		t.Fatalf("roots = %v, want the usage alone", rendering.Roots)
	}
	probe := findNode(t, rendering.Roots, "LayoutPorts::Solo::probe")
	if len(probe.Ports) != 1 || !probe.Ports[0].Exposed {
		t.Fatalf("pins of the usage = %+v, want its one exposed port", probe.Ports)
	}
	sensor := findNode(t, probe.Children, "sensor")
	if len(rendering.Edges) != 1 || rendering.Edges[0].From != probe.ID || rendering.Edges[0].FromPort != probe.Ports[0].ID || rendering.Edges[0].To != sensor.ID {
		t.Fatalf("edges = %+v, want one from the pin %s to %s", rendering.Edges, probe.Ports[0].ID, sensor.ID)
	}
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatal(err)
	}
	portSquare(t, pinnedBoxes(t, dot), probe.Ports[0].ID, Point{X: 340 + dotPinSize/2, Y: 100})
}

// Two usages of the definition each pin its port and nest its part: the exposed port is both
// their pins, and the connection the definition declares is drawn in each usage, from its pin
// to its part, and not again by itself.
func TestADefinitionsPortSeveralUsagesPinKeepsItsOwnNode(t *testing.T) {
	rendering := render(t, "layout-ports.sysml", "LayoutPorts::pairWire")
	if len(rendering.Roots) != 2 {
		t.Fatalf("roots = %v, want the two usages alone", rendering.Roots)
	}
	if len(rendering.Edges) != 2 {
		t.Fatalf("edges = %+v, want one in each usage", rendering.Edges)
	}
	for i, name := range []string{"LayoutPorts::Rig::primary", "LayoutPorts::Rig::backup"} {
		usage := findNode(t, rendering.Roots, name)
		if len(usage.Ports) != 1 || !usage.Ports[0].Exposed {
			t.Fatalf("pins of %s = %+v, want its one exposed port", name, usage.Ports)
		}
		sensor := findNode(t, usage.Children, "sensor")
		if edge := rendering.Edges[i]; edge.From != usage.ID || edge.FromPort != usage.Ports[0].ID || edge.To != sensor.ID {
			t.Errorf("edge %d = %+v, want from the pin %s of %s to %s", i, edge, usage.Ports[0].ID, name, sensor.ID)
		}
	}
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatal(err)
	}
	portSquare(t, pinnedBoxes(t, dot), findNode(t, rendering.Roots, "LayoutPorts::Rig::primary").Ports[0].ID, Point{X: 340 + dotPinSize/2, Y: 100})
}

// A definition's port exposed beside the usage with nothing connected to it is still drawn,
// as the usage's pin, under the minimal display.
func TestAnExposedUnconnectedPortIsDrawnUnderTheMinimalDisplay(t *testing.T) {
	rendering := render(t, "layout-ports.sysml", "LayoutPorts::bare")
	if len(rendering.Roots) != 1 {
		t.Fatalf("roots = %v, want the usage alone", rendering.Roots)
	}
	probe := rendering.Roots[0]
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(dot, `xlabel="tap"`); got != 1 {
		t.Errorf("DOT names the exposed port %d times, want once:\n%s", got, dot)
	}
	portSquare(t, pinnedBoxes(t, dot), probe.Ports[0].ID, Point{X: 340 + dotPinSize/2, Y: 100})
}

// A sized port nested in a sized port, in a tree: the outer port is squared first, so the
// inner square sits on the outer square's border, not on the rectangle it replaced.
func TestANestedSizedPortAttachesToItsParentsSquare(t *testing.T) {
	rendering := render(t, "layout-ports.sysml", "LayoutPorts::nested")
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatal(err)
	}
	boxes := pinnedBoxes(t, dot)
	bus := child(t, rendering, "LayoutPorts::Hub", "bus")
	lane := bus.Children[0]
	portSquare(t, boxes, bus.ID, Point{X: 240 + dotPinSize/2, Y: 118})
	portSquare(t, boxes, lane.ID, Point{X: 240 - dotPinSize/2, Y: 118 + dotPinSize/2})
	if outer, inner := boxes[bus.ID], boxes[lane.ID]; math.Abs(inner.high.X-outer.low.X) > 0.5 {
		t.Errorf("the nested square %v does not touch the outer square %v", inner, outer)
	}
}
