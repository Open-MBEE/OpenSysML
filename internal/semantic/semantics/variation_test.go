package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// member looks up a member of a symbol, for a variant declared in a variation.
func member(t *testing.T, m *Model, owner *symbols.Symbol, name string) *symbols.Symbol {
	t.Helper()
	got, ok := m.LookupMember(owner, name)
	if !ok {
		t.Fatalf("member %q of %s not found", name, owner.Name)
	}
	return got
}

const variationSource = `
	part def Diamond { attribute cut; }
	abstract part family : Diamond {
		variation attribute :>> cut {
			variant attribute cutShallow { attribute cost = 200.0; }
			variant attribute cutIdeal { attribute cost = 250.0; }
		}
	}
	part chosen :> family { attribute :>> cut = cut::cutIdeal; }
`

func TestVariationAndVariantModifiers(t *testing.T) {
	m, root := buildModel(t, variationSource)
	family := sym(t, root, "family")
	cut := member(t, m, family, "cut")
	ideal := member(t, m, cut, "cutIdeal")

	if !DeclaresVariation(cut) {
		t.Error("the redefinition of cut declares `variation`")
	}
	if DeclaresVariant(cut) {
		t.Error("a variation is not itself a variant")
	}
	if !DeclaresVariant(ideal) {
		t.Error("cutIdeal declares `variant`")
	}
	if got := VariationOwning(ideal); got != cut {
		t.Errorf("VariationOwning(cutIdeal) = %v, want cut", got)
	}
	if got := VariationOwning(cut); got != nil {
		t.Errorf("VariationOwning(cut) = %v, want nil", got)
	}
	if VariantValue(ideal) != nil {
		t.Error("cutIdeal declares no value, so it stands for an object of itself")
	}
}

// An enumeration definition is a variation without writing the modifier, and
// its values are variants of it; a usage typed by it holds a value, not a choice.
func TestIsVariationIncludesEnumerationDefinitions(t *testing.T) {
	m, root := buildModel(t, `
		enum def Color { red; green; }
		attribute def Plain;
		attribute c : Color;
	`)
	color := sym(t, root, "Color")
	if !IsVariation(color) {
		t.Error("an enumeration definition is a variation")
	}
	if DeclaresVariation(color) {
		t.Error("an enumeration definition does not declare the modifier")
	}
	if IsVariation(sym(t, root, "Plain")) || IsVariation(nil) {
		t.Error("an attribute definition, or nothing, is no variation")
	}
	if m.IsVariationFeature(sym(t, root, "c")) {
		t.Error("an enumeration-typed attribute is not a variation point")
	}
	if EnumerationDefinitionOwning(member(t, m, color, "red")) != color {
		t.Error("red is owned by Color")
	}
	if EnumerationDefinitionOwning(color) != nil {
		t.Error("Color's owner is not an enumeration definition")
	}
	red := member(t, m, color, "red")
	if !IsVariant(red) || DeclaresVariant(red) {
		t.Error("an enumerated value is a variant without declaring `variant`")
	}
	if IsVariant(sym(t, root, "c")) || IsVariant(nil) {
		t.Error("an enumeration-typed attribute, or nothing, is no variant")
	}
	if !m.IsVariationFeature(color) {
		t.Error("an enumeration definition is a variation point")
	}
	if VariationOwning(red) != color || m.VariationPointOwning(red) != color {
		t.Error("an enumerated value is a variant of the enumeration owning it")
	}
	if m.VariationPointOwning(sym(t, root, "c")) != nil {
		t.Error("an enumeration-typed attribute is a variant of nothing")
	}
}

// The variant queries the runtime and solver select through offer an
// enumeration's values as its variants, inherited ones included, and let a
// usage typed by the enumeration select any of them.
func TestVariantsOfAnEnumeration(t *testing.T) {
	m, root := buildModel(t, `
		package Paint {
			enum def Finish { matte; gloss; }
		}
		variation attribute def Cut { variant attribute ideal; }
		part def Panel {
			attribute finish : Paint::Finish;
			enum def Coat { thin; thick; }
		}
	`)
	finish := member(t, m, sym(t, root, "Paint"), "Finish")
	matte, gloss := member(t, m, finish, "matte"), member(t, m, finish, "gloss")
	if variants := m.VariantsOf(finish); len(variants) != 2 || variants[0] != matte || variants[1] != gloss {
		t.Fatalf("VariantsOf(Finish) = %v, want matte then gloss", variants)
	}
	if got, ok := m.VariantOf(finish, "gloss"); !ok || got != gloss {
		t.Errorf("VariantOf(Finish, gloss) = %v, %v", got, ok)
	}
	if _, ok := m.VariantOf(finish, "satin"); ok {
		t.Error("Finish offers no satin")
	}
	if !m.SelectsVariantOf(finish, matte) {
		t.Error("an enumeration selects among its own values")
	}
	panel := sym(t, root, "Panel")
	attr := member(t, m, panel, "finish")
	if !m.SelectsVariantOf(attr, gloss) {
		t.Error("a usage typed by an enumeration selects among its values")
	}
	if m.SelectsVariantOf(finish, member(t, m, sym(t, root, "Cut"), "ideal")) {
		t.Error("a variant of another variation is not one of Finish")
	}
	coat := member(t, m, panel, "Coat")
	if variants := m.VariantsOf(coat); len(variants) != 2 {
		t.Errorf("VariantsOf(Panel::Coat) = %v, want thin and thick", variants)
	}
	if m.SelectsVariantOf(attr, member(t, m, coat, "thin")) {
		t.Error("a value of a nested enumeration is not a value of Finish")
	}
}

// The metaclass features read by element filters and queries report the derived
// value, so an enumeration definition is a variation and its values variants.
func TestReflectiveVariationFeaturesOfEnumerations(t *testing.T) {
	m, root := buildModel(t, `
		enum def Color { red; green; }
		attribute def Plain;
		variation attribute def Cut { variant attribute ideal; attribute cost; }
	`)
	color := sym(t, root, "Color")
	cut := sym(t, root, "Cut")
	want := []struct {
		sym     *symbols.Symbol
		feature string
		value   bool
	}{
		{color, "isVariation", true},
		{sym(t, root, "Plain"), "isVariation", false},
		{cut, "isVariation", true},
		{member(t, m, color, "red"), "isVariant", true},
		{member(t, m, cut, "ideal"), "isVariant", true},
		{member(t, m, cut, "cost"), "isVariant", false},
	}
	for _, tc := range want {
		got, ok := m.ReflectiveFeatureValue(tc.sym, tc.feature)
		if !ok {
			t.Fatalf("%s::%s is not derived", tc.sym.Name, tc.feature)
		}
		if got.Kind != symbols.FilterValueBool || got.Bool != tc.value {
			t.Errorf("%s::%s = %+v, want %v", tc.sym.Name, tc.feature, got, tc.value)
		}
	}
}

// A variation point is the feature declaring the modifier and any feature
// specializing it, since a redefinition does not restate the modifier.
func TestIsVariationFeatureThroughSpecialization(t *testing.T) {
	m, root := buildModel(t, variationSource)
	family := sym(t, root, "family")
	cut := member(t, m, family, "cut")
	chosenCut := member(t, m, sym(t, root, "chosen"), "cut")

	if !m.IsVariationFeature(cut) {
		t.Error("cut declares `variation`")
	}
	if !m.IsVariationFeature(chosenCut) {
		t.Error("the redefinition of cut in chosen is a variation point too")
	}
	if m.IsVariationFeature(member(t, m, sym(t, root, "Diamond"), "cut")) {
		t.Error("the definition's plain cut is not a variation point")
	}
}

// The variants a feature offers are those of the variation it specializes, so a
// specialization selects among the same choices as the family it refines.
func TestVariantsOfInheritedThroughRedefinition(t *testing.T) {
	m, root := buildModel(t, variationSource)
	family := sym(t, root, "family")
	cut := member(t, m, family, "cut")
	chosenCut := member(t, m, sym(t, root, "chosen"), "cut")

	for _, owner := range []*symbols.Symbol{cut, chosenCut} {
		variants := m.VariantsOf(owner)
		if len(variants) != 2 ||
			variants[0].Name != "cutShallow" || variants[1].Name != "cutIdeal" {
			t.Fatalf("VariantsOf(%s) = %v, want cutShallow then cutIdeal", owner.Name, variants)
		}
		ideal, ok := m.VariantOf(owner, "cutIdeal")
		if !ok || ideal.Name != "cutIdeal" {
			t.Errorf("VariantOf(%s, cutIdeal) = (%v, %v)", owner.Name, ideal, ok)
		}
		if _, ok := m.VariantOf(owner, "nope"); ok {
			t.Errorf("VariantOf(%s, nope) found a variant", owner.Name)
		}
		if !m.SelectsVariantOf(owner, ideal) {
			t.Errorf("%s may be bound to cutIdeal", owner.Name)
		}
	}
}

// A `variant` inherited from a type that is not a variation point offers no
// choice, so the offered list agrees with what a selection accepts.
func TestVariantsOfExcludesAMisplacedInheritedVariant(t *testing.T) {
	m, root := buildModel(t, `
		attribute def Base { variant attribute misplaced = 1.0; }
		part def Widget {
			variation attribute pick : Base { variant attribute cheap = 2.0; }
		}
	`)
	pick := member(t, m, sym(t, root, "Widget"), "pick")
	misplaced := member(t, m, pick, "misplaced")

	variants := m.VariantsOf(pick)
	if len(variants) != 1 || variants[0].Name != "cheap" {
		t.Fatalf("VariantsOf(pick) = %v, want cheap alone", variants)
	}
	if _, ok := m.VariantOf(pick, "misplaced"); ok {
		t.Error("VariantOf(pick, misplaced) offered a variant of Base")
	}
	if m.SelectsVariantOf(pick, misplaced) {
		t.Error("pick may not be bound to a variant of Base")
	}
}

// A variant of another variation is not a choice a feature may be bound to.
func TestSelectsVariantOfRejectsForeignVariant(t *testing.T) {
	m, root := buildModel(t, `
		part def Diamond { attribute cut; attribute color; }
		abstract part family : Diamond {
			variation attribute :>> cut {
				variant attribute cutIdeal { attribute cost = 250.0; }
			}
			variation attribute :>> color {
				variant attribute colorWhite { attribute cost = 100.0; }
			}
		}
	`)
	family := sym(t, root, "family")
	cut := member(t, m, family, "cut")
	white := member(t, m, member(t, m, family, "color"), "colorWhite")

	if m.SelectsVariantOf(cut, white) {
		t.Error("colorWhite is a variant of color, not of cut")
	}
	if m.SelectsVariantOf(cut, cut) {
		t.Error("a variation is not a variant of itself")
	}
	if m.SelectsVariantOf(nil, white) || m.SelectsVariantOf(cut, nil) {
		t.Error("a missing symbol selects nothing")
	}
}
