package lsp

import (
	"bytes"
	"context"
	"unicode/utf8"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/core/highlight"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// semanticTokensProvider is the LSP 3.16 capability shape, declared here because
// the protocol library's options type predates the legend. Deltas are not served.
type semanticTokensProvider struct {
	Legend protocol.SemanticTokensLegend `json:"legend"`
	Full   bool                          `json:"full"`
	Range  bool                          `json:"range"`
}

// semanticTokensLegend is the legend the server advertises: the token types and
// modifiers highlight emits, in the order an encoded token indexes them by.
func semanticTokensLegend() protocol.SemanticTokensLegend {
	classes := highlight.Classes()
	types := make([]protocol.SemanticTokenTypes, len(classes))
	for i, c := range classes {
		types[i] = protocol.SemanticTokenTypes(c.String())
	}
	mods := highlight.Modifiers()
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
	return &protocol.SemanticTokens{Data: encodeTokens(content, toks)}, nil
}

// SemanticTokensRange answers the tokens overlapping a range, the document's
// tokens filtered: highlighting a name resolves the whole document either way.
func (s *Server) SemanticTokensRange(ctx context.Context, params *protocol.SemanticTokensRangeParams) (*protocol.SemanticTokens, error) {
	content, toks := s.ws.HighlightTokens(uriToName(params.TextDocument.URI))
	if content == nil {
		return &protocol.SemanticTokens{}, nil
	}
	want := rangeToSpan(content, params.Range)
	var in []highlight.Token
	for _, tok := range toks {
		if tok.Span.Offset < want.End() && tok.Span.End() > want.Offset {
			in = append(in, tok)
		}
	}
	return &protocol.SemanticTokens{Data: encodeTokens(content, in)}, nil
}

// encodeTokens encodes tokens relative to their predecessor in UTF-16 units,
// splitting a multi-line token per line as the encoding requires.
func encodeTokens(content []byte, toks []highlight.Token) []uint32 {
	data := make([]uint32, 0, 5*len(toks))
	prevLine, prevChar := uint32(0), uint32(0)
	cursor := &lineCursor{content: content}
	for _, tok := range toks {
		for _, part := range splitLines(content, tok.Span) {
			pos := cursor.positionAt(part.Offset)
			length := utf16Len(content[part.Offset:part.End()])
			deltaLine := pos.Line - prevLine
			deltaChar := pos.Character
			if deltaLine == 0 {
				deltaChar = pos.Character - prevChar
			}
			data = append(data, deltaLine, deltaChar, uint32Clamp(length),
				uint32Clamp(int(tok.Class)), uint32(tok.Modifiers))
			prevLine, prevChar = pos.Line, pos.Character
		}
	}
	return data
}

// lineCursor converts increasing byte offsets to positions in one forward pass,
// so encoding a whole document's tokens stays linear in its length.
type lineCursor struct {
	content []byte
	offset  int
	line    int
	char    int
}

// positionAt is the position of offset, resuming from the last one answered. An
// offset behind that one restarts the pass, so the result never depends on order.
func (c *lineCursor) positionAt(offset int) protocol.Position {
	if offset < 0 {
		offset = 0
	}
	if offset > len(c.content) {
		offset = len(c.content)
	}
	if offset < c.offset {
		c.offset, c.line, c.char = 0, 0, 0
	}
	for c.offset < offset {
		r, size := utf8.DecodeRune(c.content[c.offset:])
		c.offset += size
		if r == '\n' {
			c.line, c.char = c.line+1, 0
			continue
		}
		c.char += utf16RuneLen(r)
	}
	return protocol.Position{Line: uint32Clamp(c.line), Character: uint32Clamp(c.char)}
}

// splitLines cuts a span into one span per line it covers, dropping the line
// terminators and any resulting empty piece.
func splitLines(content []byte, sp source.Span) []source.Span {
	if sp.Len <= 0 || sp.Offset < 0 || sp.End() > len(content) {
		return nil
	}
	var out []source.Span
	start := sp.Offset
	for start < sp.End() {
		end := sp.End()
		if nl := bytes.IndexByte(content[start:end], '\n'); nl >= 0 {
			end = start + nl
		}
		trimmed := end
		if trimmed > start && content[trimmed-1] == '\r' {
			trimmed--
		}
		if trimmed > start {
			out = append(out, source.Span{Offset: start, Len: trimmed - start})
		}
		start = end + 1
	}
	return out
}
