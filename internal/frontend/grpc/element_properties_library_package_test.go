package grpc

import "testing"

// A library package answers LibraryPackage::isStandard as its `standard` keyword states it (KerML 8.3.4.13.3).
func TestElementPropertiesAnswerALibraryPackagesIsStandard(t *testing.T) {
	r := parseForElementProperties(t, "standard library package S { part def A; }\nlibrary package L { part def B; }")
	for name, want := range map[string]bool{"S": true, "L": false} {
		property := r.property(name, "isStandard")
		if property.Unsupported != "" || len(property.Values) != 1 || property.Values[0].GetBoolValue() != want {
			t.Errorf("%s.isStandard = %+v, want %t", name, property, want)
		}
	}
}
