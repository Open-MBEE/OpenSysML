package pack

import (
	"bytes"
	"errors"
	"math"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	w := NewWriter()
	w.Uint(0)
	w.Uint(127)
	w.Uint(128)
	w.Uint(math.MaxUint64)
	w.Int(0)
	w.Int(-1)
	w.Int(63)
	w.Int(-64)
	w.Int(math.MinInt64)
	w.Int(math.MaxInt64)
	w.Len(0)
	w.Len(3)
	w.Bool(true)
	w.Bool(false)
	w.Float(0)
	w.Float(-2.5)
	w.Float(math.Inf(1))
	w.String("")
	w.String("part")
	w.String("part")
	w.String("définition")

	r, err := NewReader(w.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []uint64{0, 127, 128, math.MaxUint64} {
		if got := r.Uint(); got != want {
			t.Errorf("Uint = %d, want %d", got, want)
		}
	}
	for _, want := range []int64{0, -1, 63, -64, math.MinInt64, math.MaxInt64} {
		if got := r.Int(); got != want {
			t.Errorf("Int = %d, want %d", got, want)
		}
	}
	for _, want := range []int{0, 3} {
		if got := r.Len(); got != want {
			t.Errorf("Len = %d, want %d", got, want)
		}
	}
	if !r.Bool() || r.Bool() {
		t.Error("Bool did not round-trip")
	}
	for _, want := range []float64{0, -2.5, math.Inf(1)} {
		if got := r.Float(); got != want {
			t.Errorf("Float = %v, want %v", got, want)
		}
	}
	for _, want := range []string{"", "part", "part", "définition"} {
		if got := r.String(); got != want {
			t.Errorf("String = %q, want %q", got, want)
		}
	}
	if !r.Done() {
		t.Errorf("not done: err %v", r.Err())
	}
	if len(r.table) != 3 {
		t.Errorf("string table has %d entries, want 3 (one per distinct string)", len(r.table))
	}
}

func TestSectionsAreIndependent(t *testing.T) {
	w := NewWriter()
	w.Section(func(w *Writer) { w.String("a"); w.Uint(1) })
	w.Section(func(w *Writer) { w.String("b"); w.String("a") })
	w.Uint(9)

	r, err := NewReader(w.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	first, second := r.Section(), r.Section()
	if got := r.Uint(); got != 9 || !r.Done() {
		t.Fatalf("after sections: %d, done %v, err %v", got, r.Done(), r.Err())
	}
	if second.String() != "b" || second.String() != "a" || !second.Done() {
		t.Error("second section did not decode on its own")
	}
	if first.String() != "a" || first.Uint() != 1 || !first.Done() {
		t.Error("first section did not decode on its own")
	}
	first.Uint()
	if first.Err() == nil || second.Err() != nil || r.Err() != nil {
		t.Error("an over-read of one section leaked into another")
	}
}

func TestStickyError(t *testing.T) {
	w := NewWriter()
	w.Uint(5)
	r, err := NewReader(w.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	r.Uint()
	r.Uint()
	first := r.Err()
	if !errors.Is(first, ErrCorrupt) {
		t.Fatalf("over-read: err = %v", first)
	}
	r.Fail("later")
	if r.Err() != first || r.Uint() != 0 || r.Done() {
		t.Error("the first error was not kept")
	}
}

// body is the stream lengths and streams of a body holding the given bytes.
func body(streams map[int][]byte) []byte {
	var out, all []byte
	for i := range numStreams {
		out = append(out, byte(len(streams[i])))
		all = append(all, streams[i]...)
	}
	return append(out, all...)
}

func TestMalformedTable(t *testing.T) {
	// A one-string table, then a body of that string and the number one.
	good := append([]byte{1, 3, 'a', 'b', 'c'}, body(map[int][]byte{streamString: {0}, streamUint: {1}})...)
	for i := 1; i < len(good); i++ {
		if _, err := NewReader(good[:i]); !errors.Is(err, ErrCorrupt) {
			t.Errorf("stream truncated to %d bytes: err = %v", i, err)
		}
	}
	if r, err := NewReader(good); err != nil || r.String() != "abc" || r.Uint() != 1 || !r.Done() {
		t.Errorf("the whole stream did not decode: %v", err)
	}
	// Three lengths whose sum exceeds the data; the third would not fit even
	// though each alone does.
	tooLong := append([]byte{3, 126, 126, 126}, make([]byte, 126)...)
	cases := map[string][]byte{
		"count past data":     {0x7f},
		"length past data":    {1, 0x7f},
		"lengths sum past":    {2, 2, 2, 'a', 'b', 'c'},
		"lengths pile up":     tooLong,
		"unterminated varint": {0x80},
		"no stream lengths":   {0},
		"stream past data":    append([]byte{0}, body(map[int][]byte{streamUint: {1}})[:numStreams]...),
	}
	for name, data := range cases {
		if _, err := NewReader(data); !errors.Is(err, ErrCorrupt) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if r, err := NewReader(append(append([]byte{0}, body(nil)...), 0)); err != nil || r.Done() {
		t.Errorf("a byte after the streams: err = %v, done %v", err, err == nil && r.Done())
	}
	corrupt := func(name string, streams map[int][]byte, read func(*Reader)) {
		t.Helper()
		r, err := NewReader(append([]byte{0}, body(streams)...))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if read(r); !errors.Is(r.Err(), ErrCorrupt) {
			t.Errorf("%s: err = %v", name, r.Err())
		}
	}
	corrupt("multi-byte varint without its tail", map[int][]byte{streamUint: {0x80}}, func(r *Reader) { r.Uint() })
	corrupt("unterminated varint tail", map[int][]byte{streamInt: {0x80}, streamIntTail: {0x80}}, func(r *Reader) { r.Int() })
	overflow := []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x02} // 2^57, one bit past a uint64's tail
	corrupt("varint past 64 bits", map[int][]byte{streamUint: {0x80}, streamUintTail: overflow}, func(r *Reader) { r.Uint() })
	corrupt("unterminated length", map[int][]byte{streamLen: {0x80}}, func(r *Reader) { r.Len() })
	corrupt("length past the bytes left", map[int][]byte{streamLen: {3}, streamBool: {1, 1}}, func(r *Reader) { r.Len() })
	corrupt("string index into an empty table", map[int][]byte{streamString: {0}}, func(r *Reader) { _ = r.String() })
	corrupt("bool past its stream", map[int][]byte{streamUint: {1}}, func(r *Reader) { r.Bool() })
	corrupt("float past its stream", map[int][]byte{streamFloat: {1, 2, 3, 4, 5, 6, 7}}, func(r *Reader) { r.Float() })
	// A section claiming two bools where the stream holds one, though three bytes are left overall.
	short := map[int][]byte{streamLen: {0, 2, 0, 0, 0, 0, 0, 0, 0}, streamBool: {1}, streamUint: {1, 1}}
	corrupt("section past its stream", short, func(r *Reader) {
		if sub := r.Section(); !errors.Is(sub.Err(), ErrCorrupt) {
			t.Errorf("section past its stream: the section's err = %v", sub.Err())
		}
	})
}

func TestStreamsKeepKindsApart(t *testing.T) {
	w := NewWriter()
	for i := range 4 {
		w.Uint(uint64(300 + i))
		w.Bool(i%2 == 0)
		w.Int(int64(-i))
		w.Len(i)
		w.Float(float64(i))
		w.String("s")
	}
	want := map[int]int{
		streamLen: 4, streamBool: 4, streamFloat: 32, streamUint: 4, streamUintTail: 4,
		streamInt: 4, streamIntTail: 0, streamString: 4, streamStringTail: 0,
	}
	for i, s := range w.streams {
		if len(s) != want[i] {
			t.Errorf("stream %d holds %d bytes, want %d", i, len(s), want[i])
		}
	}
	r, err := NewReader(w.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	for i := range 4 {
		if r.Uint() != uint64(300+i) || r.Bool() != (i%2 == 0) || r.Int() != int64(-i) ||
			r.Len() != i || r.Float() != float64(i) || r.String() != "s" {
			t.Fatalf("round %d did not decode", i)
		}
	}
	if !r.Done() {
		t.Errorf("not done: err %v", r.Err())
	}
}

func TestArena(t *testing.T) {
	var a Arena[int]
	if a.Take(0) != nil {
		t.Error("Take(0) is not nil")
	}
	x, y := a.Take(3), a.Take(2)
	if len(x) != 3 || cap(x) != 3 || len(y) != 2 || cap(y) != 2 {
		t.Errorf("shapes: %d/%d, %d/%d", len(x), cap(x), len(y), cap(y))
	}
	if x = append(x, 7); y[0] != 0 || x[3] != 7 {
		t.Error("appending to one slice wrote into its neighbour")
	}
	big := a.Take(arenaChunk)
	if len(big) != arenaChunk || cap(big) != arenaChunk {
		t.Error("a large request is not its own allocation")
	}
}

func TestDiscardKeepsNothing(t *testing.T) {
	w := NewDiscard()
	w.Uint(300)
	w.Int(-7)
	w.Len(3)
	w.Bool(true)
	w.Float(1.5)
	w.String("kept nowhere")
	ran := false
	w.Section(func(sub *Writer) {
		ran = true
		sub.String("nor here")
	})
	if !ran {
		t.Fatal("Section did not run its writer")
	}
	if got, empty := w.Bytes(), NewWriter().Bytes(); !bytes.Equal(got, empty) {
		t.Fatalf("a discarding writer produced %v, want the empty stream %v", got, empty)
	}
}
