package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

// DidOpen registers a newly opened document with the workspace. The buffer the
// editor sends can differ from what was read from disk, so when the open moved
// the workspace the other open documents are refreshed too; a buffer holding
// what the folder scan indexed changes nothing they report. A document under
// none of the session's folders has its directory indexed, so its imports of
// sibling files resolve. A library document is served from the bundled text the
// index holds, so opening one changes nothing.
func (s *Server) DidOpen(ctx context.Context, params *protocol.DidOpenTextDocumentParams) error {
	if isLibraryURI(params.TextDocument.URI) {
		return nil
	}
	name := uriToName(params.TextDocument.URI)
	before := s.ws.Generation()
	s.debugEdit(ctx, "", func() {
		s.ws.Open(name, []byte(params.TextDocument.Text), int(params.TextDocument.Version))
		s.indexOpenedDirectory(name)
	})
	s.publishDiagnostics(ctx, name)
	if s.ws.Generation() != before {
		s.queueOpenDiagnostics(ctx, name)
	}
	return nil
}

// DidChange applies content changes (full-replace or incremental) then updates
// the workspace document, publishing the edited document at once and the other
// open ones once typing settles: the edit changes what they resolve.
func (s *Server) DidChange(ctx context.Context, params *protocol.DidChangeTextDocumentParams) error {
	if isLibraryURI(params.TextDocument.URI) {
		return s.refuseLibraryChange(ctx, params.TextDocument.URI)
	}
	name := uriToName(params.TextDocument.URI)
	doc := s.ws.Document(name)
	var content []byte
	if doc != nil {
		content = append([]byte(nil), doc.Content...)
	}
	for _, ch := range params.ContentChanges {
		content = applyContentChange(content, ch)
	}
	s.debugEdit(ctx, "", func() { s.ws.Update(name, content, int(params.TextDocument.Version)) })
	s.publishDiagnostics(ctx, name)
	s.queueOpenDiagnostics(ctx, name)
	return nil
}

// applyContentChange applies a single change: a whole-document replace, or an
// incremental splice at its range. RangeLength is intentionally unused for the
// splice: it is a UTF-16 code-unit count (not a byte count), and the
// authoritative delete extent is Range.
func applyContentChange(content []byte, ch protocol.TextDocumentContentChangeEvent) []byte {
	var rng protocol.Range
	var text string
	switch ch := ch.(type) {
	case *protocol.TextDocumentContentChangeWholeDocument:
		return []byte(ch.Text)
	case *protocol.TextDocumentContentChangePartial:
		rng, text = ch.Range, ch.Text
	default:
		return content
	}
	sp := rangeToSpan(content, rng)
	start := min(sp.Offset, len(content))
	end := min(sp.End(), len(content))
	out := make([]byte, 0, start+len(text)+(len(content)-end))
	out = append(out, content[:start]...)
	out = append(out, text...)
	out = append(out, content[end:]...)
	return out
}

// DidClose marks the document closed, re-reading its on-disk content first so
// closing a tab does not unindex names other documents resolve through it. Its
// markers are withdrawn: only an open document has a set that keeps pace with
// the workspace.
func (s *Server) DidClose(ctx context.Context, params *protocol.DidCloseTextDocumentParams) error {
	if isLibraryURI(params.TextDocument.URI) {
		return nil
	}
	name := uriToName(params.TextDocument.URI)
	s.debugEdit(ctx, name, func() {
		s.loadFromDisk(name)
		s.ws.Close(name)
		s.releaseOpenedDirectories(name)
	})
	s.clearDiagnostics(ctx, name)
	s.queueOpenDiagnostics(ctx, name)
	return nil
}

// DidSave refreshes diagnostics for every open document, since an edit to one
// file changes what the others resolve, and writes the saved document's record.
func (s *Server) DidSave(ctx context.Context, params *protocol.DidSaveTextDocumentParams) error {
	if isLibraryURI(params.TextDocument.URI) {
		return nil
	}
	name := uriToName(params.TextDocument.URI)
	s.publishDiagnostics(ctx, name)
	s.refreshOpenDiagnostics(ctx, name)
	// The saved document is analyzed: its record is written for the next
	// workspace to hold it closed as.
	if err := s.ws.WriteRecord(name); err != nil {
		_ = s.client.LogMessage(ctx, &protocol.LogMessageParams{Type: protocol.MessageTypeLog, Message: err.Error()})
	}
	return nil
}

// changeHandler reads textDocument/didChange off the wire first. A client that
// sends "range": null means the whole document, which the protocol library's
// decoding cannot tell from an insertion at the origin; such a change is
// rewritten without the key before the library decodes it. Every other
// didChange goes to the library untouched.
func (s *Server) changeHandler(inner jsonrpc2.Handler) jsonrpc2.Handler {
	return func(ctx context.Context, req *jsonrpc2.Request) (any, error) {
		if req.Method() != protocol.MethodTextDocumentDidChange {
			return inner(ctx, req)
		}
		normalized, ok := withoutNullRanges(req.Params())
		if !ok {
			return inner(ctx, req)
		}
		var params protocol.DidChangeTextDocumentParams
		if err := protocol.Unmarshal(normalized, &params); err != nil {
			return nil, fmt.Errorf("%w: %w", jsonrpc2.ErrParse, err)
		}
		return nil, s.DidChange(ctx, &params)
	}
}

// withoutNullRanges drops a null "range" from each content change of a
// didChange, reporting whether any was present.
func withoutNullRanges(raw []byte) ([]byte, bool) {
	var envelope struct {
		TextDocument   json.RawMessage   `json:"textDocument"`
		ContentChanges []json.RawMessage `json:"contentChanges"`
	}
	if json.Unmarshal(raw, &envelope) != nil {
		return nil, false
	}
	rewritten := false
	for i, change := range envelope.ContentChanges {
		var fields map[string]json.RawMessage
		if json.Unmarshal(change, &fields) != nil {
			continue
		}
		rng, ok := fields["range"]
		if !ok || !bytes.Equal(bytes.TrimSpace(rng), []byte("null")) {
			continue
		}
		delete(fields, "range")
		normalized, err := json.Marshal(fields)
		if err != nil {
			return nil, false
		}
		envelope.ContentChanges[i] = normalized
		rewritten = true
	}
	if !rewritten {
		return nil, false
	}
	out, err := json.Marshal(envelope)
	return out, err == nil
}
