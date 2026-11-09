package grpc

import "testing"

// A constructor owns only its result, which owns one `in` feature per argument
// named after the instantiated type's feature it values; a named argument's
// feature redefines that feature, and argument lists the values in the type's
// declared feature order (KerML 8.3.4.8.3).
func TestGetElementPropertiesShapesAConstructorExpression(t *testing.T) {
	r := parseForElementProperties(t, `package P {
		private import ScalarValues::*;
		item def Ping { attribute seq : Integer; attribute tag : String; }
		attribute a = new Ping(1, "x");
		attribute b = new Ping(tag = "y", seq = 2);
		action def A { action s send new Ping(seq = 1); }
	}`)
	ids := func(id, property string) []string {
		t.Helper()
		resp := r.read(id, property)
		if resp.Error != "" {
			t.Fatalf("%s.%s: %s", id, property, resp.Error)
		}
		if len(resp.Properties) != 1 || resp.Properties[0].Unsupported != "" {
			t.Fatalf("%s.%s = %v", id, property, resp.Properties)
		}
		var out []string
		for _, value := range resp.Properties[0].Values {
			if el := value.GetElement(); el != nil {
				out = append(out, el.Id+":"+el.Metaclass)
			} else {
				out = append(out, value.GetStringValue())
			}
		}
		return out
	}
	check := func(id, property string, want ...string) {
		t.Helper()
		got := ids(id, property)
		if len(got) != len(want) {
			t.Errorf("%s.%s = %v, want %v", id, property, got, want)
			return
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%s.%s = %v, want %v", id, property, got, want)
				return
			}
		}
	}
	check("P__a_pvalue", "ownedFeature", "P__a_pvalue_pout:Feature")
	check("P__a_pvalue", "input")
	check("P__a_pvalue_pout", "ownedFeature", "P__a_pvalue_pin0:Feature", "P__a_pvalue_pin1:Feature")
	check("P__a_pvalue_pout", "ownedRelationship", "P__a_pvalue_pin0_om:FeatureMembership", "P__a_pvalue_pin1_om:FeatureMembership")
	check("P__a_pvalue_pin1", "name", "tag")
	check("P__a_pvalue_pin1", "declaredName")
	check("P__a_pvalue_pin1", "direction", "in")
	check("P__a_pvalue_pin1", "ownedRedefinition")
	check("P__a_pvalue", "argument", "P__a_pvalue_pa0:LiteralInteger", "P__a_pvalue_pa1:LiteralString")

	check("P__b_pvalue_pout", "ownedFeature", "P__b_pvalue_pin0:Feature", "P__b_pvalue_pin1:Feature")
	check("P__b_pvalue_pin0", "name", "tag")
	check("P__b_pvalue_pin0", "ownedRedefinition", "P__b_pvalue_pin0_rd0:Redefinition")
	check("P__b_pvalue_pin0_rd0", "redefinedFeature", "P__Ping__tag:AttributeUsage")
	check("P__b_pvalue", "argument", "P__b_pvalue_pn1:LiteralInteger", "P__b_pvalue_pn0:LiteralString")

	check("P__A__s", "payloadArgument", "P__A__s___400_pvalue:ConstructorExpression")
	check("P__A__s___400_pvalue", "ownedFeature", "P__A__s___400_pvalue_pout:Feature")
	check("P__A__s___400_pvalue_pout", "ownedFeature", "P__A__s___400_pvalue_pin0:Feature")
	check("P__A__s___400_pvalue_pin0", "name", "seq")
	check("P__A__s___400_pvalue_pin0", "ownedRedefinition", "P__A__s___400_pvalue_pin0_rd0:Redefinition")
	check("P__A__s___400_pvalue", "argument", "P__A__s___400_pvalue_pn0:LiteralInteger")
}
