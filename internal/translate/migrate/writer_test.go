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

// A braced clause keeps its braces when its body is empty, so what follows it
// on the next line still belongs to the same statement.
func TestWriterBracedKeepsEmptyBody(t *testing.T) {
	w := &writer{}
	w.line("transition first S1")
	w.indented(func() {
		w.braced("do action effect", func() {})
		w.line("then S2;")
		w.braced("do action", func() { w.line("assign x := 1;") })
	})
	want := "transition first S1\n    do action effect { }\n    then S2;\n    do action {\n        assign x := 1;\n    }\n"
	if got := w.String(); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

// A trailed statement ends with its trailer when its body is empty, and opens a
// body led by the lead line otherwise.
func TestWriterTrailed(t *testing.T) {
	w := &writer{}
	w.trailed("dependency A to B", "; /* «Trace» */", "/* «Trace» */", func() {})
	w.trailed("dependency A to C", "; /* «Trace» */", "/* «Trace» */", func() { w.line("@X;") })
	want := "dependency A to B; /* «Trace» */\ndependency A to C {\n    /* «Trace» */\n    @X;\n}\n"
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
