package view

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// findNode is the node of the rendering named name, which must be there.
func findNode(t *testing.T, roots []*Node, name string) *Node {
	t.Helper()
	var found *Node
	var walk func([]*Node)
	walk = func(nodes []*Node) {
		for _, node := range nodes {
			if node.Name == name && found == nil {
				found = node
			}
			walk(node.Children)
		}
	}
	walk(roots)
	if found == nil {
		t.Fatalf("no node %q; nodes: %v", name, sortedKeys(nodeNames(roots)))
	}
	return found
}

// A Layout declared about an element inside the view's body positions the
// element in that view, over the position written inline on the element.
func TestViewLocalLayoutOverridesTheInlineOne(t *testing.T) {
	rendering := render(t, "layout.sysml", "PlantViews::placedView")
	pump := findNode(t, rendering.Roots, "pump")
	want := &Geometry{X: 300, Y: 40, Collapsed: true}
	if !reflect.DeepEqual(pump.Geometry, want) {
		t.Errorf("pump geometry = %+v, want %+v", pump.Geometry, want)
	}
	tank := findNode(t, rendering.Roots, "tank")
	want = &Geometry{X: 500, Y: 40, Width: 120, Height: 60, HasSize: true}
	if !reflect.DeepEqual(tank.Geometry, want) {
		t.Errorf("tank geometry = %+v, want %+v", tank.Geometry, want)
	}
}

// A view positioning nothing shows the inline position of an element and no
// position for one without.
func TestInlineLayoutIsTheFallbackInAnotherView(t *testing.T) {
	rendering := render(t, "layout.sysml", "PlantViews::plainView")
	pump := findNode(t, rendering.Roots, "pump")
	want := &Geometry{X: 10, Y: 20, Width: 100, Height: 50, HasSize: true}
	if !reflect.DeepEqual(pump.Geometry, want) {
		t.Errorf("pump geometry = %+v, want %+v", pump.Geometry, want)
	}
	if tank := findNode(t, rendering.Roots, "tank"); tank.Geometry != nil {
		t.Errorf("tank geometry = %+v, want none", tank.Geometry)
	}
	for _, edge := range rendering.Edges {
		if want := []Point{{60, 45}, {200, 45}}; !reflect.DeepEqual(edge.Route, want) {
			t.Errorf("route of %s = %v, want the inline %v", edge.Label, edge.Route, want)
		}
	}
}

// A Route about a connection inside the view routes its edge there.
func TestConnectionRouteComesFromTheView(t *testing.T) {
	rendering := render(t, "layout.sysml", "PlantViews::placedView")
	if len(rendering.Edges) != 1 {
		t.Fatalf("edges = %+v, want one", rendering.Edges)
	}
	want := []Point{{400, 70}, {450, 120}, {500, 70}}
	if !reflect.DeepEqual(rendering.Edges[0].Route, want) {
		t.Errorf("route = %v, want %v", rendering.Edges[0].Route, want)
	}
}

// A transition's Route, written in its body, routes the edge the lowered state
// graph gives it, whether or not the transition is named; the states are placed
// by the view's Layouts.
func TestTransitionRouteAndStateLayoutReachTheStateRendering(t *testing.T) {
	rendering := render(t, "layout.sysml", "PlantViews::machineView")
	off := findNode(t, rendering.Roots, "off")
	if want := (&Geometry{X: 0, Y: 0}); !reflect.DeepEqual(off.Geometry, want) {
		t.Errorf("off geometry = %+v, want %+v", off.Geometry, want)
	}
	on := findNode(t, rendering.Roots, "on")
	if want := (&Geometry{X: 0, Y: 100, Width: 80, Height: 40, HasSize: true}); !reflect.DeepEqual(on.Geometry, want) {
		t.Errorf("on geometry = %+v, want %+v", on.Geometry, want)
	}
	if routed := routedEdges(rendering); len(routed) != 2 {
		t.Fatalf("routed edges = %+v, want the named off_on and the unnamed on to off", routed)
	}
	if got, want := routeBetween(rendering, off.ID, on.ID), []Point{{50, 10}, {50, 90}}; !reflect.DeepEqual(got, want) {
		t.Errorf("route of off_on = %v, want %v", got, want)
	}
	if got, want := routeBetween(rendering, on.ID, off.ID), []Point{{30, 90}, {30, 10}}; !reflect.DeepEqual(got, want) {
		t.Errorf("route of the unnamed transition = %v, want %v", got, want)
	}
}

// The lowered action graph's nodes and successions trace back to the elements
// they were written as, so a Layout on an action and a Route on a succession
// reach the action rendering.
func TestActionLayoutAndSuccessionRouteReachTheActionRendering(t *testing.T) {
	rendering := render(t, "layout.sysml", "PlantViews::driveView")
	provide := findNode(t, rendering.Roots, "provide")
	if want := (&Geometry{X: 20, Y: 30}); !reflect.DeepEqual(provide.Geometry, want) {
		t.Errorf("provide geometry = %+v, want %+v", provide.Geometry, want)
	}
	park := findNode(t, rendering.Roots, "park")
	if want := (&Geometry{X: 20, Y: 150}); !reflect.DeepEqual(park.Geometry, want) {
		t.Errorf("park geometry = %+v, want %+v", park.Geometry, want)
	}
	routed := routedEdges(rendering)
	if len(routed) != 1 || routed[0].From != provide.ID || routed[0].To != park.ID {
		t.Fatalf("routed edges = %+v, want the one from provide to park", routed)
	}
	if want := []Point{{20, 60}, {20, 120}}; !reflect.DeepEqual(routed[0].Route, want) {
		t.Errorf("route = %v, want %v", routed[0].Route, want)
	}
}

// The states a usage inherits from another document's definition are declared
// outside its scope, and still take the view's Layouts and the inline Routes.
func TestInheritedStatesKeepTheirGeometry(t *testing.T) {
	machine := renderIn(t, "PlantUsages::inheritedMachineView", "layout-usages.sysml", "layout.sysml")
	if len(machine.Notices) != 0 {
		t.Errorf("notices = %v, want none", machine.Notices)
	}
	off := findNode(t, machine.Roots, "off")
	if want := (&Geometry{X: 5, Y: 5}); !reflect.DeepEqual(off.Geometry, want) {
		t.Errorf("off geometry = %+v, want %+v", off.Geometry, want)
	}
	on := findNode(t, machine.Roots, "on")
	if on.Geometry != nil {
		t.Errorf("on geometry = %+v, want none in this view", on.Geometry)
	}
	if got := routedEdges(machine); len(got) != 2 {
		t.Errorf("routed edges = %+v, want the inline routes of both transitions", got)
	}
	if got, want := routeBetween(machine, off.ID, on.ID), []Point{{50, 10}, {50, 90}}; !reflect.DeepEqual(got, want) {
		t.Errorf("route of off_on = %v, want the inline %v", got, want)
	}
	if got, want := routeBetween(machine, on.ID, off.ID), []Point{{30, 90}, {30, 10}}; !reflect.DeepEqual(got, want) {
		t.Errorf("route of the unnamed transition = %v, want the inline %v", got, want)
	}
}

// The nodes an action usage inherits from its definition keep their inline
// positions, take the view's, and the usage's own succession over them its route.
func TestInheritedActionNodesKeepTheirGeometry(t *testing.T) {
	drive := render(t, "layout.sysml", "PlantActions::inheritedDriveView")
	if len(drive.Notices) != 0 {
		t.Errorf("notices = %v, want none", drive.Notices)
	}
	provide := findNode(t, drive.Roots, "provide")
	if want := (&Geometry{X: 20, Y: 30}); !reflect.DeepEqual(provide.Geometry, want) {
		t.Errorf("provide geometry = %+v, want the inline %+v", provide.Geometry, want)
	}
	park := findNode(t, drive.Roots, "park")
	if want := (&Geometry{X: 7, Y: 8}); !reflect.DeepEqual(park.Geometry, want) {
		t.Errorf("park geometry = %+v, want the view's %+v", park.Geometry, want)
	}
	if got := routedEdges(drive); len(got) != 1 || got[0].From != provide.ID || got[0].To != park.ID ||
		!reflect.DeepEqual(got[0].Route, []Point{{1, 2}, {3, 4}}) {
		t.Errorf("routed edges = %+v, want the usage's route from provide to park", got)
	}
}

// routeBetween is the route of the edge from one node to another, nil when
// the rendering draws none.
func routeBetween(rendering *Rendering, from, to string) []Point {
	for _, edge := range rendering.Edges {
		if edge.From == from && edge.To == to {
			return edge.Route
		}
	}
	return nil
}

// routedEdges are the edges of a rendering that carry a route.
func routedEdges(rendering *Rendering) []Edge {
	var routed []Edge
	for _, edge := range rendering.Edges {
		if len(edge.Route) > 0 {
			routed = append(routed, edge)
		}
	}
	return routed
}

// A Canvas in the view's body reaches the rendering; a view stating none in its
// body has none, whatever is stated about it from outside.
func TestCanvasReachesTheRendering(t *testing.T) {
	rendering := render(t, "layout.sysml", "PlantViews::placedView")
	want := &Canvas{Unit: "px", Width: 1200, Height: 800, HasSize: true}
	if !reflect.DeepEqual(rendering.Canvas, want) {
		t.Errorf("canvas = %+v, want %+v", rendering.Canvas, want)
	}
	if plain := render(t, "layout.sysml", "PlantViews::plainView"); plain.Canvas != nil {
		t.Errorf("plainView canvas = %+v, want none from outside its body", plain.Canvas)
	}
}

// An explicit extent of zero is a size the writers show; an unbound one is not.
func TestZeroCanvasExtentIsASize(t *testing.T) {
	rendering := renderIn(t, "PlantUsages::zeroCanvasView", "layout-usages.sysml", "layout.sysml")
	want := &Canvas{Unit: "mm", HasSize: true}
	if !reflect.DeepEqual(rendering.Canvas, want) {
		t.Fatalf("canvas = %+v, want %+v", rendering.Canvas, want)
	}
	if mermaid := rendering.Mermaid(); !strings.Contains(mermaid, "%% canvas: unit=mm w=0 h=0\n") {
		t.Errorf("Mermaid lacks the zero extent:\n%s", mermaid)
	}
	if text := rendering.Text(); !strings.Contains(text, "canvas size 0×0 in mm\n") {
		t.Errorf("text lacks the zero extent:\n%s", text)
	}
	if got := canvasText(&Canvas{Unit: "mm"}); got != "canvas in mm" {
		t.Errorf("canvasText without an extent = %q", got)
	}
}

// The writers show the geometry: Mermaid as comments after the header, text as
// a suffix on each positioned node and edge.
func TestWritersShowTheGeometry(t *testing.T) {
	rendering := render(t, "layout.sysml", "PlantViews::placedView")
	checkGolden(t, filepath.Join("testdata", "layout.text.golden"), rendering.Text())
	checkGolden(t, filepath.Join("testdata", "layout.mermaid.golden"), rendering.Mermaid())
	mermaid := rendering.Mermaid()
	for _, want := range []string{
		"%% canvas: unit=px w=1200 h=800\n",
		" x=300 y=40 collapsed\n",
		" x=500 y=40 w=120 h=60\n",
		" 400,70 450,120 500,70\n",
	} {
		if !strings.Contains(mermaid, want) {
			t.Errorf("Mermaid lacks %q:\n%s", want, mermaid)
		}
	}
	text := rendering.Text()
	for _, want := range []string{"canvas size 1200×800 in px", "pump (Pump) at (300, 40) collapsed", "tank (Tank) at (500, 40) size 120×60"} {
		if !strings.Contains(text, want) {
			t.Errorf("text lacks %q:\n%s", want, text)
		}
	}
}

// A rendering outside any view — the pseudo-view of a document — keeps only the
// inline positions.
func TestRenderExposedKeepsInlineGeometryOnly(t *testing.T) {
	r, idx := loadFixture(t, "layout.sysml")
	rendering, err := r.RenderExposed([]*symbols.Symbol{lookup(t, idx, "Plant::Loop")}, KindInterconnection, "")
	if err != nil {
		t.Fatal(err)
	}
	pump := findNode(t, rendering.Roots, "pump")
	if want := (&Geometry{X: 10, Y: 20, Width: 100, Height: 50, HasSize: true}); !reflect.DeepEqual(pump.Geometry, want) {
		t.Errorf("pump geometry = %+v, want %+v", pump.Geometry, want)
	}
	if rendering.Canvas != nil {
		t.Errorf("canvas = %+v, want none outside a view", rendering.Canvas)
	}
}

// Clone copies the geometry, so changing the copy leaves the original alone.
func TestCloneCopiesTheGeometry(t *testing.T) {
	rendering := render(t, "layout.sysml", "PlantViews::placedView")
	clone := rendering.Clone()
	clone.Canvas.Width = 1
	clone.Edges[0].Route[0].X = 1
	findNode(t, clone.Roots, "pump").Geometry.X = 1
	if rendering.Canvas.Width != 1200 || rendering.Edges[0].Route[0].X != 400 || findNode(t, rendering.Roots, "pump").Geometry.X != 300 {
		t.Errorf("the clone shares geometry with the original")
	}
}

// A model with no layout annotation renders byte for byte as before: no comment,
// no suffix, and the existing goldens stand.
func TestNoAnnotationLeavesTheOutputUnchanged(t *testing.T) {
	for _, tc := range []struct{ name, file, view string }{
		{"tree", "tree.sysml", "VehicleViews::vehicleView"},
		{"interconnection", "interconnection.sysml", "PlantViews::loopView"},
		{"state", "state.sysml", "MachineViews::vehicleStates"},
		{"action", "action.sysml", "FlowViews::driveView"},
	} {
		rendering := render(t, tc.file, tc.view)
		if rendering.Canvas != nil {
			t.Errorf("%s: canvas = %+v, want none", tc.name, rendering.Canvas)
		}
		for _, edge := range rendering.Edges {
			if edge.Route != nil {
				t.Errorf("%s: edge %s->%s has route %v", tc.name, edge.From, edge.To, edge.Route)
			}
		}
		for _, node := range rendering.Data().Nodes {
			if node.Geometry != nil {
				t.Errorf("%s: node %s has geometry %+v", tc.name, node.ID, node.Geometry)
			}
		}
		for _, out := range []string{rendering.Text(), rendering.Mermaid()} {
			if strings.Contains(out, "%% layout") || strings.Contains(out, "%% route") || strings.Contains(out, "%% canvas") || strings.Contains(out, " at (") {
				t.Errorf("%s: output carries geometry:\n%s", tc.name, out)
			}
		}
		checkGolden(t, filepath.Join("testdata", tc.name+".text.golden"), rendering.Text())
	}
}
