package docrender

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docir"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

// This file lists what a document shows that a backend may draw or typeset
// out of process — its graph-shaped diagrams and its formulas — and the
// captions it sets, in the order and with the sources the HTML and Markdown
// backends write them.

// Diagram is one graph-shaped diagram of a document: its declared name and
// its source in the requested diagram form.
type Diagram struct {
	Name   string
	Source string
}

// Diagrams lists the document's graph-shaped diagrams in document order, each
// with the source the backends write for it in form (Mermaid when empty) with
// unplaced nodes placed as unplaced says. A table-kind view is a table, not a
// diagram, and is left out.
func Diagrams(document *docir.Document, form view.Form, unplaced view.Unplaced, style view.DrawingStyle) ([]Diagram, error) {
	if document == nil {
		return nil, &Error{Kind: ErrorNilDocument}
	}
	resolved, err := diagramForm(form)
	if err != nil {
		return nil, err
	}
	if err := checkStyle(style); err != nil {
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
			source, err := diagramSource(node.Name(), rendering, figureOptions(node, unplaced, style), resolved)
			if err != nil {
				return err
			}
			diagrams = append(diagrams, Diagram{Name: node.Name(), Source: source})
		}
		return nil
	}
	if err := walk(document.Content()); err != nil {
		return nil, err
	}
	return diagrams, nil
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

// Captions lists the document's table, diagram and formula captions in
// document order: each is the emphasized paragraph the Markdown backend
// writes ahead of its block, for a consumer telling a caption from a
// paragraph that happens to be emphasized. Each is listed as written, without
// surrounding blanks; a blank caption is written nowhere and listed nowhere.
func Captions(document *docir.Document) []string {
	if document == nil {
		return nil
	}
	var captions []string
	var walk func(nodes []docir.Content)
	walk = func(nodes []docir.Content) {
		for _, node := range nodes {
			switch node.Kind() {
			case docir.ContentTable, docir.ContentDiagram, docir.ContentFormula:
				if caption := strings.TrimSpace(node.Caption()); caption != "" {
					captions = append(captions, caption)
				}
			}
			walk(node.Children())
		}
	}
	walk(document.Content())
	return captions
}
