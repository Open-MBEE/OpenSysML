package queryexec

import "testing"

// TestForLoopVariableAndItsHeadHaveElementIDs: a `for` variable, the membership owning it and
// the typing its head states each read the id the conversion mints for them.
func TestForLoopVariableAndItsHeadHaveElementIDs(t *testing.T) {
	fixture := loadExecutionSource(t, `package Observatory {
	package P {
		part def Component;
		part def Special;
		action def A {
			in components : Component[*];
			action sweep for c : Component, Special in components { }
		}
	}
}`)
	ids := ownedElementIDs(t, fixture, "P")
	const variable = "/Package/OwningMembership/ActionDefinition/FeatureMembership/ForLoopActionUsage/FeatureMembership/ReferenceUsage"
	for id, want := range map[string]string{
		"Observatory__P__A__sweep_ploopVariable":     variable,
		"Observatory__P__A__sweep_ploopVariable_ft0": variable + "/FeatureTyping",
		"Observatory__P__A__sweep_ploopVariable_ft1": variable + "/FeatureTyping",
	} {
		if ids[id] != want {
			t.Errorf("elementId %q is that of %q, want %q", id, ids[id], want)
		}
	}
}
