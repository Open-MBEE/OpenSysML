package docrender

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/view"
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
	w := &htmlWriter{form: form}
	if err := w.writeFigure("", "d", caption, rendering, options); err != nil {
		t.Fatalf("writeFigure: %v", err)
	}
	return w.b.String()
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
	w := &htmlWriter{form: view.FormDot}
	var typed *Error
	if err := w.writeFigure("", "d", "", graphRendering(view.KindSequence), view.Options{}); !errors.As(err, &typed) || typed.Kind != ErrorUnrenderableForm {
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
	if sequence := renderedFigureForm(t, "", graphRendering(view.KindSequence), "", view.FormPlantUML); !strings.Contains(sequence, `<pre class="plantuml">`) || !strings.Contains(sequence, "participant &#34;**a**") || !strings.Contains(sequence, "n0 -&gt; n1\n") {
		t.Errorf("sequence as plantuml:\n%s", sequence)
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
	if dot := renderedFigureForm(t, "Masses", &view.Rendering{Kind: view.KindTable, Columns: []string{"name"}, Rows: [][]string{{"optics"}}}, "", view.FormDot); !strings.Contains(dot, `<table class="sysml-table"`) || strings.Contains(dot, "<pre") {
		t.Errorf("a table is not a table in the dot form:\n%s", dot)
	}
	empty := renderedFigure(t, "", &view.Rendering{Kind: view.KindTable}, "")
	if !strings.Contains(empty, "the view exposes nothing; the rendering is empty") {
		t.Errorf("empty table unexplained: %s", empty)
	}
}

// TestHTMLDiagramErrors checks the typed errors for a diagram with no
// rendering and for a kind no renderer can draw.
func TestHTMLDiagramErrors(t *testing.T) {
	w := &htmlWriter{form: view.FormMermaid}
	var typed *Error
	if err := w.writeFigure("", "d", "", nil, view.Options{}); !errors.As(err, &typed) || typed.Kind != ErrorMissingRendering {
		t.Fatalf("error = %v, want %s", err, ErrorMissingRendering)
	}
	for _, kind := range []view.Kind{view.KindTextual, view.KindGeometry} {
		err := w.writeFigure("", "d", "", &view.Rendering{Kind: kind}, view.Options{})
		if !errors.As(err, &typed) || typed.Kind != ErrorUnrenderableDiagram {
			t.Fatalf("%s: error = %v, want %s", kind, err, ErrorUnrenderableDiagram)
		}
		if !strings.Contains(typed.Error(), "which HTML cannot draw") {
			t.Errorf("%s: message names the wrong backend: %s", kind, typed.Error())
		}
	}
}
