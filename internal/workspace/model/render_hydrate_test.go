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

// A pseudo-view of a recorded document parses that document before walking its
// declarations; a document the rendering does not reach stays recorded.
func TestRenderPseudoViewHydratesTheRenderedDocument(t *testing.T) {
	ws := recordedStations(t)
	rendering := renderRecorded(t, ws, "station.sysml", "#tree:Stations::station")
	names := strings.Join(drawnNames(rendering), " ")
	if !strings.Contains(names, "pump") || !strings.Contains(names, "tank") {
		t.Fatalf("drew %q, want the parts of station", names)
	}
	if ws.Recorded("station.sysml") {
		t.Error("station.sysml is still held as its record after a rendering drew it")
	}
	if !ws.Recorded("views.sysml") {
		t.Error("views.sysml was parsed for a rendering that does not draw it")
	}
}
