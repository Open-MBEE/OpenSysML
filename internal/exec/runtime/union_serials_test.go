package runtime

import (
	"math/rand"
	"slices"
	"testing"
)

// TestUnionSerials checks the merge against a set union on random sorted inputs,
// and that it neither writes into its arguments nor copies when b adds nothing.
func TestUnionSerials(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	sorted := func(n, span int) []int64 {
		set := map[int64]bool{}
		for len(set) < n {
			set[int64(rng.Intn(span))] = true
		}
		out := make([]int64, 0, n)
		for s := range set {
			out = append(out, s)
		}
		slices.Sort(out)
		return out
	}
	for i := 0; i < 2000; i++ {
		a := sorted(rng.Intn(6), 12)
		b := sorted(rng.Intn(6), 12)
		if rng.Intn(3) == 0 && len(a) > 0 {
			b = slices.Clone(a[:rng.Intn(len(a)+1)])
		}
		wantSet := map[int64]bool{}
		for _, s := range a {
			wantSet[s] = true
		}
		for _, s := range b {
			wantSet[s] = true
		}
		want := make([]int64, 0, len(wantSet))
		for s := range wantSet {
			want = append(want, s)
		}
		slices.Sort(want)
		a0, b0 := slices.Clone(a), slices.Clone(b)
		got := unionSerials(a, b)
		if !slices.Equal(got, want) {
			t.Fatalf("unionSerials(%v, %v) = %v, want %v", a0, b0, got, want)
		}
		if !slices.Equal(a, a0) || !slices.Equal(b, b0) {
			t.Fatalf("unionSerials(%v, %v) wrote into an argument: %v %v", a0, b0, a, b)
		}
		if len(a) == 0 {
			continue
		}
		if aliases, addsNothing := &got[0] == &a[0], len(want) == len(a); aliases != addsNothing {
			t.Fatalf("unionSerials(%v, %v) copied = %v, want copying only when b adds", a0, b0, !aliases)
		}
	}
}
