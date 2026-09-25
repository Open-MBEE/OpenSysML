package docrender

import (
	"html"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

// TestDocumentDiagramsSettleUnplacedNodes checks the unplaced-node placement
// reaches every document backend's DOT figures: the Markdown fence, the HTML
// figure and the artwork source a PDF draws leave a node no Layout places
// undrawn by default and set it in a strip below the drawing when asked.
func TestDocumentDiagramsSettleUnplacedNodes(t *testing.T) {
	document := fixtureDocument(t, filepath.Join("testdata", "placed_report.sysml"), "Placed::PlacedReport")
	const (
		omitted = "// not represented: 1 node(s) without a position, left undrawn\n"
		striped = "// not represented: 1 node(s) without a position, drawn in a strip below the drawing\n"
		spare   = "<b>spare : Recorder</b>"
	)
	check := func(t *testing.T, backend, got string, unplaced view.Unplaced) {
		t.Helper()
		notice, drawn := omitted, false
		if unplaced == view.UnplacedStrip {
			notice, drawn = striped, true
		}
		if !strings.Contains(got, notice) {
			t.Errorf("%s (%q): notice %q missing:\n%s", backend, unplaced, notice, got)
		}
		if strings.Contains(got, spare) != drawn {
			t.Errorf("%s (%q): spare drawn = %v, want %v:\n%s", backend, unplaced, !drawn, drawn, got)
		}
		if !strings.Contains(got, "// layout: neato -n\n") {
			t.Errorf("%s (%q): every drawn node is pinned, so the header names neato -n:\n%s", backend, unplaced, got)
		}
	}
	for _, unplaced := range []view.Unplaced{"", view.UnplacedOmit, view.UnplacedStrip} {
		markdown, err := Markdown(document, MarkdownOptions{DiagramForm: view.FormDot, Unplaced: unplaced})
		if err != nil {
			t.Fatalf("Markdown(%q): %v", unplaced, err)
		}
		check(t, "markdown", markdown, unplaced)
		page, err := HTML(document, HTMLOptions{DiagramForm: view.FormDot, Unplaced: unplaced})
		if err != nil {
			t.Fatalf("HTML(%q): %v", unplaced, err)
		}
		check(t, "html", html.UnescapeString(page), unplaced)
		diagrams, err := Diagrams(document, view.FormDot, unplaced, "")
		if err != nil {
			t.Fatalf("Diagrams(%q): %v", unplaced, err)
		}
		if len(diagrams) != 1 {
			t.Fatalf("Diagrams(%q) = %+v, want the one figure", unplaced, diagrams)
		}
		check(t, "artwork", diagrams[0].Source+"\n", unplaced)
	}
	for _, backend := range []string{"markdown", "html", "artwork"} {
		var err error
		switch backend {
		case "markdown":
			_, err = Markdown(document, MarkdownOptions{DiagramForm: view.FormDot, Unplaced: "below"})
		case "html":
			_, err = HTML(document, HTMLOptions{DiagramForm: view.FormDot, Unplaced: "below"})
		default:
			_, err = Diagrams(document, view.FormDot, "below", "")
		}
		if err == nil || !strings.Contains(err.Error(), `unknown placement "below"`) {
			t.Errorf("%s: an unknown placement is not refused: %v", backend, err)
		}
	}
}
