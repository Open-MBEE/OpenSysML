package migrate

import (
	"runtime"
	"strings"
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

// Each block marks the made-up names it declared as it closes, one marker line
// per body: a nested block's names never leak to the block enclosing it, a
// block marked early is not marked twice, and the root block is marked last.
func TestWriterMarksMadeUpNamesPerBlock(t *testing.T) {
	marker := func(names []string) string { return "metadata M about " + strings.Join(names, ", ") + ";" }
	w := &writer{marker: marker}
	w.block("part def A", func() {
		w.line("part 'fork';")
		w.madeUp("'fork'")
		w.block("action def B", func() {
			w.line("action final;")
			w.madeUp("final")
			w.line("action start;")
			w.madeUp("start")
		})
		w.block("part def C", func() { w.line("attribute x;") })
		w.line("part p;")
		w.madeUp("p")
	})
	w.block("part def D", func() {
		w.line("part q;")
		w.madeUp("q")
		w.markMadeUp(marker)
		w.line("part r;")
	})
	w.line("part top;")
	w.madeUp("top")
	want := "part def A {\n" +
		"    part 'fork';\n" +
		"    action def B {\n        action final;\n        action start;\n        metadata M about final, start;\n    }\n" +
		"    part def C {\n        attribute x;\n    }\n" +
		"    part p;\n" +
		"    metadata M about 'fork', p;\n}\n" +
		"part def D {\n    part q;\n    metadata M about q;\n    part r;\n}\n" +
		"part top;\nmetadata M about top;\n"
	if got := w.String(); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

// A block declaring no made-up names, and a writer with no marker, write none.
func TestWriterMarksNothingWithoutMadeUpNames(t *testing.T) {
	w := &writer{marker: func(names []string) string { return "metadata M about " + strings.Join(names, ", ") + ";" }}
	w.block("part def A", func() { w.line("attribute x;") })
	if got, want := w.String(), "part def A {\n    attribute x;\n}\n"; got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	unmarked := &writer{}
	unmarked.block("part def A", func() { unmarked.line("part p;"); unmarked.madeUp("p") })
	if got, want := unmarked.String(), "part def A {\n    part p;\n}\n"; got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}
