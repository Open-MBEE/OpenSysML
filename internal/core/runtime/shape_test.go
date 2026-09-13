package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
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
	ctx := NewContext(NewModel(model, resolver), testMaxSteps)

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
	ctx := NewContext(NewModel(model, resolver), testMaxSteps)

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
	ctx := NewContext(NewModel(model, resolver), testMaxSteps)

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
	ctx := NewContext(NewModel(model, resolver), testMaxSteps)

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
	ctx := NewContext(NewModel(model, resolver), testMaxSteps)

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
