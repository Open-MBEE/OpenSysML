package grpc

import "testing"

// Each parameter of an expression body is an element of its own, read back by
// the id the form writes for it, with its own typing and membership.
func TestElementPropertiesTellsBodyParametersApart(t *testing.T) {
	r := parseForElementProperties(t, `package BP {
    private import ControlFunctions::*;
    part def Rollup {
        attribute masses : ScalarValues::Real[0..*];
        attribute total : ScalarValues::Real =
            masses->reduce { in a : ScalarValues::Real; in b : ScalarValues::Real; a + b };
    }
}`)
	first, second := "BP__Rollup__total_pvalue_pa0_pin0", "BP__Rollup__total_pvalue_pa0_pin1"
	for id, want := range map[string]string{first: "a", second: "b"} {
		got := r.read(id, "declaredName")
		if got.Error != "" || len(got.Properties) != 1 || len(got.Properties[0].Values) != 1 {
			t.Fatalf("%s = %v (%s)", id, got.Properties, got.Error)
		}
		if name := got.Properties[0].Values[0].GetStringValue(); name != want {
			t.Errorf("%s.declaredName = %q, want %q", id, name, want)
		}
	}
	typing := r.property(second+"_ft0", "typedFeature")
	if len(typing.Values) != 1 || typing.Values[0].GetElement().GetId() != second {
		t.Errorf("%s_ft0.typedFeature = %v, want %s", second, typing.Values, second)
	}
	membership := r.property(second+"_om", "ownedMemberFeature")
	if len(membership.Values) != 1 || membership.Values[0].GetElement().GetId() != second {
		t.Errorf("%s_om.ownedMemberFeature = %v, want %s", second, membership.Values, second)
	}
}
