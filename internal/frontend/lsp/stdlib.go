package lsp

import (
	"context"
	"encoding/json"
	"fmt"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// MethodStdlibContent serves the text of a bundled library document by its
// sysml-stdlib URI, which is how a client opens one.
const MethodStdlibContent = "opensysml/stdlibContent"

// stdlibContentParams names the library document whose text is asked for.
type stdlibContentParams struct {
	URI protocol.DocumentURI `json:"uri"`
}

// stdlibContentResult is the text of a library document, read-only.
type stdlibContentResult struct {
	Text string `json:"text"`
}

// StdlibContent answers opensysml/stdlibContent with the bundled text of the
// library document params names, or an error for a URI that names none.
func (s *Server) StdlibContent(params *stdlibContentParams) (*stdlibContentResult, error) {
	name, ok := libraryURIName(params.URI)
	if !ok {
		return nil, jsonrpc2.Errorf(jsonrpc2.InvalidParams, "%q is no %s: URI of a standard library document", params.URI, LibraryScheme)
	}
	doc := s.ws.LibraryDocument(name)
	if doc == nil {
		return nil, jsonrpc2.Errorf(jsonrpc2.InvalidParams, "the standard library has no document %q", name)
	}
	return &stdlibContentResult{Text: string(doc.Content)}, nil
}

// stdlibHandler dispatches opensysml/stdlibContent; other methods fall through.
func (s *Server) stdlibHandler(inner jsonrpc2.Handler) jsonrpc2.Handler {
	return func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
		if req.Method() != MethodStdlibContent {
			return inner(ctx, reply, req)
		}
		var params stdlibContentParams
		if err := json.Unmarshal(req.Params(), &params); err != nil {
			return reply(ctx, nil, fmt.Errorf("%s: %w", jsonrpc2.ErrParse, err))
		}
		result, err := s.StdlibContent(&params)
		return reply(ctx, result, err)
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
