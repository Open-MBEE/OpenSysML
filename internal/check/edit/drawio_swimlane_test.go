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
	return swimlaneRoundTripOf(t, string(content), viewName, pick, edit)
}

// swimlaneRoundTripOf is swimlaneRoundTrip over the model content declares.
func swimlaneRoundTripOf(t *testing.T, content, viewName string, pick func(*view.DrawioFile) *view.DrawioCell, edit func(document, id string) string) (*view.DrawioFile, *view.DrawioImport, *view.DrawioFile) {
	t.Helper()
	m, rendering, options := drawioModel(t, content, viewName)
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
// named by the flow it is drawn in, which is pinned with it when no Layout places that flow.
func TestDrawioMovedPlacedSwimlaneBesideANestedLane(t *testing.T) {
	lane := func(f *view.DrawioFile) *view.DrawioCell {
		return drawioFirst(f, func(c *view.DrawioCell) bool { return c.Vertex && strings.Contains(c.Label, "arm") })
	}
	edited, imp, re := swimlaneRoundTrip(t, "PlacedLane::nestedView", lane, shifted(t, 40, 30))
	about := slices.IndexFunc(imp.Edits, func(e view.DrawioEdit) bool { return e.Kind == view.DrawioEditLayout && e.FQN == "PlacedLane::arm" })
	if about < 0 || !slices.Equal(imp.Edits[about].Within, []string{"PlacedLane::assemble"}) {
		t.Fatalf("edits = %+v, want a Layout about PlacedLane::arm within PlacedLane::assemble", imp.Edits)
	}
	if layout := imp.Edits[about].Layout; layout.X != 360 || layout.Y != 150 {
		t.Errorf("layout = %+v, want 360,150", layout)
	}
	if was, again := lane(edited), lane(re); again == nil || !drawioSameBox(was.Geometry, again.Geometry) {
		t.Errorf("swimlane was at %+v, re-exported at %+v", was.Geometry, again)
	}
}

const unnamedHolderSwimlaneModel = `package K {
	private import DiagramLayout::*;
	part def Robot;
	action run {
		action build { action cut; }
		then action pack { action wrap; }
	}
	part arm { perform run.build.cut; perform run.pack.wrap; }
	part : Robot { perform run.build; perform run.pack; }
	view v : StandardViewDefinitions::ActionFlowView {
		expose run;
		metadata Layout about arm { x = 320; y = 120; width = 220; height = 180; }
	}
}
`

// The placed lane drawn inside the lane of a performer with no name, moved in the file, is
// still named by the nodes it is drawn in.
func TestDrawioMovedPlacedSwimlaneInsideAnUnnamedPerformersLane(t *testing.T) {
	lane := func(f *view.DrawioFile) *view.DrawioCell {
		return drawioFirst(f, func(c *view.DrawioCell) bool { return c.Vertex && strings.Contains(c.Label, "arm") })
	}
	edited, imp, re := swimlaneRoundTripOf(t, unnamedHolderSwimlaneModel, "K::v", lane, shifted(t, 40, 30))
	about := slices.IndexFunc(imp.Edits, func(e view.DrawioEdit) bool { return e.Kind == view.DrawioEditLayout && e.FQN == "K::arm" })
	if about < 0 || !slices.Equal(imp.Edits[about].Within, []string{"K::run", "", "K::run::build"}) {
		t.Fatalf("edits = %+v, want a Layout about K::arm within K::run, a node with no name, K::run::build", imp.Edits)
	}
	if was, again := lane(edited), lane(re); again == nil || !drawioSameBox(was.Geometry, again.Geometry) {
		t.Errorf("swimlane was at %+v, re-exported at %+v", was.Geometry, again)
	}
}

const twoFlowLaneModel = `package T {
	private import DiagramLayout::*;
	action runA { action load; then action ship; }
	action runB { action pack; then action post; }
	part line {
		part robot { perform runA.load; }
		part packer { perform runB.pack; }
	}
	view both : StandardViewDefinitions::ActionFlowView {
		expose runA;
		expose runB;
		metadata Layout about line::robot { x = 320; y = 120; width = 220; height = 180; }
		metadata Layout about line::packer { x = 700; y = 120; width = 220; height = 180; }
	}
	view one : StandardViewDefinitions::ActionFlowView {
		expose runA;
		expose runB;
		metadata Layout about line::robot { x = 320; y = 120; width = 220; height = 180; }
	}
	view free : StandardViewDefinitions::ActionFlowView {
		expose runA;
		expose T::idle;
		metadata Layout about line::robot { x = 320; y = 120; width = 220; height = 180; }
	}
	view bare : StandardViewDefinitions::ActionFlowView {
		expose runA;
		expose runB;
	}
	action idle { action rest; then action wake; }
}
`

// A lane moved in the file of a view placing no node places every flow the file shows: the
// re-export leaves no other flow undrawn, with or without a Layout about a performer.
func TestDrawioMovedSwimlaneKeepsTheOtherFlowsOfAViewPlacingNoNode(t *testing.T) {
	count := func(f *view.DrawioFile) (vertices, edges int) {
		for i := range f.Cells {
			switch c := &f.Cells[i]; {
			case c.Vertex && !c.Relative:
				vertices++
			case c.Edge:
				edges++
			}
		}
		return vertices, edges
	}
	for _, name := range []string{"T::both", "T::one", "T::free", "T::bare"} {
		t.Run(name, func(t *testing.T) {
			edited, _, re := swimlaneRoundTripOf(t, twoFlowLaneModel, name, laneOf("T::runA::load"), shifted(t, 40, 30))
			wantV, wantE := count(edited)
			if v, e := count(re); v != wantV || e != wantE {
				t.Errorf("re-exported %d vertices and %d edges, the file had %d and %d", v, e, wantV, wantE)
			}
			for _, fqn := range []string{"T::runA::ship", "T::runB::post", "T::idle::wake"} {
				was, again := cellNamed(fqn)(edited), cellNamed(fqn)(re)
				if was != nil && (again == nil || !drawioSameBox(was.Geometry, again.Geometry)) {
					t.Errorf("%s was at %+v, re-exported at %+v", fqn, was.Geometry, again)
				}
			}
		})
	}
}

const unnamedStepLaneModel = `package U {
	action runA { action load; then action ship; }
	action runB { action pack; then action; then action post; }
	part robot { perform runA.load; }
	view both : StandardViewDefinitions::ActionFlowView {
		expose runA;
		expose runB;
	}
	view other : StandardViewDefinitions::ActionFlowView {
		expose runB;
	}
}
`

// A lane moved in a view placing no node writes nothing on the declaration of a step the
// file left alone: a step with no name in another flow stays unplaced, and so does every
// other view drawing it.
func TestDrawioMovedSwimlaneWritesNoLayoutOnAnUntouchedUnnamedStep(t *testing.T) {
	m, rendering, options := drawioModel(t, unnamedStepLaneModel, "U::both")
	document, exported := drawioExport(t, rendering, options)
	lane := laneOf("U::runA::load")(exported)
	if lane == nil {
		t.Fatal("the export has no swimlane")
	}
	_, imp := drawioImport(t, drawioImporter(t, rendering, options), rendering.View, shifted(t, 40, 30)(document, lane.ID))
	for _, e := range imp.Edits {
		if e.Kind == view.DrawioEditLayout && e.FQN == "" {
			t.Errorf("a Layout is written on a declaration no name reaches: %+v", e)
		}
	}
	res, err := Apply(m, DrawioOperations(imp, false))
	if err != nil {
		t.Fatalf("apply %+v: %v", imp.Edits, err)
	}
	count := func(content string) (vertices, edges int) {
		_, r, o := drawioModel(t, content, "U::other")
		_, f := drawioExport(t, r, o)
		for i := range f.Cells {
			switch c := &f.Cells[i]; {
			case c.Vertex && !c.Relative:
				vertices++
			case c.Edge:
				edges++
			}
		}
		return vertices, edges
	}
	wantV, wantE := count(unnamedStepLaneModel)
	if v, e := count(string(res.Content)); v != wantV || e != wantE {
		t.Errorf("U::other draws %d vertices and %d edges after the import into U::both, it drew %d and %d", v, e, wantV, wantE)
	}
}

// A node moved in the file of a view whose only Layouts are about performers places every
// flow the file shows: the re-export leaves none undrawn and each placed lane where it was.
func TestDrawioMovedNodeKeepsTheFlowsAndLanesOfAViewPlacingSwimlanesAlone(t *testing.T) {
	count := func(f *view.DrawioFile) (vertices, edges int) {
		for i := range f.Cells {
			switch c := &f.Cells[i]; {
			case c.Vertex && !c.Relative:
				vertices++
			case c.Edge:
				edges++
			}
		}
		return vertices, edges
	}
	for _, c := range []struct{ view, moved string }{
		{"T::both", "T::runB::pack"}, {"T::both", "T::runA::ship"},
		{"T::one", "T::runA::load"}, {"T::one", "T::runB::post"},
		{"T::free", "T::idle::rest"}, {"T::free", "T::runA::load"},
	} {
		t.Run(c.view+" "+c.moved, func(t *testing.T) {
			edited, imp, re := swimlaneRoundTripOf(t, twoFlowLaneModel, c.view, cellNamed(c.moved), shifted(t, 40, 30))
			wantV, wantE := count(edited)
			if v, e := count(re); v != wantV || e != wantE {
				t.Errorf("re-exported %d vertices and %d edges, the file had %d and %d (edits %+v)", v, e, wantV, wantE, imp.Edits)
			}
			lane := laneOf("T::runA::load")
			if was, again := lane(edited), lane(re); was == nil || again == nil || was.Geometry.X != again.Geometry.X || was.Geometry.Y != again.Geometry.Y {
				t.Errorf("the lane of line.robot was at %+v, re-exported at %+v", was, again)
			}
			if was, again := cellNamed(c.moved)(edited), cellNamed(c.moved)(re); again == nil || !drawioSameBox(was.Geometry, again.Geometry) {
				t.Errorf("%s was at %+v, re-exported at %+v", c.moved, was.Geometry, again)
			}
		})
	}
}

const twiceShownStepModel = `package D {
	private import DiagramLayout::*;
	action runA { action load; then action weld; }
	action runB { action pack; then action ship; }
	part robot { perform runA.load; }
	view twice : StandardViewDefinitions::ActionFlowView {
		expose runA;
		expose runB;
		expose runA::weld;
		metadata Layout about robot { x = 320; y = 120; }
	}
}`

// A step the view shows twice, in its flow and on its own, has one Layout: placing the flows
// around a moved copy keeps the place the file moved it to.
func TestDrawioMovedNodeShownTwiceKeepsItsMoveBesideAPlacedSwimlane(t *testing.T) {
	const weld = "D::runA::weld"
	alone := func(f *view.DrawioFile) *view.DrawioCell {
		return drawioFirst(f, func(c *view.DrawioCell) bool {
			parent := f.Cell(c.Parent)
			return c.FQN == weld && (parent == nil || parent.FQN == "")
		})
	}
	edited, imp, _ := swimlaneRoundTripOf(t, twiceShownStepModel, "D::twice", alone, shifted(t, 40, 30))
	moved := alone(edited)
	var layouts []view.DrawioEdit
	for _, e := range imp.Edits {
		if e.Kind == view.DrawioEditLayout && e.FQN == weld {
			layouts = append(layouts, e)
		}
	}
	if len(layouts) != 1 || layouts[0].Layout.X != moved.Geometry.X || layouts[0].Layout.Y != moved.Geometry.Y {
		t.Errorf("the import writes %+v about %s, the file moved it to %+v", layouts, weld, moved.Geometry)
	}
}

const unnamedStepOwnFlowModel = `package G {
	private import DiagramLayout::*;
	action runA { action load; then action; then action weld; }
	action runB { action pack; then action ship; }
	part robot { perform runA.load; }
	part packer { perform runB.pack; }
	view both : StandardViewDefinitions::ActionFlowView {
		expose runA;
		expose runB;
		metadata Layout about robot { x = 320; y = 120; }
	}
	view onlyA : StandardViewDefinitions::ActionFlowView {
		expose runA;
	}
}
`

// A move in the flow holding a step with no name writes nothing on that step's declaration,
// whichever of the flow's cells the file moved: the import says so, and a view drawing the
// flow alone draws it as before.
func TestDrawioMoveInAFlowWritesNoLayoutOnItsUntouchedUnnamedStep(t *testing.T) {
	count := func(content string) (vertices, edges int) {
		_, r, o := drawioModel(t, content, "G::onlyA")
		_, f := drawioExport(t, r, o)
		for i := range f.Cells {
			switch c := &f.Cells[i]; {
			case c.Vertex && !c.Relative:
				vertices++
			case c.Edge:
				edges++
			}
		}
		return vertices, edges
	}
	wantV, wantE := count(unnamedStepOwnFlowModel)
	for name, pick := range map[string]func(*view.DrawioFile) *view.DrawioCell{
		"lane":  laneOf("G::runA::load"),
		"node":  cellNamed("G::runA::load"),
		"step":  cellNamed("G::runA::weld"),
		"frame": cellNamed("G::runA"),
	} {
		t.Run(name, func(t *testing.T) {
			m, rendering, options := drawioModel(t, unnamedStepOwnFlowModel, "G::both")
			document, exported := drawioExport(t, rendering, options)
			cell := pick(exported)
			if cell == nil {
				t.Fatal("the export has no cell to move")
			}
			_, imp := drawioImport(t, drawioImporter(t, rendering, options), rendering.View, shifted(t, 40, 30)(document, cell.ID))
			for _, e := range imp.Edits {
				if e.Kind == view.DrawioEditLayout && e.FQN == "" {
					t.Errorf("a Layout is written on a declaration no name reaches: %+v", e)
				}
			}
			if !slices.ContainsFunc(imp.Notices, func(n string) bool { return strings.Contains(n, "G::runA: holds a node with no name") }) {
				t.Errorf("the import does not say it left the unnamed step unplaced: %q", imp.Notices)
			}
			res, err := Apply(m, DrawioOperations(imp, false))
			if err != nil {
				t.Fatalf("apply %+v: %v", imp.Edits, err)
			}
			if v, e := count(string(res.Content)); v != wantV || e != wantE {
				t.Errorf("G::onlyA draws %d vertices and %d edges after the import into G::both, it drew %d and %d", v, e, wantV, wantE)
			}
		})
	}
}
