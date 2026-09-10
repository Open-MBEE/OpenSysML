package resolve

import "testing"

// An event occurrence declares a feature like any other, so its name resolves
// wherever a feature's does: from a value expression, and from the trigger of a
// transition that accepts it (SysML v2 §8.3.13).
func TestResolveEventOccurrenceFeature(t *testing.T) {
	for _, src := range []string{
		"occurrence def O; part def P { event occurrence o : O; }",
		"occurrence def O; part def P { event occurrence o : O; attribute a = o; }",
		"occurrence def O; part def P { occurrence o : O; event o; }",
		"occurrence def O; part def P { event occurrence o : O; state s; state t; accept o then s; }",
		"occurrence def O; action def A { event occurrence o : O; accept o; }",
	} {
		r := resolveDoc(t, "<t>", src)
		if len(r.Diagnostics) != 0 {
			t.Errorf("%s: expected no diagnostics, got %v", src, r.Diagnostics)
		}
	}
}

// `event <name>` references an occurrence rather than declaring one, so a name
// that declares nothing is unresolved; the declaration form is
// `event occurrence <name>`.
func TestResolveEventOccurrenceReferenceUnresolved(t *testing.T) {
	r := resolveDoc(t, "<t>", "occurrence def O; part def P { event e : O; }")
	if len(r.Diagnostics) != 1 {
		t.Fatalf("expected one unresolved-reference diagnostic, got %v", r.Diagnostics)
	}
}
