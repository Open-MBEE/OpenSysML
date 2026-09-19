package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// These tests drive textDocument/didChange with the framed JSON an editor sends,
// where an omitted range (full replace) and a {0,0}-{0,0} range (insert) differ.

const wireURI = "file:///tmp/wire.sysml"

const wireSource = "package Vehicles {\n    part def Wheel;\n    part w : Wheel;\n}\n"

// wireSession is a served Server reached only through its framed stream;
// a reader goroutine hands messages to the test, which picks them by id or method.
type wireSession struct {
	t       *testing.T
	s       *Server
	client  net.Conn
	msgs    chan map[string]any
	readErr chan error
	pending []map[string]any
	nextID  int
}

// wireMessage is one JSON-RPC message whose params are written verbatim.
type wireMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int            `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

const wireTimeout = 30 * time.Second

func newWireSession(t *testing.T) *wireSession {
	t.Helper()
	client, server := net.Pipe()
	s := NewServer(model.NewWorkspace())
	done := make(chan error, 1)
	go func() { done <- s.Run(context.Background(), server) }()
	if err := client.SetDeadline(time.Now().Add(wireTimeout)); err != nil {
		t.Fatalf("SetDeadline: %v", err)
	}
	w := &wireSession{
		t:       t,
		s:       s,
		client:  client,
		msgs:    make(chan map[string]any, 256),
		readErr: make(chan error, 1),
	}
	// net.Pipe is unbuffered: the server's notifications must be drained while
	// the test goroutine writes, or the two sides deadlock.
	go func() {
		r := bufio.NewReader(client)
		for {
			msg, err := readFramedMessage(r)
			if err != nil {
				w.readErr <- err
				return
			}
			w.msgs <- msg
		}
	}()
	t.Cleanup(func() {
		_ = client.Close()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("Run did not return after the stream closed")
		}
	})
	return w
}

// notify sends a notification whose params are the given JSON text.
func (w *wireSession) notify(method, params string) {
	w.t.Helper()
	msg := wireMessage{JSONRPC: "2.0", Method: method, Params: json.RawMessage(params)}
	if err := writeMessage(w.client, msg); err != nil {
		w.t.Fatalf("write %s: %v", method, err)
	}
}

// call sends a request with the given JSON params and returns its result.
func (w *wireSession) call(method, params string) any {
	w.t.Helper()
	w.nextID++
	id := w.nextID
	msg := wireMessage{JSONRPC: "2.0", ID: &id, Method: method, Params: json.RawMessage(params)}
	if err := writeMessage(w.client, msg); err != nil {
		w.t.Fatalf("write %s: %v", method, err)
	}
	reply := w.next(fmt.Sprintf("reply to %s (id %d)", method, id), func(m map[string]any) bool {
		got, ok := m["id"].(float64)
		return ok && int(got) == id
	})
	if failure := reply["error"]; failure != nil {
		w.t.Fatalf("%s answered with error %v", method, failure)
	}
	return reply["result"]
}

// awaitDiagnostics returns the next publishDiagnostics payload for uri.
func (w *wireSession) awaitDiagnostics(uri string) []any {
	w.t.Helper()
	msg := w.next("publishDiagnostics for "+uri, func(m map[string]any) bool {
		if m["method"] != "textDocument/publishDiagnostics" {
			return false
		}
		params, _ := m["params"].(map[string]any)
		return params["uri"] == uri
	})
	params, _ := msg["params"].(map[string]any)
	diags, ok := params["diagnostics"].([]any)
	if !ok {
		w.t.Fatalf("publishDiagnostics params = %v, want a diagnostics array", params)
	}
	return diags
}

// next returns the first message matching want; unmatched ones are kept for
// later pickers, since replies and notifications interleave.
func (w *wireSession) next(what string, want func(map[string]any) bool) map[string]any {
	w.t.Helper()
	for i, m := range w.pending {
		if want(m) {
			w.pending = append(w.pending[:i], w.pending[i+1:]...)
			return m
		}
	}
	deadline := time.After(wireTimeout)
	for {
		select {
		case m := <-w.msgs:
			if want(m) {
				return m
			}
			w.pending = append(w.pending, m)
		case err := <-w.readErr:
			w.t.Fatalf("stream ended while waiting for %s: %v", what, err)
		case <-deadline:
			w.t.Fatalf("no %s within %v", what, wireTimeout)
		}
	}
}

// open initializes the session and opens wireURI with text, consuming its diagnostics.
func (w *wireSession) open(text string) {
	w.t.Helper()
	w.call("initialize", `{"capabilities":{}}`)
	w.notify("initialized", `{}`)
	w.notify("textDocument/didOpen", fmt.Sprintf(
		`{"textDocument":{"uri":%q,"languageId":"sysml","version":1,"text":%s}}`, wireURI, jsonString(text)))
	if diags := w.awaitDiagnostics(wireURI); len(diags) != 0 {
		w.t.Fatalf("didOpen published %v, want a clean document to start from", diags)
	}
}

// didChange sends the given JSON change objects verbatim for wireURI and
// returns the diagnostics published for the result.
func (w *wireSession) didChange(version int, changes ...string) []any {
	w.t.Helper()
	w.notify("textDocument/didChange", fmt.Sprintf(
		`{"textDocument":{"uri":%q,"version":%d},"contentChanges":[%s]}`,
		wireURI, version, strings.Join(changes, ",")))
	return w.awaitDiagnostics(wireURI)
}

// content is the text the workspace holds for wireURI.
func (w *wireSession) content() string {
	w.t.Helper()
	doc := w.s.ws.Document(uriToName(wireURI))
	if doc == nil {
		w.t.Fatalf("%s is not in the workspace", wireURI)
	}
	return string(doc.Content)
}

// symbolNames returns the top-level documentSymbol names of wireURI.
func (w *wireSession) symbolNames() []string {
	w.t.Helper()
	result := w.call("textDocument/documentSymbol", fmt.Sprintf(`{"textDocument":{"uri":%q}}`, wireURI))
	symbols, ok := result.([]any)
	if !ok {
		w.t.Fatalf("documentSymbol result = %v, want an array", result)
	}
	names := make([]string, 0, len(symbols))
	for _, sym := range symbols {
		fields, _ := sym.(map[string]any)
		name, _ := fields["name"].(string)
		names = append(names, name)
	}
	return names
}

// hoverText returns the hover contents at the given position, or "".
func (w *wireSession) hoverText(line, character int) string {
	w.t.Helper()
	result := w.call("textDocument/hover", fmt.Sprintf(
		`{"textDocument":{"uri":%q},"position":{"line":%d,"character":%d}}`, wireURI, line, character))
	if result == nil {
		return ""
	}
	hover, _ := result.(map[string]any)
	contents, _ := hover["contents"].(map[string]any)
	value, _ := contents["value"].(string)
	return value
}

// diagnosticSources lists the source of each published diagnostic.
func diagnosticSources(diags []any) []string {
	out := make([]string, 0, len(diags))
	for _, d := range diags {
		fields, _ := d.(map[string]any)
		source, _ := fields["source"].(string)
		out = append(out, source)
	}
	return out
}

func jsonString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// A change without "range" replaces the whole document.
func TestWireDidChangeOmittedRangeReplacesDocument(t *testing.T) {
	w := newWireSession(t)
	w.open(wireSource)
	if got := w.symbolNames(); !slices.Equal(got, []string{"Vehicles"}) {
		t.Fatalf("symbols before the change = %v, want [Vehicles]", got)
	}

	const replacement = "package Boats {\n    part def Hull;\n}\n"
	if diags := w.didChange(2, `{"text":`+jsonString(replacement)+`}`); len(diags) != 0 {
		t.Fatalf("replacement published %v, want no diagnostics", diags)
	}
	if got := w.content(); got != replacement {
		t.Errorf("content = %q, want %q", got, replacement)
	}
	if got := w.symbolNames(); !slices.Equal(got, []string{"Boats"}) {
		t.Errorf("symbols after the change = %v, want [Boats]", got)
	}
	// "Hull" sits on line 1 at column 13 of the replacement; "Wheel" used to.
	if hover := w.hoverText(1, 13); !strings.Contains(hover, "Hull") || strings.Contains(hover, "Wheel") {
		t.Errorf("hover on the new text = %q, want Hull and not Wheel", hover)
	}
}

// A {0,0}-{0,0} range with non-empty text inserts at the start; the original follows.
func TestWireDidChangeZeroRangeInsertsAtStart(t *testing.T) {
	w := newWireSession(t)
	w.open(wireSource)

	if diags := w.didChange(2,
		`{"range":{"start":{"line":0,"character":0},"end":{"line":0,"character":0}},"text":"package Prefix;\n"}`,
	); len(diags) != 0 {
		t.Fatalf("insertion published %v, want no diagnostics", diags)
	}
	if got, want := w.content(), "package Prefix;\n"+wireSource; got != want {
		t.Errorf("content = %q, want %q", got, want)
	}
	if got := w.symbolNames(); !slices.Equal(got, []string{"Prefix", "Vehicles"}) {
		t.Errorf("symbols = %v, want [Prefix Vehicles]", got)
	}
	// "Wheel" moved down one line with the insertion, to line 2 column 13.
	if hover := w.hoverText(2, 13); !strings.Contains(hover, "Wheel") {
		t.Errorf("hover on the shifted original text = %q, want Wheel", hover)
	}
}

// LSP declares range optional, never null; a null is read as no range (full replacement).
func TestWireDidChangeNullRangeReplacesDocument(t *testing.T) {
	w := newWireSession(t)
	w.open(wireSource)

	const replacement = "package Boats {\n    part def Hull;\n}\n"
	if diags := w.didChange(2, `{"range":null,"text":`+jsonString(replacement)+`}`); len(diags) != 0 {
		t.Fatalf("replacement published %v, want no diagnostics", diags)
	}
	if got := w.content(); got != replacement {
		t.Errorf("content = %q, want %q", got, replacement)
	}
	if got := w.symbolNames(); !slices.Equal(got, []string{"Boats"}) {
		t.Errorf("symbols = %v, want [Boats]", got)
	}
}

// The changes of one notification apply in order against the accumulating text.
func TestWireDidChangeMixedBatchAppliesInOrder(t *testing.T) {
	w := newWireSession(t)
	w.open(wireSource)

	diags := w.didChange(2,
		// Wheel -> Tire on line 1, then the reference on line 2.
		`{"range":{"start":{"line":1,"character":13},"end":{"line":1,"character":18}},"text":"Tire"}`,
		`{"range":{"start":{"line":2,"character":13},"end":{"line":2,"character":18}},"text":"Tire"}`,
		`{"text":"package Final {\n}\n"}`,
		`{"range":{"start":{"line":1,"character":0},"end":{"line":1,"character":0}},"text":"    part def Added;\n"}`,
	)
	if len(diags) != 0 {
		t.Fatalf("batch published %v, want no diagnostics", diags)
	}
	const want = "package Final {\n    part def Added;\n}\n"
	if got := w.content(); got != want {
		t.Errorf("content = %q, want %q", got, want)
	}
	result := w.call("textDocument/documentSymbol", fmt.Sprintf(`{"textDocument":{"uri":%q}}`, wireURI))
	symbols, _ := result.([]any)
	if len(symbols) != 1 {
		t.Fatalf("documentSymbol = %v, want the one package Final", result)
	}
	final, _ := symbols[0].(map[string]any)
	children, _ := final["children"].([]any)
	if final["name"] != "Final" || len(children) != 1 {
		t.Fatalf("documentSymbol = %v, want Final with one child", result)
	}
	if child, _ := children[0].(map[string]any); child["name"] != "Added" {
		t.Errorf("child of Final = %v, want Added", child)
	}
}

// Both the full-replacement and incremental paths republish diagnostics:
// breaking the document reports an error, mending it withdraws the error.
func TestWireDidChangeRepublishesDiagnosticsOnBothPaths(t *testing.T) {
	t.Run("full replacement", func(t *testing.T) {
		w := newWireSession(t)
		w.open(wireSource)

		diags := w.didChange(2, `{"text":"package Broken {\n"}`)
		if sources := diagnosticSources(diags); len(diags) == 0 || sources[0] != "syntax" {
			t.Fatalf("malformed replacement published %v, want a syntax diagnostic", diags)
		}
		if diags := w.didChange(3, `{"text":"package Mended {\n}\n"}`); len(diags) != 0 {
			t.Fatalf("valid replacement published %v, want the diagnostics withdrawn", diags)
		}
		if got := w.symbolNames(); !slices.Equal(got, []string{"Mended"}) {
			t.Errorf("symbols = %v, want [Mended]", got)
		}
	})

	t.Run("incremental", func(t *testing.T) {
		w := newWireSession(t)
		w.open(wireSource)

		// Delete the closing brace on line 3.
		diags := w.didChange(2, `{"range":{"start":{"line":3,"character":0},"end":{"line":3,"character":1}},"text":""}`)
		if sources := diagnosticSources(diags); len(diags) == 0 || sources[0] != "syntax" {
			t.Fatalf("deleting the brace published %v, want a syntax diagnostic", diags)
		}
		if got, want := w.content(), strings.TrimSuffix(wireSource, "}\n")+"\n"; got != want {
			t.Fatalf("content = %q, want %q", got, want)
		}
		// Put it back.
		if diags := w.didChange(3, `{"range":{"start":{"line":3,"character":0},"end":{"line":3,"character":0}},"text":"}"}`); len(diags) != 0 {
			t.Fatalf("restoring the brace published %v, want the diagnostics withdrawn", diags)
		}
		if got := w.content(); got != wireSource {
			t.Errorf("content = %q, want the original %q", got, wireSource)
		}
	})
}

// textDocument/rangeFormatting is dispatched over the wire and answers only
// the edits on the requested lines.
func TestWireRangeFormattingAnswersEditsOnSelectedLines(t *testing.T) {
	w := newWireSession(t)
	w.open("package Vehicles {\n  part def Wheel;\n  part w : Wheel;\n}\n")

	request := func(startLine, endLine int) []any {
		result := w.call("textDocument/rangeFormatting", fmt.Sprintf(
			`{"textDocument":{"uri":%q},"range":{"start":{"line":%d,"character":0},"end":{"line":%d,"character":0}},"options":{"tabSize":4,"insertSpaces":true}}`,
			wireURI, startLine, endLine))
		if result == nil {
			return nil
		}
		edits, ok := result.([]any)
		if !ok {
			t.Fatalf("rangeFormatting result = %v, want an array", result)
		}
		return edits
	}

	edits := request(2, 3)
	if len(edits) != 1 {
		t.Fatalf("edits for line 2 = %v, want exactly one", edits)
	}
	want := map[string]any{
		"range":   map[string]any{"start": map[string]any{"line": 2.0, "character": 2.0}, "end": map[string]any{"line": 2.0, "character": 2.0}},
		"newText": "  ",
	}
	if got := edits[0]; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("edit = %v, want %v", got, want)
	}

	if edits := request(0, 1); len(edits) != 0 {
		t.Errorf("edits for the well-formatted first line = %v, want none", edits)
	}
	if edits := request(0, 4); len(edits) != 2 {
		t.Errorf("edits for the whole file = %v, want both indentation fixes", edits)
	}
}
