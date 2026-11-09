package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// A state owns one StateSubactionMembership per `entry`, `do` or `exit` member (SysML.xtext
// EntryActionMember…): a bare `entry;` owns nothing through it, a statement or usage is owned through it.
func TestReflectiveStateOwnsItsSubactionMemberships(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
	attribute x : ScalarValues::Boolean;
	state def S {
		entry;
		do assign x := true;
		exit action a {}
	}
}`)
	state := nestedSym(t, root, "P::S")
	var got []string
	for _, membership := range m.OwnedMemberships(state) {
		mc := m.MetaclassOf(membership)
		if mc == nil || mc.Name != "StateSubactionMembership" {
			continue
		}
		kind, ok := m.ReflectiveFeatureValue(membership, "kind")
		if !ok || kind.Kind != symbols.FilterValueRef {
			t.Fatalf("subaction membership kind = %v (present %t), want a literal", kind, ok)
		}
		member := "nothing"
		if element := membership.Implicit.Member; element != nil {
			if mc := m.MetaclassOf(element); mc != nil {
				member = mc.Name
			}
		}
		got = append(got, kind.RefFQN+" "+member)
	}
	want := []string{
		"SysML::Systems::StateSubactionKind::entry nothing",
		"SysML::Systems::StateSubactionKind::do AssignmentActionUsage",
		"SysML::Systems::StateSubactionKind::exit ActionUsage",
	}
	if len(got) != len(want) {
		t.Fatalf("subaction memberships = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("subaction membership %d = %q, want %q", i, got[i], want[i])
		}
	}
}
