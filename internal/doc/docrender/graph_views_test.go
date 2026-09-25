package docrender

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

// fakeDrawer is a DiagramDrawer that records what it is asked to draw and
// answers with a stub SVG per DOT diagram.
type fakeDrawer struct {
	available bool
	drawn     []Diagram
	err       error
}

func (d *fakeDrawer) Available() bool { return d.available }

func (d *fakeDrawer) Draw(diagrams []Diagram) ([]string, error) {
	d.drawn = diagrams
	if d.err != nil {
		return nil, d.err
	}
	svg := make([]string, len(diagrams))
	for i, diagram := range diagrams {
		if diagram.Form == view.FormDot {
			svg[i] = `<?xml version="1.0"?>` + "\n" + `<svg xmlns="http://www.w3.org/2000/svg"><title>` + diagram.Name + "</title></svg>\n"
		}
	}
	return svg, nil
}

// A Diagram block whose source is a typed StateTransitionView or ActionFlowView
// draws the graph of the behavior the view exposes in every backend. Both views
// are positioned, so the automatic choice writes DOT: drawn to inline SVG with
// Graphviz at hand, fenced as source with none to draw it.
func TestGraphFormViewsDrawn(t *testing.T) {
	path := filepath.Join("testdata", "graph_views.sysml")
	document := fixtureDocument(t, path, "Pump::Handbook")
	md, err := Markdown(document, MarkdownOptions{})
	if err != nil {
		t.Fatalf("render Markdown: %v", err)
	}
	for _, want := range []string{
		"*Pump Modes*", "// kind: state", "Idle", "Running", "accept Start",
		"*Priming*", "// kind: action", "Fill", "Vent", "// layout: neato -n2",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("Markdown lacks %q:\n%s", want, md)
		}
	}
	if n := strings.Count(md, "```dot"); n != 2 || strings.Contains(md, "```mermaid") || strings.Contains(md, "<!--") {
		t.Errorf("Markdown writes %d DOT figures, want 2 and no Mermaid or notice:\n%s", n, md)
	}
	if strings.Contains(md, "exposes nothing") {
		t.Errorf("a graph-form figure is empty:\n%s", md)
	}
	page, err := HTML(document, HTMLOptions{Fragment: true})
	if err != nil {
		t.Fatalf("render HTML: %v", err)
	}
	if n := strings.Count(page, `<pre class="dot">`); n != 2 || strings.Contains(page, "mermaid") {
		t.Errorf("HTML writes %d DOT figures, want 2 and no Mermaid:\n%s", n, page)
	}

	drawer := &fakeDrawer{available: true}
	md, err = Markdown(document, MarkdownOptions{Drawer: drawer})
	if err != nil {
		t.Fatalf("render Markdown with Graphviz: %v", err)
	}
	if len(drawer.drawn) != 2 || drawer.drawn[0].Form != view.FormDot || drawer.drawn[1].Form != view.FormDot {
		t.Fatalf("drawer asked for %+v, want both DOT diagrams", drawer.drawn)
	}
	if strings.Count(md, "<figure class=\"sysml-diagram\">\n<svg ") != 2 || strings.Contains(md, "```") || strings.Contains(md, "<?xml") {
		t.Errorf("Markdown with Graphviz does not inline both SVGs:\n%s", md)
	}
	page, err = HTML(document, HTMLOptions{Fragment: true, Drawer: &fakeDrawer{available: true}})
	if err != nil {
		t.Fatalf("render HTML with Graphviz: %v", err)
	}
	if strings.Count(page, "<title>") != 2 || strings.Contains(page, "<pre") || strings.Contains(page, "<?xml") {
		t.Errorf("HTML with Graphviz does not inline both SVGs:\n%s", page)
	}
	if _, err := Markdown(document, MarkdownOptions{Drawer: &fakeDrawer{available: true, err: errors.New("dot: exit 1")}}); err == nil || !strings.Contains(err.Error(), "dot: exit 1") {
		t.Errorf("a failed drawing is not the render's error: %v", err)
	}
}

// Without Graphviz the automatic choice falls back to Mermaid for a positioned
// view, and both backends say so where the figure stands, while a form asked
// for explicitly is written whatever the drawer.
func TestGraphFormViewsFallBackWithoutGraphviz(t *testing.T) {
	path := filepath.Join("testdata", "graph_views.sysml")
	document := fixtureDocument(t, path, "Pump::Handbook")
	const notice = "drawn as Mermaid, not at its stated positions: Graphviz (dot) is not installed"
	md, err := Markdown(document, MarkdownOptions{Drawer: &fakeDrawer{}})
	if err != nil {
		t.Fatalf("render Markdown: %v", err)
	}
	if strings.Count(md, "```mermaid") != 2 || strings.Count(md, "*"+notice+"*") != 2 || !strings.Contains(md, "stateDiagram-v2") || !strings.Contains(md, "flowchart TD") {
		t.Errorf("Markdown without Graphviz does not fall back visibly:\n%s", md)
	}
	page, err := HTML(document, HTMLOptions{Fragment: true, Drawer: &fakeDrawer{}})
	if err != nil {
		t.Fatalf("render HTML: %v", err)
	}
	if strings.Count(page, `<pre class="mermaid">`) != 2 || strings.Count(page, `<p class="sysml-diagram-notice"><em>`+notice+"</em></p>") != 2 {
		t.Errorf("HTML without Graphviz does not fall back visibly:\n%s", page)
	}
	drawer := &fakeDrawer{available: true}
	md, err = Markdown(document, MarkdownOptions{DiagramForm: view.FormMermaid, Drawer: drawer})
	if err != nil {
		t.Fatalf("render Mermaid Markdown: %v", err)
	}
	if drawer.drawn != nil || strings.Count(md, "```mermaid") != 2 || strings.Contains(md, "<!--") {
		t.Errorf("an explicit form is not written as asked:\n%s", md)
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
