package docpdf

import (
	"bytes"
	"compress/zlib"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docir"
	"github.com/Open-MBEE/OpenSysML/internal/doc/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
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

// installedConverter returns the named converter, skipping the test when a
// tool it needs is not installed; the contract itself is tested with fakes.
// Prince is commercial and never provisioned, so it skips even when the
// toolchain is mandatory.
func installedConverter(t *testing.T, engine string) Converter {
	t.Helper()
	converter, err := EngineNamed(engine)
	if err != nil {
		t.Fatal(err)
	}
	if err := converter.Available(); err != nil {
		var docErr *Error
		if !errors.As(err, &docErr) || docErr.Kind != ErrorToolMissing {
			t.Fatal(err)
		}
		if converter.Name() == princeTool.name {
			t.Skipf("%s not installed: %v", engine, err)
		}
		skipWithout(t, engine, err)
	}
	return converter
}

// renderInstalled renders document with an installed engine and returns the
// PDF's text as pdftotext extracts it.
func renderInstalled(t *testing.T, document *docir.Document, engine string, opts Options) (pdf []byte, text string) {
	t.Helper()
	installedConverter(t, engine)
	pdf, err := Render(document, engine, opts)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.HasPrefix(string(pdf), "%PDF-") {
		t.Fatalf("output is no PDF: %.16q", pdf)
	}
	return pdf, pdfText(t, pdf)
}

// pdfText extracts a PDF's text with pdftotext; absent, it skips the test, or
// fails it when the toolchain is mandatory.
func pdfText(t *testing.T, pdf []byte) string {
	t.Helper()
	pdftotext, err := exec.LookPath("pdftotext")
	if err != nil {
		skipWithout(t, "pdftotext", err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "doc.pdf"), pdf, 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(pdftotext, "-layout", filepath.Join(dir, "doc.pdf"), "-").Output() // #nosec G204 -- pdftotext from PATH, fixed arguments
	if err != nil {
		t.Fatalf("pdftotext: %v", err)
	}
	return string(out)
}

// pdfImages lists a PDF's raster images as pdfimages reports them, one line
// each; an absent pdfimages is handled as in pdfText.
func pdfImages(t *testing.T, pdf []byte) string {
	t.Helper()
	pdfimages, err := exec.LookPath("pdfimages")
	if err != nil {
		skipWithout(t, "pdfimages", err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "doc.pdf"), pdf, 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(pdfimages, "-list", filepath.Join(dir, "doc.pdf")).Output() // #nosec G204 -- pdfimages from PATH, fixed arguments
	if err != nil {
		t.Fatalf("pdfimages: %v", err)
	}
	return string(out)
}

// TestRenderStylesheetAssetsBesideTheOutput renders through each installed
// converter with a reader stylesheet whose relative @import and url() name
// files beside the PDF, as an HTML page's sheet would name files beside the
// page, and reads back that both were found: the imported sheet's generated
// text and the 12x12 fixture image.
func TestRenderStylesheetAssetsBesideTheOutput(t *testing.T) {
	out := t.TempDir()
	mark, err := os.ReadFile(filepath.Join("testdata", "mark.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "mark.png"), mark, 0o600); err != nil {
		t.Fatal(err)
	}
	beside := ".sysml-title::after, h1.title::after { content: \" IMPORTEDBESIDE\" }\n"
	if err := os.WriteFile(filepath.Join(out, "beside.css"), []byte(beside), 0o600); err != nil {
		t.Fatal(err)
	}
	sheet := docrender.InlineStylesheet("@import url(\"beside.css\");\n.sysml-title::before, h1.title::before { content: url(mark.png) }\n")
	document := plainDocument(t)
	for _, engine := range Engines() {
		t.Run(engine, func(t *testing.T) {
			pdf, text := renderInstalled(t, document, engine, Options{Stylesheets: []docrender.Stylesheet{sheet}, BaseDir: out})
			if !strings.Contains(text, "IMPORTEDBESIDE") {
				t.Errorf("the imported sheet beside the PDF did not apply:\n%s", text)
			}
			images := pdfImages(t, pdf)
			if !regexp.MustCompile(`(?m)^\s*1\s+0\s+image\s+12\s+12\s`).MatchString(images) {
				t.Errorf("the image beside the PDF was not drawn:\n%s", images)
			}
		})
	}
}

// TestRenderWithInstalledEngines exercises each real converter when its tools
// are installed, and skips otherwise.
func TestRenderWithInstalledEngines(t *testing.T) {
	document := plainDocument(t)
	for _, engine := range Engines() {
		t.Run(engine, func(t *testing.T) {
			_, text := renderInstalled(t, document, engine, Options{TOC: true, NumberSections: true})
			if !strings.Contains(text, "One paragraph.") {
				t.Fatalf("paragraph missing from the PDF text:\n%s", text)
			}
		})
	}
}

// TestRenderTelescopeWithInstalledEngines renders the telescope report — the
// document every other backend's golden is cut from — through each installed
// converter with mermaid-cli, and reads back what the page must show: the
// headings, table cells, styled definitions and captions, and none of the
// Markdown escapes or Mermaid source.
func TestRenderTelescopeWithInstalledEngines(t *testing.T) {
	if _, err := mermaidTool.locate(""); err != nil {
		skipWithout(t, "mmdc", err)
	}
	document := telescopeDocument(t)
	for _, engine := range Engines() {
		t.Run(engine, func(t *testing.T) {
			_, text := renderInstalled(t, document, engine, Options{TitlePage: true, TOC: true, NumberSections: true})
			for _, want := range []string{
				"Telescope Mass Report",
				"Subsystems grouped by zone",
				"Imaging chain interconnection",
				"Observatory states, left to right",
				"M1",
				"Actuators that phase the mirror segments.",
			} {
				if !strings.Contains(text, want) {
					t.Errorf("PDF text lacks %q:\n%s", want, text)
				}
			}
			for _, stray := range []string{"flowchart", "stateDiagram", `\|`, `\*`, "<!--", "**"} {
				if strings.Contains(text, stray) {
					t.Errorf("PDF text carries %q:\n%s", stray, text)
				}
			}
		})
	}
}

// TestRenderInlineRunsWithInstalledEngines renders a document using every
// inline construct — styled runs, links, reference links to anchors, and
// grouped-table headings — through each installed converter, and checks the
// runs read as prose rather than as Markdown.
func TestRenderInlineRunsWithInstalledEngines(t *testing.T) {
	document := sourceDocument(t, "inline.sysml", `package Inline {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	part def Subsystem { attribute zone : String; }
	part mirror : Subsystem { attribute redefines zone = "hot"; }
	part mount : Subsystem { attribute redefines zone = "cold"; }

	calc def Zones :> Query {
		in root : Element;
		Project(
			source = WhereType(source = OwnedElements(source = root), type = "Inline::Subsystem"),
			properties = ("name", "zone")
		)
	}

	part def Report :> Document {
		attribute redefines title = "Inline Report";
		part masses : Section {
			attribute redefines title = "Masses";
			part p : Paragraph {
				part a : Span { attribute redefines text = "The"; }
				part b : Span { attribute redefines text = "margin"; attribute redefines style = "emphasis"; }
				part c : Span { attribute redefines text = "is"; }
				part d : Span { attribute redefines text = "critical"; attribute redefines style = "strong"; }
				part e : Span { attribute redefines text = "for"; }
				part f : Span { attribute redefines text = "m > 0"; attribute redefines style = "code"; }
				part g : Span { attribute redefines text = "per the"; }
				part h : Link { attribute redefines text = "spec"; attribute redefines target = "https://example.com/spec.md"; }
			}
			part q : Paragraph {
				part a : Span { attribute redefines text = "See"; }
				part r : Ref { ref redefines target = zones; }
				part b : Span { attribute redefines text = "below."; }
			}
			part zones : Table {
				attribute redefines caption = "Subsystems by zone";
				attribute redefines groupBy = "zone";
				calc rows : Zones { in root = Inline; }
			}
		}
	}
}
`, "Inline::Report")
	for _, engine := range Engines() {
		t.Run(engine, func(t *testing.T) {
			_, text := renderInstalled(t, document, engine, Options{})
			for _, want := range []string{"The margin is critical for m > 0 per the spec", "See Subsystems by zone below.", "zone: hot", "zone: cold", "mirror"} {
				if !strings.Contains(text, want) {
					t.Errorf("PDF text lacks %q:\n%s", want, text)
				}
			}
			for _, stray := range []string{"*", "`", "](", "<a "} {
				if strings.Contains(text, stray) {
					t.Errorf("PDF text carries %q:\n%s", stray, text)
				}
			}
		})
	}
}

// TestRenderFormulasWithInstalledKatex typesets the optics report's formulas
// when katex is installed, and skips otherwise: the page holds KaTeX's markup
// and none of the LaTeX source, and each installed engine lays it out.
func TestRenderFormulasWithInstalledKatex(t *testing.T) {
	if _, err := katexTool.locate(""); err != nil {
		skipWithout(t, "katex", err)
	}
	document := mathDocument(t)
	dir := t.TempDir()
	typeset, err := renderFormulas(dir, docrender.Formulas(document))
	if err != nil {
		t.Fatalf("renderFormulas: %v", err)
	}
	page, err := docrender.HTML(document, htmlOptions(Options{}, dir, nil, typeset))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<link rel="stylesheet" href="` + fileURL(filepath.Join(dir, typeset.css)) + `">`,
		`<span class="sysml-math"><span class="katex">`,
		`<div class="sysml-math"><span class="katex-display"><span class="katex">`,
		`<span class="mord mathnormal">A</span>`,
		`<span class="mrel">∝</span>`,
		`<span class="mord mathnormal mtight">λ</span>`,
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("page missing %q:\n%s", want, page)
		}
	}
	for _, stray := range []string{`\propto`, `\frac`, `\lambda`, `\(`, `\[`, "<math", "<annotation"} {
		if strings.Contains(page, stray) {
			t.Fatalf("page leaks %q:\n%s", stray, page)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "katex", "fonts", "KaTeX_Main-Regular.woff2")); err != nil {
		t.Fatalf("KaTeX fonts not copied: %v", err)
	}
	for _, engine := range Engines() {
		t.Run(engine, func(t *testing.T) {
			_, text := renderInstalled(t, document, engine, Options{TOC: true})
			for _, want := range []string{"Collecting area of a circular mirror", "Rayleigh criterion", "each $ of budget"} {
				if !strings.Contains(text, want) {
					t.Errorf("PDF text lacks %q:\n%s", want, text)
				}
			}
			for _, stray := range []string{`\frac`, `\theta`, `\propto`, "$$", `\(`} {
				if strings.Contains(text, stray) {
					t.Errorf("PDF text carries LaTeX %q:\n%s", stray, text)
				}
			}
		})
	}
}

// TestRenderStateReportWithInstalledEngines renders the state-and-event report
// through each installed converter and reads back the state paths, event
// summaries and unit-bearing instants as prose.
func TestRenderStateReportWithInstalledEngines(t *testing.T) {
	document := stateDocument(t)
	for _, engine := range Engines() {
		t.Run(engine, func(t *testing.T) {
			_, text := renderInstalled(t, document, engine, Options{TOC: true})
			for _, want := range []string{
				"Lamp Report",
				"Active states of every lamp",
				"on.dim",
				"1 [s]",
				"level = 3",
				"t=0 lamp1.lp: enter: on",
				"lamp1.lp in on.fast",
			} {
				if !strings.Contains(text, want) {
					t.Errorf("PDF text lacks %q:\n%s", want, text)
				}
			}
			for _, stray := range []string{`\[`, "<!--", "*"} {
				if strings.Contains(text, stray) {
					t.Errorf("PDF text carries %q:\n%s", stray, text)
				}
			}
		})
	}
}

// TestRenderDiagramsWithInstalledMermaid renders the telescope report's two
// diagrams when mermaid-cli and an engine are installed, and skips otherwise.
func TestRenderDiagramsWithInstalledMermaid(t *testing.T) {
	if _, err := mermaidTool.locate(""); err != nil {
		skipWithout(t, "mmdc", err)
	}
	_, text := renderInstalled(t, telescopeDocument(t), "", Options{})
	if !strings.Contains(text, "Imaging chain interconnection") {
		t.Fatalf("diagram caption missing:\n%s", text)
	}
}

// TestRenderDiagramsWithInstalledGraphviz draws the telescope report's
// diagrams as DOT through a real Graphviz when one is installed, and skips
// otherwise; a positioned diagram is laid out by the engine its header names,
// at the coordinates it states.
func TestRenderDiagramsWithInstalledGraphviz(t *testing.T) {
	if _, err := graphvizTool.locate(""); err != nil {
		skipWithout(t, "Graphviz dot", err)
	}
	dir := t.TempDir()
	diagrams, err := docrender.Diagrams(telescopeDocument(t), view.FormDot, "")
	if err != nil {
		t.Fatal(err)
	}
	positioned := docrender.Diagram{Name: "placed", Source: "// kind: interconnection\n// layout: neato -n\ngraph G {\n  node [shape=box];\n  Pump [pos=\"0,0\"];\n  Tank [pos=\"200,100\"];\n  Pump -- Tank [label=\"supply\"];\n}"}
	images, err := drawDiagrams(dir, append(diagrams, positioned), view.FormDot)
	if err != nil {
		t.Fatalf("drawDiagrams: %v", err)
	}
	if len(images) != 3 || images[0] != "diagram-1.svg" || images[2] != "diagram-3.svg" {
		t.Fatalf("images = %q", images)
	}
	for i, image := range images {
		if err := checkSVG(filepath.Join(dir, image)); err != nil {
			t.Fatalf("diagram %d: %v", i+1, err)
		}
	}
	placed, err := os.ReadFile(filepath.Join(dir, "diagram-3.svg"))
	if err != nil || !strings.Contains(string(placed), "supply") {
		t.Fatalf("neato -n SVG: %v\n%s", err, placed)
	}
	// With -n the stated positions are kept, so Tank sits 200 points right of
	// Pump: the SVG's Tank text is right of the Pump text.
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

	_, text := renderInstalled(t, telescopeDocument(t), "", Options{DiagramForm: view.FormDot})
	if strings.Contains(text, "digraph") || strings.Contains(text, dotNotice[:40]) {
		t.Fatalf("DOT source or its notice reached the PDF:\n%s", text)
	}
	if !strings.Contains(text, "Imaging chain interconnection") {
		t.Fatalf("diagram caption missing:\n%s", text)
	}
}

// TestRenderDiagramsWithInstalledPlantUML draws the telescope report's
// diagrams as PlantUML through a real jar when OPENSYSML_PLANTUML_JAR and java
// are set, and skips otherwise.
func TestRenderDiagramsWithInstalledPlantUML(t *testing.T) {
	if _, err := locatePlantUMLJar(); err != nil {
		skipWithout(t, "the PlantUML jar", err)
	}
	if _, err := javaTool.locate(""); err != nil {
		skipWithout(t, "java", err)
	}
	dir := t.TempDir()
	diagrams, err := docrender.Diagrams(telescopeDocument(t), view.FormPlantUML, "")
	if err != nil {
		t.Fatal(err)
	}
	images, err := drawDiagrams(dir, diagrams, view.FormPlantUML)
	if err != nil {
		t.Fatalf("drawDiagrams: %v", err)
	}
	if len(images) != 2 || images[0] != "diagram-1.svg" || images[1] != "diagram-2.svg" {
		t.Fatalf("images = %q", images)
	}
	svg, err := os.ReadFile(filepath.Join(dir, "diagram-1.svg"))
	if err != nil || !strings.HasPrefix(string(svg), "<svg") {
		t.Fatalf("PlantUML SVG: %v\n%s", err, svg)
	}

	// A diagram the jar rejects is the typed failure, with what it said.
	rejected := []docrender.Diagram{{Name: "bad", Source: "@startuml\nclass A\nA --> \n@enduml"}}
	_, err = drawDiagrams(t.TempDir(), rejected, view.FormPlantUML)
	var docErr *Error
	if !errors.As(err, &docErr) || docErr.Kind != ErrorToolFailed || !strings.Contains(docErr.Detail, "Syntax Error") {
		t.Fatalf("rejected diagram: got %v, want ErrorToolFailed with the jar's message", err)
	}

	_, text := renderInstalled(t, telescopeDocument(t), "", Options{DiagramForm: view.FormPlantUML})
	if strings.Contains(text, "@startuml") || strings.Contains(text, plantumlNotice[:40]) {
		t.Fatalf("PlantUML source or its notice reached the PDF:\n%s", text)
	}
	if !strings.Contains(text, "Imaging chain interconnection") {
		t.Fatalf("diagram caption missing:\n%s", text)
	}
}

// TestRenderWideTableLandscapeWithInstalledEngines renders a captioned,
// grouped seven-column table under its section heading through each installed
// converter and reads back a landscape page between two portrait ones.
func TestRenderWideTableLandscapeWithInstalledEngines(t *testing.T) {
	document := wideTableDocument(t)
	for _, engine := range Engines() {
		t.Run(engine, func(t *testing.T) {
			pdf, _ := renderInstalled(t, document, engine, Options{})
			pages := pageOrientations(t, pdf)
			if len(pages) != 3 || pages[0] != "portrait" || pages[1] != "landscape" || pages[2] != "portrait" {
				t.Fatalf("pages are %v, want portrait, landscape, portrait", pages)
			}
		})
	}
}

// TestRenderTallFigureFitsThePageWithInstalledEngines renders a forty-step
// action flow through each installed converter with mermaid-cli and reads back
// that the figure is scaled onto one page, its first node (the language's
// `start`, headed as `initial`) and last step and its caption together, rather
// than cut at the page's foot.
func TestRenderTallFigureFitsThePageWithInstalledEngines(t *testing.T) {
	if _, err := mermaidTool.locate(""); err != nil {
		skipWithout(t, "mmdc", err)
	}
	document := tallFlowDocument(t)
	for _, engine := range Engines() {
		t.Run(engine, func(t *testing.T) {
			_, text := renderInstalled(t, document, engine, Options{})
			for _, want := range []string{"An opening paragraph.", "A closing paragraph."} {
				if !strings.Contains(text, want) {
					t.Errorf("PDF text lacks %q:\n%s", want, text)
				}
			}
			for _, page := range strings.Split(text, "\f") {
				if strings.Contains(page, "initial") && strings.Contains(page, "step40") && strings.Contains(page, "Forty steps in a column") {
					return
				}
			}
			t.Fatalf("no page holds the figure's first and last step with its caption:\n%s", text)
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
