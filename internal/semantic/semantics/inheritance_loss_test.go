package semantics

import (
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// lookedUp is LookupMember's answer, nil when sym has no member of that name.
func lookedUp(m *Model, sym *symbols.Symbol, name string) *symbols.Symbol {
	found, _ := m.LookupMember(sym, name)
	return found
}

// An unnamed redefinition takes the redefined feature's name and short name
// (KerML 8.3.3.3.4 effectiveName), not the alias another membership gives it.
func TestUnnamedRedefinitionDoesNotAnswerToAnAliasOfTheRedefinedFeature(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def A { part <s> a; alias aa for a; }
		part def B :> A { part :>> a; }
		part def C :> A { part c :>> a; }
		part def D :> B;
	}`)
	p := sym(t, root, "P")
	b := nested(t, p.Scope, "B")
	redefiner := b.Scope.LookupLocalAll("a")[0]
	for _, owner := range []string{"B", "D"} {
		typ := nested(t, p.Scope, owner)
		for _, name := range []string{"a", "s"} {
			if got := lookedUp(m, typ, name); got != redefiner {
				t.Errorf("LookupMember(%s, %s) = %v, want B's redefinition", owner, name, got)
			}
		}
		if got := lookedUp(m, typ, "aa"); got != nil {
			t.Errorf("LookupMember(%s, aa) = %v, want none: the alias names the removed feature", owner, got)
		}
		if names := visibleNames(m, typ); names["aa"] != 0 {
			t.Errorf("%s still lists the alias: %v", owner, names)
		}
	}
	c := nested(t, p.Scope, "C")
	for _, name := range []string{"a", "s", "aa"} {
		if got := lookedUp(m, c, name); got != nil {
			t.Errorf("LookupMember(C, %s) = %v, want none under a named redefinition", name, got)
		}
	}
}

// An inherited feature that redefines what an owned feature directly redefines
// is removed with it (KerML 8.3.3.1.10 removeRedefinedFeatures, condition 2).
func TestOwnedRedefinitionRemovesAnInheritedRedefinitionOfTheSameFeature(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def A { part a; }
		part def B :> A { part b :>> a; }
		part def C :> B { part c :>> A::a; }
		part def D :> C;
		part def E :> B { part e :>> b; }
	}`)
	p := sym(t, root, "P")
	a := nested(t, p.Scope, "A", "a")
	b := nested(t, p.Scope, "B", "b")
	for _, owner := range []string{"C", "D"} {
		typ := nested(t, p.Scope, owner)
		if !m.InheritanceMasked(typ, b) || !m.InheritanceMasked(typ, a) {
			t.Errorf("%s must inherit neither a nor b", owner)
		}
		if names := visibleNames(m, typ); names["a"] != 0 || names["b"] != 0 || names["c"] != 1 {
			t.Errorf("%s lists %v, want c alone", owner, names)
		}
		if got := lookedUp(m, typ, "b"); got != nil {
			t.Errorf("LookupMember(%s, b) = %v, want none", owner, got)
		}
	}
	c := nested(t, p.Scope, "C")
	if !m.OwnRedefinitionMasked(c, b) {
		t.Errorf("C's own redefinition of a keeps b from its specializations")
	}
	if got := m.RedefinedFeatures(nested(t, c.Scope, "c")); len(got) != 1 || got[0] != a {
		t.Errorf("RedefinedFeatures(C::c) = %v, want [A::a]", got)
	}
	if names := visibleNames(m, nested(t, p.Scope, "E")); names["b"] != 0 || names["a"] != 0 || names["e"] != 1 {
		t.Errorf("E lists %v, want e alone", names)
	}
}

// A private redefinition is not inheritable (KerML 8.3.3.1.10 inheritableMemberships),
// so the feature it redefines is still inherited along another specialization.
func TestPrivateRedefinitionRemovesNothingBeyondItsOwner(t *testing.T) {
	for _, generals := range []string{"B, D", "D, B"} {
		m, root := buildModel(t, `package P {
			part def A { part a; }
			part def B :> A { private part b :>> a; }
			part def D :> A;
			part def E :> `+generals+`;
			part def F :> B;
		}`)
		p := sym(t, root, "P")
		a := nested(t, p.Scope, "A", "a")
		e := nested(t, p.Scope, "E")
		if got := lookedUp(m, e, "a"); got != a {
			t.Errorf("E :> %s: LookupMember(E, a) = %v, want A::a through D", generals, got)
		}
		if m.InheritanceMasked(e, a) || visibleNames(m, e)["a"] != 1 {
			t.Errorf("E :> %s: a must be inherited once: %v", generals, visibleNames(m, e))
		}
		if got, ok := m.LookupContributedMember(e, "a"); !ok || got != a {
			t.Errorf("E :> %s: LookupContributedMember(E, a) = %v, want A::a", generals, got)
		}
		f := nested(t, p.Scope, "F")
		if got := lookedUp(m, f, "a"); got != nil {
			t.Errorf("LookupMember(F, a) = %v, want none: B, its only general, does not hold a", got)
		}
		if _, ok := m.LookupContributedMember(f, "a"); ok {
			t.Errorf("F :> B is passed no a")
		}
	}
}

// A redefining feature a supertype has itself lost removes nothing further down:
// C loses b, so D, inheriting a2 through X, keeps it, while c still removes a.
func TestRedefinitionLostBySupertypeRemovesNothingBelowIt(t *testing.T) {
	for _, generals := range []string{"C, X", "X, C"} {
		m, root := buildModel(t, `package P {
			part def A { part a; part a2; }
			part def B :> A { part b :>> a, a2; }
			part def C :> B { part c :>> A::a; }
			part def X :> A;
			part def D :> `+generals+`;
		}`)
		p := sym(t, root, "P")
		d := nested(t, p.Scope, "D")
		names := visibleNames(m, d)
		if names["a2"] != 1 || names["c"] != 1 || names["a"] != 0 || names["b"] != 0 {
			t.Errorf("D :> %s lists %v, want a2 and c", generals, names)
		}
		if got := lookedUp(m, d, "a2"); got != nested(t, p.Scope, "A", "a2") {
			t.Errorf("D :> %s: LookupMember(D, a2) = %v, want A::a2", generals, got)
		}
		for _, name := range []string{"a", "b"} {
			if got := lookedUp(m, d, name); got != nil {
				t.Errorf("D :> %s: LookupMember(D, %s) = %v, want none", generals, name, got)
			}
		}
	}
}

// A public redefinition one general passes on removes the feature another
// general still holds (KerML 8.3.3.1.10 removeRedefinedFeatures, condition 1).
func TestPassedOnRedefinitionRemovesTheFeatureAnotherGeneralHolds(t *testing.T) {
	for _, generals := range []string{"B, D", "D, B"} {
		m, root := buildModel(t, `package P {
			part def A { part a; }
			part def B :> A { part b :>> a; }
			part def D :> A;
			part def E :> `+generals+`;
		}`)
		p := sym(t, root, "P")
		e := nested(t, p.Scope, "E")
		if got := lookedUp(m, e, "a"); got != nil {
			t.Errorf("E :> %s: LookupMember(E, a) = %v, want none", generals, got)
		}
		if names := visibleNames(m, e); names["a"] != 0 || names["b"] != 1 {
			t.Errorf("E :> %s lists %v, want b alone", generals, names)
		}
	}
}

// The unnamed redefinition a type declares stands in for the redefined feature
// in what the type passes on, and is never among what the type loses.
func TestOwnUnnamedRedefinitionIsWhatTheTypePassesOn(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def A { part a; }
		part def B :> A { part :>> a; }
	}`)
	p := sym(t, root, "P")
	a := nested(t, p.Scope, "A", "a")
	b := nested(t, p.Scope, "B")
	redefiner := b.Scope.LookupLocalAll("a")[0]
	if !m.OwnRedefinitionMasked(b, a) || m.OwnRedefinitionMasked(b, redefiner) {
		t.Fatalf("B's own redefinition removes A::a and not itself")
	}
	if got := m.OwnNamingRedefiner(b, a); got != redefiner {
		t.Fatalf("OwnNamingRedefiner(B, A::a) = %v, want B's redefinition", got)
	}
}

// A reader served a memoized loss depends on the document declaring the type,
// so editing that document reaches what the reader derived from it.
func TestInheritanceLossReadersDependOnTheDeclaringDocument(t *testing.T) {
	idx := symbols.NewIndex()
	r := resolve.New(idx)
	m := NewModel(r)
	r.SetModel(m)
	r.Track()
	add := func(name, src string) {
		p := parser.New(source.New(name, []byte(src)))
		root := p.ParseFile()
		if len(p.Diagnostics) != 0 {
			t.Fatalf("parse diagnostics: %v", p.Diagnostics)
		}
		idx.AddDocument(name, root)
	}
	add("a.sysml", "package A { part def Base { part a; } part def Derived :> Base { part b :>> a; } }")
	add("b.sysml", "package B { part def Other; }")
	r.Invalidate(idx.TakeChanges())
	pkg := sym(t, idx.DocumentRoot("a.sysml"), "A")
	derived := sym(t, pkg.Scope, "Derived")
	a := nested(t, pkg.Scope, "Base", "a")
	masked := func(from string) bool {
		var out bool
		r.InDocument(from, func() { out = m.InheritanceMasked(derived, a) })
		return out
	}
	if !masked("a.sysml") {
		t.Fatalf("Derived must not inherit a")
	}
	if _, ok := m.redefLoss[derived]; !ok {
		t.Fatalf("the settled loss of Derived was not memoized")
	}
	if !masked("b.sysml") {
		t.Fatalf("the memoized loss of Derived must answer alike")
	}
	if deps := r.Dependents("a.sysml"); len(deps) != 1 || deps[0] != "b.sysml" {
		t.Fatalf("a.sysml's dependents = %v, want b.sysml, served from the memo", deps)
	}
	add("a.sysml", "package A { part def Base { part a; } part def Derived :> Base; }")
	ch := idx.TakeChanges()
	ch.Docs = map[string]bool{"a.sysml": true}
	r.Invalidate(ch)
	if len(m.redefLoss) != 0 || len(m.redefOwn) != 0 {
		t.Fatalf("%d losses and %d own redefinitions survive the edit, want none", len(m.redefLoss), len(m.redefOwn))
	}
}

// A redefinition naming a feature a general has lost stays unresolved whichever sibling
// redefinition is resolved first (pilot Xpect VisibilityTests_Protected_Redefine_0).
func TestLostRedefinitionTargetStaysUnresolvedBesideASiblingRedefinition(t *testing.T) {
	for _, members := range []string{
		"feature gg redefines bb; feature jj redefines PPP1;",
		"feature jj redefines PPP1; feature gg redefines bb;",
	} {
		m, root, r, doc := buildUnresolvedModel(t, "t.kerml", source.KindKerML, `package P {
			classifier PP { protected feature PPP1; }
			classifier A specializes PP { feature bb redefines PPP1; }
			classifier B specializes A {}
			classifier C specializes B { `+members+` }
		}`)
		r.ResolveDocument("t.kerml", doc)
		if len(r.Diagnostics) != 1 {
			t.Errorf("%q: diagnostics = %v, want PPP1 alone unresolved", members, r.Diagnostics)
		}
		c := nested(t, sym(t, root, "P").Scope, "C")
		if got := m.RedefinedFeatures(nested(t, c.Scope, "jj")); len(got) != 0 {
			t.Errorf("%q: RedefinedFeatures(C::jj) = %v, want none", members, got)
		}
	}
}

// A redefinition target resolved in a type asks for that type's loss while the type's own
// redefinitions are unsettled: its supertype's loss is settled all the same, and stays memoized.
func TestSupertypeLossStaysMemoizedWhenTheSubtypeWalkIsCutShort(t *testing.T) {
	m, root, r, _ := buildUnresolvedModel(t, "t.sysml", source.KindSysML,
		"package P { part def Base { part a; } part def Derived :> Base { part b :>> a; } }")
	pkg := sym(t, root, "P")
	base, derived := sym(t, pkg.Scope, "Base"), sym(t, pkg.Scope, "Derived")
	b := nested(t, pkg.Scope, "Derived", "b")
	a := nested(t, pkg.Scope, "Base", "a")
	target := RelationshipsOf(b)[0].Target
	if got, ok := r.ResolveRedefinitionTarget(b.OwnerScope, b.Decl, target); !ok || got != a {
		t.Fatalf("ResolveRedefinitionTarget(b :>> a) = %v, %v, want Base::a", got, ok)
	}
	if _, ok := m.redefLoss[base]; !ok {
		t.Errorf("the settled loss of Base was not memoized")
	}
	if _, ok := m.redefLoss[derived]; ok {
		t.Errorf("the loss of Derived, cut short by its own redefinition, was memoized")
	}
	if !m.InheritanceMasked(derived, a) {
		t.Errorf("Derived must not inherit a")
	}
}

// A type removing nothing more holds its supertype's loss as that same set, and one removing
// more holds its own, the supertype's left as it was.
func TestInheritanceLossSharesASupertypesSetUntilTheTypeRemovesMore(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def A { part a; part x; }
		part def B :> A { part b :>> a; }
		part def C :> B;
		part def D :> B { part d :>> x; }
	}`)
	p := sym(t, root, "P")
	a := nested(t, p.Scope, "A", "a")
	x := nested(t, p.Scope, "A", "x")
	b, c, d := nested(t, p.Scope, "B"), nested(t, p.Scope, "C"), nested(t, p.Scope, "D")
	lossOfB, lossOfC, lossOfD := m.inheritanceLoss(b), m.inheritanceLoss(c), m.inheritanceLoss(d)
	if !lossOfB.full[a] || lossOfB.full[x] || len(lossOfB.full) != 1 {
		t.Fatalf("B loses %v, want a alone", lossOfB.full)
	}
	if !sameSet(lossOfC.unoffered, lossOfB.full) || !sameSet(lossOfC.inherited, lossOfB.full) || !sameSet(lossOfC.full, lossOfB.full) {
		t.Errorf("C removes nothing more, so its loss is B's set, not a copy of it")
	}
	if !lossOfD.full[a] || !lossOfD.full[x] || len(lossOfD.full) != 2 {
		t.Errorf("D loses %v, want a and x", lossOfD.full)
	}
	if lossOfD.inherited[x] || len(lossOfB.full) != 1 {
		t.Errorf("D's own redefinition reached a set it shares: inherited %v, B %v", lossOfD.inherited, lossOfB.full)
	}
}

// sameSet reports whether a and b are one map: a write to one shows in the other.
func sameSet(a, b map[*symbols.Symbol]bool) bool {
	return reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer()
}
