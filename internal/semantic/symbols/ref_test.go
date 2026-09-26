package symbols

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A reference into a scope a record does not keep — a metadata body, a
// control-flow body, a constraint body — could not be restored from the
// record, so no ElementRef names such a member; one into a kept anonymous
// scope round-trips through the recorded scope tree by member ordinal.
func TestRefToRefusesMembersOfUnrecordedScopes(t *testing.T) {
	idx := buildIndex(t, map[string]string{
		"a.sysml": `package A {
	part def D {
		@Lib::Widget { attribute inBody; }
		part anon : D { part deep; }
	}
	action def Act {
		if true { part inBranch; }
	}
}`,
	})
	root := idx.DocumentRoot("a.sysml")
	rec, err := RecordScope(root, func(*Symbol) bool { return true }, func(*Symbol) LibraryFacts { return LibraryFacts{} })
	if err != nil {
		t.Fatal(err)
	}
	recorded, err := BuildRecorded(rec, "a.sysml")
	if err != nil {
		t.Fatal(err)
	}
	other := NewIndex()
	other.AddRecordedDocument("a.sysml", source.KindSysML, recorded, nil)

	var refused, kept int
	var walk func(s *Scope)
	walk = func(s *Scope) {
		for _, sym := range s.members {
			ref, ok := idx.RefTo(sym)
			if !s.inRecord() {
				refused++
				if ok {
					t.Errorf("%s in an unrecorded scope has the reference %+v", FQNOf(sym), ref)
				}
			} else if ok {
				kept++
				if got := other.Element(ref); got == nil || got.Name != sym.Name || FQNOf(got) != FQNOf(sym) {
					t.Errorf("%+v for %s restores %v", ref, FQNOf(sym), got)
				}
			}
		}
		for _, child := range s.children {
			walk(child)
		}
	}
	walk(root)
	if refused < 2 || kept < 4 {
		t.Fatalf("%d refused, %d kept: the fixture exercises neither side", refused, kept)
	}
}

// A reference is written while its target's name may be unique and read into
// an index other documents reached first; it names the declaring document so
// that another declaring the same name does not take it over.
func TestElementRefKeepsItsDocumentUnderADuplicate(t *testing.T) {
	const z = "package P { part def T; part def U :> T; }"
	written := buildIndex(t, map[string]string{"z.sysml": z})
	target := written.LookupQualified("P::T")
	if len(target) != 1 {
		t.Fatalf("P::T: %v", target)
	}
	ref, ok := written.RefTo(target[0])
	if !ok {
		t.Fatal("RefTo(P::T) names nothing")
	}
	read := buildIndex(t, map[string]string{
		"a.sysml": "package P { part def T; }",
		"z.sysml": z,
	})
	if decls := read.LookupQualified("P::T"); len(decls) != 2 {
		t.Fatalf("P::T is declared %d times, want 2", len(decls))
	}
	got := read.Element(ref)
	if got == nil || got.DocName != "z.sysml" || FQNOf(got) != "P::T" {
		t.Fatalf("Element(%+v) = %v, want z.sysml's P::T", ref, got)
	}
}
