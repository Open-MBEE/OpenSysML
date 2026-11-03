package lsp

import (
	"context"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// MethodStdlibContent serves the text of a bundled library document by its
// sysml-stdlib URI, which is how a client opens one.
const MethodStdlibContent = "opensysml/stdlibContent"

// StdlibContentParams names the library document whose text is asked for.
type StdlibContentParams struct {
	URI uri.URI `json:"uri"`
}

// StdlibContentResult is the text of a library document, read-only.
type StdlibContentResult struct {
	Text string `json:"text"`
}

// StdlibContent answers opensysml/stdlibContent with the bundled text of the
// library document params names, or an error for a URI that names none.
func (s *Server) StdlibContent(params *StdlibContentParams) (*StdlibContentResult, error) {
	name, ok := libraryURIName(params.URI)
	if !ok {
		return nil, jsonrpc2.Errorf(jsonrpc2.InvalidParams, "%q is no %s: URI of a standard library document", params.URI, LibraryScheme)
	}
	doc := s.ws.LibraryDocument(name)
	if doc == nil {
		return nil, jsonrpc2.Errorf(jsonrpc2.InvalidParams, "the standard library has no document %q", name)
	}
	return &StdlibContentResult{Text: string(doc.Content)}, nil
}

// stdlibHandler dispatches opensysml/stdlibContent; other methods fall through.
func (s *Server) stdlibHandler(inner jsonrpc2.Handler) jsonrpc2.Handler {
	return func(ctx context.Context, req *jsonrpc2.Request) (any, error) {
		if req.Method() != MethodStdlibContent {
			return inner(ctx, req)
		}
		params, err := decodeParams[StdlibContentParams](req)
		if err != nil {
			return nil, err
		}
		return s.StdlibContent(params)
	}
}

// refuseLibraryChange declines an edit to a read-only library document: it is
// never applied, and the client is told why.
func (s *Server) refuseLibraryChange(ctx context.Context, u uri.URI) error {
	err := jsonrpc2.Errorf(jsonrpc2.InvalidRequest, "%s is a read-only standard library document: the change was not applied", u)
	if s.client != nil {
		_ = s.client.ShowMessage(ctx, &protocol.ShowMessageParams{Type: protocol.MessageTypeError, Message: err.Message})
	}
	return err
}
