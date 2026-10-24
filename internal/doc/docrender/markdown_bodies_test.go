package docrender

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestMarkdownBodiesReportGolden locks the Markdown of a report whose prose is
// written as Markdown bodies: both shapes, lists, links, references, math
// and escaped metacharacters.
func TestMarkdownBodiesReportGolden(t *testing.T) {
	got := renderFixtureDocument(t, filepath.Join("testdata", "markdown_report.sysml"), "Prose::ProseReport")
	checkGolden(t, got, filepath.Join("testdata", "markdown_report.golden.md"))
}

// TestHTMLBodiesReportGolden locks the HTML fragment of the same report.
func TestHTMLBodiesReportGolden(t *testing.T) {
	got := renderFixtureHTML(t, filepath.Join("testdata", "markdown_report.sysml"), "Prose::ProseReport",
		HTMLOptions{Fragment: true})
	checkGolden(t, got, filepath.Join("testdata", "markdown_report.fragment.golden.html"))
}

// Runs cut from a body keep the spacing they were written with: one space
// between words, none before punctuation, and the backends' escaping applies
// to the prose but not to the mathematics.
func TestMarkdownBodiesSpacingAndEscaping(t *testing.T) {
	got := renderFixtureDocument(t, filepath.Join("testdata", "markdown_report.sysml"), "Prose::ProseReport")
	for _, want := range []string{
		"This report is *generated* by `sysml -render-document` ([OpenSysML](<https://opensysml.org/>)); see [Subsystem Masses](#breakdown), the mount and the type.",
		`Mass scales as $m \propto D^{2.5}_{\text{eff}}$ and each \$ of budget buys about 1 cm^2; a literal \*star\* and \& stay prose, as does a \| pipe.`,
		"- first item with **strong** text\n- second item with a [link](<https://example.com/a b>)",
		"1. one\n2. two",
		"The **named** paragraph keeps its anchor; see [classic](#breakdown-classic).",
		"A section body closes the section.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendering does not contain %q\n%s", want, got)
		}
	}
	if strings.Contains(got, "  ") {
		t.Errorf("rendering carries a double space\n%s", got)
	}
}

// Runs cut from a body join exactly as written — markup touching its
// neighbouring text gains no space — and a reference inside a list item gives
// its target the anchor it links to.
func TestMarkdownBodiesAdjacentRunsAndListAnchors(t *testing.T) {
	fixture := filepath.Join("testdata", "markdown_adjacency.sysml")
	md := renderFixtureDocument(t, fixture, "Prose::AdjacencyReport")
	for _, want := range []string{
		"a*bold*b, c`code`d, [x](<https://example.com/x>)y and $m$! Then [reading](#reading)'s turn.",
		"- see [reading](#reading)\n- and [closing](#closing)",
		`<a id="reading"></a>`,
		`<a id="closing"></a>`,
	} {
		if !strings.Contains(md, want) {
			t.Errorf("Markdown does not contain %q\n%s", want, md)
		}
	}
	html := renderFixtureHTML(t, fixture, "Prose::AdjacencyReport", HTMLOptions{Fragment: true})
	for _, want := range []string{"a<em>bold</em>b, c<code>code</code>d,", `id="reading"`, `id="closing"`} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML does not contain %q\n%s", want, html)
		}
	}
}
