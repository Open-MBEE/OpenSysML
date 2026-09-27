package view

import (
	"fmt"
	"strings"
	"testing"
)

// standInDOT is the Cameo DOT form of one of the positioned action views of
// standin.sysml, under options.
func standInDOT(t *testing.T, view string, options Options) (*Rendering, string) {
	t.Helper()
	rendering := render(t, "standin.sysml", view)
	options.Style = StyleCameo
	source, err := rendering.DOTWith(options)
	if err != nil {
		t.Fatalf("DOTWith: %v", err)
	}
	checkDOTSyntax(t, source)
	return rendering, source
}

// A join the migration marks as standing for no source node, which no diagram
// positions, stays in the rendering but is elided from a positioned drawing
// that leaves unplaced nodes undrawn: the edge into it and the edge out of it
// are drawn as one along their routes, and a pair without a route is left
// undrawn. Where only the route out of the join is given, the joined edge
// starts on its source's border rather than at the join's old position.
func TestDOTElidesMigrationStandIns(t *testing.T) {
	rendering, source := standInDOT(t, "StandInViews::threadedView", Options{})
	wait := findStandIn(rendering.Roots[0].Children)
	if wait == nil || wait.Name != "wait" || wait.ID != "n4" {
		t.Fatalf("the rendering does not keep the stand-in join: %+v", wait)
	}
	for _, node := range allNodes(rendering.Roots) {
		if node.Kind == "metadata" {
			t.Errorf("marker drawn as node %q : %q", node.Name, node.Type)
		}
	}
	if !strings.Contains(source, "// not represented: 1 control node(s) a migration made up, which no diagram positions, elided\n") {
		t.Errorf("elision unaccounted for:\n%s", source)
	}
	if dotLine(source, `"n4"`) != "" || strings.Contains(source, "fillcolor=black") {
		t.Errorf("the stand-in is drawn:\n%s", source)
	}
	if line := dotLine(source, `"n1" -> "n3"`); !strings.Contains(line, `pos="e,60,100 60,240`) {
		t.Errorf("a to c is not drawn along both routes: %q\n%s", line, source)
	}
	if line := dotLine(source, `"n2" -> "n3"`); !strings.Contains(line, `pos="e,60,100 165,240 165,240 60,180`) {
		t.Errorf("b to c does not leave b's border for the route out of the join: %q\n%s", line, source)
	}
}

// A note on an edge into or out of an elided stand-in goes on every edge the
// two are joined into; one on the stand-in itself goes with it.
func TestElideNodeMovesNotesOntoJoinedEdges(t *testing.T) {
	route := []Point{{X: 0, Y: 0}, {X: 10, Y: 10}}
	out := &Rendering{
		Roots: []*Node{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "w", Kind: "join", StandIn: true}},
		Edges: []Edge{{From: "a", To: "w", Route: route}, {From: "b", To: "w"}, {From: "w", To: "c", Route: route}},
		Notes: []Note{{Text: "in", EdgeFrom: "a", EdgeTo: "w"}, {Text: "out", EdgeFrom: "w", EdgeTo: "c"},
			{Text: "on", Anchor: "w"}, {Text: "apart", EdgeFrom: "a", EdgeTo: "b"}},
	}
	elideNode(out, "w", map[string]*Geometry{})
	var got []string
	for _, note := range out.Notes {
		got = append(got, note.Text+":"+note.Anchor+note.EdgeFrom+"->"+note.EdgeTo)
	}
	want := []string{"in:a->c", "out:a->c", "out:b->c", "apart:a->b"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("notes after eliding w = %v, want %v", got, want)
	}
}

// Two routes joined through an elided node keep every waypoint of both: the
// point where they meet once, and the route out's first point where it starts
// off the route in's end, so the bend it makes there is drawn.
func TestJoinEdgesKeepsBothRoutesWaypoints(t *testing.T) {
	in := Edge{From: "a", To: "w", Route: []Point{{X: 0, Y: 20}, {X: 10, Y: 20}}}
	for _, tc := range []struct {
		name string
		out  []Point
		want string
	}{
		{"meeting", []Point{{X: 10, Y: 20}, {X: 10, Y: 50}}, "0,20 10,20 10,50"},
		{"apart", []Point{{X: 30, Y: 20}, {X: 30, Y: 50}}, "0,20 10,20 30,20 30,50"},
	} {
		joined, ok := joinEdges(in, Edge{From: "w", To: "c", Route: tc.out}, map[string]*Geometry{})
		var got []string
		for _, p := range joined.Route {
			got = append(got, fmt.Sprintf("%g,%g", p.X, p.Y))
		}
		if !ok || strings.Join(got, " ") != tc.want {
			t.Errorf("%s: joined route = %v, %v; want %q, true", tc.name, got, ok, tc.want)
		}
	}
}

// The same join is kept, and drawn as a bar in the strip with all three of its
// edges, when the drawing sets unplaced nodes in a strip: there it has a place.
func TestDOTStripKeepsMigrationStandIns(t *testing.T) {
	_, source := standInDOT(t, "StandInViews::threadedView", Options{Unplaced: UnplacedStrip})
	if strings.Contains(source, "elided") {
		t.Errorf("the stand-in is elided from a strip drawing:\n%s", source)
	}
	if line := dotLine(source, `"n4"`); !strings.Contains(line, `fillcolor=black`) || !strings.Contains(line, `pos="`) {
		t.Errorf("the stand-in is not drawn as a placed bar: %q\n%s", line, source)
	}
	for _, edge := range []string{`"n1" -> "n4"`, `"n2" -> "n4"`, `"n4" -> "n3"`} {
		if !strings.Contains(source, "\n  "+edge) {
			t.Errorf("no edge %s through the stand-in:\n%s", edge, source)
		}
	}
	if strings.Contains(source, `"n2" -> "n3"`) {
		t.Errorf("b to c is drawn past the stand-in:\n%s", source)
	}
}

// A fork the source had but left unnamed carries a synthesized name like a
// stand-in, yet is a node of the diagram: it is kept, and drawn as a bar where
// its routes meet.
func TestDOTKeepsUnnamedControlNodesOfTheSource(t *testing.T) {
	rendering, source := standInDOT(t, "StandInViews::forkedView", Options{})
	var fork *Node
	for _, node := range rendering.Roots[0].Children {
		if node.Kind == "fork" {
			fork = node
		}
	}
	if fork == nil {
		t.Fatalf("the fork is elided:\n%s", source)
	}
	if !fork.NameSynthesized || fork.StandIn {
		t.Errorf("fork: NameSynthesized %v StandIn %v, want true false", fork.NameSynthesized, fork.StandIn)
	}
	if strings.Contains(source, "not represented") {
		t.Errorf("the fork is accounted for as unrepresented:\n%s", source)
	}
	line := dotLine(source, `"n4"`)
	for _, attr := range []string{`fillcolor=black`, `label=""`, `pos="60,180!"`} {
		if !strings.Contains(line, attr) {
			t.Errorf("fork bar lacks %s: %q\n%s", attr, line, source)
		}
	}
	for _, edge := range []string{`"n1" -> "n4" [`, `"n4" -> "n2" [`, `"n4" -> "n3";`} {
		if !strings.Contains(source, "\n  "+edge) {
			t.Errorf("no edge %s through the fork:\n%s", edge, source)
		}
	}
}

// A pin several routed flows leave sits at the mean of where their routes
// start, on the action's border; each flow keeps its route but for its first
// point, brought onto the pin's border so the spline touches the one square.
func TestDOTSharedPinSitsBetweenItsRoutes(t *testing.T) {
	_, source := standInDOT(t, "StandInViews::sharedView", Options{})
	if line := dotLine(source, `"n1.0"`); !strings.Contains(line, `pos="70,134!"`) {
		t.Errorf("shared pin is not between its routes' starts at x=50 and x=90: %q\n%s", line, source)
	}
	for edge, start := range map[string]string{
		`"n1.0" -> "n2.0"`: `pos="e,50,80 68,128 68,128 `,
		`"n1.0" -> "n3.0"`: `pos="e,270,80 75,128 75,128 90,110 `,
	} {
		if line := dotLine(source, edge); !strings.Contains(line, start) {
			t.Errorf("%s does not start on the pin (64..76 x 128..140) and follow its own route: %q\n%s", edge, line, source)
		}
	}
}

// A flow drawn at a pin has no label of its own — the pin says what flows —
// unless it has a name of its own; a name the migration made up is none. That
// holds of a flow drawn at a pin at one end alone as well as at both.
func TestDOTPinnedFlowLabelsAreTheirOwnNames(t *testing.T) {
	rendering, source := standInDOT(t, "StandInViews::sharedView", Options{})
	for edge, label := range map[string]string{
		`"n1.0" -> "n2.0"`: `label="emit to left"`,
		`"n1.0" -> "n3.0"`: ``,
	} {
		line := dotLine(source, edge)
		if line == "" {
			t.Errorf("no edge %s:\n%s", edge, source)
		} else if label == "" && strings.Contains(line, "label=") {
			t.Errorf("%s, named by the migration, is labelled: %q", edge, line)
		} else if !strings.Contains(line, label) {
			t.Errorf("%s lacks %s: %q", edge, label, line)
		}
	}
	w := newDOTWriter(rendering, Options{Style: StyleCameo})
	for _, edge := range []Edge{
		{Label: "value to value", Name: "feed", FromPort: "n1.0", Kind: EdgeFlow},
		{Label: "value to value", Name: "feed", ToPort: "n2.0", Kind: EdgeFlow},
	} {
		if got := w.edgeText(edge); got != "feed" {
			t.Errorf("edgeText(%+v) = %q, want the flow's own name", edge, got)
		}
		edge.Name = ""
		if got := w.edgeText(edge); got != "" {
			t.Errorf("edgeText(%+v) = %q, want none: the pin names what flows", edge, got)
		}
	}
}

// An action written without a name, made-up or otherwise, is labelled by what
// it does like one whose name the migration made up; a named one is not. A body
// that sends and does more besides is not labelled by the send alone.
func TestActionTextOfAnonymousActions(t *testing.T) {
	rendering := render(t, "anonymous-actions.sysml", "AnonymousViews::countedView")
	texts := map[string]string{}
	for _, node := range allNodes(rendering.Roots) {
		if node.Kind == "action" {
			if node.NameSynthesized {
				t.Errorf("%s %q is taken for named by a migration", node.ID, node.Name)
			}
			texts[node.ID] = node.Name + "|" + node.Text
		}
	}
	want := map[string]string{"n1": "tally|", "n2": "|n := n + 1", "n3": "|Go", "n4": "|true", "n6": "|Go", "n7": "|"}
	for id, text := range want {
		if texts[id] != text {
			t.Errorf("%s name|text = %q, want %q", id, texts[id], text)
		}
	}
	source, err := rendering.DOTWith(Options{Style: StyleCameo})
	if err != nil {
		t.Fatalf("DOTWith: %v", err)
	}
	for id, label := range map[string]string{"n2": "n := n + 1", "n3": "Go", "n4": "true", "n6": "Go"} {
		if line := dotLine(source, `"`+id+`"`); !strings.Contains(line, label) || strings.Contains(line, "action") {
			t.Errorf("%s is not labelled by what it does alone: %q", id, line)
		}
	}
	if line := dotLine(source, `"n7"`); !strings.Contains(line, "<b>action</b>") || strings.Contains(line, "Go") {
		t.Errorf("n7, sending and assigning, is not headed by its kind: %q", line)
	}
}

// An action no diagram positions, drawn in the strip below the drawing, has its
// pins placed on the box the strip gives it.
func TestDOTStripPlacesPinsOfUnplacedActions(t *testing.T) {
	_, source := standInDOT(t, "StandInViews::sharedView", Options{Unplaced: UnplacedStrip})
	action, pin := dotLine(source, `"n4"`), dotLine(source, `"n4.0"`)
	if !strings.Contains(action, `label=<<b>loose : Take</b>>`) || !strings.Contains(action, `pos="51.5,-42!"`) {
		t.Fatalf("loose is not drawn in the strip: %q\n%s", action, source)
	}
	if !strings.Contains(pin, `xlabel="value"`) || !strings.Contains(pin, `pos="51.5,-18!"`) {
		t.Errorf("loose's pin is not on its strip box's top border: %q\n%s", pin, source)
	}
	if dotLine(source, `"n1.0" -> "n4.0"`) == "" {
		t.Errorf("the flow to the strip-drawn pin is undrawn:\n%s", source)
	}
}

// The rendering's data marks the join a migration made up as a stand-in, as
// the tree does, and no authored node.
func TestDataKeepsStandInMarker(t *testing.T) {
	rendering := render(t, "standin.sysml", "StandInViews::threadedView")
	standIns := map[string]bool{}
	for _, node := range rendering.Data().Nodes {
		if node.StandIn {
			standIns[node.ID] = true
		}
	}
	if len(standIns) != 1 || !standIns["n4"] {
		t.Errorf("stand-ins in the data = %v, want n4 alone", standIns)
	}
}
