package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// A transition's link parameters vary, as the pilot exports them (isVariable true: referential usages
// an occurrence owns); the succession's ends do not. The payload's case needs the library: tests/export.
func TestTransitionLinkParametersMayTimeVary(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
    attribute def Sig;
    state def S {
        state idle;
        state busy;
        transition go first idle accept Sig then busy;
    }
}
`)
	s := nested(t, sym(t, root, "P").Scope, "S").Scope
	members := ownedMembers(m, nested(t, s, "go"))
	if len(members) != 5 {
		t.Fatalf("go owns %d members, want 5 (source, two link parameters, accepter, succession)", len(members))
	}
	varying := map[string]*symbols.Symbol{"link parameter": members[1], "trigger parameter": members[2]}
	for name, part := range varying {
		if got, ok := m.ReflectiveFeatureValue(part, "isVariable"); !ok || !got.Bool {
			t.Errorf("the %s's isVariable = %v (%t), want true", name, got, ok)
		}
	}
	for i, end := range ownedMembers(m, members[4]) {
		if got, ok := m.ReflectiveFeatureValue(end, "isVariable"); !ok || got.Bool {
			t.Errorf("the succession's end %d isVariable = %v (%t), want false", i, got, ok)
		}
	}
}
