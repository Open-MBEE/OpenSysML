package lsp

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
)

func partialChange(line, from, to uint32, text string) protocol.TextDocumentContentChangeEvent {
	return &protocol.TextDocumentContentChangePartial{
		Range: protocol.Range{
			Start: protocol.Position{Line: line, Character: from},
			End:   protocol.Position{Line: line, Character: to},
		},
		Text: text,
	}
}

// TestContentChangesResolveAgainstTheEditedText applies a batch whose later
// ranges only make sense in the text the earlier changes produced.
func TestContentChangesResolveAgainstTheEditedText(t *testing.T) {
	changes := []protocol.TextDocumentContentChangeEvent{
		partialChange(0, 4, 4, "\npart b;\n"),
		partialChange(1, 5, 6, "c"),
		partialChange(2, 0, 0, "é"),
		&protocol.TextDocumentContentChangeWholeDocument{Text: "x\ny"},
		partialChange(1, 1, 1, "z"),
	}
	got := applyContentChanges([]byte("part a;\nend"), changes)
	if want := "x\nyz"; string(got) != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
	got = applyContentChanges([]byte("part a;\nend"), changes[:3])
	if want := "part\npart c;\né a;\nend"; string(got) != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
}

// BenchmarkContentChanges applies a batch of single-character edits, each on
// its own line, to a document of the same number of lines.
func BenchmarkContentChanges(b *testing.B) {
	for _, count := range []int{100, 1000} {
		content := []byte(strings.Repeat("part def Vehicle { attribute mass; }\n", count))
		changes := make([]protocol.TextDocumentContentChangeEvent, count)
		for i := range changes {
			changes[i] = partialChange(uint32(i), 0, 0, "x") // #nosec G115 -- a small loop index
		}
		b.Run(fmt.Sprintf("changes=%d", count), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if out := applyContentChanges(bytes.Clone(content), changes); len(out) != len(content)+count {
					b.Fatalf("len = %d", len(out))
				}
			}
		})
	}
}
