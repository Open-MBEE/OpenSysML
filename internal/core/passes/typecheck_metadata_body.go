package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// checkPrefixMetadata types the body of an `@M { f = v; }` annotation: each
// body feature as the usage it is, and each value against the type of the
// feature of M it restates, which the body feature declares no type of its own for.
func (tc *typeChecker) checkPrefixMetadata(scope *symbols.Scope, prefix *ast.PrefixMetadata) {
	if prefix == nil || len(prefix.Body) == 0 {
		return
	}
	body := childScopeOr(scope, prefix)
	tc.walk(body, prefix.Body)
	if prefix.Type == nil {
		return
	}
	tc.checkMetadataBindings(tc.expr.resolveTarget(scope, prefix.Type), body, prefix.Body)
}

// checkMetadataUsageBody types the values the body of `metadata m : M { f = v; }`
// binds against the features of M they restate; the walk typed the body's usages.
func (tc *typeChecker) checkMetadataUsageBody(scope *symbols.Scope, u *ast.Usage) {
	if u.Kind != ast.UsageMetadata || len(u.Members) == 0 {
		return
	}
	for _, rel := range u.Relationships {
		if rel == nil || rel.Kind != ast.RelTyping || rel.Target == nil {
			continue
		}
		tc.checkMetadataBindings(tc.expr.resolveTarget(scope, rel.Target), childScopeOr(scope, u), u.Members)
		return
	}
}

// checkMetadataBindings checks, at every depth of a metadata body whose type is
// owner, that a value bound to a restated feature is of that feature's scalar type.
// A value the model decides is left to whatever reads the annotation, which
// judges it as the value it evaluates to; one only a run decides has no reader before then.
func (tc *typeChecker) checkMetadataBindings(owner *symbols.Symbol, scope *symbols.Scope, body []ast.Node) {
	if owner == nil {
		return
	}
	for _, node := range body {
		usage, ok := unwrapMembership(node).(*ast.Usage)
		if !ok || usage.Kind == ast.UsageMetadata {
			continue
		}
		target := tc.metadataBodyTarget(owner, scope, usage)
		if target == nil {
			continue
		}
		if usage.Value != nil && !hasTypingRelationship(usage.Relationships) {
			want := semantics.PrimUnknown
			if typ := tc.expr.featureValueType(target); typ != nil {
				want = tc.expr.model.PrimTypeOf(typ)
			}
			for _, element := range valueElements(usage.Value) {
				if tc.expr.model.ModelLevelEvaluable(scope, element) {
					continue
				}
				tc.expr.checkScalarBinding(element, tc.expr.silent().infer(scope, element), want)
			}
		}
		tc.checkMetadataBindings(target, childScopeOr(scope, usage), usage.Members)
	}
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
