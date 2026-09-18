package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const (
	msgRedefinePackageLevel  = "A package-level feature cannot be redefined"
	msgRedefineSameFeaturing = "Featuring types of redefining feature and redefined feature cannot be the same"
	msgRedefineEndFeature    = "Redefining feature must be an end feature"
)

// checkW10BRedefinition implements the KerML redefinition constraints the
// reference reports and we did not: a package-level feature cannot be
// redefined, redefining and redefined features cannot share a featuring type,
// and an end feature can only be redefined by an end feature
// (KerML §7.4.9 validateRedefinition*).
func (cc *constraintChecker) checkW10BRedefinition(sym *symbols.Symbol) {
	if sym == nil || !isUsageKind(sym.Kind) {
		return
	}
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelRedefines || rel.Target == nil {
			continue
		}
		redefined := cc.resolveRelationshipTarget(sym, rel)
		if redefined == nil || redefined == sym || !isUsageKind(redefined.Kind) {
			continue
		}
		switch {
		// A package-level feature is featured by Anything, so redefining it from
		// inside a type is legal; only another package-level feature conflicts.
		case isPackageLevelFeature(redefined) && isPackageLevelFeature(sym):
			cc.addRedefineDiag(rel.Target, msgRedefinePackageLevel, "redefinition-package-level")
		case cc.sameFeaturingType(sym, redefined):
			cc.addRedefineDiag(rel.Target, msgRedefineSameFeaturing, "redefinition-same-featuring")
		case w10bIsEndFeature(redefined) && !w10bIsEndFeature(sym):
			cc.addRedefineDiag(rel.Target, msgRedefineEndFeature, "redefinition-not-end")
		}
	}
}

func (cc *constraintChecker) addRedefineDiag(target ast.Node, msg, code string) {
	cc.diags = append(cc.diags, diag.Diagnostic{
		Severity: diag.SeverityError,
		Span:     target.Span(),
		Message:  msg,
		Code:     code,
		Source:   "constraint",
	})
}

// sameFeaturingType reports whether the redefining and redefined features are
// featured by exactly the same type, which leaves the redefinition with nothing
// to specialize.
func (cc *constraintChecker) sameFeaturingType(sym, redefined *symbols.Symbol) bool {
	mine := cc.featuringContexts(sym)
	theirs := cc.featuringContexts(redefined)
	if len(mine) == 0 || len(theirs) == 0 {
		return false
	}
	for _, m := range mine {
		for _, t := range theirs {
			if m == t {
				return true
			}
		}
	}
	return false
}

// w10bIsEndFeature reports whether sym is an end feature: an `end` modifier, or
// a participant of a `connect` clause, which is an end by position.
func w10bIsEndFeature(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if sym.Kind == symbols.SymbolConnectorEnd {
		return true
	}
	u, ok := sym.Decl.(*ast.Usage)
	return ok && u.IsEnd
}
