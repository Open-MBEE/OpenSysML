// Package docir evaluates compiled document plans into an immutable,
// backend-agnostic document tree with provenance on every node.
package docir

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/provenance"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

// ContentKind classifies one evaluated content node.
type ContentKind string

const (
	ContentSection     ContentKind = "section"
	ContentParagraph   ContentKind = "paragraph"
	ContentTable       ContentKind = "table"
	ContentList        ContentKind = "list"
	ContentDefinitions ContentKind = "definitions"
	ContentDiagram     ContentKind = "diagram"
)

// ListStyle is the rendering style of an evaluated list.
type ListStyle string

const (
	ListBullet ListStyle = "bullet"
	ListNumber ListStyle = "number"
)

// RunKind classifies one text run: plain prose, an inline style, a link to
// an external destination, or a reference to another content node.
type RunKind string

const (
	RunPlain    RunKind = "plain"
	RunEmphasis RunKind = "emphasis"
	RunStrong   RunKind = "strong"
	RunCode     RunKind = "code"
	RunLink     RunKind = "link"
	RunRef      RunKind = "ref"
)

// TextRun is one piece of paragraph or list-item text with its provenance:
// static text carries its declaration, query-backed text its query value.
type TextRun struct {
	kind     RunKind
	text     string
	target   string
	document string
	origin   provenance.Origin
}

// Kind returns the classification of the run; the zero value is plain.
func (r TextRun) Kind() RunKind {
	if r.kind == "" {
		return RunPlain
	}
	return r.kind
}

// Text returns the run's text.
func (r TextRun) Text() string { return r.text }

// Target returns a link run's destination, or a reference run's anchor —
// the stable identifier of the referenced content node. A reference to
// another document's root has no anchor.
func (r TextRun) Target() string { return r.target }

// TargetDocument returns the fully-qualified name of the document a
// reference run targets, or "" when it targets its own document.
func (r TextRun) TargetDocument() string { return r.document }

// Origin returns the source declaration or query value behind the run.
func (r TextRun) Origin() provenance.Origin { return r.origin }

// TableGroup is one group of a grouped table's rows sharing a group-column
// value, in order of first appearance.
type TableGroup struct {
	key  string
	rows []queryexec.Row
}

// Key returns the shared group-column text of the group's rows.
func (g TableGroup) Key() string { return g.key }

// Rows returns the group's rows in result order.
func (g TableGroup) Rows() []queryexec.Row { return append([]queryexec.Row(nil), g.rows...) }

func cloneGroups(groups []TableGroup) []TableGroup {
	out := make([]TableGroup, len(groups))
	for i, group := range groups {
		out[i] = TableGroup{key: group.key, rows: append([]queryexec.Row(nil), group.rows...)}
	}
	return out
}

// ListItem is one evaluated list item, produced from one query row.
type ListItem struct {
	runs    []TextRun
	element queryexec.Value
	origin  provenance.Origin
}

// Runs returns the item's text runs in column order.
func (i ListItem) Runs() []TextRun { return append([]TextRun(nil), i.runs...) }

// Element returns the model element the item's query row selected.
func (i ListItem) Element() queryexec.Value { return i.element }

// Origin returns the query row behind the item.
func (i ListItem) Origin() provenance.Origin { return i.origin }

// Definition is one evaluated definitions entry, produced from one query row.
type Definition struct {
	term        []TextRun
	description []TextRun
	element     queryexec.Value
	origin      provenance.Origin
}

// Term returns the runs naming the entry, one per value of the term column.
func (d Definition) Term() []TextRun { return append([]TextRun(nil), d.term...) }

// Description returns the runs describing the entry, one per value of the
// description column.
func (d Definition) Description() []TextRun { return append([]TextRun(nil), d.description...) }

// Element returns the model element the entry's query row selected.
func (d Definition) Element() queryexec.Value { return d.element }

// Origin returns the query row behind the entry.
func (d Definition) Origin() provenance.Origin { return d.origin }

// Content is one evaluated content node: a section, paragraph, table, list,
// definitions block, or diagram.
type Content struct {
	kind        ContentKind
	name        string
	title       string
	caption     string
	style       ListStyle
	anchor      string
	groupBy     string
	runs        []TextRun
	columns     []queryexec.Column
	rows        []queryexec.Row
	groups      []TableGroup
	items       []ListItem
	definitions []Definition
	rendering   *view.Rendering
	direction   view.Direction
	children    []Content
	query       string
	queryOrigin provenance.Origin
	origin      provenance.Origin
}

// Kind returns the classification of the node.
func (c Content) Kind() ContentKind { return c.kind }

// Name returns the declared name of the node, empty when anonymous.
func (c Content) Name() string { return c.name }

// Title returns the title of a section.
func (c Content) Title() string { return c.title }

// Caption returns the caption of a table or diagram.
func (c Content) Caption() string { return c.caption }

// Style returns the style of a list.
func (c Content) Style() ListStyle { return c.style }

// Anchor returns the node's stable identifier when a reference run targets
// it, empty otherwise.
func (c Content) Anchor() string { return c.anchor }

// GroupBy returns the projected column a table groups its rows by, empty
// when ungrouped.
func (c Content) GroupBy() string { return c.groupBy }

// Groups returns the ordered row groups of a grouped table.
func (c Content) Groups() []TableGroup { return cloneGroups(c.groups) }

// Runs returns the text runs of a paragraph in row and column order.
func (c Content) Runs() []TextRun { return append([]TextRun(nil), c.runs...) }

// Columns returns the ordered projected columns of a table, preserved even
// when the query returned no rows.
func (c Content) Columns() []queryexec.Column { return append([]queryexec.Column(nil), c.columns...) }

// Rows returns the ordered rows of a table with their typed cells.
func (c Content) Rows() []queryexec.Row { return append([]queryexec.Row(nil), c.rows...) }

// Items returns the ordered items of a list.
func (c Content) Items() []ListItem {
	out := make([]ListItem, len(c.items))
	for i, item := range c.items {
		out[i] = ListItem{runs: append([]TextRun(nil), item.runs...), element: item.element, origin: item.origin}
	}
	return out
}

// Definitions returns the ordered entries of a definitions block.
func (c Content) Definitions() []Definition {
	out := make([]Definition, len(c.definitions))
	for i, entry := range c.definitions {
		out[i] = Definition{
			term:        append([]TextRun(nil), entry.term...),
			description: append([]TextRun(nil), entry.description...),
			element:     entry.element,
			origin:      entry.origin,
		}
	}
	return out
}

// Rendering returns a copy of the resolved view content of a diagram, nil
// for every other kind.
func (c Content) Rendering() *view.Rendering { return c.rendering.Clone() }

// Direction returns the stated flow direction of a diagram, empty for the
// kind's default.
func (c Content) Direction() view.Direction { return c.direction }

// Children returns the nested content of a section in declaration order.
func (c Content) Children() []Content { return cloneContent(c.children) }

// Query returns the fully-qualified name of the query behind a query-backed
// node, empty for sections and static paragraphs.
func (c Content) Query() string { return c.query }

// QueryOrigin returns the declaration of the query behind a query-backed node.
func (c Content) QueryOrigin() provenance.Origin { return c.queryOrigin }

// Origin returns the source declaration behind the node.
func (c Content) Origin() provenance.Origin { return c.origin }

func cloneContent(content []Content) []Content {
	out := make([]Content, len(content))
	for i, child := range content {
		out[i] = Content{
			kind:        child.kind,
			name:        child.name,
			title:       child.title,
			caption:     child.caption,
			style:       child.style,
			anchor:      child.anchor,
			groupBy:     child.groupBy,
			runs:        append([]TextRun(nil), child.runs...),
			columns:     append([]queryexec.Column(nil), child.columns...),
			rows:        append([]queryexec.Row(nil), child.rows...),
			groups:      cloneGroups(child.groups),
			items:       child.Items(),
			definitions: child.Definitions(),
			rendering:   child.rendering.Clone(),
			direction:   child.direction,
			children:    cloneContent(child.children),
			query:       child.query,
			queryOrigin: child.queryOrigin,
			origin:      child.origin,
		}
	}
	return out
}

// Document is an immutable evaluated document.
type Document struct {
	name    string
	title   string
	content []Content
	origin  provenance.Origin
}

// Name returns the fully-qualified name of the document definition.
func (d *Document) Name() string { return d.name }

// Title returns the document title.
func (d *Document) Title() string { return d.title }

// Content returns the evaluated top-level content in declaration order.
func (d *Document) Content() []Content { return cloneContent(d.content) }

// Origin returns the source declaration behind the document.
func (d *Document) Origin() provenance.Origin { return d.origin }
