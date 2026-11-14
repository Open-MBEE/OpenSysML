package semantics

import (
	"strings"
	"testing"
)

// A `transition` owns the Membership naming its `first` source and an `in` link parameter through a
// ParameterMembership (a second ahead of a trigger), as the pilot exports them; a state body's `then x;` owns neither.
func TestTransitionOwnsItsSourceMembershipAndLinkParameters(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
    attribute def Sig;
    state def S {
        state idle;
        state busy;
        transition go first idle then busy;
        transition wake first busy accept Sig then idle;
        transition if true then idle;
        state outer {
            state nested;
        }
        transition deep first nested then busy;
    }
    state def T {
        state idle;
        then busy;
        state busy;
    }
}
`)
	s := nested(t, sym(t, root, "P").Scope, "S").Scope
	for name, want := range map[string]string{
		"go":   "Membership:StateUsage ParameterMembership:ReferenceUsage OwningMembership:SuccessionAsUsage",
		"wake": "Membership:StateUsage ParameterMembership:ReferenceUsage ParameterMembership:ReferenceUsage TransitionFeatureMembership:AcceptActionUsage OwningMembership:SuccessionAsUsage",
	} {
		if got := ownedMemberKinds(m, nested(t, s, name)); got != want {
			t.Errorf("%s owns %q, want %q", name, got, want)
		}
	}
	if got := ownedMemberKinds(m, nested(t, s, "deep")); got != "Membership:StateUsage ParameterMembership:ReferenceUsage OwningMembership:SuccessionAsUsage" {
		t.Errorf("deep owns %q, want the Membership of the nested source first", got)
	} else if source := ownedMembers(m, nested(t, s, "deep"))[0]; source.Name != "nested" {
		t.Errorf("deep's source member = %q, want nested", source.Name)
	}
	var guarded, stateBody []string
	for _, rel := range m.OwnedRelationships(nested(t, sym(t, root, "P").Scope, "S")) {
		if member := rel.Implicit.Member; member != nil && member.Name == "" && member.DeclaresTransition() {
			guarded = append(guarded, ownedMemberKinds(m, member))
		}
	}
	for _, rel := range m.OwnedRelationships(nested(t, sym(t, root, "P").Scope, "T")) {
		if member := rel.Implicit.Member; member != nil && member.DeclaresTransition() {
			stateBody = append(stateBody, ownedMemberKinds(m, member))
		}
	}
	if want := "ParameterMembership:ReferenceUsage TransitionFeatureMembership:LiteralBoolean OwningMembership:SuccessionAsUsage"; len(guarded) != 1 || guarded[0] != want {
		t.Errorf("the guarded transition owns %q, want [%q]", guarded, want)
	}
	// A state body's `then busy;` owns the Membership naming the state before it and a link parameter too.
	if want := "Membership:StateUsage ParameterMembership:ReferenceUsage OwningMembership:SuccessionAsUsage"; len(stateBody) != 1 || stateBody[0] != want {
		t.Errorf("the state body's `then busy;` owns %q, want [%q]", stateBody, want)
	}
	link := ownedMembers(m, nested(t, s, "go"))[1]
	if got, ok := m.ReflectiveFeatureValue(link, "direction"); !ok || !strings.HasSuffix(got.RefFQN, "::in") {
		t.Errorf("the link parameter's direction = %v (%t), want in", got, ok)
	}
}
