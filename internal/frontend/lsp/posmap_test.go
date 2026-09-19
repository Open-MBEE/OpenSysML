package lsp

import (
	"testing"

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
