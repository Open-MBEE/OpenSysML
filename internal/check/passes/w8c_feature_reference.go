package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/check/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// KerMLValidator's message for a referent, chain-expression target or chaining
// feature that is not a valid feature.
const msgReferentIsFeature = "Must be a valid feature"

// FeatureReferencePass checks the features an expression names: a referent must
// be a feature (KerML 8.4.4.6.2/8.4.4.7.2), a featured one must be reachable
// from where it is named (8.3.4.5.2 validateSubsettingFeaturingTypes), and each
// feature of a declared chain must be featured within the one before it
// (8.3.3.3.4 validateFeatureChainingFeatureConformance).
type FeatureReferencePass struct{}

func (FeatureReferencePass) Level() PassLevel { return LevelConstraint }

func (FeatureReferencePass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	c := &featureReferenceChecker{cc: &constraintChecker{
		model:    ctx.Model(),
		resolver: ctx.Resolver(),
	}}
	w := &kit.Walker{Ctx: ctx}
	w.Walk(rootScope, c.checkSymbol)
	c.walkFilters(rootScope, make(map[*symbols.Scope]bool))
	return c.diags
}

// Operators taking a type, not a feature, as an operand.
var w8cTypeOperators = map[ast.OperatorKind]bool{
	ast.OpMeta:    true,
	ast.OpAll:     true,
	ast.OpAs:      true,
	ast.OpIsType:  true,
	ast.OpHasType: true,
	ast.OpAt:      true,
	ast.OpMetaAt:  true,
}

// w8cOwnsVariants reports whether sym's members are variants (enumeration
// literals included), which are owned rather than featured.
func w8cOwnsVariants(sym *symbols.Symbol) bool {
	if sym.Recorded() {
		return sym.Facts.Modifiers.Has(symbols.ModVariation) ||
			sym.Facts.DefKind == ast.DefEnumeration && sym.Facts.Node == symbols.NodeDefinition ||
			sym.Facts.UsageKind == ast.UsageEnumeration && sym.Facts.Node == symbols.NodeUsage
	}
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		return d.IsVariation || d.Kind == ast.DefEnumeration
	case *ast.Usage:
		return d.IsVariation || d.Kind == ast.UsageEnumeration
	}
	return false
}

type featureReferenceChecker struct {
	cc    *constraintChecker
	diags []diag.Diagnostic
}

// refSite is where a reference is written: the declaration owning it, and
// whether it stands in that declaration's body, which the declaration features.
type refSite struct {
	sym             *symbols.Symbol
	inBody          bool
	inElementFilter bool
}

// accessibleFrom reports whether target is reachable from the site.
func (c *featureReferenceChecker) accessibleFrom(site refSite, target *symbols.Symbol) bool {
	if site.inBody && site.sym != nil && c.cc.featuredWithin(target, site.sym) {
		return true
	}
	return c.cc.redefinedAccessible(site.sym, target, map[*symbols.Symbol]bool{})
}

func (c *featureReferenceChecker) checkSymbol(sym *symbols.Symbol) {
	scope := sym.OwnerScope
	if sym.Scope != nil {
		scope = sym.Scope
	}
	switch d := sym.Decl.(type) {
	case *ast.Usage:
		c.checkDeclaredChains(sym, d)
		c.checkBindingEnds(sym, d)
		c.walkExpr(refSite{sym: sym}, scope, d.Value)
		c.walkMembers(refSite{sym: sym, inBody: true}, scope, d.Members)
	case *ast.Definition:
		c.walkMembers(refSite{sym: sym, inBody: true}, scope, d.Members)
	}
}

// walkFilters visits every namespace filter in the document's scope tree.
func (c *featureReferenceChecker) walkFilters(scope *symbols.Scope, seen map[*symbols.Scope]bool) {
	if scope == nil || seen[scope] {
		return
	}
	seen[scope] = true
	for _, f := range symbols.NamespaceFiltersIn(scope) {
		c.walkExpr(refSite{inElementFilter: true}, f.Scope, f.Expr)
	}
	for _, f := range symbols.ImportFiltersIn(scope) {
		c.walkExpr(refSite{inElementFilter: true}, f.Scope, f.Expr)
	}
	scope.ForEachMember(func(sym *symbols.Symbol) bool {
		if sym != nil {
			c.walkFilters(sym.Scope, seen)
		}
		return true
	})
}

// walkMembers visits the expressions a body writes outside a declaration of its
// own: constraint conditions, calc results, guards and assignments. A nested
// declaration is a symbol and is visited as one, so it is skipped here.
func (c *featureReferenceChecker) walkMembers(site refSite, scope *symbols.Scope, members []ast.Node) {
	for _, m := range members {
		if mem, ok := m.(*ast.Membership); ok {
			m = mem.Member
		}
		c.walkMember(site, scope, m)
	}
}

// walkMember visits one body member. Members owning a scope of their own resolve
// their expressions in it, as the type checker does.
func (c *featureReferenceChecker) walkMember(site refSite, scope *symbols.Scope, m ast.Node) {
	switch n := m.(type) {
	case *ast.ConstraintMember:
		c.walkExpr(site, scope, n.Expression)
		c.walkConstraintBody(site, scope, n, n.Body)
	case *ast.AssumeMember:
		c.walkExpr(site, scope, n.Expression)
		c.walkExpr(site, scope, n.Value)
		c.walkConstraintBody(site, scope, n, n.Body)
	case *ast.RequireMember:
		c.walkExpr(site, scope, n.Expression)
		c.walkExpr(site, scope, n.Value)
		c.walkConstraintBody(site, scope, n, n.Body)
	case *ast.SubjectMember:
		c.walkExpr(site, scope, n.BindingExpr)
	case *ast.AssignmentActionNode:
		c.walkExpr(site, scope, n.Value)
	case *ast.ActionExecutionNode:
		c.walkExpr(site, scope, n.Expression)
	case *ast.IfActionNode:
		c.walkExpr(site, scope, n.Condition)
		for _, branch := range n.Branches() {
			c.walkMember(site, scope, branch)
		}
	case *ast.IfBranchNode:
		c.walkMembers(site, childScopeOr(scope, n), n.Body)
	case *ast.WhileLoopActionNode:
		body := childScopeOr(scope, n)
		c.walkExpr(site, scope, n.Collection)
		c.walkExpr(site, body, n.Condition)
		c.walkExpr(site, body, n.Until)
		c.walkMembers(site, body, n.Body)
	case *ast.TransitionMember:
		if change, ok := n.Trigger.(*ast.ChangeEvent); ok {
			c.walkExpr(site, scope, change.Condition)
		}
		// The guard, effect and body are the transition's own, so they reach
		// its features, the payload parameter its trigger declares included.
		body := symbols.TriggerScope(scope, n)
		if body != scope {
			if owner := body.Owner(); owner != nil {
				site = refSite{sym: owner, inBody: true}
			}
		}
		c.walkExpr(site, body, n.Guard)
		c.walkMembers(site, body, n.Effect)
		c.walkMembers(site, body, n.Members)
	case *ast.StateNode:
		body := childScopeOr(scope, n)
		c.walkMembers(site, body, n.Entry)
		c.walkMembers(site, body, n.Do)
		c.walkMembers(site, body, n.Exit)
		c.walkMembers(site, body, n.Substates)
		for _, region := range n.Regions {
			c.walkMember(site, body, region)
		}
	case *ast.StateRegion:
		c.walkMembers(site, childScopeOr(scope, n), n.States)
	case *ast.InitialNode:
		c.walkMembers(site, childScopeOr(scope, n), n.Members)
	case *ast.SuccessionEdge:
		c.walkMembers(site, childScopeOr(scope, n), n.Members)
	case *ast.ForkNode, *ast.JoinNode, *ast.MergeNode, *ast.DecisionNode:
		c.walkMembers(site, childScopeOr(scope, m), ast.NodeBodyMembers(m))
	case *ast.SendStatement:
		c.walkExpr(site, scope, n.Message)
		c.walkExpr(site, scope, n.Target)
		c.walkExpr(site, scope, n.Receiver)
		c.walkMembers(site, childScopeOr(scope, n), n.Members)
	case *ast.EntryMember:
		c.walkMembers(site, scope, n.Actions)
	case *ast.DoMember:
		c.walkMembers(site, scope, n.Actions)
	case *ast.ExitMember:
		c.walkMembers(site, scope, n.Actions)
	default:
		// A bare expression member is a body's implicit result, as in a calc
		// body whose result is its last expression.
		c.walkExpr(site, scope, m)
	}
}

// walkConstraintBody visits a nested constraint's body from the constraint usage
// the member owns, whose own parameters the body reads.
func (c *featureReferenceChecker) walkConstraintBody(site refSite, scope *symbols.Scope, decl ast.Node, body []ast.Node) {
	inner := symbols.ConstraintBodyScope(scope, decl)
	if inner != scope {
		if owner := inner.Owner(); owner != nil {
			site = refSite{sym: owner, inBody: true}
		}
	}
	c.walkMembers(site, inner, body)
}

// walkExpr visits the references an expression names. Nested declarations are
// symbols of their own and are visited as such.
func (c *featureReferenceChecker) walkExpr(site refSite, scope *symbols.Scope, n ast.Node) {
	switch e := n.(type) {
	case nil:
		return
	case *ast.FeatureReference:
		c.checkReferent(site, scope, e, e.Span())
	case *ast.FeatureChainExpr:
		c.walkExpr(site, scope, e.Operand)
		span := e.Span()
		if e.Member != nil {
			span = e.Member.Span()
		}
		c.checkReferent(site, scope, e, span)
	case *ast.OperatorExpr:
		// These operators name types rather than features.
		if w8cTypeOperators[e.Operator] {
			return
		}
		for _, o := range e.Operands {
			c.walkExpr(site, scope, o)
		}
	case *ast.IndexExpr:
		c.walkExpr(site, scope, e.Operand)
		c.walkExpr(site, scope, e.Index)
	case *ast.InvocationExpr:
		// An invocation names its function, and may pass one as an argument,
		// so neither position is a feature reference.
	case *ast.ConstructorExpr:
		for _, a := range e.Args {
			c.walkExpr(site, scope, a)
		}
		for _, na := range e.NamedArgs {
			c.walkExpr(site, scope, na.Value)
		}
	case *ast.SequenceExpr:
		for _, el := range e.Elements {
			c.walkExpr(site, scope, el)
		}
	}
}

// checkReferent reports a referent that is not a feature, or a feature that the
// naming context cannot reach.
func (c *featureReferenceChecker) checkReferent(site refSite, scope *symbols.Scope, ref ast.Node, span source.Span) {
	chain, isChain := ref.(*ast.FeatureChainExpr)
	target, ok := c.cc.resolver.ResolveTarget(scope, ref)
	if !ok || target == nil || target == site.sym {
		return
	}
	if !isUsageKind(target.Kind) {
		c.diags = append(c.diags, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     span,
			Message:  msgReferentIsFeature,
			Code:     "feature-reference-referent",
			Source:   "constraint",
		})
		return
	}
	// A single-segment chain member is a feature of the preceding one, so the
	// naming context need not reach it; a qualified member names it through its
	// own namespace and is checked like any other reference.
	if isChain && (chain.Member == nil || len(chain.Member.Parts) < 2) {
		return
	}
	// The base usage's `that` is implicit in every usage body, so it is
	// reachable wherever a usage names it.
	if c.cc.resolver.IsBaseThat(target) {
		return
	}
	// A variant is an owned member of its variation, not a feature of it, so
	// it carries no featuring type to be accessible from.
	if semantics.DeclaresVariant(target) {
		return
	}
	// A feature with no featuring type is accessible everywhere, so only a
	// featured one is checked.
	ctxs := c.cc.featuringContexts(target)
	if len(ctxs) == 0 {
		return
	}
	for _, ctx := range ctxs {
		if w8cOwnsVariants(ctx) {
			return
		}
	}
	if site.inElementFilter {
		if c.cc.libraryDeclared(target) {
			return
		}
	} else if c.accessibleFrom(site, target) {
		return
	}
	// A feature of an implicit node (an accept parameter's action) has no
	// nameable dot path, and our scoping shares it with sibling nodes (W6C row
	// ~952, a deliberate divergence from the reference).
	if w8cOwnedByImplicitNode(target) || semantics.AcceptPayload(target) {
		return
	}
	msg, code := msgSubsettingFeaturingTypes, "feature-reference-featuring-types"
	if isChain && !site.inElementFilter {
		msg, code = msgReferentIsFeature, "feature-reference-referent"
	}
	c.diags = append(c.diags, diag.Diagnostic{
		Severity: diag.SeverityError,
		Span:     span,
		Message:  msg,
		Code:     code,
		Source:   "constraint",
	})
}

// w8cOwnedByImplicitNode reports whether target is owned by an unnamed usage,
// which the parser creates for an implicit node such as an accept action.
func w8cOwnedByImplicitNode(target *symbols.Symbol) bool {
	if target == nil || target.OwnerScope == nil {
		return false
	}
	owner := target.OwnerScope.Owner()
	return owner != nil && owner.Name == "" && isUsageKind(owner.Kind)
}

// checkBindingEnds checks that each end of a binding names a feature, resolved
// where the binding is declared.
func (c *featureReferenceChecker) checkBindingEnds(sym *symbols.Symbol, d *ast.Usage) {
	if d.Kind != ast.UsageBinding {
		return
	}
	for _, end := range d.ConnectorEnds {
		target := end.AttachedTarget()
		if target == nil {
			continue
		}
		if _, failed := target.(*ast.ErrorNode); failed {
			continue
		}
		c.checkReferent(refSite{sym: sym}, sym.OwnerScope, target, target.Span())
	}
}

// checkDeclaredChains checks the feature chains a usage's header writes; a flow
// end's last segment names the end's nested feature, so only its prefix is a chain.
func (c *featureReferenceChecker) checkDeclaredChains(sym *symbols.Symbol, d *ast.Usage) {
	for _, rel := range d.Relationships {
		if rel != nil {
			c.checkChainTarget(sym.OwnerScope, rel.Target)
		}
	}
	for _, end := range w8dConnectorEndTargets(d) {
		if d.FlowEnds != nil && (end == d.FlowEnds.From || end == d.FlowEnds.To) {
			if chain, ok := end.(*ast.FeatureChainExpr); ok {
				c.checkChainTarget(sym.OwnerScope, chain.Operand)
			}
			continue
		}
		c.checkChainTarget(sym.OwnerScope, end)
	}
}

// checkChainTarget checks a relationship target written as a feature chain:
// every chaining feature must be featured within the one before it.
func (c *featureReferenceChecker) checkChainTarget(scope *symbols.Scope, n ast.Node) {
	chain, ok := n.(*ast.FeatureChainExpr)
	if !ok {
		return
	}
	c.checkChainTarget(scope, chain.Operand)
	target, ok := c.cc.resolver.ResolveTarget(scope, chain)
	if !ok || target == nil || !isUsageKind(target.Kind) {
		return
	}
	if !c.chainMemberFeaturedWithin(scope, chain, target) {
		span := chain.Span()
		if chain.Member != nil {
			span = chain.Member.Span()
		}
		c.reportChainMember(span)
	}
}

// chainMemberFeaturedWithin reports whether the chain's last member is featured
// within the feature its operand names; an alias or import may reach one featured elsewhere.
func (c *featureReferenceChecker) chainMemberFeaturedWithin(scope *symbols.Scope, chain *ast.FeatureChainExpr, target *symbols.Symbol) bool {
	if c.cc.resolver.IsBaseThat(target) {
		return true
	}
	if tu, ok := target.Decl.(*ast.Usage); ok && tu.IsVariant {
		return true
	}
	ctxs := c.cc.featuringContexts(target)
	if len(ctxs) == 0 {
		return true
	}
	for _, ctx := range ctxs {
		if w8cOwnsVariants(ctx) {
			return true
		}
	}
	prev, ok := c.cc.resolver.ResolveTarget(scope, chain.Operand)
	if !ok || prev == nil || !isUsageKind(prev.Kind) {
		return true
	}
	for _, ctx := range ctxs {
		if !c.chainingFeatureConforms(prev, ctx) {
			return false
		}
	}
	return true
}

// chainingFeatureConforms reports whether prev specializes the featuring type
// ctx, through the features it references as well as its declared generals.
func (c *featureReferenceChecker) chainingFeatureConforms(prev, ctx *symbols.Symbol) bool {
	if c.cc.featuringContextConforms(prev, ctx) {
		return true
	}
	for _, src := range c.cc.model.MemberSources(prev) {
		if src == ctx || c.cc.model.Conforms(src, ctx) {
			return true
		}
	}
	return false
}

func (c *featureReferenceChecker) reportChainMember(span source.Span) {
	c.diags = append(c.diags, diag.Diagnostic{
		Severity: diag.SeverityError,
		Span:     span,
		Message:  msgReferentIsFeature,
		Code:     "feature-chain-conformance",
		Source:   "constraint",
	})
}
