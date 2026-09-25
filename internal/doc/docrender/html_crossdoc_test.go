package docrender

import (
	"path/filepath"
	"strings"
	"testing"
)

// renderFixtureHTMLSet renders every named document of a fixture as HTML,
// keyed by the file name a rendered set writes it to.
func renderFixtureHTMLSet(t *testing.T, path string, names []string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, document := range fixtureDocumentSet(t, path, names) {
		rendered, err := HTML(document, HTMLOptions{
			Stylesheets:         []Stylesheet{{Href: StylesheetFileName}},
			NoDefaultStylesheet: true,
		})
		if err != nil {
			t.Fatalf("render document %s as HTML: %v", document.Name(), err)
		}
		out[DocumentHTMLFileName(document.Name())] = rendered
	}
	return out
}

// TestHTMLLinkedDocumentsGolden locks the rendering of a linked set: one
// shared stylesheet link per file, relative links between files, and anchors
// on the referenced blocks.
func TestHTMLLinkedDocumentsGolden(t *testing.T) {
	rendered := renderFixtureHTMLSet(t,
		filepath.Join("testdata", "linked_reports.sysml"),
		[]string{"Observatory::SystemReport", "Observatory::Mass Appendix"})
	for _, file := range []string{
		"Observatory-SystemReport.html",
		"Observatory-Mass.20Appendix.html",
	} {
		got, ok := rendered[file]
		if !ok {
			t.Fatalf("no document rendered as %s; got %v", file, keys(rendered))
		}
		checkGolden(t, got, filepath.Join("testdata", "linked-html", file))
	}
}

// TestHTMLCrossDocumentLinksResolve checks cross-document references point at
// the other document's HTML file, and at an anchor that file declares.
func TestHTMLCrossDocumentLinksResolve(t *testing.T) {
	rendered := renderFixtureHTMLSet(t,
		filepath.Join("testdata", "linked_reports.sysml"),
		[]string{"Observatory::SystemReport", "Observatory::Mass Appendix"})
	report := rendered["Observatory-SystemReport.html"]
	for _, want := range []string{
		`href="Observatory-Mass.20Appendix.html#tables-masses" data-document="Observatory::Mass Appendix"`,
		`href="Observatory-Mass.20Appendix.html" data-document="Observatory::Mass Appendix"`,
		`<link rel="stylesheet" href="sysml-document.css">`,
	} {
		if !strings.Contains(report, want) {
			t.Errorf("report lacks %q:\n%s", want, report)
		}
	}
	appendix := rendered["Observatory-Mass.20Appendix.html"]
	if !strings.Contains(appendix, `id="tables-masses"`) {
		t.Errorf("appendix lacks the referenced identifier:\n%s", appendix)
	}
	if !strings.Contains(appendix, `href="Observatory-SystemReport.html"`) {
		t.Errorf("appendix lacks the back link:\n%s", appendix)
	}
	// The Markdown file names are untouched by the HTML backend.
	if got := DocumentFileName("Observatory::Mass Appendix"); got != "Observatory-Mass.20Appendix.md" {
		t.Errorf("DocumentFileName = %q", got)
	}
}

// TestDocumentHTMLFileNameEncoding checks HTML file names derive from
// qualified names with the same escaping as anchors.
func TestDocumentHTMLFileNameEncoding(t *testing.T) {
	for fqn, want := range map[string]string{
		"Reports::MassReport":   "Reports-MassReport.html",
		"Observatory::Mass 1/2": "Observatory-Mass.201.2F2.html",
		"Solo":                  "Solo.html",
	} {
		if got := DocumentHTMLFileName(fqn); got != want {
			t.Errorf("DocumentHTMLFileName(%q) = %q, want %q", fqn, got, want)
		}
	}
}
