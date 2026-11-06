package runtime

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
)

// referenceUnion is the sorted union without duplicates, element by element.
func referenceUnion(a, b []int64) []int64 {
	out := slices.Clone(a)
	for _, s := range b {
		if i, found := slices.BinarySearch(out, s); !found {
			out = slices.Insert(out, i, s)
		}
	}
	return out
}

// randomSerials is a sorted, duplicate-free set of up to n serials drawn below limit.
func randomSerials(r *rand.Rand, n int, limit int64) []int64 {
	out := make([]int64, 0, n)
	for range r.IntN(n + 1) {
		out = append(out, 1+r.Int64N(limit))
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func TestUnionSerialsMatchesReference(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for i := 0; i < 20000; i++ {
		a := randomSerials(r, 12, 40)
		b := randomSerials(r, 12, 40)
		if i%3 == 0 { // ranges apart, concatenated or appended
			for j := range b {
				b[j] += 40
			}
		}
		if i%7 == 0 { // one contains the other
			b = slices.Clone(a[:len(a)/2])
		}
		want := referenceUnion(a, b)
		aBefore, bBefore := slices.Clone(a), slices.Clone(b)
		got := unionSerials(a, b)
		if !slices.Equal(got, want) {
			t.Fatalf("unionSerials(%v, %v) = %v, want %v", a, b, got, want)
		}
		if !slices.Equal(a, aBefore) || !slices.Equal(b, bBefore) {
			t.Fatalf("unionSerials(%v, %v) wrote an operand: now %v, %v", aBefore, bBefore, a, b)
		}
		if len(want) == len(a) && !sameSerials(got, a) {
			t.Fatalf("unionSerials(%v, %v) copied a, which is the union", a, b)
		}
		if len(want) == len(b) && len(want) != len(a) && !sameSerials(got, b) {
			t.Fatalf("unionSerials(%v, %v) copied b, which is the union", a, b)
		}
	}
}

// Two sets that are one slice each grow by a serial of their own: the first takes
// the slot in place, the second gets a fresh array, and neither sees the other's.
func TestAppendSerialKeepsSharedSetsApart(t *testing.T) {
	base := unionSerials([]int64{1, 2}, []int64{3})
	if cap(base) == len(base) {
		t.Fatalf("appended set has no room to grow in place: len %d cap %d", len(base), cap(base))
	}
	shared := base
	grownA := appendSerial(base, 7)
	grownB := appendSerial(shared, 9)
	if !slices.Equal(grownA, []int64{1, 2, 3, 7}) || !slices.Equal(grownB, []int64{1, 2, 3, 9}) {
		t.Fatalf("grown sets %v and %v", grownA, grownB)
	}
	if !slices.Equal(base, []int64{1, 2, 3}) {
		t.Fatalf("the shared set changed: %v", base)
	}
	if &grownA[0] != &base[0] {
		t.Fatalf("the first growth of a set with room did not take place in its array")
	}
	if &grownB[0] == &base[0] {
		t.Fatalf("the second growth of a shared set took a slot already claimed")
	}
	again := appendSerial(grownA, 11)
	if !slices.Equal(again, []int64{1, 2, 3, 7, 11}) || !slices.Equal(grownA, []int64{1, 2, 3, 7}) {
		t.Fatalf("growing the grown set: %v, leaving %v", again, grownA)
	}
}

// A seed joins its serial to places that stand on one set by one union, and the
// places then share the joined set.
func TestSeedSharesJoinedSet(t *testing.T) {
	p := func(name string) lower.Place { return lower.Place{Name: name} }
	alike := func(a, b lower.Place) bool { return true }
	m := orderMarks{}
	by := []int64{1, 2}
	m.rewrite(p("x"), by, 0, alike)
	m.rewrite(p("y"), by, 0, alike)
	m.seed(ChoicePoint{serial: 5, touched: []lower.Place{p("x"), p("y"), p("z")}}, 1)
	x, y, z := m.at(p("x"), alike), m.at(p("y"), alike), m.at(p("z"), alike)
	if !slices.Equal(x, []int64{1, 2, 5}) || !slices.Equal(y, []int64{1, 2, 5}) || !slices.Equal(z, []int64{5}) {
		t.Fatalf("after the seed: x %v y %v z %v", x, y, z)
	}
	if !sameSerials(x, y) {
		t.Fatalf("x and y stood on one set and now stand on two")
	}
	if !slices.Equal(by, []int64{1, 2}) {
		t.Fatalf("the set written to x and y changed: %v", by)
	}
}
