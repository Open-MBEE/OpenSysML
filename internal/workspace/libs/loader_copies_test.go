package libs

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// copiedLibrary writes two copies of a one-file library under dir; a parse of
// either copy resolves D's supertype to the B declared beside it.
func copiedLibrary(t *testing.T, dir string) {
	t.Helper()
	for _, copy := range []string{"one", "two"} {
		path := filepath.Join(dir, copy, "Lib.kerml")
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("package Lib { class B; class D :> B; }"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// supertypesByDocument is the document and name of every direct supertype of
// every symbol idx holds, keyed by the symbol's own document and name.
func supertypesByDocument(idx *symbols.Index) map[string]string {
	model := semantics.NewModel(resolve.New(idx))
	out := map[string]string{}
	var walk func(doc string, s *symbols.Scope)
	walk = func(doc string, s *symbols.Scope) {
		for _, sym := range s.Members() {
			var sups []string
			for _, sup := range model.DirectSupertypes(sym) {
				sups = append(sups, sup.DocName+"#"+symbols.FQNOf(sup))
			}
			sort.Strings(sups)
			out[doc+"#"+symbols.FQNOf(sym)] = strings.Join(sups, " ")
			if sym.Scope != nil {
				walk(doc, sym.Scope)
			}
		}
	}
	for _, doc := range idx.Documents() {
		walk(doc, idx.DocumentRoot(doc))
	}
	return out
}

// Two files of equal content are two documents, each resolving its references
// from its own place in the library: the record of one copy must not be
// restored for the other, or the warm load wires a copy's supertypes to the
// other copy's declarations where the cold load resolved them within itself.
func TestLoaderRestoresEachCopyFromItsOwnRecord(t *testing.T) {
	dir := t.TempDir()
	copiedLibrary(t, dir)
	cache := &Cache{dir: t.TempDir()}
	src := NewDirSource(dir)

	cold := symbols.NewIndex()
	if err := NewLoader(src, cache).LoadAll(cold); err != nil {
		t.Fatalf("cold load: %v", err)
	}
	warmLoader := NewLoader(src, cache)
	warm := symbols.NewIndex()
	if err := warmLoader.LoadAll(warm); err != nil {
		t.Fatalf("warm load: %v", err)
	}
	if got, want := warmLoader.Hits(), len(src.List()); got != want {
		t.Fatalf("warm load restored %d documents, want all %d", got, want)
	}

	coldSupers, warmSupers := supertypesByDocument(cold), supertypesByDocument(warm)
	for key, want := range coldSupers {
		if got := warmSupers[key]; got != want {
			t.Errorf("%s: warm supertypes %q, cold %q", key, got, want)
		}
	}
	for _, copy := range []string{"one", "two"} {
		key := copy + "/Lib.kerml#Lib::D"
		if want := copy + "/Lib.kerml#Lib::B"; coldSupers[key] != want {
			t.Errorf("%s: cold supertypes %q, want %q", key, coldSupers[key], want)
		}
	}
}

// A record names the documents its facts refer to, so equal content under two
// names is two records.
func TestCacheKeyDependsOnDocumentName(t *testing.T) {
	c := &Cache{dir: t.TempDir()}
	if c.keyFor("one/Base.kerml", []byte("alpha"), "set") == c.keyFor("two/Base.kerml", []byte("alpha"), "set") {
		t.Fatal("equal content under two names produced one cache key")
	}
}
