package lsp

import (
	"math"
	"unicode/utf8"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// positions converts between byte offsets and LSP positions of one text, from
// its line index.
type positions struct {
	content []byte
	lines   *source.LineIndex
}

// positionsOf converts in doc's text, sharing the document's cached line index.
func positionsOf(doc *model.Document) positions {
	return positions{content: doc.Content, lines: doc.Lines()}
}

// positionsFor converts in content, indexing its lines once.
func positionsFor(content []byte) positions {
	return positions{content: content, lines: source.NewLineIndex(content)}
}

// position converts a byte offset to a 0-based LSP Position whose Character is
// a UTF-16 code-unit column.
func (p positions) position(offset int) protocol.Position {
	if offset < 0 {
		offset = 0
	}
	if offset > len(p.content) {
		offset = len(p.content)
	}
	pos := p.lines.PosAt(offset)
	lineStart := offset - (pos.Col - 1)
	char := utf16Len(p.content[lineStart:offset])
	return protocol.Position{Line: uint32Clamp(pos.Line - 1), Character: uint32Clamp(char)}
}

// offset converts a 0-based LSP Position (UTF-16 column) to a byte offset.
// Out-of-range positions clamp to the end of content.
func (p positions) offset(pos protocol.Position) int {
	i := p.lines.OffsetAt(source.Pos{Line: int(pos.Line) + 1, Col: 1})
	if i < 0 {
		return len(p.content)
	}
	units := 0
	for i < len(p.content) && p.content[i] != '\n' {
		if units >= int(pos.Character) {
			break
		}
		r, size := utf8.DecodeRune(p.content[i:])
		units += utf16RuneLen(r)
		i += size
	}
	return i
}

// rangeOf converts a core byte Span to an LSP Range.
func (p positions) rangeOf(sp source.Span) protocol.Range {
	return protocol.Range{
		Start: p.position(sp.Offset),
		End:   p.position(sp.End()),
	}
}

// offsetToPosition converts a byte offset in content to a 0-based LSP Position
// whose Character is a UTF-16 code-unit column.
func offsetToPosition(content []byte, offset int) protocol.Position {
	return positionsFor(content).position(offset)
}

// positionToOffset converts a 0-based LSP Position (UTF-16 column) to a byte
// offset in content. Out-of-range positions clamp to the end of content.
func positionToOffset(content []byte, pos protocol.Position) int {
	return positionsFor(content).offset(pos)
}

// spanToRange converts a core byte Span to an LSP Range.
func spanToRange(content []byte, sp source.Span) protocol.Range {
	return positionsFor(content).rangeOf(sp)
}

// rangeToSpan converts an LSP Range to a core byte Span.
func rangeToSpan(content []byte, r protocol.Range) source.Span {
	p := positionsFor(content)
	start := p.offset(r.Start)
	end := p.offset(r.End)
	if end < start {
		end = start
	}
	return source.Span{Offset: start, Len: end - start}
}

// uint32Clamp narrows a line or column number to the protocol's uint32,
// saturating rather than wrapping.
func uint32Clamp(n int) uint32 {
	if n < 0 {
		return 0
	}
	if n > math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(n)
}

// utf16Len returns the number of UTF-16 code units in b.
func utf16Len(b []byte) int {
	n := 0
	for i := 0; i < len(b); {
		r, size := utf8.DecodeRune(b[i:])
		n += utf16RuneLen(r)
		i += size
	}
	return n
}

// utf16RuneLen returns the number of UTF-16 code units for r: 2 for
// astral-plane runes (> U+FFFF, encoded as a surrogate pair), else 1.
func utf16RuneLen(r rune) int {
	if r > 0xFFFF {
		return 2
	}
	return 1
}
