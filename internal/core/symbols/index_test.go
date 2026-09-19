package symbols

import (
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

func addDoc(t *testing.T, idx *Index, name, src string) {
	t.Helper()
	sf := source.New(name, []byte(src))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected parse diagnostics for %s: %v", name, p.Diagnostics)
	}
	idx.AddDocument(name, root)
}

func TestIndexQualifiedLookup(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package P { package Q { namespace N; } }")

	syms := idx.LookupQualified("P::Q::N")
	if len(syms) != 1 {
		t.Fatalf("LookupQualified(P::Q::N) len = %d, want 1", len(syms))
	}
	if syms[0].Kind != SymbolNamespace {
		t.Fatalf("P::Q::N kind = %v, want namespace", syms[0].Kind)
	}
	if len(idx.LookupQualified("P::Missing")) != 0 {
		t.Fatalf("LookupQualified(P::Missing) should be empty")
	}
}

func TestIndexDocumentKindUsesExplicitKindAndNameFallback(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "fallback.kerml", "class C;")
	if got := idx.DocumentKind("fallback.kerml"); got != source.KindKerML {
		t.Fatalf("DocumentKind(fallback.kerml) = %s, want KerML", got)
	}

	sf := source.NewWithKind("<content>", []byte("class C;"), source.KindKerML)
	root := parser.New(sf).ParseFile()
	idx.AddDocumentWithKind("<content>", root, source.KindKerML)
	if got := idx.DocumentKind("<content>"); got != source.KindKerML {
		t.Fatalf("DocumentKind(<content>) = %s, want KerML", got)
	}
	idx.RemoveDocument("<content>")
	if got := idx.DocumentKind("<content>"); got != source.KindUnknown {
		t.Fatalf("DocumentKind(<content>) after removal = %s, want unknown", got)
	}
}

func TestIndexAmbiguousQualified(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package P { namespace D; }")
	addDoc(t, idx, "b.sysml", "package P { namespace D; }")

	if got := len(idx.LookupQualified("P::D")); got != 2 {
		t.Fatalf("LookupQualified(P::D) len = %d, want 2 (ambiguous)", got)
	}
}

// A member a namespace declares shadows one of the same name that a wildcard
// import re-exports through it, so the qualified name stays unambiguous — the
// pattern of SI::min, which is SI's own minute and not an imported function.
func TestIndexOwnedMemberShadowsWildcardReexport(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package Functions { calc def min; calc def clamp; }")
	addDoc(t, idx, "b.sysml", "package Units { public import Functions::*; attribute <min> minute; }")
	idx.ExpandWildcardImports()

	syms := idx.LookupQualified("Units::min")
	if len(syms) != 1 {
		t.Fatalf("LookupQualified(Units::min) len = %d, want 1", len(syms))
	}
	if syms[0].Name != "minute" {
		t.Errorf("Units::min = %q, want the declared minute", syms[0].Name)
	}
	if got := len(idx.LookupQualified("Units::clamp")); got != 1 {
		t.Errorf("LookupQualified(Units::clamp) len = %d, want 1: "+
			"a re-export stays visible when nothing shadows it", got)
	}
}

// Wildcard imports chain — KerML imports Kernel::*, Kernel imports Core::*,
// Core imports Root::* — and each link names a sibling of the importing
// package, so expansion has to follow the chain to its end and look the target
// up in the enclosing namespaces, not only in the importer and the root. The
// answer must not depend on the order the importers happen to be visited in.
func TestExpandWildcardImportsChainsAndIsOrderIndependent(t *testing.T) {
	const src = `package L {
		public import Mid::*;
		package Base { part def Element; attribute def <kg> Kilogram; }
		package Mid { public import Base::*; }
	}`
	want := []string{"L::Base::Element", "L::Mid::Element", "L::Element", "L::kg", "L::Mid::kg"}

	// Repeat: the pass walks a map, so a single run could pass by luck.
	for i := 0; i < 8; i++ {
		idx := NewIndex()
		addDoc(t, idx, "l.sysml", src)
		idx.ExpandWildcardImports()
		for _, fqn := range want {
			if got := len(idx.LookupQualified(fqn)); got != 1 {
				t.Fatalf("run %d: LookupQualified(%s) len = %d, want 1", i, fqn, got)
			}
		}
	}
}

// A relative wildcard target names the innermost enclosing declaration of that
// name, not a top-level package that happens to share it.
func TestExpandWildcardImportsPrefersTheEnclosingTarget(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "outer.sysml", "package Systems { part def Outer; }")
	addDoc(t, idx, "sysml.sysml", "package SysML { public import Systems::*; package Systems { part def Inner; } }")
	idx.ExpandWildcardImports()

	if got := len(idx.LookupQualified("SysML::Inner")); got != 1 {
		t.Errorf("LookupQualified(SysML::Inner) len = %d, want 1", got)
	}
	if got := len(idx.LookupQualified("SysML::Outer")); got != 0 {
		t.Errorf("LookupQualified(SysML::Outer) len = %d, want 0: "+
			"SysML::Systems shadows the top-level Systems", got)
	}
}

// A wildcard target names the package an earlier import brought into the
// importing namespace before a top-level one of that name, since KerML 8.2.3.5
// resolves a name against a namespace's imported memberships. Here P imports
// Outer::* (re-exporting Outer::Shared as P::Shared) and then Shared::*, which
// names that P::Shared; its members are read from where it was declared, since
// a re-export registers the symbol alone and copies none of its subtree.
func TestExpandWildcardImportsFollowsAReexportedTarget(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "shared.sysml", "package Shared { part def Widget; }")
	addDoc(t, idx, "outer.sysml", "package Outer { package Shared { part def Imported; } }")
	addDoc(t, idx, "p.sysml", "package P { public import Outer::*; public import Shared::*; }")
	idx.ExpandWildcardImports()

	if got := len(idx.LookupQualified("P::Imported")); got != 1 {
		t.Errorf("LookupQualified(P::Imported) len = %d, want 1: "+
			"`import Shared::*` names the P::Shared that `import Outer::*` "+
			"brought in, and its members live under Outer::Shared", got)
	}
	if got := len(idx.LookupQualified("P::Widget")); got != 0 {
		t.Errorf("LookupQualified(P::Widget) len = %d, want 0: "+
			"the imported Shared shadows the top-level one", got)
	}
}

// A wildcard import can name its target by the package's short name, whose
// index entry holds none of the members: they live under the declared FQN.
func TestExpandWildcardImportsResolvesAShortNameTarget(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "lib.sysml", "package <USCU> USCustomaryUnits { part def Inch; }")
	addDoc(t, idx, "p.sysml", "package P { public import USCU::*; }")
	idx.ExpandWildcardImports()

	if got := len(idx.LookupQualified("P::Inch")); got != 1 {
		t.Errorf("LookupQualified(P::Inch) len = %d, want 1: "+
			"`import USCU::*` names USCustomaryUnits, whose members live under its "+
			"declared name", got)
	}
}

// A cycle of wildcard imports brings a package its own members back; they stay
// owned, so a declaration still shadows a name another import brought in.
func TestExpandWildcardImportsKeepsAnOwnedNameOwnedAcrossACycle(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml",
		"package A { part def Widget; public import B::*; public import C::*; }")
	addDoc(t, idx, "b.sysml", "package B { public import A::*; }")
	addDoc(t, idx, "c.sysml", "package C { part def Widget; }")
	idx.ExpandWildcardImports()

	got := idx.LookupQualified("A::Widget")
	if len(got) != 1 || idx.declaredAt.at(got[0]) != "A::Widget" {
		t.Errorf("LookupQualified(A::Widget) = %d symbol(s), want A's own: "+
			"`import B::*` re-exporting it back does not make it borrowed", len(got))
	}
}

// A name is exported when any import that surfaced it was public, so importing
// a namespace publicly still passes it on after a private import of the same
// name reached it first (KerML 8.2.3.3).
func TestExpandWildcardImportsExportsAPubliclyReimportedName(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "base.sysml", "package Base { part def Shared; }")
	addDoc(t, idx, "mid.sysml", "package Mid { public import Base::*; }")
	addDoc(t, idx, "top.sysml",
		"package Top { private import Base::*; public import Mid::*; }")
	addDoc(t, idx, "user.sysml", "package User { public import Top::*; }")
	idx.ExpandWildcardImports()

	if got := len(idx.LookupQualified("User::Shared")); got != 1 {
		t.Errorf("LookupQualified(User::Shared) len = %d, want 1: "+
			"Top imports Shared publicly through Mid as well", got)
	}
}

// Following a re-exported target only works while the name means one thing: two
// imports bringing in different packages of that name leave it ambiguous, and an
// ambiguous target imports nothing rather than picking one of them.
func TestExpandWildcardImportsIgnoresAnAmbiguousTarget(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package A { package Shared { part def FromA; } }")
	addDoc(t, idx, "b.sysml", "package B { package Shared { part def FromB; } }")
	addDoc(t, idx, "p.sysml",
		"package P { public import A::*; public import B::*; public import Shared::*; }")
	idx.ExpandWildcardImports()

	if got := len(idx.fqn.at("P::Shared")); got != 2 {
		t.Fatalf("P::Shared names %d symbols, want the 2 re-exports this case needs", got)
	}
	for _, fqn := range []string{"P::FromA", "P::FromB"} {
		if got := len(idx.LookupQualified(fqn)); got != 0 {
			t.Errorf("LookupQualified(%s) len = %d, want 0: "+
				"`import Shared::*` cannot choose between A::Shared and B::Shared", fqn, got)
		}
	}
}

// A declared package stays a usable wildcard target when an import re-exported
// something of the same name alongside it — the declaration shadows the
// re-export, as it does for any lookup.
func TestExpandWildcardImportsTargetSurvivesAReexportOfItsName(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "base.sysml", "package Base { package Util { part def FromBase; } }")
	addDoc(t, idx, "lib.sysml", "package Lib { public import Base::*; "+
		"package Util { part def A; } package Sub { public import Util::*; } }")
	idx.ExpandWildcardImports()

	if got := len(idx.LookupQualified("Lib::Sub::A")); got != 1 {
		t.Errorf("LookupQualified(Lib::Sub::A) len = %d, want 1: "+
			"`import Util::*` names Lib's own Util", got)
	}
	if got := len(idx.LookupQualified("Lib::Sub::FromBase")); got != 0 {
		t.Errorf("LookupQualified(Lib::Sub::FromBase) len = %d, want 0: "+
			"Lib::Util shadows the re-exported Base::Util", got)
	}
}

// A private import is visible only inside the namespace that declares it, so a
// package importing that namespace must not see what it privately imported, and
// neither must a qualified reference through that namespace's own name
// (KerML 8.2.3.3).
func TestExpandWildcardImportsDoesNotCarryOnAPrivateImport(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "base.sysml", "package Base { part def Hidden; part def Shown; }")
	addDoc(t, idx, "mid.sysml", "package Mid { private import Base::*; part def Own; }")
	addDoc(t, idx, "top.sysml", "package Top { public import Mid::*; }")
	idx.ExpandWildcardImports()

	if got := len(idx.LookupQualified("Mid::Hidden")); got != 0 {
		t.Errorf("LookupQualified(Mid::Hidden) len = %d, want 0: "+
			"Mid does not re-export what it imported privately", got)
	}
	if got := len(idx.LookupQualifiedFrom("Mid::Hidden", "Mid")); got != 1 {
		t.Errorf("LookupQualifiedFrom(Mid::Hidden, Mid) len = %d, want 1: "+
			"a private import is visible inside Mid", got)
	}
	if got := len(idx.LookupQualified("Top::Own")); got != 1 {
		t.Errorf("LookupQualified(Top::Own) len = %d, want 1: Mid::Own is public", got)
	}
	if got := len(idx.LookupQualified("Top::Hidden")); got != 0 {
		t.Errorf("LookupQualified(Top::Hidden) len = %d, want 0: "+
			"Mid imported Base privately, so Top does not see Base's members", got)
	}
}

// A privately imported name is visible from inside the importing namespace and
// from anything nested in it, and from nowhere else — not from a sibling, and
// not from a namespace whose name merely starts with the same text.
func TestLookupQualifiedFromSeesAPrivateImportOnlyFromWithin(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "base.sysml", "package Base { part def Hidden; }")
	addDoc(t, idx, "mid.sysml",
		"package Mid { private import Base::*; package Inner { part def I; } }")
	addDoc(t, idx, "midden.sysml", "package Midden { part def M; }")
	idx.ExpandWildcardImports()

	for _, from := range []string{"Mid", "Mid::Inner"} {
		if got := len(idx.LookupQualifiedFrom("Mid::Hidden", from)); got != 1 {
			t.Errorf("LookupQualifiedFrom(Mid::Hidden, %s) len = %d, want 1: "+
				"a private import is visible throughout the namespace declaring it",
				from, got)
		}
	}
	for _, from := range []string{"", "Midden", "Other::Mid"} {
		if got := len(idx.LookupQualifiedFrom("Mid::Hidden", from)); got != 0 {
			t.Errorf("LookupQualifiedFrom(Mid::Hidden, %q) len = %d, want 0: "+
				"Mid's private import is not visible there", from, got)
		}
	}
}

// HiddenFrom is what a caller that has another route to a name — an
// inheritance-aware member search over the index's direct children — asks
// before taking it, so it answers only for a name nothing but a private import
// surfaced, seen from outside the importing namespace.
func TestHiddenFromReportsOnlyPrivatelySurfacedNames(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "base.sysml", "package Base { part def Hidden; part def Shown; }")
	addDoc(t, idx, "mid.sysml",
		"package Mid { private import Base::*; part def Own; package Inner { part def I; } }")
	addDoc(t, idx, "pub.sysml", "package Pub { public import Base::*; }")
	idx.ExpandWildcardImports()

	if !idx.HiddenFrom("Mid::Hidden", "") {
		t.Errorf("HiddenFrom(Mid::Hidden, \"\") = false, want true")
	}
	for _, from := range []string{"Mid", "Mid::Inner"} {
		if idx.HiddenFrom("Mid::Hidden", from) {
			t.Errorf("HiddenFrom(Mid::Hidden, %s) = true, want false: the namespace "+
				"declaring the private import sees it", from)
		}
	}
	for _, fqn := range []string{"Mid::Own", "Pub::Shown", "Base::Hidden", "Mid::Missing"} {
		if idx.HiddenFrom(fqn, "") {
			t.Errorf("HiddenFrom(%s, \"\") = true, want false", fqn)
		}
	}
}

// A public wildcard import is unaffected: the name it surfaces stays reachable
// by a qualified reference through the importing namespace, from anywhere.
func TestLookupQualifiedReachesAPubliclyImportedName(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "base.sysml", "package Base { part def Shown; }")
	addDoc(t, idx, "mid.sysml", "package Mid { public import Base::*; }")
	idx.ExpandWildcardImports()

	if got := len(idx.LookupQualified("Mid::Shown")); got != 1 {
		t.Errorf("LookupQualified(Mid::Shown) len = %d, want 1: "+
			"a public import re-exports the name it surfaces", got)
	}
}

// Chained: A privately imports B::*, and C imports A::*. C sees neither B's
// members through A nor, since A never re-exported them, through its own name.
func TestLookupQualifiedAcrossAChainedPrivateImport(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "b.sysml", "package B { part def Deep; }")
	addDoc(t, idx, "a.sysml", "package A { private import B::*; part def Own; }")
	addDoc(t, idx, "c.sysml", "package C { public import A::*; }")
	idx.ExpandWildcardImports()

	if got := len(idx.LookupQualified("C::Own")); got != 1 {
		t.Errorf("LookupQualified(C::Own) len = %d, want 1: A::Own is public", got)
	}
	for _, fqn := range []string{"C::Deep", "A::Deep"} {
		if got := len(idx.LookupQualified(fqn)); got != 0 {
			t.Errorf("LookupQualified(%s) len = %d, want 0: "+
				"B::Deep reaches A privately and stops there", fqn, got)
		}
	}
	if got := len(idx.LookupQualifiedFrom("A::Deep", "A")); got != 1 {
		t.Errorf("LookupQualifiedFrom(A::Deep, A) len = %d, want 1: "+
			"A's own private import is visible inside A", got)
	}
	if got := len(idx.LookupQualifiedFrom("C::Deep", "C")); got != 0 {
		t.Errorf("LookupQualifiedFrom(C::Deep, C) len = %d, want 0: "+
			"C never received B's members at all", got)
	}
}

func TestIndexDocumentRoot(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package P;")
	rs := idx.DocumentRoot("a.sysml")
	if rs == nil {
		t.Fatalf("DocumentRoot(a.sysml) = nil")
	}
	if _, ok := rs.LookupLocal("P"); !ok {
		t.Fatalf("document root missing P")
	}
	if idx.DocumentRoot("missing.sysml") != nil {
		t.Fatalf("DocumentRoot(missing) should be nil")
	}
}

func TestIndexShortNameNotDuplicatedInFQN(t *testing.T) {
	// A package with both short and primary names registers one symbol; the
	// FQN uses the primary name. Both local keys still resolve via the scope.
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package <p> Primary { namespace N; }")
	if len(idx.LookupQualified("Primary::N")) != 1 {
		t.Fatalf("Primary::N not indexed")
	}
}

func TestIndexScopeRegistrationOrderIsDeterministic(t *testing.T) {
	sf := source.New("duplicate.sysml", []byte("package P { namespace A; namespace B; namespace A; }"))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected parse diagnostics: %v", p.Diagnostics)
	}
	scope := Build(root)

	first := NewIndex()
	first.indexScope("duplicate.sysml", scope, "")
	second := NewIndex()
	second.indexScope("duplicate.sysml", scope, "")

	for _, fqn := range []string{"P", "P::A", "P::B"} {
		got, want := first.LookupQualified(fqn), second.LookupQualified(fqn)
		if len(got) != len(want) {
			t.Fatalf("LookupQualified(%q) lengths = %d and %d", fqn, len(got), len(want))
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("LookupQualified(%q)[%d] differs: %p and %p", fqn, i, got[i], want[i])
			}
		}
	}

	pkg, ok := scope.LookupLocal("P")
	if !ok {
		t.Fatal("scope missing package P")
	}
	as := pkg.Scope.LookupLocalAll("A")
	if len(as) != 2 {
		t.Fatalf("package P has %d A members, want 2", len(as))
	}
	if got := first.Declaring("P::A"); got != as[0] {
		t.Fatalf("Declaring(P::A) = %p, want first declaration %p", got, as[0])
	}
}

func TestIndexRemoveDocument(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package P { namespace N; }")
	if got := idx.LookupQualified("P::N"); len(got) != 1 {
		t.Fatalf("before remove: P::N = %d symbols, want 1", len(got))
	}
	idx.RemoveDocument("a.sysml")
	if got := idx.LookupQualified("P::N"); len(got) != 0 {
		t.Fatalf("after remove: P::N = %d symbols, want 0", len(got))
	}
	if idx.DocumentRoot("a.sysml") != nil {
		t.Fatalf("after remove: DocumentRoot should be nil")
	}
}

func TestIndexReAddReplacesStaleEntries(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package P { namespace Old; }")
	addDoc(t, idx, "a.sysml", "package P { namespace New; }")
	if got := idx.LookupQualified("P::Old"); len(got) != 0 {
		t.Fatalf("P::Old = %d symbols after re-add, want 0 (stale)", len(got))
	}
	if got := idx.LookupQualified("P::New"); len(got) != 1 {
		t.Fatalf("P::New = %d symbols after re-add, want 1", len(got))
	}
	if got := idx.LookupQualified("P"); len(got) != 1 {
		t.Fatalf("P = %d symbols after re-add, want 1 (not doubled)", len(got))
	}
}

func TestIndexRemoveUnknownDocumentNoop(t *testing.T) {
	idx := NewIndex()
	idx.RemoveDocument("missing.sysml") // must not panic
	addDoc(t, idx, "a.sysml", "package P;")
	idx.RemoveDocument("b.sysml") // unrelated doc untouched
	if got := idx.LookupQualified("P"); len(got) != 1 {
		t.Fatalf("P = %d after removing unrelated doc, want 1", len(got))
	}
}

// A wildcard import enumerates its target's members from the index, so a
// re-added document must not leave the removed declaration enumerable.
func TestExpandWildcardImportsForgetsARemovedMember(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package Lib { namespace Old; }")
	addDoc(t, idx, "b.sysml", "package App { public import Lib::*; }")
	idx.ExpandWildcardImports()
	if got := len(idx.LookupQualified("App::Old")); got != 1 {
		t.Fatalf("App::Old = %d symbols, want 1", got)
	}

	addDoc(t, idx, "a.sysml", "package Lib { namespace New; }")
	idx.ExpandWildcardImports()
	if got := len(idx.LookupQualified("App::New")); got != 1 {
		t.Errorf("App::New = %d symbols after re-add, want 1", got)
	}
	if got := idx.exportedChildren("Lib"); len(got) != 1 || got[0].Name != "New" {
		t.Errorf("exportedChildren(Lib) = %v, want just New", got)
	}
}

// A wildcard import of library content surfaces its members, which are parsed
// declarations marked as library on every load path.
func TestExpandWildcardImportsReachesLibraryMembers(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "lib.sysml", "package Lib { namespace Thing; }")
	idx.MarkLibrary("lib.sysml")
	addDoc(t, idx, "b.sysml", "package App { public import Lib::*; }")
	idx.ExpandWildcardImports()

	if got := len(idx.LookupQualified("App::Thing")); got != 1 {
		t.Errorf("App::Thing = %d symbols, want 1", got)
	}
}

// A privately imported name is a member of the importing namespace, but only
// from inside it (KerML 8.2.3.3): the enumeration a wildcard import of that
// namespace reads must drop it, and the one a reference from within it reads
// must not.
func TestLookupDirectChildrenFromDropsPrivatelyImportedNames(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "base.sysml", "package Base { namespace Hidden; }")
	addDoc(t, idx, "mid.sysml", "package Mid { private import Base::*; namespace Own; }")
	idx.ExpandWildcardImports()

	names := func(syms []*Symbol) []string {
		var out []string
		for _, sym := range syms {
			out = append(out, sym.Name)
		}
		return out
	}
	if got := names(idx.LookupDirectChildrenFrom("Mid", "")); len(got) != 1 || got[0] != "Own" {
		t.Errorf("LookupDirectChildrenFrom(Mid, outside) = %v, want just Own", got)
	}
	if got := len(idx.LookupDirectChildrenFrom("Mid", "Mid::Inner")); got != 2 {
		t.Errorf("LookupDirectChildrenFrom(Mid, Mid::Inner) = %d symbols, want 2", got)
	}
	if got := len(idx.LookupDirectChildren("Mid")); got != 2 {
		t.Errorf("LookupDirectChildren(Mid) = %d symbols, want 2", got)
	}
}

func TestLookupDirectChildrenFromSharesVisibilityCache(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "base.sysml", "package Base { namespace Hidden; }")
	addDoc(t, idx, "mid.sysml", "package Mid { private import Base::*; namespace Own; }")
	idx.ExpandWildcardImports()

	idx.LookupDirectChildrenFrom("Mid", "Other")
	idx.LookupDirectChildrenFrom("Mid", "Another")
	if got := len(idx.directChildrenCache); got != 1 {
		t.Fatalf("direct-children cache has %d entries for equivalent visibility lookups, want 1", got)
	}
}

func TestLookupDirectChildrenInvalidatesAfterWrite(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package P { namespace A; }")
	if got := len(idx.LookupDirectChildren("P")); got != 1 {
		t.Fatalf("initial direct children = %d, want 1", got)
	}

	addDoc(t, idx, "b.sysml", "package P { namespace B; }")
	got := idx.LookupDirectChildren("P")
	if len(got) != 2 {
		t.Fatalf("direct children after write = %d, want 2", len(got))
	}
	if got[0].Name != "A" || got[1].Name != "B" {
		t.Fatalf("direct children after write = %q, want [A B]", []string{got[0].Name, got[1].Name})
	}
}

// A by-name lookup answers exactly what a scan of LookupDirectChildren for the
// name would: the children bearing it as their leaf name or short name, in
// enumeration order, with the private-import visibility of the plain lookup.
func TestLookupDirectChildrenNamedMatchesScan(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "base.sysml", "package Base { namespace Own; namespace Hidden; }")
	addDoc(t, idx, "mid.sysml", "package Mid { private import Base::*; namespace Own; namespace <short> Long; }")
	idx.ExpandWildcardImports()

	scan := func(all []*Symbol, name string) []*Symbol {
		var out []*Symbol
		for _, sym := range all {
			if LastSegment(sym.Name) == name || sym.ShortName == name {
				out = append(out, sym)
			}
		}
		return out
	}
	same := func(a, b []*Symbol) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}
	for _, name := range []string{"Own", "Hidden", "Long", "short", "Missing"} {
		for _, from := range []string{"", "Mid::Inner"} {
			got := idx.LookupDirectChildrenNamedFrom("Mid", from, name)
			want := scan(idx.LookupDirectChildrenFrom("Mid", from), name)
			if !same(got, want) {
				t.Errorf("LookupDirectChildrenNamedFrom(Mid, %q, %q) = %d symbols, want %d", from, name, len(got), len(want))
			}
		}
		got := idx.LookupDirectChildrenNamed("Mid", name)
		want := scan(idx.LookupDirectChildren("Mid"), name)
		if !same(got, want) {
			t.Errorf("LookupDirectChildrenNamed(Mid, %q) = %d symbols, want %d", name, len(got), len(want))
		}
	}
	if got := len(idx.LookupDirectChildrenNamedFrom("Mid", "", "Own")); got != 1 {
		t.Errorf("Own from outside = %d symbols, want just Mid's own", got)
	}
	if got := len(idx.LookupDirectChildrenNamedFrom("Mid", "Mid::Inner", "Own")); got != 2 {
		t.Errorf("Own from inside = %d symbols, want Mid's and the privately imported one", got)
	}
	if got := len(idx.LookupDirectChildrenNamed("Mid", "short")); got != 1 {
		t.Errorf("short name lookup = %d symbols, want 1", got)
	}
}

func TestLookupDirectChildrenNamedInvalidatesAfterWrite(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package P { namespace A; }")
	if got := len(idx.LookupDirectChildrenNamed("P", "B")); got != 0 {
		t.Fatalf("B before write = %d symbols, want 0", got)
	}
	addDoc(t, idx, "b.sysml", "package P { namespace B; }")
	if got := len(idx.LookupDirectChildrenNamed("P", "B")); got != 1 {
		t.Fatalf("B after write = %d symbols, want 1", got)
	}
}

func TestLookupDirectChildrenCachesIdenticalLookup(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package P { namespace A; }")

	first := idx.LookupDirectChildren("P")
	second := idx.LookupDirectChildren("P")
	if len(idx.directChildrenCache) != 1 {
		t.Fatalf("direct-children cache has %d entries, want 1", len(idx.directChildrenCache))
	}
	if len(first) == 0 || &first[0] != &second[0] {
		t.Fatal("identical direct-children lookup did not return the cached slice")
	}
}

// The index records the conditions a re-export is subject to instead of judging
// them: an import's filter clause and the `filter` members of the namespace it
// imports into gate the name it surfaced, while a name the namespace declares
// itself is never gated (KerML 8.2.4).
func TestExpandWildcardImportsGatesAFilteredReexport(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "base.sysml", "package Base { part def Belt; part def Bolt; }")
	addDoc(t, idx, "safe.sysml", `package Safe {
		public import Base::*[@Safety];
		filter @Safety;
		part def Own;
	}`)
	idx.ExpandWildcardImports()

	belt := lookupOne(t, idx, "Base::Belt")
	routes := idx.ReexportGates("", "Safe::Belt", belt, "")
	if len(routes) != 1 {
		t.Fatalf("Safe::Belt is gated by %d routes, want 1", len(routes))
	}
	// The import's clause and the namespace's filter member, composed.
	if len(routes[0]) != 2 {
		t.Errorf("the route to Safe::Belt carries %d conditions, want 2", len(routes[0]))
	}
	own := lookupOne(t, idx, "Safe::Own")
	if routes := idx.ReexportGates("", "Safe::Own", own, ""); len(routes) != 0 {
		t.Errorf("the declared member Safe::Own is gated by %d routes, want none", len(routes))
	}
}

// A name two imports surface is reached through either of them, so the index
// records a route each: an unfiltered import re-exports the name whatever a
// filtered import of the same namespace would reject.
func TestExpandWildcardImportsKeepsAnUnfilteredRoute(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "base.sysml", "package Base { part def Belt; }")
	addDoc(t, idx, "both.sysml", `package Both {
		public import Base::*[@Safety];
		public import Base::*;
	}`)
	idx.ExpandWildcardImports()

	belt := lookupOne(t, idx, "Base::Belt")
	routes := idx.ReexportGates("", "Both::Belt", belt, "")
	if len(routes) != 1 || len(routes[0]) != 0 {
		t.Fatalf("Both::Belt is gated by %v, want a single unconditional route", routes)
	}
}

// Expansion repeats over an importer whenever its imports may have changed, so
// the conditions of one import must be recorded once however often it is read.
func TestExpandWildcardImportsRecordsAGateOnce(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "base.sysml", "package Base { part def Belt; }")
	addDoc(t, idx, "safe.sysml", "package Safe { public import Base::*[@Safety]; }")
	idx.ExpandWildcardImports()
	idx.ExpandWildcardImports()

	belt := lookupOne(t, idx, "Base::Belt")
	if routes := idx.ReexportGates("", "Safe::Belt", belt, ""); len(routes) != 1 {
		t.Errorf("Safe::Belt is gated by %d routes after two expansions, want 1", len(routes))
	}
}

// A namespace importing a filtering one onward carries that namespace's
// conditions along with its own, so the onward route is no wider than the one it
// imports through.
func TestExpandWildcardImportsCarriesGatesOnward(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "base.sysml", "package Base { part def Belt; }")
	addDoc(t, idx, "safe.sysml", `package Safe {
		public import Base::*;
		filter @Safety;
	}`)
	addDoc(t, idx, "onward.sysml", "package Onward { public import Safe::*; }")
	addDoc(t, idx, "further.sysml", `package Further {
		public import Onward::*[@Mandatory];
	}`)
	idx.ExpandWildcardImports()

	belt := lookupOne(t, idx, "Base::Belt")
	for fqn, want := range map[string]int{
		"Safe::Belt":    1, // the namespace's own filter member
		"Onward::Belt":  1, // inherited from Safe, which added nothing of its own
		"Further::Belt": 2, // Safe's filter member and this import's clause
	} {
		routes := idx.ReexportGates("", fqn, belt, "")
		if len(routes) != 1 {
			t.Fatalf("%s is gated by %d routes, want 1", fqn, len(routes))
		}
		if len(routes[0]) != want {
			t.Errorf("the route to %s carries %d conditions, want %d", fqn, len(routes[0]), want)
		}
	}
}

// An unconditional route into a filtering namespace stays a route of its own
// when a further namespace imports it onward, so a name reachable unfiltered
// through one import is not narrowed by another.
func TestExpandWildcardImportsCarriesEachRouteOnward(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "base.sysml", "package Base { part def Belt; }")
	addDoc(t, idx, "mid.sysml", `package Mid {
		public import Base::*[@Safety];
		public import Base::*;
	}`)
	addDoc(t, idx, "onward.sysml", "package Onward { public import Mid::*; }")
	idx.ExpandWildcardImports()

	belt := lookupOne(t, idx, "Base::Belt")
	routes := idx.ReexportGates("", "Onward::Belt", belt, "")
	if len(routes) != 1 || len(routes[0]) != 0 {
		t.Fatalf("Onward::Belt is gated by %v, want a single unconditional route", routes)
	}
}

// A route widening a claim after a namespace importing it onward was already
// derived has to reach that importer: the wider route arrives in a later round,
// and the importer copied the narrower one (see routesOnward).
func TestExpandWildcardImportsCarriesAWidenedRouteOnward(t *testing.T) {
	// Repeat: derivation order decides which round widens the claim.
	for i := 0; i < 8; i++ {
		idx := NewIndex()
		addDoc(t, idx, "base.sysml", "package Base { part def Belt; }")
		addDoc(t, idx, "a.sysml", "package Aonward { public import Safe::*; }")
		addDoc(t, idx, "safe.sysml", `package Safe {
			public import Base::*[@Safety];
			public import Zalt::*;
		}`)
		addDoc(t, idx, "zalt.sysml", "package Zalt { public import Base::*; }")
		idx.ExpandWildcardImports()

		belt := lookupOne(t, idx, "Base::Belt")
		for _, fqn := range []string{"Safe::Belt", "Aonward::Belt"} {
			routes := idx.ReexportGates("", fqn, belt, "")
			if len(routes) != 1 || len(routes[0]) != 0 {
				t.Fatalf("run %d: %s is gated by %v, want a single unconditional route", i, fqn, routes)
			}
		}
	}
}

// A cycle of filtered imports settles: composing a route with a condition it
// already carries adds nothing, so the routes stay bounded and expansion
// terminates instead of composing ever longer condition chains.
func TestExpandWildcardImportsSettlesOnACycleOfFilters(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package A { part def X; }")
	addDoc(t, idx, "b.sysml", "package B { public import A::*[@a]; public import C::*[@c]; }")
	addDoc(t, idx, "c.sysml", "package C { public import B::*[@b]; }")

	done := make(chan struct{})
	go func() { defer close(done); idx.ExpandWildcardImports() }()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("expansion did not settle on a cycle of filtered imports")
	}

	x := lookupOne(t, idx, "A::X")
	for fqn, want := range map[string]int{"B::X": 1, "C::X": 2} {
		routes := idx.ReexportGates("", fqn, x, "")
		if len(routes) != 1 || len(routes[0]) != want {
			t.Errorf("%s is gated by %v, want a single route of %d conditions", fqn, routes, want)
		}
	}
}

// A namespace's filters may be declared by another document than its imports,
// so adding or removing that document has to re-derive the routes into it: a
// gate recorded before a filter arrived would keep admitting everything.
func TestNamespaceFilterFromAnotherDocumentRegatesReexports(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "base.sysml", "package Base { part def Belt; }")
	addDoc(t, idx, "imports.sysml", "package Safe { public import Base::*; }")
	idx.ExpandWildcardImports()

	belt := lookupOne(t, idx, "Base::Belt")
	if routes := idx.ReexportGates("", "Safe::Belt", belt, ""); len(routes) != 1 || len(routes[0]) != 0 {
		t.Fatalf("Safe::Belt is gated by %v before any filter, want a single unconditional route", routes)
	}

	addDoc(t, idx, "filter.sysml", "package Safe { filter @Safety; }")
	idx.ExpandWildcardImports()
	routes := idx.ReexportGates("", "Safe::Belt", belt, "")
	if len(routes) != 1 || len(routes[0]) != 1 {
		t.Fatalf("Safe::Belt is gated by %v once a filter is declared, want one route of one condition", routes)
	}

	idx.RemoveDocument("filter.sysml")
	if routes := idx.ReexportGates("", "Safe::Belt", belt, ""); len(routes) != 1 || len(routes[0]) != 0 {
		t.Fatalf("Safe::Belt is gated by %v once the filter is gone, want a single unconditional route", routes)
	}
}

// Two documents can import the same namespace into one package, so dropping the
// document whose import is unfiltered has to take its unconditional route with
// it — otherwise the surviving document's filter would keep admitting everything.
func TestDroppingAnUnfilteredImportLeavesTheSurvivingFilterInForce(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "base.sysml", "package Base { part def Belt; }")
	addDoc(t, idx, "plain.sysml", "package Safe { public import Base::*; }")
	addDoc(t, idx, "filtered.sysml", "package Safe { public import Base::*[@Safety]; }")
	idx.ExpandWildcardImports()

	belt := lookupOne(t, idx, "Base::Belt")
	routes := idx.ReexportGates("", "Safe::Belt", belt, "")
	if len(routes) != 2 {
		t.Fatalf("Safe::Belt is gated by %v, want a route per importing document", routes)
	}

	idx.RemoveDocument("plain.sysml")
	routes = idx.ReexportGates("", "Safe::Belt", belt, "")
	if len(routes) != 1 || len(routes[0]) != 1 {
		t.Fatalf("Safe::Belt is gated by %v once the unfiltered import is gone, want one route of one condition", routes)
	}
}

// A private import is a route only for lookups made inside the importing
// namespace, so its unconditional route must not answer one from outside and
// defeat the filter of a public import of the same namespace.
func TestAPrivateRouteDoesNotAnswerALookupFromOutside(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "base.sysml", "package Base { part def Belt; }")
	addDoc(t, idx, "app.sysml", `package Safe {
		private import Base::*;
		public import Base::*[@Safety];
	}`)
	idx.ExpandWildcardImports()

	belt := lookupOne(t, idx, "Base::Belt")
	routes := idx.ReexportGates("", "Safe::Belt", belt, "")
	if len(routes) != 1 || len(routes[0]) != 1 {
		t.Fatalf("Safe::Belt is gated by %v from outside, want only the public import's filtered route", routes)
	}
	routes = idx.ReexportGates("", "Safe::Belt", belt, "Safe")
	if len(routes) != 2 {
		t.Errorf("Safe::Belt is gated by %v from within Safe, want the private route too", routes)
	}
}

// A root-level re-export belongs to the importing document's own root namespace,
// so a lookup made in another document does not reach it.
func TestARootReexportIsVisibleOnlyInItsOwnDocument(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "base.sysml", "package Base { part def Belt; }")
	addDoc(t, idx, "importer.sysml", "import Base::*;")
	addDoc(t, idx, "other.sysml", "package Other;")
	idx.ExpandWildcardImports()

	belt := lookupOne(t, idx, "Base::Belt")
	if !idx.ReexportVisible("importer.sysml", "Belt", belt) {
		t.Error("the importing document's own root import surfaces Belt there")
	}
	if idx.ReexportVisible("other.sysml", "Belt", belt) {
		t.Error("another document does not see a name imported into importer.sysml's root")
	}
	if !idx.ReexportVisible("other.sysml", "Base::Belt", belt) {
		t.Error("Base::Belt is declared, not re-exported, and is visible everywhere")
	}
}

// lookupOne returns the single symbol registered under fqn.
func lookupOne(t *testing.T, idx *Index, fqn string) *Symbol {
	t.Helper()
	syms := idx.LookupQualified(fqn)
	if len(syms) != 1 {
		t.Fatalf("%s names %d symbols, want 1", fqn, len(syms))
	}
	return syms[0]
}

// Each document owns its root namespace, so a `filter` one document states
// there gates its own root-level imports and no other document's.
func TestRootNamespaceFiltersGateOnlyTheirOwnDocument(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "base.sysml", "package Base { part def Belt; }")
	addDoc(t, idx, "filtered.sysml", "import Base::*;\nfilter @Safety;")
	addDoc(t, idx, "plain.sysml", "import Base::*;")
	idx.ExpandWildcardImports()

	belt := lookupOne(t, idx, "Base::Belt")
	routes := idx.ReexportGates("filtered.sysml", "Belt", belt, "")
	if len(routes) != 1 || len(routes[0]) != 1 {
		t.Fatalf("the filtering document's route to Belt carries %v, want one condition", routes)
	}
	routes = idx.ReexportGates("plain.sysml", "Belt", belt, "")
	if len(routes) != 1 || len(routes[0]) != 0 {
		t.Errorf("the other document's route to Belt carries %v, want no condition", routes)
	}
}
