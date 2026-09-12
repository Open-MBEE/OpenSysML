package docpdf

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const sampleMarkdown = "# Mass Report\n\nGenerated for review\\.\n\n## Components\n\n| Name | Mass \\| kg |\n| --- | --- |\n| Mirror | 120 |\n| Strut<br>Assembly | 4\\.5 |\n\n<!-- caption -->\n*Table 1\\. Masses*\n\n- primary\n- secondary\n\n### Ordering\n\n1. first\n2. second\n\n```mermaid\nflowchart LR\n  a --> b\n```\n\n<!-- caption -->\n*Figure 1\\. Flow*\n"

func TestParseBlocks(t *testing.T) {
	blocks, err := parseBlocks(sampleMarkdown)
	if err != nil {
		t.Fatalf("parseBlocks: %v", err)
	}
	kinds := make([]blockKind, len(blocks))
	for i, b := range blocks {
		kinds[i] = b.Kind
	}
	want := []blockKind{blockHeading, blockParagraph, blockHeading, blockTable, blockCaption, blockList, blockHeading, blockList, blockMermaid, blockCaption}
	if len(kinds) != len(want) {
		t.Fatalf("got %d blocks, want %d: %v", len(kinds), len(want), kinds)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("block %d: got kind %d, want %d", i, kinds[i], want[i])
		}
	}
	table := blocks[3]
	if got := table.Header[1]; got != "Mass \\| kg" {
		t.Fatalf("escaped pipe header: got %q", got)
	}
	if got := table.Rows[1][0]; got != "Strut<br>Assembly" {
		t.Fatalf("folded newline cell: got %q", got)
	}
	if !blocks[7].Ordered || blocks[7].Items[1] != "second" {
		t.Fatalf("numbered list: got %+v", blocks[7])
	}
	if !strings.Contains(blocks[8].Source, "a --> b") {
		t.Fatalf("mermaid source: got %q", blocks[8].Source)
	}
}

func TestParseBlocksSkipsHTMLComments(t *testing.T) {
	md := "# T\n\n<!-- Parts::table — table rendering -->\n<!-- not represented: attribute mass is not projected -->\n| Name |\n| --- |\n| Mirror |\n"
	blocks, err := parseBlocks(md)
	if err != nil {
		t.Fatalf("parseBlocks: %v", err)
	}
	if len(blocks) != 2 || blocks[0].Kind != blockHeading || blocks[1].Kind != blockTable {
		t.Fatalf("comments not skipped: %+v", blocks)
	}
}

func TestParseBlocksHyphenOnlyRows(t *testing.T) {
	md := "| Name | Note |\n| --- | --- |\n| --- | - |\n| - | ---- |\n"
	blocks, err := parseBlocks(md)
	if err != nil {
		t.Fatalf("parseBlocks: %v", err)
	}
	if len(blocks) != 1 || len(blocks[0].Rows) != 2 {
		t.Fatalf("hyphen-only rows dropped: %+v", blocks)
	}
	if blocks[0].Rows[0][0] != "---" || blocks[0].Rows[1][1] != "----" {
		t.Fatalf("hyphen cells: %+v", blocks[0].Rows)
	}
}

func TestParseBlocksEmptyNumberedItem(t *testing.T) {
	blocks, err := parseBlocks("1. \n2. second\n")
	if err != nil {
		t.Fatalf("parseBlocks: %v", err)
	}
	if len(blocks) != 1 || !blocks[0].Ordered {
		t.Fatalf("empty item split the list: %+v", blocks)
	}
	if len(blocks[0].Items) != 2 || blocks[0].Items[0] != "" || blocks[0].Items[1] != "second" {
		t.Fatalf("items: %+v", blocks[0].Items)
	}
}

func TestIsCaptionEscapes(t *testing.T) {
	for line, want := range map[string]bool{
		`*Figure 1\. Flow*`:  true,
		`*ends with \\*`:     true,  // escaped backslash, live closer
		`*unterminated\*`:    false, // escaped closer
		`*inner * asterisk*`: false,
		`**`:                 false,
		`*x*`:                true,
	} {
		if got := isCaption(line); got != want {
			t.Fatalf("isCaption(%q) = %v, want %v", line, got, want)
		}
	}
}

func TestParseBlocksCaptionNeedsMarker(t *testing.T) {
	md := "# T\n\n<!-- caption -->\n*Table 1\\. Masses*\n\n*just emphasis*\n\n<!-- caption -->\n*Figure 1\\. Flow*\n"
	blocks, err := parseBlocks(md)
	if err != nil {
		t.Fatalf("parseBlocks: %v", err)
	}
	want := []blockKind{blockHeading, blockCaption, blockParagraph, blockCaption}
	if len(blocks) != len(want) {
		t.Fatalf("got %d blocks, want %d: %+v", len(blocks), len(want), blocks)
	}
	for i := range want {
		if blocks[i].Kind != want[i] {
			t.Fatalf("block %d: got kind %d, want %d", i, blocks[i].Kind, want[i])
		}
	}
	if blocks[1].Text != `Table 1\. Masses` || blocks[3].Text != `Figure 1\. Flow` {
		t.Fatalf("caption texts: %q, %q", blocks[1].Text, blocks[3].Text)
	}
	if blocks[2].Text != "*just emphasis*" {
		t.Fatalf("emphasis paragraph: %q", blocks[2].Text)
	}
}

func TestParseBlocksDanglingCaptionMarker(t *testing.T) {
	for _, md := range []string{
		"# T\n\n<!-- caption -->\n",
		"# T\n\n<!-- caption -->\nplain paragraph\n",
		"# T\n\n<!-- caption -->\n*inner * asterisk*\n",
	} {
		_, err := parseBlocks(md)
		var docErr *Error
		if !errors.As(err, &docErr) || docErr.Kind != ErrorDanglingCaption {
			t.Fatalf("parseBlocks(%q) = %v, want ErrorDanglingCaption", md, err)
		}
		if err.Error() == "" {
			t.Fatal("empty error message")
		}
	}
}

func TestDocumentHTMLCaptionVersusEmphasisParagraph(t *testing.T) {
	md := "# T\n\n<!-- caption -->\n*Table 1\\. Masses*\n\n*just emphasis*\n"
	blocks, err := parseBlocks(md)
	if err != nil {
		t.Fatalf("parseBlocks: %v", err)
	}
	page := documentHTML(blocks, nil, Options{})
	if !strings.Contains(page, `<p class="caption"><em>Table 1. Masses</em></p>`) {
		t.Fatalf("caption not styled as caption:\n%s", page)
	}
	if !strings.Contains(page, "<p><em>just emphasis</em></p>") {
		t.Fatalf("emphasis paragraph not a plain paragraph:\n%s", page)
	}
	if strings.Contains(page, `<p class="caption"><em>just emphasis</em></p>`) {
		t.Fatalf("emphasis paragraph styled as caption:\n%s", page)
	}
}

func TestMarkdownWithSpanCaptions(t *testing.T) {
	md := "# T\n\n<!-- caption -->\n*Table 1\\. Masses*\n\n*just emphasis*\n"
	got := markdownWithSpanCaptions(md)
	want := "# T\n\n[*Table 1\\. Masses*]{.caption}\n\n*just emphasis*\n"
	if got != want {
		t.Fatalf("markdownWithSpanCaptions = %q, want %q", got, want)
	}
}

func TestParseBlocksUnclosedFence(t *testing.T) {
	for _, fence := range []string{"```mermaid\nflowchart LR\n", "```dot\ndigraph {\n"} {
		_, err := parseBlocks("# T\n\n" + fence)
		var docErr *Error
		if !errors.As(err, &docErr) || docErr.Kind != ErrorUnclosedFence {
			t.Fatalf("%q: got %v, want ErrorUnclosedFence", fence, err)
		}
	}
}

// dotMarkdown is a document whose one diagram is written as DOT: a `&` and a
// quoted ID check the source reaches the page escaped, not interpreted.
const dotMarkdown = "# T\n\n## Flow\n\n```dot\n// kind: action\ndigraph \"a & b\" {\n  \"n0\" -> \"n1\";\n}\n```\n\n<!-- caption -->\n*Figure 1\\. Flow*\n"

// A DOT fence parses to its own block and is kept as source under a notice
// on both converter inputs; no diagram tool is looked for.
func TestDOTBlockIsKeptAsSource(t *testing.T) {
	blocks, err := parseBlocks(dotMarkdown)
	if err != nil {
		t.Fatalf("parseBlocks: %v", err)
	}
	if len(blocks) != 4 || blocks[2].Kind != blockDOT || !strings.Contains(blocks[2].Source, `"n0" -> "n1";`) {
		t.Fatalf("blocks = %+v", blocks)
	}
	t.Setenv("PATH", t.TempDir())
	t.Setenv(MermaidEnv, "")
	images, err := renderDiagrams(t.TempDir(), blocks)
	if err != nil || len(images) != 0 {
		t.Fatalf("renderDiagrams = %v, %v; want no images and no tool lookup", images, err)
	}
	page := documentHTML(blocks, nil, Options{})
	for _, want := range []string{
		`<figure class="dot"><p class="notice"><em>` + dotNotice + `</em></p>`,
		"<pre>// kind: action\ndigraph &#34;a &amp; b&#34; {\n  &#34;n0&#34; -&gt; &#34;n1&#34;;\n}</pre></figure>",
		`<p class="caption"><em>Figure 1. Flow</em></p>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("HTML missing %q:\n%s", want, page)
		}
	}
	if strings.Contains(page, "<img") {
		t.Errorf("a DOT block became an image:\n%s", page)
	}
	md := markdownWithImages(dotMarkdown, nil)
	if !strings.Contains(md, "*"+dotNotice+"*\n\n```dot\n// kind: action\n") || !strings.Contains(md, "\n}\n```\n") {
		t.Errorf("Markdown lacks the notice ahead of the fence:\n%s", md)
	}
	if strings.Contains(md, "![diagram]") {
		t.Errorf("a DOT fence became an image reference:\n%s", md)
	}
}

// A document mixing both forms renders its Mermaid to images and its DOT to
// source, each in block order.
func TestDOTAndMermaidBlocksTogether(t *testing.T) {
	md := sampleMarkdown + "\n" + strings.TrimPrefix(dotMarkdown, "# T\n")
	blocks, err := parseBlocks(md)
	if err != nil {
		t.Fatalf("parseBlocks: %v", err)
	}
	page := documentHTML(blocks, []string{"diagram-1.svg"}, Options{})
	image, dot := strings.Index(page, `<img src="diagram-1.svg"`), strings.Index(page, `<figure class="dot">`)
	if image < 0 || dot < 0 || image > dot {
		t.Fatalf("image at %d, DOT at %d:\n%s", image, dot, page)
	}
	out := markdownWithImages(md, []string{"diagram-1.svg"})
	if strings.Contains(out, "```mermaid") || !strings.Contains(out, "![diagram](diagram-1.svg)") || !strings.Contains(out, "```dot\n") {
		t.Fatalf("mixed Markdown:\n%s", out)
	}
}

// Rendering a document whose only diagram is DOT needs no Mermaid CLI.
func TestRenderDOTOnlyDocumentNeedsNoDiagramTool(t *testing.T) {
	dir := t.TempDir()
	seenPath := filepath.Join(dir, "input-seen.html")
	fakeTool(t, dir, "weasyprint", WeasyPrintEnv, `while IFS= read -r line; do printf '%s\n' "$line"; done < "$1" > "`+seenPath+`"
printf '%%PDF-1.7 fake' > "$2"
`)
	t.Setenv("PATH", dir)
	t.Setenv(MermaidEnv, "")
	pdf, err := Render(dotMarkdown, "weasyprint", Options{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.HasPrefix(string(pdf), "%PDF-") {
		t.Fatalf("output is no PDF: %q", pdf)
	}
	seen, err := os.ReadFile(seenPath)
	if err != nil {
		t.Fatalf("converter input: %v", err)
	}
	if !strings.Contains(string(seen), dotNotice) || !strings.Contains(string(seen), "digraph &#34;a &amp; b&#34;") {
		t.Fatalf("converter input lacks the DOT notice or source:\n%s", seen)
	}
}

func TestUnescape(t *testing.T) {
	if got := unescape(`Mass \| kg \* 2\\`); got != `Mass | kg * 2\` {
		t.Fatalf("unescape: got %q", got)
	}
	if got := unescape(`no\wescapes`); got != `no\wescapes` {
		t.Fatalf("non-punct escape: got %q", got)
	}
}

func TestDocumentHTML(t *testing.T) {
	blocks, err := parseBlocks(sampleMarkdown)
	if err != nil {
		t.Fatalf("parseBlocks: %v", err)
	}
	opts := Options{TitlePage: true, TOC: true, NumberSections: true}
	page := documentHTML(blocks, []string{"diagram-1.svg"}, opts)
	if page != documentHTML(blocks, []string{"diagram-1.svg"}, opts) {
		t.Fatal("HTML generation is nondeterministic")
	}
	for _, want := range []string{
		"<title>Mass Report</title>",
		`<div class="title-page"><h1>Mass Report</h1></div>`,
		`<nav class="toc">`,
		`<a href="#sec-1">1 Components</a>`,
		`<a href="#sec-1-1">1.1 Ordering</a>`,
		`<h2 id="sec-1"><span class="section-number">1</span> Components</h2>`,
		"<th>Mass | kg</th>",
		"<td>Strut<br>Assembly</td>",
		`<img src="diagram-1.svg"`,
		"<ol>",
		"<li>secondary</li>",
		"<em>Table 1. Masses</em>",
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("HTML missing %q:\n%s", want, page)
		}
	}
	plain := documentHTML(blocks, []string{"diagram-1.svg"}, Options{})
	if strings.Contains(plain, `<div class="title-page">`) || strings.Contains(plain, `<nav class="toc">`) || strings.Contains(plain, `<span class="section-number">`) {
		t.Fatal("options leaked into default HTML")
	}
	if !strings.Contains(plain, "<h1>Mass Report</h1>") {
		t.Fatal("default HTML lost the title heading")
	}
}

func TestMarkdownWithImages(t *testing.T) {
	got := markdownWithImages(sampleMarkdown, []string{"diagram-1.svg"})
	if strings.Contains(got, "```mermaid") {
		t.Fatalf("fence not replaced:\n%s", got)
	}
	if !strings.Contains(got, "![diagram](diagram-1.svg)") {
		t.Fatalf("image reference missing:\n%s", got)
	}
}

func TestEngineNamed(t *testing.T) {
	for _, name := range Engines() {
		c, err := EngineNamed(name)
		if err != nil || c.Name() != name {
			t.Fatalf("EngineNamed(%q): %v, %v", name, c, err)
		}
	}
	c, err := EngineNamed("")
	if err != nil || c.Name() != Engines()[0] {
		t.Fatalf("default engine: %v, %v", c, err)
	}
	_, err = EngineNamed("latex")
	var docErr *Error
	if !errors.As(err, &docErr) || docErr.Kind != ErrorUnknownEngine {
		t.Fatalf("unknown engine: got %v", err)
	}
	if !strings.Contains(err.Error(), "weasyprint, pandoc, prince") {
		t.Fatalf("unknown-engine message: %q", err.Error())
	}
}

func TestConverterCapabilities(t *testing.T) {
	want := map[string]Capabilities{
		"weasyprint": {Input: InputHTML, Tools: []string{"weasyprint"}},
		"pandoc":     {Input: InputMarkdown, Tools: []string{"pandoc", "weasyprint"}, NativeOptions: true},
		"prince":     {Input: InputHTML, Tools: []string{"prince"}},
	}
	for _, name := range Engines() {
		c, err := EngineNamed(name)
		if err != nil {
			t.Fatalf("EngineNamed(%q): %v", name, err)
		}
		got := c.Capabilities()
		if got.Input != want[name].Input || got.NativeOptions != want[name].NativeOptions ||
			strings.Join(got.Tools, ",") != strings.Join(want[name].Tools, ",") {
			t.Fatalf("%s capabilities: got %+v, want %+v", name, got, want[name])
		}
	}
}

func TestRenderToolMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	for _, tool := range []string{PandocEnv, WeasyPrintEnv, PrinceEnv, MermaidEnv} {
		t.Setenv(tool, "")
	}
	for _, engine := range Engines() {
		_, err := Render("# T\n\nbody\n", engine, Options{})
		var docErr *Error
		if !errors.As(err, &docErr) || docErr.Kind != ErrorToolMissing {
			t.Fatalf("engine %s: got %v, want ErrorToolMissing", engine, err)
		}
		if !strings.Contains(err.Error(), "the "+engine+" engine needs") {
			t.Fatalf("engine %s: message %q", engine, err.Error())
		}
		if !strings.Contains(err.Error(), "-pdf-engine") {
			t.Fatalf("engine %s: message misses the remedy: %q", engine, err.Error())
		}
	}
}

// fakeTool writes an executable shell script into dir and points envVar at it.
func fakeTool(t *testing.T, dir, name, envVar, script string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake tool scripts need a POSIX shell")
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o700); err != nil { // #nosec G306 -- the fake tool must be executable
		t.Fatal(err)
	}
	t.Setenv(envVar, path)
}

func TestRenderWithFakeConverter(t *testing.T) {
	dir := t.TempDir()
	fakeTool(t, dir, "weasyprint", WeasyPrintEnv, `printf '%%PDF-1.7 fake' > "$2"`+"\n")
	pdf, err := Render("# T\n\nbody\n", "weasyprint", Options{TOC: true})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.HasPrefix(string(pdf), "%PDF-") {
		t.Fatalf("output is no PDF: %q", pdf)
	}
}

func TestRenderDiagramsWithFakeTools(t *testing.T) {
	dir := t.TempDir()
	fakeTool(t, dir, "mmdc", MermaidEnv, `out=""
while [ $# -gt 0 ]; do [ "$1" = "--output" ] && out="$2"; shift; done
printf '<svg xmlns="http://www.w3.org/2000/svg"/>' > "$out"
`)
	fakeTool(t, dir, "weasyprint", WeasyPrintEnv, `cp "$1" input-seen.html
printf '%%PDF-1.7 fake' > "$2"
`)
	pdf, err := Render(sampleMarkdown, "weasyprint", Options{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.HasPrefix(string(pdf), "%PDF-") {
		t.Fatalf("output is no PDF: %q", pdf)
	}
}

func TestRenderDiagramToolMissing(t *testing.T) {
	dir := t.TempDir()
	fakeTool(t, dir, "weasyprint", WeasyPrintEnv, `printf '%%PDF-1.7 fake' > "$2"`+"\n")
	t.Setenv("PATH", dir)
	t.Setenv(MermaidEnv, "")
	_, err := Render(sampleMarkdown, "weasyprint", Options{})
	var docErr *Error
	if !errors.As(err, &docErr) || docErr.Kind != ErrorToolMissing {
		t.Fatalf("got %v, want ErrorToolMissing", err)
	}
	if !strings.Contains(err.Error(), "diagrams") || !strings.Contains(err.Error(), "mmdc") {
		t.Fatalf("diagram-tool message: %q", err.Error())
	}
}

func TestRenderToolFailed(t *testing.T) {
	dir := t.TempDir()
	fakeTool(t, dir, "weasyprint", WeasyPrintEnv, `echo "boom: bad input" >&2
exit 3
`)
	_, err := Render("# T\n\nbody\n", "weasyprint", Options{})
	var docErr *Error
	if !errors.As(err, &docErr) || docErr.Kind != ErrorToolFailed {
		t.Fatalf("got %v, want ErrorToolFailed", err)
	}
	if !strings.Contains(docErr.Detail, "boom: bad input") {
		t.Fatalf("stderr not carried: %q", docErr.Detail)
	}
}

func TestRenderNoPDF(t *testing.T) {
	dir := t.TempDir()
	fakeTool(t, dir, "weasyprint", WeasyPrintEnv, "exit 0\n")
	_, err := Render("# T\n\nbody\n", "weasyprint", Options{})
	var docErr *Error
	if !errors.As(err, &docErr) || docErr.Kind != ErrorNoPDF {
		t.Fatalf("got %v, want ErrorNoPDF", err)
	}
}

func TestParseTelescopeGolden(t *testing.T) {
	golden, err := os.ReadFile(filepath.Join("..", "core", "docrender", "testdata", "telescope_report.golden.md"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	blocks, err := parseBlocks(string(golden))
	if err != nil {
		t.Fatalf("parseBlocks: %v", err)
	}
	var mermaid, tables, headings int
	for _, b := range blocks {
		switch b.Kind {
		case blockMermaid:
			mermaid++
		case blockTable:
			tables++
		case blockHeading:
			headings++
		}
	}
	if mermaid != 2 {
		t.Fatalf("got %d mermaid blocks, want 2", mermaid)
	}
	if tables == 0 || headings == 0 {
		t.Fatalf("got %d tables, %d headings", tables, headings)
	}
	page := documentHTML(blocks, []string{"diagram-1.svg", "diagram-2.svg"}, Options{TitlePage: true, TOC: true, NumberSections: true})
	if !strings.Contains(page, "diagram-2.svg") {
		t.Fatal("second diagram missing from HTML")
	}
	// The golden's definitions entries are paragraphs whose term is a strong
	// run, so they must reach the PDF as markup rather than literal asterisks.
	for _, want := range []string{
		"<p><strong>M1</strong> — The primary mirror assembly. Collects light | not *heat* from the target.</p>",
		"<p><strong>M3|*</strong></p>",
		"<p>Actuators that phase the mirror segments.</p>",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("definitions entry missing from HTML: %q\n%s", want, page)
		}
	}
}
