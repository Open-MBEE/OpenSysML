package solve

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// document is a named source a fixture indexes.
type document struct {
	path, src string
}

// fixture indexes a model over the standard library and returns a runtime
// context and the index to look symbols up in. path names the document, which
// appears in the provenance a script records.
func fixture(t *testing.T, path, src string) (*runtime.Context, *symbols.Index) {
	t.Helper()
	return fixtureDocuments(t, document{path, src})
}

// fixtureDocuments indexes several documents over the standard library, in the
// order given.
func fixtureDocuments(t *testing.T, docs ...document) (*runtime.Context, *symbols.Index) {
	t.Helper()
	idx := libraryIndex()
	sources := make([]*source.SourceFile, 0, len(docs))
	for _, doc := range docs {
		sf := source.New(doc.path, []byte(doc.src))
		idx.AddDocument(doc.path, parser.New(sf).ParseFile())
		sources = append(sources, sf)
	}
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	ctx := runtime.NewContext(runtime.NewModel(semantics.NewModel(resolver), resolver), 10000)
	for _, sf := range sources {
		ctx.Model().RegisterSource(sf)
	}
	return ctx, idx
}

// fixtureFile indexes a .sysml file from testdata, named by its base name so a
// script's provenance does not carry the checkout's path.
func fixtureFile(t *testing.T, name string) (*runtime.Context, *symbols.Index) {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return fixture(t, name, string(src))
}

// libraryIndex is an index over the process-wide frozen standard library, which
// is what makes units, quantity value types and the scalar value types resolve.
func libraryIndex() *symbols.Index {
	return libs.NewModelIndex()
}

// symbolNamed returns the single symbol with that qualified name.
func symbolNamed(t *testing.T, idx *symbols.Index, fqn string) *symbols.Symbol {
	t.Helper()
	matches := idx.LookupQualified(fqn)
	if len(matches) != 1 {
		t.Fatalf("%s matched %d symbols, want 1", fqn, len(matches))
	}
	return matches[0]
}
