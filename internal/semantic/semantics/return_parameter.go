package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// DeclaresFunction reports whether decl declares a KerML Function or Expression
// or a SysML kind specializing one.
func DeclaresFunction(decl ast.Node) bool {
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

// ResultParameterOwnerValid reports whether sym's owner is valid under KerML
// validateReturnParameterMembershipOwningType.
func ResultParameterOwnerValid(sym *symbols.Symbol) bool {
	if sym == nil || sym.OwnerScope == nil {
		return true
	}
	scope := sym.OwnerScope
	if owner := scope.Owner(); owner != nil {
		return DeclaresFunction(owner.Decl)
	}
	return DeclaresFunction(scope.Node())
}
