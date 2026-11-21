package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

func TestReflectiveAcceptActionOwnsItsReceiverParameter(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
    item def Data;
    port def CommIF;
    part def Station {
        port commIF : CommIF;
        action run {
            action receive accept d : Data via commIF;
        }
    }
}`)
	receive := nestedSym(t, root, "P::Station::run::receive")
	memberships := m.OwnedMemberships(receive)
	var receiver *symbols.Symbol
	for _, membership := range memberships {
		member := membership.Implicit.Member
		if member != nil && member.Expr != nil && member.Expr.Role == symbols.ExpressionPayload {
			if mc := m.MetaclassOf(membership); mc == nil || mc.Name != "ParameterMembership" {
				t.Fatalf("receiver membership is a %v, want ParameterMembership", mc)
			}
			receiver = member
		}
	}
	if receiver == nil {
		t.Fatalf("receive owns no receiver parameter among %d memberships", len(memberships))
	}
	if last := memberships[len(memberships)-1].Implicit.Member; last != receiver {
		t.Errorf("receiver is not the last owned member")
	}
	values, ok := m.ReflectiveElements(receiver, "ownedMembership")
	if !ok || len(values) != 1 {
		t.Fatalf("receiver ownedMembership = %v (present %t), want its FeatureValue", values, ok)
	}
	if mc := m.MetaclassOf(values[0]); mc == nil || mc.Name != "FeatureValue" {
		t.Fatalf("receiver's membership is a %v, want FeatureValue", mc)
	}
	value := values[0].Implicit.Member
	if mc := m.MetaclassOf(value); mc == nil || mc.Name != "FeatureReferenceExpression" {
		t.Fatalf("receiver value is a %v, want FeatureReferenceExpression", mc)
	}
	referent, ok := m.ReflectiveElements(value, "referent")
	if !ok || len(referent) != 1 || referent[0].Name != "commIF" {
		t.Errorf("receiver value referent = %v (present %t), want commIF", referent, ok)
	}
}
