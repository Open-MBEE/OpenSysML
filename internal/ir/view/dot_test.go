package view

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestGoldenDOT locks the DOT of each kind that has one, from the models the
// Mermaid goldens use, and checks each is well-formed DOT.
func TestGoldenDOT(t *testing.T) {
	cases := []struct {
		name string
		file string
		view string
		kind Kind
	}{
		{"tree", "tree.sysml", "VehicleViews::vehicleView", KindTree},
		{"interconnection", "interconnection.sysml", "PlantViews::loopView", KindInterconnection},
		{"state", "state.sysml", "MachineViews::vehicleStates", KindState},
		{"state-entry", "state-entry.sysml", "MachineViews::thermostat", KindState},
		{"action", "action.sysml", "FlowViews::driveView", KindAction},
		{"filters", "filters.sysml", "FilteredViews::safetyView", KindTree},
	}
	for _, tc := range cases {
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
			for _, want := range []string{"// view: " + tc.view, "// kind: " + string(tc.kind), "// layout: dot", "digraph " + dotQuote(tc.view) + " {", "node [shape=box, style=filled, fillcolor=white, color=\"#181818\", fontname=\"Helvetica\", fontsize=14, penwidth=0.5];", "edge [color=\"#181818\", fontname=\"Helvetica\", fontsize=13, penwidth=1];"} {
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
		if !kind.SupportsForm(kind.MachineForm()) {
			t.Errorf("%s does not support its machine form", kind)
		}
		if !kind.SupportsForm(FormText) {
			t.Errorf("%s does not support the text form", kind)
		}
		if kind.MachineForm() != FormMermaid && kind.SupportsForm(FormMermaid) || kind.MachineForm() != FormMarkdown && kind.SupportsForm(FormMarkdown) {
			t.Errorf("%s supports a machine form that is not its own", kind)
		}
	}
	if got := KindTree.SupportedForms(); fmt.Sprint(got) != "[text mermaid dot plantuml]" {
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
	if err == nil || !strings.Contains(err.Error(), "ask for text, mermaid, dot or plantuml") {
		t.Errorf("markdown of a tree error = %v, want it to offer dot", err)
	}
	_, err = render(t, "tree.sysml", "VehicleViews::vehicleView").Write("svg")
	if err == nil || !strings.Contains(err.Error(), "text, mermaid, markdown, dot and plantuml") {
		t.Errorf("unknown form error = %v, want it to list dot", err)
	}
}

// Every identifier and edge label is quoted with its quotes and backslashes
// escaped; a node or cluster label is an HTML string with markup as entities.
func TestDOTQuotesEveryIdentifierAndLabel(t *testing.T) {
	rendering := &Rendering{
		View: `Odd::view "quoted" and \backslashed`,
		Kind: KindInterconnection,
		Roots: []*Node{
			{ID: `a"b`, Kind: "part", Name: `say "hi"`, Detail: `C:\path`},
			{ID: `c\d`, Kind: "part", Name: "plain", Type: "A<B> & C", Children: []*Node{
				{ID: "e", Kind: "port", Name: "line one\nline two"},
			}},
		},
		Edges: []Edge{{From: `a"b`, To: "e", Label: `"quoted" \ <label> & more`, Kind: EdgeConnection}},
	}
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	for _, want := range []string{
		`digraph "Odd::view \"quoted\" and \\backslashed" {`,
		`"a\"b" [style="rounded,filled", label=<<b>say &#34;hi&#34;</b><br/><font point-size="10"><i>«part»</i></font><br/>C:\path>];`,
		`subgraph "cluster_c\\d" {`,
		`label=<<b>plain : A&lt;B&gt; &amp; C</b><br/><font point-size="10"><i>«part»</i></font>>;`,
		`"e" [style="rounded,filled", label=<<b>line one<br/>line two</b><br/><font point-size="10"><i>«port»</i></font>>];`,
		`"a\"b" -> "e" [label="\"quoted\" \\ <label> & more", arrowhead=none, penwidth=3];`,
	} {
		if !strings.Contains(dot, want) {
			t.Errorf("DOT lacks %q:\n%s", want, dot)
		}
	}
	if got := dotQuote(`a"b\c` + "\n"); got != `"a\"b\\c\n"` {
		t.Errorf("dotQuote = %s", got)
	}
	if got := dotEscape(`<a> & "b"` + "\nc"); got != `&lt;a&gt; &amp; &#34;b&#34;<br/>c` {
		t.Errorf("dotEscape = %s", got)
	}
}

// A label is the bold name (` : Type` for a typed usage), the kind in
// guillemets at 10pt, then the notes; a nameless node leads with its kind.
func TestDOTLabelShape(t *testing.T) {
	cases := []struct {
		name string
		node *Node
		want string
	}{
		{"typed usage", &Node{Kind: "part", Name: "pump", Type: "Pump"},
			`<<b>pump : Pump</b><br/><font point-size="10"><i>«part»</i></font>>`},
		{"untyped usage", &Node{Kind: "attribute", Name: "power"},
			`<<b>power</b><br/><font point-size="10"><i>«attribute»</i></font>>`},
		{"definition", &Node{Kind: "part def", Name: "Plant::Loop"},
			`<<b>Plant::Loop</b><br/><font point-size="10"><i>«part def»</i></font>>`},
		{"name-less", &Node{Kind: "connect"}, `<<b>connect</b>>`},
		{"name-less typed", &Node{Kind: "part", Type: "Pump"}, `<<b>part</b>>`},
		{"name-less with note", &Node{Kind: "connect", Detail: "already shown"}, `<<b>connect</b><br/>already shown>`},
		{"notes", &Node{Kind: "part", Name: "sensor", Type: "Pump", Detail: "already shown as n1, collapsed"},
			`<<b>sensor : Pump</b><br/><font point-size="10"><i>«part»</i></font><br/>already shown as n1, collapsed>`},
		{"state note", &Node{Kind: "state", Name: "off", Detail: "initial"},
			`<<b>off</b><br/><font point-size="10"><i>«state»</i></font><br/>initial>`},
		{"escaped", &Node{Kind: "part", Name: `a<b> & "c"`, Type: "T<U>", Detail: "x > y"},
			`<<b>a&lt;b&gt; &amp; &#34;c&#34; : T&lt;U&gt;</b><br/><font point-size="10"><i>«part»</i></font><br/>x &gt; y>`},
	}
	for _, tc := range cases {
		if got := dotLabel(tc.node); got != "label="+tc.want {
			t.Errorf("%s: dotLabel = %s, want %s", tc.name, got, tc.want)
		}
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
  graph [fontname="Helvetica", compound=true];
  node [shape=box, style=filled, fillcolor=white, color="#181818", fontname="Helvetica", fontsize=14, penwidth=0.5];
  edge [color="#181818", fontname="Helvetica", fontsize=13, penwidth=1];
  subgraph "cluster_n0" {
    label=<<b>Outer</b><br/><font point-size="10"><i>«part def»</i></font>>;
    color=black;
    penwidth=0.5;
    "n0" [shape=point, style=invis, width=0, height=0, label=""];
    subgraph "cluster_n1" {
      label=<<b>inner</b><br/><font point-size="10"><i>«part»</i></font>>;
      color=black;
      penwidth=0.5;
      "n1" [shape=point, style=invis, width=0, height=0, label=""];
      "n2" [style="rounded,filled", label=<<b>p</b><br/><font point-size="10"><i>«port»</i></font>>];
      "n3" [style="rounded,filled", label=<<b>q</b><br/><font point-size="10"><i>«port»</i></font>>];
    }
    "n4" [style="rounded,filled", label=<<b>leaf</b><br/><font point-size="10"><i>«part»</i></font>>];
  }
  "n5" [label=<<b>Other</b><br/><font point-size="10"><i>«part def»</i></font>>];
  "n4" -> "n1" [arrowhead=none, penwidth=3, lhead="cluster_n1"];
  "n1" -> "n5" [label="out", style=dashed, ltail="cluster_n1"];
  "n2" -> "n3" [arrowhead=none, penwidth=3];
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
	for _, want := range []string{`"n0" -> "n1" [arrowhead=none];`, `"n1" -> "n2" [arrowhead=none];`, `"n4" -> "n1" [arrowhead=none, penwidth=3];`} {
		if !strings.Contains(dot, want) {
			t.Errorf("tree DOT lacks %q:\n%s", want, dot)
		}
	}
}

// Each direction is the graph's rankdir, and no direction leaves it out.
func TestDOTDirections(t *testing.T) {
	rendering := render(t, "state.sysml", "MachineViews::vehicleStates")
	for _, direction := range []Direction{DirectionTopBottom, DirectionLeftRight, DirectionRightLeft, DirectionBottomTop} {
		dot, err := rendering.DOTWith(Options{Direction: direction})
		if err != nil {
			t.Fatalf("DOTWith(%s): %v", direction, err)
		}
		checkDOTSyntax(t, dot)
		if want := "  graph [fontname=\"Helvetica\", rankdir=" + string(direction) + ", compound=true];\n"; !strings.Contains(dot, want) {
			t.Errorf("DOTWith(%s) lacks %q:\n%s", direction, want, dot)
		}
	}
	dot, err := rendering.DOTWith(Options{})
	if err != nil {
		t.Fatalf("DOTWith(Options{}): %v", err)
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
	if strings.Contains(dot, "graph [fontname=\"Helvetica\", ") {
		t.Errorf("plain DOT writes a graph attribute list:\n%s", dot)
	}
}

// Every edge kind is drawn in the style parallel to its Mermaid arrow.
func TestDOTEdgeKinds(t *testing.T) {
	styles := map[EdgeKind]string{
		EdgeConnection: `"a" -> "b" [label="k", arrowhead=none, penwidth=3];`,
		EdgeTransition: `"a" -> "b" [label="k"];`,
		EdgeSuccession: `"a" -> "b" [label="k"];`,
		EdgeFlow:       `"a" -> "b" [label="k", style=dashed];`,
		EdgeBinding:    `"a" -> "b" [label="k", arrowhead=none];`,
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
	if got := strings.Count(dot, "[shape=point, fillcolor=black, label=\"\"]"); got != starts || starts == 0 {
		t.Errorf("DOT draws %d start points, want %d:\n%s", got, starts, dot)
	}
	for _, want := range []string{
		`[style="rounded,filled", label=<<b>heating</b><br/><font point-size="10"><i>«state»</i></font>>];`,
		`subgraph "cluster_n6" {`,
		`label=<<b>lights</b><br/><font point-size="10"><i>«region»</i></font>>;`,
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
	// An action's initial and final nodes are circles; a state's detail is the
	// line after its keyword.
	final := &Rendering{View: "V", Kind: KindAction, Roots: []*Node{
		{ID: "a", Kind: "final", Name: "done"}, {ID: "b", Kind: "state", Name: "off", Detail: "initial"}, {ID: "c", Kind: "initial", Name: "go"}}}
	dot, err = final.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	for _, want := range []string{
		`"a" [shape=doublecircle, label=<<b>done</b><br/><font point-size="10"><i>«final»</i></font>>];`,
		`"b" [style="rounded,filled", label=<<b>off</b><br/><font point-size="10"><i>«state»</i></font><br/>initial>];`,
		`"c" [shape=circle, label=<<b>go</b><br/><font point-size="10"><i>«initial»</i></font>>];`,
	} {
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
  graph [fontname="Helvetica"];
  node [shape=box, style=filled, fillcolor=white, color="#181818", fontname="Helvetica", fontsize=14, penwidth=0.5];
  edge [color="#181818", fontname="Helvetica", fontsize=13, penwidth=1];
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

// The DiagramLayout geometry is written as Graphviz reads it: a positioned node
// is pinned at its centre in points, y up from the canvas's bottom edge, sized
// in inches — fixed when the Layout sizes it, fitted to its label when not; a
// route is a `pos` spline through its waypoints; the canvas is held by a pinned
// point at each corner; the header names `neato` while a node is unpositioned.
func TestDOTWritesTheGeometry(t *testing.T) {
	rendering := render(t, "layout.sysml", "PlantViews::placedView")
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkGolden(t, filepath.Join("testdata", "layout.dot.golden"), dot)
	checkDOTSyntax(t, dot)
	for _, want := range []string{
		"// canvas: unit=px w=1200 h=800\n// layout: neato\n",
		"  graph [fontname=\"Helvetica\", inputscale=72, dpi=72];\n  node [shape=box, style=filled, fillcolor=white, color=\"#181818\", fontname=\"Helvetica\", fontsize=14, penwidth=0.5];\n  edge [color=\"#181818\", fontname=\"Helvetica\", fontsize=13, penwidth=1];\n  \"canvas:0\" [shape=point, style=invis, width=0, height=0, label=\"\", pos=\"0,800!\", pin=true];\n  \"canvas:1\" [shape=point, style=invis, width=0, height=0, label=\"\", pos=\"1200,0!\", pin=true];\n  subgraph",
		// pump: top-left (300, 40), no size, so the centre of a 109x37 box fitted
		// to eleven 14pt glyphs over a 10pt keyword line, stated but not fixed, collapsed.
		`"n1" [style="rounded,filled", label=<<b>pump : Pump</b><br/><font point-size="10"><i>«part»</i></font>>, pos="359,741.5!", pin=true, width=1.6388888888888888, height=0.5138888888888888, comment="collapsed"];`,
		// tank: top-left (500, 40), 120x60, so centre (560, 70) -> y 730 from a canvas 800 high.
		`"n2" [style="rounded,filled", label=<<b>tank : Tank</b><br/><font point-size="10"><i>«part»</i></font>>, pos="560,730!", pin=true, width=1.6666666666666667, height=0.8333333333333334, fixedsize=true];`,
		`"n1" -> "n2" [label="supply", arrowhead=none, penwidth=3, pos="400,730 400,730 450,680 450,680 450,680 500,730 500,730"];`,
	} {
		if !strings.Contains(dot, want) {
			t.Errorf("DOT lacks %q:\n%s", want, dot)
		}
	}
	// Without a canvas height, y is negated; the inline Layout of pump is kept
	// and the unpositioned tank leaves the graph to neato.
	plain, err := render(t, "layout.sysml", "PlantViews::plainView").DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, plain)
	for _, want := range []string{
		"// layout: neato\ndigraph",
		"  graph [fontname=\"Helvetica\", inputscale=72, dpi=72];\n",
		`"n1" [style="rounded,filled", label=<<b>pump : Pump</b><br/><font point-size="10"><i>«part»</i></font>>, pos="60,-45!", pin=true, width=1.3888888888888888, height=0.6944444444444444, fixedsize=true];`,
		`"n2" [style="rounded,filled", label=<<b>tank : Tank</b><br/><font point-size="10"><i>«part»</i></font>>];`,
		`pos="60,-45 60,-45 200,-45 200,-45"`,
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("plain DOT lacks %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "canvas") {
		t.Errorf("plain DOT states a canvas it has none of:\n%s", plain)
	}
	// States and transitions are placed the same way; a pseudo-state is not.
	machine, err := render(t, "layout.sysml", "PlantViews::machineView").DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, machine)
	for _, want := range []string{
		`"n3" [shape=point, fillcolor=black, label=""];`,
		// off: three lines, 14pt, 10pt and 14pt, so a 75x54 box from its top-left (0, 0).
		`[style="rounded,filled", label=<<b>off</b><br/><font point-size="10"><i>«state»</i></font><br/>initial>, pos="37.5,-27!", pin=true, width=1.0416666666666667, height=0.75];`,
		`[style="rounded,filled", label=<<b>on</b><br/><font point-size="10"><i>«state»</i></font>>, pos="40,-120!", pin=true, width=1.1111111111111112, height=0.5555555555555556, fixedsize=true];`,
		`"n1" -> "n2" [label="off_on", pos="50,-10 50,-10 50,-90 50,-90"];`,
		`"n2" -> "n1" [pos="30,-90 30,-90 30,-10 30,-10"];`,
	} {
		if !strings.Contains(machine, want) {
			t.Errorf("machine DOT lacks %q:\n%s", want, machine)
		}
	}
	// The Mermaid form keeps the same geometry as comments, so neither drops it.
	mermaid := rendering.Mermaid()
	for _, want := range []string{"%% layout: n1 x=300 y=40 collapsed", "%% layout: n2 x=500 y=40 w=120 h=60", "%% route: n1->n2 400,70 450,120 500,70"} {
		if !strings.Contains(mermaid, want) {
			t.Errorf("Mermaid lacks %q:\n%s", want, mermaid)
		}
	}
}

// A graph whose every node is positioned is written for `neato -n`, and for
// `neato -n2` once any edge is routed; a route of one waypoint is noticed,
// as are routes while a node is left for `neato` to place; a cluster
// states its box — the stated one, or the one from its corner round its
// members — and pins its anchor at the centre; a tree pins the node itself.
func TestDOTPinsEveryNode(t *testing.T) {
	rendering := &Rendering{
		View:   "Pinned::view",
		Kind:   KindInterconnection,
		Canvas: &Canvas{Unit: "px", Width: 400, Height: 300, HasSize: true},
		Roots: []*Node{
			{ID: "n0", Kind: "part def", Name: "Outer", Geometry: &Geometry{X: 10, Y: 20, Width: 200, Height: 100, HasSize: true, Collapsed: true}, Children: []*Node{
				{ID: "n1", Kind: "part", Name: "a", Geometry: &Geometry{X: 20, Y: 30, Width: 72, Height: 36, HasSize: true}},
				{ID: "n2", Kind: "part", Name: "b", Geometry: &Geometry{X: 120, Y: 30}},
			}},
			{ID: "n3", Kind: "part def", Name: "Other", Geometry: &Geometry{X: 300, Y: 200}, Children: []*Node{
				{ID: "n4", Kind: "port", Name: "p", Geometry: &Geometry{X: 310, Y: 210}},
			}},
		},
		Edges: []Edge{
			{From: "n1", To: "n2", Kind: EdgeConnection, Route: []Point{{X: 92, Y: 48}, {X: 120, Y: 48}}},
			{From: "n2", To: "n3", Kind: EdgeFlow, Route: []Point{{X: 174, Y: 48}}},
		},
	}
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	want := `// view: Pinned::view
// kind: interconnection
// not represented: route of n2->n3 is one waypoint, (174, 48); a line needs two
// canvas: unit=px w=400 h=300
// layout: neato -n2
digraph "Pinned::view" {
  graph [fontname="Helvetica", compound=true, inputscale=72, dpi=72];
  node [shape=box, style=filled, fillcolor=white, color="#181818", fontname="Helvetica", fontsize=14, penwidth=0.5];
  edge [color="#181818", fontname="Helvetica", fontsize=13, penwidth=1];
  "canvas:0" [shape=point, style=invis, width=0, height=0, label="", pos="0,300!", pin=true];
  "canvas:1" [shape=point, style=invis, width=0, height=0, label="", pos="400,0!", pin=true];
  subgraph "cluster_n0" {
    label=<<b>Outer</b><br/><font point-size="10"><i>«part def»</i></font>>;
    color=black;
    penwidth=0.5;
    bb="10,180,210,280";
    comment="collapsed";
    "n0" [shape=point, style=invis, width=0, height=0, label="", pos="110,230!", pin=true];
    "n1" [style="rounded,filled", label=<<b>a</b><br/><font point-size="10"><i>«part»</i></font>>, pos="56,252!", pin=true, width=1, height=0.5, fixedsize=true];
    "n2" [style="rounded,filled", label=<<b>b</b><br/><font point-size="10"><i>«part»</i></font>>, pos="147,251.5!", pin=true, width=0.75, height=0.5138888888888888];
  }
  subgraph "cluster_n3" {
    label=<<b>Other</b><br/><font point-size="10"><i>«part def»</i></font>>;
    color=black;
    penwidth=0.5;
    bb="300,45,372,100";
    "n3" [shape=point, style=invis, width=0, height=0, label="", pos="336,72.5!", pin=true];
    "n4" [style="rounded,filled", label=<<b>p</b><br/><font point-size="10"><i>«port»</i></font>>, pos="337,71.5!", pin=true, width=0.75, height=0.5138888888888888];
  }
  "n1" -> "n2" [arrowhead=none, penwidth=3, pos="92,252 92,252 120,252 120,252"];
  "n2" -> "n3" [style=dashed, lhead="cluster_n3"];
}
`
	if dot != want {
		t.Errorf("DOT:\n%s\nwant:\n%s", dot, want)
	}
	// Routing the last edge leaves nothing to notice.
	rendering.Edges[1].Route = append(rendering.Edges[1].Route, Point{X: 300, Y: 220})
	dot, err = rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	if !strings.Contains(dot, "// layout: neato -n2\n") || !strings.Contains(dot, `"n2" -> "n3" [style=dashed, pos="174,252 174,252 300,80 300,80", lhead="cluster_n3"];`) || strings.Contains(dot, "not represented") {
		t.Errorf("fully routed DOT:\n%s", dot)
	}
	// Leaving a node unpositioned hands the graph to `neato`, which redraws
	// the routes it is given, so they are noticed.
	rendering.Roots[1].Children[0].Geometry = nil
	dot, err = rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	if !strings.Contains(dot, "// not represented: 2 route(s) written as pos; neato redraws every edge, only neato -n2 keeps them\n// canvas: unit=px w=400 h=300\n// layout: neato\n") {
		t.Errorf("partly positioned DOT:\n%s", dot)
	}
	rendering.Roots[1].Children[0].Geometry = &Geometry{X: 310, Y: 210}
	// A tree has no cluster, so the node with children is pinned itself and no
	// `bb` is written; the canvas corners are still pinned. Its containment edges
	// have no route, which `neato -n2` draws beside the routed ones.
	rendering.Kind = KindTree
	dot, err = rendering.DOT()
	if err != nil {
		t.Fatalf("DOT of tree: %v", err)
	}
	checkDOTSyntax(t, dot)
	if strings.Contains(dot, "not represented") {
		t.Errorf("tree DOT notices something:\n%s", dot)
	}
	for _, want := range []string{
		"// layout: neato -n2\n",
		"  graph [fontname=\"Helvetica\", inputscale=72, dpi=72];\n  node [shape=box, style=filled, fillcolor=white, color=\"#181818\", fontname=\"Helvetica\", fontsize=14, penwidth=0.5];\n  edge [color=\"#181818\", fontname=\"Helvetica\", fontsize=13, penwidth=1];\n  \"canvas:0\" [shape=point, style=invis, width=0, height=0, label=\"\", pos=\"0,300!\", pin=true];\n  \"canvas:1\" [shape=point, style=invis, width=0, height=0, label=\"\", pos=\"400,0!\", pin=true];\n  \"n0\"",
		`"n0" [label=<<b>Outer</b><br/><font point-size="10"><i>«part def»</i></font>>, pos="110,230!", pin=true, width=2.7777777777777777, height=1.3888888888888888, fixedsize=true, comment="collapsed"];`,
		`"n3" [label=<<b>Other</b><br/><font point-size="10"><i>«part def»</i></font>>, pos="338,81.5!", pin=true, width=1.0555555555555556, height=0.5138888888888888];`,
	} {
		if !strings.Contains(dot, want) {
			t.Errorf("tree DOT lacks %q:\n%s", want, dot)
		}
	}
	if strings.Contains(dot, "bb=") {
		t.Errorf("tree DOT states a cluster box:\n%s", dot)
	}
	// Pseudo-states are centred on their own shapes: a point's fixed size, a
	// circle round the label's diagonal.
	pseudo := &Rendering{View: "V", Kind: KindState, Roots: []*Node{
		{ID: "s", Kind: startKind, Geometry: &Geometry{X: 0, Y: 0}},
		{ID: "i", Kind: "initial", Name: "go", Geometry: &Geometry{X: 100, Y: 0}},
		{ID: "f", Kind: "final", Name: "done", Geometry: &Geometry{X: 200, Y: 0, Width: 10, Height: 10, HasSize: true}},
	}}
	dot, err = pseudo.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	for _, want := range []string{
		`"s" [shape=point, fillcolor=black, label="", pos="1.8,-1.8!", pin=true];`,
		`"i" [shape=circle, label=<<b>go</b><br/><font point-size="10"><i>«initial»</i></font>>, pos="140,-40!", pin=true, width=1.1111111111111112, height=1.1111111111111112];`,
		`"f" [shape=doublecircle, label=<<b>done</b><br/><font point-size="10"><i>«final»</i></font>>, pos="205,-5!", pin=true, width=0.1388888888888889, height=0.1388888888888889, fixedsize=true];`,
	} {
		if !strings.Contains(dot, want) {
			t.Errorf("pseudo-state DOT lacks %q:\n%s", want, dot)
		}
	}
	// A canvas of zero extent is still a size: both corners at the origin, y
	// flipped against nothing.
	zero := &Rendering{View: "V", Kind: KindTree, Canvas: &Canvas{Unit: "mm", HasSize: true},
		Roots: []*Node{{ID: "n0", Kind: "part", Name: "a", Geometry: &Geometry{X: 27, Y: 18}}}}
	dot, err = zero.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	for _, want := range []string{"// canvas: unit=mm w=0 h=0\n", `"canvas:0" [shape=point, style=invis, width=0, height=0, label="", pos="0,0!", pin=true];`, `"canvas:1" [shape=point, style=invis, width=0, height=0, label="", pos="0,0!", pin=true];`, `pos="54,-36.5!"`} {
		if !strings.Contains(dot, want) {
			t.Errorf("zero-canvas DOT lacks %q:\n%s", want, dot)
		}
	}
	// A cluster with a corner but no positioned member has no box to state;
	// its anchor is pinned at the corner and neato places the members.
	corner := &Rendering{View: "V", Kind: KindInterconnection, Roots: []*Node{
		{ID: "n0", Kind: "part def", Name: "Outer", Geometry: &Geometry{X: 30, Y: 40}, Children: []*Node{{ID: "n1", Kind: "part", Name: "a"}}},
	}}
	dot, err = corner.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	if !strings.Contains(dot, "// layout: neato\n") || strings.Contains(dot, "bb=") || !strings.Contains(dot, `"n0" [shape=point, style=invis, width=0, height=0, label="", pos="30,-40!", pin=true];`) {
		t.Errorf("corner-only cluster DOT:\n%s", dot)
	}
	// A canvas with a unit alone is named in the header and pins nothing, and
	// so does a sized one while no node is positioned for it to hold.
	unit := &Rendering{View: "V", Kind: KindTree, Canvas: &Canvas{Unit: "px"}, Roots: []*Node{{ID: "n0", Kind: "part", Name: "a"}}}
	dot, err = unit.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	if !strings.Contains(dot, "// canvas: unit=px\n// layout: dot\n") || strings.Contains(dot, "graph [fontname=\"Helvetica\", ") || strings.Contains(dot, `"canvas:0"`) {
		t.Errorf("unit-only canvas DOT:\n%s", dot)
	}
	unit.Canvas = &Canvas{Unit: "px", Width: 400, Height: 300, HasSize: true}
	dot, err = unit.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	if !strings.Contains(dot, "// canvas: unit=px w=400 h=300\n// layout: dot\n") || strings.Contains(dot, "graph [fontname=\"Helvetica\", ") || strings.Contains(dot, `"canvas:0"`) {
		t.Errorf("unpositioned sized-canvas DOT:\n%s", dot)
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
		case !tok.quoted && (tok.text == "graph" || tok.text == "node" || tok.text == "edge"):
			i = checkDOTAttributes(t, tokens, i, dot, &clipped)
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
				i = checkDOTAttributes(t, tokens, i, dot, &clipped)
			}
		case tok.quoted && i+1 < len(tokens) && tokens[i+1].text == "[":
			nodes[tok.text] = true
			i = checkDOTAttributes(t, tokens, i, dot, &clipped)
		case tok.quoted && i+1 < len(tokens) && tokens[i+1].text == "{":
			// The digraph's own name.
		case !tok.quoted && i+2 < len(tokens) && tokens[i+1].text == "=":
			// A cluster's attribute statement: label, style, color, penwidth, bb, comment.
			value := tokens[i+2]
			if (tok.text == "label" || tok.text == "bb" || tok.text == "comment") && !value.quoted {
				t.Fatalf("cluster %s is not quoted at token %d:\n%s", tok.text, i, dot)
			}
			if value.html {
				if tok.text != "label" {
					t.Fatalf("cluster %s is an HTML string, which only a label may be:\n%s", tok.text, dot)
				}
				checkDOTHTMLLabel(t, value.text, dot)
			}
			if tok.text == "bb" {
				checkDOTGeometry(t, "bb", value.text, dot)
			}
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
// returns the index of its closing bracket, collecting lhead/ltail clusters.
func checkDOTAttributes(t *testing.T, tokens []dotToken, i int, dot string, clipped *[]string) int {
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
		if (name == "label" || name == "lhead" || name == "ltail" || name == "pos" || name == "bb" || name == "comment") && !value.quoted {
			t.Fatalf("attribute %s has a bare value %q:\n%s", name, value.text, dot)
		}
		if value.html {
			if name != "label" {
				t.Fatalf("attribute %s is an HTML string, which only a label may be:\n%s", name, dot)
			}
			checkDOTHTMLLabel(t, value.text, dot)
		}
		if name == "lhead" || name == "ltail" {
			*clipped = append(*clipped, value.text)
		}
		if name == "pos" || name == "bb" {
			checkDOTGeometry(t, name, value.text, dot)
		}
		if name == "width" || name == "height" || name == "inputscale" || name == "dpi" || name == "penwidth" || name == "fontsize" {
			if v, err := strconv.ParseFloat(value.text, 64); err != nil || v < 0 {
				t.Fatalf("attribute %s=%q is not a non-negative number:\n%s", name, value.text, dot)
			}
		}
		i += 2
	}
	if i >= len(tokens) {
		t.Fatalf("attribute list never closes:\n%s", dot)
	}
	return i
}

// checkDOTHTMLLabel checks an HTML-like label as Graphviz reads it: only the
// tags the writer uses, properly nested, and no bare `<`, `>` or `&` in the text.
func checkDOTHTMLLabel(t *testing.T, label, dot string) {
	t.Helper()
	var open []string
	for i := 0; i < len(label); i++ {
		switch label[i] {
		case '<':
			end := strings.IndexByte(label[i:], '>')
			if end < 0 {
				t.Fatalf("HTML label %q: tag never closes:\n%s", label, dot)
			}
			tag := label[i+1 : i+end]
			i += end
			switch {
			case tag == "br/":
			case tag == "b" || tag == "i" || strings.HasPrefix(tag, `font point-size="`) && strings.HasSuffix(tag, `"`):
				open = append(open, strings.Fields(tag)[0])
			case strings.HasPrefix(tag, "/"):
				if len(open) == 0 || open[len(open)-1] != tag[1:] {
					t.Fatalf("HTML label %q: </%s> closes nothing open:\n%s", label, tag[1:], dot)
				}
				open = open[:len(open)-1]
			default:
				t.Fatalf("HTML label %q: unexpected tag <%s>:\n%s", label, tag, dot)
			}
		case '>':
			t.Fatalf("HTML label %q: bare > at %d:\n%s", label, i, dot)
		case '&':
			end := strings.IndexByte(label[i:], ';')
			if end < 0 {
				t.Fatalf("HTML label %q: bare & at %d:\n%s", label, i, dot)
			}
			entity := label[i+1 : i+end]
			if entity != "amp" && entity != "lt" && entity != "gt" && entity != "quot" && (!strings.HasPrefix(entity, "#") || len(entity) < 2) {
				t.Fatalf("HTML label %q: unknown entity &%s;:\n%s", label, entity, dot)
			}
			i += end
		}
	}
	if len(open) > 0 {
		t.Fatalf("HTML label %q leaves <%s> open:\n%s", label, open[len(open)-1], dot)
	}
}

// checkDOTGeometry checks a geometry attribute's value as Graphviz reads it: a
// node `pos` is one pinned point, an edge `pos` a cubic B-spline of 3n+1
// points, `bb` two corners.
func checkDOTGeometry(t *testing.T, name, value, dot string) {
	t.Helper()
	points := strings.Fields(value)
	for i, point := range points {
		if name == "pos" && len(points) == 1 {
			point = strings.TrimSuffix(point, "!")
		}
		coords := strings.Split(point, ",")
		if name == "bb" && len(coords) == 4 || name != "bb" && len(coords) == 2 {
			for _, coord := range coords {
				if _, err := strconv.ParseFloat(coord, 64); err != nil {
					t.Fatalf("%s=%q: %q is no coordinate:\n%s", name, value, coord, dot)
				}
			}
			continue
		}
		t.Fatalf("%s=%q: point %d %q is malformed:\n%s", name, value, i, point, dot)
	}
	switch {
	case len(points) == 0:
		t.Fatalf("%s is empty:\n%s", name, dot)
	case name != "pos" && len(points) != 1:
		t.Fatalf("%s=%q has %d points, want one:\n%s", name, value, len(points), dot)
	case name == "pos" && len(points) == 1 && !strings.HasSuffix(value, "!"):
		t.Fatalf("node pos=%q is not pinned:\n%s", value, dot)
	case name == "pos" && len(points) > 1 && len(points)%3 != 1:
		t.Fatalf("edge pos=%q has %d points, not 3n+1:\n%s", value, len(points), dot)
	}
}

// dotToken is one token of a DOT text: a quoted string with its escapes
// resolved, an HTML string with its markup kept, or a bare word or punctuation.
type dotToken struct {
	text   string
	quoted bool // a quoted or HTML string, which DOT reads as one ID
	html   bool // an HTML string, `<...>`
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
		case c == '<':
			// An HTML string runs to the `>` that balances its opening `<`.
			start, depth := i, 0
			for ; i < len(dot); i++ {
				if dot[i] == '<' {
					depth++
				} else if dot[i] == '>' {
					if depth--; depth == 0 {
						break
					}
				}
			}
			if i >= len(dot) {
				return nil, errors.New("unterminated HTML string")
			}
			tokens = append(tokens, dotToken{text: dot[start+1 : i], quoted: true, html: true})
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
