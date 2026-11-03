package docrender

import (
	"path/filepath"
	"testing"
)

// TestMarkdownWholeBodyReportGolden locks the Markdown of a report whose
// outline is one Markdown body: headings nested by level among the parts.
func TestMarkdownWholeBodyReportGolden(t *testing.T) {
	got := renderFixtureDocument(t, filepath.Join("testdata", "whole_report.sysml"), "Whole::WholeReport")
	checkGolden(t, got, filepath.Join("testdata", "whole_report.golden.md"))
}

// TestHTMLWholeBodyReportGolden locks the numbered HTML of the same report,
// title page and table of contents included: markup-born and part-born
// sections number and list alike.
func TestHTMLWholeBodyReportGolden(t *testing.T) {
	got := renderFixtureHTML(t, filepath.Join("testdata", "whole_report.sysml"), "Whole::WholeReport",
		HTMLOptions{Fragment: true, TitlePage: true, TOC: true, NumberSections: true})
	checkGolden(t, got, filepath.Join("testdata", "whole_report.numbered.golden.html"))
}
