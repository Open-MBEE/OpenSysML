package docpdf

import (
	"bytes"
	"compress/zlib"
	"errors"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
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
	page, err := docrender.HTML(document, pageOptions(t, Options{}, dir, nil, typeset))
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
	diagrams, err := docrender.Diagrams(telescopeDocument(t), docrender.DiagramOptions{Form: view.FormDot})
	if err != nil {
		t.Fatal(err)
	}
	positioned := docrender.Diagram{Name: "placed", Form: view.FormDot, Source: "// kind: interconnection\n// layout: neato -n\ngraph G {\n  node [shape=box];\n  Pump [pos=\"0,0\"];\n  Tank [pos=\"200,100\"];\n  Pump -- Tank [label=\"supply\"];\n}"}
	images, err := drawDiagrams(dir, append(diagrams, positioned))
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

// TestRenderCameoDiagramWithInstalledGraphviz draws a fully positioned state
// machine in the cameo style through a real Graphviz: the pinned `neato -n2`
// layout, Cameo's Arial text, the frame header, a Style's own colours, the
// note with its anchor, and an arrowhead on a routed transition; then the PDF
// carries the figure rather than its DOT source.
func TestRenderCameoDiagramWithInstalledGraphviz(t *testing.T) {
	if _, err := graphvizTool.locate(""); err != nil {
		skipWithout(t, "Graphviz dot", err)
	}
	document := fixtureDocument(t, filepath.Join("testdata", "cameo_report.sysml"), "Instrument::CameoReport")
	diagrams, err := docrender.Diagrams(document, docrender.DiagramOptions{Form: view.FormDot, Style: view.StyleCameo})
	if err != nil {
		t.Fatal(err)
	}
	if len(diagrams) != 1 || !strings.Contains(diagrams[0].Source, "// layout: neato -n2\n") {
		t.Fatalf("cameo diagram is not pinned with neato -n2:\n%+v", diagrams)
	}
	dir := t.TempDir()
	images, err := drawDiagrams(dir, diagrams)
	if err != nil {
		t.Fatalf("drawDiagrams: %v", err)
	}
	if len(images) != 1 {
		t.Fatalf("images = %q", images)
	}
	if err := checkSVG(filepath.Join(dir, images[0])); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, images[0]))
	if err != nil {
		t.Fatal(err)
	}
	svg := string(raw)
	for _, want := range []string{
		`font-family="Arial"`, "font-size=\"11.00\"",
		">stm</text>", "State Machine",
		"do / MonitorPEAS", "InitializePEAS",
		"Runs once at power&#45;up.", `stroke-dasharray`,
		`fill="#f2dcdb"`, `stroke="#9c0006"`,
		"accept Start", "<polygon",
	} {
		if !strings.Contains(svg, want) {
			t.Errorf("cameo SVG lacks %q", want)
		}
	}
	if strings.Contains(svg, "Helvetica") || strings.Contains(svg, "«state»") {
		t.Errorf("cameo SVG carries the Pilot look")
	}
	if t.Failed() {
		t.Log(svg)
	}

	_, text := renderInstalled(t, document, "", Options{DiagramForm: view.FormDot, Style: view.StyleCameo})
	if strings.Contains(text, "digraph") || strings.Contains(text, dotNotice[:40]) {
		t.Fatalf("DOT source or its notice reached the PDF:\n%s", text)
	}
	if !strings.Contains(text, "PEAS states, as Cameo drew them") {
		t.Fatalf("diagram caption missing:\n%s", text)
	}
}

// TestRenderPositionedDiagramByDefaultWithInstalledGraphviz renders the
// positioned cameo fixture with no DiagramForm stated: the automatic choice
// picks DOT, Graphviz draws it, and the PDF carries the figure with no Mermaid
// fallback notice; Graphviz.Draw returns the same SVG for the inline backends.
func TestRenderPositionedDiagramByDefaultWithInstalledGraphviz(t *testing.T) {
	if !(Graphviz{}).Available() {
		_, err := graphvizTool.locate("")
		skipWithout(t, "Graphviz dot", err)
	}
	document := fixtureDocument(t, filepath.Join("testdata", "cameo_report.sysml"), "Instrument::CameoReport")
	diagrams, err := docrender.Diagrams(document, docrender.DiagramOptions{Style: view.StyleCameo})
	if err != nil {
		t.Fatal(err)
	}
	if len(diagrams) != 1 || diagrams[0].Form != view.FormDot || diagrams[0].Fallback != "" {
		t.Fatalf("automatic choice for a positioned view = %+v, want dot without fallback", diagrams)
	}
	svgs, err := Graphviz{}.Draw(diagrams)
	if err != nil {
		t.Fatalf("Graphviz.Draw: %v", err)
	}
	if len(svgs) != 1 || !strings.Contains(svgs[0], "<svg") || !strings.Contains(svgs[0], "do / MonitorPEAS") {
		t.Fatalf("Graphviz.Draw SVG = %q", svgs)
	}
	_, text := renderInstalled(t, document, "", Options{Style: view.StyleCameo})
	if strings.Contains(text, "digraph") || strings.Contains(text, "stateDiagram") || strings.Contains(text, "drawn as Mermaid") {
		t.Fatalf("source or a fallback notice reached the PDF:\n%s", text)
	}
	if !strings.Contains(text, "PEAS states, as Cameo drew them") {
		t.Fatalf("diagram caption missing:\n%s", text)
	}
}

// TestRenderNestedActionNotesWithInstalledGraphviz renders the nested-action
// report, whose notes anchor to a nested action's drawn node: the automatic
// choice picks DOT, and Graphviz draws it — an anchor to a node the drawing
// does not declare would make `dot -n` fail — with the note text in the SVG.
func TestRenderNestedActionNotesWithInstalledGraphviz(t *testing.T) {
	if !(Graphviz{}).Available() {
		_, err := graphvizTool.locate("")
		skipWithout(t, "Graphviz dot", err)
	}
	document := fixtureDocument(t, filepath.Join("testdata", "nested_notes_report.sysml"), "Nested::NestedNotesReport")
	diagrams, err := docrender.Diagrams(document, docrender.DiagramOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(diagrams) != 1 || diagrams[0].Form != view.FormDot || diagrams[0].Fallback != "" {
		t.Fatalf("automatic choice for a positioned view = %+v, want dot without fallback", diagrams)
	}
	svgs, err := Graphviz{}.Draw(diagrams)
	if err != nil {
		t.Fatalf("Graphviz.Draw: %v", err)
	}
	if len(svgs) != 1 || !strings.Contains(svgs[0], "<svg") || !strings.Contains(svgs[0], "these values") {
		t.Fatalf("Graphviz.Draw SVG = %q", svgs)
	}
}

// TestRenderPositionedDiagramFallsBackWithInstalledMermaid renders the Cameo
// report with Graphviz pointed nowhere: the positioned view falls back to a
// Mermaid drawing, the notice saying so and the caption both reach the PDF,
// through the default engine and through pandoc, whose filter marks the
// caption past the notice.
func TestRenderPositionedDiagramFallsBackWithInstalledMermaid(t *testing.T) {
	if _, err := mermaidTool.locate(""); err != nil {
		skipWithout(t, "mmdc", err)
	}
	t.Setenv(DotEnv, filepath.Join(t.TempDir(), "no-dot"))
	if (Graphviz{}).Available() {
		t.Fatal("Graphviz is available with OPENSYSML_DOT pointed at nothing")
	}
	document := fixtureDocument(t, filepath.Join("testdata", "cameo_report.sysml"), "Instrument::CameoReport")
	for _, engine := range []string{"", pandocTool.name} {
		t.Run(engine, func(t *testing.T) {
			_, text := renderInstalled(t, document, engine, Options{})
			for _, want := range []string{"PEAS states, as Cameo drew them", "drawn as Mermaid, not at its stated positions"} {
				if !strings.Contains(text, want) {
					t.Errorf("PDF text lacks %q:\n%s", want, text)
				}
			}
			if strings.Contains(text, "stateDiagram") || strings.Contains(text, "digraph") {
				t.Fatalf("diagram source reached the PDF:\n%s", text)
			}
		})
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
	diagrams, err := docrender.Diagrams(telescopeDocument(t), docrender.DiagramOptions{Form: view.FormPlantUML})
	if err != nil {
		t.Fatal(err)
	}
	images, err := drawDiagrams(dir, diagrams)
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
	rejected := []docrender.Diagram{{Name: "bad", Form: view.FormPlantUML, Source: "@startuml\nclass A\nA --> \n@enduml"}}
	_, err = drawDiagrams(t.TempDir(), rejected)
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

// TestRenderThemesWithInstalledEngines reads page size, embedded faces and body
// size back from each theme's PDF; pandoc keeps refusing themes with a typed error.
func TestRenderThemesWithInstalledEngines(t *testing.T) {
	if _, err := mermaidTool.locate(""); err != nil {
		skipWithout(t, "mmdc", err)
	}
	letter, a4 := [2]float64{612, 792}, [2]float64{595.3, 841.9}
	times := []string{"Times", "Liberation-Serif", "LiberationSerif", "Nimbus-Roman", "NimbusRoman"}
	arial := []string{"Arial", "Helvetica", "Liberation-Sans", "LiberationSans", "Nimbus-Sans", "NimbusSans"}
	courier := []string{"Courier", "Liberation-Mono", "LiberationMono", "Nimbus-Mono", "NimbusMono"}
	cases := []struct {
		theme string
		page  [2]float64
		body  float64
		faces [][]string
	}{
		{"", a4, 11, [][]string{times, arial, courier}},
		{"print", a4, 11, [][]string{times, courier}},
		{"report", a4, 12.8, [][]string{{"Charter", "SourceSerif", "Source-Serif", "Georgia", "Times", "Liberation-Serif", "LiberationSerif"}}},
		{"nasa", letter, 12, [][]string{times, arial, courier}},
		{"ieee", letter, 10, [][]string{times, courier}},
		{"acm", letter, 10, [][]string{{"Libertinus", "LinLibertine", "Linux-Libertine", "Times", "Liberation-Serif", "LiberationSerif"}, {"Libertinus", "LinBiolinum", "Linux-Biolinum", "Arial", "Helvetica", "Liberation-Sans", "LiberationSans"}, courier}},
	}
	telescope, prose := telescopeDocument(t), proseDocument(t)
	for _, engine := range Engines() {
		for _, tc := range cases {
			name := tc.theme
			if name == "" {
				name = docrender.DefaultTheme
			}
			t.Run(engine+"/"+name, func(t *testing.T) {
				opts := Options{Theme: tc.theme, TitlePage: true, TOC: true, NumberSections: true}
				if engine == pandocTool.name && tc.theme != "" {
					installedConverter(t, engine)
					_, err := Render(telescope, engine, opts)
					var docErr *Error
					if !errors.As(err, &docErr) || docErr.Kind != ErrorUnsupportedOption || docErr.Option != "-html-theme" {
						t.Fatalf("pandoc took -html-theme %s: %v", tc.theme, err)
					}
					return
				}
				pdf, _ := renderInstalled(t, telescope, engine, opts)
				for i, box := range pageBoxes(t, pdf) {
					if math.Abs(box[0]-tc.page[0]) > 0.5 || math.Abs(box[1]-tc.page[1]) > 0.5 {
						t.Errorf("page %d is %v points, want %v", i+1, box, tc.page)
					}
				}
				fonts := pdfFonts(t, pdf)
				for _, face := range tc.faces {
					if !fontAmong(fonts, face) {
						t.Errorf("no face of %v embedded; the PDF's fonts are %v", face, fonts)
					}
				}
				for _, font := range fonts {
					if strings.Contains(font, "DejaVu") {
						t.Errorf("a generic family fell through to %s; the PDF's fonts are %v", font, fonts)
					}
				}
				pdf, _ = renderInstalled(t, prose, engine, Options{Theme: tc.theme})
				sizes := pdfTextSizes(t, pdf)
				if got := dominantSize(sizes); math.Abs(got-tc.body) > 0.15 {
					t.Errorf("running text is set at %gpt, want %gpt; sizes %v", got, tc.body, sizes)
				}
			})
		}
	}
}

// TestRenderThemeTablesWithInstalledEngines reads back the size an ordinary
// (three-column, portrait) table's text is set at under each convention theme,
// the default's body size being the control.
func TestRenderThemeTablesWithInstalledEngines(t *testing.T) {
	cases := []struct {
		theme string
		table float64
	}{
		{"", 11},
		{"nasa", 11},
		{"ieee", 8},
		{"acm", 9},
	}
	document := narrowTableDocument(t)
	for _, engine := range Engines() {
		if engine == pandocTool.name {
			continue
		}
		for _, tc := range cases {
			name := tc.theme
			if name == "" {
				name = docrender.DefaultTheme
			}
			t.Run(engine+"/"+name, func(t *testing.T) {
				pdf, _ := renderInstalled(t, document, engine, Options{Theme: tc.theme})
				if pages := pageOrientations(t, pdf); len(pages) != 1 || pages[0] != "portrait" {
					t.Fatalf("pages are %v, want one portrait page", pages)
				}
				sizes := pdfTextSizes(t, pdf)
				if got := dominantSize(sizes); math.Abs(got-tc.table) > 0.15 {
					t.Errorf("table text is set at %gpt, want %gpt; sizes %v", got, tc.table, sizes)
				}
			})
		}
	}
}

// TestRenderNASAPageNumbersWithInstalledEngines reads the footers back from a
// nasa report opening with running text ahead of its first section, and one
// opening with a landscape table: front matter counts in roman, the body from 1.
func TestRenderNASAPageNumbersWithInstalledEngines(t *testing.T) {
	lead, wideFirst := leadDocument(t), wideFirstDocument(t)
	cases := []struct {
		name     string
		document *docir.Document
		opts     Options
		footers  []string
	}{
		{"body", lead, Options{Theme: "nasa"}, []string{"1", "2"}},
		{"toc", lead, Options{Theme: "nasa", TOC: true}, []string{"i", "1", "2"}},
		{"title-page", lead, Options{Theme: "nasa", TitlePage: true}, []string{"", "1", "2"}},
		{"title-page-toc", lead, Options{Theme: "nasa", TitlePage: true, TOC: true}, []string{"", "ii", "1", "2"}},
		{"wide-first-toc", wideFirst, Options{Theme: "nasa", TOC: true}, []string{"i", "1", "2"}},
		{"wide-first-title-page-toc", wideFirst, Options{Theme: "nasa", TitlePage: true, TOC: true}, []string{"", "ii", "1", "2"}},
	}
	for _, engine := range Engines() {
		if engine == pandocTool.name {
			continue
		}
		for _, tc := range cases {
			t.Run(engine+"/"+tc.name, func(t *testing.T) {
				_, text := renderInstalled(t, tc.document, engine, tc.opts)
				if got := pageFooters(text); !slices.Equal(got, tc.footers) {
					t.Fatalf("page footers are %q, want %q", got, tc.footers)
				}
			})
		}
	}
}

// pageFooters returns the last line of text on each page of pdftotext's
// layout output; a page whose last line is not a page number has "".
func pageFooters(text string) []string {
	number := regexp.MustCompile(`^(\d+|[ivxlc]+)$`)
	var footers []string
	for _, page := range strings.Split(strings.TrimSuffix(text, "\f"), "\f") {
		last := ""
		for _, line := range strings.Split(page, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				last = line
			}
		}
		if !number.MatchString(last) {
			last = ""
		}
		footers = append(footers, last)
	}
	return footers
}

// TestRenderGenericFamilyWithInstalledEngines is the named stacks' control: a
// page asking for bare serif gets fontconfig's DejaVu, the default sheet does not.
func TestRenderGenericFamilyWithInstalledEngines(t *testing.T) {
	fcMatch, err := exec.LookPath("fc-match")
	if err != nil {
		skipWithout(t, "fc-match", err)
	}
	out, err := exec.Command(fcMatch, "serif").Output() // #nosec G204 -- fc-match from PATH, fixed arguments
	if err != nil {
		t.Fatalf("fc-match: %v", err)
	}
	if !strings.Contains(string(out), "DejaVu") {
		t.Skipf("generic serif resolves to %q here, not DejaVu", strings.TrimSpace(string(out)))
	}
	generic := docrender.InlineStylesheet("body { font-family: serif }")
	for _, engine := range Engines() {
		if engine == pandocTool.name {
			continue
		}
		t.Run(engine, func(t *testing.T) {
			pdf, _ := renderInstalled(t, plainDocument(t), engine, Options{NoDefaultStylesheet: true, Stylesheets: []docrender.Stylesheet{generic}})
			if fonts := pdfFonts(t, pdf); !fontAmong(fonts, []string{"DejaVu"}) {
				t.Fatalf("generic serif did not resolve to DejaVu on the page; fonts %v", fonts)
			}
			pdf, _ = renderInstalled(t, plainDocument(t), engine, Options{})
			if fonts := pdfFonts(t, pdf); fontAmong(fonts, []string{"DejaVu"}) {
				t.Fatalf("the default sheet let a generic family through; fonts %v", fonts)
			}
		})
	}
}

// fontAmong reports whether any embedded font name carries one of the names.
func fontAmong(fonts, names []string) bool {
	for _, font := range fonts {
		for _, name := range names {
			if strings.Contains(font, name) {
				return true
			}
		}
	}
	return false
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
	var pages []string
	for _, box := range pageBoxes(t, pdf) {
		if box[0] > box[1] {
			pages = append(pages, "landscape")
		} else {
			pages = append(pages, "portrait")
		}
	}
	return pages
}

// pageBoxes reads each page's width and height in points from the /MediaBox
// entries of a PDF, the flate-compressed object streams included.
func pageBoxes(t *testing.T, pdf []byte) [][2]float64 {
	t.Helper()
	box := regexp.MustCompile(`/MediaBox \[\s*[-\d.]+\s+[-\d.]+\s+([-\d.]+)\s+([-\d.]+)\s*\]`)
	var pages [][2]float64
	for _, data := range pdfStreams(pdf) {
		for _, m := range box.FindAllSubmatch(data, -1) {
			width, errW := strconv.ParseFloat(string(m[1]), 64)
			height, errH := strconv.ParseFloat(string(m[2]), 64)
			if errW != nil || errH != nil {
				t.Fatalf("unreadable /MediaBox %q", m[0])
			}
			pages = append(pages, [2]float64{width, height})
		}
	}
	if len(pages) == 0 {
		t.Fatal("no /MediaBox found in the PDF")
	}
	return pages
}

// pdfStreams is a PDF's bytes followed by every flate stream in it inflated,
// so a regular expression sees the dictionaries and content streams alike.
func pdfStreams(pdf []byte) [][]byte {
	streams := [][]byte{pdf}
	rest := pdf
	for {
		i := bytes.Index(rest, []byte("stream\n"))
		if i < 0 {
			return streams
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
		streams = append(streams, data)
	}
}

// pdfFonts lists the /BaseFont names a PDF embeds, subset tags stripped,
// sorted and without repeats.
func pdfFonts(t *testing.T, pdf []byte) []string {
	t.Helper()
	base := regexp.MustCompile(`/BaseFont\s*/([^\s/>\]]+)`)
	subset := regexp.MustCompile(`^[A-Z]{6}\+`)
	seen := map[string]bool{}
	var fonts []string
	for _, data := range pdfStreams(pdf) {
		for _, m := range base.FindAllSubmatch(data, -1) {
			name := subset.ReplaceAllString(string(m[1]), "")
			if !seen[name] {
				seen[name] = true
				fonts = append(fonts, name)
			}
		}
	}
	if len(fonts) == 0 {
		t.Fatal("no /BaseFont found in the PDF")
	}
	sort.Strings(fonts)
	return fonts
}

// pdfTextSizes tallies a PDF's glyphs by the point size they are set at, each
// Tf size read through the text and graphics matrices scaling it.
func pdfTextSizes(t *testing.T, pdf []byte) map[float64]int {
	t.Helper()
	sizes := map[float64]int{}
	for _, data := range pdfStreams(pdf) {
		if !bytes.Contains(data, []byte(" Tf")) || !bytes.Contains(data, []byte("BT")) {
			continue
		}
		tallyTextSizes(sizes, string(data))
	}
	if len(sizes) == 0 {
		t.Fatal("no text found in the PDF's content streams")
	}
	return sizes
}

// tallyTextSizes walks one content stream, tracking the q/Q stack of cm
// scales, the Tm scale and the Tf size, and counts each shown glyph.
func tallyTextSizes(sizes map[float64]int, content string) {
	scale := 1.0
	var stack []float64
	text, size := 1.0, 0.0
	var operands []string
	hex := regexp.MustCompile(`<([0-9A-Fa-f]*)>`)
	number := func(i int) float64 {
		if i < 0 || i >= len(operands) {
			return 0
		}
		v, _ := strconv.ParseFloat(operands[i], 64)
		return v
	}
	for _, token := range strings.Fields(content) {
		switch token {
		case "q":
			stack = append(stack, scale)
		case "Q":
			if n := len(stack); n > 0 {
				scale, stack = stack[n-1], stack[:n-1]
			}
		case "cm":
			scale *= math.Hypot(number(len(operands)-6), number(len(operands)-5))
		case "BT":
			text = 1
		case "Tm":
			text = math.Hypot(number(len(operands)-6), number(len(operands)-5))
		case "Tf":
			size = number(len(operands) - 1)
		case "Tj", "TJ", "'", `"`:
			glyphs := 0
			for _, m := range hex.FindAllStringSubmatch(strings.Join(operands, ""), -1) {
				glyphs += len(m[1]) / 4
			}
			if glyphs > 0 {
				sizes[math.Round(size*text*scale*10)/10] += glyphs
			}
		default:
			operands = append(operands, token)
			continue
		}
		operands = operands[:0]
	}
}

// dominantSize is the point size most of a PDF's glyphs are set at.
func dominantSize(sizes map[float64]int) float64 {
	best, most := 0.0, -1
	for size, count := range sizes {
		if count > most || count == most && size < best {
			best, most = size, count
		}
	}
	return best
}

// TestRenderImageBlocksWithInstalledEngines checks each converter draws the
// image a sourceless document names beside the output: pdfimages lists it.
func TestRenderImageBlocksWithInstalledEngines(t *testing.T) {
	base := t.TempDir()
	writeMark(t, filepath.Join(base, "images", "mark.png"))
	document := imageDocSource(t, "<stdin>", `"images/mark.png"`)
	for _, engine := range Engines() {
		t.Run(engine, func(t *testing.T) {
			pdf, text := renderInstalled(t, document, engine, Options{BaseDir: base})
			if !strings.Contains(text, "The survey mark") {
				t.Errorf("caption missing:\n%s", text)
			}
			images := pdfImages(t, pdf)
			if !regexp.MustCompile(`(?m)^\s*1\s+0\s+image\s+`).MatchString(images) {
				t.Errorf("the image beside the PDF was not drawn:\n%s", images)
			}
		})
	}
}

// TestRenderNumberedCaptionsWithInstalledEngines checks every converter lays
// out the same numbered captions, the image resolved beside the source.
func TestRenderNumberedCaptionsWithInstalledEngines(t *testing.T) {
	if _, err := mermaidTool.locate(""); err != nil {
		skipWithout(t, "mmdc", err)
	}
	base := t.TempDir()
	writeMark(t, filepath.Join(base, "images", "mark.png"))
	content, err := os.ReadFile(filepath.Join("..", "docrender", "testdata", "numbered_report.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	document := sourceDocument(t, filepath.Join(base, "numbered_report.sysml"), string(content), "Numbered::NumberedReport")
	for _, engine := range Engines() {
		t.Run(engine, func(t *testing.T) {
			_, text := renderInstalled(t, document, engine, Options{BaseDir: t.TempDir(), NumberFigures: true, DiagramForm: view.FormMermaid})
			for _, want := range []string{
				"Table 1. Optical parts",
				"Figure 1. How the parts connect",
				"Collecting area",
				"Figure 2",
				"Table 2",
				"Table 3. The parts as rows",
				"Figure 3. The survey mark",
			} {
				if !strings.Contains(text, want) {
					t.Errorf("PDF text lacks %q:\n%s", want, text)
				}
			}
			if strings.Contains(text, "Figure 4") || strings.Contains(text, "Table 4") || strings.Contains(text, "*") {
				t.Errorf("PDF text carries a stray number or emphasis marker:\n%s", text)
			}
		})
	}
}

// TestRenderTwentyColumnTableWithInstalledEngines renders a sized
// twenty-column table of two dozen rows through each installed converter and
// reads back the wide-table policy: the column set split into continuation
// tables of at most DefaultTableColumns, every one on landscape pages that
// each repeat the header and the row-naming first column, the row names set
// whole, and no page left to a fragment of a row or two.
func TestRenderTwentyColumnTableWithInstalledEngines(t *testing.T) {
	const rows = 24
	document := wideResultsDocument(t, rows)
	for _, engine := range Engines() {
		t.Run(engine, func(t *testing.T) {
			pdf, text := renderInstalled(t, document, engine, Options{NumberFigures: true})
			orientations := pageOrientations(t, pdf)
			pages := strings.Split(strings.TrimRight(text, "\f\n"), "\f")
			if len(pages) != len(orientations) {
				t.Fatalf("pdftotext reads %d pages, /MediaBox %d", len(pages), len(orientations))
			}
			if len(pages) > 5 {
				t.Fatalf("%d pages for %d rows over two continuation tables:\n%s", len(pages), rows, text)
			}
			landscape, continued := 0, 0
			inContinuation := false
			for i, page := range pages {
				names := strings.Count(page, "Alignment Scenario")
				if names == 0 {
					continue
				}
				if orientations[i] != "landscape" {
					t.Errorf("page %d holds table rows in %s", i+1, orientations[i])
				}
				landscape++
				if names < 3 {
					t.Errorf("page %d holds a fragment of %d rows", i+1, names)
				}
				if head := page[:strings.Index(page, "Alignment Scenario")]; !strings.Contains(head, "name") {
					t.Errorf("page %d repeats no header ahead of its rows:\n%s", i+1, page)
				}
				first, rest, split := strings.Cut(page, "(continued)")
				if split {
					continued++
				} else if inContinuation {
					first, rest = "", page
				}
				if strings.Contains(first, "tAcquisition") {
					t.Errorf("page %d sets the last column in the first table:\n%s", i+1, page)
				}
				if strings.Contains(rest, "postSegXchgTimeLimit") {
					t.Errorf("page %d sets the first value column in a continuation table:\n%s", i+1, page)
				}
				inContinuation = inContinuation || split
			}
			if landscape < 2 || continued == 0 {
				t.Fatalf("%d landscape table pages, %d continued; want the second table on its own pages:\n%s", landscape, continued, text)
			}
			if n := strings.Count(text, "Alignment Scenario 24"); n != 2 {
				t.Errorf("the last row's name is set %d times, want once per table:\n%s", n, text)
			}
			if n := strings.Count(text, "Alignment timing results"); n != 2 {
				t.Errorf("the caption is set %d times, want once per table:\n%s", n, text)
			}
			if !strings.Contains(text, "Table 1. Alignment timing results (continued)") {
				t.Errorf("the continuation keeps no caption number:\n%s", text)
			}
		})
	}
}
