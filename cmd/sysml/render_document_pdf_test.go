package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/docpdf"
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
	fixture := filepath.Join("..", "..", "internal", "core", "docrender", "testdata", "telescope_report.sysml")
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
	fixture := filepath.Join("..", "..", "internal", "core", "docrender", "testdata", "telescope_report.sysml")
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
	if got := strings.Count(string(page), `<figure class="dot">`); got != 2 {
		t.Errorf("converter input carries %d DOT figures, want 2:\n%s", got, page)
	}
	for _, want := range []string{
		"Graphviz DOT, which the PDF backend does not draw",
		"digraph &#34;Observatory::interconnectView&#34; {",
		"<table",
	} {
		if !strings.Contains(string(page), want) {
			t.Errorf("converter input misses %q:\n%s", want, page)
		}
	}
	if strings.Contains(string(page), "diagram-1.svg") {
		t.Errorf("a diagram was drawn by the Mermaid tool:\n%s", page)
	}
}

// TestRenderDocumentPDFEngineMissing checks the typed degradation: with no
// converter installed, PDF output fails precisely and Markdown still works.
func TestRenderDocumentPDFEngineMissing(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	fixture := filepath.Join("..", "..", "internal", "core", "docrender", "testdata", "telescope_report.sysml")
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
