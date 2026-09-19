package document

import (
	"errors"

	"github.com/Open-MBEE/OpenSysML/internal/core/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/ir/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// querySource names this pass in the diagnostics it emits.
const querySource = "document-query"

// QueryPass validates native document-query definitions.
type QueryPass struct{}

func (QueryPass) Level() kit.PassLevel { return kit.LevelConstraint }

func (QueryPass) ElementScoped() {
	// A marker: each query definition is gated on its own, so there is nothing to do.
}

func (QueryPass) Run(ctx *kit.Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	scope := ctx.Index.DocumentRoot(name)
	if scope == nil {
		return nil
	}
	var diagnostics []diag.Diagnostic
	kit.WalkSymbols(ctx, scope, func(sym *symbols.Symbol) {
		if !queryplan.IsQueryDefinition(ctx.Index, ctx.Model(), sym) {
			return
		}
		if ctx.DownstreamOfFailure(sym.Decl) {
			return
		}
		if _, err := queryplan.Compile(ctx.Index, ctx.Model(), ctx.Resolver(), sym); err != nil {
			var planning *queryplan.Error
			if errors.As(err, &planning) {
				if planning.Origin.Doc != "" && planning.Origin.Doc != name {
					return
				}
				if ctx.DownstreamSpan(planning.Origin.Span) {
					return
				}
			}
			diagnostics = append(diagnostics, queryDiagnostic(err))
		}
	})
	return diagnostics
}

func queryDiagnostic(err error) diag.Diagnostic {
	var planning *queryplan.Error
	if !errors.As(err, &planning) {
		return diag.Diagnostic{
			Severity: diag.SeverityError,
			Message:  err.Error(),
			Code:     querySource,
			Source:   querySource,
		}
	}
	return diag.Diagnostic{
		Severity: diag.SeverityError,
		Span:     planning.Origin.Span,
		Message:  planning.Error(),
		Code:     querySource + "-" + string(planning.Kind),
		Source:   querySource,
	}
}
