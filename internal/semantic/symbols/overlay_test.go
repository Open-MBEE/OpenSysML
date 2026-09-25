package symbols

import (
	"sort"
	"strconv"
	"sync"
	"testing"
)

// baseDoc is the document a frozen base holds in these cases, standing in for
// the bundled library: the documents an overlay adds are the model's own.
const baseDoc = "base.sysml"

// frozenBase indexes docs into one index and freezes it, so overlays can be
// built over it.
func frozenBase(t *testing.T, docs map[string]string) *Index {
	t.Helper()
	idx := buildIndex(t, docs)
	idx.Freeze()
	return idx
}

// overlayOver builds an overlay over base holding docs, expanded to the
// fixpoint, as a model's index over a shared library holds the user's document.
func overlayOver(t *testing.T, base *Index, docs map[string]string) *Index {
	t.Helper()
	idx := NewOverlay(base)
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

// An overlay over a frozen base must answer exactly as an index built whole over
// the same documents — every registered name, where each came from, the child
// links and the imports still driving expansion. That equality is what makes one
// shared library index sound, so it is asserted over the import shapes whose
// re-exports cross the base/overlay boundary in either direction.
func TestOverlayEqualsAnIndexBuiltWhole(t *testing.T) {
	cases := []struct {
		name string
		base string
		user string
	}{
		{
			name: "user imports the base",
			base: "package Lib2 { part def Widget; }",
			user: "package User { public import Lib2::*; }",
		},
		{
			name: "base imports a namespace the user declares",
			base: "package Mid { public import Late::*; }",
			user: "package Late { part def Arrived; }",
		},
		{
			name: "chain from base through user back into base",
			base: "package A { public import User::*; } package C { part def Deep; }",
			user: "package User { public import C::*; }",
		},
		{
			name: "private import in the base, public in the user",
			base: "package Mid { private import Lib::*; }",
			user: "package Top { public import Mid::*; public import Lib::*; }",
		},
		{
			name: "cycle across the boundary",
			base: "package A { public import B::*; public import Lib::*; }",
			user: "package B { public import A::*; }",
		},
		{
			name: "user re-declares a namespace the base declares",
			base: "package Shared { part def FromBase; } package P { public import Shared::*; }",
			user: "package Shared { part def FromUser; }",
		},
		{
			name: "document-root import in the user",
			base: "package Lib2 { part def Widget; }",
			user: "public import Lib2::*;",
		},
		{
			name: "nested importer over the boundary",
			base: "package Outer { package Inner { public import User::*; } }",
			user: "package User { part def Own; }",
		},
		{
			name: "filtered import of base content",
			base: "package Lib2 { part def Widget; }",
			user: "package User { public import Lib2::*[@Safety]; }",
		},
	}

	const userDoc = "user.sysml"
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			docs := map[string]string{baseDoc: tc.base, userDoc: tc.user}
			over := overlayOver(t, frozenBase(t, map[string]string{baseDoc: tc.base}),
				map[string]string{userDoc: tc.user})
			if got, want := indexState(over), indexState(buildIndex(t, docs)); got != want {
				t.Errorf("an overlay differs from an index built whole:\n%s", diffLines(want, got))
			}

			// Expanding again changes nothing: the overlay reached the fixpoint.
			over.ExpandWildcardImports()
			if got, want := indexState(over), indexState(buildIndex(t, docs)); got != want {
				t.Errorf("expanding the overlay again changed it:\n%s", diffLines(want, got))
			}
		})
	}
}

// An overlay must answer every lookup route as an index built whole does, not
// only the state the two agree on: a read that consulted a base table directly
// would still pass a state comparison while returning a base document's answer.
func TestOverlayAnswersEveryLookupAsAnIndexBuiltWhole(t *testing.T) {
	docs := map[string]string{
		baseDoc: "package Shared { part def FromBase; } package P { public import Shared::*; } " +
			"package Filtered { public import Shared::*[@Safety]; }",
		"user.sysml": "package User { public import P::*; private import Shared::*; } " +
			"public import P::*;",
	}
	base := frozenBase(t, map[string]string{baseDoc: docs[baseDoc]})
	over := overlayOver(t, base, map[string]string{"user.sysml": docs["user.sysml"]})
	whole := buildIndex(t, docs)

	if got, want := len(over.FQNs()), len(whole.FQNs()); got != want {
		t.Fatalf("overlay names %d FQNs, want %d", got, want)
	}
	froms := append([]string{"", "User", "P", "Shared", "Filtered"}, over.FQNs()...)
	for _, fqn := range over.FQNs() {
		if got, want := symNames(over.LookupQualified(fqn)), symNames(whole.LookupQualified(fqn)); !equalStrings(got, want) {
			t.Errorf("LookupQualified(%q) = %v, want %v", fqn, got, want)
		}
		if got, want := symNames(over.LookupDirectChildren(fqn)), symNames(whole.LookupDirectChildren(fqn)); !equalStrings(got, want) {
			t.Errorf("LookupDirectChildren(%q) = %v, want %v", fqn, got, want)
		}
		for _, from := range froms {
			if got, want := symNames(over.LookupQualifiedFrom(fqn, from)), symNames(whole.LookupQualifiedFrom(fqn, from)); !equalStrings(got, want) {
				t.Errorf("LookupQualifiedFrom(%q, %q) = %v, want %v", fqn, from, got, want)
			}
			if got, want := symNames(over.LookupDirectChildrenFrom(fqn, from)), symNames(whole.LookupDirectChildrenFrom(fqn, from)); !equalStrings(got, want) {
				t.Errorf("LookupDirectChildrenFrom(%q, %q) = %v, want %v", fqn, from, got, want)
			}
		}
		mine, theirs := sortedSyms(over, fqn), sortedSyms(whole, fqn)
		if len(mine) != len(theirs) {
			t.Errorf("%q names %d symbols, want %d", fqn, len(mine), len(theirs))
			continue
		}
		for i := range mine {
			if over.Library(mine[i]) != whole.Library(theirs[i]) {
				t.Errorf("Library(%s under %q) = %v, want %v",
					mine[i].Name, fqn, over.Library(mine[i]), whole.Library(theirs[i]))
			}
		}
		if got, want := over.NamespaceFiltersOf(fqn), whole.NamespaceFiltersOf(fqn); len(got) != len(want) {
			t.Errorf("NamespaceFiltersOf(%q) = %d filters, want %d", fqn, len(got), len(want))
		}
	}
	for _, doc := range []string{baseDoc, "user.sysml"} {
		if got, want := bindingNames(over.TopLevelBindings(doc)), bindingNames(whole.TopLevelBindings(doc)); !equalStrings(got, want) {
			t.Errorf("TopLevelBindings(%q) = %v, want %v", doc, got, want)
		}
		for _, fqn := range over.FQNs() {
			mine, theirs := sortedSyms(over, fqn), sortedSyms(whole, fqn)
			if len(mine) != len(theirs) {
				continue // already reported
			}
			for i := range mine {
				if got, want := over.ReexportVisible(doc, fqn, mine[i]), whole.ReexportVisible(doc, fqn, theirs[i]); got != want {
					t.Errorf("ReexportVisible(%q, %q, %s) = %v, want %v", doc, fqn, mine[i].Name, got, want)
				}
				if got, want := len(over.ReexportGates(doc, fqn, mine[i], "")), len(whole.ReexportGates(doc, fqn, theirs[i], "")); got != want {
					t.Errorf("ReexportGates(%q, %q, %s) = %d routes, want %d", doc, fqn, mine[i].Name, got, want)
				}
			}
		}
	}
}

// A base document a model removes — the library file an eviction drops, or a
// library type an edit shadows — has to leave the overlay as a fresh build over
// what remains, while the base keeps it for every other model.
func TestOverlayRemovalOfABaseDocumentLeavesTheBaseIntact(t *testing.T) {
	docs := map[string]string{
		baseDoc:      "package Shared { part def FromBase; } package P { public import Shared::*; }",
		"user.sysml": "package User { public import P::*; }",
	}
	base := frozenBase(t, map[string]string{baseDoc: docs[baseDoc]})
	before := indexState(base)

	over := overlayOver(t, base, map[string]string{"user.sysml": docs["user.sysml"]})
	over.RemoveDocument(baseDoc)
	fresh := buildIndex(t, map[string]string{"user.sysml": docs["user.sysml"]})
	if got, want := indexState(over), indexState(fresh); got != want {
		t.Errorf("removing a base document left an index a fresh build would not produce:\n%s",
			diffLines(want, got))
	}
	if got := indexState(base); got != before {
		t.Errorf("removing a base document through an overlay changed the base:\n%s",
			diffLines(before, got))
	}

	// Another model over the same base still resolves what the first removed.
	other := overlayOver(t, base, map[string]string{"user.sysml": docs["user.sysml"]})
	if got := len(other.LookupQualified("User::FromBase")); got != 1 {
		t.Errorf("User::FromBase = %d symbols in another model, want 1", got)
	}
}

// A user document declaring a name the library also declares makes an import
// target ambiguous, which stops the re-exports that came by it — the one case
// where a model has to subtract from what the base derived. Removing the
// document restores them, in that model alone.
func TestOverlaySuppressesAnAmbiguatedBaseImport(t *testing.T) {
	base := frozenBase(t, map[string]string{
		baseDoc: "package Shared { part def FromBase; } package P { public import Shared::*; }",
	})
	if got := len(base.LookupQualified("P::FromBase")); got != 1 {
		t.Fatalf("P::FromBase = %d symbols in the base, want 1", got)
	}

	over := NewOverlay(base)
	addDoc(t, over, "user.sysml", "package Shared { part def FromUser; }")
	over.ExpandWildcardImports()
	if got := len(over.LookupQualified("P::FromBase")); got != 0 {
		t.Errorf("P::FromBase = %d symbols while Shared is ambiguous, want 0", got)
	}
	if got := len(base.LookupQualified("P::FromBase")); got != 1 {
		t.Errorf("P::FromBase = %d symbols in the base, want 1: one model cannot change it", got)
	}

	over.RemoveDocument("user.sysml")
	if got := len(over.LookupQualified("P::FromBase")); got != 1 {
		t.Errorf("P::FromBase = %d symbols once Shared is unambiguous again, want 1", got)
	}
	if got, want := indexState(over), indexState(base); got != want {
		t.Errorf("the overlay does not answer as its base once its document is gone:\n%s",
			diffLines(want, got))
	}
}

// Two models sharing one library index must not see each other's documents,
// concurrently or not: they write into their own overlay, and read the base.
func TestConcurrentOverlaysDoNotSeeEachOther(t *testing.T) {
	base := frozenBase(t, map[string]string{
		baseDoc: "package Shared { part def FromBase; } package P { public import Shared::*; }",
	})
	before := indexState(base)

	const models = 8
	var wg sync.WaitGroup
	errs := make(chan string, models*4)
	for i := 0; i < models; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			me := strconv.Itoa(i)
			over := NewOverlay(base)
			addDoc(t, over, docNameOf(i),
				"package Mine"+me+" { public import P::*; part def Own"+me+"; }")
			over.ExpandWildcardImports()

			if len(over.LookupQualified("Mine"+me+"::FromBase")) != 1 {
				errs <- "model " + me + " does not see the base's name through its own import"
			}
			for j := 0; j < models; j++ {
				if j == i {
					continue
				}
				other := strconv.Itoa(j)
				if len(over.LookupQualified("Mine"+other+"::Own"+other)) != 0 {
					errs <- "model " + me + " sees model " + other + "'s document"
				}
				if over.DocumentRoot(docNameOf(j)) != nil {
					errs <- "model " + me + " holds model " + other + "'s document root"
				}
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for msg := range errs {
		t.Error(msg)
	}
	if got := indexState(base); got != before {
		t.Errorf("the models changed the base they share:\n%s", diffLines(before, got))
	}
}

// A frozen index is shared by every model over it, so a write to it would be a
// write to all of them: it panics rather than corrupting them.
func TestFrozenIndexRejectsWrites(t *testing.T) {
	base := frozenBase(t, map[string]string{baseDoc: "package Lib2 { part def Widget; }"})
	for _, tc := range []struct {
		name  string
		write func()
	}{
		{"AddDocument", func() { addDoc(t, base, "user.sysml", "package User;") }},
		{"RemoveDocument", func() { base.RemoveDocument(baseDoc) }},
		{"MarkLibrary", func() { base.MarkLibrary(baseDoc) }},
		{"ExpandWildcardImports", func() { base.ExpandWildcardImports() }},
		{"SetNamespaceFilters", func() { base.SetNamespaceFilters("Lib2", "user.sysml", nil) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("%s on a frozen index did not panic", tc.name)
				}
			}()
			tc.write()
		})
	}
}

// Freezing settles the index first, so a base is at the fixpoint its overlays
// read, and freezing twice is a no-op.
func TestFreezeSettlesTheIndex(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, libDoc, libSource)
	idx.MarkLibrary(libDoc)
	addDoc(t, idx, baseDoc, "package P { public import Lib::*; }")
	idx.Freeze()
	if !idx.Frozen() {
		t.Fatal("Freeze did not freeze the index")
	}
	idx.Freeze() // no-op, and must not panic on its own expansion
	if got := len(idx.LookupQualified("P::Widget")); got != 1 {
		t.Errorf("P::Widget = %d symbols in a frozen base, want 1: Freeze expands first", got)
	}
}

// sortedSyms returns the symbols under fqn in name order, so the same name in
// two indexes can be compared: a symbol's identity differs between them.
func sortedSyms(idx *Index, fqn string) []*Symbol {
	syms := append([]*Symbol(nil), idx.LookupQualified(fqn)...)
	sort.Slice(syms, func(i, j int) bool { return syms[i].Name < syms[j].Name })
	return syms
}

// symNames renders the symbols a lookup returned, so two indexes can be compared
// without depending on symbol identity.
func symNames(syms []*Symbol) []string {
	out := make([]string, 0, len(syms))
	for _, sym := range syms {
		out = append(out, sym.Name)
	}
	sort.Strings(out)
	return out
}

// bindingNames renders the root bindings of a document in a comparable form.
func bindingNames(bindings []RootBinding) []string {
	out := make([]string, 0, len(bindings))
	for _, b := range bindings {
		out = append(out, b.Name+"="+b.Sym.Name)
	}
	sort.Strings(out)
	return out
}

func equalStrings(a, b []string) bool {
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

func docNameOf(i int) string { return "user" + strconv.Itoa(i) + ".sysml" }
