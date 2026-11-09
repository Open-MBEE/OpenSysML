package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

func TestReflectiveDefinitionOwnsItsFirstThenSuccession(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
    occurrence def Flight {
        timeslice preflight[1];
        first preflight then inflight;
        timeslice inflight[1];
    }
}`)
	flight := nestedSym(t, root, "P::Flight")
	var successions []*symbols.Symbol
	for _, membership := range m.OwnedMemberships(flight) {
		member := membership.Implicit.Member
		if mc := m.MetaclassOf(member); mc != nil && mc.Name == "SuccessionAsUsage" {
			successions = append(successions, member)
		}
	}
	if len(successions) != 1 {
		t.Fatalf("Flight owns %d successions, want 1", len(successions))
	}
	succession := successions[0]
	ends, ok := m.ReflectiveElements(succession, "connectorEnd")
	if !ok || len(ends) != 2 {
		t.Fatalf("connectorEnd = %v (present %t), want two ends", ends, ok)
	}
	for i, want := range []string{"preflight", "inflight"} {
		referenced, ok := m.ReflectiveElements(ends[i], "ownedReferenceSubsetting")
		if !ok || len(referenced) != 1 {
			t.Fatalf("end %d ownedReferenceSubsetting = %v (present %t), want one", i, referenced, ok)
		}
		targets, ok := m.ReflectiveElements(referenced[0], "referencedFeature")
		if !ok || len(targets) != 1 || targets[0].Name != want {
			got := "nothing"
			if len(targets) == 1 {
				got = targets[0].Name
			}
			t.Errorf("end %d refers to %s, want %s", i, got, want)
		}
	}
}
