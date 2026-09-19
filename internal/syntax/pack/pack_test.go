package pack

import (
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

func TestMalformedTable(t *testing.T) {
	// A one-string table, then a body of that string and the number one.
	good := []byte{1, 3, 'a', 'b', 'c', 0, 1}
	for i := 1; i < len(good)-2; i++ {
		if _, err := NewReader(good[:i]); !errors.Is(err, ErrCorrupt) {
			t.Errorf("table truncated to %d bytes: err = %v", i, err)
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
	}
	for name, data := range cases {
		if _, err := NewReader(data); !errors.Is(err, ErrCorrupt) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	r, err := NewReader([]byte{0, 0x80})
	if err != nil {
		t.Fatal(err)
	}
	if _ = r.Uint(); !errors.Is(r.Err(), ErrCorrupt) {
		t.Errorf("unterminated body varint: err = %v", r.Err())
	}
	r, _ = NewReader([]byte{0, 0})
	if _ = r.String(); !errors.Is(r.Err(), ErrCorrupt) {
		t.Errorf("string index into an empty table: err = %v", r.Err())
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
