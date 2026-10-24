package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const wholeBodiesFixture = "../../internal/doc/docrender/testdata/whole_report.sysml"

// TestRenderDocumentWholeBodies renders a report whose outline is one
// Markdown body: its headings are sections the title page, table of contents
// and numbering options treat exactly as the part-born section after them.
func TestRenderDocumentWholeBodies(t *testing.T) {
	binary := buildCLI(t)
	source, err := os.ReadFile(wholeBodiesFixture)
	if err != nil {
		t.Fatal(err)
	}
	model := string(source)

	wantReport(t, check(t, binary, model, "-render-document", "Whole::WholeReport"), 0,
		"## Overview", "### Mass budget", "#### Margins", "## Appendix", "### Derivation",
		"[Mass budget](#overview-mass_budget)", "[Shared notes](Whole-Notes.md#shared_notes)", "back to [Overview](#overview)")

	html := filepath.Join(t.TempDir(), "report.html")
	wantReport(t, check(t, binary, model, "-render-document", "Whole::WholeReport", "-doc-form", "html",
		"-doc-title-page", "-doc-toc", "-doc-number-sections", "-o", html), 0)
	page, err := os.ReadFile(html)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<nav class="sysml-toc"`,
		`<li><a href="#overview-mass_budget_2-margins">`,
		`<h2><span class="sysml-section-number">1</span> Overview</h2>`,
		`<h3><span class="sysml-section-number">1.2</span> Mass budget</h3>`,
		`<h4><span class="sysml-section-number">1.2.1</span> Margins</h4>`,
		`<h2><span class="sysml-section-number">3</span> Appendix</h2>`,
		`<h3><span class="sysml-section-number">3.1</span> Derivation</h3>`,
	} {
		if !strings.Contains(string(page), want) {
			t.Errorf("HTML lacks %q", want)
		}
	}

	deep := strings.Replace(model, "### Margins", "##### Margins", 1)
	wantReport(t, check(t, binary, deep, "-render-document", "Whole::WholeReport"), 2,
		"markup-heading-level", `level 5 heading "Margins" is out of sequence: expected a level between 1 and 3 (body line 18)`)
}
