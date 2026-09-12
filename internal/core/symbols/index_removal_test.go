package symbols

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

// libDoc and libSource are the standard library as every load path holds it: a
// parsed document marked library content.
const libDoc = "lib.sysml"
const libSource = "package Lib { namespace Widget; namespace <kg> Kilogram; }"

// buildIndex indexes docs (in name order, so the result does not depend on map
// iteration order) plus the library, and expands to the fixpoint.
func buildIndex(t *testing.T, docs map[string]string) *Index {
	t.Helper()
	idx := NewIndex()
	addDoc(t, idx, libDoc, libSource)
	idx.MarkLibrary(libDoc)
	names := make([]string, 0, len(docs))
	for name := range docs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		addDoc(t, idx, name, docs[name])
	}
	idx.ExpandWildcardImports()
	return idx
}

// indexState renders everything the index makes observable: every registered
// fully-qualified name with the symbols under it, how each got there (declared
// or re-exported, hidden or not), the namespace each symbol was declared in, the
// child links a wildcard import enumerates, and the imports still driving
// expansion. Two indexes agreeing on it agree on every lookup.
func indexState(idx *Index) string {
	var b strings.Builder
	for _, fqn := range idx.FQNs() {
		descs := make([]string, 0, len(idx.fqn.at(fqn)))
		for _, sym := range idx.fqn.at(fqn) {
			descs = append(descs, describeEntry(idx, fqn, sym))
		}
		sort.Strings(descs)
		fmt.Fprintf(&b, "%s -> %s\n", fqn, strings.Join(descs, " "))
	}

	parents := idx.children.keys()
	sort.Strings(parents)
	for _, parent := range parents {
		kids := idx.childKeys(parent)
		fmt.Fprintf(&b, "children %q -> %v\n", parent, kids)
	}

	importers := idx.wildcardMeta.keys()
	sort.Strings(importers)
	for _, pkgFQN := range importers {
		fmt.Fprintf(&b, "imports %q -> %v\n", pkgFQN, idx.WildcardImportsOf(pkgFQN))
	}

	syms := idx.declaredAt.keys()
	declared := make([]string, 0, len(syms))
	for _, sym := range syms {
		declared = append(declared, idx.declaredAt.at(sym))
	}
	sort.Strings(declared)
	fmt.Fprintf(&b, "declaredAt -> %v\n", declared)
	return b.String()
}

// describeEntry renders one symbol registered under fqn without depending on its
// identity, which differs between two indexes over the same documents.
func describeEntry(idx *Index, fqn string, sym *Symbol) string {
	origin := "declared"
	switch {
	case idx.hidden.at(fqn).has(sym):
		origin = "reexported-hidden"
	case idx.reexported.at(fqn).has(sym):
		origin = "reexported"
	}
	return fmt.Sprintf("[%s kind=%v short=%q from=%q %s]",
		sym.Name, sym.Kind, sym.ShortName, idx.declaredAt.at(sym), origin)
}

// Removing a document must leave the index a fresh build over the remaining
// documents produces — the whole index, not the handful of names a case is
// about. That equality is what makes reusing an index across edits sound, so it
// is asserted over the import shapes whose re-exports removal has to unwind.
func TestRemoveDocumentEqualsFreshBuild(t *testing.T) {
	cases := []struct {
		name string
		docs map[string]string
	}{
		{
			name: "public wildcard",
			docs: map[string]string{
				"a.sysml": "package A { public import Lib::*; }",
				"b.sysml": "package B { part def Own; }",
			},
		},
		{
			name: "private wildcard",
			docs: map[string]string{
				"a.sysml": "package A { private import Lib::*; }",
				"b.sysml": "package B { private import Lib::*; }",
			},
		},
		{
			name: "name imported by two documents",
			docs: map[string]string{
				"a.sysml": "package Shared { public import Lib::*; }",
				"b.sysml": "package Shared { public import Lib::*; }",
			},
		},
		{
			name: "public import of a privately imported name",
			docs: map[string]string{
				"a.sysml": "package Shared { public import Lib::*; }",
				"b.sysml": "package Shared { private import Lib::*; }",
			},
		},
		{
			name: "transitive chain",
			docs: map[string]string{
				"a.sysml": "package Mid { public import Lib::*; }",
				"b.sysml": "package Top { public import Mid::*; }",
			},
		},
		{
			// A name Mid exports publicly reaches Top, and stops reaching it once
			// only Mid's private import of it remains.
			name: "privately imported name no longer carried on",
			docs: map[string]string{
				"a.sysml": "package Mid { public import Lib::*; }",
				"b.sysml": "package Mid2 { private import Lib::*; public import Mid::*; } " +
					"package Top { public import Mid2::*; }",
			},
		},
		{
			name: "transitive chain broken in the middle",
			docs: map[string]string{
				"a.sysml": "package Mid { public import Lib::*; }",
				"b.sysml": "package Top { public import Mid::*; } package Far { public import Top::*; }",
			},
		},
		{
			name: "two routes to the same name",
			docs: map[string]string{
				"a.sysml": "package Left { public import Lib::*; }",
				"b.sysml": "package Right { public import Lib::*; } " +
					"package Both { public import Left::*; public import Right::*; }",
			},
		},
		{
			name: "cycle of imports",
			docs: map[string]string{
				"a.sysml": "package A { public import B::*; public import Lib::*; }",
				"b.sysml": "package B { public import A::*; }",
			},
		},
		{
			name: "target does not exist",
			docs: map[string]string{
				"a.sysml": "package A { public import Missing::*; }",
				"b.sysml": "package B { public import Absent::*; part def Own; }",
			},
		},
		{
			name: "removal makes an ambiguous target resolvable",
			docs: map[string]string{
				"a.sysml": "package Ambiguous { part def FromA; }",
				"b.sysml": "package Ambiguous { part def FromB; } " +
					"package User { public import Ambiguous::*; }",
			},
		},
		{
			name: "short name re-export",
			docs: map[string]string{
				"a.sysml": "package A { public import Lib::*; }",
				"b.sysml": "package B { public import Lib::*; attribute def <mm> Millimetre; }",
			},
		},
		{
			name: "import shadowed by a declared member",
			docs: map[string]string{
				"a.sysml": "package Units { public import Lib::*; }",
				"b.sysml": "package Units { attribute <kg> kilo; }",
			},
		},
		{
			name: "removed document declares what another re-exports",
			docs: map[string]string{
				"a.sysml": "package Source { part def Exported; }",
				"b.sysml": "package Consumer { public import Source::*; }",
			},
		},
		{
			name: "document root import",
			docs: map[string]string{
				"a.sysml": "public import Lib::*;",
				"b.sysml": "package B { public import Lib::*; }",
			},
		},
		{
			name: "nested importer",
			docs: map[string]string{
				"a.sysml": "package Outer { package Inner { public import Lib::*; } }",
				"b.sysml": "package Other { public import Outer::Inner::*; }",
			},
		},
	}

	const removed = "a.sysml"
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := tc.docs[removed]; !ok {
				t.Fatalf("case has no %s to remove", removed)
			}
			remaining := make(map[string]string, len(tc.docs))
			for name, src := range tc.docs {
				if name != removed {
					remaining[name] = src
				}
			}

			reused := buildIndex(t, tc.docs)
			before := indexState(reused)
			reused.RemoveDocument(removed)
			fresh := buildIndex(t, remaining)
			if got, want := indexState(reused), indexState(fresh); got != want {
				t.Errorf("after removing %s the index differs from a fresh build:\n%s",
					removed, diffLines(want, got))
			}

			// Expanding again changes nothing: removal left the fixpoint.
			reused.ExpandWildcardImports()
			if got, want := indexState(reused), indexState(fresh); got != want {
				t.Errorf("expanding after removal changed the index:\n%s", diffLines(want, got))
			}

			// Re-adding the document restores exactly what removing it took away.
			addDoc(t, reused, removed, tc.docs[removed])
			reused.ExpandWildcardImports()
			if got := indexState(reused); got != before {
				t.Errorf("re-adding %s did not restore the index:\n%s",
					removed, diffLines(before, got))
			}
		})
	}
}

// A file-level import re-exports into the document root, where a name has no
// enclosing namespace to be dropped along with. Editing away the declaration it
// surfaced must still take the re-export back.
func TestEditingTheTargetOfAFileLevelImportDropsItsReexport(t *testing.T) {
	const importer, target = "a.sysml", "b.sysml"
	docs := map[string]string{
		importer: "public import Src::*;",
		target:   "package Src { part def Kept; part def Dropped; }",
	}
	reused := buildIndex(t, docs)
	if len(reused.LookupQualified("Dropped")) == 0 {
		t.Fatal("a file-level import should surface Dropped at the document root")
	}

	docs[target] = "package Src { part def Kept; }"
	addDoc(t, reused, target, docs[target])
	reused.ExpandWildcardImports()

	if got := len(reused.LookupQualified("Dropped")); got != 0 {
		t.Errorf("Dropped = %d symbols after its declaration was edited away, want 0", got)
	}
	if len(reused.LookupQualified("Kept")) == 0 {
		t.Error("Kept no longer resolves through the file-level import")
	}
	if got, want := indexState(reused), indexState(buildIndex(t, docs)); got != want {
		t.Errorf("editing %s left an index a fresh build would not produce:\n%s",
			target, diffLines(want, got))
	}
}

// Deriving every importer from an empty re-export state — what expansion falls
// back to when its incremental rounds do not settle — has to rebuild exactly what
// was there, including a target that only resolves once another importer has been
// derived, which takes more than one round.
func TestExpandWildcardImportsRederivesEverythingFromScratch(t *testing.T) {
	docs := map[string]string{
		// Aaa's target exists only once Mid, which sorts after it, has re-exported
		// Thing, so deriving Aaa again in a later round is what surfaces Aaa::Deep.
		"a.sysml": "package Mid { public import Lib::*; public import Src::*; } " +
			"package Cycle1 { public import Cycle2::*; } " +
			"package Aaa { public import Mid::Thing::*; }",
		"b.sysml": "package Top { public import Mid::*; } " +
			"package Cycle2 { public import Cycle1::*; public import Lib::*; } " +
			"package Src { package Thing { part def Deep; } }",
	}
	idx := buildIndex(t, docs)
	want := indexState(idx)
	if len(idx.LookupQualified("Aaa::Deep")) == 0 {
		t.Fatal("Aaa::Deep should be surfaced through Mid's re-export of Thing")
	}

	idx.purgeAllReexports()
	if got := len(idx.LookupQualified("Top::Widget")); got != 0 {
		t.Fatalf("Top::Widget = %d symbols after purging re-exports, want 0", got)
	}
	for idx.expandRound(true) {
	}
	if got := indexState(idx); got != want {
		t.Errorf("re-deriving from scratch did not rebuild the index:\n%s", diffLines(want, got))
	}
}

// diffLines reports the lines of want and got that differ, in order.
func diffLines(want, got string) string {
	wantLines, gotLines := strings.Split(want, "\n"), strings.Split(got, "\n")
	inWant := make(map[string]bool, len(wantLines))
	for _, line := range wantLines {
		inWant[line] = true
	}
	inGot := make(map[string]bool, len(gotLines))
	for _, line := range gotLines {
		inGot[line] = true
	}
	var b strings.Builder
	for _, line := range wantLines {
		if line != "" && !inGot[line] {
			fmt.Fprintf(&b, "-%s\n", line)
		}
	}
	for _, line := range gotLines {
		if line != "" && !inWant[line] {
			fmt.Fprintf(&b, "+%s\n", line)
		}
	}
	return b.String()
}

// The names a removed document's wildcard import surfaced stop resolving, which
// is what lets an index be reused across edits instead of rebuilt.
func TestRemoveDocumentDropsItsReexports(t *testing.T) {
	idx := buildIndex(t, map[string]string{
		"a.sysml": "package A { public import Lib::*; }",
	})
	if got := len(idx.LookupQualified("A::Widget")); got != 1 {
		t.Fatalf("A::Widget = %d symbols before removal, want 1", got)
	}

	idx.RemoveDocument("a.sysml")
	for _, fqn := range []string{"A", "A::Widget", "A::Kilogram", "A::kg"} {
		if got := len(idx.LookupQualified(fqn)); got != 0 {
			t.Errorf("%s = %d symbols after removal, want 0", fqn, got)
		}
	}
	if got := idx.WildcardImportsOf("A"); len(got) != 0 {
		t.Errorf("WildcardImportsOf(A) = %v after removal, want none: "+
			"a removed document's imports are not expanded again", got)
	}
	if got := len(idx.LookupQualified("Lib::Widget")); got != 1 {
		t.Errorf("Lib::Widget = %d symbols, want 1: the library was not touched", got)
	}
}

// A re-export two documents surface survives the removal of either one, and the
// declaration site the real declaration owns is never claimed or dropped by a
// re-export of it.
func TestRemoveDocumentKeepsAReexportAnotherDocumentSurfaces(t *testing.T) {
	idx := buildIndex(t, map[string]string{
		"a.sysml": "package Shared { public import Lib::*; }",
		"b.sysml": "package Shared { public import Lib::*; }",
	})
	widget := idx.LookupQualified("Lib::Widget")
	if len(widget) != 1 {
		t.Fatalf("Lib::Widget = %d symbols, want 1", len(widget))
	}
	if got := idx.declaredAt.at(widget[0]); got != "Lib::Widget" {
		t.Fatalf("declaredAt = %q, want Lib::Widget: a re-export is not a declaration", got)
	}

	idx.RemoveDocument("a.sysml")
	if got := len(idx.LookupQualified("Shared::Widget")); got != 1 {
		t.Errorf("Shared::Widget = %d symbols, want 1: b.sysml still imports Lib", got)
	}
	if got := idx.declaredAt.at(widget[0]); got != "Lib::Widget" {
		t.Errorf("declaredAt = %q after removal, want Lib::Widget", got)
	}
}

// A name a public import exported is hidden again once only a private import of
// it remains (KerML 8.2.3.3), so it is visible inside the importing package and
// nowhere else.
func TestRemoveDocumentRehidesAPrivatelyImportedName(t *testing.T) {
	idx := buildIndex(t, map[string]string{
		"a.sysml": "package Shared { public import Lib::*; }",
		"b.sysml": "package Shared { private import Lib::*; }",
	})
	if got := len(idx.LookupQualified("Shared::Widget")); got != 1 {
		t.Fatalf("Shared::Widget = %d symbols, want 1: a.sysml imports Lib publicly", got)
	}

	idx.RemoveDocument("a.sysml")
	if got := len(idx.LookupQualified("Shared::Widget")); got != 0 {
		t.Errorf("Shared::Widget = %d symbols, want 0: only a private import remains", got)
	}
	if got := len(idx.LookupQualifiedFrom("Shared::Widget", "Shared")); got != 1 {
		t.Errorf("LookupQualifiedFrom(Shared::Widget, Shared) = %d, want 1: "+
			"the private import is still visible inside Shared", got)
	}
}
