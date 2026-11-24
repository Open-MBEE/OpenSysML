package export_test

import (
	"slices"
	"testing"
)

// The element form answers a constructor's function as the reader does: Performances::Evaluation
// (KerML 8.3.4.7.3), not the instantiated type the reader once answered, kept as a mapping value.
func TestAPIJSONConstructorFunctionIsTheEvaluationTypingIt(t *testing.T) {
	elements := apiJSONOf(t, `package P {
	attribute def Sig { attribute g; }
	attribute gain;
	attribute s = new Sig(gain);
}`)
	constructor := elements["P__s_pvalue"]
	if constructor["@type"] != "ConstructorExpression" {
		t.Fatalf("P__s_pvalue is a %v, want a ConstructorExpression", constructor["@type"])
	}
	evaluation := []string{libraryID(t, "Performances::Evaluation")}
	for _, property := range []string{"function", "behavior", "type"} {
		if got := refIDs(constructor[property]); !slices.Equal(got, evaluation) {
			t.Errorf("%s = %v, want Performances::Evaluation %v", property, got, evaluation)
		}
	}
	if got, want := refIDs(constructor["instantiatedType"]), []string{"P__Sig"}; !slices.Equal(got, want) {
		t.Errorf("instantiatedType = %v, want %v", got, want)
	}
	mapping, _ := constructor["sysx:mappingValues"].(map[string]any)
	if got, want := refIDs(mapping["function"]), []string{"P__Sig"}; !slices.Equal(got, want) {
		t.Errorf("the mapping value of function is %v, want the instantiated type %v", got, want)
	}
}
