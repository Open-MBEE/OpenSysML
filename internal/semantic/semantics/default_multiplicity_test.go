package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestDefaultMultiplicityMember checks that a usage whose multiplicity is the
// implicit [1..1] owns a Multiplicity stating it — its Type::multiplicity, the
// member of the last of its owned memberships, an OwningMembership — while a
// declared range, a subsetting usage, an action, a variant, an enumerated value,
// a redefinition of a package-level feature and a KerML feature own none.
func TestDefaultMultiplicityMember(t *testing.T) {
	src := `package DM {
    part def Vehicle {
        attribute mass : ScalarValues::Real;
        attribute tags : ScalarValues::String [2];
        part wheel : Wheel;
        part spare :> wheel;
        action go;
        part trim : Trim { variant part leather : Trim; }
        part cap redefines lid;
    }
    part lid : Wheel;
    part def Wheel;
    part def Trim;
    enum def Color { red; }
}`
	for _, fixture := range reflectiveFixtures(t, "default-multiplicity.sysml", source.KindSysML, src) {
		model := fixture.model
		mass := relFixtureSymbol(t, fixture, "mass")
		member := model.DefaultMultiplicityMember(mass)
		if member == nil {
			t.Fatal("DefaultMultiplicityMember(mass) = nil, want the Multiplicity stating its implicit [1..1]")
		}
		if again := model.DefaultMultiplicityMember(mass); again != member {
			t.Error("DefaultMultiplicityMember(mass) changed between reads")
		}
		if !IsDefaultMultiplicity(member) || !member.Implied() {
			t.Error("the member is not recognised as an implied default multiplicity")
		}
		if got := model.fqnOf(model.MetaclassOf(member)); got != "KerML::Core::Multiplicity" {
			t.Errorf("metaclass = %s, want KerML::Core::Multiplicity", got)
		}
		assertRelFeature(t, model, mass, "multiplicity", member)
		assertRelFeature(t, model, member, "owner", mass)
		if members, _ := model.ReflectiveElements(mass, "ownedMember"); len(members) != 1 || members[0] != member {
			t.Errorf("mass ownedMember = %d elements, want the default multiplicity alone", len(members))
		}
		if elements, _ := model.ReflectiveElements(mass, "ownedElement"); len(elements) == 0 || elements[len(elements)-1] != member {
			t.Errorf("mass ownedElement = %d elements, want the default multiplicity last", len(elements))
		}
		rels := model.OwnedRelationships(mass)
		if len(rels) == 0 {
			t.Fatal("mass owns no relationship")
		}
		membership := rels[len(rels)-1]
		if membership.Implicit == nil || membership.Implicit.Member != member {
			t.Fatalf("the last owned relationship of mass is %s, want the default multiplicity's membership", relMetaName(model, membership))
		}
		if got := relMetaName(model, membership); got != "OwningMembership" {
			t.Errorf("membership metaclass = %s, want OwningMembership", got)
		}
		if got := model.OwningMembership(member); got != membership {
			t.Error("OwningMembership(member) is not the membership mass owns it through")
		}
		for sym, feature := range map[*symbols.Symbol]string{membership: "memberName", member: "qualifiedName"} {
			if value, ok := model.ReflectiveFeatureValues(sym, feature); !ok || len(value) > 1 || (len(value) == 1 && value[0].Kind != symbols.FilterValueEmpty) {
				t.Errorf("%s = %v, %t; want none: the member is unnamed", feature, value, ok)
			}
		}
		if symbols.KeyOf(member) == symbols.KeyOf(mass) || symbols.SameElement(member, mass) {
			t.Error("KeyOf(default multiplicity) collides with its usage")
		}
		for _, name := range []string{"tags", "spare", "go", "leather", "red", "cap"} {
			sym := relFixtureSymbol(t, fixture, name)
			if model.DefaultMultiplicityMember(sym) != nil {
				t.Errorf("%s owns a default multiplicity, want none", name)
			}
		}
		if got, _ := model.ReflectiveElements(relFixtureSymbol(t, fixture, "tags"), "multiplicity"); len(got) != 0 {
			t.Errorf("tags multiplicity = %d elements, want none: a declared range is no member", len(got))
		}
	}
	for _, fixture := range reflectiveFixtures(t, "default-multiplicity.kerml", source.KindKerML, "package DK {\n    classifier C { feature f : C; }\n}") {
		if fixture.model.DefaultMultiplicityMember(relFixtureSymbol(t, fixture, "f")) != nil {
			t.Error("a KerML feature owns a default multiplicity; only a SysML usage takes the implicit [1..1]")
		}
	}
}
