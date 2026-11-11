package resolve_test

import (
	"slices"
	"strings"
	"testing"
)

// unresolvedIn is the names src leaves unresolved, sorted.
func unresolvedIn(t *testing.T, src string) []string {
	t.Helper()
	r, _, _ := resolvedDoc(t, src)
	const lead = "unresolved reference: "
	var names []string
	for _, d := range r.Diagnostics {
		_, rest, ok := strings.Cut(d.Message, lead)
		if !ok {
			t.Fatalf("unexpected diagnostic: %s", d.Message)
		}
		name, _, _ := strings.Cut(rest, " ")
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func wantUnresolved(t *testing.T, src string, want ...string) {
	t.Helper()
	if got := unresolvedIn(t, src); !slices.Equal(got, want) {
		t.Fatalf("unresolved names = %v, want %v", got, want)
	}
}

// An imported membership has its Import's visibility, not its declaration's
// (KerML 8.3.2.4.5 visibilityOf), so a qualified name follows the import.
func TestQualifiedNameFollowsTheAdmittingImport(t *testing.T) {
	wantUnresolved(t, `
		package S { private part def H; part def V; }
		package PubAll { public import all S::*; }
		package PrivAll { private import all S::*; }
		package Pub { public import S::*; }
		package PubMember { public import all S::H; }
		package Use {
			part a : PubAll::H;
			part b : PubAll::V;
			part c : PrivAll::H;
			part d : Pub::H;
			part e : Pub::V;
			part f : PrivAll::V;
			part g : PubMember::H;
		}
		package Onward { private import PubAll::*; part h : H; }
		package OnwardMember { private import PubMember::*; part h : H; }
	`, "PrivAll::H", "PrivAll::V", "Pub::H")
}

// An `import all` of one membership names any membership of the namespace, a
// privately imported one included, but not one its import never took.
func TestImportAllNamesAPrivatelyImportedMembership(t *testing.T) {
	wantUnresolved(t, `
		package S { private part def H; part def V; part def W; }
		package Mid { private import S::*; }
		package MidMember { private import S::W; }
		package Use {
			private import all Mid::V;
			private import all MidMember::W;
			part v : V;
			part w : W;
		}
		package Never { private import all Mid::H; }
		package Ordinary { private import Mid::V; }
	`, "Mid::H", "Mid::V")
}

// `import all Mid::*` takes Mid's private member and what Mid imported
// privately, never what Mid's own ordinary import left behind.
func TestImportAllTakesEveryMembershipOfItsTargetOnly(t *testing.T) {
	const model = `
		package ReviewSource { public part def Visible; private part def Hidden; }
		package Mid { private import ReviewSource::*; private part def OwnHidden; }
	`
	wantUnresolved(t, model+`
		package Use { private import all Mid::*; part a : OwnHidden; part b : Visible; part c : Hidden; }
	`, "Hidden")
	wantUnresolved(t, model+`
		package Use { private import Mid::*; part a : OwnHidden; part b : Visible; part c : Hidden; }
	`, "Hidden", "OwnHidden", "Visible")
	wantUnresolved(t, model+`
		package Onward { private import all Mid::*; }
		package Use { private import all Onward::*; part a : OwnHidden; part b : Visible; part c : Hidden; }
	`, "Hidden")
	wantUnresolved(t, model+`
		package Onward { private import all Mid::*; }
		package Use { private import Onward::*; part a : OwnHidden; part b : Visible; part c : Hidden; }
	`, "Hidden", "OwnHidden", "Visible")
}

// A recursive import descends the namespaces its target owns, the private ones
// only under `import all`, and not the ones imported there (KerML 8.3.2.4.5).
func TestRecursiveImportDescendsOwnedNamespaces(t *testing.T) {
	wantUnresolved(t, `
		package Lib {
			part def A { part def Deep; }
			private part def P { part def PDeep; }
		}
		package Mid { public import Lib::*; part def M { part def MDeep; } }
		package ThroughMid { private import Mid::**; part d : Deep; part m : MDeep; part a : A; }
		package Ordinary { private import Lib::**; part d : Deep; part p : PDeep; }
		package All { private import all Lib::**; part d : Deep; part p : PDeep; }
	`, "Deep", "PDeep")
}

// A filtered import takes what its condition selects of its target's public
// memberships, and an `import all` of the importer takes no more than that.
func TestImportAllKeepsAnIntermediateFilter(t *testing.T) {
	wantUnresolved(t, `
		package Meta { metadata def Safety; }
		package S {
			part def Tagged { @Meta::Safety; }
			part def Plain;
			private part def HiddenTagged { @Meta::Safety; }
		}
		package Mid { private import S::*[@Meta::Safety]; }
		package Use { private import all Mid::*; part a : Tagged; part b : Plain; part c : HiddenTagged; }
		package Member { private import all Mid::Tagged; private import all Mid::Plain; }
	`, "HiddenTagged", "Mid::Plain", "Plain")
}

// Two namespaces importing all of each other each hold the other's own
// memberships once: the import of an excluded namespace is ignored.
func TestImportAllCycleTerminates(t *testing.T) {
	const model = `
		package A { private import all B::*; private part def AH; }
		package B { private import all A::*; private part def BH; }
	`
	wantUnresolved(t, model+`package Use { private import all A::*; part x : AH; part y : BH; }`)
	wantUnresolved(t, model+`package Use { private import A::*; part x : AH; part y : BH; }`, "AH", "BH")
	wantUnresolved(t, `
		package C1 { public import all C2::*; private part def InC1; }
		package C2 { public import all C1::*; private part def InC2; }
		package Use { private import C1::*; part x : InC2; }
		package Use2 { private import C2::*; part y : InC1; part z : C2::InC1; }
	`)
}
