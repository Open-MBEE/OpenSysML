package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// buildModel parses src, indexes and resolves it, and returns the semantic
// model plus the document root scope for symbol lookups.
func buildModel(t *testing.T, src string) (*Model, *symbols.Scope) {
	return buildModelNamed(t, "t.sysml", src)
}

func buildModelNamed(t *testing.T, name, src string) (*Model, *symbols.Scope) {
	return buildModelNamedWithKind(t, name, source.KindOf(name), src)
}

func buildModelNamedWithKind(t *testing.T, name string, kind source.Kind, src string) (*Model, *symbols.Scope) {
	t.Helper()
	m, root, r, doc := buildUnresolvedModel(t, name, kind, src)
	r.ResolveDocument(name, doc)
	return m, root
}

// buildUnresolvedModel indexes src without resolving it, so a test controls
// which semantic query resolves each name first.
func buildUnresolvedModel(t *testing.T, name string, kind source.Kind, src string) (*Model, *symbols.Scope, *resolve.Resolver, *ast.RootNamespace) {
	t.Helper()
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := symbols.NewIndex()
	idx.AddDocumentWithKind(name, root, kind)
	r := resolve.New(idx)
	m := NewModel(r)
	// The workspace attaches the model before resolving so that feature chains
	// see inherited and contributed members; mirror that here.
	r.SetModel(m)
	return m, idx.DocumentRoot(name), r, root
}

// sym looks up the first symbol named key at the document root scope.
func sym(t *testing.T, scope *symbols.Scope, key string) *symbols.Symbol {
	t.Helper()
	s, ok := scope.LookupLocal(key)
	if !ok {
		t.Fatalf("symbol %q not found", key)
	}
	return s
}

func TestDirectSupertypes(t *testing.T) {
	m, root := buildModel(t, "part def A; part def B specializes A;")
	a := sym(t, root, "A")
	b := sym(t, root, "B")
	supers := m.DirectSupertypes(b)
	if len(supers) != 1 || supers[0] != a {
		t.Fatalf("DirectSupertypes(B) = %v, want [A]", supers)
	}
	if len(m.DirectSupertypes(a)) != 0 {
		t.Fatalf("DirectSupertypes(A) should be empty")
	}
}

// A subsetting names a sibling redefinition, whether it took the redefined
// feature's name or restates it; the inherited feature is shadowed (KerML 7.3.4.5).
func TestDirectSupertypesSubsetsSiblingRedefinition(t *testing.T) {
	for _, decl := range []string{"feature :>> x;", "feature x :>> x;", "private feature :>> x;", "protected feature :>> x;", "protected feature x :>> x;"} {
		m, root := buildModelNamed(t, "t.kerml", `package P {
		class A { feature x; }
		class B :> A {
			`+decl+`
			feature y :> x;
		}
	}`)
		p := sym(t, root, "P")
		b := sym(t, p.Scope, "B")
		a := sym(t, p.Scope, "A")
		redefining := sym(t, b.Scope, "x")
		inherited := sym(t, a.Scope, "x")
		if got := m.DirectSupertypes(sym(t, b.Scope, "y")); len(got) != 1 || got[0] != redefining {
			t.Fatalf("%s: DirectSupertypes(y) = %v, want the redefining B::x", decl, got)
		}
		if got := m.DirectSupertypes(redefining); len(got) != 1 || got[0] != inherited {
			t.Fatalf("%s: DirectSupertypes(B::x) = %v, want A::x", decl, got)
		}
	}
}

func TestDirectSupertypesResolvesAliasTargets(t *testing.T) {
	m, root := buildModel(t, `
		part def Base { part inherited; }
		alias BaseAlias for Base;
		alias BaseAlias2 for BaseAlias;
		part def Derived :> BaseAlias2;
		part usage : BaseAlias2;
	`)
	base := sym(t, root, "Base")
	derived := sym(t, root, "Derived")
	usage := sym(t, root, "usage")
	for _, tc := range []struct {
		name string
		sym  *symbols.Symbol
	}{
		{"Derived", derived},
		{"usage", usage},
	} {
		supers := m.DirectSupertypes(tc.sym)
		if len(supers) != 1 || supers[0] != base {
			t.Fatalf("DirectSupertypes(%s) = %v, want [Base]", tc.name, supers)
		}
	}
	if _, ok := m.LookupMember(derived, "inherited"); !ok {
		t.Fatal("alias supertype members should be visible on Derived")
	}
	if _, ok := m.LookupMember(usage, "inherited"); !ok {
		t.Fatal("alias typing target members should be visible on usage")
	}
}

func TestDirectSupertypesMemoizedAndDeduped(t *testing.T) {
	// A specializes B twice (odd but legal syntax); dedupe to one edge.
	m, root := buildModel(t, "part def B; part def A specializes B, B;")
	a := sym(t, root, "A")
	b := sym(t, root, "B")
	supers := m.DirectSupertypes(a)
	if len(supers) != 1 || supers[0] != b {
		t.Fatalf("DirectSupertypes(A) = %v, want [B] (deduped)", supers)
	}
	// Second call returns the memoized slice.
	if got := m.DirectSupertypes(a); len(got) != 1 || got[0] != b {
		t.Fatalf("memoized DirectSupertypes(A) = %v", got)
	}
}

func TestAllSupertypesTransitive(t *testing.T) {
	m, root := buildModel(t, "part def A; part def B specializes A; part def C specializes B;")
	a := sym(t, root, "A")
	b := sym(t, root, "B")
	c := sym(t, root, "C")
	all := m.AllSupertypes(c)
	if len(all) != 2 {
		t.Fatalf("AllSupertypes(C) = %v, want 2 entries", all)
	}
	got := map[*symbols.Symbol]bool{all[0]: true, all[1]: true}
	if !got[a] || !got[b] {
		t.Fatalf("AllSupertypes(C) missing A or B: %v", all)
	}
}

func TestInheritedFeatureNamedUsesDeclarationOrderForUnrelatedSupertypes(t *testing.T) {
	m, root := buildModel(t, `
		part def A { feature shared; }
		part def B { feature shared; }
		part def C specializes A, B { feature local; }
	`)
	a := sym(t, root, "A")
	c := sym(t, root, "C")
	local := sym(t, c.Scope, "local")
	got := m.inheritedFeatureNamed(local, "shared")
	if got == nil || got.OwnerScope == nil || got.OwnerScope.Owner() != a {
		t.Fatalf("inherited shared = %v, want A::shared from declaration order", got)
	}
}

func TestConforms(t *testing.T) {
	m, root := buildModel(t, "part def A; part def B specializes A; part def C specializes B; part def X;")
	a := sym(t, root, "A")
	c := sym(t, root, "C")
	x := sym(t, root, "X")
	if !m.Conforms(c, a) {
		t.Fatalf("C should conform to A")
	}
	if !m.Conforms(a, a) {
		t.Fatalf("A should conform to itself")
	}
	if m.Conforms(a, c) {
		t.Fatalf("A should not conform to C")
	}
	if m.Conforms(x, a) {
		t.Fatalf("unrelated X should not conform to A")
	}
}

func TestConformsAcrossUsageTyping(t *testing.T) {
	// A usage typed by a def conforms to that def.
	m, root := buildModel(t, "part def Engine; part e : Engine;")
	engine := sym(t, root, "Engine")
	e := sym(t, root, "e")
	if !m.Conforms(e, engine) {
		t.Fatalf("usage e should conform to its type Engine")
	}
}

func TestNoCycleOnAcyclicGraph(t *testing.T) {
	m, root := buildModel(t, "part def A; part def B specializes A;")
	if m.HasSpecializationCycle(sym(t, root, "A")) || m.HasSpecializationCycle(sym(t, root, "B")) {
		t.Fatalf("no cycle expected in acyclic graph")
	}
}

func TestDetectsTwoNodeCycle(t *testing.T) {
	m, root := buildModel(t, "part def A specializes B; part def B specializes A;")
	if !m.HasSpecializationCycle(sym(t, root, "A")) {
		t.Fatalf("expected cycle A<->B detected from A")
	}
	if !m.HasSpecializationCycle(sym(t, root, "B")) {
		t.Fatalf("expected cycle A<->B detected from B")
	}
}

func TestDetectsThreeNodeCycle(t *testing.T) {
	m, root := buildModel(t,
		"part def A specializes C; part def B specializes A; part def C specializes B;")
	for _, n := range []string{"A", "B", "C"} {
		if !m.HasSpecializationCycle(sym(t, root, n)) {
			t.Fatalf("expected 3-node cycle detected from %s", n)
		}
	}
}

func TestDetectsSelfSpecialization(t *testing.T) {
	m, root := buildModel(t, "part def A specializes A;")
	if !m.HasSpecializationCycle(sym(t, root, "A")) {
		t.Fatalf("expected self-specialization cycle")
	}
}

func TestAllSupertypesSafeOnCycle(t *testing.T) {
	// Must terminate (not infinite-loop) on a cyclic graph.
	m, root := buildModel(t, "part def A specializes B; part def B specializes A;")
	_ = m.AllSupertypes(sym(t, root, "A"))
	_ = m.Conforms(sym(t, root, "A"), sym(t, root, "B"))
}

// Every type specializes Base::Anything (KerML 8.3.2.1), whether or not the
// chain to it is declared: a part def has none, and its features are still
// redefinable against a feature typed by Anything.
func TestConformsToAnything(t *testing.T) {
	m, root := buildModel(t, `package Base { part def Anything; }
		part def Engine; part def X;`)
	base := sym(t, root, "Base")
	anything := sym(t, base.Scope, "Anything")
	engine := sym(t, root, "Engine")

	if !m.Conforms(engine, anything) {
		t.Fatalf("Engine should conform to Base::Anything")
	}
	if m.Conforms(anything, engine) {
		t.Fatalf("Base::Anything should not conform to Engine")
	}
	if m.Conforms(engine, sym(t, root, "X")) {
		t.Fatalf("unrelated X should still not be conformed to")
	}
}

// Resolving a nameless parameter's typing asks for that parameter's supertypes
// (it may be named by what it redefines), and the typing itself is still on the
// resolver's stack then. The answer computed there is provisional: memoizing it
// would leave the parameter untyped for every later query.
func TestDirectSupertypesNotMemoizedWhileOwnTypingResolves(t *testing.T) {
	m, root := buildModel(t, `package Q { attribute def Len; }
		calc def C { return : Q::Len; }`)
	c := sym(t, root, "C")
	var result *symbols.Symbol
	for _, member := range c.Scope.AnonymousMembers() {
		result = member
	}
	if result == nil {
		t.Fatal("the nameless return parameter was not indexed")
	}
	q := sym(t, root, "Q")
	if got := m.DirectSupertypes(result); len(got) != 1 || got[0] != sym(t, q.Scope, "Len") {
		t.Fatalf("DirectSupertypes(return) = %v, want [Q::Len]", got)
	}
}

// A chain target (`subsets x.f`) whose lookup the cycle guard cuts short is
// provisional like a qualified name's: memoized, it would drop `f` for good.
func TestDirectSupertypesNotMemoizedWhileChainTargetResolves(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		owner     string
	}{
		{"leading name on the stack", `package T {
			feature x { feature f; }
			feature a subsets a::b {
				feature b subsets x.f;
			}
		}`, "x"},
		{"member owner typing on the stack", `package T {
			classifier X { feature f; }
			feature a subsets a::b {
				feature y : X;
				feature b subsets y.f;
			}
		}`, "X"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, root := buildModelNamed(t, "t.kerml", tc.src)
			pkg := sym(t, root, "T")
			f := sym(t, sym(t, pkg.Scope, tc.owner).Scope, "f")
			a := sym(t, pkg.Scope, "a")
			b := sym(t, a.Scope, "b")
			for i := 0; i < 2; i++ {
				if got := m.DirectSupertypes(b); len(got) != 1 || got[0] != f {
					t.Fatalf("query %d: DirectSupertypes(b) = %v, want [f]", i+1, got)
				}
			}
			if got := m.DirectSupertypes(a); len(got) != 1 || got[0] != b {
				t.Fatalf("DirectSupertypes(a) = %v, want [b]", got)
			}
		})
	}
}

// A general resolved while its owner's supertypes are still being computed
// cannot see what the owner inherits, so it falls back to an outer name of the
// same spelling. That answer holds for the query that made it only: memoized,
// by the resolver or the model, the inherited general would be lost for good.
func TestDirectSupertypesNotMemoizedOnOuterFallback(t *testing.T) {
	const src = `package Base { classifier Anything; }
	package P {
		classifier X;
		classifier Lib { classifier X; }
		classifier A specializes Lib, D::Q {
			classifier C specializes X { classifier Q; }
		}
		classifier D specializes A::C {}
	}`
	for _, tc := range []struct {
		name  string
		build func(t *testing.T) (*Model, *symbols.Scope)
	}{
		{"owner queried first", func(t *testing.T) (*Model, *symbols.Scope) {
			m, root, _, _ := buildUnresolvedModel(t, "t.kerml", source.KindKerML, src)
			pkg := sym(t, root, "P")
			a := sym(t, pkg.Scope, "A")
			lib := sym(t, pkg.Scope, "Lib")
			q := sym(t, sym(t, a.Scope, "C").Scope, "Q")
			if got := m.DirectSupertypes(a); len(got) != 2 || got[0] != lib || got[1] != q {
				t.Fatalf("DirectSupertypes(A) = %v, want [Lib, A::C::Q]", got)
			}
			return m, root
		}},
		{"document resolved first", func(t *testing.T) (*Model, *symbols.Scope) {
			return buildModelNamed(t, "t.kerml", src)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, root := tc.build(t)
			pkg := sym(t, root, "P")
			c := sym(t, sym(t, pkg.Scope, "A").Scope, "C")
			inherited := sym(t, sym(t, pkg.Scope, "Lib").Scope, "X")
			for i := 0; i < 2; i++ {
				if got := m.DirectSupertypes(c); len(got) != 1 || got[0] != inherited {
					t.Fatalf("query %d: DirectSupertypes(C) = %v, want [Lib::X]", i+1, got)
				}
			}
			if _, cached := m.directSupers[c]; !cached {
				t.Fatalf("a settled answer should be memoized")
			}
		})
	}
}

// A member specializing its own owner is a cycle even though its general is
// resolved while the owner's is (Xpect CircleInheritance, CircleProblem5).
func TestDirectSupertypesKeptAcrossOwnerCycleGuard(t *testing.T) {
	t.Run("member specializes its owner", func(t *testing.T) {
		m, root := buildModelNamed(t, "t.kerml", `package Base { classifier Anything; }
		package Test1 {
			classifier <'A_Id'> A specializes A::B {
				classifier <'B_Id'> B specializes A, Base::Anything {}
			}
		}`)
		pkg := sym(t, root, "Test1")
		a := sym(t, pkg.Scope, "A")
		b := sym(t, a.Scope, "B")
		anything := sym(t, sym(t, root, "Base").Scope, "Anything")
		if got := m.DirectSupertypes(b); len(got) != 2 || got[0] != a || got[1] != anything {
			t.Fatalf("DirectSupertypes(B) = %v, want [A, Base::Anything]", got)
		}
		if got := m.DirectSupertypes(a); len(got) != 1 || got[0] != b {
			t.Fatalf("DirectSupertypes(A) = %v, want [B]", got)
		}
	})
	t.Run("member specializes its owner through a sibling", func(t *testing.T) {
		m, root := buildModelNamed(t, "t.kerml", `package Base { classifier Anything; }
		package Test1 {
			classifier A specializes D, Base::Anything {
				classifier B specializes C {}
			}
			classifier C specializes A {}
			classifier D specializes A::B {}
		}`)
		pkg := sym(t, root, "Test1")
		a := sym(t, pkg.Scope, "A")
		b := sym(t, a.Scope, "B")
		c := sym(t, pkg.Scope, "C")
		d := sym(t, pkg.Scope, "D")
		if got := m.DirectSupertypes(d); len(got) != 1 || got[0] != b {
			t.Fatalf("DirectSupertypes(D) = %v, want [A::B]", got)
		}
		if got := m.DirectSupertypes(b); len(got) != 1 || got[0] != c {
			t.Fatalf("DirectSupertypes(B) = %v, want [C]", got)
		}
		if got := m.DirectSupertypes(c); len(got) != 1 || got[0] != a {
			t.Fatalf("DirectSupertypes(C) = %v, want [A]", got)
		}
	})
}

// A closure built while a metadata annotation type is unresolved must not be
// memoized, or the base type it contributes never appears once it resolves.
func TestAllSupertypesNotMemoizedWhileMetadataProvisional(t *testing.T) {
	m, root := buildModel(t, "part def A; #Unknown part def B specializes A;")
	b := sym(t, root, "B")
	if got := m.AllSupertypes(b); len(got) != 1 || got[0] != sym(t, root, "A") {
		t.Fatalf("AllSupertypes(B) = %v, want [A]", got)
	}
	if !m.provisionalSupers[b] {
		t.Fatalf("B's supertypes should be provisional while #Unknown is unresolved")
	}
	if _, cached := m.allSupers[b]; cached {
		t.Fatalf("a provisional closure must not be memoized")
	}

	m2, root2 := buildModel(t, "metadata def Safety :> Base::Metaobject; part def A; #Safety part def B specializes A;")
	b2 := sym(t, root2, "B")
	m2.AllSupertypes(b2)
	if _, cached := m2.allSupers[b2]; !cached {
		t.Fatalf("a complete closure should be memoized")
	}
}
