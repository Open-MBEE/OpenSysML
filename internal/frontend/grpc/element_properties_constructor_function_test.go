package grpc

import (
	"slices"
	"testing"
)

// Expression::function is the Function typing the expression (KerML 8.3.4.7.3): Performances::Evaluation.
// The instantiated type P::Sig the reader once answered is an AttributeDefinition and no Function: not restored.
func TestGetElementPropertiesAnswersAConstructorExpressionsFunction(t *testing.T) {
	// Sig declares no feature for the argument, which the reader answers over all the same.
	r := parseForElementPropertiesWithErrors(t, `package P {
		attribute def Sig;
		port def SP;
		part def T {
			port sp : SP;
			attribute gain;
			action sendRef send new Sig(gain) via sp;
		}
	}`)
	const constructor = "P__T__sendRef___400_pvalue"
	for _, property := range []string{"function", "behavior", "type"} {
		if got, want := answeredElements(t, r, constructor, property), []string{"Performances::Evaluation"}; !slices.Equal(got, want) {
			t.Errorf("%s = %v, want %v", property, got, want)
		}
	}
	resp := r.read(constructor, "function")
	if metaclass := resp.Properties[0].Values[0].GetElement().Metaclass; metaclass != "Function" {
		t.Errorf("function is a %s, want a Function", metaclass)
	}
	if got, want := answeredElements(t, r, constructor, "instantiatedType"), []string{"P::Sig"}; !slices.Equal(got, want) {
		t.Errorf("instantiatedType = %v, want %v", got, want)
	}
}
