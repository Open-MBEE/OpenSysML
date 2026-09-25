package lsp

import (
	"context"
	"encoding/json"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

// rawContentChange mirrors protocol.TextDocumentContentChangeEvent but keeps
// Range as a pointer so we can distinguish an omitted range (full-document
// replace) from an incremental edit whose range happens to be {0,0}-{0,0}
// (an insertion at the very start of the document). protocol's own type uses a
// value Range with `json:"range"` (no omitempty), so both cases decode to the
// zero Range and become indistinguishable once dispatched by the library.
type rawContentChange struct {
	Range       *protocol.Range `json:"range"`
	RangeLength uint32          `json:"rangeLength,omitempty"`
	Text        string          `json:"text"`
}

// rawDidChangeParams is the minimal shape of textDocument/didChange we decode
// ourselves so the pointer-valued Range survives.
type rawDidChangeParams struct {
	TextDocument struct {
		URI     protocol.DocumentURI `json:"uri"`
		Version int32                `json:"version"`
	} `json:"textDocument"`
	ContentChanges []rawContentChange `json:"contentChanges"`
}

// changeHandler wraps protocol.ServerHandler and intercepts
// textDocument/didChange, decoding it with a pointer-valued Range so full
// replaces and start-of-document insertions are handled correctly. All other
// methods fall through to the wrapped handler.
func (s *Server) changeHandler(inner jsonrpc2.Handler) jsonrpc2.Handler {
	return func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
		if req.Method() != protocol.MethodTextDocumentDidChange {
			return inner(ctx, reply, req)
		}
		var params rawDidChangeParams
		if err := json.Unmarshal(req.Params(), &params); err != nil {
			return reply(ctx, nil, err)
		}
		if isLibraryURI(params.TextDocument.URI) {
			return reply(ctx, nil, s.refuseLibraryChange(ctx, params.TextDocument.URI))
		}
		s.applyDidChange(ctx, uriToName(params.TextDocument.URI), params.ContentChanges, int(params.TextDocument.Version))
		return reply(ctx, nil, nil)
	}
}

// applyDidChange folds the content changes into the current document text and
// updates the workspace, publishing the edited document at once and the other
// open ones once typing settles: the edit changes what they resolve. A change
// with a nil Range is a full-document replace, otherwise an incremental splice.
func (s *Server) applyDidChange(ctx context.Context, name string, changes []rawContentChange, version int) {
	doc := s.ws.Document(name)
	var content []byte
	if doc != nil {
		content = append([]byte(nil), doc.Content...)
	}
	for _, ch := range changes {
		content = applyRawContentChange(content, ch)
	}
	s.debugEdit(ctx, "", func() { s.ws.Update(name, content, version) })
	s.publishDiagnostics(ctx, name)
	s.queueOpenDiagnostics(ctx, name)
}

// applyRawContentChange applies a single change. Nil Range means full replace;
// a JSON null (not valid LSP, where range is optional) decodes to nil as well.
// RangeLength is intentionally unused for the splice: it is a UTF-16 code-unit
// count (not a byte count), and the authoritative delete extent is Range.
func applyRawContentChange(content []byte, ch rawContentChange) []byte {
	if ch.Range == nil {
		return []byte(ch.Text)
	}
	sp := rangeToSpan(content, *ch.Range)
	start := sp.Offset
	end := sp.End()
	if start > len(content) {
		start = len(content)
	}
	if end > len(content) {
		end = len(content)
	}
	out := make([]byte, 0, start+len(ch.Text)+(len(content)-end))
	out = append(out, content[:start]...)
	out = append(out, ch.Text...)
	out = append(out, content[end:]...)
	return out
}
