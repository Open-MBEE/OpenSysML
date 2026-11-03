package lsptools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

const stations = `package Stations {
	part def Pump { port inlet; port outlet; }
	part def Tank { port drain; }
	part station {
		part pump : Pump;
		part tank : Tank;
		connect pump.outlet to tank.drain;
	}
}
`

const stationViews = `package StationViews {
	private import StandardViewDefinitions::*;
	view def StationView {
		expose Stations::station::**;
	}
	view stationView : StationView;
}
`

// recordedWorkspace is a workspace over a record cache warmed with the files
// under root, so a host over it holds them as their interface records.
func recordedWorkspace(t *testing.T, root string, names ...string) *model.Workspace {
	t.Helper()
	cache, err := libs.NewCacheIn(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var inputs []model.Input
	var paths []string
	for _, name := range names {
		path := filepath.Join(root, name)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, model.Input{Name: path, Content: content, Version: 1})
		paths = append(paths, path)
	}
	warm := model.NewWorkspace(model.WithRecordCache(cache))
	warm.OpenAll(inputs)
	warm.DiagnosticsAll(paths)
	return model.NewWorkspace(model.WithRecordCache(cache))
}

// The first render_view on a fresh host whose files are held as their records
// draws the exposed elements: nothing else need be asked about the document first.
func TestRenderViewFirstOverRecordedDocuments(t *testing.T) {
	root := t.TempDir()
	write(t, root, "station.sysml", stations)
	write(t, root, "views.sysml", stationViews)
	ws := recordedWorkspace(t, root, "station.sysml", "views.sysml")
	f := newFixtureOver(t, root, ws)
	for _, name := range []string{"station.sysml", "views.sysml"} {
		if !ws.Recorded(filepath.Join(root, name)) {
			t.Fatalf("%s is not held as its record: the fixture is vacuous", name)
		}
	}
	var render struct {
		Artifact string `json:"artifact"`
		Nodes    []struct {
			Name string `json:"name"`
			FQN  string `json:"fqn"`
		} `json:"nodes"`
	}
	for _, in := range []map[string]any{
		{"path": "views.sysml", "form": "text", "view": "StationViews::stationView"},
		{"path": "station.sysml", "form": "text", "view": "#tree:Stations::station"},
	} {
		f.ok("render_view", in, &render)
		drawn := map[string]bool{}
		for _, n := range render.Nodes {
			drawn[n.Name], drawn[n.FQN] = true, true
		}
		for _, want := range []string{"pump", "tank"} {
			if !drawn[want] && !drawn["Stations::station::"+want] {
				t.Errorf("render_view(%v) drew %v, want %s among them:\n%s", in, drawn, want, render.Artifact)
			}
		}
		if in["path"] == "station.sysml" && !drawn["Stations::station::pump"] {
			t.Errorf("render_view(%v) names no FQN of the rendered document's own declarations: %v", in, drawn)
		}
		ws = recordedWorkspace(t, root, "station.sysml", "views.sysml")
		f = newFixtureOver(t, root, ws)
	}
}
