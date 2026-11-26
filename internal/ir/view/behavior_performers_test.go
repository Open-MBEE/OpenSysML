package view

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// laneOf is the swimlane holding the node named name, nil when none does.
func laneOf(t *testing.T, rendering *Rendering, name string) *Node {
	t.Helper()
	target := findNode(t, rendering.Roots, name)
	for _, node := range allNodes(rendering.Roots) {
		if node.Kind == swimlaneKind && slices.Contains(node.Children, target) {
			return node
		}
	}
	return nil
}

// performedByNames are the names a node's `performed by` compartment lists, nil when it has none.
func performedByNames(node *Node) []string {
	var names []string
	for _, compartment := range node.Compartments {
		if compartment.Name != performedByCompartment {
			continue
		}
		for _, row := range compartment.Rows {
			names = append(names, row.Name)
		}
	}
	return names
}

// The flow a perform action usage of a part performs has a lane for each nested
// performer, the unperformed nodes outside (§8.2.3.17 perform-actions-swimlanes).
func TestSwimlanesGroupThePerformedFlowByPerformer(t *testing.T) {
	rendering := render(t, "action-swimlanes.sysml", "SwimLanes::performedView")
	root := rendering.Roots[0]
	if root.Kind != performActionKind {
		t.Errorf("root kind = %q, want %q", root.Kind, performActionKind)
	}
	for action, performer := range map[string]string{
		"action1": "part0.part1", "action4": "part0.part1", "action2": "part0.part2", "action3": "part0.part2",
	} {
		lane := laneOf(t, rendering, action)
		if lane == nil {
			t.Errorf("%s is in no swimlane, want that of %s", action, performer)
			continue
		}
		if lane.Name != performer || lane.Detail != performActionsLabel {
			t.Errorf("%s is in lane %q (%q), want %q (%q)", action, lane.Name, lane.Detail, performer, performActionsLabel)
		}
	}
	var lanes int
	for _, child := range root.Children {
		switch child.Kind {
		case swimlaneKind:
			lanes++
		case "initial", "final":
		default:
			t.Errorf("%s %q is drawn outside the swimlanes", child.Kind, child.Name)
		}
	}
	if lanes != 2 {
		t.Errorf("%d swimlanes, want one for each of the two performers", lanes)
	}
	// The performers declared outside part0 perform another occurrence of the flow.
	if names := performedByNames(findNode(t, rendering.Roots, "action4")); names != nil {
		t.Errorf("action4 lists performers %v from outside the performing part", names)
	}
	assertRenderingEdgeEndpoints(t, rendering)
	if len(rendering.Edges) != 5 {
		t.Errorf("%d successions, want the flow's 5 kept across the lanes", len(rendering.Edges))
	}
}

// An action several elements perform stays out of the lanes and lists them in
// `performed by` (§8.2.3.17 performed-by-compartment), as the flow's own action does.
func TestPerformedByCompartmentListsSeveralPerformers(t *testing.T) {
	rendering := render(t, "action-swimlanes.sysml", "SwimLanes::actionView")
	root := rendering.Roots[0]
	if got, want := performedByNames(root), []string{"part0", "backup"}; !slices.Equal(got, want) {
		t.Errorf("action0 performed by %v, want %v", got, want)
	}
	if lane := laneOf(t, rendering, "action4"); lane != nil {
		t.Errorf("action4, performed twice, is in the lane of %s", lane.Name)
	}
	if got, want := performedByNames(findNode(t, rendering.Roots, "action4")), []string{"part0.part1", "backup.standby"}; !slices.Equal(got, want) {
		t.Errorf("action4 performed by %v, want %v", got, want)
	}
	if lane := laneOf(t, rendering, "action1"); lane == nil || lane.Name != "part0.part1" {
		t.Errorf("action1 lane = %v, want part0.part1", lane)
	}
}

// Lanes are drawn in the flow owning the performed nodes, nested flows included,
// for a definition, an action and an unnamed usage as for a named part.
func TestSwimlanesOfNestedFlowsAndPerformerShapes(t *testing.T) {
	rendering := render(t, "action-swimlanes.sysml", "LaneShapes::assembleView")
	for action, performer := range map[string]string{"cut": "Robot", "weld": "Robot", "inspect": "supervise", "pack": ": Robot"} {
		if lane := laneOf(t, rendering, action); lane == nil || lane.Name != performer {
			t.Errorf("%s lane = %v, want %q", action, lane, performer)
		}
	}
	build := findNode(t, rendering.Roots, "build")
	if lane := laneOf(t, rendering, "build"); lane != nil || performedByNames(build) != nil {
		t.Errorf("build, which nothing performs, is drawn with a performer")
	}
	if cut := laneOf(t, rendering, "cut"); !slices.Contains(build.Children, cut) {
		t.Errorf("the lane of cut is not drawn in build, the flow that owns it")
	}
	if got, want := performedByNames(findNode(t, rendering.Roots, "fetch")), []string{"loader", "bench"}; !slices.Equal(got, want) {
		t.Errorf("fetch performed by %v, want %v", got, want)
	}
}

// A model with no perform action usages draws no lane and no compartment.
func TestNoSwimlaneWithoutPerformers(t *testing.T) {
	rendering := render(t, "action.sysml", "FlowViews::driveView")
	for _, node := range allNodes(rendering.Roots) {
		if node.Kind == swimlaneKind || performedByNames(node) != nil {
			t.Errorf("%s %q is drawn with a performer the model does not state", node.Kind, node.Name)
		}
	}
}

// Every form draws a swimlane dashed, headed «performer», its name and
// `perform actions`, with the nodes it holds nested in it.
func TestSwimlaneInEveryForm(t *testing.T) {
	rendering := render(t, "action-swimlanes.sysml", "SwimLanes::performedView")
	dashed := map[Form]string{FormDot: "style=dashed", FormPlantUML: "<<swimlane>> <<region>>", FormD2: "class: region", FormDrawio: "dashed=1"}
	for _, form := range []Form{FormDot, FormMermaid, FormPlantUML, FormD2, FormDrawio} {
		out, err := rendering.Write(form)
		if err != nil {
			t.Fatalf("write %s: %v", form, err)
		}
		for _, want := range []string{"«performer»", "part0.part1", "part0.part2", performActionsLabel, "«perform action»", dashed[form]} {
			if !strings.Contains(out, want) {
				t.Errorf("%s lacks %q", form, want)
			}
		}
	}
}

// The grid layout attaches the lanes on their vertical edges, aligned along
// their top and bottom edges (§8.2.3.17, the note on the swimlane production).
func TestSwimlanesAbutAndAlign(t *testing.T) {
	rendering := render(t, "action-swimlanes.sysml", "LaneShapes::assembleView")
	a := newArranger(labelsOf(rendering.Roots, false, nil, rendering.Shown), rendering.portView(Options{}.Ports))
	a.layout(rendering)
	root := rendering.Roots[0]
	var lanes []drawioBox
	for _, child := range root.Children {
		if child.Kind == swimlaneKind {
			lanes = append(lanes, a.boxes[child.ID])
		}
	}
	if len(lanes) != 2 {
		t.Fatalf("%d lanes in the root flow, want 2", len(lanes))
	}
	if lanes[0].right() != lanes[1].X {
		t.Errorf("lanes are not attached: the first ends at %v, the second starts at %v", lanes[0].right(), lanes[1].X)
	}
	if lanes[0].Y != lanes[1].Y || lanes[0].Height != lanes[1].Height {
		t.Errorf("lanes are not aligned: %+v and %+v", lanes[0], lanes[1])
	}
	frame := a.boxes[root.ID]
	for _, child := range root.Children {
		box := a.boxes[child.ID]
		if box.X < frame.X || box.Y < frame.Y || box.right() > frame.right() || box.bottom() > frame.bottom() {
			t.Errorf("%s %q at %+v leaves its flow %+v", child.Kind, child.Name, box, frame)
		}
		if child.Kind == swimlaneKind {
			continue
		}
		for _, lane := range lanes {
			if box.X < lane.right() && lane.X < box.right() && box.Y < lane.bottom() && lane.Y < box.bottom() {
				t.Errorf("%s %q at %+v overlaps a lane %+v", child.Kind, child.Name, box, lane)
			}
		}
	}
}

// swimlaneImportCase is the action flow whose nodes two performers' lanes hold.
func swimlaneImportCase(t *testing.T) (c struct {
	name string
	file string
	view string
	kind Kind
}, golden string) {
	t.Helper()
	for _, c = range plantumlGoldenCases {
		if c.name == "action-swimlanes-action" {
			data, err := os.ReadFile(filepath.Join("testdata", c.name+".drawio.golden"))
			if err != nil {
				t.Fatal(err)
			}
			return c, string(data)
		}
	}
	t.Fatal("no action-swimlanes-action golden case")
	return c, ""
}

// TestDrawioImportNeverPlacesThePerformerOfASwimlane: a lane is no element, so
// moving it or a node in it writes no Layout on the performer's declaration.
func TestDrawioImportNeverPlacesThePerformerOfASwimlane(t *testing.T) {
	c, golden := swimlaneImportCase(t)
	for _, moved := range []string{"n1", "n7"} {
		document := mutateCell(t, golden, moved, func(cell string) string {
			return setGeometryAttr(t, cell, "y", "140")
		})
		imp := drawioGoldenImport(t, c, document)
		for _, edit := range imp.Edits {
			if !strings.HasPrefix(edit.FQN, "SwimLanes::action0") {
				t.Errorf("moving %s: edit %s of %q (%+v), want only the flow's own nodes placed", moved, edit.Kind, edit.FQN, edit.Origin)
			}
		}
		if moved == "n1" {
			editOf(t, imp, DrawioEditLayout, "n1")
		}
	}
}

// A swimlane no Layout places is drawn around its nodes: one it holds sits where the
// flow's frame puts it, whatever else the flow places.
func TestSwimlaneIsDrawnAroundThePlacedNodesItHolds(t *testing.T) {
	rendering := render(t, "action-swimlanes-placed.sysml", "PlacedLanes::placedView")
	a := newArranger(labelsOf(rendering.Roots, false, nil, rendering.Shown), rendering.portView(Options{}.Ports))
	a.layout(rendering)
	lane := laneOf(t, rendering, "load")
	if lane == nil {
		t.Fatal("load is in no lane")
	}
	if box := a.boxes[findNode(t, rendering.Roots, "load").ID]; box.X != 300 || box.Y != 200 {
		t.Errorf("load at %+v, want its Layout's 300,200", box)
	}
	if box := a.boxes[findNode(t, rendering.Roots, "ship").ID]; box.X != 60 || box.Y != 150 {
		t.Errorf("ship at %+v, want its Layout's 60,150", box)
	}
	frame := a.boxes[lane.ID]
	for _, held := range lane.Children {
		box := a.boxes[held.ID]
		if box.X <= frame.X || box.Y <= frame.Y || box.right() >= frame.right() || box.bottom() >= frame.bottom() {
			t.Errorf("%q at %+v is not inside its lane %+v", held.Name, box, frame)
		}
	}
}

// A Layout about a performer in the view places the swimlane drawn for it, the nodes it holds inside.
func TestSwimlaneIsPlacedByTheLayoutOfItsPerformer(t *testing.T) {
	r, idx := loadFixtures(t, "action-swimlane-layout.sysml")
	viewSym := lookup(t, idx, "PlacedLane::laneView")
	rendering, err := r.Render(viewSym)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	lane := laneOf(t, rendering, "load")
	if lane == nil {
		t.Fatal("load is in no lane")
	}
	if g := lane.Geometry; !g.placed() || g.X != 320 || g.Y != 120 || !g.HasSize {
		t.Fatalf("lane geometry %+v, want the Layout about line::robot", g)
	}
	a := newArranger(labelsOf(rendering.Roots, false, nil, rendering.Shown), rendering.portView(Options{}.Ports))
	a.layout(rendering)
	frame := a.boxes[lane.ID]
	if frame.X != 320 || frame.Y != 120 || frame.Width != 220 || frame.Height != 180 {
		t.Errorf("lane at %+v, want its Layout's 320,120 220x180", frame)
	}
	if box := a.boxes[findNode(t, rendering.Roots, "load").ID]; box.X <= frame.X || box.Y <= frame.Y || box.right() >= frame.right() || box.bottom() >= frame.bottom() {
		t.Errorf("load at %+v is not inside its lane %+v", box, frame)
	}
	drawn, err := r.DrawnIn(viewSym)
	if err != nil {
		t.Fatalf("DrawnIn: %v", err)
	}
	if !drawn.Node(lookup(t, idx, "PlacedLane::line::robot")) {
		t.Error("the view does not draw the performer as a node a Layout positions")
	}
}

// A placed lane the unplaced flow's frame moves under its header takes the nodes placed in it along.
func TestPlacedSwimlaneKeepsItsPlacedNodesWhereTheFrameMovesIt(t *testing.T) {
	rendering := render(t, "action-swimlane-layout.sysml", "PlacedLane::heldView")
	lane := laneOf(t, rendering, "load")
	if lane == nil {
		t.Fatal("load is in no lane")
	}
	a := newArranger(labelsOf(rendering.Roots, false, nil, rendering.Shown), rendering.portView(Options{}.Ports))
	a.layout(rendering)
	frame, box := a.boxes[lane.ID], a.boxes[findNode(t, rendering.Roots, "load").ID]
	if dx, dy := box.X-frame.X, box.Y-frame.Y; dx != 30 || dy != 70 {
		t.Errorf("load at %+v in its lane at %+v, want the stated offset of 30,70", box, frame)
	}
}

// One Layout about a performer places one lane, the outermost: a lane in a nested flow is drawn around its nodes.
func TestSwimlaneLayoutPlacesTheOutermostLaneOfItsPerformer(t *testing.T) {
	rendering := render(t, "action-swimlane-layout.sysml", "PlacedLane::nestedView")
	outer, inner := laneOf(t, rendering, "fetch"), laneOf(t, rendering, "cut")
	if outer == nil || inner == nil {
		t.Fatalf("lanes of fetch and cut: %v, %v, want both", outer, inner)
	}
	if g := outer.Geometry; !g.placed() || g.X != 320 || g.Y != 120 {
		t.Errorf("outer lane geometry %+v, want the Layout about arm", g)
	}
	if inner.Geometry.placed() {
		t.Errorf("nested lane geometry %+v, want none", inner.Geometry)
	}
}

// The nodes of a performed flow are the ones the perform action usage inherits,
// so a Layout naming one through that usage places it.
func TestPerformedFlowNodesArePlacedThroughThePerformActionUsage(t *testing.T) {
	rendering := render(t, "action-swimlanes-placed.sysml", "PlacedLanes::performedView")
	load := findNode(t, rendering.Roots, "load")
	if !load.Inherited {
		t.Error("load is not drawn as inherited by the perform action usage")
	}
	if g := load.Geometry; !g.placed() || g.X != 300 || g.Y != 200 {
		t.Errorf("load geometry %+v, want the Layout about line::run::load", g)
	}
	if len(rendering.Notices) != 0 {
		t.Errorf("notices %q, want none", rendering.Notices)
	}
}
