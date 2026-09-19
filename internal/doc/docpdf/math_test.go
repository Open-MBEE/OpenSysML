package docpdf

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestParseBlocksFormula checks that a $$ block parses into a formula block
// holding its LaTeX lines, and that one never closed is a typed error.
func TestParseBlocksFormula(t *testing.T) {
	md := "# T\n\n<!-- caption -->\n*Area*\n\n$$\nA = \\pi r^2\n= \\frac{\\pi D^2}{4}\n$$\n\n# After\n"
	blocks, err := parseBlocks(md)
	if err != nil {
		t.Fatalf("parseBlocks: %v", err)
	}
	kinds := []blockKind{blockHeading, blockCaption, blockFormula, blockHeading}
	if len(blocks) != len(kinds) {
		t.Fatalf("got %d blocks, want %d: %+v", len(blocks), len(kinds), blocks)
	}
	for i, want := range kinds {
		if blocks[i].Kind != want {
			t.Fatalf("block %d: got kind %d, want %d", i, blocks[i].Kind, want)
		}
	}
	if blocks[2].Source != "A = \\pi r^2\n= \\frac{\\pi D^2}{4}" {
		t.Fatalf("formula source %q", blocks[2].Source)
	}
	_, err = parseBlocks("# T\n\n$$\nA = 1\n\n# swallowed\n")
	var docErr *Error
	if !errors.As(err, &docErr) || docErr.Kind != ErrorUnclosedMath {
		t.Fatalf("unclosed $$: got %v, want ErrorUnclosedMath", err)
	}
	if !strings.Contains(err.Error(), "display-math block") {
		t.Fatalf("message %q", err.Error())
	}
}

// TestSplitMath checks where inline formulas begin and end: unescaped
// dollars outside code spans and links, never empty, never unclosed.
func TestSplitMath(t *testing.T) {
	for _, c := range []struct {
		in   string
		want []segment
	}{
		{"plain", []segment{{text: "plain"}}},
		{"", []segment{{text: ""}}},
		{"$x$", []segment{{text: "x", math: true}}},
		{"a $x$ b", []segment{{text: "a "}, {text: "x", math: true}, {text: " b"}}},
		{`$a_1 \cdot b$`, []segment{{text: `a_1 \cdot b`, math: true}}},
		{`costs \$5`, []segment{{text: `costs \$5`}}},
		{`\$5 and $x$`, []segment{{text: `\$5 and `}, {text: "x", math: true}}},
		{`$\$ per m^2$`, []segment{{text: `\$ per m^2`, math: true}}},
		{`$x\\$`, []segment{{text: `x\\`, math: true}}},
		{"`$x$`", []segment{{text: "`$x$`"}}},
		{"[$5](<https://example.com/$>) $y$", []segment{{text: "[$5](<https://example.com/$>) "}, {text: "y", math: true}}},
		{"unclosed $x", []segment{{text: "unclosed $x"}}},
		{"empty $$ here", []segment{{text: "empty $$ here"}}},
		{"$x$$y$", []segment{{text: "x", math: true}, {text: "y", math: true}}},
	} {
		if got := splitMath(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("splitMath(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

// TestInlineHTMLMath checks that inline formulas render as the typeset HTML
// in a math span, and as their escaped source when none was typeset.
func TestInlineHTMLMath(t *testing.T) {
	typeset := formulas{html: map[formula]string{
		{Source: "E = mc^2"}: `<span class="katex">E</span>`,
	}}
	for _, c := range []struct{ in, want string }{
		{"Einstein: $E = mc^2$\\.", `Einstein: <span class="math"><span class="katex">E</span></span>.`},
		{"*em* $E = mc^2$ **st**", `<em>em</em> <span class="math"><span class="katex">E</span></span> <strong>st</strong>`},
		{`$a < b$`, `<span class="math">a &lt; b</span>`},
		{`\$5 buys $x$`, `$5 buys <span class="math">x</span>`},
		{"`$x$`", "<code>$x$</code>"},
		{"unclosed $x", "unclosed $x"},
	} {
		if got := inlineHTML(c.in, typeset); got != c.want {
			t.Errorf("inlineHTML(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestCollectFormulas checks that every prose line and display block is
// searched, in document order, each distinct formula once.
func TestCollectFormulas(t *testing.T) {
	blocks := []block{
		{Kind: blockHeading, Level: 1, Text: "Report on $x$"},
		{Kind: blockParagraph, Text: "$x$ and $y$"},
		{Kind: blockCaption, Text: "Table with $z$"},
		{Kind: blockTable, Header: []string{"$h$", "b"}, Rows: [][]string{{"$c$", "$x$"}}},
		{Kind: blockList, Items: []string{"$i$", "plain"}},
		{Kind: blockFormula, Source: "x"},
		{Kind: blockFormula, Source: "x"},
		{Kind: blockMermaid, Source: "graph TD; A-->B[$]"},
		{Kind: blockDOT, Source: "digraph { $ }"},
	}
	want := []formula{
		{Source: "x"}, {Source: "y"}, {Source: "z"}, {Source: "h"}, {Source: "c"}, {Source: "i"},
		{Source: "x", Display: true},
	}
	if got := collectFormulas(blocks); !reflect.DeepEqual(got, want) {
		t.Fatalf("collectFormulas = %+v, want %+v", got, want)
	}
}

// TestDocumentHTMLFormulas checks the page a converter lays out: the KaTeX
// stylesheet linked, display formulas in their own block, inline ones in
// math spans, and the fallback of escaped source when nothing was typeset.
func TestDocumentHTMLFormulas(t *testing.T) {
	md := "# T\n\nMass $m \\propto D^{2.5}$ costs \\$5\\.\n\n<!-- caption -->\n*Area*\n\n$$\nA = \\pi r^2\n$$\n\n| q | v |\n| --- | --- |\n| area | $\\pi r^2$ |\n"
	blocks, err := parseBlocks(md)
	if err != nil {
		t.Fatalf("parseBlocks: %v", err)
	}
	typeset := formulas{
		css: "katex/katex.min.css",
		html: map[formula]string{
			{Source: `m \propto D^{2.5}`}:          `<span class="katex">M</span>`,
			{Source: `\pi r^2`}:                    `<span class="katex">P</span>`,
			{Source: `A = \pi r^2`, Display: true}: `<span class="katex-display"><span class="katex">A</span></span>`,
		},
	}
	page := documentHTML(blocks, artwork{math: typeset}, Options{})
	for _, want := range []string{
		`<link rel="stylesheet" href="katex/katex.min.css">`,
		`<p>Mass <span class="math"><span class="katex">M</span></span> costs $5.</p>`,
		`<p class="caption"><em>Area</em></p>`,
		`<div class="formula"><span class="katex-display"><span class="katex">A</span></span></div>`,
		`<td><span class="math"><span class="katex">P</span></span></td>`,
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("HTML missing %q:\n%s", want, page)
		}
	}
	for _, stray := range []string{`\propto`, "$$", `\pi`} {
		if strings.Contains(page, stray) {
			t.Fatalf("HTML leaks LaTeX %q:\n%s", stray, page)
		}
	}
	plain := documentHTML(blocks, artwork{}, Options{})
	if strings.Contains(plain, "<link") {
		t.Fatalf("page without formulas links a stylesheet:\n%s", plain)
	}
	if !strings.Contains(plain, `<div class="formula">A = \pi r^2</div>`) || !strings.Contains(plain, `<span class="math">m \propto D^{2.5}</span>`) {
		t.Fatalf("untypeset formulas do not show their source:\n%s", plain)
	}
}

// TestMarkdownWithFormulas checks the Markdown a converter reads itself:
// each formula replaced by its typeset HTML in a raw-attribute span or
// block, prose and fenced code kept as written.
func TestMarkdownWithFormulas(t *testing.T) {
	typeset := formulas{html: map[formula]string{
		{Source: "x"}:                `<span class="katex">` + "`x`" + `</span>`,
		{Source: "y", Display: true}: `<span class="katex-display">Y</span>`,
	}}
	md := strings.Join([]string{
		"# T",
		"",
		"Inline $x$ and \\$5 and `$x$`\\.",
		"",
		"[*Caption with $x$*]{.caption}",
		"",
		"$$",
		"y",
		"$$",
		"",
		"```dot",
		"digraph { a [label=\"$x$\"] }",
		"```",
		"",
	}, "\n")
	want := strings.Join([]string{
		"# T",
		"",
		"Inline ``<span class=\"math\"><span class=\"katex\">`x`</span></span>``{=html} and \\$5 and `$x$`\\.",
		"",
		"[*Caption with ``<span class=\"math\"><span class=\"katex\">`x`</span></span>``{=html}*]{.caption}",
		"",
		"```{=html}",
		`<div class="formula"><span class="katex-display">Y</span></div>`,
		"```",
		"",
		"```dot",
		"digraph { a [label=\"$x$\"] }",
		"```",
		"",
	}, "\n")
	if got := markdownWithFormulas(md, typeset); got != want {
		t.Fatalf("markdownWithFormulas =\n%s\nwant\n%s", got, want)
	}
	if got := markdownWithFormulas("plain\n", formulas{}); got != "plain\n" {
		t.Fatalf("plain Markdown changed: %q", got)
	}
}

// fakeKatex installs a katex stand-in that writes a recognizable span around
// the input, marking display mode, plus a stylesheet and fonts beside it as
// the real package lays them out.
func fakeKatex(t *testing.T, dir string) {
	t.Helper()
	pkg := filepath.Join(dir, "katex-pkg")
	if err := os.MkdirAll(filepath.Join(pkg, "dist", "fonts"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "dist", "katex.min.css"), []byte(".katex{font-family:KaTeX_Main}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "dist", "fonts", "KaTeX_Main-Regular.woff2"), []byte("font"), 0o600); err != nil {
		t.Fatal(err)
	}
	fakeTool(t, pkg, "katex", KatexEnv, strings.Join([]string{
		`mode=inline`,
		`while [ $# -gt 0 ]; do`,
		`  case "$1" in`,
		`    --input) in="$2"; shift 2;;`,
		`    --output) out="$2"; shift 2;;`,
		`    --display-mode) mode=display; shift;;`,
		`    *) shift;;`,
		`  esac`,
		`done`,
		`printf '<span class="katex %s">%s</span>' "$mode" "$(cat "$in")" > "$out"`,
		"",
	}, "\n"))
	t.Setenv(KatexCSSEnv, "")
}

// mathMarkdown is a document with inline math in prose and a table, a
// captioned display formula, and a literal dollar sign.
const mathMarkdown = "# Optics\n\nAperture area scales as $A \\propto D^2$ for \\$5\\.\n\n<!-- caption -->\n*Rayleigh criterion*\n\n$$\n\\theta = 1.22\\,\\frac{\\lambda}{D}\n$$\n\n| q | v |\n| --- | --- |\n| area | $\\pi r^2$ |\n"

// TestRenderFormulasWithFakeTools checks the whole preparation for an
// HTML-reading converter: KaTeX run once per distinct formula, its
// stylesheet and fonts copied beside the page, and the page showing the
// typeset HTML rather than LaTeX.
func TestRenderFormulasWithFakeTools(t *testing.T) {
	dir := t.TempDir()
	fakeKatex(t, dir)
	capture := filepath.Join(dir, "capture")
	fakeTool(t, dir, "weasyprint", WeasyPrintEnv,
		`cp "$1" "`+capture+`.html"; ls -R "$(dirname "$1")" > "`+capture+`.ls"; printf '%%PDF-1.7 fake' > "$2"`+"\n")
	pdf, err := Render(mathMarkdown, "weasyprint", Options{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.HasPrefix(string(pdf), "%PDF-") {
		t.Fatalf("output is no PDF: %q", pdf)
	}
	page, err := os.ReadFile(capture + ".html")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<link rel="stylesheet" href="katex/katex.min.css">`,
		`<p>Aperture area scales as <span class="math"><span class="katex inline">A \propto D^2</span></span> for $5.</p>`,
		`<div class="formula"><span class="katex display">\theta = 1.22\,\frac{\lambda}{D}</span></div>`,
		`<td><span class="math"><span class="katex inline">\pi r^2</span></span></td>`,
	} {
		if !strings.Contains(string(page), want) {
			t.Fatalf("HTML missing %q:\n%s", want, page)
		}
	}
	listing, err := os.ReadFile(capture + ".ls")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"katex.min.css", "KaTeX_Main-Regular.woff2", "formula-1.tex", "formula-3.html"} {
		if !strings.Contains(string(listing), want) {
			t.Fatalf("working directory lacks %s:\n%s", want, listing)
		}
	}
	if strings.Contains(string(listing), "formula-4.tex") {
		t.Fatalf("more formulas typeset than the document has:\n%s", listing)
	}
}

// TestRenderFormulasForPandoc checks the Markdown-reading path: the KaTeX
// stylesheet passed as a second --css and the formulas rewritten as raw HTML.
func TestRenderFormulasForPandoc(t *testing.T) {
	dir := t.TempDir()
	fakeKatex(t, dir)
	capture := filepath.Join(dir, "capture")
	fakeTool(t, dir, "pandoc", PandocEnv,
		`echo "$@" > "`+capture+`.args"; cp "$1" "`+capture+`.md"; out=""; while [ $# -gt 0 ]; do [ "$1" = "--output" ] && out="$2"; shift; done; printf '%%PDF-1.7 fake' > "$out"`+"\n")
	fakeTool(t, dir, "weasyprint", WeasyPrintEnv, "exit 0\n")
	if _, err := Render(mathMarkdown, "pandoc", Options{}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	args, err := os.ReadFile(capture + ".args")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(args), "--css pandoc.css --output document.pdf --css katex/katex.min.css") {
		t.Fatalf("pandoc arguments lack the KaTeX stylesheet: %s", args)
	}
	md, err := os.ReadFile(capture + ".md")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Aperture area scales as `<span class=\"math\"><span class=\"katex inline\">A \\propto D^2</span></span>`{=html} for \\$5\\.",
		"[*Rayleigh criterion*]{.caption}",
		"```{=html}\n<div class=\"formula\"><span class=\"katex display\">\\theta = 1.22\\,\\frac{\\lambda}{D}</span></div>\n```",
		"| area | `<span class=\"math\"><span class=\"katex inline\">\\pi r^2</span></span>`{=html} |",
	} {
		if !strings.Contains(string(md), want) {
			t.Fatalf("Markdown missing %q:\n%s", want, md)
		}
	}
	if strings.Contains(string(md), "$$") {
		t.Fatalf("Markdown keeps a $$ block:\n%s", md)
	}
}

// TestRenderWithoutFormulasNeedsNoKatex checks that a document with no math
// renders with katex absent, and the working directory holds no stylesheet.
func TestRenderWithoutFormulasNeedsNoKatex(t *testing.T) {
	dir := t.TempDir()
	capture := filepath.Join(dir, "capture")
	fakeTool(t, dir, "weasyprint", WeasyPrintEnv, `cp "$1" "`+capture+`.html"; printf '%%PDF-1.7 fake' > "$2"`+"\n")
	t.Setenv(KatexEnv, filepath.Join(dir, "no-katex-here"))
	if _, err := Render("# T\n\nCosts \\$5 in `$code$`\\.\n", "weasyprint", Options{}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	page, err := os.ReadFile(capture + ".html")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(page), "<link") || !strings.Contains(string(page), "<p>Costs $5 in <code>$code$</code>.</p>") {
		t.Fatalf("page:\n%s", page)
	}
}

// TestRenderFormulasKatexMissing checks the typed error when a document has
// formulas but no katex is installed, and when its stylesheet is missing.
func TestRenderFormulasKatexMissing(t *testing.T) {
	dir := t.TempDir()
	fakeTool(t, dir, "weasyprint", WeasyPrintEnv, "exit 0\n")
	t.Setenv("PATH", dir)
	t.Setenv(KatexEnv, "")
	t.Setenv(KatexCSSEnv, "")
	_, err := Render(mathMarkdown, "weasyprint", Options{})
	var docErr *Error
	if !errors.As(err, &docErr) || docErr.Kind != ErrorToolMissing || docErr.Tool != "katex" || docErr.EnvVar != KatexEnv {
		t.Fatalf("got %v, want ErrorToolMissing for katex", err)
	}
	msg := err.Error()
	for _, want := range []string{"typesetting the document's formulas needs katex", "point OPENSYSML_KATEX at it"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("message %q lacks %q", msg, want)
		}
	}
	if strings.Contains(msg, "-pdf-engine") {
		t.Fatalf("message offers another engine, which would not help: %q", msg)
	}

	fakeTool(t, dir, "katex", KatexEnv, "exit 0\n")
	_, err = Render(mathMarkdown, "weasyprint", Options{})
	if !errors.As(err, &docErr) || docErr.Kind != ErrorToolMissing || docErr.Tool != "katex.min.css" || docErr.EnvVar != KatexCSSEnv {
		t.Fatalf("got %v, want ErrorToolMissing for the stylesheet", err)
	}
	if !strings.Contains(err.Error(), "point OPENSYSML_KATEX_CSS at it") {
		t.Fatalf("message %q", err.Error())
	}

	t.Setenv(KatexCSSEnv, filepath.Join(dir, "nowhere.css"))
	_, err = Render(mathMarkdown, "weasyprint", Options{})
	if !errors.As(err, &docErr) || docErr.Kind != ErrorToolMissing || docErr.Tool != filepath.Join(dir, "nowhere.css") {
		t.Fatalf("got %v, want ErrorToolMissing for the named stylesheet", err)
	}
}

// TestRenderFormulasStylesheetOverride checks that KatexCSSEnv names the
// stylesheet to copy, fonts beside it, when katex lives elsewhere.
func TestRenderFormulasStylesheetOverride(t *testing.T) {
	dir := t.TempDir()
	fakeKatex(t, dir)
	css := filepath.Join(dir, "elsewhere", "katex.min.css")
	if err := os.MkdirAll(filepath.Join(dir, "elsewhere", "fonts"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(css, []byte(".katex{color:red}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "elsewhere", "fonts", "Other.woff2"), []byte("font"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(KatexCSSEnv, css)
	capture := filepath.Join(dir, "capture")
	fakeTool(t, dir, "weasyprint", WeasyPrintEnv,
		`cat "$(dirname "$1")/katex/katex.min.css" > "`+capture+`.css"; ls "$(dirname "$1")/katex/fonts" > "`+capture+`.fonts"; printf '%%PDF-1.7 fake' > "$2"`+"\n")
	if _, err := Render(mathMarkdown, "weasyprint", Options{}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	copied, err := os.ReadFile(capture + ".css")
	if err != nil {
		t.Fatal(err)
	}
	fonts, err := os.ReadFile(capture + ".fonts")
	if err != nil {
		t.Fatal(err)
	}
	if string(copied) != ".katex{color:red}\n" || strings.TrimSpace(string(fonts)) != "Other.woff2" {
		t.Fatalf("copied %q with fonts %q", copied, fonts)
	}
}

// TestRenderFormulasKatexFails checks that malformed LaTeX surfaces as the
// parse error katex reports, without its stack trace.
func TestRenderFormulasKatexFails(t *testing.T) {
	dir := t.TempDir()
	fakeKatex(t, dir)
	fakeTool(t, filepath.Join(dir, "katex-pkg"), "katex", KatexEnv, strings.Join([]string{
		`printf '%s\n' 'ParseError: KaTeX parse error: Expected group after '"'"'\frac'"'"' at end of input: \frac{a}' >&2`,
		`printf '%s\n' '    at new ParseError (/x/katex.js:1:1)' >&2`,
		`printf '%s\n' '    at Parser.parseGroup (/x/katex.js:2:2)' >&2`,
		`exit 1`,
		"",
	}, "\n"))
	fakeTool(t, dir, "weasyprint", WeasyPrintEnv, "exit 0\n")
	_, err := Render("# T\n\n$\\frac{a}$\n", "weasyprint", Options{})
	var docErr *Error
	if !errors.As(err, &docErr) || docErr.Kind != ErrorToolFailed || docErr.Tool != "katex" {
		t.Fatalf("got %v, want ErrorToolFailed for katex", err)
	}
	if docErr.Detail != `KaTeX parse error: Expected group after '\frac' at end of input: \frac{a}` {
		t.Fatalf("detail %q", docErr.Detail)
	}

	fakeTool(t, filepath.Join(dir, "katex-pkg"), "katex", KatexEnv, "exit 0\n")
	_, err = Render("# T\n\n$x$\n", "weasyprint", Options{})
	if !errors.As(err, &docErr) || docErr.Kind != ErrorToolFailed || !strings.Contains(docErr.Detail, "wrote no HTML for formula-1.tex") {
		t.Fatalf("got %v, want ErrorToolFailed for missing output", err)
	}
}
