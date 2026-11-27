package symbols

import "testing"

// What the imports of a namespace bring apart under the first segment of a wildcard
// import hides each other (KerML 7.2.5.4): the segment is sought further out, as name
// lookup seeks it.
func TestWildcardImportPastImportsThatHideEachOther(t *testing.T) {
	idx := buildIndex(t, map[string]string{"m.sysml": `
		package Q { package X { part def B; } }
		package R { package X { part def C; } }
		package P {
			package X { part def A; package Y { part def AY; } }
			package Named { private import Q::X; private import R::X; package N { public import X::*; } }
			package Below { private import Q::X; private import R::X; package N { public import X::Y::*; } }
			package Wild { private import Q::X; public import R::*; package N { public import X::*; } }
			package Once { private import Q::X; private import Q::*; package N { public import X::*; } }
			package Alone { private import Q::X; package N { public import X::*; } }
		}`})
	idx.ExpandWildcardImports()
	for fqn, want := range map[string]int{
		"P::Named::N::A":  1,
		"P::Named::N::B":  0,
		"P::Named::N::C":  0,
		"P::Below::N::AY": 1,
		"P::Wild::N::A":   1,
		"P::Wild::N::B":   0,
		"P::Wild::N::C":   0,
		"P::Once::N::A":   0,
		"P::Alone::N::A":  0,
	} {
		if got := len(idx.LookupQualified(fqn)); got != want {
			t.Errorf("LookupQualified(%s) = %d symbols, want %d", fqn, got, want)
		}
	}
}
