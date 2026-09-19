package resolve

import (
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// trackedIndex is an index with a tracking resolver over it and the roots of
// its documents, which the tests analyze and edit.
type trackedIndex struct {
	t     *testing.T
	idx   *symbols.Index
	r     *Resolver
	roots map[string]*ast.RootNamespace
}

func newTrackedIndex(t *testing.T, docs map[string]string) *trackedIndex {
	t.Helper()
	w := &trackedIndex{t: t, idx: symbols.NewIndex(), roots: map[string]*ast.RootNamespace{}}
	for name, src := range docs {
		w.roots[name] = parsedRoot(t, name, src)
		w.idx.AddDocument(name, w.roots[name])
	}
	w.r = New(w.idx)
	w.r.Track()
	return w
}

// analyze resolves doc's references as its analysis would.
func (w *trackedIndex) analyze(doc string) {
	w.r.InDocument(doc, func() { w.r.ResolveDocument(doc, w.roots[doc]) })
}

func (w *trackedIndex) analyzeAll() {
	for name := range w.roots {
		w.analyze(name)
	}
}

// put replaces doc and invalidates through the changes the index recorded, as
// the workspace does; it returns the documents dropped.
func (w *trackedIndex) put(doc, src string) []string {
	w.t.Helper()
	w.roots[doc] = parsedRoot(w.t, doc, src)
	w.idx.AddDocument(doc, w.roots[doc])
	ch := w.idx.TakeChanges()
	if ch.Docs == nil {
		ch.Docs = map[string]bool{}
	}
	ch.Docs[doc] = true
	return w.r.Invalidate(ch)
}

func (w *trackedIndex) owned(doc string) bool { return w.r.Owned(doc) > 0 }

func TestFramesOwnWhatAnAnalysisMemoizes(t *testing.T) {
	w := newTrackedIndex(t, map[string]string{
		"a.sysml": "package A { part def X; part x : X; }",
		"b.sysml": "package B { part def Y; part y : Y; }",
	})
	w.analyzeAll()
	if !w.owned("a.sysml") || !w.owned("b.sysml") {
		t.Fatalf("owned: a %d, b %d; want both > 0", w.r.Owned("a.sysml"), w.r.Owned("b.sysml"))
	}
	if deps := w.r.Dependents("a.sysml"); len(deps) != 0 {
		t.Fatalf("a.sysml has dependents %v, want none", deps)
	}
}

func TestFramesDependOnTheDocumentASymbolCameFrom(t *testing.T) {
	w := newTrackedIndex(t, map[string]string{
		"lib.sysml":   "package Lib { part def Base; }",
		"user.sysml":  "package User { part def Derived :> Lib::Base; }",
		"other.sysml": "package Other { part def Alone; part alone : Alone; }",
	})
	w.analyzeAll()
	if got := w.r.Dependents("lib.sysml"); !reflect.DeepEqual(got, []string{"user.sysml"}) {
		t.Fatalf("dependents of lib.sysml = %v, want [user.sysml]", got)
	}
	dropped := w.put("lib.sysml", "package Lib { part def Base; part def More; }")
	if !reflect.DeepEqual(dropped, []string{"lib.sysml", "user.sysml"}) {
		t.Fatalf("dropped %v, want [lib.sysml user.sysml]", dropped)
	}
	if !w.owned("other.sysml") {
		t.Fatal("other.sysml, which read nothing of lib.sysml, lost what it memoized")
	}
	if w.owned("user.sysml") {
		t.Fatal("user.sysml, which specializes Lib::Base, kept what it memoized")
	}
}

func TestFramesDependOnAnImportedNamespace(t *testing.T) {
	w := newTrackedIndex(t, map[string]string{
		"lib.sysml":  "package Lib { part def Base; }",
		"user.sysml": "package User { private import Lib::*; part def Derived :> Base; }",
	})
	w.analyzeAll()
	// A name added to the imported namespace could shadow or newly resolve
	// what the importer sees, so the importer is dropped.
	dropped := w.put("lib.sysml", "package Lib { part def Base; part def Derived; }")
	if !reflect.DeepEqual(dropped, []string{"lib.sysml", "user.sysml"}) {
		t.Fatalf("dropped %v, want [lib.sysml user.sysml]", dropped)
	}
}

func TestFramesDependOnAPackageSpanningDocuments(t *testing.T) {
	w := newTrackedIndex(t, map[string]string{
		"a.sysml": "package P { part def X; }",
		"b.sysml": "package P { part y : X; }",
		"c.sysml": "package Q { part def Z; part z : Z; }",
	})
	w.analyzeAll()
	dropped := w.put("a.sysml", "package P { part def X2; }")
	if !reflect.DeepEqual(dropped, []string{"a.sysml", "b.sysml"}) {
		t.Fatalf("dropped %v, want [a.sysml b.sysml]", dropped)
	}
	if !w.owned("c.sysml") {
		t.Fatal("c.sysml, in another package, lost what it memoized")
	}
}

func TestFramesInvalidateDependentsTransitively(t *testing.T) {
	w := newTrackedIndex(t, map[string]string{
		"a.sysml": "package A { part def X; }",
		"b.sysml": "package B { part def Y :> A::X; }",
		"c.sysml": "package C { part def Z :> B::Y; }",
		"d.sysml": "package D { part def W; part w : W; }",
	})
	w.analyzeAll()
	dropped := w.put("a.sysml", "package A { part def X { attribute a; } }")
	if !reflect.DeepEqual(dropped, []string{"a.sysml", "b.sysml", "c.sysml"}) {
		t.Fatalf("dropped %v, want [a.sysml b.sysml c.sysml]", dropped)
	}
	if !w.owned("d.sysml") {
		t.Fatal("d.sysml, reading nothing of the others, lost what it memoized")
	}
	// Re-analysis restores the relation, so a second edit finds it again.
	w.analyzeAll()
	if got := w.r.Dependents("a.sysml"); !reflect.DeepEqual(got, []string{"b.sysml"}) {
		t.Fatalf("dependents of a.sysml after re-analysis = %v, want [b.sysml]", got)
	}
}

func TestFramesReleaseAReplacedDocumentsEntries(t *testing.T) {
	w := newTrackedIndex(t, map[string]string{
		"a.sysml": "package A { part def X; part x : X; part y : X; }",
	})
	w.analyze("a.sysml")
	before := w.r.Owned("a.sysml")
	for i := 0; i < 50; i++ {
		w.put("a.sysml", "package A { part def X; part x : X; part y : X; }")
		w.analyze("a.sysml")
		if got := w.r.Owned("a.sysml"); got != before {
			t.Fatalf("edit %d: a.sysml owns %d entries, want %d as after the first analysis", i, got, before)
		}
	}
}

func TestFramesGatherIsNoDependencyOfTheAnalysis(t *testing.T) {
	w := newTrackedIndex(t, map[string]string{
		"a.sysml": "package A { part def X; }",
		"b.sysml": "package B { part def Y :> A::X; }",
	})
	w.analyze("a.sysml")
	w.r.InDocument("b.sysml", func() {
		w.r.Gather("a.sysml", func() { w.r.ResolveDocument("a.sysml", w.roots["a.sysml"]) })
	})
	if deps := w.r.Dependents("a.sysml"); len(deps) != 0 {
		t.Fatalf("a gather from b.sysml made it depend on a.sysml: %v", deps)
	}
	if got := w.r.Dependents(GatherFrame("a.sysml")); len(got) != 0 {
		t.Fatalf("the gather frame has dependents %v, want none", got)
	}
	if doc, ok := GatheredDoc(GatherFrame("a.sysml")); !ok || doc != "a.sysml" {
		t.Fatalf("GatheredDoc(GatherFrame(a.sysml)) = %q, %v", doc, ok)
	}
	dropped := w.put("a.sysml", "package A { part def X2; }")
	if !reflect.DeepEqual(dropped, []string{"a.sysml", GatherFrame("a.sysml")}) {
		t.Fatalf("dropped %v, want the document and its gather frame", dropped)
	}
}

func TestFramesReadingTheWholeIndexSurviveAJudgmentChange(t *testing.T) {
	w := newTrackedIndex(t, map[string]string{
		"a.sysml": "package A { part def X; }",
		"b.sysml": "package B { part def Y :> A::X; }",
	})
	w.analyze("a.sysml")
	w.r.InDocument("b.sysml", func() {
		w.r.ReadAllNames()
		w.r.ReadName("\x00judgment/b")
		w.r.suggestTable()
	})
	table := w.r.names
	if table == nil {
		t.Fatal("the analysis built no suggestion table")
	}
	if dropped := w.r.Invalidate(symbols.Changes{Names: map[string]bool{"\x00judgment/a": true}}); len(dropped) != 0 {
		t.Fatalf("a judgment b.sysml never read dropped %v", dropped)
	}
	if w.r.names != table {
		t.Fatal("a judgment change rebuilt the suggestion table")
	}
	if dropped := w.r.Invalidate(symbols.Changes{Names: map[string]bool{"\x00judgment/b": true}}); !reflect.DeepEqual(dropped, []string{"b.sysml"}) {
		t.Fatalf("the judgment b.sysml read dropped %v, want b.sysml", dropped)
	}
	if w.r.names != table {
		t.Fatal("a judgment change rebuilt the suggestion table")
	}
	w.r.InDocument("b.sysml", func() { w.r.ReadAllNames() })
	if dropped := w.put("a.sysml", "package A { part def X2; }"); !reflect.DeepEqual(dropped, []string{"a.sysml", "b.sysml"}) {
		t.Fatalf("a registration dropped %v, want the whole-index reader too", dropped)
	}
	if w.r.names != table {
		t.Fatal("a registration rebuilt the suggestion table instead of refreshing it")
	}
	if got := table.Declared("X2"); !reflect.DeepEqual(got, []string{"A::X2"}) {
		t.Fatalf("the refreshed table declares X2 as %v, want A::X2", got)
	}
	if got := table.Declared("X"); len(got) != 0 {
		t.Fatalf("the refreshed table still declares X as %v", got)
	}
}
