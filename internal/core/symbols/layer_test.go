package symbols

import (
	"slices"
	"testing"
)

// A mark an overlay adds or clears must not reach the frozen layer below it, and
// clearing the last mark under a name must forget the name.
func TestMarksOverAFrozenLayerLeaveItUntouched(t *testing.T) {
	a, b, c := &Symbol{Name: "a"}, &Symbol{Name: "b"}, &Symbol{Name: "c"}
	base := newLayer[string, symbolSet](&indexGeneration{})
	setMark(base, "P::x", a)
	setMark(base, "P::x", b)
	setMark(base, "P::y", c)

	over := overLayer(base, &indexGeneration{})
	setMark(over, "P::x", c)
	setMark(over, "P::x", c) // already marked: no change
	clearMark(over, "P::x", a)
	clearMark(over, "P::y", c)
	clearMark(over, "P::z", c) // never marked: no change

	if got := base.at("P::x"); !slices.Equal(got, symbolSet{a, b}) {
		t.Fatalf("base P::x changed under the overlay: %v", got)
	}
	if got := base.at("P::y"); !slices.Equal(got, symbolSet{c}) {
		t.Fatalf("base P::y changed under the overlay: %v", got)
	}
	if got := over.at("P::x"); !slices.Equal(got, symbolSet{b, c}) {
		t.Fatalf("overlay P::x = %v, want [b c]", got)
	}
	if _, ok := over.get("P::y"); ok {
		t.Fatal("overlay still holds P::y after its only mark was cleared")
	}
	if _, ok := over.get("P::z"); ok {
		t.Fatal("clearing an absent mark created an entry")
	}
}

// insertSorted keeps each layer's slice sorted and duplicate-free, copying the
// frozen layer's slice before the first insertion rather than growing it in place.
func TestInsertSortedCopiesTheFrozenSliceOnce(t *testing.T) {
	base := newLayer[string, []string](&indexGeneration{})
	for _, s := range []string{"P::m", "P::c", "P::x", "P::c"} {
		insertSorted(base, "P", s)
	}
	if got := base.at("P"); !slices.Equal(got, []string{"P::c", "P::m", "P::x"}) {
		t.Fatalf("base P = %v", got)
	}

	over := overLayer(base, &indexGeneration{})
	insertSorted(over, "P", "P::m") // present below: nothing to own
	if over.owns("P") {
		t.Fatal("inserting a present entry copied the slice")
	}
	insertSorted(over, "P", "P::a")
	insertSorted(over, "P", "P::z")
	insertSorted(over, "P", "P::a")

	if got := base.at("P"); !slices.Equal(got, []string{"P::c", "P::m", "P::x"}) {
		t.Fatalf("base P changed under the overlay: %v", got)
	}
	if got := over.at("P"); !slices.Equal(got, []string{"P::a", "P::c", "P::m", "P::x", "P::z"}) {
		t.Fatalf("overlay P = %v", got)
	}
}

func TestLastSeparatorMatchesStringsLastIndex(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want int
	}{
		{"", -1}, {":", -1}, {"::", 0}, {"A", -1}, {"A::B", 1}, {"A::B::C", 4},
		{"A:::B", 2}, {"'Z::First'::Engine", 10}, {"A::", 1},
	} {
		if got := lastSeparator(tc.in); got != tc.want {
			t.Errorf("lastSeparator(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
