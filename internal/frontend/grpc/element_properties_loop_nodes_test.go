package grpc

import (
	"slices"
	"testing"
)

// A for loop owns its variable once, first, as a ReferenceUsage with an id of its own (SysML.xtext
// ForVariableDeclarationMember), whether the loop is a named usage or a bare statement.
func TestGetElementPropertiesOwnsAForLoopsVariable(t *testing.T) {
	r := parseForElementProperties(t, `package P {
		private import ScalarValues::*;
		action def A {
			action forAct for i : Integer in (1, 2, 3) { action each; }
			for j : Integer in (1, 2) { action inner; }
		}
	}`)
	loops := answeredElements(t, r, "P__A", "ownedFeature")
	if len(loops) != 2 || loops[0] != "P::A::forAct" {
		t.Fatalf("A.ownedFeature = %v, want forAct and the unnamed loop", loops)
	}
	unnamed := r.property("P__A", "ownedFeature").Values[1].GetElement().Id
	for _, tc := range []struct{ loop, name, body string }{
		{"P__A__forAct", "i", "P::A::forAct::each"},
		{unnamed, "j", ""},
	} {
		variable := tc.loop + "_ploopVariable"
		features := r.property(tc.loop, "ownedFeature").Values
		if len(features) != 2 || features[0].GetElement().GetId() != variable || features[0].GetElement().GetMetaclass() != "ReferenceUsage" ||
			tc.body != "" && features[1].GetElement().GetQualifiedName() != tc.body {
			t.Errorf("%s.ownedFeature = %v, want the ReferenceUsage %s then the body's member", tc.loop, features, variable)
		}
		for _, property := range []string{"nestedReference", "loopVariable"} {
			if got := r.property(tc.loop, property).Values; len(got) != 1 || got[0].GetElement().GetId() != variable {
				t.Errorf("%s.%s = %v, want %s", tc.loop, property, got, variable)
			}
		}
		if got := r.property(variable, "declaredName").Values; len(got) != 1 || got[0].GetStringValue() != tc.name {
			t.Errorf("%s.declaredName = %v, want %s", variable, got, tc.name)
		}
		if got := answeredElements(t, r, variable, "type"); !slices.Equal(got, []string{"ScalarValues::Integer"}) {
			t.Errorf("%s.type = %v, want ScalarValues::Integer", variable, got)
		}
		owner := r.property(variable, "owner").Values
		if len(owner) != 1 || owner[0].GetElement().GetId() != tc.loop {
			t.Errorf("%s.owner = %v, want %s", variable, owner, tc.loop)
		}
		membership := r.property(variable, "owningMembership").Values
		if len(membership) != 1 || membership[0].GetElement().GetMetaclass() != "FeatureMembership" {
			t.Errorf("%s.owningMembership = %v, want a FeatureMembership", variable, membership)
		}
	}
}
