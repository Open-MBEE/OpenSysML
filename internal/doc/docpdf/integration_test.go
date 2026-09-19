package docpdf

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// toolchainRequiredEnv is set in CI after the toolchain script has run, so an
// absent tool fails these tests instead of skipping them.
const toolchainRequiredEnv = "OPENSYSML_REQUIRE_PDF_TOOLCHAIN"

// skipWithout skips the calling test for a tool that is not installed — unless
// the toolchain is declared mandatory, when it fails.
func skipWithout(t *testing.T, what string, err error) {
	t.Helper()
	if required := os.Getenv(toolchainRequiredEnv); required != "" {
		t.Fatalf("%s=%s but %s not installed: %v", toolchainRequiredEnv, required, what, err)
	}
	t.Skipf("%s not installed: %v", what, err)
}

// requireEngine skips the calling test unless the named converter (the default
// one for "") is installed. Prince is commercial and never provisioned, so it
// skips even when the toolchain is mandatory.
func requireEngine(t *testing.T, engine string) {
	t.Helper()
	converter, err := EngineNamed(engine)
	if err != nil {
		t.Fatal(err)
	}
	err = converter.Available()
	if err == nil {
		return
	}
	var docErr *Error
	if !errors.As(err, &docErr) || docErr.Kind != ErrorToolMissing {
		t.Fatal(err)
	}
	if converter.Name() == princeTool.name {
		t.Skipf("%s not installed: %v", converter.Name(), err)
	}
	skipWithout(t, converter.Name(), err)
}

// TestRenderWithInstalledEngines exercises each real converter when its tools
// are installed, and skips otherwise; the contract itself is tested with
// fakes in docpdf_test.go.
func TestRenderWithInstalledEngines(t *testing.T) {
	for _, engine := range Engines() {
		t.Run(engine, func(t *testing.T) {
			requireEngine(t, engine)
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
			requireEngine(t, engine)
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
		skipWithout(t, "katex", err)
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
			requireEngine(t, engine)
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
		skipWithout(t, "mmdc", err)
	}
	requireEngine(t, "")
	markdown := "# Diagram Test\n\n```mermaid\nflowchart LR\n  a --> b\n```\n"
	pdf, err := Render(markdown, "", Options{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.HasPrefix(string(pdf), "%PDF-") {
		t.Fatalf("output is no PDF: %.16q", pdf)
	}
}

// TestRenderDiagramsWithInstalledGraphviz draws a DOT block through a real
// Graphviz when one is installed, and skips otherwise: a positioned block is
// laid out by the engine its header names, at the coordinates it states.
func TestRenderDiagramsWithInstalledGraphviz(t *testing.T) {
	if _, err := graphvizTool.locate(""); err != nil {
		skipWithout(t, "Graphviz dot", err)
	}
	dir := t.TempDir()
	positioned := "// kind: interconnection\n// layout: neato -n\ngraph G {\n  node [shape=box];\n  Pump [pos=\"0,0\"];\n  Tank [pos=\"200,100\"];\n  Pump -- Tank [label=\"supply\"];\n}"
	markdown := "# Diagram Test\n\n```dot\n// kind: action\ndigraph G {\n  start -> \"Fill tank\" -> stop;\n}\n```\n\n```dot\n" + positioned + "\n```\n"
	blocks, err := parseBlocks(markdown)
	if err != nil {
		t.Fatal(err)
	}
	diagrams, err := renderDiagrams(dir, blocks)
	if err != nil {
		t.Fatalf("renderDiagrams: %v", err)
	}
	if len(diagrams) != 2 || diagrams[0].Image != "diagram-1.svg" || diagrams[1].Image != "diagram-2.svg" {
		t.Fatalf("diagrams = %+v", diagrams)
	}
	flow, err := os.ReadFile(filepath.Join(dir, "diagram-1.svg"))
	if err != nil || !strings.Contains(string(flow), "<svg") || !strings.Contains(string(flow), "Fill tank") {
		t.Fatalf("dot layout SVG: %v\n%s", err, flow)
	}
	placed, err := os.ReadFile(filepath.Join(dir, "diagram-2.svg"))
	if err != nil || !strings.Contains(string(placed), "<svg") || !strings.Contains(string(placed), "supply") {
		t.Fatalf("neato -n SVG: %v\n%s", err, placed)
	}
	// With -n the stated positions are kept, so Tank sits 200 points right of
	// and 100 above Pump: the SVG's Tank text is right of the Pump text.
	pump, tank := strings.Index(string(placed), ">Pump</text>"), strings.Index(string(placed), ">Tank</text>")
	if pump < 0 || tank < 0 {
		t.Fatalf("labels missing from the neato SVG:\n%s", placed)
	}
	x := func(at int) float64 {
		text := string(placed)[:at]
		text = text[strings.LastIndex(text, "<text"):]
		_, after, _ := strings.Cut(text, `x="`)
		before, _, _ := strings.Cut(after, `"`)
		v, err := strconv.ParseFloat(before, 64)
		if err != nil {
			t.Fatalf("text x %q: %v", before, err)
		}
		return v
	}
	if px, tx := x(pump), x(tank); tx <= px {
		t.Fatalf("Tank (x=%v) is not right of Pump (x=%v):\n%s", tx, px, placed)
	}

	requireEngine(t, "")
	pdf, err := Render(markdown, "", Options{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.HasPrefix(string(pdf), "%PDF-") {
		t.Fatalf("output is no PDF: %.16q", pdf)
	}
}

// TestRenderDiagramsWithInstalledPlantUML draws a PlantUML block through a
// real jar when OPENSYSML_PLANTUML_JAR and java are set, and skips otherwise.
func TestRenderDiagramsWithInstalledPlantUML(t *testing.T) {
	if _, err := locatePlantUMLJar(); err != nil {
		skipWithout(t, "the PlantUML jar", err)
	}
	if _, err := javaTool.locate(""); err != nil {
		skipWithout(t, "java", err)
	}
	dir := t.TempDir()
	markdown := "# Diagram Test\n\n```plantuml\n@startuml\nstate \"Fill tank\" as fill\n[*] --> fill\nfill --> [*]\n@enduml\n```\n"
	blocks, err := parseBlocks(markdown)
	if err != nil {
		t.Fatal(err)
	}
	diagrams, err := renderDiagrams(dir, blocks)
	if err != nil {
		t.Fatalf("renderDiagrams: %v", err)
	}
	if len(diagrams) != 1 || diagrams[0].Image != "diagram-1.svg" {
		t.Fatalf("diagrams = %+v", diagrams)
	}
	svg, err := os.ReadFile(filepath.Join(dir, "diagram-1.svg"))
	if err != nil || !strings.HasPrefix(string(svg), "<svg") || !strings.Contains(string(svg), "Fill tank") {
		t.Fatalf("PlantUML SVG: %v\n%s", err, svg)
	}

	// A block the jar rejects is the typed failure, with what it said.
	_, err = renderDiagrams(t.TempDir(), []block{{Kind: blockPlantUML, Source: "@startuml\nclass A\nA --> \n@enduml"}})
	var docErr *Error
	if !errors.As(err, &docErr) || docErr.Kind != ErrorToolFailed || !strings.Contains(docErr.Detail, "Syntax Error") {
		t.Fatalf("rejected block: got %v, want ErrorToolFailed with the jar's message", err)
	}

	requireEngine(t, "")
	pdf, err := Render(markdown, "", Options{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.HasPrefix(string(pdf), "%PDF-") {
		t.Fatalf("output is no PDF: %.16q", pdf)
	}
}
