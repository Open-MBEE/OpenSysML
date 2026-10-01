package docrender

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
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
	w := &markdownWriter{opts: DiagramOptions{Form: form}}
	blocks, err := w.diagramFigure("d", caption, rendering, view.Options{Direction: direction})
	if err != nil {
		t.Fatalf("diagramFigure: %v", err)
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
		for _, want := range []string{"digraph {", "graph [fontname=\"Helvetica\", rankdir=LR];", `"n0" -> "n1" [arrowhead=none, penwidth=3];`} {
			if !strings.Contains(got, want) {
				t.Errorf("%s: missing %q:\n%s", kind, want, got)
			}
		}
		if strings.Contains(got, "flowchart") || strings.Contains(got, "stateDiagram") {
			t.Errorf("%s: Mermaid in a dot fence:\n%s", kind, got)
		}
	}
	got := renderedDiagramForm(t, "Chain", graphRendering(view.KindTree), "", view.FormDot)
	if !strings.HasPrefix(got, "*Chain*\n\n```dot\n") {
		t.Errorf("captioned dot: %s", got)
	}
}

// A render in the PlantUML form writes each graph-shaped diagram, the sequence
// included, as a plantuml fence in the diagram's direction.
func TestDiagramPlantUMLForm(t *testing.T) {
	for _, kind := range []view.Kind{view.KindTree, view.KindInterconnection, view.KindAction, view.KindState} {
		got := renderedDiagramForm(t, "", graphRendering(kind), view.DirectionLeftRight, view.FormPlantUML)
		if !strings.HasPrefix(got, "```plantuml\n@startuml\n' "+string(kind)+" rendering") || !strings.HasSuffix(got, "\n@enduml\n```") {
			t.Errorf("%s: not a plantuml fence:\n%s", kind, got)
		}
		for _, want := range []string{"<style>\n", "</style>\n", "left to right direction\n", ` as n0 <<part>> <<usage>>`} {
			if !strings.Contains(got, want) {
				t.Errorf("%s: missing %q:\n%s", kind, want, got)
			}
		}
		if strings.Contains(got, "flowchart") || strings.Contains(got, "digraph") {
			t.Errorf("%s: another form in a plantuml fence:\n%s", kind, got)
		}
	}
	sequence := renderedDiagramForm(t, "", graphRendering(view.KindSequence), "", view.FormPlantUML)
	if !strings.Contains(sequence, `participant "**a**\n<size:10>//«part»//</size>" as n0`) || !strings.Contains(sequence, "n0 -> n1\n") {
		t.Errorf("sequence as plantuml:\n%s", sequence)
	}
	got := renderedDiagramForm(t, "Chain", graphRendering(view.KindTree), "", view.FormPlantUML)
	if !strings.HasPrefix(got, "*Chain*\n\n```plantuml\n") {
		t.Errorf("captioned plantuml: %s", got)
	}
}

// The diagram form is checked once per render: a form no diagram is written
// as is a typed error before anything is written, while empty picks per
// diagram — Mermaid for one nothing positions.
func TestDiagramFormResolution(t *testing.T) {
	if form, fallback := (DiagramOptions{}).formFor(graphRendering(view.KindTree)); form != view.FormMermaid || fallback != "" {
		t.Fatalf("automatic form of an unpositioned view = %q, %q", form, fallback)
	}
	for _, form := range view.DiagramForms() {
		if err := (DiagramOptions{Form: form}).check(); err != nil {
			t.Fatalf("check(%s): %v", form, err)
		}
		if got, _ := (DiagramOptions{Form: form}).formFor(graphRendering(view.KindTree)); got != form {
			t.Fatalf("formFor(%s) = %q", form, got)
		}
	}
	var typed *Error
	for _, form := range []view.Form{view.FormText, view.FormMarkdown, "svg"} {
		err := (DiagramOptions{Form: form}).check()
		if !errors.As(err, &typed) || typed.Kind != ErrorUnknownForm || typed.DiagramForm != form {
			t.Fatalf("%s: error = %v", form, err)
		}
		if !strings.Contains(err.Error(), `"`+string(form)+`"`) || !strings.Contains(err.Error(), "mermaid, dot, plantuml") {
			t.Errorf("%s: message = %q", form, err)
		}
	}
}

// A kind the chosen form does not write is a typed error naming both, and a
// table-kind view is a pipe table whichever form is chosen.
func TestDiagramFormErrors(t *testing.T) {
	var typed *Error
	_, err := (&markdownWriter{opts: DiagramOptions{Form: view.FormDot}}).diagramFigure("d", "", graphRendering(view.KindSequence), view.Options{})
	if !errors.As(err, &typed) || typed.Kind != ErrorUnrenderableForm || typed.Actual != "sequence" || typed.DiagramForm != view.FormDot {
		t.Fatalf("sequence as dot: error = %v", err)
	}
	if !strings.Contains(err.Error(), `kind "sequence"`) || !strings.Contains(err.Error(), "not written as dot") {
		t.Errorf("message = %q", err)
	}
	if _, err := (&markdownWriter{opts: DiagramOptions{Form: view.FormPlantUML}}).diagramFigure("d", "", graphRendering(view.KindSequence), view.Options{}); err != nil {
		t.Fatalf("sequence as plantuml: %v", err)
	}
	table := &view.Rendering{Kind: view.KindTable, Columns: []string{"a"}, Rows: [][]string{{"x"}}}
	for _, form := range []view.Form{view.FormDot, view.FormPlantUML} {
		if got := renderedDiagramForm(t, "", table, "", form); !strings.Contains(got, "| a |") || !strings.Contains(got, "| x |") {
			t.Errorf("a table is not a table in the %s form:\n%s", form, got)
		}
	}
}

func TestDiagramCaption(t *testing.T) {
	got := renderedDiagram(t, "flow of a|b", graphRendering(view.KindTree), "")
	if !strings.HasPrefix(got, "*flow of a\\|b*\n\n```mermaid") {
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
	want := "*Masses*\n\n<!-- table rendering -->\n| name | mass |\n| --- | --- |\n| optics | 8.5 |\n| mount\\|base | 15 |"
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
	_, err := (&markdownWriter{opts: DiagramOptions{Form: view.FormMermaid}}).diagramFigure("d", "", nil, view.Options{})
	var typed *Error
	if !errors.As(err, &typed) || typed.Kind != ErrorMissingRendering {
		t.Fatalf("error = %v, want %s", err, ErrorMissingRendering)
	}
}

func TestDiagramUnrenderableKind(t *testing.T) {
	for _, kind := range []view.Kind{view.KindTextual, view.KindGeometry} {
		_, err := (&markdownWriter{opts: DiagramOptions{Form: view.FormMermaid}}).diagramFigure("d", "", &view.Rendering{Kind: kind}, view.Options{})
		var typed *Error
		if !errors.As(err, &typed) || typed.Kind != ErrorUnrenderableDiagram {
			t.Fatalf("%s: error = %v, want %s", kind, err, ErrorUnrenderableDiagram)
		}
		if typed.Actual != string(kind) {
			t.Fatalf("%s: actual = %q", kind, typed.Actual)
		}
	}
}

// TestDiagramOversized checks a Mermaid chart past the ceiling a chart is drawn
// under is refused by the Markdown and HTML backends, with its size and the
// ceiling, while the same diagram is written in another form; a chart of as
// many nodes, and one edge, is drawn.
func TestDiagramOversized(t *testing.T) {
	crowded := graphRendering(view.KindInterconnection)
	for i := 0; i < view.MermaidEdgeCeiling; i++ {
		crowded.Roots = append(crowded.Roots, &view.Node{ID: fmt.Sprintf("m%d", i), Kind: "part", Name: "m"})
	}
	if got := renderedDiagram(t, "", crowded, ""); strings.Count(got, "\n") <= view.MermaidEdgeCeiling {
		t.Errorf("a chart of %d nodes and one edge is not drawn:\n%.200s", view.MermaidEdgeCeiling, got)
	}
	rendering := graphRendering(view.KindTree)
	for i := 0; i < view.MermaidEdgeCeiling; i++ {
		rendering.Edges = append(rendering.Edges, view.Edge{From: "n0", To: "n1"})
	}
	var typed *Error
	_, err := (&markdownWriter{opts: DiagramOptions{Form: view.FormMermaid}}).diagramFigure("Wide", "", rendering, view.Options{})
	if !errors.As(err, &typed) || typed.Kind != ErrorOversizedDiagram {
		t.Fatalf("Markdown: error = %v, want %s", err, ErrorOversizedDiagram)
	}
	if typed.Content != "Wide" || typed.Edges <= view.MermaidEdgeCeiling || typed.TextSize <= 0 {
		t.Errorf("error sizes the chart as %d characters and %d edges", typed.TextSize, typed.Edges)
	}
	for _, want := range []string{"diagram Wide is ", " edges of mermaid, past the 1000000 characters and 10000 edges", "another diagram form"} {
		if !strings.Contains(typed.Error(), want) {
			t.Errorf("message lacks %q: %s", want, typed.Error())
		}
	}
	w := &htmlWriter{forms: DiagramOptions{Form: view.FormMermaid}}
	if err := w.writeFigure("", "Wide", caption{}, rendering, view.Options{}); !errors.As(err, &typed) || typed.Kind != ErrorOversizedDiagram {
		t.Fatalf("HTML: error = %v, want %s", err, ErrorOversizedDiagram)
	}
	if got := renderedDiagramForm(t, "", rendering, "", view.FormDot); !strings.HasPrefix(got, "```dot\n") {
		t.Errorf("the oversized diagram is not written as dot:\n%.80s", got)
	}
}

func TestDiagramDeterminism(t *testing.T) {
	first := renderedDiagram(t, "c", graphRendering(view.KindState), view.DirectionLeftRight)
	second := renderedDiagram(t, "c", graphRendering(view.KindState), view.DirectionLeftRight)
	if first != second {
		t.Fatalf("renderings differ:\n%s\n---\n%s", first, second)
	}
}
