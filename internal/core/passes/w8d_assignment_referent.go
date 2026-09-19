package passes

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

const (
	msgAssignmentReferent            = "An assignment must have a referent."
	msgAssignmentReferentTimeVarying = "Referent must be time varying."
)

// AssignmentReferentPass checks SysML AssignmentActionUsage referents.
type AssignmentReferentPass struct{}

func (AssignmentReferentPass) Level() PassLevel { return LevelConstraint }

func (AssignmentReferentPass) ElementScoped() { /* marker: per-element gating */ }

func (AssignmentReferentPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	c := &assignmentReferentChecker{
		ctx:        ctx,
		model:      ctx.Model(),
		resolver:   ctx.Resolver(),
		occurrence: assignmentOccurrenceLibraryPresent(ctx),
	}
	c.walk(rootScope, root.Members)
	return c.diags
}

func assignmentOccurrenceLibraryPresent(ctx *Context) bool {
	return len(ctx.Index.LookupQualified("Occurrences::Occurrence")) > 0
}

type assignmentReferentChecker struct {
	ctx      *Context
	model    *semantics.Model
	resolver *resolve.Resolver
	inCalc   bool
	// occurrence: Occurrences::Occurrence is loaded, so time-varying is decidable.
	occurrence bool
	diags      []diag.Diagnostic
}

// enterBody records whether the body being walked is a calculation's and returns
// the function restoring what the enclosing body was.
func (c *assignmentReferentChecker) enterBody(isCalc bool) func() {
	was := c.inCalc
	c.inCalc = isCalc
	return func() { c.inCalc = was }
}

func (c *assignmentReferentChecker) walk(scope *symbols.Scope, members []ast.Node) {
	for _, member := range members {
		c.walkNode(scope, unwrapType(member))
	}
}

func (c *assignmentReferentChecker) walkNode(scope *symbols.Scope, node ast.Node) {
	switch n := node.(type) {
	case *ast.Definition:
		defer c.enterBody(n.Kind == ast.DefCalc)()
		c.walk(childScopeOr(scope, n), n.Members)
	case *ast.Usage:
		defer c.enterBody(n.Kind == ast.UsageCalc)()
		c.walk(childScopeOr(scope, n), n.Members)
	case *ast.Package:
		c.walk(childScopeOr(scope, n), n.Members)
	case *ast.Namespace:
		c.walk(childScopeOr(scope, n), n.Members)
	case *ast.SubjectMember:
		c.walk(childScopeOr(scope, n), n.Body)
	case *ast.ConstraintMember:
		c.walk(symbols.ConstraintBodyScope(scope, n), n.Body)
	case *ast.AssumeMember:
		c.walk(symbols.ConstraintBodyScope(scope, n), n.Body)
	case *ast.RequireMember:
		c.walk(symbols.ConstraintBodyScope(scope, n), n.Body)
	case *ast.EntryMember:
		c.walk(scope, n.Actions)
	case *ast.DoMember:
		c.walk(scope, n.Actions)
	case *ast.ExitMember:
		c.walk(scope, n.Actions)
	case *ast.InitialNode:
		c.walk(childScopeOr(scope, n), n.Members)
	case *ast.ForkNode, *ast.JoinNode, *ast.MergeNode, *ast.DecisionNode:
		c.walk(childScopeOr(scope, n), ast.NodeBodyMembers(n))
	case *ast.StateNode:
		body := childScopeOr(scope, n)
		c.walk(body, n.Entry)
		c.walk(body, n.Do)
		c.walk(body, n.Exit)
		c.walk(body, n.Substates)
		for _, region := range n.Regions {
			c.walkNode(body, region)
		}
	case *ast.StateRegion:
		c.walk(childScopeOr(scope, n), n.States)
	case *ast.TransitionMember:
		body := symbols.TriggerScope(scope, n)
		c.walk(body, n.Effect)
		c.walk(body, n.Members)
	case *ast.SendStatement:
		c.walk(childScopeOr(scope, n), n.Members)
	case *ast.SuccessionEdge:
		c.walk(childScopeOr(scope, n), n.Members)
	case *ast.WhileLoopActionNode:
		c.walk(childScopeOr(scope, n), n.Body)
	case *ast.IfActionNode:
		for _, branch := range n.Branches() {
			c.walkNode(scope, branch)
		}
	case *ast.IfBranchNode:
		c.walk(childScopeOr(scope, n), n.Body)
	case *ast.AssignmentActionNode:
		c.check(scope, n)
	}
}

func childScopeOr(scope *symbols.Scope, node ast.Node) *symbols.Scope {
	if child := childScopeOf(scope, node); child != nil {
		return child
	}
	return scope
}

func (c *assignmentReferentChecker) check(scope *symbols.Scope, assignment *ast.AssignmentActionNode) {
	if c.ctx.DownstreamOfFailure(assignment.Target) {
		return
	}
	if chain, isChain := assignment.Target.(*ast.FeatureChainExpr); isChain {
		c.checkChain(scope, chain)
	}
	referent, ok := c.resolver.ResolveTarget(scope, assignment.Target)
	if !ok || referent == nil {
		return
	}
	span := assignment.Target.Span()
	if _, targetSpan := ast.TargetName(assignment.Target); targetSpan != (span) {
		span = targetSpan
	}
	// The referent is the feature the target names (SysML v2 §8.3.16.2
	// AssignmentActionUsage::referent); a type or a namespace is none.
	if !referent.IsFeature() {
		c.report(span, fmt.Sprintf("%s %s is declared `%s`, not a feature.",
			msgAssignmentReferent, targetText(assignment.Target), referent.Notation()),
			"assignment-referent")
		return
	}
	if c.referentMayTimeVary(referent) {
		return
	}
	c.report(span, msgAssignmentReferentTimeVarying, "assignment-referent-time-varying")
}

// referentMayTimeVary reports whether the referent's value may vary over time
// (SysML v2 §8.3.16.2, Usage::mayTimeVary); a named multiplicity never does.
func (c *assignmentReferentChecker) referentMayTimeVary(referent *symbols.Symbol) bool {
	if referent.Kind == symbols.SymbolMultiplicity {
		return false
	}
	if _, ok := referent.Decl.(*ast.Usage); !ok || !c.occurrence {
		return true
	}
	return c.model.UsageMayTimeVary(referent)
}

// targetText renders an assignment target as written, for a message about it.
func targetText(target ast.Node) string {
	if qn := ast.AsQualifiedName(target); qn != nil {
		return lower.EndpointText(qn)
	}
	return lower.FeaturePath(target)
}

// checkChain reports a chained assignment target the runtime cannot write: one
// written in a calculation body, or one stepping through a feature that may hold
// several objects.
func (c *assignmentReferentChecker) checkChain(scope *symbols.Scope, chain *ast.FeatureChainExpr) {
	path := lower.FeaturePath(chain)
	if path == "" {
		return
	}
	if c.inCalc {
		c.report(chain.Span(), fmt.Sprintf(
			"A calculation must not write %s, a feature of another object.", path),
			"assignment-chain-in-calc")
		return
	}
	for _, step := range chainSteps(chain) {
		sym, ok := c.resolver.ResolveTarget(scope, step)
		if !ok || sym == nil {
			continue
		}
		upper := c.model.EffectiveMultiplicityOf(sym).Upper
		if !upper.Known || (!upper.Infinite && upper.Value <= 1) {
			continue
		}
		c.report(chain.Span(), fmt.Sprintf(
			"Assignment target %s steps through %s, which may hold several objects.",
			path, lower.FeaturePath(step)),
			"assignment-chain-step-not-one-object")
		return
	}
}

// chainSteps returns the operands a chained target steps through, innermost
// first: `a.b.c` steps through `a` and `a.b`.
func chainSteps(chain *ast.FeatureChainExpr) []ast.Node {
	var steps []ast.Node
	for {
		steps = append([]ast.Node{chain.Operand}, steps...)
		inner, nested := chain.Operand.(*ast.FeatureChainExpr)
		if !nested {
			return steps
		}
		chain = inner
	}
}

func (c *assignmentReferentChecker) report(span source.Span, message, code string) {
	c.diags = append(c.diags, diag.Diagnostic{
		Severity: diag.SeverityError,
		Span:     span,
		Message:  message,
		Code:     code,
		Source:   "constraint",
	})
}
