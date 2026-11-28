package view

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
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

// One Layout about a performer places one lane in the whole view: its lane in another exposed flow is drawn around its nodes.
func TestSwimlaneLayoutPlacesOneLaneAcrossTheFlowsOfAView(t *testing.T) {
	rendering := render(t, "action-swimlane-layout.sysml", "PlacedLane::sharedView")
	first, second := laneOf(t, rendering, "load"), laneOf(t, rendering, "pack")
	if first == nil || second == nil {
		t.Fatalf("lanes of load and pack: %v, %v, want both", first, second)
	}
	if g := first.Geometry; !g.placed() || g.X != 320 || g.Y != 120 {
		t.Errorf("first flow's lane geometry %+v, want the Layout about station::robot", g)
	}
	if second.Geometry.placed() {
		t.Errorf("second flow's lane geometry %+v, want none", second.Geometry)
	}
}

// Folding the flow of the placed lane leaves the Layout with it: the lane of the next flow stays drawn around its nodes.
func TestSwimlaneLayoutStaysWithItsFoldedLane(t *testing.T) {
	rendering := render(t, "action-swimlane-layout.sysml", "PlacedLane::foldedFlowView")
	for _, node := range allNodes(rendering.Roots) {
		if node.Name == "load" {
			t.Fatal("load is drawn in its folded flow")
		}
	}
	lane := laneOf(t, rendering, "pack")
	if lane == nil {
		t.Fatal("pack is in no lane")
	}
	if lane.Geometry != nil || !lane.Fitted {
		t.Errorf("lane geometry %+v fitted %v, want the lane fitted around pack", lane.Geometry, lane.Fitted)
	}
}

// Folding one flow changes nothing of the lane of another: which lane the Layout places does not depend on what is folded.
func TestSwimlaneLayoutIsTheSameLanesWhateverIsFolded(t *testing.T) {
	lanes := func(view string) map[string]Node {
		out := map[string]Node{}
		for _, flow := range render(t, "action-swimlane-layout.sysml", view).Roots {
			for _, child := range flow.Children {
				if child.Kind == swimlaneKind {
					out[flow.Name] = Node{Geometry: child.Geometry, Fitted: child.Fitted}
				}
			}
		}
		return out
	}
	open := lanes("PlacedLane::pinnedView")
	if len(open) != 2 || open["PlacedLane::runA"].Geometry == nil || !open["PlacedLane::runB"].Fitted {
		t.Fatalf("lanes of pinnedView %+v, want the lane of runA placed and that of runB fitted", open)
	}
	for view, kept := range map[string]string{"PlacedLane::pinnedFirstFoldedView": "PlacedLane::runB", "PlacedLane::pinnedSecondFoldedView": "PlacedLane::runA"} {
		folded := lanes(view)
		if len(folded) != 1 {
			t.Fatalf("%s draws lanes %+v, want that of %s alone", view, folded, kept)
		}
		was, now := open[kept], folded[kept]
		if now.Fitted != was.Fitted || (now.Geometry == nil) != (was.Geometry == nil) || (was.Geometry != nil && *now.Geometry != *was.Geometry) {
			t.Errorf("%s: lane of %s is %+v fitted %v, unfolded %+v fitted %v", view, kept, now.Geometry, now.Fitted, was.Geometry, was.Fitted)
		}
	}
}

// A Layout about a performer that only folds is one lane's too: its lane in another flow keeps its nodes.
func TestSwimlaneLayoutFoldsOneLane(t *testing.T) {
	rendering := render(t, "action-swimlane-layout.sysml", "PlacedLane::foldedLaneView")
	var lanes []*Node
	for _, node := range allNodes(rendering.Roots) {
		if node.Kind == swimlaneKind {
			lanes = append(lanes, node)
		}
	}
	if len(lanes) != 2 || !collapsed(lanes[0]) || len(lanes[0].Children) != 0 {
		t.Fatalf("lanes %+v, want two, the first folded", lanes)
	}
	if lanes[1].Geometry != nil || laneOf(t, rendering, "pack") != lanes[1] {
		t.Errorf("second lane geometry %+v children %v, want no Layout and pack held", lanes[1].Geometry, lanes[1].Children)
	}
}

// A rendering of an action outside any view reads the performer's own Layout, and places one lane by it as a view does.
func TestSwimlaneLayoutPlacesOneLaneOutsideAView(t *testing.T) {
	r, idx := loadFixtures(t, "action-swimlane-layout.sysml")
	for _, kind := range []Kind{KindAction, KindMixed} {
		rendering, err := r.RenderExposed([]*symbols.Symbol{lookup(t, idx, "PlacedLane::weld")}, kind, "#"+string(kind))
		if err != nil {
			t.Fatalf("render %s: %v", kind, err)
		}
		outer, inner := laneOf(t, rendering, "tack"), laneOf(t, rendering, "bead")
		if outer == nil || inner == nil {
			t.Fatalf("%s: lanes of tack and bead: %v, %v, want both", kind, outer, inner)
		}
		if g := outer.Geometry; !g.placed() || g.X != 320 || g.Y != 120 {
			t.Errorf("%s: outer lane geometry %+v, want the Layout of torch", kind, g)
		}
		if inner.Geometry != nil || !inner.Fitted || outer.Fitted {
			t.Errorf("%s: nested lane geometry %+v fitted %v, outer fitted %v, want the nested lane alone fitted", kind, inner.Geometry, inner.Fitted, outer.Fitted)
		}
	}
}

// A Layout about a performer alone places its swimlane and pins no node: every
// form still draws the whole flow, as it does for a view with no Layout.
func TestSwimlanePlacedAloneLeavesNoNodeUndrawn(t *testing.T) {
	for _, view := range []string{"LaneAlone::cornerView", "LaneAlone::sizedView"} {
		rendering := render(t, "action-swimlane-layout-alone.sysml", view)
		if lane := laneOf(t, rendering, "load"); lane == nil || !lane.Geometry.placed() {
			t.Fatalf("%s: the lane of load is not placed by the Layout about line::robot", view)
		}
		if p := placeRendering(rendering); p.positioned() {
			t.Errorf("%s: %d of %d nodes pinned, want none by a swimlane alone", view, p.count, p.nodes)
		}
		for _, form := range []Form{FormDot, FormMermaid, FormPlantUML, FormD2, FormDrawio} {
			out, err := rendering.Write(form)
			if err != nil {
				t.Fatalf("%s: write %s: %v", view, form, err)
			}
			if strings.Contains(out, "left undrawn") {
				t.Errorf("%s: %s leaves nodes undrawn", view, form)
			}
			for _, want := range []string{"load", "weld", "ship", "line.robot", "line.welder"} {
				if !strings.Contains(out, want) {
					t.Errorf("%s: %s lacks %q", view, form, want)
				}
			}
		}
	}
}

// The draw.io form draws a swimlane placed alone where its Layout says, at the stated size
// when it states one, with the flow no Layout places drawn around it.
func TestSwimlanePlacedAloneKeepsItsStatedPlaceInDrawio(t *testing.T) {
	for view, sized := range map[string]bool{"LaneAlone::cornerView": false, "LaneAlone::sizedView": true} {
		rendering := render(t, "action-swimlane-layout-alone.sysml", view)
		out, err := rendering.Write(FormDrawio)
		if err != nil {
			t.Fatalf("%s: write drawio: %v", view, err)
		}
		file, err := ReadDrawio([]byte(out), view)
		if err != nil {
			t.Fatalf("%s: read drawio: %v", view, err)
		}
		var lane *DrawioCell
		edges := 0
		for i := range file.Cells {
			switch cell := &file.Cells[i]; {
			case cell.Vertex && strings.Contains(cell.Label, "line.robot"):
				lane = cell
			case cell.Edge:
				edges++
			}
		}
		if lane == nil {
			t.Fatalf("%s: no swimlane of line.robot:\n%s", view, out)
		}
		if box := lane.Geometry; box.X != 320 || box.Y != 120 {
			t.Errorf("%s: lane drawn at %v,%v, its Layout says 320,120", view, box.X, box.Y)
		}
		if box := lane.Geometry; sized && (box.Width != 220 || box.Height != 180) {
			t.Errorf("%s: lane drawn %vx%v, its Layout says 220x180", view, box.Width, box.Height)
		}
		if edges < 2 {
			t.Errorf("%s: %d edges drawn, want the two successions of the flow", view, edges)
		}
	}
}

// Two flows whose swimlanes are placed one under the other are drawn apart: the nodes of
// the upper flow no Layout places sit beside its lane, not under it where the lower flow is.
func TestSwimlanesPlacedAloneKeepTheirFlowsApartInDrawio(t *testing.T) {
	overlap := func(a, b DrawioBox) bool {
		return a.X < b.X+b.Width && b.X < a.X+a.Width &&
			a.Y < b.Y+b.Height && b.Y < a.Y+a.Height
	}
	// apart is whether the lanes leave the second flow's header room under the first lane.
	for _, c := range []struct {
		view  string
		y     float64
		apart bool
	}{{"LaneStacked::stackedView", 420, true}, {"LaneStacked::closeView", 330, false}} {
		t.Run(c.view, func(t *testing.T) {
			out, err := render(t, "action-swimlane-layout-stacked.sysml", c.view).Write(FormDrawio)
			if err != nil {
				t.Fatalf("write drawio: %v", err)
			}
			file, err := ReadDrawio([]byte(out), c.view)
			if err != nil {
				t.Fatalf("read drawio: %v", err)
			}
			named := func(fqn string) DrawioBox {
				for i := range file.Cells {
					if file.Cells[i].FQN == fqn {
						return file.Cells[i].Geometry
					}
				}
				t.Fatalf("no cell of %s:\n%s", fqn, out)
				return DrawioBox{}
			}
			lanes := map[string]DrawioBox{}
			for i := range file.Cells {
				for _, performer := range []string{"line.robot", "line.packer"} {
					if cell := &file.Cells[i]; cell.Vertex && strings.Contains(cell.Label, performer) {
						lanes[performer] = cell.Geometry
					}
				}
			}
			if at := lanes["line.robot"]; at.X != 320 || at.Y != 120 {
				t.Errorf("lane of line.robot drawn at %v,%v, its Layout says 320,120", at.X, at.Y)
			}
			if at := lanes["line.packer"]; at.X != 320 || at.Y != c.y {
				t.Errorf("lane of line.packer drawn at %v,%v, its Layout says 320,%v", at.X, at.Y, c.y)
			}
			// Stated nearer than the lower flow's header is tall, the lanes cannot both be kept clear: the file says so.
			const crowded = "swimlane line.packer is placed 48 px below swimlane line.robot, in another flow: the frame of LaneStacked::runB needs 92 px there and is drawn over swimlane line.robot (142×44 px), its step load (64×28 px), the frame of LaneStacked::runA (174×60 px)"
			if said := strings.Contains(out, crowded); said == c.apart {
				t.Errorf("notice of a frame drawn over a swimlane: %v, want %v:\n%s", said, !c.apart, out)
			}
			if c.apart && strings.Contains(out, "is drawn over") {
				t.Errorf("lanes drawn clear of each other are noticed as covered:\n%s", out)
			}
			runA, runB := named("LaneStacked::runA"), named("LaneStacked::runB")
			if c.apart && overlap(runA, runB) {
				t.Errorf("flow runA at %+v overlaps flow runB at %+v", runA, runB)
			}
			for _, step := range []string{"weld", "grind", "paint"} {
				box := named("LaneStacked::runA::" + step)
				if overlap(box, lanes["line.packer"]) || overlap(box, runB) {
					t.Errorf("%s at %+v is covered by flow runB at %+v, its lane at %+v", step, box, runB, lanes["line.packer"])
				}
			}
		})
	}
}
