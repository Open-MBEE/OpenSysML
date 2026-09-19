package semantics

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func TestImplicitRoleRedefinitionDeduplicatesDiamond(t *testing.T) {
	m, root := buildModel(t, `package P {
		verification def Base {
			objective baseObjective { attribute inherited; }
		}
		verification def Left :> Base;
		verification def Right :> Base;
		verification def Derived :> Left, Right {
			objective derivedObjective;
		}
	}`)
	p := sym(t, root, "P")
	baseObjective := nested(t, nested(t, p.Scope, "Base").Scope, "baseObjective")
	derivedObjective := nested(t, nested(t, p.Scope, "Derived").Scope, "derivedObjective")

	got := m.ImplicitRoleRedefinitions(derivedObjective)
	if len(got) != 1 || got[0] != baseObjective {
		t.Fatalf("ImplicitRoleRedefinitions(derivedObjective) = %v, want [baseObjective]", got)
	}
}

// A role redefines the same role of every general: naming one by `:>>` leaves
// the others implicit, and naming something else leaves them all implicit.
func TestImplicitRoleRedefinitionSkipsOnlyTheExplicitlyRedefinedRole(t *testing.T) {
	m, root := buildModel(t, `package P {
		requirement def Weight { subject w; attribute limit; }
		requirement def Volume { subject vol; }
		requirement def Both :> Weight, Volume { subject s :>> w; }
		requirement def Neither :> Weight, Volume { subject s :>> limit; }
	}`)
	p := sym(t, root, "P")
	w := nested(t, nested(t, p.Scope, "Weight").Scope, "w")
	vol := nested(t, nested(t, p.Scope, "Volume").Scope, "vol")

	both := nested(t, nested(t, p.Scope, "Both").Scope, "s")
	if got := m.ImplicitRoleRedefinitions(both); len(got) != 1 || got[0] != vol {
		t.Errorf("ImplicitRoleRedefinitions(Both::s) = %v, want [vol]", got)
	}
	neither := nested(t, nested(t, p.Scope, "Neither").Scope, "s")
	if got := m.ImplicitRoleRedefinitions(neither); len(got) != 2 || got[0] != w || got[1] != vol {
		t.Errorf("ImplicitRoleRedefinitions(Neither::s) = %v, want [w vol]", got)
	}
}

func TestBoundSubjectInheritsTheRedefinedSubjectsMembers(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def Truck { attribute payload; }
		requirement def Req { subject truck : Truck; }
		part truck : Truck;
		requirement bound : Req { subject truck = truck; }
	}`)
	p := sym(t, root, "P")
	subject := nested(t, nested(t, p.Scope, "bound").Scope, "truck")
	payload := nested(t, nested(t, p.Scope, "Truck").Scope, "payload")

	got, ok := m.LookupMember(subject, "payload")
	if !ok || got != payload {
		t.Fatalf("LookupMember(bound's subject, %q) = %v, %v, want the truck's payload", "payload", got, ok)
	}
}

func TestImplicitRoleRedefinitionMatchesRole(t *testing.T) {
	m, root := buildModel(t, `package P {
		verification def Base {
			subject baseSubject;
			objective baseObjective;
		}
		verification def Derived :> Base {
			objective derivedObjective;
		}
	}`)
	p := sym(t, root, "P")
	base := nested(t, p.Scope, "Base")
	baseObjective := nested(t, base.Scope, "baseObjective")
	baseSubject := nested(t, base.Scope, "baseSubject")
	derivedObjective := nested(t, nested(t, p.Scope, "Derived").Scope, "derivedObjective")

	got := m.ImplicitRoleRedefinitions(derivedObjective)
	if len(got) != 1 || got[0] != baseObjective {
		t.Fatalf("ImplicitRoleRedefinitions(derivedObjective) = %v, want [baseObjective]", got)
	}
	if got[0] == baseSubject {
		t.Fatal("objective implicitly redefined the inherited subject")
	}
}

func TestImplicitRoleRedefinitionSurvivesExplicitRedefinition(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def X;
		requirement def A { subject s : X; }
		requirement def B { subject s : X; }
		requirement def C :> A, B { subject s :>> A::s; }
	}`)
	p := sym(t, root, "P")
	aSubject := nested(t, nested(t, p.Scope, "A").Scope, "s")
	bSubject := nested(t, nested(t, p.Scope, "B").Scope, "s")
	cSubject := nested(t, nested(t, p.Scope, "C").Scope, "s")

	if got := RelationshipsOf(cSubject); len(got) != 1 || got[0].Kind != ast.RelRedefines {
		t.Fatalf("RelationshipsOf(C's subject) = %v, want its one redefinition", got)
	}
	if got := m.ImplicitRoleRedefinitions(cSubject); len(got) != 1 || got[0] != bSubject {
		t.Fatalf("ImplicitRoleRedefinitions(C's subject) = %v, want [B::s]", got)
	}
	got := m.AllRedefinedFeatures(cSubject)
	if len(got) != 2 || got[0] != aSubject || got[1] != bSubject {
		t.Fatalf("AllRedefinedFeatures(C's subject) = %v, want [A::s B::s]", got)
	}
}

// A `render` member redefines the library `viewRendering` and nothing else: a
// rendering of another name in a derived view leaves the inherited one in
// place (SysML v2 8.3.26 RenderingUsage), as the pilot resolves it.
func TestRenderMemberRedefinesOnlyTheLibraryViewRendering(t *testing.T) {
	m, root := stdlibModelWithDoc(t, "views.sysml", `package P {
		rendering def AsTree;
		rendering def AsTable;
		view def Base { render rendering r : AsTree { attribute a; } }
		view def Derived :> Base { render rendering s : AsTable; }
		view def Twice :> Base { render rendering :>> r; }
	}`)
	p := sym(t, root, "P")
	r := nested(t, p.Scope, "Base", "r")
	derived := nested(t, p.Scope, "Derived")
	s := nested(t, derived.Scope, "s")

	if got := m.ImplicitRoleRedefinitions(s); len(got) != 1 || !symbols.HasFQN(got[0], viewRenderingFQN) {
		t.Fatalf("ImplicitRoleRedefinitions(s) = %v, want [%s]", got, viewRenderingFQN)
	}
	if got := m.AllRedefinedFeatures(s); len(got) != 1 || !symbols.HasFQN(got[0], viewRenderingFQN) {
		t.Fatalf("AllRedefinedFeatures(s) = %v, want [%s]", got, viewRenderingFQN)
	}
	if inherited, ok := m.LookupMember(derived, "r"); !ok || inherited != r {
		t.Fatalf("LookupMember(Derived, r) = %v, %v; want Base::r", inherited, ok)
	}
	if _, ok := m.LookupMember(r, "a"); !ok {
		t.Fatal("Base::r::a is not reachable through the inherited rendering")
	}

	twice := nested(t, p.Scope, "Twice")
	if got := m.ImplicitRoleRedefinitions(nested(t, twice.Scope, "r")); len(got) != 1 || !symbols.HasFQN(got[0], viewRenderingFQN) {
		t.Fatalf("ImplicitRoleRedefinitions(Twice's render) = %v, want [%s]", got, viewRenderingFQN)
	}
	if got, ok := m.LookupMember(twice, "r"); !ok || got.OwnerScope != twice.Scope {
		t.Fatalf("LookupMember(Twice, r) = %v, %v; want the redefining render", got, ok)
	}
}

// An analysis case may state several objectives; each redefines the general's
// objective at its own position, so a later one keeps that one's type and members.
func TestAnalysisObjectivesRedefineByPosition(t *testing.T) {
	m, root := buildModel(t, `package P {
		requirement def Min { attribute a; }
		requirement def Max { attribute b; }
		analysis def Base {
			objective cheapest : Min;
			objective widestMargin : Max;
		}
		analysis def Derived :> Base { objective; objective; objective; }
		analysis derived : Base { objective; objective; objective; }
	}`)
	p := sym(t, root, "P")
	base := nested(t, p.Scope, "Base")
	cheapest := nested(t, base.Scope, "cheapest")
	widestMargin := nested(t, base.Scope, "widestMargin")
	minDef := nested(t, p.Scope, "Min")
	maxDef := nested(t, p.Scope, "Max")
	for _, name := range []string{"Derived", "derived"} {
		owner := nested(t, p.Scope, name)
		owned, inherited := m.ObjectivesOf(owner)
		if len(owned) != 3 || len(inherited) != 0 {
			t.Fatalf("ObjectivesOf(%s) = %v, %v; want three owned, none inherited", name, owned, inherited)
		}
		first, second, third := owned[0], owned[1], owned[2]
		if got := m.ImplicitRoleRedefinitions(first); len(got) != 1 || got[0] != cheapest {
			t.Errorf("ImplicitRoleRedefinitions(%s's first objective) = %v, want [cheapest]", name, got)
		}
		if got := m.AllSupertypes(first); len(got) != 2 || got[0] != cheapest || got[1] != minDef {
			t.Errorf("AllSupertypes(%s's first objective) = %v, want [cheapest Min]", name, got)
		}
		if got := m.ImplicitRoleRedefinitions(second); len(got) != 1 || got[0] != widestMargin {
			t.Errorf("ImplicitRoleRedefinitions(%s's second objective) = %v, want [widestMargin]", name, got)
		}
		if got := m.AllSupertypes(second); len(got) != 2 || got[0] != widestMargin || got[1] != maxDef {
			t.Errorf("AllSupertypes(%s's second objective) = %v, want [widestMargin Max]", name, got)
		}
		if got, ok := m.LookupMember(second, "b"); !ok || got != nested(t, maxDef.Scope, "b") {
			t.Errorf("LookupMember(%s's second objective, b) = %v, %v; want Max::b", name, got, ok)
		}
		if _, ok := m.LookupMember(second, "a"); ok {
			t.Errorf("%s's second objective sees Min::a through the first general objective", name)
		}
		if got := m.ImplicitRoleRedefinitions(third); len(got) != 0 {
			t.Errorf("ImplicitRoleRedefinitions(%s's third objective) = %v, want none: the general states two", name, got)
		}
		if got := m.AllSupertypes(third); len(got) != 0 {
			t.Errorf("AllSupertypes(%s's third objective) = %v, want none", name, got)
		}
	}
}

// A general restating only some objectives still presents the rest at their
// positions, so a specialization's later objective finds the one it redefines.
func TestAnalysisObjectivesRedefineThroughPartialRestatement(t *testing.T) {
	m, root := buildModel(t, `package P {
		requirement def Min { attribute a; }
		requirement def Max { attribute b; }
		analysis def Base {
			objective cheapest : Min;
			objective widestMargin : Max;
		}
		analysis def Mid :> Base { objective; }
		analysis def Derived :> Mid { objective; objective; }
		analysis derived : Mid { objective; objective; }
		analysis def Deep :> Derived { objective; objective; }
	}`)
	p := sym(t, root, "P")
	base := nested(t, p.Scope, "Base")
	cheapest := nested(t, base.Scope, "cheapest")
	widestMargin := nested(t, base.Scope, "widestMargin")
	minA := nested(t, nested(t, p.Scope, "Min").Scope, "a")
	maxB := nested(t, nested(t, p.Scope, "Max").Scope, "b")

	midOwned, midInherited := m.ObjectivesOf(nested(t, p.Scope, "Mid"))
	if len(midOwned) != 1 || len(midInherited) != 1 || midInherited[0] != widestMargin {
		t.Fatalf("ObjectivesOf(Mid) = %v, %v; want [<restated>], [widestMargin]", midOwned, midInherited)
	}
	restated := midOwned[0]
	for _, name := range []string{"Derived", "derived"} {
		owned, inherited := m.ObjectivesOf(nested(t, p.Scope, name))
		if len(owned) != 2 || len(inherited) != 0 {
			t.Fatalf("ObjectivesOf(%s) = %v, %v; want two owned, none inherited", name, owned, inherited)
		}
		if got := m.AllRedefinedFeatures(owned[0]); len(got) != 2 || got[0] != restated || got[1] != cheapest {
			t.Errorf("AllRedefinedFeatures(%s's first objective) = %v, want [Mid's restatement cheapest]", name, got)
		}
		if got, ok := m.LookupMember(owned[0], "a"); !ok || got != minA {
			t.Errorf("LookupMember(%s's first objective, a) = %v, %v; want Min::a", name, got, ok)
		}
		if got := m.ImplicitRoleRedefinitions(owned[1]); len(got) != 1 || got[0] != widestMargin {
			t.Errorf("ImplicitRoleRedefinitions(%s's second objective) = %v, want [widestMargin]", name, got)
		}
		if got, ok := m.LookupMember(owned[1], "b"); !ok || got != maxB {
			t.Errorf("LookupMember(%s's second objective, b) = %v, %v; want Max::b", name, got, ok)
		}
	}
	derivedOwned, _ := m.ObjectivesOf(nested(t, p.Scope, "Derived"))
	deepOwned, _ := m.ObjectivesOf(nested(t, p.Scope, "Deep"))
	if got := m.ImplicitRoleRedefinitions(deepOwned[1]); len(got) != 1 || got[0] != derivedOwned[1] {
		t.Errorf("ImplicitRoleRedefinitions(Deep's second objective) = %v, want [Derived's second]", got)
	}
	if got := m.AllRedefinedFeatures(deepOwned[1]); len(got) != 2 || got[0] != derivedOwned[1] || got[1] != widestMargin {
		t.Errorf("AllRedefinedFeatures(Deep's second objective) = %v, want [Derived's second widestMargin]", got)
	}
}

// Generals sharing an ancestor each present its objectives in full, whichever is
// walked first: a sibling restating one position does not hide the rest.
func TestAnalysisObjectivesSurviveDiamondPartialRestatement(t *testing.T) {
	m, root := buildModel(t, `package P {
		requirement def Min { attribute a; }
		requirement def Max { attribute b; }
		analysis def Base {
			objective cheapest : Min;
			objective widestMargin : Max;
		}
		analysis def Left :> Base { objective :>> widestMargin; }
		analysis def Right :> Base;
		analysis def Derived :> Left, Right { objective; objective; }
		analysis def Mirror :> Right, Left { objective; objective; }
	}`)
	p := sym(t, root, "P")
	base := nested(t, p.Scope, "Base")
	cheapest := nested(t, base.Scope, "cheapest")
	widestMargin := nested(t, base.Scope, "widestMargin")
	minA := nested(t, nested(t, p.Scope, "Min").Scope, "a")
	maxB := nested(t, nested(t, p.Scope, "Max").Scope, "b")
	leftOwned, leftInherited := m.ObjectivesOf(nested(t, p.Scope, "Left"))
	if len(leftOwned) != 1 || len(leftInherited) != 0 {
		t.Fatalf("ObjectivesOf(Left) = %v, %v; want [<restated>], []", leftOwned, leftInherited)
	}
	restated := leftOwned[0]
	for _, name := range []string{"Derived", "Mirror"} {
		owned, inherited := m.ObjectivesOf(nested(t, p.Scope, name))
		if len(owned) != 2 || len(inherited) != 0 {
			t.Fatalf("ObjectivesOf(%s) = %v, %v; want two owned, none inherited", name, owned, inherited)
		}
		if got := m.ImplicitRoleRedefinitions(owned[0]); len(got) != 2 || !containsAll(got, restated, cheapest) {
			t.Errorf("ImplicitRoleRedefinitions(%s's first objective) = %v, want Left's restatement and cheapest", name, got)
		}
		if got, ok := m.LookupMember(owned[0], "a"); !ok || got != minA {
			t.Errorf("LookupMember(%s's first objective, a) = %v, %v; want Min::a", name, got, ok)
		}
		if got := m.ImplicitRoleRedefinitions(owned[1]); len(got) != 1 || got[0] != widestMargin {
			t.Errorf("ImplicitRoleRedefinitions(%s's second objective) = %v, want [widestMargin]", name, got)
		}
		if got, ok := m.LookupMember(owned[1], "b"); !ok || got != maxB {
			t.Errorf("LookupMember(%s's second objective, b) = %v, %v; want Max::b", name, got, ok)
		}
		if _, ok := m.LookupMember(owned[1], "a"); ok {
			t.Errorf("%s's second objective sees Min::a through the first position", name)
		}
	}
}

func containsAll(got []*symbols.Symbol, want ...*symbols.Symbol) bool {
	for _, w := range want {
		found := false
		for _, g := range got {
			if g == w {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// Only the first owned objective redefines, and it takes the first objective
// of each general; ObjectivesOf then reports what remains inherited.
func TestObjectivesOfMasksOnlyTheFirstOfEachGeneral(t *testing.T) {
	m, root := buildModel(t, `package P {
		case def C { objective o1; objective o2; }
		case def C1 :> C { objective o5; }
		case def B1 { objective b1; }
		case def B2 { objective b2; }
		case def D :> B1, B2;
		case def D2 :> B1, B2 { objective d; }
		case def E :> B1 { objective e1; objective e2; }
		case def C3 :> C { objective o6 :>> o2; }
	}`)
	p := sym(t, root, "P")
	c := nested(t, p.Scope, "C")
	o1 := nested(t, c.Scope, "o1")
	o2 := nested(t, c.Scope, "o2")
	b1 := nested(t, nested(t, p.Scope, "B1").Scope, "b1")
	b2 := nested(t, nested(t, p.Scope, "B2").Scope, "b2")

	o5 := nested(t, nested(t, p.Scope, "C1").Scope, "o5")
	if got := m.ImplicitRoleRedefinitions(o5); len(got) != 1 || got[0] != o1 {
		t.Errorf("ImplicitRoleRedefinitions(C1::o5) = %v, want [o1]", got)
	}
	if owned, inherited := m.ObjectivesOf(nested(t, p.Scope, "C1")); len(owned) != 1 || len(inherited) != 1 || inherited[0] != o2 {
		t.Errorf("ObjectivesOf(C1) = %v, %v; want [o5], [o2]", owned, inherited)
	}
	if owned, inherited := m.ObjectivesOf(nested(t, p.Scope, "D")); len(owned) != 0 || len(inherited) != 2 || inherited[0] != b1 || inherited[1] != b2 {
		t.Errorf("ObjectivesOf(D) = %v, %v; want [], [b1 b2]", owned, inherited)
	}
	if owned, inherited := m.ObjectivesOf(nested(t, p.Scope, "D2")); len(owned) != 1 || len(inherited) != 0 {
		t.Errorf("ObjectivesOf(D2) = %v, %v; want [d], []", owned, inherited)
	}
	e := nested(t, p.Scope, "E")
	if got := m.ImplicitRoleRedefinitions(nested(t, e.Scope, "e2")); len(got) != 0 {
		t.Errorf("ImplicitRoleRedefinitions(E::e2) = %v, want none: only the first owned objective redefines", got)
	}
	if owned, inherited := m.ObjectivesOf(e); len(owned) != 2 || len(inherited) != 0 {
		t.Errorf("ObjectivesOf(E) = %v, %v; want [e1 e2], []", owned, inherited)
	}
	if owned, inherited := m.ObjectivesOf(nested(t, p.Scope, "C3")); len(owned) != 1 || len(inherited) != 0 {
		t.Errorf("ObjectivesOf(C3) = %v, %v; want [o6], []: o6 redefines o2 by clause and o1 by role", owned, inherited)
	}
}

// A restatement deeper in one branch of a diamond replaces the common ancestor's
// objective seen through the other, whichever branch is written first, so a
// further specialization's positions line up with the restatement.
func TestAnalysisObjectivesAlignAcrossUnevenDiamond(t *testing.T) {
	m, root := buildModel(t, `package P {
		analysis def Base { objective b1; objective b2; }
		analysis def A :> Base;
		analysis def Mid :> Base { objective m1; }
		analysis def B :> Mid;
		analysis def D :> A, B;
		analysis def Reversed :> B, A;
		analysis def E :> D { objective e1; objective e2; objective e3; }
		analysis def F :> Reversed { objective f1; objective f2; objective f3; }
	}`)
	p := sym(t, root, "P")
	base := nested(t, p.Scope, "Base")
	b1 := nested(t, base.Scope, "b1")
	b2 := nested(t, base.Scope, "b2")
	m1 := nested(t, nested(t, p.Scope, "Mid").Scope, "m1")
	for _, name := range []string{"D", "Reversed"} {
		owned, inherited := m.ObjectivesOf(nested(t, p.Scope, name))
		if len(owned) != 0 || len(inherited) != 2 || !contains(inherited, m1) || !contains(inherited, b2) {
			t.Errorf("ObjectivesOf(%s) = %v, %v; want [], {m1 b2}", name, owned, inherited)
		}
	}
	for _, tc := range []struct {
		owner string
		names [3]string
	}{{"E", [3]string{"e1", "e2", "e3"}}, {"F", [3]string{"f1", "f2", "f3"}}} {
		owner := nested(t, p.Scope, tc.owner)
		first := nested(t, owner.Scope, tc.names[0])
		if got := m.ImplicitRoleRedefinitions(first); len(got) != 1 || got[0] != m1 {
			t.Errorf("ImplicitRoleRedefinitions(%s::%s) = %v, want [m1]", tc.owner, tc.names[0], got)
		}
		if got := m.AllSupertypes(first); len(got) != 2 || got[0] != m1 || got[1] != b1 {
			t.Errorf("AllSupertypes(%s::%s) = %v, want [m1 b1]", tc.owner, tc.names[0], got)
		}
		if got := m.ImplicitRoleRedefinitions(nested(t, owner.Scope, tc.names[1])); len(got) != 1 || got[0] != b2 {
			t.Errorf("ImplicitRoleRedefinitions(%s::%s) = %v, want [b2]", tc.owner, tc.names[1], got)
		}
		if got := m.ImplicitRoleRedefinitions(nested(t, owner.Scope, tc.names[2])); len(got) != 0 {
			t.Errorf("ImplicitRoleRedefinitions(%s::%s) = %v, want none: the generals state two", tc.owner, tc.names[2], got)
		}
	}
}

// Reference subsetting is a subsetting, so a usage inherits the roles of the usage it
// references: alone, beside an owned one (which redefines it by role), beside one
// inherited from its definition, and through a chain of references.
func TestRolesInheritThroughReferenceSubsetting(t *testing.T) {
	m, root := buildModel(t, `package P {
		case def CD { objective o; }
		case c0 { objective o0; }
		case alone ::> c0;
		case owned ::> c0 { objective o1; }
		case both : CD ::> c0;
		case mid ::> c0;
		case deep : CD ::> mid;
		requirement def RD { subject s; }
		requirement r0 { subject s0; }
		requirement rAlone ::> r0;
		requirement rOwned ::> r0 { subject s1; }
		requirement rBoth : RD ::> r0;
	}`)
	p := sym(t, root, "P")
	o := nested(t, nested(t, p.Scope, "CD").Scope, "o")
	o0 := nested(t, nested(t, p.Scope, "c0").Scope, "o0")
	s := nested(t, nested(t, p.Scope, "RD").Scope, "s")
	s0 := nested(t, nested(t, p.Scope, "r0").Scope, "s0")

	if owned, inherited := m.ObjectivesOf(nested(t, p.Scope, "alone")); len(owned) != 0 || len(inherited) != 1 || inherited[0] != o0 {
		t.Errorf("ObjectivesOf(alone) = %v, %v; want [], [o0]", owned, inherited)
	}
	o1 := nested(t, nested(t, p.Scope, "owned").Scope, "o1")
	if got := m.ImplicitRoleRedefinitions(o1); len(got) != 1 || got[0] != o0 {
		t.Errorf("ImplicitRoleRedefinitions(owned::o1) = %v, want [o0]", got)
	}
	if owned, inherited := m.ObjectivesOf(nested(t, p.Scope, "owned")); len(owned) != 1 || len(inherited) != 0 {
		t.Errorf("ObjectivesOf(owned) = %v, %v; want [o1], []", owned, inherited)
	}
	for _, name := range []string{"both", "deep"} {
		owned, inherited := m.ObjectivesOf(nested(t, p.Scope, name))
		if len(owned) != 0 || len(inherited) != 2 || inherited[0] != o || inherited[1] != o0 {
			t.Errorf("ObjectivesOf(%s) = %v, %v; want [], [o o0]", name, owned, inherited)
		}
	}
	if got := m.SubjectParameterOf(nested(t, p.Scope, "rAlone")); got != s0 {
		t.Errorf("SubjectParameterOf(rAlone) = %v, want s0", got)
	}
	s1 := nested(t, nested(t, p.Scope, "rOwned").Scope, "s1")
	if got := m.ImplicitRoleRedefinitions(s1); len(got) != 1 || got[0] != s0 {
		t.Errorf("ImplicitRoleRedefinitions(rOwned::s1) = %v, want [s0]", got)
	}
	if owned, inherited := m.SubjectsOf(nested(t, p.Scope, "rBoth")); len(owned) != 0 || len(inherited) != 2 || inherited[0] != s || inherited[1] != s0 {
		t.Errorf("SubjectsOf(rBoth) = %v, %v; want [], [s s0]", owned, inherited)
	}
}

// An actor or stakeholder redefines the general's effective actor at its position, named
// alike or not, unless a `:>>` clause governs; a general's own actors come before inherited ones.
func TestActorsRedefineByPosition(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def Ship; part def Pilot; part def Tug;
		requirement def Piloted { subject t : Ship; actor pilots : Pilot[2]; actor escort : Tug; }
		requirement same : Piloted { actor pilots; actor escort; }
		requirement renamed : Piloted { actor crew; actor tug; }
		requirement explicit : Piloted { actor :>> escort; }
		requirement def Guarded :> Piloted { subject :>> t; actor guard : Ship; }
		requirement guarded : Guarded { actor a; actor b; actor c; }
		requirement def Owned { subject t : Ship; stakeholder owner : Pilot; actor driver : Pilot; }
		requirement owned : Owned { stakeholder o; actor d; }
		use case def Ride { subject vehicle : Ship; actor driver : Pilot; }
		use case ride : Ride { actor d; }
	}`)
	p := sym(t, root, "P")
	piloted := nested(t, p.Scope, "Piloted")
	pilots, escort := nested(t, piloted.Scope, "pilots"), nested(t, piloted.Scope, "escort")
	pilotDef, shipDef := nested(t, p.Scope, "Pilot"), nested(t, p.Scope, "Ship")
	guard := nested(t, nested(t, p.Scope, "Guarded").Scope, "guard")
	ride, ownedDef := nested(t, p.Scope, "Ride"), nested(t, p.Scope, "Owned")

	for _, tc := range []struct {
		owner, actor string
		want         []*symbols.Symbol
	}{
		{"same", "pilots", []*symbols.Symbol{pilots}},
		{"same", "escort", []*symbols.Symbol{escort}},
		{"renamed", "crew", []*symbols.Symbol{pilots}},
		{"renamed", "tug", []*symbols.Symbol{escort}},
		{"explicit", "escort", nil},
		{"Guarded", "guard", []*symbols.Symbol{pilots}},
		{"guarded", "a", []*symbols.Symbol{guard}},
		{"guarded", "b", []*symbols.Symbol{escort}},
		{"guarded", "c", nil},
		{"owned", "o", []*symbols.Symbol{nested(t, ownedDef.Scope, "owner")}},
		{"owned", "d", []*symbols.Symbol{nested(t, ownedDef.Scope, "driver")}},
		{"ride", "d", []*symbols.Symbol{nested(t, ride.Scope, "driver")}},
	} {
		actor := nested(t, nested(t, p.Scope, tc.owner).Scope, tc.actor)
		if got := m.ImplicitRoleRedefinitions(actor); !slices.Equal(got, tc.want) {
			t.Errorf("ImplicitRoleRedefinitions(%s::%s) = %v, want %v", tc.owner, tc.actor, got, tc.want)
		}
	}
	crew := nested(t, nested(t, p.Scope, "renamed").Scope, "crew")
	if got := m.AllSupertypes(crew); len(got) < 2 || got[0] != pilots || got[1] != pilotDef {
		t.Errorf("AllSupertypes(renamed::crew) = %v, want [pilots Pilot ...]", got)
	}
	a := nested(t, nested(t, p.Scope, "guarded").Scope, "a")
	if got := m.AllSupertypes(a); !containsAll(got, guard, shipDef, pilots, pilotDef) {
		t.Errorf("AllSupertypes(guarded::a) = %v, want guard, Ship, pilots and Pilot among them", got)
	}
}

// A restatement of an actor met through one branch of a diamond stands for the actor it
// restates met through the other, whichever branch is written first.
func TestActorRestatementWinsAcrossDiamond(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def A; part def R :> A; part def Ship;
		use case def Base { subject s : Ship; actor a : A; }
		use case def Left :> Base;
		use case def Right :> Base { actor r : R; }
		use case def LR :> Left, Right;
		use case def RL :> Right, Left;
		use case lr : LR { actor x; }
		use case rl : RL { actor y; }
		use case def Both :> Left, Right { actor z; }
		use case def Deep :> LR { actor w; actor extra : A; }
	}`)
	p := sym(t, root, "P")
	r := nested(t, nested(t, p.Scope, "Right").Scope, "r")
	rDef := nested(t, p.Scope, "R")
	for _, tc := range []struct{ owner, actor string }{{"lr", "x"}, {"rl", "y"}, {"Both", "z"}, {"Deep", "w"}} {
		actor := nested(t, nested(t, p.Scope, tc.owner).Scope, tc.actor)
		if got := m.ImplicitRoleRedefinitions(actor); len(got) != 1 || got[0] != r {
			t.Errorf("ImplicitRoleRedefinitions(%s::%s) = %v, want [r]", tc.owner, tc.actor, got)
		}
		if got := m.AllSupertypes(actor); !containsAll(got, r, rDef) {
			t.Errorf("AllSupertypes(%s::%s) = %v, want r and R among them", tc.owner, tc.actor, got)
		}
	}
	extra := nested(t, nested(t, p.Scope, "Deep").Scope, "extra")
	if got := m.ImplicitRoleRedefinitions(extra); len(got) != 0 {
		t.Errorf("ImplicitRoleRedefinitions(Deep::extra) = %v, want none: the diamond has one actor", got)
	}
}

// Actors through a tower of diamonds are read once per case, not once per path.
func TestActorsThroughLayeredDiamondsAreLinear(t *testing.T) {
	const layers = 60
	var b strings.Builder
	b.WriteString("package P {\n\tpart def A;\n\tuse case def C0 { subject s; actor a0 : A; }\n")
	for i := 1; i <= layers; i++ {
		fmt.Fprintf(&b, "\tuse case def L%d :> C%d;\n\tuse case def R%d :> C%d;\n", i, i-1, i, i-1)
		if i == layers/2 {
			fmt.Fprintf(&b, "\tuse case def C%d :> L%d, R%d { actor mid; }\n", i, i, i)
		} else {
			fmt.Fprintf(&b, "\tuse case def C%d :> L%d, R%d;\n", i, i, i)
		}
	}
	fmt.Fprintf(&b, "\tuse case def Top :> C%d { actor top; }\n}", layers)
	m, root := buildModel(t, b.String())
	p := sym(t, root, "P")
	a0 := nested(t, nested(t, p.Scope, "C0").Scope, "a0")
	mid := nested(t, nested(t, p.Scope, fmt.Sprintf("C%d", layers/2)).Scope, "mid")
	top := nested(t, nested(t, p.Scope, "Top").Scope, "top")
	if got := m.ImplicitRoleRedefinitions(mid); len(got) != 1 || got[0] != a0 {
		t.Errorf("ImplicitRoleRedefinitions(mid) = %v, want [a0]", got)
	}
	if got := m.ImplicitRoleRedefinitions(top); len(got) != 1 || got[0] != mid {
		t.Errorf("ImplicitRoleRedefinitions(top) = %v, want [mid]", got)
	}
	if got := m.AllSupertypes(top); !containsAll(got, mid, a0, nested(t, p.Scope, "A")) {
		t.Errorf("AllSupertypes(top) = %v, want mid, a0 and A among them", got)
	}
}

// The subject parameter is the one that survives redefinition: when one branch of a
// diamond restates the common ancestor's subject, the restatement wins whichever branch
// is written first, and whether the branch is a general or a referenced usage.
func TestSubjectParameterSurvivesDiamondRedefinition(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def A;
		part def B :> A;
		requirement def Base { subject s : A; }
		requirement def L :> Base;
		requirement r : Base { subject s2 : B :>> s; }
		requirement d : L ::> r;
		requirement def D :> L, Base { subject s3 : B :>> s; }
		requirement def E :> L, D;
		requirement def F :> D, L;
		requirement e : E;
	}`)
	p := sym(t, root, "P")
	s2 := nested(t, nested(t, p.Scope, "r").Scope, "s2")
	s3 := nested(t, nested(t, p.Scope, "D").Scope, "s3")
	for _, tc := range []struct {
		name string
		want *symbols.Symbol
	}{{"d", s2}, {"E", s3}, {"F", s3}, {"e", s3}} {
		if got := m.SubjectParameterOf(nested(t, p.Scope, tc.name)); got != tc.want {
			t.Errorf("SubjectParameterOf(%s) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// Objectives through a tower of diamonds are read once per case, not once per path:
// sixty stacked diamonds have 2^60 paths, and the restatement halfway up still wins.
func TestObjectivesThroughLayeredDiamondsAreLinear(t *testing.T) {
	const layers = 60
	var b strings.Builder
	b.WriteString("package P {\n\tverification def C0 { objective o0; }\n")
	for i := 1; i <= layers; i++ {
		fmt.Fprintf(&b, "\tverification def L%d :> C%d;\n\tverification def R%d :> C%d;\n", i, i-1, i, i-1)
		if i == layers/2 {
			fmt.Fprintf(&b, "\tverification def C%d :> L%d, R%d { objective mid; }\n", i, i, i)
		} else {
			fmt.Fprintf(&b, "\tverification def C%d :> L%d, R%d;\n", i, i, i)
		}
	}
	fmt.Fprintf(&b, "\tverification def Top :> C%d { objective top; }\n}", layers)
	m, root := buildModel(t, b.String())
	p := sym(t, root, "P")
	o0 := nested(t, nested(t, p.Scope, "C0").Scope, "o0")
	mid := nested(t, nested(t, p.Scope, fmt.Sprintf("C%d", layers/2)).Scope, "mid")
	top := nested(t, nested(t, p.Scope, "Top").Scope, "top")
	if got := m.ImplicitRoleRedefinitions(mid); len(got) != 1 || got[0] != o0 {
		t.Errorf("ImplicitRoleRedefinitions(mid) = %v, want [o0]", got)
	}
	if got := m.ImplicitRoleRedefinitions(top); len(got) != 1 || got[0] != mid {
		t.Errorf("ImplicitRoleRedefinitions(top) = %v, want [mid]", got)
	}
	if owned, inherited := m.ObjectivesOf(nested(t, p.Scope, fmt.Sprintf("C%d", layers))); len(owned) != 0 || len(inherited) != 1 || inherited[0] != mid {
		t.Errorf("ObjectivesOf(C%d) = %v, %v; want [], [mid]", layers, owned, inherited)
	}
}
