package view

import (
	"path/filepath"
	"strings"
	"testing"
)

// naryForms are the forms the n-ary dot and flow-on-connection goldens are
// recorded in: the ones that draw nodes and edges.
var naryForms = []Form{FormDot, FormPlantUML, FormMermaid, FormD2, FormDrawio}

// TestGoldenNaryDotsAndFlowsOnConnections locks the drawing of SysML v2
// §8.2.3.3/§8.2.3.13 n-ary dots and §8.2.3.16 flow-on-connection in every form.
func TestGoldenNaryDotsAndFlowsOnConnections(t *testing.T) {
	cases := []struct {
		name, view string
		kind       Kind
	}{
		{"nary-connection", "NaryViews::wheelLinks", KindInterconnection},
		{"nary-flow-on-connection", "NaryViews::carLinks", KindInterconnection},
		{"nary-connection-def", "NaryViews::definitions", KindTree},
		{"nary-dependency", "NaryViews::dependencies", KindTree},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rendering := render(t, "nary.sysml", tc.view)
			if rendering.Kind != tc.kind {
				t.Errorf("kind = %q, want %q", rendering.Kind, tc.kind)
			}
			checkGolden(t, filepath.Join("testdata", tc.name+".text.golden"), rendering.Text())
			for _, form := range naryForms {
				machine, err := rendering.Write(form)
				if err != nil {
					t.Fatalf("write %s: %v", form, err)
				}
				checkGolden(t, filepath.Join("testdata", tc.name+"."+string(form)+".golden"), machine)
			}
		})
	}
}

// dotNodes are the n-ary dots of a rendering, by kind.
func dotNodes(rendering *Rendering) map[string][]*Node {
	dots := map[string][]*Node{}
	var walk func(nodes []*Node)
	walk = func(nodes []*Node) {
		for _, node := range nodes {
			if isDotKind(node.Kind) {
				dots[node.Kind] = append(dots[node.Kind], node)
			}
			walk(node.Children)
		}
	}
	walk(rendering.Roots)
	return dots
}

// edgesFrom are the edges leaving a node, in order.
func edgesFrom(rendering *Rendering, id string) []Edge {
	var out []Edge
	for _, edge := range rendering.Edges {
		if edge.From == id {
			out = append(out, edge)
		}
	}
	return out
}

// A connection usage of three ends is a dot nested in the owner, named for the connection,
// with a labelled segment to each end and no pairwise line; an unnamed one has no label.
func TestNaryConnectionIsADotWithASegmentPerEnd(t *testing.T) {
	rendering := render(t, "nary.sysml", "NaryViews::wheelLinks")
	dots := dotNodes(rendering)[connectionDotKind]
	if len(dots) != 2 {
		t.Fatalf("connection dots = %d, want 2:\n%s", len(dots), rendering.Text())
	}
	wheel := rendering.Roots[0]
	for _, dot := range dots {
		if dot.Kind != connectionDotKind {
			t.Errorf("dot kind = %q", dot.Kind)
		}
		nested := false
		for _, child := range wheel.Children {
			nested = nested || child == dot
		}
		if !nested {
			t.Errorf("dot %s is not nested in the Wheel node", dot.ID)
		}
		segments := edgesFrom(rendering, dot.ID)
		if len(segments) != 3 {
			t.Errorf("dot %s has %d segments, want 3", dot.ID, len(segments))
		}
		for _, segment := range segments {
			if segment.Kind != EdgeConnection || segment.Label != "" {
				t.Errorf("segment %+v: want an unlabelled connection edge", segment)
			}
		}
	}
	named, unnamed := dots[0], dots[1]
	if named.Name != "spokes" || unnamed.Name != "" {
		t.Errorf("dot names = %q, %q; want spokes and none", named.Name, unnamed.Name)
	}
	var labels []string
	for _, segment := range edgesFrom(rendering, named.ID) {
		labels = append(labels, segment.ToLabel)
	}
	if got := strings.Join(labels, ","); got != "hub,left,right" {
		t.Errorf("role names at the ends = %q, want hub,left,right", got)
	}
	for _, edge := range rendering.Edges {
		if edge.Kind == EdgeConnection && !strings.HasPrefix(edge.From, named.ID) && !strings.HasPrefix(edge.From, unnamed.ID) {
			t.Errorf("a connection line %s → %s is drawn beside the dots", edge.From, edge.To)
		}
	}
}

// A flow whose ends are a connection's ports is an item box splitting the connection's line,
// its arrow the way the item goes; a flow between ports no connection joins stays an edge.
func TestFlowsOnAConnectionAreItemBoxesOnItsLine(t *testing.T) {
	rendering := render(t, "nary.sysml", "NaryViews::carLinks")
	var boxes []*Node
	var walk func(nodes []*Node)
	walk = func(nodes []*Node) {
		for _, node := range nodes {
			if itemBox(node) {
				boxes = append(boxes, node)
			}
			walk(node.Children)
		}
	}
	walk(rendering.Roots)
	if len(boxes) != 3 {
		t.Fatalf("item boxes = %d, want 3:\n%s", len(boxes), rendering.Text())
	}
	want := []struct{ kind, name, text, arrow string }{
		{"flow", "", "of Fuel", arrowRight},
		{"flow", "", "of Reading", arrowLeft},
		{"message", "", "of Reading", arrowRight},
	}
	for i, box := range boxes {
		if box.Kind != want[i].kind || box.Name != want[i].name || box.Text != want[i].text || box.Arrow != want[i].arrow {
			t.Errorf("box %d = {%q %q %q %q}, want %v", i, box.Kind, box.Name, box.Text, box.Arrow, want[i])
		}
	}
	var connections, flows int
	for _, edge := range rendering.Edges {
		switch edge.Kind {
		case EdgeConnection:
			connections++
		case EdgeFlow:
			flows++
		}
	}
	if connections != 4 || flows != 1 {
		t.Errorf("edges: %d connection segments and %d flows; want 4 segments (two ports, three boxes) and the gauge flow", connections, flows)
	}
	// Each form writes the keyword and arrow of every box, the message's `>` as the form escapes it.
	escaped := map[Form]string{FormMermaid: "—#gt;", FormPlantUML: "—<U+003E>", FormDrawio: "—&gt;", FormDot: "—&gt;", FormD2: messageArrowRight}
	for form, message := range escaped {
		out, err := rendering.Write(form)
		if err != nil {
			t.Fatal(err)
		}
		for _, glyph := range []string{"«flow»", "«message»", flowArrowRight, flowArrowLeft, message} {
			if !strings.Contains(out, glyph) {
				t.Errorf("%s form lacks %q", form, glyph)
			}
		}
	}
}

// A dependency of several clients or suppliers is a named dot with a dashed headless link from
// each client and an arrow to each supplier; a connection def of three ends a dot in the tree.
func TestNaryDependencyAndConnectionDefAreDots(t *testing.T) {
	rendering := render(t, "nary.sysml", "NaryViews::dependencies")
	dots := dotNodes(rendering)[dependencyDotKind]
	if len(dots) != 1 || !strings.HasSuffix(dots[0].Name, "rules") {
		t.Fatalf("dependency dots = %v, want one named rules:\n%s", dots, rendering.Text())
	}
	var clients, suppliers int
	for _, edge := range rendering.Edges {
		switch {
		case edge.Kind == EdgeDependencyClient && edge.To == dots[0].ID:
			clients++
		case edge.Kind == EdgeDependency && edge.From == dots[0].ID && edge.Label == "":
			suppliers++
		default:
			t.Errorf("unexpected edge %+v", edge)
		}
	}
	if clients != 2 || suppliers != 2 {
		t.Errorf("client links = %d, supplier links = %d; want 2 and 2", clients, suppliers)
	}

	tree := render(t, "nary.sysml", "NaryViews::definitions")
	defs := dotNodes(tree)[connectionDefDotKind]
	if len(defs) != 1 || defs[0].Name != "Nary::Triad" {
		t.Fatalf("connection def dots = %v, want one named Nary::Triad:\n%s", defs, tree.Text())
	}
	segments := edgesFrom(tree, defs[0].ID)
	if len(segments) != 3 {
		t.Fatalf("def segments = %d, want 3:\n%s", len(segments), tree.Text())
	}
	var labels []string
	for _, segment := range segments {
		if segment.Kind != EdgeAssociation {
			t.Errorf("segment %+v is no association line", segment)
		}
		labels = append(labels, segment.ToLabel)
	}
	if got := strings.Join(labels, ","); got != "hub[1],left[1],right[1]" {
		t.Errorf("end labels = %q, want hub[1],left[1],right[1]", got)
	}
}

func TestSplitRouteKeepsALoneWaypointOnTheFirstSegment(t *testing.T) {
	pieces := splitRoute([]Point{{X: 150, Y: 80}}, 2)
	if len(pieces) != 3 || len(pieces[0]) != 1 || pieces[0][0] != (Point{X: 150, Y: 80}) || pieces[1] != nil || pieces[2] != nil {
		t.Fatalf("splitRoute of one waypoint = %v, want it on the first of three pieces", pieces)
	}
	if splitRoute(nil, 1) != nil {
		t.Fatal("splitRoute of no route is not nil")
	}
}

// A Route about an n-ary relationship steers none of its segments: one polyline
// has no place for the dot, so each segment is laid out on its own, a notice
// says the route was not followed, and the relationship is not steerable, so a
// draw.io import refuses to reroute a segment rather than write a Route nothing follows.
func TestRoutedNaryRelationshipSegmentsCarryNoRoute(t *testing.T) {
	for _, tc := range []struct{ view, name, kind string }{
		{"NaryViews::routedRules", "rules", dependencyDotKind},
		{"NaryViews::routedSpokes", "spokes", connectionDotKind},
		{"NaryViews::routedWheel", "spokes", connectionDotKind},
	} {
		rendering := render(t, "nary.sysml", tc.view)
		var dot *Node
		for _, candidate := range dotNodes(rendering)[tc.kind] {
			if strings.HasSuffix(candidate.Name, tc.name) {
				dot = candidate
			}
		}
		if dot == nil {
			t.Fatalf("%s: no %s named %s:\n%s", tc.view, tc.kind, tc.name, rendering.Text())
		}
		segments := 0
		for _, edge := range rendering.Edges {
			if edge.From != dot.ID && edge.To != dot.ID {
				continue
			}
			segments++
			if len(edge.Route) != 0 {
				t.Errorf("%s: segment %s → %s carries the relationship's route %v", tc.view, edge.From, edge.To, edge.Route)
			}
			if rendering.drawn.Steers(edge.Origin, edge.Kind) {
				t.Errorf("%s: segment %s → %s is steerable, so an import would write a Route the segments never follow", tc.view, edge.From, edge.To)
			}
		}
		if segments < 3 {
			t.Errorf("%s: %d segments at the dot, want at least 3", tc.view, segments)
		}
		noticed := false
		for _, notice := range rendering.Notices {
			noticed = noticed || strings.Contains(notice, tc.name+" states a route")
		}
		if !noticed {
			t.Errorf("%s: notices = %q, want one saying %s states a route", tc.view, rendering.Notices, tc.name)
		}
	}
}
