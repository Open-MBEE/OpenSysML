package docpdf

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

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
	document := plainDocument(t)
	for _, engine := range Engines() {
		_, err := Render(document, engine, Options{})
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

// TestRenderUnsupportedOptionBeforeTools checks an option pandoc cannot take
// is reported as such even when pandoc itself is not installed.
func TestRenderUnsupportedOptionBeforeTools(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv(PandocEnv, "")
	document := plainDocument(t)
	for option, opts := range map[string]Options{
		"-html-theme":          {Theme: "report"},
		"-html-no-default-css": {NoDefaultStylesheet: true},
	} {
		_, err := Render(document, "pandoc", opts)
		var docErr *Error
		if !errors.As(err, &docErr) || docErr.Kind != ErrorUnsupportedOption {
			t.Fatalf("%s: got %v, want ErrorUnsupportedOption", option, err)
		}
		if docErr.Option != option || !strings.Contains(err.Error(), option) {
			t.Fatalf("%s: error names %q: %v", option, docErr.Option, err)
		}
	}
}

// TestRenderMalformedStylesheetEveryEngine checks a reader stylesheet the HTML
// backend rejects is rejected ahead of every engine, tools present or not,
// while an empty inline sheet stays a stylesheet for the Markdown-reading one.
func TestRenderMalformedStylesheetEveryEngine(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	for _, tool := range []string{PandocEnv, WeasyPrintEnv, PrinceEnv} {
		t.Setenv(tool, "")
	}
	document := plainDocument(t)
	malformed := map[docrender.ErrorKind]docrender.Stylesheet{
		docrender.ErrorAmbiguousStylesheet: {Content: "body { color: red }", Href: "theme.css"},
		docrender.ErrorEmptyStylesheet:     {},
		docrender.ErrorUnsafeStylesheet:    docrender.InlineStylesheet("</style><script>"),
	}
	for _, engine := range Engines() {
		for kind, sheet := range malformed {
			_, err := Render(document, engine, Options{Stylesheets: []docrender.Stylesheet{sheet}})
			var renderErr *docrender.Error
			if !errors.As(err, &renderErr) || renderErr.Kind != kind {
				t.Fatalf("engine %s, %s: got %v, want %s", engine, kind, err, kind)
			}
		}
	}

	fakeTool(t, dir, "pandoc", PandocEnv,
		`echo "$@" > "`+dir+`/args"; out=""; while [ $# -gt 0 ]; do [ "$1" = "--output" ] && out="$2"; shift; done; printf '%%PDF-1.7 fake' > "$out"`+"\n")
	fakeTool(t, dir, "weasyprint", WeasyPrintEnv, "exit 0\n")
	if _, err := Render(document, "pandoc", Options{Stylesheets: []docrender.Stylesheet{docrender.InlineStylesheet("")}}); err != nil {
		t.Fatalf("empty inline stylesheet for pandoc: %v", err)
	}
	args, err := os.ReadFile(filepath.Join(dir, "args"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(args), "--include-in-header reader-stylesheets.html") {
		t.Fatalf("pandoc arguments lack the empty reader stylesheet: %s", args)
	}
}

// TestRenderBaseDirEveryEngine checks each engine is handed BaseDir as the
// base URL for the document's relative references, so a reader sheet's
// url() and @import resolve beside the PDF as they do beside an HTML page,
// while the working directory's own files are referenced by absolute file URL.
func TestRenderBaseDirEveryEngine(t *testing.T) {
	dir := t.TempDir()
	fakeMermaid(t, dir)
	out := filepath.Join(dir, "out")
	if err := os.Mkdir(out, 0o750); err != nil {
		t.Fatal(err)
	}
	base := "file://" + filepath.ToSlash(out) + "/"
	sheets := []docrender.Stylesheet{
		docrender.InlineStylesheet("@import url(\"beside.css\");\n.sysml-title::before { content: url(mark.png) }\n"),
		docrender.LinkedStylesheet("https://example.test/site.css"),
	}
	document := telescopeDocument(t)
	capture := filepath.Join(dir, "capture")
	record := `echo "$@" > "` + capture + `.args"; pwd > "` + capture + `.dir"; cp "$1" "` + capture + `.in"; `
	writePDF := `out="$2"; while [ $# -gt 0 ]; do case "$1" in --output|-o) out="$2";; esac; shift; done; printf '%%PDF-1.7 fake' > "$out"` + "\n"
	fakeTool(t, dir, "weasyprint", WeasyPrintEnv, record+writePDF)
	fakeTool(t, dir, "prince", PrinceEnv, record+writePDF)
	fakeTool(t, dir, "pandoc", PandocEnv, record+`cp "$(pwd)/reader-stylesheets.html" "`+capture+`.head"; `+writePDF)

	for _, engine := range []string{"weasyprint", "prince"} {
		if _, err := Render(document, engine, Options{Stylesheets: sheets, BaseDir: out}); err != nil {
			t.Fatalf("%s: Render: %v", engine, err)
		}
		args, err := os.ReadFile(capture + ".args")
		if err != nil {
			t.Fatal(err)
		}
		want := "document.html document.pdf --base-url " + base
		if engine == "prince" {
			want = "document.html -o document.pdf --baseurl=" + base
		}
		if strings.TrimSpace(string(args)) != want {
			t.Errorf("%s arguments: got %q, want %q", engine, strings.TrimSpace(string(args)), want)
		}
		page, err := os.ReadFile(capture + ".in")
		if err != nil {
			t.Fatal(err)
		}
		images := fileRefs(captureDir(t, capture), []string{"diagram-1.svg", "diagram-2.svg"})
		for _, want := range []string{
			docrender.StylesheetMarkup(sheets),
			`<img src="` + images[0] + `" alt="Imaging chain interconnection">`,
			`<img src="` + images[1] + `" alt="Observatory states, left to right">`,
		} {
			if !strings.Contains(string(page), want) {
				t.Errorf("%s page lacks %q:\n%s", engine, want, page)
			}
		}
		if strings.Contains(string(page), filepath.ToSlash(out)) {
			t.Errorf("%s page references the output directory:\n%s", engine, page)
		}
	}

	if _, err := Render(document, "pandoc", Options{Stylesheets: sheets, BaseDir: out}); err != nil {
		t.Fatalf("pandoc: Render: %v", err)
	}
	args, err := os.ReadFile(capture + ".args")
	if err != nil {
		t.Fatal(err)
	}
	work := captureDir(t, capture)
	for _, want := range []string{
		"--css " + fileURL(filepath.Join(work, "pandoc.css")),
		"--resource-path . --resource-path " + out + " ",
		"--pdf-engine-opt=--base-url=" + base,
		"--include-in-header reader-stylesheets.html",
	} {
		if !strings.Contains(string(args), want) {
			t.Errorf("pandoc arguments lack %q: %s", want, args)
		}
	}
	if strings.Contains(string(args), "--css reader") {
		t.Errorf("pandoc links a reader sheet as a file, whose url() would resolve beside it: %s", args)
	}
	head, err := os.ReadFile(capture + ".head")
	if err != nil {
		t.Fatal(err)
	}
	if string(head) != docrender.StylesheetMarkup(sheets) {
		t.Errorf("pandoc header include is not the reader's sheets as the HTML backend attaches them:\n%s", head)
	}

	if _, err := Render(document, "weasyprint", Options{}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if args, err = os.ReadFile(capture + ".args"); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if want := "--base-url file://" + filepath.ToSlash(cwd) + "/"; !strings.HasSuffix(strings.TrimSpace(string(args)), want) {
		t.Errorf("BaseDir unset: got %q, want the current directory %q", args, want)
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

// fakeMermaid installs an mmdc stand-in that writes an empty SVG.
func fakeMermaid(t *testing.T, dir string) {
	t.Helper()
	fakeTool(t, dir, "mmdc", MermaidEnv, `out=""
while [ $# -gt 0 ]; do [ "$1" = "--output" ] && out="$2"; shift; done
printf '<svg xmlns="http://www.w3.org/2000/svg"/>' > "$out"
`)
}

// captureWeasyPrint installs a weasyprint stand-in that keeps the page it is
// handed and a listing of the working directory beside capture, and writes a
// fake PDF; the page and listing are returned by readCapture.
func captureWeasyPrint(t *testing.T, dir string) string {
	t.Helper()
	capture := filepath.Join(dir, "capture")
	fakeTool(t, dir, "weasyprint", WeasyPrintEnv,
		`cp "$1" "`+capture+`.html"; ls -R "$(dirname "$1")" > "`+capture+`.ls"; pwd > "`+capture+`.dir"; echo "$@" > "`+capture+`.args"; printf '%%PDF-1.7 fake' > "$2"`+"\n")
	return capture
}

// captureDir is the working directory a capturing fake tool ran in.
func captureDir(t *testing.T, capture string) string {
	t.Helper()
	dir, err := os.ReadFile(capture + ".dir")
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(dir))
}

func readCapture(t *testing.T, capture string) (page, listing string) {
	t.Helper()
	html, err := os.ReadFile(capture + ".html")
	if err != nil {
		t.Fatal(err)
	}
	ls, err := os.ReadFile(capture + ".ls")
	if err != nil {
		t.Fatal(err)
	}
	return string(html), string(ls)
}

func TestRenderWithFakeConverter(t *testing.T) {
	dir := t.TempDir()
	capture := captureWeasyPrint(t, dir)
	pdf, err := Render(plainDocument(t), "weasyprint", Options{TOC: true, TitlePage: true, NumberSections: true})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.HasPrefix(string(pdf), "%PDF-") {
		t.Fatalf("output is no PDF: %q", pdf)
	}
	page, _ := readCapture(t, capture)
	for _, want := range []string{
		"<!DOCTYPE html>",
		`<article class="sysml-document"`,
		`<header class="sysml-title-page">`,
		`<p class="sysml-paragraph" data-content="paragraph" data-name="intro">One paragraph.</p>`,
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("page missing %q:\n%s", want, page)
		}
	}
}

// TestRenderHTMLIsTheBackendsPage checks that an HTML-reading engine is
// handed the HTML backend's markup — its vocabulary, its default sheet — and
// the print stylesheet over it, with no markup of the PDF backend's own.
func TestRenderHTMLIsTheBackendsPage(t *testing.T) {
	dir := t.TempDir()
	fakeMermaid(t, dir)
	capture := captureWeasyPrint(t, dir)
	document := telescopeDocument(t)
	if _, err := Render(document, "weasyprint", Options{TitlePage: true, TOC: true, NumberSections: true}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	page, listing := readCapture(t, capture)
	images := fileRefs(captureDir(t, capture), []string{"diagram-1.svg", "diagram-2.svg"})
	want, err := docrender.HTML(document, docrender.HTMLOptions{
		TitlePage: true, TOC: true, NumberSections: true,
		Stylesheets:   []docrender.Stylesheet{docrender.InlineStylesheet(PrintStylesheet)},
		DiagramImages: images,
	})
	if err != nil {
		t.Fatal(err)
	}
	if page != want {
		t.Fatalf("page differs from the HTML backend's with the print stylesheet:\n%s", page)
	}
	for _, want := range []string{
		`<img src="` + images[0] + `" alt="Imaging chain interconnection">`,
		`<img src="` + images[1] + `" alt="Observatory states, left to right">`,
		`<dt class="sysml-term">M3|*</dt>`,
		`<caption class="sysml-caption">All subsystems by mass</caption>`,
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("page missing %q:\n%s", want, page)
		}
	}
	for _, stray := range []string{`<pre class="mermaid">`, "flowchart", `class="caption"`, " style=\""} {
		if strings.Contains(page, stray) {
			t.Fatalf("page carries %q:\n%s", stray, page)
		}
	}
	for _, want := range []string{"diagram-1.mmd", "diagram-1.svg", "diagram-2.svg", "mermaid-config.json"} {
		if !strings.Contains(listing, want) {
			t.Fatalf("working directory lacks %s:\n%s", want, listing)
		}
	}
	if strings.Contains(listing, "diagram-3") {
		t.Fatalf("more diagrams drawn than the document has:\n%s", listing)
	}
}

// TestPrintStylesheetContract holds the print stylesheet to the HTML
// backend's override contract: a later cascade layer, declared before use,
// every value a --sysml-* token, and the reader's sheets after it unlayered.
func TestPrintStylesheetContract(t *testing.T) {
	sheet := PrintStylesheet
	layer := "@layer opensysml-print"
	if !strings.HasPrefix(strings.TrimSpace(stripCSSComments(sheet)), layer+";") {
		t.Fatalf("print stylesheet does not declare its layer first:\n%.200s", sheet)
	}
	if !strings.Contains(sheet, layer+" {") {
		t.Fatal("print stylesheet has no layered block")
	}
	if !strings.Contains(sheet, "@page {") {
		t.Fatal("print stylesheet sets no page")
	}
	if strings.Contains(sheet, "!important") {
		t.Fatal("print stylesheet uses !important")
	}
	tokenless := regexp.MustCompile(`(?m)^\s{4,}([a-z-]+):\s*([^;]+);`)
	for _, m := range tokenless.FindAllStringSubmatch(sheet, -1) {
		property, value := m[1], m[2]
		if strings.HasPrefix(property, "--") || strings.Contains(value, "var(--sysml-") {
			continue
		}
		switch property {
		case "content", "display", "flex-direction", "justify-content", "text-align", "box-sizing",
			"break-after", "break-before", "break-inside", "page-break-after", "page-break-before", "page-break-inside",
			"white-space", "overflow-wrap", "word-break", "orphans", "widows", "max-width", "height", "margin", "padding",
			"border-collapse", "size", "overflow-x", "line-height", "text-decoration":
			continue
		}
		t.Errorf("declaration %s: %s resolves through no --sysml-* token", property, value)
	}
	page, err := docrender.HTML(plainDocument(t), htmlOptions(Options{
		Stylesheets: []docrender.Stylesheet{docrender.InlineStylesheet(".sysml-document { color: red }")},
	}, t.TempDir(), nil, formulas{}))
	if err != nil {
		t.Fatal(err)
	}
	def := strings.Index(page, "@layer opensysml {")
	print := strings.Index(page, "@layer opensysml-print {")
	reader := strings.Index(page, ".sysml-document { color: red }")
	if def < 0 || print < def || reader < print {
		t.Fatalf("stylesheet order default=%d print=%d reader=%d:\n%s", def, print, reader, page)
	}
	if strings.Count(page, "@layer opensysml-print {") != 1 {
		t.Fatal("print stylesheet inlined more than once")
	}
	if strings.Contains(page[reader:], "@layer") {
		t.Fatal("the reader's stylesheet is layered")
	}
}

// TestPrintStylesheetKeepsTablesWithinThePage checks the page handed to the
// engines sizes a table to the text width and wraps a cell's long unbreakable
// tokens, so a list of qualified names cannot push columns off the page, and
// keeps each row on one page.
func TestPrintStylesheetKeepsTablesWithinThePage(t *testing.T) {
	page, err := docrender.HTML(plainDocument(t), htmlOptions(Options{}, t.TempDir(), nil, formulas{}))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"--sysml-table-width: 100%;",
		"width: var(--sysml-table-width);",
		".sysml-document .sysml-table th,\n  .sysml-document .sysml-table td {\n    overflow-wrap: anywhere;",
		".sysml-document .sysml-table tr,",
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("page lacks %q: a wide cell would run off the page or a row split across pages", want)
		}
	}
	rows := strings.Index(page, ".sysml-document .sysml-table tr,")
	if avoid := strings.Index(page[rows:], "break-inside: avoid;"); avoid < 0 || avoid > 200 {
		t.Fatalf("table rows are not kept on one page:\n%s", page[rows:rows+300])
	}
}

// TestPandocStylesheetKeepsTablesWithinThePage checks pandoc's stylesheet
// holds the same table contract as the print stylesheet: full text width,
// wrapped long tokens, and rows kept on one page rather than a whole table.
func TestPandocStylesheetKeepsTablesWithinThePage(t *testing.T) {
	css := stripCSSComments(pandocStylesheet)
	for _, want := range []string{
		"table {\n  border-collapse: collapse;\n  margin: 0.8em 0;\n  width: 100%;\n}",
		"tr {\n  break-inside: avoid;\n  page-break-inside: avoid;\n}",
		"th, td {\n  border: 0.5pt solid #666666;\n  padding: 0.3em 0.6em;\n  text-align: left;\n  overflow-wrap: anywhere;\n}",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("pandoc stylesheet lacks %q: a wide cell would run off the page or a row split across pages", want)
		}
	}
}

func stripCSSComments(css string) string {
	return regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(css, "")
}

// TestRenderReaderStylesheets checks -html-css reaches the PDF page as it
// reaches the HTML form: inline sheets inlined, linked sheets linked, in
// order, after the print stylesheet; and the theme and default sheet options
// shape the page as they do for HTML.
func TestRenderReaderStylesheets(t *testing.T) {
	dir := t.TempDir()
	capture := captureWeasyPrint(t, dir)
	if _, err := Render(plainDocument(t), "weasyprint", Options{
		Theme: "report",
		Stylesheets: []docrender.Stylesheet{
			docrender.InlineStylesheet(".sysml-document { --sysml-accent: teal }"),
			docrender.LinkedStylesheet("https://example.com/house.css"),
		},
	}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	page, _ := readCapture(t, capture)
	order := []string{"@layer opensysml {", "@layer opensysml-print {", ".sysml-document { --sysml-accent: teal }", `<link rel="stylesheet" href="https://example.com/house.css">`}
	last := -1
	for _, want := range order {
		at := strings.Index(page, want)
		if at <= last {
			t.Fatalf("%q out of order (at %d, after %d):\n%s", want, at, last, page)
		}
		last = at
	}
	theme, err := docrender.ThemeStylesheet("report")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(page, theme) {
		t.Fatal("theme not laid under the print stylesheet")
	}

	if _, err := Render(plainDocument(t), "weasyprint", Options{
		NoDefaultStylesheet: true,
		Stylesheets:         []docrender.Stylesheet{docrender.InlineStylesheet("@page { size: letter }")},
	}); err != nil {
		t.Fatalf("Render without default stylesheet: %v", err)
	}
	page, _ = readCapture(t, capture)
	if strings.Contains(page, "@layer") || !strings.Contains(page, "@page { size: letter }") {
		t.Fatalf("page without the default stylesheet:\n%s", page)
	}
}

// TestRenderDiagramFormKeepsSourceForOtherForms checks that a document whose
// diagrams are asked for as DOT or PlantUML needs no diagram tool and shows
// each as source, since the PDF backend draws only Mermaid.
func TestRenderDiagramFormKeepsSourceForOtherForms(t *testing.T) {
	dir := t.TempDir()
	capture := captureWeasyPrint(t, dir)
	t.Setenv(MermaidEnv, filepath.Join(dir, "no-mmdc-here"))
	for _, form := range []view.Form{view.FormDot, view.FormPlantUML} {
		if _, err := Render(telescopeDocument(t), "weasyprint", Options{DiagramForm: form}); err != nil {
			t.Fatalf("Render %s: %v", form, err)
		}
		page, listing := readCapture(t, capture)
		if strings.Count(page, `<pre class="`+string(form)+`">`) != 2 || strings.Contains(page, "<img") {
			t.Fatalf("%s page:\n%s", form, page)
		}
		if strings.Contains(listing, "diagram-") {
			t.Fatalf("%s drew a diagram:\n%s", form, listing)
		}
	}
}

func TestRenderDiagramToolMissing(t *testing.T) {
	dir := t.TempDir()
	fakeTool(t, dir, "weasyprint", WeasyPrintEnv, `printf '%%PDF-1.7 fake' > "$2"`+"\n")
	t.Setenv(MermaidEnv, filepath.Join(dir, "no-mmdc-here"))
	_, err := Render(telescopeDocument(t), "weasyprint", Options{})
	var docErr *Error
	if !errors.As(err, &docErr) || docErr.Kind != ErrorToolMissing {
		t.Fatalf("got %v, want ErrorToolMissing", err)
	}
	if !strings.Contains(err.Error(), "diagrams") || !strings.Contains(err.Error(), "mmdc") {
		t.Fatalf("diagram-tool message: %q", err.Error())
	}
}

func TestRenderDiagramToolWritesNothing(t *testing.T) {
	dir := t.TempDir()
	fakeTool(t, dir, "mmdc", MermaidEnv, "exit 0\n")
	fakeTool(t, dir, "weasyprint", WeasyPrintEnv, `printf '%%PDF-1.7 fake' > "$2"`+"\n")
	_, err := Render(telescopeDocument(t), "weasyprint", Options{})
	var docErr *Error
	if !errors.As(err, &docErr) || docErr.Kind != ErrorToolFailed || docErr.Tool != "mmdc" {
		t.Fatalf("got %v, want ErrorToolFailed for mmdc", err)
	}
	if !strings.Contains(docErr.Detail, "wrote no SVG for diagram-1.mmd") {
		t.Fatalf("detail %q", docErr.Detail)
	}
}

func TestRenderToolFailed(t *testing.T) {
	dir := t.TempDir()
	fakeTool(t, dir, "weasyprint", WeasyPrintEnv, `echo "boom: bad input" >&2
exit 3
`)
	_, err := Render(plainDocument(t), "weasyprint", Options{})
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
	_, err := Render(plainDocument(t), "weasyprint", Options{})
	var docErr *Error
	if !errors.As(err, &docErr) || docErr.Kind != ErrorNoPDF {
		t.Fatalf("got %v, want ErrorNoPDF", err)
	}
}

func TestRenderNilDocument(t *testing.T) {
	dir := t.TempDir()
	fakeTool(t, dir, "weasyprint", WeasyPrintEnv, "exit 0\n")
	_, err := Render(nil, "weasyprint", Options{})
	var docErr *docrender.Error
	if !errors.As(err, &docErr) || docErr.Kind != docrender.ErrorNilDocument {
		t.Fatalf("got %v, want docrender.ErrorNilDocument", err)
	}
}

// TestRenderForPandoc checks the Markdown-reading path: the Markdown
// backend's text handed over as written, with pandoc's own stylesheet and a
// Lua filter that swaps the drawn diagrams in for their fences.
func TestRenderForPandoc(t *testing.T) {
	dir := t.TempDir()
	fakeMermaid(t, dir)
	capture := filepath.Join(dir, "capture")
	fakeTool(t, dir, "pandoc", PandocEnv,
		`echo "$@" > "`+capture+`.args"; pwd > "`+capture+`.dir"; cp "$1" "`+capture+`.md"; cp "$(dirname "$1")/artwork.lua" "`+capture+`.lua"; cp "$(dirname "$1")/pandoc.css" "`+capture+`.css"; out=""; while [ $# -gt 0 ]; do [ "$1" = "--output" ] && out="$2"; shift; done; printf '%%PDF-1.7 fake' > "$out"`+"\n")
	fakeTool(t, dir, "weasyprint", WeasyPrintEnv, "exit 0\n")
	document := telescopeDocument(t)
	if _, err := Render(document, "pandoc", Options{TitlePage: true, TOC: true, NumberSections: true}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	args, err := os.ReadFile(capture + ".args")
	if err != nil {
		t.Fatal(err)
	}
	work := captureDir(t, capture)
	for _, want := range []string{"--css " + fileURL(filepath.Join(work, "pandoc.css")), "--lua-filter artwork.lua", "--toc", "--number-sections", "--pdf-engine "} {
		if !strings.Contains(string(args), want) {
			t.Fatalf("pandoc arguments lack %q: %s", want, args)
		}
	}
	md, err := os.ReadFile(capture + ".md")
	if err != nil {
		t.Fatal(err)
	}
	want, err := docrender.Markdown(document, docrender.MarkdownOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if string(md) != want {
		t.Fatalf("pandoc is handed other than the Markdown backend's text:\n%s", md)
	}
	filter, err := os.ReadFile(capture + ".lua")
	if err != nil {
		t.Fatal(err)
	}
	images := fileRefs(work, []string{"diagram-1.svg", "diagram-2.svg"})
	for _, want := range []string{`local form = "mermaid"`, `local images = {"` + images[0] + `", "` + images[1] + `"}`, "local math = {\n}"} {
		if !strings.Contains(string(filter), want) {
			t.Fatalf("filter lacks %q:\n%s", want, filter)
		}
	}
	css, err := os.ReadFile(capture + ".css")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(css), "header#title-block-header { page-break-after: always;") || strings.Contains(stripCSSComments(string(css)), ".sysml-") {
		t.Fatalf("pandoc stylesheet:\n%s", css)
	}
}

// TestRenderForPandocKeepsOtherFormsUnderNotice checks a document whose
// diagrams are asked for as DOT or PlantUML is handed to pandoc with a filter
// that draws nothing and sets the notice the print stylesheet sets over the
// HTML backend's page, so both converter inputs say the same.
func TestRenderForPandocKeepsOtherFormsUnderNotice(t *testing.T) {
	dir := t.TempDir()
	capture := filepath.Join(dir, "capture")
	fakeTool(t, dir, "pandoc", PandocEnv,
		`cp "$(dirname "$1")/artwork.lua" "`+capture+`.lua"; out=""; while [ $# -gt 0 ]; do [ "$1" = "--output" ] && out="$2"; shift; done; printf '%%PDF-1.7 fake' > "$out"`+"\n")
	fakeTool(t, dir, "weasyprint", WeasyPrintEnv, "exit 0\n")
	t.Setenv(MermaidEnv, filepath.Join(dir, "no-mmdc-here"))
	for form, notice := range map[view.Form]string{view.FormDot: dotNotice, view.FormPlantUML: plantumlNotice} {
		if _, err := Render(telescopeDocument(t), "pandoc", Options{DiagramForm: form}); err != nil {
			t.Fatalf("Render %s: %v", form, err)
		}
		filter, err := os.ReadFile(capture + ".lua")
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{`local form = "` + string(form) + `"`, `local images = {"", ""}`, luaString(notice)} {
			if !strings.Contains(string(filter), want) {
				t.Fatalf("%s filter lacks %q:\n%s", form, want, filter)
			}
		}
		if !strings.Contains(PrintStylesheet, `content: "`+notice+`";`) {
			t.Fatalf("print stylesheet does not set the %s notice %q", form, notice)
		}
	}
}

// TestRenderForPandocWithoutArtwork checks a document with nothing to draw or
// typeset is handed to pandoc without a filter.
func TestRenderForPandocWithoutArtwork(t *testing.T) {
	dir := t.TempDir()
	capture := filepath.Join(dir, "capture")
	fakeTool(t, dir, "pandoc", PandocEnv,
		`echo "$@" > "`+capture+`.args"; ls "$(dirname "$1")" > "`+capture+`.ls"; out=""; while [ $# -gt 0 ]; do [ "$1" = "--output" ] && out="$2"; shift; done; printf '%%PDF-1.7 fake' > "$out"`+"\n")
	fakeTool(t, dir, "weasyprint", WeasyPrintEnv, "exit 0\n")
	if _, err := Render(plainDocument(t), "pandoc", Options{}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	args, err := os.ReadFile(capture + ".args")
	if err != nil {
		t.Fatal(err)
	}
	listing, err := os.ReadFile(capture + ".ls")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(args), "--lua-filter") || strings.Contains(string(listing), "artwork.lua") {
		t.Fatalf("filter written for a document without artwork:\n%s\n%s", args, listing)
	}
}

func TestLuaString(t *testing.T) {
	cases := map[string]string{
		``:                       `""`,
		`plain`:                  `"plain"`,
		`a "quoted" \ backslash`: `"a \"quoted\" \\ backslash"`,
		"two\nlines\r":           `"two\nlines\r"`,
		"tab\tbell\x07":          `"tab\009bell\007"`,
		`\frac{a}{b}`:            `"\\frac{a}{b}"`,
	}
	for in, want := range cases {
		if got := luaString(in); got != want {
			t.Errorf("luaString(%q) = %s, want %s", in, got, want)
		}
	}
}

// TestRenderStateReportPage hands the engines the state-and-event report as the
// HTML backend's page: three captioned tables whose cells carry each state's
// machine and path or each event's kind and instant, and two lists of summaries.
func TestRenderStateReportPage(t *testing.T) {
	dir := t.TempDir()
	capture := captureWeasyPrint(t, dir)
	document := stateDocument(t)
	if _, err := Render(document, "weasyprint", Options{TOC: true}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	page, _ := readCapture(t, capture)
	want, err := docrender.HTML(document, docrender.HTMLOptions{
		TOC:         true,
		Stylesheets: []docrender.Stylesheet{docrender.InlineStylesheet(PrintStylesheet)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if page != want {
		t.Fatalf("page differs from the HTML backend's with the print stylesheet:\n%s", page)
	}
	if got := strings.Count(page, `<caption class="sysml-caption">`); got != 3 {
		t.Errorf("page has %d captions, want 3:\n%s", got, page)
	}
	if got := strings.Count(page, `<ul class="sysml-list"`); got != 2 {
		t.Errorf("page has %d lists, want 2:\n%s", got, page)
	}
	for _, want := range []string{
		`<caption class="sysml-caption">Active states of every lamp</caption>`,
		`<tr class="sysml-row" data-object="#1" data-machine="lp" data-state="on.dim" data-region="light" data-element="Lamps::LampMachine::on::light::dim" data-element-kind="stateUsage">`,
		`<tr class="sysml-row" data-object="#1" data-event-kind="accept" data-time="1" data-element="Lamps::LampMachine" data-element-kind="stateDef">`,
		`<span class="sysml-value" data-value-kind="quantity" data-magnitude="1" data-unit="s">1 [s]</span>`,
		`<span class="sysml-value" data-value-kind="string">level = 3</span>`,
		`<li class="sysml-item" data-object="#1" data-event-kind="entry" data-time="0" data-element="Lamps::LampMachine" data-element-kind="stateDef">t=0 lamp1.lp: enter: on</li>`,
		`<li class="sysml-item" data-object="#1" data-machine="lp" data-state="on.fast" data-region="fan" data-element="Lamps::LampMachine::on::fan::fast" data-element-kind="stateUsage">lamp1.lp in on.fast</li>`,
		"@layer opensysml-print",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("state report markup missing from the page: %q\n%s", want, page)
		}
	}
	for _, stray := range []string{`\[`, "<!-- caption -->", `class="caption"`, " style=\""} {
		if strings.Contains(page, stray) {
			t.Errorf("page leaks %q:\n%s", stray, page)
		}
	}
}
