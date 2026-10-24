package runtime

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessVariations covers variations and variants: unselected options, chains
// through them and selections made by two owners or none.
func TestRuntimeRobustnessVariations(t *testing.T) {
	t.Run("variation_without_a_selected_variant", testVariationWithoutASelectedVariant)
	t.Run("variation_bound_to_what_is_not_a_variant", testVariationBoundToWhatIsNotAVariant)
	t.Run("variation_bound_to_two_variants", testVariationBoundToTwoVariants)
	t.Run("variation_read_through_its_declaration", testVariationReadThroughItsDeclaration)
	t.Run("chain_through_an_unselected_variation_part", testChainThroughAnUnselectedVariationPart)
	t.Run("classify_an_unselected_optional_variation", testClassifyAnUnselectedOptionalVariation)
	t.Run("repeated_reads_of_a_variant_object", testRepeatedReadsOfAVariantObject)
	t.Run("two_owners_selecting_one_variant", testTwoOwnersSelectingOneVariant)
	t.Run("two_ownerless_selections_of_one_variant", testTwoOwnerlessSelectionsOfOneVariant)
	t.Run("variant_outside_a_variation", testVariantOutsideAVariation)
	t.Run("variant_under_a_redefined_variation", testVariantUnderARedefinedVariation)
	t.Run("chain_through_a_variant_reading_an_unselected_variation", testChainThroughAVariantReadingAnUnselectedVariation)
}

// testVariationWithoutASelectedVariant: a variation nothing selects a variant
// for has no value, so reading it says so rather than answering the variation's
// own empty object or one of the variants arbitrarily.
func testVariationWithoutASelectedVariant(t *testing.T) {
	src := fmt.Sprintf(variationFamily, `part unconfigured :> family;`)
	got, err := variationFeatureValueInSource(t, src, "test::unconfigured", "cut")
	if !errors.Is(err, ErrVariationUnselected) {
		t.Errorf("cut = (%v, %v), want ErrVariationUnselected", got, err)
	}
	// The failure names the feature, so a model with many variation points says
	// which one is unconfigured.
	if err != nil && !strings.Contains(err.Error(), "cut") {
		t.Errorf("error %q does not name the variation", err)
	}
}

// variationReadFromDeclaration evaluates a usage's value with no bound object,
// so a variation is read through its declaration rather than through a feature value.
func variationReadFromDeclaration(t *testing.T, src, probe string) (Value, error) {
	t.Helper()
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	matches := idx.LookupQualified(probe)
	if len(matches) != 1 {
		t.Fatalf("%s: %d matching symbols, want 1", probe, len(matches))
	}
	usage, ok := matches[0].Decl.(*ast.Usage)
	if !ok || usage.Value == nil {
		t.Fatalf("%s: no value expression", probe)
	}
	return ctx.EvalWithScope(usage.Value, matches[0].OwnerScope)
}

// testVariationReadThroughItsDeclaration: a variation read without a bound
// object is bound the same way as one read from a feature value, so what a legal
// selection is does not depend on how the model is inspected.
func testVariationReadThroughItsDeclaration(t *testing.T) {
	for _, tt := range []struct {
		name, decl, probe string
		want              error
	}{
		{
			"not_a_variant",
			`part chosen :> family { attribute :>> cut = 250.0; attribute probe = cut; }`,
			"test::chosen::probe", errNotAVariant,
		},
		{
			"two_variants",
			`part chosen :> family { attribute :>> cut = (cut::cutIdeal, cut::cutShallow); attribute probe = cut; }`,
			"test::chosen::probe", errMultipleVariants,
		},
		{
			"unselected",
			`part chosen :> family { attribute probe = cut; }`,
			"test::chosen::probe", ErrVariationUnselected,
		},
		{
			"qualified_not_a_variant",
			`part chosen :> family { attribute :>> cut = 250.0; }
			 attribute probe = chosen::cut;`,
			"test::probe", errNotAVariant,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := variationReadFromDeclaration(t, fmt.Sprintf(variationFamily, tt.decl), tt.probe)
			if !errors.Is(err, tt.want) {
				t.Errorf("%s = (%v, %v), want %v", tt.probe, got, err, tt.want)
			}
		})
	}
}

// testChainThroughAnUnselectedVariationPart: a variation part is no occurrence
// of itself, so a chain through one nothing selected a variant for reports that
// rather than reading an object of the variation.
func testChainThroughAnUnselectedVariationPart(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::Real;
		part def Engine { attribute power : Real; }
		variation part engine : Engine {
			variant part electric : Engine { attribute :>> power = 100.0; }
			variant part diesel : Engine { attribute :>> power = 200.0; }
		}
		part probe { attribute p : Real = engine.power; }
	}`
	got, err := variationFeatureValueInSource(t, src, "test::probe", "p")
	if !errors.Is(err, ErrVariationUnselected) {
		t.Errorf("p = (%v, %v), want ErrVariationUnselected", got, err)
	}
}

// testClassifyAnUnselectedOptionalVariation: an optional variation nothing selects
// a variant for is a choice, not an empty collection, so `istype` reports that
// rather than vacuously matching every type.
func testClassifyAnUnselectedOptionalVariation(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		part def Engine;
		part def Motor :> Engine;
		variation part engine : Engine[0..1] {
			variant part electric : Motor;
			variant part diesel : Engine;
		}
		attribute isMotor : Boolean = engine istype Motor;
		attribute hasMotor : Boolean = engine hastype Motor;
	}`
	for _, probe := range []string{"test::isMotor", "test::hasMotor"} {
		got, err := variationReadFromDeclaration(t, src, probe)
		if !errors.Is(err, ErrVariationUnselected) {
			t.Errorf("%s = (%v, %v), want ErrVariationUnselected", probe, got, err)
		}
	}
}

// testVariationBoundToWhatIsNotAVariant: a selection naming something that is
// not a variant of the variation is reported, whether the name is unknown, a
// variant of another variation, or an ordinary value.
func testVariationBoundToWhatIsNotAVariant(t *testing.T) {
	for _, tt := range []struct{ name, selection string }{
		{"unknown_name", `part chosen :> family { attribute :>> cut = cut::nope; }`},
		{"variant_of_another_variation", `part chosen :> family { attribute :>> cut = color::colorWhite; }`},
		{"ordinary_value", `part chosen :> family { attribute :>> cut = 250.0; }`},
		{"collection_of_ordinary_values", `part chosen :> family { attribute :>> cut = (250.0, 200.0); }`},
		{"variant_mixed_with_an_ordinary_value", `part chosen :> family { attribute :>> cut = (cut::cutIdeal, 250.0); }`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := variationFeatureValueInSource(t, fmt.Sprintf(variationFamily, tt.selection), "test::chosen", "cut")
			if !errors.Is(err, errNotAVariant) {
				t.Errorf("cut = (%v, %v), want errNotAVariant", got, err)
			}
		})
	}
}

// testVariationBoundToTwoVariants: a variation stands for one variant, so a
// selection of several is reported rather than silently taking the first.
func testVariationBoundToTwoVariants(t *testing.T) {
	src := fmt.Sprintf(variationFamily,
		`part chosen :> family { attribute :>> cut = (cut::cutIdeal, cut::cutShallow); }`)
	got, err := variationFeatureValueInSource(t, src, "test::chosen", "cut")
	if !errors.Is(err, errMultipleVariants) {
		t.Fatalf("cut = (%v, %v), want errMultipleVariants", got, err)
	}
	for _, name := range []string{"cutIdeal", "cutShallow"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not name the selection %s", err, name)
		}
	}
}

// testRepeatedReadsOfAVariantObject: the object a selected variant stands for is
// materialized once, so evaluating a chain through it repeatedly neither piles up
// objects nor exhausts the step budget.
func testRepeatedReadsOfAVariantObject(t *testing.T) {
	src := `
	package test {
		part def Engine { attribute power; }
		abstract part family {
			variation part engine : Engine {
				variant part electric : Engine { attribute :>> power = 150.0; }
				variant part petrol : Engine { attribute :>> power = 120.0; }
			}
		}
		part chosen :> family { part :>> engine = engine::electric; }
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	variant := oneSymbol(t, idx, "test::family::engine::electric")
	variation := oneSymbol(t, idx, "test::family::engine")
	first, err := ctx.variantValue(variation, variant, 1)
	if err != nil {
		t.Fatalf("variantValue: %v", err)
	}
	count := len(ctx.instances)
	for i := 0; i < 10; i++ {
		again, err := ctx.variantValue(variation, variant, 1)
		if err != nil {
			t.Fatalf("variantValue (read %d): %v", i+2, err)
		}
		if again.Instance != first.Instance {
			t.Fatalf("read %d gave instance %d, want %d", i+2, again.Instance, first.Instance)
		}
	}
	if len(ctx.instances) != count {
		t.Errorf("instances grew from %d to %d over repeated reads", count, len(ctx.instances))
	}
}

// testTwoOwnersSelectingOneVariant: a variant is selected per owning object, so
// two owners of one variation each hold their own object of the variant rather
// than sharing one whose materialized feature values the other reads.
func testTwoOwnersSelectingOneVariant(t *testing.T) {
	src := `
	package test {
		part def Engine { attribute power; }
		abstract part family {
			variation part engine : Engine {
				variant part electric : Engine { attribute :>> power = 150.0; }
				variant part petrol : Engine { attribute :>> power = 120.0; }
			}
		}
		part sedan :> family { part :>> engine = engine::electric; }
		part coupe :> family { part :>> engine = engine::electric; }
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	ids := make([]int64, 0, 2)
	for _, usage := range []string{"test::sedan", "test::coupe"} {
		inst, err := ctx.Instantiate(oneSymbol(t, idx, usage))
		if err != nil {
			t.Fatalf("%s: %v", usage, err)
		}
		fv, err := inst.GetFeatureValue(ctx, "engine")
		if err != nil {
			t.Fatalf("%s.engine: %v", usage, err)
		}
		id, ok := fv.Value.Object()
		if !ok {
			t.Fatalf("%s.engine = %v, want an object of the selected variant", usage, fv.Value)
		}
		ids = append(ids, id)
	}
	if ids[0] == ids[1] {
		t.Errorf("sedan and coupe share engine object %d", ids[0])
	}
}

// testTwoOwnerlessSelectionsOfOneVariant: a variation read through its
// declaration has no owning object, so two variation points selecting one
// variant must still stand for an object each rather than share one.
func testTwoOwnerlessSelectionsOfOneVariant(t *testing.T) {
	src := `
	package test {
		part def Engine { attribute power; }
		abstract part family {
			variation part engine : Engine {
				variant part electric : Engine { attribute :>> power = 150.0; }
			}
		}
		part sedan :> family { part :>> engine = engine::electric; }
		part coupe :> family { part :>> engine = engine::electric; }
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	variant := oneSymbol(t, idx, "test::family::engine::electric")
	ids := make([]int64, 0, 2)
	for _, variation := range []string{"test::sedan::engine", "test::coupe::engine"} {
		val, err := ctx.variantValue(oneSymbol(t, idx, variation), variant, 0)
		if err != nil {
			t.Fatalf("%s: %v", variation, err)
		}
		ids = append(ids, val.Instance)
	}
	if ids[0] == ids[1] {
		t.Errorf("sedan.engine and coupe.engine share object %d", ids[0])
	}
}

// testVariantOutsideAVariation: `variant` on a member whose owner is not a
// variation offers no choice, so the member stays an ordinary feature instead of
// silently holding no value.
func testVariantOutsideAVariation(t *testing.T) {
	src := `
	package test {
		part def Widget { variant attribute misplaced = 1.0; }
		part widget : Widget;
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "test::widget"))
	if err != nil {
		t.Fatalf("Instantiate(test::widget): %v", err)
	}
	fv, err := inst.GetFeatureValue(ctx, "misplaced")
	if err != nil {
		t.Fatalf("widget.misplaced: %v", err)
	}
	if fv.Value.Kind != ValConst || fv.Value.Const.AsReal() != 1.0 {
		t.Errorf("widget.misplaced = %v, want 1", fv.Value)
	}
}

// testVariantUnderARedefinedVariation: a usage redefining a variation usage is a
// variation point without restating the modifier, so the variants under it stay
// choices that specialize it instead of materializing feature values.
func testVariantUnderARedefinedVariation(t *testing.T) {
	src := `
	package test {
		part def Engine { attribute power; }
		abstract part family {
			variation part engine : Engine {
				variant part petrol : Engine { attribute :>> power = 90.0; }
			}
		}
		abstract part refined :> family {
			part :>> engine {
				variant part electric { attribute :>> power = 150.0; }
			}
		}
		part sedan :> refined { part :>> engine = engine::electric; }
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "test::sedan"))
	if err != nil {
		t.Fatalf("Instantiate(test::sedan): %v", err)
	}
	if fv, err := inst.GetFeatureValue(ctx, "electric"); err == nil {
		t.Errorf("sedan.electric materialized a feature value: %v", fv.Value)
	}
	fv, err := inst.GetFeatureValue(ctx, "engine")
	if err != nil {
		t.Fatalf("sedan.engine: %v", err)
	}
	if fv.Value.Kind != ValVariant || fv.Value.Instance == 0 {
		t.Fatalf("sedan.engine = %v, want the selected variant's object", fv.Value)
	}
	power, err := ctx.instances[fv.Value.Instance].GetFeatureValue(ctx, "power")
	if err != nil {
		t.Fatalf("sedan.engine.power: %v", err)
	}
	if power.Value.Kind != ValConst || power.Value.Const.AsReal() != 150.0 {
		t.Errorf("sedan.engine.power = %v, want 150", power.Value)
	}
}

// testChainThroughAVariantReadingAnUnselectedVariation: a variant named with no
// selecting object is read through the object it stands for, so a member of it
// that reads an unselected variation fails as that read does, naming the
// variation, rather than failing to chain through the variant at all.
func testChainThroughAVariantReadingAnUnselectedVariation(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::Real;
		part def Engine { attribute power : Real; }
		variation part engine : Engine {
			variant part electric : Engine { attribute :>> power = 100.0; }
			variant part diesel : Engine { attribute :>> power = 200.0; }
		}
		part def Body { attribute m : Real; }
		variation part def Case :> Body {
			variant part powered : Case { attribute :>> m = engine.power; }
			variant part plain : Case { attribute :>> m = 1.0; }
		}
		part probe {
			attribute p : Real = Case::powered.m;
			attribute q : Real = Case::plain.m;
		}
	}`
	got, err := variationFeatureValueInSource(t, src, "test::probe", "p")
	if !errors.Is(err, ErrVariationUnselected) {
		t.Errorf("p = (%v, %v), want ErrVariationUnselected", got, err)
	}
	if err != nil && !strings.Contains(err.Error(), "engine") {
		t.Errorf("error %q does not name the unselected variation", err)
	}
	// The sibling variant reads through all the same.
	got, err = variationFeatureValueInSource(t, src, "test::probe", "q")
	if err != nil || got.Kind != ValConst || got.Const.AsReal() != 1.0 {
		t.Errorf("q = (%v, %v), want 1.0", got, err)
	}
}
