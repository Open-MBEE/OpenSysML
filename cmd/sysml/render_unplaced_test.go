package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRenderUnplaced checks -render-unplaced on every path that draws a DOT
// figure: a positioned drawing leaves the nodes no Layout places undrawn by
// default and sets them in a strip below it under `strip`, the same on
// -render, -render-all, -render-document and -render-documents; an unknown
// placement and a placement with nothing to render are refused.
func TestRenderUnplaced(t *testing.T) {
	binary := buildCLI(t)
	fixture := filepath.Join("..", "..", "internal", "doc", "docrender", "testdata", "placed_report.sysml")
	const (
		omitted = "// not represented: 1 node(s) without a position, left undrawn\n"
		striped = "// not represented: 1 node(s) without a position, drawn in a strip below the drawing\n"
		spare   = "<b>spare : Recorder</b>"
	)
	check := func(t *testing.T, path, got string, strip bool) {
		t.Helper()
		notice := omitted
		if strip {
			notice = striped
		}
		if !strings.Contains(got, notice) {
			t.Errorf("%s: notice %q missing:\n%s", path, notice, got)
		}
		if strings.Contains(got, spare) != strip {
			t.Errorf("%s: spare drawn = %v, want %v:\n%s", path, !strip, strip, got)
		}
		if strings.Contains(got, "neato -n\n") && strings.Contains(got, `pos="82,-42.5!"`) != strip {
			t.Errorf("%s: the strip pins spare below the canvas only when asked:\n%s", path, got)
		}
	}
	readFile := func(t *testing.T, path string) string {
		t.Helper()
		content, err := os.ReadFile(path) // #nosec G304 -- the test chose this path.
		if err != nil {
			t.Fatal(err)
		}
		return string(content)
	}

	for _, strip := range []bool{false, true} {
		var placement []string
		if strip {
			placement = []string{"-render-unplaced", "strip"}
		}
		one := runFiles(t, binary, []string{fixture}, append([]string{"-render", "Placed::placedView", "-render-form", "dot"}, placement...)...)
		if one.status != exitHolds {
			t.Fatalf("-render exit status = %d, want %d\n%s", one.status, exitHolds, one.output())
		}
		check(t, "-render", one.stdout, strip)

		dir := filepath.Join(t.TempDir(), "rendered")
		all := runFiles(t, binary, []string{fixture}, append([]string{"-render-all", dir, "-render-form", "dot"}, placement...)...)
		if all.status != exitHolds {
			t.Fatalf("-render-all exit status = %d, want %d\n%s", all.status, exitHolds, all.output())
		}
		check(t, "-render-all", readFile(t, filepath.Join(dir, "Placed.placedView.dot")), strip)

		doc := runFiles(t, binary, []string{fixture}, append([]string{"-render-document", "Placed::PlacedReport", "-diagram-form", "dot"}, placement...)...)
		if doc.status != exitHolds {
			t.Fatalf("-render-document exit status = %d, want %d\n%s", doc.status, exitHolds, doc.output())
		}
		check(t, "-render-document", doc.stdout, strip)

		site := filepath.Join(t.TempDir(), "docs")
		docs := runFiles(t, binary, []string{fixture}, append([]string{"-render-documents", site, "-diagram-form", "dot"}, placement...)...)
		if docs.status != exitHolds {
			t.Fatalf("-render-documents exit status = %d, want %d\n%s", docs.status, exitHolds, docs.output())
		}
		pages, err := filepath.Glob(filepath.Join(site, "*.md"))
		if err != nil || len(pages) != 1 {
			t.Fatalf("-render-documents wrote %v, want one page: %v", pages, err)
		}
		check(t, "-render-documents", readFile(t, pages[0]), strip)
	}

	mermaid := runFiles(t, binary, []string{fixture}, "-render", "Placed::placedView", "-render-form", "mermaid", "-render-unplaced", "strip")
	if mermaid.status != exitHolds || strings.Contains(mermaid.stdout, "pos=") {
		t.Errorf("Mermaid under -render-unplaced = %d\n%s", mermaid.status, mermaid.output())
	}

	unknown := runFiles(t, binary, []string{fixture}, "-render", "Placed::placedView", "-render-form", "dot", "-render-unplaced", "below")
	if unknown.status != exitUnevaluable || !strings.Contains(unknown.stderr, `-render-unplaced: unknown placement "below" of unplaced nodes; the placements are omit, strip`) {
		t.Errorf("an unknown placement = %d\n%s", unknown.status, unknown.output())
	}
	if unknown.stdout != "" {
		t.Errorf("an unknown placement wrote an artifact:\n%s", unknown.stdout)
	}

	alone := runFiles(t, binary, []string{fixture}, "-render-unplaced", "strip")
	if alone.status != 2 || !strings.Contains(alone.stderr, "-render-unplaced places the unplaced nodes of a positioned DOT drawing") {
		t.Errorf("a placement without a view = %d\n%s", alone.status, alone.output())
	}
}
