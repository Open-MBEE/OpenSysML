package view

import (
	"maps"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
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
		t.Fatalf("no node %q; nodes: %v", name, slices.Sorted(maps.Keys(nodeNames(roots))))
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

// The states and transitions a usage inherits are located in the document
// declaring the definition, not in the usage's; the usage itself stays in its own.
func TestInheritedStatesAreLocatedInTheirDefinitionsDocument(t *testing.T) {
	machine := renderIn(t, "PlantUsages::inheritedMachineView", "layout-usages.sysml", "layout.sysml")
	usage := findNode(t, machine.Roots, "PlantUsages::machine")
	if usage.Origin.Doc != "layout-usages.sysml" {
		t.Errorf("machine is located in %q, want layout-usages.sysml", usage.Origin.Doc)
	}
	for _, name := range []string{"off", "on"} {
		if got := findNode(t, machine.Roots, name).Origin.Doc; got != "layout.sysml" {
			t.Errorf("%s is located in %q, want layout.sysml, where Plant::Machine declares it", name, got)
		}
	}
	for _, edge := range machine.Edges {
		if edge.Origin.Doc != "layout.sysml" {
			t.Errorf("edge %s -> %s is located in %q, want layout.sysml", edge.From, edge.To, edge.Origin.Doc)
		}
	}
}

// The regions a usage inherits from a parallel definition — those of the machine
// itself and those of a composite state within it — are located in the document
// declaring the definition, as the states in them are.
func TestInheritedRegionsAreLocatedInTheirDefinitionsDocument(t *testing.T) {
	machine := renderIn(t, "RegionUsages::controllerView", "regions-usages.sysml", "regions.sysml")
	usage := findNode(t, machine.Roots, "RegionUsages::controller")
	if usage.Origin.Doc != "regions-usages.sysml" {
		t.Errorf("controller is located in %q, want regions-usages.sysml", usage.Origin.Doc)
	}
	var regions []*Node
	var walk func([]*Node)
	walk = func(nodes []*Node) {
		for _, node := range nodes {
			if node.Kind == "region" {
				regions = append(regions, node)
			}
			walk(node.Children)
		}
	}
	walk(machine.Roots)
	if len(regions) != 4 {
		t.Fatalf("got %d regions, want 4 (sensing, acting, pumps, valves); notices %v, nodes %v",
			len(regions), machine.Notices, slices.Sorted(maps.Keys(nodeNames(machine.Roots))))
	}
	for _, region := range regions {
		if region.Origin.Doc != "regions.sysml" {
			t.Errorf("region %s is located in %q, want regions.sysml, where Regions::Controller declares it", region.Name, region.Origin.Doc)
		}
		if !region.Origin.Located() {
			t.Errorf("region %s has no located declaration", region.Name)
		}
	}
	for _, name := range []string{"idle", "busy", "off", "on", "shut", "open", "waiting"} {
		if got := findNode(t, machine.Roots, name).Origin.Doc; got != "regions.sysml" {
			t.Errorf("%s is located in %q, want regions.sysml", name, got)
		}
	}
}

// A pseudostate of the definition's own body, which no state owns, is inherited
// like its states: located in the definition's document, at its declaration,
// and placed by the view's Layout about it.
func TestInheritedTopLevelPseudostatesAreLocatedInTheirDefinitionsDocument(t *testing.T) {
	machine := renderIn(t, "RegionUsages::controllerView", "regions-usages.sysml", "regions.sysml")
	usage := findNode(t, machine.Roots, "RegionUsages::controller")
	pick := findNode(t, machine.Roots, "pick")
	if pick.Kind != "choice" {
		t.Errorf("pick is a %q, want a choice", pick.Kind)
	}
	if !slices.Contains(usage.Children, pick) {
		t.Errorf("pick is not a child of the usage; its children are %v", slices.Sorted(maps.Keys(nodeNames(usage.Children))))
	}
	if pick.Origin.Doc != "regions.sysml" {
		t.Errorf("pick is located in %q, want regions.sysml, where Regions::Controller declares it", pick.Origin.Doc)
	}
	if !pick.Origin.Located() {
		t.Fatal("pick has no located declaration")
	}
	sf := fixtureText(t, "regions.sysml")
	if got := sf.Text(pick.Origin.Span); !strings.HasPrefix(got, "choice pick;") {
		t.Errorf("pick's origin in regions.sysml spans %q, want its declaration", got)
	}
	if want := (&Geometry{X: 15, Y: 25}); !reflect.DeepEqual(pick.Geometry, want) {
		t.Errorf("pick geometry = %+v, want the view's %+v", pick.Geometry, want)
	}
}

// A pseudostate a state usage inherits from the definition typing it — nested in
// the definition's body, or in a composite state of it — is located in the
// definition's document at its declaration, not in the usage's document at the
// same offsets.
func TestInheritedNestedPseudostatesAreLocatedInTheirDefinitionsDocument(t *testing.T) {
	sf := fixtureText(t, "regions.sysml")
	machine := renderIn(t, "RegionUsages::plantView", "regions-usages.sysml", "regions.sysml")
	if len(machine.Notices) != 0 {
		t.Fatalf("notices = %v, want none", machine.Notices)
	}
	cases := []struct {
		owner, pseudo, kind, decl string
	}{
		{"running", "retry", "choice", "choice retry;"},
		{"go", "settle", "junction", "junction settle;"},
	}
	for _, tc := range cases {
		owner := findNode(t, machine.Roots, tc.owner)
		pseudo := findNode(t, machine.Roots, tc.pseudo)
		if pseudo.Kind != tc.kind {
			t.Errorf("%s is a %q, want a %s", tc.pseudo, pseudo.Kind, tc.kind)
		}
		if !slices.Contains(owner.Children, pseudo) {
			t.Errorf("%s is not a child of %s; its children are %v", tc.pseudo, tc.owner, slices.Sorted(maps.Keys(nodeNames(owner.Children))))
		}
		if pseudo.Origin.Doc != "regions.sysml" {
			t.Errorf("%s is located in %q, want regions.sysml, where its definition declares it", tc.pseudo, pseudo.Origin.Doc)
		}
		if !pseudo.Origin.Located() {
			t.Fatalf("%s has no located declaration", tc.pseudo)
		}
		if got := sf.Text(pseudo.Origin.Span); !strings.HasPrefix(got, tc.decl) {
			t.Errorf("%s's origin in regions.sysml spans %q, want its declaration", tc.pseudo, got)
		}
	}
	if got := findNode(t, machine.Roots, "running").Origin.Doc; got != "regions-usages.sysml" {
		t.Errorf("running is located in %q, want regions-usages.sysml, where the usage is written", got)
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
	for _, want := range []string{"canvas size 1200×800 in px", "pump : Pump at (300, 40) collapsed", "tank : Tank at (500, 40) size 120×60"} {
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

// A Style about an element inside the view's body colours it there, over the
// one written inline; a Style about a connection colours its edge.
func TestViewLocalStyleOverridesTheInlineOne(t *testing.T) {
	rendering := render(t, "layout.sysml", "PlantViews::placedView")
	pump := findNode(t, rendering.Roots, "pump")
	want := &Style{Fill: "#FFE8BD", Line: "#333333", Font: "Arial", FontSize: 11, Bold: true}
	if !reflect.DeepEqual(pump.Style, want) {
		t.Errorf("pump style = %+v, want %+v", pump.Style, want)
	}
	if tank := findNode(t, rendering.Roots, "tank"); tank.Style != nil {
		t.Errorf("tank style = %+v, want none", tank.Style)
	}
	if len(rendering.Edges) != 1 || !reflect.DeepEqual(rendering.Edges[0].Style, &Style{Line: "#0000FF"}) {
		t.Errorf("edges = %+v, want one styled #0000FF", rendering.Edges)
	}
}

// Notes reach the rendering anchored to the node they annotate, the inline one
// with the view's own, a Note about a connection anchored to its edge by its
// ends, and a Note about the view itself is free on the canvas.
func TestNotesReachTheRenderingWithTheirAnchors(t *testing.T) {
	rendering := render(t, "layout.sysml", "PlantViews::placedView")
	pump := findNode(t, rendering.Roots, "pump")
	tank := findNode(t, rendering.Roots, "tank")
	want := []Note{
		{Text: "always", Anchor: pump.ID, X: 10, Y: 90},
		{Text: "anchored", Anchor: tank.ID, X: 650, Y: 40, Width: 100, Height: 30, HasSize: true},
		{Text: "check pressure", EdgeFrom: pump.ID, EdgeTo: tank.ID, X: 420, Y: 140},
		{Text: "free", X: 0, Y: 700},
	}
	if !reflect.DeepEqual(rendering.Notes, want) {
		t.Errorf("notes = %+v, want %+v", rendering.Notes, want)
	}
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	// The routed edge's note anchors to a point at the middle of its longest segment.
	if !strings.Contains(dot, `"note:2:on" [shape=point, width=0, height=0, style=invis, pos="425,705!", pin=true]`) ||
		!strings.Contains(dot, `"note:2" -> "note:2:on" [style=dashed`) {
		t.Errorf("the connector's note is not anchored on its route:\n%s", dot)
	}
	text := rendering.Text()
	if !strings.Contains(text, `"check pressure" on pump -> tank at (420, 140)`) {
		t.Errorf("the connector's note is not listed on its edge:\n%s", text)
	}
}

// A Note about a nested action anchors to the node drawn for it, once: the
// body's own flow renders under the same node, so the anchor never dangles.
func TestNestedActionNoteAnchorsToItsDrawnNode(t *testing.T) {
	rendering := render(t, "action_note.sysml", "NoteViews::nestedNoteView")
	if len(rendering.Roots) != 1 {
		t.Fatalf("roots = %d, want 1", len(rendering.Roots))
	}
	var a *Node
	for _, child := range rendering.Roots[0].Children {
		if child.Name == "a" {
			a = child
		}
	}
	if a == nil {
		t.Fatalf("no node named a under the root")
	}
	want := []Note{{Text: "watch", Anchor: a.ID, X: 30, Y: 30}}
	if !reflect.DeepEqual(rendering.Notes, want) {
		t.Errorf("notes = %+v, want %+v", rendering.Notes, want)
	}
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	anchors := regexp.MustCompile(`"note:\d+" -> "([^"]+)"`)
	for _, m := range anchors.FindAllStringSubmatch(dot, -1) {
		if !strings.Contains(dot, `"`+m[1]+`" [`) {
			t.Errorf("note anchor %q declares no node:\n%s", m[1], dot)
		}
	}
}

// A view colouring nothing shows the inline Style and the inline Note alone.
func TestInlineStyleIsTheFallbackInAnotherView(t *testing.T) {
	rendering := render(t, "layout.sysml", "PlantViews::plainView")
	pump := findNode(t, rendering.Roots, "pump")
	if want := (&Style{Fill: "#FFFFDC"}); !reflect.DeepEqual(pump.Style, want) {
		t.Errorf("pump style = %+v, want the inline %+v", pump.Style, want)
	}
	if want := []Note{{Text: "always", Anchor: pump.ID, X: 10, Y: 90}}; !reflect.DeepEqual(rendering.Notes, want) {
		t.Errorf("notes = %+v, want %+v", rendering.Notes, want)
	}
}

// The lowered state graph's nodes and transitions trace back to the elements
// they were written as, so Styles and Notes reach the state rendering.
func TestStateStyleAndNoteReachTheStateRendering(t *testing.T) {
	rendering := render(t, "layout.sysml", "PlantViews::machineView")
	on := findNode(t, rendering.Roots, "on")
	if want := (&Style{Fill: "#EBEBD7"}); !reflect.DeepEqual(on.Style, want) {
		t.Errorf("on style = %+v, want %+v", on.Style, want)
	}
	off := findNode(t, rendering.Roots, "off")
	if want := []Note{{Text: "resting", Anchor: on.ID, X: 100, Y: 100},
		{Text: "on demand", EdgeFrom: off.ID, EdgeTo: on.ID, X: 70, Y: 40}}; !reflect.DeepEqual(rendering.Notes, want) {
		t.Errorf("notes = %+v, want %+v", rendering.Notes, want)
	}
	var styled []Edge
	for _, edge := range rendering.Edges {
		if edge.Style != nil {
			styled = append(styled, edge)
		}
	}
	if len(styled) != 1 || styled[0].From != off.ID || styled[0].To != on.ID || styled[0].Style.Line != "#FF0000" {
		t.Errorf("styled edges = %+v, want off_on alone in #FF0000", styled)
	}
}

// A Style that does not read as one is noticed and colours nothing.
func TestMalformedStyleIsNoticed(t *testing.T) {
	rendering := render(t, "layout.sysml", "PlantViews::badStyleView")
	if on := findNode(t, rendering.Roots, "on"); on.Style != nil {
		t.Errorf("on style = %+v, want none", on.Style)
	}
	if len(rendering.Notices) != 1 || !strings.Contains(rendering.Notices[0], "Style") {
		t.Errorf("notices = %q, want one about the Style", rendering.Notices)
	}
}

// Clone copies the styles and notes, so changing the copy leaves the original.
func TestCloneCopiesStylesAndNotes(t *testing.T) {
	rendering := render(t, "layout.sysml", "PlantViews::placedView")
	clone := rendering.Clone()
	findNode(t, clone.Roots, "pump").Style.Fill = "#000000"
	clone.Edges[0].Style.Line = "#000000"
	clone.Notes[0].Text = "changed"
	if got := findNode(t, rendering.Roots, "pump").Style.Fill; got != "#FFE8BD" {
		t.Errorf("original pump fill = %q after editing the clone", got)
	}
	if got := rendering.Edges[0].Style.Line; got != "#0000FF" {
		t.Errorf("original edge line = %q after editing the clone", got)
	}
	if got := rendering.Notes[0].Text; got != "always" {
		t.Errorf("original note = %q after editing the clone", got)
	}
}
