package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docpdf"
)

// fakePDFTool writes an executable shell script into dir and returns its path.
func fakePDFTool(t *testing.T, dir, name, script string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake tool scripts need a POSIX shell")
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o700); err != nil { // #nosec G306 -- the fake tool must be executable
		t.Fatal(err)
	}
	return path
}

// TestRenderDocumentPDF renders the committed telescope fixture to PDF
// through fake converter tools, checking the full CLI path: mermaid
// pre-rendering, the HTML backend's page under the print stylesheet,
// converter invocation, and the artifact.
func TestRenderDocumentPDF(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	seen := filepath.Join(dir, "input-seen.html")
	weasyprint := fakePDFTool(t, dir, "weasyprint", `cp "$1" `+seen+`
printf '%%PDF-1.7 fake' > "$2"
`)
	mmdc := fakePDFTool(t, dir, "mmdc", `out=""
while [ $# -gt 0 ]; do [ "$1" = "--output" ] && out="$2"; shift; done
printf '<svg xmlns="http://www.w3.org/2000/svg"/>' > "$out"
`)
	fixture := filepath.Join("..", "..", "internal", "doc", "docrender", "testdata", "telescope_report.sysml")
	out := filepath.Join(dir, "report.pdf")

	cmd := exec.Command(binary, fixture, "-render-document", "Observatory::MassReport",
		"-doc-form", "pdf", "-pdf-title-page", "-pdf-toc", "-pdf-number-sections", "-o", out)
	cmd.Env = append(os.Environ(), docpdf.WeasyPrintEnv+"="+weasyprint, docpdf.MermaidEnv+"="+mmdc)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("render: %v\n%s", err, output)
	}
	pdf, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(pdf), "%PDF-") {
		t.Errorf("artifact is no PDF: %.16q", pdf)
	}
	page, err := os.ReadFile(seen)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"<title>Telescope Mass Report</title>",
		`<article class="sysml-document"`,
		`<header class="sysml-title-page">`,
		`<nav class="sysml-toc"`,
		`<span class="sysml-section-number">`,
		"@layer opensysml {",
		"@layer opensysml-print {",
		`<img src="diagram-1.svg"`,
		`<img src="diagram-2.svg"`,
		`<figcaption class="sysml-caption">`,
	} {
		if !strings.Contains(string(page), want) {
			t.Errorf("converter input misses %q", want)
		}
	}
	if strings.Contains(string(page), "<!-- caption -->") || strings.Contains(string(page), " style=") {
		t.Errorf("converter input carries a caption marker or inline style:\n%s", page)
	}
}

// TestRenderDocumentPDFStylesheets checks the stylesheet options reach the
// PDF as they reach an HTML page: a -html-css file lands after the print
// layer, a theme layers over the default sheet, and -html-no-default-css
// leaves both layered sheets out, so the reader's sheet styles the page alone.
func TestRenderDocumentPDFStylesheets(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	seen := filepath.Join(dir, "input-seen.html")
	weasyprint := fakePDFTool(t, dir, "weasyprint", `cp "$1" `+seen+`
printf '%%PDF-1.7 fake' > "$2"
`)
	mmdc := fakePDFTool(t, dir, "mmdc", `out=""
while [ $# -gt 0 ]; do [ "$1" = "--output" ] && out="$2"; shift; done
printf '<svg xmlns="http://www.w3.org/2000/svg"/>' > "$out"
`)
	css := filepath.Join(dir, "theme.css")
	if err := os.WriteFile(css, []byte(".sysml-document { color: rebeccapurple; }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join("..", "..", "internal", "core", "docrender", "testdata", "telescope_report.sysml")
	out := filepath.Join(dir, "report.pdf")
	env := append(os.Environ(), docpdf.WeasyPrintEnv+"="+weasyprint, docpdf.MermaidEnv+"="+mmdc)

	cmd := exec.Command(binary, fixture, "-render-document", "Observatory::MassReport", "-doc-form", "pdf",
		"-html-theme", "report", "-html-css", css, "-html-css", "https://example.test/site.css", "-o", out)
	cmd.Env = env
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("render: %v\n%s", err, output)
	}
	page, err := os.ReadFile(seen)
	if err != nil {
		t.Fatal(err)
	}
	def := strings.Index(string(page), "@layer opensysml {")
	print := strings.Index(string(page), "@layer opensysml-print {")
	reader := strings.Index(string(page), "rebeccapurple")
	link := strings.Index(string(page), `<link rel="stylesheet" href="https://example.test/site.css">`)
	if def < 0 || print < def || reader < print || link < reader {
		t.Errorf("stylesheet order default=%d print=%d reader=%d link=%d:\n%s", def, print, reader, link, page)
	}
	if !strings.Contains(string(page), "/* report:") {
		t.Errorf("converter input misses the report theme:\n%s", page)
	}

	cmd = exec.Command(binary, fixture, "-render-document", "Observatory::MassReport", "-doc-form", "pdf",
		"-html-no-default-css", "-html-css", css, "-o", out)
	cmd.Env = env
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("render: %v\n%s", err, output)
	}
	if page, err = os.ReadFile(seen); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(page), "@layer") {
		t.Errorf("converter input carries a layered sheet under -html-no-default-css:\n%s", page)
	}
	if !strings.Contains(string(page), "rebeccapurple") {
		t.Errorf("converter input misses the reader's sheet:\n%s", page)
	}
}

// TestRenderDocumentPDFPandocStylesheets checks the pandoc engine, which
// writes its own HTML, takes -html-css sheets and refuses the options that
// shape the HTML backend's page.
func TestRenderDocumentPDFPandocStylesheets(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	seen := filepath.Join(dir, "args-seen.txt")
	pandoc := fakePDFTool(t, dir, "pandoc", `printf '%s\n' "$@" > `+seen+`
out=""
while [ $# -gt 0 ]; do [ "$1" = "--output" ] && out="$2"; shift; done
printf '%%PDF-1.7 fake' > "$out"
`)
	weasyprint := fakePDFTool(t, dir, "weasyprint", `exit 1
`)
	mmdc := fakePDFTool(t, dir, "mmdc", `out=""
while [ $# -gt 0 ]; do [ "$1" = "--output" ] && out="$2"; shift; done
printf '<svg xmlns="http://www.w3.org/2000/svg"/>' > "$out"
`)
	css := filepath.Join(dir, "theme.css")
	if err := os.WriteFile(css, []byte(".sysml-document { color: rebeccapurple; }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join("..", "..", "internal", "core", "docrender", "testdata", "telescope_report.sysml")
	out := filepath.Join(dir, "report.pdf")
	env := append(os.Environ(), docpdf.PandocEnv+"="+pandoc, docpdf.WeasyPrintEnv+"="+weasyprint, docpdf.MermaidEnv+"="+mmdc)

	cmd := exec.Command(binary, fixture, "-render-document", "Observatory::MassReport", "-doc-form", "pdf",
		"-pdf-engine", "pandoc", "-html-css", css, "-html-css", "https://example.test/site.css", "-o", out)
	cmd.Env = env
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("render: %v\n%s", err, output)
	}
	args, err := os.ReadFile(seen)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--css\npandoc.css\n", "--css\nreader-1.css\n", "--css\nhttps://example.test/site.css\n"} {
		if !strings.Contains(string(args), want) {
			t.Errorf("pandoc arguments miss %q:\n%s", want, args)
		}
	}

	for _, flag := range []string{"-html-theme=report", "-html-no-default-css"} {
		cmd := exec.Command(binary, fixture, "-render-document", "Observatory::MassReport", "-doc-form", "pdf",
			"-pdf-engine", "pandoc", flag, "-o", out)
		cmd.Env = env
		output, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("pandoc accepted %s:\n%s", flag, output)
		}
		for _, want := range []string{strings.SplitN(flag, "=", 2)[0], "pandoc engine does not read", "-pdf-engine"} {
			if !strings.Contains(string(output), want) {
				t.Errorf("report for %s misses %q:\n%s", flag, want, output)
			}
		}
	}
}

// TestRenderDocumentPDFDiagramFormDot checks a PDF run under -diagram-form dot
// keeps every diagram as DOT source behind the notice and never runs mmdc.
func TestRenderDocumentPDFDiagramFormDot(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	seen := filepath.Join(dir, "input-seen.html")
	weasyprint := fakePDFTool(t, dir, "weasyprint", `cp "$1" `+seen+`
printf '%%PDF-1.7 fake' > "$2"
`)
	mmdc := fakePDFTool(t, dir, "mmdc", `echo "mmdc must not run" >&2; exit 1
`)
	fixture := filepath.Join("..", "..", "internal", "doc", "docrender", "testdata", "telescope_report.sysml")
	out := filepath.Join(dir, "report.pdf")

	cmd := exec.Command(binary, fixture, "-render-document", "Observatory::MassReport",
		"-doc-form", "pdf", "-diagram-form", "dot", "-o", out)
	cmd.Env = append(os.Environ(), docpdf.WeasyPrintEnv+"="+weasyprint, docpdf.MermaidEnv+"="+mmdc)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("render: %v\n%s", err, output)
	}
	page, err := os.ReadFile(seen)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(page), `<pre class="dot">`); got != 2 {
		t.Errorf("converter input carries %d DOT figures, want 2:\n%s", got, page)
	}
	for _, want := range []string{
		"Graphviz DOT, which the PDF backend does not draw",
		"digraph &#34;Observatory::interconnectView&#34; {",
		`<table class="sysml-table"`,
	} {
		if !strings.Contains(string(page), want) {
			t.Errorf("converter input misses %q:\n%s", want, page)
		}
	}
	if strings.Contains(string(page), "diagram-1.svg") {
		t.Errorf("a diagram was drawn by the Mermaid tool:\n%s", page)
	}
}

// TestRenderDocumentPDFDiagramFormPlantUML checks a PDF run under
// -diagram-form plantuml keeps every diagram as PlantUML source behind the
// notice and never runs mmdc.
func TestRenderDocumentPDFDiagramFormPlantUML(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	seen := filepath.Join(dir, "input-seen.html")
	weasyprint := fakePDFTool(t, dir, "weasyprint", `cp "$1" `+seen+`
printf '%%PDF-1.7 fake' > "$2"
`)
	mmdc := fakePDFTool(t, dir, "mmdc", `echo "mmdc must not run" >&2; exit 1
`)
	fixture := filepath.Join("..", "..", "internal", "doc", "docrender", "testdata", "telescope_report.sysml")
	out := filepath.Join(dir, "report.pdf")

	cmd := exec.Command(binary, fixture, "-render-document", "Observatory::MassReport",
		"-doc-form", "pdf", "-diagram-form", "plantuml", "-o", out)
	cmd.Env = append(os.Environ(), docpdf.WeasyPrintEnv+"="+weasyprint, docpdf.MermaidEnv+"="+mmdc)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("render: %v\n%s", err, output)
	}
	page, err := os.ReadFile(seen)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(page), `<pre class="plantuml">`); got != 2 {
		t.Errorf("converter input carries %d PlantUML figures, want 2:\n%s", got, page)
	}
	for _, want := range []string{
		"PlantUML, which the PDF backend does not draw",
		`<pre class="plantuml">@startuml` + "\n&#39; Observatory::interconnectView — interconnection rendering",
		"@enduml</pre>",
		`<table class="sysml-table"`,
	} {
		if !strings.Contains(string(page), want) {
			t.Errorf("converter input misses %q:\n%s", want, page)
		}
	}
	if strings.Contains(string(page), "diagram-1.svg") || strings.Contains(string(page), `class="dot"`) {
		t.Errorf("a diagram was drawn by the Mermaid tool or written as DOT:\n%s", page)
	}
}

// TestRenderDocumentPDFEngineMissing checks the typed degradation: with no
// converter installed, PDF output fails precisely and Markdown still works.
func TestRenderDocumentPDFEngineMissing(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	fixture := filepath.Join("..", "..", "internal", "doc", "docrender", "testdata", "telescope_report.sysml")
	out := filepath.Join(dir, "report.pdf")

	cmd := exec.Command(binary, fixture, "-render-document", "Observatory::MassReport", "-doc-form", "pdf", "-o", out)
	cmd.Env = append(os.Environ(), "PATH="+dir,
		docpdf.WeasyPrintEnv+"=", docpdf.PandocEnv+"=", docpdf.PrinceEnv+"=", docpdf.MermaidEnv+"=")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("PDF rendering succeeded without a converter:\n%s", output)
	}
	for _, want := range []string{"weasyprint", "not found", "-pdf-engine"} {
		if !strings.Contains(string(output), want) {
			t.Errorf("report misses %q:\n%s", want, output)
		}
	}

	// Markdown output needs none of the converters.
	md := filepath.Join(dir, "report.md")
	cmd = exec.Command(binary, fixture, "-render-document", "Observatory::MassReport", "-o", md)
	cmd.Env = append(os.Environ(), "PATH="+dir)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("markdown render: %v\n%s", err, output)
	}
}

// TestRenderDocumentPDFFlagConflicts checks the PDF flag combinations the run
// refuses.
func TestRenderDocumentPDFFlagConflicts(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport", "-doc-form", "pdf"),
		2, "name the file to write with -o")
	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport", "-doc-form", "latex"),
		2, "unknown document form", "markdown, html or pdf")
	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport", "-pdf-toc"),
		2, "-doc-form pdf")
	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport", "-doc-form", "pdf", "-o", "x.pdf", "-pdf-engine", "latex"),
		2, "unknown PDF engine", "weasyprint, pandoc, prince")
	wantReport(t, check(t, binary, documentModel, "-doc-form", "pdf"),
		2, "apply to -render-document")
	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport", "-doc-form", "pdf", "-o", "x.pdf", "-html-fragment"),
		2, "-html-fragment writes the document element alone", "whole page")
	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport", "-doc-form", "pdf", "-o", "x.pdf", "-html-mermaid", "cdn"),
		2, "-html-mermaid loads a script", "mermaid-cli")
	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport", "-doc-form", "pdf", "-o", "x.pdf", "-html-math", "cdn"),
		2, "-html-math loads a script", "KaTeX")
	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport", "-doc-form", "pdf", "-o", "x.pdf", "-html-css", filepath.Join(t.TempDir(), "absent.css")),
		2, "read stylesheet")
}
