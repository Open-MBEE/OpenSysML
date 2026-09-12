// Package docplan compiles native SysML v2 document definitions into
// immutable document plans that reference compiled query programs.
package docplan

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/provenance"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

// ContentKind classifies one planned content node.
type ContentKind string

const (
	ContentSection     ContentKind = "section"
	ContentParagraph   ContentKind = "paragraph"
	ContentTable       ContentKind = "table"
	ContentList        ContentKind = "list"
	ContentDefinitions ContentKind = "definitions"
	ContentDiagram     ContentKind = "diagram"
)

// ListStyle is the declared rendering style of a planned list.
type ListStyle string

const (
	ListBullet ListStyle = "bullet"
	ListNumber ListStyle = "number"
)

// RunKind classifies one planned inline run of a paragraph.
type RunKind string

const (
	RunSpan RunKind = "span"
	RunLink RunKind = "link"
	RunRef  RunKind = "ref"
)

// RunStyle is the declared inline style of a span run.
type RunStyle string

const (
	StylePlain    RunStyle = "plain"
	StyleEmphasis RunStyle = "emphasis"
	StyleStrong   RunStyle = "strong"
	StyleCode     RunStyle = "code"
)

// Run is one planned inline run: a styled span, a link, or a reference to
// a content block of this or another document, or another document's root.
type Run struct {
	kind        RunKind
	text        string
	style       RunStyle
	target      string
	refSym      *symbols.Symbol
	refRoot     *symbols.Symbol
	ref         []string
	refDocument string
	origin      provenance.Origin
}

// Kind returns the classification of the run.
func (r Run) Kind() RunKind { return r.kind }

// Text returns the run's text; for a reference, the resolved label when the
// run states none.
func (r Run) Text() string { return r.text }

// Style returns the inline style of a span run.
func (r Run) Style() RunStyle { return r.style }

// Target returns the destination of a link run.
func (r Run) Target() string { return r.target }

// RefPath returns the named content path of a reference run, from the
// target document's root to the referenced content block; empty when the
// run references another document's root.
func (r Run) RefPath() []string { return append([]string(nil), r.ref...) }

// RefDocument returns the fully-qualified name of the document a reference
// run targets, or "" when it targets the document being planned.
func (r Run) RefDocument() string { return r.refDocument }

// Origin returns the source declaration behind the run.
func (r Run) Origin() provenance.Origin { return r.origin }

func cloneRuns(runs []Run) []Run {
	out := make([]Run, len(runs))
	for i, run := range runs {
		out[i] = run
		out[i].ref = append([]string(nil), run.ref...)
	}
	return out
}

// TemplateKind classifies one planned column run of a query-backed node.
type TemplateKind string

const (
	TemplateSpan TemplateKind = "span"
	TemplateLink TemplateKind = "link"
)

// ColumnRun maps one projected column of a query-backed paragraph or list
// to a styled run per result row.
type ColumnRun struct {
	kind         TemplateKind
	column       string
	style        RunStyle
	styleColumn  string
	targetColumn string
	origin       provenance.Origin
}

// Kind returns the classification of the column run.
func (r ColumnRun) Kind() TemplateKind { return r.kind }

// Column returns the projected column the run's text comes from.
func (r ColumnRun) Column() string { return r.column }

// Style returns the fixed inline style of a span column run.
func (r ColumnRun) Style() RunStyle { return r.style }

// StyleColumn returns the projected column supplying each row's style,
// empty when the style is fixed.
func (r ColumnRun) StyleColumn() string { return r.styleColumn }

// TargetColumn returns the projected column supplying each row's link
// destination.
func (r ColumnRun) TargetColumn() string { return r.targetColumn }

// Origin returns the source declaration behind the column run.
func (r ColumnRun) Origin() provenance.Origin { return r.origin }

// BindingKind classifies one planned binding value.
type BindingKind string

const (
	BindingElement BindingKind = "element"
	BindingString  BindingKind = "string"
	BindingInteger BindingKind = "integer"
	BindingReal    BindingKind = "real"
	BindingBoolean BindingKind = "boolean"
)

// BindingValue is one statically planned value for a query parameter.
type BindingValue struct {
	kind    BindingKind
	element *symbols.Symbol
	text    string
	integer int64
	real    float64
	boolean bool
	origin  provenance.Origin
}

// Kind returns the classification of the value.
func (v BindingValue) Kind() BindingKind { return v.kind }

// Element returns the bound model element when the value is an element.
func (v BindingValue) Element() (*symbols.Symbol, bool) {
	return v.element, v.kind == BindingElement
}

// String returns the bound text when the value is a string.
func (v BindingValue) String() (string, bool) { return v.text, v.kind == BindingString }

// Integer returns the bound integer when the value is an integer.
func (v BindingValue) Integer() (int64, bool) { return v.integer, v.kind == BindingInteger }

// Real returns the bound real when the value is a real.
func (v BindingValue) Real() (float64, bool) { return v.real, v.kind == BindingReal }

// Boolean returns the bound boolean when the value is a boolean.
func (v BindingValue) Boolean() (bool, bool) { return v.boolean, v.kind == BindingBoolean }

// Origin returns the source declaration behind the value.
func (v BindingValue) Origin() provenance.Origin { return v.origin }

// Binding supplies planned values to one parameter of a referenced query.
type Binding struct {
	parameter string
	values    []BindingValue
	origin    provenance.Origin
}

// Parameter returns the bound parameter name.
func (b Binding) Parameter() string { return b.parameter }

// Values returns the planned values in declaration order.
func (b Binding) Values() []BindingValue { return append([]BindingValue(nil), b.values...) }

// Origin returns the source declaration behind the binding.
func (b Binding) Origin() provenance.Origin { return b.origin }

// QueryRef is a planned reference to a compiled query with its bindings.
type QueryRef struct {
	entry    string
	program  *queryplan.Program
	bindings []Binding
	origin   provenance.Origin
}

// Entry returns the fully-qualified name of the referenced query.
func (q *QueryRef) Entry() string { return q.entry }

// Program returns the compiled program of the referenced query.
func (q *QueryRef) Program() *queryplan.Program { return q.program }

// Bindings returns the planned bindings in declaration order.
func (q *QueryRef) Bindings() []Binding {
	out := make([]Binding, len(q.bindings))
	for i, binding := range q.bindings {
		out[i] = Binding{
			parameter: binding.parameter,
			values:    append([]BindingValue(nil), binding.values...),
			origin:    binding.origin,
		}
	}
	return out
}

// Origin returns the source declaration behind the reference.
func (q *QueryRef) Origin() provenance.Origin { return q.origin }

// DiagramRef is a planned reference to what a diagram renders: a declared
// view usage, or a plain element with the rendering kind the diagram states.
type DiagramRef struct {
	view      *symbols.Symbol
	target    *symbols.Symbol
	kind      view.Kind
	stated    string
	direction view.Direction
	origin    provenance.Origin
}

// View returns the declared view usage the diagram renders, when it names one.
func (d *DiagramRef) View() (*symbols.Symbol, bool) { return d.view, d.view != nil }

// Target returns the plain element the diagram renders, when it names one.
func (d *DiagramRef) Target() (*symbols.Symbol, bool) { return d.target, d.target != nil }

// Kind returns the resolved rendering kind.
func (d *DiagramRef) Kind() view.Kind { return d.kind }

// Stated returns how the kind was decided, the way a rendering reports it.
func (d *DiagramRef) Stated() string { return d.stated }

// Direction returns the stated flow direction, empty for the kind's default.
func (d *DiagramRef) Direction() view.Direction { return d.direction }

// Origin returns the source declaration behind the reference.
func (d *DiagramRef) Origin() provenance.Origin { return d.origin }

// Content is one planned content node: a section, paragraph, table, list,
// definitions, or diagram.
type Content struct {
	kind        ContentKind
	name        string
	title       string
	text        string
	caption     string
	style       ListStyle
	groupBy     string
	term        string
	description string
	runs        []Run
	columnRuns  []ColumnRun
	query       *QueryRef
	diagram     *DiagramRef
	children    []Content
	origin      provenance.Origin
}

// Kind returns the classification of the node.
func (c Content) Kind() ContentKind { return c.kind }

// Name returns the declared name of the node, empty when anonymous.
func (c Content) Name() string { return c.name }

// Title returns the declared title of a section.
func (c Content) Title() string { return c.title }

// Text returns the static text of a paragraph, empty when query-backed.
func (c Content) Text() string { return c.text }

// Caption returns the declared caption of a table or diagram.
func (c Content) Caption() string { return c.caption }

// Style returns the declared style of a list.
func (c Content) Style() ListStyle { return c.style }

// GroupBy returns the projected column a table groups its rows by, empty
// when ungrouped.
func (c Content) GroupBy() string { return c.groupBy }

// Term returns the projected column naming each entry of a definitions node.
func (c Content) Term() string { return c.term }

// Description returns the projected column describing each entry of a
// definitions node.
func (c Content) Description() string { return c.description }

// Runs returns the planned inline runs of a paragraph in declaration order.
func (c Content) Runs() []Run { return cloneRuns(c.runs) }

// ColumnRuns returns the planned column runs of a query-backed paragraph or
// list in declaration order.
func (c Content) ColumnRuns() []ColumnRun { return append([]ColumnRun(nil), c.columnRuns...) }

// Query returns the referenced query of a query-backed node, or nil.
func (c Content) Query() *QueryRef { return c.query }

// Diagram returns the planned view reference of a diagram node, or nil.
func (c Content) Diagram() *DiagramRef { return c.diagram }

// Children returns the nested content of a section in declaration order.
func (c Content) Children() []Content { return cloneContent(c.children) }

// Origin returns the source declaration behind the node.
func (c Content) Origin() provenance.Origin { return c.origin }

func cloneContent(content []Content) []Content {
	out := make([]Content, len(content))
	for i, child := range content {
		out[i] = Content{
			kind:        child.kind,
			name:        child.name,
			title:       child.title,
			text:        child.text,
			caption:     child.caption,
			style:       child.style,
			groupBy:     child.groupBy,
			term:        child.term,
			description: child.description,
			runs:        cloneRuns(child.runs),
			columnRuns:  append([]ColumnRun(nil), child.columnRuns...),
			query:       child.query,
			diagram:     child.diagram,
			children:    cloneContent(child.children),
			origin:      child.origin,
		}
	}
	return out
}

// Plan is an immutable compiled document definition.
type Plan struct {
	compiled bool
	name     string
	title    string
	content  []Content
	origin   provenance.Origin
}

// Compiled reports whether the plan was produced by Compile.
func (p *Plan) Compiled() bool { return p != nil && p.compiled }

// Name returns the fully-qualified name of the document definition.
func (p *Plan) Name() string { return p.name }

// Title returns the declared document title.
func (p *Plan) Title() string { return p.title }

// Content returns the planned top-level content in declaration order.
func (p *Plan) Content() []Content { return cloneContent(p.content) }

// Origin returns the source declaration behind the document.
func (p *Plan) Origin() provenance.Origin { return p.origin }
