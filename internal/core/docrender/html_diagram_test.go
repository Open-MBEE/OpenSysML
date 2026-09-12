package docrender

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

func renderedFigure(t *testing.T, caption string, rendering *view.Rendering, direction view.Direction) string {
	t.Helper()
	return renderedFigureForm(t, caption, rendering, direction, "")
}

func renderedFigureForm(t *testing.T, caption string, rendering *view.Rendering, direction view.Direction, form view.Form) string {
	t.Helper()
	w := &htmlWriter{}
	if err := w.writeFigure("", "d", caption, rendering, direction, form); err != nil {
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

// TestHTMLDiagramDotForm checks a diagram stating the DOT form is the DOT
// digraph in a pre element classed by its form, and a wrong form writes nothing.
func TestHTMLDiagramDotForm(t *testing.T) {
	got := renderedFigureForm(t, "Chain", graphRendering(view.KindState), view.DirectionLeftRight, view.FormDot)
	for _, want := range []string{`data-diagram-kind="state"`, `data-direction="LR"`, `<pre class="dot">// kind: state`, "graph [rankdir=LR];", `&#34;n0&#34; -&gt; &#34;n1&#34; [arrowhead=none];`, `<figcaption class="sysml-caption">Chain</figcaption>`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "mermaid") {
		t.Errorf("Mermaid in a DOT figure:\n%s", got)
	}
	if got, want := renderedFigureForm(t, "", graphRendering(view.KindTree), "", view.FormMermaid), renderedFigure(t, "", graphRendering(view.KindTree), ""); got != want {
		t.Errorf("stated mermaid differs from the default:\n%s\n%s", got, want)
	}
	w := &htmlWriter{}
	var typed *Error
	if err := w.writeFigure("", "d", "", graphRendering(view.KindSequence), "", view.FormDot); !errors.As(err, &typed) || typed.Kind != ErrorUnrenderableForm {
		t.Fatalf("sequence as dot: error = %v", err)
	}
	if err := w.writeFigure("", "d", "", graphRendering(view.KindTree), "", "svg"); !errors.As(err, &typed) || typed.Kind != ErrorUnknownForm {
		t.Fatalf("svg: error = %v", err)
	}
	if w.b.Len() != 0 {
		t.Errorf("a refused figure left output behind:\n%s", w.b.String())
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
	empty := renderedFigure(t, "", &view.Rendering{Kind: view.KindTable}, "")
	if !strings.Contains(empty, "the view exposes nothing; the rendering is empty") {
		t.Errorf("empty table unexplained: %s", empty)
	}
}

// TestHTMLDiagramErrors checks the typed errors for a diagram with no
// rendering and for a kind no renderer can draw.
func TestHTMLDiagramErrors(t *testing.T) {
	w := &htmlWriter{}
	var typed *Error
	if err := w.writeFigure("", "d", "", nil, "", ""); !errors.As(err, &typed) || typed.Kind != ErrorMissingRendering {
		t.Fatalf("error = %v, want %s", err, ErrorMissingRendering)
	}
	for _, kind := range []view.Kind{view.KindTextual, view.KindGeometry} {
		err := w.writeFigure("", "d", "", &view.Rendering{Kind: kind}, "", "")
		if !errors.As(err, &typed) || typed.Kind != ErrorUnrenderableDiagram {
			t.Fatalf("%s: error = %v, want %s", kind, err, ErrorUnrenderableDiagram)
		}
		if !strings.Contains(typed.Error(), "which HTML cannot draw") {
			t.Errorf("%s: message names the wrong backend: %s", kind, typed.Error())
		}
	}
}
