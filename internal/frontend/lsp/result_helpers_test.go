package lsp

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

// The protocol library answers several requests as union types; these helpers
// read the arm this server answers with, failing the test on any other.

// hoverMarkup is the Markdown or plain-text content of a hover.
func hoverMarkup(t testing.TB, h *protocol.Hover) protocol.MarkupContent {
	t.Helper()
	if h == nil {
		t.Fatalf("hover = nil")
	}
	content, ok := h.Contents.(*protocol.MarkupContent)
	if !ok || content == nil {
		t.Fatalf("hover contents = %T, want *protocol.MarkupContent", h.Contents)
	}
	return *content
}

// definitions is Definition answered as the locations it lists, none for nil.
func definitions(t testing.TB, s *Server, ctx context.Context, params *protocol.DefinitionParams) ([]protocol.Location, error) {
	t.Helper()
	res, err := s.Definition(ctx, params)
	if err != nil || res == nil {
		return nil, err
	}
	locs, ok := res.(protocol.LocationSlice)
	if !ok {
		t.Fatalf("definition result = %T, want protocol.LocationSlice", res)
	}
	return []protocol.Location(locs), nil
}

// completions is Completion answered as the list it returns.
func completions(t testing.TB, s *Server, ctx context.Context, params *protocol.CompletionParams) (*protocol.CompletionList, error) {
	t.Helper()
	res, err := s.Completion(ctx, params)
	if err != nil || res == nil {
		return nil, err
	}
	list, ok := res.(*protocol.CompletionList)
	if !ok {
		t.Fatalf("completion result = %T, want *protocol.CompletionList", res)
	}
	return list, nil
}

// documentSymbols is DocumentSymbol answered as the symbol tree, none for nil.
func documentSymbols(t testing.TB, s *Server, ctx context.Context, params *protocol.DocumentSymbolParams) ([]protocol.DocumentSymbol, error) {
	t.Helper()
	res, err := s.DocumentSymbol(ctx, params)
	if err != nil || res == nil {
		return nil, err
	}
	syms, ok := res.(protocol.DocumentSymbolSlice)
	if !ok {
		t.Fatalf("document symbol result = %T, want protocol.DocumentSymbolSlice", res)
	}
	return []protocol.DocumentSymbol(syms), nil
}

// workspaceSymbols is Symbols answered as the flat list, none for nil.
func workspaceSymbols(t testing.TB, s *Server, ctx context.Context, params *protocol.WorkspaceSymbolParams) ([]protocol.SymbolInformation, error) {
	t.Helper()
	res, err := s.Symbols(ctx, params)
	if err != nil || res == nil {
		return nil, err
	}
	syms, ok := res.(protocol.SymbolInformationSlice)
	if !ok {
		t.Fatalf("workspace symbol result = %T, want protocol.SymbolInformationSlice", res)
	}
	return []protocol.SymbolInformation(syms), nil
}

// prepareRename is PrepareRename answered as the range it offers, nil for none.
func prepareRename(t testing.TB, s *Server, ctx context.Context, params *protocol.PrepareRenameParams) (*protocol.Range, error) {
	t.Helper()
	res, err := s.PrepareRename(ctx, params)
	if err != nil || res == nil {
		return nil, err
	}
	rng, ok := res.(*protocol.Range)
	if !ok {
		t.Fatalf("prepare rename result = %T, want *protocol.Range", res)
	}
	return rng, nil
}

// codeActions is CodeAction answered as the actions it lists.
func codeActions(t testing.TB, s *Server, ctx context.Context, params *protocol.CodeActionParams) ([]protocol.CodeAction, error) {
	t.Helper()
	res, err := s.CodeAction(ctx, params)
	if err != nil {
		return nil, err
	}
	out := make([]protocol.CodeAction, 0, len(res))
	for _, item := range res {
		action, ok := item.(*protocol.CodeAction)
		if !ok || action == nil {
			t.Fatalf("code action = %T, want *protocol.CodeAction", item)
		}
		out = append(out, *action)
	}
	return out, nil
}

// optString is the value of an optional string, empty when absent.
func optString(o protocol.Optional[string]) string {
	v, _ := o.Get()
	return v
}

// nullableUint32 is the value of a nullable count, zero when null or absent.
func nullableUint32(n protocol.Nullable[uint32]) uint32 {
	v, _ := n.Get()
	return v
}

// diagnosticMessage is the text of a diagnostic's message.
func diagnosticMessage(t testing.TB, d protocol.Diagnostic) string {
	t.Helper()
	msg, ok := d.Message.(protocol.String)
	if !ok {
		t.Fatalf("diagnostic message = %T, want protocol.String", d.Message)
	}
	return string(msg)
}

// diagnosticCode is the code of a diagnostic, empty when it has none.
func diagnosticCode(d protocol.Diagnostic) string {
	code, _ := d.Code.(protocol.String)
	return string(code)
}

// mustLSPAny encodes a value as the opaque JSON a command argument or lens data is.
func mustLSPAny(t testing.TB, v any) protocol.LSPAny {
	t.Helper()
	raw, err := lspAny(v)
	if err != nil {
		t.Fatalf("encode %v: %v", v, err)
	}
	return raw
}

// decodeLSPAny decodes opaque JSON into out.
func decodeLSPAny(t testing.TB, raw protocol.LSPAny, out any) {
	t.Helper()
	if err := json.Unmarshal(raw, out); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
}

// experimentalCaps is the experimental server capabilities as the client reads them.
func experimentalCaps(t testing.TB, res *protocol.InitializeResult) map[string]any {
	t.Helper()
	var out map[string]any
	decodeLSPAny(t, res.Capabilities.Experimental, &out)
	for key, value := range out {
		if list, ok := value.([]any); ok {
			names := make([]string, 0, len(list))
			for _, item := range list {
				if name, ok := item.(string); ok {
					names = append(names, name)
				}
			}
			if len(names) == len(list) {
				out[key] = names
			}
		}
	}
	return out
}

// textDocumentEdit is the i-th document change of an edit, which must be one.
func textDocumentEdit(t testing.TB, edit *protocol.WorkspaceEdit, i int) *protocol.TextDocumentEdit {
	t.Helper()
	if edit == nil || i >= len(edit.DocumentChanges) {
		t.Fatalf("workspace edit has no document change %d", i)
	}
	change, ok := edit.DocumentChanges[i].(*protocol.TextDocumentEdit)
	if !ok || change == nil {
		t.Fatalf("document change %d = %T, want *protocol.TextDocumentEdit", i, edit.DocumentChanges[i])
	}
	return change
}

// workspaceFolders lists folders the way initialize carries them.
func workspaceFolders(folders ...protocol.WorkspaceFolder) protocol.WorkspaceFoldersInitializeParams {
	return protocol.WorkspaceFoldersInitializeParams{WorkspaceFolders: protocol.NewNullable(folders)}
}

// applyFullChange replaces the named document's text as a didChange carrying
// the whole document does.
func applyFullChange(ctx context.Context, s *Server, name, text string, version int) {
	_ = s.DidChange(ctx, &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: nameToURI(name)},
			Version:                int32(version), // #nosec G115 -- test versions are small.
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{&protocol.TextDocumentContentChangeWholeDocument{Text: text}},
	})
}

// hintLabel is the text of an inlay hint's label.
func hintLabel(h protocol.InlayHint) string {
	label, _ := h.Label.(protocol.String)
	return string(label)
}

// padded reports whether an inlay hint asks for padding.
func padded(flag *bool) bool { return flag != nil && *flag }

// dispatcher serves a handler over an in-memory connection: the library builds
// requests on its read loop alone, so a handler is driven the way a client does.
type dispatcher struct {
	client jsonrpc2.Conn
	served chan handled
}

// handled is what one request's handler returned.
type handled struct {
	method string
	result any
	err    error
}

// newDispatcher serves handler until the test ends.
func newDispatcher(t testing.TB, handler jsonrpc2.Handler) *dispatcher {
	t.Helper()
	d := &dispatcher{served: make(chan handled, 64)}
	clientEnd, serverEnd := net.Pipe()
	server := jsonrpc2.NewConn(jsonrpc2.NewStream(serverEnd), jsonrpc2.WithCodec(lspCodec{}))
	server.Go(context.Background(), func(ctx context.Context, req *jsonrpc2.Request) (any, error) {
		result, err := handler(ctx, req)
		d.served <- handled{method: req.Method(), result: result, err: err}
		return result, err
	})
	d.client = jsonrpc2.NewConn(jsonrpc2.NewStream(clientEnd), jsonrpc2.WithCodec(lspCodec{}))
	d.client.Go(context.Background(), jsonrpc2.MethodNotFoundHandler)
	t.Cleanup(func() {
		_ = d.client.Close()
		_ = server.Close()
	})
	return d
}

// call sends a request and returns the raw result the client receives, or the
// error the handler answered with, as it returned it.
func (d *dispatcher) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	var raw protocol.LSPAny
	_, callErr := d.client.Call(ctx, method, params, &raw)
	h := <-d.served
	if h.err != nil {
		return nil, h.err
	}
	if callErr != nil {
		return nil, callErr
	}
	return json.RawMessage(raw), nil
}

// notify sends a notification and reports what the handler made of it.
func (d *dispatcher) notify(t testing.TB, ctx context.Context, method string, params any) handled {
	t.Helper()
	if err := d.client.Notify(ctx, method, params); err != nil {
		t.Fatalf("notify %s: %v", method, err)
	}
	select {
	case h := <-d.served:
		return h
	case <-time.After(10 * time.Second):
		t.Fatalf("%s was never handled", method)
		return handled{}
	}
}

// actionKind is the kind of a code action, empty when it has none.
func actionKind(a protocol.CodeAction) protocol.CodeActionKind {
	if a.Kind == nil {
		return ""
	}
	return *a.Kind
}

// diagnosticText is the text of a diagnostic's message, empty for any other arm.
func diagnosticText(d protocol.Diagnostic) string {
	msg, _ := d.Message.(protocol.String)
	return string(msg)
}

// textEditsOf lists the plain text edits a document change holds.
func textEditsOf(t testing.TB, change *protocol.TextDocumentEdit) []protocol.TextEdit {
	t.Helper()
	out := make([]protocol.TextEdit, 0, len(change.Edits))
	for _, element := range change.Edits {
		edit, ok := element.(*protocol.TextEdit)
		if !ok || edit == nil {
			t.Fatalf("edit element = %T, want *protocol.TextEdit", element)
		}
		out = append(out, *edit)
	}
	return out
}
