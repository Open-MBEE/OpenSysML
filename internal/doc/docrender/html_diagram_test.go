package docrender

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

func renderedFigure(t *testing.T, caption string, rendering *view.Rendering, direction view.Direction) string {
	t.Helper()
	return renderedFigureForm(t, caption, rendering, direction, view.FormMermaid)
}

func renderedFigureForm(t *testing.T, caption string, rendering *view.Rendering, direction view.Direction, form view.Form) string {
	t.Helper()
	return renderedFigureOptions(t, caption, rendering, view.Options{Direction: direction}, form)
}

func renderedFigureOptions(t *testing.T, caption string, rendering *view.Rendering, options view.Options, form view.Form) string {
	t.Helper()
	w := &htmlWriter{forms: DiagramOptions{Form: form}}
	if err := w.writeFigure("", "d", captionOf(caption), rendering, options); err != nil {
		t.Fatalf("writeFigure: %v", err)
	}
	return w.b.String()
}

func TestHTMLDiagramMermaidInlinesLocalPictures(t *testing.T) {
	dir := t.TempDir()
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "picture.png")
	if err := os.WriteFile(path, data.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	rendering := &view.Rendering{
		Kind:  view.KindTree,
		Roots: []*view.Node{{ID: "n", Kind: "part", Name: "pictured"}},
		Pictures: []view.Picture{{
			Location: path,
			Width:    20,
			Height:   20,
		}},
	}
	got := renderedFigure(t, "", rendering, "")
	if !strings.Contains(got, "data:image/png;base64,") || strings.Contains(got, path) {
		t.Errorf("local picture was not inlined into Mermaid source:\n%s", got)
	}
}

func TestHTMLDiagramMermaidRefusesUnsafePictures(t *testing.T) {
	dir := t.TempDir()
	active := filepath.Join(dir, "script.svg")
	if err := os.WriteFile(active, []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script/></svg>`), 0o600); err != nil {
		t.Fatal(err)
	}
	rendering := &view.Rendering{
		Kind:  view.KindInterconnection,
		Roots: []*view.Node{{ID: "n", Kind: "part", Name: "pictured"}},
		Pictures: []view.Picture{
			{Location: active, X: 1, Y: 2, Width: 20, Height: 30},
			{Location: "https://example.org/a.png", X: 3, Y: 4, Width: 40, Height: 50},
		},
	}
	got := renderedFigure(t, "", rendering, "")
	for _, unwanted := range []string{"data:image/svg+xml", "<script", `img: &#34;https://example.org/a.png`} {
		if strings.Contains(got, unwanted) {
			t.Errorf("HTML contains refused picture content %q:\n%s", unwanted, got)
		}
	}
	if count := strings.Count(got, "&lt;script"); count != 1 {
		t.Errorf("HTML has %d escaped script construct(s), want only the notice: %s", count, got)
	}
	for _, want := range []string{
		"the SVG has active content (&lt;script&gt;)",
		"remote pictures are not drawn",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("HTML lacks escaped picture notice %q:\n%s", want, got)
		}
	}
}

// TestHTMLDiagramMermaidKinds checks every graph-shaped kind is Mermaid source
// in a figure, drawn in the diagram's direction.
func TestHTMLDiagramMermaidKinds(t *testing.T) {
	for kind, want := range map[view.Kind]string{
		view.KindTree:            "flowchart TD",
		view.KindInterconnection: "flowchart LR",
		view.KindAction:          "flowchart TD",
		view.KindState:           "stateDiagram-v2",
		view.KindSequence:        "sequenceDiagram",
	} {
		got := renderedFigure(t, "", graphRendering(kind), "")
		if !strings.Contains(got, `<pre class="mermaid">`) || !strings.Contains(got, want) {
			t.Errorf("%s: %s", kind, got)
		}
		if !strings.Contains(got, `data-diagram-kind="`+string(kind)+`"`) {
			t.Errorf("%s: kind not carried: %s", kind, got)
		}
	}
	got := renderedFigure(t, "flow of a|b", graphRendering(view.KindTree), view.DirectionRightLeft)
	if !strings.Contains(got, `data-direction="RL"`) || !strings.Contains(got, "flowchart RL") {
		t.Errorf("direction not carried: %s", got)
	}
	if !strings.Contains(got, `<figcaption class="sysml-caption">flow of a|b</figcaption>`) {
		t.Errorf("caption: %s", got)
	}
}

// TestHTMLDiagramDotForm checks a DOT render writes each diagram as its digraph
// in a pre element classed by the form, and a kind DOT cannot write fails.
func TestHTMLDiagramDotForm(t *testing.T) {
	got := renderedFigureForm(t, "Chain", graphRendering(view.KindState), view.DirectionLeftRight, view.FormDot)
	for _, want := range []string{`data-diagram-kind="state"`, `data-direction="LR"`, `<pre class="dot">// kind: state`, "graph [fontname=&#34;Helvetica&#34;, rankdir=LR];", `&#34;n0&#34; -&gt; &#34;n1&#34; [arrowhead=none, penwidth=3];`, `<figcaption class="sysml-caption">Chain</figcaption>`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "mermaid") {
		t.Errorf("Mermaid in a DOT figure:\n%s", got)
	}
	got = renderedFigureOptions(t, "", graphRendering(view.KindTree), view.Options{Palette: view.PaletteOkabeIto}, view.FormDot)
	if !strings.Contains(got, `data-palette="okabe-ito"`) || !strings.Contains(got, `fillcolor=&#34;#`) {
		t.Errorf("palette not carried into the figure:\n%s", got)
	}
	if strings.Contains(renderedFigureForm(t, "", graphRendering(view.KindTree), "", view.FormDot), "data-palette") {
		t.Errorf("an unfilled figure carries a palette attribute")
	}
	mermaid := renderedFigureOptions(t, "", graphRendering(view.KindTree), view.Options{Palette: view.PaletteOkabeIto}, view.FormMermaid)
	if !strings.Contains(mermaid, `data-palette="okabe-ito"`) || !strings.Contains(mermaid, "classDef palette0 ") {
		t.Errorf("Mermaid palette not carried into the figure:\n%s", mermaid)
	}
	w := &htmlWriter{forms: DiagramOptions{Form: view.FormDot}}
	var typed *Error
	if err := w.writeFigure("", "d", caption{}, graphRendering(view.KindSequence), view.Options{}); !errors.As(err, &typed) || typed.Kind != ErrorUnrenderableForm {
		t.Fatalf("sequence as dot: error = %v", err)
	}
	if w.b.Len() != 0 {
		t.Errorf("a refused figure left output behind:\n%s", w.b.String())
	}
}

// TestHTMLDiagramPlantUMLForm checks a PlantUML render writes each diagram as
// its source in a pre element classed by the form, the palette as fills, and
// the sequence DOT refuses.
func TestHTMLDiagramPlantUMLForm(t *testing.T) {
	got := renderedFigureForm(t, "Chain", graphRendering(view.KindState), view.DirectionLeftRight, view.FormPlantUML)
	for _, want := range []string{`data-diagram-kind="state"`, `data-direction="LR"`, `<pre class="plantuml">@startuml` + "\n&#39; state rendering", "&lt;style&gt;\n", "left to right direction\n", "n0 -[thickness=3]- n1\n", "@enduml</pre>", `<figcaption class="sysml-caption">Chain</figcaption>`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "mermaid") || strings.Contains(got, "digraph") {
		t.Errorf("another form in a PlantUML figure:\n%s", got)
	}
	got = renderedFigureOptions(t, "", graphRendering(view.KindTree), view.Options{Palette: view.PaletteOkabeIto}, view.FormPlantUML)
	if !strings.Contains(got, `data-palette="okabe-ito"`) || !strings.Contains(got, `&lt;&lt;usage&gt;&gt; #`) {
		t.Errorf("palette not carried into the figure:\n%s", got)
	}
	if strings.Contains(renderedFigureForm(t, "", graphRendering(view.KindTree), "", view.FormPlantUML), "data-palette") {
		t.Errorf("an unfilled figure carries a palette attribute")
	}
	if sequence := renderedFigureForm(t, "", graphRendering(view.KindSequence), "", view.FormPlantUML); !strings.Contains(sequence, `<pre class="plantuml">`) || !strings.Contains(sequence, "participant &#34;&lt;size:10&gt;//«part»//&lt;/size&gt;\\n**a**&#34;") || !strings.Contains(sequence, "n0 -&gt; n1\n") {
		t.Errorf("sequence as plantuml:\n%s", sequence)
	}
}

// TestHTMLDiagramD2Form checks a D2 render writes each diagram as its source
// in a pre element classed by the form, the palette as fills, and the
// sequence as D2's sequence diagram.
func TestHTMLDiagramD2Form(t *testing.T) {
	got := renderedFigureForm(t, "Chain", graphRendering(view.KindState), view.DirectionLeftRight, view.FormD2)
	for _, want := range []string{`data-diagram-kind="state"`, `data-direction="LR"`, `<pre class="d2"># state rendering`, "direction: right\n", "classes: {\n", "\nn0 -- n1: { class: connection }</pre>", `<figcaption class="sysml-caption">Chain</figcaption>`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "mermaid") || strings.Contains(got, "digraph") || strings.Contains(got, "@startuml") {
		t.Errorf("another form in a D2 figure:\n%s", got)
	}
	got = renderedFigureOptions(t, "", graphRendering(view.KindTree), view.Options{Palette: view.PaletteOkabeIto}, view.FormD2)
	if !strings.Contains(got, `data-palette="okabe-ito"`) || !strings.Contains(got, `style: { fill: &#34;#`) {
		t.Errorf("palette not carried into the figure:\n%s", got)
	}
	if strings.Contains(renderedFigureForm(t, "", graphRendering(view.KindTree), "", view.FormD2), "data-palette") {
		t.Errorf("an unfilled figure carries a palette attribute")
	}
	if sequence := renderedFigureForm(t, "", graphRendering(view.KindSequence), "", view.FormD2); !strings.Contains(sequence, `<pre class="d2">`) || !strings.Contains(sequence, "shape: sequence_diagram\n") || !strings.Contains(sequence, "  n0 -- n1: { class: connection }\n}</pre>") {
		t.Errorf("sequence as d2:\n%s", sequence)
	}
}

// TestHTMLDiagramTableKind checks a table-kind view renders as a real table,
// keeps its notices as comments, and explains an empty rendering.
func TestHTMLDiagramTableKind(t *testing.T) {
	got := renderedFigure(t, "Masses", &view.Rendering{
		Kind:    view.KindTable,
		Columns: []string{"name", "mass"},
		Rows:    [][]string{{"optics", "8.5"}, {"mount|base", "15"}},
		Notices: []string{"attribute stage --> not projected"},
	}, "")
	for _, want := range []string{
		"<!-- not represented: attribute stage - -> not projected -->",
		`<table class="sysml-table" data-content="table">`,
		`<th scope="col" data-column="mass">mass</th>`,
		`<td class="sysml-cell" data-column="name">mount|base</td>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("table figure lacks %q:\n%s", want, got)
		}
	}
	for _, form := range []view.Form{view.FormDot, view.FormD2} {
		if got := renderedFigureForm(t, "Masses", &view.Rendering{Kind: view.KindTable, Columns: []string{"name"}, Rows: [][]string{{"optics"}}}, "", form); !strings.Contains(got, `<table class="sysml-table"`) || strings.Contains(got, "<pre") {
			t.Errorf("a table is not a table in the %s form:\n%s", form, got)
		}
	}
	empty := renderedFigure(t, "", &view.Rendering{Kind: view.KindTable}, "")
	if !strings.Contains(empty, "the view exposes nothing; the rendering is empty") {
		t.Errorf("empty table unexplained: %s", empty)
	}
}

func TestHTMLDiagramMatrixKindIsATable(t *testing.T) {
	for _, form := range []view.Form{view.FormDot, view.FormD2} {
		got := renderedFigureForm(t, "Relationships", &view.Rendering{
			Kind:    view.KindMatrix,
			Columns: []string{"Source / Target", "Observatory::target"},
			Rows:    [][]string{{"Observatory::source", "satisfy"}},
		}, "", form)
		for _, want := range []string{
			`<table class="sysml-table" data-content="matrix">`,
			`<th scope="col" data-column="Source / Target">Source / Target</th>`,
			`<td class="sysml-cell" data-column="Observatory::target">satisfy</td>`,
		} {
			if !strings.Contains(got, want) {
				t.Errorf("matrix figure in %s lacks %q:\n%s", form, want, got)
			}
		}
		if strings.Contains(got, "<pre") {
			t.Errorf("matrix was written as a graph in %s:\n%s", form, got)
		}
	}
}

// TestHTMLDiagramErrors checks the typed errors for a diagram with no
// rendering and for a kind no renderer can draw.
func TestHTMLDiagramErrors(t *testing.T) {
	w := &htmlWriter{forms: DiagramOptions{Form: view.FormMermaid}}
	var typed *Error
	if err := w.writeFigure("", "d", caption{}, nil, view.Options{}); !errors.As(err, &typed) || typed.Kind != ErrorMissingRendering {
		t.Fatalf("error = %v, want %s", err, ErrorMissingRendering)
	}
	for _, kind := range []view.Kind{view.KindTextual, view.KindGeometry} {
		err := w.writeFigure("", "d", caption{}, &view.Rendering{Kind: kind}, view.Options{})
		if !errors.As(err, &typed) || typed.Kind != ErrorUnrenderableDiagram {
			t.Fatalf("%s: error = %v, want %s", kind, err, ErrorUnrenderableDiagram)
		}
		if !strings.Contains(typed.Error(), "which HTML cannot draw") {
			t.Errorf("%s: message names the wrong backend: %s", kind, typed.Error())
		}
	}
}

// captionOf is an unnumbered caption with the given text.
func captionOf(text string) caption {
	return caption{text: text}
}
