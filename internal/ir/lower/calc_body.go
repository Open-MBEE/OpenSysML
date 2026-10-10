package lower

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// CalcBody lowers the computation a calculation or case body states, in
// declaration order and in the scope it was written in. A member stating no
// computation — an input parameter, documentation, a nested definition — is
// skipped. A result the body declares names the value answered with, not a
// step, so it runs after the steps; a `return` inside a branch or loop stops
// the body there. owner is the declaration whose body members are: a case's
// body whose members include action nodes performs them as its steps, lowered
// as one Block over the flow they state (caseSteps); a calc's does not.
// Without a resolver the flow's `@Probability` annotations go unread, as in
// ToActionGraph; a caller holding one uses CalcBodyWith.
func CalcBody(owner ast.Node, members []ast.Node, scope *symbols.Scope) []Statement {
	return CalcBodyWith(owner, members, scope, nil)
}

// CalcBodyWith is CalcBody reading the metadata the resolver identifies: a
// weighted succession among a case's steps keeps its weight.
func CalcBodyWith(owner ast.Node, members []ast.Node, scope *symbols.Scope, resolver *resolve.Resolver) []Statement {
	body, _ := CalcBodyWithOrder(owner, members, scope, resolver)
	return body
}

// CalcBodyWithOrder lowers the body and its calculation-statement order.
func CalcBodyWithOrder(owner ast.Node, members []ast.Node, scope *symbols.Scope, resolver *resolve.Resolver) ([]Statement, *StatementOrder) {
	body := make([]ast.Node, 0, len(members))
	for _, member := range members {
		if actual := unwrapMembership(member); actual != nil {
			body = append(body, actual)
		}
	}
	if PerformsSteps(owner) && len(flowNodesAmong(body)) > 0 {
		stmts := caseSteps(owner, body, scope, resolver)
		return stmts, nil
	}
	stmts, order := calcBodyStatements(body, scope, resolver, !PerformsSteps(owner))
	if PerformsSteps(owner) {
		return stmts, nil
	}
	for i, stmt := range stmts {
		stmts[i] = calcStatementOrders(stmt)
	}
	return stmts, order
}

func calcBodyStatements(body []ast.Node, scope *symbols.Scope, resolver *resolve.Resolver, ignoreNonStatementSuccessions bool) ([]Statement, *StatementOrder) {
	members := make([]ast.Node, 0, len(body))
	for _, member := range body {
		if actual := unwrapMembership(member); actual != nil && !isAnnotation(actual) {
			members = append(members, actual)
		}
	}
	var stmts, results []Statement
	for _, member := range members {
		if _, ok := member.(*ast.SuccessionEdge); ok {
			continue
		}
		stmt, ok := calcBodyStep(member, scope, resolver, ignoreNonStatementSuccessions)
		if !ok {
			continue
		}
		if _, isResult := stmt.(Return); isResult {
			results = append(results, stmt)
			continue
		}
		stmts = append(stmts, stmt)
	}
	bodyStmts := append(stmts, results...)
	order := CalcBodyStatementOrder(scope, members, CalcSteps(bodyStmts))
	return bodyStmts, order
}

func calcBodyStep(member ast.Node, scope *symbols.Scope, resolver *resolve.Resolver, ignoreNonStatementSuccessions bool) (Statement, bool) {
	if ignoreNonStatementSuccessions {
		if usage, ok := member.(*ast.Usage); ok && (usage.IsSuccessionFlow() || usage.Kind == ast.UsageSuccession) {
			return nil, false
		}
	}
	return calcStep(member, scope, resolver)
}

// CalcSteps returns the statements an invocation performs, excluding bindings
// of output features that are read when their values are needed.
func CalcSteps(body []Statement) []Statement {
	steps := make([]Statement, 0, len(body))
	for _, stmt := range body {
		ret, ok := stmt.(Return)
		if ok && isOutputBinding(ret.Node) {
			continue
		}
		steps = append(steps, stmt)
	}
	return steps
}

func isOutputBinding(node ast.Node) bool {
	usage, ok := node.(*ast.Usage)
	return ok && usage.Direction == ast.DirOut && !usage.IsResult
}

// calcStep lowers one member of a calculation body and reports whether it
// states a step or a result; a result is a Return.
func calcStep(member ast.Node, scope *symbols.Scope, resolver *resolve.Resolver) (Statement, bool) {
	switch m := member.(type) {
	case *ast.Usage:
		if m.Direction == ast.DirIn || m.Direction == ast.DirInOut {
			// An input parameter is bound by the invocation, not by the body.
			return nil, false
		}
		if m.IsSuccessionFlow() || m.Kind == ast.UsageSuccession {
			return Unsupported{Description: statedFlowKeyword(member), Node: member, Scope: scope}, true
		}
		return usageStatement(m, scope)
	case *ast.Definition, *ast.Documentation, *ast.Comment, *ast.Import, *ast.Alias:
		// Declares a member of the calculation, not a step of it.
		return nil, false
	case *ast.SuccessionEdge:
		return nil, false
	case *ast.ControlFlowEdge, *ast.InitialNode, *ast.ForkNode, *ast.JoinNode,
		*ast.MergeNode, *ast.DecisionNode, *ast.FinalNode:
		return Unsupported{Description: statedFlowKeyword(member), Node: member, Scope: scope}, true
	default:
		if ast.IsExpression(member) {
			return Return{Value: member, Node: member, Scope: scope}, true
		}
		return lowerStatement(member, scope, resolver, nil), true
	}
}

func calcBlock(block Block, members []ast.Node) Block {
	normalized := make([]ast.Node, 0, len(members))
	for _, member := range members {
		if actual := unwrapMembership(member); actual != nil && !isAnnotation(actual) {
			normalized = append(normalized, actual)
		}
	}
	if block.Graph == nil {
		block.Order = CalcBodyStatementOrder(block.Scope, normalized, block.Statements)
		for i, stmt := range block.Statements {
			block.Statements[i] = calcStatementOrders(stmt)
		}
	} else {
		if steps, ok := block.Graph.StatementList(); ok && block.Stated {
			block.Order = CalcBodyStatementOrder(block.Scope, normalized, steps)
		} else {
			block.Order = CalcBodyStatementOrder(block.Scope, nil, nil)
		}
		calcGraphStatementOrders(block.Graph)
	}
	return block
}

func calcGraphStatementOrders(graph *ActionGraph) {
	if graph == nil {
		return
	}
	if graph.StatementOrders == nil {
		graph.StatementOrders = make(map[ast.Node]*StatementOrder)
	}
	for _, node := range graph.Nodes {
		stmts := graph.Bodies[node]
		members := ast.NodeBodyMembers(node)
		if usage, ok := node.(*ast.Usage); ok {
			members = usage.Members
		}
		scope := graph.Scopes[node]
		if scope == nil {
			scope = graph.Scope
		}
		if len(stmts) > 0 {
			graph.StatementOrders[node] = CalcBodyStatementOrder(scope, members, stmts)
		}
		for i, stmt := range stmts {
			stmts[i] = calcStatementOrders(stmt)
		}
		graph.Bodies[node] = stmts
		if subflow := graph.Subflows[node]; subflow != nil && subflow.Graph != nil {
			if steps, ok := subflow.Graph.StatementList(); ok {
				graph.StatementOrders[node] = CalcBodyStatementOrder(scope, members, steps)
			}
		}
	}
	for _, subflow := range graph.Subflows {
		if subflow != nil {
			calcGraphStatementOrders(subflow.Graph)
		}
	}
}

func calcStatementOrders(stmt Statement) Statement {
	switch nested := stmt.(type) {
	case If:
		if node, ok := nested.Node.(*ast.IfActionNode); ok {
			if node.Then != nil {
				nested.Then = calcBlock(nested.Then, node.Then.Body)
			}
			if node.Else != nil && nested.Else != nil {
				els := calcBlock(*nested.Else, node.Else.Body)
				nested.Else = &els
			}
		}
		return nested
	case Loop:
		if node, ok := nested.Node.(*ast.WhileLoopActionNode); ok {
			nested.Body = calcBlock(nested.Body, node.Body)
		}
		return nested
	case Block:
		if node, ok := nested.Node.(*ast.Usage); ok {
			return calcBlock(nested, node.Members)
		}
	}
	return stmt
}

func calcSuccessionEnd(member ast.Node, name *ast.QualifiedName, stmts []Statement, index map[ast.Node]int) (int, bool) {
	if member != nil {
		node := unwrapMembership(member)
		i, ok := index[node]
		return i, ok
	}
	if name == nil || len(name.Parts) == 0 {
		return 0, false
	}
	target := name.Parts[len(name.Parts)-1].Text
	for i, stmt := range stmts {
		if _, result := stmt.(Return); result {
			continue
		}
		if unsupported, ok := stmt.(Unsupported); ok {
			if _, succession := unsupported.Node.(*ast.SuccessionEdge); succession {
				continue
			}
		}
		if statementDeclares(statementNode(stmt), target) {
			return i, true
		}
	}
	return 0, false
}

func statementNode(stmt Statement) ast.Node {
	switch s := stmt.(type) {
	case Assign:
		return s.Node
	case Block:
		return s.Node
	case Declare:
		return s.Node
	case DeclareUsage:
		return s.Node
	case Effect:
		return s.Node
	case If:
		return s.Node
	case Loop:
		return s.Node
	case Return:
		return s.Node
	case Send:
		return s.Node
	case Unsupported:
		return s.Node
	default:
		return nil
	}
}

func statementDeclares(node ast.Node, name string) bool {
	switch n := node.(type) {
	case *ast.Usage:
		actual, _ := ast.EffectiveName(n)
		return actual == name
	case *ast.InitialNode:
		return n.Name() == name
	}
	return false
}

// usageStatement lowers a usage written in a statement position: a bound result
// parameter returns the value it binds, an accept parameter states an effect,
// and an attribute declares a value the statements around it read and write.
func usageStatement(u *ast.Usage, scope *symbols.Scope) (Statement, bool) {
	if u.IsAccept {
		return Effect{Kind: EffectAccept, Node: u, Scope: scope}, true
	}
	name, _ := ast.EffectiveName(u)
	// A result parameter binding no value only names the result, so it states no
	// step of the computation.
	if u.IsResult || u.Direction == ast.DirOut {
		if u.Value == nil {
			return nil, false
		}
		// An initial value (`:=`) is what the output holds when the body starts,
		// for its assignments to replace; a binding (`=`) is the value it returns.
		if u.ValueIsInitial && name != "" {
			return Declare{Name: name, Value: u.Value, Binding: valueIsBinding(u.Value, u.ValueIsInitial, u.ValueIsDefault), Node: u, Scope: scope}, true
		}
		return Return{Value: u.Value, Node: u, Scope: scope}, true
	}
	if u.Kind == ast.UsageAttribute && name != "" {
		return Declare{Name: name, Value: u.Value, Binding: valueIsBinding(u.Value, u.ValueIsInitial, u.ValueIsDefault), Node: u, Scope: scope}, true
	}
	if (u.Kind == ast.UsageCalc || u.Kind == ast.UsageAnalysisCase || u.Kind == ast.UsageVerificationCase) && name != "" {
		return DeclareUsage{Name: name, Node: u, Scope: scope}, true
	}
	return nil, false
}

// Returns reports whether the statements return a value on some path, so a
// calculation whose body only computes can be told from one that has no result.
func Returns(stmts []Statement) bool {
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case Return:
			return true
		case If:
			if blockReturns(s.Then) {
				return true
			}
			if s.Else != nil && blockReturns(*s.Else) {
				return true
			}
		case Loop:
			if blockReturns(s.Body) {
				return true
			}
		case Block:
			if blockReturns(s) {
				return true
			}
		}
	}
	return false
}

// blockReturns reports whether a block returns a value on some path, wherever
// its statements live.
func blockReturns(block Block) bool {
	return Returns(block.Steps())
}
