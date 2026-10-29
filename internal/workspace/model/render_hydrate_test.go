package model

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

var (
	stationBase = []byte(`package Stations {
	part def Pump { port inlet; port outlet; }
	part def Tank { port drain; }
	part station {
		part pump : Pump;
		part tank : Tank;
		connect pump.outlet to tank.drain;
	}
}
`)
	stationViews = []byte(`package StationViews {
	private import StandardViewDefinitions::*;
	view def StationView {
		expose Stations::station::**;
	}
	view stationView : StationView;
}
`)
)

func stationInputs() []Input {
	return []Input{
		{Name: "station.sysml", Content: stationBase, Version: 1},
		{Name: "views.sysml", Content: stationViews, Version: 1},
	}
}

// recordedStations is a workspace holding the station documents as the records
// a warm cache serves.
func recordedStations(t *testing.T) *Workspace {
	t.Helper()
	cache, err := libs.NewCacheIn(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	warm := NewWorkspace(WithRecordCache(cache))
	warm.OpenAll(stationInputs())
	warm.DiagnosticsAll([]string{"station.sysml", "views.sysml"})
	ws := NewWorkspace(WithRecordCache(cache))
	ws.OpenAll(stationInputs())
	for _, name := range []string{"station.sysml", "views.sysml"} {
		if !ws.Recorded(name) {
			t.Fatalf("%s is not held as its record: the fixture is vacuous", name)
		}
	}
	return ws
}

func drawnNames(rendering *view.Rendering) []string {
	var names []string
	var walk func(nodes []*view.Node)
	walk = func(nodes []*view.Node) {
		for _, n := range nodes {
			names = append(names, n.Name)
			walk(n.Children)
		}
	}
	walk(rendering.Roots)
	return names
}

func renderRecorded(t *testing.T, ws *Workspace, doc, fqn string) *view.Rendering {
	t.Helper()
	var rendering *view.Rendering
	if err := ws.Read(func(r *Reading) (err error) {
		rendering, _, err = r.RenderView(doc, fqn)
		return err
	}); err != nil {
		t.Fatalf("render %s %s: %v", doc, fqn, err)
	}
	return rendering
}

// The first rendering of a recorded document draws the bodies of what the view
// exposes: the rendered document and the documents declaring the exposed
// elements are parsed first, since a record carries no declarations to walk.
func TestRenderViewHydratesTheDocumentsItDraws(t *testing.T) {
	ws := recordedStations(t)
	rendering := renderRecorded(t, ws, "views.sysml", "StationViews::stationView")
	names := strings.Join(drawnNames(rendering), " ")
	if !strings.Contains(names, "pump") || !strings.Contains(names, "tank") {
		t.Fatalf("drew %q, want the parts of station", names)
	}
	for _, name := range []string{"station.sysml", "views.sysml"} {
		if ws.Recorded(name) {
			t.Errorf("%s is still held as its record after a rendering drew it", name)
		}
	}
}

// A pseudo-view of a recorded document parses the workspace before walking its
// declarations, as a declared view does.
func TestRenderPseudoViewHydratesRecordedDocuments(t *testing.T) {
	ws := recordedStations(t)
	rendering := renderRecorded(t, ws, "station.sysml", "#tree:Stations::station")
	names := strings.Join(drawnNames(rendering), " ")
	if !strings.Contains(names, "pump") || !strings.Contains(names, "tank") {
		t.Fatalf("drew %q, want the parts of station", names)
	}
	for _, name := range []string{"station.sysml", "views.sysml"} {
		if ws.Recorded(name) {
			t.Errorf("%s is still held as its record after a rendering", name)
		}
	}
}

// What a rendering reaches beyond the exposed elements is drawn too: the body
// of a recorded definition an exposed usage inherits from, and what a nested
// view of the rendered one exposes from another recorded document.
func TestRenderDrawsRecordedDefinitionsAndNestedExposures(t *testing.T) {
	inputs := []Input{
		{Name: "plant.sysml", Content: []byte("package Plant { part def Machine { part motor; } }"), Version: 1},
		{Name: "uses.sysml", Content: []byte("package Uses { private import Plant::*; part machine : Machine; }"), Version: 1},
		{Name: "station.sysml", Content: stationBase, Version: 1},
		{Name: "views.sysml", Content: []byte(`package Nested {
	view parent {
		expose Uses::machine;
		view child { expose Stations::station::**; }
	}
}`), Version: 1},
	}
	names := []string{"plant.sysml", "uses.sysml", "station.sysml", "views.sysml"}
	cache, err := libs.NewCacheIn(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	warm := NewWorkspace(WithRecordCache(cache))
	warm.OpenAll(inputs)
	warm.DiagnosticsAll(names)
	ws := NewWorkspace(WithRecordCache(cache))
	ws.OpenAll(inputs)
	for _, name := range names {
		if !ws.Recorded(name) {
			t.Fatalf("%s is not held as its record: the fixture is vacuous", name)
		}
	}
	rendering := renderRecorded(t, ws, "views.sysml", "Nested::parent")
	drawn := strings.Join(drawnNames(rendering), " ")
	for _, want := range []string{"machine", "motor", "pump", "tank"} {
		if !strings.Contains(drawn, want) {
			t.Errorf("drew %q, want %s among them", drawn, want)
		}
	}
}
