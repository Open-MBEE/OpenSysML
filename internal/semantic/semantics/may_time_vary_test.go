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
// payload is a PayloadFeature, no usage, and never varies in time; a variant's
// VariantMembership is no FeatureMembership, so it has no owningType.
func TestMayTimeVaryReadsTheEffectiveComposite(t *testing.T) {
	src := `package Vary {
	state def S;
	part def Engine { perform action generateTorque; exhibit state s : S; action own; part sub : Engine; }
	requirement def R;
	requirement r : R { require constraint c2 { true } assume constraint c4 { true } constraint c3 { true } }
	item def Fuel;
	part def Car { part tank; part eng; flow fuelFlow of fuel : Fuel from tank to eng; }
	variation part def EngineChoices :> Engine { variant part e4 : Engine; variant part e6 : Engine; part plain : Engine; }
	attribute values : Real[*];
	attribute positives = values->select { in v : Real; v > 0 };
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
		choices, ok := vary.Scope.LookupLocal("EngineChoices")
		if !ok || choices.Scope == nil {
			t.Fatal("Vary::EngineChoices not found")
		}
		flow, ok := car.Scope.LookupLocal("fuelFlow")
		if !ok || flow.Scope == nil {
			t.Fatal("Vary::Car::fuelFlow not found")
		}
		// An expression body's parameter is owned by the Expression, an Occurrence;
		// an interface record keeps no expression body, so only the resolved model reads it.
		if i == 0 {
			item := bodyParameter(t, fixture.root, "v")
			for name, got := range map[string]bool{"mayTimeVary": fixture.model.UsageMayTimeVary(item), "isVariable": fixture.model.FeatureIsVariable(item)} {
				if !got {
					t.Errorf("fixture %d: body parameter v %s = false, want true", i, name)
				}
			}
		}
		if fuel, ok := flow.Scope.LookupLocal("fuel"); !ok {
			t.Fatal("Vary::Car::fuelFlow::fuel not found")
		} else if meta := fixture.model.MetaclassOf(fuel); meta == nil || meta.Name != "PayloadFeature" {
			t.Errorf("fixture %d: fuelFlow::fuel metaclass = %v, want PayloadFeature", i, meta)
		}
		for _, c := range []struct {
			owner *symbols.Symbol
			name  string
			want  bool
		}{
			{engine, "generateTorque", true}, {engine, "s", true}, {engine, "own", false},
			{r, "c2", true}, {r, "c4", true}, {r, "c3", true},
			{car, "fuelFlow", false}, {flow, "fuel", false},
			{choices, "e4", false}, {choices, "e6", false}, {choices, "plain", true},
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

// A KerML body parameter is a KerML Feature: Usage::mayTimeVary is not its to derive.
func TestKerMLBodyParameterDoesNotVaryInTime(t *testing.T) {
	const src = `package P {
		attribute values : Integer[*];
		attribute positives = values->select { in item : Integer; item > 0 };
	}`
	m, root := buildModelWithStdlibNamedKind(t, "body.kerml", source.KindKerML, src)
	param := bodyParameter(t, root, "item")
	if m.UsageMayTimeVary(param) {
		t.Error("KerML body parameter item: mayTimeVary = true, want false")
	}
	if m.FeatureIsVariable(param) {
		t.Error("KerML body parameter item: isVariable = true, want false")
	}
}
