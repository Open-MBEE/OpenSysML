package view

import (
	"encoding/csv"
	"errors"
	"slices"
	"strings"
	"testing"
)

// readDelimited reads a CSV or TSV artifact back with encoding/csv.
func readDelimited(t *testing.T, artifact string, comma rune) [][]string {
	t.Helper()
	r := csv.NewReader(strings.NewReader(artifact))
	r.Comma = comma
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("read the artifact back: %v\n%s", err, artifact)
	}
	return records
}

// A table is written as CSV and TSV: a header record of its columns, then one
// record per row, read back by encoding/csv as the rows they were.
func TestTableFormsAreDelimitedValues(t *testing.T) {
	rendering := render(t, "table.sysml", "TableViews::partsTable")
	for _, tc := range []struct {
		form  Form
		comma rune
	}{{FormCSV, ','}, {FormTSV, '\t'}} {
		t.Run(string(tc.form), func(t *testing.T) {
			artifact, err := rendering.Write(tc.form)
			if err != nil {
				t.Fatalf("write %s: %v", tc.form, err)
			}
			records := readDelimited(t, artifact, tc.comma)
			if !slices.Equal(records[0], TableColumns()) {
				t.Errorf("header = %q, want %q", records[0], TableColumns())
			}
			if got, want := len(records)-1, len(rendering.Rows); got != want || want == 0 {
				t.Fatalf("%d records after the header, want %d", got, want)
			}
			for i, row := range rendering.Rows {
				if !slices.Equal(records[i+1], padRow(row, len(TableColumns()))) {
					t.Errorf("record %d = %q, want %q", i+1, records[i+1], row)
				}
			}
			if strings.Contains(artifact, "<!--") || strings.Contains(artifact, "| Element |") {
				t.Errorf("%s carries Markdown:\n%s", tc.form, artifact)
			}
		})
	}
}

// A cell holding the delimiter, a quote or a line break is quoted, a short row
// is padded to the columns, and every cell reads back as it was written.
func TestDelimitedFormsQuoteWhatACellHolds(t *testing.T) {
	rendering := &Rendering{
		Kind:    KindTable,
		View:    "Demo::odd",
		Columns: []string{"Element", "Note"},
		Rows: [][]string{
			{"'a, b'", "says \"hi\""},
			{"tab\there", "two\nlines"},
			{"short"},
		},
		Notices: []string{"nested view Demo::odd is nested in itself; listed once"},
	}
	want := [][]string{{"Element", "Note"}, {"'a, b'", "says \"hi\""}, {"tab\there", "two\nlines"}, {"short", ""}}
	csvText, err := rendering.Write(FormCSV)
	if err != nil {
		t.Fatal(err)
	}
	if got := "Element,Note\n\"'a, b'\",\"says \"\"hi\"\"\"\ntab\there,\"two\nlines\"\nshort,\n"; csvText != got {
		t.Errorf("csv =\n%q\nwant\n%q", csvText, got)
	}
	tsvText, err := rendering.Write(FormTSV)
	if err != nil {
		t.Fatal(err)
	}
	if got := "Element\tNote\n'a, b'\t\"says \"\"hi\"\"\"\n\"tab\there\"\t\"two\nlines\"\nshort\t\n"; tsvText != got {
		t.Errorf("tsv =\n%q\nwant\n%q", tsvText, got)
	}
	for form, comma := range map[string]rune{csvText: ',', tsvText: '\t'} {
		if got := readDelimited(t, form, comma); !slices.EqualFunc(got, want, slices.Equal[[]string]) {
			t.Errorf("read back %q, want %q", got, want)
		}
	}
	if strings.Contains(csvText, "nested") || strings.Contains(tsvText, "nested") {
		t.Error("a notice was written into the records")
	}
}

// An empty table is its header alone, and one naming no columns takes the
// default headings.
func TestDelimitedFormsOfAnEmptyTable(t *testing.T) {
	artifact, err := (&Rendering{Kind: KindTable}).Write(FormCSV)
	if err != nil {
		t.Fatal(err)
	}
	if artifact != "Element,Kind,Type,Declared in\n" {
		t.Errorf("empty csv = %q", artifact)
	}
}

// Only a table is written as CSV or TSV; any other kind is a typed error
// naming the forms it is written in.
func TestDelimitedFormsAreForTablesOnly(t *testing.T) {
	for _, kind := range Kinds() {
		for _, form := range []Form{FormCSV, FormTSV} {
			if got := kind.SupportsForm(form); got != (kind == KindTable) {
				t.Errorf("%s.SupportsForm(%s) = %v", kind, form, got)
			}
		}
	}
	_, err := render(t, "tree.sysml", "VehicleViews::vehicleView").Write(FormCSV)
	var wrong *WrongFormError
	if !errors.As(err, &wrong) || !strings.Contains(err.Error(), "ask for text, mermaid, dot or plantuml") {
		t.Errorf("csv of a tree error = %v, want a *WrongFormError offering the tree's forms", err)
	}
	tree := render(t, "tree.sysml", "VehicleViews::vehicleView")
	for form, write := range map[Form]func() (string, error){FormCSV: tree.CSV, FormTSV: tree.TSV} {
		if out, err := write(); !errors.As(err, &wrong) || wrong.Form != form || out != "" {
			t.Errorf("direct %s of a tree = %q, %v, want a *WrongFormError", form, out, err)
		}
	}
	if FormCSV.TakesPalette() || FormTSV.TakesPalette() {
		t.Error("a delimited form takes a palette")
	}
}
