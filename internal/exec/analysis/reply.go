package analysis

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// ReplyFormat is how a tool's reply is read once the process has run.
type ReplyFormat string

const (
	// ReplyObject is the protocol's one JSON object, the default.
	ReplyObject ReplyFormat = "object"
	// ReplyJSON reads each output by an RFC 6901 pointer into one JSON value.
	ReplyJSON ReplyFormat = "json"
	// ReplyCSV reads each output as a cell of a CSV record.
	ReplyCSV ReplyFormat = "csv"
	// ReplyLines reads each output as the value of a `key = value` line or a regex group.
	ReplyLines ReplyFormat = "lines"
	// ReplyExitCode is the process's exit status as the one output.
	ReplyExitCode ReplyFormat = "exitcode"
)

// Reply is a tool entry's `reply` block: where the reply is read from and where each output
// is found in it. Absent, or `format: "object"`, the reply is the protocol's JSON object.
type Reply struct {
	// Format names how the reply is read; empty reads the `object` protocol.
	Format ReplyFormat `json:"format,omitempty"`
	// Source is `stdout` or `file:<template>`; the template may name {outputDir} alone.
	Source string `json:"source,omitempty"`
	// Outputs maps each tool variable to the selector finding its value; required for
	// every format but `object`.
	Outputs map[string]*ReplyOutput `json:"outputs,omitempty"`
	// Header is whether the first CSV record names the columns; default true.
	Header *bool `json:"header,omitempty"`
	// Delimiter is the CSV field separator, one rune; default a comma.
	Delimiter string `json:"delimiter,omitempty"`
	// Regex is an RE2 expression whose named groups are the `lines` outputs.
	Regex string `json:"regex,omitempty"`
	// Success are the `exitcode` statuses that render true; default [0].
	Success []int `json:"success,omitempty"`
	// ErrorPath is a `json` pointer to the tool's own refusal message.
	ErrorPath string `json:"errorPath,omitempty"`
	// ErrorColumn is the `csv` column holding the same.
	ErrorColumn *Column `json:"errorColumn,omitempty"`
	// ErrorKey is the `lines` key holding the same.
	ErrorKey string `json:"errorKey,omitempty"`

	// compiled holds what checkReply parsed once the block was accepted.
	compiled *compiledReply
}

// compiledReply is the block's selectors, parsed at manifest load.
type compiledReply struct {
	source       template
	header       bool
	delimiter    rune
	regex        *regexp.Regexp
	success      map[int]bool
	errorPath    pointer
	paths        map[string]pointer
	unitPaths    map[string]pointer
	exitVariable string
}

// ReplyOutput is one output's selector; which fields are read depends on the format.
type ReplyOutput struct {
	// Path is the `json` RFC 6901 pointer to the value; the empty pointer names the document.
	Path *string `json:"path,omitempty"`
	// Column is the `csv` column: a header name or a zero-based index.
	Column *Column `json:"column,omitempty"`
	// Row selects among the CSV data records: `first`, `last` or a zero-based index.
	Row *Row `json:"row,omitempty"`
	// Key is the `lines` key; default the variable's name.
	Key string `json:"key,omitempty"`
	// Type is what the text is read as; default `number` (`exitcode`: `boolean`).
	Type ValueType `json:"type,omitempty"`
	// Unit is the value's unit as a fixed expression.
	Unit string `json:"unit,omitempty"`
	// UnitPath is a `json` pointer to a string holding the unit.
	UnitPath string `json:"unitPath,omitempty"`
	// UnitColumn is the `csv` column holding the unit, read from the same row.
	UnitColumn *Column `json:"unitColumn,omitempty"`
}

// ValueType is what an output's text is read as.
type ValueType string

const (
	// TypeNumber is an Integer when the text is one, else a finite Real.
	TypeNumber ValueType = "number"
	// TypeInteger is an Integer alone.
	TypeInteger ValueType = "integer"
	// TypeReal is a finite Real, an integer literal included.
	TypeReal ValueType = "real"
	// TypeBoolean is `true`/`false`, case-insensitive.
	TypeBoolean ValueType = "boolean"
	// TypeString is the text as written.
	TypeString ValueType = "string"
)

// Column is a CSV column: a header name, or a zero-based index as a JSON integer.
type Column struct {
	Name    string
	Index   int
	ByIndex bool
}

// UnmarshalJSON reads a name or a zero-based index; a negative index, a fraction, a
// boolean or null is malformed.
func (c *Column) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err == nil {
		*c = Column{Name: name}
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err != nil {
		return fmt.Errorf("column %s is not a name or a zero-based index", strings.TrimSpace(string(data)))
	}
	i, err := strconv.ParseInt(n.String(), 10, 64)
	if err != nil || i < 0 || i > math.MaxInt {
		return fmt.Errorf("column %s is not a name or a zero-based index", n.String())
	}
	*c = Column{Index: int(i), ByIndex: true}
	return nil
}

// MarshalJSON writes the column as it is read.
func (c Column) MarshalJSON() ([]byte, error) {
	if c.ByIndex {
		return json.Marshal(c.Index)
	}
	return json.Marshal(c.Name)
}

// String spells the column for an error: the name, or the index.
func (c Column) String() string {
	if c.ByIndex {
		return strconv.Itoa(c.Index)
	}
	return c.Name
}

// RowKind is which data records a row selector picks.
type RowKind int

const (
	// RowFirst is the first data record.
	RowFirst RowKind = iota
	// RowLast is the final one, the default.
	RowLast
	// RowIndex is the zero-based nth.
	RowIndex
	// RowAll is every data record in order; parsed, refused at load for now.
	RowAll
)

// Row selects among the CSV data records.
type Row struct {
	Kind  RowKind
	Index int
}

// UnmarshalJSON reads `first`, `last`, `all` or a zero-based index.
func (r *Row) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err == nil {
		switch name {
		case "first":
			*r = Row{Kind: RowFirst}
		case "last":
			*r = Row{Kind: RowLast}
		case "all":
			*r = Row{Kind: RowAll}
		default:
			return fmt.Errorf("row %q is not one of first, last and all, or a zero-based index", name)
		}
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err != nil {
		return fmt.Errorf("row %s is not one of first, last and all, or a zero-based index", strings.TrimSpace(string(data)))
	}
	i, err := strconv.ParseInt(n.String(), 10, 64)
	if err != nil || i < 0 || i > math.MaxInt {
		return fmt.Errorf("row %s is not one of first, last and all, or a zero-based index", n.String())
	}
	*r = Row{Kind: RowIndex, Index: int(i)}
	return nil
}

// MarshalJSON writes the row as it is read: a name, or the index as a JSON integer.
func (r Row) MarshalJSON() ([]byte, error) {
	if r.Kind == RowIndex {
		return json.Marshal(r.Index)
	}
	return json.Marshal(r.String())
}

// String spells the row for an error.
func (r Row) String() string {
	switch r.Kind {
	case RowFirst:
		return "first"
	case RowLast:
		return "last"
	case RowAll:
		return "all"
	}
	return strconv.Itoa(r.Index)
}

// pointer is a parsed RFC 6901 JSON pointer: the tokens between the slashes, unescaped.
type pointer []string

// parsePointer reads an RFC 6901 pointer: the empty string is the whole document, else a
// `/`-separated list of tokens where `~1` spells `/` and `~0` spells `~`.
func parsePointer(text string) (pointer, error) {
	if text == "" {
		return pointer{}, nil
	}
	if !strings.HasPrefix(text, "/") {
		return nil, fmt.Errorf("pointer %q does not start with /", text)
	}
	var p pointer
	for _, token := range strings.Split(text[1:], "/") {
		var unescaped strings.Builder
		for i := 0; i < len(token); i++ {
			if token[i] == '~' {
				if i+1 >= len(token) || (token[i+1] != '0' && token[i+1] != '1') {
					return nil, fmt.Errorf("pointer %q: ~ is escaped only as ~0 and ~1", text)
				}
				unescaped.WriteByte(map[byte]byte{'0': '~', '1': '/'}[token[i+1]])
				i++
				continue
			}
			unescaped.WriteByte(token[i])
		}
		p = append(p, unescaped.String())
	}
	return p, nil
}

// String spells the pointer as it was written, for an error.
func (p pointer) String() string {
	var b strings.Builder
	for _, token := range p {
		b.WriteByte('/')
		b.WriteString(strings.ReplaceAll(strings.ReplaceAll(token, "~", "~0"), "/", "~1"))
	}
	return b.String()
}

// resolve walks doc — the output of a decoder with UseNumber — by the tokens: a map member
// by key, an array by a decimal index; false where the walk cannot continue.
func (p pointer) resolve(doc any) (any, bool) {
	held := doc
	for _, token := range p {
		switch node := held.(type) {
		case map[string]any:
			v, ok := node[token]
			if !ok {
				return nil, false
			}
			held = v
		case []any:
			i, ok := arrayIndex(token, len(node))
			if !ok {
				return nil, false
			}
			held = node[i]
		default:
			return nil, false
		}
	}
	return held, true
}

// arrayIndex is the token as an index into an array of size n: a decimal integer without
// sign or leading zeros, in range.
func arrayIndex(token string, n int) (int, bool) {
	if token == "" || (len(token) > 1 && token[0] == '0') {
		return 0, false
	}
	for i := 0; i < len(token); i++ {
		if token[i] < '0' || token[i] > '9' {
			return 0, false
		}
	}
	i, err := strconv.Atoi(token)
	if err != nil || i >= n {
		return 0, false
	}
	return i, true
}

// checkReply validates an entry's reply block once its invocation's templates are parsed:
// defaults are filled, every member is checked against its format's admission, and the
// selectors that can be parsed at load — pointers, the regex, the source template — are.
func checkReply(entry *ToolEntry) error {
	r := entry.Reply
	if r == nil {
		return nil
	}
	compiled := &compiledReply{header: true, delimiter: ',', success: map[int]bool{0: true},
		paths: make(map[string]pointer), unitPaths: make(map[string]pointer)}
	format := r.Format
	if format == "" {
		format = ReplyObject
	}
	switch format {
	case ReplyObject, ReplyJSON, ReplyCSV, ReplyLines, ReplyExitCode:
	default:
		return fmt.Errorf("reply.format %q is not one of object, json, csv, lines and exitcode", r.Format)
	}
	if format == ReplyExitCode && r.Source != "" {
		return fmt.Errorf("reply.source %q is not read under exitcode", r.Source)
	}
	if err := checkReplySource(entry, compiled); err != nil {
		return err
	}
	if format == ReplyObject {
		if err := checkReplyObject(r); err != nil {
			return err
		}
		r.Format = format
		r.compiled = compiled
		return nil
	}
	if len(r.Outputs) == 0 {
		return fmt.Errorf("reply.outputs is required under format %s", format)
	}
	var unknown []string
	for variable := range r.Outputs {
		if !entry.Accepts(variable) {
			unknown = append(unknown, variable)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return fmt.Errorf("reply.outputs names %q, which variables does not list", unknown[0])
	}
	for _, field := range []struct {
		name  string
		set   bool
		admit ReplyFormat
	}{
		{"header", r.Header != nil, ReplyCSV},
		{"delimiter", r.Delimiter != "", ReplyCSV},
		{"regex", r.Regex != "", ReplyLines},
		{"success", r.Success != nil, ReplyExitCode},
		{"errorPath", r.ErrorPath != "", ReplyJSON},
		{"errorColumn", r.ErrorColumn != nil, ReplyCSV},
		{"errorKey", r.ErrorKey != "", ReplyLines},
	} {
		if field.set && field.admit != format {
			return fmt.Errorf("reply.%s is not a %s member", field.name, format)
		}
	}
	for _, name := range entry.Variables {
		o, ok := r.Outputs[name]
		if !ok {
			continue
		}
		if o == nil {
			return fmt.Errorf("reply.outputs.%s is null", name)
		}
		if err := checkReplyOutput(format, name, o); err != nil {
			return err
		}
	}
	var err error
	switch format {
	case ReplyJSON:
		err = checkReplyJSON(r, entry.Variables, compiled)
	case ReplyCSV:
		err = checkReplyCSV(r, entry.Variables, compiled)
	case ReplyLines:
		err = checkReplyLines(r, entry.Variables, compiled)
	case ReplyExitCode:
		err = checkReplyExitCode(r, entry.Variables, compiled)
	}
	if err != nil {
		return err
	}
	r.Format = format
	r.compiled = compiled
	return nil
}

// checkReplySource parses a `file:` source's template: {outputDir} is the one placeholder
// admitted, and the entry's invocation must hand the directory to the tool.
func checkReplySource(entry *ToolEntry, compiled *compiledReply) error {
	r := entry.Reply
	if r.Source == "" || r.Source == "stdout" {
		return nil
	}
	text, isFile := strings.CutPrefix(r.Source, "file:")
	if !isFile {
		return fmt.Errorf("reply.source %q is not stdout or file:<template>", r.Source)
	}
	if text == "" {
		return errors.New("reply.source names no file after the file: prefix")
	}
	t, err := parseTemplate(text)
	if err != nil {
		return fmt.Errorf("reply.source: %v", err)
	}
	for _, s := range t {
		if s.hole != nil && s.hole.name != holeOutputDir {
			return fmt.Errorf("reply.source admits {outputDir} and no other placeholder, got {%s}", s.hole.name)
		}
	}
	if !t.uses(holeOutputDir) {
		return fmt.Errorf("reply.source %q does not use {outputDir}, which alone says where the tool wrote", r.Source)
	}
	if entry.Invocation == nil || entry.Invocation.compiled == nil || !entry.Invocation.compiled.outputDir {
		return errors.New("reply.source names {outputDir} but no invocation template hands it to the tool")
	}
	if f := entry.Invocation.InputFile; f != nil {
		rendered, err := t.render(scope{outputDir: "D"})
		if err == nil && filepath.Clean(rendered) == filepath.Clean(filepath.Join("D", f.Name)) {
			return fmt.Errorf("reply.source %q is the invocation's inputFile, which the tool did not write", r.Source)
		}
	}
	compiled.source = t
	return nil
}

// checkReplyObject refuses every member but source under the object protocol.
func checkReplyObject(r *Reply) error {
	for _, field := range []struct {
		name string
		set  bool
	}{
		{"outputs", r.Outputs != nil},
		{"header", r.Header != nil},
		{"delimiter", r.Delimiter != ""},
		{"regex", r.Regex != ""},
		{"success", r.Success != nil},
		{"errorPath", r.ErrorPath != ""},
		{"errorColumn", r.ErrorColumn != nil},
		{"errorKey", r.ErrorKey != ""},
	} {
		if field.set {
			return fmt.Errorf("reply.%s is not an object member", field.name)
		}
	}
	return nil
}

// checkReplyOutput checks one output's members against the format's admission and fills
// its defaults.
func checkReplyOutput(format ReplyFormat, variable string, o *ReplyOutput) error {
	if o.Type == "" {
		o.Type = TypeNumber
		if format == ReplyExitCode {
			o.Type = TypeBoolean
		}
	}
	switch o.Type {
	case TypeNumber, TypeInteger, TypeReal, TypeBoolean, TypeString:
	default:
		return fmt.Errorf("reply.outputs.%s.type %q is not one of number, integer, real, boolean and string", variable, o.Type)
	}
	for _, field := range []struct {
		name  string
		set   bool
		admit ReplyFormat
	}{
		{"path", o.Path != nil, ReplyJSON},
		{"column", o.Column != nil, ReplyCSV},
		{"row", o.Row != nil, ReplyCSV},
		{"key", o.Key != "", ReplyLines},
		{"unitPath", o.UnitPath != "", ReplyJSON},
		{"unitColumn", o.UnitColumn != nil, ReplyCSV},
	} {
		if field.set && field.admit != format {
			return fmt.Errorf("reply.outputs.%s.%s is not a %s member", variable, field.name, format)
		}
	}
	if format == ReplyExitCode {
		if o.Unit != "" {
			return fmt.Errorf("reply.outputs.%s.unit is not a %s member", variable, format)
		}
		if o.Type != TypeBoolean && o.Type != TypeInteger {
			return fmt.Errorf("reply.outputs.%s.type %q is not one of boolean and integer under exitcode", variable, o.Type)
		}
		return nil
	}
	if o.Unit != "" && (o.UnitPath != "" || o.UnitColumn != nil) {
		return fmt.Errorf("reply.outputs.%s names both unit and a unit selector", variable)
	}
	if o.Type == TypeBoolean || o.Type == TypeString {
		if o.Unit != "" || o.UnitPath != "" || o.UnitColumn != nil {
			return fmt.Errorf("reply.outputs.%s: a %s has no unit", variable, o.Type)
		}
	}
	switch format {
	case ReplyJSON:
		if o.Path == nil {
			return fmt.Errorf("reply.outputs.%s needs a path", variable)
		}
	case ReplyCSV:
		if o.Column == nil {
			return fmt.Errorf("reply.outputs.%s needs a column", variable)
		}
		if o.Row == nil {
			o.Row = &Row{Kind: RowLast}
		}
		if o.Row.Kind == RowAll {
			return fmt.Errorf("reply.outputs.%s.row: all is not supported yet", variable)
		}
	}
	return nil
}

// checkReplyJSON parses the pointers: each output's path and unitPath, and errorPath.
func checkReplyJSON(r *Reply, variables []string, compiled *compiledReply) error {
	for _, variable := range r.outputsOrdered(variables, nil) {
		o := r.Outputs[variable]
		p, err := parsePointer(*o.Path)
		if err != nil {
			return fmt.Errorf("reply.outputs.%s.path: %v", variable, err)
		}
		compiled.paths[variable] = p
		if o.UnitPath != "" {
			p, err := parsePointer(o.UnitPath)
			if err != nil {
				return fmt.Errorf("reply.outputs.%s.unitPath: %v", variable, err)
			}
			compiled.unitPaths[variable] = p
		}
	}
	if r.ErrorPath != "" {
		p, err := parsePointer(r.ErrorPath)
		if err != nil {
			return fmt.Errorf("reply.errorPath: %v", err)
		}
		compiled.errorPath = p
	}
	return nil
}

// checkReplyCSV checks the delimiter and that a name column is used only with a header.
func checkReplyCSV(r *Reply, variables []string, compiled *compiledReply) error {
	if r.Header != nil {
		compiled.header = *r.Header
	}
	if r.Delimiter != "" {
		d, size := utf8.DecodeRuneInString(r.Delimiter)
		if size != len(r.Delimiter) || !validDelimiter(d) {
			return fmt.Errorf("reply.delimiter %q is not a single valid delimiter", r.Delimiter)
		}
		compiled.delimiter = d
	}
	if !compiled.header {
		for _, variable := range r.outputsOrdered(variables, nil) {
			o := r.Outputs[variable]
			if o.Column != nil && !o.Column.ByIndex {
				return fmt.Errorf("reply.outputs.%s.column %q names a column but header is false", variable, o.Column.Name)
			}
			if o.UnitColumn != nil && !o.UnitColumn.ByIndex {
				return fmt.Errorf("reply.outputs.%s.unitColumn %q names a column but header is false", variable, o.UnitColumn.Name)
			}
		}
		if r.ErrorColumn != nil && !r.ErrorColumn.ByIndex {
			return fmt.Errorf("reply.errorColumn %q names a column but header is false", r.ErrorColumn.Name)
		}
	}
	return nil
}

// validDelimiter is what encoding/csv accepts as a field separator.
func validDelimiter(d rune) bool {
	return d != 0 && d != '"' && d != '\r' && d != '\n' && utf8.ValidRune(d) && d != utf8.RuneError
}

// checkReplyLines compiles the regex and checks the key forms against it: with a regex
// every named group is an output and every output a group, and no key or errorKey is read;
// without one no two outputs may share a key.
func checkReplyLines(r *Reply, variables []string, compiled *compiledReply) error {
	if r.Regex == "" {
		byKey := make(map[string]string, len(r.Outputs))
		for _, variable := range r.outputsOrdered(variables, nil) {
			o := r.Outputs[variable]
			if o.Key == "" {
				o.Key = variable
			}
			if other, taken := byKey[o.Key]; taken {
				return fmt.Errorf("reply.outputs.%s and reply.outputs.%s share the key %q", other, variable, o.Key)
			}
			byKey[o.Key] = variable
		}
		return nil
	}
	for _, variable := range r.outputsOrdered(variables, nil) {
		if o := r.Outputs[variable]; o.Key != "" {
			return fmt.Errorf("reply.outputs.%s.key is not read beside regex", variable)
		}
	}
	if r.ErrorKey != "" {
		return errors.New("reply.errorKey is not read under regex")
	}
	re, err := regexp.Compile(r.Regex)
	if err != nil {
		return fmt.Errorf("reply.regex: %v", err)
	}
	named := make(map[string]bool)
	for _, name := range re.SubexpNames()[1:] {
		if name == "" {
			continue
		}
		if _, ok := r.Outputs[name]; !ok {
			return fmt.Errorf("reply.regex names group %q, which outputs does not list", name)
		}
		if named[name] {
			return fmt.Errorf("reply.regex names group %q twice", name)
		}
		named[name] = true
	}
	for _, variable := range r.outputsOrdered(variables, nil) {
		if !named[variable] {
			return fmt.Errorf("reply.outputs.%s is named by no group of reply.regex", variable)
		}
	}
	compiled.regex = re
	return nil
}

// checkReplyExitCode checks the one output and the success list.
func checkReplyExitCode(r *Reply, variables []string, compiled *compiledReply) error {
	if len(r.Outputs) != 1 {
		return fmt.Errorf("reply.outputs names %d variables; exitcode reads exactly one", len(r.Outputs))
	}
	compiled.exitVariable = r.outputsOrdered(variables, nil)[0]
	compiled.success = make(map[int]bool, len(r.Success))
	if r.Success == nil {
		compiled.success[0] = true
	}
	for i, code := range r.Success {
		if code < 0 {
			return fmt.Errorf("reply.success[%d] is negative", i)
		}
		if compiled.success[code] {
			return fmt.Errorf("reply.success names %d twice", code)
		}
		compiled.success[code] = true
	}
	return nil
}

// toolFault is one failure of a reply read.
func toolFault(tool string, kind runtime.ToolErrorKind, format string, args ...any) error {
	return &runtime.ToolError{Tool: tool, Kind: kind, Detail: fmt.Sprintf(format, args...)}
}

// readExitCode is the process's exit status as the one output: true when it is among
// success, or the status itself as an Integer.
func (r *Reply) readExitCode(ex *execution, wanted map[string]bool) map[string]runtime.ToolValue {
	variable := r.compiled.exitVariable
	if wanted != nil && !wanted[variable] {
		return map[string]runtime.ToolValue{}
	}
	o := r.Outputs[variable]
	if o.Type == TypeInteger {
		return map[string]runtime.ToolValue{variable: {Value: semantics.Value{Kind: semantics.ValInt, Int: int64(ex.exit)}}}
	}
	return map[string]runtime.ToolValue{variable: {Value: semantics.Value{Kind: semantics.ValBool, Bool: r.compiled.success[ex.exit]}}}
}

// typeText reads one cell or line value as the declared type.
func typeText(text string, t ValueType) (runtime.ToolValue, error) {
	switch t {
	case TypeInteger:
		i, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return runtime.ToolValue{}, fmt.Errorf("%s is not an integer", text)
		}
		return runtime.ToolValue{Value: semantics.Value{Kind: semantics.ValInt, Int: i}}, nil
	case TypeReal:
		f, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return runtime.ToolValue{}, fmt.Errorf("%s is not a number", text)
		}
		if math.IsInf(f, 0) || math.IsNaN(f) {
			return runtime.ToolValue{}, fmt.Errorf("%s is not a finite number", text)
		}
		return runtime.ToolValue{Value: semantics.Value{Kind: semantics.ValReal, Real: f}}, nil
	case TypeBoolean:
		if strings.EqualFold(text, "true") {
			return runtime.ToolValue{Value: semantics.Value{Kind: semantics.ValBool, Bool: true}}, nil
		}
		if strings.EqualFold(text, "false") {
			return runtime.ToolValue{Value: semantics.Value{Kind: semantics.ValBool, Bool: false}}, nil
		}
		return runtime.ToolValue{}, fmt.Errorf("%s is not a boolean", text)
	case TypeString:
		return runtime.ToolValue{Text: text}, nil
	}
	if i, err := strconv.ParseInt(text, 10, 64); err == nil {
		return runtime.ToolValue{Value: semantics.Value{Kind: semantics.ValInt, Int: i}}, nil
	}
	f, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return runtime.ToolValue{}, fmt.Errorf("%s is not a number", text)
	}
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return runtime.ToolValue{}, fmt.Errorf("%s is not a finite number", text)
	}
	return runtime.ToolValue{Value: semantics.Value{Kind: semantics.ValReal, Real: f}}, nil
}

// outputsOrdered is each output's variable in the entry's variables order, so the first
// failure of a read or a load check is deterministic.
func (r *Reply) outputsOrdered(variables []string, wanted map[string]bool) []string {
	names := make([]string, 0, len(r.Outputs))
	for _, name := range variables {
		if _, ok := r.Outputs[name]; ok && (wanted == nil || wanted[name]) {
			names = append(names, name)
		}
	}
	return names
}

// readJSON parses the one JSON value and resolves each output's pointer into it, the
// refusal at errorPath first.
func (r *Reply) readJSON(entry ToolEntry, source []byte, wanted map[string]bool) (map[string]runtime.ToolValue, error) {
	tool := entry.ToolName
	if len(bytes.TrimSpace(source)) == 0 {
		return nil, toolFault(tool, runtime.ToolMalformed, "wrote nothing")
	}
	dec := json.NewDecoder(bytes.NewReader(source))
	dec.UseNumber()
	var doc any
	if err := dec.Decode(&doc); err != nil {
		return nil, toolFault(tool, runtime.ToolMalformed, "the reply is not one JSON value: %v", err)
	}
	var trailing json.RawMessage
	if err := dec.Decode(&trailing); err != io.EOF {
		return nil, toolFault(tool, runtime.ToolMalformed, "the reply is more than one JSON value")
	}
	if path, twice := repeatedKey(source); twice {
		return nil, toolFault(tool, runtime.ToolMalformed, "the reply names %s twice", path)
	}
	if r.ErrorPath != "" {
		if v, found := r.compiled.errorPath.resolve(doc); found {
			message, isText := v.(string)
			if !isText {
				return nil, toolFault(tool, runtime.ToolMalformed, "errorPath %s holds %s, not a message",
					r.ErrorPath, jsonKindOf(v))
			}
			if message != "" {
				return nil, toolFault(tool, runtime.ToolRefused, "%s", message)
			}
		}
	}
	outputs := make(map[string]runtime.ToolValue, len(r.Outputs))
	for _, variable := range r.outputsOrdered(entry.Variables, wanted) {
		o := r.Outputs[variable]
		p := r.compiled.paths[variable]
		v, found := p.resolve(doc)
		if !found {
			return nil, toolFault(tool, runtime.ToolMissingOutput, "%s at %s: nothing there", variable, p)
		}
		value, err := jsonOutputValue(o, v)
		if err != nil {
			return nil, toolFault(tool, runtime.ToolMalformed, "%s at %s: %v", variable, p, err)
		}
		if up, ok := r.compiled.unitPaths[variable]; ok {
			u, found := up.resolve(doc)
			if !found {
				return nil, toolFault(tool, runtime.ToolMissingOutput, "%s at %s: nothing there", variable, up)
			}
			unit, isText := u.(string)
			if !isText {
				return nil, toolFault(tool, runtime.ToolMalformed, "%s at %s: %s is not a unit", variable, up, jsonKindOf(u))
			}
			if unit == "" {
				return nil, toolFault(tool, runtime.ToolMalformed, "%s at %s: an empty unit", variable, up)
			}
			value.Unit = unit
		}
		outputs[variable] = value
	}
	return outputs, nil
}

// jsonKindOf names a decoded JSON value as an error does.
func jsonKindOf(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case map[string]any:
		return "an object"
	case []any:
		return "an array"
	case json.Number:
		return "a number"
	case bool:
		return "a boolean"
	case string:
		return "a string"
	}
	return "a value"
}

// jsonOutputValue reads one resolved value against the declared type: a number, boolean or
// string scalar; an array is not supported yet, an object or null is malformed.
func jsonOutputValue(o *ReplyOutput, v any) (runtime.ToolValue, error) {
	out := runtime.ToolValue{Unit: o.Unit}
	switch value := v.(type) {
	case []any:
		return runtime.ToolValue{}, errors.New("an array is not supported yet")
	case map[string]any:
		return runtime.ToolValue{}, errors.New("an object is not a number, boolean or string")
	case nil:
		return runtime.ToolValue{}, errors.New("null is not a number, boolean or string")
	case json.Number:
		switch o.Type {
		case TypeBoolean:
			return runtime.ToolValue{}, errors.New("number where boolean expected")
		case TypeString:
			return runtime.ToolValue{}, errors.New("number where string expected")
		}
		var held semantics.Value
		if i, err := strconv.ParseInt(value.String(), 10, 64); err == nil {
			held = semantics.Value{Kind: semantics.ValInt, Int: i}
		} else {
			f, err := strconv.ParseFloat(value.String(), 64)
			if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
				return runtime.ToolValue{}, fmt.Errorf("%s is not a finite number", value.String())
			}
			held = semantics.Value{Kind: semantics.ValReal, Real: f}
		}
		switch o.Type {
		case TypeInteger:
			if held.Kind != semantics.ValInt {
				return runtime.ToolValue{}, fmt.Errorf("%s is not an integer", value.String())
			}
		case TypeReal:
			if held.Kind == semantics.ValInt {
				held = semantics.Value{Kind: semantics.ValReal, Real: float64(held.Int)}
			}
		}
		out.Value = held
	case bool:
		if o.Type != TypeBoolean {
			return runtime.ToolValue{}, fmt.Errorf("boolean where %s expected", o.Type)
		}
		out.Value = semantics.Value{Kind: semantics.ValBool, Bool: value}
	case string:
		if o.Type != TypeString {
			return runtime.ToolValue{}, fmt.Errorf("string where %s expected", o.Type)
		}
		out.Text = value
	}
	return out, nil
}

// readCSV parses the records and reads each output's cell: the refusal at errorColumn first.
func (r *Reply) readCSV(entry ToolEntry, source []byte, wanted map[string]bool) (map[string]runtime.ToolValue, error) {
	tool := entry.ToolName
	source = bytes.TrimPrefix(source, []byte("\xef\xbb\xbf"))
	reader := csv.NewReader(bytes.NewReader(source))
	reader.Comma = r.compiled.delimiter
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, toolFault(tool, runtime.ToolMalformed, "the reply is not CSV: %v", err)
	}
	if len(records) > 0 {
		width := len(records[0])
		for i, record := range records[1:] {
			if len(record) != width {
				return nil, toolFault(tool, runtime.ToolMalformed, "record %d has %d fields, not %d", i+2, len(record), width)
			}
		}
	}
	var data [][]string
	headerByName := make(map[string]int)
	if r.compiled.header {
		if len(records) == 0 {
			return nil, toolFault(tool, runtime.ToolMalformed, "wrote no header record")
		}
		for i, name := range records[0] {
			if _, dup := headerByName[name]; dup {
				return nil, toolFault(tool, runtime.ToolMalformed, "the header names %s twice", name)
			}
			headerByName[name] = i
		}
		data = records[1:]
	} else {
		data = records
	}
	width := 0
	if len(records) > 0 {
		width = len(records[0])
	}
	indexOf := func(c *Column) (int, error) {
		if c.ByIndex {
			if c.Index < 0 || c.Index >= width {
				return 0, fmt.Errorf("column %d is past the %d fields of a record", c.Index, width)
			}
			return c.Index, nil
		}
		i, ok := headerByName[c.Name]
		if !ok {
			return 0, fmt.Errorf("column %s is not in the header", c.Name)
		}
		return i, nil
	}
	if r.ErrorColumn != nil && len(data) > 0 {
		col, err := indexOf(r.ErrorColumn)
		if err != nil {
			return nil, toolFault(tool, runtime.ToolMalformed, "%v", err)
		}
		if message := strings.TrimSpace(data[len(data)-1][col]); message != "" {
			return nil, toolFault(tool, runtime.ToolRefused, "%s", message)
		}
	}
	outputs := make(map[string]runtime.ToolValue, len(r.Outputs))
	for _, variable := range r.outputsOrdered(entry.Variables, wanted) {
		o := r.Outputs[variable]
		col, err := indexOf(o.Column)
		if err != nil {
			return nil, toolFault(tool, runtime.ToolMalformed, "%s in column %s: %v", variable, o.Column, err)
		}
		row, err := csvRow(o.Row, len(data))
		if err != nil {
			return nil, toolFault(tool, runtime.ToolMissingOutput, "%s in column %s, row %s: %v", variable, o.Column, o.Row, err)
		}
		if row < 0 || row >= len(data) {
			return nil, toolFault(tool, runtime.ToolMalformed, "%s in column %s, row %s: only %d rows", variable, o.Column, o.Row, len(data))
		}
		cell := data[row][col]
		if o.Type != TypeString {
			cell = strings.TrimSpace(cell)
		}
		if cell == "" && o.Type != TypeString {
			return nil, toolFault(tool, runtime.ToolMissingOutput, "%s in column %s, row %s: an empty cell", variable, o.Column, o.Row)
		}
		value, err := typeText(cell, o.Type)
		if err != nil {
			return nil, toolFault(tool, runtime.ToolMalformed, "%s in column %s, row %s: %v", variable, o.Column, o.Row, err)
		}
		value.Unit = o.Unit
		if o.UnitColumn != nil {
			ucol, err := indexOf(o.UnitColumn)
			if err != nil {
				return nil, toolFault(tool, runtime.ToolMalformed, "unit %v", err)
			}
			unit := strings.TrimSpace(data[row][ucol])
			if unit == "" {
				return nil, toolFault(tool, runtime.ToolMalformed, "unit cell in column %s, row %s is empty", o.UnitColumn, o.Row)
			}
			value.Unit = unit
		}
		outputs[variable] = value
	}
	return outputs, nil
}

// csvRow is the data record a selector picks; false when there are none.
func csvRow(row *Row, n int) (int, error) {
	if n == 0 {
		return 0, errors.New("no data record")
	}
	switch row.Kind {
	case RowFirst:
		return 0, nil
	case RowLast:
		return n - 1, nil
	}
	return row.Index, nil
}

// readLines parses `key = value` lines or matches the regex's named groups per line, the
// refusal at errorKey first. Line numbers are 1-based.
func (r *Reply) readLines(entry ToolEntry, source []byte, wanted map[string]bool) (map[string]runtime.ToolValue, error) {
	tool := entry.ToolName
	raw := strings.Split(string(source), "\n")
	lines := make([]string, len(raw))
	for i, line := range raw {
		lines[i] = strings.TrimSuffix(line, "\r")
	}
	if r.compiled.regex != nil {
		return r.readLinesRegex(entry, lines, wanted)
	}
	type hit struct {
		value string
		line  int
	}
	found := make(map[string][]hit)
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		at := strings.IndexAny(line, "=:")
		if at < 0 {
			continue
		}
		key := strings.TrimSpace(line[:at])
		found[key] = append(found[key], hit{value: strings.TrimSpace(line[at+1:]), line: i + 1})
	}
	if r.ErrorKey != "" {
		for _, h := range found[r.ErrorKey] {
			if h.value != "" {
				return nil, toolFault(tool, runtime.ToolRefused, "%s", h.value)
			}
		}
	}
	outputs := make(map[string]runtime.ToolValue, len(r.Outputs))
	for _, variable := range r.outputsOrdered(entry.Variables, wanted) {
		o := r.Outputs[variable]
		hits := found[o.Key]
		if len(hits) == 0 {
			return nil, toolFault(tool, runtime.ToolMissingOutput, "%s under key %s: no line", variable, o.Key)
		}
		if len(hits) > 1 {
			return nil, toolFault(tool, runtime.ToolMalformed, "%s under key %s, lines %d and %d", variable, o.Key, hits[0].line, hits[1].line)
		}
		value, err := typeText(hits[0].value, o.Type)
		if err != nil {
			return nil, toolFault(tool, runtime.ToolMalformed, "%s under key %s, line %d: %v", variable, o.Key, hits[0].line, err)
		}
		value.Unit = o.Unit
		outputs[variable] = value
	}
	return outputs, nil
}

// readLinesRegex reads each output from the named group matching it on exactly one line.
func (r *Reply) readLinesRegex(entry ToolEntry, lines []string, wanted map[string]bool) (map[string]runtime.ToolValue, error) {
	tool := entry.ToolName
	re := r.compiled.regex
	outputs := make(map[string]runtime.ToolValue, len(r.Outputs))
	for _, variable := range r.outputsOrdered(entry.Variables, wanted) {
		o := r.Outputs[variable]
		group := re.SubexpIndex(variable)
		var text string
		matched, at := 0, 0
		for i, line := range lines {
			m := re.FindStringSubmatchIndex(line)
			if m == nil || m[2*group] < 0 {
				continue
			}
			if matched == 0 {
				text, at = line[m[2*group]:m[2*group+1]], i+1
			}
			matched++
			if matched == 2 {
				return nil, toolFault(tool, runtime.ToolMalformed, "%s from group %s, lines %d and %d", variable, variable, at, i+1)
			}
		}
		if matched == 0 {
			return nil, toolFault(tool, runtime.ToolMissingOutput, "%s from group %s: no line", variable, variable)
		}
		value, err := typeText(text, o.Type)
		if err != nil {
			return nil, toolFault(tool, runtime.ToolMalformed, "%s from group %s, line %d: %v", variable, variable, at, err)
		}
		value.Unit = o.Unit
		outputs[variable] = value
	}
	return outputs, nil
}
