package model

import (
	"fmt"
	"sort"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

const refIndexSrc = "package Shapes { part def Cube; alias Box for Cube; part p : Box; }\n" +
	"package Uses { part b : Shapes::Box; part c : Shapes::Cube; }\n"

func oneSymbol(t *testing.T, ws *Workspace, fqn string) *symbols.Symbol {
	t.Helper()
	syms := ws.LookupQualified(fqn)
	if len(syms) != 1 {
		t.Fatalf("%s = %d symbols, want 1", fqn, len(syms))
	}
	return syms[0]
}

func spanTexts(locs []ReferenceLocation) []string {
	out := make([]string, len(locs))
	for i, l := range locs {
		out[i] = fmt.Sprintf("%s@%d", l.Content[l.Span.Offset:l.Span.End()], l.Span.Offset)
	}
	return out
}

// A segment is listed under the element it reaches and under the name it writes:
// an alias use counts for both alias and target, but only the alias for renaming.
func TestReferenceIndexListsBothIdentities(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("shapes.sysml", []byte(refIndexSrc), 1)
	cube, box, shapes := oneSymbol(t, ws, "Shapes::Cube"), oneSymbol(t, ws, "Shapes::Box"), oneSymbol(t, ws, "Shapes")

	want := func(got []ReferenceLocation, texts ...string) {
		t.Helper()
		if g := fmt.Sprint(spanTexts(got)); g != fmt.Sprint(texts) {
			t.Errorf("got %s, want %v", g, texts)
		}
	}
	want(ws.ReferencesTo(cube), "Cube@46", "Box@61", "Box@100", "Cube@122")
	want(ws.NameReferencesTo(cube, "Cube"), "Cube@46", "Cube@122")
	want(ws.ReferencesTo(box), "Box@61", "Box@100")
	want(ws.NameReferencesTo(box, "Box"), "Box@61", "Box@100")
	want(ws.ReferencesTo(shapes), "Shapes@92", "Shapes@114")
	if ws.ReferencesTo(nil) != nil || ws.NameReferencesTo(nil, "Cube") != nil {
		t.Error("nil target should list nothing")
	}
}

// A short-named element is reached by both spellings, but each name is written
// only where it is spelled: renaming one leaves references using the other alone.
func TestReferenceIndexTellsShortNameFromLongName(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("short.sysml", []byte("package P { part def <O> Old { part x; } part a : O; part b : Old; part c : P::O::x; }\n"), 1)
	old := oneSymbol(t, ws, "P::Old")

	want := func(got []ReferenceLocation, texts ...string) {
		t.Helper()
		if g := fmt.Sprint(spanTexts(got)); g != fmt.Sprint(texts) {
			t.Errorf("got %s, want %v", g, texts)
		}
	}
	want(ws.ReferencesTo(old), "O@50", "Old@62", "O@79")
	want(ws.NameReferencesTo(old, "Old"), "Old@62")
	want(ws.NameReferencesTo(old, "O"), "O@50", "O@79")
	want(ws.NameReferencesTo(old, "Fresh"))
}

// Locations come in document-name then position order, addressing the text the
// index was built from.
func TestReferenceIndexOrdersAcrossDocuments(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("b.sysml", []byte("package B { import A::*; part y : X; part z : A::X; }"), 1)
	ws.Open("a.sysml", []byte("package A { part def X; part x : X; }"), 1)
	locs := ws.ReferencesTo(oneSymbol(t, ws, "A::X"))
	var got []string
	for _, l := range locs {
		got = append(got, l.Doc+":"+string(l.Content[l.Span.Offset:l.Span.End()]))
	}
	if fmt.Sprint(got) != "[a.sysml:X b.sysml:X b.sysml:X]" {
		t.Fatalf("locations = %v", got)
	}
}

// The index is built on the first query, one table per document; a mutation
// drops the tables of the documents it moved and keeps the rest, so a query
// never reads a document set it was not built over.
func TestReferenceIndexRebuiltLazilyAfterChanges(t *testing.T) {
	ws := NewWorkspace()
	ws.SetOnDisk("a.sysml", []byte("package A { part def X; }"))
	ws.Open("b.sysml", []byte("package B { part y : A::X; }"), 1)
	if ws.refs != nil {
		t.Fatal("index built before any query")
	}
	x := oneSymbol(t, ws, "A::X")
	if n := len(ws.ReferencesTo(x)); n != 1 || ws.refs == nil {
		t.Fatalf("references = %d (index %v), want 1 and a built index", n, ws.refs != nil)
	}
	ws.SetConformanceMode(ws.ConformanceMode())
	if ws.refs == nil {
		t.Fatal("an unchanged mode dropped the index")
	}
	tables := func() []string {
		var out []string
		if ws.refs != nil {
			for doc := range ws.refs.docs {
				out = append(out, doc)
			}
		}
		sort.Strings(out)
		return out
	}
	for _, step := range []struct {
		name   string
		mutate func()
		kept   []string
		want   int
	}{
		{"Update", func() { ws.Update("b.sysml", []byte("package B { part y : A::X; part z : A::X; }"), 2) }, []string{"a.sysml"}, 2},
		{"SetOnDisk", func() { ws.SetOnDisk("c.sysml", []byte("package C { part w : A::X; }")) }, []string{"a.sysml", "b.sysml"}, 3},
		{"Close", func() { ws.Close("b.sysml") }, []string{"a.sysml", "c.sysml"}, 1},
		{"SetConformanceMode", func() { ws.SetConformanceMode(diag.ConformanceStrict) }, nil, 1},
		{"DeleteOnDisk", func() { ws.DeleteOnDisk("c.sysml") }, []string{"a.sysml"}, 0},
		{"Remove", func() { ws.Remove("a.sysml") }, nil, 0},
	} {
		if ws.refs == nil {
			t.Fatalf("%s: index not built by the query before it", step.name)
		}
		step.mutate()
		if got := tables(); fmt.Sprint(got) != fmt.Sprint(step.kept) {
			t.Fatalf("%s: tables kept across the change = %v, want %v", step.name, got, step.kept)
		}
		if n := len(ws.ReferencesTo(x)); n != step.want {
			t.Fatalf("%s: references = %d, want %d", step.name, n, step.want)
		}
	}
	if n := len(ws.ReferencesTo(x)); n != 0 {
		t.Fatalf("references after removing the declaring document = %d, want 0", n)
	}
}

// Queries and edits interleave freely; run under -race this pins the locking.
func TestReferenceIndexConcurrentQueriesAndUpdates(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("a.sysml", []byte("package A { part def X; }"), 1)
	for i := 0; i < 8; i++ {
		ws.Open(fmt.Sprintf("u%d.sysml", i), []byte(fmt.Sprintf("package U%d { part y : A::X; }", i)), 1)
	}
	x := oneSymbol(t, ws, "A::X")
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(2)
		go func(g int) {
			defer wg.Done()
			for v := 2; v < 30; v++ {
				ws.Update(fmt.Sprintf("u%d.sysml", g), []byte(fmt.Sprintf("package U%d { part y : A::X; part z%d : A::X; }", g, v)), v)
			}
		}(g)
		go func() {
			defer wg.Done()
			for v := 0; v < 30; v++ {
				if n := len(ws.ReferencesTo(x)); n < 8 || n > 12 {
					t.Errorf("references = %d, want between 8 and 12", n)
				}
				ws.NameReferencesTo(x, "X")
			}
		}()
	}
	wg.Wait()
}
