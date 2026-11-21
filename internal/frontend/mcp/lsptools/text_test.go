package lsptools

import (
	"bytes"
	"testing"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/toolkit"
)

// The position conversions agree on every boundary of a text with CRLF and LF
// line ends, wide characters and no final line break.
func TestPositionConversions(t *testing.T) {
	content := []byte("part a;\r\n\n// é𝄞x\nlast")
	offsets := []struct {
		pos    protocol.Position
		offset int
		at     toolkit.Position
	}{
		{protocol.Position{Line: 0, Character: 0}, 0, toolkit.Position{Line: 1, Column: 1}},
		{protocol.Position{Line: 0, Character: 5}, 5, toolkit.Position{Line: 1, Column: 6}},
		{protocol.Position{Line: 0, Character: 99}, 8, toolkit.Position{Line: 1, Column: 9}},
		{protocol.Position{Line: 1, Character: 0}, 9, toolkit.Position{Line: 2, Column: 1}},
		{protocol.Position{Line: 1, Character: 4}, 9, toolkit.Position{Line: 2, Column: 1}},
		{protocol.Position{Line: 2, Character: 4}, 15, toolkit.Position{Line: 3, Column: 5}},
		{protocol.Position{Line: 2, Character: 6}, 19, toolkit.Position{Line: 3, Column: 6}},
		{protocol.Position{Line: 3, Character: 4}, 25, toolkit.Position{Line: 4, Column: 5}},
		{protocol.Position{Line: 3, Character: 9}, 25, toolkit.Position{Line: 4, Column: 5}},
		{protocol.Position{Line: 4, Character: 0}, 25, toolkit.Position{Line: 4, Column: 5}},
	}
	for _, c := range offsets {
		if got := byteOffset(content, c.pos); got != c.offset {
			t.Errorf("byteOffset(%+v) = %d, want %d", c.pos, got, c.offset)
		}
		if got := offsetPosition(content, c.offset); got != c.at {
			t.Errorf("offsetPosition(%d) = %+v, want %+v", c.offset, got, c.at)
		}
	}
	if got := offsetPosition(content, -3); got != (toolkit.Position{Line: 1, Column: 1}) {
		t.Errorf("offsetPosition(-3) = %+v, want the start", got)
	}
	if got := offsetPosition(content, 999); got != (toolkit.Position{Line: 4, Column: 5}) {
		t.Errorf("offsetPosition(999) = %+v, want the end", got)
	}
	for line, want := range []string{"part a;", "", "// é𝄞x", "last", ""} {
		if got := lineAt(content, uint32(line)); got != want {
			t.Errorf("lineAt(%d) = %q, want %q", line, got, want)
		}
	}
	if got := lineAt(nil, 0); got != "" {
		t.Errorf("lineAt of no content = %q, want it empty", got)
	}
	if got := lineAt([]byte("a\n"), 1); got != "" {
		t.Errorf("lineAt past a final line break = %q, want the empty last line", got)
	}
}

// benchmarkDocument is a model of count declarations, one to a line.
func benchmarkDocument(count int) []byte {
	return bytes.Repeat([]byte("    attribute mass : ScalarValues::Real = 1200.0;\r\n"), count)
}

func BenchmarkByteOffset(b *testing.B) {
	content := benchmarkDocument(5000)
	pos := protocol.Position{Line: 4999, Character: 20}
	b.ReportAllocs()
	for b.Loop() {
		byteOffset(content, pos)
	}
}

func BenchmarkOffsetPosition(b *testing.B) {
	content := benchmarkDocument(5000)
	b.ReportAllocs()
	for b.Loop() {
		offsetPosition(content, len(content)-10)
	}
}

func BenchmarkFromProtocol(b *testing.B) {
	content := benchmarkDocument(5000)
	pos := protocol.Position{Line: 4999, Character: 20}
	b.ReportAllocs()
	for b.Loop() {
		fromProtocol(content, pos)
	}
}
