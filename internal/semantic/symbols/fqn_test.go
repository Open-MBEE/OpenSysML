package symbols

import "testing"

// HasFQN answers what FQNOf would build, for the name a symbol has and for the
// prefixes, suffixes and near-misses of it that it does not.
func TestHasFQNMatchesFQNOf(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package P { package Q { part def Vehicle { attribute mass; } } }")

	for _, fqn := range []string{"P", "P::Q", "P::Q::Vehicle", "P::Q::Vehicle::mass"} {
		syms := idx.LookupQualified(fqn)
		if len(syms) != 1 {
			t.Fatalf("LookupQualified(%s) len = %d, want 1", fqn, len(syms))
		}
		sym := syms[0]
		if got := FQNOf(sym); got != fqn {
			t.Fatalf("FQNOf(%s) = %q", fqn, got)
		}
		if !HasFQN(sym, fqn) {
			t.Errorf("HasFQN(%s, %q) = false, want true", fqn, fqn)
		}
		for _, other := range []string{"", "Q", "Vehicle", "mass", "P::Q::Vehicle::mass::x",
			"P::Q::Vehicle", "X::" + fqn, fqn + "::x", fqn + "x"} {
			if want := other == fqn; HasFQN(sym, other) != want {
				t.Errorf("HasFQN(%s, %q) = %v, want %v", fqn, other, !want, want)
			}
		}
	}
}

func TestHasFQNNilSymbol(t *testing.T) {
	if !HasFQN(nil, "") {
		t.Error(`HasFQN(nil, "") = false, want true`)
	}
	if HasFQN(nil, "P") {
		t.Error(`HasFQN(nil, "P") = true, want false`)
	}
}

// An unnamed owner contributes no segment: a member declared inside an
// anonymous part of Mid is Mid::inner, never Mid::::inner, and HasFQN agrees.
func TestFQNOfSkipsUnnamedOwners(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "mid.sysml", "package Mid { part { part inner { attribute mass; } } }")

	mid := idx.LookupQualified("Mid")
	if len(mid) != 1 || mid[0].Scope == nil {
		t.Fatalf("LookupQualified(Mid) = %v, want one symbol with a scope", mid)
	}
	anon := mid[0].Scope.AnonymousMembers()
	if len(anon) != 1 || anon[0].Scope == nil {
		t.Fatalf("Mid has %d anonymous members, want the part with a scope", len(anon))
	}
	inner, ok := anon[0].Scope.LookupLocal("inner")
	if !ok {
		t.Fatal("the anonymous part declares no inner")
	}
	mass, ok := inner.Scope.LookupLocal("mass")
	if !ok {
		t.Fatal("inner declares no mass")
	}
	for sym, want := range map[*Symbol]string{inner: "Mid::inner", mass: "Mid::inner::mass"} {
		if got := FQNOf(sym); got != want {
			t.Errorf("FQNOf(%s) = %q, want %q", sym.Name, got, want)
		}
		if !HasFQN(sym, want) {
			t.Errorf("HasFQN(%s, %q) = false, want true", sym.Name, want)
		}
		for _, other := range []string{"Mid::::" + sym.Name, sym.Name, "Mid"} {
			if HasFQN(sym, other) {
				t.Errorf("HasFQN(%s, %q) = true, want false", sym.Name, other)
			}
		}
	}
}
