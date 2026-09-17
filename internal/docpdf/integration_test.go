package docpdf

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRenderWithInstalledEngines exercises each real converter when its tools
// are installed, and skips otherwise; the contract itself is tested with
// fakes in docpdf_test.go.
func TestRenderWithInstalledEngines(t *testing.T) {
	for _, engine := range Engines() {
		t.Run(engine, func(t *testing.T) {
			converter, err := EngineNamed(engine)
			if err != nil {
				t.Fatal(err)
			}
			if err := converter.Available(); err != nil {
				var docErr *Error
				if errors.As(err, &docErr) && docErr.Kind == ErrorToolMissing {
					t.Skipf("%s not installed: %v", engine, err)
				}
				t.Fatal(err)
			}
			pdf, err := Render("# Smoke Test\n\nOne paragraph\\.\n", engine, Options{TOC: true, NumberSections: true})
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if !strings.HasPrefix(string(pdf), "%PDF-") {
				t.Fatalf("output is no PDF: %.16q", pdf)
			}
		})
	}
}

// TestRenderInlineRunsWithInstalledEngines renders a document using every
// inline construct — styled runs, links, reference links to anchors, and
// grouped-table headings — through each installed converter.
func TestRenderInlineRunsWithInstalledEngines(t *testing.T) {
	markdown := strings.Join([]string{
		"# Inline Report",
		"",
		`<a id="Inline.20Report-masses"></a>`,
		"",
		"## Masses",
		"",
		"The *margin* is **critical** for `m > 0` per [spec](<https://example.com/spec.md>)\\.",
		"",
		"See [the masses](#Inline.20Report-masses) above\\.",
		"",
		"**zone: hot**",
		"",
		"| name | zone |",
		"| --- | --- |",
		"| Mirror | hot |",
		"",
	}, "\n")
	for _, engine := range Engines() {
		t.Run(engine, func(t *testing.T) {
			converter, err := EngineNamed(engine)
			if err != nil {
				t.Fatal(err)
			}
			if err := converter.Available(); err != nil {
				var docErr *Error
				if errors.As(err, &docErr) && docErr.Kind == ErrorToolMissing {
					t.Skipf("%s not installed: %v", engine, err)
				}
				t.Fatal(err)
			}
			pdf, err := Render(markdown, engine, Options{})
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if !strings.HasPrefix(string(pdf), "%PDF-") {
				t.Fatalf("output is no PDF: %.16q", pdf)
			}
		})
	}
}

// TestRenderFormulasWithInstalledKatex typesets real formulas when katex is
// installed, and skips otherwise: the page holds KaTeX's markup and none of
// the LaTeX source, and each installed engine lays it out.
func TestRenderFormulasWithInstalledKatex(t *testing.T) {
	if _, err := katexTool.locate(""); err != nil {
		t.Skipf("katex not installed: %v", err)
	}
	blocks, err := parseBlocks(mathMarkdown)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	typeset, err := renderFormulas(dir, blocks)
	if err != nil {
		t.Fatalf("renderFormulas: %v", err)
	}
	page := documentHTML(blocks, artwork{math: typeset}, Options{})
	for _, want := range []string{
		`<link rel="stylesheet" href="katex/katex.min.css">`,
		`<span class="math"><span class="katex">`,
		`<div class="formula"><span class="katex-display"><span class="katex">`,
		`<span class="mord mathnormal">A</span>`,
		`<span class="mrel">∝</span>`,
		`<span class="mord mathnormal">λ</span>`,
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("page missing %q:\n%s", want, page)
		}
	}
	for _, stray := range []string{`\propto`, `\frac`, `\lambda`, "$$", "$A", "<math", "<annotation"} {
		if strings.Contains(page, stray) {
			t.Fatalf("page leaks %q:\n%s", stray, page)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "katex", "fonts", "KaTeX_Main-Regular.woff2")); err != nil {
		t.Fatalf("KaTeX fonts not copied: %v", err)
	}
	for _, engine := range Engines() {
		t.Run(engine, func(t *testing.T) {
			converter, err := EngineNamed(engine)
			if err != nil {
				t.Fatal(err)
			}
			if err := converter.Available(); err != nil {
				var docErr *Error
				if errors.As(err, &docErr) && docErr.Kind == ErrorToolMissing {
					t.Skipf("%s not installed: %v", engine, err)
				}
				t.Fatal(err)
			}
			pdf, err := Render(mathMarkdown, engine, Options{TOC: true})
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if !strings.HasPrefix(string(pdf), "%PDF-") {
				t.Fatalf("output is no PDF: %.16q", pdf)
			}
		})
	}
}

// TestRenderDiagramsWithInstalledMermaid renders a real diagram when

// TestRenderStateReportWithInstalledEngines renders the state-and-event report
// golden through each installed converter, and skips otherwise.
func TestRenderStateReportWithInstalledEngines(t *testing.T) {
	golden, err := os.ReadFile(filepath.Join("..", "core", "docrender", "testdata", "state_report.golden.md"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	for _, engine := range Engines() {
		t.Run(engine, func(t *testing.T) {
			converter, err := EngineNamed(engine)
			if err != nil {
				t.Fatal(err)
			}
			if err := converter.Available(); err != nil {
				var docErr *Error
				if errors.As(err, &docErr) && docErr.Kind == ErrorToolMissing {
					t.Skipf("%s not installed: %v", engine, err)
				}
				t.Fatal(err)
			}
			pdf, err := Render(string(golden), engine, Options{TOC: true})
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if !strings.HasPrefix(string(pdf), "%PDF-") {
				t.Fatalf("output is no PDF: %.16q", pdf)
			}
		})
	}
}

// TestRenderDiagramsWithInstalledMermaid renders a real diagram when
// mermaid-cli and an engine are installed, and skips otherwise.
func TestRenderDiagramsWithInstalledMermaid(t *testing.T) {
	if _, err := mermaidTool.locate(""); err != nil {
		t.Skipf("mmdc not installed: %v", err)
	}
	converter, err := EngineNamed("")
	if err != nil {
		t.Fatal(err)
	}
	if err := converter.Available(); err != nil {
		t.Skipf("%s not installed: %v", converter.Name(), err)
	}
	markdown := "# Diagram Test\n\n```mermaid\nflowchart LR\n  a --> b\n```\n"
	pdf, err := Render(markdown, "", Options{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.HasPrefix(string(pdf), "%PDF-") {
		t.Fatalf("output is no PDF: %.16q", pdf)
	}
}
