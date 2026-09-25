package behavior_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

func newTestIndexFromDoc(name string, root *ast.RootNamespace) *symbols.Index {
	idx := libs.NewModelIndex()
	idx.AddDocument(name, root)
	return idx
}

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
