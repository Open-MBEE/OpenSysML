package passes

import (
	"errors"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// documentQuerySource names this pass in the diagnostics it emits.
const documentQuerySource = "document-query"

// DocumentQueryPass validates native document-query definitions.
type DocumentQueryPass struct{}

func (DocumentQueryPass) Level() PassLevel { return LevelConstraint }

func (DocumentQueryPass) ElementScoped() {
	// A marker: each query definition is gated on its own, so there is nothing to do.
}

func (DocumentQueryPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
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
			diagnostics = append(diagnostics, documentQueryDiagnostic(err))
		}
	})
	return diagnostics
}

func documentQueryDiagnostic(err error) diag.Diagnostic {
	var planning *queryplan.Error
	if !errors.As(err, &planning) {
		return diag.Diagnostic{
			Severity: diag.SeverityError,
			Message:  err.Error(),
			Code:     documentQuerySource,
			Source:   documentQuerySource,
		}
	}
	return diag.Diagnostic{
		Severity: diag.SeverityError,
		Span:     planning.Origin.Span,
		Message:  planning.Error(),
		Code:     documentQuerySource + "-" + string(planning.Kind),
		Source:   documentQuerySource,
	}
}
