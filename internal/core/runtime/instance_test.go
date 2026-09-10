package runtime

import (
	"testing"
)

func TestInstantiate_SimplePartDef(t *testing.T) {
	src := `
		part def Wheel {
			attribute diameter: Real = 0.5;
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(NewModel(model, resolver), 1000)

	wheelSym := resolveSymbol(t, root, "Wheel")

	inst, err := ctx.Instantiate(wheelSym)
	if err != nil {
		t.Fatalf("Instantiate failed: %v", err)
	}

	if inst.ID != 1 {
		t.Errorf("expected ID=1, got %d", inst.ID)
	}

	if len(inst.FeatureValues) != 1 {
		t.Fatalf("expected 1 feature value, got %d", len(inst.FeatureValues))
	}

	diameterFeatureValue, ok := inst.FeatureValues["diameter"]
	if !ok {
		t.Fatal("expected 'diameter' feature value")
	}

	// Check default value evaluated
	if diameterFeatureValue.Value.Kind != ValConst {
		t.Errorf("expected ValConst, got %v", diameterFeatureValue.Value.Kind)
	}
	if diameterFeatureValue.Value.Const.Real != 0.5 {
		t.Errorf("expected Real=0.5, got %f", diameterFeatureValue.Value.Const.Real)
	}
}

// An unbounded upper bound carries the numeric value 0, so a feature declared
// [*] must not be mistaken for a scalar and filled with a single default.
func TestInstantiate_UnboundedDefaultIsNotAScalar(t *testing.T) {
	src := `
		part def Wheel {
			attribute diameter: Real = 0.5;
		}
		part def Car {
			part wheels: Wheel[0..*] = 1;
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(NewModel(model, resolver), 1000)

	inst, err := ctx.Instantiate(resolveSymbol(t, root, "Car"))
	if err != nil {
		t.Fatalf("Instantiate failed: %v", err)
	}
	fv, err := inst.GetFeatureValue(ctx, "wheels")
	if err != nil {
		t.Fatalf("GetFeatureValue failed: %v", err)
	}
	if fv.Value.Kind != ValInvalid {
		t.Errorf("unbounded feature value holds a scalar value %v", fv.Value)
	}
	if fv.Values.Kind != ValSequence {
		t.Errorf("expected a sequence, got %v", fv.Values.Kind)
	}
}

func TestInstantiate_IDAllocation(t *testing.T) {
	src := `part def A {}`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(NewModel(model, resolver), 1000)

	aSym := resolveSymbol(t, root, "A")

	inst1, _ := ctx.Instantiate(aSym)
	inst2, _ := ctx.Instantiate(aSym)

	if inst1.ID == inst2.ID {
		t.Error("expected unique IDs")
	}
}

func TestGetFeatureValue_LazyComposite(t *testing.T) {
	src := `
		part def Engine {}
		part def Car {
			part engine: Engine;
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(NewModel(model, resolver), 1000)

	carSym := resolveSymbol(t, root, "Car")
	inst, err := ctx.Instantiate(carSym)
	if err != nil {
		t.Fatalf("Instantiate failed: %v", err)
	}

	// Verify engine feature value NOT materialized initially
	engineFeatureValue := inst.FeatureValues["engine"]
	if engineFeatureValue == nil {
		t.Fatal("expected engine feature value")
	}
	if engineFeatureValue.Materialized {
		t.Error("expected engine feature value NOT materialized after Instantiate")
	}

	// Call GetFeatureValue → should lazy-materialize
	fv, err := inst.GetFeatureValue(ctx, "engine")
	if err != nil {
		t.Fatalf("GetFeatureValue failed: %v", err)
	}

	if !fv.Materialized {
		t.Error("expected engine feature value materialized after GetFeatureValue")
	}

	if fv.Value.Kind != ValInstance {
		t.Errorf("expected ValInstance, got %v", fv.Value.Kind)
	}

	// Verify child instance exists in registry
	childInst, ok := ctx.getInstance(fv.Value.Instance)
	if !ok {
		t.Error("expected child instance registered")
	}
	if childInst.Type.Name != "Engine" {
		t.Errorf("expected Engine type, got %s", childInst.Type.Name)
	}
}

// A multi-valued feature holds its default's contents, not <unknown>: a single
// value written on it is the collection's one element.
func TestMultiValuedDefaultMaterializes(t *testing.T) {
	src := `
		part def Rig {
			attribute mass = 100.0;
			attribute doubles[0..*] = mass * 2.0;
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(NewModel(model, resolver), 1000)

	inst, err := ctx.Instantiate(resolveSymbol(t, root, "Rig"))
	if err != nil {
		t.Fatalf("Instantiate failed: %v", err)
	}
	fv, err := inst.GetFeatureValue(ctx, "doubles")
	if err != nil {
		t.Fatalf("GetFeatureValue failed: %v", err)
	}
	if fv.Values.Kind != ValSequence {
		t.Fatalf("Values.Kind = %v, want a sequence", fv.Values.Kind)
	}
	elements := fv.Values.Sequence().Elements()
	if len(elements) != 1 || elements[0].Const.Real != 200.0 {
		t.Errorf("doubles = %v, want [200]", elements)
	}
}

// A multi-valued feature that is typed as well as given a default holds the
// default's contents rather than an instantiation of its type.
func TestTypedMultiValuedDefaultHoldsItsContents(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			private import ScalarValues::Real;
			part def Sub { attribute volume : Real; }
			part def Ctx {
				part subsystem : Sub[0..*];
				attribute volumes : Real[0..*] = subsystem.volume;
			}
			part ctx : Ctx {
				part a : Sub :> subsystem { attribute :>> volume = 2.0; }
				part b : Sub :> subsystem { attribute :>> volume = 3.5; }
			}
		}
	`))
	matches := idx.LookupQualified("test::ctx")
	if len(matches) != 1 {
		t.Fatalf("test::ctx: %d matching symbols, want 1", len(matches))
	}
	inst, err := ctx.Instantiate(matches[0])
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	fv, err := inst.GetFeatureValue(ctx, "volumes")
	if err != nil {
		t.Fatalf("GetFeatureValue(volumes): %v", err)
	}
	if got := FormatTraceValue(fv.HeldValue()); got != "(2.0, 3.5)" {
		t.Errorf("volumes = %s, want (2.0, 3.5)", got)
	}
}

// A composite multi-valued part given a default holds the very objects the
// default names, rather than fresh anonymous objects of its type.
func TestCompositeMultiValuedDefaultHoldsTheNamedObjects(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			private import ScalarValues::Real;
			part def Sub { attribute volume : Real; }
			part def Bay {
				part left : Sub { attribute :>> volume = 2.0; }
				part right : Sub { attribute :>> volume = 3.5; }
				part stowed : Sub[2] = (left, right);
			}
			part bay : Bay;
		}
	`))
	matches := idx.LookupQualified("test::bay")
	if len(matches) != 1 {
		t.Fatalf("test::bay: %d matching symbols, want 1", len(matches))
	}
	inst, err := ctx.Instantiate(matches[0])
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	var want []int64
	for _, name := range []string{"left", "right"} {
		fv, err := inst.GetFeatureValue(ctx, name)
		if err != nil {
			t.Fatalf("GetFeatureValue(%s): %v", name, err)
		}
		want = append(want, fv.Value.Instance)
	}
	fv, err := inst.GetFeatureValue(ctx, "stowed")
	if err != nil {
		t.Fatalf("GetFeatureValue(stowed): %v", err)
	}
	elements := elementsOf(fv.HeldValue())
	if len(elements) != 2 {
		t.Fatalf("stowed holds %d element(s), want 2", len(elements))
	}
	for i, el := range elements {
		if el.Kind != ValInstance || el.Instance != want[i] {
			t.Errorf("stowed[%d] = %v, want instance %d", i, el, want[i])
		}
	}
}

// A nested part usage with a body of its own is an object shaped by that body:
// what it redeclares must win over what its type declares.
func TestNestedUsageBodyOverridesItsType(t *testing.T) {
	src := `
		part def Engine {
			attribute power = 200.0;
		}
		part def Car {
			part engine : Engine {
				attribute power redefines Engine::power = 250.0;
			}
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(NewModel(model, resolver), 1000)

	car, err := ctx.Instantiate(resolveSymbol(t, root, "Car"))
	if err != nil {
		t.Fatalf("Instantiate failed: %v", err)
	}
	if got := nestedReal(t, ctx, car, "engine", "power"); got != 250.0 {
		t.Errorf("engine.power = %v, want 250 (the usage body's value)", got)
	}
}

// A written default takes precedence over instantiation in GetFeatureValue, so the
// feature holds that value and is not something to materialize an object from.
func TestCompositeTypeOfIgnoresDefaultedFeature(t *testing.T) {
	src := `
		attribute def Temp {
			attribute v = 1.0;
		}
		part def Gauge {
			attribute plain : Temp;
			attribute written : Temp = 5.0;
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(NewModel(model, resolver), 1000)

	features := ctx.FeaturesOf(resolveSymbol(t, root, "Gauge"))
	for i := range features {
		feat := &features[i]
		composite := ctx.CompositeTypeOf(feat)
		if feat.Name == "written" && composite != nil {
			t.Errorf("written holds 5.0, but reports %s as composite", composite.Name)
		}
		if feat.Name == "plain" && composite == nil {
			t.Error("plain is materialized from Temp, want it reported as composite")
		}
	}
}

// An untyped nested part is still an object: its body is its shape, so it
// materializes and its members hold values.
func TestUntypedNestedPartMaterializes(t *testing.T) {
	src := `
		part def Car {
			attribute mass = 1500.0;
			part engine {
				attribute power = 300.0;
			}
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(NewModel(model, resolver), 1000)

	car, err := ctx.Instantiate(resolveSymbol(t, root, "Car"))
	if err != nil {
		t.Fatalf("Instantiate failed: %v", err)
	}
	if got := nestedReal(t, ctx, car, "engine", "power"); got != 300.0 {
		t.Errorf("engine.power = %v, want 300", got)
	}
}

// A redefining declaration whose body values a feature the value it inherits
// would supply governs over that value: the more specific declaration holds, so
// the body is neither dropped nor read alongside the inherited value.
func TestBodyGovernsAnInheritedValue(t *testing.T) {
	src := `
		attribute def Cost {
			attribute v = 1.0;
			attribute w = 2.0;
		}
		part def Ring {
			attribute template : Cost { attribute :>> v = 9.0; attribute :>> w = 8.0; }
			attribute cost : Cost = template;
		}
		part def Band :> Ring {
			attribute :>> cost { attribute :>> v = 11.0; }
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(NewModel(model, resolver), 1000)

	band, err := ctx.Instantiate(resolveSymbol(t, root, "Band"))
	if err != nil {
		t.Fatalf("Instantiate failed: %v", err)
	}
	if got := nestedReal(t, ctx, band, "cost", "v"); got != 11.0 {
		t.Errorf("cost.v = %v, want 11 (the body's value, not the inherited one)", got)
	}
	if got := nestedReal(t, ctx, band, "cost", "w"); got != 2.0 {
		t.Errorf("cost.w = %v, want 2 (Cost's own default, the inherited value being governed over)", got)
	}
	for _, feat := range ctx.FeaturesOf(resolveSymbol(t, root, "Ring")) {
		if feat.Name == "cost" && ctx.CompositeTypeOf(&feat) != nil {
			t.Error("Ring::cost is bound to template, want it reported as no composite")
		}
	}
}

// A renamed redefinition (`fine :>> cost { ... }`) governs the inherited value as
// a same-named one does: the two names share the object its body describes.
func TestRenamedRedefinitionBodyGovernsAnInheritedValue(t *testing.T) {
	src := `
		attribute def Cost {
			attribute v = 1.0;
			attribute w = 2.0;
		}
		part def Ring {
			attribute template : Cost { attribute :>> v = 9.0; attribute :>> w = 8.0; }
			attribute cost : Cost default template;
		}
		part def Band :> Ring {
			attribute fine :>> cost { attribute :>> v = 11.0; }
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(NewModel(model, resolver), 1000)

	band, err := ctx.Instantiate(resolveSymbol(t, root, "Band"))
	if err != nil {
		t.Fatalf("Instantiate failed: %v", err)
	}
	for _, name := range []string{"fine", "cost"} {
		if got := nestedReal(t, ctx, band, name, "v"); got != 11.0 {
			t.Errorf("%s.v = %v, want 11 (the body's value, not the inherited default)", name, got)
		}
		if got := nestedReal(t, ctx, band, name, "w"); got != 2.0 {
			t.Errorf("%s.w = %v, want 2 (Cost's own default)", name, got)
		}
	}
}

// A condition read without an object agrees with materializing: a value a body
// governs over is not the value the condition sees, so the feature is reported
// uninitialized rather than judged against the superseded value.
func TestConditionsDoNotReadAGovernedOverValue(t *testing.T) {
	src := `
		attribute def Cost { attribute v = 1.0; }
		part def Ring {
			attribute template : Cost { attribute :>> v = 9.0; }
			attribute cost : Cost = template;
			attribute plain : Cost = template;
		}
		part def Band :> Ring {
			attribute :>> cost { attribute :>> v = 11.0; }
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(NewModel(model, resolver), 1000)

	features := ctx.conditionFeatures(resolveSymbol(t, root, "Band"))
	if got, ok := features["cost"]; !ok || got.expr != nil {
		t.Errorf("cost reads %v, want no expression: its body governs over the inherited value", got.expr)
	}
	if got, ok := features["plain"]; !ok || got.expr == nil {
		t.Error("plain reads no expression, want the inherited value it is still bound to")
	}
}

// nestedReal reads a Real out of the instance held by one of inst's feature values.
func nestedReal(t *testing.T, ctx *Context, inst *Instance, featureName, nestedName string) float64 {
	t.Helper()
	fv, err := inst.GetFeatureValue(ctx, featureName)
	if err != nil {
		t.Fatalf("GetFeatureValue(%q) failed: %v", featureName, err)
	}
	if fv.Value.Kind != ValInstance {
		t.Fatalf("feature value %q holds %v, want a nested instance", featureName, fv.Value.Kind)
	}
	nested, ok := ctx.Instance(fv.Value.Instance)
	if !ok {
		t.Fatalf("feature value %q references unknown instance %d", featureName, fv.Value.Instance)
	}
	nestedFeatureValue, err := nested.GetFeatureValue(ctx, nestedName)
	if err != nil {
		t.Fatalf("GetFeatureValue(%q) failed: %v", nestedName, err)
	}
	return nestedFeatureValue.Value.Const.Real
}

// A default that is no value at all holds nothing, so what the multiplicity
// check counted is what the feature value stores.
func TestNullDefaultHoldsNoElements(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, `
		part def Rig {
			attribute nothing[0..*] = null;
		}
	`)
	ctx := NewContext(NewModel(model, resolver), 1000)
	inst, err := ctx.Instantiate(resolveSymbol(t, root, "Rig"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	fv, err := inst.GetFeatureValue(ctx, "nothing")
	if err != nil {
		t.Fatalf("GetFeatureValue: %v", err)
	}
	if elements := elementsOf(fv.HeldValue()); len(elements) != 0 {
		t.Errorf("nothing holds %v, want no elements", elements)
	}
}
