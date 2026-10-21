package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// TransitionGuardPass checks that each transition guard is Boolean-valued,
// including the guarded successions of an action body (`first a if x then b;`).
type TransitionGuardPass struct{}

// transitionGuardContext names the expression a guard's Boolean check reports on.
const transitionGuardContext = "transition guard"

// transitionGuardBooleanFormat is the message of a guard that is not Boolean.
const transitionGuardBooleanFormat = transitionGuardContext + " must be Boolean, found %s"

// codeTransitionGuardExpression is the code of a guard that is not Boolean
// (SysML v2 validateTransitionFeatureMembershipGuardExpression).
const codeTransitionGuardExpression = "transition-feature-membership-guard-expression"

func (TransitionGuardPass) Level() PassLevel { return LevelType }

func (TransitionGuardPass) ElementScoped() { /* marker: per-element gating */ }

func (TransitionGuardPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	c := &transitionGuardChecker{
		ctx:  ctx,
		expr: &exprChecker{resolver: ctx.Resolver(), model: ctx.Model(), lang: ctx.Kind},
	}
	c.walk(rootScope, root.Members)
	return c.expr.diagnostics()
}

type transitionGuardChecker struct {
	ctx  *Context
	expr *exprChecker
}

func (c *transitionGuardChecker) walk(scope *symbols.Scope, members []ast.Node) {
	for _, member := range members {
		c.walkNode(scope, unwrapType(member))
	}
}

func (c *transitionGuardChecker) walkNode(scope *symbols.Scope, node ast.Node) {
	switch n := node.(type) {
	case *ast.Definition:
		c.walk(childScopeOr(scope, n), n.Members)
	case *ast.Usage:
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
		if !c.ctx.DownstreamOfFailure(n.Guard) {
			c.expr.checkCondition(scope, n.Guard, codeTransitionGuardExpression, transitionGuardBooleanFormat, false)
		}
		c.walk(childScopeOr(scope, n), n.Members)
	case *ast.ControlFlowEdge:
		if !c.ctx.DownstreamOfFailure(n.Guard) {
			c.expr.checkCondition(scope, n.Guard, codeTransitionGuardExpression, transitionGuardBooleanFormat, false)
		}
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
		if !c.ctx.DownstreamOfFailure(n.Guard) {
			c.expr.checkCondition(body, n.Guard, codeTransitionGuardExpression, transitionGuardBooleanFormat, false)
		}
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
	}
}
