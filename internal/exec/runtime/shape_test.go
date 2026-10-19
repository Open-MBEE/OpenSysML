package runtime

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

const testMaxSteps = 10000

func TestFeaturesOf(t *testing.T) {
	code := `
		part def Base {
			attribute x : Integer;
		}
		part def Derived :> Base {
			attribute y : Real;
		}
	`
	model, resolver, rootScope := parseAndBuildModel(t, code)
	ctx := NewContext(typedModel(model, resolver), testMaxSteps)

	derivedSym := resolveSymbol(t, rootScope, "Derived")
	features := ctx.FeaturesOf(derivedSym)

	if len(features) != 2 {
		t.Fatalf("expected 2 features, got %d", len(features))
	}

	// MembersOf returns local first, then inherited
	// features[0] = y (local), features[1] = x (inherited)
	if features[0].Name != "y" {
		t.Errorf("features[0].Name = %q, want %q", features[0].Name, "y")
	}
	if features[0].OwnerType.Name != "Derived" {
		t.Errorf("features[0].OwnerType.Name = %q, want %q", features[0].OwnerType.Name, "Derived")
	}

	if features[1].Name != "x" {
		t.Errorf("features[1].Name = %q, want %q", features[1].Name, "x")
	}
	if features[1].OwnerType.Name != "Base" {
		t.Errorf("features[1].OwnerType.Name = %q, want %q", features[1].OwnerType.Name, "Base")
	}
}

func TestFeaturesOf_Redefinition(t *testing.T) {
	code := `
		attribute def MyInt;
		attribute def MyReal;
		part def Base {
			attribute x : MyInt;
		}
		part def Derived :> Base {
			attribute x : MyReal redefines Base::x;
		}
	`
	model, resolver, rootScope := parseAndBuildModel(t, code)
	ctx := NewContext(typedModel(model, resolver), testMaxSteps)

	derivedSym := resolveSymbol(t, rootScope, "Derived")
	features := ctx.FeaturesOf(derivedSym)

	if len(features) != 1 {
		t.Fatalf("expected 1 feature (x redefined), got %d", len(features))
	}

	if features[0].Name != "x" {
		t.Errorf("features[0].Name = %q, want %q", features[0].Name, "x")
	}
	if features[0].OwnerType.Name != "Derived" {
		t.Errorf("features[0].OwnerType.Name = %q, want %q (redefining feature should win)", features[0].OwnerType.Name, "Derived")
	}

	// Type should be MyReal (the redefining feature's type)
	if features[0].Type == nil || features[0].Type.Name != "MyReal" {
		t.Errorf("features[0].Type should be MyReal, got %v", features[0].Type)
	}
}

// A renaming redefinition that restates no type is typed by the feature it
// redefines, so it materializes the same objects under the new name.
func TestFeaturesOf_TypeInheritedThroughRedefinition(t *testing.T) {
	code := `
		attribute def Cost { attribute v; }
		part def Ring { attribute ringCost : Cost; }
		part def Band :> Ring { attribute bandCost :>> ringCost; }
	`
	model, resolver, rootScope := parseAndBuildModel(t, code)
	ctx := NewContext(typedModel(model, resolver), testMaxSteps)

	features := ctx.FeaturesOf(resolveSymbol(t, rootScope, "Band"))
	if len(features) == 0 || features[0].Name != "bandCost" {
		t.Fatalf("FeaturesOf(Band) = %v, want bandCost first", features)
	}
	if features[0].Type == nil || features[0].Type.Name != "Cost" {
		t.Errorf("bandCost.Type = %v, want Cost", features[0].Type)
	}
}

func TestFeaturesOf_Multiplicity(t *testing.T) {
	code := `
		part def Thing {
			attribute items : Integer [0..10];
		}
	`
	model, resolver, rootScope := parseAndBuildModel(t, code)
	ctx := NewContext(typedModel(model, resolver), testMaxSteps)

	thingSym := resolveSymbol(t, rootScope, "Thing")
	features := ctx.FeaturesOf(thingSym)

	if len(features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(features))
	}

	mult := features[0].Multiplicity
	if !mult.Lower.Known || mult.Lower.Value != 0 {
		t.Errorf("expected Lower=0, got %+v", mult.Lower)
	}
	if !mult.Upper.Known || mult.Upper.Value != 10 {
		t.Errorf("expected Upper=10, got %+v", mult.Upper)
	}
}

func TestFeaturesOf_DefaultValue(t *testing.T) {
	code := `
		part def Thing {
			attribute count : Integer = 42;
		}
	`
	model, resolver, rootScope := parseAndBuildModel(t, code)
	ctx := NewContext(typedModel(model, resolver), testMaxSteps)

	thingSym := resolveSymbol(t, rootScope, "Thing")
	features := ctx.FeaturesOf(thingSym)

	if len(features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(features))
	}

	if features[0].DefaultValue == nil {
		t.Fatal("expected DefaultValue, got nil")
	}

	// DefaultValue is ast.Node — should be *ast.LiteralInteger
	litInt, ok := features[0].DefaultValue.(*ast.LiteralInteger)
	if !ok {
		t.Fatalf("expected *ast.LiteralInteger, got %T", features[0].DefaultValue)
	}
	if litInt.Value != "42" {
		t.Errorf("expected literal value %q, got %q", "42", litInt.Value)
	}
}

// TestFeaturesOf_TypeConjoinedThroughSubsetting: a feature subsetting one typed by a
// specialization of its type holds that specialization (KerML 1.0 §7.3.4.4).
func TestFeaturesOf_TypeConjoinedThroughSubsetting(t *testing.T) {
	code := `
		part def Sheet { attribute area; }
		part def Framed :> Sheet { attribute border; }
		part def Holder {
			part frames : Framed [0..*];
			part front : Sheet [1] :> frames;
		}
	`
	model, resolver, rootScope := parseAndBuildModel(t, code)
	ctx := NewContext(typedModel(model, resolver), testMaxSteps)

	front := effectiveFeatureNamed(t, ctx.FeaturesOf(resolveSymbol(t, rootScope, "Holder")), "front")
	if front.Type == nil || front.Type.Name != "Framed" {
		t.Fatalf("front.Type = %v, want Framed", front.Type)
	}
	if extra := ctx.conjoinedTypes(front.Symbol, front.Type); len(extra) != 0 {
		t.Errorf("conjoinedTypes(front) = %v, want Framed alone to stand for Sheet and Framed", extra)
	}
	if got := sortedFeatureNames(ctx.FeaturesOf(ctx.CompositeTypeOf(&front))); len(got) != 2 || got[0] != "area" || got[1] != "border" {
		t.Errorf("features of what front holds = %v, want [area border]", got)
	}
}

// TestFeaturesOf_EveryStatedTypeIsConjoined: `part both : A, B` holds values of every
// stated type; the first is the type its objects are made as, the rest conjoined.
func TestFeaturesOf_EveryStatedTypeIsConjoined(t *testing.T) {
	code := `
		part def A { attribute a; }
		part def B { attribute b; }
		part def Holder { part both : A, B [1]; }
	`
	model, resolver, rootScope := parseAndBuildModel(t, code)
	ctx := NewContext(typedModel(model, resolver), testMaxSteps)

	both := effectiveFeatureNamed(t, ctx.FeaturesOf(resolveSymbol(t, rootScope, "Holder")), "both")
	if both.Type == nil || both.Type.Name != "A" {
		t.Fatalf("both.Type = %v, want A", both.Type)
	}
	if got := typeNames(ctx.effectiveTypes(both.Symbol)); len(got) != 2 || got[0] != "A" || got[1] != "B" {
		t.Errorf("effectiveTypes(both) = %v, want [A B]", got)
	}
	if extra := ctx.conjoinedTypes(both.Symbol, both.Type); len(extra) != 1 || extra[0].Name != "B" {
		t.Errorf("conjoinedTypes(both, A) = %v, want [B]", typeNames(extra))
	}
}

func typeNames(types []*symbols.Symbol) []string {
	names := make([]string, 0, len(types))
	for _, typ := range types {
		names = append(names, typ.Name)
	}
	return names
}

// TestFeaturesOf_ConjunctionOfUnrelatedTypes: when neither type stands for the other, the
// object is classified by both and carries both's members; a shared name is one feature.
func TestFeaturesOf_ConjunctionOfUnrelatedTypes(t *testing.T) {
	code := `
		part def Panel { attribute area; attribute weight; }
		part def Framed { attribute border; attribute weight; }
		part def Holder {
			part frames : Framed [0..*];
			part front : Panel [1] :> frames;
		}
	`
	model, resolver, rootScope := parseAndBuildModel(t, code)
	ctx := NewContext(typedModel(model, resolver), testMaxSteps)

	holderSym := resolveSymbol(t, rootScope, "Holder")
	front := effectiveFeatureNamed(t, ctx.FeaturesOf(holderSym), "front")
	if front.Type == nil || front.Type.Name != "Panel" {
		t.Fatalf("front.Type = %v, want the stated Panel", front.Type)
	}
	if typ := ctx.CompositeTypeOf(&front); typ != front.Type {
		t.Fatalf("CompositeTypeOf(front) = %v, want Panel", typ)
	}
	if extra := ctx.conjoinedTypes(front.Symbol, front.Type); len(extra) != 1 || extra[0].Name != "Framed" {
		t.Fatalf("conjoinedTypes(front) = %v, want [Framed]", extra)
	}

	holder, err := ctx.materialize(holderSym, 0, nil, "")
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	fv, err := holder.GetFeatureValue(ctx, "front")
	if err != nil {
		t.Fatalf("holder.front: %v", err)
	}
	obj, ok := ctx.Instance(fv.HeldValue().Instance)
	if !ok {
		t.Fatalf("holder.front = %s, want an object", FormatValue(fv.HeldValue()))
	}
	names := make([]string, 0, len(obj.FeatureValues))
	for name := range obj.FeatureValues {
		names = append(names, name)
	}
	slices.Sort(names)
	if len(names) != 3 || names[0] != "area" || names[1] != "border" || names[2] != "weight" {
		t.Errorf("features of what front holds = %v, want [area border weight]", names)
	}
	for _, typ := range []string{"Panel", "Framed"} {
		if !ctx.instanceConforms(obj, resolveSymbol(t, rootScope, typ)) {
			t.Errorf("front's object is not a %s, want it classified by both types", typ)
		}
	}
}

// TestFeaturesOf_StatedTypeStandsWhenNotSpecialized: an inherited type refines the stated
// one only when it specializes it; a state usage typed by another keeps that usage.
func TestFeaturesOf_StatedTypeStandsWhenNotSpecialized(t *testing.T) {
	code := `
		state Machine { entry; then idle; state idle; }
		part def Host { exhibit state machine : Machine; }
	`
	model, resolver, rootScope := parseAndBuildLibraryModel(t, code)
	ctx := NewContext(typedModel(model, resolver), testMaxSteps)

	machine := effectiveFeatureNamed(t, ctx.FeaturesOf(resolveSymbol(t, rootScope, "Host")), "machine")
	if machine.Type == nil || machine.Type.Name != "Machine" {
		t.Errorf("machine.Type = %v, want the stated Machine", machine.Type)
	}
}

// TestFeaturesOf_ParameterKeepsItsStatedType: a parameter's positional redefinition of a
// general parameter contributes a type only when the parameter states none.
func TestFeaturesOf_ParameterKeepsItsStatedType(t *testing.T) {
	code := `
		private import ScalarValues::*;
		action def Base { out n : Natural; out m : Natural; }
		action def Derived :> Base { out r : Real; out s; }
	`
	model, resolver, rootScope := parseAndBuildLibraryModel(t, code)
	ctx := NewContext(typedModel(model, resolver), testMaxSteps)

	features := ctx.FeaturesOf(resolveSymbol(t, rootScope, "Derived"))
	if r := effectiveFeatureNamed(t, features, "r"); r.Type == nil || r.Type.Name != "Real" {
		t.Errorf("r.Type = %v, want the stated Real", r.Type)
	}
	if s := effectiveFeatureNamed(t, features, "s"); s.Type == nil || s.Type.Name != "Natural" {
		t.Errorf("s.Type = %v, want Natural from the parameter it redefines", s.Type)
	}
}

func effectiveFeatureNamed(t *testing.T, features []EffectiveFeature, name string) EffectiveFeature {
	t.Helper()
	for _, f := range features {
		if f.Name == name {
			return f
		}
	}
	t.Fatalf("no feature %s in %v", name, sortedFeatureNames(features))
	return EffectiveFeature{}
}
