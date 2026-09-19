package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

const msgSubsettingFeaturingTypes = "Must be an accessible feature (use dot notation for nesting)"

// checkSubsettingFeaturingTypes implements KerML validateSubsettingFeaturingTypes:
// a subsetted feature that is featured within some type must be accessible from
// the subsetting feature's featuring context. A redefinition is reported by
// checkRedefinition instead, so only `:>` is checked here.
func (cc *constraintChecker) checkSubsettingFeaturingTypes(sym *symbols.Symbol) {
	// Only a Subsetting is checked: `:>` between classifiers is a
	// Subclassification, which the rule does not constrain.
	if sym == nil || !isUsageKind(sym.Kind) {
		return
	}
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelSubsets || rel.Target == nil {
			continue
		}
		if ast.IsFeatureChain(rel.Target) {
			continue
		}
		targetNode := rel.Target
		if fr, ok := targetNode.(*ast.FeatureReference); ok {
			targetNode = fr.Name
		}
		qn, ok := targetNode.(*ast.QualifiedName)
		if !ok {
			continue
		}
		scope := sym.OwnerScope
		if sym.Scope != nil {
			scope = sym.Scope
		}
		target, ok := cc.resolver.ResolveQualified(scope, qn)
		if !ok || target == nil || target == sym || !isUsageKind(target.Kind) {
			continue
		}
		// A package-level feature has no featuring type, so it is accessible
		// everywhere and the reference does not check it.
		if len(cc.featuringContexts(target)) == 0 {
			continue
		}
		if cc.redefinedAccessible(sym, target, map[*symbols.Symbol]bool{}) {
			continue
		}
		cc.diags = append(cc.diags, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     rel.Target.Span(),
			Message:  msgSubsettingFeaturingTypes,
			Code:     "subsetting-featuring-types",
			Source:   "constraint",
		})
	}
}
