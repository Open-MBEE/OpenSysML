package view

import (
	"encoding/csv"
	"strings"
)

// CSV is a tabular rendering as comma-separated values: the columns as a header
// record, then one record per row, each field quoted as RFC 4180 quotes it.
func (r *Rendering) CSV() (string, error) { return r.delimited(FormCSV, ',') }

// TSV is CSV with a tab between fields, a field holding a tab, a quote or a
// line break quoted as CSV quotes it, so encoding/csv reads it back.
func (r *Rendering) TSV() (string, error) { return r.delimited(FormTSV, '\t') }

// delimited writes the header and the rows as records, comma separating the
// fields; a kind not written in form is a *WrongFormError. It carries no
// notice, which the rendering reports beside the artifact.
func (r *Rendering) delimited(form Form, comma rune) (string, error) {
	if !r.Kind.SupportsForm(form) {
		return "", &WrongFormError{Form: form, Kind: r.Kind, View: r.View}
	}
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
