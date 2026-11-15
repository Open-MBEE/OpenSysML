package resolve

import (
	"fmt"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"testing"
)

// A large same-name group needs one witness per member, not every pair.
func TestImportedCollisionWitnessAllocations(t *testing.T) {
	const n = 256
	members := make([]importedMember, n)
	for i := range members {
		members[i].sym = &symbols.Symbol{Name: "x", Kind: symbols.SymbolPartUsage}
	}
	r := New(symbols.NewIndex())
	allocs := testing.AllocsPerRun(5, func() {
		c := &importCollisions{names: []string{"x"}, byName: map[string][]importedMember{"x": members}, hidden: map[boundName]bool{}, collided: map[string][]importedMember{}}
		r.hideImportCollisions(c)
		if len(c.collided["x"]) != n {
			t.Fatalf("hidden %d members, want %d", len(c.collided["x"]), n)
		}
	})
	if allocs > n*4 {
		t.Fatalf("%.0f allocations for %d members; want at most %d", allocs, n, n*4)
	}
}

func TestImportedCollisionWitnessIdentity(t *testing.T) {
	r := resolveDoc(t, "witness.sysml", `package A { part x; part y; alias otherX for x; }`)
	x := r.idx.LookupQualified("A::x")[0]
	y := r.idx.LookupQualified("A::y")[0]
	alias := r.idx.LookupQualified("A::otherX")[0]
	for _, tc := range []struct {
		name    string
		symbols []*symbols.Symbol
		want    bool
	}{
		{"self", []*symbols.Symbol{x}, false},
		{"same element", []*symbols.Symbol{x, x}, false},
		{"alias of same element", []*symbols.Symbol{x, alias}, false},
		{"distinct", []*symbols.Symbol{x, y}, true},
		{"later witness", []*symbols.Symbol{x, alias, x, y}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			members := make([]importedMember, len(tc.symbols))
			for i, sym := range tc.symbols {
				members[i].sym = sym
			}
			if got := r.hasImportedDuplicate(members, 0); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func BenchmarkHideImportCollisions(b *testing.B) {
	for _, n := range []int{256, 512, 1024, 2048} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			members := make([]importedMember, n)
			for i := range members {
				members[i].sym = &symbols.Symbol{Name: "x", Kind: symbols.SymbolPartUsage}
			}
			r := New(symbols.NewIndex())
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				c := &importCollisions{names: []string{"x"}, byName: map[string][]importedMember{"x": members}, hidden: map[boundName]bool{}, collided: map[string][]importedMember{}}
				r.hideImportCollisions(c)
				if len(c.collided["x"]) != n {
					b.Fatal("lost collisions")
				}
			}
		})
	}
}
