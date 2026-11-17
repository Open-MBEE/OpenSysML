package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A flow end's ReferenceUsage is owned by the FlowEnd, an Occurrence-typed feature redefining
// Flows::Flow::source or target (SysML v2.1 §8.3.16.6), so it varies in time, as the pilot exports it.
func TestFlowEndFeatureVariesInTime(t *testing.T) {
	src := `package P {
		item def Photon;
		part def A { out x : Photon; }
		part def B { in y : Photon; }
		part a : A;
		part b : B;
		flow f from a.x to b.y;
	}`
	m, root := buildModelWithLibraryNamedKind(t, "flow-end-feature.sysml", source.KindSysML, src)
	p, ok := root.LookupLocal("P")
	if !ok {
		t.Fatal("P not found")
	}
	f, ok := p.Scope.LookupLocal("f")
	if !ok {
		t.Fatal("P::f not found")
	}
	ends := m.FlowEndMembers(f)
	if len(ends) != 2 {
		t.Fatalf("P::f owns %d flow ends, want 2", len(ends))
	}
	for i, end := range ends {
		feature := m.FlowEndFeatureMember(end)
		if feature == nil {
			t.Fatalf("end %d owns no ReferenceUsage", i)
		}
		if !m.FeatureIsVariable(feature) || !m.UsageMayTimeVary(feature) {
			t.Errorf("end %s's %s: isVariable = %v, mayTimeVary = %v, want true", m.EffectiveNameOf(end), m.EffectiveNameOf(feature), m.FeatureIsVariable(feature), m.UsageMayTimeVary(feature))
		}
		if m.FeatureIsVariable(end) {
			t.Errorf("end %s itself: isVariable = true, want false", m.EffectiveNameOf(end))
		}
	}
}
