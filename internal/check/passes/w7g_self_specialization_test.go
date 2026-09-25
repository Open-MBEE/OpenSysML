package passes

import "testing"

// A usage subsetting itself is a one-element specialization cycle: the graph
// drops the edge, so the check has to read the declaration. `redefines n` with
// nothing to redefine is an unresolved reference instead, at the name tier.
func TestW7GUsageSubsettingItselfIsACycle(t *testing.T) {
	for _, src := range []string{
		"package C { part p4 :> p4; }",
		"package C { part p4 subsets p4; }",
		"package C { attribute a :> a; }",
	} {
		if !hasCode(constraintDiags(t, src), "specialization-cycle") {
			t.Fatalf("expected a specialization cycle for %q", src)
		}
	}
}

// A same-named subsetting or redefinition of an *inherited* feature targets that
// feature, not itself, and must stay silent.
func TestW7GRedefiningAnInheritedFeatureIsNotACycle(t *testing.T) {
	for _, src := range []string{
		"package I { part def A { part x; } part def B :> A { part x :> x; } }",
		"package I { part def A { part x; } part def B :> A { part x redefines x; } }",
		"package I { part def A { part x; } part def B :> A { part y :> x; } }",
	} {
		if hasCode(constraintDiags(t, src), "specialization-cycle") {
			t.Fatalf("unexpected specialization cycle for %q", src)
		}
	}
}
