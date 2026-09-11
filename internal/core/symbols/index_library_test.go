package symbols

import "testing"

// A suggestion prefers the user's own declarations, so the index has to say
// which symbols came from bundled library content: what the library document
// declares, not what it merely re-exports, and only while it is loaded.
func TestLibraryProvenance(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "lib.sysml", "package Lib { part def Widget; }")
	addDoc(t, idx, "shared.sysml", "package Shared { public import Lib::*; }")
	addDoc(t, idx, "user.sysml", "package Mine { part def Gadget; }")
	idx.ExpandWildcardImports()
	idx.MarkLibrary("lib.sysml")

	widget := declaring(t, idx, "Lib::Widget")
	if !idx.Library(widget) {
		t.Errorf("Lib::Widget is not marked library, want marked")
	}
	if gadget := declaring(t, idx, "Mine::Gadget"); idx.Library(gadget) {
		t.Errorf("Mine::Gadget is marked library, want the user's own")
	}
	if idx.Library(nil) {
		t.Errorf("Library(nil) = true, want false")
	}

	// Shared re-exports Widget rather than declaring it, so removing Shared
	// leaves the declaration — and its provenance — alone.
	idx.RemoveDocument("shared.sysml")
	if !idx.Library(widget) {
		t.Errorf("Lib::Widget lost its library mark when a re-exporting document was removed")
	}

	idx.RemoveDocument("lib.sysml")
	if idx.Library(widget) {
		t.Errorf("Lib::Widget is still marked library after its document was removed")
	}
}

// HasLibrary follows the library marks: none until a document is marked, none again
// once every marked document is unmarked or removed.
func TestHasLibrary(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "lib.sysml", "package Lib { part def Widget; }")
	addDoc(t, idx, "user.sysml", "package Mine { part def Gadget; }")
	if idx.HasLibrary() {
		t.Fatal("HasLibrary() = true before any document is marked library")
	}
	idx.MarkLibrary("lib.sysml")
	if !idx.HasLibrary() {
		t.Fatal("HasLibrary() = false with lib.sysml marked library")
	}
	idx.MarkLibraryTier("lib.sysml", TierNone)
	if idx.HasLibrary() {
		t.Fatal("HasLibrary() = true after lib.sysml was unmarked")
	}
	idx.MarkLibrary("lib.sysml")
	idx.RemoveDocument("lib.sysml")
	if idx.HasLibrary() {
		t.Fatal("HasLibrary() = true after the only library document was removed")
	}
}

// declaring returns the symbol fqn declares, failing the test when there is none.
func declaring(t *testing.T, idx *Index, fqn string) *Symbol {
	t.Helper()
	sym := idx.Declaring(fqn)
	if sym == nil {
		t.Fatalf("%s declares nothing", fqn)
	}
	return sym
}
