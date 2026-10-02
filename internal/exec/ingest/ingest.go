// Package ingest reads tabular and record data — CSV, TSV, JSON and JSON Lines —
// into the feature values it assigns to model elements, one row per element.
package ingest

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// Format is a representation the data is read from.
type Format string

const (
	FormatCSV   Format = "csv"
	FormatTSV   Format = "tsv"
	FormatJSON  Format = "json"
	FormatJSONL Format = "jsonl"
)

// Formats lists the formats Read accepts.
func Formats() []Format { return []Format{FormatCSV, FormatTSV, FormatJSON, FormatJSONL} }

// ParseFormat reads a format name; ndjson is JSON Lines.
func ParseFormat(name string) (Format, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "csv":
		return FormatCSV, nil
	case "tsv", "tab":
		return FormatTSV, nil
	case "json":
		return FormatJSON, nil
	case "jsonl", "ndjson":
		return FormatJSONL, nil
	}
	return "", fmt.Errorf("unknown data format %q; the formats are csv, tsv, json and jsonl", name)
}

// FormatOfPath is the format a data file's extension names.
func FormatOfPath(path string) (Format, bool) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".csv":
		return FormatCSV, true
	case ".tsv", ".tab":
		return FormatTSV, true
	case ".json":
		return FormatJSON, true
	case ".jsonl", ".ndjson":
		return FormatJSONL, true
	}
	return "", false
}

// Kind is what a cell's source says of its value.
type Kind int

const (
	// KindText is a delimited cell, whose type the feature it sets decides.
	KindText Kind = iota
	KindNumber
	KindBoolean
	KindString
)

// Cell is one value a row assigns to a feature of its element.
type Cell struct {
	Feature string
	Text    string
	Kind    Kind
	// Type is the mapping's declared type for the value, "" to follow the feature's.
	Type string
	Unit string
	// Where locates the value in the input, for a diagnostic.
	Where string
}

// Row is one record: the element it describes and the values it assigns.
type Row struct {
	Element string
	Cells   []Cell
	Where   string
}

// Read reads the rows of data, named name in diagnostics, as format under the
// mapping m, which may be nil. An empty cell or a JSON null assigns nothing.
func Read(name string, data []byte, format Format, m *Map) ([]Row, error) {
	if m == nil {
		m = &Map{}
	}
	if err := m.check(); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	var rows []Row
	var err error
	switch format {
	case FormatCSV, FormatTSV:
		comma := ','
		if format == FormatTSV {
			comma = '\t'
		}
		if m.Delimiter != "" {
			comma, err = delimiterOf(m.Delimiter)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
		}
		rows, err = readDelimited(name, data, comma, m)
	case FormatJSON:
		rows, err = readJSON(name, data, m)
	case FormatJSONL:
		rows, err = readJSONLines(name, data, m)
	default:
		return nil, fmt.Errorf("%s: unknown data format %q", name, format)
	}
	if err != nil {
		return nil, err
	}
	if err := checkDuplicates(rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func delimiterOf(text string) (rune, error) {
	switch text {
	case `\t`, "tab":
		return '\t', nil
	}
	r, size := utf8.DecodeRuneInString(text)
	if size != len(text) || r == utf8.RuneError || r == '"' || r == '\r' || r == '\n' {
		return 0, fmt.Errorf("delimiter %q is not one character a CSV reader can split on", text)
	}
	return r, nil
}

// SplitHeader takes a unit written in brackets off a column name: `mass [kg]`
// names column mass in kilograms.
func SplitHeader(header string) (name, unit string) {
	header = strings.TrimSpace(header)
	if strings.HasSuffix(header, "]") {
		if i := strings.LastIndex(header, "["); i > 0 {
			return strings.TrimSpace(header[:i]), strings.TrimSpace(header[i+1 : len(header)-1])
		}
	}
	return header, ""
}

// column is a delimited header's column as a feature reads it.
type column struct {
	index int
	name  string
	unit  string
}

func readDelimited(name string, data []byte, comma rune, m *Map) ([]Row, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = comma
	header, err := r.Read()
	if errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s: no header record", name)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	columns := map[string]column{}
	var order []string
	for i, h := range header {
		n, unit := SplitHeader(h)
		if n == "" {
			return nil, fmt.Errorf("%s line 1: column %d has no name", name, i+1)
		}
		if _, dup := columns[n]; dup {
			return nil, fmt.Errorf("%s line 1: column %s is named twice", name, n)
		}
		columns[n] = column{index: i, name: n, unit: unit}
		order = append(order, n)
	}
	if long := longForm(columns, m); long {
		return readLongForm(name, r, columns, m)
	}
	elementColumn := m.Element.columnOr(defaultElementColumn)
	ec, ok := columns[elementColumn]
	if !ok {
		return nil, fmt.Errorf("%s line 1: no %s column names the element each row sets; name it with the mapping's element", name, elementColumn)
	}
	fields, err := m.delimitedFields(order, columns, elementColumn)
	if err != nil {
		return nil, fmt.Errorf("%s line 1: %w", name, err)
	}
	var rows []Row
	for {
		record, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		line, _ := r.FieldPos(0)
		where := fmt.Sprintf("%s line %d", name, line)
		element := strings.TrimSpace(record[ec.index])
		if element == "" {
			if allEmpty(record) {
				continue
			}
			return nil, fmt.Errorf("%s, column %s: no element is named", where, elementColumn)
		}
		row := Row{Element: m.Element.Prefix + element, Where: where}
		for _, f := range fields {
			text := strings.TrimSpace(record[f.column.index])
			if text == "" {
				continue
			}
			unit := f.unit
			if f.unitColumn != nil {
				unit = strings.TrimSpace(record[f.unitColumn.index])
			}
			row.Cells = append(row.Cells, Cell{
				Feature: f.feature, Text: text, Kind: KindText, Type: f.typ, Unit: unit,
				Where: fmt.Sprintf("%s, column %s", where, header[f.column.index]),
			})
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func allEmpty(record []string) bool {
	for _, cell := range record {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

// longForm reports whether a header is element, feature and value, with unit
// optional: one value per row rather than one element.
func longForm(columns map[string]column, m *Map) bool {
	if len(m.Features) > 0 || m.Element.Column != "" {
		return false
	}
	_, unit := columns["unit"]
	want := 3
	if unit {
		want = 4
	}
	_, e := columns["element"]
	_, f := columns["feature"]
	_, v := columns["value"]
	return e && f && v && len(columns) == want
}

func readLongForm(name string, r *csv.Reader, columns map[string]column, m *Map) ([]Row, error) {
	var rows []Row
	for {
		record, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		line, _ := r.FieldPos(0)
		where := fmt.Sprintf("%s line %d", name, line)
		element := strings.TrimSpace(record[columns["element"].index])
		feature := strings.TrimSpace(record[columns["feature"].index])
		value := strings.TrimSpace(record[columns["value"].index])
		if element == "" && feature == "" && value == "" {
			continue
		}
		if element == "" || feature == "" {
			return nil, fmt.Errorf("%s: a row names its element and its feature", where)
		}
		feature, headerUnit := SplitHeader(feature)
		unit := headerUnit
		if c, ok := columns["unit"]; ok && strings.TrimSpace(record[c.index]) != "" {
			unit = strings.TrimSpace(record[c.index])
		}
		row := Row{Element: m.Element.Prefix + element, Where: where}
		if value != "" {
			row.Cells = []Cell{{Feature: feature, Text: value, Kind: KindText, Unit: unit, Where: where + ", column value"}}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func readJSON(name string, data []byte, m *Map) ([]Row, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var doc any
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("%s: text follows the JSON value; read JSON Lines as jsonl", name)
	}
	records := doc
	if m.Records != "" {
		var err error
		records, err = pointer(doc, m.Records)
		if err != nil {
			return nil, fmt.Errorf("%s: records %s: %w", name, m.Records, err)
		}
	}
	list, ok := records.([]any)
	if !ok {
		return nil, fmt.Errorf("%s: the records are not a JSON array; point to them with the mapping's records", name)
	}
	order, err := recordKeyOrder(data, m.Records)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	rows := make([]Row, 0, len(list))
	for i, rec := range list {
		var keys []string
		if i < len(order) {
			keys = order[i]
		}
		row, err := m.jsonRow(fmt.Sprintf("%s record %d", name, i+1), rec, keys)
		if err != nil {
			return nil, err
		}
		if row != nil {
			rows = append(rows, *row)
		}
	}
	return rows, nil
}

func readJSONLines(name string, data []byte, m *Map) ([]Row, error) {
	var rows []Row
	for i, line := range bytes.Split(data, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		where := fmt.Sprintf("%s line %d", name, i+1)
		dec := json.NewDecoder(bytes.NewReader(line))
		dec.UseNumber()
		var rec any
		if err := dec.Decode(&rec); err != nil {
			return nil, fmt.Errorf("%s: %w", where, err)
		}
		if _, err := dec.Token(); err != io.EOF {
			return nil, fmt.Errorf("%s: a line holds one JSON value; text follows it", where)
		}
		keys, err := objectKeys(line)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", where, err)
		}
		row, err := m.jsonRow(where, rec, keys)
		if err != nil {
			return nil, err
		}
		if row != nil {
			rows = append(rows, *row)
		}
	}
	return rows, nil
}

// jsonRow reads one JSON record, its top-level keys in keys' order; nil for a
// record setting nothing and naming no element.
func (m *Map) jsonRow(where string, rec any, keys []string) (*Row, error) {
	obj, ok := rec.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: a record is a JSON object", where)
	}
	var element any
	elementKey := m.Element.columnOr(defaultElementColumn)
	if m.Element.Path != "" {
		v, err := pointer(obj, m.Element.Path)
		if err != nil {
			return nil, fmt.Errorf("%s, %s: %w", where, m.Element.Path, err)
		}
		element = v
		elementKey = ""
	} else {
		element = obj[elementKey]
	}
	elementText, ok := element.(string)
	if !ok || strings.TrimSpace(elementText) == "" {
		if len(obj) == 0 {
			return nil, nil
		}
		at := m.Element.Path
		if at == "" {
			at = "field " + elementKey
		}
		return nil, fmt.Errorf("%s, %s: no element is named; a record names its element as a string", where, at)
	}
	row := &Row{Element: m.Element.Prefix + strings.TrimSpace(elementText), Where: where}
	fields, err := m.jsonFields(obj, keys, elementKey)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", where, err)
	}
	for _, f := range fields {
		at := "field " + f.key
		v, ok := obj[f.key]
		if f.path != "" {
			at = f.path
			var err error
			v, err = pointer(obj, f.path)
			if err != nil {
				if errors.Is(err, errNoMember) {
					continue
				}
				return nil, fmt.Errorf("%s, %s: %w", where, at, err)
			}
		} else if !ok {
			continue
		}
		cell, set, err := jsonCell(v)
		if err != nil {
			return nil, fmt.Errorf("%s, %s: %w", where, at, err)
		}
		if !set {
			continue
		}
		cell.Feature, cell.Type, cell.Unit, cell.Where = f.feature, f.typ, f.unit, where+", "+at
		if f.unitPath != "" {
			u, err := pointer(obj, f.unitPath)
			if err != nil {
				return nil, fmt.Errorf("%s, %s: %w", where, f.unitPath, err)
			}
			s, ok := u.(string)
			if !ok {
				return nil, fmt.Errorf("%s, %s: a unit is a string", where, f.unitPath)
			}
			cell.Unit = strings.TrimSpace(s)
		}
		row.Cells = append(row.Cells, cell)
	}
	return row, nil
}

func jsonCell(v any) (Cell, bool, error) {
	switch v := v.(type) {
	case nil:
		return Cell{}, false, nil
	case json.Number:
		return Cell{Text: v.String(), Kind: KindNumber}, true, nil
	case bool:
		return Cell{Text: fmt.Sprint(v), Kind: KindBoolean}, true, nil
	case string:
		if v == "" {
			return Cell{}, false, nil
		}
		return Cell{Text: v, Kind: KindString}, true, nil
	}
	return Cell{}, false, errors.New("the value is not a number, boolean or string; map a field inside it with a path")
}

// checkDuplicates refuses a feature of one element set twice.
func checkDuplicates(rows []Row) error {
	seen := map[[2]string]string{}
	for _, row := range rows {
		for _, c := range row.Cells {
			key := [2]string{row.Element, c.Feature}
			if first, ok := seen[key]; ok {
				return fmt.Errorf("%s: %s::%s is already set at %s", c.Where, row.Element, c.Feature, first)
			}
			seen[key] = c.Where
		}
	}
	return nil
}
