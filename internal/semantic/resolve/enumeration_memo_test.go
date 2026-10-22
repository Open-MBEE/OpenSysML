package resolve

import (
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// enumerationResolver resolves src as d.sysml and returns the resolver with the
// scope named ns and its first import.
func enumerationResolver(t *testing.T, src, ns string) (*Resolver, *symbols.Scope, *ast.Import) {
	t.Helper()
	p := parser.New(source.New("d.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc("d.sysml", root)
	idx.ExpandWildcardImports()
	r := New(idx)
	r.ResolveDocument("d.sysml", root)
	scope := idx.LookupQualified(ns)[0].Scope
	return r, scope, r.scopeImports(scope)[0]
}

// Enumerating the members an import surfaces again costs nothing: the first
// enumeration is kept for the query asking it again (a rename, a references index).
func TestImportedMembersAreEnumeratedOncePerResolver(t *testing.T) {
	r, scope, imp := enumerationResolver(t, `package L { part def A; part def B; part def C; }
package Q { public import L::*; }
package R { public import Q::*; }`, "R")
	if got := len(r.importedMembersInto(scope, scope, imp, false)); got != 3 {
		t.Fatalf("the import brings %d members, want 3", got)
	}
	got := testing.AllocsPerRun(20, func() { r.importedMembersInto(scope, scope, imp, false) })
	if got != 0 {
		t.Errorf("enumerating the import again allocates %.2f objects, want 0", got)
	}
}

// An enumeration kept is dropped with the document it read: after the imported
// package gains a member, enumerating the import again surfaces it.
func TestImportedMembersKeptAreDroppedOnEdit(t *testing.T) {
	w := newTrackedIndex(t, map[string]string{
		"l.sysml": "package L { part def A; }",
		"r.sysml": "package R { public import L::*; part x : A; }",
	})
	w.analyzeAll()
	scope := w.idx.LookupQualified("R")[0].Scope
	imp := w.r.scopeImports(scope)[0]
	if got := len(w.r.ImportedMembersInto(scope, scope, imp)); got != 1 {
		t.Fatalf("the import brings %d members, want 1", got)
	}
	w.put("l.sysml", "package L { part def A; part def B; }")
	if got := len(w.r.ImportedMembersInto(scope, scope, imp)); got != 2 {
		t.Fatalf("after the edit the import brings %d members, want 2", got)
	}
}

// A document reading an enumeration another document kept depends on what that
// enumeration read: an edit of the imported package drops the reader too.
func TestImportedMembersKeptMakeTheirReaderDepend(t *testing.T) {
	w := newTrackedIndex(t, map[string]string{
		"l.sysml": "package L { part def A; }",
		"q.sysml": "package Q { public import L::*; part def C; }",
		"r.sysml": "package R { public import Q::*; }",
		"s.sysml": "package S { public import Q::*; }",
	})
	for _, doc := range []string{"r.sysml", "s.sysml"} {
		scope := w.idx.LookupQualified(strings.ToUpper(doc[:1]))[0].Scope
		imp := w.r.scopeImports(scope)[0]
		w.r.InDocument(doc, func() {
			if got := len(w.r.ImportedMembersInto(scope, scope, imp)); got != 2 {
				t.Fatalf("%s: the import brings %d members, want 2", doc, got)
			}
		})
	}
	if deps := w.r.Dependents("l.sysml"); !slices.Contains(deps, "s.sysml") {
		t.Errorf("l.sysml's dependents are %v, want s.sysml among them", deps)
	}
	dropped := w.put("l.sysml", "package L { part def A; part def C; }")
	if !slices.Contains(dropped, "s.sysml") {
		t.Errorf("editing L dropped %v, want s.sysml among them", dropped)
	}
}

// A filter condition's own names resolve unfiltered (KerML 8.2.4): enumerating
// a filtered import inside a condition surfaces every member, not the kept
// filtered enumeration.
func TestImportedMembersInAConditionAreNotTheFilteredOnes(t *testing.T) {
	idx := expandedIndexOf(t, map[string]string{
		"lib.sysml": "package Lib { part def SafeBelt; part def Wheel; }",
		"app.sysml": "package App { public import Lib::*[@Safety]; }",
	})
	r := judgingResolver(idx)
	scope := idx.LookupQualified("App")[0].Scope
	imp := r.scopeImports(scope)[0]
	if got := len(r.importedMembersInto(scope, scope, imp, false)); got != 1 {
		t.Fatalf("the filtered import brings %d members, want 1", got)
	}
	var inCondition int
	r.InCondition(func() { inCondition = len(r.importedMembersInto(scope, scope, imp, false)) })
	if inCondition != 2 {
		t.Errorf("in a condition the import brings %d members, want 2", inCondition)
	}
	if got := len(r.importedMembersInto(scope, scope, imp, false)); got != 1 {
		t.Errorf("after the condition the filtered import brings %d members, want 1", got)
	}
}
