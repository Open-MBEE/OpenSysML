package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// Pilot SysMLValidator (2026-05) checkRequirementVerificationMembership with
// UsageUtil.isLegalVerification, constraint
// validateRequirementVerificationMembershipOwningType.
const msgVerificationOutsideObjective = "A requirement verification must be in the objective of a verification case."

// W8DVerificationPass checks that a `verify` appears only in the objective of a
// verification case (SysML v2 §8.3.24).
type W8DVerificationPass struct{}

func (W8DVerificationPass) Level() PassLevel { return LevelConstraint }

func (W8DVerificationPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	var diags []diag.Diagnostic
	kit.WalkSymbols(ctx, rootScope, func(sym *symbols.Symbol) {
		if !w8dIsVerify(sym) || w8dLegalVerification(sym) {
			return
		}
		diags = append(diags, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     sym.Decl.Span(),
			Message:  msgVerificationOutsideObjective,
			Code:     "verification-owning-type",
			Source:   "constraint",
		})
	})
	return diags
}

// w8dIsVerify reports whether sym is a `verify`, which shares its usage kind with
// `satisfy` — only the keyword tells a verification from a satisfaction.
func w8dIsVerify(sym *symbols.Symbol) bool {
	u, ok := sym.Decl.(*ast.Usage)
	return ok && u.Kind == ast.UsageSatisfy && u.Keyword == "verify"
}

// w8dLegalVerification reports whether sym is owned by the objective of a
// verification case, the one place a verification belongs.
func w8dLegalVerification(sym *symbols.Symbol) bool {
	if sym.OwnerScope == nil {
		return false
	}
	objective := sym.OwnerScope.Owner()
	if objective == nil || !w8dIsObjective(objective.Decl) {
		return false
	}
	if objective.OwnerScope == nil {
		return false
	}
	return w8dIsVerificationCase(objective.OwnerScope.Owner())
}

func w8dIsObjective(decl ast.Node) bool {
	u, ok := decl.(*ast.Usage)
	return ok && u.Kind == ast.UsageObjective
}

func w8dIsVerificationCase(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		return d.Kind == ast.DefVerificationCase
	case *ast.Usage:
		return d.Kind == ast.UsageVerificationCase
	}
	return false
}
