package export_test

import (
	"slices"
	"testing"
)

// The element form gives a named constructor argument's result feature the Redefinition of the feature
// it names and orders argument by the instantiated type's features (KerML 8.3.4.8.3), library or not.
func TestAPIJSONConstructorArgumentsFollowTheInstantiatedTypesFeatures(t *testing.T) {
	elements := apiJSONOf(t, `package R {
	private import ShapeItems::*;
	private import SI::*;
	attribute def Box { attribute l; attribute w; attribute h; }
	item c1 = new Cuboid(4800 [mm], 1840 [mm], 1350 [mm]);
	attribute c3 = new Box(h = 1 [mm], l = 2 [mm], w = 3 [mm]);
	item c4 = new RectangularCuboid(height = 1 [mm], length = 2 [mm], width = 3 [mm]);
}`)
	check := func(id, property string, want ...string) {
		t.Helper()
		if got := refIDs(elements[id][property]); !slices.Equal(got, want) {
			t.Errorf("%s.%s = %v, want %v", id, property, got, want)
		}
	}
	check("R__c1_pvalue", "instantiatedType", libraryID(t, "ShapeItems::Cuboid"))
	check("R__c1_pvalue", "argument", "R__c1_pvalue_pa0", "R__c1_pvalue_pa1", "R__c1_pvalue_pa2")
	check("R__c1_pvalue_pin0", "ownedRedefinition")

	check("R__c3_pvalue", "instantiatedType", "R__Box")
	check("R__c3_pvalue", "argument", "R__c3_pvalue_pn1", "R__c3_pvalue_pn2", "R__c3_pvalue_pn0")
	for index, feature := range []string{"R__Box__h", "R__Box__l", "R__Box__w"} {
		result := "R__c3_pvalue_pin" + string(rune('0'+index))
		check(result, "ownedRedefinition", result+"_rd0")
		check(result+"_rd0", "redefinedFeature", feature)
		check(result+"_rd0", "redefiningFeature", result)
	}

	check("R__c4_pvalue", "instantiatedType", libraryID(t, "ShapeItems::RectangularCuboid"))
	check("R__c4_pvalue", "argument", "R__c4_pvalue_pn1", "R__c4_pvalue_pn2", "R__c4_pvalue_pn0")
	for index, name := range []string{"height", "length", "width"} {
		result := "R__c4_pvalue_pin" + string(rune('0'+index))
		check(result+"_rd0", "redefinedFeature", libraryID(t, "ShapeItems::RectangularCuboid::"+name))
	}
}
