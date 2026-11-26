package symbols

import "testing"

// A wildcard import reads the namespace its target resolves to: the one its own
// or an enclosing namespace imports by name, not a same-named one owned further
// out, whose members it must not surface as well (KerML 8.2.3.5.4).
func TestWildcardImportTargetFollowsAMembershipImport(t *testing.T) {
	const q = "package Q { package X { part def B; package Y { part def C; } } }"
	const r = "package R { package X { part def E; } }"
	idx := buildIndex(t, map[string]string{"q.sysml": q, "r.sysml": r, "p.sysml": `
		package P {
			package X { part def A; package Y { part def D; } }
			package K { private import Q::X; package N { public import X::*; } }
			package Own { private import Q::X; public import X::*; }
			package Deep { private import Q::X; package DeepIn { public import X::Y::*; } }
			package Rooted { private import $::Q::X; package RootedIn { public import X::*; } }
			package Self { private import X::X; public import X::*; }
			package Missing { private import Q::Absent::X; public import X::*; }
			package Hidden { private import Q::X; private import R::X; package HiddenIn { public import X::*; } }
			package Twice { private import Q::X; private import $::Q::X; package TwiceIn { public import X::*; } }
			package None { package NoneIn { public import X::*; } }
		}`})

	for fqn, want := range map[string]int{
		"P::K::N::B":             1,
		"P::K::N::A":             0,
		"P::Own::B":              1,
		"P::Own::A":              0,
		"P::Deep::DeepIn::C":     1,
		"P::Deep::DeepIn::D":     0,
		"P::Rooted::RootedIn::B": 1,
		"P::Rooted::RootedIn::A": 0,
		"P::Self::A":             1,
		"P::Missing::A":          1,
		"P::Hidden::HiddenIn::A": 1,
		"P::Hidden::HiddenIn::B": 0,
		"P::Hidden::HiddenIn::E": 0,
		"P::Twice::TwiceIn::B":   1,
		"P::Twice::TwiceIn::A":   0,
		"P::None::NoneIn::A":     1,
		"P::None::NoneIn::B":     0,
	} {
		wantNamed(t, idx, fqn, want)
	}

	// Without the membership import the target is the member owned further out again.
	idx.RemoveDocument("p.sysml")
	addDoc(t, idx, "p.sysml", `
		package P {
			package X { part def A; }
			package K { package N { public import X::*; } }
		}`)
	idx.ExpandWildcardImports()
	wantNamed(t, idx, "P::K::N::A", 1)
	wantNamed(t, idx, "P::K::N::B", 0)
}
