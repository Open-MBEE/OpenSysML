package docir

import (
	"github.com/Open-MBEE/OpenSysML/internal/doc/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// QueryResult is the document a QVTS::QueryResult view is written as: titled by
// the view, its header as definitions, its exposed elements as the table's rows.
func QueryResult(rendering *view.Rendering) *Document {
	definitions := make([]Definition, len(rendering.Header))
	for i, field := range rendering.Header {
		definitions[i] = Definition{
			term:        []TextRun{{kind: RunPlain, text: field.Name}},
			description: []TextRun{{kind: RunPlain, text: field.Value}},
		}
	}
	columns := make([]queryexec.Column, len(rendering.Columns))
	for i, label := range rendering.Columns {
		columns[i] = queryexec.StaticColumn(label, 0, symbols.Origin{})
	}
	rows := make([]queryexec.Row, len(rendering.Rows))
	for i, cells := range rendering.Rows {
		rows[i] = queryexec.StaticRow(cells, symbols.Origin{})
	}
	body := Content{kind: ContentTable, columns: columns, rows: rows}
	if len(rows) == 0 {
		// A view exposing nothing says so, as its rendering does, in place of a table of no rows.
		body = Content{kind: ContentParagraph, runs: []TextRun{{kind: RunPlain, text: rendering.EmptyReason()}}}
	}
	return &Document{name: rendering.View, title: rendering.View, content: []Content{
		{kind: ContentDefinitions, definitions: definitions},
		body,
	}}
}
