package export

import (
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

func importProbeElements(t *testing.T, model string) map[string]map[string]any {
	t.Helper()
	document, err := WriteAPIJSONWithOptions(duplicateGraph(t, model, IDQualifiedName), APIJSONOptions{InheritedMembers: true})
	if err != nil {
		t.Fatal(err)
	}
	return booleansElements(t, document)
}

// probeReferences lists the references of a derived property that name the probe's own
// elements; the library's carry identifiers the fixture does not choose.
func probeReferences(t *testing.T, elements map[string]map[string]any, id, property string) []string {
	t.Helper()
	element, ok := elements[id]
	if !ok {
		t.Fatalf("no element %s", id)
	}
	if _, stated := element[property]; !stated {
		t.Fatalf("%s states no %s", id, property)
	}
	var own []string
	for _, reference := range referenceIDs(t, element, property) {
		if strings.HasPrefix(reference, "Probe") {
			own = append(own, reference)
		}
	}
	return own
}

func wantReferences(t *testing.T, elements map[string]map[string]any, id, property string, want ...string) {
	t.Helper()
	if got := probeReferences(t, elements, id, property); !slices.Equal(got, want) {
		t.Errorf("%s.%s = %v, want %v", id, property, got, want)
	}
}

// A namespace import brings the public memberships of the namespace, a membership import
// the one it names, and membership and member follow (KerML 8.3.2.4.2, 8.3.2.4.5).
func TestAPIJSONGraphImportedMembershipOfNamespaceAndMembershipImports(t *testing.T) {
	elements := importProbeElements(t, `package Probe {
	package Lib { part def A; private part def Hidden; part def B; }
	package ByNamespace { import Lib::*; part def Own; }
	package ByMembership { import Lib::B; }
	package ByAll { import all Lib::*; }
}`)
	wantReferences(t, elements, "Probe__ByNamespace", "importedMembership", "Probe__Lib__A_om", "Probe__Lib__B_om")
	wantReferences(t, elements, "Probe__ByNamespace", "membership",
		"Probe__ByNamespace__Own_om", "Probe__Lib__A_om", "Probe__Lib__B_om")
	wantReferences(t, elements, "Probe__ByNamespace", "member", "Probe__ByNamespace__Own", "Probe__Lib__A", "Probe__Lib__B")
	wantReferences(t, elements, "Probe__ByMembership", "importedMembership", "Probe__Lib__B_om")
	wantReferences(t, elements, "Probe__ByMembership", "member", "Probe__Lib__B")
	wantReferences(t, elements, "Probe__ByAll", "importedMembership",
		"Probe__Lib__A_om", "Probe__Lib__Hidden_om", "Probe__Lib__B_om")
	completed, _ := elements["Probe__ByNamespace"][apiJSONCompletedPropertiesKey].([]any)
	for _, name := range []any{"importedMembership", "membership", "member"} {
		if !slices.Contains(completed, name) {
			t.Errorf("%s = %v does not name %s", apiJSONCompletedPropertiesKey, completed, name)
		}
	}
}

// A recursive import descends the namespaces the imported one owns, not those it merely
// imports, and a recursive membership import brings the named membership too (7.2.5.4).
func TestAPIJSONGraphImportedMembershipOfRecursiveImports(t *testing.T) {
	elements := importProbeElements(t, `package Probe {
	package Other { package Deep { part def Unreached; } }
	package Lib {
		public import Other::*;
		part def A;
		package Inner { part def B; private part def Hidden; package Innermost { part def C; } }
	}
	package Recursive { import Lib::*::**; }
	package RecursiveMembership { import Lib::Inner::**; }
}`)
	wantReferences(t, elements, "Probe__Recursive", "importedMembership",
		"Probe__Lib__A_om", "Probe__Lib__Inner_om", "Probe__Other__Deep_om",
		"Probe__Lib__Inner__B_om", "Probe__Lib__Inner__Innermost_om", "Probe__Lib__Inner__Innermost__C_om")
	wantReferences(t, elements, "Probe__RecursiveMembership", "importedMembership",
		"Probe__Lib__Inner_om", "Probe__Lib__Inner__B_om", "Probe__Lib__Inner__Innermost_om",
		"Probe__Lib__Inner__Innermost__C_om")
}

// Only a public import re-exports, so what a namespace imports privately or protectedly
// stays out of what importing it brings (8.3.2.4.5 visibleMemberships).
func TestAPIJSONGraphImportedMembershipFollowsImportVisibility(t *testing.T) {
	elements := importProbeElements(t, `package Probe {
	package Lib { part def A; part def B; part def C; }
	package Reexport { public import Lib::A; private import Lib::B; protected import Lib::C; }
	package Client { import Reexport::*; }
}`)
	wantReferences(t, elements, "Probe__Reexport", "importedMembership",
		"Probe__Lib__A_om", "Probe__Lib__B_om", "Probe__Lib__C_om")
	wantReferences(t, elements, "Probe__Client", "importedMembership", "Probe__Lib__A_om")
}

// An import cycle ends where it returns to a namespace already on the path (the excluded
// argument of 8.3.2.4.2 importedMemberships), and a namespace does not import itself.
func TestAPIJSONGraphImportedMembershipEndsImportCycles(t *testing.T) {
	elements := importProbeElements(t, `package Probe {
	package R { public import S::*; part def r1; }
	package S { public import R::*; part def s1; }
	package Self { public import Self::*; public import Nest::**; part def u1; }
	package Nest { package W { public import Self::*; part def w1; } }
	part def C { public import D::*; part c; }
	part def D :> C { part d; }
}`)
	wantReferences(t, elements, "Probe__R", "importedMembership", "Probe__S__s1_om")
	wantReferences(t, elements, "Probe__S", "importedMembership", "Probe__R__r1_om")
	wantReferences(t, elements, "Probe__Self", "importedMembership",
		"Probe__Nest_om", "Probe__Nest__W_om", "Probe__Nest__W__w1_om")
	wantReferences(t, elements, "Probe__Nest__W", "importedMembership",
		"Probe__Self__u1_om", "Probe__Nest_om", "Probe__Nest__W_om")
	// D inherits c from C, which importing D would bring back to C under its own name.
	wantReferences(t, elements, "Probe__C", "importedMembership", "Probe__D__d_om")
	wantReferences(t, elements, "Probe__D", "inheritedMembership", "Probe__C__c_om", "Probe__D__d_om")
}

// An imported membership not distinguishable (8.3.2.4.3) from an owned or inherited one
// is hidden, and two such imported ones hide each other (7.2.5.4, 7.3.2.1).
func TestAPIJSONGraphImportedMembershipHidesCollisions(t *testing.T) {
	elements := importProbeElements(t, `package Probe {
	package L1 { part def X; part def Y; part x; part def Sh; }
	package L2 { part def X; part def Z; item def x; alias Sh for L1::Sh; }
	package BothImported { import L1::*; import L2::*; }
	package OwnedHides { import L1::*; part def Y; }
	part def Base { part def Z; }
	part def InheritedHides :> Base { import L2::*; }
}`)
	// X names two part definitions and Sh a definition and its alias; the usage x and the
	// item definition x are of metaclasses neither of which conforms to the other.
	wantReferences(t, elements, "Probe__BothImported", "importedMembership",
		"Probe__L1__Y_om", "Probe__L1__x_om", "Probe__L2__Z_om", "Probe__L2__x_om")
	wantReferences(t, elements, "Probe__OwnedHides", "importedMembership",
		"Probe__L1__X_om", "Probe__L1__x_om", "Probe__L1__Sh_om")
	wantReferences(t, elements, "Probe__OwnedHides", "member",
		"Probe__OwnedHides__Y", "Probe__L1__X", "Probe__L1__x", "Probe__L1__Sh")
	wantReferences(t, elements, "Probe__InheritedHides", "importedMembership",
		"Probe__L2__X_om", "Probe__L2__x_om", "Probe__L2__Sh")
}

// A filtered import and a package's filter conditions keep the memberships whose member
// meets every condition (7.2.5.4, 8.3.4.13.4 includeAsMember).
func TestAPIJSONGraphImportedMembershipAppliesFilters(t *testing.T) {
	elements := importProbeElements(t, `package Probe {
	metadata def Safety { attribute isMandatory : ScalarValues::Boolean; }
	metadata def Security;
	package Lib {
		part a { @Safety { isMandatory = true; } }
		part b { @Safety { isMandatory = false; } }
		part c { @Security; }
		part d;
		part def E;
		metadata s : Security about d;
	}
	package Bracket { import Lib::*[@Safety]; }
	package Mandatory { import Lib::*; filter @Safety and Safety::isMandatory; }
	package Either { import Lib::*; filter @Safety or @Security; }
	package Neither { import Lib::*; filter not (@Safety or @Security); }
	package Usages { import Lib::*[@SysML::PartUsage]; }
}`)
	wantReferences(t, elements, "Probe__Bracket", "importedMembership", "Probe__Lib__a_om", "Probe__Lib__b_om")
	wantReferences(t, elements, "Probe__Mandatory", "importedMembership", "Probe__Lib__a_om")
	wantReferences(t, elements, "Probe__Either", "importedMembership",
		"Probe__Lib__a_om", "Probe__Lib__b_om", "Probe__Lib__c_om", "Probe__Lib__d_om")
	wantReferences(t, elements, "Probe__Usages", "importedMembership",
		"Probe__Lib__a_om", "Probe__Lib__b_om", "Probe__Lib__c_om", "Probe__Lib__d_om")
	neither := probeReferences(t, elements, "Probe__Neither", "importedMembership")
	if !slices.Contains(neither, "Probe__Lib__E_om") || slices.Contains(neither, "Probe__Lib__a_om") ||
		slices.Contains(neither, "Probe__Lib__d_om") {
		t.Errorf("Neither.importedMembership = %v, want E without the annotated parts", neither)
	}
}

// A type inherits what its supertypes import publicly or protectedly, not privately
// (8.3.3.1.10 nonPrivateMemberships), and its members include what it imports itself.
func TestAPIJSONGraphTypeInheritsNonPrivateImports(t *testing.T) {
	elements := importProbeElements(t, `package Probe {
	package Lib { part def X; part def Y; part def Z; }
	part def A { public import Lib::X; protected import Lib::Y; private import Lib::Z; part a; }
	part def B :> A { part b; }
}`)
	wantReferences(t, elements, "Probe__A", "importedMembership",
		"Probe__Lib__X_om", "Probe__Lib__Y_om", "Probe__Lib__Z_om")
	wantReferences(t, elements, "Probe__A", "member", "Probe__A__a", "Probe__Lib__X", "Probe__Lib__Y", "Probe__Lib__Z")
	wantReferences(t, elements, "Probe__B", "inheritedMembership",
		"Probe__A__a_om", "Probe__Lib__X_om", "Probe__Lib__Y_om")
	wantReferences(t, elements, "Probe__B", "member", "Probe__B__b", "Probe__A__a", "Probe__Lib__X", "Probe__Lib__Y")
}

// A library namespace the graph only references is imported from its declarations: the
// memberships are the library's own, and a library membership import names one of them.
func TestAPIJSONGraphImportedMembershipOfLibraryNamespaces(t *testing.T) {
	elements := importProbeElements(t, `package Probe {
	package Scalars { import ScalarValues::*; }
	package One { import ScalarValues::Real; }
}`)
	all := referenceIDs(t, elements["Probe__Scalars"], "importedMembership")
	one := referenceIDs(t, elements["Probe__One"], "importedMembership")
	if len(one) != 1 || !slices.Contains(all, one[0]) {
		t.Errorf("One imports %v, want the one membership of Real among the %d of ScalarValues::*", one, len(all))
	}
	if len(all) < 10 {
		t.Errorf("Scalars imports %d memberships, want every public member of ScalarValues", len(all))
	}
	if members := referenceIDs(t, elements["Probe__Scalars"], "member"); len(members) != len(all) {
		t.Errorf("Scalars has %d members for %d imported memberships", len(members), len(all))
	}
}

// A library namespace's memberships include the ones no name reaches: its documentation, owned
// through an unnamed membership, and its aliases, each a Membership of its own (KerML 8.3.2.4).
func TestAPIJSONGraphImportedMembershipOfUnnamedLibraryMemberships(t *testing.T) {
	elements := importProbeElements(t, `package Probe {
	package Scalars { import ScalarValues::*; }
	package Quantities { import ISQThermodynamics::*; }
	package Chained { import ISQ::*; }
}`)
	if all := referenceIDs(t, elements["Probe__Scalars"], "importedMembership"); len(all) != 12 {
		t.Errorf("Scalars imports %d memberships, want the 11 types of ScalarValues and its documentation", len(all))
	}
	base := libs.SharedBase()
	resolver := resolve.New(base)
	catalog := identity.LibraryCatalogWithOpenSysML(base)
	owner := resolver.Index().LookupQualified("ISQThermodynamics")[0]
	aliasSymbol, found := owner.Scope.LookupLocal("TemperatureValue")
	if !found {
		t.Fatal("ISQThermodynamics has no alias TemperatureValue")
	}
	alias, ok := identity.AliasMembershipOf(semantics.NewModel(resolver), catalog, aliasSymbol)
	if !ok {
		t.Fatal("the alias TemperatureValue has no membership id")
	}
	target, ok := catalog.ElementForSymbol(resolver.Index().LookupQualified("ISQBase::ThermodynamicTemperatureValue")[0])
	if !ok {
		t.Fatal("ISQBase::ThermodynamicTemperatureValue is not catalogued")
	}
	imported := referenceIDs(t, elements["Probe__Quantities"], "importedMembership")
	if !slices.Contains(imported, alias.ID) {
		t.Errorf("ISQThermodynamics::* imports no membership %s of the alias TemperatureValue", alias.ID)
	}
	if slices.Contains(imported, target.OwningMembershipID) {
		t.Errorf("ISQThermodynamics::* imports the owning membership of the alias's target, which ISQBase owns")
	}
	// ISQ imports ISQThermodynamics::* publicly, so what that import brings ISQ::* brings too.
	chained := referenceIDs(t, elements["Probe__Chained"], "importedMembership")
	for _, id := range imported {
		if !slices.Contains(chained, id) {
			t.Errorf("ISQ::* imports no membership %s that ISQThermodynamics::* imports", id)
		}
	}
	if members := referenceIDs(t, elements["Probe__Quantities"], "member"); !slices.Contains(members, target.ID) {
		t.Errorf("the alias membership names no member %s", target.ID)
	}
}

// A model read back from its element form alone derives the same three properties.
func TestAPIJSONGraphImportedMembershipSurvivesGraphOnlyReconstruction(t *testing.T) {
	const model = `package Probe {
	metadata def Safety;
	package Lib { part def A; part b { @Safety; } package Inner { part def C; } }
	package Client { import Lib::*::**; import ScalarValues::Real; part def Own; }
	package Filtered { import Lib::*[@Safety]; }
	part def T { protected import Lib::A; }
	part def U :> T;
}`
	first, err := WriteAPIJSONWithOptions(duplicateGraph(t, model, IDQualifiedName), APIJSONOptions{InheritedMembers: true})
	if err != nil {
		t.Fatal(err)
	}
	// The document read back states the owned structure only, none of the derived lists.
	owned, err := WriteAPIJSON(duplicateGraph(t, model, IDQualifiedName))
	if err != nil {
		t.Fatal(err)
	}
	for id, element := range booleansElements(t, owned) {
		if kind, _ := element["@type"].(string); strings.HasSuffix(kind, "Import") {
			continue // An import states the membership it imports, its own property of that name.
		}
		for _, property := range []string{"importedMembership", "membership", "member"} {
			if _, stated := element[property]; stated {
				t.Fatalf("%s states %s without the inherited members", id, property)
			}
		}
	}
	graph, err := ReadAPIJSON(owned)
	if err != nil {
		t.Fatal(err)
	}
	second, err := WriteAPIJSONWithOptions(graph, APIJSONOptions{InheritedMembers: true})
	if err != nil {
		t.Fatal(err)
	}
	before, after := booleansElements(t, first), booleansElements(t, second)
	for _, id := range []string{"Probe__Client", "Probe__Filtered", "Probe__T", "Probe__U"} {
		for _, property := range []string{"importedMembership", "membership", "member"} {
			if id == "Probe__U" && property == "importedMembership" {
				continue // U imports nothing, and an empty derived list is not written.
			}
			want := referenceIDs(t, before[id], property)
			if got := referenceIDs(t, after[id], property); !slices.Equal(got, want) {
				t.Errorf("%s.%s read back as %v, want %v", id, property, got, want)
			}
		}
	}
	// The usage b is a namespace too, so the recursion brings its unnamed metadata usage.
	wantReferences(t, after, "Probe__Client", "importedMembership",
		"Probe__Lib__A_om", "Probe__Lib__b_om", "Probe__Lib__Inner_om", "Probe__Lib__b___400_om",
		"Probe__Lib__Inner__C_om")
	wantReferences(t, after, "Probe__Filtered", "importedMembership", "Probe__Lib__b_om")
	wantReferences(t, after, "Probe__U", "inheritedMembership", "Probe__Lib__A_om")
}

// A membership imported twice is as visible as its most visible import makes it, whichever
// comes first (8.3.2.4.5 visibleMemberships: "Memberships imported with a visibility of public").
func TestAPIJSONGraphImportedMembershipKeepsTheMostVisibleImport(t *testing.T) {
	elements := importProbeElements(t, `package Probe {
	package Lib { part def A; part def B; }
	package Reexport { private import Lib::*; public import Lib::A; }
	package Client { import Reexport::*; }
	part def General { private import Lib::B; protected import Lib::B; }
	part def Special :> General;
}`)
	wantReferences(t, elements, "Probe__Reexport", "importedMembership", "Probe__Lib__A_om", "Probe__Lib__B_om")
	wantReferences(t, elements, "Probe__Client", "importedMembership", "Probe__Lib__A_om")
	wantReferences(t, elements, "Probe__Special", "inheritedMembership", "Probe__Lib__B_om")
}
