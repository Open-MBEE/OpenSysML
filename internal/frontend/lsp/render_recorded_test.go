package lsp

import (
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

const recordedStations = `package Stations {
	part def Pump { port inlet; port outlet; }
	part def Tank { port drain; }
	part station {
		part pump : Pump;
		part tank : Tank;
		connect pump.outlet to tank.drain;
	}
}
`

const recordedStationViews = `package StationViews {
	private import StandardViewDefinitions::*;
	view def StationView {
		expose Stations::station::**;
	}
	view stationView : StationView;
}
`

// opensysml/render on documents a workspace holds as their interface records
// draws the exposed elements, before anything else is asked about them.
func TestRenderDrawsRecordedDocuments(t *testing.T) {
	dir := t.TempDir()
	stationURI := uri.File(filepath.Join(dir, "station.sysml"))
	viewsURI := uri.File(filepath.Join(dir, "views.sysml"))
	inputs := []model.Input{
		{Name: stationURI.Filename(), Content: []byte(recordedStations), Version: 1},
		{Name: viewsURI.Filename(), Content: []byte(recordedStationViews), Version: 1},
	}
	cache, err := libs.NewCacheIn(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	warm := model.NewWorkspace(model.WithRecordCache(cache))
	warm.OpenAll(inputs)
	warm.DiagnosticsAll([]string{inputs[0].Name, inputs[1].Name})

	ws := model.NewWorkspace(model.WithRecordCache(cache))
	ws.OpenAll(inputs)
	for _, in := range inputs {
		if !ws.Recorded(in.Name) {
			t.Fatalf("%s is not held as its record: the fixture is vacuous", in.Name)
		}
	}
	s := NewServer(ws)
	res, err := s.Render(&RenderParams{TextDocument: protocol.TextDocumentIdentifier{URI: viewsURI}, View: "StationViews::stationView", Form: "text"})
	if err != nil {
		t.Fatal(err)
	}
	drawn := map[string]bool{}
	for _, n := range res.Nodes {
		drawn[n.Name] = true
	}
	if !drawn["pump"] || !drawn["tank"] {
		t.Errorf("rendered %v, want pump and tank among them:\n%s", drawn, res.Artifact)
	}
}
