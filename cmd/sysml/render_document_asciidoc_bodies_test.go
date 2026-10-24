package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/docplan/markup"
	"github.com/Open-MBEE/OpenSysML/internal/ir/docplan/markup/markuptest"
)

const asciiDocBodiesFixture = "../../internal/doc/docrender/testdata/asciidoc_report.sysml"

// TestRenderDocumentAsciiDocBodies renders a report whose prose is AsciiDoc
// bodies through every output form, with the recorded converter standing in
// for Asciidoctor.js, and reports a missing converter by name.
func TestRenderDocumentAsciiDocBodies(t *testing.T) {
	binary := buildCLI(t)
	source, err := os.ReadFile(asciiDocBodiesFixture)
	if err != nil {
		t.Fatal(err)
	}
	model := string(source)

	t.Setenv(markup.AsciiDocEnv, filepath.Join(t.TempDir(), "missing-converter"))
	wantReport(t, check(t, binary, model, "-render-document", "Prose::AsciiDocReport"),
		2, "markup-tool-missing", "download-doc-pdf-toolchain.sh", markup.AsciiDocEnv)

	converter, err := markuptest.RecordedConverter(t.TempDir(), "../../internal/ir/docplan/markup/testdata")
	if err != nil {
		t.Skip(err)
	}
	t.Setenv(markup.AsciiDocEnv, converter)

	markdown := check(t, binary, model, "-render-document", "Prose::AsciiDocReport")
	wantReport(t, markdown, 0, "**Note:** Mind the **gap**.", "| Part | Mass |", "| Mirror | 12 kg |",
		"![The optics](figures/optics.png)", "[the summary](#summary)", "- two *soft*", "1. first")

	dir := t.TempDir()
	html := filepath.Join(dir, "report.html")
	wantReport(t, check(t, binary, model, "-render-document", "Prose::AsciiDocReport", "-doc-form", "html", "-o", html), 0)
	page, err := os.ReadFile(html)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<table", "data-column=\"Part\">Part</th>", "<td", "Mirror", "<strong>Note:</strong>", "figures/optics.png", "m \\propto D^2"} {
		if !strings.Contains(string(page), want) {
			t.Errorf("HTML lacks %q", want)
		}
	}

	if _, err := exec.LookPath(weasyPrintOrEnv()); err != nil {
		t.Skipf("no WeasyPrint for the PDF form: %v", err)
	}
	pdf := filepath.Join(dir, "report.pdf")
	wantReport(t, check(t, binary, model, "-render-document", "Prose::AsciiDocReport", "-doc-form", "pdf", "-o", pdf), 0)
	if data, err := os.ReadFile(pdf); err != nil || !strings.HasPrefix(string(data), "%PDF") {
		t.Errorf("PDF not written: %v", err)
	}
}

// weasyPrintOrEnv names the WeasyPrint the PDF backend would run.
func weasyPrintOrEnv() string {
	if path := strings.TrimSpace(os.Getenv("OPENSYSML_WEASYPRINT")); path != "" {
		return path
	}
	return "weasyprint"
}
