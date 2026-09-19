package passes

import "testing"

// A portion's values are portions of the instances that feature it, which do
// not vary between snapshots (KerML validateFeaturePortionNotVariable).
func TestPortionFeatureCannotBeVariable(t *testing.T) {
	const src = `
		class C {
			var feature x;
			portion var feature y;
		}`
	diags := diagsIn(t, "a.kerml", src, "constraint")
	if !hasCode(diags, "feature-portion-not-variable") {
		t.Fatalf("expected feature-portion-not-variable, got %v", diags)
	}
	for _, d := range diags {
		if d.Code == "feature-portion-not-variable" && d.Message != msgPortionFeatureVariable {
			t.Errorf("got %q", d.Message)
		}
	}
}

// A composite feature is not a portion, so `var` alone stays legal.
func TestCompositeVariableFeatureIsAllowed(t *testing.T) {
	const src = `
		class C {
			composite var feature x;
		}`
	if hasCode(diagsIn(t, "a.kerml", src, "constraint"), "feature-portion-not-variable") {
		t.Fatal("composite is not a portion")
	}
}
