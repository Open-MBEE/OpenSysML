package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRenderDocumentAsciiDocFlag checks the scripted surface of AsciiDoc
// rendering: the document on stdout, the model facts in AsciiDoc blocks, an
// artifact written to -o, and the HTML and PDF options refused.
func TestRenderDocumentAsciiDocFlag(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, documentModel, "-render-document", "Reports::MassReport", "-doc-form", "asciidoc")
	wantReport(t, got, 0,
		"= Telescope Mass Report\n",
		"\n== ",
		".Heavy subsystems by mass\n[options=\"header\"]\n|===\n| name | mass\n",
		"\n|===\n")
	if strings.Contains(got.stdout, "<article") || strings.Contains(got.stdout, "# ") {
		t.Errorf("the artifact is neither HTML nor Markdown:\n%s", got.stdout)
	}

	out := filepath.Join(t.TempDir(), "report.adoc")
	wantReport(t, check(t, binary, documentModel,
		"-render-document", "Reports::MassReport", "-doc-form", "asciidoc", "-o", out),
		0, "wrote "+out, "(asciidoc,")
	written, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(written), "= Telescope Mass Report\n") {
		t.Errorf("written artifact is not an AsciiDoc document:\n%s", written)
	}

	wantReport(t, check(t, binary, documentModel,
		"-render-document", "Reports::MassReport", "-doc-form", "asciidoc", "-html-fragment"),
		2, "ask for it with -doc-form html")
	wantReport(t, check(t, binary, documentModel,
		"-render-document", "Reports::MassReport", "-doc-form", "asciidoctor"),
		2, `unknown document form "asciidoctor"; -doc-form takes markdown, asciidoc, html or pdf`)
}

// TestRenderDocumentsAsciiDoc checks a set written as AsciiDoc: .adoc files
// whose cross-document references are xref macros into each other.
func TestRenderDocumentsAsciiDoc(t *testing.T) {
	binary := buildCLI(t)
	dir := filepath.Join(t.TempDir(), "rendered")

	got := check(t, binary, linkedModel, "-render-documents", dir, "-doc-form", "asciidoc")
	if got.status != 0 {
		t.Fatalf("exit = %d\n%s", got.status, got.output())
	}
	if !strings.Contains(got.stderr, "(asciidoc,") {
		t.Errorf("the written files are not reported as asciidoc:\n%s", got.stderr)
	}
	report, err := os.ReadFile(filepath.Join(dir, "Reports-MainReport.adoc"))
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	if !strings.Contains(string(report), "xref:Reports-Appendix.adoc#tables[Detail Tables]") {
		t.Errorf("report lacks cross-document reference:\n%s", report)
	}
	appendix, err := os.ReadFile(filepath.Join(dir, "Reports-Appendix.adoc"))
	if err != nil {
		t.Fatalf("read appendix: %v", err)
	}
	if !strings.Contains(string(appendix), "[[tables]]\n") {
		t.Errorf("appendix lacks referenced anchor:\n%s", appendix)
	}
	if !strings.Contains(string(appendix), "xref:Reports-MainReport.adoc[Main Report]") {
		t.Errorf("appendix lacks root reference:\n%s", appendix)
	}
}
