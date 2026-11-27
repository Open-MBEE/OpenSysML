package docrender

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestConnectorEndCellsRenderTheirLabels checks that a cell holding connector
// ends prints each end's label in Markdown and in HTML, not an empty cell.
func TestConnectorEndCellsRenderTheirLabels(t *testing.T) {
	fixture := filepath.Join("testdata", "connector_end_report.sysml")
	markdown := renderFixtureDocument(t, fixture, "Bench::ConnectionReport")
	html := renderFixtureHTML(t, fixture, "Bench::ConnectionReport", HTMLOptions{Fragment: true})
	for _, label := range []string{"camera.usb", "hub.socket"} {
		if !strings.Contains(markdown, label) {
			t.Errorf("Markdown lacks the connector end %q:\n%s", label, markdown)
		}
		if !strings.Contains(html, label) {
			t.Errorf("HTML lacks the connector end %q:\n%s", label, html)
		}
	}
}
