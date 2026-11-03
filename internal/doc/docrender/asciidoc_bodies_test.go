package docrender

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/docplan/markup"
	"github.com/Open-MBEE/OpenSysML/internal/ir/docplan/markup/markuptest"
)

const asciiDocRecordings = "../../ir/docplan/markup/testdata"

// useRecordedAsciiDoc points the AsciiDoc parser at the recorded converter,
// so the fixture renders without node.
func useRecordedAsciiDoc(t *testing.T) {
	t.Helper()
	converter, err := markuptest.RecordedConverter(t.TempDir(), asciiDocRecordings)
	if err != nil {
		t.Skip(err)
	}
	t.Setenv(markup.AsciiDocEnv, converter)
}

// TestAsciiDocBodiesReportGolden locks the Markdown of a report whose prose is
// written as AsciiDoc bodies: both shapes, lists, an admonition, a table, a
// formula, an image, links, references and math.
func TestAsciiDocBodiesReportGolden(t *testing.T) {
	useRecordedAsciiDoc(t)
	got := renderFixtureDocument(t, filepath.Join("testdata", "asciidoc_report.sysml"), "Prose::AsciiDocReport")
	checkGolden(t, got, filepath.Join("testdata", "asciidoc_report.golden.md"))
}

// TestHTMLAsciiDocBodiesReportGolden locks the HTML fragment of the same report.
func TestHTMLAsciiDocBodiesReportGolden(t *testing.T) {
	useRecordedAsciiDoc(t)
	got := renderFixtureHTML(t, filepath.Join("testdata", "asciidoc_report.sysml"), "Prose::AsciiDocReport",
		HTMLOptions{Fragment: true})
	checkGolden(t, got, filepath.Join("testdata", "asciidoc_report.fragment.golden.html"))
}

// TestAsciiDocBodiesAsciiDocGolden locks the round trip: prose written as
// AsciiDoc bodies rendered back out with -doc-form asciidoc.
func TestAsciiDocBodiesAsciiDocGolden(t *testing.T) {
	useRecordedAsciiDoc(t)
	got := renderFixtureAsciiDoc(t, "asciidoc_report.sysml", "Prose::AsciiDocReport", AsciiDocOptions{})
	checkGolden(t, got, filepath.Join("testdata", "asciidoc_report.golden.adoc"))
}

// The table, formula and image written in the body reach the Markdown as the
// content blocks they map onto, and the admonition leads with its label.
func TestAsciiDocBodiesContentBlocks(t *testing.T) {
	useRecordedAsciiDoc(t)
	got := renderFixtureDocument(t, filepath.Join("testdata", "asciidoc_report.sysml"), "Prose::AsciiDocReport")
	for _, want := range []string{
		"**Note:** Mind the **gap**.",
		"| Part | Mass |",
		"| Mirror | 12 kg |",
		"$$\nm \\propto D^2\n$$",
		"![The optics](https://example.test/figures/optics.png)",
		"- two *soft*",
		"1. first",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendering does not contain %q\n%s", want, got)
		}
	}
}
