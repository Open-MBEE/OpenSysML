package symbols

import (
	"fmt"
	"strings"
	"testing"
)

// A wildcard import whose target its own or an enclosing namespace imports by name
// is left to name lookup: the index re-exports no member for it (KerML 8.2.3.5.4).
func TestWildcardImportTargetBelowAMembershipImport(t *testing.T) {
	const q = "package Q { package X { part def B; package Y { part def C; } private package Kept { part def Z; } } }"
	const r = "package R { package X { part def E; } }"
	const m = `
		package M1 { public import Q::X; }
		package M2 { public import M1::X; }
		package Names { alias X for Q::X; }
		package Mixed { private import Q::X; public import R::X; }
		package Both { private import Q::X; public import Q::X; }
		package Hides { private import Q::X; }
		package Loop1 { public import Loop2::X; }
		package Loop2 { public import Loop1::X; }
		package W1 { public import Q::*; }
		package W2 { public import W1::*; public import W3::*; }
		package W3 { public import W2::*; }`
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
	idx := buildIndex(t, map[string]string{"q.sysml": q, "r.sysml": r, "m.sysml": m, "p.sysml": `
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
			package Repeated { ` + strings.Repeat("private import X; ", 12) + ` public import X::*; }
			package Above { ` + strings.Repeat("private import X; ", 12) + ` package AboveIn { public import X::*; } }
			package Lattice { ` + lattice.String() + ` public import X::*; }
			package Alias { private alias X for Q::X; package AliasIn { public import X::*; } }
			package Named { private import Names::X; package NamedIn { public import X::*; } }
			package Chain { private import M2::X; package ChainIn { public import X::*; } }
			package Path { private import $::M2; package PathIn { public import M2::X::Y::*; } }
			package Private { public import M1::X::Kept::*; }
			package Half { private import Mixed::X; package HalfIn { public import X::*; } }
			package HalfPath { public import Mixed::X::*; }
			package Same { public import Both::X::*; }
			package Unseen { private import Hides::X; package UnseenIn { public import X::*; } }
			package Loop { private import Loop1::X; package LoopIn { public import X::*; } }
			package Wild { private import W3::X; package WildIn { public import X::*; } }
			package None { package NoneIn { public import X::*; } }
		}`})

	for fqn, want := range map[string]int{
		"P::K::N::B":             0,
		"P::K::N::A":             0,
		"P::Own::B":              0,
		"P::Own::A":              0,
		"P::Deep::DeepIn::C":     0,
		"P::Deep::DeepIn::D":     0,
		"P::Rooted::RootedIn::B": 0,
		"P::Rooted::RootedIn::A": 0,
		"P::Self::A":             0,
		"P::Missing::A":          0,
		"P::Hidden::HiddenIn::A": 0,
		"P::Hidden::HiddenIn::B": 0,
		"P::Hidden::HiddenIn::E": 0,
		"P::Twice::TwiceIn::B":   0,
		"P::Twice::TwiceIn::A":   0,
		"P::Repeated::A":         0,
		"P::Above::AboveIn::A":   0,
		"P::Lattice::A":          0,
		"P::Alias::AliasIn::A":   0,
		"P::Named::NamedIn::A":   0,
		"P::Chain::ChainIn::B":   0,
		"P::Chain::ChainIn::A":   0,
		"P::Path::PathIn::C":     0,
		"P::Private::Z":          0,
		"P::Half::HalfIn::A":     0,
		"P::Half::HalfIn::E":     0,
		"P::HalfPath::E":         0,
		"P::Same::B":             0,
		"P::Unseen::UnseenIn::A": 0,
		"P::Unseen::UnseenIn::B": 0,
		"P::Loop::LoopIn::A":     0,
		"P::Wild::WildIn::B":     0,
		"P::Wild::WildIn::A":     0,
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

// The index reads a wildcard import's target where nothing but declarations and
// wildcard re-exports bind it, so that qualified lookup finds the members.
func TestWildcardImportTargetTheIndexReads(t *testing.T) {
	idx := buildIndex(t, map[string]string{"d.sysml": `
		package Q { package X { part def B; package Y { part def C; } } package A { } }
		package R { package X { part def E; } }
		package Bridge { public import Q::*; }
		package Keeps { private import Q::*; }
		package KeepsBeside { private import Q::*; package X { part def O; } }
		package P {
			package Past { package One { public import Keeps::X::*; } package Deep { public import Keeps::X::Y::*; } package All { public import all Keeps::X::*; } }
			package Beside { public import KeepsBeside::X::*; }
			package Outer { package X { public import R::X; part def A; } package Reads { public import X::*; } }
			package Owns { package X { part def O; } private import Q::X; package OwnsIn { public import X::*; } }
			package Kept { package X { private package Hid { part def D; } } package All { public import all X::Hid::*; } package Plain { public import X::Hid::*; } }
			package Through { public import Bridge::X::Y::*; }
			// Each import's target names the other's first segment: expansion still settles.
			package Mutual {
				package A { public import R::X; } package X { public import Q::A; }
				package Both { public import A::*; public import X::*; }
			}
		}`})
	for fqn, want := range map[string]int{
		"P::Outer::Reads::A": 1, // the import's own re-export of X does not rebind its target
		"P::Owns::OwnsIn::O": 1, // an owned member is found before an import of its name
		"P::Owns::OwnsIn::B": 0,
		"P::Kept::All::D":    1, // `import all` reaches a private segment
		"P::Kept::Plain::D":  0,
		"P::Through::C":      1, // below a segment a global qualifier re-exports
		"P::Past::One::B":    0, // a segment only a private import brought is not named from outside
		"P::Past::Deep::C":   0,
		"P::Past::All::B":    1,
		"P::Beside::O":       1, // an owned segment is named past a private import of its name
		"P::Beside::B":       0,
	} {
		wantNamed(t, idx, fqn, want)
	}
}

// A name several elements bear, or one may carry as a short name, may be rebound.
func TestNamesSeveral(t *testing.T) {
	idx := buildIndex(t, map[string]string{"d.sysml": `
		package Q { package X { part def B; } part def <s> Long; }
		package R { package X; alias Other for Q; }
		package W { public import Q::*; }`})
	for name, want := range map[string]bool{"X": true, "B": false, "Q": false, "s": true, "Other": false, "None": false} {
		if got := idx.NamesSeveral(name); got != want {
			t.Errorf("NamesSeveral(%q) = %t, want %t", name, got, want)
		}
	}
}
