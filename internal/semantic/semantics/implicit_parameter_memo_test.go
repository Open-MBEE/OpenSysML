package semantics

import "testing"

// The redefinition closure of a feature is computed once per model, and a
// parameter's implicit redefinitions take its position from the owner's
// memoized parameter list instead of rebuilding it: repeated queries allocate
// nothing beyond what the owner's implied generals cost.
func TestRedefinedFeaturesAreComputedOncePerSymbol(t *testing.T) {
	m, root := buildModel(t, `package P {
		action def Base { in x; in y; out z; }
		action def Proc :> Base { in a; in b; out c; }
	}`)
	p := sym(t, root, "P")
	proc := nested(t, p.Scope, "Proc")
	a := nested(t, p.Scope, "Proc", "a")
	wantX := nested(t, p.Scope, "Base", "x")
	if got := m.AllRedefinedFeatures(a); len(got) != 1 || got[0] != wantX {
		t.Fatalf("AllRedefinedFeatures(Proc::a) = %v, want [Base::x]", got)
	}
	if got := testing.AllocsPerRun(20, func() { m.AllRedefinedFeatures(a) }); got != 0 {
		t.Fatalf("a repeated AllRedefinedFeatures query allocates %.0f times, want 0", got)
	}
	generals := testing.AllocsPerRun(20, func() { m.ImplicitGenerals(proc) })
	if got := testing.AllocsPerRun(20, func() { m.ImplicitParameterRedefinitions(a) }); got > generals+1 {
		t.Fatalf("a repeated ImplicitParameterRedefinitions query allocates %.0f times, want at most its result beyond the owner's implied generals (%.0f)", got, generals)
	}
}
