package grpc

import "testing"

// The result of a constructor expression is typed by the instantiated type, read as a send payload
// and as a feature value (KerML 8.3.4.8.3 checkConstructorExpressionResultSpecialization).
func TestGetElementPropertiesTypesAConstructorResult(t *testing.T) {
	r := parseForElementProperties(t, `package P {
		private import ScalarValues::*;
		attribute def Sig { attribute level : Real; }
		port def SigPort;
		calc def F { in x : Real; return : Real; }
		action def Emit {
			in sp : SigPort; in gain : Real;
			action sendIt send new Sig(gain) via sp;
			attribute sigValue : Sig = new Sig(gain);
			attribute scaled = F(gain);
		}
	}`)
	qualifiedNames := func(id, property string) []string {
		t.Helper()
		var out []string
		for _, value := range r.property(id, property).Values {
			if el := value.GetElement(); el != nil {
				out = append(out, el.QualifiedName)
			} else {
				out = append(out, value.GetStringValue())
			}
		}
		return out
	}
	for _, tc := range []struct{ constructor, instantiated string }{
		{"P__Emit__sendIt___400_pvalue", "P::Sig"},
		{"P__Emit__sigValue_pvalue", "P::Sig"},
	} {
		if got := qualifiedNames(tc.constructor, "instantiatedType"); len(got) != 1 || got[0] != tc.instantiated {
			t.Errorf("%s.instantiatedType = %v, want [%s]", tc.constructor, got, tc.instantiated)
		}
		result := tc.constructor + "_pout"
		if got := qualifiedNames(result, "type"); len(got) != 1 || got[0] != tc.instantiated {
			t.Errorf("%s.type = %v, want [%s]", result, got, tc.instantiated)
		}
		if got := qualifiedNames(result, "direction"); len(got) != 1 || got[0] != "out" {
			t.Errorf("%s.direction = %v, want [out]", result, got)
		}
	}
	// An invocation's result takes the type of the function's result it redefines.
	if got := qualifiedNames("P__Emit__scaled_pvalue_pout", "type"); len(got) != 1 || got[0] != "ScalarValues::Real" {
		t.Errorf("P__Emit__scaled_pvalue_pout.type = %v, want [ScalarValues::Real]", got)
	}
}
