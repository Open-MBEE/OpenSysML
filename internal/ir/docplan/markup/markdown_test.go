package markup

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// parse parses body as Markdown through the registry, as the planner does.
func parse(t *testing.T, body string) []Block {
	t.Helper()
	p, ok := For("Markdown")
	if !ok {
		t.Fatal("no Markdown parser")
	}
	blocks, err := p.Parse(body)
	if err != nil {
		t.Fatalf("parse %q: %v", body, err)
	}
	return blocks
}

func TestForMatchesLanguageCaseInsensitively(t *testing.T) {
	for _, language := range []string{"markdown", "Markdown", "MARKDOWN"} {
		if _, ok := For(language); !ok {
			t.Errorf("For(%q) found no parser", language)
		}
	}
	if _, ok := For("AsciiDoc"); !ok {
		t.Error("For(AsciiDoc) found no parser")
	}
	if _, ok := For("rst"); ok {
		t.Error("For(rst) found a parser")
	}
}

// Each inline construct maps onto one run of the kind and style the manual
// documents, and adjacent text of one style merges.
func TestMarkdownInlineRuns(t *testing.T) {
	body := "Plain *em* **strong** `co\\de` [lbl](https://x.io/a) <https://y.io> " +
		"[ref](sysml:a.b) [](sysml:c) [Mass:](sysml:value:a.mass) [](sysml:value:a.b) " +
		"$m \\propto D^2$ \\*lit\\* &amp; end"
	blocks := parse(t, body)
	if len(blocks) != 1 || blocks[0].Kind != BlockParagraph {
		t.Fatalf("got %+v, want one paragraph", blocks)
	}
	got := make([]Run, len(blocks[0].Runs))
	for i, run := range blocks[0].Runs {
		run.Span = Span{}
		got[i] = run
	}
	want := []Run{
		{Kind: RunText, Style: StylePlain, Text: "Plain "},
		{Kind: RunText, Style: StyleEmphasis, Text: "em"},
		{Kind: RunText, Style: StylePlain, Text: " "},
		{Kind: RunText, Style: StyleStrong, Text: "strong"},
		{Kind: RunText, Style: StylePlain, Text: " "},
		{Kind: RunText, Style: StyleCode, Text: "co\\de"},
		{Kind: RunText, Style: StylePlain, Text: " "},
		{Kind: RunLink, Text: "lbl", Destination: "https://x.io/a"},
		{Kind: RunText, Style: StylePlain, Text: " "},
		{Kind: RunLink, Text: "https://y.io", Destination: "https://y.io"},
		{Kind: RunText, Style: StylePlain, Text: " "},
		{Kind: RunRef, Text: "ref", Destination: "a.b"},
		{Kind: RunText, Style: StylePlain, Text: " "},
		{Kind: RunRef, Text: "", Destination: "c"},
		{Kind: RunText, Style: StylePlain, Text: " "},
		{Kind: RunValue, Text: "Mass:", Destination: "a.mass"},
		{Kind: RunText, Style: StylePlain, Text: " "},
		{Kind: RunValue, Text: "", Destination: "a.b"},
		{Kind: RunText, Style: StylePlain, Text: " "},
		{Kind: RunMath, Text: "m \\propto D^2"},
		{Kind: RunText, Style: StylePlain, Text: " *lit* & end"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("runs differ\ngot:  %+v\nwant: %+v", got, want)
	}
}

// Every run's span covers the body text it came from.
func TestMarkdownRunSpans(t *testing.T) {
	body := "Say *hi* to [x](sysml:y) and $a_b$\nnext line."
	blocks := parse(t, body)
	for _, run := range blocks[0].Runs {
		at := body[run.Span.Start:run.Span.Stop]
		switch run.Kind {
		case RunText:
			if !strings.Contains(at, strings.TrimSpace(run.Text)) {
				t.Errorf("run %+v spans %q", run, at)
			}
		case RunRef:
			if at != "[x](sysml:y)" {
				t.Errorf("ref spans %q", at)
			}
		case RunMath:
			if at != "$a_b$" {
				t.Errorf("math spans %q", at)
			}
		}
	}
	if span := blocks[0].Span; body[span.Start:span.Stop] != body {
		t.Errorf("paragraph spans %q", body[span.Start:span.Stop])
	}
}

// Soft and hard line breaks read as one space.
func TestMarkdownLineBreaks(t *testing.T) {
	blocks := parse(t, "one\ntwo  \nthree\\\nfour")
	if got := blocks[0].Runs[0].Text; got != "one two three four" {
		t.Errorf("got %q", got)
	}
}

// Dollars that do not open mathematics stay prose, and an escaped dollar never closes it.
func TestMarkdownMathDelimiters(t *testing.T) {
	for body, want := range map[string]string{
		"costs $5 and $10": "costs $5 and $10",
		"a $ b $ c":        "a $ b $ c",
		"\\$x$ stays":      "$x$ stays",
		"$$display$$ not":  "$$display$$ not",
	} {
		blocks := parse(t, body)
		if len(blocks[0].Runs) != 1 || blocks[0].Runs[0].Kind != RunText || blocks[0].Runs[0].Text != want {
			t.Errorf("%q: got %+v, want one text run %q", body, blocks[0].Runs, want)
		}
	}
	for body, want := range map[string]string{
		"cost $\\$ 10^6$ here": "\\$ 10^6",
		"$5 \\$ open$ closes":  "5 \\$ open",
	} {
		blocks := parse(t, body)
		if len(blocks[0].Runs) < 2 || blocks[0].Runs[len(blocks[0].Runs)-2].Kind != RunMath || blocks[0].Runs[len(blocks[0].Runs)-2].Text != want {
			t.Errorf("%q: escaped dollar inside math: %+v", body, blocks[0].Runs)
		}
	}
}

// Bullet and ordered lists of single paragraphs map onto list blocks in order.
func TestMarkdownLists(t *testing.T) {
	blocks := parse(t, "Intro.\n\n- one *a*\n- two\n\n1. first\n2. second\n\nOutro.")
	kinds := make([]BlockKind, len(blocks))
	for i, block := range blocks {
		kinds[i] = block.Kind
	}
	if want := []BlockKind{BlockParagraph, BlockList, BlockList, BlockParagraph}; !reflect.DeepEqual(kinds, want) {
		t.Fatalf("kinds %v, want %v", kinds, want)
	}
	if blocks[1].Ordered || !blocks[2].Ordered {
		t.Errorf("ordered flags: %v %v", blocks[1].Ordered, blocks[2].Ordered)
	}
	if len(blocks[1].Items) != 2 || len(blocks[1].Items[0].Runs) != 2 || blocks[1].Items[0].Runs[1].Style != StyleEmphasis {
		t.Errorf("bullet items %+v", blocks[1].Items)
	}
	if got := blocks[2].Items[1].Runs[0].Text; got != "second" {
		t.Errorf("second item %q", got)
	}
	body := "Intro.\n\n- one *a*\n- two\n\n1. first\n2. second\n\nOutro."
	if span := blocks[1].Span; body[span.Start:span.Stop] != "one *a*\n- two" {
		t.Errorf("list spans %q", body[span.Start:span.Stop])
	}
}

// ATX and setext headings are heading blocks at their level, their text
// flattened to a title and their span the heading's own lines.
func TestMarkdownHeadings(t *testing.T) {
	body := "Intro.\n\n# Top *one*\n\nText.\n\n## The `sysml` tool ##\n\nSetext\n======\n\nLower\n-----\n\n###### Six"
	p, _ := For("markdown")
	blocks, err := p.Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	type heading struct {
		level int
		title string
		span  string
	}
	var got []heading
	for _, block := range blocks {
		if block.Kind == BlockHeading {
			got = append(got, heading{block.Level, block.Title, body[block.Span.Start:block.Span.Stop]})
		}
	}
	want := []heading{
		{1, "Top one", "# Top *one*"},
		{2, "The sysml tool", "## The `sysml` tool ##"},
		{1, "Setext", "Setext\n======"},
		{2, "Lower", "Lower\n-----"},
		{6, "Six", "###### Six"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("headings:\n got %+v\nwant %+v", got, want)
	}
	if len(blocks) != 7 || blocks[0].Kind != BlockParagraph || blocks[2].Kind != BlockParagraph {
		t.Errorf("blocks around the headings: %+v", blocks)
	}
	for _, block := range got {
		if block.title != strings.TrimSpace(block.title) {
			t.Errorf("title %q is not trimmed", block.title)
		}
	}
}

// Every construct no content block carries is refused by name and body line.
func TestMarkdownRefusedConstructs(t *testing.T) {
	for body, want := range map[string]string{
		"# [x](https://a.io)":                  "link in a heading at body line 1",
		"## see [](sysml:x)":                   "reference in a heading at body line 1",
		"Sum $a$\n---":                         "formula in a heading at body line 1",
		"> quoted":                             "block quote at body line 1",
		"    code":                             "code block at body line 1",
		"```\ncode\n```":                       "fenced code block at body line 1",
		"<div>\n</div>":                        "HTML block at body line 1",
		"a <b>bold</b>":                        "inline HTML at body line 1",
		"![alt](pic.png)":                      "image at body line 1",
		"a\n\n---":                             "thematic break at body line 3",
		"- a\n  - nested":                      "nested list at body line 2",
		"- a\n\n  second para":                 "list item of more than one block at body line 1",
		"3. three":                             "ordered list starting at 3 at body line 1",
		"[x](https://a.io \"title\")":          "link title at body line 1",
		"***both***":                           "nested emphasis at body line 1",
		"*[x](https://a.io)*":                  "link inside emphasis at body line 1",
		"*`code`*":                             "code span inside emphasis at body line 1",
		"*$m$*":                                "mathematics inside emphasis at body line 1",
		"[*x*](https://a.io)":                  "styled link text at body line 1",
		"a\n\n| a | b |\n|---|---|\n| 1 | 2 |": "",
	} {
		p, _ := For("markdown")
		_, err := p.Parse(body)
		if want == "" {
			if err != nil {
				t.Errorf("%q: refused as %v, want a paragraph (no GFM)", body, err)
			}
			continue
		}
		var unsupported *UnsupportedConstructError
		if !errors.As(err, &unsupported) || unsupported.Error() != "unsupported "+want {
			t.Errorf("%q: got %v, want unsupported %s", body, err, want)
		}
	}
}
