package grpc

import "testing"

// A name an objective's body writes resolves past the subject its generals declare, which the
// objective's own subject redefines (SysML 8.3.18; KerML 8.3.3.3.6), to the case's subject.
func TestGetElementPropertiesResolvesPastARedefinedInheritedSubject(t *testing.T) {
	r := parseForElementProperties(t, `package P {
	part def Lander;
	requirement def R { subject lander : Lander; }
	analysis def A { subject lander : Lander; objective o : R { subject s = lander; attribute q = lander; } }
	analysis def B { subject lander : Lander; objective o : R { subject = lander; } }
}`)
	for element, want := range map[string]string{
		"P__A__o__s_pvalue_preferent":      "P__A__lander",
		"P__A__o__q_pvalue_preferent":      "P__A__lander",
		"P__B__o__lander_pvalue_preferent": "P__B__o__lander",
	} {
		resp := r.read(element, "memberElement")
		if resp.Error != "" {
			t.Fatalf("%s: %s", element, resp.Error)
		}
		if len(resp.Properties) != 1 || len(resp.Properties[0].Values) != 1 || resp.Properties[0].Values[0].GetElement() == nil {
			t.Fatalf("%s memberElement = %v", element, resp.Properties)
		}
		if got := resp.Properties[0].Values[0].GetElement().Id; got != want {
			t.Errorf("%s memberElement = %s, want %s", element, got, want)
		}
	}
}
