package ingest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const defaultElementColumn = "element"

// Map says how input columns and JSON fields map onto elements and features.
// Every field is optional: a column or field names the feature it sets, a
// bracketed suffix its unit, and the element column names the element.
type Map struct {
	// As is what the data is imported as; "values" sets feature values.
	As string `json:"as,omitempty"`
	// Format overrides the format the data file's extension names.
	Format string `json:"format,omitempty"`
	// Delimiter is the one character a delimited file's cells are split on.
	Delimiter string `json:"delimiter,omitempty"`
	// Records is a JSON Pointer to the array of records in a JSON document.
	Records  string           `json:"records,omitempty"`
	Element  Field            `json:"element,omitempty"`
	Features map[string]Field `json:"features,omitempty"`
	// Ignore names columns or top-level fields that set nothing.
	Ignore []string `json:"ignore,omitempty"`
}

// Field locates one value in a record: a column, or a JSON Pointer path.
type Field struct {
	Column string `json:"column,omitempty"`
	Path   string `json:"path,omitempty"`
	// Prefix is written before an element name the record gives.
	Prefix     string `json:"prefix,omitempty"`
	Unit       string `json:"unit,omitempty"`
	UnitColumn string `json:"unitColumn,omitempty"`
	UnitPath   string `json:"unitPath,omitempty"`
	// Type is string, boolean, integer, real or number; "" follows the feature.
	Type string `json:"type,omitempty"`
}

// ParseMap reads a mapping file, refusing a key it does not define.
func ParseMap(data []byte) (*Map, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var m Map
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("mapping: %w", err)
	}
	if err := m.check(); err != nil {
		return nil, err
	}
	return &m, nil
}

func (m *Map) check() error {
	switch m.As {
	case "", "values":
	default:
		return fmt.Errorf("mapping: as %q is not supported; import as values", m.As)
	}
	if m.Format != "" {
		if _, err := ParseFormat(m.Format); err != nil {
			return fmt.Errorf("mapping: %w", err)
		}
	}
	for name, f := range m.Features {
		switch f.Type {
		case "", "string", "boolean", "integer", "real", "number":
		default:
			return fmt.Errorf("mapping: feature %s: type %q is not string, boolean, integer, real or number", name, f.Type)
		}
		if f.Unit != "" && (f.UnitColumn != "" || f.UnitPath != "") {
			return fmt.Errorf("mapping: feature %s: name its unit once, as unit, unitColumn or unitPath", name)
		}
		if f.Column != "" && f.Path != "" {
			return fmt.Errorf("mapping: feature %s: name its value once, as column or path", name)
		}
	}
	if m.Element.Column != "" && m.Element.Path != "" {
		return errors.New("mapping: element: name it once, as column or path")
	}
	return nil
}

func (f Field) columnOr(name string) string {
	if f.Column != "" {
		return f.Column
	}
	return name
}

// delimitedField is one column a feature's value is read from.
type delimitedField struct {
	feature    string
	column     column
	unit       string
	unitColumn *column
	typ        string
}

func (m *Map) delimitedFields(order []string, columns map[string]column, elementColumn string) ([]delimitedField, error) {
	if m.Records != "" || m.Element.Path != "" {
		return nil, errors.New("records and element paths are JSON Pointers; a delimited file names columns")
	}
	consumed := map[string]bool{elementColumn: true}
	for _, name := range m.Ignore {
		consumed[name] = true
	}
	var fields []delimitedField
	for _, feature := range sortedKeys(m.Features) {
		f := m.Features[feature]
		if f.Path != "" || f.UnitPath != "" {
			return nil, fmt.Errorf("feature %s: a path is a JSON Pointer; a delimited file names a column", feature)
		}
		name := f.columnOr(feature)
		c, ok := columns[name]
		if !ok {
			return nil, fmt.Errorf("feature %s: there is no column %s", feature, name)
		}
		consumed[name] = true
		field := delimitedField{feature: feature, column: c, unit: c.unit, typ: f.Type}
		if f.Unit != "" {
			field.unit = f.Unit
		}
		if f.UnitColumn != "" {
			uc, ok := columns[f.UnitColumn]
			if !ok {
				return nil, fmt.Errorf("feature %s: there is no unit column %s", feature, f.UnitColumn)
			}
			consumed[f.UnitColumn] = true
			field.unitColumn = &uc
		}
		fields = append(fields, field)
	}
	for _, name := range order {
		if consumed[name] {
			continue
		}
		c := columns[name]
		fields = append(fields, delimitedField{feature: name, column: c, unit: c.unit})
	}
	sort.SliceStable(fields, func(i, j int) bool { return fields[i].column.index < fields[j].column.index })
	return fields, nil
}

// jsonField is one member or path a feature's value is read from.
type jsonField struct {
	feature  string
	key      string
	path     string
	unit     string
	unitPath string
	typ      string
}

func (m *Map) jsonFields(obj map[string]any, keys []string, elementKey string) ([]jsonField, error) {
	consumed := map[string]bool{}
	if elementKey != "" {
		consumed[elementKey] = true
	}
	for _, name := range m.Ignore {
		consumed[name] = true
	}
	var fields []jsonField
	for _, feature := range sortedKeys(m.Features) {
		f := m.Features[feature]
		field := jsonField{feature: feature, path: f.Path, unit: f.Unit, unitPath: f.UnitPath, typ: f.Type}
		if f.Path == "" {
			field.key = f.columnOr(feature)
			consumed[field.key] = true
		} else if segs := strings.Split(f.Path, "/"); len(segs) > 1 {
			consumed[unescapePointer(segs[1])] = true
		}
		if f.UnitColumn != "" {
			field.unitPath = "/" + escapePointer(f.UnitColumn)
			consumed[f.UnitColumn] = true
		}
		fields = append(fields, field)
	}
	if keys == nil {
		keys = sortedKeys(obj)
	}
	for _, key := range keys {
		if consumed[key] {
			continue
		}
		name, unit := SplitHeader(key)
		fields = append(fields, jsonField{feature: name, key: key, unit: unit})
	}
	return fields, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

var errNoMember = errors.New("there is no such member")

// pointer resolves a JSON Pointer (RFC 6901) in a decoded document.
func pointer(doc any, p string) (any, error) {
	if p == "" {
		return doc, nil
	}
	if !strings.HasPrefix(p, "/") {
		return nil, fmt.Errorf("%q is not a JSON Pointer; it starts with /", p)
	}
	cur := doc
	for _, seg := range strings.Split(p[1:], "/") {
		seg = unescapePointer(seg)
		switch v := cur.(type) {
		case map[string]any:
			next, ok := v[seg]
			if !ok {
				return nil, errNoMember
			}
			cur = next
		case []any:
			i, err := strconv.Atoi(seg)
			if err != nil || i < 0 || i >= len(v) {
				return nil, errNoMember
			}
			cur = v[i]
		default:
			return nil, errNoMember
		}
	}
	return cur, nil
}

func unescapePointer(seg string) string {
	return strings.ReplaceAll(strings.ReplaceAll(seg, "~1", "/"), "~0", "~")
}

func escapePointer(seg string) string {
	return strings.ReplaceAll(strings.ReplaceAll(seg, "~", "~0"), "/", "~1")
}

// recordKeyOrder is each record's top-level keys in the order the document
// writes them, so the values a record sets are reported in that order.
func recordKeyOrder(data []byte, records string) ([][]string, error) {
	raw := json.RawMessage(data)
	if records != "" {
		for _, seg := range strings.Split(records[1:], "/") {
			seg = unescapePointer(seg)
			var obj map[string]json.RawMessage
			if err := json.Unmarshal(raw, &obj); err == nil {
				raw = obj[seg]
				continue
			}
			var list []json.RawMessage
			if err := json.Unmarshal(raw, &list); err != nil {
				return nil, err
			}
			i, err := strconv.Atoi(seg)
			if err != nil || i < 0 || i >= len(list) {
				return nil, errNoMember
			}
			raw = list[i]
		}
	}
	var list []json.RawMessage
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}
	out := make([][]string, len(list))
	for i, rec := range list {
		keys, err := objectKeys(rec)
		if err != nil {
			return nil, err
		}
		out[i] = keys
	}
	return out, nil
}

// objectKeys is a JSON object's keys in written order; nil for another value.
func objectKeys(raw []byte) ([]string, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, nil
	}
	var keys []string
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, _ := tok.(string)
		keys = append(keys, key)
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return nil, err
		}
	}
	return keys, nil
}
