package grpc

import (
	"strings"
	"testing"
)

// A library parameter named only by redefinition — BooleanEvaluation's
// `return : Boolean;` — is answered by its catalogued id and the qualified
// name of its effective name, not by its owner's name with an empty segment.
func TestGetElementPropertiesNamesAnUnnamedLibraryParameterByItsEffectiveName(t *testing.T) {
	r := parseForElementProperties(t, `package P {
		private import ScalarValues::*;
		constraint def Cons { in x : Real; x > 0 }
	}`)
	resp := r.read("P__Cons", "directedFeature")
	if len(resp.Properties) != 1 || resp.Properties[0].Unsupported != "" {
		t.Fatalf("directedFeature: %v", resp.Properties)
	}
	var seen []string
	for _, value := range resp.Properties[0].Values {
		element := value.GetElement()
		seen = append(seen, element.GetQualifiedName())
		if element.GetQualifiedName() == "Performances::BooleanEvaluation::result" && element.GetId() != "bca3d873-62ee-59a0-b51a-85657345ae1d" {
			t.Errorf("the library return is %q, want the bundled library's id for it", element.GetId())
		}
		if strings.HasSuffix(element.GetId(), "::") || strings.HasSuffix(element.GetQualifiedName(), "::") {
			t.Errorf("element %q (%q) spelled with an empty name segment", element.GetId(), element.GetQualifiedName())
		}
	}
	if joined := strings.Join(seen, " "); !strings.Contains(joined, "Performances::BooleanEvaluation::result") {
		t.Errorf("directedFeature = %s, want Performances::BooleanEvaluation::result among them", joined)
	}
}
