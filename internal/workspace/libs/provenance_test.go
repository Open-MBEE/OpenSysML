package libs

import (
	"errors"
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// provenanceWorkspace is an index over user documents with their digests, the
// Sources a Provenance is attributed and checked against.
type provenanceWorkspace struct {
	idx      *symbols.Index
	digests  map[string]string
	shared   map[string][]string
	standIns map[string]string
}

func newProvenanceWorkspace(t *testing.T, docs map[string]string) *provenanceWorkspace {
	t.Helper()
	w := &provenanceWorkspace{idx: symbols.NewIndex(), digests: map[string]string{}, shared: map[string][]string{}, standIns: map[string]string{}}
	for name, src := range docs {
		w.add(t, name, src)
	}
	w.idx.ExpandWildcardImports()
	return w
}

func (w *provenanceWorkspace) add(t *testing.T, name, src string) {
	t.Helper()
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected parse diagnostics for %s: %v", name, p.Diagnostics)
	}
	w.idx.AddDocument(name, root)
	w.digests[name] = "digest of " + src
}

func (w *provenanceWorkspace) sources() Sources {
	return NewSources(w.idx,
		func(name string) ([]string, bool) { docs, ok := w.shared[name]; return docs, ok },
		func(doc string) (string, bool) { d, ok := w.digests[doc]; return d, ok },
		func(doc string) string { return w.standIns[doc] })
}

func TestAttributeNamesTheAnsweringDocumentsWithTheirDigests(t *testing.T) {
	w := newProvenanceWorkspace(t, map[string]string{
		"a.sysml": "package P { part def Y; part def <s> Z; }",
		"b.sysml": "package Q { part def W; }",
		"c.sysml": "package R { public import P::*; public import Q::*; }",
		"d.sysml": "package Other { part def Q; }",
	})
	reads := resolve.Reads{
		Names:      []string{"P::Y", "R::Y", "P::Nothing"},
		Namespaces: []string{"P", "R"},
		Segments:   []string{"s", "Q"},
		Docs:       []string{"d.sysml"},
	}
	p, err := Attribute(reads, w.sources())
	if err != nil {
		t.Fatalf("Attribute: %v", err)
	}
	wantAnswerers := map[string][]string{
		"n:P::Y":    {"a.sysml"},
		"n:R::Y":    {"a.sysml", "c.sysml"},
		"s:P":       {"a.sysml"},
		"s:R":       {"a.sysml", "b.sysml", "c.sysml"},
		"g:s":       {"a.sysml"},
		"d:d.sysml": {"d.sysml"},
	}
	if !reflect.DeepEqual(p.Answerers, wantAnswerers) {
		t.Fatalf("Answerers = %v, want %v", p.Answerers, wantAnswerers)
	}
	wantDigests := map[string]string{
		"a.sysml": w.digests["a.sysml"], "b.sysml": w.digests["b.sysml"],
		"c.sysml": w.digests["c.sysml"], "d.sysml": w.digests["d.sysml"],
	}
	if !reflect.DeepEqual(p.Digests, wantDigests) {
		t.Fatalf("Digests = %v, want %v", p.Digests, wantDigests)
	}
	if !reflect.DeepEqual(p.Reads, reads) {
		t.Fatalf("Reads = %+v, want the reads attributed", p.Reads)
	}
	reads.Names[0] = "changed"
	if p.Reads.Names[0] != "P::Y" {
		t.Fatal("Attribute shares the caller's reads")
	}
}

func TestProvenanceHoldsWhileTheAnsweringDocumentsStand(t *testing.T) {
	docs := map[string]string{
		"a.sysml": "package P { part def Y; }",
		"c.sysml": "package R { public import P::*; }",
		"d.sysml": "package Other { part def Q; }",
	}
	reads := resolve.Reads{Names: []string{"P::Y", "R::Y"}, Namespaces: []string{"P"}, Segments: []string{"Y"}}
	w := newProvenanceWorkspace(t, docs)
	p, err := Attribute(reads, w.sources())
	if err != nil {
		t.Fatalf("Attribute: %v", err)
	}
	if !p.Valid(w.sources()) {
		t.Fatal("a provenance does not hold over the state it was attributed against")
	}
	if !p.Valid(newProvenanceWorkspace(t, docs).sources()) {
		t.Fatal("a provenance does not hold over another index of the same documents")
	}

	cases := map[string]func(w *provenanceWorkspace){
		"an answering document changed": func(w *provenanceWorkspace) {
			w.digests["a.sysml"] = "other bytes"
		},
		"the importing document changed": func(w *provenanceWorkspace) {
			w.digests["c.sysml"] = "other bytes"
		},
		"a new document answers the name": func(w *provenanceWorkspace) {
			w.add(t, "e.sysml", "package P { part def Y; }")
			w.idx.ExpandWildcardImports()
		},
		"a new document adds a member of the namespace": func(w *provenanceWorkspace) {
			w.add(t, "e.sysml", "package P { part def Z; }")
			w.idx.ExpandWildcardImports()
		},
		"a new document short-names the segment": func(w *provenanceWorkspace) {
			w.add(t, "e.sysml", "package S { part def <Y> Long; }")
			w.idx.ExpandWildcardImports()
		},
		"an answering document was removed": func(w *provenanceWorkspace) {
			w.idx.RemoveDocument("a.sysml")
			delete(w.digests, "a.sysml")
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			w := newProvenanceWorkspace(t, docs)
			change(w)
			if p.Valid(w.sources()) {
				t.Fatal("the provenance still holds")
			}
		})
	}
	t.Run("an unrelated document changed", func(t *testing.T) {
		w := newProvenanceWorkspace(t, docs)
		w.digests["d.sysml"] = "other bytes"
		w.add(t, "e.sysml", "package Elsewhere { part def Y; }")
		w.idx.ExpandWildcardImports()
		if !p.Valid(w.sources()) {
			t.Fatal("a change to a document no read named did not hold")
		}
	})
	if (*Provenance)(nil).Valid(w.sources()) {
		t.Fatal("a nil provenance holds")
	}
}

func TestProvenanceOverSharedStateFollowsItsContributors(t *testing.T) {
	docs := map[string]string{
		"a.sysml": "package P { part def Y; }",
		"b.sysml": "package Q { part def Z; }",
	}
	w := newProvenanceWorkspace(t, docs)
	w.shared["\x00mosa/present/P::Y"] = []string{"a.sysml"}
	reads := resolve.Reads{Names: []string{"\x00mosa/present/P::Y"}}
	p, err := Attribute(reads, w.sources())
	if err != nil {
		t.Fatalf("Attribute: %v", err)
	}
	if got := p.Answerers["n:\x00mosa/present/P::Y"]; !reflect.DeepEqual(got, []string{"a.sysml"}) {
		t.Fatalf("Answerers of the shared read = %v, want its contributors", got)
	}
	if !p.Valid(w.sources()) {
		t.Fatal("the provenance does not hold over the state it was attributed against")
	}
	w.shared["\x00mosa/present/P::Y"] = []string{"a.sysml", "b.sysml"}
	if p.Valid(w.sources()) {
		t.Fatal("the provenance holds after another document contributes to the shared state")
	}
	delete(w.shared, "\x00mosa/present/P::Y")
	if p.Valid(w.sources()) {
		t.Fatal("the provenance holds where nothing answers for the shared state")
	}
	if _, err := Attribute(reads, w.sources()); !errors.Is(err, ErrUnrecordable) {
		t.Fatalf("Attribute over a shared read nothing answers for = %v, want ErrUnrecordable", err)
	}
	if _, err := Attribute(resolve.Reads{Docs: []string{"\x00oosem/methods"}}, w.sources()); !errors.Is(err, ErrUnrecordable) {
		t.Fatalf("Attribute over a shared document nothing answers for = %v, want ErrUnrecordable", err)
	}
}

func TestProvenanceCloneSharesNothing(t *testing.T) {
	p := &Provenance{
		Reads:     resolve.Reads{Names: []string{"P::Y"}, Docs: []string{"d.sysml"}},
		Answerers: map[string][]string{"n:P::Y": {"a.sysml"}},
		Digests:   map[string]string{"a.sysml": "x"},
	}
	c := p.Clone()
	if !reflect.DeepEqual(c, p) {
		t.Fatalf("Clone = %+v, want %+v", c, p)
	}
	c.Reads.Names[0] = "changed"
	c.Answerers["n:P::Y"][0] = "changed"
	c.Answerers["new"] = nil
	c.Digests["a.sysml"] = "changed"
	if p.Reads.Names[0] != "P::Y" || p.Answerers["n:P::Y"][0] != "a.sysml" || len(p.Answerers) != 1 || p.Digests["a.sysml"] != "x" {
		t.Fatalf("Clone shares with its original: %+v", p)
	}
	if (*Provenance)(nil).Clone() != nil {
		t.Fatal("nil does not clone to nil")
	}
}

// One Sources memoizes what the index answers, so it is for one state of the
// index; a fresh one sees a change.
func TestSourcesMemoizeTheIndexForOneState(t *testing.T) {
	w := newProvenanceWorkspace(t, map[string]string{"a.sysml": "package P { part def Y; }"})
	reads := resolve.Reads{Namespaces: []string{"P"}, Segments: []string{"s"}}
	src := w.sources()
	p, err := Attribute(reads, src)
	if err != nil {
		t.Fatalf("Attribute: %v", err)
	}
	w.add(t, "b.sysml", "package P { part def <s> Z; }")
	w.idx.ExpandWildcardImports()
	if !p.Valid(src) {
		t.Fatal("a Sources taken before the change answered from the changed index")
	}
	if p.Valid(w.sources()) {
		t.Fatal("a fresh Sources did not see the change")
	}
	if p.Valid(Sources{Index: w.idx, Contributors: src.Contributors, Digest: src.Digest}) {
		t.Fatal("an unmemoized Sources did not see the change")
	}
}

// A version of a library file standing in for it answers as the library
// document it displaces: a record written under the library holds beside the
// version, and one written beside it names it nowhere, since the key's library
// identity covers its text.
func TestProvenanceSeesAVersionAsTheLibraryDocument(t *testing.T) {
	w := newProvenanceWorkspace(t, map[string]string{
		"a.sysml": "package P { part def Y; }",
	})
	w.shared["\x00identity/a.sysml"] = []string{"a.sysml"}
	reads := resolve.Reads{Names: []string{"\x00identity/a.sysml", "P::Y"}}
	p, err := Attribute(reads, w.sources())
	if err != nil {
		t.Fatalf("Attribute: %v", err)
	}

	w.add(t, "copy.kerml", "package ScalarValues { datatype Real; }")
	w.idx.ExpandWildcardImports()
	w.shared["\x00identity/a.sysml"] = []string{"a.sysml", "copy.kerml"}
	if p.Valid(w.sources()) {
		t.Fatal("a workspace document the record never saw joined an answer, yet the record holds")
	}
	w.standIns["copy.kerml"] = "Kernel Libraries/Kernel Data Type Library/ScalarValues.kerml"
	if !p.Valid(w.sources()) {
		t.Error("the version stands in for a library document, yet the record does not hold beside it")
	}

	beside, err := Attribute(reads, w.sources())
	if err != nil {
		t.Fatalf("Attribute beside the version: %v", err)
	}
	if !reflect.DeepEqual(beside.Answerers, p.Answerers) || !reflect.DeepEqual(beside.Digests, p.Digests) {
		t.Errorf("attributed beside the version: %v %v, want %v %v", beside.Answerers, beside.Digests, p.Answerers, p.Digests)
	}
}
