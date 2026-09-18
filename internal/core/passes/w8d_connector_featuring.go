package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Pilot KerMLValidator (2026-05) doCheckConnector (validateConnectorTypeFeaturing)
// and checkSubsetting (validateSubsettingFeaturingTypes), which share the message:
// between them they reject every end not featured within the connector's own
// featuring type, whether or not it is reachable outwards.
const msgConnectorTypeFeaturing = "Must be an accessible feature (use dot notation for nesting)"

// w8dConnectorKinds are the usage kinds that are Connectors, whose ends the rule
// constrains. A succession's ends are states or actions of its owner, which the
// state and action passes already relate, so it is left to them.
var w8dConnectorKinds = map[ast.UsageKind]bool{
	ast.UsageConnection: true,
	ast.UsageConnector:  true,
	ast.UsageInterface:  true,
	ast.UsageAllocation: true,
	ast.UsageBinding:    true,
	ast.UsageFlow:       true,
}

// W8DConnectorFeaturingPass checks that each end of a connector names a feature
// featured within the connector's own featuring type (KerML 1.0 §8.3.3.3,
// Feature::isFeaturedWithin): a nested feature is reached with dot notation, not
// with `::`.
type W8DConnectorFeaturingPass struct{}

func (W8DConnectorFeaturingPass) Level() PassLevel { return LevelConstraint }

func (W8DConnectorFeaturingPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	cc := &constraintChecker{model: ctx.Model(), resolver: ctx.Resolver(), seen: make(map[*symbols.Symbol]bool)}
	var diags []diag.Diagnostic
	w8dWalkSymbols(ctx, rootScope, func(sym *symbols.Symbol) {
		u, ok := sym.Decl.(*ast.Usage)
		if !ok || !w8dConnectorKinds[u.Kind] {
			return
		}
		// A variant is owned by a VariantMembership, not a FeatureMembership, so
		// it has no featuring type of its own and every feature is accessible.
		if semantics.DeclaresVariant(sym) {
			return
		}
		// Ends name features of the connector's owner, so they resolve in the
		// enclosing scope as the resolver does, never to the connector's own ends.
		scope := sym.OwnerScope
		contexts := cc.featuringContexts(sym)
		for _, end := range w8dConnectorEndTargets(u) {
			qn := w8dEndRootName(end)
			if qn == nil {
				continue
			}
			target, ok := cc.resolver.ResolveQualified(scope, qn)
			if !ok || target == nil || target == sym || !isUsageKind(target.Kind) {
				continue
			}
			// A package-level feature and an enumeration literal have no
			// featuring type, so every connector reaches them.
			if len(cc.featuringContexts(target)) == 0 || w8dEnumLiteral(target) {
				continue
			}
			if w8dEndAccessible(cc, contexts, target) {
				continue
			}
			diags = append(diags, diag.Diagnostic{
				Severity: diag.SeverityError,
				Span:     end.Span(),
				Message:  msgConnectorTypeFeaturing,
				Code:     "connector-type-featuring",
				Source:   "constraint",
			})
		}
	})
	return diags
}

// w8dEnumLiteral reports whether sym is a literal of an enumeration definition.
func w8dEnumLiteral(sym *symbols.Symbol) bool {
	if sym.OwnerScope == nil {
		return false
	}
	owner := sym.OwnerScope.Owner()
	if owner == nil {
		return false
	}
	def, ok := owner.Decl.(*ast.Definition)
	return ok && def.Kind == ast.DefEnumeration
}

// w8dEndAccessible reports whether an end target is featured within every
// featuring type of the connector, which is what the rule demands.
func w8dEndAccessible(cc *constraintChecker, contexts []*symbols.Symbol, target *symbols.Symbol) bool {
	if len(contexts) == 0 {
		return true
	}
	for _, t := range contexts {
		if !cc.featuredWithin(target, t) {
			return false
		}
	}
	return true
}

// w8dConnectorEndTargets returns the nodes naming the connector's ends: the end
// clauses of `connect`/`flow`, and for a binding the `bind` end plus the feature
// on the other side of `=`.
func w8dConnectorEndTargets(u *ast.Usage) []ast.Node {
	var ends []ast.Node
	for _, ce := range u.ConnectorEnds {
		if target := ce.AttachedTarget(); target != nil {
			ends = append(ends, target)
		}
	}
	if u.FlowEnds != nil {
		if u.FlowEnds.From != nil {
			ends = append(ends, u.FlowEnds.From)
		}
		if u.FlowEnds.To != nil {
			ends = append(ends, u.FlowEnds.To)
		}
	}
	return ends
}

// w8dEndRootName returns the name whose accessibility decides the end's: the
// root of a feature chain, since the chain's later segments are reached through
// it, and otherwise the qualified name itself.
func w8dEndRootName(end ast.Node) *ast.QualifiedName {
	switch n := end.(type) {
	case *ast.QualifiedName:
		return n
	case *ast.FeatureReference:
		return w8dEndRootName(n.Name)
	case *ast.FeatureChainExpr:
		return w8dEndRootName(n.Operand)
	}
	return nil
}
