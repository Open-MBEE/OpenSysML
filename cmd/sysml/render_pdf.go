//go:build !sysml_prod && !sysml_nodocpdf

package main

import (
	"flag"
	"path/filepath"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docpdf"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
	_ "github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext/graphviz" // registers the Graphviz REPL commands
)

func init() {
	docpdfFeature.link(func(fs *flag.FlagSet) {
		fs.BoolVar(&pdfTitlePage, "doc-title-page", false, "Put the document title on a page of its own (html or pdf)")
		fs.BoolVar(&pdfTOC, "doc-toc", false, "Write a table of contents ahead of the content (html or pdf)")
		fs.BoolVar(&pdfNumbering, "doc-number-sections", false, "Number the section headings hierarchically (html or pdf)")
		fs.StringVar(&pdfEngine, "pdf-engine", "", "Converter -doc-form pdf drives: weasyprint (default), pandoc or prince")
		fs.StringVar(&htmlTheme, "html-theme", "", "Style the HTML page or PDF with a bundled theme layered over the default stylesheet: default, acm, ieee, modern, nasa, print or report")
		fs.Var(&htmlCSS, "html-css", "Style the HTML or PDF with this stylesheet too: a file is inlined, a URL is linked (repeatable, applied in order after the default sheet)")
		fs.BoolVar(&htmlNoCSS, "html-no-default-css", false, "Leave the default stylesheet out, so only -html-css sheets style the HTML or PDF")
		fs.BoolVar(&htmlFragment, "html-fragment", false, "Write the document element alone, without the page shell or a stylesheet, to embed in a page of your own")
		fs.BoolVar(&htmlShowCSS, "html-default-css", false, "Write the default document stylesheet, or with -html-theme that theme's whole sheet, and exit")
		fs.StringVar(&htmlMermaid, "html-mermaid", "", "Have the page load Mermaid to draw its diagrams: cdn loads a pinned release from jsDelivr, a URL the script it names")
		fs.StringVar(&htmlMath, "html-math", "", "Have the page load MathJax to typeset its formulas: cdn loads a pinned release from jsDelivr, a URL the script it names")
		fs.BoolVar(&pdfTitlePage, "pdf-title-page", false, "Former name of -doc-title-page, which also shapes HTML")
		fs.BoolVar(&pdfTOC, "pdf-toc", false, "Former name of -doc-toc, which also shapes HTML")
		fs.BoolVar(&pdfNumbering, "pdf-number-sections", false, "Former name of -doc-number-sections, which also shapes HTML")
	})
}

// renderPDF lays the -render-document document out as a PDF written to -o.
func renderPDF(sess *repl.Session) error {
	opts, err := pdfOptions()
	if err != nil {
		return err
	}
	document, err := sess.EvaluateDocument(renderDoc)
	if err != nil {
		return err
	}
	pdf, err := docpdf.Render(document, pdfEngine, opts)
	if err != nil {
		return err
	}
	return writePDFArtifact(pdf)
}

// pdfOptions resolves the PDF flags: the deliverable options and the
// stylesheet options, which reach the PDF as they reach an HTML page, their
// relative references resolving against the PDF's directory.
func pdfOptions() (docpdf.Options, error) {
	page, err := htmlOptions()
	if err != nil {
		return docpdf.Options{}, err
	}
	return docpdf.Options{
		TitlePage:           pdfTitlePage,
		TOC:                 pdfTOC,
		NumberSections:      pdfNumbering,
		NumberFigures:       docNumberFigures,
		Theme:               page.Theme,
		NoDefaultStylesheet: page.NoDefaultStylesheet,
		Stylesheets:         page.Stylesheets,
		BaseDir:             filepath.Dir(outputPath),
		DiagramForm:         page.DiagramForm,
		Unplaced:            page.Unplaced,
		Style:               page.Style,
	}, nil
}
