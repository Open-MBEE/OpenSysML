package model

import "testing"

// Opening a buffer that holds the text of a document the folder scan already
// parsed is not an edit: the index stands, nothing is invalidated, and only the
// document's version and open state move.
func TestOpenOfIndexedTextKeepsTheIndex(t *testing.T) {
	ws := NewWorkspace()
	src := []byte("package P { part def X; }")
	ws.SetOnDisk("a.sysml", src)
	ws.Diagnostics("a.sysml")
	gen := ws.Generation()

	ws.Open("a.sysml", src, 3)
	if ws.Generation() != gen {
		t.Fatalf("opening the indexed text moved the generation from %d to %d", gen, ws.Generation())
	}
	if _, cached := ws.diagCache["a.sysml"]; !cached {
		t.Fatal("opening the indexed text dropped its cached diagnostics")
	}
	doc := ws.Document("a.sysml")
	if doc == nil || doc.Version != 3 || doc.AST == nil {
		t.Fatalf("document after open = %+v, want version 3 with its tree", doc)
	}
	if !ws.IsOpen("a.sysml") {
		t.Fatal("the buffer is not open")
	}
	if syms := ws.LookupQualified("P::X"); len(syms) != 1 {
		t.Fatalf("P::X = %d symbols, want 1", len(syms))
	}

	ws.Open("a.sysml", []byte("package P { part def Y; }"), 4)
	if ws.Generation() == gen {
		t.Fatal("opening other text left the generation alone")
	}
	if syms := ws.LookupQualified("P::Y"); len(syms) != 1 {
		t.Fatalf("P::Y = %d symbols, want 1 from the buffer", len(syms))
	}
}

// A document held as its record is parsed when its text is opened: an open
// buffer answers with its tree, which a record has none of.
func TestOpenOfRecordedTextHydrates(t *testing.T) {
	cache, _ := recordCache(t)
	ws := NewWorkspace(WithRecordCache(cache))
	ws.SetOnDiskAll([]Input{{Name: "base.sysml", Content: hydrateBase}, {Name: "user.sysml", Content: hydrateUser}})
	if !ws.Recorded("base.sysml") {
		t.Fatal("base.sysml is not held as its record on a warm cache")
	}
	ws.Open("base.sysml", hydrateBase, 1)
	if ws.Recorded("base.sysml") {
		t.Fatal("the open buffer is still held as its record")
	}
	if doc := ws.Document("base.sysml"); doc == nil || doc.AST == nil || doc.Version != 1 {
		t.Fatalf("document after open = %+v, want version 1 with its tree", doc)
	}
}

// SetOnDiskAll holds the files as one batch: one invalidation for them all, an
// open buffer left authoritative with its file's content recorded for its close.
func TestSetOnDiskAllHoldsClosedFilesAsOneBatch(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("b.sysml", []byte("package Buf { part def B; }"), 1)
	ws.Diagnostics("b.sysml")
	gen := ws.Generation()

	ws.SetOnDiskAll([]Input{
		{Name: "a.sysml", Content: []byte("package A { part def X; }")},
		{Name: "b.sysml", Content: []byte("package Disk { part def D; }")},
		{Name: "c.sysml", Content: []byte("package C { private import A::*; part x : X; }")},
	})
	if ws.Generation() != gen+1 {
		t.Fatalf("the batch moved the generation from %d to %d, want one step", gen, ws.Generation())
	}
	for _, fqn := range []string{"A::X", "C::x", "Buf::B"} {
		if syms := ws.LookupQualified(fqn); len(syms) != 1 {
			t.Errorf("%s = %d symbols, want 1", fqn, len(syms))
		}
	}
	if syms := ws.LookupQualified("Disk::D"); len(syms) != 0 {
		t.Errorf("Disk::D = %d symbols, want none while the buffer is open", len(syms))
	}
	if ws.IsOpen("a.sysml") || ws.IsOpen("c.sysml") || !ws.IsOpen("b.sysml") {
		t.Fatal("the batch changed which documents are open")
	}
	if diags := ws.Diagnostics("c.sysml"); len(diags) != 0 {
		t.Fatalf("c.sysml reports %v, want A::X resolved from the same batch", messagesOf(diags))
	}

	ws.Close("b.sysml")
	if syms := ws.LookupQualified("Disk::D"); len(syms) != 1 {
		t.Fatalf("Disk::D = %d symbols after the close, want the file's content", len(syms))
	}
}
