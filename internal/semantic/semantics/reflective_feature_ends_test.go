package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Feature::owningFeatureMembership, endOwningType, ownedTypeFeaturing, ownedFeatureInverting,
// ownedFeatureChaining, featureTarget and crossFeature on meta values (KerML 1.1 §8.3.3.3.4).
func TestReflectiveFeatureEnds(t *testing.T) {
	const src = `package P {
    class A { feature sub : B; }
    class B;
    class C {
        feature f : A;
        feature g crosses f;
        feature g2 crosses f.sub;
        feature h : B subsets f.sub;
        feature k : A featured by A inverse of A::sub;
        end feature e : A;
    }
    feature top : A;
}`
	for _, fixture := range reflectiveFixtures(t, "feature_ends.kerml", source.KindKerML, src) {
		model := fixture.model
		a := nestedSym(t, fixture.root, "P::A")
		sub := nestedSym(t, fixture.root, "P::A::sub")
		f := nestedSym(t, fixture.root, "P::C::f")
		g := nestedSym(t, fixture.root, "P::C::g")
		h := nestedSym(t, fixture.root, "P::C::h")
		k := nestedSym(t, fixture.root, "P::C::k")
		e := nestedSym(t, fixture.root, "P::C::e")
		top := nestedSym(t, fixture.root, "P::top")

		assertReflectiveElements(t, model, f, "crossFeature")
		assertReflectiveElements(t, model, g, "crossFeature")
		assertReflectiveElements(t, model, nestedSym(t, fixture.root, "P::C::g2"), "crossFeature", "P::A::sub")
		assertReflectiveElements(t, model, e, "crossFeature")

		assertReflectiveElements(t, model, f, "featureTarget", "P::C::f")
		assertReflectiveElements(t, model, f, "ownedFeatureChaining")
		_, chain := singleRelationship(t, model, h, "ownedSubsetting", "KerML::Core::Subsetting")
		assertReflectiveElements(t, model, chain, "featureTarget", "P::A::sub")
		if chainings := reflectiveElementsOf(t, model, chain, "ownedFeatureChaining"); len(chainings) != 2 {
			t.Errorf("chain f.sub ownedFeatureChaining = %d relationships, want 2", len(chainings))
		}

		featurings := reflectiveElementsOf(t, model, k, "ownedTypeFeaturing")
		if len(featurings) != 1 {
			t.Fatalf("k ownedTypeFeaturing = %d relationships, want 1", len(featurings))
		}
		assertRelMetaclass(t, model, featurings[0], "TypeFeaturing")
		assertRelFeature(t, model, featurings[0], "featuringType", a)
		invertings := reflectiveElementsOf(t, model, k, "ownedFeatureInverting")
		if len(invertings) != 1 {
			t.Fatalf("k ownedFeatureInverting = %d relationships, want 1", len(invertings))
		}
		assertRelMetaclass(t, model, invertings[0], "FeatureInverting")
		assertRelFeature(t, model, invertings[0], "invertingFeature", sub)
		assertReflectiveElements(t, model, f, "ownedTypeFeaturing")
		assertReflectiveElements(t, model, f, "ownedFeatureInverting")

		memberships := reflectiveElementsOf(t, model, f, "owningFeatureMembership")
		if len(memberships) != 1 || memberships[0] != model.OwningMembership(f) {
			t.Errorf("f owningFeatureMembership = %v, want its owning membership", fqns(memberships))
		} else {
			assertRelMetaclass(t, model, memberships[0], "FeatureMembership")
		}
		assertReflectiveElements(t, model, top, "owningFeatureMembership")

		assertReflectiveElements(t, model, e, "endOwningType", "P::C")
		assertReflectiveElements(t, model, f, "endOwningType")

		c := nestedSym(t, fixture.root, "P::C")
		for _, feature := range []string{"owningFeatureMembership", "endOwningType", "ownedTypeFeaturing",
			"ownedFeatureInverting", "ownedFeatureChaining", "featureTarget", "crossFeature"} {
			if _, ok := model.ReflectiveElements(c, feature); ok {
				t.Errorf("P::C.%s is derived; a class is no Feature", feature)
			}
		}
	}
}

// Usage::variantMembership and Definition::variantMembership are the VariantMemberships
// a variation owns; `variant` their variant usages (SysML v2 §8.3.6.4, §8.3.6.3).
func TestReflectiveVariants(t *testing.T) {
	const src = `package P {
    part def A;
    variation part def VA { variant part va1 : A; variant part va2 : A; }
    part def Sys {
        variation part v : A { variant part v1 : A; variant part v2 : A; part plain : A; }
        part x : A;
    }
    enum def Color { red; green; }
}`
	for _, fixture := range reflectiveFixtures(t, "variants.sysml", source.KindSysML, src) {
		model := fixture.model
		v := nestedSym(t, fixture.root, "P::Sys::v")
		memberships := reflectiveElementsOf(t, model, v, "variantMembership")
		if len(memberships) != 2 {
			t.Fatalf("v variantMembership = %d memberships, want 2", len(memberships))
		}
		for i, membership := range memberships {
			assertRelMetaclass(t, model, membership, "VariantMembership")
			if membership.Implicit == nil || membership.Implicit.Member == nil || membership.Implicit.Member.Name != []string{"v1", "v2"}[i] {
				t.Errorf("v variantMembership[%d] holds %s", i, fqns([]*symbols.Symbol{membership}))
			}
		}
		assertReflectiveElements(t, model, v, "variant", "P::Sys::v::v1", "P::Sys::v::v2")
		x := nestedSym(t, fixture.root, "P::Sys::x")
		assertReflectiveElements(t, model, x, "variantMembership")
		assertReflectiveElements(t, model, x, "variant")
		assertReflectiveElements(t, model, nestedSym(t, fixture.root, "P::VA"), "variant", "P::VA::va1", "P::VA::va2")
		assertReflectiveElements(t, model, nestedSym(t, fixture.root, "P::Color"), "variant", "P::Color::red", "P::Color::green")
		assertReflectiveElements(t, model, nestedSym(t, fixture.root, "P::A"), "variant")
		for _, feature := range []string{"variant", "variantMembership"} {
			if _, ok := model.ReflectiveElements(nestedSym(t, fixture.root, "P"), feature); ok {
				t.Errorf("P.%s is derived; a package is neither Usage nor Definition", feature)
			}
		}
	}
}
