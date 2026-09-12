package view

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// dotGoldenCase is one model whose DOT is locked as a golden: every case but
// the positioned one is laid out by `dot` and carries no geometry.
type dotGoldenCase struct {
	name   string
	file   string
	view   string
	kind   Kind
	layout string
}

var dotGoldenCases = []dotGoldenCase{
	{"tree", "tree.sysml", "VehicleViews::vehicleView", KindTree, "dot"},
	{"interconnection", "interconnection.sysml", "PlantViews::loopView", KindInterconnection, "dot"},
	{"state", "state.sysml", "MachineViews::vehicleStates", KindState, "dot"},
	{"state-entry", "state-entry.sysml", "MachineViews::thermostat", KindState, "dot"},
	{"action", "action.sysml", "FlowViews::driveView", KindAction, "dot"},
	{"filters", "filters.sysml", "FilteredViews::safetyView", KindTree, "dot"},
	{"layout", "layout.sysml", "PlantViews::placedView", KindInterconnection, "neato -n2"},
}

// TestGoldenDOT locks the DOT of each kind that has one, from the models the
// Mermaid goldens use, and checks each is well-formed DOT.
func TestGoldenDOT(t *testing.T) {
	for _, tc := range dotGoldenCases {
		t.Run(tc.name, func(t *testing.T) {
			rendering := render(t, tc.file, tc.view)
			if rendering.Kind != tc.kind {
				t.Errorf("kind = %q, want %q", rendering.Kind, tc.kind)
			}
			dot, err := rendering.Write(FormDot)
			if err != nil {
				t.Fatalf("write dot: %v", err)
			}
			direct, err := rendering.DOT()
			if err != nil || direct != dot {
				t.Errorf("DOT() = %q, %v; want what Write(FormDot) writes", direct, err)
			}
			checkGolden(t, filepath.Join("testdata", tc.name+".dot.golden"), dot)
			checkDOTSyntax(t, dot)
			for _, want := range []string{"// view: " + tc.view, "// kind: " + string(tc.kind), "// layout: " + tc.layout + "\n", "digraph " + dotQuote(tc.view) + " {", "node [shape=box];"} {
				if !strings.Contains(dot, want) {
					t.Errorf("DOT lacks %q:\n%s", want, dot)
				}
			}
			if strings.Contains(dot, "rankdir") {
				t.Errorf("DOT states a rankdir with no direction asked for:\n%s", dot)
			}
		})
	}
}

// Every kind that is drawn as a graph has the DOT form; a table and a sequence
// diagram have none, and asking is a typed error naming the kind and the form.
func TestDOTFormSupport(t *testing.T) {
	for _, kind := range Kinds() {
		want := kind == KindTree || kind == KindInterconnection || kind == KindState || kind == KindAction
		if got := kind.SupportsForm(FormDot); got != want {
			t.Errorf("%s.SupportsForm(dot) = %v, want %v", kind, got, want)
		}
		if got := kind.SupportsForm(kind.MachineForm()); !got {
			t.Errorf("%s does not support its machine form", kind)
		}
		if !kind.SupportsForm(FormText) {
			t.Errorf("%s does not support the text form", kind)
		}
		if kind.MachineForm() != FormMermaid && kind.SupportsForm(FormMermaid) || kind.MachineForm() != FormMarkdown && kind.SupportsForm(FormMarkdown) {
			t.Errorf("%s supports a machine form that is not its own", kind)
		}
	}
	if got := KindTree.SupportedForms(); fmt.Sprint(got) != "[text mermaid dot]" {
		t.Errorf("tree forms = %v", got)
	}
	if got := KindTable.SupportedForms(); fmt.Sprint(got) != "[text markdown]" {
		t.Errorf("table forms = %v", got)
	}
	unsupported := []struct {
		file, view string
		kind       Kind
	}{
		{"sequence.sysml", "SequenceViews::pubSubView", KindSequence},
		{"table.sysml", "TableViews::partsTable", KindTable},
	}
	for _, tc := range unsupported {
		t.Run(string(tc.kind), func(t *testing.T) {
			rendering := render(t, tc.file, tc.view)
			for name, write := range map[string]func() (string, error){
				"Write": func() (string, error) { return rendering.Write(FormDot) },
				"DOT":   rendering.DOT,
			} {
				out, err := write()
				var wrong *WrongFormError
				if !errors.As(err, &wrong) || !errors.Is(err, ErrWrongForm) {
					t.Fatalf("%s error = %v, want a *WrongFormError", name, err)
				}
				if out != "" {
					t.Errorf("%s wrote %q alongside the error", name, out)
				}
				if wrong.Form != FormDot || wrong.Kind != tc.kind {
					t.Errorf("%s error = %+v, want form dot of kind %s", name, wrong, tc.kind)
				}
				for _, want := range []string{string(tc.kind), "dot", string(FormText), string(tc.kind.MachineForm())} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("%s error %q does not say %q", name, err, want)
					}
				}
			}
		})
	}
	// The wrong-form error of a graph-shaped kind offers DOT among its forms.
	_, err := render(t, "tree.sysml", "VehicleViews::vehicleView").Write(FormMarkdown)
	if err == nil || !strings.Contains(err.Error(), "ask for text, mermaid or dot") {
		t.Errorf("markdown of a tree error = %v, want it to offer dot", err)
	}
	_, err = render(t, "tree.sysml", "VehicleViews::vehicleView").Write("svg")
	if err == nil || !strings.Contains(err.Error(), "text, mermaid, markdown and dot") {
		t.Errorf("unknown form error = %v, want it to list dot", err)
	}
}

// Every identifier and label is written quoted, with the quotes and
// backslashes it carries escaped, so a name holding either is still one ID.
func TestDOTQuotesEveryIdentifierAndLabel(t *testing.T) {
	rendering := &Rendering{
		View: `Odd::view "quoted" and \backslashed`,
		Kind: KindInterconnection,
		Roots: []*Node{
			{ID: `a"b`, Kind: "part", Name: `say "hi"`, Detail: `C:\path`},
			{ID: `c\d`, Kind: "part", Name: "plain", Children: []*Node{
				{ID: "e", Kind: "port", Name: "line one\nline two"},
			}},
		},
		Edges: []Edge{{From: `a"b`, To: "e", Label: `"quoted" \ label`, Kind: EdgeConnection}},
	}
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	for _, want := range []string{
		`digraph "Odd::view \"quoted\" and \\backslashed" {`,
		`"a\"b" [label="part say \"hi\"\nC:\\path"];`,
		`subgraph "cluster_c\\d" {`,
		`"e" [label="port line one\nline two"];`,
		`"a\"b" -> "e" [label="\"quoted\" \\ label", arrowhead=none];`,
	} {
		if !strings.Contains(dot, want) {
			t.Errorf("DOT lacks %q:\n%s", want, dot)
		}
	}
	if got := dotQuote(`a"b\c` + "\n"); got != `"a\"b\\c\n"` {
		t.Errorf("dotQuote = %s", got)
	}
}

// A node with children is a cluster, nested as deep as the rendering nests;
// an edge ending at one names its anchor and is clipped at its border, unless
// the other end is inside it.
func TestDOTNestedClusters(t *testing.T) {
	rendering := &Rendering{
		View: "Nested::view",
		Kind: KindInterconnection,
		Roots: []*Node{
			{ID: "n0", Kind: "part def", Name: "Outer", Children: []*Node{
				{ID: "n1", Kind: "part", Name: "inner", Children: []*Node{
					{ID: "n2", Kind: "port", Name: "p"},
					{ID: "n3", Kind: "port", Name: "q"},
				}},
				{ID: "n4", Kind: "part", Name: "leaf"},
			}},
			{ID: "n5", Kind: "part def", Name: "Other"},
		},
		Edges: []Edge{
			{From: "n4", To: "n1", Kind: EdgeConnection},
			{From: "n1", To: "n5", Label: "out", Kind: EdgeFlow},
			{From: "n2", To: "n3", Kind: EdgeConnection},
			{From: "n0", To: "n1", Kind: EdgeTransition},
			{From: "n2", To: "n0", Kind: EdgeTransition},
		},
	}
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	want := `// view: Nested::view
// kind: interconnection
// layout: dot
digraph "Nested::view" {
  graph [compound=true];
  node [shape=box];
  subgraph "cluster_n0" {
    label="part def Outer";
    "n0" [shape=point, style=invis, width=0, height=0, label=""];
    subgraph "cluster_n1" {
      label="part inner";
      "n1" [shape=point, style=invis, width=0, height=0, label=""];
      "n2" [label="port p"];
      "n3" [label="port q"];
    }
    "n4" [label="part leaf"];
  }
  "n5" [label="part def Other"];
  "n4" -> "n1" [arrowhead=none, lhead="cluster_n1"];
  "n1" -> "n5" [label="out", style=dashed, ltail="cluster_n1"];
  "n2" -> "n3" [arrowhead=none];
  "n0" -> "n1" [lhead="cluster_n1"];
  "n2" -> "n0";
}
`
	if dot != want {
		t.Errorf("DOT:\n%s\nwant:\n%s", dot, want)
	}
	// A tree draws containment as edges, so it declares no cluster.
	rendering.Kind = KindTree
	dot, err = rendering.DOT()
	if err != nil {
		t.Fatalf("DOT of tree: %v", err)
	}
	checkDOTSyntax(t, dot)
	if strings.Contains(dot, "subgraph") || strings.Contains(dot, "compound") {
		t.Errorf("tree DOT declares a cluster:\n%s", dot)
	}
	for _, want := range []string{`"n0" -> "n1" [arrowhead=none];`, `"n1" -> "n2" [arrowhead=none];`, `"n4" -> "n1" [arrowhead=none];`} {
		if !strings.Contains(dot, want) {
			t.Errorf("tree DOT lacks %q:\n%s", want, dot)
		}
	}
}

// Each direction is the graph's rankdir, and no direction leaves it out.
func TestDOTDirections(t *testing.T) {
	rendering := render(t, "state.sysml", "MachineViews::vehicleStates")
	for _, direction := range []Direction{DirectionTopBottom, DirectionLeftRight, DirectionRightLeft, DirectionBottomTop} {
		dot, err := rendering.DOTDirected(direction)
		if err != nil {
			t.Fatalf("DOTDirected(%s): %v", direction, err)
		}
		checkDOTSyntax(t, dot)
		if want := "  graph [rankdir=" + string(direction) + ", compound=true];\n"; !strings.Contains(dot, want) {
			t.Errorf("DOTDirected(%s) lacks %q:\n%s", direction, want, dot)
		}
	}
	dot, err := rendering.DOTDirected("")
	if err != nil {
		t.Fatalf("DOTDirected(\"\"): %v", err)
	}
	if strings.Contains(dot, "rankdir") {
		t.Errorf("no direction still writes a rankdir:\n%s", dot)
	}
	// A graph with neither a direction nor a clipped edge has no graph attributes.
	plain := &Rendering{View: "V", Kind: KindTree, Roots: []*Node{{ID: "n0", Kind: "part", Name: "a"}}}
	dot, err = plain.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	if strings.Contains(dot, "graph [") {
		t.Errorf("plain DOT writes a graph attribute list:\n%s", dot)
	}
}

// Every edge kind is drawn in the style parallel to its Mermaid arrow.
func TestDOTEdgeKinds(t *testing.T) {
	styles := map[EdgeKind]string{
		EdgeConnection: `"a" -> "b" [label="k", arrowhead=none];`,
		EdgeTransition: `"a" -> "b" [label="k"];`,
		EdgeSuccession: `"a" -> "b" [label="k"];`,
		EdgeFlow:       `"a" -> "b" [label="k", style=dashed];`,
	}
	for kind := EdgeConnection; kind.String() != "edge"; kind++ {
		want, ok := styles[kind]
		if !ok {
			t.Fatalf("edge kind %s has no DOT style under test", kind)
		}
		rendering := &Rendering{View: "V", Kind: KindAction,
			Roots: []*Node{{ID: "a", Kind: "action", Name: "a"}, {ID: "b", Kind: "action", Name: "b"}},
			Edges: []Edge{{From: "a", To: "b", Label: "k", Kind: kind}}}
		dot, err := rendering.DOT()
		if err != nil {
			t.Fatalf("DOT: %v", err)
		}
		checkDOTSyntax(t, dot)
		if !strings.Contains(dot, want) {
			t.Errorf("%s edge: DOT lacks %q:\n%s", kind, want, dot)
		}
		// Mermaid draws the same distinction: a line, a dashed arrow, an arrow.
		arrow := mermaidArrow(kind)
		switch {
		case strings.Contains(want, "arrowhead=none") != (arrow == "---"):
			t.Errorf("%s: DOT arrowhead and Mermaid arrow %q disagree", kind, arrow)
		case strings.Contains(want, "dashed") != strings.Contains(arrow, "."):
			t.Errorf("%s: DOT style and Mermaid arrow %q disagree", kind, arrow)
		}
	}
	// An edge with no label has no label attribute, and none at all when plain.
	plain := &Rendering{View: "V", Kind: KindAction,
		Roots: []*Node{{ID: "a", Kind: "action", Name: "a"}, {ID: "b", Kind: "action", Name: "b"}},
		Edges: []Edge{{From: "a", To: "b", Kind: EdgeSuccession}}}
	dot, err := plain.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	if !strings.Contains(dot, "  \"a\" -> \"b\";\n") {
		t.Errorf("plain edge is not bare:\n%s", dot)
	}
}

// States are rounded boxes, each body's start a point, and every transition
// label is carried as the Mermaid form carries it.
func TestDOTStateShapesAndLabels(t *testing.T) {
	rendering := render(t, "state-entry.sysml", "MachineViews::thermostat")
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	starts := 0
	for _, root := range rendering.Roots {
		starts += countKind(root, startKind)
	}
	if got := strings.Count(dot, "[shape=point, label=\"\"]"); got != starts || starts == 0 {
		t.Errorf("DOT draws %d start points, want %d:\n%s", got, starts, dot)
	}
	for _, want := range []string{
		`[shape=box, style=rounded, label="state heating"];`,
		`subgraph "cluster_n6" {`,
		`label="region lights";`,
		`style=dashed;`,
		`"n12" -> "n1" [label="[cold]"];`,
		`"n12" -> "n2" [label="[not cold]", lhead="cluster_n2"];`,
		`"n2" -> "n5" [ltail="cluster_n2", lhead="cluster_n5"];`,
		`[label="[not dark]"]`,
	} {
		if !strings.Contains(dot, want) {
			t.Errorf("DOT lacks %q:\n%s", want, dot)
		}
	}
	mermaid := rendering.Mermaid()
	for _, edge := range rendering.Edges {
		if edge.Label == "" {
			continue
		}
		if !strings.Contains(dot, "label="+dotQuote(edge.Label)) {
			t.Errorf("DOT drops the transition label %q", edge.Label)
		}
		if !strings.Contains(mermaid, ": "+mermaidText(edge.Label)) {
			t.Errorf("Mermaid drops the transition label %q", edge.Label)
		}
	}
	// An action's initial and final nodes are circles; a state's detail is its
	// second line.
	final := &Rendering{View: "V", Kind: KindAction, Roots: []*Node{
		{ID: "a", Kind: "final", Name: "done"}, {ID: "b", Kind: "state", Name: "off", Detail: "initial"}, {ID: "c", Kind: "initial", Name: "go"}}}
	dot, err = final.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	for _, want := range []string{`"a" [shape=doublecircle, label="final done"];`, `"b" [shape=box, style=rounded, label="state off\ninitial"];`, `"c" [shape=circle, label="initial go"];`} {
		if !strings.Contains(dot, want) {
			t.Errorf("DOT lacks %q:\n%s", want, dot)
		}
	}
}

// countKind counts the nodes under node of the kind.
func countKind(node *Node, kind string) int {
	n := 0
	if node.Kind == kind {
		n++
	}
	for _, child := range node.Children {
		n += countKind(child, kind)
	}
	return n
}

// An empty rendering is one plaintext node saying why, and the header carries
// every notice and how the kind was stated.
func TestDOTEmptyAndNotices(t *testing.T) {
	rendering := &Rendering{View: "V::empty", Kind: KindAction, Stated: "render asActionFlow",
		Notices: []string{"part p is no action; an action rendering does not show it", "second"}}
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	want := `// view: V::empty
// kind: action
// stated: render asActionFlow
// not represented: part p is no action; an action rendering does not show it
// not represented: second
// layout: dot
digraph "V::empty" {
  node [shape=box];
  "empty" [shape=plaintext, label="the rendering is empty: nothing the view exposes is shown by an action rendering"];
}
`
	if dot != want {
		t.Errorf("DOT:\n%s\nwant:\n%s", dot, want)
	}
	// A rendering of no named view is an anonymous digraph.
	anonymous := &Rendering{Kind: KindTree, Roots: []*Node{{ID: "n0", Kind: "part", Name: "a"}}}
	dot, err = anonymous.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	if !strings.HasPrefix(dot, "// kind: tree\n// layout: dot\ndigraph {\n") {
		t.Errorf("anonymous DOT:\n%s", dot)
	}
}

// placedRendering is a hand-built interconnection diagram with a canvas of the
// unit: a sizeless node, a sized node, and a routed connection between them.
func placedRendering(unit string) *Rendering {
	return &Rendering{
		View: "Placed::view",
		Kind: KindInterconnection,
		Roots: []*Node{
			{ID: "a", Kind: "part", Name: "a", Geometry: &Geometry{X: 100, Y: 50}},
			{ID: "b", Kind: "part", Name: "b", Geometry: &Geometry{X: 200, Y: 100, Width: 80, Height: 40, HasSize: true}},
		},
		Edges:  []Edge{{From: "a", To: "b", Kind: EdgeConnection, Route: []Point{{X: 100, Y: 50}, {X: 160, Y: 50}, {X: 240, Y: 120}}}},
		Canvas: &Canvas{Unit: unit, Width: 400, Height: 300, HasSize: true},
	}
}

// A placed node is pinned at its centre in points, y flipped against the canvas
// height, with its size in inches: 1 px is 0.75 pt (96 dpi), and a `pt` canvas
// converts one to one. A sizeless node's point is its centre.
func TestDOTWritesPositions(t *testing.T) {
	dot, err := placedRendering("px").DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	for _, want := range []string{
		// (100, 50) px from the top left of a 300 px high canvas: x 75 pt, y (300-50)*0.75 pt.
		`"a" [label="part a", pos="75,187.5!", pin=true];`,
		// Centre (240, 120) px; 80×40 px is 0.8333×0.4167 in at 96 px/in.
		`"b" [label="part b", pos="180,135!", pin=true, width=0.8333, height=0.4167, fixedsize=true];`,
	} {
		if !strings.Contains(dot, want) {
			t.Errorf("px DOT lacks %q:\n%s", want, dot)
		}
	}
	dot, err = placedRendering("pt").DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	for _, want := range []string{
		`"a" [label="part a", pos="100,250!", pin=true];`,
		`"b" [label="part b", pos="240,180!", pin=true, width=1.1111, height=0.5556, fixedsize=true];`,
		`graph [bb="0,0,400,300"];`,
	} {
		if !strings.Contains(dot, want) {
			t.Errorf("pt DOT lacks %q:\n%s", want, dot)
		}
	}
	if strings.Contains(dot, "not represented") {
		t.Errorf("a pt canvas is noticed:\n%s", dot)
	}
	// Without a canvas extent the y axis flips against the lowest edge of the
	// placed nodes, 100 + 40 here, computed once for every position.
	unsized := placedRendering("px")
	unsized.Canvas = nil
	dot, err = unsized.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	for _, want := range []string{
		`"a" [label="part a", pos="75,67.5!", pin=true];`,
		`"b" [label="part b", pos="180,15!", pin=true, width=0.8333, height=0.4167, fixedsize=true];`,
		`pos="75,67.5 90,67.5 105,67.5 120,67.5 140,50 160,32.5 180,15"`,
	} {
		if !strings.Contains(dot, want) {
			t.Errorf("canvas-less DOT lacks %q:\n%s", want, dot)
		}
	}
	if strings.Contains(dot, "bb=") {
		t.Errorf("canvas-less DOT states a bounding box:\n%s", dot)
	}
	// A converted number is written to four decimals at most and never as -0.
	for v, want := range map[float64]string{0: "0", -0.00001: "0", 1.0 / 3: "0.3333", 2.0 / 3: "0.6667", 120: "120", 12.5: "12.5"} {
		if got := dotNumber(v); got != want {
			t.Errorf("dotNumber(%v) = %s, want %s", v, got, want)
		}
	}
}

// A route is written as the B-spline that draws its polyline: for each segment
// P→Q the control points P, P+(Q-P)/3, P+2(Q-P)/3, Q, with Q shared, so n
// waypoints are 3(n-1)+1 points; a single waypoint draws nothing and is noticed.
func TestDOTWritesRoutesAsSplines(t *testing.T) {
	dot, err := placedRendering("px").DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	// (100,50)→(160,50)→(240,120) px is (75,187.5)→(120,187.5)→(180,135) pt.
	want := `"a" -> "b" [arrowhead=none, pos="75,187.5 90,187.5 105,187.5 120,187.5 140,170 160,152.5 180,135"];`
	if !strings.Contains(dot, want) {
		t.Errorf("DOT lacks %q:\n%s", want, dot)
	}
	if strings.Contains(dot, "lp=") || strings.Contains(dot, "splines") {
		t.Errorf("DOT positions the label or sets splines:\n%s", dot)
	}
	for n := 2; n <= 5; n++ {
		route := make([]Point, n)
		for i := range route {
			route[i] = Point{X: float64(i * 30), Y: float64(i * i)}
		}
		rendering := placedRendering("px")
		rendering.Edges[0].Route = route
		dot, err := rendering.DOT()
		if err != nil {
			t.Fatalf("DOT: %v", err)
		}
		checkDOTSyntax(t, dot)
		_, after, _ := strings.Cut(dot, `"a" -> "b" [arrowhead=none, pos="`)
		spline, _, _ := strings.Cut(after, `"`)
		if got := len(strings.Split(spline, " ")); got != 3*(n-1)+1 {
			t.Errorf("%d waypoints: spline %q has %d points, want %d", n, spline, got, 3*(n-1)+1)
		}
	}
	single := placedRendering("px")
	single.Edges[0].Route = single.Edges[0].Route[:1]
	dot, err = single.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	if !strings.Contains(dot, "// not represented: the route of a -> b: one waypoint draws no line\n") || !strings.Contains(dot, "// layout: neato -n\n") {
		t.Errorf("a one-point route is not noticed:\n%s", dot)
	}
	if strings.Contains(dot, `"a" -> "b" [arrowhead=none, pos=`) {
		t.Errorf("a one-point route is written:\n%s", dot)
	}
}

// The header names the engine the file is written for: `dot` with no geometry,
// `neato -n` when nodes are placed, `neato -n2` when an edge is routed too;
// nodes left unplaced are noticed, since `neato -n` needs a position on each.
func TestDOTHeaderNamesNeatoWhenPositioned(t *testing.T) {
	plain := &Rendering{View: "V", Kind: KindAction,
		Roots: []*Node{{ID: "a", Kind: "action", Name: "a"}, {ID: "b", Kind: "action", Name: "b"}},
		Edges: []Edge{{From: "a", To: "b", Kind: EdgeSuccession}}}
	dot, err := plain.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	if !strings.HasPrefix(dot, "// view: V\n// kind: action\n// layout: dot\ndigraph \"V\" {\n") {
		t.Errorf("unplaced header:\n%s", dot)
	}
	plain.Roots[0].Geometry = &Geometry{X: 10, Y: 20}
	dot, err = plain.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	if !strings.HasPrefix(dot, "// view: V\n// kind: action\n// not represented: no position for 1 of 2 nodes (neato -n needs one on every node; neato -s72 keeps the pinned ones and places the rest)\n// layout: neato -n\ndigraph \"V\" {\n") {
		t.Errorf("partly placed header:\n%s", dot)
	}
	plain.Roots[1].Geometry = &Geometry{X: 10, Y: 80}
	dot, err = plain.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	if !strings.HasPrefix(dot, "// view: V\n// kind: action\n// layout: neato -n\ndigraph \"V\" {\n") {
		t.Errorf("placed header:\n%s", dot)
	}
	plain.Edges[0].Route = []Point{{X: 10, Y: 20}, {X: 10, Y: 80}}
	dot, err = plain.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	if !strings.HasPrefix(dot, "// view: V\n// kind: action\n// layout: neato -n2\ndigraph \"V\" {\n") {
		t.Errorf("routed header:\n%s", dot)
	}
	// A route alone asks for -n2 as well, the flip height is then 0, and the
	// nodes -n2 needs a position on are noticed.
	plain.Roots[0].Geometry, plain.Roots[1].Geometry = nil, nil
	dot, err = plain.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	if !strings.Contains(dot, "// layout: neato -n2\n") || !strings.Contains(dot, `pos="7.5,-15 7.5,-30 7.5,-45 7.5,-60"`) || !strings.Contains(dot, "// not represented: no position for 2 of 2 nodes") {
		t.Errorf("route-only DOT:\n%s", dot)
	}
}

// A sized canvas is the root graph's `bb` in points, after the direction and
// `compound`; a canvas without an extent, or none, adds nothing.
func TestDOTCanvasBoundingBox(t *testing.T) {
	rendering := render(t, "layout.sysml", "PlantViews::placedView")
	dot, err := rendering.DOTDirected(DirectionLeftRight)
	if err != nil {
		t.Fatalf("DOTDirected: %v", err)
	}
	checkDOTSyntax(t, dot)
	if !strings.Contains(dot, "  graph [rankdir=LR, bb=\"0,0,900,600\"];\n") {
		t.Errorf("1200×800 px canvas is not bb 900×600 pt after rankdir:\n%s", dot)
	}
	if strings.Contains(dot, " size=") {
		t.Errorf("DOT sets size:\n%s", dot)
	}
	clipped := &Rendering{View: "V", Kind: KindInterconnection,
		Roots:  []*Node{{ID: "a", Kind: "part", Name: "a", Children: []*Node{{ID: "c", Kind: "port", Name: "c"}}}, {ID: "b", Kind: "part", Name: "b"}},
		Edges:  []Edge{{From: "a", To: "b", Kind: EdgeConnection}},
		Canvas: &Canvas{Unit: "pt", Width: 50, Height: 20.5, HasSize: true}}
	dot, err = clipped.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	if !strings.Contains(dot, "  graph [compound=true, bb=\"0,0,50,20.5\"];\n") || !strings.Contains(dot, "// layout: dot\n") {
		t.Errorf("compound and bb:\n%s", dot)
	}
	clipped.Canvas.HasSize = false
	dot, err = clipped.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	if !strings.Contains(dot, "  graph [compound=true];\n") {
		t.Errorf("an unsized canvas changes the graph attributes:\n%s", dot)
	}
	if plain := render(t, "layout.sysml", "PlantViews::plainView"); plain.Canvas != nil {
		t.Fatalf("plainView has a canvas: %+v", plain.Canvas)
	} else if dot, err := plain.DOT(); err != nil || strings.Contains(dot, "bb=") {
		t.Errorf("no canvas, yet a bb (%v):\n%s", err, dot)
	}
}

// A canvas unit the writer does not convert falls back to px, and the header
// says so; a unit that converts nothing is not noticed.
func TestDOTUnknownCanvasUnitIsNoticed(t *testing.T) {
	dot, err := placedRendering("mm").DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	px, err := placedRendering("px").DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	notice := "// not represented: canvas unit \"mm\"; positions are converted as px (1 px = 0.75 pt)\n"
	if !strings.Contains(dot, notice) {
		t.Errorf("mm DOT lacks the notice:\n%s", dot)
	}
	if got := strings.Replace(dot, notice, "", 1); got != px {
		t.Errorf("mm DOT is not the px DOT plus the notice:\n%s\nwant:\n%s", got, px)
	}
	idle := &Rendering{View: "V", Kind: KindTree, Roots: []*Node{{ID: "a", Kind: "part", Name: "a"}}, Canvas: &Canvas{Unit: "mm"}}
	dot, err = idle.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	if strings.Contains(dot, "not represented") || !strings.Contains(dot, "// layout: dot\n") {
		t.Errorf("a unit converting nothing is noticed:\n%s", dot)
	}
	// A rendering's own notices come first, then the writer's.
	noticed := placedRendering("mm")
	noticed.Notices = []string{"first"}
	dot, err = noticed.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	if !strings.Contains(dot, "// not represented: first\n"+notice+"// layout: neato -n2\n") {
		t.Errorf("notice order:\n%s", dot)
	}
}

// A rendering with no geometry writes the bytes the goldens hold: no position,
// pin, size or bounding box, and the `dot` header.
func TestDOTWithoutGeometryIsUnchanged(t *testing.T) {
	for _, tc := range dotGoldenCases {
		if tc.name == "layout" {
			continue
		}
		rendering := render(t, tc.file, tc.view)
		if rendering.Canvas != nil || anyGeometry(rendering) {
			t.Fatalf("%s carries geometry", tc.name)
		}
		want, err := os.ReadFile(filepath.Join("testdata", tc.name+".dot.golden"))
		if err != nil {
			t.Fatal(err)
		}
		dot, err := rendering.DOT()
		if err != nil {
			t.Fatalf("%s: DOT: %v", tc.name, err)
		}
		if dot != string(want) {
			t.Errorf("%s: DOT differs from its golden:\n%s", tc.name, dot)
		}
		for _, absent := range []string{"pos=", "pin=", "fixedsize", "bb=", "neato"} {
			if strings.Contains(dot, absent) {
				t.Errorf("%s: DOT without geometry carries %q:\n%s", tc.name, absent, dot)
			}
		}
	}
}

// anyGeometry reports whether a node or edge of the rendering is placed.
func anyGeometry(r *Rendering) bool {
	var placed func(node *Node) bool
	placed = func(node *Node) bool {
		if node.Geometry != nil {
			return true
		}
		for _, child := range node.Children {
			if placed(child) {
				return true
			}
		}
		return false
	}
	for _, root := range r.Roots {
		if placed(root) {
			return true
		}
	}
	for _, edge := range r.Edges {
		if len(edge.Route) > 0 {
			return true
		}
	}
	return false
}

// A collapsed node with children is a leaf: no cluster, its label as it was,
// its children unwritten, an edge at a child drawn at the collapsed node and an
// edge between two of its children dropped. The unplaced cluster around it has
// its anchor set amid its placed members, unpinned.
func TestDOTCollapsedNodeIsLeaf(t *testing.T) {
	rendering := &Rendering{
		View: "V",
		Kind: KindInterconnection,
		Roots: []*Node{
			{ID: "n0", Kind: "part def", Name: "Outer", Children: []*Node{
				{ID: "n1", Kind: "part", Name: "folded", Detail: "Kind", Geometry: &Geometry{X: 40, Y: 40, Width: 80, Height: 40, HasSize: true, Collapsed: true}, Children: []*Node{
					{ID: "n2", Kind: "port", Name: "p", Geometry: &Geometry{X: 40, Y: 60}},
					{ID: "n3", Kind: "part", Name: "deep", Children: []*Node{{ID: "n4", Kind: "port", Name: "q"}}},
				}},
				{ID: "n5", Kind: "part", Name: "open", Geometry: &Geometry{X: 200, Y: 40}, Children: []*Node{{ID: "n6", Kind: "port", Name: "r"}}},
			}},
		},
		Edges: []Edge{
			{From: "n2", To: "n6", Kind: EdgeConnection},
			{From: "n2", To: "n4", Kind: EdgeConnection},
			{From: "n5", To: "n1", Kind: EdgeFlow},
		},
	}
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	want := `// view: V
// kind: interconnection
// not represented: no position for 1 of 4 nodes (neato -n needs one on every node; neato -s72 keeps the pinned ones and places the rest)
// not represented: the box of n0, which has no size (neato -n draws a cluster from its bb alone)
// not represented: the box of n5, which has no size (neato -n draws a cluster from its bb alone)
// layout: neato -n
digraph "V" {
  graph [compound=true];
  node [shape=box];
  subgraph "cluster_n0" {
    label="part def Outer";
    "n0" [shape=point, style=invis, width=0, height=0, label="", pos="90,15"];
    "n1" [label="part folded\nKind", pos="60,15!", pin=true, width=0.8333, height=0.4167, fixedsize=true];
    subgraph "cluster_n5" {
      label="part open";
      "n5" [shape=point, style=invis, width=0, height=0, label="", pos="150,30!", pin=true];
      "n6" [label="port r"];
    }
  }
  "n1" -> "n6" [arrowhead=none];
  "n5" -> "n1" [style=dashed, ltail="cluster_n5"];
}
`
	if dot != want {
		t.Errorf("DOT:\n%s\nwant:\n%s", dot, want)
	}
	// A collapsed node without children is an ordinary leaf, and a tree folds
	// the same way: no containment edge to a hidden child.
	rendering.Kind = KindTree
	dot, err = rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	for _, absent := range []string{`"n2"`, `"n3"`, `"n4"`, "subgraph"} {
		if strings.Contains(dot, absent) {
			t.Errorf("tree DOT writes %s under a collapsed node:\n%s", absent, dot)
		}
	}
	for _, want := range []string{`"n0" -> "n1" [arrowhead=none];`, `"n5" -> "n6" [arrowhead=none];`, `"n1" -> "n6" [arrowhead=none];`} {
		if !strings.Contains(dot, want) {
			t.Errorf("tree DOT lacks %q:\n%s", want, dot)
		}
	}
	fixture := render(t, "layout.sysml", "PlantViews::placedView")
	dot, err = fixture.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	if !strings.Contains(dot, `"n1" [label="part pump\nPump", pos="225,570!", pin=true];`) {
		t.Errorf("the collapsed pump is not a pinned leaf with its label:\n%s", dot)
	}
}

// A placed cluster cannot be pinned itself: its anchor node is pinned at its
// centre, and when it is sized the subgraph states its box as `bb`, which
// `neato -n` draws the cluster from.
func TestDOTClusterGeometryOnAnchor(t *testing.T) {
	rendering := &Rendering{
		View: "V",
		Kind: KindState,
		Roots: []*Node{
			{ID: "n0", Kind: "state def", Name: "M", Geometry: &Geometry{X: 0, Y: 0, Width: 400, Height: 200, HasSize: true}, Children: []*Node{
				{ID: "n1", Kind: "region", Name: "r", Geometry: &Geometry{X: 20, Y: 40, Width: 200, Height: 100, HasSize: true}, Children: []*Node{
					{ID: "n2", Kind: "state", Name: "s", Geometry: &Geometry{X: 40, Y: 60, Width: 80, Height: 40, HasSize: true}},
				}},
				{ID: "n3", Kind: "state", Name: "t", Geometry: &Geometry{X: 300, Y: 60, Width: 80, Height: 40, HasSize: true}},
			}},
		},
		Edges:  []Edge{{From: "n1", To: "n3", Kind: EdgeTransition, Label: "go"}},
		Canvas: &Canvas{Unit: "px", Width: 400, Height: 200, HasSize: true},
	}
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	want := `// view: V
// kind: state
// layout: neato -n
digraph "V" {
  graph [compound=true, bb="0,0,300,150"];
  node [shape=box];
  subgraph "cluster_n0" {
    label="state def M";
    bb="0,0,300,150";
    "n0" [shape=point, style=invis, width=0, height=0, label="", pos="150,75!", pin=true];
    subgraph "cluster_n1" {
      label="region r";
      style=dashed;
      bb="15,45,165,120";
      "n1" [shape=point, style=invis, width=0, height=0, label="", pos="90,82.5!", pin=true];
      "n2" [shape=box, style=rounded, label="state s", pos="60,90!", pin=true, width=0.8333, height=0.4167, fixedsize=true];
    }
    "n3" [shape=box, style=rounded, label="state t", pos="255,90!", pin=true, width=0.8333, height=0.4167, fixedsize=true];
  }
  "n1" -> "n3" [label="go", ltail="cluster_n1"];
}
`
	if dot != want {
		t.Errorf("DOT:\n%s\nwant:\n%s", dot, want)
	}
	// A placed cluster without a size pins its anchor and states no bb, which
	// leaves neato -n nothing to draw the cluster from; the header says so.
	rendering.Roots[0].Children[0].Geometry.HasSize = false
	dot, err = rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	if !strings.Contains(dot, "// not represented: the box of n1, which has no size (neato -n draws a cluster from its bb alone)\n") {
		t.Errorf("unsized cluster is not noticed:\n%s", dot)
	}
	if !strings.Contains(dot, `"n1" [shape=point, style=invis, width=0, height=0, label="", pos="15,120!", pin=true];`) || strings.Contains(dot, `bb="15,`) {
		t.Errorf("unsized cluster anchor:\n%s", dot)
	}
	// An unplaced cluster is not pinned: its anchor is set at the centre of its
	// placed members' extent so neato -n has a position for it, and it is noticed.
	rendering.Roots[0].Children[0].Geometry = nil
	dot, err = rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	if !strings.Contains(dot, `"n1" [shape=point, style=invis, width=0, height=0, label="", pos="60,90"];`) || strings.Contains(dot, "no position for") {
		t.Errorf("unplaced cluster anchor:\n%s", dot)
	}
	// A cluster with no placed member at all has no anchor position, and counts
	// as unplaced.
	rendering.Roots[0].Children[0].Children[0].Geometry = nil
	dot, err = rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	if !strings.Contains(dot, `"n1" [shape=point, style=invis, width=0, height=0, label=""];`) || !strings.Contains(dot, "// not represented: no position for 2 of 4 nodes") {
		t.Errorf("unplaced cluster with no placed member:\n%s", dot)
	}
}

// The layout golden is the same bytes on every write of the same rendering and
// of a fresh rendering of the model.
func TestDOTLayoutGoldenIsDeterministic(t *testing.T) {
	rendering := render(t, "layout.sysml", "PlantViews::placedView")
	first, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	second, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	fresh, err := render(t, "layout.sysml", "PlantViews::placedView").DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	if first != second || first != fresh {
		t.Errorf("DOT differs between writes:\n%s\n---\n%s\n---\n%s", first, second, fresh)
	}
	if !strings.Contains(first, `pos="`) {
		t.Errorf("layout DOT carries no position:\n%s", first)
	}
}

// checkDOTSyntax checks dot is well-formed without Graphviz: braces balance,
// no ID is bare, edges join declared nodes, lhead/ltail name declared clusters.
func checkDOTSyntax(t *testing.T, dot string) {
	t.Helper()
	tokens, err := tokenizeDOT(dot)
	if err != nil {
		t.Fatalf("DOT does not tokenize: %v\n%s", err, dot)
	}
	nodes := map[string]bool{}
	clusters := map[string]bool{}
	type edge struct{ from, to string }
	var edges []edge
	var clipped []string
	depth := 0
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		switch {
		case tok.text == "{" && !tok.quoted:
			depth++
		case tok.text == "}" && !tok.quoted:
			depth--
			if depth < 0 {
				t.Fatalf("DOT closes more braces than it opens:\n%s", dot)
			}
		case !tok.quoted && tok.text == "digraph":
		case !tok.quoted && (tok.text == "graph" || tok.text == "node"):
			i = checkDOTAttributes(t, tokens, i, dot, &clipped, false)
		case !tok.quoted && tok.text == "subgraph":
			if i+1 >= len(tokens) || !tokens[i+1].quoted {
				t.Fatalf("subgraph without a quoted name at token %d:\n%s", i, dot)
			}
			clusters[tokens[i+1].text] = true
			i++
		case tok.quoted && i+1 < len(tokens) && tokens[i+1].text == "->":
			if i+2 >= len(tokens) || !tokens[i+2].quoted {
				t.Fatalf("edge to an unquoted end at token %d:\n%s", i, dot)
			}
			edges = append(edges, edge{tok.text, tokens[i+2].text})
			i += 2
			if i+1 < len(tokens) && tokens[i+1].text == "[" {
				i = checkDOTAttributes(t, tokens, i, dot, &clipped, true)
			}
		case tok.quoted && i+1 < len(tokens) && tokens[i+1].text == "[":
			nodes[tok.text] = true
			i = checkDOTAttributes(t, tokens, i, dot, &clipped, false)
		case tok.quoted && i+1 < len(tokens) && tokens[i+1].text == "{":
			// The digraph's own name.
		case !tok.quoted && tok.text == "label" && i+2 < len(tokens) && tokens[i+1].text == "=":
			// A cluster's label statement.
			if !tokens[i+2].quoted {
				t.Fatalf("cluster label is not quoted at token %d:\n%s", i, dot)
			}
			i += 2
		case !tok.quoted && tok.text == "style" && i+2 < len(tokens) && tokens[i+1].text == "=":
			i += 2
		case !tok.quoted && tok.text == "bb" && i+2 < len(tokens) && tokens[i+1].text == "=":
			// A cluster's bounding box statement.
			checkDOTBoundingBox(t, tokens[i+2], dot)
			i += 2
		case !tok.quoted && tok.text == ";":
		default:
			t.Fatalf("unexpected token %q (quoted=%v) at %d, an ID written bare:\n%s", tok.text, tok.quoted, i, dot)
		}
	}
	if depth != 0 {
		t.Fatalf("DOT leaves %d braces open:\n%s", depth, dot)
	}
	for _, e := range edges {
		for _, end := range []string{e.from, e.to} {
			if !nodes[end] && !clusters[end] {
				t.Errorf("edge %q -> %q names %q, which no node or cluster declares:\n%s", e.from, e.to, end, dot)
			}
		}
	}
	for _, cluster := range clipped {
		if !clusters[cluster] {
			t.Errorf("edge is clipped at %q, which no subgraph declares:\n%s", cluster, dot)
		}
	}
}

// checkDOTAttributes checks the attribute list opening after tokens[i] and
// returns the index of its closing bracket, collecting lhead/ltail clusters. A
// `pos` parses as a pinned point on a node and as a spline on an edge.
func checkDOTAttributes(t *testing.T, tokens []dotToken, i int, dot string, clipped *[]string, edge bool) int {
	t.Helper()
	i++
	if i >= len(tokens) || tokens[i].text != "[" {
		t.Fatalf("expected an attribute list at token %d:\n%s", i, dot)
	}
	for i++; i < len(tokens) && tokens[i].text != "]"; i++ {
		if tokens[i].text == "," && !tokens[i].quoted {
			continue
		}
		if tokens[i].quoted || i+2 >= len(tokens) || tokens[i+1].text != "=" {
			t.Fatalf("attribute list is not name=value at token %d (%q):\n%s", i, tokens[i].text, dot)
		}
		name, value := tokens[i].text, tokens[i+2]
		if (name == "label" || name == "lhead" || name == "ltail") && !value.quoted {
			t.Fatalf("attribute %s has a bare value %q:\n%s", name, value.text, dot)
		}
		if name == "lhead" || name == "ltail" {
			*clipped = append(*clipped, value.text)
		}
		switch {
		case name == "pos" && edge:
			checkDOTSpline(t, value, dot)
		case name == "pos":
			// A placed node is pinned, `x,y!`; an unplaced cluster's anchor is set, `x,y`.
			if !value.quoted || !isDOTPoint(strings.TrimSuffix(value.text, "!")) {
				t.Fatalf("node pos %q is not a <num>,<num>[!] point:\n%s", value.text, dot)
			}
		case name == "bb":
			checkDOTBoundingBox(t, value, dot)
		case name == "width" || name == "height":
			if value.quoted || !isDOTNumber(value.text) {
				t.Fatalf("%s %q is not a bare number:\n%s", name, value.text, dot)
			}
		}
		i += 2
	}
	if i >= len(tokens) {
		t.Fatalf("attribute list never closes:\n%s", dot)
	}
	return i
}

// checkDOTSpline checks an edge's pos is a splineType: a quoted list of
// 3n+1 points, n ≥ 1.
func checkDOTSpline(t *testing.T, value dotToken, dot string) {
	t.Helper()
	points := strings.Split(value.text, " ")
	if !value.quoted || len(points) < 4 || len(points)%3 != 1 {
		t.Fatalf("edge pos %q is not 3n+1 points:\n%s", value.text, dot)
	}
	for _, p := range points {
		if !isDOTPoint(p) {
			t.Fatalf("edge pos point %q is not <num>,<num>:\n%s", p, dot)
		}
	}
}

// checkDOTBoundingBox checks a bb is a quoted `llx,lly,urx,ury` with the
// lower-left corner below and left of the upper-right one.
func checkDOTBoundingBox(t *testing.T, value dotToken, dot string) {
	t.Helper()
	parts := strings.Split(value.text, ",")
	if !value.quoted || len(parts) != 4 {
		t.Fatalf("bb %q is not llx,lly,urx,ury:\n%s", value.text, dot)
	}
	var corners [4]float64
	for i, part := range parts {
		v, err := strconv.ParseFloat(part, 64)
		if err != nil {
			t.Fatalf("bb %q holds %q, no number:\n%s", value.text, part, dot)
		}
		corners[i] = v
	}
	if corners[0] > corners[2] || corners[1] > corners[3] {
		t.Fatalf("bb %q is not lower-left then upper-right:\n%s", value.text, dot)
	}
}

// isDOTPoint reports whether text is `<num>,<num>`.
func isDOTPoint(text string) bool {
	x, y, ok := strings.Cut(text, ",")
	return ok && isDOTNumber(x) && isDOTNumber(y)
}

// isDOTNumber reports whether text is a decimal number.
func isDOTNumber(text string) bool {
	_, err := strconv.ParseFloat(text, 64)
	return err == nil && text != ""
}

// dotToken is one token of a DOT text: a quoted string with its escapes
// resolved, or a bare word or punctuation.
type dotToken struct {
	text   string
	quoted bool
}

// tokenizeDOT splits DOT into tokens, dropping `//` comments.
func tokenizeDOT(dot string) ([]dotToken, error) {
	var tokens []dotToken
	for i := 0; i < len(dot); i++ {
		c := dot[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n':
		case c == '/' && i+1 < len(dot) && dot[i+1] == '/':
			for i < len(dot) && dot[i] != '\n' {
				i++
			}
		case c == '"':
			var b strings.Builder
			i++
			for ; i < len(dot) && dot[i] != '"'; i++ {
				if dot[i] == '\\' {
					if i+1 >= len(dot) {
						return nil, errors.New("backslash ends the text")
					}
					i++
					if dot[i] == 'n' {
						b.WriteByte('\n')
						continue
					}
				}
				b.WriteByte(dot[i])
			}
			if i >= len(dot) {
				return nil, errors.New("unterminated quoted string")
			}
			tokens = append(tokens, dotToken{text: b.String(), quoted: true})
		case c == '-' && i+1 < len(dot) && dot[i+1] == '>':
			tokens = append(tokens, dotToken{text: "->"})
			i++
		case strings.ContainsRune("{}[];,=", rune(c)):
			tokens = append(tokens, dotToken{text: string(c)})
		case c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.':
			start := i
			for i+1 < len(dot) && (dot[i+1] == '_' || dot[i+1] == '.' || dot[i+1] >= 'a' && dot[i+1] <= 'z' || dot[i+1] >= 'A' && dot[i+1] <= 'Z' || dot[i+1] >= '0' && dot[i+1] <= '9') {
				i++
			}
			tokens = append(tokens, dotToken{text: dot[start : i+1]})
		default:
			return nil, fmt.Errorf("unexpected byte %q at %d", c, i)
		}
	}
	return tokens, nil
}
