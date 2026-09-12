package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestRenderDocumentHTMLFlag checks the scripted surface of HTML rendering:
// a whole page on stdout, the model facts on the markup, and an artifact
// written to -o.
func TestRenderDocumentHTMLFlag(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, documentModel, "-render-document", "Reports::MassReport", "-doc-form", "html")
	wantReport(t, got, 0,
		"<!DOCTYPE html>",
		"@layer opensysml",
		`<article class="sysml-document" data-document="Reports::MassReport">`,
		`<h1 class="sysml-title">Telescope Mass Report</h1>`,
		`<section class="sysml-section"`,
		`<caption class="sysml-caption">Heavy subsystems by mass</caption>`,
		`<th scope="col" data-column="mass">mass</th>`,
		`data-element-kind="partUsage"`)
	if strings.Contains(got.stderr, "<article") {
		t.Errorf("the artifact belongs on stdout, not stderr:\n%s", got.stderr)
	}

	out := filepath.Join(t.TempDir(), "report.html")
	wantReport(t, check(t, binary, documentModel,
		"-render-document", "Reports::MassReport", "-doc-form", "html", "-o", out),
		0, "wrote "+out, "(html,")
	written, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(written), "<!DOCTYPE html>") {
		t.Errorf("written artifact is not a page:\n%s", written)
	}
}

// TestRenderDocumentHTMLStylesheets checks the stylesheet options: a file is
// inlined after the default layer, a URL is linked, the default sheet can be
// left out, a fragment carries neither, and the default sheet is writable on
// its own.
func TestRenderDocumentHTMLStylesheets(t *testing.T) {
	binary := buildCLI(t)
	css := filepath.Join(t.TempDir(), "theme.css")
	if err := os.WriteFile(css, []byte(".sysml-document { color: rebeccapurple; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := check(t, binary, documentModel, "-render-document", "Reports::MassReport",
		"-doc-form", "html", "-html-css", css, "-html-css", "https://example.test/site.css")
	wantReport(t, got, 0,
		"@layer opensysml",
		"rebeccapurple",
		`<link rel="stylesheet" href="https://example.test/site.css">`)
	if strings.Index(got.stdout, "@layer opensysml {") > strings.Index(got.stdout, "rebeccapurple") {
		t.Errorf("supplied CSS must follow the default layer:\n%s", got.stdout)
	}

	// A sheet that declares nothing is still the sheet the run named.
	empty := filepath.Join(t.TempDir(), "empty.css")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport",
		"-doc-form", "html", "-html-css", empty), 0, "<!DOCTYPE html>", "@layer opensysml")

	bare := check(t, binary, documentModel, "-render-document", "Reports::MassReport",
		"-doc-form", "html", "-html-no-default-css")
	wantReport(t, bare, 0, "<!DOCTYPE html>")
	if strings.Contains(bare.stdout, "@layer opensysml") {
		t.Errorf("-html-no-default-css left the default sheet in:\n%s", bare.stdout)
	}

	fragment := check(t, binary, documentModel, "-render-document", "Reports::MassReport",
		"-doc-form", "html", "-html-fragment")
	wantReport(t, fragment, 0, `<article class="sysml-document"`)
	if strings.Contains(fragment.stdout, "<!DOCTYPE html>") || strings.Contains(fragment.stdout, "<style>") {
		t.Errorf("a fragment carries neither page shell nor stylesheet:\n%s", fragment.stdout)
	}

	sheet := runCommand(t, exec.Command(binary, "-html-default-css"))
	wantReport(t, sheet, 0, "@layer opensysml;", "--sysml-font-body")
	if strings.Contains(sheet.stdout, "<article") {
		t.Errorf("-html-default-css writes CSS, not a document:\n%s", sheet.stdout)
	}

	// Writing the sheet is the whole run, so it stands in for no other.
	wantReport(t, check(t, binary, documentModel, "-html-default-css"),
		2, "ask for it without model files")
	wantReport(t, runCommand(t, exec.Command(binary, "-html-default-css", "-render-document", "Reports::MassReport")),
		2, "ask for it in its own run")
	wantReport(t, runCommand(t, exec.Command(binary, "-html-default-css", "-convert", "kerml")),
		2, "ask for it in its own run")
	wantReport(t, runCommand(t, exec.Command(binary, "-html-default-css", "-html-no-default-css")),
		2, "not the sheet")
	wantReport(t, runCommand(t, exec.Command(binary, "-html-default-css", "-sync-diff", "repo.ttl")),
		2, "ask for it in its own run")
	wantReport(t, runCommand(t, exec.Command(binary, "-html-default-css", "-sync-apply", "https://example.test/api")),
		2, "ask for it in its own run")
	for _, unrelated := range [][]string{
		{"-from", "kerml"},
		{"-strict"},
		{"-sync-base", "base.ttl"},
		{"-sync-state", "state.json"},
		{"-sync-confirm-deletes"},
		{"-sync-mint-ids"},
		{"-sync-annotate", "all"},
	} {
		wantReport(t, runCommand(t, exec.Command(binary, append([]string{"-html-default-css"}, unrelated...)...)),
			2, "do not apply")
	}

	// -o is where the sheet is written, so it stays supported.
	destination := filepath.Join(t.TempDir(), "sysml-document.css")
	wantReport(t, runCommand(t, exec.Command(binary, "-html-default-css", "-o", destination)), 0)
	written, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("read written stylesheet: %v", err)
	}
	if !strings.Contains(string(written), "@layer opensysml;") {
		t.Errorf("-o did not receive the default stylesheet:\n%s", written)
	}
}

// TestRenderDocumentHTMLDocumentOptions checks the title page, contents and
// numbering options shape HTML output too, under their -doc- names.
func TestRenderDocumentHTMLDocumentOptions(t *testing.T) {
	binary := buildCLI(t)
	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport",
		"-doc-form", "html", "-doc-title-page", "-doc-toc", "-doc-number-sections"),
		0,
		`<header class="sysml-title-page">`,
		`<nav class="sysml-toc"`,
		`<span class="sysml-section-number">1</span>`)
}

// TestRenderDocumentHTMLMermaid checks -html-mermaid loads the pinned CDN
// release or the URL named, on a single page and on every page of a set.
func TestRenderDocumentHTMLMermaid(t *testing.T) {
	binary := buildCLI(t)
	pinned := `<script src="https://cdn.jsdelivr.net/npm/mermaid@11.16.1/dist/mermaid.min.js"></script>`

	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport",
		"-doc-form", "html", "-html-mermaid", "cdn"), 0, pinned, "</article>\n"+pinned+"\n</body>")
	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport",
		"-doc-form", "html", "-html-mermaid", "https://example.test/mermaid.js"),
		0, `<script src="https://example.test/mermaid.js"></script>`)
	plain := check(t, binary, documentModel, "-render-document", "Reports::MassReport", "-doc-form", "html")
	wantReport(t, plain, 0, "<!DOCTYPE html>")
	if strings.Contains(plain.stdout, "<script") {
		t.Errorf("a page loads no script unless asked:\n%s", plain.stdout)
	}

	dir := filepath.Join(t.TempDir(), "site")
	wantReport(t, check(t, binary, documentModel, "-render-documents", dir, "-doc-form", "html", "-html-mermaid", "cdn"), 0)
	pages, err := filepath.Glob(filepath.Join(dir, "*.html"))
	if err != nil || len(pages) == 0 {
		t.Fatalf("set wrote no pages: %v", err)
	}
	for _, page := range pages {
		content, err := os.ReadFile(page)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), pinned) {
			t.Errorf("%s does not load Mermaid:\n%s", page, content)
		}
	}

	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport",
		"-doc-form", "html", "-html-mermaid", "mermaid.js"),
		2, "-html-mermaid takes cdn or the URL of a Mermaid script")
	wantReport(t, check(t, binary, documentModel, "-render-documents", dir,
		"-doc-form", "html", "-html-mermaid", "mermaid.js"),
		2, "-html-mermaid takes cdn or the URL of a Mermaid script")
	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport",
		"-doc-form", "html", "-html-mermaid="), 2, "-html-mermaid is empty")
	wantReport(t, check(t, binary, documentModel, "-html-mermaid="), 2, "-html-mermaid is empty")
	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport",
		"-doc-form", "html", "-html-fragment", "-html-mermaid", "cdn"),
		2, "load Mermaid in the page you embed it in")
	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport", "-html-mermaid", "cdn"),
		2, "-doc-form html")
	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport",
		"-doc-form", "pdf", "-o", filepath.Join(t.TempDir(), "r.pdf"), "-html-mermaid", "cdn"),
		2, "-doc-form html")
	wantReport(t, check(t, binary, documentModel, "-html-mermaid", "cdn"),
		2, "apply to -render-document")
	wantReport(t, runCommand(t, exec.Command(binary, "-html-default-css", "-html-mermaid", "cdn")),
		2, "not the sheet")
}

// TestRenderDocumentHTMLDiagramForm checks -diagram-form reaches the HTML
// backend: DOT on request, Mermaid otherwise, the table a table either way.
func TestRenderDocumentHTMLDiagramForm(t *testing.T) {
	binary := buildCLI(t)
	fixture := filepath.Join("..", "..", "internal", "core", "docrender", "testdata", "telescope_report.sysml")
	dot := runCommand(t, exec.Command(binary, fixture, "-render-document", "Observatory::MassReport",
		"-doc-form", "html", "-diagram-form", "dot"))
	wantReport(t, dot, 0,
		`<pre class="dot">// view: Observatory::interconnectView`,
		"digraph &#34;Observatory::interconnectView&#34; {",
		`<table class="sysml-table"`)
	if strings.Contains(dot.stdout, `class="mermaid"`) {
		t.Errorf("a diagram is still Mermaid under -diagram-form dot:\n%s", dot.stdout)
	}
	wantReport(t, runCommand(t, exec.Command(binary, fixture, "-render-document", "Observatory::MassReport", "-doc-form", "html")),
		0, `<pre class="mermaid">`)
	wantReport(t, runCommand(t, exec.Command(binary, fixture, "-render-document", "Observatory::MassReport",
		"-doc-form", "html", "-diagram-form", "svg")), 2, `unknown diagram form "svg"`)
}

// TestRenderDocumentHTMLTheme checks -html-theme layers a bundled theme over
// the default sheet on a page, in a set's shared sheet and in the sheet
// -html-default-css writes, and refuses what it cannot style.
func TestRenderDocumentHTMLTheme(t *testing.T) {
	binary := buildCLI(t)

	themed := check(t, binary, documentModel, "-render-document", "Reports::MassReport",
		"-doc-form", "html", "-html-theme", "report")
	wantReport(t, themed, 0, "<!DOCTYPE html>", "--sysml-font-body: system-ui", "/* report:")
	if strings.Index(themed.stdout, "/* report:") < strings.Index(themed.stdout, "--sysml-font-body: system-ui") {
		t.Errorf("the theme must follow the default sheet it overrides:\n%s", themed.stdout)
	}
	if strings.Count(themed.stdout, "<style>") != 1 {
		t.Errorf("default sheet and theme share one style element:\n%s", themed.stdout)
	}
	// The default theme, named, is the default sheet alone.
	plain := check(t, binary, documentModel, "-render-document", "Reports::MassReport", "-doc-form", "html")
	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport",
		"-doc-form", "html", "-html-theme", "default"), 0, plain.stdout)
	for _, name := range []string{"modern", "print"} {
		wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport",
			"-doc-form", "html", "-html-theme", name), 0, "/* "+name+":")
	}

	dir := filepath.Join(t.TempDir(), "site")
	wantReport(t, check(t, binary, documentModel, "-render-documents", dir, "-doc-form", "html", "-html-theme", "modern"), 0)
	sheet, err := os.ReadFile(filepath.Join(dir, "sysml-document.css"))
	if err != nil {
		t.Fatalf("set wrote no shared sheet: %v", err)
	}
	if !strings.Contains(string(sheet), "@layer opensysml;") || !strings.Contains(string(sheet), "/* modern:") {
		t.Errorf("the set's sheet carries default and theme:\n%s", sheet)
	}

	wantReport(t, runCommand(t, exec.Command(binary, "-html-default-css", "-html-theme", "print")),
		0, "@layer opensysml;", "/* print:")
	wantReport(t, runCommand(t, exec.Command(binary, "-html-default-css", "-html-theme", "fancy")),
		2, `no bundled theme is named "fancy"`, "default, modern, print, report")

	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport",
		"-doc-form", "html", "-html-theme", "fancy"), 2, `no bundled theme is named "fancy"`)
	wantReport(t, check(t, binary, documentModel, "-render-documents", dir,
		"-doc-form", "html", "-html-theme", "../document"), 2, "no bundled theme is named")
	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport",
		"-doc-form", "html", "-html-theme="), 2, "-html-theme is empty")
	wantReport(t, check(t, binary, documentModel, "-html-theme="), 2, "-html-theme is empty")
	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport",
		"-doc-form", "html", "-html-fragment", "-html-theme", "report"), 2, "-html-theme styles a whole page")
	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport",
		"-doc-form", "html", "-html-no-default-css", "-html-theme", "report"), 2, "ask for one or the other")
	wantReport(t, check(t, binary, documentModel, "-render-documents", dir,
		"-doc-form", "html", "-html-no-default-css", "-html-theme", "report"), 2, "ask for one or the other")
	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport", "-html-theme", "report"),
		2, "-doc-form html")
	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport",
		"-doc-form", "pdf", "-o", filepath.Join(t.TempDir(), "r.pdf"), "-html-theme", "report"),
		2, "-doc-form html")
	wantReport(t, check(t, binary, documentModel, "-html-theme", "report"),
		2, "apply to -render-document")
}

// TestRenderDocumentHTMLFlagConflicts checks the HTML flag combinations the
// run refuses.
func TestRenderDocumentHTMLFlagConflicts(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport", "-html-css", "theme.css"),
		2, "-doc-form html")
	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport",
		"-doc-form", "html", "-pdf-engine", "weasyprint"),
		2, "-doc-form html needs no external converter")
	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport",
		"-doc-form", "html", "-html-fragment", "-html-css", "theme.css"),
		2, "style the page you embed it in")
	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport",
		"-doc-form", "html", "-html-fragment", "-html-no-default-css"),
		2, "-html-fragment already writes no stylesheet")
	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport",
		"-doc-form", "html", "-html-css", filepath.Join(t.TempDir(), "absent.css")),
		2, "read stylesheet")
	wantReport(t, check(t, binary, documentModel, "-html-css", "theme.css"),
		2, "apply to -render-document")
}

// TestRenderDocumentHTMLStylesheetURLScheme checks a stylesheet URL is linked
// whatever case its scheme is written in, rather than read as a file.
func TestRenderDocumentHTMLStylesheetURLScheme(t *testing.T) {
	binary := buildCLI(t)
	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport",
		"-doc-form", "html", "-html-css", "HTTPS://Example.test/Site.css"),
		0, `<link rel="stylesheet" href="HTTPS://Example.test/Site.css">`)
}

// TestSetStylesheetNameLength checks a stylesheet name whose escaping outgrows
// the file name limit is shortened to a distinct, still-suffixed name.
func TestSetStylesheetNameLength(t *testing.T) {
	taken := map[string]bool{}
	long := strings.Repeat("é", 120) + ".css"
	name := setStylesheetName(long, taken)
	if len(name) > 255 {
		t.Errorf("name is %d bytes: %q", len(name), name)
	}
	if !strings.HasSuffix(name, ".css") {
		t.Errorf("name lost its extension: %q", name)
	}
	other := setStylesheetName(strings.Repeat("é", 121)+".css", taken)
	if other == name {
		t.Errorf("two stylesheets share the name %q", name)
	}
	if len(other) > 255 {
		t.Errorf("name is %d bytes: %q", len(other), other)
	}
}
