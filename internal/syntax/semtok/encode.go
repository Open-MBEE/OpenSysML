// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

package semtok

import (
	"bytes"
	"math"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Position is a zero-based line and character, the shape the LSP reports in;
// the character counts UTF-16 code units.
type Position struct {
	Line      uint32
	Character uint32
}

// Encode encodes tokens relative to their predecessor in UTF-16 units,
// splitting a multi-line token per line as the encoding requires: the data
// array a SemanticTokens answer carries.
func Encode(content []byte, toks []Token) []uint32 {
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
func (c *lineCursor) positionAt(offset int) Position {
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
		c.char += utf16.RuneLen(r)
	}
	return Position{Line: uint32Clamp(c.line), Character: uint32Clamp(c.char)}
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

// utf16Len returns the number of UTF-16 code units in b.
func utf16Len(b []byte) int {
	n := 0
	for i := 0; i < len(b); {
		r, size := utf8.DecodeRune(b[i:])
		n += utf16.RuneLen(r)
		i += size
	}
	return n
}

// uint32Clamp narrows a count to the encoding's uint32, saturating rather than
// wrapping.
func uint32Clamp(n int) uint32 {
	if n < 0 {
		return 0
	}
	if n > math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(n)
}
