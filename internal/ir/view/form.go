package view

import (
	"errors"
	"fmt"
	"strings"
)

// Form is a written form of a rendering: the human-readable text every kind has,
// the machine-readable form of the kind — a Mermaid diagram for the
// graph-shaped kinds, a Markdown table for the tabular one — and the Graphviz
// DOT and PlantUML forms a graph-shaped kind can be asked for instead.
type Form string

const (
	// FormText is the human-readable form, which the REPL prints.
	FormText Form = "text"
	// FormMermaid is a Mermaid diagram of a graph-shaped rendering.
	FormMermaid Form = "mermaid"
	// FormMarkdown is a Markdown table of a tabular rendering.
	FormMarkdown Form = "markdown"
	// FormDot is a Graphviz DOT digraph of a graph-shaped rendering.
	FormDot Form = "dot"
	// FormPlantUML is a PlantUML diagram of a graph-shaped or sequence rendering.
	FormPlantUML Form = "plantuml"
)

// Forms are the forms a rendering can be asked for, in the order they are
// offered.
func Forms() []Form { return []Form{FormText, FormMermaid, FormMarkdown, FormDot, FormPlantUML} }

// DiagramForms are the forms a document render writes its graph-shaped
// diagrams as; a table-kind view is written as a table whichever is chosen.
func DiagramForms() []Form { return []Form{FormMermaid, FormDot, FormPlantUML} }

// FormNames spells the forms as a list, for help and error text.
func FormNames(forms []Form) string {
	names := make([]string, len(forms))
	for i, form := range forms {
		names[i] = string(form)
	}
	return strings.Join(names, ", ")
}

// The widths the text form is written to.
const (
	// WidthUnbounded writes every column as wide as its widest cell, which is
	// what a form written to a file or a pipe rather than a terminal is.
	WidthUnbounded = 0
	// minColumnWidth is how narrow a column is made to fit a terminal before the
	// row is left to overflow it.
	minColumnWidth = 8
	// columnGap separates the columns of the text form.
	columnGap = 2
)

// MachineForm is the machine-readable form of renderings of this kind.
func (k Kind) MachineForm() Form {
	if k == KindTable {
		return FormMarkdown
	}
	return FormMermaid
}

// SupportsForm reports whether renderings of the kind are written in form:
// every kind has the text form and its machine form, the kinds drawn as a
// graph of nodes and edges have the DOT and PlantUML forms as well, and a
// sequence has PlantUML's sequence grammar.
func (k Kind) SupportsForm(form Form) bool {
	switch form {
	case FormText:
		return true
	case FormMermaid, FormMarkdown:
		return k.MachineForm() == form
	case FormDot:
		switch k {
		case KindTree, KindInterconnection, KindState, KindAction:
			return true
		}
	case FormPlantUML:
		switch k {
		case KindTree, KindInterconnection, KindState, KindAction, KindSequence:
			return true
		}
	}
	return false
}

// SupportedForms are the forms renderings of the kind are written in, in the
// order Forms offers them.
func (k Kind) SupportedForms() []Form {
	var forms []Form
	for _, form := range Forms() {
		if k.SupportsForm(form) {
			forms = append(forms, form)
		}
	}
	return forms
}

// ErrWrongForm is a form asked for that renderings of the kind are not written
// in. WrongFormError wraps it.
var ErrWrongForm = errors.New("rendering is not written in that form")

// WrongFormError is a form asked for that does not fit the rendering's kind: a
// Mermaid diagram of a table, a Markdown table of a diagram, or DOT of a
// sequence. It names the forms the kind is written in instead, and never
// writes another form silently.
type WrongFormError struct {
	// Form is the form asked for, Kind the rendering's kind, and View the view
	// rendered, by qualified name.
	Form Form
	Kind Kind
	View string
}

func (e *WrongFormError) Error() string {
	msg := fmt.Sprintf("%s %s rendering is not written as %s; ask for %s",
		e.Kind.article(), e.Kind, e.Form, joinForms(e.Kind.SupportedForms(), "or"))
	if e.View == "" {
		return msg
	}
	return e.View + ": " + msg
}

// joinForms spells forms as prose, the last joined by the conjunction: "text,
// mermaid or dot".
func joinForms(forms []Form, conjunction string) string {
	if len(forms) < 2 {
		return FormNames(forms)
	}
	return FormNames(forms[:len(forms)-1]) + " " + conjunction + " " + string(forms[len(forms)-1])
}

func (e *WrongFormError) Unwrap() error { return ErrWrongForm }

// Options are what a rendering is written with beside its form. Each form
// takes the ones that apply to it: the text form its Width, the Mermaid form
// its Direction and Unplaced, the DOT and PlantUML forms their Direction,
// Palette and Unplaced. A form ignores the rest, the Mermaid form saying so
// of a Palette in a comment.
type Options struct {
	// Direction is the flow direction a graph-shaped form is drawn in; empty
	// leaves each kind's default.
	Direction Direction
	// Palette is the palette the DOT and PlantUML forms fill nodes from, by
	// keyword family; empty draws in black and white.
	Palette Palette
	// Unplaced is what a graph-shaped form does with the nodes a positioned
	// drawing leaves unplaced; empty leaves them undrawn, as UnplacedOmit does.
	Unplaced Unplaced
	// Width is the width the text form is written to fit; WidthUnbounded
	// writes every column as wide as its widest cell.
	Width int
}

// Write is the rendering in form, with the default options: no particular
// width, each kind's direction, black and white.
func (r *Rendering) Write(form Form) (string, error) {
	return r.WriteWith(form, Options{})
}

// WriteWith is the rendering in form, written with options. A form the kind
// is not written in is a *WrongFormError, and an unknown form names the ones
// there are.
func (r *Rendering) WriteWith(form Form, options Options) (string, error) {
	switch form {
	case FormText:
		return r.TextWidth(options.Width), nil
	case FormMermaid, FormMarkdown, FormDot, FormPlantUML:
		if !r.Kind.SupportsForm(form) {
			return "", &WrongFormError{Form: form, Kind: r.Kind, View: r.View}
		}
		switch form {
		case FormMarkdown:
			return r.Markdown(), nil
		case FormDot:
			return r.DOTWith(options)
		case FormPlantUML:
			return r.PlantUMLWith(options)
		}
		return r.MermaidWith(options), nil
	}
	return "", fmt.Errorf("unknown rendering form %q; the forms are %s", form, joinForms(Forms(), "and"))
}
