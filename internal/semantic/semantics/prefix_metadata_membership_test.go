package semantics

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

const prefixMembershipFixtureSrc = `package C {
    metadata def Tg;
    metadata def Tg2;
    part def Base;
    #Tg package M {
        #Tg #Tg2 part q : Base { @Tg2; part inner; }
        #Tg part def D :> Base;
        part a; part b;
        #Tg dependency dp from a to b { @Tg2; }
        action act { in #Tg x; }
        individual #Tg occurrence def Ind;
        part def Sub :> D;
    }
}`

// prefixMembers is the `#M` prefix features among members, in order.
func prefixMembers(members []*symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, member := range members {
		if member.IsPrefixMetadata() {
			out = append(out, member)
		}
	}
	return out
}

// A `#M` prefix is a MetadataUsage its element owns through an OwningMembership
// that leads the element's relationships (SysML.xtext PrefixMetadataMember), on a
// package, definition, usage and parameter alike, parsed and recorded.
func TestPrefixMetadataIsOwnedThroughAnOwningMembership(t *testing.T) {
	for _, fixture := range reflectiveFixtures(t, "prefix-memberships.sysml", source.KindSysML, prefixMembershipFixtureSrc) {
		model := fixture.model
		for _, tc := range []struct {
			name     string
			prefixes int
		}{{"M", 1}, {"q", 2}, {"D", 1}, {"x", 1}} {
			owner := relFixtureSymbol(t, fixture, tc.name)
			memberships := model.OwnedMemberships(owner)
			if len(memberships) < tc.prefixes {
				t.Fatalf("%s owns %d memberships, want its %d prefixes among them", tc.name, len(memberships), tc.prefixes)
			}
			assertRelFeature(t, model, owner, "ownedAnnotation")
			assertRelFeature(t, model, owner, "ownedMembership", memberships...)
			var members []*symbols.Symbol
			for _, membership := range memberships {
				members = append(members, membership.Implicit.Member)
			}
			assertRelFeature(t, model, owner, "ownedMember", members...)
			if got := prefixMembers(members); len(got) != tc.prefixes || !slices.Equal(got, members[:tc.prefixes]) {
				t.Errorf("%s owns prefixes %v, want %d leading its members", tc.name, got, tc.prefixes)
			}
			rels, _ := model.ReflectiveElements(owner, "ownedRelationship")
			if len(rels) < tc.prefixes || !slices.Equal(rels[:tc.prefixes], memberships[:tc.prefixes]) {
				t.Errorf("%s ownedRelationship does not start with its %d prefix memberships", tc.name, tc.prefixes)
			}
			for i, prefix := range members[:tc.prefixes] {
				assertRelMetaclass(t, model, memberships[i], "OwningMembership")
				assertRelFeature(t, model, memberships[i], "ownedMemberElement", prefix)
				assertRelFeature(t, model, memberships[i], "membershipOwningNamespace", owner)
				assertRelFeature(t, model, prefix, "owningMembership", memberships[i])
				assertRelFeature(t, model, prefix, "owningRelationship", memberships[i])
				assertRelFeature(t, model, prefix, "owningNamespace", owner)
				assertRelFeature(t, model, prefix, "owner", owner)
				assertRelFeature(t, model, prefix, "owningAnnotatingRelationship")
				assertRelFeature(t, model, prefix, "annotation")
				assertRelFeature(t, model, prefix, "annotatedElement", owner)
			}
			for _, feature := range []string{"member", "ownedElement"} {
				got, _ := model.ReflectiveElements(owner, feature)
				if len(got) < tc.prefixes || !slices.Equal(got[:tc.prefixes], members[:tc.prefixes]) {
					t.Errorf("%s %s does not start with its %d prefixes", tc.name, feature, tc.prefixes)
				}
			}
		}
	}
}

// The prefix memberships precede the head's clauses and the
// body; `individual` and its Multiplicity precede them (OccurrenceDefinitionPrefix).
func TestPrefixMetadataMembershipOrder(t *testing.T) {
	for _, fixture := range reflectiveFixtures(t, "prefix-order.sysml", source.KindSysML, prefixMembershipFixtureSrc) {
		model := fixture.model
		q := relFixtureSymbol(t, fixture, "q")
		memberships := model.OwnedMemberships(q)
		// #Tg, #Tg2, @Tg2, inner.
		if len(memberships) != 4 || !memberships[0].Implicit.Member.IsPrefixMetadata() || !memberships[1].Implicit.Member.IsPrefixMetadata() {
			t.Fatalf("q owns %d memberships, want its two prefixes, then its two body members", len(memberships))
		}
		body := memberships[2].Implicit.Member
		if body.Kind != symbols.SymbolMetadataUsage || body.IsPrefixMetadata() {
			t.Errorf("q's third membership owns %v, want the body's @Tg2", body)
		}
		rels, _ := model.ReflectiveElements(q, "ownedRelationship")
		written := model.ImplicitRelationships(q)
		if len(written) == 0 || !slices.Equal(rels[:2], memberships[:2]) || rels[2] != written[0] {
			t.Errorf("q ownedRelationship = %v, want the prefix memberships, then the typing clause", rels)
		}

		ind := relFixtureSymbol(t, fixture, "Ind")
		indMemberships := model.OwnedMemberships(ind)
		if len(indMemberships) != 2 || !IsDefaultMultiplicity(indMemberships[0].Implicit.Member) || !indMemberships[1].Implicit.Member.IsPrefixMetadata() {
			t.Errorf("Ind owns %d memberships, want the individual's Multiplicity, then the #Tg prefix", len(indMemberships))
		}
		indRels, _ := model.ReflectiveElements(ind, "ownedRelationship")
		if !slices.Equal(indRels, indMemberships) {
			t.Errorf("Ind ownedRelationship = %v, want its two memberships in order", indRels)
		}
	}
}

// A dependency is no namespace: it owns its `#M` prefix through an Annotation
// (SysML.xtext Dependency, PrefixMetadataAnnotation), as it does its body's.
func TestDependencyPrefixMetadataIsOwnedThroughAnAnnotation(t *testing.T) {
	for _, fixture := range reflectiveFixtures(t, "prefix-dependency.sysml", source.KindSysML, prefixMembershipFixtureSrc) {
		model := fixture.model
		dp := relFixtureSymbol(t, fixture, "dp")
		anns := model.OwnedAnnotations(dp)
		if len(anns) != 2 {
			t.Fatalf("dp owns %d annotations, want its prefix's and its body's", len(anns))
		}
		prefix := anns[0].Implicit.Member
		if !prefix.IsPrefixMetadata() || anns[1].Implicit.Member.IsPrefixMetadata() {
			t.Fatalf("dp's annotations own %v and %v, want the prefix first", prefix, anns[1].Implicit.Member)
		}
		assertRelMetaclass(t, model, anns[0], "Annotation")
		assertRelFeature(t, model, dp, "ownedAnnotation", anns...)
		assertRelFeature(t, model, dp, "ownedRelationship", anns...)
		assertRelUnsupported(t, model, dp, "ownedMembership")
		assertRelFeature(t, model, anns[0], "ownedAnnotatingElement", prefix)
		assertRelFeature(t, model, anns[0], "owningAnnotatedElement", dp)
		assertRelFeature(t, model, prefix, "owningAnnotatingRelationship", anns[0])
		assertRelFeature(t, model, prefix, "owningRelationship", anns[0])
		assertRelFeature(t, model, prefix, "owningMembership")
		assertRelFeature(t, model, prefix, "annotation", anns[0])
		assertRelFeature(t, model, prefix, "annotatedElement", dp)
	}
}

// A prefix membership is inherited like any other public membership
// (KerML 8.3.3.1.10 Type::inheritableMemberships).
func TestPrefixMetadataMembershipIsInherited(t *testing.T) {
	for _, fixture := range reflectiveFixtures(t, "prefix-inherited.sysml", source.KindSysML, prefixMembershipFixtureSrc) {
		model := fixture.model
		d := relFixtureSymbol(t, fixture, "D")
		sub := relFixtureSymbol(t, fixture, "Sub")
		prefix := model.OwnedMemberships(d)[0]
		inherited, ok := model.ReflectiveElements(sub, "inheritedMembership")
		if !ok || !containsElement(inherited, prefix) {
			t.Errorf("Sub inheritedMembership holds D's prefix membership: %t, want true", containsElement(inherited, prefix))
		}
		assertRelFeature(t, model, sub, "ownedMembership")
	}
}
