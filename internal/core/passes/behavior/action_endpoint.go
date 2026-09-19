package behavior

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// CodeEndpointNotANode marks an action endpoint naming something other than an
// action node accepted by the action lowerer.
const CodeEndpointNotANode = "endpoint-not-a-node"

// ActionEndpointPass checks endpoints written in action bodies against the
// nodes accepted by action lowering.
type ActionEndpointPass struct{}

// Level reports that this pass consumes name-resolution results.
func (ActionEndpointPass) Level() kit.PassLevel { return kit.LevelNameResolution }

// ElementScoped lets each endpoint subject gate independently on lower failures.
func (ActionEndpointPass) ElementScoped() { /* marker: per-element gating */ }

// Run checks named endpoints in every action body in the document.
func (ActionEndpointPass) Run(ctx *kit.Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	scope := ctx.Index.DocumentRoot(name)
	if scope == nil {
		return nil
	}
	c := &actionEndpointChecker{ctx: ctx}
	c.walk(scope, root.Members)
	return c.diags
}

type actionEndpointChecker struct {
	ctx   *kit.Context
	diags []diag.Diagnostic
}

// walk visits package, namespace, declaration, and behavioral body members.
func (c *actionEndpointChecker) walk(scope *symbols.Scope, members []ast.Node) {
	for _, member := range members {
		c.walkNode(scope, kit.UnwrapMembership(member))
	}
}

// walkNode checks action declarations and descends through every body shape.
func (c *actionEndpointChecker) walkNode(scope *symbols.Scope, decl ast.Node) {
	child := kit.BodyScope(scope, decl)
	switch n := decl.(type) {
	case *ast.Package:
		c.walk(child, n.Members)
	case *ast.Namespace:
		c.walk(child, n.Members)
	case *ast.Definition:
		if n.Kind == ast.DefAction {
			c.checkBody(n, child)
		}
		c.walk(child, n.Members)
	case *ast.Usage:
		if n.Kind == ast.UsageAction {
			c.checkBody(n, child)
		}
		c.walk(child, n.Members)
	case *ast.SubjectMember:
		c.walk(child, n.Body)
	case *ast.EntryMember:
		c.walk(child, n.Actions)
	case *ast.DoMember:
		c.walk(child, n.Actions)
	case *ast.ExitMember:
		c.walk(child, n.Actions)
	case *ast.InitialNode, *ast.ForkNode, *ast.JoinNode, *ast.MergeNode, *ast.DecisionNode:
		c.walk(child, ast.NodeBodyMembers(n))
	case *ast.StateNode:
		c.walk(child, n.Entry)
		c.walk(child, n.Do)
		c.walk(child, n.Exit)
		c.walk(child, n.Substates)
		for _, region := range n.Regions {
			c.walkNode(child, region)
		}
	case *ast.StateRegion:
		c.walk(child, n.States)
	case *ast.TransitionMember:
		c.walk(child, n.Members)
	case *ast.SendStatement:
		c.walk(child, n.Members)
	case *ast.SuccessionEdge:
		c.walk(child, n.Members)
	case *ast.WhileLoopActionNode:
		c.walk(child, n.Body)
	case *ast.IfActionNode:
		for _, branch := range n.Branches() {
			c.walkNode(child, branch)
		}
	case *ast.IfBranchNode:
		c.walk(child, n.Body)
	}
}

// checkBody checks the endpoints of one action definition or usage.
func (c *actionEndpointChecker) checkBody(decl ast.Node, scope *symbols.Scope) {
	nodes, hasInitial, err := lower.ActionNodes(decl, scope)
	if err != nil {
		return
	}
	for _, member := range ast.DeclMembers(decl) {
		n := kit.UnwrapMembership(member)
		switch v := n.(type) {
		case *ast.InitialNode:
			if v.Successor == nil {
				continue
			}
			c.checkEndpoint(scope, nodes, hasInitial, v.First, true, v)
			c.checkEndpoint(scope, nodes, hasInitial, v.Successor, false, v)
		case *ast.SuccessionEdge:
			if v.SourceMember == nil && !v.SourceImplied {
				c.checkEndpoint(scope, nodes, hasInitial, v.Source, true, v)
			}
			if v.TargetMember == nil && !v.TargetImplied {
				c.checkEndpoint(scope, nodes, hasInitial, v.Target, false, v)
			}
		case *ast.ControlFlowEdge:
			if v.SourceMember == nil && !v.SourceImplied {
				c.checkEndpoint(scope, nodes, hasInitial, v.Source, true, v)
			}
			if v.TargetMember == nil && !v.TargetImplied {
				c.checkEndpoint(scope, nodes, hasInitial, v.Target, false, v)
			}
		case *ast.TransitionMember:
			if v.Source != nil {
				c.checkEndpoint(scope, nodes, hasInitial, v.Source, true, v)
			}
			c.checkEndpoint(scope, nodes, hasInitial, v.Target, false, v)
		case *ast.Usage:
			if v.Kind != ast.UsageSuccession || len(v.ConnectorEnds) != 2 {
				continue
			}
			c.checkEndpoint(scope, nodes, hasInitial, connectorEndReference(v.ConnectorEnds[0]), true, v)
			c.checkEndpoint(scope, nodes, hasInitial, connectorEndReference(v.ConnectorEnds[1]), false, v)
		}
	}
}

// checkEndpoint reports a resolved endpoint that is not an action node.
func (c *actionEndpointChecker) checkEndpoint(
	scope *symbols.Scope,
	nodes []ast.Node,
	hasInitial bool,
	ref ast.Node,
	sourceEnd bool,
	subject ast.Node,
) {
	if ref == nil || lower.ActionEndpointAccepted(nodes, hasInitial, ref, sourceEnd) {
		return
	}
	// A feature chain rooted outside the action graph is legal notation; its
	// runtime support belongs to feature-chain execution rather than this check.
	if _, ok := ref.(*ast.FeatureChainExpr); ok {
		return
	}
	qn := actionEndpointName(ref)
	if qn == nil {
		return
	}
	sym, ok := c.ctx.Resolver().EndpointSymbol(scope, qn)
	_, _, _, uncertain := resolve.ActionNodeInScope(scope, qn)
	if uncertain || !ok || sym == nil || c.ctx.DownstreamOfFailure(sym.Decl) ||
		c.ctx.DownstreamOfFailure(subject) {
		return
	}
	c.diags = append(c.diags, diag.Diagnostic{
		Severity: diag.SeverityError,
		Span:     ref.Span(),
		Message:  "succession endpoint " + actionEndpointText(ref) + " is not an action node",
		Code:     CodeEndpointNotANode,
		Source:   "action-endpoint",
	})
}

// connectorEndReference preserves feature-chain references used by action flows.
func connectorEndReference(end *ast.ConnectorEnd) ast.Node {
	if end == nil {
		return nil
	}
	return end.AttachedTarget()
}

func actionEndpointName(ref ast.Node) *ast.QualifiedName {
	if qn := ast.AsQualifiedName(ref); qn != nil {
		return qn
	}
	chain, ok := ref.(*ast.FeatureChainExpr)
	if !ok {
		return nil
	}
	return actionEndpointName(chain.Operand)
}

func actionEndpointText(ref ast.Node) string {
	if text := lower.FeaturePath(ref); text != "" {
		return text
	}
	return ast.SimpleName(ref)
}
