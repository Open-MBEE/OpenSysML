package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/check/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// checkPrefixMetadata types the body of an `@M { f = v; }` annotation: each body
// feature as the usage it is, a value bound to a restated feature of M as bound to it.
func (tc *typeChecker) checkPrefixMetadata(scope *symbols.Scope, prefix *ast.PrefixMetadata) {
	if prefix == nil || len(prefix.Body) == 0 {
		return
	}
	body := childScopeOr(scope, prefix)
	if prefix.Type != nil {
		tc.markMetadataBindings(tc.expr.resolveTarget(scope, prefix.Type), body, prefix.Body)
	}
	tc.walk(body, prefix.Body)
}

// markMetadataUsageBody marks the values the body of `metadata m : M { f = v; }` binds
// to the features of M they restate, ahead of the walk that checks them.
func (tc *typeChecker) markMetadataUsageBody(scope *symbols.Scope, u *ast.Usage) {
	if u.Kind != ast.UsageMetadata || len(u.Members) == 0 {
		return
	}
	for _, rel := range u.Relationships {
		if rel == nil || rel.Kind != ast.RelTyping || rel.Target == nil {
			continue
		}
		tc.markMetadataBindings(tc.expr.resolveTarget(scope, rel.Target), childScopeOr(scope, u), u.Members)
		return
	}
}

// markMetadataBindings records, at every depth of a metadata body whose type is owner,
// the feature of owner each body declaration typed by nothing of its own restates.
func (tc *typeChecker) markMetadataBindings(owner *symbols.Symbol, scope *symbols.Scope, body []ast.Node) {
	if owner == nil {
		return
	}
	for _, node := range body {
		usage, ok := kit.UnwrapMembership(node).(*ast.Usage)
		if !ok || usage.Kind == ast.UsageMetadata {
			continue
		}
		target := tc.metadataBodyTarget(owner, scope, usage)
		if target == nil {
			continue
		}
		if usage.Value != nil && !hasTypingRelationship(usage.Relationships) {
			if tc.metadataTargets == nil {
				tc.metadataTargets = map[*ast.Usage]*symbols.Symbol{}
			}
			tc.metadataTargets[usage] = target
		}
		tc.markMetadataBindings(target, childScopeOr(scope, usage), usage.Members)
	}
}

// checkMetadataBinding checks the value of a marked body declaration as bound to the
// feature it restates: by that feature's types, dimension, multiplicity and uniqueness.
func (tc *typeChecker) checkMetadataBinding(scope *symbols.Scope, u *ast.Usage, target *symbols.Symbol) {
	td, ok := featureDeclOf(target.Decl)
	if !ok {
		return
	}
	tc.expr.checkBoundValue(scope, target.OwnerScope, td, u.Value, nil)
}

// metadataBodyTarget is the feature of owner a body declaration restates: the one
// its `:>>` names, or the one of its name; nil when it restates none.
func (tc *typeChecker) metadataBodyTarget(owner *symbols.Symbol, scope *symbols.Scope, usage *ast.Usage) *symbols.Symbol {
	for _, rel := range usage.Relationships {
		if rel == nil || rel.Kind != ast.RelRedefines {
			continue
		}
		declared := declaredSymbol(scope, usage)
		if declared == nil {
			return nil
		}
		for _, feature := range tc.expr.model.RedefinedFeatures(declared) {
			if featureOwner := ownerType(feature); featureOwner != nil && tc.expr.model.Conforms(owner, featureOwner) {
				return feature
			}
		}
		return nil
	}
	return symbols.MetadataBodyTarget(tc.expr.model, owner, usage.Ident)
}
