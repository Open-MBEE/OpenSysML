package view

import (
	"fmt"
	"strings"
)

// Markdown is the machine-readable form of a tabular rendering: a GitHub-flavored
// Markdown table, which is what a table is read as where the models are — in
// documentation and in an editor — since Mermaid has no table grammar. What the
// rendering could not represent is written as Markdown comments, so no notice is
// lost.
func (r *Rendering) Markdown() string {
	return r.MarkdownCells(markdownCell)
}

// MarkdownCells is Markdown with the caller's cell escaping, for a host whose
// Markdown dialect escapes more than a pipe and a newline.
func (r *Rendering) MarkdownCells(escape func(string) string) string {
	var b strings.Builder
	if r.View == "" {
		fmt.Fprintf(&b, "<!-- %s rendering", r.Kind)
	} else {
		fmt.Fprintf(&b, "<!-- %s — %s rendering", r.View, r.Kind)
	}
	if r.Stated != "" {
		fmt.Fprintf(&b, " (%s)", r.Stated)
	}
	b.WriteString(" -->\n")
	for _, notice := range r.Notices {
		fmt.Fprintf(&b, "<!-- not represented: %s -->\n", markdownComment(notice))
	}
	if r.Empty() {
		fmt.Fprintf(&b, "\n%s\n", r.EmptyReason())
		return b.String()
	}
	columns := r.Columns
	if len(columns) == 0 {
		columns = tableColumns
	}
	writeMarkdownRow(&b, columns, escape)
	rule := make([]string, len(columns))
	for i := range rule {
		rule[i] = "---"
	}
	writeMarkdownRow(&b, rule, escape)
	for _, row := range r.Rows {
		writeMarkdownRow(&b, padRow(row, len(columns)), escape)
	}
	return b.String()
}

// writeMarkdownRow writes one row of a Markdown table, escaping what a cell may
// not carry literally.
func writeMarkdownRow(b *strings.Builder, cells []string, escape func(string) string) {
	quoted := make([]string, 0, len(cells))
	for _, cell := range cells {
		quoted = append(quoted, escape(cell))
	}
	fmt.Fprintf(b, "| %s |\n", strings.Join(quoted, " | "))
}

// padRow gives a row one cell per column, so a short row does not shift the
// table.
func padRow(row []string, columns int) []string {
	if len(row) >= columns {
		return row[:columns]
	}
	return append(append([]string(nil), row...), make([]string, columns-len(row))...)
}

// markdownCell escapes a pipe, which would otherwise start a new cell, and a
// newline, which would end the row.
func markdownCell(cell string) string {
	return strings.NewReplacer("|", "\\|", "\n", " ").Replace(cell)
}

// markdownComment escapes the sequence that would close a comment early.
func markdownComment(text string) string {
	return strings.NewReplacer("-->", "--\\>", "\n", " ").Replace(text)
}
