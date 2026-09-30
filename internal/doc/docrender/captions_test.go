package docrender

import (
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

const numberedFixture = "numbered_report.sysml"

// numberedCaptions are the fixture's captions in document order: a formula is
// not numbered, and a block without a caption still takes its number.
var numberedCaptions = []string{
	"Table 1. Optical parts",
	"Figure 1. How the parts connect",
	"Collecting area",
	"Figure 2",
	"Table 2",
	"Table 3. The parts as rows",
	"Figure 3. The survey mark",
}

// TestCaptionsNumbered checks the caption list the PDF filter matches numbers as the backends write.
func TestCaptionsNumbered(t *testing.T) {
	document := fixtureDocument(t, filepath.Join("testdata", numberedFixture), "Numbered::NumberedReport")
	if got := Captions(document, true); !reflect.DeepEqual(got, numberedCaptions) {
		t.Errorf("numbered captions:\n got %q\nwant %q", got, numberedCaptions)
	}
	plain := []string{"Optical parts", "How the parts connect", "Collecting area", "The parts as rows", "The survey mark"}
	if got := Captions(document, false); !reflect.DeepEqual(got, plain) {
		t.Errorf("unnumbered captions:\n got %q\nwant %q", got, plain)
	}
}

// TestMarkdownNumbersFiguresAndTables checks the numbered captions are the
// Markdown caption paragraphs, and unnumbered by default.
func TestMarkdownNumbersFiguresAndTables(t *testing.T) {
	document := fixtureDocument(t, filepath.Join("testdata", numberedFixture), "Numbered::NumberedReport")
	numbered, err := Markdown(document, MarkdownOptions{DiagramForm: view.FormMermaid, NumberFigures: true})
	if err != nil {
		t.Fatalf("render numbered: %v", err)
	}
	for _, caption := range numberedCaptions {
		if !strings.Contains(numbered, "\n*"+inline(caption)+"*\n") {
			t.Errorf("caption %q is not written as a paragraph of its own:\n%s", caption, numbered)
		}
	}
	plain, err := Markdown(document, MarkdownOptions{DiagramForm: view.FormMermaid})
	if err != nil {
		t.Fatalf("render plain: %v", err)
	}
	if strings.Contains(plain, "Figure 1") || strings.Contains(plain, "Table 1") || !strings.Contains(plain, "\n*Optical parts*\n") {
		t.Errorf("numbering is on by default:\n%s", plain)
	}
}

// TestHTMLNumbersFiguresAndTables checks every HTML caption element carries
// the number in its own span, and an image's alt stays the stated caption.
func TestHTMLNumbersFiguresAndTables(t *testing.T) {
	path := filepath.Join("testdata", numberedFixture)
	numbered := renderFixtureHTML(t, path, "Numbered::NumberedReport", HTMLOptions{DiagramForm: view.FormMermaid, NumberFigures: true, Fragment: true})
	captions := regexp.MustCompile(`<(?:caption|figcaption) class="sysml-caption">(.*?)</(?:caption|figcaption)>`).FindAllStringSubmatch(numbered, -1)
	var got []string
	for _, match := range captions {
		got = append(got, match[1])
	}
	want := []string{
		`<span class="sysml-caption-number">Table 1.</span> Optical parts`,
		`<span class="sysml-caption-number">Figure 1.</span> How the parts connect`,
		`Collecting area`,
		`<span class="sysml-caption-number">Figure 2</span>`,
		`<span class="sysml-caption-number">Table 2</span>`,
		`<span class="sysml-caption-number">Table 3.</span> The parts as rows`,
		`<span class="sysml-caption-number">Figure 3.</span> The survey mark`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("HTML captions:\n got %q\nwant %q\n%s", got, want, numbered)
	}
	if !strings.Contains(numbered, `alt="The survey mark"`) {
		t.Errorf("the image's alt is not the stated caption:\n%s", numbered)
	}
	plain := renderFixtureHTML(t, path, "Numbered::NumberedReport", HTMLOptions{DiagramForm: view.FormMermaid, Fragment: true})
	if strings.Contains(plain, "sysml-caption-number") || !strings.Contains(plain, `<caption class="sysml-caption">Optical parts</caption>`) {
		t.Errorf("numbering is on by default:\n%s", plain)
	}
}

// TestCaptionTextAcrossBackends checks HTML captions read as the caption list, so a PDF from either backend agrees.
func TestCaptionTextAcrossBackends(t *testing.T) {
	document := fixtureDocument(t, filepath.Join("testdata", numberedFixture), "Numbered::NumberedReport")
	page, err := HTML(document, HTMLOptions{DiagramForm: view.FormMermaid, NumberFigures: true, Fragment: true})
	if err != nil {
		t.Fatalf("render HTML: %v", err)
	}
	tags := regexp.MustCompile(`<[^>]+>`)
	captions := regexp.MustCompile(`<(?:caption|figcaption) class="sysml-caption">(.*?)</(?:caption|figcaption)>`).FindAllStringSubmatch(page, -1)
	var got []string
	for _, match := range captions {
		got = append(got, tags.ReplaceAllString(match[1], ""))
	}
	if !reflect.DeepEqual(got, numberedCaptions) {
		t.Errorf("HTML caption text differs from the caption list:\n got %q\nwant %q", got, numberedCaptions)
	}
}
