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
	page := documentHTML(blocks, artwork{}, Options{})
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
	page := documentHTML(blocks, artwork{diagrams: []diagram{{Image: "diagram-1.svg"}}}, opts)
	if page != documentHTML(blocks, artwork{diagrams: []diagram{{Image: "diagram-1.svg"}}}, opts) {
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
	plain := documentHTML(blocks, artwork{diagrams: []diagram{{Image: "diagram-1.svg"}}}, Options{})
	if strings.Contains(plain, `<div class="title-page">`) || strings.Contains(plain, `<nav class="toc">`) || strings.Contains(plain, `<span class="section-number">`) {
		t.Fatal("options leaked into default HTML")
	}
	if !strings.Contains(plain, "<h1>Mass Report</h1>") {
		t.Fatal("default HTML lost the title heading")
	}
}

func TestStyleSheetKeepsTablesWithinThePage(t *testing.T) {
	for _, want := range []string{
		"table { border-collapse: collapse; margin: 0.8em 0; width: 100%; }",
		"overflow-wrap: anywhere;",
	} {
		if !strings.Contains(styleSheet, want) {
			t.Fatalf("stylesheet lacks %q: a wide cell such as a list of qualified names would run off the page", want)
		}
	}
}

// TestStyleSheetSetsWideTablesLandscape checks that a table of seven or more columns, with its
// heading and caption, is set on a landscape page without the sibling-chain form cssselect2 misreads.
func TestStyleSheetSetsWideTablesLandscape(t *testing.T) {
	wide := "table:has(thead > tr > th:nth-child(7))"
	for _, want := range []string{
		"@page wide { size: landscape; }",
		"body { page: main; }",
		wide + " { page: wide; font-size: 9pt; }",
		"p:has(+ " + wide + "),",
		":is(p.caption, p:has(.caption)):has(+ p:has(+ " + wide + ")),",
		":is(h1, h2, h3, h4, h5, h6):has(+ :is(p.caption, p:has(.caption)):has(+ p:has(+ " + wide + "))) { page: wide; }",
		"th { overflow-wrap: normal; }",
		":is(h1, h2, h3, h4, h5, h6), :is(p.caption, p:has(.caption)) { break-after: avoid; }\np:has(+ table) { break-after: avoid; }",
	} {
		if !strings.Contains(styleSheet, want) {
			t.Fatalf("stylesheet lacks %q", want)
		}
	}
	if strings.Contains(styleSheet, "+ p + ") {
		t.Fatal("stylesheet chains sibling combinators inside :has(), which cssselect2 evaluates against the first sibling alone")
	}
}

// TestRenderForPandocLeavesDefaultStylesOff checks that pandoc is given only the print
// stylesheet; a document-css variable, even "false", switches its screen layout back on.
func TestRenderForPandocLeavesDefaultStylesOff(t *testing.T) {
	dir := t.TempDir()
	capture := filepath.Join(dir, "capture.args")
	fakeTool(t, dir, "pandoc", PandocEnv,
		`echo "$@" > "`+capture+`"; out=""; while [ $# -gt 0 ]; do [ "$1" = "--output" ] && out="$2"; shift; done; printf '%%PDF-1.7 fake' > "$out"`+"\n")
	fakeTool(t, dir, "weasyprint", WeasyPrintEnv, "exit 0\n")
	if _, err := Render("# Plain Report\n\nOne paragraph\\.\n", "pandoc", Options{}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	args, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(args), "--css pandoc.css") {
		t.Fatalf("pandoc arguments lack the print stylesheet: %s", args)
	}
	if strings.Contains(string(args), "document-css") {
		t.Fatalf("pandoc arguments set document-css: %s", args)
	}
}

func TestMarkdownWithImages(t *testing.T) {
	got := markdownWithImages(sampleMarkdown, []diagram{{Image: "diagram-1.svg"}})
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
	golden, err := os.ReadFile(filepath.Join("..", "docrender", "testdata", "telescope_report.golden.md"))
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
	page := documentHTML(blocks, artwork{diagrams: []diagram{{Image: "diagram-1.svg"}, {Image: "diagram-2.svg"}}}, Options{TitlePage: true, TOC: true, NumberSections: true})
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

// TestParseStateReportGolden lays out the state-and-event report: three captioned
// tables and two lists of row summaries, unit brackets escaped, state paths literal.
func TestParseStateReportGolden(t *testing.T) {
	golden, err := os.ReadFile(filepath.Join("..", "docrender", "testdata", "state_report.golden.md"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	blocks, err := parseBlocks(string(golden))
	if err != nil {
		t.Fatalf("parseBlocks: %v", err)
	}
	var tables, captions, lists int
	for _, b := range blocks {
		switch b.Kind {
		case blockTable:
			tables++
		case blockCaption:
			captions++
		case blockList:
			lists++
		}
	}
	if tables != 3 || captions != 3 || lists != 2 {
		t.Fatalf("got %d tables, %d captions, %d lists; want 3, 3, 2", tables, captions, lists)
	}
	page := documentHTML(blocks, artwork{}, Options{})
	for _, want := range []string{
		`<p class="caption"><em>Active states of every lamp</em></p>`,
		"<td>on.dim</td>",
		"<td>1 [s]</td>",
		"<td>level = 3</td>",
		"<li>t=0 lamp1.lp: enter: on</li>",
		"<li>lamp1.lp in on.fast</li>",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("state report markup missing from HTML: %q\n%s", want, page)
		}
	}
	for _, stray := range []string{`\[`, "<!-- caption -->"} {
		if strings.Contains(page, stray) {
			t.Errorf("page leaks %q:\n%s", stray, page)
		}
	}
}
