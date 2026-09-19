package libs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// documentedLibrary declares one element with a doc body.
const documentedLibrary = `package Lib {
	part def Rig {
		doc /* A Rig holds the instruments. */
	}
}
`

// docBodySpan finds the doc body of Lib::Rig in idx: the document it is in and
// the span the index computed for it.
func docBodySpan(t *testing.T, idx *symbols.Index) (string, source.Span) {
	t.Helper()
	rigs := idx.LookupQualified("Lib::Rig")
	if len(rigs) != 1 || rigs[0].Scope == nil {
		t.Fatalf("Lib::Rig resolves to %d symbols, want one with a scope", len(rigs))
	}
	var doc *symbols.Symbol
	rigs[0].Scope.ForEachMember(func(member *symbols.Symbol) bool {
		if member.Kind == symbols.SymbolDocumentation {
			doc = member
		}
		return doc == nil
	})
	if doc == nil {
		t.Fatal("Lib::Rig has no documentation member")
	}
	decl, ok := doc.Decl.(*ast.Documentation)
	if !ok {
		t.Fatalf("documentation member declares %T, want *ast.Documentation", doc.Decl)
	}
	return doc.DocName, decl.BodySpan
}

// Text over the source a frozen library was built from answers the spans of
// that library's index from the bytes it was built over, however the files
// change afterwards, and answers nothing for a name that is no library file.
func TestTextServesTheBytesAFrozenLibraryWasBuiltFrom(t *testing.T) {
	dir := writeLibrary(t, documentedLibrary)
	t.Setenv(LibraryPathEnvVar, dir)

	idx, src := FrozenLibrary()
	doc, span := docBodySpan(t, idx)
	text := Text(src)
	const want = "/* A Rig holds the instruments. */"
	if got := text(doc, span); got != want {
		t.Fatalf("Text(%q, %v) = %q, want %q", doc, span, got, want)
	}

	if err := os.WriteFile(filepath.Join(dir, doc), []byte("package Lib { part def Rig; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := text(doc, span); got != want {
		t.Errorf("Text after the file changed = %q, want the bytes the index was built from", got)
	}
	if got := text("Other.sysml", span); got != "" {
		t.Errorf("Text of a name that is no library file = %q, want none", got)
	}
	if Text(nil) != nil {
		t.Error("Text(nil) is not nil")
	}
}

// Text over any other source reads the file it is asked for once and serves
// its spans, so an index built over a directory can be read through it.
func TestTextReadsAnySourceOnce(t *testing.T) {
	dir := writeLibrary(t, documentedLibrary)
	src := NewDirSource(dir)
	idx := symbols.NewIndex()
	loadInto(idx, src)
	doc, span := docBodySpan(t, idx)

	text := Text(src)
	const want = "/* A Rig holds the instruments. */"
	if got := text(doc, span); got != want {
		t.Fatalf("Text(%q, %v) = %q, want %q", doc, span, got, want)
	}
	if err := os.WriteFile(filepath.Join(dir, doc), []byte("package Lib { part def Rig; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := text(doc, span); got != want {
		t.Errorf("Text after the file changed = %q, want the text first read", got)
	}
	if got := text("Other.sysml", span); got != "" {
		t.Errorf("Text of a missing file = %q, want none", got)
	}
}
