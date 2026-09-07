package migrate

import (
	"runtime"
	"testing"
)

func TestWriterBlocks(t *testing.T) {
	w := &writer{}
	w.block("part def A", func() {
		w.line("attribute x;")
		w.block("part p : B", func() {})
	})
	w.block("part def B", func() {})
	want := "part def A {\n    attribute x;\n    part p : B;\n}\npart def B;\n"
	if got := w.String(); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

// Sibling blocks must not copy the output written before them: the bytes
// allocated stay within a small factor of the output.
func TestWriterSiblingBlocksAreLinear(t *testing.T) {
	const n = 20000
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	w := &writer{}
	for i := 0; i < n; i++ {
		w.block("part def X", func() { w.line("attribute a;") })
	}
	out := w.String()
	runtime.ReadMemStats(&after)
	allocated := after.TotalAlloc - before.TotalAlloc
	if allocated > uint64(len(out))*16 {
		t.Errorf("allocated %d bytes for %d bytes of output; a quadratic writer copies far more", allocated, len(out))
	}
}

func BenchmarkWriterSiblingBlocks(b *testing.B) {
	for range b.N {
		w := &writer{}
		for i := 0; i < 20000; i++ {
			w.block("part def X", func() { w.line("attribute a;") })
		}
	}
}
