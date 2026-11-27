package symbols

import (
	"fmt"
	"strings"
	"testing"
)

// A wildcard import beside one left to name lookup is bound in the index as name lookup
// reads it: through what the other brings when that bears its first segment.
func TestWildcardImportBesideOneLeftToNameLookup(t *testing.T) {
	// Every import of Lattice names a package only another of its imports names.
	var lattice strings.Builder
	for depth, from := 0, []string{"X"}; depth < 10; depth++ {
		var to []string
		for i := range 6 {
			to = append(to, fmt.Sprintf("L%d_%d", depth, i))
		}
		for _, name := range from {
			for _, qualifier := range to {
				fmt.Fprintf(&lattice, "private import %s::%s; ", qualifier, name)
			}
		}
		from = to
	}
	idx := buildIndex(t, map[string]string{"d.sysml": `
		package Q { part def B; }
		package X { }
		package R { package X { part def E; } package Has { package Q { part def Z; } } }
		package Names { public import R::X; }
		package Brings { public import R::Has::*; }
		package P {
			package Named { private import R::X; public import X::*; public import Q::*; }
			package Alias { private alias Z for R::X; public import Z::*; public import Q::*; }
			package Chain { private import Names::X; public import X::*; public import Q::*; }
			package Twice { private import R::X; private import Names::X; public import X::*; public import Q::*; }
			package Above { private import R::X; public import X::*; package In { public import Q::*; } }
			package Outer { private import R::X; package Mid { public import X::*; package In { public import Q::*; } } }
			package Own { private import R::Has; public import Has::*; public import Q::*; }
			package Carried { private import $::Brings; public import Brings::*; public import Q::*; }
			package Path { private import $::R; private import R::Has::*; public import Q::*; }
			package Lattice { ` + lattice.String() + ` public import X::*; public import Q::*; }
			package Below { private import R::Has; public import Has::*; package In { public import Q::*; } }
		}`})
	for fqn, want := range map[string]int{
		"P::Named::B":          1,
		"P::Alias::B":          1,
		"P::Chain::B":          1,
		"P::Twice::B":          1, // neither X the two imports may name has a Q
		"P::Above::In::B":      1,
		"P::Outer::Mid::In::B": 1,
		"P::Own::B":            1, // name lookup reads no Q through a Has only an import binds
		"P::Below::In::B":      1,
		"P::Lattice::B":        1, // each import of the lattice is read once
		"P::Carried::B":        0, // Brings carries a Q of its own, which name lookup reads
		"P::Carried::Z":        1,
		"P::Path::B":           0,
		"P::Path::Z":           1,
	} {
		wantNamed(t, idx, fqn, want)
	}
}

// Whether the import left to name lookup brings the other's first segment follows edits.
func TestWildcardImportBesideOneLeftToNameLookupFollowsEdits(t *testing.T) {
	const p = "package P { private import $::R; private import R::X::*; public import Q::*; }"
	const plain, bearing = "package R { package X { part def E; } }", "package R { package X { part def E; package Q { part def Z; } } }"
	live := map[string]string{"q.sysml": "package Q { part def B; }", "r.sysml": plain, "p.sysml": p}
	idx := NewIndex()
	for _, name := range sortedNames(live) {
		addDoc(t, idx, name, live[name])
	}
	idx.ExpandWildcardImports()
	wantNamed(t, idx, "P::B", 1)
	for stage, edit := range []struct {
		r    string
		want int
	}{{bearing, 0}, {plain, 1}, {bearing, 0}} {
		idx.RemoveDocument("r.sysml")
		live["r.sysml"] = edit.r
		addDoc(t, idx, "r.sysml", edit.r)
		idx.ExpandWildcardImports()
		wantNamed(t, idx, "P::B", edit.want)
		wantNamed(t, idx, "P::Z", 1-edit.want)
		if !holdsImportStateOfAFreshBuild(t, idx, live) {
			t.Errorf("edit %d: the index differs from one built afresh", stage)
		}
	}
}
