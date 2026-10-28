package document

import (
	"errors"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/ir/docplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// planSource names this pass in the diagnostics it emits.
const planSource = "document-plan"

// HostDependent reports whether d answers to the host the analysis ran on
// rather than to the document — a markup converter found, missing or failing
// — so a verdict holding it is not kept for another host.
func HostDependent(d diag.Diagnostic) bool {
	if d.Source != planSource {
		return false
	}
	kind, ok := strings.CutPrefix(d.Code, planSource+"-")
	return ok && docplan.ErrorKind(kind).HostDependent()
}

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
		warnings, err := docplan.Validate(ctx.Index, ctx.Model(), ctx.Resolver(), sym)
		for _, warning := range warnings {
			if reportedHere(ctx, name, warning) {
				diagnostics = append(diagnostics, planDiagnostic(warning))
			}
		}
		if err != nil && reportedHere(ctx, name, err) {
			diagnostics = append(diagnostics, planDiagnostic(err))
		}
	})
	return diagnostics
}

// reportedHere is whether a planning error is this document's to report: one
// placed in another document, or downstream of a failure, is not.
func reportedHere(ctx *kit.Context, name string, err error) bool {
	var planning *docplan.Error
	if !errors.As(err, &planning) {
		return true
	}
	if planning.Origin.Doc != "" && planning.Origin.Doc != name {
		return false
	}
	return !ctx.DownstreamSpan(planning.Origin.Span)
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
		Severity: planning.Kind.Severity(),
		Span:     planning.Origin.Span,
		Message:  planning.Error(),
		Code:     planSource + "-" + string(planning.Kind),
		Source:   planSource,
	}
}
