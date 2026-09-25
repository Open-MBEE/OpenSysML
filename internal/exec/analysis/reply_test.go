package analysis

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// jsonPath is the *string a programmatic output's Path holds.
func jsonPath(s string) *string { return &s }

// replyEntryText is a tool entry with the reply block given, as a manifest file spells it.
func replyEntryText(reply string) string {
	return `{"toolName": "Thermal", "executable": "solve", "variables": ["mass", "T_max", "v_out", "done", "code", "note", "a", "b"], "reply": ` + reply + `}`
}

func TestManifestReplyAcceptedShapes(t *testing.T) {
	cases := map[string]string{
		"json":        `{"format": "json", "outputs": {"T_max": {"path": "/results/0/T_max", "unitPath": "/units/T_max"}}, "errorPath": "/error"}`,
		"json root":   `{"format": "json", "outputs": {"T_max": {"path": ""}}}`,
		"csv":         `{"format": "csv", "header": true, "delimiter": ";", "outputs": {"T_max": {"column": "T_max", "row": "first", "unitColumn": "U"}}, "errorColumn": "error"}`,
		"csv index":   `{"format": "csv", "header": false, "delimiter": "\t", "outputs": {"T_max": {"column": 2, "row": 3}}}`,
		"lines":       `{"format": "lines", "outputs": {"T_max": {"key": "Tmax"}, "done": {"type": "boolean"}}, "errorKey": "error"}`,
		"lines regex": `{"format": "lines", "regex": "T=(?P<T_max>[0-9.]+)", "outputs": {"T_max": {"type": "real"}}}`,
		"exitcode":    `{"format": "exitcode", "success": [0, 3], "outputs": {"done": {"type": "boolean"}}}`,
		"object":      `{"format": "object"}`,
		"stdout":      `{"format": "csv", "source": "stdout", "outputs": {"T_max": {"column": 0}}}`,
	}
	for name, reply := range cases {
		t.Run(name, func(t *testing.T) {
			dir := writeManifest(t, map[string]string{"thermal.json": replyEntryText(reply)})
			entries, err := LoadManifest(dir)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			if len(entries) != 1 || entries[0].Reply == nil || entries[0].Reply.compiled == nil {
				t.Fatalf("entries %+v, want one with its reply compiled", entries)
			}
		})
	}
	// A file source is admitted beside an invocation that hands {outputDir} to the tool.
	dir := writeManifest(t, map[string]string{"thermal.json": `{"toolName": "Thermal", "executable": "solve", "variables": ["T_max"],
		"invocation": {"args": ["{outputDir}"]},
		"reply": {"format": "json", "source": "file:{outputDir}/result.json", "outputs": {"T_max": {"path": "/T_max"}}}}`})
	if _, err := LoadManifest(dir); err != nil {
		t.Fatalf("file source beside an {outputDir} invocation: %v", err)
	}
	// The object protocol admits a file source too, for a tool writing the object to one.
	dir = writeManifest(t, map[string]string{"thermal.json": `{"toolName": "Thermal", "executable": "solve", "variables": ["T_max"],
		"invocation": {"args": ["{outputDir}"]},
		"reply": {"format": "object", "source": "file:{outputDir}/result.json"}}`})
	if _, err := LoadManifest(dir); err != nil {
		t.Fatalf("object reply with a file source: %v", err)
	}
	// A file source beside an inputFile is admitted while the names differ.
	dir = writeManifest(t, map[string]string{"thermal.json": `{"toolName": "Thermal", "executable": "solve", "variables": ["T_max"],
		"invocation": {"args": ["{outputDir}"], "inputFile": {"format": "csv", "name": "inputs.csv"}},
		"reply": {"format": "json", "source": "file:{outputDir}/result.json", "outputs": {"T_max": {"path": "/T_max"}}}}`})
	if _, err := LoadManifest(dir); err != nil {
		t.Fatalf("file source beside a differently named inputFile: %v", err)
	}
}

func TestManifestReplyRefusedShapes(t *testing.T) {
	cases := map[string]struct {
		text   string
		detail string
	}{
		"unknown key in reply":  {replyEntryText(`{"format": "csv", "shell": true, "outputs": {"T_max": {"column": 0}}}`), `unknown field "shell"`},
		"unknown key in output": {replyEntryText(`{"format": "csv", "outputs": {"T_max": {"column": 0, "page": 1}}}`), `unknown field "page"`},
		"format unknown":        {replyEntryText(`{"format": "yaml", "outputs": {}}`), `reply.format "yaml" is not one of object, json, csv, lines and exitcode`},
		"output not a variable": {replyEntryText(`{"format": "csv", "outputs": {"Tmax": {"column": 0}}}`), `reply.outputs names "Tmax", which variables does not list`},
		"null output":           {replyEntryText(`{"format": "csv", "outputs": {"T_max": null}}`), `the entry sets reply.outputs.T_max to null`},
		"null outputs":          {replyEntryText(`{"format": "object", "outputs": null}`), `the entry sets reply.outputs to null`},
		"null header":           {replyEntryText(`{"format": "csv", "header": null, "outputs": {"T_max": {"column": 0}}}`), `the entry sets reply.header to null`},
		"duplicate regex group": {replyEntryText(`{"format": "lines", "regex": "(?P<code>\\d+)|code=(?P<code>\\d+)", "outputs": {"code": {}}}`), `reply.regex names group "code" twice`},
		"key naming the variable": {replyEntryText(`{"format": "lines", "regex": "(?P<code>\\d+)", "outputs": {"code": {"key": "code"}}}`),
			`reply.outputs.code.key is not read beside regex`},
		"source is the inputFile": {`{"toolName": "Thermal", "executable": "solve", "variables": ["T_max"],
			"invocation": {"args": ["{outputDir}"], "inputFile": {"format": "csv", "name": "result.json"}},
			"reply": {"format": "json", "source": "file:{outputDir}/result.json", "outputs": {"T_max": {"path": "/x"}}}}`,
			`reply.source "file:{outputDir}/result.json" is the invocation's inputFile, which the tool did not write`},
		"column under json":         {replyEntryText(`{"format": "json", "outputs": {"T_max": {"path": "/x", "column": 0}}}`), `reply.outputs.T_max.column is not a json member`},
		"regex under csv":           {replyEntryText(`{"format": "csv", "regex": "x", "outputs": {"T_max": {"column": 0}}}`), `reply.regex is not a csv member`},
		"success under json":        {replyEntryText(`{"format": "json", "success": [0], "outputs": {"T_max": {"path": "/x"}}}`), `reply.success is not a json member`},
		"unit on boolean":           {replyEntryText(`{"format": "csv", "outputs": {"done": {"column": 0, "type": "boolean", "unit": "s"}}}`), `a boolean has no unit`},
		"unit on string":            {replyEntryText(`{"format": "csv", "outputs": {"note": {"column": 0, "type": "string", "unit": "s"}}}`), `a string has no unit`},
		"unit with unitColumn":      {replyEntryText(`{"format": "csv", "outputs": {"T_max": {"column": 0, "unit": "K", "unitColumn": "U"}}}`), `names both unit and a unit selector`},
		"row all":                   {replyEntryText(`{"format": "csv", "outputs": {"T_max": {"column": 0, "row": "all"}}}`), `reply.outputs.T_max.row: all is not supported yet`},
		"row negative":              {replyEntryText(`{"format": "csv", "outputs": {"T_max": {"column": 0, "row": -1}}}`), `row -1 is not`},
		"column negative":           {replyEntryText(`{"format": "csv", "outputs": {"T_max": {"column": -2}}}`), `column -2 is not`},
		"headerless name":           {replyEntryText(`{"format": "csv", "header": false, "outputs": {"T_max": {"column": "T_max"}}}`), `names a column but header is false`},
		"bad delimiter":             {replyEntryText(`{"format": "csv", "delimiter": ";;", "outputs": {"T_max": {"column": 0}}}`), `reply.delimiter ";;" is not a single valid delimiter`},
		"quote delimiter":           {replyEntryText(`{"format": "csv", "delimiter": "\"", "outputs": {"T_max": {"column": 0}}}`), `is not a single valid delimiter`},
		"malformed pointer":         {replyEntryText(`{"format": "json", "outputs": {"T_max": {"path": "T_max"}}}`), `does not start with /`},
		"bad tilde escape":          {replyEntryText(`{"format": "json", "outputs": {"T_max": {"path": "/a~2"}}}`), `~ is escaped only as ~0 and ~1`},
		"bad regex":                 {replyEntryText(`{"format": "lines", "regex": "(", "outputs": {"T_max": {}}}`), `reply.regex`},
		"group not an output":       {replyEntryText(`{"format": "lines", "regex": "(?P<x>.)", "outputs": {"T_max": {}}}`), `names group "x", which outputs does not list`},
		"output without group":      {replyEntryText(`{"format": "lines", "regex": "(?P<T_max>.)", "outputs": {"T_max": {}, "done": {}}}`), `reply.outputs.done is named by no group`},
		"key beside regex":          {replyEntryText(`{"format": "lines", "regex": "(?P<T_max>.)", "outputs": {"T_max": {"key": "t"}}}`), `key is not read beside regex`},
		"errorKey beside regex":     {replyEntryText(`{"format": "lines", "regex": "(?P<T_max>.)", "errorKey": "error", "outputs": {"T_max": {}}}`), `errorKey is not read under regex`},
		"exitcode two outputs":      {replyEntryText(`{"format": "exitcode", "outputs": {"done": {}, "code": {}}}`), `exitcode reads exactly one`},
		"exitcode real":             {replyEntryText(`{"format": "exitcode", "outputs": {"code": {"type": "real"}}}`), `not one of boolean and integer`},
		"exitcode unit":             {replyEntryText(`{"format": "exitcode", "outputs": {"code": {"type": "integer", "unit": "s"}}}`), `unit is not a exitcode member`},
		"exitcode source":           {replyEntryText(`{"format": "exitcode", "source": "stdout", "outputs": {"done": {}}}`), `not read under exitcode`},
		"shared key":                {replyEntryText(`{"format": "lines", "outputs": {"a": {"key": "k"}, "b": {"key": "k"}}}`), `share the key "k"`},
		"csv needs column":          {replyEntryText(`{"format": "csv", "outputs": {"T_max": {}}}`), `reply.outputs.T_max needs a column`},
		"json needs path":           {replyEntryText(`{"format": "json", "outputs": {"T_max": {}}}`), `reply.outputs.T_max needs a path`},
		"outputs empty":             {replyEntryText(`{"format": "csv", "outputs": {}}`), `reply.outputs is required`},
		"object with outputs":       {replyEntryText(`{"format": "object", "outputs": {"T_max": {}}}`), `reply.outputs is not an object member`},
		"object with empty outputs": {replyEntryText(`{"format": "object", "outputs": {}}`), `reply.outputs is not an object member`},
		"object with header":        {replyEntryText(`{"format": "object", "header": false}`), `reply.header is not an object member`},
		"source unknown":            {replyEntryText(`{"format": "csv", "source": "stderr", "outputs": {"T_max": {"column": 0}}}`), `is not stdout or file:<template>`},
		"file without outputDir": {`{"toolName": "T", "executable": "solve", "variables": ["T_max"],
			"reply": {"format": "csv", "source": "file:result.csv", "outputs": {"T_max": {"column": 0}}}}`, `does not use {outputDir}`},
		"file with a variable": {`{"toolName": "T", "executable": "solve", "variables": ["mass", "T_max"],
			"reply": {"format": "csv", "source": "file:{outputDir}/{mass}.csv", "outputs": {"T_max": {"column": 0}}}}`, `admits {outputDir} and no other placeholder`},
		"file without invocation": {`{"toolName": "T", "executable": "solve", "variables": ["T_max"],
			"reply": {"format": "csv", "source": "file:{outputDir}/r.csv", "outputs": {"T_max": {"column": 0}}}}`, `no invocation template hands it to the tool`},
		"empty file": {`{"toolName": "T", "executable": "solve", "variables": ["T_max"],
			"reply": {"format": "csv", "source": "file:", "outputs": {"T_max": {"column": 0}}}}`, `names no file after the file: prefix`},
		"repeated key": {`{"toolName": "T", "executable": "solve", "variables": ["T_max"],
			"reply": {"format": "csv", "outputs": {"T_max": {"column": 0}, "T_max": {"column": 1}}}}`, `names reply.outputs.T_max twice`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dir := writeManifest(t, map[string]string{"entry.json": tc.text})
			_, err := LoadManifest(dir)
			var fault *ManifestError
			if !errors.As(err, &fault) || !errors.Is(err, ErrManifest) {
				t.Fatalf("load: %v, want a ManifestError", err)
			}
			if !strings.Contains(err.Error(), tc.detail) {
				t.Errorf("error %q, want it to say %q", err, tc.detail)
			}
		})
	}
}

func TestPointerParsesAndEscapes(t *testing.T) {
	valid := map[string]pointer{
		"":         {},
		"/a":       {"a"},
		"/a/b":     {"a", "b"},
		"/a~0b":    {"a~b"},
		"/a~1b":    {"a/b"},
		"/a~0~1b":  {"a~/b"},
		"/":        {""},
		"/0/1/01x": {"0", "1", "01x"},
	}
	for text, want := range valid {
		got, err := parsePointer(text)
		if err != nil {
			t.Errorf("%q: %v", text, err)
			continue
		}
		if len(got) != len(want) {
			t.Errorf("%q = %v, want %v", text, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%q = %v, want %v", text, got, want)
			}
		}
	}
	for _, text := range []string{"a", "/a~", "/a~2", "x/y"} {
		if _, err := parsePointer(text); err == nil {
			t.Errorf("%q parsed, want malformed", text)
		}
	}
}

// replyOf compiles one block against the variables the driver declares, for a reader test.
func replyOf(t *testing.T, r *Reply) (*Reply, ToolEntry) {
	t.Helper()
	entry := ToolEntry{ToolName: "Thermal", Executable: "solve",
		Variables: []string{"mass", "T_max", "v_out", "done", "code", "note"}, Reply: r}
	if err := checkReply(&entry); err != nil {
		t.Fatalf("checkReply: %v", err)
	}
	return entry.Reply, entry
}

// readReply parses source with a compiled reply; requested names the outputs the call
// asks for, all of them when none is given. It selects faults as tool.go's read does:
// the first requested variable's, in sorted order. The kind of a fault is checked by callers.
func readReply(t *testing.T, r *Reply, entry ToolEntry, source string, requested ...string) (map[string]runtime.ToolValue, error) {
	t.Helper()
	var outputs map[string]runtime.ToolValue
	var faults map[string]error
	var err error
	switch r.Format {
	case ReplyJSON:
		outputs, faults, err = r.readJSON(entry, []byte(source))
	case ReplyCSV:
		outputs, faults, err = r.readCSV(entry, []byte(source))
	default:
		outputs, faults, err = r.readLines(entry, []byte(source))
	}
	if err != nil {
		return nil, err
	}
	if len(requested) == 0 {
		requested = r.outputsOrdered(entry.Variables)
	}
	sorted := append([]string(nil), requested...)
	sort.Strings(sorted)
	bound := make(map[string]runtime.ToolValue, len(sorted))
	for _, variable := range sorted {
		if fault, ok := faults[variable]; ok {
			return nil, fault
		}
		if value, ok := outputs[variable]; ok {
			bound[variable] = value
		}
	}
	return bound, nil
}

func replyKind(err error) runtime.ToolErrorKind {
	var fault *runtime.ToolError
	if errors.As(err, &fault) {
		return fault.Kind
	}
	return -1
}

func TestReplyReadsJSON(t *testing.T) {
	r, entry := replyOf(t, &Reply{Format: ReplyJSON, ErrorPath: "/error", Outputs: map[string]*ReplyOutput{
		"T_max": {Path: jsonPath("/results/0/T_max"), UnitPath: "/units/T_max"},
		"v_out": {Path: jsonPath("/results/0/v_out"), UnitPath: "/units/v_out"},
		"done":  {Path: jsonPath("/results/0/done"), Type: TypeBoolean},
		"code":  {Path: jsonPath("/results/0/code"), Type: TypeInteger},
		"note":  {Path: jsonPath("/results/0/note"), Type: TypeString},
	}})
	source := `{"results":[{"T_max":341.2,"v_out":36,"done":true,"code":7,"note":"it ran"}],
		"units":{"T_max":"K","v_out":"km/h"},"error":""}`
	out, err := readReply(t, r, entry, source)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got := out["T_max"]; got.Value.Kind != semantics.ValReal || got.Value.Real != 341.2 || got.Unit != "K" {
		t.Errorf("T_max = %+v", got)
	}
	if got := out["code"]; got.Value.Kind != semantics.ValInt || got.Value.Int != 7 {
		t.Errorf("code = %+v", got)
	}
	if got := out["done"]; got.Value.Kind != semantics.ValBool || !got.Value.Bool {
		t.Errorf("done = %+v", got)
	}
	if got := out["note"]; got.Text != "it ran" {
		t.Errorf("note = %+v", got)
	}

	cases := map[string]struct {
		source string
		kind   runtime.ToolErrorKind
		detail string
	}{
		"empty":              {"", runtime.ToolMalformed, "wrote nothing"},
		"trailing value":     {`{"results":[]} {"x":1}`, runtime.ToolMalformed, "more than one JSON value"},
		"repeated key":       {`{"results":[],"results":[]}`, runtime.ToolMalformed, "names results twice"},
		"missing pointer":    {`{"results":[{}],"units":{}}`, runtime.ToolMissingOutput, "T_max at /results/0/T_max: nothing there"},
		"array value":        {`{"results":[{"T_max":[1,2]}],"units":{}}`, runtime.ToolMalformed, "an array is not supported yet"},
		"object value":       {`{"results":[{"T_max":{"x":1}}],"units":{}}`, runtime.ToolMalformed, "an object is not"},
		"null value":         {`{"results":[{"T_max":null}],"units":{}}`, runtime.ToolMalformed, "null is not"},
		"kind disagreement":  {`{"results":[{"T_max":"hot"}],"units":{}}`, runtime.ToolMalformed, "string where number expected"},
		"boolean for number": {`{"results":[{"T_max":true}],"units":{}}`, runtime.ToolMalformed, "boolean where number expected"},
		"missing unit":       {`{"results":[{"T_max":1,"v_out":36,"done":true,"code":7,"note":"n"}],"units":{}}`, runtime.ToolMissingOutput, "T_max at /units/T_max"},
		"empty unit":         {`{"results":[{"T_max":1,"v_out":36,"done":true,"code":7,"note":"n"}],"units":{"T_max":""}}`, runtime.ToolMalformed, "empty unit"},
		"refusal":            {`{"results":[],"units":{},"error":"did not converge"}`, runtime.ToolRefused, "did not converge"},
		"refusal not text":   {`{"results":[],"units":{},"error":42}`, runtime.ToolMalformed, "not a message"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := readReply(t, r, entry, tc.source)
			if replyKind(err) != tc.kind || !strings.Contains(err.Error(), tc.detail) {
				t.Errorf("read: %v, want kind %s saying %q", err, tc.kind, tc.detail)
			}
		})
	}
	// errorPath that does not resolve, or resolves empty, is not a refusal.
	for _, source := range []string{`{"results":[],"units":{}}`, source} {
		if _, err := readReply(t, r, entry, source); replyKind(err) == runtime.ToolRefused {
			t.Errorf("%s refused, want no refusal", source)
		}
	}
}

// The `~0` and `~1` escapes and an array index resolve as RFC 6901 spells them, and the
// whole document is the empty pointer.
func TestReplyReadsJSONPointerForms(t *testing.T) {
	r, entry := replyOf(t, &Reply{Format: ReplyJSON, Outputs: map[string]*ReplyOutput{
		"T_max": {Path: jsonPath("/a~1b/c~0d")},
		"v_out": {Path: jsonPath("/list/1")},
	}})
	out, err := readReply(t, r, entry, `{"a/b":{"c~d":4.5},"list":[10,20],"done":true,"code":9,"note":"x"}`)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if out["T_max"].Value.Real != 4.5 || out["v_out"].Value.Int != 20 {
		t.Errorf("outputs %+v", out)
	}
}

func TestReplyReadsCSV(t *testing.T) {
	r, entry := replyOf(t, &Reply{Format: ReplyCSV, ErrorColumn: &Column{Name: "err"}, Outputs: map[string]*ReplyOutput{
		"T_max": {Column: &Column{Name: "T_max"}, UnitColumn: &Column{Name: "U"}},
		"v_out": {Column: &Column{Name: "v_out"}, UnitColumn: &Column{Name: "VU"}},
		"done":  {Column: &Column{Name: "done"}, Type: TypeBoolean},
		"code":  {Column: &Column{Index: 5, ByIndex: true}, Type: TypeInteger},
		"note":  {Column: &Column{Name: "note"}, Type: TypeString},
	}})
	source := "T_max,U,v_out,VU,done,code,note,err\n340.9,K,36,km/h,true,0,first,\n341.2,K,36,km/h,FALSE,7, last ,\n"
	out, err := readReply(t, r, entry, source)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got := out["T_max"]; got.Value.Real != 341.2 || got.Unit != "K" {
		t.Errorf("T_max = %+v", got)
	}
	if got := out["done"]; !got.Value.Bool && got.Value.Kind == semantics.ValBool {
		// FALSE → false
	} else {
		t.Errorf("done = %+v, want false", got)
	}
	if got := out["note"]; got.Text != " last " {
		t.Errorf("note = %q, want it untrimmed under type string", got.Text)
	}

	// A leading byte-order mark does not become part of the first header name.
	if _, err := readReply(t, r, entry, "\xef\xbb\xbf"+source); err != nil {
		t.Errorf("read with BOM: %v", err)
	}
}

// A nil kind means the case must read clean.
func TestReplyReadsCSVFailures(t *testing.T) {
	r, entry := replyOf(t, &Reply{Format: ReplyCSV, ErrorColumn: &Column{Name: "err"}, Outputs: map[string]*ReplyOutput{
		"T_max": {Column: &Column{Name: "T_max"}},
		"code":  {Column: &Column{Index: 1, ByIndex: true}, Type: TypeInteger},
	}})
	cases := map[string]struct {
		source string
		kind   runtime.ToolErrorKind
		detail string
	}{
		"no header":     {"", runtime.ToolMalformed, "wrote no header record"},
		"no data":       {"T_max,code,err\n", runtime.ToolMissingOutput, "no data record"},
		"ragged":        {"T_max,code,err\n1,2,\n3\n", runtime.ToolMalformed, "record 3 has 1 fields, not 3"},
		"dup header":    {"T_max,T_max,err\n1,2,\n", runtime.ToolMalformed, "names T_max twice"},
		"name absent":   {"code,err\n1,\n", runtime.ToolMalformed, "column T_max is not in the header"},
		"empty cell":    {"T_max,code,err\n ,2,\n", runtime.ToolMissingOutput, "an empty cell"},
		"not a number":  {"T_max,code,err\nn/a,2,\n", runtime.ToolMalformed, "n/a is not a number"},
		"not integer":   {"T_max,code,err\n1,12.0,\n", runtime.ToolMalformed, "12.0 is not an integer"},
		"refusal":       {"T_max,code,err\n1,2,failed\n", runtime.ToolRefused, "failed"},
		"empty refusal": {"T_max,code,err\n1,2, \n", -1, ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := readReply(t, r, entry, tc.source)
			if tc.kind < 0 {
				if err != nil {
					t.Fatalf("read: %v, want no error", err)
				}
				return
			}
			if replyKind(err) != tc.kind {
				t.Fatalf("read: %v, want kind %s", err, tc.kind)
			}
			if tc.detail != "" && !strings.Contains(err.Error(), tc.detail) {
				t.Errorf("error %q, want %q", err, tc.detail)
			}
		})
	}
	// An index column past the record's width is malformed.
	r, entry = replyOf(t, &Reply{Format: ReplyCSV, Outputs: map[string]*ReplyOutput{
		"T_max": {Column: &Column{Index: 9, ByIndex: true}}}})
	if _, err := readReply(t, r, entry, "T_max,code\n1,2\n"); replyKind(err) != runtime.ToolMalformed || !strings.Contains(err.Error(), "past the 2 fields") {
		t.Errorf("index past: %v, want malformed naming the width", err)
	}
}

func TestReplyReadsCSVRowForms(t *testing.T) {
	source := "T_max\n300.0\n310.5\n341.2\n"
	for row, want := range map[*Row]float64{
		{Kind: RowFirst}:           300.0,
		{Kind: RowLast}:            341.2,
		{Kind: RowIndex, Index: 1}: 310.5,
	} {
		r, entry := replyOf(t, &Reply{Format: ReplyCSV, Outputs: map[string]*ReplyOutput{
			"T_max": {Column: &Column{Name: "T_max"}, Row: row}}})
		out, err := readReply(t, r, entry, source)
		if err != nil || out["T_max"].Value.Real != want {
			t.Errorf("row %s = %v, %v; want %f", row, out["T_max"], err, want)
		}
	}
	r, entry := replyOf(t, &Reply{Format: ReplyCSV, Outputs: map[string]*ReplyOutput{
		"T_max": {Column: &Column{Name: "T_max"}, Row: &Row{Kind: RowIndex, Index: 7}}}})
	if _, err := readReply(t, r, entry, source); replyKind(err) != runtime.ToolMalformed || !strings.Contains(err.Error(), "row 7: only 3 rows") {
		t.Errorf("row 7: %v, want malformed naming the row and the count", err)
	}
}

// Without a header the columns are zero-based indices; a tab delimiter is one rune.
func TestReplyReadsCSVHeaderlessAndTab(t *testing.T) {
	headerFalse := false
	r, entry := replyOf(t, &Reply{Format: ReplyCSV, Header: &headerFalse, Delimiter: "\t",
		Outputs: map[string]*ReplyOutput{"T_max": {Column: &Column{Index: 1, ByIndex: true}}}})
	out, err := readReply(t, r, entry, "341\t341.2\n")
	if err != nil || out["T_max"].Value.Real != 341.2 {
		t.Fatalf("read: %v, %+v", err, out["T_max"])
	}
}

func TestReplyReadsLines(t *testing.T) {
	r, entry := replyOf(t, &Reply{Format: ReplyLines, ErrorKey: "error", Outputs: map[string]*ReplyOutput{
		"T_max": {},
		"done":  {Key: "finished", Type: TypeBoolean},
		"note":  {Type: TypeString},
	}})
	source := "starting solve\nT_max = 341.2\nfinished: TRUE\nnote = it ran: twice = is fine\nerror:\nall done\n"
	out, err := readReply(t, r, entry, source)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if out["T_max"].Value.Real != 341.2 || !out["done"].Value.Bool || out["note"].Text != "it ran: twice = is fine" {
		t.Errorf("outputs %+v", out)
	}

	cases := map[string]struct {
		source string
		kind   runtime.ToolErrorKind
		detail string
	}{
		"missing key":   {"done = true\n", runtime.ToolMissingOutput, "T_max under key T_max: no line"},
		"key twice":     {"T_max = 1\nT_max = 2\n", runtime.ToolMalformed, "lines 1 and 2"},
		"not a number":  {"T_max = n/a\n", runtime.ToolMalformed, "line 1"},
		"refusal":       {"error = did not converge\nT_max = 1\n", runtime.ToolRefused, "did not converge"},
		"empty refusal": {"error: \n", runtime.ToolMissingOutput, "T_max"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := readReply(t, r, entry, tc.source)
			if replyKind(err) != tc.kind || !strings.Contains(err.Error(), tc.detail) {
				t.Errorf("read: %v, want kind %s saying %q", err, tc.kind, tc.detail)
			}
		})
	}
}

func TestReplyReadsLinesRegex(t *testing.T) {
	r, entry := replyOf(t, &Reply{Format: ReplyLines, Regex: `T=(?P<T_max>[0-9.]+)`, Outputs: map[string]*ReplyOutput{
		"T_max": {Type: TypeReal}}})
	out, err := readReply(t, r, entry, "chatter\nT=341.2K and more\n")
	if err != nil || out["T_max"].Value.Real != 341.2 {
		t.Fatalf("read: %v, %+v", err, out["T_max"])
	}
	if _, err := readReply(t, r, entry, "T=1\nT=2\n"); replyKind(err) != runtime.ToolMalformed || !strings.Contains(err.Error(), "lines 1 and 2") {
		t.Errorf("two matches: %v, want malformed", err)
	}
	if _, err := readReply(t, r, entry, "nothing\n"); replyKind(err) != runtime.ToolMissingOutput {
		t.Errorf("no match: %v, want missing", err)
	}
}

func TestReplyReadsExitCode(t *testing.T) {
	r, _ := replyOf(t, &Reply{Format: ReplyExitCode, Success: []int{0, 3}, Outputs: map[string]*ReplyOutput{
		"done": {Type: TypeBoolean}}})
	for code, want := range map[int]bool{0: true, 3: true, 1: false} {
		out := r.readExitCode(&execution{exit: code})
		if out["done"].Value.Bool != want {
			t.Errorf("exit %d = %+v; want %v", code, out["done"], want)
		}
	}
	r, _ = replyOf(t, &Reply{Format: ReplyExitCode, Outputs: map[string]*ReplyOutput{
		"code": {Type: TypeInteger}}})
	out := r.readExitCode(&execution{exit: 3})
	if out["code"].Value.Int != 3 {
		t.Errorf("exit 3 as integer = %+v", out["code"])
	}
}

// typeText reads each ValueType; a non-finite number is malformed everywhere.
func TestTypeText(t *testing.T) {
	cases := map[string]struct {
		text  string
		typ   ValueType
		check func(runtime.ToolValue) bool
	}{
		"integer as number":  {"12", TypeNumber, func(v runtime.ToolValue) bool { return v.Value.Kind == semantics.ValInt && v.Value.Int == 12 }},
		"real as number":     {"1.5e2", TypeNumber, func(v runtime.ToolValue) bool { return v.Value.Kind == semantics.ValReal && v.Value.Real == 150 }},
		"integer":            {"12", TypeInteger, func(v runtime.ToolValue) bool { return v.Value.Kind == semantics.ValInt }},
		"real of an integer": {"12", TypeReal, func(v runtime.ToolValue) bool { return v.Value.Kind == semantics.ValReal && v.Value.Real == 12 }},
		"boolean":            {"TRUE", TypeBoolean, func(v runtime.ToolValue) bool { return v.Value.Bool }},
		"string":             {" a b ", TypeString, func(v runtime.ToolValue) bool { return v.Text == " a b " }},
	}
	for name, tc := range cases {
		v, err := typeText(tc.text, tc.typ)
		if err != nil || !tc.check(v) {
			t.Errorf("%s: %v, %+v", name, err, v)
		}
	}
	for _, tc := range []struct {
		text string
		typ  ValueType
	}{
		{"12.0", TypeInteger}, {"inf", TypeReal}, {"nan", TypeNumber}, {"yes", TypeBoolean}, {"x", TypeNumber},
	} {
		if _, err := typeText(tc.text, tc.typ); err == nil {
			t.Errorf("%s as %s read, want malformed", tc.text, tc.typ)
		}
	}
	if _, err := typeText("1e400", TypeNumber); err == nil {
		t.Error("1e400 read as a number, want a non-finite refusal")
	}
}

// With two outputs offending, the load refusal is the earlier variable's, always.
func TestManifestReplyRefusalsAreDeterministic(t *testing.T) {
	dir := writeManifest(t, map[string]string{"thermal.json": replyEntryText(
		`{"format": "json", "outputs": {"a": {"path": "x/y"}, "b": {"path": "x/y"}}}`)})
	_, err := LoadManifest(dir)
	if err == nil || !strings.Contains(err.Error(), "reply.outputs.a.path") {
		t.Fatalf("load: %v, want the refusal for a, the earlier variable", err)
	}
}

// A `file:` source reads only a regular file within OPENSYSML_TOOL_MAX_OUTPUT.
func TestReplySourceRefusals(t *testing.T) {
	source, err := parseTemplate("{outputDir}/result.json")
	if err != nil {
		t.Fatalf("parseTemplate: %v", err)
	}
	dir := t.TempDir()
	entry := ToolEntry{ToolName: "Thermal", Reply: &Reply{compiled: &compiledReply{source: source}}}
	e := toolEngine{entry: entry}
	process := &composed{tempDir: dir}
	path := filepath.Join(dir, "result.json")

	t.Setenv(OutputLimitEnv, "8")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", 64)), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err = e.replySource(&execution{}, process)
	if replyKind(err) != runtime.ToolMalformed ||
		!strings.Contains(err.Error(), "wrote more than 8 bytes to result.json ("+OutputLimitEnv+")") {
		t.Errorf("over the limit: %v", err)
	}

	if err := os.Remove(path); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	_, err = e.replySource(&execution{}, process)
	if replyKind(err) != runtime.ToolMalformed || !strings.Contains(err.Error(), "result.json is not a regular file") {
		t.Errorf("a directory: %v", err)
	}

	// A symlinked ancestor under {outputDir} may not lead out of it.
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "result.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("write outside: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "link")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	linked, err := parseTemplate("{outputDir}/link/result.json")
	if err != nil {
		t.Fatalf("parseTemplate: %v", err)
	}
	linkedEngine := toolEngine{entry: ToolEntry{ToolName: "Thermal", Reply: &Reply{compiled: &compiledReply{source: linked}}}}
	_, err = linkedEngine.replySource(&execution{}, process)
	if replyKind(err) != runtime.ToolMalformed || !strings.Contains(err.Error(), "escapes {outputDir}") {
		t.Errorf("a symlinked ancestor: %v, want escapes {outputDir}", err)
	}

	// A file the engine cannot open is the same malformed fault, not a panic.
	if os.Geteuid() != 0 {
		if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
			t.Fatalf("rewrite: %v", err)
		}
		if err := os.Chmod(path, 0o000); err != nil {
			t.Fatalf("chmod: %v", err)
		}
		_, err = e.replySource(&execution{}, process)
		var fault *runtime.ToolError
		if !errors.As(err, &fault) || fault.Kind != runtime.ToolMalformed ||
			!strings.HasPrefix(fault.Detail, "cannot read result.json") {
			t.Errorf("an unreadable file: %v, want cannot read result.json", err)
		}
	}
}

// Only the outputs the call requests are read; another mapped output may be absent.
func TestReplyReadsRequestedOutputs(t *testing.T) {
	r, entry := replyOf(t, &Reply{Format: ReplyJSON, Outputs: map[string]*ReplyOutput{
		"T_max": {Path: jsonPath("/T_max")},
		"v_out": {Path: jsonPath("/v_out")},
	}})
	out, err := readReply(t, r, entry, `{"T_max": 341.2}`, "T_max")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(out) != 1 || out["T_max"].Value.Real != 341.2 {
		t.Errorf("outputs = %+v, want only T_max", out)
	}
	// Requesting an output that has no manifest selector reads what is mapped, nothing more.
	out, err = readReply(t, r, entry, `{"T_max": 341.2, "v_out": 36}`, "T_max", "done")
	if err != nil || len(out) != 1 {
		t.Errorf("an unmapped request: %v, %+v", err, out)
	}
}

// A fault one unrequested output earns is recorded, not raised; the same fault on a
// requested output fails the read with it.
func TestReplyFaultsApplyOnlyToRequestedOutputs(t *testing.T) {
	source := `{"T_max": 341.2}`
	r, entry := replyOf(t, &Reply{Format: ReplyJSON, Outputs: map[string]*ReplyOutput{
		"T_max": {Path: jsonPath("/T_max")},
		"v_out": {Path: jsonPath("/v_out")},
	}})
	if _, err := readReply(t, r, entry, source, "T_max"); err != nil {
		t.Errorf("the unrequested missing output: %v, want no fault", err)
	}
	if _, err := readReply(t, r, entry, source, "T_max", "v_out"); replyKind(err) != runtime.ToolMissingOutput ||
		!strings.Contains(err.Error(), "v_out") {
		t.Errorf("the requested missing output: %v, want missing", err)
	}
}

// A programmatic selector below zero is the same malformed fault as one too large, not
// a panic.
func TestReplyNegativeSelectors(t *testing.T) {
	source := "T_max,code\n1,2\n"
	r, entry := replyOf(t, &Reply{Format: ReplyCSV, Outputs: map[string]*ReplyOutput{
		"T_max": {Column: &Column{Index: -1, ByIndex: true}},
	}})
	if _, err := readReply(t, r, entry, source); replyKind(err) != runtime.ToolMalformed ||
		!strings.Contains(err.Error(), "past the 2 fields") {
		t.Errorf("negative column: %v", err)
	}
	r, entry = replyOf(t, &Reply{Format: ReplyCSV, Outputs: map[string]*ReplyOutput{
		"T_max": {Column: &Column{Name: "T_max"}, Row: &Row{Kind: RowIndex, Index: -1}},
	}})
	if _, err := readReply(t, r, entry, source); replyKind(err) != runtime.ToolMalformed ||
		!strings.Contains(err.Error(), "only 1 rows") {
		t.Errorf("negative row: %v", err)
	}
}

// The empty pointer selects the whole document, so a bare scalar is the output.
func TestReplyReadsJSONRootPointer(t *testing.T) {
	r, entry := replyOf(t, &Reply{Format: ReplyJSON, Outputs: map[string]*ReplyOutput{
		"T_max": {Path: jsonPath("")},
	}})
	out, err := readReply(t, r, entry, `341.2`)
	if err != nil || out["T_max"].Value.Real != 341.2 {
		t.Fatalf("root pointer: %v, %+v; want 341.2", err, out)
	}
}

// A row marshals back to the form it is read: a name, or the index as a JSON integer.
func TestReplyRowRoundTrip(t *testing.T) {
	for text, want := range map[string]Row{`"first"`: {Kind: RowFirst}, `"last"`: {Kind: RowLast}, `3`: {Kind: RowIndex, Index: 3}} {
		data, err := want.MarshalJSON()
		if err != nil || string(data) != text {
			t.Fatalf("%+v marshals %s, %v; want %s", want, data, err, text)
		}
		var got Row
		if err := json.Unmarshal(data, &got); err != nil || got != want {
			t.Errorf("%s round-trips %+v, %v; want %+v", text, got, err, want)
		}
	}
}

// nullMember walks the whole document, nested objects and arrays included, naming the
// first member set to null as a struct decode would hide it.
func TestNullMember(t *testing.T) {
	doc := `{"a": {"b": [{"c": 1}, {"d": null}]}, "e": null}`
	if path, found := nullMember([]byte(doc)); !found || path != "a.b.d" {
		t.Errorf("nullMember = %q, %v; want a.b.d", path, found)
	}
	if path, found := nullMember([]byte(`{"a": {"b": [null]}, "c": [{"d": 0}]}`)); found {
		t.Errorf("array elements are not members: %q", path)
	}
	if path, found := nullMember([]byte(`{"a": 1, "b": {"c": "x"}}`)); found {
		t.Errorf("no nulls: %q", path)
	}
	if _, found := nullMember([]byte(`{"a":`)); found {
		t.Error("malformed input reports no null member")
	}
}
