package grpc

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

// TestModelKeyHashesTheDelimitedFields pins the key to the SHA-256 of the
// fields joined with their lengths: a hash a client holds must keep resolving.
func TestModelKeyHashesTheDelimitedFields(t *testing.T) {
	content := strings.Repeat("part def Vehicle;\n", 1000)
	fields := []string{"model.sysml", "sysml", content, ""}

	var joined strings.Builder
	fmt.Fprintf(&joined, "%s\x00%d", "strict", 1)
	for _, field := range fields {
		fmt.Fprintf(&joined, "\x00%d\x00%s", len(field), field)
	}
	want := fmt.Sprintf("%x", sha256.Sum256([]byte(joined.String())))

	key := newModelKey()
	key.format("%s\x00%d", "strict", 1)
	for _, field := range fields {
		key.field(field)
	}
	if got := key.sum(); got != want {
		t.Fatalf("key = %s, want %s", got, want)
	}
}

func BenchmarkModelKey(b *testing.B) {
	content := strings.Repeat("part def Vehicle;\n", 60000)
	b.ReportAllocs()
	b.SetBytes(int64(len(content)))
	for b.Loop() {
		key := newModelKey()
		key.format("%s\x00%d", "strict", 1)
		key.field("model.sysml")
		key.field("sysml")
		key.field(content)
		_ = key.sum()
	}
}
