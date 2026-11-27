package edit

import (
	"os"
	"path/filepath"
	"regexp"
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

// foldedIn sets the fold of the cell id of the document.
func foldedIn(t *testing.T, collapsed string) func(document, id string) string {
	return func(document, id string) string {
		return drawioMutate(t, document, id, func(cell string) string {
			if strings.Contains(cell, ` collapsed="`) {
				return regexp.MustCompile(` collapsed="[01]"`).ReplaceAllString(cell, ` collapsed="`+collapsed+`"`)
			}
			return strings.Replace(cell, "<mxCell ", `<mxCell collapsed="`+collapsed+`" `, 1)
		})
	}
}

// laneIn picks the swimlane drawn in the frame of the flow of a qualified name.
func laneIn(flow string) func(*view.DrawioFile) *view.DrawioCell {
	return func(f *view.DrawioFile) *view.DrawioCell {
		frame := cellNamed(flow)(f)
		return drawioFirst(f, func(c *view.DrawioCell) bool {
			return frame != nil && c.Vertex && c.Parent == frame.ID && strings.Contains(c.Label, "«performer»")
		})
	}
}

// Folding a flow in the file moves no lane of another flow: the Layout about the performer
// stays with its first lane, drawn or folded away.
func TestDrawioFoldedFlowLeavesTheSwimlaneOfAnother(t *testing.T) {
	for _, c := range []struct {
		name, view, flow, other, step string
		alone                         bool // the fold is the import's one edit: nothing else needs pinning
	}{
		{"the placed lane's flow", "PlacedLane::sharedView", "PlacedLane::runA", "PlacedLane::runB", "PlacedLane::runB::pack", false},
		{"the other flow", "PlacedLane::sharedView", "PlacedLane::runB", "PlacedLane::runA", "PlacedLane::runA::load", false},
		{"the placed lane's flow, both flows placed", "PlacedLane::pinnedView", "PlacedLane::runA", "PlacedLane::runB", "PlacedLane::runB::pack", true},
		{"the other flow, both flows placed", "PlacedLane::pinnedView", "PlacedLane::runB", "PlacedLane::runA", "", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			edited, imp, re := swimlaneRoundTrip(t, c.view, cellNamed(c.flow), foldedIn(t, "1"))
			if !slices.ContainsFunc(imp.Edits, func(e view.DrawioEdit) bool { return e.FQN == c.flow && e.Layout != nil && e.Layout.Collapsed }) {
				t.Fatalf("edits = %+v, want the fold of %s", imp.Edits, c.flow)
			}
			if c.alone && len(imp.Edits) != 1 {
				t.Errorf("edits = %+v, want the fold of %s alone", imp.Edits, c.flow)
			}
			if folded := cellNamed(c.flow)(re); folded == nil || !folded.Collapsed {
				t.Errorf("%s re-exported as %+v, want it folded", c.flow, folded)
			}
			// A flow no Layout places is arranged anew beside the folded one, as with no lane at all: its lane and step keep their place in it.
			wasFlow, flow := cellNamed(c.other)(edited), cellNamed(c.other)(re)
			if wasFlow == nil || flow == nil {
				t.Fatalf("%s: in the file %v, re-exported %v, want both", c.other, wasFlow, flow)
			}
			if c.alone && !drawioSameBox(wasFlow.Geometry, flow.Geometry) {
				t.Errorf("%s was at %+v, re-exported at %+v", c.other, wasFlow.Geometry, flow.Geometry)
			}
			picks := map[string]func(*view.DrawioFile) *view.DrawioCell{"swimlane": laneIn(c.other)}
			if c.step != "" {
				picks["step"] = cellNamed(c.step)
			}
			for name, pick := range picks {
				was, again := pick(edited), pick(re)
				if was == nil || again == nil {
					t.Fatalf("%s of %s: in the file %v, re-exported %v, want both", name, c.other, was, again)
				}
				in := again.Geometry
				in.X, in.Y = in.X-flow.Geometry.X+wasFlow.Geometry.X, in.Y-flow.Geometry.Y+wasFlow.Geometry.Y
				if !drawioSameBox(was.Geometry, in) || was.Parent != again.Parent {
					t.Errorf("%s of %s was at %+v in %s, re-exported at %+v in %s, its flow from %+v to %+v", name, c.other, was.Geometry, was.Parent, again.Geometry, again.Parent, wasFlow.Geometry, flow.Geometry)
				}
			}
			if c.step != "" && cellNamed(c.step)(re).Parent != laneIn(c.other)(re).ID {
				t.Errorf("%s re-exported outside its swimlane", c.step)
			}
		})
	}
}

// Beside the folded flow of the placed lane, a moved lane is one no Layout places: no Layout about the performer is written.
func TestDrawioMovedSwimlaneBesideItsFoldedPlacedLane(t *testing.T) {
	_, imp, _ := swimlaneRoundTrip(t, "PlacedLane::foldedFlowView", laneOf("PlacedLane::runB::pack"), shifted(t, 40, 30))
	for _, e := range imp.Edits {
		if e.FQN == "PlacedLane::station::robot" {
			t.Errorf("edits = %+v, want none about PlacedLane::station::robot", imp.Edits)
		}
	}
}

// The placed lane of a performer with another lane in a nested flow, moved in the file, is
// named by the flow it is drawn in, whether or not a Layout places that flow.
func TestDrawioMovedPlacedSwimlaneBesideANestedLane(t *testing.T) {
	lane := func(f *view.DrawioFile) *view.DrawioCell {
		return drawioFirst(f, func(c *view.DrawioCell) bool { return c.Vertex && strings.Contains(c.Label, "arm") })
	}
	edited, imp, re := swimlaneRoundTrip(t, "PlacedLane::nestedView", lane, shifted(t, 40, 30))
	if len(imp.Edits) != 1 || imp.Edits[0].Kind != view.DrawioEditLayout || imp.Edits[0].FQN != "PlacedLane::arm" {
		t.Fatalf("edits = %+v, want one Layout about PlacedLane::arm", imp.Edits)
	}
	if layout := imp.Edits[0].Layout; layout.X != 360 || layout.Y != 150 {
		t.Errorf("layout = %+v, want 360,150", layout)
	}
	if was, again := lane(edited), lane(re); again == nil || !drawioSameBox(was.Geometry, again.Geometry) {
		t.Errorf("swimlane was at %+v, re-exported at %+v", was.Geometry, again)
	}
}
