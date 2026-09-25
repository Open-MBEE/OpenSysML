package docrender

import (
	"html"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

// TestDocumentDiagramsTakeADrawingStyle checks the drawing style reaches every
// document backend's DOT figures: the Markdown fence, the HTML figure and the
// artwork source a PDF draws keep the Pilot look by default and draw the Cameo
// frame under `cameo`, the pinned layout kept; a style there is none of is
// refused, and the other forms are not changed by one.
func TestDocumentDiagramsTakeADrawingStyle(t *testing.T) {
	document := fixtureDocument(t, filepath.Join("testdata", "placed_report.sysml"), "Placed::PlacedReport")
	const frame = `subgraph "cluster_frame"`
	check := func(t *testing.T, backend, got string, style view.DrawingStyle) {
		t.Helper()
		cameo := style == view.StyleCameo
		if strings.Contains(got, frame) != cameo {
			t.Errorf("%s (%q): Cameo frame drawn = %v, want %v:\n%s", backend, style, !cameo, cameo, got)
		}
		if strings.Contains(got, `fontname="Arial"`) != cameo {
			t.Errorf("%s (%q): Arial set = %v, want %v:\n%s", backend, style, !cameo, cameo, got)
		}
		if !strings.Contains(got, "// layout: neato -n\n") {
			t.Errorf("%s (%q): every drawn node is pinned, so the header names neato -n:\n%s", backend, style, got)
		}
	}
	for _, style := range []view.DrawingStyle{"", view.StylePilot, view.StyleCameo} {
		markdown, err := Markdown(document, MarkdownOptions{DiagramForm: view.FormDot, Style: style})
		if err != nil {
			t.Fatalf("Markdown(%q): %v", style, err)
		}
		check(t, "markdown", markdown, style)
		page, err := HTML(document, HTMLOptions{DiagramForm: view.FormDot, Style: style})
		if err != nil {
			t.Fatalf("HTML(%q): %v", style, err)
		}
		check(t, "html", html.UnescapeString(page), style)
		if want := `data-style="` + string(style) + `"`; strings.Contains(page, want) != (style != "") {
			t.Errorf("HTML(%q) figure states %s = %v, want %v:\n%s", style, want, style == "", style != "", page)
		}
		diagrams, err := Diagrams(document, DiagramOptions{Form: view.FormDot, Style: style})
		if err != nil {
			t.Fatalf("Diagrams(%q): %v", style, err)
		}
		if len(diagrams) != 1 {
			t.Fatalf("Diagrams(%q) = %+v, want the one figure", style, diagrams)
		}
		check(t, "artwork", diagrams[0].Source+"\n", style)
	}

	plain, err := Markdown(document, MarkdownOptions{DiagramForm: view.FormMermaid})
	if err != nil {
		t.Fatal(err)
	}
	styled, err := Markdown(document, MarkdownOptions{DiagramForm: view.FormMermaid, Style: view.StyleCameo})
	if err != nil {
		t.Fatal(err)
	}
	if styled == plain || !strings.Contains(styled, "style cameo; only the DOT form draws a diagram in a style") {
		t.Errorf("a Mermaid figure under a style says it draws none:\n%s", styled)
	}

	for _, backend := range []string{"markdown", "html", "artwork"} {
		var err error
		switch backend {
		case "markdown":
			_, err = Markdown(document, MarkdownOptions{DiagramForm: view.FormDot, Style: "magicdraw"})
		case "html":
			_, err = HTML(document, HTMLOptions{DiagramForm: view.FormDot, Style: "magicdraw"})
		default:
			_, err = Diagrams(document, DiagramOptions{Form: view.FormDot, Style: "magicdraw"})
		}
		if err == nil || !strings.Contains(err.Error(), `unknown drawing style "magicdraw"`) {
			t.Errorf("%s: an unknown drawing style is not refused: %v", backend, err)
		}
	}
}
