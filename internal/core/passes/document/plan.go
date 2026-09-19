package document

import (
	"errors"

	"github.com/Open-MBEE/OpenSysML/internal/core/docplan"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// planSource names this pass in the diagnostics it emits.
const planSource = "document-plan"

// PlanPass validates native document definitions.
type PlanPass struct{}

func (PlanPass) Level() kit.PassLevel { return kit.LevelConstraint }

func (PlanPass) ElementScoped() {
	// A marker: each document definition is gated on its own, so there is nothing to do.
}

func (PlanPass) Run(ctx *kit.Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	scope := ctx.Index.DocumentRoot(name)
	if scope == nil {
		return nil
	}
	var diagnostics []diag.Diagnostic
	kit.WalkSymbols(ctx, scope, func(sym *symbols.Symbol) {
		if !docplan.IsDocumentDefinition(ctx.Index, ctx.Model(), sym) {
			return
		}
		if ctx.DownstreamOfFailure(sym.Decl) {
			return
		}
		if _, err := docplan.Compile(ctx.Index, ctx.Model(), ctx.Resolver(), sym); err != nil {
			var planning *docplan.Error
			if errors.As(err, &planning) {
				if planning.Origin.Doc != "" && planning.Origin.Doc != name {
					return
				}
				if ctx.DownstreamSpan(planning.Origin.Span) {
					return
				}
			}
			diagnostics = append(diagnostics, planDiagnostic(err))
		}
	})
	return diagnostics
}

func planDiagnostic(err error) diag.Diagnostic {
	var planning *docplan.Error
	if !errors.As(err, &planning) {
		return diag.Diagnostic{
			Severity: diag.SeverityError,
			Message:  err.Error(),
			Code:     planSource,
			Source:   planSource,
		}
	}
	return diag.Diagnostic{
		Severity: diag.SeverityError,
		Span:     planning.Origin.Span,
		Message:  planning.Error(),
		Code:     planSource + "-" + string(planning.Kind),
		Source:   planSource,
	}
}
