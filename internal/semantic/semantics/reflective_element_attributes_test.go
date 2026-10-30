package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestReflectiveElementAttributesEveryElementCarries: aliasIds is none and
// isImpliedIncluded false on a definition, a usage, a package and a membership.
func TestReflectiveElementAttributesEveryElementCarries(t *testing.T) {
	const src = `package P {
		part def D;
		part u : D;
		package Q;
	}`
	for _, fixture := range reflectiveFixtures(t, "attributes.sysml", source.KindSysML, src) {
		for _, path := range []string{"P", "P::D", "P::u", "P::Q"} {
			sym := nestedSym(t, fixture.root, path)
			values, ok := fixture.model.ReflectiveFeatureValues(sym, "aliasIds")
			if !ok || len(values) != 0 {
				t.Errorf("%s.aliasIds = %v (present %t), want none", path, values, ok)
			}
			got, ok := fixture.model.ReflectiveFeatureValue(sym, "isImpliedIncluded")
			if !ok || got.Kind != symbols.FilterValueBool || got.Bool {
				t.Errorf("%s.isImpliedIncluded = %v (present %t), want false", path, got, ok)
			}
		}
		owner := nestedSym(t, fixture.root, "P")
		memberships, ok := fixture.model.ReflectiveElements(owner, "ownedMembership")
		if !ok || len(memberships) == 0 {
			t.Fatalf("P.ownedMembership = %v (present %t)", memberships, ok)
		}
		values, ok := fixture.model.ReflectiveFeatureValues(memberships[0], "aliasIds")
		if !ok || len(values) != 0 {
			t.Errorf("a membership's aliasIds = %v (present %t), want none", values, ok)
		}
		got, ok := fixture.model.ReflectiveFeatureValue(memberships[0], "isImpliedIncluded")
		if !ok || got.Bool {
			t.Errorf("a membership's isImpliedIncluded = %v (present %t), want false", got, ok)
		}
	}
}

// TestReflectiveRequirementReqIdIsTheDeclaredShortName: reqId redefines
// declaredShortName, so it is the short name or none.
func TestReflectiveRequirementReqIdIsTheDeclaredShortName(t *testing.T) {
	const src = `package P {
		requirement def <'R-1'> Mass;
		requirement <'R-1.1'> mass : Mass;
		requirement unnumbered : Mass;
		part def NotARequirement;
	}`
	for _, fixture := range reflectiveFixtures(t, "reqid.sysml", source.KindSysML, src) {
		for path, want := range map[string]string{"P::Mass": "R-1", "P::mass": "R-1.1"} {
			sym := nestedSym(t, fixture.root, path)
			got, ok := fixture.model.ReflectiveFeatureValue(sym, "reqId")
			if !ok || got.Kind != symbols.FilterValueString || got.Str != want {
				t.Errorf("%s.reqId = %v (present %t), want %q", path, got, ok, want)
			}
		}
		sym := nestedSym(t, fixture.root, "P::unnumbered")
		values, ok := fixture.model.ReflectiveFeatureValues(sym, "reqId")
		if !ok || len(values) != 0 {
			t.Errorf("unnumbered.reqId = %v (present %t), want none", values, ok)
		}
		if _, ok := fixture.model.ReflectiveFeatureValue(nestedSym(t, fixture.root, "P::NotARequirement"), "reqId"); ok {
			t.Errorf("a part definition answers reqId")
		}
	}
}

// TestReflectiveInvariantIsNegated: an `assert constraint` is an
// AssertConstraintUsage (an Invariant) whose isNegated is its `not`; a plain
// constraint usage is a ConstraintUsage and answers nothing.
func TestReflectiveInvariantIsNegated(t *testing.T) {
	const src = `package P {
		constraint def C;
		assert not constraint negated : C;
		assert constraint asserted : C;
		constraint plain : C;
	}`
	for _, fixture := range reflectiveFixtures(t, "negated.sysml", source.KindSysML, src) {
		for path, want := range map[string]bool{"P::negated": true, "P::asserted": false} {
			sym := nestedSym(t, fixture.root, path)
			if meta := fixture.model.MetaclassOf(sym); meta == nil || meta.Name != "AssertConstraintUsage" {
				t.Errorf("%s is a %v, want AssertConstraintUsage", path, meta)
			}
			got, ok := fixture.model.ReflectiveFeatureValue(sym, "isNegated")
			if !ok || got.Kind != symbols.FilterValueBool || got.Bool != want {
				t.Errorf("%s.isNegated = %v (present %t), want %t", path, got, ok, want)
			}
		}
		plain := nestedSym(t, fixture.root, "P::plain")
		if meta := fixture.model.MetaclassOf(plain); meta == nil || meta.Name != "ConstraintUsage" {
			t.Errorf("plain is a %v, want ConstraintUsage", meta)
		}
		if _, ok := fixture.model.ReflectiveFeatureValue(plain, "isNegated"); ok {
			t.Errorf("a plain constraint usage answers isNegated")
		}
	}
}

// TestReflectiveDeclaredConnectorAsARelationship: a connection usage is a
// Relationship that is not implied, owns no related element and is owned by
// none; its ends (reference usages declared by `connect`) are not sufficient.
func TestReflectiveDeclaredConnectorAsARelationship(t *testing.T) {
	const src = `package P {
		part a; part b;
		connection c connect a to b;
	}`
	for _, fixture := range reflectiveFixtures(t, "connector.sysml", source.KindSysML, src) {
		c := nestedSym(t, fixture.root, "P::c")
		got, ok := fixture.model.ReflectiveFeatureValue(c, "isImplied")
		if !ok || got.Kind != symbols.FilterValueBool || got.Bool {
			t.Errorf("c.isImplied = %v (present %t), want false", got, ok)
		}
		for _, feature := range []string{"ownedRelatedElement", "owningRelatedElement"} {
			elements, ok := fixture.model.ReflectiveElements(c, feature)
			if !ok || len(elements) != 0 {
				t.Errorf("c.%s = %v (present %t), want none", feature, elements, ok)
			}
		}
		ends, ok := fixture.model.OwnedConnectorEnds(c)
		if !ok || len(ends) != 2 {
			t.Fatalf("c has ends %v (connector %t), want two", ends, ok)
		}
		for i, end := range ends {
			if end == nil {
				t.Fatalf("end %d has no symbol", i)
			}
			got, ok := fixture.model.ReflectiveFeatureValue(end, "isSufficient")
			if !ok || got.Kind != symbols.FilterValueBool || got.Bool {
				t.Errorf("end %d isSufficient = %v (present %t), want false", i, got, ok)
			}
		}
		if _, ok := fixture.model.ReflectiveFeatureValue(nestedSym(t, fixture.root, "P::a"), "isImplied"); ok {
			t.Errorf("a part answers isImplied")
		}
	}
}
