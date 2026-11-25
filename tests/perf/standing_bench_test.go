package perf

import (
	"fmt"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// wideFootprint is a move of n statements, each writing its own feature from a
// read of its own, plus a guard's read: the shape of a transition assigning many attributes.
func wideFootprint(n int) lower.Footprint {
	var fp lower.Footprint
	for i := range n {
		r := lower.Place{Name: fmt.Sprintf("r%d", i), Sym: &symbols.Symbol{}, Direct: true}
		w := lower.Place{Name: fmt.Sprintf("w%d", i), Sym: &symbols.Symbol{}, Direct: true}
		fp.Reads, fp.Writes = append(fp.Reads, r), append(fp.Writes, w)
		fp.Flows = append(fp.Flows, lower.Flow{Reads: []lower.Place{r}, Writes: []lower.Place{w}})
	}
	fp.Reads = append(fp.Reads, lower.Place{Name: "guard"})
	return fp
}

// TestStandingDecidesOnlyConflictingPlaces pins that a place is tried only against
// the places it may conflict with, so two wide moves cost their width, not its square.
func TestStandingDecidesOnlyConflictingPlaces(t *testing.T) {
	for _, n := range []int{10, 40, 160} {
		fp := wideFootprint(n)
		s := fp.Standing()
		tried := 0
		meet := func(p, q lower.Place) bool { tried++; return p.Conflicts(q) }
		for _, w := range fp.Writes {
			if _, reading := s.WritesReading(w, meet); !s.MeetsWrite(w, meet) || s.MeetsRead(w, meet) || reading {
				t.Errorf("n=%d: write %s meets its own write only", n, w.Name)
			}
		}
		for _, r := range fp.Reads {
			writes, ok := s.WritesReading(r, meet)
			if !ok || (r.Name != "guard" && len(writes) != 1) || (r.Name == "guard" && len(writes) != n) {
				t.Errorf("n=%d: reading %s gives %d writes, %t", n, r.Name, len(writes), ok)
			}
		}
		if want := len(fp.Writes) + len(fp.Reads); tried != want {
			t.Errorf("n=%d: deciding %d places tried meet %d times, want %d (each its own conflicting place)", n, want, tried, want)
		}
	}
}

// BenchmarkStandingWritesReading decides every write of one wide move against
// another of the same shape, as a due-order draw does for each pair of executors.
func BenchmarkStandingWritesReading(b *testing.B) {
	for _, n := range []int{10, 40, 160} {
		b.Run(fmt.Sprintf("writes=%d", n), func(b *testing.B) {
			f, g := wideFootprint(n), wideFootprint(n)
			s := g.Standing()
			meet := func(p, q lower.Place) bool { return p.Conflicts(q) }
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				for _, w := range f.Writes {
					s.MeetsWrite(w, meet)
					s.WritesReading(w, meet)
				}
			}
		})
	}
}
