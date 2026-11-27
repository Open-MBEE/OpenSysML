package symbols

import "testing"

// A wildcard import whose first segment a named import binds re-exports the members
// of the namespace name lookup reads there, beside what other imports bring.
func TestWildcardImportBelowANamedImportHoldsWhatNameLookupReads(t *testing.T) {
	idx := buildIndex(t, map[string]string{"d.sysml": `
		package Q { package X { part def B; } }
		package U { part def D; }
		package R { package X { part def E; } }
		package P {
			package X { part def A; }
			package Own { public import R::*; package N { private import R::X; public import X::*; } }
			package Once { private import Q::X; private import Q::*; package N { public import X::*; } }
			package Beside { private import R::X; public import X::*; public import U::*; }
			package Apart { private import Q::X; public import R::*; package N { public import X::*; } }
		}`})
	for fqn, want := range map[string]int{
		"P::Own::N::E":   1,
		"P::Own::N::A":   0,
		"P::Once::N::B":  1,
		"P::Once::N::A":  0,
		"P::Beside::E":   1,
		"P::Beside::D":   1,
		"P::Beside::A":   0,
		"P::Apart::N::A": 1, // what a named and a wildcard import bring apart hides each other
		"P::Apart::N::B": 0,
		"P::Apart::N::E": 0,
	} {
		wantNamed(t, idx, fqn, want)
	}
}

// The namespace such an import is bound to follows edits of what the named import names.
func TestWildcardImportBelowANamedImportFollowsEdits(t *testing.T) {
	const p = "package P { package X { part def A; } package K { private import Q::X; package N { public import X::*; } } }"
	const bearing, bare, apart = "package Q { package X { part def B; } }", "package Q { }", "package Q { package X { part def C; } }"
	live := map[string]string{"q.sysml": bearing, "p.sysml": p}
	idx := NewIndex()
	for _, name := range sortedNames(live) {
		addDoc(t, idx, name, live[name])
	}
	idx.ExpandWildcardImports()
	wantNamed(t, idx, "P::K::N::B", 1)
	for stage, edit := range []struct {
		q       string
		a, b, c int
	}{{bare, 1, 0, 0}, {apart, 0, 0, 1}, {bearing, 0, 1, 0}} {
		idx.RemoveDocument("q.sysml")
		live["q.sysml"] = edit.q
		addDoc(t, idx, "q.sysml", edit.q)
		idx.ExpandWildcardImports()
		wantNamed(t, idx, "P::K::N::A", edit.a)
		wantNamed(t, idx, "P::K::N::B", edit.b)
		wantNamed(t, idx, "P::K::N::C", edit.c)
		if !holdsImportStateOfAFreshBuild(t, idx, live) {
			t.Errorf("edit %d: the index differs from one built afresh", stage)
		}
	}
}
