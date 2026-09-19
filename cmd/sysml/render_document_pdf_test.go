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
// pre-rendering, HTML generation, converter invocation, and the artifact.
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
		`<div class="title-page">`,
		`<nav class="toc">`,
		`<span class="section-number">`,
		"diagram-1.svg",
		"diagram-2.svg",
	} {
		if !strings.Contains(string(page), want) {
			t.Errorf("converter input misses %q", want)
		}
	}
}

// fakeSVGWriter is a fake diagram tool writing an SVG to the file its -o
// argument names, or to stdout without one, and logging its arguments.
func fakeSVGWriter(t *testing.T, dir, name string) (path, log string) {
	t.Helper()
	log = filepath.Join(dir, name+".log")
	path = fakePDFTool(t, dir, name, `printf 'args:%s\n' "$*" >> `+log+`
out=""
while [ $# -gt 0 ]; do [ "$1" = "-o" ] && out="$2"; shift; done
svg='<svg xmlns="http://www.w3.org/2000/svg"><text>drawn by `+name+`</text></svg>'
if [ -n "$out" ]; then printf '%s' "$svg" > "$out"; else printf '%s' "$svg"; fi
`)
	return path, log
}

// TestRenderDocumentPDFDiagramFormDot checks a PDF run under -diagram-form dot
// never runs mmdc: without Graphviz every diagram is kept as DOT source behind
// a notice naming OPENSYSML_DOT, and with it every diagram is drawn by dot.
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
	render := func(dot string) string {
		t.Helper()
		cmd := exec.Command(binary, fixture, "-render-document", "Observatory::MassReport",
			"-doc-form", "pdf", "-diagram-form", "dot", "-o", out)
		cmd.Env = append(os.Environ(), docpdf.WeasyPrintEnv+"="+weasyprint, docpdf.MermaidEnv+"="+mmdc, docpdf.DotEnv+"="+dot)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("render: %v\n%s", err, output)
		}
		page, err := os.ReadFile(seen)
		if err != nil {
			t.Fatal(err)
		}
		return string(page)
	}

	page := render(filepath.Join(dir, "absent", "dot"))
	if got := strings.Count(page, `<figure class="dot">`); got != 2 {
		t.Errorf("converter input carries %d DOT figures, want 2:\n%s", got, page)
	}
	for _, want := range []string{
		"Graphviz DOT, which the PDF backend did not draw",
		"point " + docpdf.DotEnv + " at it",
		"digraph &#34;Observatory::interconnectView&#34; {",
		"<table",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("converter input misses %q:\n%s", want, page)
		}
	}
	if strings.Contains(page, "diagram-1.svg") {
		t.Errorf("a diagram was drawn without Graphviz:\n%s", page)
	}

	dot, log := fakeSVGWriter(t, dir, "dot")
	page = render(dot)
	if strings.Contains(page, `<figure class="dot">`) || !strings.Contains(page, `<img src="diagram-1.svg"`) || !strings.Contains(page, `<img src="diagram-2.svg"`) {
		t.Errorf("converter input does not reference both drawn diagrams:\n%s", page)
	}
	args, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(args), "-Tsvg -o diagram-1.svg diagram-1.dot") || !strings.Contains(string(args), "-Tsvg -o diagram-2.svg diagram-2.dot") {
		t.Errorf("dot invocations:\n%s", args)
	}
}

// TestRenderDocumentPDFDiagramFormPlantUML checks a PDF run under
// -diagram-form plantuml never runs mmdc: without the jar every diagram is
// kept as PlantUML source behind a notice naming OPENSYSML_PLANTUML_JAR, and
// with the jar and java every diagram is drawn through the jar in pipe mode.
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
	render := func(jar, java string) string {
		t.Helper()
		cmd := exec.Command(binary, fixture, "-render-document", "Observatory::MassReport",
			"-doc-form", "pdf", "-diagram-form", "plantuml", "-o", out)
		cmd.Env = append(os.Environ(), docpdf.WeasyPrintEnv+"="+weasyprint, docpdf.MermaidEnv+"="+mmdc,
			docpdf.PlantUMLJarEnv+"="+jar, docpdf.JavaEnv+"="+java)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("render: %v\n%s", err, output)
		}
		page, err := os.ReadFile(seen)
		if err != nil {
			t.Fatal(err)
		}
		return string(page)
	}

	page := render("", "")
	if got := strings.Count(page, `<figure class="plantuml">`); got != 2 {
		t.Errorf("converter input carries %d PlantUML figures, want 2:\n%s", got, page)
	}
	for _, want := range []string{
		"PlantUML, which the PDF backend did not draw",
		"point " + docpdf.PlantUMLJarEnv + " at it",
		"<pre>@startuml\n&#39; Observatory::interconnectView — interconnection rendering",
		"@enduml</pre>",
		"<table",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("converter input misses %q:\n%s", want, page)
		}
	}
	if strings.Contains(page, "diagram-1.svg") || strings.Contains(page, `class="dot"`) {
		t.Errorf("a diagram was drawn without the jar, or written as DOT:\n%s", page)
	}

	jar := filepath.Join(dir, "plantuml.jar")
	if err := os.WriteFile(jar, []byte("PK"), 0o600); err != nil {
		t.Fatal(err)
	}
	java, log := fakeSVGWriter(t, dir, "java")
	page = render(jar, java)
	if strings.Contains(page, `<figure class="plantuml">`) || !strings.Contains(page, `<img src="diagram-1.svg"`) || !strings.Contains(page, `<img src="diagram-2.svg"`) {
		t.Errorf("converter input does not reference both drawn diagrams:\n%s", page)
	}
	args, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(args), "args:-Djava.awt.headless=true -jar "+jar+" -tsvg -pipe\n") != 2 {
		t.Errorf("java invocations:\n%s", args)
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
}
