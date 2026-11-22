package source

import (
	"math/rand/v2"
	"slices"
	"testing"
)

// TestSpliceMatchesARebuiltIndex edits a text at random and holds the spliced
// index to the one built from the edited text.
func TestSpliceMatchesARebuiltIndex(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	pieces := []string{"", "\n", "a", "ab\n", "\n\n", "x\ny\nz", "é\n"}
	content := []byte("first\nsecond\n\nfourth")
	index := NewLineIndex(content)
	for step := range 2000 {
		start := rng.IntN(len(content) + 1)
		end := start + rng.IntN(len(content)-start+1)
		text := pieces[rng.IntN(len(pieces))]
		content = slices.Concat(content[:start], []byte(text), content[end:])
		index.Splice(content, start, end, len(text))
		if want := NewLineIndex(content); !slices.Equal(index.lineStarts, want.lineStarts) {
			t.Fatalf("step %d: replacing [%d,%d) by %q gives line starts %v, want %v", step, start, end, text, index.lineStarts, want.lineStarts)
		}
		if got := index.PosAt(len(content)); got != NewLineIndex(content).PosAt(len(content)) {
			t.Fatalf("step %d: end position %v", step, got)
		}
	}
}
