package resolve

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// stubJudge stands in for the semantic model when the question under test is
// where resolution consults a filter, not what a condition means: it admits the
// elements whose name carries the marker, as `@Safety` admits the annotated
// ones. The evaluator itself is tested in semantics.
type stubJudge struct{ marker string }

func (stubJudge) LookupMember(*symbols.Symbol, string) (*symbols.Symbol, bool) {
	return nil, false
}

func (stubJudge) LookupContributedMember(*symbols.Symbol, string) (*symbols.Symbol, bool) {
	return nil, false
}

func (j stubJudge) SatisfiesElementFilter(_ symbols.ElementFilter, cand *symbols.Symbol) bool {
	return cand != nil && strings.Contains(cand.Name, j.marker)
}

// expandedIndexOf indexes docs and expands their wildcard imports, so the names
// an import surfaces are registered the way a workspace registers them.
func expandedIndexOf(t *testing.T, docs map[string]string) *symbols.Index {
	t.Helper()
	idx := indexOf(t, docs)
	idx.ExpandWildcardImports()
	return idx
}

// judgingResolver resolves over idx with a filter judge attached, the way a
// workspace attaches its semantic model.
func judgingResolver(idx *symbols.Index) *Resolver {
	r := New(idx)
	r.SetModel(stubJudge{marker: "Safe"})
	return r
}

const filterLib = `package Lib {
	part def SafeBelt;
	part def Radio;
}`

// An import's filter clause restricts what that import brings in (SysML v2
// 7.4.4): a name it rejects is not resolvable through the importing namespace,
// by either the unqualified or the qualified route.
func TestFilteredImportHidesARejectedElement(t *testing.T) {
	idx := expandedIndexOf(t, map[string]string{
		"lib.sysml": filterLib,
		"app.sysml": "package App { public import Lib::*[@Safety]; }",
	})
	app := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")

	if _, ok := judgingResolver(idx).ResolveName(app, "SafeBelt", ident("SafeBelt")); !ok {
		t.Error("the filter admits SafeBelt, which must resolve through the filtered import")
	}
	if _, ok := judgingResolver(idx).ResolveName(app, "Radio", ident("Radio")); ok {
		t.Error("the filter rejects Radio, which must not resolve through the filtered import")
	}
	if _, ok := judgingResolver(idx).ResolveQualified(nil, qn(false, "App", "Radio")); ok {
		t.Error("App::Radio names an element the import filtered out, so it must not resolve")
	}
	if _, ok := judgingResolver(idx).ResolveQualified(nil, qn(false, "Lib", "Radio")); !ok {
		t.Error("Lib::Radio is a declared member of Lib and no filter applies to it")
	}
}

func TestFilteredRecursiveReexportHidesRejectedElement(t *testing.T) {
	idx := expandedIndexOf(t, map[string]string{
		"lib.sysml": filterLib,
		"mid.sysml": "package Mid { public import Lib::*; }",
		"app.sysml": "package App { public import Mid::**[@Safety]; }",
	})
	app := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")
	r := judgingResolver(idx)
	if _, ok := r.ResolveName(app, "SafeBelt", ident("SafeBelt")); !ok {
		t.Error("the recursive filter admits SafeBelt, which must resolve through the re-export")
	}
	if _, ok := r.ResolveName(app, "Radio", ident("Radio")); ok {
		t.Error("the recursive filter rejects Radio, which must not resolve through the re-export")
	}
}

// A private import surfaces a name only within the importing namespace (KerML
// 8.2.3.3), so its unfiltered route must not make a name a public filtered
// import rejects reachable from outside.
func TestAPrivateImportDoesNotDefeatAPublicFilterFromOutside(t *testing.T) {
	idx := expandedIndexOf(t, map[string]string{
		"lib.sysml": filterLib,
		"app.sysml": `package Safe {
			private import Lib::*;
			public import Lib::*[@Safety];
		}
		package Out { }`,
	})
	safe := scopeOf(t, idx.DocumentRoot("app.sysml"), "Safe")
	out := scopeOf(t, idx.DocumentRoot("app.sysml"), "Out")

	if _, ok := judgingResolver(idx).ResolveQualified(out, qn(false, "Safe", "Radio")); ok {
		t.Error("Safe's public import filters Radio out, so it must not resolve from outside Safe")
	}
	if _, ok := judgingResolver(idx).ResolveQualified(out, qn(false, "Safe", "SafeBelt")); !ok {
		t.Error("Safe's public import admits SafeBelt, which must resolve from outside Safe")
	}
	if _, ok := judgingResolver(idx).ResolveName(safe, "Radio", ident("Radio")); !ok {
		t.Error("Safe's own private import is unfiltered, so Radio is visible within Safe")
	}
}

// A root-level import surfaces its names in the importing document's own root
// namespace, so another document neither sees them nor escapes the filter that
// selected them.
func TestARootImportSurfacesNamesInItsOwnDocumentOnly(t *testing.T) {
	idx := expandedIndexOf(t, map[string]string{
		"lib.sysml":  filterLib,
		"safe.sysml": "import Lib::*[@Safety];",
		"other.sysml": `package Other {
			part a :> SafeBelt;
		}`,
	})
	r := judgingResolver(idx)
	other := scopeOf(t, idx.DocumentRoot("other.sysml"), "Other")

	if _, ok := r.ResolveName(other, "SafeBelt", ident("SafeBelt")); ok {
		t.Error("another document does not see what safe.sysml imported into its own root")
	}
	if _, ok := r.ResolveName(other, "Radio", ident("Radio")); ok {
		t.Error("Radio is filtered out of safe.sysml's root and not in another document's")
	}
	safe := idx.DocumentRoot("safe.sysml")
	if _, ok := r.ResolveName(safe, "SafeBelt", ident("SafeBelt")); !ok {
		t.Error("the importing document itself reaches the name its filter admits")
	}
}

// A namespace's `filter` members restrict the memberships its imports bring in,
// and leave the members it declares itself alone (KerML 8.2.4).
func TestNamespaceFilterKeepsDeclaredMembers(t *testing.T) {
	idx := expandedIndexOf(t, map[string]string{
		"lib.sysml": filterLib,
		"app.sysml": `package App {
			public import Lib::*;
			filter @Safety;
			part def Radiator;
		}`,
	})

	if _, ok := judgingResolver(idx).ResolveQualified(nil, qn(false, "App", "Radiator")); !ok {
		t.Error("a filter must not hide Radiator, which App declares itself")
	}
	if _, ok := judgingResolver(idx).ResolveQualified(nil, qn(false, "App", "Radio")); ok {
		t.Error("App's filter must hide Radio, which App only imports")
	}
	if _, ok := judgingResolver(idx).ResolveQualified(nil, qn(false, "App", "SafeBelt")); !ok {
		t.Error("App's filter admits SafeBelt, which it imports")
	}
}

// A filtered namespace re-exports onward only what its filter selects, so an
// outside importer of it sees the same subset — the filter travels with the
// membership rather than with one lookup route.
func TestNamespaceFilterAppliesToOnwardImporters(t *testing.T) {
	idx := expandedIndexOf(t, map[string]string{
		"lib.sysml": filterLib,
		"mid.sysml": "package Mid { public import Lib::*; filter @Safety; }",
		"app.sysml": "package App { public import Mid::*; }",
	})
	app := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")

	if _, ok := judgingResolver(idx).ResolveName(app, "SafeBelt", ident("SafeBelt")); !ok {
		t.Error("SafeBelt passes Mid's filter, so importing Mid must reach it")
	}
	if _, ok := judgingResolver(idx).ResolveName(app, "Radio", ident("Radio")); ok {
		t.Error("Radio does not pass Mid's filter, so importing Mid must not reach it")
	}
}

// An expose is an import (SysML v2 8.3.26.2), so its filter clause restricts
// what the view body sees.
func TestFilteredExposeSurfacesASubset(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"lib.sysml": filterLib,
		"app.sysml": "package App { view v { expose Lib::*[@Safety]; } }",
	})
	view := scopeOf(t, scopeOf(t, idx.DocumentRoot("app.sysml"), "App"), "v")

	if _, ok := judgingResolver(idx).ResolveName(view, "SafeBelt", ident("SafeBelt")); !ok {
		t.Error("the filtered expose admits SafeBelt, which must be visible in the view body")
	}
	if _, ok := judgingResolver(idx).ResolveName(view, "Radio", ident("Radio")); ok {
		t.Error("the filtered expose rejects Radio, which must not be visible in the view body")
	}
}

// A lookup made while a condition's own names are being resolved is unfiltered,
// so its answer is not remembered: whether a filtered-out name resolves must not
// depend on a condition having reached it first.
func TestALookupMadeWhileAConditionIsResolvedIsNotRemembered(t *testing.T) {
	idx := expandedIndexOf(t, map[string]string{
		"lib.sysml": filterLib,
		"app.sysml": "package App { public import Lib::*[@Safety]; }",
	})
	app := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")

	r := judgingResolver(idx)
	ref := ident("Radio")
	r.InCondition(func() { r.ResolveName(app, "Radio", ref) })
	if _, ok := r.ResolveName(app, "Radio", ref); ok {
		t.Error("the filter rejects Radio, which must not resolve because a condition reached it first")
	}

	r = judgingResolver(idx)
	qname := qn(false, "App", "Radio")
	r.InCondition(func() { r.ResolveQualified(nil, qname) })
	if _, ok := r.ResolveQualified(nil, qname); ok {
		t.Error("App::Radio names a filtered-out element, whichever lookup reached it first")
	}
}

func TestFilteredFailureDoesNotPoisonUnfilteredQualifiedLookup(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"app.sysml": "package App { part x; }",
	})
	app := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")
	sym, ok := app.LookupLocal("x")
	if !ok {
		t.Fatal("failed to find the declaration used as the hiding filter")
	}
	target := qn(false, "x")
	r := New(idx)

	if _, ok := r.resolveQualified(app, target, &refFilter{decl: sym.Decl}); ok {
		t.Fatal("the filtered lookup must hide the only candidate")
	}
	if got, ok := r.ResolveQualified(app, target); !ok || got != sym {
		t.Fatalf("unfiltered lookup = %v, %v; want %v after filtered failure", got, ok, sym)
	}
}

// Without a semantic model there is nothing to judge a condition with, and an
// unevaluable filter keeps the element rather than silently hiding it.
func TestFilterWithoutAModelKeepsEveryElement(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"lib.sysml": filterLib,
		"app.sysml": "package App { public import Lib::*[@Safety]; }",
	})
	app := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")

	if _, ok := New(idx).ResolveName(app, "Radio", ident("Radio")); !ok {
		t.Error("with no model attached the filter cannot be judged, so Radio must stay visible")
	}
}
