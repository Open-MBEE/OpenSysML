package repl

import (
	"testing"
	"unicode/utf16"
)

// A diagnostic's UTF-16 column is counted in place: the same count utf16.Encode
// gives for ASCII, multi-byte, astral and invalid bytes, without allocating.
func TestUTF16ColumnCountsWithoutEncoding(t *testing.T) {
	for _, line := range []string{"attribute x;", "part ü : Ü;", "part 𝒳 : 𝒴;", "part \xff : A;"} {
		content := []byte(line + "\n")
		offset := len(line) - 1
		want := len(utf16.Encode([]rune(line[:offset]))) + 1
		if got := utf16Column(content, offset, offset+1); got != want {
			t.Errorf("utf16Column(%q) = %d, want %d", line, got, want)
		}
		if got := testing.AllocsPerRun(20, func() { utf16Column(content, offset, offset+1) }); got != 0 {
			t.Errorf("utf16Column(%q) allocates %.0f times, want 0", line, got)
		}
	}
}
