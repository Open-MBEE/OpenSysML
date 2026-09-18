package behavior_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

func newTestIndexFromDoc(name string, root *ast.RootNamespace) *symbols.Index {
	idx := libs.NewModelIndex()
	idx.AddDocument(name, root)
	return idx
}

func newTestIndex() *symbols.Index {
	return libs.NewModelIndex()
}

func w8cLibraryMessagesIn(t *testing.T, name, src string) []string {
	t.Helper()
	sf := source.New(name, []byte(src))
	root := parser.New(sf).ParseFile()
	idx := newTestIndex()
	idx.AddDocument(name, root)
	idx.ExpandWildcardImports()
	var out []string
	for _, d := range passes.Analyze(name, root, nil, idx) {
		out = append(out, d.Message)
	}
	return out
}

func w8cCount(msgs []string, want string) int {
	var n int
	for _, msg := range msgs {
		if msg == want {
			n++
		}
	}
	return n
}

const (
	msgOnlyOneMultiplicity = "Only one multiplicity is allowed"
	msgMustBeCrossFeature  = "Must be the cross feature"
)

// endpointDiags runs the name-resolution tier over src, the tier an endpoint
// name is resolved at.
func endpointDiags(t *testing.T, src string) []diag.Diagnostic {
	t.Helper()
	sf := source.New("a.sysml", []byte(src))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", p.Diagnostics)
	}
	idx := libs.NewModelIndex()
	idx.AddDocument("a.sysml", root)
	ctx := passes.NewContext("a.sysml", idx, nil)
	return passes.NameResolutionPass{}.Run(ctx, "a.sysml", root)
}
