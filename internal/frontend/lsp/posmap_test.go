package lsp

import (
	"bytes"
	"testing"
	"unicode/utf8"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestOffsetToPositionASCII(t *testing.T) {
	content := []byte("package P;\nnamespace N;\n")
	// offset of 'N' in "namespace N" — line 1 (0-based), after "namespace "
	nOffset := 11 + len("namespace ")
	pos := offsetToPosition(content, nOffset)
	if pos.Line != 1 {
		t.Errorf("Line = %d, want 1", pos.Line)
	}
	if pos.Character != 10 {
		t.Errorf("Character = %d, want 10", pos.Character)
	}
}

func TestOffsetToPositionUTF16Astral(t *testing.T) {
	// "x = 😀;" — the emoji is 4 UTF-8 bytes and 2 UTF-16 code units.
	content := []byte("x = 😀;")
	semicolonByteOffset := len("x = 😀") // byte offset of ';'
	pos := offsetToPosition(content, semicolonByteOffset)
	if pos.Line != 0 {
		t.Errorf("Line = %d, want 0", pos.Line)
	}
	// "x = " is 4 UTF-16 units, emoji is 2 => ';' at UTF-16 col 6
	if pos.Character != 6 {
		t.Errorf("Character = %d, want 6", pos.Character)
	}
}

func TestPositionToOffsetRoundTrip(t *testing.T) {
	content := []byte("alpha\nbéta 😀 x\n")
	for _, off := range []int{0, 3, 6, 7, 11, 12} {
		pos := offsetToPosition(content, off)
		got := positionToOffset(content, pos)
		if got != off {
			t.Errorf("round trip offset %d -> %+v -> %d", off, pos, got)
		}
	}
}

func TestOffsetToPositionEdgeCases(t *testing.T) {
	content := []byte("ab\ncd")
	tests := []struct {
		name          string
		content       []byte
		offset        int
		wantLine      uint32
		wantCharacter uint32
	}{
		{"negative clamps to start", content, -5, 0, 0},
		{"past EOF clamps to end", content, 100, 1, 2},
		{"empty content any offset", []byte{}, 5, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pos := offsetToPosition(tt.content, tt.offset)
			if pos.Line != tt.wantLine || pos.Character != tt.wantCharacter {
				t.Errorf("offsetToPosition = %+v, want {Line:%d Character:%d}", pos, tt.wantLine, tt.wantCharacter)
			}
		})
	}
}

func TestPositionToOffsetEdgeCases(t *testing.T) {
	content := []byte("ab\ncd")
	tests := []struct {
		name string
		pos  protocol.Position
		want int
	}{
		{"line past EOF clamps to len", protocol.Position{Line: 9, Character: 0}, len(content)},
		{"character past line end stops before newline", protocol.Position{Line: 0, Character: 99}, 2},
		{"character past last line end clamps to len", protocol.Position{Line: 1, Character: 99}, len(content)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := positionToOffset(content, tt.pos); got != tt.want {
				t.Errorf("positionToOffset(%+v) = %d, want %d", tt.pos, got, tt.want)
			}
		})
	}
}

func TestPositionToOffsetInvalidUTF8NoPanic(t *testing.T) {
	content := []byte{0xff, 0xfe, '\n', 0xe4}
	// Must not panic or loop; each bad byte advances by one.
	_ = positionToOffset(content, protocol.Position{Line: 0, Character: 5})
	_ = offsetToPosition(content, 3)
}

func TestSpanToRange(t *testing.T) {
	content := []byte("package P;\nnamespace N;\n")
	sp := source.Span{Offset: 8, Len: 1} // 'P'
	r := spanToRange(content, sp)
	want := protocol.Range{
		Start: protocol.Position{Line: 0, Character: 8},
		End:   protocol.Position{Line: 0, Character: 9},
	}
	if r != want {
		t.Errorf("spanToRange = %+v, want %+v", r, want)
	}
}

// linearOffsetToPosition is the pre-index scan offsetToPosition was, kept here
// as the reference the line-indexed positions must agree with.
func linearOffsetToPosition(content []byte, offset int) protocol.Position {
	if offset < 0 {
		offset = 0
	}
	if offset > len(content) {
		offset = len(content)
	}
	line := 0
	lineStart := 0
	for i := 0; i < offset; i++ {
		if content[i] == '\n' {
			line++
			lineStart = i + 1
		}
	}
	char := utf16Len(content[lineStart:offset])
	return protocol.Position{Line: uint32Clamp(line), Character: uint32Clamp(char)}
}

// linearPositionToOffset is the pre-index walk positionToOffset was.
func linearPositionToOffset(content []byte, pos protocol.Position) int {
	line := 0
	i := 0
	for line < int(pos.Line) && i < len(content) {
		if content[i] == '\n' {
			line++
		}
		i++
	}
	units := 0
	for i < len(content) && content[i] != '\n' {
		if units >= int(pos.Character) {
			break
		}
		r, size := utf8.DecodeRune(content[i:])
		units += utf16RuneLen(r)
		i += size
	}
	return i
}

func TestPositionsAgreesWithLinearScan(t *testing.T) {
	contents := map[string][]byte{
		"ascii multi-line": []byte("package P;\nnamespace N;\npart x;"),
		"crlf":             []byte("a\r\nb\r\nc"),
		"multibyte":        []byte("café\nbéta\n"),
		"astral":           []byte("x = 😀;\ny = 😀😀;\n"),
		"empty":            {},
	}
	for name, content := range contents {
		t.Run(name+"/position", func(t *testing.T) {
			pos := positionsFor(content)
			offsets := []int{-10, -1, 0, 1, len(content) / 2, len(content) - 1, len(content), len(content) + 1, len(content) + 100}
			for _, off := range offsets {
				if got, want := pos.position(off), linearOffsetToPosition(content, off); got != want {
					t.Errorf("position(%d) = %+v, want %+v", off, got, want)
				}
			}
		})
		t.Run(name+"/offset", func(t *testing.T) {
			pos := positionsFor(content)
			positionsIn := []protocol.Position{
				{Line: 0, Character: 0},
				{Line: 0, Character: 1},
				{Line: 1, Character: 0},
				{Line: 1, Character: 3},
				{Line: 0, Character: 999},   // character past line end
				{Line: 999, Character: 0},   // line past last line
				{Line: 999, Character: 999}, // both past the end
			}
			for _, pin := range positionsIn {
				if got, want := pos.offset(pin), linearPositionToOffset(content, pin); got != want {
					t.Errorf("offset(%+v) = %d, want %d", pin, got, want)
				}
			}
		})
		t.Run(name+"/rangeOf", func(t *testing.T) {
			pos := positionsFor(content)
			spans := []source.Span{
				{Offset: 0, Len: 0},
				{Offset: 0, Len: len(content)},
				{Offset: len(content) / 2, Len: 1},
				{Offset: len(content), Len: 0},
				{Offset: len(content) + 5, Len: 3},
			}
			for _, sp := range spans {
				want := protocol.Range{
					Start: linearOffsetToPosition(content, sp.Offset),
					End:   linearOffsetToPosition(content, sp.End()),
				}
				if got := pos.rangeOf(sp); got != want {
					t.Errorf("rangeOf(%+v) = %+v, want %+v", sp, got, want)
				}
			}
		})
	}
}

func BenchmarkSpanToRangeLarge(b *testing.B) {
	var buf bytes.Buffer
	for buf.Len() < 10<<20 {
		buf.WriteString("part somePart : SomeDef { attribute x : Real; }\n")
	}
	content := buf.Bytes()
	pos := positionsFor(content)
	spans := make([]source.Span, 10000)
	for i := range spans {
		off := (i * 977) % len(content)
		spans[i] = source.Span{Offset: off, Len: 4}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, sp := range spans {
			_ = pos.rangeOf(sp)
		}
	}
}
