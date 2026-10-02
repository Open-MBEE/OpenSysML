package behavior

import (
	"github.com/Open-MBEE/OpenSysML/internal/check/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

const actionStepMultiplicitySource = "action-step-multiplicity"

// ActionStepMultiplicityPass warns about action steps the runtime cannot
// execute according to their declared multiplicity.
type ActionStepMultiplicityPass struct{}

func (ActionStepMultiplicityPass) Level() kit.PassLevel { return kit.LevelConstraint }

func (ActionStepMultiplicityPass) ElementScoped() { /* marker: per-element gating */ }

func (ActionStepMultiplicityPass) Run(ctx *kit.Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	scope := ctx.Index.DocumentRoot(name)
	if scope == nil {
		return nil
	}
	c := &actionStepMultiplicityChecker{
		ctx:      ctx,
		model:    ctx.Model(),
		visited:  make(map[*lower.ActionGraph]bool),
		reported: make(map[ast.Node]map[string]bool),
	}
	c.walk(scope, root.Members)
	return c.diags
}

type actionStepMultiplicityChecker struct {
	ctx      *kit.Context
	model    *semantics.Model
	visited  map[*lower.ActionGraph]bool
	reported map[ast.Node]map[string]bool
	diags    []diag.Diagnostic
}

func (c *actionStepMultiplicityChecker) walk(scope *symbols.Scope, members []ast.Node) {
	for _, member := range members {
		c.walkNode(scope, kit.UnwrapMembership(member))
	}
}

func (c *actionStepMultiplicityChecker) walkNode(scope *symbols.Scope, decl ast.Node) {
	child := kit.BodyScope(scope, decl)
	switch n := decl.(type) {
	case *ast.Package:
		c.walk(child, n.Members)
	case *ast.Namespace:
		c.walk(child, n.Members)
	case *ast.Definition:
		if n.Kind == ast.DefAction {
			if c.ctx.DownstreamOfFailure(n) {
				return
			}
			c.checkAction(n, child)
		}
		c.walk(child, n.Members)
	case *ast.Usage:
		if classifierBehaviorScope(scope) {
			if behavior, ok := lower.ClassifierBehaviorOf(n); ok &&
				behavior.Kind == lower.PerformedAction && behavior.Decl.Multiplicity != nil {
				if c.ctx.DownstreamOfFailure(behavior.Decl) {
					return
				}
				c.checkDeclaredMultiplicity(behavior.Decl, child)
			}
		}
		if n.Kind == ast.UsageAction {
			if c.ctx.DownstreamOfFailure(n) {
				return
			}
			c.checkAction(n, child)
		}
		c.walk(child, n.Members)
	case *ast.SubjectMember:
		c.walk(child, n.Body)
	case *ast.EntryMember:
		c.checkBehaviors(n.Actions, child)
		c.walk(child, n.Actions)
	case *ast.DoMember:
		c.checkBehaviors(n.Actions, child)
		c.walk(child, n.Actions)
	case *ast.ExitMember:
		c.checkBehaviors(n.Actions, child)
		c.walk(child, n.Actions)
	case *ast.StateNode:
		if c.ctx.DownstreamOfFailure(n) {
			return
		}
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
		c.checkBehaviors(n.Members, child)
		c.walk(child, n.Members)
	case *ast.InitialNode, *ast.ForkNode, *ast.JoinNode, *ast.MergeNode, *ast.DecisionNode:
		c.walk(child, ast.NodeBodyMembers(n))
	case *ast.SuccessionEdge:
		c.walk(child, n.Members)
	case *ast.SendStatement:
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

func classifierBehaviorScope(scope *symbols.Scope) bool {
	if scope == nil {
		return false
	}
	for current := scope; current != nil; current = current.Parent() {
		owner := current.Owner()
		if owner == nil {
			continue
		}
		definition, ok := owner.Decl.(*ast.Definition)
		if !ok {
			continue
		}
		if definition.Kind == ast.DefAction {
			return false
		}
	}
	return true
}

func (c *actionStepMultiplicityChecker) checkAction(decl ast.Node, scope *symbols.Scope) {
	if c.ctx.DownstreamOfFailure(decl) {
		return
	}
	graph, err := lower.ToActionGraphWith(decl, scope, c.ctx.Resolver())
	if err != nil {
		return
	}
	c.checkGraph(graph)
}

func (c *actionStepMultiplicityChecker) checkBehaviors(actions []ast.Node, scope *symbols.Scope) {
	for _, behavior := range lower.LowerBehaviors(actions, nil, scope, c.ctx.Resolver()) {
		if behavior.Multiplicity != nil {
			c.checkDeclaredMultiplicity(behavior.Node, behavior.Scope)
		}
		for _, statement := range behavior.Body {
			c.checkStatementGraphs(statement)
		}
	}
}

func (c *actionStepMultiplicityChecker) checkDeclaredMultiplicity(node ast.Node, scope *symbols.Scope) {
	if c.ctx.DownstreamOfFailure(node) {
		return
	}
	usage, ok := node.(*ast.Usage)
	if !ok || usage.Multiplicity == nil {
		return
	}
	graph := &lower.ActionGraph{
		Scope:          scope,
		Multiplicities: map[ast.Node]*ast.Multiplicity{node: usage.Multiplicity},
		Scopes:         map[ast.Node]*symbols.Scope{node: scope},
	}
	count, err := graph.StepCount(node, c.model)
	if err != nil {
		c.report(graph, err)
	} else if count != 1 && !lower.IsPerformedActionUsage(usage) {
		c.report(graph, graph.StepError(node, c.model, lower.StepMultiplicityUnsupportedCode,
			"the state entry, do, and exit performances have multiplicity [1]", nil))
	}
}

func (c *actionStepMultiplicityChecker) checkStatementGraphs(statement lower.Statement) {
	switch s := statement.(type) {
	case lower.Block:
		if s.Graph != nil {
			c.checkGraph(s.Graph)
		}
		for _, nested := range s.Statements {
			c.checkStatementGraphs(nested)
		}
	case lower.Loop:
		c.checkStatementGraphs(s.Body)
	case lower.If:
		c.checkStatementGraphs(s.Then)
		if s.Else != nil {
			c.checkStatementGraphs(*s.Else)
		}
	}
}

func (c *actionStepMultiplicityChecker) checkGraph(graph *lower.ActionGraph) {
	if graph == nil || c.visited[graph] {
		return
	}
	c.visited[graph] = true
	for _, node := range graph.Nodes {
		if c.ctx.DownstreamOfFailure(node) {
			continue
		}
		if graph.Multiplicities[node] == nil {
			continue
		}
		if err := graph.CheckStep(node, c.model); err != nil {
			c.report(graph, err)
		}
		for _, edge := range graph.Incoming(node) {
			literal, guarded := edge.Guard.(*ast.LiteralBool)
			if guarded && !literal.Value && edge.TargetMultiplicity != nil {
				c.report(graph, graph.StepError(node, c.model, lower.StepOrderOpenCode,
					"a false guard leaves the performances of the repeated step unordered with respect to its source", edge.Decl))
			}
		}
	}
	for _, subflow := range graph.Subflows {
		if subflow != nil {
			c.checkGraph(subflow.Graph)
		}
	}
	for _, statements := range graph.Bodies {
		for _, statement := range statements {
			c.checkStatementGraphs(statement)
		}
	}
}

func (c *actionStepMultiplicityChecker) report(graph *lower.ActionGraph, err error) {
	stepErr, ok := err.(*lower.StepMultiplicityError)
	if !ok {
		return
	}
	if c.reported[stepErr.Node] == nil {
		c.reported[stepErr.Node] = make(map[string]bool)
	}
	if c.reported[stepErr.Node][stepErr.Code] {
		return
	}
	c.reported[stepErr.Node][stepErr.Code] = true
	declaration := stepErr.Declaration
	if declaration == nil && graph != nil {
		declaration = graph.Multiplicities[stepErr.Node]
	}
	span := stepErr.Node.Span()
	if declaration != nil {
		span = declaration.Span()
	}
	c.diags = append(c.diags, diag.Diagnostic{
		Severity: diag.SeverityWarning,
		Span:     span,
		Message:  stepErr.Error(),
		Code:     stepErr.Code,
		Source:   actionStepMultiplicitySource,
	})
}
