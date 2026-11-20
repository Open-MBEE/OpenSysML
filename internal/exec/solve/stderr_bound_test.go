package solve

import (
	"bytes"
	"strings"
	"testing"
)

func TestSyncBufferKeepsNoMoreThanItsBound(t *testing.T) {
	var b syncBuffer
	chunk := bytes.Repeat([]byte("x"), 4<<10)
	for range 64 {
		n, err := b.Write(chunk)
		if n != len(chunk) || err != nil {
			t.Fatalf("Write = %d, %v; the solver must never see a short write", n, err)
		}
	}
	if kept := b.buf.Len(); kept != maxStderr {
		t.Errorf("kept %d bytes, want %d", kept, maxStderr)
	}
	if !strings.HasSuffix(b.String(), "(truncated)") {
		t.Errorf("the text does not say it was truncated")
	}
}

func TestSyncBufferKeepsShortOutputWhole(t *testing.T) {
	var b syncBuffer
	_, _ = b.Write([]byte("unknown option"))
	if got := b.String(); got != "unknown option" {
		t.Errorf("String = %q", got)
	}
}
