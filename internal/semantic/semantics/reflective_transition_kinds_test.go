package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// TestTransitionFeatureKindsReadTheirLiteral: trigger, guard and effect memberships have their
// TransitionFeatureKind literal, and every time or change trigger's expression its TriggerKind literal.
func TestTransitionFeatureKindsReadTheirLiteral(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
    private import ScalarValues::*;
    private import Time::*;
    attribute def Sig;
    part def Machine {
        attribute x : Integer = 1;
        state sm {
            entry; then idle;
            state idle; state busy; state fin; state late;
            transition t1 first idle accept Sig if x > 0 do action e1 then busy;
            transition t2 first busy accept when x > 5 then fin;
            transition t3 first fin accept at TimeOf(this) then late;
            transition t4 first late accept after 2 [SI::s] then idle;
            transition t5 first idle when x > 7 then busy;
            transition t6 first busy accept tick : Sig at TimeOf(this) then fin;
        }
    }
}
`)
	sm := nested(t, nested(t, sym(t, root, "P").Scope, "Machine").Scope, "sm").Scope
	t1 := nested(t, sm, "t1")
	var kinds []string
	for _, membership := range m.OwnedMemberships(t1) {
		if meta := m.MetaclassOf(membership); meta == nil || meta.Name != "TransitionFeatureMembership" {
			t.Errorf("t1 owns a %v, want only TransitionFeatureMemberships", meta)
			continue
		}
		got, ok := m.ReflectiveFeatureValue(membership, "kind")
		if !ok || got.Kind != symbols.FilterValueRef {
			t.Fatalf("t1 membership kind = %v (present %t), want a literal", got, ok)
		}
		kinds = append(kinds, got.RefFQN)
	}
	want := []string{
		"SysML::Systems::TransitionFeatureKind::trigger",
		"SysML::Systems::TransitionFeatureKind::guard",
		"SysML::Systems::TransitionFeatureKind::effect",
	}
	if len(kinds) != len(want) {
		t.Fatalf("t1 kinds = %v, want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Errorf("t1 kind %d = %s, want %s", i, kinds[i], want[i])
		}
	}
	for name, want := range map[string]string{
		"t2": "SysML::Systems::TriggerKind::when",
		"t3": "SysML::Systems::TriggerKind::at",
		"t4": "SysML::Systems::TriggerKind::after",
		"t5": "SysML::Systems::TriggerKind::when",
		"t6": "SysML::Systems::TriggerKind::at",
	} {
		accepter, ok := m.ReflectiveElements(nested(t, sm, name), "triggerAction")
		if !ok || len(accepter) != 1 {
			t.Fatalf("%s.triggerAction = %v (present %t)", name, accepter, ok)
		}
		argument, ok := m.ReflectiveElements(accepter[0], "payloadArgument")
		if !ok || len(argument) != 1 {
			t.Fatalf("%s.triggerAction.payloadArgument = %v (present %t)", name, argument, ok)
		}
		if meta := m.MetaclassOf(argument[0]); meta == nil || meta.Name != "TriggerInvocationExpression" {
			t.Errorf("%s payloadArgument is a %v, want a TriggerInvocationExpression", name, meta)
		}
		got, ok := m.ReflectiveFeatureValue(argument[0], "kind")
		if !ok || got.Kind != symbols.FilterValueRef || got.RefFQN != want {
			t.Errorf("%s payloadArgument.kind = %v (present %t), want the literal %s", name, got, ok, want)
		}
	}
}
