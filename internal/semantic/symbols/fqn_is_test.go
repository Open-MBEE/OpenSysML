package symbols

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// FQNIs answers as comparing FQNOf would, for a nested and a root element, and
// allocates nothing doing so.
func TestFQNIsMatchesFQNOf(t *testing.T) {
	root := parser.New(source.New("d.sysml", []byte(`package Outer {
		package Mid { part def Deep; }
		part def Top;
	}`))).ParseFile()
	idx := NewIndexFromDoc("d.sysml", root)
	cases := []struct {
		fqn, parent, leaf string
		want              bool
	}{
		{"Outer::Mid::Deep", "Outer::Mid", "Deep", true},
		{"Outer::Mid::Deep", "Outer", "Deep", false},
		{"Outer::Mid::Deep", "Mid", "Deep", false},
		{"Outer::Mid::Deep", "Outer::Mid", "Other", false},
		{"Outer::Top", "Outer", "Top", true},
		{"Outer::Top", "", "Top", false},
		{"Outer", "", "Outer", true},
	}
	for _, c := range cases {
		syms := idx.LookupQualified(c.fqn)
		if len(syms) != 1 {
			t.Fatalf("%s: %d symbols, want 1", c.fqn, len(syms))
		}
		sym := syms[0]
		if got := FQNIs(sym, c.parent, c.leaf); got != c.want {
			t.Errorf("FQNIs(%s, %q, %q) = %v, want %v (FQNOf = %q)", c.fqn, c.parent, c.leaf, got, c.want, FQNOf(sym))
		}
		if c.want && FQNOf(sym) != c.fqn {
			t.Errorf("FQNOf(%s) = %q", c.fqn, FQNOf(sym))
		}
		if allocs := testing.AllocsPerRun(20, func() { FQNIs(sym, c.parent, c.leaf) }); allocs != 0 {
			t.Errorf("FQNIs(%s, %q, %q) allocates %.0f objects, want 0", c.fqn, c.parent, c.leaf, allocs)
		}
	}
}

// EachReexportGate visits the routes ReexportGates returns, in order, stops at
// the first the visit accepts, and allocates nothing for the walk.
func TestEachReexportGateWalksReexportGates(t *testing.T) {
	root := parser.New(source.New("d.sysml", []byte(`package L { part def A; part def B; }
	package Q { public import L::*; }
	package R { public import Q::*; }`))).ParseFile()
	idx := NewIndexFromDoc("d.sysml", root)
	idx.ExpandWildcardImports()
	a := idx.LookupQualified("L::A")[0]
	for _, fqn := range []string{"Q::A", "R::A", "L::A"} {
		want := idx.ReexportGates("d.sysml", fqn, a, "")
		var got [][]ElementFilter
		routed, accepted := idx.EachReexportGate("d.sysml", fqn, a, "", func(f []ElementFilter) bool {
			got = append(got, f)
			return false
		})
		if routed != (len(want) > 0) || accepted || len(got) != len(want) {
			t.Errorf("%s: EachReexportGate visited %d routes (routed %v, accepted %v), ReexportGates has %d", fqn, len(got), routed, accepted, len(want))
		}
		if len(want) > 0 {
			visits := 0
			_, accepted := idx.EachReexportGate("d.sysml", fqn, a, "", func([]ElementFilter) bool { visits++; return true })
			if !accepted || visits != 1 {
				t.Errorf("%s: an accepting visit was called %d times (accepted %v), want once", fqn, visits, accepted)
			}
		}
		if allocs := testing.AllocsPerRun(20, func() {
			idx.EachReexportGate("d.sysml", fqn, a, "", func([]ElementFilter) bool { return false })
		}); allocs != 0 {
			t.Errorf("%s: EachReexportGate allocates %.0f objects, want 0", fqn, allocs)
		}
	}
}
