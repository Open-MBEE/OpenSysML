package lsp

import (
	"context"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/semtok"
)

// semanticTokensProvider is the LSP 3.16 capability shape, declared here because
// the protocol library's options type predates the legend. Deltas are not served.
type semanticTokensProvider struct {
	Legend protocol.SemanticTokensLegend `json:"legend"`
	Full   bool                          `json:"full"`
	Range  bool                          `json:"range"`
}

// semanticTokensLegend is the legend the server advertises: the token types and
// modifiers semtok emits, in the order an encoded token indexes them by.
func semanticTokensLegend() protocol.SemanticTokensLegend {
	classes := semtok.Classes()
	types := make([]protocol.SemanticTokenTypes, len(classes))
	for i, c := range classes {
		types[i] = protocol.SemanticTokenTypes(c.String())
	}
	mods := semtok.Modifiers()
	modifiers := make([]protocol.SemanticTokenModifiers, len(mods))
	for i, m := range mods {
		modifiers[i] = protocol.SemanticTokenModifiers(m.String())
	}
	return protocol.SemanticTokensLegend{TokenTypes: types, TokenModifiers: modifiers}
}

// SemanticTokensFull answers the semantic tokens of a whole document, a bundled
// library one included.
func (s *Server) SemanticTokensFull(ctx context.Context, params *protocol.SemanticTokensParams) (*protocol.SemanticTokens, error) {
	content, toks := s.ws.HighlightTokens(uriToName(params.TextDocument.URI))
	if content == nil {
		return &protocol.SemanticTokens{}, nil
	}
	return &protocol.SemanticTokens{Data: semtok.Encode(content, toks)}, nil
}

// SemanticTokensRange answers the tokens overlapping a range, the document's
// tokens filtered: highlighting a name resolves the whole document either way.
func (s *Server) SemanticTokensRange(ctx context.Context, params *protocol.SemanticTokensRangeParams) (*protocol.SemanticTokens, error) {
	content, toks := s.ws.HighlightTokens(uriToName(params.TextDocument.URI))
	if content == nil {
		return &protocol.SemanticTokens{}, nil
	}
	want := rangeToSpan(content, params.Range)
	var in []semtok.Token
	for _, tok := range toks {
		if tok.Span.Offset < want.End() && tok.Span.End() > want.Offset {
			in = append(in, tok)
		}
	}
	return &protocol.SemanticTokens{Data: semtok.Encode(content, in)}, nil
}
