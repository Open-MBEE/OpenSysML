package runtime

import "testing"

// A sequence a run appends to one element at a time (a loop's `body`, one object per
// iteration) grows into spare storage: the backing array is replaced a logarithmic
// number of times, never once per element.
func TestSequenceAppendedGrowsAmortized(t *testing.T) {
	const n = 10000
	seq := NewSequence()
	arrays, last := 0, (*Value)(nil)
	for i := 1; i <= n; i++ {
		seq = seq.appended(Value{Kind: ValInstance, Instance: int64(i)})
		if first := &seq.elements[0]; first != last {
			arrays, last = arrays+1, first
		}
	}
	if seq.Size() != n {
		t.Fatalf("Size = %d after %d appends, want %d", seq.Size(), n, n)
	}
	if arrays > 64 {
		t.Errorf("%d appends replaced the backing array %d times, want at most 64 (amortized growth)", n, arrays)
	}
	for i := 1; i <= n; i++ {
		if elem, _ := seq.At(i - 1); elem.Instance != int64(i) {
			t.Fatalf("element %d = %v, want object %d", i, elem, i)
		}
	}
}

// The sequence appended to keeps its length and loses its spare capacity, so the one a
// rollback restores and the one the run went on with never alias an element.
func TestSequenceAppendedLeavesTheSupersededUnaliased(t *testing.T) {
	object := func(id int64) Value { return Value{Kind: ValInstance, Instance: id} }
	base := NewSequence().appended(object(1))
	forward := base.appended(object(2))
	restored := base.appended(object(3))
	if base.Size() != 1 {
		t.Fatalf("base.Size = %d after being appended to twice, want 1", base.Size())
	}
	if elem, _ := forward.At(1); elem.Instance != 2 {
		t.Errorf("forward#(2) = %v, want object 2 (aliased by the append after rollback)", elem)
	}
	if elem, _ := restored.At(1); elem.Instance != 3 {
		t.Errorf("restored#(2) = %v, want object 3", elem)
	}
}
