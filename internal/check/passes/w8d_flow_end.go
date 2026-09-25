package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/check/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// Pilot KerMLValidator (2026-05) checkFlowEnd, constraint validateFlowEndSubsetting:
// a flow end that subsets nothing cannot be identified.
const msgFlowEndSubsetting = "Cannot identify flow end (use dot notation)"

// Pilot KerMLValidator (2026-05) checkConnector, constraint
// validateConnectorRelatedFeatures: a connector needs two related features, and
// an unidentified flow end contributes none.
const msgConnectorRelatedFeatures = "Must have at least two related elements"

// W8DFlowEndPass checks that a flow's ends name features of the flow's own
// context rather than members of an unrelated definition (SysML v2 §8.3.17).
type W8DFlowEndPass struct{}

func (W8DFlowEndPass) Level() PassLevel { return LevelConstraint }

func (W8DFlowEndPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	resolver := ctx.Resolver()
	var diags []diag.Diagnostic
	kit.WalkSymbols(ctx, rootScope, func(sym *symbols.Symbol) {
		u, ok := sym.Decl.(*ast.Usage)
		if !ok || u.Kind != ast.UsageFlow || u.FlowEnds == nil {
			return
		}
		unidentified := 0
		for _, end := range []ast.Node{u.FlowEnds.From, u.FlowEnds.To} {
			if end == nil {
				continue
			}
			qn, ok := end.(*ast.QualifiedName)
			if !ok || len(qn.Parts) < 2 {
				continue
			}
			// The first segment of a flow end names the source or target object, so
			// a definition there leaves the end with nothing to subset.
			owner, ok := resolver.PartSymbol(qn, 0)
			if !ok || owner == nil || !isDefKind(owner.Kind) {
				continue
			}
			unidentified++
			diags = append(diags, diag.Diagnostic{
				Severity: diag.SeverityError,
				Span:     end.Span(),
				Message:  msgFlowEndSubsetting,
				Code:     "flow-end-subsetting",
				Source:   "constraint",
			})
		}
		if unidentified > 0 {
			diags = append(diags, diag.Diagnostic{
				Severity: diag.SeverityError,
				Span:     u.Span(),
				Message:  msgConnectorRelatedFeatures,
				Code:     "connector-related-features",
				Source:   "constraint",
			})
		}
	})
	return diags
}
