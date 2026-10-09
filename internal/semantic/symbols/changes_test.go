package symbols

import (
	"reflect"
	"sort"
	"testing"
)

// recordedReads collects what an index reports read, as a resolver would.
type recordedReads struct {
	names, namespaces, docs []string
	all                     bool
}

func (r *recordedReads) ReadName(fqn string)        { r.names = append(r.names, fqn) }
func (r *recordedReads) ReadNamespace(fqn string)   { r.namespaces = append(r.namespaces, fqn) }
func (r *recordedReads) ReadDocument(name string)   { r.docs = append(r.docs, name) }
func (r *recordedReads) ReadAllNames()              { r.all = true }
func (r *recordedReads) ReadSegment(name string)    { r.names = append(r.names, name) }
func (r *recordedReads) sorted(s []string) []string { sort.Strings(s); return s }

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestTakeChangesNamesWhatAReplacementMoved(t *testing.T) {
	idx := buildIndex(t, map[string]string{
		"a.sysml": "package P { part def X; }",
		"b.sysml": "package Q { part def Y; }",
	})
	idx.TrackChanges()
	if ch := idx.TakeChanges(); !ch.Empty() {
		t.Fatalf("changes before any write: %v", ch)
	}
	addDoc(t, idx, "a.sysml", "package P { part def Z; }")
	ch := idx.TakeChanges()
	if got := keys(ch.Names); !reflect.DeepEqual(got, []string{"P", "P::X", "P::Z"}) {
		t.Fatalf("Names = %v, want [P P::X P::Z]", got)
	}
	if !ch.Namespaces["P"] || ch.Namespaces["Q"] {
		t.Fatalf("Namespaces = %v, want P and not Q", keys(ch.Namespaces))
	}
	if got := keys(ch.Docs); !reflect.DeepEqual(got, []string{"a.sysml"}) {
		t.Fatalf("Docs = %v, want [a.sysml]", got)
	}
	if ch := idx.TakeChanges(); !ch.Empty() {
		t.Fatalf("changes were not taken: %v", ch)
	}
}

func TestTakeChangesNamesWhatARemovalMoved(t *testing.T) {
	idx := buildIndex(t, map[string]string{
		"a.sysml": "package P { part def X; }",
	})
	idx.TrackChanges()
	idx.RemoveDocument("a.sysml")
	ch := idx.TakeChanges()
	if !ch.Names["P::X"] || !ch.Docs["a.sysml"] {
		t.Fatalf("removal recorded names %v, docs %v; want P::X and a.sysml", keys(ch.Names), keys(ch.Docs))
	}
}

func TestTakeChangesNamesALibraryMarking(t *testing.T) {
	idx := buildIndex(t, map[string]string{"lib.sysml": "package L { part def X; }"})
	idx.TrackChanges()
	idx.MarkLibrary("lib.sysml")
	if ch := idx.TakeChanges(); !ch.Docs["lib.sysml"] {
		t.Fatalf("marking a document as library recorded %v, want the document", ch)
	}
}

func TestReadRecorderHearsWhatIsRead(t *testing.T) {
	idx := buildIndex(t, map[string]string{
		"a.sysml": "package P { part def X; }",
	})
	rec := &recordedReads{}
	idx.SetReadRecorder(rec)
	idx.LookupQualified("P::X")
	idx.LookupQualified("P::Missing")
	idx.DocumentRoot("a.sysml")
	if got := rec.sorted(rec.names); !reflect.DeepEqual(got, []string{"P::Missing", "P::X"}) {
		t.Fatalf("names read = %v, want [P::Missing P::X]", got)
	}
	if got := rec.sorted(rec.docs); !reflect.DeepEqual(got, []string{"a.sysml"}) {
		t.Fatalf("documents read = %v, want [a.sysml]", got)
	}
	if rec.all {
		t.Fatal("a lookup by name reported a scan of the whole name table")
	}
	idx.WorkspaceDocuments()
	if !rec.all {
		t.Fatal("listing the documents did not report a scan of the whole name table")
	}
	idx.SetReadRecorder(nil)
	rec.names = nil
	idx.LookupQualified("P::X")
	if len(rec.names) != 0 {
		t.Fatalf("reads reported after the recorder was removed: %v", rec.names)
	}
}

// A document replaced by one declaring the same names re-registers every name
// its wildcard imports surface; those still name the same symbols, surfaced the
// same way, so none is a change. Its own declarations are new symbols, and are.
func TestTakeChangesLeavesOutWhatIsRegisteredAgainAsItWas(t *testing.T) {
	idx := buildIndex(t, map[string]string{
		"lib.sysml": "package L { part def A; part def B; }",
		"app.sysml": "package P { public import L::*; part def X; }",
	})
	idx.ExpandWildcardImports()
	idx.TrackChanges()
	replace := func(name, src string) {
		addDoc(t, idx, name, src)
		idx.ExpandWildcardImports()
	}
	replace("app.sysml", "package P { public import L::*; part def X; } // edited")
	ch := idx.TakeChanges()
	if ch.Names["P::A"] || ch.Names["P::B"] {
		t.Errorf("re-exports registered again as they were are changes: %v", keys(ch.Names))
	}
	if !ch.Names["P::X"] || !ch.Docs["app.sysml"] {
		t.Errorf("the replacement's own declaration is not a change: names %v, docs %v", keys(ch.Names), keys(ch.Docs))
	}

	// Surfaced privately instead, the same symbols read differently.
	replace("app.sysml", "package P { private import L::*; part def X; }")
	if ch := idx.TakeChanges(); !ch.Names["P::A"] || !ch.Names["P::B"] {
		t.Errorf("re-exports hidden by a private import are not changes: %v", keys(ch.Names))
	}
	// No longer imported, they are gone.
	replace("app.sysml", "package P { part def X; }")
	if ch := idx.TakeChanges(); !ch.Names["P::A"] || !ch.Names["P::B"] {
		t.Errorf("re-exports dropped with their import are not changes: %v", keys(ch.Names))
	}
	// Imported again from a library parsed again, they name new symbols.
	replace("app.sysml", "package P { public import L::*; part def X; }")
	idx.TakeChanges()
	replace("lib.sysml", "package L { part def A; part def B; } // edited")
	if ch := idx.TakeChanges(); !ch.Names["L::A"] || !ch.Names["P::A"] {
		t.Errorf("a library parsed again leaves its declarations and their re-exports unchanged: %v", keys(ch.Names))
	}
}

// A re-export two documents surface outlives the replacement of one of them
// under the other's claim. The replacement's claim is compared with the one it
// held before: the same import is no change, a differently stated one is.
func TestTakeChangesComparesAReplacedClaimBesideTheOnesThatStayed(t *testing.T) {
	idx := buildIndex(t, map[string]string{
		"base.sysml": "package L { metadata def M; part def A; }",
		"a.sysml":    "package P { public import L::*; }",
		"b.sysml":    "package P { public import L::*; }",
	})
	idx.TrackChanges()
	replace := func(src string) Changes {
		addDoc(t, idx, "b.sysml", src)
		idx.ExpandWildcardImports()
		return idx.TakeChanges()
	}
	if ch := replace("package P { public import L::*; } // edited"); ch.Names["P::A"] {
		t.Errorf("a re-export claimed again as it was is a change: %v", keys(ch.Names))
	}
	// Claimed privately beside a public claim, the name stays exported; the
	// claims alone read differently.
	if ch := replace("package P { private import L::*; }"); !ch.Names["P::A"] {
		t.Errorf("a re-export claimed privately where it was public is not a change: %v", keys(ch.Names))
	}
	if ch := replace("package P { private import L::*; } // edited"); ch.Names["P::A"] {
		t.Errorf("a re-export claimed privately again is a change: %v", keys(ch.Names))
	}
	// A second import widens the claim's routes after the first recorded them.
	if ch := replace("package P { private import L::*; public import L::*; }"); !ch.Names["P::A"] {
		t.Errorf("a re-export whose routes widened is not a change: %v", keys(ch.Names))
	}
	if ch := replace("package P { private import L::*; public import L::*; } // edited"); ch.Names["P::A"] {
		t.Errorf("a re-export whose routes were recorded again as they were is a change: %v", keys(ch.Names))
	}
	// A filter's condition is the expression declaring it, so one parsed again
	// is another condition and the route it gates another route.
	if ch := replace("package P { private import L::*[@M]; }"); !ch.Names["P::A"] {
		t.Errorf("a re-export gated where it was not is not a change: %v", keys(ch.Names))
	}
	if ch := replace("package P { private import L::*[@M]; } // edited"); !ch.Names["P::A"] {
		t.Errorf("a re-export gated by a condition parsed again is not a change: %v", keys(ch.Names))
	}
}

// What a registration noted before a write reads is what the index held then,
// whatever the writes since did to the claims under the same key.
func TestNotedRegistrationIsUnmovedByLaterClaims(t *testing.T) {
	idx := buildIndex(t, map[string]string{
		"base.sysml": "package L { part def A; }",
		"a.sysml":    "package P { public import L::*; }",
		"b.sysml":    "package P { private import L::*; }",
	})
	idx.TrackChanges()
	before := registrationOf(idx, "P::A")
	addDoc(t, idx, "b.sysml", "package P { private import L::*; public import L::*; }")
	idx.ExpandWildcardImports()
	if before.same(registrationOf(idx, "P::A")) {
		t.Fatal("the registration noted before b.sysml widened its claim reads as the one after")
	}
	addDoc(t, idx, "b.sysml", "package P { private import L::*; }")
	idx.ExpandWildcardImports()
	if !before.same(registrationOf(idx, "P::A")) {
		t.Fatal("b.sysml claiming as it first did does not read as the registration noted then")
	}
}
