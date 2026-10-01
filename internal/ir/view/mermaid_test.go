package view

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestMermaidFlowchartShapes(t *testing.T) {
	labels := labeller{}
	cases := []struct {
		name  string
		kind  string
		synth bool
		style DrawingStyle
		want  string
	}{
		{"definition", "part def", false, StylePilot, `["`},
		{"region", "region", false, StylePilot, `["`},
		{"package", "part package", false, StylePilot, `["`},
		{"usage", "part", false, StylePilot, `("`},
		{"cameo", "part", false, StyleCameo, `["`},
		{"start", startKind, true, StylePilot, `@{ shape: sm-circ, label: "" }`},
		{"initial", "initial", true, StylePilot, `@{ shape: sm-circ, label: "" }`},
		{"final", "final", true, StylePilot, `@{ shape: fr-circ, label: "" }`},
		{"terminate", terminateKind, true, StylePilot, `@{ shape: fr-circ, label: "" }`},
		{"junction", "junction", true, StylePilot, `@{ shape: f-circ, label: "" }`},
		{"fork", "fork", false, StylePilot, `@{ shape: fork, label: "fork" }`},
		{"join", "join", true, StylePilot, `@{ shape: fork, label: "" }`},
		{"decision", "decision", false, StylePilot, "{\"`**decision**"},
		{"empty decision", "decision", true, StylePilot, `{" "}`},
		{"history", shallowHistoryKind, true, StylePilot, `(("H"))`},
		{"deep history", deepHistoryKind, true, StylePilot, `(("H*"))`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node := &Node{ID: "n", Kind: tc.kind, Name: tc.kind, NameSynthesized: tc.synth}
			got := mermaidNodeShape(node, labels, Options{Style: tc.style})
			if !strings.HasPrefix(got, tc.want) {
				t.Errorf("shape = %q, want prefix %q", got, tc.want)
			}
		})
	}

	mermaid := (&Rendering{Kind: KindAction, Roots: []*Node{
		{ID: "start", Kind: startKind, NameSynthesized: true},
		{ID: "fork", Kind: "fork", Name: "split"},
	}}).Mermaid()
	if !strings.Contains(mermaid, "classDef control fill:#181818,stroke:#181818\n") ||
		!strings.Contains(mermaid, "class start,fork control\n") ||
		!strings.Contains(mermaid, "1 fork/join name(s) (split); Mermaid's fork bar draws no label") {
		t.Errorf("control symbols or fork notice missing:\n%s", mermaid)
	}
}

func TestMermaidMarkdownLabelsFallBackWhenUnsafe(t *testing.T) {
	labels := labeller{}
	for _, name := range []string{"wheels : Wheel [0..*]", "my_snake"} {
		node := &Node{ID: "n", Kind: "part", Name: name}
		if got := mermaidNodeLabel(node, labels, Options{}); !strings.Contains(got, "**") {
			t.Errorf("safe label %q has no Markdown:\n%s", name, got)
		}
	}
	for _, name := range []string{"*b*", "_x_", "a`b", "a<b", "a#b", "1. item"} {
		node := &Node{ID: "n", Kind: "part", Name: name}
		if got := mermaidNodeLabel(node, labels, Options{}); strings.Contains(got, "**") {
			t.Errorf("unsafe label %q uses Markdown:\n%s", name, got)
		}
	}
}

func TestMermaidFrontmatterAndPaletteClasses(t *testing.T) {
	for _, kind := range []Kind{KindTree, KindState, KindSequence} {
		source := (&Rendering{Kind: kind, Roots: []*Node{{ID: "n", Kind: "state", Name: "n"}}}).Mermaid()
		if !strings.HasPrefix(source, "---\nconfig:\n  theme: base\n") ||
			!strings.Contains(source, `fontFamily: "Helvetica, Arial, sans-serif"`) ||
			!strings.Contains(source, `fontSize: "14px"`) ||
			!strings.Contains(source, `actorLineColor: "#181818"`) ||
			!strings.Contains(source, `transitionColor: "#181818"`) {
			t.Errorf("%s frontmatter:\n%s", kind, source)
		}
	}
	rendering := &Rendering{Kind: KindInterconnection, Roots: []*Node{
		{ID: "a", Kind: "part", Name: "a"},
		{ID: "b", Kind: "part", Name: "b"},
	}}
	source := rendering.MermaidWith(Options{Palette: PaletteOkabeIto})
	if !FormMermaid.TakesPalette() || !strings.Contains(source, "classDef palette0 ") ||
		!strings.Contains(source, "class a,b palette0") {
		t.Errorf("palette classes are not shared in first-use order:\n%s", source)
	}
	cameo := rendering.MermaidWith(Options{Style: StyleCameo})
	if !strings.Contains(cameo, `fontFamily: "Arial, Helvetica, sans-serif"`) ||
		!strings.Contains(cameo, `fontSize: "11px"`) ||
		!strings.Contains(cameo, "%% style cameo") ||
		!strings.Contains(cameo, "Mermaid has no diagram frame with header tab") {
		t.Errorf("Cameo frontmatter or notice:\n%s", cameo)
	}
	firstFill := strings.SplitN(cameoBlockFill, ":", 2)[0]
	for _, want := range []string{
		fmt.Sprintf(`primaryColor: %q`, firstFill),
		fmt.Sprintf(`primaryBorderColor: %q`, cameoBlockLine),
		fmt.Sprintf(`textColor: %q`, cameoTextColor),
		fmt.Sprintf(`actorBkg: %q`, firstFill),
		fmt.Sprintf(`actorLineColor: %q`, cameoEdgeColor),
		fmt.Sprintf(`transitionColor: %q`, cameoEdgeColor),
	} {
		if !strings.Contains(cameo, want) {
			t.Errorf("Cameo theme variable %q missing:\n%s", want, cameo)
		}
	}
}

func TestMermaidStyles(t *testing.T) {
	node := &Style{
		Fill: "#FFFFFF", Line: "#181818", Text: "#FF0000", Font: "Times New Roman",
		FontSize: 18, Bold: true, Italic: true,
	}
	if got, want := mermaidStyleCSS(node),
		"fill:#FFFFFF,stroke:#181818,color:#FF0000,font-family:Times New Roman,font-size:18px,font-weight:bold,font-style:italic"; got != want {
		t.Errorf("node CSS = %q, want %q", got, want)
	}
	if got := mermaidStyleCSS(&Style{Font: "Arial, Helvetica, sans-serif"}); got != "" {
		t.Errorf("unsafe font family CSS = %q, want empty", got)
	}
	rendering := &Rendering{Kind: KindInterconnection, Roots: []*Node{{
		ID: "a", Kind: "part", Name: "a", Style: node,
	}, {ID: "b", Kind: "part", Name: "b"}}, Edges: []Edge{{
		From: "a", To: "b", Kind: EdgeConnection, Label: "styled",
		Style: &Style{Line: "#181818", Text: "#00AA00", Font: "Times New Roman", FontSize: 22, Bold: true, Italic: true},
	}}}
	source := rendering.MermaidWith(Options{Palette: PaletteOkabeIto})
	if !strings.Contains(source, "style a fill:#FFFFFF,stroke:#181818,color:#FF0000,font-family:Times New Roman") ||
		!strings.Contains(source, "linkStyle 0 stroke:#181818,color:#00AA00,font-family:Times New Roman,font-size:22px,font-weight:bold,font-style:italic,stroke-width:3px") {
		t.Errorf("node and edge styles are not drawn after palette classes:\n%s", source)
	}
	unrepresentable := (&Rendering{Kind: KindAction, Roots: []*Node{{
		ID: "a", Kind: "action", Name: "a", Style: &Style{Font: "Arial, Helvetica"},
	}}}).Mermaid()
	if strings.Contains(unrepresentable, "font-family:") ||
		!strings.Contains(unrepresentable, "1 node font style(s) not represented") {
		t.Errorf("unsupported font family is not reported precisely:\n%s", unrepresentable)
	}
}

func TestMermaidUnsupportedStyleCounts(t *testing.T) {
	rendering := &Rendering{Kind: KindState, Roots: []*Node{{
		ID: "a", Kind: "state", Name: "a", Style: &Style{
			Fill: "#FFFFFF", Line: "#181818", Text: "#000000", Font: "Arial, Helvetica",
		},
	}, {ID: "b", Kind: "state", Name: "b"}}, Edges: []Edge{{
		From: "a", To: "b", Style: &Style{
			Fill: "#FFFFFF", Line: "#181818", Text: "#000000", FontSize: 16, Bold: true,
		},
	}}}
	source := rendering.Mermaid()
	for _, want := range []string{
		"1 node font style(s) not represented",
		"1 edge font style(s) not represented",
		"3 style field(s) not represented",
	} {
		if !strings.Contains(source, want) {
			t.Errorf("missing precise unsupported-style notice %q:\n%s", want, source)
		}
	}
}

func TestMermaidPaletteParityWithDOT(t *testing.T) {
	definition := regexp.MustCompile(`^\s*classDef (palette[0-9]+) fill:(#[0-9A-F]{6}),stroke:(#[0-9A-F]{6})$`)
	assignment := regexp.MustCompile(`^\s*class ([^ ]+) (palette[0-9]+)$`)
	for _, tc := range plantumlGoldenCases {
		if tc.kind == KindSequence {
			continue
		}
		rendering := render(t, tc.file, tc.view)
		for _, palette := range Palettes() {
			fills, err := rendering.Fills(palette)
			if err != nil {
				t.Fatalf("%s %s fills: %v", tc.name, palette, err)
			}
			source := rendering.MermaidWith(Options{Palette: palette})
			classes, ids := map[string]Fill{}, map[string]string{}
			for _, line := range strings.Split(source, "\n") {
				if match := definition.FindStringSubmatch(line); match != nil {
					classes[match[1]] = Fill{Fill: match[2], Border: match[3]}
				}
				if match := assignment.FindStringSubmatch(line); match != nil {
					for _, id := range strings.Split(match[1], ",") {
						ids[id] = match[2]
					}
				}
			}
			for id, want := range fills {
				if got := classes[ids[id]]; got != want {
					t.Errorf("%s %s node %s Mermaid fill = %+v, want DOT fill %+v\n%s", tc.name, palette, id, got, want, source)
				}
			}
		}
	}
}

func TestGoldenMermaidPalette(t *testing.T) {
	rendering := render(t, "interconnection.sysml", "PlantViews::loopView")
	checkGolden(t, filepath.Join("testdata", "interconnection.okabe-ito.mermaid.golden"),
		rendering.MermaidWith(Options{Palette: PaletteOkabeIto}))
}

func TestMermaidEdgeSyntaxAndLinkStyleIndices(t *testing.T) {
	if got, want := []string{mermaidArrow(EdgeConnection), mermaidArrow(EdgeBinding), mermaidArrow(EdgeFlow), mermaidArrow(EdgeSuccession)},
		[]string{"===", "===", "-.->", "-->"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("arrows = %v, want %v", got, want)
	}
	rendering := &Rendering{Kind: KindTree, Roots: []*Node{{
		ID: "root", Kind: "part def", Name: "root",
		Children: []*Node{{ID: "child", Kind: "part", Name: "child"}},
	}}, Edges: []Edge{{
		From: "root", To: "child", Kind: EdgeConnection, Label: "x",
		Style: &Style{Line: "#123456", Text: "#654321", Font: "Times New Roman", FontSize: 22, Bold: true, Italic: true},
	}}, Notes: []Note{{Anchor: "root", Text: "note"}}}
	source := rendering.Mermaid()
	if !strings.Contains(source, `root ===|"x"| child`) ||
		!strings.Contains(source, "linkStyle 1 stroke:#123456,color:#654321,font-family:Times New Roman,font-size:22px,font-weight:bold,font-style:italic,stroke-width:3px") ||
		!strings.Contains(source, "note0 -.- root") ||
		!strings.HasSuffix(source, "  linkStyle 1 stroke:#123456,color:#654321,font-family:Times New Roman,font-size:22px,font-weight:bold,font-style:italic,stroke-width:3px\n") {
		t.Errorf("edge or linkStyle order:\n%s", source)
	}
	if _, edges := MermaidSize(source); edges != 4 {
		t.Errorf("source edge count = %d, want containment, edge, note anchor and ceiling offset", edges)
	}
	for _, arrow := range []string{"a --> b", "a --- b", "a -.-> b", "a -.- b", "a === b", "a->>b:"} {
		if !declaresEdge(arrow) {
			t.Errorf("declaresEdge(%q) = false", arrow)
		}
	}
	if mermaidEdges("---\nconfig:\n  theme: base\n---\nflowchart LR\n%% a --> b\n  a[\"line\n b --> c\"]\n") != 0 {
		t.Error("frontmatter, comments or Markdown-label continuation counted as an edge")
	}
}

func TestMermaidNotesInEachGrammar(t *testing.T) {
	origin := Origin{Doc: "notes.sysml", Span: source.Span{Len: 1}}
	flow := (&Rendering{Kind: KindInterconnection,
		Roots: []*Node{{ID: "a", Kind: "part", Name: "a"}, {ID: "b", Kind: "part", Name: "b"}},
		Edges: []Edge{{From: "a", To: "b", Kind: EdgeConnection}},
		Notes: []Note{
			{Text: "first", Anchor: "a", Origin: origin},
			{Text: "second", EdgeFrom: "a", EdgeTo: "b", Origin: origin},
			{Text: "free"},
		},
	}).Mermaid()
	if !strings.Contains(flow, `note0@{ shape: notch-rect, label: "first<br>second" }`) ||
		strings.Count(flow, "note0 -.- a") != 2 {
		t.Errorf("flowchart notes or anchor:\n%s", flow)
	}
	state := (&Rendering{Kind: KindState, Roots: []*Node{{
		ID: "machine", Kind: "state", Name: "machine", Children: []*Node{
			{ID: "start", Kind: startKind, NameSynthesized: true},
			{ID: "on", Kind: "state", Name: "on"},
			{ID: "final", Kind: "final", NameSynthesized: true},
		},
	}}, Edges: []Edge{{From: "start", To: "on"}, {From: "on", To: "final"}, {From: "start", To: "final"}},
		Notes: []Note{{Anchor: "on", Text: "state note"}}}).Mermaid()
	if !strings.Contains(state, "on --> [*]") || !strings.Contains(state, "[*] --> [*]") ||
		!strings.Contains(state, "note right of on") ||
		strings.Contains(state, "state \"\" as final") {
		t.Errorf("state final transition or note:\n%s", state)
	}
	sequence := (&Rendering{Kind: KindSequence,
		Roots: []*Node{{ID: "a", Kind: "part", Name: "a"}, {ID: "b", Kind: "part", Name: "b"}},
		Edges: []Edge{{From: "a", To: "b", Kind: EdgeFlow}},
		Notes: []Note{{Anchor: "a", Text: "participant"}, {EdgeFrom: "a", EdgeTo: "b", Text: "message"}, {Text: "free"}},
	}).Mermaid()
	if !strings.Contains(sequence, "Note over a: participant") ||
		!strings.Contains(sequence, "Note over a,b: message") ||
		!strings.Contains(sequence, "1 free note(s) not drawn") {
		t.Errorf("sequence notes or notice:\n%s", sequence)
	}
	stateNotes := (&Rendering{Kind: KindState, Roots: []*Node{{
		ID: "machine", Kind: "state", Name: "machine", Children: []*Node{
			{ID: "start", Kind: startKind, NameSynthesized: true},
			{ID: "on", Kind: "state", Name: "on"},
		},
	}}, Notes: []Note{
		{Anchor: "start", Text: "pseudo"},
		{Anchor: "missing", Text: "unknown"},
		{EdgeFrom: "on", EdgeTo: "missing", Text: "edge"},
	}}).Mermaid()
	if !strings.Contains(stateNotes, "3 note(s) not drawn: Mermaid state notes need an anchor to a declared state") ||
		strings.Contains(stateNotes, "note right of start") ||
		strings.Contains(stateNotes, "note right of missing") {
		t.Errorf("unsupported state note anchors:\n%s", stateNotes)
	}
	reversed := (&Rendering{Kind: KindSequence,
		Roots: []*Node{{ID: "a", Kind: "part", Name: "a"}, {ID: "b", Kind: "part", Name: "b"}},
		Edges: []Edge{{From: "b", To: "a", Kind: EdgeFlow}},
		Notes: []Note{{EdgeFrom: "a", EdgeTo: "b", Text: "reverse"}},
	}).Mermaid()
	messageAt, noteAt := strings.Index(reversed, "b->>a:"), strings.Index(reversed, "Note over a,b: reverse")
	if messageAt < 0 || noteAt < messageAt {
		t.Errorf("reversed sequence note is not anchored after the matching message:\n%s", reversed)
	}
	unresolvedFlow := (&Rendering{Kind: KindInterconnection,
		Roots: []*Node{{ID: "a", Kind: "part", Name: "a"}},
		Notes: []Note{{Anchor: "missing", Text: "node"}, {EdgeFrom: "missing", EdgeTo: "a", Text: "edge"}},
	}).Mermaid()
	if !strings.Contains(unresolvedFlow, "2 note anchor(s) not represented") ||
		strings.Contains(unresolvedFlow, "note0 -.- missing") {
		t.Errorf("unsupported flowchart note anchors:\n%s", unresolvedFlow)
	}
	unresolvedSequence := (&Rendering{Kind: KindSequence,
		Roots: []*Node{{ID: "a", Kind: "part", Name: "a"}},
		Notes: []Note{{Anchor: "missing", Text: "participant"}, {EdgeFrom: "a", EdgeTo: "missing", Text: "message"}},
	}).Mermaid()
	if !strings.Contains(unresolvedSequence, "2 note(s) not drawn: Mermaid sequence notes need declared participant anchors") ||
		strings.Contains(unresolvedSequence, "Note over missing") {
		t.Errorf("unsupported sequence note anchors:\n%s", unresolvedSequence)
	}
	emptyFlow := (&Rendering{Kind: KindTree, Notes: []Note{{Text: "free"}}}).Mermaid()
	if !strings.Contains(emptyFlow, `note0@{ shape: notch-rect, label: "free" }`) {
		t.Errorf("free note missing from an otherwise empty flowchart:\n%s", emptyFlow)
	}
}

func TestMermaidMixedStateFinalKeepsCrossBodyTarget(t *testing.T) {
	source := (&Rendering{Kind: KindState, Roots: []*Node{
		{ID: "body", Kind: "state", Name: "body", Children: []*Node{
			{ID: "start", Kind: startKind, NameSynthesized: true},
			{ID: "local", Kind: "state", Name: "local"},
			{ID: "finish", Kind: "final", NameSynthesized: true},
		}},
		{ID: "other", Kind: "state", Name: "other"},
	}, Edges: []Edge{
		{From: "start", To: "finish"},
		{From: "local", To: "finish"},
		{From: "other", To: "finish"},
	}}).Mermaid()
	if !strings.Contains(source, `state "final" as finish`) ||
		!strings.Contains(source, `[*] --> [*]`) ||
		!strings.Contains(source, `local --> [*]`) ||
		!strings.Contains(source, `other --> finish`) ||
		!strings.Contains(source, "1 transition(s) into a final cross state bodies") ||
		strings.Contains(source, `[*] --> finish`) {
		t.Errorf("mixed same-body and cross-body final transitions:\n%s", source)
	}
}

func TestMermaidPortsUseOnlyConnectedPins(t *testing.T) {
	rendering := &Rendering{Kind: KindAction, Roots: []*Node{
		{ID: "a", Kind: "action", Name: "a", Ports: []Port{
			{ID: "a.0", Name: "used"}, {ID: "a.1", Name: "unused"},
		}},
		{ID: "b", Kind: "action", Name: "b", Ports: []Port{{ID: "b.0", Name: "in"}}},
	}, Edges: []Edge{{From: "a", To: "b", FromPort: "a.0", ToPort: "b.0", Kind: EdgeFlow}}}
	source := rendering.Mermaid()
	if !strings.Contains(source, "subgraph a [") ||
		!strings.Contains(source, `a_p0["used"]`) ||
		strings.Contains(source, "a_p1") ||
		!strings.Contains(source, "a_p0 -.-> b_p0") {
		t.Errorf("port subgraphs or ends:\n%s", source)
	}
}

func TestMermaidNestedTreeContainmentUsesAnchor(t *testing.T) {
	rendering := &Rendering{Kind: KindTree, Roots: []*Node{
		{ID: "outer", Kind: "part def", Name: "outer", Children: []*Node{
			{ID: "parent", Kind: "part def", Name: "parent", Children: []*Node{
				{ID: "child", Kind: "part", Name: "child"},
			}},
		}},
	}}
	source := rendering.Mermaid()
	if !strings.Contains(source, `mermaid_anchor_parent((" "))`) ||
		!strings.Contains(source, "mermaid_anchor_parent --- child") ||
		strings.Contains(source, "\n  parent --- child\n") ||
		!strings.Contains(source, "outer --- parent") ||
		!strings.Contains(source, "class mermaid_anchor_parent treeAnchor") {
		t.Errorf("nested containment anchor or links:\n%s", source)
	}
}

func TestMermaidPicturesAndNotices(t *testing.T) {
	dir := t.TempDir()
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "local.png"), imageData.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	rendering := &Rendering{
		Kind: KindTree, Roots: []*Node{{ID: "n", Kind: "part", Name: "part"}},
		Pictures: []Picture{
			{Location: "local.png", Dir: dir, X: 10, Y: 20, Width: 40, Height: 30},
			{Location: "missing.png", Dir: dir, X: 50, Y: 60, Width: 20, Height: 10, Above: true},
		},
	}
	source := rendering.Mermaid()
	if !strings.Contains(source, `picture0@{ img: "`+filepath.ToSlash(filepath.Join(dir, "local.png"))+`", label: "", w: 40, h: 30 }`) ||
		!strings.Contains(source, "%% layout: picture0 x=10 y=20 w=40 h=30") ||
		strings.Contains(source, "picture1@{") ||
		!strings.Contains(source, "missing.png at (50, 60) size 20×10; the file does not read") {
		t.Errorf("picture source, geometry or notice:\n%s", source)
	}
}

func TestInlineMermaidImages(t *testing.T) {
	dir := t.TempDir()
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "local.png"), imageData.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	source := "---\nconfig:\n  theme: base\n---\n%% view: V\nflowchart LR\n  picture0@{ img: \"local.png\", label: \"alt\", w: 1, h: 1 }\n"
	got := InlineMermaidImages(source, dir)
	wantData := "data:image/png;base64," + base64.StdEncoding.EncodeToString(imageData.Bytes())
	if !strings.Contains(got, wantData) || strings.Contains(got, `img: "local.png"`) {
		t.Errorf("image is not inlined:\n%s", got)
	}
	missing := InlineMermaidImages(strings.ReplaceAll(source, "local.png", "missing.png"), dir)
	if strings.Contains(missing, "picture0@{") ||
		!strings.Contains(missing, "%% not represented: picture missing.png not drawn; the file does not read") {
		t.Errorf("missing image is not dropped with a notice:\n%s", missing)
	}
	unsupportedPath := filepath.Join(dir, "unsupported.img")
	if err := os.WriteFile(unsupportedPath, []byte("not an image"), 0o600); err != nil {
		t.Fatal(err)
	}
	unsupported := InlineMermaidImages(strings.ReplaceAll(source, "local.png", "unsupported.img"), dir)
	if strings.Contains(unsupported, "picture0@{") ||
		!strings.Contains(unsupported, "the file is not a supported image") {
		t.Errorf("unsupported image is not dropped with a notice:\n%s", unsupported)
	}
	large := image.NewRGBA(image.Rect(0, 0, 600, 600))
	random := uint32(1)
	for i := range large.Pix {
		random ^= random << 13
		random ^= random >> 17
		random ^= random << 5
		large.Pix[i] = byte(random >> 24)
	}
	imageData.Reset()
	if err := png.Encode(&imageData, large); err != nil {
		t.Fatal(err)
	}
	if imageData.Len() <= MermaidTextCeiling*3/4 {
		t.Fatalf("test image is %d bytes; need enough data to exceed the Mermaid text ceiling", imageData.Len())
	}
	largePath := filepath.Join(dir, "large.png")
	if err := os.WriteFile(largePath, imageData.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	overLimit := InlineMermaidImages(strings.ReplaceAll(source, "local.png", "large.png"), dir)
	if strings.Contains(overLimit, "picture0@{") ||
		!strings.Contains(overLimit, "inlining would exceed Mermaid's text ceiling") {
		t.Errorf("over-limit image is not dropped with a notice:\n%s", overLimit)
	}
}

func TestMermaidRendersWithInstalledMMDC(t *testing.T) {
	binary := os.Getenv("OPENSYSML_MMDC")
	if binary == "" {
		return
	}
	pup := os.Getenv("OPENSYSML_MMDC_PUPPETEER")
	cases := installedMermaidCases(t)
	noHTML := filepath.Join(t.TempDir(), "nohtml.json")
	if err := os.WriteFile(noHTML, []byte(`{"flowchart":{"htmlLabels":false}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, config := range []string{"", noHTML} {
		config := config
		t.Run(configName(config), func(t *testing.T) {
			if err := renderInstalledMermaidBatch(binary, pup, config, cases); err != nil {
				bad, isolated := isolateInstalledMermaidFailure(binary, pup, config, cases)
				if !isolated {
					t.Fatalf("mmdc could not render the %s batch: %v", configName(config), err)
				}
				t.Fatalf("mmdc could not render %s: %v", bad.name, err)
			}
		})
	}
}

func TestInlineMermaidImagesWithPicturesFixture(t *testing.T) {
	rendering := mermaidPicturesFixture(t)
	inlined := InlineMermaidImages(rendering.Mermaid(), "")
	if strings.Count(inlined, "data:image/png;base64,") != 2 ||
		strings.Contains(inlined, `img: "images/`) {
		t.Errorf("picture fixture was not fully inlined:\n%s", inlined)
	}
}

type installedMermaidCase struct {
	name   string
	source string
}

func installedMermaidCases(t *testing.T) []installedMermaidCase {
	t.Helper()
	var cases []installedMermaidCase
	goldens, err := filepath.Glob(filepath.Join("testdata", "*.mermaid.golden"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range goldens {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		cases = append(cases, installedMermaidCase{name: filepath.Base(path), source: string(source)})
	}
	for _, tc := range plantumlGoldenCases {
		rendering := render(t, tc.file, tc.view)
		for _, palette := range Palettes() {
			cases = append(cases, installedMermaidCase{
				name:   tc.name + " palette " + string(palette),
				source: rendering.MermaidWith(Options{Palette: palette}),
			})
		}
		cases = append(cases, installedMermaidCase{
			name: tc.name + " cameo", source: rendering.MermaidWith(Options{Style: StyleCameo}),
		})
	}
	for _, tc := range []struct {
		name, file, view string
	}{
		{"sequence-cycle", "sequence-order.sysml", "OrderingViews::deadlockView"},
		{"sequence-empty", "errors.sysml", "ErrorViews::emptySequenceView"},
	} {
		rendering := render(t, tc.file, tc.view)
		for _, palette := range Palettes() {
			cases = append(cases, installedMermaidCase{
				name:   tc.name + " palette " + string(palette),
				source: rendering.MermaidWith(Options{Palette: palette}),
			})
		}
		cases = append(cases, installedMermaidCase{
			name: tc.name + " cameo", source: rendering.MermaidWith(Options{Style: StyleCameo}),
		})
	}
	styled := &Rendering{Kind: KindInterconnection, Roots: []*Node{
		{ID: "a", Kind: "part", Name: "styled", Style: &Style{Text: "#FF0000", Font: "Times New Roman", FontSize: 18, Bold: true, Italic: true}},
		{ID: "b", Kind: "part", Name: "plain"},
	}, Edges: []Edge{{
		From: "a", To: "b", Kind: EdgeConnection, Label: "styled edge",
		Style: &Style{Text: "#00AA00", Font: "Times New Roman", FontSize: 22, Bold: true, Italic: true},
	}}}
	cases = append(cases, installedMermaidCase{name: "node and edge styles", source: styled.Mermaid()})
	dir := t.TempDir()
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	imagePath := filepath.Join(dir, "local.png")
	if err := os.WriteFile(imagePath, imageData.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	pictured := &Rendering{Kind: KindTree, Roots: []*Node{{ID: "n", Kind: "part", Name: "pictured"}},
		Pictures: []Picture{{Location: imagePath, Width: 20, Height: 20}}}
	cases = append(cases, installedMermaidCase{name: "flowchart image shape", source: pictured.Mermaid()})
	cases = append(cases, installedMermaidCase{
		name: "pictures.sysml mixedView", source: mermaidPicturesFixture(t).Mermaid(),
	})
	symbols := &Rendering{Kind: KindAction, Roots: []*Node{
		{ID: "start", Kind: startKind, NameSynthesized: true},
		{ID: "final", Kind: "final", NameSynthesized: true},
		{ID: "junction", Kind: "junction", NameSynthesized: true},
		{ID: "fork", Kind: "fork", Name: "split"},
		{ID: "decision", Kind: "decision", NameSynthesized: true},
		{ID: "choice", Kind: "choice", Name: "choice"},
		{ID: "history", Kind: shallowHistoryKind, NameSynthesized: true},
		{ID: "deep", Kind: deepHistoryKind, NameSynthesized: true},
	}}
	cases = append(cases, installedMermaidCase{name: "flowchart symbols", source: symbols.Mermaid()})
	notes := &Rendering{Kind: KindInterconnection, Roots: []*Node{
		{ID: "a", Kind: "part", Name: "a"},
		{ID: "b", Kind: "part", Name: "b"},
	}, Edges: []Edge{{From: "a", To: "b", Kind: EdgeConnection}},
		Notes: []Note{{Text: "attached", Anchor: "a"}, {Text: "edge note", EdgeFrom: "a", EdgeTo: "b"}}}
	cases = append(cases, installedMermaidCase{name: "flowchart notes", source: notes.Mermaid()})
	ports := &Rendering{Kind: KindAction, Roots: []*Node{
		{ID: "a", Kind: "action", Name: "a", Ports: []Port{{ID: "a.0", Name: "out"}}},
		{ID: "b", Kind: "action", Name: "b", Ports: []Port{{ID: "b.0", Name: "in"}}},
	}, Edges: []Edge{{From: "a", To: "b", FromPort: "a.0", ToPort: "b.0", Kind: EdgeFlow}}}
	cases = append(cases, installedMermaidCase{name: "flowchart ports", source: ports.Mermaid()})
	state := &Rendering{Kind: KindState, Roots: []*Node{{ID: "state", Kind: "state", Name: "state",
		Style: &Style{Fill: "#FFFFFF", Line: "#181818", Text: "#000000", Font: "Times New Roman", FontSize: 18}}},
		Notes: []Note{{Text: "state note", Anchor: "state"}}}
	cases = append(cases, installedMermaidCase{name: "state styles", source: state.Mermaid()})
	mixedFinal := &Rendering{Kind: KindState, Roots: []*Node{
		{ID: "body", Kind: "state", Name: "body", Children: []*Node{
			{ID: "start", Kind: startKind, NameSynthesized: true},
			{ID: "finish", Kind: "final", NameSynthesized: true},
		}},
		{ID: "other", Kind: "state", Name: "other"},
	}, Edges: []Edge{{From: "start", To: "finish"}, {From: "other", To: "finish"}}}
	cases = append(cases, installedMermaidCase{name: "mixed state final transitions", source: mixedFinal.Mermaid()})
	sequence := &Rendering{Kind: KindSequence, Roots: []*Node{
		{ID: "a", Kind: "part", Name: "a"},
		{ID: "b", Kind: "part", Name: "b"},
	}, Edges: []Edge{{From: "a", To: "b", Kind: EdgeFlow, Label: "message"}},
		Notes: []Note{{Text: "participant", Anchor: "a"}, {Text: "message", EdgeFrom: "a", EdgeTo: "b"}}}
	cases = append(cases, installedMermaidCase{name: "sequence notes", source: sequence.Mermaid()})
	return cases
}

func mermaidPicturesFixture(t *testing.T) *Rendering {
	t.Helper()
	rendering := render(t, "pictures.sysml", "Site::mixedView")
	dir := t.TempDir()
	images := filepath.Join(dir, "images")
	if err := os.Mkdir(images, 0o700); err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"bench.png", "logo.png"} {
		if err := os.WriteFile(filepath.Join(images, name), data.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for i := range rendering.Pictures {
		rendering.Pictures[i].Dir = dir
	}
	return rendering
}

func configName(config string) string {
	if config == "" {
		return "default"
	}
	return "htmlLabels-false"
}

func renderInstalledMermaidBatch(binary, puppeteer, config string, cases []installedMermaidCase) error {
	if len(cases) == 0 {
		return nil
	}
	dir, err := os.MkdirTemp("", "opensysml-mmdc-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	input, output := filepath.Join(dir, "input.md"), filepath.Join(dir, "output.md")
	var markdown strings.Builder
	for _, tc := range cases {
		fmt.Fprintf(&markdown, "<!-- %s -->\n```mermaid\n%s\n```\n\n", tc.name, tc.source)
	}
	if err := os.WriteFile(input, []byte(markdown.String()), 0o600); err != nil {
		return err
	}
	args := []string{"-i", input, "-o", output}
	if puppeteer != "" {
		args = append(args, "-p", puppeteer)
	}
	if config != "" {
		args = append(args, "-c", config)
	}
	command := exec.Command(binary, args...)
	result, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(result)))
	}
	return nil
}

func isolateInstalledMermaidFailure(binary, puppeteer, config string, cases []installedMermaidCase) (installedMermaidCase, bool) {
	if len(cases) <= 1 {
		if len(cases) == 1 {
			return cases[0], true
		}
		return installedMermaidCase{}, false
	}
	mid := len(cases) / 2
	if err := renderInstalledMermaidBatch(binary, puppeteer, config, cases[:mid]); err != nil {
		return isolateInstalledMermaidFailure(binary, puppeteer, config, cases[:mid])
	}
	if err := renderInstalledMermaidBatch(binary, puppeteer, config, cases[mid:]); err != nil {
		return isolateInstalledMermaidFailure(binary, puppeteer, config, cases[mid:])
	}
	return installedMermaidCase{}, false
}
