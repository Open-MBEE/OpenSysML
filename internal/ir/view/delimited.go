package view

import (
	"encoding/csv"
	"strings"
)

// CSV is a tabular rendering as comma-separated values: the columns as a header
// record, then one record per row, each field quoted as RFC 4180 quotes it.
func (r *Rendering) CSV() (string, error) { return r.delimited(',') }

// TSV is CSV with a tab between fields, a field holding a tab, a quote or a
// line break quoted as CSV quotes it, so encoding/csv reads it back.
func (r *Rendering) TSV() (string, error) { return r.delimited('\t') }

// delimited writes the header and the rows as records, comma between fields. It
// carries no notice, which the rendering reports beside the artifact.
func (r *Rendering) delimited(comma rune) (string, error) {
	columns := r.Columns
	if len(columns) == 0 {
		columns = tableColumns
	}
	var b strings.Builder
	w := csv.NewWriter(&b)
	w.Comma = comma
	if err := w.Write(columns); err != nil {
		return "", err
	}
	for _, row := range r.Rows {
		if err := w.Write(padRow(row, len(columns))); err != nil {
			return "", err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return "", err
	}
	return b.String(), nil
}
