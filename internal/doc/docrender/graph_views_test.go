package docrender

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

// A Diagram block whose source is a typed StateTransitionView or ActionFlowView
// draws the graph of the behavior the view exposes, in every form and backend.
func TestGraphFormViewsDrawn(t *testing.T) {
	path := filepath.Join("testdata", "graph_views.sysml")
	document := fixtureDocument(t, path, "Pump::Handbook")
	md, err := Markdown(document, MarkdownOptions{})
	if err != nil {
		t.Fatalf("render Markdown: %v", err)
	}
	for _, want := range []string{
		"*Pump Modes*", "stateDiagram-v2", "Idle", "Running", "accept Start",
		"*Priming*", "flowchart TD", "Fill", "Vent",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("Markdown lacks %q:\n%s", want, md)
		}
	}
	if n := strings.Count(md, "```mermaid"); n != 2 {
		t.Errorf("Markdown draws %d figures, want 2:\n%s", n, md)
	}
	if strings.Contains(md, "exposes nothing") {
		t.Errorf("a graph-form figure is empty:\n%s", md)
	}
	page, err := HTML(document, HTMLOptions{Fragment: true})
	if err != nil {
		t.Fatalf("render HTML: %v", err)
	}
	if n := strings.Count(page, `<pre class="mermaid">`); n != 2 {
		t.Errorf("HTML draws %d figures, want 2:\n%s", n, page)
	}
}

// The DOT form pins each node where the view's DiagramLayout put it and routes
// each edge along its recorded points.
func TestGraphFormViewsHonourLayout(t *testing.T) {
	path := filepath.Join("testdata", "graph_views.sysml")
	dot, err := Markdown(fixtureDocument(t, path, "Pump::Handbook"), MarkdownOptions{DiagramForm: view.FormDot})
	if err != nil {
		t.Fatalf("render DOT Markdown: %v", err)
	}
	if n := strings.Count(dot, "```dot"); n != 2 {
		t.Errorf("DOT Markdown draws %d figures, want 2:\n%s", n, dot)
	}
	for _, want := range []string{
		"// kind: state", "// kind: action", "// layout: neato",
		`pos="70,60!", pin=true`, `pos="230,60!", pin=true`,
		`pos="70,140!", pin=true`,
		`[label="accept Start", pos="e,180,60 120,60 120,60 170,60 170,60", lp="150,72"];`,
		`[label="'Fill to Vent'", pos="e,70,80 70,120 70,120 70,90 70,90", lp="128.5,100"];`,
	} {
		if !strings.Contains(dot, want) {
			t.Errorf("DOT lacks %q:\n%s", want, dot)
		}
	}
}
