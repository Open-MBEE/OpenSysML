// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

package semtok

import (
	"testing"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// offsetToPosition is the position a scan from the start of the document
// answers, against which the resuming cursor is checked.
func offsetToPosition(content []byte, offset int) Position {
	line, char := 0, 0
	for i := 0; i < offset; {
		r, size := utf8.DecodeRune(content[i:])
		i += size
		if r == '\n' {
			line, char = line+1, 0
		} else {
			char += utf16.RuneLen(r)
		}
	}
	return Position{Line: uint32(line), Character: uint32(char)}
}

// The cursor answers what a scan from the start of the document answers, for
// offsets in order and out of it, so encoding stays linear without changing.
func TestLineCursorMatchesAScanFromTheStart(t *testing.T) {
	content := []byte("package P {\n    // 🚗 é note\n    part def Wheel;\n\n    part w : Wheel;\n}\n")
	offsets := []int{0, 7, 12, 16, 30, 40, 55, len(content), 3, 0, len(content) - 1}
	cursor := &lineCursor{content: content}
	for _, off := range offsets {
		if got, want := cursor.positionAt(off), offsetToPosition(content, off); got != want {
			t.Errorf("positionAt(%d) = %v, want %v", off, got, want)
		}
	}
}

// Lexical classifies the tokens needing no symbol table: keywords, comments
// and literals, each covering its source text.
func TestLexicalClassifiesKeywordsCommentsAndLiterals(t *testing.T) {
	content := []byte("package P {\n    // note\n    attribute x = \"s\" + 1.5;\n}\n")
	want := map[string]Class{
		"package": ClassKeyword,
		"// note": ClassComment,
		`"s"`:     ClassString,
		"1.5":     ClassNumber,
	}
	seen := map[string]bool{}
	for _, tok := range Lexical(content) {
		text := string(content[tok.Span.Offset:tok.Span.End()])
		class, ok := want[text]
		if !ok || seen[text] {
			continue
		}
		seen[text] = true
		if tok.Class != class {
			t.Errorf("%q classified as %v, want %v", text, tok.Class, class)
		}
	}
	for text := range want {
		if !seen[text] {
			t.Errorf("no token for %q", text)
		}
	}
}

// An empty document has no tokens, and a line comment's span stops before its
// terminator rather than covering it.
func TestLexicalEmptyAndLineEnd(t *testing.T) {
	if toks := Lexical(nil); len(toks) != 0 {
		t.Errorf("tokens of an empty document = %v, want none", toks)
	}
	content := []byte("// note\n")
	toks := Lexical(content)
	if len(toks) != 1 || toks[0].Span != (source.Span{Offset: 0, Len: 7}) {
		t.Errorf("tokens = %v, want the comment without its newline", toks)
	}
}

// Every class and modifier the legend exposes must name itself, since the names
// are what an editor is told.
func TestClassAndModifierNames(t *testing.T) {
	for i, class := range Classes() {
		if class.String() == "unknown" || class.String() == "" {
			t.Errorf("class %d has no name", i)
		}
	}
	for i, mod := range Modifiers() {
		if mod.String() == "unknown" {
			t.Errorf("modifier %d has no name", i)
		}
	}
	if got := Class(-1).String(); got != "unknown" {
		t.Errorf("Class(-1) = %q", got)
	}
	if got := Modifier(1 << 20).String(); got != "unknown" {
		t.Errorf("unknown modifier bit = %q", got)
	}
}

// Encoded positions and lengths are UTF-16 code units: a token after an
// astral-plane rune is not shifted by the rune's byte length.
func TestEncodeUsesUTF16Units(t *testing.T) {
	content := []byte("// \U0001F31F\npart w;\n")
	toks := Lexical(content)
	data := Encode(content, toks)
	if len(data)%5 != 0 {
		t.Fatalf("token data length = %d, want a multiple of 5", len(data))
	}
	// The comment is "// 🌟": two ASCII chars, a space and a surrogate pair —
	// five UTF-16 units starting at line 0 character 0.
	if data[0] != 0 || data[1] != 0 || data[2] != 5 || data[3] != uint32(ClassComment) {
		t.Errorf("comment token encodes %v, want delta 0:0 length 5 class comment", data[:5])
	}
	// The part keyword opens line 1 at character 0, four units long.
	if data[5] != 1 || data[6] != 0 || data[7] != 4 || data[8] != uint32(ClassKeyword) {
		t.Errorf("keyword token encodes %v, want delta 1:0 length 4 class keyword", data[5:10])
	}
}
