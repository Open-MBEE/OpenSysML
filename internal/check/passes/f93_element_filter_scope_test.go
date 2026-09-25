package passes

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// libraryFixtureDiags analyzes a fixture under testdata/passes as the document it
// is named, whose extension decides the language, against the standard library. A
// fixture naming a library element (`Base::Anything`, the reflective metaclasses a
// filter condition reads) is unresolved without it, and an unresolved name skips
// the type tier — which would make a type-tier assertion vacuous.
func libraryFixtureDiags(t *testing.T, file string) []diag.Diagnostic {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "tests", "testdata", "passes", file))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	idx := newTestIndex()
	root := parser.New(source.New(file, data)).ParseFile()
	idx.AddDocument(file, root)
	idx.ExpandWildcardImports()
	return Analyze(file, root, nil, idx)
}

// A filter condition is a predicate over a candidate element, so `@Structure`
// holds for a KerML `struct` and a metaclass feature (`Element::name`) reads the
// candidate itself. The pinned validate-kerml is silent on this fixture, and so
// are we.
func TestF93ElementFilterOverKerMLCandidates(t *testing.T) {
	if diags := libraryFixtureDiags(t, "f93_element_filter.kerml"); len(diags) != 0 {
		t.Fatalf("expected no diagnostics, got %v", diags)
	}
}

// The same in SysML, where `@SysML::PartDefinition` already selects by metaclass:
// what remains is the metaclass feature read.
func TestF93ElementFilterOverSysMLCandidates(t *testing.T) {
	if diags := libraryFixtureDiags(t, "f93_element_filter.sysml"); len(diags) != 0 {
		t.Fatalf("expected no diagnostics, got %v", diags)
	}
}
