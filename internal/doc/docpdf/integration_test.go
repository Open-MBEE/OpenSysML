package docpdf

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docir"
	"github.com/Open-MBEE/OpenSysML/internal/doc/docrender"
)

// installedConverter returns the named converter, skipping the test when a
// tool it needs is not installed; the contract itself is tested with fakes.
func installedConverter(t *testing.T, engine string) Converter {
	t.Helper()
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
	return converter
}

// renderInstalled renders document with an installed engine and returns the
// PDF's text as pdftotext extracts it, skipping when pdftotext is absent.
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

// pdfText extracts a PDF's text with pdftotext, "" when it is not installed.
func pdfText(t *testing.T, pdf []byte) string {
	t.Helper()
	pdftotext, err := exec.LookPath("pdftotext")
	if err != nil {
		return ""
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

// TestRenderWithInstalledEngines exercises each real converter when its tools
// are installed, and skips otherwise.
func TestRenderWithInstalledEngines(t *testing.T) {
	document := plainDocument(t)
	for _, engine := range Engines() {
		t.Run(engine, func(t *testing.T) {
			_, text := renderInstalled(t, document, engine, Options{TOC: true, NumberSections: true})
			if text != "" && !strings.Contains(text, "One paragraph.") {
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
		t.Skipf("mmdc not installed: %v", err)
	}
	document := telescopeDocument(t)
	for _, engine := range Engines() {
		t.Run(engine, func(t *testing.T) {
			_, text := renderInstalled(t, document, engine, Options{TitlePage: true, TOC: true, NumberSections: true})
			if text == "" {
				t.Skip("pdftotext not installed")
			}
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
			if text == "" {
				t.Skip("pdftotext not installed")
			}
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
		t.Skipf("katex not installed: %v", err)
	}
	document := mathDocument(t)
	dir := t.TempDir()
	typeset, err := renderFormulas(dir, docrender.Formulas(document))
	if err != nil {
		t.Fatalf("renderFormulas: %v", err)
	}
	page, err := docrender.HTML(document, htmlOptions(Options{}, nil, typeset))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<link rel="stylesheet" href="katex/katex.min.css">`,
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
			if text == "" {
				t.Skip("pdftotext not installed")
			}
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
			if text == "" {
				t.Skip("pdftotext not installed")
			}
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
		t.Skipf("mmdc not installed: %v", err)
	}
	_, text := renderInstalled(t, telescopeDocument(t), "", Options{})
	if text != "" && !strings.Contains(text, "Imaging chain interconnection") {
		t.Fatalf("diagram caption missing:\n%s", text)
	}
}
