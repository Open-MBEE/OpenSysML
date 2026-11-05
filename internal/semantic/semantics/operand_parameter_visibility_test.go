package semantics

import (
	"strings"
	"testing"
)

// An operator's operand is a private parameter, an invocation's argument a public one —
// the pilot's OperandEList against KerMLExpressions.xtext ArgumentMember — on every reader.
func TestOperandParameterMembershipsArePrivateAndArgumentsPublic(t *testing.T) {
	m, root := buildModelWithStdlib(t, `
package D {
    private import ScalarValues::*;
    calc def F { in a : Real; in b : Real; return : Real = a + b; }
    attribute sum : Real = 1.0 + 2.0;
    attribute called : Real = F(1.0, 2.0);
    attribute all : Boolean = (1..3)->forAll { in i : Positive; i > 0 };
    attribute piped : Real = 1.0->F(2.0);
    attribute summed : Real = (1.0, 2.0)->reduce '+';
    calc def H { in body; return : Real = 1.0; }
    attribute applied : Real = H({ in x : Real; x });
}
`)
	d, _ := root.LookupLocal("D")
	for _, c := range []struct {
		name string
		want []string
	}{
		{"sum", []string{"private", "private"}},
		{"called", []string{"public", "public"}},
		{"all", []string{"private", "private"}},
		{"piped", []string{"private", "public"}},
		{"summed", []string{"private", "private"}},
		{"applied", []string{"public"}},
	} {
		expr := m.ValueExpression(member(t, m, d, c.name))
		if expr == nil {
			t.Fatalf("%s: no value expression", c.name)
		}
		var got []string
		for _, membership := range m.OwnedMemberships(expr) {
			if mc := m.MetaclassOf(membership); mc == nil || mc.Name != "ParameterMembership" {
				continue
			}
			value, ok := m.reflectiveFeatureValue(membership, "visibility")
			if !ok {
				t.Fatalf("%s: ParameterMembership.visibility not served", c.name)
			}
			got = append(got, value.RefFQN[strings.LastIndex(value.RefFQN, "::")+2:])
		}
		if strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Errorf("%s: ParameterMembership visibilities = %v, want %v", c.name, got, c.want)
		}
	}
}
