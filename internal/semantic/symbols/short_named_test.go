package symbols

import "testing"

func TestShortNamedFollowsIndexGeneration(t *testing.T) {
	idx := NewIndex()
	if idx.ShortNamed("s") {
		t.Fatal("ShortNamed(s) = true on an empty index")
	}
	if idx.ShortNamed("s") {
		t.Fatal("ShortNamed(s) changed on a repeated call of an unchanged index")
	}

	addDoc(t, idx, "a.sysml", "package P { part def <s> Y; }")
	if !idx.ShortNamed("s") {
		t.Fatal("ShortNamed(s) = false after adding a short-named member (stale cache)")
	}
	if !idx.ShortNamed("s") {
		t.Fatal("ShortNamed(s) changed on a repeated call of an unchanged index")
	}

	addDoc(t, idx, "a.sysml", "package P { part def Z; }")
	if idx.ShortNamed("s") {
		t.Fatal("ShortNamed(s) = true after the document dropped the short name (stale cache)")
	}

	idx.RemoveDocument("a.sysml")
	if idx.ShortNamed("s") {
		t.Fatal("ShortNamed(s) = true after removing the document (stale cache)")
	}
}

// The names ending in a segment are sorted and duplicate-free however many
// documents register them, and a re-export or a removal is seen at once.
func TestFQNsEndingInAcrossDocuments(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "b.sysml", "package Q { part def X; } package B { import Q::*; }")
	addDoc(t, idx, "a.sysml", "package P { part def X; }")
	addDoc(t, idx, "c.sysml", "package P { part def X; }")
	idx.ExpandWildcardImports()
	want := []string{"B::X", "P::X", "Q::X"}
	if got := idx.FQNsEndingIn("X", 10); !equalStrings(got, want) {
		t.Fatalf("FQNsEndingIn(X) = %v, want %v", got, want)
	}
	idx.RemoveDocument("a.sysml")
	if got := idx.FQNsEndingIn("X", 10); !equalStrings(got, want) {
		t.Fatalf("FQNsEndingIn(X) after removing one of P::X's documents = %v", got)
	}
	idx.RemoveDocument("c.sysml")
	if got := idx.FQNsEndingIn("X", 10); !equalStrings(got, []string{"B::X", "Q::X"}) {
		t.Fatalf("FQNsEndingIn(X) after removing P::X = %v", got)
	}
	idx.RemoveDocument("b.sysml")
	if got := idx.FQNsEndingIn("X", 10); len(got) != 0 {
		t.Fatalf("FQNsEndingIn(X) over nothing = %v", got)
	}
}

// The declared names ending in a segment leave out an import's re-export: the declaring
// namespace, under the name or short name, and not every package that imports it.
func TestDeclaredFQNsEndingInLeaveOutReExports(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package P { view def <gv> General; } package B { private import P::*; }")
	idx.ExpandWildcardImports()
	if got := idx.FQNsEndingIn("gv", 10); !equalStrings(got, []string{"B::gv", "P::gv"}) {
		t.Fatalf("FQNsEndingIn(gv) = %v, want the re-exports too", got)
	}
	if got := idx.DeclaredFQNsEndingIn("gv", 10); !equalStrings(got, []string{"P::gv"}) {
		t.Fatalf("DeclaredFQNsEndingIn(gv) = %v, want [P::gv]", got)
	}
	if got := idx.DeclaredFQNsEndingIn("General", 10); !equalStrings(got, []string{"P::General"}) {
		t.Fatalf("DeclaredFQNsEndingIn(General) = %v, want [P::General]", got)
	}
}
