package semantics

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestFirstMarkerIsAMembershipOfTheStart: `first start;` is a Membership (not owning) of the
// library's Actions::Action::start, as SysML.xtext InitialNodeMember, the pilot's export and the
// conversion state it — the body declares no feature named start.
func TestFirstMarkerIsAMembershipOfTheStart(t *testing.T) {
	const src = `package T {
		action observe {
			first start;
			then action acquire;
			then done;
		}
	}`
	for _, fixture := range reflectiveFixtures(t, "first-marker.sysml", source.KindSysML, src) {
		model := fixture.model
		observe := relFixtureSymbol(t, fixture, "observe")
		memberships := model.OwnedMemberships(observe)
		if len(memberships) < 2 {
			t.Fatalf("observe owns %d memberships, want the start marker's and acquire's", len(memberships))
		}
		first := memberships[0]
		assertRelMetaclass(t, model, first, "Membership")
		start, ok := model.ReflectiveElements(first, "memberElement")
		if !ok || len(start) != 1 || start[0].Name != "start" || start[0].OwnerScope == nil || start[0].OwnerScope.Owner() == nil ||
			start[0].OwnerScope.Owner().Name != "Action" || !strings.HasSuffix(start[0].DocName, "Actions.sysml") {
			t.Fatalf("memberElement of the first marker = %v (present %t), want the library's Actions::Action::start", start, ok)
		}
		assertRelFeature(t, model, first, "ownedRelatedElement")
		assertRelFeature(t, model, first, "owningRelatedElement", observe)
		assertRelFeature(t, model, first, "membershipOwningNamespace", observe)
		assertRelStrings(t, model, first, "memberName")
		assertRelRef(t, model, first, "visibility", "KerML::Root::VisibilityKind::public")
		members, _ := model.ReflectiveElements(observe, "ownedMember")
		for _, member := range members {
			if member.Name == "start" && member.OwnerScope != nil && member.OwnerScope.Owner() == observe {
				t.Errorf("observe owns a member named start; the marker declares none")
			}
		}
		features, _ := model.ReflectiveElements(observe, "ownedFeatureMembership")
		for _, membership := range features {
			if symbols.SameElement(membership, first) {
				t.Errorf("the first marker is among ownedFeatureMembership; it is a plain Membership")
			}
		}
	}
}

// TestFirstMarkerNamingNoMemberIsStillAMembership: `first action1;` naming nothing the body
// holds is a Membership all the same, as the conversion writes it, its member underived.
func TestFirstMarkerNamingNoMemberIsStillAMembership(t *testing.T) {
	const src = `package T {
		action def Flow {
			first action1;
			then action2;
		}
	}`
	for _, fixture := range reflectiveFixtures(t, "first-marker-unresolved.sysml", source.KindSysML, src) {
		model := fixture.model
		flow := relFixtureSymbol(t, fixture, "Flow")
		memberships := model.OwnedMemberships(flow)
		if len(memberships) == 0 {
			t.Fatal("Flow owns no membership; want the marker's")
		}
		first := memberships[0]
		assertRelMetaclass(t, model, first, "Membership")
		assertRelFeature(t, model, first, "owningRelatedElement", flow)
		if _, ok := model.ReflectiveElements(first, "memberElement"); ok {
			t.Error("memberElement of a marker naming no member is derived; want underived")
		}
	}
}
