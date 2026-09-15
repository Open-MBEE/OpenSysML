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
