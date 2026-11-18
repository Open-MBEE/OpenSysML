package passes

import (
	"fmt"
	"testing"
)

// redefinitionCodes are the diagnostics the redefinition featuring rules report.
var redefinitionCodes = []string{
	"redefinition-no-inherited", "subsetting-featuring-types",
	"redefinition-same-featuring", "redefinition-package-level",
}

func wantRedefinitionCodes(t *testing.T, src string, want ...string) {
	t.Helper()
	diags := constraintDiagsKerML(t, src)
	for _, code := range redefinitionCodes {
		expected := false
		for _, w := range want {
			expected = expected || w == code
		}
		if hasCode(diags, code) != expected {
			t.Errorf("%s\n%s reported = %v, want %v: %v", src, code, !expected, expected, diags)
		}
	}
}

// A classifier nested in a type is not featured by it, so its features cannot access what the
// enclosing type inherits (KerML 1.1 §8.3.3.3.4 Feature::canAccess, §8.3.3.3.10).
func TestRedefinitionFromNestedClassifierIsInaccessible(t *testing.T) {
	wantRedefinitionCodes(t,
		`class A { feature x; } class R specializes A { class Inner { feature n redefines x; } }`,
		"redefinition-no-inherited")
}

// The owning type is a featuring type beside the declared ones (deriveFeatureFeaturingType), so a
// feature of U featured by A has {A, U}, which differs from the {A} of the feature it redefines.
func TestRedefinitionFeaturedByTheRedefinedOwnerIsClean(t *testing.T) {
	wantRedefinitionCodes(t,
		`class A { feature x; } class U specializes A { feature u1 redefines x featured by A; }`)
}

// A `member` feature is owned by no FeatureMembership, so its declared featuring types are all it has.
func TestRedefinitionMemberFeatureHasOnlyDeclaredFeaturing(t *testing.T) {
	wantRedefinitionCodes(t,
		`class A { feature x; } class U specializes A { member feature u1 redefines x featured by A; }`,
		"redefinition-same-featuring")
	wantRedefinitionCodes(t,
		`class A { feature x; } class C specializes A; class H { member feature n redefines A::x featured by C; }`)
	wantRedefinitionCodes(t,
		`class A { feature x; } class H specializes A { member feature n redefines x; }`,
		"subsetting-featuring-types")
}

// validateRedefinitionFeaturingTypes reads a variable feature's featuring types as its owning type
// alone and adds Anything to any other feature's, so {CL1, Anything} differs from {CL1}.
func TestRedefinitionOfVariableFeatureComparesOwningType(t *testing.T) {
	const occ = `private import Occurrences::*; class CL1 specializes Occurrence { var feature v1; feature f; } `
	wantRedefinitionCodes(t, occ+`feature g redefines CL1::v1 featured by CL1;`)
	wantRedefinitionCodes(t, occ+`class CL2 specializes CL1 { var feature w redefines v1; feature u redefines f; }`)
	wantRedefinitionCodes(t, occ+`class CL2 specializes CL1 { feature w redefines v1; var feature u redefines f; }`)
	wantRedefinitionCodes(t, occ+`class CL2 specializes CL1 { var feature vn { feature m redefines v1; feature m2 redefines f; } }`)
	wantRedefinitionCodes(t, occ+`class CL3 specializes Occurrence { var feature v3 redefines CL1::v1; }`,
		"redefinition-no-inherited")
}

// Anything declared as a featuring type is the Anything every feature is implicitly featured by.
func TestRedefinitionDeclaredAnythingIsTheImplicitFeaturingType(t *testing.T) {
	wantRedefinitionCodes(t,
		`feature top; feature other redefines top featured by Base::Anything;`,
		"redefinition-same-featuring")
}

// The featuring types of featuring features are walked, whatever the nesting depth.
func TestRedefinitionThroughNestedFeatures(t *testing.T) {
	wantRedefinitionCodes(t,
		`class A { feature x; } class C specializes A; class P { feature c : C { feature d : A { feature e redefines x; } } }`)
	wantRedefinitionCodes(t,
		`class Q { feature x1; feature c { feature d { feature e redefines x1; } } }`)
	wantRedefinitionCodes(t,
		`class D { feature h { feature k; } feature i subsets h { feature m redefines k; } }`)
	wantRedefinitionCodes(t,
		`class D { feature h { feature k; } feature i { feature m redefines h::k; } }`,
		"redefinition-no-inherited")
	wantRedefinitionCodes(t,
		`class A { feature x; } feature b { feature z redefines A::x; }`,
		"redefinition-no-inherited")
}

// A redefined chain is featured as its first chaining feature is.
func TestRedefinitionOfChainUsesFirstChainingFeature(t *testing.T) {
	wantRedefinitionCodes(t,
		`class A { feature x : A; } class C { feature a : A; feature q { feature r redefines a.x; } }`)
	wantRedefinitionCodes(t,
		`class A { feature x : A; } class C { feature a : A; } class D { feature r redefines C::a.x; }`,
		"redefinition-no-inherited")
}

// A ReferenceSubsetting is a Subsetting, so a feature specializes the feature it references and is
// compatible with that feature's role as a featuring type (KerML 1.1 §8.3.3.3.9, Type::isCompatibleWith).
func TestRedefinitionThroughReferenceSubsetting(t *testing.T) {
	wantRedefinitionCodes(t,
		`feature battery { feature output { feature voltage; } }
		 class C { feature e references battery { feature r redefines output.voltage; feature s redefines output; } }`)
	wantRedefinitionCodes(t,
		`feature battery { feature output; } feature other;
		 class C { feature e references other { feature s redefines battery::output; } }`,
		"redefinition-no-inherited")
}

// A `member` feature is no ownedFeature, so two features holding only such members stay compatible
// through the feature they both redefine (KerML 1.1 §8.3.3.1.10 Feature::isCompatibleWith).
func TestRedefinitionThroughFeaturesWithOnlyMemberFeatures(t *testing.T) {
	const model = `class A { feature p; }
		 class B1 specializes A { feature p2 redefines p { %s feature q; } member feature x featured by p2; }
		 class B2 specializes B1 { feature p3 redefines A::p { %s feature r; } member feature y redefines x featured by p3; }`
	wantRedefinitionCodes(t, fmt.Sprintf(model, "member", "member"))
	wantRedefinitionCodes(t, fmt.Sprintf(model, "", ""), "redefinition-no-inherited")
}

// A Subsetting is held to the same Feature::canAccess as a Redefinition (KerML 1.1 §8.3.3.3.10
// validateSubsettingFeaturingTypes): a `member` feature has only the featuring types it declares.
func TestSubsettingUsesDerivedFeaturingTypes(t *testing.T) {
	wantRedefinitionCodes(t,
		`class A { feature x; } class U specializes A { member feature u subsets x; }`,
		"subsetting-featuring-types")
	wantRedefinitionCodes(t,
		`class A { feature x; } class U specializes A { member feature u subsets x featured by A; }`)
	wantRedefinitionCodes(t,
		`feature battery { feature output; }
		 class C { feature e references battery { feature s subsets output; } }`)
	wantRedefinitionCodes(t,
		`class A { feature x; } class R specializes A { class Inner { feature n subsets x; } }`,
		"subsetting-featuring-types")
}
