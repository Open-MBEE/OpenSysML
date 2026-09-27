package symbols

import "testing"

// The documents that take part in an answer are those declaring a symbol under
// the name and those whose wildcard imports surface one there.
func TestAnswerersNameDeclaringAndReexportingDocuments(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package P { part def Y; }")
	addDoc(t, idx, "b.sysml", "package Q { part def W; }")
	addDoc(t, idx, "c.sysml", "package R { public import P::*; public import Q::*; }")
	idx.ExpandWildcardImports()

	if got := idx.Answerers("P::Y"); !equalStrings(got, []string{"a.sysml"}) {
		t.Fatalf("Answerers(P::Y) = %v, want the declaring document", got)
	}
	if got := idx.Answerers("R::Y"); !equalStrings(got, []string{"a.sysml", "c.sysml"}) {
		t.Fatalf("Answerers(R::Y) = %v, want the declaring and the re-exporting document", got)
	}
	if got := idx.Answerers("P::Nothing"); got != nil {
		t.Fatalf("Answerers over an unknown name = %v, want none", got)
	}
	if got := idx.NamespaceAnswerers("P"); !equalStrings(got, []string{"a.sysml"}) {
		t.Fatalf("NamespaceAnswerers(P) = %v, want the declaring document", got)
	}
	if got := idx.NamespaceAnswerers("R"); !equalStrings(got, []string{"a.sysml", "b.sysml", "c.sysml"}) {
		t.Fatalf("NamespaceAnswerers(R) = %v, want the re-exporting document and the declaring ones", got)
	}

	idx.RemoveDocument("c.sysml")
	if got := idx.Answerers("R::Y"); got != nil {
		t.Fatalf("Answerers(R::Y) after removing the importing document = %v, want none", got)
	}
	addDoc(t, idx, "d.sysml", "package P { part def V; }")
	if got := idx.NamespaceAnswerers("P"); !equalStrings(got, []string{"a.sysml", "d.sysml"}) {
		t.Fatalf("NamespaceAnswerers(P) over two declaring documents = %v", got)
	}
}

// ShortNamed(name) is decided by the documents declaring a symbol registered
// under name that is not its own last segment; a document declaring the name
// itself does not take part.
func TestSegmentAnswerersNameTheShortNamingDocuments(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package P { part def <s> Y; }")
	addDoc(t, idx, "b.sysml", "package Q { part def s; }")
	if got := idx.SegmentAnswerers("s"); !equalStrings(got, []string{"a.sysml"}) {
		t.Fatalf("SegmentAnswerers(s) = %v, want the short-naming document alone", got)
	}
	if got := idx.SegmentAnswerers("Y"); got != nil {
		t.Fatalf("SegmentAnswerers(Y) = %v, want none for a name that is its own segment", got)
	}
	addDoc(t, idx, "c.sysml", "package R { part def <s> W; }")
	if got := idx.SegmentAnswerers("s"); !equalStrings(got, []string{"a.sysml", "c.sysml"}) {
		t.Fatalf("SegmentAnswerers(s) after a second short-naming document = %v", got)
	}
	idx.RemoveDocument("a.sysml")
	if got := idx.SegmentAnswerers("s"); !equalStrings(got, []string{"c.sysml"}) {
		t.Fatalf("SegmentAnswerers(s) after removing one short-naming document = %v", got)
	}
}
