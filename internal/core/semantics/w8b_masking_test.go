package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// visibleNames counts each name a type offers, so a masked inherited feature is
// distinguishable from one that is merely shadowed by a namesake.
func visibleNames(m *Model, sym *symbols.Symbol) map[string]int {
	out := map[string]int{}
	for _, s := range m.MembersOf(sym) {
		out[s.Name]++
		if s.ShortName != "" {
			out[s.ShortName]++
		}
	}
	return out
}

func TestRedefinitionMasksTheRedefinedName(t *testing.T) {
	m, root := buildModel(t,
		"part def A { part a; } part def B specializes A { part b redefines a; }")
	names := visibleNames(m, sym(t, root, "B"))
	if names["a"] != 0 {
		t.Fatalf("redefined `a` still visible in B: %v", names)
	}
	if names["b"] != 1 {
		t.Fatalf("redefining `b` should be visible once: %v", names)
	}
}

func TestRedefinitionMasksTheShortNameToo(t *testing.T) {
	m, root := buildModel(t,
		"part def A { part <a_id> a; } part def B specializes A { part b redefines a_id; }")
	names := visibleNames(m, sym(t, root, "B"))
	if names["a"] != 0 || names["a_id"] != 0 {
		t.Fatalf("both names of the redefined feature must be masked: %v", names)
	}
}

func TestRedefinitionByShortNameMasksTheRegularName(t *testing.T) {
	m, root := buildModel(t,
		"part def A { part <a_id> a; } part def B specializes A { part <b_id> b redefines a; }")
	names := visibleNames(m, sym(t, root, "B"))
	if names["a"] != 0 || names["a_id"] != 0 {
		t.Fatalf("naming the redefined feature by either name masks both: %v", names)
	}
	if names["b"] != 1 || names["b_id"] != 1 {
		t.Fatalf("the redefining feature keeps both of its names: %v", names)
	}
}

func TestRedefinitionMasksTransitivelyRedefinedFeatures(t *testing.T) {
	m, root := buildModel(t,
		"part def A { part a; } part def B specializes A { part b redefines a; }"+
			" part def C specializes B { part c redefines b; }")
	names := visibleNames(m, sym(t, root, "C"))
	if names["a"] != 0 || names["b"] != 0 {
		t.Fatalf("a redefinition chain masks every link: %v", names)
	}
	if names["c"] != 1 {
		t.Fatalf("redefining `c` should be visible once: %v", names)
	}
}

func TestRedefinitionClosureMasksThroughNamesakeIntermediate(t *testing.T) {
	m, root := buildModel(t,
		"part def A { part a; } part def B specializes A { part :>> a; }"+
			" part def C specializes B { part c redefines B::a; } part def D specializes B;")
	a := sym(t, root, "A").Scope.LookupLocalAll("a")[0]
	ba := sym(t, root, "B").Scope.LookupLocalAll("a")[0]
	c := sym(t, root, "C")
	if !m.InheritanceMasked(c, ba) {
		t.Fatalf("C's redefinition must mask B::a")
	}
	if !m.InheritanceMasked(c, a) {
		t.Fatalf("B::a's redefinition of A::a masks it in C too")
	}
	d := sym(t, root, "D")
	if m.InheritanceMasked(d, ba) || !m.InheritanceMasked(d, a) {
		t.Fatalf("D inherits B::a in place of A::a")
	}
}

// Siblings redefining each other close cyclically; the closure terminates and
// masks nothing, since a type never inherits its own members — pilot-refereed.
func TestCyclicRedefinitionMaskTerminates(t *testing.T) {
	m, root := buildModel(t,
		"part def A { part a redefines b; part b redefines a; }"+
			" part def B specializes A {}")
	if names := visibleNames(m, sym(t, root, "B")); names["a"] != 1 || names["b"] != 1 {
		t.Fatalf("B inherits both of A's features whatever they redefine: %v", names)
	}
}

// A chain that crosses a sibling edge masks only its inherited links — on the
// cached closure path and on the exact fallback alike. Pilot-refereed.
func TestTransitiveMaskStopsAtSiblingEdge(t *testing.T) {
	m, root := buildModel(t,
		"part def A { part a; part b redefines a; }"+
			" part def B specializes A { part c redefines b; }")
	b := sym(t, root, "B")
	a := sym(t, root, "A")
	aa, _ := a.Scope.LookupLocal("a")
	ab, _ := a.Scope.LookupLocal("b")
	c, _ := b.Scope.LookupLocal("c")
	if !m.InheritanceMasked(b, ab) || m.InheritanceMasked(b, aa) {
		t.Fatalf("B masks b and keeps a: %v", visibleNames(m, b))
	}
	exact := m.buildMask(b, []*symbols.Symbol{c})
	if !exact[ab] || exact[aa] {
		t.Fatalf("exact expansion = %v, want {b}", exact)
	}
}

func TestRedefinitionMaskFallsBackWhenClosureReachesOwner(t *testing.T) {
	m := NewModel(nil)
	owner := &symbols.Symbol{Name: "Owner", Scope: symbols.NewScope(nil, nil)}
	candidate := &symbols.Symbol{Name: "candidate"}
	m.redefined[candidate] = []*symbols.Symbol{owner}
	mask := m.buildMaskFromCandidates(owner, func(yield func(*symbols.Symbol) bool) {
		yield(candidate)
	})
	if len(mask) != 0 {
		t.Fatalf("owner-dependent fallback produced mask %v, want empty", mask)
	}
}

func TestRedefinitionClosureIsCached(t *testing.T) {
	m, root := buildModel(t,
		"part def A { part a; } part def B specializes A { part b redefines a; }")
	b := sym(t, root, "B")
	candidate, ok := b.Scope.LookupLocal("b")
	if !ok {
		t.Fatal("B declares b")
	}
	first, cyclic := m.redefinitionClosure(candidate)
	if cyclic {
		t.Fatal("simple redefinition unexpectedly cyclic")
	}
	if _, ok := m.redefClosure[candidate]; !ok {
		t.Fatal("redefinition closure was not cached")
	}
	second, cyclic := m.redefinitionClosure(candidate)
	if cyclic || len(second) != len(first) {
		t.Fatalf("cached redefinition closure = %v, %v; want %v, false", second, cyclic, first)
	}
}

func TestRedefinitionClosureNotCachedDuringRedefinedFeatures(t *testing.T) {
	m, root := buildModel(t,
		"part def A { part a; } part def B specializes A { part b redefines a; }")
	b := sym(t, root, "B")
	a := sym(t, root, "A").Scope.LookupLocalAll("a")[0]
	candidate, ok := b.Scope.LookupLocal("b")
	if !ok {
		t.Fatal("B declares b")
	}

	// Reproduce the state RedefinedFeatures establishes before resolving its
	// target: its memo is guarded while the computation is still active.
	m.redefined[candidate] = nil
	m.computingRedefinedFeatures++
	partial, cyclic := m.redefinitionClosure(candidate)
	m.computingRedefinedFeatures--
	if cyclic {
		t.Fatal("simple redefinition unexpectedly cyclic")
	}
	if len(partial) != 0 {
		t.Fatalf("incomplete closure = %v, want empty guarded result", partial)
	}
	if _, ok := m.redefClosure[candidate]; ok {
		t.Fatal("closure derived during RedefinedFeatures must not be cached")
	}

	delete(m.redefined, candidate)
	if got := m.RedefinedFeatures(candidate); len(got) != 1 || got[0] != a {
		t.Fatalf("complete redefinitions = %v, want [%v]", got, a)
	}
	if !m.InheritanceMasked(b, a) {
		t.Fatal("later mask query must use the complete redefinition closure")
	}
	if _, ok := m.redefClosure[candidate]; !ok {
		t.Fatal("complete closure was not cached after RedefinedFeatures finished")
	}
}

func TestBuildMaskSkipsNilCandidates(t *testing.T) {
	m := NewModel(nil)
	owner := &symbols.Symbol{Name: "Owner", Scope: symbols.NewScope(nil, nil)}
	mask := m.buildMaskFromCandidates(owner, func(yield func(*symbols.Symbol) bool) {
		yield(nil)
	})
	if mask != nil {
		t.Fatalf("nil candidates should produce no mask, got %v", mask)
	}
	if _, ok := m.redefClosure[nil]; ok {
		t.Fatal("nil candidate must not create a closure-cache entry")
	}
}

func TestRedefinitionMasksOnlyTheFeatureItNames(t *testing.T) {
	// Two inherited features carry the name `a`; only one is redefined, so the
	// other keeps its name and `a` stays visible (masking is by element).
	m, root := buildModel(t,
		"part def A1 { part a; } part def A2 { part a; }"+
			" part def B specializes A1, A2 { part b redefines A1::a; }")
	b := sym(t, root, "B")
	a1 := sym(t, root, "A1")
	a2 := sym(t, root, "A2")
	a1a, _ := a1.Scope.LookupLocal("a")
	a2a, _ := a2.Scope.LookupLocal("a")
	if !m.InheritanceMasked(b, a1a) {
		t.Fatalf("A1::a is redefined and must be masked in B")
	}
	if m.InheritanceMasked(b, a2a) {
		t.Fatalf("A2::a is not redefined and must stay visible in B")
	}
	if visibleNames(m, b)["a"] != 1 {
		t.Fatalf("the surviving namesake keeps the name `a`: %v", visibleNames(m, b))
	}
}

func TestSubsettingDoesNotMask(t *testing.T) {
	m, root := buildModel(t,
		"part def A { part a; } part def B specializes A { part b subsets a; }")
	names := visibleNames(m, sym(t, root, "B"))
	if names["a"] != 1 {
		t.Fatalf("subsetting is not redefinition and masks nothing: %v", names)
	}
}

func TestRedefinitionKeepingTheNameMasksNothing(t *testing.T) {
	// `part :>> a` is visible as `a` itself, so the name survives.
	m, root := buildModel(t,
		"part def A { part a; } part def B specializes A { part :>> a; }")
	names := visibleNames(m, sym(t, root, "B"))
	if names["a"] != 1 {
		t.Fatalf("a redefinition taking the redefined name keeps it visible once: %v", names)
	}
}

func TestMaskedFeatureStaysInTheUnmaskedView(t *testing.T) {
	// Execution binds the redefining and redefined features to one value, so
	// the runtime view keeps both.
	m, root := buildModel(t,
		"part def A { part a; } part def B specializes A { part b redefines a; }")
	names := map[string]bool{}
	for _, s := range m.MembersOfIncludingRedefined(sym(t, root, "B")) {
		names[s.Name] = true
	}
	if !names["a"] || !names["b"] {
		t.Fatalf("unmasked view must keep both features: %v", names)
	}
}

func TestDeclaringViewOmitsTheDeclarationAndSuspendsItsMask(t *testing.T) {
	// Resolving `b redefines a` sees `a` and not `b`: the declaration being
	// written is not yet a member and masks nothing (KerML 8.3.3.3.6). B's
	// other declarations, and the masks they cause, stay.
	m, root := buildModel(t,
		"part def A { part a; part c; } "+
			"part def B specializes A { part b redefines a; part d redefines c; }")
	b := sym(t, root, "B")
	declaring, ok := b.Scope.LookupLocal("b")
	if !ok {
		t.Fatal("B declares b")
	}
	names := map[string]bool{}
	for _, s := range m.MembersOfDeclaring(b, declaring) {
		names[s.Name] = true
	}
	if !names["a"] {
		t.Fatalf("the redefinition target must be visible to its own declaration: %v", names)
	}
	if names["b"] {
		t.Fatalf("the declaration being written is not yet a member: %v", names)
	}
	if !names["d"] || names["c"] {
		t.Fatalf("another declaration and its mask are unaffected: %v", names)
	}
}

func TestRedefinitionMasksAnAliasOfTheRedefinedFeature(t *testing.T) {
	// An alias binds a second name to the same element, and masking is by
	// element: naming either name removes both from the inheriting type.
	m, root := buildModel(t,
		"part def A { part a; alias aa for a; } part def B specializes A { part b redefines aa; }")
	names := visibleNames(m, sym(t, root, "B"))
	if names["a"] != 0 || names["aa"] != 0 {
		t.Fatalf("the redefined element and its alias must both be masked: %v", names)
	}
}

func TestRedefinitionMasksTheNameOnlyAtItsOwnLevel(t *testing.T) {
	// Masking is a namespace question: `a` is gone from B, and the nested names
	// under the redefining feature are its own, reached through it.
	m, root := buildModel(t,
		"part def A { part a { part x; } }"+
			" part def B specializes A { part b redefines a { part y; } }")
	b := sym(t, root, "B")
	if visibleNames(m, b)["a"] != 0 {
		t.Fatalf("`a` must not be a member name of B: %v", visibleNames(m, b))
	}
	bb, ok := b.Scope.LookupLocal("b")
	if !ok {
		t.Fatalf("B declares b")
	}
	nested := visibleNames(m, bb)
	if nested["y"] != 1 || nested["x"] != 1 {
		t.Fatalf("b offers its own y and the inherited x of the feature it redefines: %v", nested)
	}
}

// Masking is keyed by element, not by visibility: the member view drops what a
// redefinition masks whatever the inherited membership's visibility is, and the
// visibility filter is the caller's (KerML 8.2.3.5 composes with 7.4.7). A
// private member is not inherited at all, so a redefinition cannot name it.
func TestMaskingIsIndependentOfTheRedefinedMembershipVisibility(t *testing.T) {
	for _, vis := range []string{"", "protected "} {
		m, root := buildModel(t,
			"part def A { "+vis+"part a; } part def B specializes A { part b redefines a; }")
		if names := visibleNames(m, sym(t, root, "B")); names["a"] != 0 {
			t.Fatalf("%q member still visible in B: %v", vis, names)
		}
	}
	m, root := buildModel(t,
		"part def A { private part a; } part def B specializes A { part b redefines a; }")
	b := nested(t, sym(t, root, "B").Scope, "b")
	if got := m.RedefinedFeatures(b); len(got) != 0 {
		t.Fatalf("private `a` is not inherited, yet b redefines %v", got)
	}
}

// A redefinition of a namesake targets that namesake, not the redefining feature
// itself, whichever specialization clause is resolved first (KerML 8.3.3.3.6).
func TestSelfNamedRedefinitionTargetIsIndependentOfClauseOrder(t *testing.T) {
	for _, clauses := range []string{":>> causes :> participant", ":> participant :>> causes"} {
		m, root := buildModel(t, `package P {
			abstract occurrence causes[*];
			occurrence def Link { ref occurrence participant[*]; }
			abstract occurrence def Multicausation :> Link {
				abstract constant ref occurrence causes[1..*] `+clauses+`;
			}
		}`)
		p := sym(t, root, "P")
		outer := nested(t, p.Scope, "causes")
		inner := nested(t, p.Scope, "Multicausation", "causes")
		if got := m.RedefinedFeatures(inner); len(got) != 1 || got[0] != outer {
			t.Fatalf("%q: RedefinedFeatures(Multicausation::causes) = %v, want [P::causes]", clauses, got)
		}
	}
}
