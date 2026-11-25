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
		// The start is no ownedMember (its Membership owns nothing) but one member and one feature.
		if got := countNamed(members, "start"); got != 0 {
			t.Errorf("ownedMember of observe names start %d times, want 0", got)
		}
		for _, property := range []string{"member", "feature"} {
			elements, _ := model.ReflectiveElements(observe, property)
			if got := countNamed(elements, "start"); got != 1 {
				t.Errorf("%s of observe names start %d times, want 1", property, got)
			}
		}
	}
}

// TestFirstMarkerNamingABodyMemberListsItOnce: `first take;` before `action take` makes take a
// member once, not once per membership — Namespace::member is a set.
func TestFirstMarkerNamingABodyMemberListsItOnce(t *testing.T) {
	const src = `package T {
		action drain {
			first take;
			action take;
		}
	}`
	for _, fixture := range reflectiveFixtures(t, "first-marker-body-member.sysml", source.KindSysML, src) {
		model := fixture.model
		drain := relFixtureSymbol(t, fixture, "drain")
		var take *symbols.Symbol
		for _, candidate := range drain.Scope.LookupLocalAll("take") {
			if !symbols.IsFirstMarker(candidate) {
				take = candidate
			}
		}
		if take == nil {
			t.Fatal("drain declares no action take")
		}
		memberships := model.OwnedMemberships(drain)
		if len(memberships) != 2 {
			t.Fatalf("drain owns %d memberships, want the marker's Membership and take's FeatureMembership", len(memberships))
		}
		assertRelMetaclass(t, model, memberships[0], "Membership")
		assertRelFeature(t, model, memberships[0], "memberElement", take)
		assertRelMetaclass(t, model, memberships[1], "FeatureMembership")
		assertRelFeature(t, model, take, "owningMembership", memberships[1])
		for _, property := range []string{"ownedMember", "member", "feature", "ownedFeature"} {
			elements, _ := model.ReflectiveElements(drain, property)
			if got := countNamed(elements, "take"); got != 1 {
				t.Errorf("%s of drain names take %d times, want 1", property, got)
			}
		}
	}
}

// TestInheritedFirstMarkerIsNoFeature: a body inheriting `first take; action take;` has the
// inherited take among its members and features once, and the marker never.
func TestInheritedFirstMarkerIsNoFeature(t *testing.T) {
	const src = `package T {
		action def A {
			first take;
			action take;
		}
		action def B :> A;
	}`
	for _, fixture := range reflectiveFixtures(t, "first-marker-inherited.sysml", source.KindSysML, src) {
		model := fixture.model
		b := relFixtureSymbol(t, fixture, "B")
		for _, property := range []string{"member", "feature"} {
			elements, _ := model.ReflectiveElements(b, property)
			if got := countNamed(elements, "take"); got != 1 {
				t.Errorf("%s of B names take %d times, want 1", property, got)
			}
			for _, element := range elements {
				if symbols.IsFirstMarker(element) {
					t.Errorf("%s of B lists the first marker itself", property)
				}
			}
		}
	}
}

func countNamed(elements []*symbols.Symbol, name string) int {
	n := 0
	for _, element := range elements {
		if element.Name == name {
			n++
		}
	}
	return n
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
