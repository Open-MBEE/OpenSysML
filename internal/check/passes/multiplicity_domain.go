package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/check/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

const msgMultiplicityFeaturingTypes = "Multiplicity must have same featuring types as it feature"

// MultiplicityDomainPass checks that a feature-owned multiplicity has the same
// featuring types as its owning feature.
type MultiplicityDomainPass struct{}

func (MultiplicityDomainPass) Level() PassLevel { return LevelConstraint }

func (MultiplicityDomainPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	cc := &constraintChecker{
		model:    ctx.Model(),
		resolver: ctx.Resolver(),
		seen:     make(map[*symbols.Symbol]bool),
	}
	var diags []diag.Diagnostic
	kit.WalkSymbols(ctx, rootScope, func(sym *symbols.Symbol) {
		rel, ok := sym.Decl.(*ast.RelationshipMember)
		if !ok || sym.Kind != symbols.SymbolRelationship || rel.Kind != ast.RelFeaturedBy {
			return
		}
		multiplicity, ok := cc.resolver.ResolveTarget(sym.OwnerScope, rel.Source)
		if !ok || multiplicity == nil || multiplicity.Kind != symbols.SymbolMultiplicity {
			return
		}
		feature := multiplicity.Owner()
		if feature == nil || !feature.IsFeature() || feature.Kind == symbols.SymbolMultiplicity {
			return
		}
		target, ok := cc.resolver.ResolveTarget(sym.OwnerScope, rel.Target)
		if !ok || target == nil {
			return
		}
		for _, context := range cc.featuringContexts(feature) {
			if cc.featuringContextConforms(target, context) {
				return
			}
		}
		diags = append(diags, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     rel.Span(),
			Message:  msgMultiplicityFeaturingTypes,
			Code:     "multiplicity-featuring-type",
			Source:   "constraint",
		})
	})
	return diags
}
