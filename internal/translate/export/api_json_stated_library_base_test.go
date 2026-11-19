package export

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// A feature whose stated type is a library type inherits that type's own members, which no
// implied base supplies (KerML 8.3.3.1.10 deriveTypeInheritedMembership over the supertypes).
func TestAPIJSONStatedLibraryBaseCompletesTheInheritedFamily(t *testing.T) {
	graph := duplicateGraph(t, `package Probe {
		part def Reading {
			attribute q : Quantities::ScalarQuantityValue;
			attribute plain;
		}
		part meter : Parts::Part;
	}`, IDQualifiedName)
	document, err := WriteAPIJSONWithOptions(graph, APIJSONOptions{InheritedMembers: true})
	if err != nil {
		t.Fatal(err)
	}
	elements := booleansElements(t, document)
	catalog := identity.LibraryCatalogWithOpenSysML(libs.SharedBase())
	mRef, ok := catalog.ElementNamed("Quantities::ScalarQuantityValue::mRef")
	if !ok {
		t.Fatal("the library catalog has no Quantities::ScalarQuantityValue::mRef")
	}
	q := elements["Probe__Reading__q"]
	for _, property := range []string{"inheritedFeature", "feature", "member", "usage"} {
		if ids := referenceIDs(t, q, property); !slices.Contains(ids, mRef.ID) {
			t.Errorf("q.%s lacks Quantities::ScalarQuantityValue::mRef, the stated type's own feature", property)
		}
	}
	for _, property := range []string{"inheritedMembership", "featureMembership", "membership"} {
		if ids := referenceIDs(t, q, property); !slices.Contains(ids, mRef.OwningMembershipID) {
			t.Errorf("q.%s lacks the membership of Quantities::ScalarQuantityValue::mRef", property)
		}
	}
	plain := referenceIDs(t, elements["Probe__Reading__plain"], "inheritedMembership")
	if stated := referenceIDs(t, q, "inheritedMembership"); len(stated) <= len(plain) {
		t.Errorf("q inherits %d memberships, an untyped attribute %d: want the stated type's on top", len(stated), len(plain))
	}
	// A stated base that is also the implied one contributes its members once.
	meter := referenceIDs(t, elements["Probe__meter"], "inheritedMembership")
	if len(meter) == 0 {
		t.Fatal("meter.inheritedMembership is empty, want the members of Parts::Part")
	}
	seen := make(map[string]bool, len(meter))
	for _, id := range meter {
		if seen[id] {
			t.Errorf("meter.inheritedMembership lists %s twice", id)
		}
		seen[id] = true
	}
}

// A type's ends are its own, then its general types' at the positions it leaves open: an end
// redefines the one at its position in each supertype (KerML 8.3.3.1.10 deriveTypeEndFeature).
func TestAPIJSONEndFeatureCompletesOwnedEndsWithInheritedOnes(t *testing.T) {
	graph := duplicateGraph(t, `package Probe {
		part def A;
		part def B;
		connection def Link {
			end src : A;
			end dst : B;
		}
		connection def Narrow :> Link {
			end near : A;
		}
		part a : A;
		part b : B;
		connection link : Link connect a to b;
		connection bare : Link;
		part plain;
	}`, IDQualifiedName)
	document, err := WriteAPIJSONWithOptions(graph, APIJSONOptions{InheritedMembers: true})
	if err != nil {
		t.Fatal(err)
	}
	elements := booleansElements(t, document)
	src, dst := "Probe__Link__src", "Probe__Link__dst"
	if got := referenceIDs(t, elements["Probe__Link"], "endFeature"); !slices.Equal(got, []string{src, dst}) {
		t.Errorf("Link.endFeature = %v, want its two own ends", got)
	}
	if got := referenceIDs(t, elements["Probe__Narrow"], "endFeature"); !slices.Equal(got, []string{"Probe__Narrow__near", dst}) {
		t.Errorf("Narrow.endFeature = %v, want its own end, then the inherited second one", got)
	}
	if got := referenceIDs(t, elements["Probe__bare"], "endFeature"); !slices.Equal(got, []string{src, dst}) {
		t.Errorf("bare.endFeature = %v, want the two ends of Link and no library end under them", got)
	}
	link := referenceIDs(t, elements["Probe__link"], "endFeature")
	owned := referenceIDs(t, elements["Probe__link"], "ownedEndFeature")
	if len(owned) != 2 || !slices.Equal(link, owned) {
		t.Errorf("link.endFeature = %v, want its two own ends %v", link, owned)
	}
	// An end redefines the one at its position in each supertype, so that one is not inherited.
	narrow := referenceIDs(t, elements["Probe__Narrow"], "inheritedFeature")
	if slices.Contains(narrow, src) || !slices.Contains(narrow, dst) {
		t.Errorf("Narrow.inheritedFeature holds src=%v dst=%v, want only the end it leaves open", slices.Contains(narrow, src), slices.Contains(narrow, dst))
	}
	for _, property := range []string{"inheritedFeature", "feature", "member"} {
		if ids := referenceIDs(t, elements["Probe__link"], property); slices.Contains(ids, src) || slices.Contains(ids, dst) {
			t.Errorf("link.%s keeps an end of Link its own ends redefine", property)
		}
	}
	if got, stated := elements["Probe__plain"]["endFeature"]; stated {
		t.Errorf("plain.endFeature = %v, want none", got)
	}
}

// An inherited end a feature of the type redefines is not one of its features, so not one of its
// ends either, whether or not the redefining feature is an end (KerML 8.3.3.1.10 deriveTypeEndFeature).
func TestAPIJSONEndFeatureLeavesOutRedefinedInheritedEnds(t *testing.T) {
	graph := duplicateGraph(t, `package Probe {
		part def A;
		part def D {
			end part one : A;
			end part two : A;
		}
		part def E :> D {
			constant part :>> one;
		}
		part def F :> E {
			end part mine : A;
		}
		part def G :> D;
		part def H :> F, G;
	}`, IDQualifiedName)
	document, err := WriteAPIJSONWithOptions(graph, APIJSONOptions{InheritedMembers: true})
	if err != nil {
		t.Fatal(err)
	}
	elements := booleansElements(t, document)
	first, second, mine := "Probe__D__one", "Probe__D__two", "Probe__F__mine"
	for name, want := range map[string][]string{
		"Probe__D": {first, second},
		"Probe__E": {second},
		"Probe__F": {mine},
		"Probe__G": {first, second},
		"Probe__H": {mine},
	} {
		if got := referenceIDs(t, elements[name], "endFeature"); !slices.Equal(got, want) {
			t.Errorf("%s.endFeature = %v, want %v", name, got, want)
		}
	}
	if inherited := referenceIDs(t, elements["Probe__F"], "inheritedFeature"); slices.Contains(inherited, second) {
		t.Error("F.inheritedFeature keeps D::two, which F::mine redefines by end position")
	}
}

// An enumerated value is held by a VariantMembership, an OwningMembership that is no
// FeatureMembership (SysML 8.3.6.5): a feature typed by the enumeration inherits the
// membership and the member, never a feature or a usage.
func TestAPIJSONInheritedEnumeratedValuesAreMembersNotFeatures(t *testing.T) {
	graph := duplicateGraph(t, `package Probe {
		part def Run { attribute verdict : VerificationCases::VerdictKind; }
	}`, IDQualifiedName)
	document, err := WriteAPIJSONWithOptions(graph, APIJSONOptions{InheritedMembers: true})
	if err != nil {
		t.Fatal(err)
	}
	elements := booleansElements(t, document)
	catalog := identity.LibraryCatalogWithOpenSysML(libs.SharedBase())
	pass, ok := catalog.ElementNamed("VerificationCases::VerdictKind::pass")
	if !ok {
		t.Fatal("the library catalog has no VerificationCases::VerdictKind::pass")
	}
	verdict := elements["Probe__Run__verdict"]
	if ids := referenceIDs(t, verdict, "member"); !slices.Contains(ids, pass.ID) {
		t.Error("verdict.member lacks the enumerated value VerdictKind::pass")
	}
	if ids := referenceIDs(t, verdict, "inheritedMembership"); !slices.Contains(ids, pass.OwningMembershipID) {
		t.Error("verdict.inheritedMembership lacks the variant membership of VerdictKind::pass")
	}
	for _, property := range []string{"feature", "inheritedFeature", "usage"} {
		if _, stated := verdict[property]; !stated {
			continue
		}
		if ids := referenceIDs(t, verdict, property); slices.Contains(ids, pass.ID) {
			t.Errorf("verdict.%s lists the enumerated value VerdictKind::pass, which no FeatureMembership holds", property)
		}
	}
	if ids := referenceIDs(t, verdict, "featureMembership"); slices.Contains(ids, pass.OwningMembershipID) {
		t.Error("verdict.featureMembership lists the variant membership of VerdictKind::pass")
	}
}

// Type::endFeature is feature->select(isEnd) (KerML 8.3.3.1.10 deriveTypeEndFeature), so it follows
// what the type inherits: a private end of a supertype is no feature of the subtype.
func TestAPIJSONEndFeatureFollowsTheInheritedFeatures(t *testing.T) {
	src := `package Probe {
		classifier X;
		classifier A {
			end feature shown : X;
			private end feature hidden : X;
			private alias shownToo for shown;
			alias hiddenToo for hidden;
		}
		classifier B specializes A;
	}`
	file := source.New("probe.kerml", []byte(src))
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("the fixture does not parse: %v", p.Diagnostics)
	}
	graph, err := ToRDFWith(file, root, IDQualifiedName)
	if err != nil {
		t.Fatal(err)
	}
	document, err := WriteAPIJSONWithOptions(graph, APIJSONOptions{InheritedMembers: true})
	if err != nil {
		t.Fatal(err)
	}
	elements := booleansElements(t, document)
	for id, want := range map[string][]string{
		"Probe__A": {"Probe__A__shown", "Probe__A__hidden"},
		"Probe__B": {"Probe__A__shown"},
	} {
		got := referenceIDs(t, elements[id], "endFeature")
		if !slices.Equal(got, want) {
			t.Errorf("%s.endFeature = %v, want %v", id, got, want)
		}
		features := referenceIDs(t, elements[id], "feature")
		for _, end := range got {
			if !slices.Contains(features, end) {
				t.Errorf("%s.endFeature lists %s, which %s.feature does not", id, end, id)
			}
		}
	}
}
