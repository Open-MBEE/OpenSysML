package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRenderStyle checks -render-style on every path that writes a DOT
// figure: the Pilot look by default and the Cameo frame under `cameo`, the
// same on -render, -render-all, -render-document and -render-documents, with
// the pinned layout kept; a style there is none of and a style with nothing
// to render are refused, and a form that draws no style says so.
func TestRenderStyle(t *testing.T) {
	binary := buildCLI(t)
	fixture := filepath.Join("..", "..", "internal", "doc", "docrender", "testdata", "placed_report.sysml")
	const frame = `subgraph "cluster_frame"`
	check := func(t *testing.T, path, got string, cameo bool) {
		t.Helper()
		if strings.Contains(got, frame) != cameo || strings.Contains(got, `fontname="Arial"`) != cameo {
			t.Errorf("%s: Cameo look drawn = %v, want %v:\n%s", path, !cameo, cameo, got)
		}
		if !strings.Contains(got, "// layout: neato -n\n") {
			t.Errorf("%s: the pinned layout is not kept:\n%s", path, got)
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

	for _, cameo := range []bool{false, true} {
		var style []string
		if cameo {
			style = []string{"-render-style", "cameo"}
		}
		one := runFiles(t, binary, []string{fixture}, append([]string{"-render", "Placed::placedView", "-render-form", "dot"}, style...)...)
		if one.status != exitHolds {
			t.Fatalf("-render exit status = %d, want %d\n%s", one.status, exitHolds, one.output())
		}
		check(t, "-render", one.stdout, cameo)

		dir := filepath.Join(t.TempDir(), "rendered")
		all := runFiles(t, binary, []string{fixture}, append([]string{"-render-all", dir, "-render-form", "dot"}, style...)...)
		if all.status != exitHolds {
			t.Fatalf("-render-all exit status = %d, want %d\n%s", all.status, exitHolds, all.output())
		}
		check(t, "-render-all", readFile(t, filepath.Join(dir, "Placed.placedView.dot")), cameo)

		doc := runFiles(t, binary, []string{fixture}, append([]string{"-render-document", "Placed::PlacedReport", "-diagram-form", "dot"}, style...)...)
		if doc.status != exitHolds {
			t.Fatalf("-render-document exit status = %d, want %d\n%s", doc.status, exitHolds, doc.output())
		}
		check(t, "-render-document", doc.stdout, cameo)

		page := runFiles(t, binary, []string{fixture}, append([]string{"-render-document", "Placed::PlacedReport", "-doc-form", "html", "-diagram-form", "dot"}, style...)...)
		if page.status != exitHolds || strings.Contains(page.stdout, `data-style="cameo"`) != cameo {
			t.Errorf("-doc-form html exit status = %d, states the style = %v\n%s", page.status, !cameo, page.output())
		}

		site := filepath.Join(t.TempDir(), "docs")
		docs := runFiles(t, binary, []string{fixture}, append([]string{"-render-documents", site, "-diagram-form", "dot"}, style...)...)
		if docs.status != exitHolds {
			t.Fatalf("-render-documents exit status = %d, want %d\n%s", docs.status, exitHolds, docs.output())
		}
		pages, err := filepath.Glob(filepath.Join(site, "*.md"))
		if err != nil || len(pages) != 1 {
			t.Fatalf("-render-documents wrote %v, want one page: %v", pages, err)
		}
		check(t, "-render-documents", readFile(t, pages[0]), cameo)
	}

	mermaid := runFiles(t, binary, []string{fixture}, "-render", "Placed::placedView", "-render-form", "mermaid", "-render-style", "cameo")
	if mermaid.status != exitHolds || !strings.Contains(mermaid.stdout, "style cameo; only the DOT form draws a diagram in a style") {
		t.Errorf("Mermaid under -render-style = %d\n%s", mermaid.status, mermaid.output())
	}

	unknown := runFiles(t, binary, []string{fixture}, "-render", "Placed::placedView", "-render-form", "dot", "-render-style", "magicdraw")
	if unknown.status != exitUnevaluable || !strings.Contains(unknown.stderr, `-render-style: unknown drawing style "magicdraw"; the styles are pilot, cameo`) {
		t.Errorf("an unknown style = %d\n%s", unknown.status, unknown.output())
	}
	if unknown.stdout != "" {
		t.Errorf("an unknown style wrote an artifact:\n%s", unknown.stdout)
	}
	unknownDoc := runFiles(t, binary, []string{fixture}, "-render-document", "Placed::PlacedReport", "-diagram-form", "dot", "-render-style", "magicdraw")
	if unknownDoc.status != exitUnevaluable || !strings.Contains(unknownDoc.stderr, `-render-style: unknown drawing style "magicdraw"`) {
		t.Errorf("an unknown style on a document = %d\n%s", unknownDoc.status, unknownDoc.output())
	}

	alone := runFiles(t, binary, []string{fixture}, "-render-style", "cameo")
	if alone.status != 2 || !strings.Contains(alone.stderr, "-render-style is the drawing style of a DOT drawing") {
		t.Errorf("a style without a view = %d\n%s", alone.status, alone.output())
	}
}
