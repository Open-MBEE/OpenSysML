package grpc

import (
	"slices"
	"testing"
)

// A `for` loop owns its variable once, as its first owned feature with an id of its own
// (SysML.xtext ForVariableDeclarationMember), written as a named action node or as a statement.
func TestElementPropertiesReadAForLoopVariable(t *testing.T) {
	r := parseForElementProperties(t, `package P {
    private import ScalarValues::*;
    action def Named { action forAct for i : Integer in (1, 2, 3) { action each; } }
    action def Statement { for i : Integer in (1, 2, 3) { action each; } }
}`)
	for _, definition := range []string{"P__Named", "P__Statement"} {
		loops := propertyElementIDs(r.property(definition, "ownedFeature").Values)
		if len(loops) != 1 {
			t.Fatalf("%s owns the features %v, want its loop alone", definition, loops)
		}
		loop := loops[0]
		features := propertyElementIDs(r.property(loop, "ownedFeature").Values)
		t.Logf("%s: ownedFeature = %v", loop, features)
		if len(features) == 0 {
			t.Errorf("%s owns no feature, want its variable first", loop)
			continue
		}
		variable := features[0]
		if variable == loop || slices.Contains(features[1:], variable) {
			t.Errorf("%s: ownedFeature = %v, want the variable once under an id of its own", loop, features)
		}
		if names := r.property(variable, "declaredName").Values; len(names) != 1 || names[0].GetStringValue() != "i" {
			t.Errorf("%s: the first owned feature %s is named %v, want i", loop, variable, names)
		}
		if got := propertyElementIDs(r.property(loop, "loopVariable").Values); !slices.Equal(got, []string{variable}) {
			t.Errorf("%s: loopVariable = %v, want %s", loop, got, variable)
		}
		// The collection's parameter is a ReferenceUsage the loop nests after its variable.
		if got := propertyElementIDs(r.property(loop, "nestedReference").Values); len(got) != 2 || got[0] != variable {
			t.Errorf("%s: nestedReference = %v, want %s and the collection's parameter", loop, got, variable)
		}
		if got := propertyElementIDs(r.property(variable, "type").Values); len(got) != 1 {
			t.Errorf("%s: type = %v, want Integer alone", variable, got)
		}
		if got := propertyElementIDs(r.property(variable, "owner").Values); !slices.Equal(got, []string{loop}) {
			t.Errorf("%s: owner = %v, want %s", variable, got, loop)
		}
	}
}
