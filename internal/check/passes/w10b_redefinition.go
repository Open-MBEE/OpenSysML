package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

const (
	msgRedefinePackageLevel  = "A package-level feature cannot be redefined"
	msgRedefineSameFeaturing = "Featuring types of redefining feature and redefined feature cannot be the same"
	msgRedefineEndFeature    = "Redefining feature must be an end feature"
)

// checkW10BRedefinition implements the KerML redefinition constraints the
// reference reports and we did not: a package-level feature cannot be
// redefined, redefining and redefined features cannot share a featuring type,
// and an end feature can only be redefined by an end feature when the
// redefining feature is owned by an association or a connector
// (KerML §7.4.9 validateRedefinition*).
func (cc *constraintChecker) checkW10BRedefinition(sym *symbols.Symbol) {
	if sym == nil || !isUsageKind(sym.Kind) {
		return
	}
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelRedefines || rel.Target == nil {
			continue
		}
		if path := cc.model.ChainTargetPath(sym, rel.Kind, rel.Target); len(path) > 0 {
			if !cc.model.ChainRedefinitionFeaturingTypesDiffer(sym, path[0]) {
				cc.addRedefineDiag(rel.Target, msgRedefineSameFeaturing, "redefinition-same-featuring")
			}
			continue
		}
		redefined := cc.resolveRelationshipTarget(sym, rel)
		if redefined == nil || redefined == sym || !isUsageKind(redefined.Kind) {
			continue
		}
		switch {
		case !cc.model.RedefinitionFeaturingTypesDiffer(sym, redefined):
			// Both featured by Anything alone is the package-level pair.
			if len(cc.model.DerivedFeaturingTypes(sym)) == 0 && len(cc.model.DerivedFeaturingTypes(redefined)) == 0 {
				cc.addRedefineDiag(rel.Target, msgRedefinePackageLevel, "redefinition-package-level")
			} else {
				cc.addRedefineDiag(rel.Target, msgRedefineSameFeaturing, "redefinition-same-featuring")
			}
		case w10bIsEndFeature(redefined) && !w10bIsEndFeature(sym) && cc.ownedByAssociationOrConnector(sym):
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

// ownedByAssociationOrConnector reports whether sym's owning type is an Association or a
// Connector, the only owners a non-end redefinition of an end is reported in (KerML 1.1 §8.3.3.3.8).
func (cc *constraintChecker) ownedByAssociationOrConnector(sym *symbols.Symbol) bool {
	return cc.model.IsAssociationOrConnector(w10bOwningType(sym))
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
