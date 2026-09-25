package docpdf

import (
	_ "embed" // for the //go:embed directives below
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docir"
	"github.com/Open-MBEE/OpenSysML/internal/doc/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
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
	// stylesheet, with the theme's print companion, when it carries one, laid
	// over the print stylesheet; empty is the default sheet alone.
	Theme string

	// NoDefaultStylesheet leaves the HTML backend's default sheet out, and the
	// print stylesheet layered over it, so Stylesheets alone style the page.
	NoDefaultStylesheet bool

	// Stylesheets are the reader's, attached after the bundled sheets and
	// unlayered, so they override them as they override the HTML form.
	Stylesheets []docrender.Stylesheet

	// BaseDir is the directory a reader stylesheet's relative url() and
	// @import references resolve against, the current directory when empty:
	// the PDF's own, as an HTML page's sheets resolve against the page's.
	BaseDir string

	// Lang is the document language, "en" when empty.
	Lang string

	// DiagramForm is the source graph-shaped diagrams are drawn from, Mermaid
	// when empty.
	DiagramForm view.Form

	// Unplaced is where a diagram some Layout positions puts the nodes none
	// does: left undrawn when empty, or drawn too (a strip below a DOT drawing).
	Unplaced view.Unplaced

	// Style is the drawing style every DOT diagram is drawn in, the Pilot look
	// when empty.
	Style view.DrawingStyle
}

// PrintStylesheet is the PDF backend's print stylesheet: page geometry, the
// page counter, print fonts and breaks, over the HTML backend's default sheet
// in a later cascade layer so a reader's unlayered stylesheet still wins. It
// declares the layer a theme's print companion fills after its own.
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
	if err := checkOptions(converter, opts); err != nil {
		return nil, err
	}
	if err := converter.Available(); err != nil {
		return nil, err
	}
	diagrams, err := docrender.Diagrams(document, opts.DiagramForm, opts.Unplaced, opts.Style)
	if err != nil {
		return nil, err
	}
	formulas := docrender.Formulas(document)
	dir, err := os.MkdirTemp("", "opensysml-docpdf-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	drawn, err := drawDiagrams(dir, diagrams, opts.DiagramForm)
	if err != nil {
		return nil, err
	}
	images := fileRefs(dir, drawn)
	math, err := renderFormulas(dir, formulas)
	if err != nil {
		return nil, err
	}
	base, err := filepath.Abs(opts.BaseDir)
	if err != nil {
		return nil, err
	}
	doc := &Prepared{Dir: dir, MathCSS: math.css, BaseDir: base, Options: opts}
	switch converter.Capabilities().Input {
	case InputMarkdown:
		markdown, err := docrender.Markdown(document, docrender.MarkdownOptions{DiagramForm: opts.DiagramForm, Unplaced: opts.Unplaced, Style: opts.Style})
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
		htmlOpts, err := htmlOptions(opts, dir, images, math)
		if err != nil {
			return nil, err
		}
		page, err := docrender.HTML(document, htmlOpts)
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

// checkOptions rejects a malformed reader stylesheet for every converter, and
// the HTML backend's stylesheet choices for a converter reading Markdown,
// whose HTML carries none of the backend's classes.
func checkOptions(converter Converter, opts Options) error {
	for _, sheet := range opts.Stylesheets {
		if err := sheet.Check(); err != nil {
			return err
		}
	}
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
// sheet and theme, the print stylesheet over them, the theme's print
// companion over that, the KaTeX stylesheet when formulas were typeset, then
// the reader's sheets; the diagram images and typeset formulas take the place
// of source. The page's base is the reader's directory, so the working
// directory's files are referenced by file URL.
func htmlOptions(opts Options, dir string, images []string, math formulas) (docrender.HTMLOptions, error) {
	var sheets []docrender.Stylesheet
	if !opts.NoDefaultStylesheet {
		sheets = append(sheets, docrender.InlineStylesheet(PrintStylesheet))
		companion, err := docrender.ThemePrintStylesheet(opts.Theme)
		if err != nil {
			return docrender.HTMLOptions{}, err
		}
		if companion != "" {
			sheets = append(sheets, docrender.InlineStylesheet(companion))
		}
	}
	if math.css != "" {
		sheets = append(sheets, docrender.LinkedStylesheet(fileURL(filepath.Join(dir, math.css))))
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
		Unplaced:            opts.Unplaced,
		Style:               opts.Style,
		DiagramImages:       images,
		Math:                math.html,
	}, nil
}

// fileRefs is the file URL of each named file within dir, in order; an empty
// name stays empty.
func fileRefs(dir string, names []string) []string {
	refs := make([]string, len(names))
	for i, name := range names {
		if name != "" {
			refs[i] = fileURL(filepath.Join(dir, name))
		}
	}
	return refs
}

// dirURL is the file URL of an absolute directory with a trailing slash, so
// relative references resolve within it.
func dirURL(dir string) string {
	return strings.TrimSuffix(fileURL(dir), "/") + "/"
}

// fileURL is the file URL of an absolute path.
func fileURL(path string) string {
	p := filepath.ToSlash(path)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return (&url.URL{Scheme: "file", Path: p}).String()
}
