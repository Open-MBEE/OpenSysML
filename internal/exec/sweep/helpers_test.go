package sweep

import (
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// parseAndBuild parses the source as one test document.
func parseAndBuild(t *testing.T, src string) *ast.RootNamespace {
	t.Helper()
	return parser.New(source.New("<test>", []byte(src))).ParseFile()
}

// typedModel is a runtime model whose arguments are typed by the passes.
func typedModel(sem *semantics.Model, resolver *resolve.Resolver) *runtime.Model {
	if sem != nil {
		sem.SetArgumentTyper(passes.NewArgumentTyper(resolver, sem))
	}
	return runtime.NewModel(sem, resolver)
}

// analysisFixture is a runtime over the source indexed with the standard
// library, and the scope of its `test` package.
func analysisFixture(t *testing.T, src string) (*runtime.Context, *symbols.Scope) {
	t.Helper()
	idx := libs.NewModelIndex()
	idx.AddDocument("<test>", parseAndBuild(t, src))
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	model := typedModel(semantics.NewModel(resolver), resolver)
	model.SetExpressionParser(parser.ParseOneExpression)
	ctx := runtime.NewContext(model, 10000)
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	return ctx, pkg.Scope
}

// watchdogScale stretches every watchdog so a loaded machine does not trip it.
const watchdogScale = 6

// watchdog fires after d, scaled.
func watchdog(d time.Duration) <-chan time.Time {
	return time.After(d * watchdogScale)
}

// equalStrings reports two string slices equal element by element.
func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// librarySymbol is the first declaration the loaded library makes under fqn, nil for none.
func librarySymbol(ctx *runtime.Context, fqn string) *symbols.Symbol {
	for _, sym := range ctx.Model().Resolver().Index().LookupQualified(fqn) {
		return sym
	}
	return nil
}
