package docrender

import (
	"net/url"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docir"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// This file lists what a document shows that a backend may draw or typeset
// out of process — its graph-shaped diagrams and its formulas — and the
// captions it sets, in the order and with the sources the HTML and Markdown
// backends write them.

// Diagram is one graph-shaped diagram of a document: its declared name, the
// form it is written in, its source in that form, and, when the form is the
// automatic choice's Mermaid fallback, the reason stated in the output.
type Diagram struct {
	Name     string
	Form     view.Form
	Source   string
	Fallback string
}

// DiagramOptions is how a render writes its graph-shaped diagrams: the part
// of MarkdownOptions and HTMLOptions Diagrams lists them by.
type DiagramOptions struct {
	// Form is the form every diagram is written in; empty picks per diagram:
	// DOT for a rendering a Layout or Route positions, Mermaid otherwise.
	Form view.Form

	// WithoutGraphviz records that no Graphviz draws DOT for this render, so
	// the automatic choice falls back to Mermaid for positioned diagrams too,
	// stating so in the output. An explicit Form is written regardless.
	WithoutGraphviz bool

	// Unplaced is where a diagram some Layout positions puts the nodes none
	// does: left undrawn when empty, or drawn too (a strip below a DOT drawing).
	Unplaced view.Unplaced

	// Style is the drawing style every DOT diagram is drawn in, the Pilot look
	// when empty; the other forms draw one look.
	Style view.DrawingStyle
}

// check rejects a form there is none of and a drawing style there is none of.
func (o DiagramOptions) check() error {
	if o.Form != "" && !slices.Contains(view.DiagramForms(), o.Form) {
		return &Error{Kind: ErrorUnknownForm, DiagramForm: o.Form}
	}
	return checkStyle(o.Style)
}

// GraphvizFallbackNotice is the emphasized paragraph written between a
// positioned diagram's caption and its Mermaid source when no Graphviz drew it.
const GraphvizFallbackNotice = "drawn as Mermaid, not at its stated positions: Graphviz (dot) is not installed"

// formFor is the form rendering is written in: the one asked for, else DOT
// for a positioned view and Mermaid for the rest — and for a positioned view
// too without Graphviz, a fallback the output states.
func (o DiagramOptions) formFor(rendering *view.Rendering) (form view.Form, fallback string) {
	switch {
	case o.Form != "":
		return o.Form, ""
	case !rendering.Positioned():
		return view.FormMermaid, ""
	case o.WithoutGraphviz:
		return view.FormMermaid, GraphvizFallbackNotice
	}
	return view.FormDot, ""
}

// DiagramDrawer draws a document's DOT diagrams to SVG ahead of a render, for
// the HTML and Markdown backends to write inline where the automatic choice
// picks DOT; the PDF backend's Graphviz is one.
type DiagramDrawer interface {
	// Available reports whether Graphviz is installed to draw with.
	Available() bool
	// Draw is the SVG of each DOT diagram, in order, empty for the others.
	Draw(diagrams []Diagram) ([]string, error)
}

// drawAutomatic settles the automatic choice for a render given a drawer: with
// Graphviz at hand, every positioned diagram's SVG; without one, the Mermaid
// fallback. An explicit form, SVG drawn already, or no drawer (a caller that
// drew the diagrams itself and set WithoutGraphviz) is left as it is.
func drawAutomatic(document *docir.Document, opts *DiagramOptions, drawer DiagramDrawer, svg *[]string) error {
	if opts.Form != "" || *svg != nil || drawer == nil {
		return nil
	}
	if !drawer.Available() {
		opts.WithoutGraphviz = true
		return nil
	}
	diagrams, err := Diagrams(document, *opts)
	if err != nil {
		return err
	}
	if len(diagrams) == 0 {
		return nil
	}
	*svg, err = drawer.Draw(diagrams)
	return err
}

// Diagrams lists the document's graph-shaped diagrams in document order, each
// with the source the backends write for it as opts says. A table-kind view
// is a table, not a diagram, and is left out.
func Diagrams(document *docir.Document, opts DiagramOptions) ([]Diagram, error) {
	if document == nil {
		return nil, &Error{Kind: ErrorNilDocument}
	}
	if err := opts.check(); err != nil {
		return nil, err
	}
	var diagrams []Diagram
	var walk func(nodes []docir.Content) error
	walk = func(nodes []docir.Content) error {
		for _, node := range nodes {
			if node.Kind() != docir.ContentDiagram {
				if err := walk(node.Children()); err != nil {
					return err
				}
				continue
			}
			rendering := node.Rendering()
			if rendering == nil {
				return &Error{Kind: ErrorMissingRendering, Content: node.Name()}
			}
			if rendering.Kind == view.KindTable {
				continue
			}
			if !rendering.Kind.Supported() {
				return &Error{Kind: ErrorUnrenderableDiagram, Content: node.Name(), Actual: string(rendering.Kind)}
			}
			form, fallback := opts.formFor(rendering)
			source, err := diagramSource(node.Name(), rendering, figureOptions(node, opts), form)
			if err != nil {
				return err
			}
			diagrams = append(diagrams, Diagram{Name: node.Name(), Form: form, Source: source, Fallback: fallback})
		}
		return nil
	}
	if err := walk(document.Content()); err != nil {
		return nil, err
	}
	return diagrams, nil
}

// Image is one image block of a document: its name, the location it shows,
// the directory of the source file stating that location ("" when the
// source is no file on disk), its alt text and caption.
type Image struct {
	Name     string
	Location string
	Dir      string
	Alt      string
	Caption  string
}

// Images lists the image blocks of a document in document order.
func Images(document *docir.Document) []Image {
	if document == nil {
		return nil
	}
	var images []Image
	var walk func(nodes []docir.Content)
	walk = func(nodes []docir.Content) {
		for _, node := range nodes {
			if node.Kind() == docir.ContentImage {
				images = append(images, imageOf(node))
			}
			walk(node.Children())
		}
	}
	walk(document.Content())
	return images
}

// imageOf is the image block a content node is.
func imageOf(node docir.Content) Image {
	return Image{Name: node.Name(), Location: node.Location(), Dir: source.Dir(node.Origin().Doc), Alt: node.Alt(), Caption: node.Caption()}
}

// Remote reports a location rendered where it stands: an http(s) URL an
// engine or browser fetches, or a data URL carrying the image itself.
func (i Image) Remote() bool {
	lower := strings.ToLower(i.Location)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "data:")
}

// Relative reports a location that is a relative path: one stated relative
// to the document's source file, as DocumentQueries::Image documents.
func (i Image) Relative() bool {
	if i.Remote() || strings.HasPrefix(i.Location, "/") || filepath.IsAbs(i.Location) {
		return false
	}
	u, err := url.Parse(i.Location)
	return err != nil || u.Scheme == ""
}

// Path is the file a local location names: relative to the source file's directory,
// else to base; absolute paths and file URLs as written; "" when remote.
func (i Image) Path(base string) string {
	switch {
	case i.Remote():
		return ""
	case !i.Relative():
		if u, err := url.Parse(i.Location); err == nil && u.Scheme == "file" {
			return filepath.FromSlash(u.Path)
		}
		return i.Location
	case i.Dir != "":
		return filepath.Join(i.Dir, filepath.FromSlash(i.Location))
	}
	return filepath.Join(base, filepath.FromSlash(i.Location))
}

// Source is how a rendering written into outputDir refers to the image: a source
// file's relative path made relative to outputDir; anything else as stated.
func (i Image) Source(outputDir string) string {
	if outputDir == "" || i.Dir == "" || !i.Relative() {
		return i.Location
	}
	target := i.Path("")
	if rel, err := relativePath(outputDir, target); err == nil {
		return filepath.ToSlash(rel)
	}
	if abs, err := filepath.Abs(target); err == nil {
		return filepath.ToSlash(abs)
	}
	return filepath.ToSlash(target)
}

// relativePath is target relative to dir, the two made absolute first so a
// relative and an absolute path compare.
func relativePath(dir, target string) (string, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	return filepath.Rel(absDir, absTarget)
}

// Formula is one formula of a document as the HTML backend keys it: its LaTeX
// source, trimmed, and whether it is displayed on a line of its own.
type Formula struct {
	Source  string
	Display bool
}

// Formulas lists the document's distinct formulas in document order: every
// math run of its paragraphs, list items and definitions, and every formula
// block. Each key is one HTMLOptions.Math may carry typeset HTML for.
func Formulas(document *docir.Document) []Formula {
	if document == nil {
		return nil
	}
	var list []Formula
	seen := map[Formula]bool{}
	add := func(m Formula) {
		if !seen[m] {
			seen[m] = true
			list = append(list, m)
		}
	}
	runs := func(runs []docir.TextRun) {
		for _, run := range runs {
			if run.Kind() == docir.RunMath {
				add(inlineFormula(run.Text()))
			}
		}
	}
	var walk func(nodes []docir.Content)
	walk = func(nodes []docir.Content) {
		for _, node := range nodes {
			switch node.Kind() {
			case docir.ContentParagraph:
				runs(node.Runs())
			case docir.ContentList:
				for _, item := range node.Items() {
					runs(item.Runs())
				}
			case docir.ContentDefinitions:
				for _, entry := range node.Definitions() {
					runs(entry.Term())
					runs(entry.Description())
				}
			case docir.ContentFormula:
				add(displayFormula(node.Source()))
			}
			walk(node.Children())
		}
	}
	walk(document.Content())
	return list
}

// TeX is the formula as a TeX engine reads it and as the Markdown backend
// writes it between its delimiters: trimmed, blank lines dropped, a bare
// dollar escaped so it typesets as a dollar rather than closing math mode.
func (f Formula) TeX() string {
	if f.Display {
		return displayMath(f.Source)
	}
	return inlineMath(f.Source)
}

// inlineFormula keys an inline math run: trimmed, its newlines folded, as the
// HTML backend writes it between inline delimiters.
func inlineFormula(source string) Formula {
	return Formula{Source: strings.TrimSpace(strings.ReplaceAll(newlineNormalizer.Replace(source), "\n", " "))}
}

// displayFormula keys a formula block: trimmed with its line breaks kept, as
// the HTML backend writes it between display delimiters.
func displayFormula(source string) Formula {
	return Formula{Source: strings.TrimSpace(newlineNormalizer.Replace(source)), Display: true}
}

// Captions lists the document's table, diagram, formula and image captions in
// document order: each is the emphasized paragraph the Markdown backend
// writes ahead of its block, for a consumer telling a caption from a
// paragraph that happens to be emphasized. Each is listed as written, without
// surrounding blanks; a blank caption is written nowhere and listed nowhere.
// With numbered set, as MarkdownOptions.NumberFigures, the figures and tables
// are listed with their numbers, a block without a caption by its number alone.
func Captions(document *docir.Document, numbered bool) []string {
	if document == nil {
		return nil
	}
	var captions []string
	numbers := captionNumbering{on: numbered}
	var walk func(nodes []docir.Content)
	walk = func(nodes []docir.Content) {
		for _, node := range nodes {
			switch node.Kind() {
			case docir.ContentTable, docir.ContentDiagram, docir.ContentFormula, docir.ContentImage:
				if caption := strings.TrimSpace(numbers.caption(node).String()); caption != "" {
					captions = append(captions, caption)
				}
			}
			walk(node.Children())
		}
	}
	walk(document.Content())
	return captions
}
