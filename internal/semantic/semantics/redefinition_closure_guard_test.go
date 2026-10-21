package semantics

import "testing"

// A redefinition closure walked while a guard cuts a query short is answered
// but not memoized: once the guard lifts, the next query sees the target.
func TestRedefinitionClosureIsNotCachedWhileAGuardCutsItShort(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def W { attribute x; }
		part def A { part w : W; }
		part def B :> A { attribute z :>> w.x; }
	}`)
	p := sym(t, root, "P")
	wantX := nested(t, p.Scope, "W", "x")
	bz := nested(t, p.Scope, "B", "z")
	// As RedefinedFeatures stands mid-computation: seeded, with its lookup open.
	m.redefined[bz] = nil
	m.computingRedefined[bz] = m.resolver.Enter()
	if got := m.AllRedefinedFeatures(bz); len(got) != 0 {
		t.Fatalf("AllRedefinedFeatures(B::z) under the guard = %v, want the guard's empty answer", got)
	}
	m.resolver.Leave()
	delete(m.redefined, bz)
	delete(m.computingRedefined, bz)
	if got := m.AllRedefinedFeatures(bz); len(got) != 1 || got[0] != wantX {
		t.Fatalf("AllRedefinedFeatures(B::z) after the guard = %v, want [W::x]", got)
	}
}
