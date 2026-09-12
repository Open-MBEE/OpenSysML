package docrender

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

func graphRendering(kind view.Kind) *view.Rendering {
	return &view.Rendering{
		Kind: kind,
		Roots: []*view.Node{
			{ID: "n0", Kind: "part", Name: "a"},
			{ID: "n1", Kind: "part", Name: "b"},
		},
		Edges: []view.Edge{{From: "n0", To: "n1"}},
	}
}

func renderedDiagram(t *testing.T, caption string, rendering *view.Rendering, direction view.Direction) string {
	t.Helper()
	return renderedDiagramForm(t, caption, rendering, direction, view.FormMermaid)
}

func renderedDiagramForm(t *testing.T, caption string, rendering *view.Rendering, direction view.Direction, form view.Form) string {
	t.Helper()
	blocks, err := diagramBlocks("d", caption, rendering, direction, form)
	if err != nil {
		t.Fatalf("diagramBlocks: %v", err)
	}
	return strings.Join(blocks, "\n\n")
}

func TestDiagramMermaidKinds(t *testing.T) {
	cases := []struct {
		kind view.Kind
		want string
	}{
		{view.KindTree, "flowchart TD"},
		{view.KindInterconnection, "flowchart LR"},
		{view.KindAction, "flowchart TD"},
		{view.KindState, "stateDiagram-v2"},
		{view.KindSequence, "sequenceDiagram"},
	}
	for _, c := range cases {
		got := renderedDiagram(t, "", graphRendering(c.kind), "")
		if !strings.HasPrefix(got, "```mermaid\n") || !strings.HasSuffix(got, "\n```") {
			t.Errorf("%s: not fenced:\n%s", c.kind, got)
		}
		if !strings.Contains(got, c.want) {
			t.Errorf("%s: missing %q:\n%s", c.kind, c.want, got)
		}
	}
}

func TestDiagramDirection(t *testing.T) {
	got := renderedDiagram(t, "", graphRendering(view.KindTree), view.DirectionRightLeft)
	if !strings.Contains(got, "flowchart RL") {
		t.Errorf("tree RL: %s", got)
	}
	got = renderedDiagram(t, "", graphRendering(view.KindInterconnection), view.DirectionTopBottom)
	if !strings.Contains(got, "flowchart TB") {
		t.Errorf("interconnection TB: %s", got)
	}
	got = renderedDiagram(t, "", graphRendering(view.KindState), view.DirectionLeftRight)
	if !strings.Contains(got, "stateDiagram-v2\n  direction LR") {
		t.Errorf("state LR: %s", got)
	}
	got = renderedDiagram(t, "", graphRendering(view.KindState), "")
	if strings.Contains(got, "direction") {
		t.Errorf("undirected state carries a direction: %s", got)
	}
}

// A render in the DOT form writes each graph-shaped diagram as a dot fence of
// its digraph in the diagram's direction.
func TestDiagramDotForm(t *testing.T) {
	for _, kind := range []view.Kind{view.KindTree, view.KindInterconnection, view.KindAction, view.KindState} {
		got := renderedDiagramForm(t, "", graphRendering(kind), view.DirectionLeftRight, view.FormDot)
		if !strings.HasPrefix(got, "```dot\n// kind: "+string(kind)+"\n") || !strings.HasSuffix(got, "\n}\n```") {
			t.Errorf("%s: not a dot fence:\n%s", kind, got)
		}
		for _, want := range []string{"digraph {", "graph [rankdir=LR];", `"n0" -> "n1" [arrowhead=none];`} {
			if !strings.Contains(got, want) {
				t.Errorf("%s: missing %q:\n%s", kind, want, got)
			}
		}
		if strings.Contains(got, "flowchart") || strings.Contains(got, "stateDiagram") {
			t.Errorf("%s: Mermaid in a dot fence:\n%s", kind, got)
		}
	}
	got := renderedDiagramForm(t, "Chain", graphRendering(view.KindTree), "", view.FormDot)
	if !strings.HasPrefix(got, "<!-- caption -->\n*Chain*\n\n```dot\n") {
		t.Errorf("captioned dot: %s", got)
	}
}

// The diagram form is resolved once per render: empty is Mermaid, and a form
// no diagram is written as is a typed error before anything is written.
func TestDiagramFormResolution(t *testing.T) {
	if form, err := diagramForm(""); err != nil || form != view.FormMermaid {
		t.Fatalf("diagramForm(\"\") = %q, %v", form, err)
	}
	for _, form := range view.DiagramForms() {
		if got, err := diagramForm(form); err != nil || got != form {
			t.Fatalf("diagramForm(%s) = %q, %v", form, got, err)
		}
	}
	var typed *Error
	for _, form := range []view.Form{view.FormText, view.FormMarkdown, "svg"} {
		_, err := diagramForm(form)
		if !errors.As(err, &typed) || typed.Kind != ErrorUnknownForm || typed.DiagramForm != form {
			t.Fatalf("%s: error = %v", form, err)
		}
		if !strings.Contains(err.Error(), `"`+string(form)+`"`) || !strings.Contains(err.Error(), "mermaid, dot") {
			t.Errorf("%s: message = %q", form, err)
		}
	}
}

// A kind the chosen form does not write is a typed error naming both, and a
// table-kind view is a pipe table whichever form is chosen.
func TestDiagramFormErrors(t *testing.T) {
	var typed *Error
	_, err := diagramBlocks("d", "", graphRendering(view.KindSequence), "", view.FormDot)
	if !errors.As(err, &typed) || typed.Kind != ErrorUnrenderableForm || typed.Actual != "sequence" || typed.DiagramForm != view.FormDot {
		t.Fatalf("sequence as dot: error = %v", err)
	}
	if !strings.Contains(err.Error(), `kind "sequence"`) || !strings.Contains(err.Error(), "not written as dot") {
		t.Errorf("message = %q", err)
	}
	table := &view.Rendering{Kind: view.KindTable, Columns: []string{"a"}, Rows: [][]string{{"x"}}}
	if got := renderedDiagramForm(t, "", table, "", view.FormDot); !strings.Contains(got, "| a |") || !strings.Contains(got, "| x |") {
		t.Errorf("a table is not a table in the dot form:\n%s", got)
	}
}

func TestDiagramCaption(t *testing.T) {
	got := renderedDiagram(t, "flow of a|b", graphRendering(view.KindTree), "")
	if !strings.HasPrefix(got, "<!-- caption -->\n*flow of a\\|b*\n\n```mermaid") {
		t.Errorf("caption: %s", got)
	}
}

func TestDiagramTableKind(t *testing.T) {
	rendering := &view.Rendering{
		Kind:    view.KindTable,
		Columns: []string{"name", "mass"},
		Rows:    [][]string{{"optics", "8.5"}, {"mount|base", "15"}},
	}
	got := renderedDiagram(t, "Masses", rendering, "")
	want := "<!-- caption -->\n*Masses*\n\n<!-- table rendering -->\n| name | mass |\n| --- | --- |\n| optics | 8.5 |\n| mount\\|base | 15 |"
	if got != want {
		t.Errorf("table = %q, want %q", got, want)
	}
}

func TestDiagramTableKindEscapesMarkdownPunctuation(t *testing.T) {
	rendering := &view.Rendering{
		Kind:    view.KindTable,
		Columns: []string{"name"},
		Rows:    [][]string{{"*engine* `raw` <b>#1_a_"}},
	}
	got := renderedDiagram(t, "", rendering, "")
	if !strings.Contains(got, `| \*engine\* \`+"`"+`raw\`+"`"+` \<b>\#1\_a\_ |`) {
		t.Errorf("punctuation not literal: %s", got)
	}
}

func TestDiagramTableKindKeepsNotices(t *testing.T) {
	rendering := &view.Rendering{
		Kind:    view.KindTable,
		Columns: []string{"name"},
		Rows:    [][]string{{"optics"}},
		Notices: []string{"attribute mass is not projected"},
	}
	got := renderedDiagram(t, "", rendering, "")
	if !strings.Contains(got, "<!-- not represented: attribute mass is not projected -->") {
		t.Errorf("notice lost: %s", got)
	}
}

func TestDiagramTableKindExplainsAnEmptyRendering(t *testing.T) {
	got := renderedDiagram(t, "", &view.Rendering{Kind: view.KindTable}, "")
	if !strings.Contains(got, "the view exposes nothing; the rendering is empty") {
		t.Errorf("empty table unexplained: %s", got)
	}
}

func TestDiagramMissingRendering(t *testing.T) {
	_, err := diagramBlocks("d", "", nil, "", view.FormMermaid)
	var typed *Error
	if !errors.As(err, &typed) || typed.Kind != ErrorMissingRendering {
		t.Fatalf("error = %v, want %s", err, ErrorMissingRendering)
	}
}

func TestDiagramUnrenderableKind(t *testing.T) {
	for _, kind := range []view.Kind{view.KindTextual, view.KindGeometry} {
		_, err := diagramBlocks("d", "", &view.Rendering{Kind: kind}, "", view.FormMermaid)
		var typed *Error
		if !errors.As(err, &typed) || typed.Kind != ErrorUnrenderableDiagram {
			t.Fatalf("%s: error = %v, want %s", kind, err, ErrorUnrenderableDiagram)
		}
		if typed.Actual != string(kind) {
			t.Fatalf("%s: actual = %q", kind, typed.Actual)
		}
	}
}

func TestDiagramDeterminism(t *testing.T) {
	first := renderedDiagram(t, "c", graphRendering(view.KindState), view.DirectionLeftRight)
	second := renderedDiagram(t, "c", graphRendering(view.KindState), view.DirectionLeftRight)
	if first != second {
		t.Fatalf("renderings differ:\n%s\n---\n%s", first, second)
	}
}
