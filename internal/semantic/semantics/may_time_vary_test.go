package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// deriveUsageMayTimeVary (SysML v2.1 §8.3.6.4): a composite usage that
// specializes Actions::Action does not vary in time; a performed action and an
// exhibited state are referential by the notation, so they do; a required or
// assumed constraint is the ConstraintUsage its member declares; a flow's
// payload is a PayloadFeature, no usage, and never varies in time.
func TestMayTimeVaryReadsTheEffectiveComposite(t *testing.T) {
	src := `package Vary {
	state def S;
	part def Engine { perform action generateTorque; exhibit state s : S; action own; part sub : Engine; }
	requirement def R;
	requirement r : R { require constraint c2 { true } assume constraint c4 { true } constraint c3 { true } }
	item def Fuel;
	part def Car { part tank; part eng; flow fuelFlow of fuel : Fuel from tank to eng; }
}
`
	for i, fixture := range reflectiveFixtures(t, "vary.sysml", source.KindSysML, src) {
		vary, ok := fixture.root.LookupLocal("Vary")
		if !ok || vary.Scope == nil {
			t.Fatal("Vary not found")
		}
		engine, ok := vary.Scope.LookupLocal("Engine")
		if !ok || engine.Scope == nil {
			t.Fatal("Vary::Engine not found")
		}
		r, ok := vary.Scope.LookupLocal("r")
		if !ok || r.Scope == nil {
			t.Fatal("Vary::r not found")
		}
		car, ok := vary.Scope.LookupLocal("Car")
		if !ok || car.Scope == nil {
			t.Fatal("Vary::Car not found")
		}
		flow, ok := car.Scope.LookupLocal("fuelFlow")
		if !ok || flow.Scope == nil {
			t.Fatal("Vary::Car::fuelFlow not found")
		}
		for _, c := range []struct {
			owner *symbols.Symbol
			name  string
			want  bool
		}{
			{engine, "generateTorque", true}, {engine, "s", true}, {engine, "own", false},
			{r, "c2", true}, {r, "c4", true}, {r, "c3", true},
			{car, "fuelFlow", false}, {flow, "fuel", false},
		} {
			sym, ok := c.owner.Scope.LookupLocal(c.name)
			if !ok {
				t.Fatalf("%s::%s not found", c.owner.Name, c.name)
			}
			if got := fixture.model.UsageMayTimeVary(sym); got != c.want {
				t.Errorf("fixture %d: %s::%s mayTimeVary = %v, want %v", i, c.owner.Name, c.name, got, c.want)
			}
			if got := fixture.model.UsageMayTimeVaryForOwningType(sym, c.owner); got != c.want {
				t.Errorf("fixture %d: %s::%s mayTimeVary for owning type = %v, want %v", i, c.owner.Name, c.name, got, c.want)
			}
			if got := fixture.model.FeatureIsVariable(sym); got != c.want {
				t.Errorf("fixture %d: %s::%s isVariable = %v, want %v", i, c.owner.Name, c.name, got, c.want)
			}
		}
	}
}
