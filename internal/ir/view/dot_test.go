package view

import (
	"errors"
	"fmt"
	"math"
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
		{"name-less typed", &Node{Kind: "part", Type: "Pump"}, `<<b>: Pump</b><br/><font point-size="10"><i>«part»</i></font>>`},
		{"synthesized name", &Node{Kind: "action", Name: "call", Type: "doTracking", NameSynthesized: true},
			`<<b>: doTracking</b><br/><font point-size="10"><i>«action»</i></font>>`},
		{"synthesized name, untyped", &Node{Kind: "action", Name: "stamp2", NameSynthesized: true}, `<<b>action</b>>`},
		{"name-less with note", &Node{Kind: "connect", Detail: "already shown"}, `<<b>connect</b><br/>already shown>`},
		{"notes", &Node{Kind: "part", Name: "sensor", Type: "Pump", Detail: "already shown as n1, collapsed"},
			`<<b>sensor : Pump</b><br/><font point-size="10"><i>«part»</i></font><br/>already shown as n1, collapsed>`},
		{"state note", &Node{Kind: "state", Name: "off", Detail: "initial"},
			`<<b>off</b><br/><font point-size="10"><i>«state»</i></font><br/>initial>`},
		{"escaped", &Node{Kind: "part", Name: `a<b> & "c"`, Type: "T<U>", Detail: "x > y"},
			`<<b>a&lt;b&gt; &amp; &#34;c&#34; : T&lt;U&gt;</b><br/><font point-size="10"><i>«part»</i></font><br/>x &gt; y>`},
	}
	for _, tc := range cases {
		if got := (labeller{}).dotLabel(tc.node); got != "label="+tc.want {
			t.Errorf("%s: dotLabel = %s, want %s", tc.name, got, tc.want)
		}
	}
}

// The extent estimate measures every line of a head that breaks across lines
// in bold, and takes the keyword line to be the one after the last of them.
func TestDOTLabelExtentMultilineHead(t *testing.T) {
	l := labeller{}
	long := strings.Repeat("W", 60)
	broken := &Node{Kind: "action def", Name: `'x\n` + long + `'`}
	whole := &Node{Kind: "action def", Name: `'x` + long + `'`}
	brokenWidth, brokenHeight := l.dotLabelExtent(broken)
	wholeWidth, wholeHeight := l.dotLabelExtent(whole)
	if want := wholeWidth - 2*l.size()*dotBoldGlyphEm; math.Abs(brokenWidth-want) > 1e-9 {
		t.Errorf("width = %g, want %g: the second line is not measured in bold", brokenWidth, want)
	}
	if want := wholeHeight + l.size()*dotLineEm; brokenHeight != want {
		t.Errorf("height = %g, want %g: the second line is one head line, not the keyword", brokenHeight, want)
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
		if !strings.Contains(mermaid, ": "+mermaidTransitionText(edge.Label)) {
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
// point at each corner; the cluster round positioned members is boxed round
// them, so every node is positioned and the header names `neato -n2`.
func TestDOTWritesTheGeometry(t *testing.T) {
	rendering := render(t, "layout.sysml", "PlantViews::placedView")
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkGolden(t, filepath.Join("testdata", "layout.dot.golden"), dot)
	checkDOTSyntax(t, dot)
	for _, want := range []string{
		"// canvas: unit=px w=1200 h=800\n// layout: neato -n2\n",
		// Loop: no Layout, boxed a margin round pump and tank, its anchor at the centre.
		"    bb=\"292,692,628,768\";\n    \"n0\" [shape=point, style=invis, width=0, height=0, label=\"\", pos=\"460,730!\", pin=true];\n",
		"  graph [fontname=\"Helvetica\", layout=neato, inputscale=72, dpi=72];\n  node [shape=box, style=filled, fillcolor=white, color=\"#181818\", fontname=\"Helvetica\", fontsize=14, penwidth=0.5];\n  edge [color=\"#181818\", fontname=\"Helvetica\", fontsize=13, penwidth=1];\n  \"canvas:0\" [shape=point, style=invis, width=0, height=0, label=\"\", pos=\"0,800!\", pin=true];\n  \"canvas:1\" [shape=point, style=invis, width=0, height=0, label=\"\", pos=\"1200,0!\", pin=true];\n  subgraph",
		// pump: top-left (300, 40), no size, so the centre of a 109x37 box fitted
		// to eleven 14pt glyphs over a 10pt keyword line, stated but not fixed, collapsed.
		`"n1" [style="rounded,filled", label=<<b><b>pump : Pump</b><br/><font point-size="10"><i>«part»</i></font></b>>, fillcolor="#FFE8BD", color="#333333", fontname="Arial", fontsize=11, pos="359,741.5!", pin=true, width=1.6388888888888888, height=0.5138888888888888, comment="collapsed"];`,
		// tank: top-left (500, 40), 120x60, so centre (560, 70) -> y 730 from a canvas 800 high.
		`"n2" [style="rounded,filled", label=<<b>tank : Tank</b><br/><font point-size="10"><i>«part»</i></font>>, margin=0, pos="560,730!", pin=true, width=1.6666666666666667, height=0.8333333333333334, fixedsize=true];`,
		// Headless, so no `e,` point; the label sits up and right of the first leg.
		`"n1" -> "n2" [label="supply", arrowhead=none, penwidth=3, color="#0000FF", pos="400,730 400,730 450,680 450,680 450,680 500,730 500,730", lp="443.5,723.5"];`,
	} {
		if !strings.Contains(dot, want) {
			t.Errorf("DOT lacks %q:\n%s", want, dot)
		}
	}
	// Without a canvas height, y is negated; the inline Layout of pump is kept,
	// its head wrapped where eleven bold 14pt glyphs overrun the stated 100pt width,
	// and tank, with none, takes the end of the inline route: its 118x37
	// label-fitted box centred 59 back from (200, 45) along the route's last leg.
	plain, err := render(t, "layout.sysml", "PlantViews::plainView").DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, plain)
	for _, want := range []string{
		"// layout: neato -n2\ndigraph",
		"  graph [fontname=\"Helvetica\", layout=neato, inputscale=72, dpi=72];\n",
		`"n1" [style="rounded,filled", label=<<b>pump<br/>: Pump</b><br/><font point-size="10"><i>«part»</i></font>>, margin=0, fillcolor="#FFFFDC", pos="60,-45!", pin=true, width=1.3888888888888888, height=0.6944444444444444, fixedsize=true];`,
		`"n2" [style="rounded,filled", label=<<b>tank : Tank</b><br/><font point-size="10"><i>«part»</i></font>>, pos="259,-45!", pin=true, width=1.6388888888888888, height=0.5138888888888888];`,
		`pos="60,-45 60,-45 200,-45 200,-45"`,
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("plain DOT lacks %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "canvas") {
		t.Errorf("plain DOT states a canvas it has none of:\n%s", plain)
	}
	// States and transitions are placed the same way; the start, with neither
	// a Layout nor a routed transition, takes its place above the state it enters:
	// its 14.4pt dot 20 clear of off's top, on off's centre line. The routed
	// transitions end at `e,` points 10 short of their last waypoint, so Graphviz
	// draws their heads; the label of off_on sits right of its vertical run.
	machine, err := render(t, "layout.sysml", "PlantViews::machineView").DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, machine)
	if strings.Contains(machine, "not represented") {
		t.Errorf("machine DOT leaves the start unplaced:\n%s", machine)
	}
	for _, want := range []string{
		"// layout: neato -n2\ndigraph",
		// off: three lines, 14pt, 10pt and 14pt, so a 75x54 box from its top-left (0, 0).
		`[style="rounded,filled", label=<<b>off</b><br/><font point-size="10"><i>«state»</i></font><br/>initial>, pos="37.5,-27!", pin=true, width=1.0416666666666667, height=0.75];`,
		`[style="rounded,filled", label=<<b>on</b><br/><font point-size="10"><i>«state»</i></font>>, margin=0, fillcolor="#EBEBD7", pos="40,-120!", pin=true, width=1.1111111111111112, height=0.5555555555555556, fixedsize=true];`,
		`"n3" [shape=point, fillcolor=black, label="", pos="37.5,21.8!", pin=true];`,
		`bb="-8,-148,88,31.6";`,
		`"n3" -> "n1";`,
		`"n1" -> "n2" [label="off_on", color="#FF0000", pos="e,50,-90 50,-10 50,-10 50,-80 50,-80", lp="77.5,-50"];`,
		`"n2" -> "n1" [pos="e,30,-10 30,-90 30,-90 30,-20 30,-20"];`,
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
// as is a node left unplaced, which is not drawn; a cluster
// states its box — the stated one, or the one from its corner round its
// members — and pins its anchor at the centre; a stated cluster's label is
// fitted to the strip above its topmost stated member; a tree pins the node itself.
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
  graph [fontname="Helvetica", compound=true, layout=neato, inputscale=72, dpi=72];
  node [shape=box, style=filled, fillcolor=white, color="#181818", fontname="Helvetica", fontsize=14, penwidth=0.5];
  edge [color="#181818", fontname="Helvetica", fontsize=13, penwidth=1];
  "canvas:0" [shape=point, style=invis, width=0, height=0, label="", pos="0,300!", pin=true];
  "canvas:1" [shape=point, style=invis, width=0, height=0, label="", pos="400,0!", pin=true];
  subgraph "cluster_n0" {
    label=<<font point-size="8"><b>Outer</b></font>>;
    color=black;
    penwidth=0.5;
    bb="10,180,210,280";
    comment="collapsed";
    "n0" [shape=point, style=invis, width=0, height=0, label="", pos="110,230!", pin=true];
    "n1" [style="rounded,filled", label=<<b>a</b><br/><font point-size="10"><i>«part»</i></font>>, margin=0, pos="56,252!", pin=true, width=1, height=0.5, fixedsize=true];
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
	if !strings.Contains(dot, "// layout: neato -n2\n") || !strings.Contains(dot, `"n2" -> "n3" [style=dashed, pos="e,300,80 174,252 174,252 294,88 294,88", lhead="cluster_n3"];`) || strings.Contains(dot, "not represented") {
		t.Errorf("fully routed DOT:\n%s", dot)
	}
	// Leaving a node unpositioned leaves it undrawn, the rest pinned as before.
	rendering.Roots[1].Children[0].Geometry = nil
	dot, err = rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	if !strings.Contains(dot, "// not represented: 1 node(s) without a position, left undrawn\n// canvas: unit=px w=400 h=300\n// layout: neato -n2\n") || strings.Contains(dot, `"n4"`) {
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
		"  graph [fontname=\"Helvetica\", layout=neato, inputscale=72, dpi=72];\n  node [shape=box, style=filled, fillcolor=white, color=\"#181818\", fontname=\"Helvetica\", fontsize=14, penwidth=0.5];\n  edge [color=\"#181818\", fontname=\"Helvetica\", fontsize=13, penwidth=1];\n  \"canvas:0\" [shape=point, style=invis, width=0, height=0, label=\"\", pos=\"0,300!\", pin=true];\n  \"canvas:1\" [shape=point, style=invis, width=0, height=0, label=\"\", pos=\"400,0!\", pin=true];\n  \"n0\"",
		// Outer's box holds a's, 10px below its top: its title is fitted to that strip, at the top.
		`"n0" [label=<<font point-size="8"><b>Outer</b></font>>, margin=0, labelloc=t, pos="110,230!", pin=true, width=2.7777777777777777, height=1.3888888888888888, fixedsize=true, comment="collapsed"];`,
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
	// circle round the label's diagonal, a stated box drawn as the bare symbol
	// with the name beside it.
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
		`"f" [shape=doublecircle, fillcolor=black, label="", xlabel="done", pos="205,-5!", pin=true, width=0.1388888888888889, height=0.1388888888888889, fixedsize=true];`,
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
	// its anchor is pinned at the corner and the unplaced member is left undrawn.
	corner := &Rendering{View: "V", Kind: KindInterconnection, Roots: []*Node{
		{ID: "n0", Kind: "part def", Name: "Outer", Geometry: &Geometry{X: 30, Y: 40}, Children: []*Node{{ID: "n1", Kind: "part", Name: "a"}}},
	}}
	dot, err = corner.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	if !strings.Contains(dot, "// layout: neato -n\n") || strings.Contains(dot, "bb=") || strings.Contains(dot, `"n1"`) || !strings.Contains(dot, `"n0" [shape=point, style=invis, width=0, height=0, label="", pos="30,-40!", pin=true];`) {
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

// A node with no Layout takes its position from the routes that meet it: the
// box its label is fitted to, centred one reach back from the route's end along
// the end segment, so the route meets its border; several routes are averaged.
// The cluster round them is boxed a margin round every placed member. Every
// node then has a position and the header names `neato -n2`; a stated Layout
// still wins over the route, a one-point route places nothing, and a node with
// neither Layout nor route is left undrawn, with the edges at it.
func TestDOTPlacesNodesFromRoutes(t *testing.T) {
	rendering := &Rendering{
		View:   "Routed::view",
		Kind:   KindAction,
		Canvas: &Canvas{Unit: "px", Width: 250, Height: 326, HasSize: true},
		Roots: []*Node{
			{ID: "n0", Kind: "action def", Name: "SendAck", Children: []*Node{
				{ID: "n1", Kind: "initial", Name: "start"},
				{ID: "n2", Kind: "action", Name: "wait"},
				{ID: "n3", Kind: "action", Name: "send", Geometry: &Geometry{X: 120, Y: 200, Width: 80, Height: 40, HasSize: true}},
				{ID: "n4", Kind: "final", Name: "done"},
			}},
		},
		Edges: []Edge{
			// start: an 80pt circle (its label's diagonal), so centred 40 above (160, 53).
			// wait: a 64x37 box, 18.5 above (160, 92) and 18.5 below (160, 129): (160, 110.5) both ways.
			{From: "n1", To: "n2", Kind: EdgeSuccession, Route: []Point{{X: 160, Y: 53}, {X: 160, Y: 92}}},
			{From: "n2", To: "n3", Kind: EdgeSuccession, Route: []Point{{X: 160, Y: 129}, {X: 160, Y: 200}}},
			// send keeps its stated box, centred at (160, 220), not the route's (150, 240).
			// done: a 69pt circle, 34.5 below (150, 280); the cluster is 8 out from every box.
			{From: "n3", To: "n4", Kind: EdgeSuccession, Route: []Point{{X: 150, Y: 240}, {X: 150, Y: 280}}},
		},
	}
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	want := `// view: Routed::view
// kind: action
// canvas: unit=px w=250 h=326
// layout: neato -n2
digraph "Routed::view" {
  graph [fontname="Helvetica", layout=neato, inputscale=72, dpi=72];
  node [shape=box, style=filled, fillcolor=white, color="#181818", fontname="Helvetica", fontsize=14, penwidth=0.5];
  edge [color="#181818", fontname="Helvetica", fontsize=13, penwidth=1];
  "canvas:0" [shape=point, style=invis, width=0, height=0, label="", pos="0,326!", pin=true];
  "canvas:1" [shape=point, style=invis, width=0, height=0, label="", pos="250,0!", pin=true];
  subgraph "cluster_n0" {
    label=<<b>SendAck</b><br/><font point-size="10"><i>«action def»</i></font>>;
    color=black;
    penwidth=0.5;
    bb="107.5,-31,208,361";
    "n0" [shape=point, style=invis, width=0, height=0, label="", pos="157.75,165!", pin=true];
    "n1" [shape=circle, label=<<b>start</b><br/><font point-size="10"><i>«initial»</i></font>>, pos="160,313!", pin=true, width=1.1111111111111112, height=1.1111111111111112];
    "n2" [style="rounded,filled", label=<<b>wait</b><br/><font point-size="10"><i>«action»</i></font>>, pos="160,215.5!", pin=true, width=0.8888888888888888, height=0.5138888888888888];
    "n3" [style="rounded,filled", label=<<b>send</b><br/><font point-size="10"><i>«action»</i></font>>, margin=0, pos="160,106!", pin=true, width=1.1111111111111112, height=0.5555555555555556, fixedsize=true];
    "n4" [shape=doublecircle, label=<<b>done</b><br/><font point-size="10"><i>«final»</i></font>>, pos="150,11.5!", pin=true, width=0.9583333333333334, height=0.9583333333333334];
  }
  "n1" -> "n2" [pos="e,160,234 160,273 160,273 160,244 160,244"];
  "n2" -> "n3" [pos="e,160,126 160,197 160,197 160,136 160,136"];
  "n3" -> "n4" [pos="e,150,46 150,86 150,86 150,56 150,56"];
}
`
	if dot != want {
		t.Errorf("DOT:\n%s\nwant:\n%s", dot, want)
	}
	// An unnamed pseudo-state takes the UML dot's 0.2in; a diagonal end segment
	// is followed back to the box's border, here the bottom edge of wait's 37pt.
	diagonal := &Rendering{View: "V", Kind: KindAction, Roots: []*Node{
		{ID: "a", Kind: "action", Name: "wait"},
		{ID: "f", Kind: "final"},
	}, Edges: []Edge{{From: "a", To: "f", Kind: EdgeSuccession, Route: []Point{{X: 100, Y: 100}, {X: 137, Y: 137}, {X: 200, Y: 137}}}}}
	dot, err = diagonal.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	for _, want := range []string{
		"// layout: neato -n2\n",
		`"a" [style="rounded,filled", label=<<b>wait</b><br/><font point-size="10"><i>«action»</i></font>>, pos="81.5,-81.5!", pin=true, width=0.8888888888888888, height=0.5138888888888888];`,
		`"f" [shape=doublecircle, fillcolor=black, label="", pos="207.2,-137!", pin=true, width=0.2, height=0.2];`,
	} {
		if !strings.Contains(dot, want) {
			t.Errorf("diagonal DOT lacks %q:\n%s", want, dot)
		}
	}
	// A route of one waypoint places nothing, nor does an edge with none; the
	// final node it reaches takes its place below send instead, unrouted to.
	rendering.Edges[2].Route = rendering.Edges[2].Route[:1]
	dot, err = rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	if !strings.Contains(dot, "// not represented: route of n3->n4 is one waypoint, (150, 240); a line needs two\n// canvas: unit=px w=250 h=326\n// layout: neato -n2\n") ||
		!strings.Contains(dot, `"n4" [shape=doublecircle, label=<<b>done</b><br/><font point-size="10"><i>«final»</i></font>>, pos="160,31.5!", pin=true, width=0.9583333333333334, height=0.9583333333333334];`) ||
		!strings.Contains(dot, `"n3" -> "n4";`) || strings.Contains(dot, "without a position") {
		t.Errorf("one-waypoint DOT:\n%s", dot)
	}
	rendering.Edges = rendering.Edges[:2]
	rendering.Roots[0].Children = append(rendering.Roots[0].Children, &Node{ID: "n5", Kind: "action", Name: "value"})
	dot, err = rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	if !strings.Contains(dot, "// not represented: 2 node(s) without a position, left undrawn\n// canvas: unit=px w=250 h=326\n// layout: neato -n2\n") || strings.Contains(dot, `"n5"`) {
		t.Errorf("unrouted-node DOT:\n%s", dot)
	}
	// A tree has no clusters to box round members: the parent itself is left
	// undrawn when nothing routes to it, its children drawn with no edge to it.
	rendering.Roots[0].Children = rendering.Roots[0].Children[:4]
	rendering.Kind = KindTree
	dot, err = rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	if !strings.Contains(dot, "// layout: neato -n2\n") || strings.Contains(dot, "bb=") || strings.Contains(dot, `"n0"`) || !strings.Contains(dot, `"n1" -> "n2" [pos=`) {
		t.Errorf("tree DOT:\n%s", dot)
	}
}

// A drawing some Layout positions leaves the nodes none does undrawn, with the
// edges at them, and says so; asked for the strip, it sets them in rows below
// everything placed — canvas, boxes and routes — a gap clear of it and of one
// another, an unplaced cluster round its own members, a tree's nodes each on
// their own; a drawing nothing positions is laid out by `dot` as ever.
func TestDOTSettlesUnplacedNodes(t *testing.T) {
	rendering := func() *Rendering {
		return &Rendering{
			View:   "V",
			Kind:   KindInterconnection,
			Canvas: &Canvas{Unit: "px", Width: 300, Height: 100, HasSize: true},
			Roots: []*Node{
				{ID: "placed", Kind: "part", Name: "pump", Geometry: &Geometry{X: 10, Y: 10, Width: 100, Height: 40, HasSize: true}},
				{ID: "low", Kind: "part", Name: "tank", Geometry: &Geometry{X: 150, Y: 120, Width: 60, Height: 30, HasSize: true}},
				{ID: "loose", Kind: "part", Name: "spare"},
				{ID: "wide", Kind: "part def", Name: "A rather long definition name"},
				{ID: "group", Kind: "part def", Name: "Group", Children: []*Node{
					{ID: "g1", Kind: "part", Name: "g1"},
					{ID: "g2", Kind: "part", Name: "g2"},
				}},
			},
			Edges: []Edge{
				{From: "placed", To: "low", Kind: EdgeConnection, Route: []Point{{X: 60, Y: 50}, {X: 60, Y: 170}, {X: 150, Y: 170}}},
				{From: "placed", To: "loose", Kind: EdgeConnection},
				{From: "loose", To: "g1", Kind: EdgeFlow},
			},
		}
	}
	dot, err := rendering().DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	if !strings.Contains(dot, "// not represented: 5 node(s) without a position, left undrawn, and 2 edge(s) at them\n// canvas: unit=px w=300 h=100\n// layout: neato -n2\n") {
		t.Errorf("omitting DOT header:\n%s", dot)
	}
	for _, id := range []string{"loose", "wide", "group", "g1", "g2"} {
		if strings.Contains(dot, dotQuote(id)) {
			t.Errorf("omitting DOT draws unplaced %s:\n%s", id, dot)
		}
	}
	if !strings.Contains(dot, `"placed" -> "low" [arrowhead=none, penwidth=3, pos=`) || strings.Count(dot, " -> ") != 1 {
		t.Errorf("omitting DOT edges:\n%s", dot)
	}
	// The strip: the canvas is 300 wide but tank reaches to y=170 with its route,
	// so the strip starts at y=194 and rows wrap at x=300.
	dot, err = rendering().DOTWith(Options{Unplaced: UnplacedStrip})
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	if !strings.Contains(dot, "// not represented: 5 node(s) without a position, drawn in a strip below the drawing\n// canvas: unit=px w=300 h=100\n// layout: neato -n2\n") {
		t.Errorf("strip DOT header:\n%s", dot)
	}
	for _, want := range []string{
		// spare: 63x37 from (0, 194), so its centre is (31.5, 212.5), y flipped
		// against the 100-high canvas; the long name, 284 wide, would overrun 300
		// beside it, so it heads the next row, 24 below.
		`"loose" [style="rounded,filled", label=<<b>spare</b><br/><font point-size="10"><i>«part»</i></font>>, pos="31.5,-112.5!", pin=true, width=0.875, height=0.5138888888888888];`,
		`"wide" [label=<<b>A rather long definition name</b><br/><font point-size="10"><i>«part def»</i></font>>, pos="142,-173.5!", pin=true, width=3.9444444444444446, height=0.5138888888888888];`,
		// Group heads the third row at y=316: its title, then g1 and g2 side by
		// side, a margin of 8 in from its box.
		"    bb=\"0,-298,148,-216\";\n",
		`"g1" [style="rounded,filled", label=<<b>g1</b><br/><font point-size="10"><i>«part»</i></font>>, pos="35,-271.5!", pin=true, width=0.75, height=0.5138888888888888];`,
		`"g2" [style="rounded,filled", label=<<b>g2</b><br/><font point-size="10"><i>«part»</i></font>>, pos="113,-271.5!", pin=true, width=0.75, height=0.5138888888888888];`,
		`"placed" -> "loose" [arrowhead=none, penwidth=3];`,
		`"loose" -> "g1" [style=dashed];`,
	} {
		if !strings.Contains(dot, want) {
			t.Errorf("strip DOT lacks %q:\n%s", want, dot)
		}
	}
	assertStripBelow(t, rendering(), 170)
	// A tree sets each unplaced node on its own, the containment edges drawn to them.
	tree := rendering()
	tree.Kind = KindTree
	dot, err = tree.DOTWith(Options{Unplaced: UnplacedStrip})
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	if strings.Contains(dot, "bb=") || !strings.Contains(dot, `"group" -> "g1" [arrowhead=none];`) ||
		!strings.Contains(dot, `"group" [label=<<b>Group</b><br/><font point-size="10"><i>«part def»</i></font>>, pos="38,-234.5!", pin=true, width=1.0555555555555556, height=0.5138888888888888];`) ||
		!strings.Contains(dot, `"g2" [style="rounded,filled", label=<<b>g2</b><br/><font point-size="10"><i>«part»</i></font>>, pos="205,-234.5!", pin=true, width=0.75, height=0.5138888888888888];`) {
		t.Errorf("tree strip DOT:\n%s", dot)
	}
	assertStripBelow(t, tree, 170)
	// Nothing positioned: every node drawn, laid out by dot, whatever is asked.
	for _, unplaced := range []Unplaced{"", UnplacedOmit, UnplacedStrip} {
		plain := rendering()
		plain.Canvas = nil
		for _, root := range plain.Roots {
			root.Geometry = nil
		}
		plain.Edges[0].Route = nil
		dot, err = plain.DOTWith(Options{Unplaced: unplaced})
		if err != nil {
			t.Fatalf("%q: DOT: %v", unplaced, err)
		}
		if !strings.Contains(dot, "// kind: interconnection\n// layout: dot\n") || strings.Contains(dot, "pos=") || strings.Count(dot, " -> ") != 3 {
			t.Errorf("%q: unpositioned DOT:\n%s", unplaced, dot)
		}
	}
	// A placement no name has is refused, naming the ones there are.
	if _, err := rendering().DOTWith(Options{Unplaced: "pile"}); err == nil || !errors.Is(err, ErrUnknownUnplaced) || err.Error() != `unknown placement "pile" of unplaced nodes; the placements are omit, strip` {
		t.Errorf("unknown placement: %v", err)
	}
}

// assertStripBelow checks every node the strip places sits below everything
// the Layouts place — the canvas and every stated box and route end at floor —
// and that no two strip boxes overlap.
func assertStripBelow(t *testing.T, r *Rendering, floor float64) {
	t.Helper()
	w := newDOTWriter(r, Options{})
	stated := map[string]nodeBox{}
	for id, box := range w.boxes {
		stated[id] = box
	}
	w.stripUnplaced(r.Roots, r.Edges)
	var strip []nodeBox
	for id, box := range w.boxes {
		if _, ok := stated[id]; ok {
			continue
		}
		if box.low.Y < floor+dotStripGap {
			t.Errorf("strip box %s at y=%v is not below the drawing's %v", id, box.low.Y, floor)
		}
		strip = append(strip, box)
	}
	if len(strip) == 0 {
		t.Fatal("the strip placed nothing")
	}
	for i, a := range strip {
		for _, b := range strip[i+1:] {
			if a.encloses(b) || b.encloses(a) {
				continue // a cluster round its members
			}
			if a.low.X < b.high.X && b.low.X < a.high.X && a.low.Y < b.high.Y && b.low.Y < a.high.Y {
				t.Errorf("strip boxes overlap: %+v and %+v", a, b)
			}
		}
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
		if (name == "label" || name == "xlabel" || name == "lhead" || name == "ltail" || name == "pos" || name == "bb" || name == "comment") && !value.quoted {
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
		if name == "pos" || name == "bb" || name == "lp" {
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
			case tag == "br/" || tag == "hr/":
			case tag == "b" || tag == "i" || tag == "tr" || tag == "td" || tag == `td align="left"` ||
				tag == `table border="0" cellborder="0" cellspacing="0" cellpadding="2"` ||
				strings.HasPrefix(tag, `font point-size="`) && strings.HasSuffix(tag, `"`):
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
// node `pos` or an `lp` is one point, a node's pinned, an edge `pos` a cubic
// B-spline of 3n+1 points behind an optional `e,x,y` endpoint, `bb` two corners.
func checkDOTGeometry(t *testing.T, name, value, dot string) {
	t.Helper()
	points := strings.Fields(value)
	if name == "pos" && len(points) > 1 && strings.HasPrefix(points[0], "e,") {
		checkDOTGeometry(t, "lp", strings.TrimPrefix(points[0], "e,"), dot)
		points = points[1:]
	}
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

// A pseudostate with a stated position keeps it: the route reaching it is not
// followed to place it, and the box it stands in has that geometry.
func TestDOTKeepsStatedPseudostatePositions(t *testing.T) {
	rendering := &Rendering{View: "V", Kind: KindState, Canvas: &Canvas{Unit: "px", Width: 300, Height: 300, HasSize: true}, Roots: []*Node{
		{ID: "i", Kind: "initial", Geometry: &Geometry{X: 60, Y: 40, Width: 16, Height: 16, HasSize: true}},
		{ID: "s", Kind: "state", Name: "Idle", Geometry: &Geometry{X: 40, Y: 100, Width: 120, Height: 50, HasSize: true}},
		{ID: "f", Kind: "final", Geometry: &Geometry{X: 200, Y: 100, Width: 20, Height: 20, HasSize: true}},
	}, Edges: []Edge{
		{From: "i", To: "s", Kind: EdgeTransition, Route: []Point{{X: 68, Y: 56}, {X: 68, Y: 100}}},
		{From: "s", To: "f", Kind: EdgeTransition, Route: []Point{{X: 160, Y: 125}, {X: 200, Y: 110}}},
	}}
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	for _, want := range []string{
		"// layout: neato -n2\n",
		`"i" [shape=circle, fillcolor=black, label="", pos="68,252!", pin=true, width=0.2222222222222222, height=0.2222222222222222, fixedsize=true];`,
		`"f" [shape=doublecircle, fillcolor=black, label="", pos="210,190!", pin=true, width=0.2777777777777778, height=0.2777777777777778, fixedsize=true];`,
		`"i" -> "s" [pos="e,68,200 68,244 68,244 68,210 68,210"];`,
	} {
		if !strings.Contains(dot, want) {
			t.Errorf("DOT lacks %q:\n%s", want, dot)
		}
	}
	if strings.Contains(dot, "not represented") {
		t.Errorf("DOT drops something:\n%s", dot)
	}
}
