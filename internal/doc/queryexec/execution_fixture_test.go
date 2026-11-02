package queryexec

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

func loadExecutionSource(t *testing.T, content string) executionFixture {
	t.Helper()
	return loadExecutionSources(t, content, nil)
}

// loadExecutionSources loads the SysML fixture beside other documents, keyed by file name.
func loadExecutionSources(t *testing.T, content string, others map[string]string) executionFixture {
	t.Helper()
	index := libs.NewModelIndex()
	name := "query-execution.sysml"
	sf := source.New(name, []byte(content))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse fixture: %v", p.Diagnostics)
	}
	index.AddDocument(name, root)
	for other, text := range others {
		op := parser.New(source.New(other, []byte(text)))
		otherRoot := op.ParseFile()
		if len(op.Diagnostics) > 0 {
			t.Fatalf("parse %s: %v", other, op.Diagnostics)
		}
		index.AddDocument(other, otherRoot)
	}
	index.ExpandWildcardImports()
	resolver := resolve.New(index)
	model := passes.NewTypedModel(resolver)
	model.SetSourceText(func(doc string, span source.Span) string {
		if doc != name {
			return ""
		}
		return sf.Text(span)
	})
	return executionFixture{
		index:    index,
		model:    model,
		resolver: resolver,
	}
}
