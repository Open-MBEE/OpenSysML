package docrender

import (
	"path/filepath"
	"testing"
)

// TestValuesReportGolden locks the Markdown of a report whose prose reads
// model values: a quantity with its unit, a string, a boolean, an
// enumeration literal and a multi-valued attribute, labelled and bare.
func TestValuesReportGolden(t *testing.T) {
	got := renderFixtureDocument(t, filepath.Join("testdata", "values_report.sysml"), "Values::ValuesReport")
	checkGolden(t, got, filepath.Join("testdata", "values_report.golden.md"))
}

// TestHTMLValuesReportGolden locks the HTML fragment of the same report.
func TestHTMLValuesReportGolden(t *testing.T) {
	got := renderFixtureHTML(t, filepath.Join("testdata", "values_report.sysml"), "Values::ValuesReport",
		HTMLOptions{Fragment: true})
	checkGolden(t, got, filepath.Join("testdata", "values_report.fragment.golden.html"))
}
