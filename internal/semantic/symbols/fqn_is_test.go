package symbols

import (
	"reflect"
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

// EachReexportGate visits the routes ReexportGates returns, values and order
// alike, over public, private and filtered imports; it stops at the first accepted.
func TestEachReexportGateWalksReexportGates(t *testing.T) {
	root := parser.New(source.New("d.sysml", []byte(`package L { part def A; part def B; metadata def M; }
	package Q { public import L::*; }
	package R { public import Q::*; }
	package S { private import L::*; }
	package F { public import L::*[@M]; }`))).ParseFile()
	idx := NewIndexFromDoc("d.sysml", root)
	idx.ExpandWildcardImports()
	a := idx.LookupQualified("L::A")[0]
	cases := []struct{ fqn, from string }{
		{"Q::A", ""}, {"R::A", ""}, {"L::A", ""}, {"S::A", ""}, {"S::A", "S"}, {"F::A", ""}, {"F::A", "F"},
	}
	for _, c := range cases {
		want := idx.ReexportGates("d.sysml", c.fqn, a, c.from)
		switch {
		case c.fqn == "S::A" && (len(want) > 0) != (c.from == "S"):
			t.Errorf("S::A from %q has %d routes; a private route answers only from inside", c.from, len(want))
		case c.fqn == "F::A" && (len(want) == 0 || len(want[0]) == 0):
			t.Errorf("F::A has routes %v, want a filtered one", want)
		}
		var got [][]ElementFilter
		routed, accepted := idx.EachReexportGate("d.sysml", c.fqn, a, c.from, func(f []ElementFilter) bool {
			got = append(got, f)
			return false
		})
		if routed != (len(want) > 0) || accepted || !reflect.DeepEqual(got, want) {
			t.Errorf("%s from %q: EachReexportGate visited %v (routed %v, accepted %v), ReexportGates has %v", c.fqn, c.from, got, routed, accepted, want)
		}
		if len(want) > 0 {
			visits := 0
			_, accepted := idx.EachReexportGate("d.sysml", c.fqn, a, c.from, func([]ElementFilter) bool { visits++; return true })
			if !accepted || visits != 1 {
				t.Errorf("%s from %q: an accepting visit was called %d times (accepted %v), want once", c.fqn, c.from, visits, accepted)
			}
		}
		if allocs := testing.AllocsPerRun(20, func() {
			idx.EachReexportGate("d.sysml", c.fqn, a, c.from, func([]ElementFilter) bool { return false })
		}); allocs != 0 {
			t.Errorf("%s from %q: EachReexportGate allocates %.0f objects, want 0", c.fqn, c.from, allocs)
		}
	}
}
