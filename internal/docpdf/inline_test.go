package docpdf

import (
	"strings"
	"testing"
)

// TestInlineHTMLPlainText checks that escaped prose unescapes and
// HTML-escapes, with non-punctuation escapes kept literal.
func TestInlineHTMLPlainText(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{`Generated for review\.`, "Generated for review."},
		{`Mass \| kg \* 2\\`, `Mass | kg * 2\`},
		{`a \< b \& c`, "a &lt; b &amp; c"},
		{`no\wescapes`, `no\wescapes`},
		{`trailing\`, `trailing\`},
		{"", ""},
	} {
		if got := inlineHTML(c.in, formulas{}); got != c.want {
			t.Errorf("inlineHTML(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestInlineHTMLEmphasis checks emphasis and strong spans, including the
// flanking whitespace and escaped delimiters docrender writes.
func TestInlineHTMLEmphasis(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"*plain*", "<em>plain</em>"},
		{`*with \*stars\**`, "<em>with *stars*</em>"},
		{" *padded* ", " <em>padded</em> "},
		{"\t*tabbed*\t", "\t<em>tabbed</em>\t"},
		{`**bold\_move**`, "<strong>bold_move</strong>"},
		{`**a\|b**`, "<strong>a|b</strong>"},
		{"before **mid** after", "before <strong>mid</strong> after"},
		{"*em* and **strong**", "<em>em</em> and <strong>strong</strong>"},
		{"unclosed *span", "unclosed *span"},
		{"unclosed **span", "unclosed **span"},
	} {
		if got := inlineHTML(c.in, formulas{}); got != c.want {
			t.Errorf("inlineHTML(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestInlineHTMLCodeSpans checks the inverse of docrender's code-span fence
// contract: longer fences, padding, and content kept verbatim.
func TestInlineHTMLCodeSpans(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"`x > 1`", "<code>x &gt; 1</code>"},
		{"`` a `tick` ``", "<code>a `tick`</code>"},
		{"``` ``double`` ```", "<code>``double``</code>"},
		{"`` `leading ``", "<code>`leading</code>"},
		{"`  padded  `", "<code> padded </code>"},
		{"`  `", "<code></code>"},
		{"`two lines`", "<code>two lines</code>"},
		{"see `code` here", "see <code>code</code> here"},
		{"unclosed `span", "unclosed `span"},
		{"`*not em*`", "<code>*not em*</code>"},
	} {
		if got := inlineHTML(c.in, formulas{}); got != c.want {
			t.Errorf("inlineHTML(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestInlineHTMLLinks checks inline links: pointy-bracket destinations with
// their escapes undone, and reference links kept as fragment hrefs.
func TestInlineHTMLLinks(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"[spec](<https://example.com/spec(v2).md>)", `<a href="https://example.com/spec(v2).md">spec</a>`},
		{"[a b](<https://example.com/a b>)", `<a href="https://example.com/a b">a b</a>`},
		{`[x](<a\\b>)`, `<a href="a\b">x</a>`},
		{`[x](<a\<b\>c>)`, `<a href="a&lt;b&gt;c">x</a>`},
		{"[x](<a%0Ab>)", `<a href="a%0Ab">x</a>`},
		{"[items](#items)", `<a href="#items">items</a>`},
		{"see [items](#Report-items) here", `see <a href="#Report-items">items</a> here`},
		{`[label \*escaped\*](#a.2Eb)`, `<a href="#a.2Eb">label *escaped*</a>`},
		{`\[not a link\](x)`, "[not a link](x)"},
		{"[no destination]", "[no destination]"},
		{"[bare](plain)", "[bare](plain)"},
		{"[unclosed](<dest", "[unclosed](&lt;dest"},
		{"[unclosed](#ref", "[unclosed](#ref"},
	} {
		if got := inlineHTML(c.in, formulas{}); got != c.want {
			t.Errorf("inlineHTML(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestInlineHTMLMixedRuns checks a paragraph mixing every run kind, as
// docrender joins runs with single spaces.
func TestInlineHTMLMixedRuns(t *testing.T) {
	in := `The *margin* is **critical** for ` + "`m > 0`" + ` per [spec](<https://example.com/spec.md>) and [table](#Report-masses)\.`
	want := `The <em>margin</em> is <strong>critical</strong> for <code>m &gt; 0</code> per <a href="https://example.com/spec.md">spec</a> and <a href="#Report-masses">table</a>.`
	if got := inlineHTML(in, formulas{}); got != want {
		t.Errorf("inlineHTML(%q) = %q, want %q", in, got, want)
	}
}

// TestParseBlocksAnchor checks that docrender's standalone anchor lines parse
// into anchor blocks with their identifiers.
func TestParseBlocksAnchor(t *testing.T) {
	md := "# T\n\n<a id=\"Report-items\"></a>\n\n## Items\n\n<a id=\"a.2Eb-c\"></a>\n\nparagraph\n"
	blocks, err := parseBlocks(md)
	if err != nil {
		t.Fatalf("parseBlocks: %v", err)
	}
	kinds := []blockKind{blockHeading, blockAnchor, blockHeading, blockAnchor, blockParagraph}
	if len(blocks) != len(kinds) {
		t.Fatalf("got %d blocks, want %d: %+v", len(blocks), len(kinds), blocks)
	}
	for i, want := range kinds {
		if blocks[i].Kind != want {
			t.Fatalf("block %d: got kind %d, want %d", i, blocks[i].Kind, want)
		}
	}
	if blocks[1].Anchor != "Report-items" || blocks[3].Anchor != "a.2Eb-c" {
		t.Fatalf("anchor ids: %q, %q", blocks[1].Anchor, blocks[3].Anchor)
	}
}

// TestDocumentHTMLInlineRuns checks that a document using every inline
// construct docrender emits renders as semantic HTML: styled runs, links,
// native anchors, fragment references, and grouped-table headings.
func TestDocumentHTMLInlineRuns(t *testing.T) {
	md := strings.Join([]string{
		"# Inline Report",
		"",
		`<a id="Inline.20Report-masses"></a>`,
		"",
		"## Masses",
		"",
		"The *margin* is **critical** for `m > 0` per [spec](<https://example.com/spec(v2).md>)\\.",
		"",
		"See [the masses](#Inline.20Report-masses) above\\.",
		"",
		"**zone: hot**",
		"",
		"| name | zone |",
		"| --- | --- |",
		"| Mirror | hot |",
		"",
		"**zone: cold**",
		"",
		"| name | zone |",
		"| --- | --- |",
		"| Strut | cold |",
		"",
		"- item with *style*",
		"",
		"<!-- caption -->",
		"*Table 1\\. Grouped masses*",
		"",
	}, "\n")
	blocks, err := parseBlocks(md)
	if err != nil {
		t.Fatalf("parseBlocks: %v", err)
	}
	page := documentHTML(blocks, artwork{}, Options{})
	for _, want := range []string{
		`<a id="Inline.20Report-masses"></a>`,
		"<p>The <em>margin</em> is <strong>critical</strong> for <code>m &gt; 0</code> per " +
			`<a href="https://example.com/spec(v2).md">spec</a>.</p>`,
		`<p>See <a href="#Inline.20Report-masses">the masses</a> above.</p>`,
		"<p><strong>zone: hot</strong></p>",
		"<p><strong>zone: cold</strong></p>",
		"<li>item with <em>style</em></li>",
		`<p class="caption"><em>Table 1. Grouped masses</em></p>`,
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("HTML missing %q:\n%s", want, page)
		}
	}
	for _, stray := range []string{`\*`, "**zone", `<p>&lt;a id=`} {
		if strings.Contains(page, stray) {
			t.Fatalf("HTML leaks literal Markdown %q:\n%s", stray, page)
		}
	}
}
