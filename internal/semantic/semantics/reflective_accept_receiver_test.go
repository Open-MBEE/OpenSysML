package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// TestViaTriggerReflectsItsReceiver: an `accept … via p` accept owns a receiver parameter after its
// payload, valued by a FeatureReferenceExpression to p that is its receiverArgument (SysML v2.1 §8.3.17.2).
func TestViaTriggerReflectsItsReceiver(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package V {
    item def Sig;
    port def P { in item s : Sig; }
    part def M {
        port p : P;
        state sm {
            entry; then idle;
            state idle; state busy;
            transition t1 first idle accept Sig via p then busy;
            transition t2 first busy accept s : Sig via p then idle;
            transition t3 first idle accept Sig then busy;
        }
    }
}
`)
	machine := nested(t, sym(t, root, "V").Scope, "M")
	port := nested(t, machine.Scope, "p")
	sm := nested(t, machine.Scope, "sm").Scope
	accept := func(name string) *symbols.Symbol {
		t.Helper()
		accepter, _ := m.ReflectiveElements(nested(t, sm, name), "triggerAction")
		if len(accepter) != 1 {
			t.Fatalf("%s triggerAction = %v, want one accept", name, accepter)
		}
		return accepter[0]
	}
	for _, name := range []string{"t1", "t2"} {
		accepter := accept(name)
		args, ok := m.ReflectiveElements(accepter, "receiverArgument")
		if !ok || len(args) != 1 || m.MetaclassOf(args[0]).Name != "FeatureReferenceExpression" {
			t.Fatalf("%s receiverArgument = %v (derived %t), want one FeatureReferenceExpression", name, args, ok)
		}
		if referent, _ := m.ReflectiveElements(args[0], "referent"); len(referent) != 1 || referent[0] != port {
			t.Errorf("%s receiverArgument referent = %v, want the port p", name, referent)
		}
		payload, _ := m.ReflectiveElements(accepter, "payloadParameter")
		for _, feature := range []string{"ownedFeature", "ownedMember", "parameter", "input", "directedFeature"} {
			got, _ := m.ReflectiveElements(accepter, feature)
			if len(got) != 2 || len(payload) != 1 || got[0] != payload[0] {
				t.Fatalf("%s accept %s = %v, want the payload then the receiver", name, feature, got)
			}
		}
		owned, _ := m.ReflectiveElements(accepter, "ownedFeature")
		memberships := m.OwnedMemberships(accepter)
		if len(memberships) != 2 {
			t.Fatalf("%s accept ownedMembership = %v, want two", name, memberships)
		}
		receiver := owned[1]
		if symbols.KeyOf(receiver) == symbols.KeyOf(owned[0]) || symbols.KeyOf(args[0]) == symbols.KeyOf(m.ValueExpression(owned[0])) {
			t.Errorf("%s receiver shares an identity with the payload", name)
		}
		if symbols.KeyOf(memberships[1]) == symbols.KeyOf(memberships[0]) {
			t.Errorf("%s accept parameter memberships share an identity", name)
		}
		if value := m.ValueExpression(receiver); value != args[0] {
			t.Errorf("%s receiver value = %v, want its receiverArgument", name, value)
		}
		if got, ok := m.ReflectiveFeatureValue(receiver, "direction"); !ok || got.RefFQN == "" || got.RefFQN[len(got.RefFQN)-2:] != "in" {
			t.Errorf("%s receiver direction = %v, want in", name, got)
		}
		for _, membership := range memberships {
			if meta := m.MetaclassOf(membership); meta == nil || meta.Name != "ParameterMembership" {
				t.Errorf("%s accept owns a %v, want ParameterMemberships", name, meta)
			}
		}
		if elements, _ := m.ReflectiveElements(receiver, "ownedElement"); len(elements) != 1 || elements[0] != args[0] {
			t.Errorf("%s receiver ownedElement = %v, want only its value", name, elements)
		}
	}
	plain := accept("t3")
	if args, ok := m.ReflectiveElements(plain, "receiverArgument"); !ok || len(args) != 0 {
		t.Errorf("t3 receiverArgument = %v (derived %t), want none", args, ok)
	}
	if params, _ := m.ReflectiveElements(plain, "parameter"); len(params) != 1 {
		t.Errorf("t3 parameter = %v, want only the payload", params)
	}
}
