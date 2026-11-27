package edit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

// swimlaneRoundTrip exports a view of the placed-swimlane fixture, edits one cell of the
// file, imports and applies it, and returns the edited file, the import and the re-export.
func swimlaneRoundTrip(t *testing.T, viewName string, pick func(*view.DrawioCell) bool, edit func(cell string) string) (*view.DrawioFile, *view.DrawioImport, *view.DrawioFile) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(viewFixtures, "action-swimlane-layout.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	m, rendering, options := drawioModel(t, string(content), viewName)
	document, exported := drawioExport(t, rendering, options)
	cell := drawioFirst(exported, pick)
	if cell == nil {
		t.Fatalf("the export of %s has no cell to edit", viewName)
	}
	edited, imp := drawioImport(t, drawioImporter(t, rendering, options), rendering.View, drawioMutate(t, document, cell.ID, edit))
	res, err := Apply(m, DrawioOperations(imp, false))
	if err != nil {
		t.Fatalf("apply %+v: %v", imp.Edits, err)
	}
	_, redrawn, reoptions := drawioModel(t, string(res.Content), viewName)
	_, re := drawioExport(t, redrawn, reoptions)
	return edited, imp, re
}

func swimlaneCell(c *view.DrawioCell) bool {
	return c.Vertex && strings.Contains(c.Label, "line.robot")
}

// A resized flow no Layout places is pinned where the file shows it, and with it the
// placed swimlane drawn in its frame and the node placed in that lane.
func TestDrawioResizedFlowPinsItsPlacedSwimlane(t *testing.T) {
	flow := func(c *view.DrawioCell) bool { return c.FQN == "PlacedLane::run" }
	load := func(c *view.DrawioCell) bool { return c.FQN == "PlacedLane::run::load" }
	edited, imp, re := swimlaneRoundTrip(t, "PlacedLane::heldView", flow, func(cell string) string {
		return drawioShift(cell, "width", 100)
	})
	for name, pick := range map[string]func(*view.DrawioCell) bool{"flow": flow, "swimlane": swimlaneCell, "load": load} {
		was, again := drawioFirst(edited, pick), drawioFirst(re, pick)
		if was == nil || again == nil {
			t.Fatalf("%s: in the file %v, re-exported %v, want both", name, was, again)
		}
		if !drawioSameBox(was.Geometry, again.Geometry) {
			t.Errorf("%s was at %+v, re-exported at %+v (edits %+v)", name, was.Geometry, again.Geometry, imp.Edits)
		}
	}
}

// A placed swimlane moved in the file is a Layout about its performer: the lane is the node the view draws for it.
func TestDrawioMovedPlacedSwimlaneIsALayoutAboutItsPerformer(t *testing.T) {
	edited, imp, re := swimlaneRoundTrip(t, "PlacedLane::laneView", swimlaneCell, func(cell string) string {
		return drawioShift(drawioShift(cell, "x", 40), "y", 30)
	})
	if len(imp.Edits) != 1 || imp.Edits[0].Kind != view.DrawioEditLayout || imp.Edits[0].FQN != "PlacedLane::line::robot" {
		t.Fatalf("edits = %+v, want one Layout about PlacedLane::line::robot", imp.Edits)
	}
	if layout := imp.Edits[0].Layout; layout.X != 360 || layout.Y != 150 || !layout.HasSize || layout.Width != 220 || layout.Height != 180 {
		t.Errorf("layout = %+v, want 360,150 with its stated 220x180", layout)
	}
	if was, again := drawioFirst(edited, swimlaneCell), drawioFirst(re, swimlaneCell); again == nil || !drawioSameBox(was.Geometry, again.Geometry) {
		t.Errorf("swimlane was at %+v, re-exported at %+v", was.Geometry, again)
	}
}
