package toolkit

import (
	"testing"

	"go.lsp.dev/protocol"
)

func TestPositionRoundTripsThroughUTF16(t *testing.T) {
	line := "part 😀x : A;"
	cases := []struct {
		pos  Position
		want protocol.Position
	}{
		{Position{1, 1}, protocol.Position{Line: 0, Character: 0}},
		{Position{3, 6}, protocol.Position{Line: 2, Character: 5}},
		// The emoji is one character, four bytes and two UTF-16 units: columns
		// past it are one more unit than characters, never bytes.
		{Position{1, 6}, protocol.Position{Line: 0, Character: 5}},
		{Position{1, 7}, protocol.Position{Line: 0, Character: 7}},
		{Position{1, 8}, protocol.Position{Line: 0, Character: 8}},
		{Position{1, 12}, protocol.Position{Line: 0, Character: 12}},
		// A column past the end of the line is the end of the line.
		{Position{1, 40}, protocol.Position{Line: 0, Character: 13}},
		{Position{0, 0}, protocol.Position{Line: 0, Character: 0}},
	}
	for _, tc := range cases {
		got := tc.pos.ToProtocol(line)
		if got != tc.want {
			t.Errorf("%+v.ToProtocol = %+v, want %+v", tc.pos, got, tc.want)
		}
		if tc.pos.Line < 1 || tc.pos.Column < 1 || tc.pos.Column > 13 {
			continue
		}
		if back := FromProtocol(got, line); back != tc.pos {
			t.Errorf("FromProtocol(%+v) = %+v, want %+v", got, back, tc.pos)
		}
	}
}

func TestColumnsCountCharactersNotBytes(t *testing.T) {
	line := "ÀÀÀÀx : A;"
	got := Position{1, 5}.ToProtocol(line)
	if got != (protocol.Position{Line: 0, Character: 4}) {
		t.Errorf("column 5 of %q = %+v, want character 4", line, got)
	}
	if back := FromProtocol(got, line); back != (Position{1, 5}) {
		t.Errorf("FromProtocol(%+v) = %+v", got, back)
	}
}

func TestAnnotationsSayWhatAToolDoes(t *testing.T) {
	if a := ReadOnly("Hover"); !a.ReadOnlyHint || a.Title != "Hover" || *a.OpenWorldHint {
		t.Errorf("ReadOnly = %+v", a)
	}
	if a := Mutating("Rename", true); a.ReadOnlyHint || !a.IdempotentHint || !*a.DestructiveHint {
		t.Errorf("Mutating = %+v", a)
	}
}
