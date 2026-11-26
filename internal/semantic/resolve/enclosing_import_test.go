package resolve

import (
	"fmt"
	"strings"
	"testing"
)

// A name an enclosing namespace imports is found there before a same-named
// member owned further out (KerML 8.2.3.5.4).
func TestEnclosingImportPrecedesOuterMember(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `
		package Q { part def T; }
		package P {
			part def T;
			package K1 { private import Q::*; part v : T; }
			package K2 { private import Q::*; package K3 { part v : T; } }
			package K4 { public import Q::*; package K5 { part v : T; } }
			package K6 { private import Q::T; package K7 { part v : T; } }
			package K8 { private import Q::*; package K9 { package K10 { part v : T; } } }
			part def C { private import Q::*; part cInner { part v : T; } }
			part usage { private import Q::*; part uInner { part v : T; } }
			state def S { private import Q::*; state sInner { part v : T; } }
			action def A {
				action plainOuter {
					private import Q::*;
					action plainInner { part v : T; }
					action innerLoop loop { part v : T; }
				}
			}
			package Own { private import Q::*; package Mid { part def T; package In { part v : T; } } }
			package None { package NoneIn { part v : T; } }
		}
	`)
	assertNoUnresolved(t, r)

	for _, tc := range []struct{ scope, want string }{
		{"K1", "Q::T"},
		{"K3", "Q::T"},
		{"K5", "Q::T"},
		{"K7", "Q::T"},
		{"K10", "Q::T"},
		{"cInner", "Q::T"},
		{"uInner", "Q::T"},
		{"sInner", "Q::T"},
		{"plainInner", "Q::T"},
		{"innerLoop", "Q::T"},
		{"In", "P::Own::Mid::T"},
		{"NoneIn", "P::T"},
	} {
		sym, ok := r.LookupName(scopeNamed(t, r, "d.sysml", tc.scope), "T")
		if !ok {
			t.Errorf("%s: T does not resolve", tc.scope)
			continue
		}
		if got := r.registeredFQN(sym); got != tc.want {
			t.Errorf("%s: T resolves to %s, want %s", tc.scope, got, tc.want)
		}
	}
}

// A member a namespace inherits is still found before a name imported further out.
func TestInheritedMemberPrecedesOuterImport(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `
		package Q { part def T; }
		package P {
			private import Q::*;
			part def Base { part def T; }
			part def C :> Base { part inner { part v : T; } }
		}
	`)
	assertNoUnresolved(t, r)

	sym, ok := r.LookupName(scopeNamed(t, r, "d.sysml", "inner"), "T")
	if !ok {
		t.Fatal("T does not resolve")
	}
	if got := r.registeredFQN(sym); got != "P::Base::T" {
		t.Errorf("T resolves to %s, want P::Base::T", got)
	}
}

// The leading segment of a qualified name follows the same order: a package
// importing its own name reaches the import, not itself, and `$::` roots the name.
func TestImportedNameHidesTheImportingPackageFromItself(t *testing.T) {
	const src = `
		package U { part def V; }
		package V {
			private import U::*;
			part def G;
			part x : %s;
		}
	`
	r := resolveDoc(t, "d.sysml", fmt.Sprintf(src, "V::G"))
	if len(r.Diagnostics) != 1 || !strings.Contains(r.Diagnostics[0].Message, "V::G") {
		t.Errorf("V::G inside V: diagnostics %v, want it unresolved under U::V", r.Diagnostics)
	}

	assertNoUnresolved(t, resolveDoc(t, "d.sysml", fmt.Sprintf(src, "$::V::G")))
}

// A call reaches what its name resolves to: the calculation an enclosing
// namespace imports, not a same-named one owned further out.
func TestEnclosingImportPrecedesOuterMemberForInvocation(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `
		package Q { calc def f { return : ScalarValues::Integer = 2; } }
		package P {
			calc def f { return : ScalarValues::Integer = 1; }
			package K1 { private import Q::*; package In1 { attribute x = f(); } }
			package K2 { private import Q::f; package In2 { attribute x = f(); } }
			package Own { private import Q::*; package Mid { calc def f; package In3 { attribute x = f(); } } }
			package None { package In4 { attribute x = f(); } }
		}
	`)

	for _, tc := range []struct{ scope, want string }{
		{"In1", "Q::f"},
		{"In2", "Q::f"},
		{"In3", "P::Own::Mid::f"},
		{"In4", "P::f"},
	} {
		cands := r.InvocationCandidates(scopeNamed(t, r, "d.sysml", tc.scope), qn(false, "f"))
		if len(cands) != 1 {
			t.Errorf("%s: f has %d candidates, want 1", tc.scope, len(cands))
			continue
		}
		if got := r.registeredFQN(cands[0]); got != tc.want {
			t.Errorf("%s: f calls %s, want %s", tc.scope, got, tc.want)
		}
	}
}

// Imports of distinct namespaces under one name hide each other (KerML 7.2.5.4),
// so a wildcard import of that name below them reads the namespace owned further
// out, and surfaces the members of neither import.
func TestWildcardImportBelowMutuallyHidingImports(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `
		package Q { package X { part def B; } }
		package R { package X { part def C; } }
		package P {
			package X { part def A; }
			package K { private import Q::X; private import R::X; package N { public import X::*; } }
			part a : K::N::A;
			part b : K::N::B;
			part c : K::N::C;
		}
	`)

	if len(r.Diagnostics) != 2 || !strings.Contains(r.Diagnostics[0].Message, "K::N::B") ||
		!strings.Contains(r.Diagnostics[1].Message, "K::N::C") {
		t.Errorf("diagnostics %v, want K::N::B and K::N::C unresolved", r.Diagnostics)
	}

	scope := scopeNamed(t, r, "d.sysml", "P")
	sym, ok := r.ResolveQualified(scope, qn(false, "K", "N", "A"))
	if !ok {
		t.Error("K::N::A does not resolve")
	} else if got := r.registeredFQN(sym); got != "P::X::A" {
		t.Errorf("K::N::A resolves to %s, want P::X::A", got)
	}
	for _, name := range []string{"B", "C"} {
		if sym, ok := r.ResolveQualified(scope, qn(false, "K", "N", name)); ok {
			t.Errorf("K::N::%s resolves to %s, want it unresolved", name, r.registeredFQN(sym))
		}
	}
}

// An alias an enclosing namespace declares or imports names the wildcard
// import's target like an import of it: the aliased namespace's members only.
func TestWildcardImportBelowAnAlias(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `
		package Q { package X { part def B; } }
		package M { alias X for Q::X; }
		package P {
			package X { part def A; }
			package Declared { private alias X for Q::X; package N { public import X::*; } }
			package Imported { private import M::X; package N { public import X::*; } }
		}
	`)
	assertNoUnresolved(t, r)

	scope := scopeNamed(t, r, "d.sysml", "P")
	for _, importer := range []string{"Declared", "Imported"} {
		sym, ok := r.ResolveQualified(scope, qn(false, importer, "N", "B"))
		if !ok {
			t.Errorf("%s::N::B does not resolve", importer)
		} else if got := r.registeredFQN(sym); got != "Q::X::B" {
			t.Errorf("%s::N::B resolves to %s, want Q::X::B", importer, got)
		}
		if sym, ok := r.ResolveQualified(scope, qn(false, importer, "N", "A")); ok {
			t.Errorf("%s::N::A resolves to %s, want it unresolved", importer, r.registeredFQN(sym))
		}
	}
}
