package diagram

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Pilot SysMLValidator (2026-05) checkViewDefinition/checkViewUsage over
// checkAtMostOneFeature(ViewRenderingMembership), constraints
// validateViewDefinitionOnlyOnvViewRendering/validateViewUsageOnlyOneRendering.
const (
	msgOnlyOneViewDefinitionRendering = "A view definition may have at most one view rendering."
	msgOnlyOneViewRendering           = "A view may have at most one view rendering."
)

// ViewRenderingPass checks that a view owns at most one view rendering (SysML
// v2 §8.3.26); a plain `rendering` member is no view rendering and does not count.
type ViewRenderingPass struct{}

func (ViewRenderingPass) Level() kit.PassLevel { return kit.LevelConstraint }

func (ViewRenderingPass) Run(ctx *kit.Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	var diags []diag.Diagnostic
	kit.WalkSymbols(ctx, rootScope, func(sym *symbols.Symbol) {
		msg, ok := viewRenderingMessage(sym.Decl)
		if !ok {
			return
		}
		var renderings []ast.Node
		for _, member := range ast.DeclMembers(sym.Decl) {
			owned := member
			if membership, ok := member.(*ast.Membership); ok {
				owned = membership.Member
			}
			if u, isUsage := owned.(*ast.Usage); isUsage && u.Kind == ast.UsageViewRendering {
				renderings = append(renderings, member)
			}
		}
		if len(renderings) < 2 {
			return
		}
		// The reference errors on every rendering after the first, leaving the
		// first as the view's rendering.
		for _, extra := range renderings[1:] {
			diags = append(diags, diag.Diagnostic{
				Severity: diag.SeverityError,
				Span:     extra.Span(),
				Message:  msg,
				Code:     "only-one-view-rendering",
				Source:   "constraint",
			})
		}
	})
	return diags
}

// viewRenderingMessage returns the reference's message for the view kind decl
// declares, and whether it is a view at all.
func viewRenderingMessage(decl ast.Node) (string, bool) {
	switch d := decl.(type) {
	case *ast.Definition:
		if d.Kind == ast.DefView {
			return msgOnlyOneViewDefinitionRendering, true
		}
	case *ast.Usage:
		if d.Kind == ast.UsageView {
			return msgOnlyOneViewRendering, true
		}
	}
	return "", false
}
