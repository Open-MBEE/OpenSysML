package lsp

import (
	"context"
	"encoding/json"
	"testing"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

func TestDidOpenRegistersDocument(t *testing.T) {
	s := NewServer(model.NewWorkspace())
	u := uri.File("/tmp/a.sysml")
	err := s.DidOpen(context.Background(), &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:     u,
			Version: 1,
			Text:    "package P;\n",
		},
	})
	if err != nil {
		t.Fatalf("DidOpen error: %v", err)
	}
	doc := s.ws.Document(u.Filename())
	if doc == nil {
		t.Fatal("document not registered")
	}
	if string(doc.Content) != "package P;\n" {
		t.Errorf("content = %q", doc.Content)
	}
}

// sendDidChange drives the raw changeHandler exactly as the transport would:
// it builds a textDocument/didChange notification whose JSON matches what a
// client sends, so an omitted "range" stays omitted (full replace) and a
// present "range" is decoded as a pointer (incremental).
func sendDidChange(t *testing.T, s *Server, u uri.URI, version int32, rawChanges []json.RawMessage) {
	t.Helper()
	params := map[string]any{
		"textDocument": map[string]any{"uri": u, "version": version},
		"contentChanges": func() []json.RawMessage {
			return rawChanges
		}(),
	}
	req, err := jsonrpc2.NewNotification(protocol.MethodTextDocumentDidChange, params)
	if err != nil {
		t.Fatalf("NewNotification: %v", err)
	}
	handler := s.changeHandler(protocol.ServerHandler(s, jsonrpc2.MethodNotFoundHandler))
	reply := func(ctx context.Context, result any, err error) error { return err }
	if err := handler(context.Background(), reply, req); err != nil {
		t.Fatalf("changeHandler: %v", err)
	}
}

func openDoc(t *testing.T, s *Server, u uri.URI, text string) {
	t.Helper()
	if err := s.DidOpen(context.Background(), &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: u, Version: 1, Text: text},
	}); err != nil {
		t.Fatalf("DidOpen error: %v", err)
	}
}

func TestDidChangeFullReplace(t *testing.T) {
	s := NewServer(model.NewWorkspace())
	u := uri.File("/tmp/a.sysml")
	openDoc(t, s, u, "package P;\n")
	// A full-document replace omits "range" entirely.
	sendDidChange(t, s, u, 2, []json.RawMessage{
		json.RawMessage(`{"text":"package Q;\n"}`),
	})
	if got := string(s.ws.Document(u.Filename()).Content); got != "package Q;\n" {
		t.Errorf("content = %q, want %q", got, "package Q;\n")
	}
}

func TestDidChangeIncremental(t *testing.T) {
	s := NewServer(model.NewWorkspace())
	u := uri.File("/tmp/a.sysml")
	openDoc(t, s, u, "package P;\n")
	// Replace the single character 'P' (line 0, chars 8..9) with "Hello".
	sendDidChange(t, s, u, 2, []json.RawMessage{
		json.RawMessage(`{"range":{"start":{"line":0,"character":8},"end":{"line":0,"character":9}},"text":"Hello"}`),
	})
	if got := string(s.ws.Document(u.Filename()).Content); got != "package Hello;\n" {
		t.Errorf("content = %q, want %q", got, "package Hello;\n")
	}
}

// Regression: an incremental insertion at {0,0} must NOT be mistaken for a full
// replace. The range is present (start == end == {0,0}), so it splices at the
// front instead of wiping the buffer.
func TestDidChangeIncrementalInsertAtStart(t *testing.T) {
	s := NewServer(model.NewWorkspace())
	u := uri.File("/tmp/a.sysml")
	openDoc(t, s, u, "package P;\n")
	sendDidChange(t, s, u, 2, []json.RawMessage{
		json.RawMessage(`{"range":{"start":{"line":0,"character":0},"end":{"line":0,"character":0}},"text":"X"}`),
	})
	if got := string(s.ws.Document(u.Filename()).Content); got != "Xpackage P;\n" {
		t.Errorf("content = %q, want %q", got, "Xpackage P;\n")
	}
}

// Regression: a multi-change notification whose first edit is an insert at the
// start must apply all edits sequentially against the accumulating buffer.
func TestDidChangeMultiChangeStartingAtOffsetZero(t *testing.T) {
	s := NewServer(model.NewWorkspace())
	u := uri.File("/tmp/a.sysml")
	openDoc(t, s, u, "abc\n")
	sendDidChange(t, s, u, 2, []json.RawMessage{
		json.RawMessage(`{"range":{"start":{"line":0,"character":0},"end":{"line":0,"character":0}},"text":"X"}`),
		json.RawMessage(`{"range":{"start":{"line":0,"character":4},"end":{"line":0,"character":4}},"text":"Y"}`),
	})
	if got := string(s.ws.Document(u.Filename()).Content); got != "XabcY\n" {
		t.Errorf("content = %q, want %q", got, "XabcY\n")
	}
}

// Regression: an incremental edit after an astral (surrogate-pair) rune uses
// UTF-16 columns; the byte offset must skip the 4-byte emoji correctly.
func TestDidChangeIncrementalAstral(t *testing.T) {
	s := NewServer(model.NewWorkspace())
	u := uri.File("/tmp/a.sysml")
	openDoc(t, s, u, "😀ab\n") // emoji = 2 UTF-16 units; 'a' at char 2, 'b' at char 3
	// Replace 'a' (chars 2..3) with "Z".
	sendDidChange(t, s, u, 2, []json.RawMessage{
		json.RawMessage(`{"range":{"start":{"line":0,"character":2},"end":{"line":0,"character":3}},"text":"Z"}`),
	})
	if got := string(s.ws.Document(u.Filename()).Content); got != "😀Zb\n" {
		t.Errorf("content = %q, want %q", got, "😀Zb\n")
	}
}

func TestDidCloseRemovesOpenState(t *testing.T) {
	s := NewServer(model.NewWorkspace())
	u := uri.File("/tmp/a.sysml")
	openDoc(t, s, u, "package P;\n")
	if err := s.DidClose(context.Background(), &protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: u},
	}); err != nil {
		t.Fatalf("DidClose error: %v", err)
	}
	// With no on-disk backing, closing removes the document entirely.
	if doc := s.ws.Document(u.Filename()); doc != nil {
		t.Errorf("document still present after close: %q", doc.Content)
	}
}
