package passes

import "testing"

// The pilot rejects a flow end that names a member of an unrelated definition:
// the end subsets nothing, and the flow is left without two related features
// (Relationship_invalid_relatedElement1.sysml.xt).
func TestW8DFlowEndMustBeIdentifiable(t *testing.T) {
	src := `package p {
	part def A {
		part def B { out ref myOut; }
		part def C { in ref myIn; }
		flow XXX from B::myOut to C::myIn;
	}
}`
	w8dWantLines(t, src, "flow-end-subsetting", 5, 5)
	w8dWantLines(t, src, "connector-related-features", 5)
}

// Dot notation identifies both ends, so a legal flow stays silent.
func TestW8DIdentifiableFlowEndsStaySilent(t *testing.T) {
	src := `package p {
	part def B { out ref myOut; }
	part def C { in ref myIn; }
	part def A {
		part b : B;
		part c : C;
		flow XXX from b.myOut to c.myIn;
	}
}`
	for _, code := range []string{"flow-end-subsetting", "connector-related-features"} {
		if lines := w8dLines(t, src, code); len(lines) != 0 {
			t.Errorf("unexpected %s diagnostics at %v", code, lines)
		}
	}
}
