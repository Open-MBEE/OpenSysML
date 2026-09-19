package docpdf

import (
	"bytes"
	"compress/zlib"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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
	golden, err := os.ReadFile(filepath.Join("..", "docrender", "testdata", "state_report.golden.md"))
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

// TestRenderWideTableLandscapeWithInstalledEngines renders a seven-column table through each
// installed converter and reads back a landscape page between two portrait ones.
func TestRenderWideTableLandscapeWithInstalledEngines(t *testing.T) {
	markdown := strings.Join([]string{
		"# Wide Report",
		"",
		"An opening paragraph\\.",
		"",
		"## Matrix",
		"",
		"<!-- caption -->",
		"*Every requirement*",
		"",
		"**team: Power**",
		"",
		"| a | b | c | d | e | f | g |",
		"| --- | --- | --- | --- | --- | --- | --- |",
		"| 1 | 2 | 3 | 4 | 5 | 6 | 7 |",
		"",
		"## Afterwards",
		"",
		"A closing paragraph\\.",
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
			pages := pageOrientations(t, pdf)
			if len(pages) != 3 || pages[0] != "portrait" || pages[1] != "landscape" || pages[2] != "portrait" {
				t.Fatalf("pages are %v, want portrait, landscape, portrait", pages)
			}
		})
	}
}

// pageOrientations reads each page's orientation from the /MediaBox entries of
// a PDF, inflating the object streams the converters write pages into.
func pageOrientations(t *testing.T, pdf []byte) []string {
	t.Helper()
	box := regexp.MustCompile(`/MediaBox \[\s*[-\d.]+\s+[-\d.]+\s+([-\d.]+)\s+([-\d.]+)\s*\]`)
	var pages []string
	collect := func(data []byte) {
		for _, m := range box.FindAllSubmatch(data, -1) {
			width, errW := strconv.ParseFloat(string(m[1]), 64)
			height, errH := strconv.ParseFloat(string(m[2]), 64)
			if errW != nil || errH != nil {
				t.Fatalf("unreadable /MediaBox %q", m[0])
			}
			if width > height {
				pages = append(pages, "landscape")
			} else {
				pages = append(pages, "portrait")
			}
		}
	}
	collect(pdf)
	rest := pdf
	for {
		i := bytes.Index(rest, []byte("stream\n"))
		if i < 0 {
			break
		}
		rest = rest[i+len("stream\n"):]
		reader, err := zlib.NewReader(bytes.NewReader(rest))
		if err != nil {
			continue
		}
		data, err := io.ReadAll(reader)
		if err != nil && len(data) == 0 {
			continue
		}
		collect(data)
	}
	if len(pages) == 0 {
		t.Fatal("no /MediaBox found in the PDF")
	}
	return pages
}
