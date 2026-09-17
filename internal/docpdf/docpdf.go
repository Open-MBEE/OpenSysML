package docpdf

import (
	_ "embed"
	"os"
	"path/filepath"

	"github.com/Open-MBEE/OpenSysML/internal/core/docir"
	"github.com/Open-MBEE/OpenSysML/internal/core/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

// Options are the deliverable choices of the PDF backend: the document
// options every engine applies, and the stylesheet choices the HTML-input
// engines take, meaning what they mean for -doc-form html.
type Options struct {
	// TitlePage puts the document title in a page of its own.
	TitlePage bool

	// TOC writes a table of contents ahead of the content.
	TOC bool

	// NumberSections numbers the section headings hierarchically.
	NumberSections bool

	// Theme names the HTML backend's bundled theme laid under the print
	// stylesheet; empty is the default sheet alone.
	Theme string

	// NoDefaultStylesheet leaves the HTML backend's default sheet out, and the
	// print stylesheet layered over it, so Stylesheets alone style the page.
	NoDefaultStylesheet bool

	// Stylesheets are the reader's, attached after the print stylesheet and
	// unlayered, so they override it as they override the HTML form.
	Stylesheets []docrender.Stylesheet

	// Lang is the document language, "en" when empty.
	Lang string

	// DiagramForm is the source graph-shaped diagrams are drawn from, Mermaid
	// when empty.
	DiagramForm view.Form
}

// PrintStylesheet is the PDF backend's print stylesheet: page geometry, the
// page counter, print fonts and breaks, over the HTML backend's default sheet
// in a later cascade layer so a reader's unlayered stylesheet still wins.
//
//go:embed print.css
var PrintStylesheet string

// The document files Render lays out in the working directory.
const (
	markdownFileName = "document.md"
	htmlFileName     = "document.html"
)

// Render lays an evaluated document out as PDF with the named engine: its
// graph-shaped diagrams drawn and its formulas typeset ahead of conversion,
// then the HTML backend's markup with the print stylesheet for an engine
// reading HTML, or the Markdown backend's text for one reading Markdown.
func Render(document *docir.Document, engine string, opts Options) ([]byte, error) {
	converter, err := EngineNamed(engine)
	if err != nil {
		return nil, err
	}
	if err := converter.Available(); err != nil {
		return nil, err
	}
	if err := checkOptions(converter, opts); err != nil {
		return nil, err
	}
	diagrams, err := docrender.Diagrams(document, opts.DiagramForm)
	if err != nil {
		return nil, err
	}
	formulas := docrender.Formulas(document)
	dir, err := os.MkdirTemp("", "opensysml-docpdf-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	images, err := drawDiagrams(dir, diagrams, opts.DiagramForm)
	if err != nil {
		return nil, err
	}
	math, err := renderFormulas(dir, formulas)
	if err != nil {
		return nil, err
	}
	doc := &Prepared{Dir: dir, MathCSS: math.css, Options: opts}
	switch converter.Capabilities().Input {
	case InputMarkdown:
		markdown, err := docrender.Markdown(document, docrender.MarkdownOptions{DiagramForm: opts.DiagramForm})
		if err != nil {
			return nil, err
		}
		doc.MarkdownFile = markdownFileName
		if err := os.WriteFile(filepath.Join(dir, doc.MarkdownFile), []byte(markdown), 0o600); err != nil {
			return nil, err
		}
		if doc.Filter, err = writeArtworkFilter(dir, opts.DiagramForm, images, math, docrender.Captions(document)); err != nil {
			return nil, err
		}
	case InputHTML:
		page, err := docrender.HTML(document, htmlOptions(opts, images, math))
		if err != nil {
			return nil, err
		}
		doc.HTMLFile = htmlFileName
		if err := os.WriteFile(filepath.Join(dir, doc.HTMLFile), []byte(page), 0o600); err != nil {
			return nil, err
		}
	}
	return converter.Convert(doc)
}

// checkOptions rejects the HTML backend's stylesheet choices for a converter
// reading Markdown, whose HTML carries none of the backend's classes.
func checkOptions(converter Converter, opts Options) error {
	if converter.Capabilities().Input != InputMarkdown {
		return nil
	}
	switch {
	case opts.Theme != "":
		return &Error{Kind: ErrorUnsupportedOption, Engine: converter.Name(), Option: "-html-theme"}
	case opts.NoDefaultStylesheet:
		return &Error{Kind: ErrorUnsupportedOption, Engine: converter.Name(), Option: "-html-no-default-css"}
	}
	return nil
}

// htmlOptions shapes the HTML backend's page for a print engine: the default
// sheet and theme, the print stylesheet over them, the KaTeX stylesheet when
// formulas were typeset, then the reader's sheets; the diagram images and
// typeset formulas take the place of source.
func htmlOptions(opts Options, images []string, math formulas) docrender.HTMLOptions {
	var sheets []docrender.Stylesheet
	if !opts.NoDefaultStylesheet {
		sheets = append(sheets, docrender.InlineStylesheet(PrintStylesheet))
	}
	if math.css != "" {
		sheets = append(sheets, docrender.LinkedStylesheet(math.css))
	}
	sheets = append(sheets, opts.Stylesheets...)
	return docrender.HTMLOptions{
		NoDefaultStylesheet: opts.NoDefaultStylesheet,
		Theme:               opts.Theme,
		Stylesheets:         sheets,
		TitlePage:           opts.TitlePage,
		TOC:                 opts.TOC,
		NumberSections:      opts.NumberSections,
		Lang:                opts.Lang,
		DiagramForm:         opts.DiagramForm,
		DiagramImages:       images,
		Math:                math.html,
	}
}

// drawDiagrams draws the document's graph-shaped diagrams into dir, returning
// one image file name per diagram in order; an empty name keeps that diagram
// as source. Mermaid source is drawn with mermaid-cli; a document whose
// diagrams are in another form, or has none, needs no diagram tool.
func drawDiagrams(dir string, diagrams []docrender.Diagram, form view.Form) ([]string, error) {
	if len(diagrams) == 0 {
		return nil, nil
	}
	sources := make([]string, len(diagrams))
	for i, diagram := range diagrams {
		sources[i] = diagram.Source
	}
	switch form {
	case "", view.FormMermaid:
		return renderDiagrams(dir, sources)
	}
	return make([]string, len(diagrams)), nil
}
