package view

import (
	"errors"
	"fmt"
	"path/filepath"
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
			for _, want := range []string{"// view: " + tc.view, "// kind: " + string(tc.kind), "// layout: dot", "digraph " + dotQuote(tc.view) + " {", "node [shape=box];"} {
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
		case !tok.quoted && tok.text == "label" && i+2 < len(tokens) && tokens[i+1].text == "=":
			// A cluster's label statement.
			if !tokens[i+2].quoted {
				t.Fatalf("cluster label is not quoted at token %d:\n%s", i, dot)
			}
			i += 2
		case !tok.quoted && tok.text == "style" && i+2 < len(tokens) && tokens[i+1].text == "=":
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
		if (name == "label" || name == "lhead" || name == "ltail") && !value.quoted {
			t.Fatalf("attribute %s has a bare value %q:\n%s", name, value.text, dot)
		}
		if name == "lhead" || name == "ltail" {
			*clipped = append(*clipped, value.text)
		}
		i += 2
	}
	if i >= len(tokens) {
		t.Fatalf("attribute list never closes:\n%s", dot)
	}
	return i
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
