package grpc

import (
	"slices"
	"testing"
)

// A constructor's arguments value its result's features in the order of the instantiated type's
// features they redefine (KerML 8.3.4.8.3), a library type's as a workspace type's.
func TestGetElementPropertiesBindsALibraryConstructorsArguments(t *testing.T) {
	r := parseForElementProperties(t, `package R {
		private import ShapeItems::*;
		private import SI::*;
		attribute def Box { attribute l; attribute w; attribute h; }
		item c1 = new Cuboid(4800 [mm], 1840 [mm], 1350 [mm]);
		attribute c2 = new Box(4800 [mm], 1840 [mm], 1350 [mm]);
		item c4 = new RectangularCuboid(height = 1 [mm], length = 2 [mm], width = 3 [mm]);
	}`)
	check := func(id, property string, want ...string) {
		t.Helper()
		if got := answeredElements(t, r, id, property); !slices.Equal(got, want) {
			t.Errorf("%s.%s = %v, want %v", id, property, got, want)
		}
	}
	check("R__c1_pvalue", "instantiatedType", "ShapeItems::Cuboid")
	check("R__c1_pvalue", "argument",
		"R__c1_pvalue_pa0:OperatorExpression", "R__c1_pvalue_pa1:OperatorExpression", "R__c1_pvalue_pa2:OperatorExpression")
	check("R__c2_pvalue", "instantiatedType", "R::Box")
	check("R__c2_pvalue", "argument",
		"R__c2_pvalue_pa0:OperatorExpression", "R__c2_pvalue_pa1:OperatorExpression", "R__c2_pvalue_pa2:OperatorExpression")

	// Each named argument's result feature redefines the library feature it names
	// (checkConstructorExpressionResultFeatureRedefinition), which orders the arguments.
	check("R__c4_pvalue", "instantiatedType", "ShapeItems::RectangularCuboid")
	for index, name := range []string{"height", "length", "width"} {
		feature := "R__c4_pvalue_pin" + string(rune('0'+index))
		check(feature, "ownedRedefinition", feature+"_rd0:Redefinition")
		check(feature+"_rd0", "redefinedFeature", "ShapeItems::RectangularCuboid::"+name)
	}
	check("R__c4_pvalue", "argument",
		"R__c4_pvalue_pn1:OperatorExpression", "R__c4_pvalue_pn2:OperatorExpression", "R__c4_pvalue_pn0:OperatorExpression")
}

// Arguments outnumbering the public features of the instantiated type have no feature to bind to.
func TestGetElementPropertiesRefusesUnbindableConstructorArguments(t *testing.T) {
	r := parseForElementPropertiesWithErrors(t, `package R {
		attribute def One { attribute l; attribute w; }
		attribute c = new One(1, 2, 3, 4, 5, 6, 7, 8);
	}`)
	resp := r.read("R__c_pvalue", "argument")
	if len(resp.Properties) != 1 || resp.Properties[0].Unsupported == "" {
		t.Fatalf("argument = %v, want it refused as unbindable", resp.Properties)
	}
}
