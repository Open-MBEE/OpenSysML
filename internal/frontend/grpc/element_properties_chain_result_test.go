package grpc

import "testing"

// A feature chain expression instantiates the '.' operator function and its result has the
// target feature's types (KerML 8.3.4.8.4, checkFeatureChainExpressionResultSpecialization).
func TestGetElementPropertiesTypesAFeatureChainExpression(t *testing.T) {
	r := parseForElementProperties(t, `package Example {
		part def M { attribute id : ScalarValues::String; attribute n : ScalarValues::Integer; }
		part def W {
			part machine : M { attribute :>> id = "123"; }
			attribute targetId : ScalarValues::String = machine.id;
			attribute untyped = machine.id;
			attribute grouped = (machine).id;
			part line { part station : M; }
			attribute nested = line.station.n;
			part special : S;
			attribute inherited = special.id;
		}
		part def S :> M;
	}`)
	for _, tc := range []struct{ element, property, want string }{
		{"Example__W__targetId_pvalue", "instantiatedType", "ControlFunctions::."},
		{"Example__W__targetId_pvalue", "function", "ControlFunctions::."},
		{"Example__W__targetId_pvalue", "targetFeature", "Example::W::machine::id"},
		{"Example__W__targetId_pvalue_pout", "type", "ScalarValues::String"},
		{"Example__W__untyped_pvalue", "instantiatedType", "ControlFunctions::."},
		{"Example__W__untyped_pvalue_pout", "type", "ScalarValues::String"},
		{"Example__W__grouped_pvalue", "instantiatedType", "ControlFunctions::."},
		{"Example__W__grouped_pvalue_pout", "type", "ScalarValues::String"},
		{"Example__W__nested_pvalue", "instantiatedType", "ControlFunctions::."},
		{"Example__W__nested_pvalue_pout", "type", "ScalarValues::Integer"},
		{"Example__W__inherited_pvalue_pout", "type", "ScalarValues::String"},
	} {
		t.Run(tc.element+"/"+tc.property, func(t *testing.T) {
			resp := r.read(tc.element, tc.property)
			if resp.Error != "" {
				t.Fatal(resp.Error)
			}
			if len(resp.Properties) != 1 || len(resp.Properties[0].Values) != 1 || resp.Properties[0].Values[0].GetElement() == nil {
				t.Fatalf("%s = %v, want the single element %s", tc.property, resp.Properties, tc.want)
			}
			if got := resp.Properties[0].Values[0].GetElement().QualifiedName; got != tc.want {
				t.Errorf("%s = %s, want %s", tc.property, got, tc.want)
			}
		})
	}
}
