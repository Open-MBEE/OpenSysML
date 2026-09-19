package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func memberNames(syms []*symbols.Symbol) map[string]bool {
	out := make(map[string]bool)
	for _, s := range syms {
		out[s.Name] = true
	}
	return out
}

func TestMembersOfLocalOnly(t *testing.T) {
	m, root := buildModel(t, "part def C { part a; part b; }")
	c := sym(t, root, "C")
	names := memberNames(m.MembersOf(c))
	if !names["a"] || !names["b"] {
		t.Fatalf("MembersOf(C) = %v, want a and b", names)
	}
}

func TestMembersOfInherited(t *testing.T) {
	m, root := buildModel(t,
		"part def Base { part a; } part def Sub specializes Base { part b; }")
	sub := sym(t, root, "Sub")
	names := memberNames(m.MembersOf(sub))
	if !names["a"] || !names["b"] {
		t.Fatalf("MembersOf(Sub) = %v, want inherited a and local b", names)
	}
}

func TestMembersOfMasking(t *testing.T) {
	// Sub redeclares `a`; the local one masks the inherited one (single `a`).
	m, root := buildModel(t,
		"part def Base { part a; } part def Sub specializes Base { part a; }")
	base := sym(t, root, "Base")
	sub := sym(t, root, "Sub")
	baseA, _ := base.Scope.LookupLocal("a")
	subA, _ := sub.Scope.LookupLocal("a")

	members := m.MembersOf(sub)
	count := 0
	var got *symbols.Symbol
	for _, s := range members {
		if s.Name == "a" {
			count++
			got = s
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly one visible `a`, got %d", count)
	}
	if got != subA {
		t.Fatalf("local `a` should mask inherited; got the base one")
	}
	if got == baseA {
		t.Fatalf("visible `a` should be the subtype's, not base's")
	}
}

func TestMembersOfTransitiveInheritance(t *testing.T) {
	m, root := buildModel(t,
		"part def A { part x; } part def B specializes A { part y; } part def C specializes B { part z; }")
	c := sym(t, root, "C")
	names := memberNames(m.MembersOf(c))
	for _, n := range []string{"x", "y", "z"} {
		if !names[n] {
			t.Fatalf("MembersOf(C) missing %q: %v", n, names)
		}
	}
}

func TestLookupMemberInherited(t *testing.T) {
	m, root := buildModel(t,
		"part def Base { part a; } part def Sub specializes Base;")
	sub := sym(t, root, "Sub")
	if _, ok := m.LookupMember(sub, "a"); !ok {
		t.Fatalf("LookupMember(Sub, a) should find inherited member")
	}
	if _, ok := m.LookupMember(sub, "nope"); ok {
		t.Fatalf("LookupMember(Sub, nope) should not resolve")
	}
}

func TestHasMemberAgreesWithMembersOf(t *testing.T) {
	// Sub masks Base's `a` by name and `r` by redefinition; `b` is inherited.
	m, root := buildModel(t,
		"part def Base { part a; part b; part r; } part def Sub specializes Base { part a; part x redefines r; }")
	base := sym(t, root, "Base")
	sub := sym(t, root, "Sub")
	var candidates []*symbols.Symbol
	for _, owner := range []*symbols.Symbol{base, sub} {
		candidates = append(candidates, m.MembersOf(owner)...)
	}
	// Answer before the members are memoized, then after.
	for pass := 0; pass < 2; pass++ {
		members := m.MembersOf(sub)
		for _, c := range candidates {
			if got, want := m.HasMember(sub, c), containsSymbol(members, c); got != want {
				t.Fatalf("pass %d: HasMember(Sub, %s) = %v, MembersOf says %v", pass, c.Name, got, want)
			}
		}
	}
	baseA, _ := base.Scope.LookupLocal("a")
	baseR, _ := base.Scope.LookupLocal("r")
	baseB, _ := base.Scope.LookupLocal("b")
	if m.HasMember(sub, baseA) || m.HasMember(sub, baseR) || !m.HasMember(sub, baseB) {
		t.Fatalf("HasMember(Sub): a=%v r=%v b=%v, want false false true",
			m.HasMember(sub, baseA), m.HasMember(sub, baseR), m.HasMember(sub, baseB))
	}
}

// A member named by a reference that resolves to nothing, or to no feature,
// binds no name: MembersOf omits it exactly where LookupMember does.
func TestMembersOfOmitsDerivedNamesTheirTargetsDoNotSupply(t *testing.T) {
	m, root := buildModel(t, `package P {
		constraint def Q;
		requirement def RD { constraint q : Q; }
		requirement r : RD { require zz; require Q; }
		requirement s : r;
	}`)

	pkg := sym(t, root, "P")
	q := sym(t, sym(t, pkg.Scope, "RD").Scope, "q")
	for _, owner := range []string{"r", "s"} {
		o := sym(t, pkg.Scope, owner)
		names := memberNames(m.MembersOf(o))
		for _, name := range []string{"zz", "Q"} {
			if names[name] {
				t.Errorf("MembersOf(%s) lists %q, whose target names no feature", owner, name)
			}
			if got, ok := m.LookupMember(o, name); ok {
				t.Errorf("LookupMember(%s, %q) = %v, want none", owner, name, got)
			}
		}
		if !names["q"] {
			t.Errorf("MembersOf(%s) lacks the inherited q", owner)
		}
		if got, ok := m.LookupMember(o, "q"); !ok || got != q {
			t.Errorf("LookupMember(%s, q) = %v, want RD::q", owner, got)
		}
	}
}

// Enumerating e1's members asks whether its `:>> length` binds that name, which
// resolves the redefinition's target through e1's own inherited members while
// that very resolution is under way. The provisional "no" must not stick.
func TestMembersOfKeepsARedefinitionWhoseTargetIsStillResolving(t *testing.T) {
	src := `package P {
		attribute def Q;
		item def Curve { attribute length : Q; item edges [*]; }
		item def Line :> Curve { attribute :>> length [1]; }
		item def Polygon :> Curve { item :>> edges : Line; }
		item def Quad :> Polygon {
			item :>> edges [2] = (e1, e2);
			item e1 [1];
			item e2 [1];
		}
		item def Rectangle :> Quad {
			attribute :>> length [1];
			item :>> e1 { attribute :>> length = Rectangle::length; }
			item :>> e2 { attribute :>> length = e1.length; }
		}
	}`
	for _, first := range []string{"e1", "Rectangle"} {
		t.Run("MembersOf "+first+" first", func(t *testing.T) {
			m, root, _, _ := buildUnresolvedModel(t, "t.sysml", source.KindSysML, src)
			pkg := sym(t, root, "P")
			rect := sym(t, pkg.Scope, "Rectangle")
			curveLength := sym(t, sym(t, pkg.Scope, "Curve").Scope, "length")
			if first == "e1" {
				m.MembersOf(sym(t, rect.Scope, "e1"))
			} else {
				m.MembersOf(rect)
			}
			for _, edge := range []string{"e1", "e2"} {
				e := sym(t, rect.Scope, edge)
				l, ok := m.LookupMember(e, "length")
				if !ok || l.OwnerScope != e.Scope {
					t.Errorf("LookupMember(%s, length) = %v, %v, want the redefinition", edge, l, ok)
					continue
				}
				if !memberNames(m.MembersOf(e))["length"] {
					t.Errorf("MembersOf(%s) lacks length", edge)
				}
				found := false
				for _, r := range m.AllRedefinedFeatures(l) {
					found = found || r == curveLength
				}
				if !found {
					t.Errorf("%s.length does not redefine Curve::length", edge)
				}
			}
		})
	}
}
