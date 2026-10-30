package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// deriveUsageMayTimeVary (SysML v2.1 §8.3.6.4): a composite usage that
// specializes Actions::Action does not vary in time; a performed action and an
// exhibited state are referential by the notation, so they do.
func TestMayTimeVaryReadsTheEffectiveComposite(t *testing.T) {
	src := `package Vary {
	state def S;
	part def Engine { perform action generateTorque; exhibit state s : S; action own; part sub : Engine; }
}
`
	for _, fixture := range reflectiveFixtures(t, "vary.sysml", source.KindSysML, src) {
		vary, ok := fixture.root.LookupLocal("Vary")
		if !ok || vary.Scope == nil {
			t.Fatal("Vary not found")
		}
		engine, ok := vary.Scope.LookupLocal("Engine")
		if !ok || engine.Scope == nil {
			t.Fatal("Vary::Engine not found")
		}
		for name, want := range map[string]bool{"generateTorque": true, "s": true, "own": false, "sub": true} {
			sym, ok := engine.Scope.LookupLocal(name)
			if !ok {
				t.Fatalf("Vary::Engine::%s not found", name)
			}
			if got := fixture.model.UsageMayTimeVary(sym); got != want {
				t.Errorf("Vary::Engine::%s mayTimeVary = %v, want %v", name, got, want)
			}
		}
	}
}
