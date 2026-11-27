package edit

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

// swimlaneRoundTrip exports a view of the placed-swimlane fixture, edits one cell of the
// file, imports and applies it, and returns the edited file, the import and the re-export.
func swimlaneRoundTrip(t *testing.T, viewName string, pick func(*view.DrawioFile) *view.DrawioCell, edit func(document, id string) string) (*view.DrawioFile, *view.DrawioImport, *view.DrawioFile) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(viewFixtures, "action-swimlane-layout.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	m, rendering, options := drawioModel(t, string(content), viewName)
	document, exported := drawioExport(t, rendering, options)
	cell := pick(exported)
	if cell == nil {
		t.Fatalf("the export of %s has no cell to edit", viewName)
	}
	edited, imp := drawioImport(t, drawioImporter(t, rendering, options), rendering.View, edit(document, cell.ID))
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

// cellNamed picks the cell of the element of a qualified name; laneOf the swimlane holding it.
func cellNamed(fqn string) func(*view.DrawioFile) *view.DrawioCell {
	return func(f *view.DrawioFile) *view.DrawioCell {
		return drawioFirst(f, func(c *view.DrawioCell) bool { return c.FQN == fqn })
	}
}

func laneOf(fqn string) func(*view.DrawioFile) *view.DrawioCell {
	return func(f *view.DrawioFile) *view.DrawioCell {
		if held := cellNamed(fqn)(f); held != nil {
			return f.Cell(held.Parent)
		}
		return nil
	}
}

// shifted moves the cell id of the document by dx, dy.
func shifted(t *testing.T, dx, dy float64) func(document, id string) string {
	return func(document, id string) string {
		return drawioMutate(t, document, id, func(cell string) string {
			return drawioShift(drawioShift(cell, "x", dx), "y", dy)
		})
	}
}

// A resized flow no Layout places is pinned where the file shows it, and with it the
// placed swimlane drawn in its frame and the node placed in that lane.
func TestDrawioResizedFlowPinsItsPlacedSwimlane(t *testing.T) {
	flow, load := cellNamed("PlacedLane::run"), cellNamed("PlacedLane::run::load")
	edited, imp, re := swimlaneRoundTrip(t, "PlacedLane::heldView", flow, func(document, id string) string {
		return drawioMutate(t, document, id, func(cell string) string { return drawioShift(cell, "width", 100) })
	})
	for name, pick := range map[string]func(*view.DrawioFile) *view.DrawioCell{"flow": flow, "swimlane": laneOf("PlacedLane::run::load"), "load": load} {
		was, again := pick(edited), pick(re)
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
	lane := func(f *view.DrawioFile) *view.DrawioCell { return drawioFirst(f, swimlaneCell) }
	edited, imp, re := swimlaneRoundTrip(t, "PlacedLane::laneView", lane, shifted(t, 40, 30))
	if len(imp.Edits) != 1 || imp.Edits[0].Kind != view.DrawioEditLayout || imp.Edits[0].FQN != "PlacedLane::line::robot" {
		t.Fatalf("edits = %+v, want one Layout about PlacedLane::line::robot", imp.Edits)
	}
	if layout := imp.Edits[0].Layout; layout.X != 360 || layout.Y != 150 || !layout.HasSize || layout.Width != 220 || layout.Height != 180 {
		t.Errorf("layout = %+v, want 360,150 with its stated 220x180", layout)
	}
	if was, again := lane(edited), lane(re); again == nil || !drawioSameBox(was.Geometry, again.Geometry) {
		t.Errorf("swimlane was at %+v, re-exported at %+v", was.Geometry, again)
	}
}

// A performer's lane moved in one flow leaves its lane in another flow of the view, and the
// node placed in that lane, where they were in their own flow.
func TestDrawioMovedSwimlaneLeavesItsPerformersOtherLane(t *testing.T) {
	moved, other := laneOf("PlacedLane::runA::load"), laneOf("PlacedLane::runB::pack")
	edited, imp, re := swimlaneRoundTrip(t, "PlacedLane::sharedView", moved, shifted(t, 40, 30))
	within := func(f *view.DrawioFile, pick func(*view.DrawioFile) *view.DrawioCell, flow string) view.DrawioBox {
		cell, frame := pick(f), cellNamed(flow)(f)
		if cell == nil || frame == nil {
			t.Fatalf("no cell in %s (edits %+v)", flow, imp.Edits)
		}
		box := cell.Geometry
		box.X, box.Y = box.X-frame.Geometry.X, box.Y-frame.Geometry.Y
		return box
	}
	for _, c := range []struct {
		name string
		pick func(*view.DrawioFile) *view.DrawioCell
		flow string
	}{
		{"the moved swimlane", moved, "PlacedLane::runA"},
		{"load", cellNamed("PlacedLane::runA::load"), "PlacedLane::runA"},
		{"the other swimlane", other, "PlacedLane::runB"},
		{"pack", cellNamed("PlacedLane::runB::pack"), "PlacedLane::runB"},
	} {
		if was, again := within(edited, c.pick, c.flow), within(re, c.pick, c.flow); !drawioSameBox(was, again) {
			t.Errorf("%s was at %+v in its flow, re-exported at %+v (edits %+v)", c.name, was, again, imp.Edits)
		}
	}
}

// With the performer's first flow folded, the lane drawn in the next flow is the placed one:
// moved in the file, it is a Layout about the performer, and it is re-exported where the file put it.
func TestDrawioMovedSwimlaneOfTheFlowBesideAFoldedOne(t *testing.T) {
	lane := laneOf("PlacedLane::runB::pack")
	edited, imp, re := swimlaneRoundTrip(t, "PlacedLane::foldedFlowView", lane, shifted(t, 40, 30))
	at := slices.IndexFunc(imp.Edits, func(e view.DrawioEdit) bool {
		return e.Kind == view.DrawioEditLayout && e.FQN == "PlacedLane::station::robot"
	})
	if at < 0 {
		t.Fatalf("edits = %+v, want a Layout about PlacedLane::station::robot", imp.Edits)
	}
	for name, pick := range map[string]func(*view.DrawioFile) *view.DrawioCell{"flow": cellNamed("PlacedLane::runB"), "swimlane": lane, "pack": cellNamed("PlacedLane::runB::pack")} {
		was, again := pick(edited), pick(re)
		if was == nil || again == nil {
			t.Fatalf("%s: in the file %v, re-exported %v, want both", name, was, again)
		}
		if !drawioSameBox(was.Geometry, again.Geometry) {
			t.Errorf("%s was at %+v, re-exported at %+v (edits %+v)", name, was.Geometry, again.Geometry, imp.Edits)
		}
	}
}
