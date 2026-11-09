package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func TestReflectiveGuardedSuccessionOwnsItsTransitionParts(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
    action def TakePicture {
        action focus { out ok : ScalarValues::Boolean; }
        action shoot;
        first focus if focus.ok then shoot;
    }
}`)
	takePicture := nestedSym(t, root, "P::TakePicture")
	var guarded *symbols.Symbol
	for _, membership := range m.OwnedMemberships(takePicture) {
		if member := membership.Implicit.Member; member != nil {
			if _, ok := member.Decl.(*ast.TransitionMember); ok {
				guarded = member
			}
		}
	}
	if guarded == nil {
		t.Fatal("TakePicture owns no guarded succession")
	}
	if mc := m.MetaclassOf(guarded); mc == nil || mc.Name != "TransitionUsage" {
		t.Fatalf("guarded succession is a %v, want TransitionUsage", mc)
	}
	memberships := m.OwnedMemberships(guarded)
	var kinds []string
	for _, membership := range memberships {
		if mc := m.MetaclassOf(membership); mc != nil {
			kinds = append(kinds, mc.Name)
		}
	}
	want := []string{"ParameterMembership", "Membership", "TransitionFeatureMembership", "OwningMembership"}
	if len(kinds) != len(want) {
		t.Fatalf("guarded succession memberships = %v, want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("guarded succession memberships = %v, want %v", kinds, want)
		}
	}
	if source := memberships[1].Implicit.Member; source == nil || source.Name != "focus" {
		t.Errorf("source membership names %v, want focus", source)
	}
	succession := memberships[3].Implicit.Member
	if mc := m.MetaclassOf(succession); mc == nil || mc.Name != "SuccessionAsUsage" {
		t.Fatalf("succession is a %v, want SuccessionAsUsage", mc)
	}
	ends, ok := m.ReflectiveElements(succession, "connectorEnd")
	if !ok || len(ends) != 2 {
		t.Fatalf("succession connectorEnd = %v (present %t), want two ends", ends, ok)
	}
	guard, ok := m.ReflectiveElements(guarded, "guardExpression")
	if !ok || len(guard) != 1 {
		t.Fatalf("guardExpression = %v (present %t), want the `if` expression", guard, ok)
	}
	if guard[0] != memberships[2].Implicit.Member {
		t.Errorf("guardExpression is not the guard membership's member")
	}
}
