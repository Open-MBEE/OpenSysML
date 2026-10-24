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
// checkRedefinition (checkPackageLevelRedefinition), so only `:>` is checked here.
func (cc *constraintChecker) checkSubsettingFeaturingTypes(sym *symbols.Symbol) {
	if sym == nil {
		return
	}
	// `specialization subset f :> g;` is the same Subsetting written as a member of its own.
	if rel, ok := sym.Decl.(*ast.RelationshipMember); ok && sym.Kind == symbols.SymbolRelationship {
		if rel.Kind == ast.RelSubsets && rel.Source != nil && rel.Target != nil {
			if subsetting, ok := cc.resolver.ResolveTarget(sym.OwnerScope, rel.Source); ok && subsetting != nil {
				cc.checkSubsettedFeatureAccessible(subsetting, sym.OwnerScope, rel.Target)
			}
		}
		return
	}
	// Only a Subsetting is checked: `:>` between classifiers is a
	// Subclassification, which the rule does not constrain.
	if !isUsageKind(sym.Kind) {
		return
	}
	scope := sym.OwnerScope
	if sym.Scope != nil {
		scope = sym.Scope
	}
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelSubsets || rel.Target == nil {
			continue
		}
		cc.checkSubsettedFeatureAccessible(sym, scope, rel.Target)
	}
}

// checkSubsettedFeatureAccessible reports target, named in scope as a feature sym
// subsets, when it is featured within a type sym's featuring context cannot reach.
func (cc *constraintChecker) checkSubsettedFeatureAccessible(sym *symbols.Symbol, scope *symbols.Scope, target ast.Node) {
	if !isUsageKind(sym.Kind) || ast.IsFeatureChain(target) {
		return
	}
	targetNode := target
	if fr, ok := targetNode.(*ast.FeatureReference); ok {
		targetNode = fr.Name
	}
	qn, ok := targetNode.(*ast.QualifiedName)
	if !ok {
		return
	}
	subsetted, ok := cc.resolver.ResolveQualified(scope, qn)
	if !ok || subsetted == nil || subsetted == sym || !isUsageKind(subsetted.Kind) {
		return
	}
	// A package-level feature has no featuring type, so it is accessible
	// everywhere and the reference does not check it.
	if len(cc.featuringContexts(subsetted)) == 0 {
		return
	}
	if cc.redefinedAccessible(sym, subsetted, map[*symbols.Symbol]bool{}) {
		return
	}
	cc.diags = append(cc.diags, diag.Diagnostic{
		Severity: diag.SeverityError,
		Span:     target.Span(),
		Message:  msgSubsettingFeaturingTypes,
		Code:     "subsetting-featuring-types",
		Source:   "constraint",
	})
}
