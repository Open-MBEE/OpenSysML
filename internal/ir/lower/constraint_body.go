package lower

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// ConstraintStep lowers one member of a constraint body to the step it states,
// as calcStep does for a calculation body: statements run in declaration order
// except for precedence stated by successions between steps. false
// for what a constraint body states other ways: the parameters a check binds
// (`in x`), the conditions the body exists to hold (expressions and nested
// constraint, require, assume and assert members), the unnamed bindings a
// parameter redefinition writes, and members declaring nothing a step can run.
// Successions between steps supply precedence; other successions and control
// nodes remain unexecutable because a verdict does not run a token flow.
func ConstraintStep(member ast.Node, scope *symbols.Scope) (Statement, bool) {
	if actual := unwrapMembership(member); actual != nil {
		member = actual
	}
	switch m := member.(type) {
	case *ast.ConstraintMember, *ast.RequireMember, *ast.AssumeMember, *ast.SubjectMember:
		// Conditions and the subject declaration of the body, evaluated in the
		// state the steps leave — not steps of it.
		return nil, false
	case *ast.SuccessionEdge, *ast.ControlFlowEdge, *ast.InitialNode, *ast.ForkNode,
		*ast.JoinNode, *ast.MergeNode, *ast.DecisionNode, *ast.FinalNode:
		return Unsupported{Description: statedFlowKeyword(member), Node: member, Scope: scope}, true
	case *ast.WhileLoopActionNode:
		return Loop{
			Kind:       m.Kind,
			Condition:  m.Condition,
			Until:      m.Until,
			Variable:   m.Variable.Name,
			Collection: m.Collection,
			Body:       constraintLowerBlock(m, m.Body, childScope(scope, m)),
			Node:       m,
			Scope:      scope,
		}, true
	case *ast.IfActionNode:
		lowered := If{Condition: m.Condition, Node: m, Scope: scope}
		if m.Then != nil {
			block := constraintLowerBlock(m.Then, m.Then.Body, childScope(scope, m.Then))
			lowered.Then = block
		}
		if m.Else != nil {
			block := constraintLowerBlock(m.Else, m.Else.Body, childScope(scope, m.Else))
			lowered.Else = &block
		}
		return lowered, true
	case *ast.Usage:
		if m.Kind == ast.UsageAction && m.IsBodyParameter {
			return constraintLowerBlock(m, m.Members, childScope(scope, m)), true
		}
		if m.Direction == ast.DirIn || m.Direction == ast.DirInOut {
			// A parameter is bound by the check, not by the body.
			return nil, false
		}
		if m.IsSuccessionFlow() || m.Kind == ast.UsageSuccession {
			return Unsupported{Description: statedFlowKeyword(member), Node: member, Scope: scope}, true
		}
		if stmt, ok := usageStatement(m, scope); ok {
			return stmt, true
		}
		if m.IsTerminate {
			return Effect{Kind: EffectTerminate, Node: m, Scope: scope, Terminates: TerminateEnclosing}, true
		}
		if m.Kind == ast.UsageAction {
			// An action usage states a performance of the action — refused by
			// the host as an effect outside the body's own performance.
			return performEffect(m, scope), true
		}
		// An unnamed binding (`:>> limit = 5.0`), an actor or a feature of
		// another kind states something the check's environment holds rather
		// than a step performing.
		return nil, false
	case *ast.Definition, *ast.Documentation, *ast.Comment, *ast.Import, *ast.Alias:
		return nil, false
	default:
		if ast.IsExpression(member) {
			return nil, false
		}
		return lowerStatement(member, scope), true
	}
}

func constraintLowerBlock(owner ast.Node, members []ast.Node, scope *symbols.Scope) Block {
	block := Block{Node: owner, Scope: scope}
	for _, member := range members {
		actual := unwrapMembership(member)
		if actual == nil || isAnnotation(actual) {
			continue
		}
		if stmt, ok := ConstraintStep(actual, scope); ok {
			block.Statements = append(block.Statements, stmt)
		}
	}
	return constraintBlockOrder(block, members)
}

// statedFlowKeyword names the statement a flow member was written with, as the
// refusal to run it in a verdict must name it — a verdict orders steps by
// declaration, not by the successions stated.
func statedFlowKeyword(node ast.Node) string {
	switch n := node.(type) {
	case *ast.InitialNode:
		return "`first` statement"
	case *ast.ForkNode:
		return "`fork` statement"
	case *ast.JoinNode:
		return "`join` statement"
	case *ast.MergeNode:
		return "`merge` statement"
	case *ast.DecisionNode:
		return "`decide` statement"
	case *ast.FinalNode:
		return "`done` statement"
	case *ast.ControlFlowEdge:
		if n.IsElse {
			return "`else` succession"
		}
		return "`if` succession"
	case *ast.Usage:
		if n.IsSuccessionFlow() {
			return "`succession flow` statement"
		}
		return "`succession` statement"
	default:
		return "`then` statement"
	}
}

func constraintStatementOrder(scope *symbols.Scope, members []ast.Node, stmts []Statement) *StatementOrder {
	before, matched := statementPrecedence(members, stmts)
	order := bodyStatementOrder(&ActionGraph{Scope: scope}, nil, stmts, before)
	order.skipSuccessions(stmts, matched)
	return order
}

func constraintNestedOrders(stmts []Statement) {
	for i, stmt := range stmts {
		switch nested := stmt.(type) {
		case If:
			node, ok := nested.Node.(*ast.IfActionNode)
			if !ok {
				continue
			}
			if node.Then != nil {
				nested.Then = constraintBlockOrder(nested.Then, node.Then.Body)
			}
			if node.Else != nil && nested.Else != nil {
				els := constraintBlockOrder(*nested.Else, node.Else.Body)
				nested.Else = &els
			}
			stmts[i] = nested
		case Loop:
			node, ok := nested.Node.(*ast.WhileLoopActionNode)
			if !ok {
				continue
			}
			nested.Body = constraintBlockOrder(nested.Body, node.Body)
			stmts[i] = nested
		case Block:
			if node, ok := nested.Node.(*ast.Usage); ok {
				stmts[i] = constraintBlockOrder(nested, node.Members)
			}
		}
	}
}

func constraintBlockOrder(block Block, members []ast.Node) Block {
	if block.Graph == nil {
		block.Statements = discardMatchedSuccessions(members, block.Statements)
		block.Order = constraintStatementOrder(block.Scope, members, block.Statements)
		constraintNestedOrders(block.Statements)
		return block
	}
	if steps, ok := block.Graph.StatementList(); ok && block.Stated {
		block.Order = constraintStatementOrder(block.Scope, members, steps)
	} else {
		block.Order = ConstraintBodyStatementOrder(block.Scope, nil)
	}
	constraintGraphStatementOrders(block.Graph)
	return block
}

func constraintGraphStatementOrders(graph *ActionGraph) {
	if graph == nil {
		return
	}
	if graph.StatementOrders == nil {
		graph.StatementOrders = make(map[ast.Node]*StatementOrder)
	}
	for _, node := range graph.Nodes {
		stmts := graph.Bodies[node]
		nodeMembers := ast.NodeBodyMembers(node)
		if usage, ok := node.(*ast.Usage); ok {
			nodeMembers = usage.Members
		}
		scope := graph.Scopes[node]
		if scope == nil {
			scope = graph.Scope
		}
		if len(stmts) > 0 {
			stmts = discardMatchedSuccessions(nodeMembers, stmts)
			graph.Bodies[node] = stmts
			graph.StatementOrders[node] = constraintStatementOrder(scope, nodeMembers, stmts)
			constraintNestedOrders(stmts)
		}
		if subflow := graph.Subflows[node]; subflow != nil && subflow.Graph != nil {
			if steps, ok := subflow.Graph.StatementList(); ok {
				graph.StatementOrders[node] = constraintStatementOrder(scope, nodeMembers, steps)
			}
		}
	}
	for _, subflow := range graph.Subflows {
		if subflow != nil {
			constraintGraphStatementOrders(subflow.Graph)
		}
	}
}
