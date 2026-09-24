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
