package view

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// The digraph opens with the Standard B&W defaults once, as graph, node and
// edge statements: Helvetica text, white fills, thin #181818 lines, edge text a
// point smaller than node text.
func TestDOTStandardDefaults(t *testing.T) {
	rendering := render(t, "interconnection.sysml", "PlantViews::loopView")
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	want := `digraph "PlantViews::loopView" {
  graph [fontname="Helvetica"];
  node [shape=box, style=filled, fillcolor=white, color="#181818", fontname="Helvetica", fontsize=14, penwidth=0.5];
  edge [color="#181818", fontname="Helvetica", fontsize=13, penwidth=1];
`
	if !strings.Contains(dot, want) {
		t.Errorf("DOT lacks the defaults %q:\n%s", want, dot)
	}
	// No node or edge restates a default, and nothing is coloured.
	for _, line := range strings.Split(dot, "\n") {
		if strings.HasPrefix(line, "//") || strings.HasPrefix(line, "  graph [") || strings.HasPrefix(line, "  node [") || strings.HasPrefix(line, "  edge [") {
			continue
		}
		for _, attr := range []string{"fontname", "fontsize", "fillcolor=white", "shape=box", "color=\"#181818\""} {
			if strings.Contains(line, attr) {
				t.Errorf("line %q restates the default %s", line, attr)
			}
		}
	}
	if strings.Contains(dot, "fontcolor") {
		t.Errorf("DOT sets a font colour; black is the default:\n%s", dot)
	}
}

// A definition is a square box and a usage a rounded one, by keyword: `… def`
// and the KerML classifiers are definitions, everything else a usage; a
// pseudo-state or control node is neither.
func TestDOTDefinitionsSquareUsagesRounded(t *testing.T) {
	kinds := map[string]bool{
		"part def": true, "action def": true, "state def": true, "use case def": true, "class": true, "datatype": true,
		"assoc struct": true, "behavior": true, "metaclass": true,
		"part": false, "item": false, "port": false, "attribute": false, "action": false, "state": false, "perform": false,
		"connection": false, "connect": false, "use case": false, "view": false, "statements": false, "node": false,
	}
	for kind, definition := range kinds {
		if got := isDefinitionKind(kind); got != definition {
			t.Errorf("isDefinitionKind(%q) = %v, want %v", kind, got, definition)
		}
		node := &Node{ID: "n", Kind: kind, Name: "x"}
		dot, err := (&Rendering{View: "V", Kind: KindInterconnection, Roots: []*Node{node}}).DOT()
		if err != nil {
			t.Fatalf("DOT: %v", err)
		}
		rounded := strings.Contains(dot, `"n" [style="rounded,filled", label=`)
		if rounded == definition {
			t.Errorf("%s: rounded=%v, want %v:\n%s", kind, rounded, !definition, dot)
		}
		if definition && !strings.Contains(dot, `"n" [label=`) {
			t.Errorf("%s: a definition lists only its label:\n%s", kind, dot)
		}
		if strings.Contains(dot, `"n" [shape=`) {
			t.Errorf("%s restates the default shape:\n%s", kind, dot)
		}
	}
	for _, kind := range []string{"fork", "join", "merge", "decision", "choice", "junction", "shallow history", "deep history"} {
		dot, err := (&Rendering{View: "V", Kind: KindAction, Roots: []*Node{{ID: "n", Kind: kind, Name: "x"}}}).DOT()
		if err != nil {
			t.Fatalf("DOT: %v", err)
		}
		if strings.Contains(dot, "rounded") {
			t.Errorf("%s is rounded, want a control node's square box:\n%s", kind, dot)
		}
	}
}

// An initial pseudo-state with no name is the UML filled dot and a final one the
// filled double ring; one the rendering names keeps its labelled circle. A
// body's start stays a point, filled black under the white node default.
func TestDOTPseudostateRules(t *testing.T) {
	cases := []struct {
		name string
		node *Node
		want string
	}{
		{"unnamed initial", &Node{ID: "n", Kind: "initial"}, `"n" [shape=circle, fillcolor=black, label="", width=0.2];`},
		{"unnamed final", &Node{ID: "n", Kind: "final"}, `"n" [shape=doublecircle, fillcolor=black, label="", width=0.2];`},
		{"named initial", &Node{ID: "n", Kind: "initial", Name: "go"},
			`"n" [shape=circle, label=<<b>go</b><br/><font point-size="10"><i>«initial»</i></font>>];`},
		{"named final", &Node{ID: "n", Kind: "final", Name: "done"},
			`"n" [shape=doublecircle, label=<<b>done</b><br/><font point-size="10"><i>«final»</i></font>>];`},
		{"start", &Node{ID: "n", Kind: startKind}, `"n" [shape=point, fillcolor=black, label=""];`},
		{"placed unnamed initial", &Node{ID: "n", Kind: "initial", Geometry: &Geometry{X: 10, Y: 10, HasSize: true, Width: 36, Height: 36}},
			`"n" [shape=circle, fillcolor=black, label="", pos="28,-28!", pin=true, width=0.5, height=0.5, fixedsize=true];`},
	}
	for _, tc := range cases {
		for _, palette := range []Palette{"", PaletteOkabeIto} {
			dot, err := (&Rendering{View: "V", Kind: KindAction, Roots: []*Node{tc.node}}).DOTWith(Options{Palette: palette})
			if err != nil {
				t.Fatalf("%s: DOT: %v", tc.name, err)
			}
			checkDOTSyntax(t, dot)
			if !strings.Contains(dot, tc.want) {
				t.Errorf("%s under palette %q: DOT lacks %q:\n%s", tc.name, palette, tc.want, dot)
			}
		}
	}
}

// A cluster keeps no fill and a black border: at penwidth 1.5 for a package,
// 0.5 for an element's body, dashed at 0.5 for a region; its style, colour and
// width come after its label and before its box.
func TestDOTClusterBorders(t *testing.T) {
	rendering := &Rendering{View: "V", Kind: KindInterconnection, Roots: []*Node{
		{ID: "n0", Kind: "package", Name: "Pkg", Children: []*Node{
			{ID: "n1", Kind: "part def", Name: "Def", Children: []*Node{
				{ID: "n2", Kind: "part", Name: "usage", Children: []*Node{
					{ID: "n3", Kind: "port", Name: "p"},
				}},
			}},
			{ID: "n4", Kind: "library package", Name: "Lib", Children: []*Node{{ID: "n5", Kind: "item", Name: "i"}}},
			{ID: "n6", Kind: "region", Name: "r", Children: []*Node{{ID: "n7", Kind: "state", Name: "s"}}},
			{ID: "n8", Kind: "state", Name: "body", Children: []*Node{{ID: "n9", Kind: "state", Name: "t"}},
				Geometry: &Geometry{X: 0, Y: 0, HasSize: true, Width: 100, Height: 50}},
		}},
	}}
	// body is placed, so the rest is drawn only when asked for, in the strip.
	for _, palette := range []Palette{"", PaletteTolBright} {
		dot, err := rendering.DOTWith(Options{Palette: palette, Unplaced: UnplacedStrip})
		if err != nil {
			t.Fatalf("DOT: %v", err)
		}
		checkDOTSyntax(t, dot)
		for _, want := range []string{
			"  subgraph \"cluster_n0\" {\n    label=<<b>Pkg</b><br/><font point-size=\"10\"><i>«package»</i></font>>;\n    color=black;\n    penwidth=1.5;\n",
			"    subgraph \"cluster_n1\" {\n      label=<<b>Def</b><br/><font point-size=\"10\"><i>«part def»</i></font>>;\n      color=black;\n      penwidth=0.5;\n",
			"      subgraph \"cluster_n2\" {\n        label=<<b>usage</b><br/><font point-size=\"10\"><i>«part»</i></font>>;\n        color=black;\n        penwidth=0.5;\n",
			"    subgraph \"cluster_n4\" {\n      label=<<b>Lib</b><br/><font point-size=\"10\"><i>«library package»</i></font>>;\n      color=black;\n      penwidth=1.5;\n",
			"    subgraph \"cluster_n6\" {\n      label=<<b>r</b><br/><font point-size=\"10\"><i>«region»</i></font>>;\n      style=dashed;\n      color=black;\n      penwidth=0.5;\n",
			"    subgraph \"cluster_n8\" {\n      label=<<b>body</b><br/><font point-size=\"10\"><i>«state»</i></font>>;\n      color=black;\n      penwidth=0.5;\n      bb=",
		} {
			if !strings.Contains(dot, want) {
				t.Errorf("palette %q: DOT lacks %q:\n%s", palette, want, dot)
			}
		}
		if strings.Contains(dot, "style=filled;") || strings.Contains(dot, "fillcolor=\"#") && palette == "" {
			t.Errorf("palette %q: a cluster is filled:\n%s", palette, dot)
		}
	}
	if got := dotClusterPenwidth(&Node{Kind: "package"}); got != "1.5" {
		t.Errorf("package penwidth = %s", got)
	}
	if got := dotClusterPenwidth(&Node{Kind: "part def"}); got != "0.5" {
		t.Errorf("part def penwidth = %s", got)
	}
}

// A connection is drawn at penwidth 3 with no arrowhead, the Pilot's thick
// connector; flows stay dashed, successions and transitions plain arrows.
func TestDOTConnectionPenwidth(t *testing.T) {
	rendering := &Rendering{View: "V", Kind: KindInterconnection,
		Roots: []*Node{{ID: "a", Kind: "part", Name: "a"}, {ID: "b", Kind: "part", Name: "b"}},
		Edges: []Edge{
			{From: "a", To: "b", Kind: EdgeConnection, Label: "c"},
			{From: "a", To: "b", Kind: EdgeFlow},
			{From: "a", To: "b", Kind: EdgeSuccession},
			{From: "a", To: "b", Kind: EdgeTransition},
		}}
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	want := `  "a" -> "b" [label="c", arrowhead=none, penwidth=3];
  "a" -> "b" [style=dashed];
  "a" -> "b";
  "a" -> "b";
}
`
	if !strings.HasSuffix(dot, want) {
		t.Errorf("DOT edges:\n%s\nwant to end with:\n%s", dot, want)
	}
	if got := strings.Count(dot, "penwidth=3"); got != 1 {
		t.Errorf("penwidth=3 written %d times, want once", got)
	}
}

// Under a palette a plain node is filled by keyword family: a definition in the
// family colour, a usage in its tint, both bordered in the untinted colour at
// penwidth 1; pseudo-states and clusters keep the black-and-white rules, and
// the layout is untouched.
func TestDOTPaletteFills(t *testing.T) {
	rendering := &Rendering{View: "V", Kind: KindInterconnection, Roots: []*Node{
		{ID: "n0", Kind: "part def", Name: "Def", Children: []*Node{
			{ID: "n1", Kind: "part", Name: "p", Geometry: &Geometry{X: 10, Y: 20, HasSize: true, Width: 100, Height: 50}},
			{ID: "n2", Kind: "port", Name: "q"},
			{ID: "n3", Kind: "item def", Name: "I"},
			{ID: "n4", Kind: "initial"},
			{ID: "n5", Kind: startKind},
			{ID: "n6", Kind: "view", Name: "v"},
		}},
	}, Edges: []Edge{{From: "n1", To: "n2", Kind: EdgeConnection}}}
	// p is placed, so the rest is drawn only when asked for, in the strip.
	plain, err := rendering.DOTWith(Options{Unplaced: UnplacedStrip})
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	dot, err := rendering.DOTWith(Options{Palette: PaletteOkabeIto, Unplaced: UnplacedStrip})
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	part, port, item, other := paletteColors[PaletteOkabeIto][0], paletteColors[PaletteOkabeIto][2], paletteColors[PaletteOkabeIto][1], paletteColors[PaletteOkabeIto][18%8]
	for _, want := range []string{
		`"n1" [style="rounded,filled", fillcolor="` + paletteFill(part, true) + `", color="` + part + `", penwidth=1, label=<<b>p</b><br/><font point-size="10"><i>«part»</i></font>>, margin=0, pos="60,-45!", pin=true, width=1.3888888888888888, height=0.6944444444444444, fixedsize=true];`,
		`"n2" [style="rounded,filled", fillcolor="` + paletteFill(port, true) + `", color="` + port + `", penwidth=1, label=`,
		`"n3" [fillcolor="` + item + `", color="` + item + `", penwidth=1, label=`,
		`"n4" [shape=circle, fillcolor=black, label="", pos=`,
		`"n5" [shape=point, fillcolor=black, label="", pos=`,
		`"n6" [style="rounded,filled", fillcolor="` + paletteFill(other, true) + `", color="` + other + `", penwidth=1, label=`,
		"    label=<<b>Def</b><br/><font point-size=\"10\"><i>«part def»</i></font>>;\n    color=black;\n    penwidth=0.5;\n",
		`"n1" -> "n2" [arrowhead=none, penwidth=3];`,
	} {
		if !strings.Contains(dot, want) {
			t.Errorf("okabe-ito DOT lacks %q:\n%s", want, dot)
		}
	}
	if paletteFill(item, false) != item {
		t.Errorf("item def fill %s is not the family colour %s", paletteFill(item, false), item)
	}
	// Stripping the fills gives back the black-and-white text, line for line.
	stripped := dot
	for _, color := range []string{part, port, item, other} {
		stripped = strings.ReplaceAll(stripped, `fillcolor="`+paletteFill(color, true)+`", color="`+color+`", penwidth=1, `, "")
		stripped = strings.ReplaceAll(stripped, `fillcolor="`+paletteFill(color, false)+`", color="`+color+`", penwidth=1, `, "")
	}
	if stripped != plain {
		t.Errorf("palette changes more than fills:\n%s\nblack and white:\n%s", dot, plain)
	}
	// A tree fills the nodes that an interconnection draws as clusters.
	tree, err := (&Rendering{View: "V", Kind: KindTree, Roots: rendering.Roots}).DOTWith(Options{Palette: PaletteOkabeIto, Unplaced: UnplacedStrip})
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	if !strings.Contains(tree, `"n0" [fillcolor="`+part+`", color="`+part+`", penwidth=1, label=`) {
		t.Errorf("tree DOT does not fill the root part def:\n%s", tree)
	}
}

// A sequential palette is sampled over the families present, darkest first, so
// a diagram of three families spans the whole ramp; a qualitative palette keeps
// each family's fixed colour whatever else is drawn.
func TestDOTSequentialPaletteSpansFamiliesPresent(t *testing.T) {
	roots := []*Node{
		{ID: "a", Kind: "state def", Name: "S"},
		{ID: "b", Kind: "port", Name: "p"},
		{ID: "c", Kind: "part", Name: "q"},
		{ID: "d", Kind: "state", Name: "s"},
	}
	dot, err := (&Rendering{View: "V", Kind: KindTree, Roots: roots}).DOTWith(Options{Palette: PaletteViridis})
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	viridis := paletteColors[PaletteViridis]
	part, port, state := viridis[0], viridis[8], viridis[15]
	for _, want := range []string{
		`"a" [fillcolor="` + paletteFill(state, false) + `", color="` + state + `", penwidth=1, label=`,
		`"b" [style="rounded,filled", fillcolor="` + paletteFill(port, true) + `", color="` + port + `", penwidth=1, label=`,
		`"c" [style="rounded,filled", fillcolor="` + paletteFill(part, true) + `", color="` + part + `", penwidth=1, label=`,
		`"d" [style="rounded,filled", fillcolor="` + paletteFill(state, true) + `", color="` + state + `", penwidth=1, label=`,
	} {
		if !strings.Contains(dot, want) {
			t.Errorf("viridis DOT lacks %q:\n%s", want, dot)
		}
	}
	one, err := (&Rendering{View: "V", Kind: KindTree, Roots: roots[1:2]}).DOTWith(Options{Palette: PaletteCividis})
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	if darkest := paletteColors[PaletteCividis][0]; !strings.Contains(one, `color="`+darkest+`"`) {
		t.Errorf("a lone family is not the darkest stop %s:\n%s", darkest, one)
	}
	fixed, err := (&Rendering{View: "V", Kind: KindTree, Roots: roots[1:2]}).DOTWith(Options{Palette: PaletteOkabeIto})
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	if port := paletteColors[PaletteOkabeIto][2]; !strings.Contains(fixed, `color="`+port+`"`) {
		t.Errorf("a lone port is not the port family's colour %s:\n%s", port, fixed)
	}
}

// A Palette value no registry entry has is refused at the DOT boundary as the
// UnknownPaletteError every surface raises, with no artifact written.
func TestDOTRefusesAnUnregisteredPalette(t *testing.T) {
	rendering := &Rendering{View: "V", Kind: KindTree, Roots: []*Node{{ID: "n0", Kind: "part def", Name: "Def"}}}
	dot, err := rendering.DOTWith(Options{Palette: Palette("rainbow")})
	var unknown *UnknownPaletteError
	if !errors.As(err, &unknown) || unknown.Name != "rainbow" {
		t.Fatalf("DOTWith(rainbow) = %q, %v; want an UnknownPaletteError naming rainbow", dot, err)
	}
	if dot != "" {
		t.Errorf("DOTWith(rainbow) wrote %q beside the error", dot)
	}
	if _, err := rendering.WriteWith(FormDot, Options{Palette: Palette("rainbow")}); !errors.Is(err, ErrUnknownPalette) {
		t.Errorf("WriteWith(dot, rainbow) = %v, want ErrUnknownPalette", err)
	}
}

// The Mermaid form notes a palette it cannot draw; the text and Markdown forms
// take none and say nothing; a palette never changes the DOT header.
func TestPaletteOnOtherForms(t *testing.T) {
	rendering := render(t, "state.sysml", "MachineViews::vehicleStates")
	mermaid, err := rendering.WriteWith(FormMermaid, Options{Palette: PaletteBrewerSet2})
	if err != nil {
		t.Fatalf("mermaid: %v", err)
	}
	if !strings.Contains(mermaid, "%% not represented: palette brewer-set2; only the DOT and PlantUML forms fill nodes by keyword family\n") {
		t.Errorf("Mermaid does not note the palette:\n%s", mermaid)
	}
	if strings.Contains(mermaid, "fill:") || strings.Contains(mermaid, "classDef") || strings.Contains(mermaid, "theme") {
		t.Errorf("Mermaid is themed by the palette:\n%s", mermaid)
	}
	if plain := rendering.Mermaid(); strings.Contains(plain, "palette") {
		t.Errorf("Mermaid notes a palette none was asked for:\n%s", plain)
	}
	text, err := rendering.WriteWith(FormText, Options{Palette: PaletteBrewerSet2})
	if err != nil || text != rendering.Text() {
		t.Errorf("text form under a palette differs: %v\n%s", err, text)
	}
	table := render(t, "table.sysml", "TableViews::partsTable")
	markdown, err := table.WriteWith(FormMarkdown, Options{Palette: PaletteBrewerSet2})
	if err != nil || markdown != table.Markdown() {
		t.Errorf("markdown form under a palette differs: %v\n%s", err, markdown)
	}
	dot, err := rendering.DOTWith(Options{Palette: PaletteBrewerSet2})
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	if strings.Contains(dot, "// not represented: palette") {
		t.Errorf("DOT notes the palette it draws:\n%s", dot)
	}
}

// A name with markup characters and line breaks is a valid HTML label under the
// italic keyword line, each character an entity and each newline a break.
func TestDOTEscapesNamesInStyledLabels(t *testing.T) {
	node := &Node{ID: "n", Kind: "part", Name: "a & b <c> \"d\" 'e'\nf", Type: "T<'x'>", Detail: "g > h\n'i'"}
	dot, err := (&Rendering{View: "V", Kind: KindInterconnection, Roots: []*Node{node}}).DOTWith(Options{Palette: PaletteOkabeIto})
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	want := `label=<<b>a &amp; b &lt;c&gt; &#34;d&#34; &#39;e&#39;<br/>f : T&lt;&#39;x&#39;&gt;</b><br/><font point-size="10"><i>«part»</i></font><br/>g &gt; h<br/>&#39;i&#39;>`
	if !strings.Contains(dot, want) {
		t.Errorf("DOT lacks %q:\n%s", want, dot)
	}
	if got := dotEscape("'"); got != "&#39;" {
		t.Errorf("dotEscape(') = %q", got)
	}
}

// The palette goldens: the interconnection and state renderings in okabe-ito,
// the tree in viridis. Each is its black-and-white golden with fills added.
func TestGoldenDOTPalettes(t *testing.T) {
	cases := []struct {
		name, file, view string
		palette          Palette
	}{
		{"interconnection", "interconnection.sysml", "PlantViews::loopView", PaletteOkabeIto},
		{"state", "state.sysml", "MachineViews::vehicleStates", PaletteOkabeIto},
		{"tree", "tree.sysml", "VehicleViews::vehicleView", PaletteViridis},
	}
	for _, tc := range cases {
		t.Run(tc.name+"-"+string(tc.palette), func(t *testing.T) {
			rendering := render(t, tc.file, tc.view)
			dot, err := rendering.WriteWith(FormDot, Options{Palette: tc.palette})
			if err != nil {
				t.Fatalf("write dot: %v", err)
			}
			checkGolden(t, filepath.Join("testdata", tc.name+"."+string(tc.palette)+".dot.golden"), dot)
			checkDOTSyntax(t, dot)
			plain, err := rendering.DOT()
			if err != nil {
				t.Fatalf("DOT: %v", err)
			}
			if palette, bw := strings.Split(dot, "\n"), strings.Split(plain, "\n"); len(palette) != len(bw) {
				t.Errorf("palette DOT has %d lines, black and white %d", len(palette), len(bw))
			} else {
				for i := range bw {
					if !sameButFill(palette[i], bw[i]) {
						t.Errorf("line %d differs in more than fill:\n%s\n%s", i+1, palette[i], bw[i])
					}
				}
			}
			if !strings.Contains(dot, "fillcolor=\"#") {
				t.Errorf("palette DOT fills nothing:\n%s", dot)
			}
		})
	}
}

// sameButFill reports whether a palette line is the black-and-white line with a
// fill, border colour and penwidth 1 added, or the same line.
func sameButFill(palette, bw string) bool {
	if palette == bw {
		return true
	}
	start := strings.Index(palette, `fillcolor="#`)
	end := strings.Index(palette, "penwidth=1, ")
	if start < 0 || end < start {
		return false
	}
	return palette[:start]+palette[end+len("penwidth=1, "):] == bw
}
