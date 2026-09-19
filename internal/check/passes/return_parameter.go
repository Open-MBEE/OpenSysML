package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// msgReturnParameterOwner reports a `return` parameter owned by a type that is
// no function or expression (KerML validateReturnParameterMembershipOwningType).
const msgReturnParameterOwner = "Return parameter membership not allowed: only a function or expression (calculation, constraint) declares a result; write `out` for an output"

// checkReturnParameterOwner reports a result parameter whose owning type is no
// function or expression.
func (cc *constraintChecker) checkReturnParameterOwner(sym *symbols.Symbol) {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || !usage.IsResult || sym.OwnerScope == nil || functionBodyScope(sym.OwnerScope) {
		return
	}
	cc.diags = append(cc.diags, diag.Diagnostic{
		Severity: diag.SeverityError,
		Span:     usage.Span(),
		Message:  msgReturnParameterOwner,
		Code:     "return-parameter-owner",
		Source:   "constraint",
	})
}

// functionBodyScope reports whether scope is the body of a function or an
// expression, in either notation.
func functionBodyScope(scope *symbols.Scope) bool {
	if owner := scope.Owner(); owner != nil {
		return functionDecl(owner.Decl)
	}
	return functionDecl(scope.Node())
}

// functionDecl reports whether decl declares a KerML Function or Expression or a
// SysML kind specializing one (calculations, constraints, requirements, cases).
func functionDecl(decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.Definition:
		switch d.Kind {
		case ast.DefCalc, ast.DefConstraint, ast.DefPredicate, ast.DefBool,
			ast.DefRequirement, ast.DefConcern, ast.DefViewpoint,
			ast.DefCase, ast.DefAnalysisCase, ast.DefVerificationCase, ast.DefUseCase:
			return true
		}
	case *ast.Usage:
		switch d.Kind {
		case ast.UsageCalc, ast.UsageExpr, ast.UsageConstraint, ast.UsagePredicate, ast.UsageBool,
			ast.UsageRequirement, ast.UsageConcern, ast.UsageViewpoint,
			ast.UsageSatisfy, ast.UsageObjective, ast.UsageFramedConcern,
			ast.UsageCase, ast.UsageAnalysisCase, ast.UsageVerificationCase, ast.UsageUseCase:
			return true
		}
	case *ast.BodyExpr, *ast.ConstraintMember, *ast.AssumeMember, *ast.RequireMember:
		return true
	}
	return false
}
