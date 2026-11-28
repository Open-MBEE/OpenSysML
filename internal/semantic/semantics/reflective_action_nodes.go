package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// statementNodes are the anonymous action nodes (SysML v2 §7.16.8) sym's body writes as statements,
// synthesized here because the symbol table scopes their bodies but registers no symbol for them.
func (m *Model) statementNodes(sym *symbols.Symbol) []*symbols.Symbol {
	if m == nil || sym == nil || sym.Implicit != nil || sym.Expr != nil && !isLoopBody(sym) || sym.Recorded() || IsLoopVariable(sym) {
		// A `for` variable shares its loop's node but owns none of the loop's body.
		return nil
	}
	if cached, ok := m.statementSymbols[sym]; ok {
		return cached
	}
	// A conditional owns no scope of its own: its branches' bodies are scoped
	// in the namespace the statement is written in.
	lookup := sym.Scope
	if lookup == nil {
		lookup = sym.OwnerScope
	}
	var out []*symbols.Symbol
	body := statementBodyOf(sym.Decl)
	if statement := nodeStatementOf(sym.Decl); statement != nil && !isLoopBody(sym) {
		// A node written as its statement (`action a if c … else …`, `action a
		// while c { … }`) owns the statement's branches and body as its own.
		body = statementBodyOf(statement)
		if lookup != nil {
			if child := lookup.ChildFor(statement); child != nil {
				lookup = child
				if m.statementScopes == nil {
					m.statementOwners = make(map[*symbols.Symbol]*symbols.Symbol)
					m.statementScopes = make(map[*symbols.Scope]*symbols.Symbol)
				}
				journal(m, m.statementScopes, child, statement)
				m.statementScopes[child] = sym
			}
		}
	}
	if loop := loopStatementOf(sym); loop != nil && loop.BodyParameter() == nil {
		// A loop owns its `{ … }` body as one action, which owns what the body writes
		// (SysML.xtext ActionBodyParameterMember).
		out = append(out, &symbols.Symbol{
			Kind:       symbols.SymbolActionUsage,
			Decl:       loop,
			DeclSpan:   loopBodySpan(loop),
			DocName:    sym.DocName,
			Visibility: sym.Visibility,
			Scope:      lookup,
			Expr:       &symbols.ExpressionElement{Owner: sym, Role: symbols.ExpressionLoopBody},
		})
		body = nil
	}
	for _, member := range body {
		if wrapper, ok := member.(*ast.Membership); ok {
			member = wrapper.Member
		}
		if !IsStatementNode(member) || lookup != nil && lookup.MemberDeclaring(member) != nil {
			// A bare assign, terminate or perform is the action usage the builder declares (statementNodeKind).
			continue
		}
		kind := symbols.SymbolActionUsage
		if isBodySuccessionNode(member) {
			kind = symbols.SymbolSuccessionUsage
		}
		node := &symbols.Symbol{
			Kind:       kind,
			Decl:       member,
			DeclSpan:   member.Span(),
			DocName:    sym.DocName,
			Visibility: sym.Visibility,
			OwnerScope: lookup,
		}
		if lookup != nil {
			node.Scope = lookup.ChildFor(member)
		}
		if m.statementOwners == nil {
			m.statementOwners = make(map[*symbols.Symbol]*symbols.Symbol)
			m.statementScopes = make(map[*symbols.Scope]*symbols.Symbol)
		}
		journal(m, m.statementOwners, node, member)
		m.statementOwners[node] = sym
		if node.Scope != nil {
			journal(m, m.statementScopes, node.Scope, member)
			m.statementScopes[node.Scope] = node
		}
		out = append(out, node)
	}
	if m.statementSymbols == nil {
		m.statementSymbols = make(map[*symbols.Symbol][]*symbols.Symbol)
	}
	journal(m, m.statementSymbols, sym, sym.Decl)
	m.statementSymbols[sym] = out
	return out
}

// statementOwner is the owner the scope table cannot name: a synthesized node's
// owner, or the statement node whose body scope declares sym.
func (m *Model) statementOwner(sym *symbols.Symbol) *symbols.Symbol {
	if m == nil || sym == nil || sym.Expr != nil || sym.Implicit != nil {
		return nil
	}
	if owner, ok := m.statementOwners[sym]; ok {
		return owner
	}
	owner := m.statementScopeOwner(sym.OwnerScope)
	if IsLoopVariable(sym) {
		// The loop owns its variable itself; its body owns the scope's other members.
		return owner
	}
	return m.loopBodyOr(owner)
}

// loopBodyOr is the body of the loop owner, which owns what the loop's scope declares, or owner
// itself when it is no loop.
func (m *Model) loopBodyOr(owner *symbols.Symbol) *symbols.Symbol {
	if loopStatementOf(owner) == nil {
		return owner
	}
	if nodes := m.statementNodes(owner); len(nodes) == 1 {
		return nodes[0]
	}
	return owner
}

// loopStatementOf is the loop sym is: a bare loop statement's node or a node written as a loop
// (`action a while c { … }`); nil for the loop's variable and body, and for any other symbol.
func loopStatementOf(sym *symbols.Symbol) *ast.WhileLoopActionNode {
	if sym == nil || sym.Expr != nil || sym.Implicit != nil || IsLoopVariable(sym) {
		return nil
	}
	if loop, ok := sym.Decl.(*ast.WhileLoopActionNode); ok {
		return loop
	}
	loop, _ := nodeStatementOf(sym.Decl).(*ast.WhileLoopActionNode)
	return loop
}

// isWrittenLoopBody reports whether sym is the action a loop's body is written as (ast.WhileLoopActionNode.BodyParameter).
func isWrittenLoopBody(sym *symbols.Symbol) bool {
	if sym == nil || sym.Expr != nil || sym.Implicit != nil || sym.OwnerScope == nil {
		return false
	}
	loop, ok := sym.OwnerScope.Node().(*ast.WhileLoopActionNode)
	return ok && loop.BodyParameter() == sym.Decl
}

// isLoopBody reports whether sym is the action a loop's body is (statementNodes).
func isLoopBody(sym *symbols.Symbol) bool {
	return sym != nil && sym.Expr != nil && sym.Expr.Role == symbols.ExpressionLoopBody
}

// loopBodySpan places a loop's body: after what its head writes, up to its `until` condition.
func loopBodySpan(loop *ast.WhileLoopActionNode) source.Span {
	span := loop.Span()
	start, end := span.Offset, span.End()
	head, until := loop.Condition, loop.Until
	switch loop.Kind {
	case ast.LoopFor:
		head = loop.Collection
	case ast.LoopUntil:
		// `loop { … } until c;` states its one condition after the body.
		head, until = nil, loop.Condition
	}
	if head != nil {
		start = head.Span().End()
	}
	if until != nil {
		end = until.Span().Offset
	}
	return source.Span{Offset: start, Len: max(end-start, 0)}
}

// statementScopeOwner is the synthesized statement node whose body scope is
// scope; nil when scope belongs to a symbol or to no statement.
func (m *Model) statementScopeOwner(scope *symbols.Scope) *symbols.Symbol {
	if scope == nil || scope.Owner() != nil || !IsStatementNode(scope.Node()) {
		return nil
	}
	if node, ok := m.statementScopes[scope]; ok {
		return node
	}
	parent := scope.Parent()
	if parent == nil {
		return nil
	}
	owner := parent.Owner()
	if owner == nil {
		owner = m.statementScopeOwner(parent)
	}
	if owner == nil {
		return nil
	}
	if body := m.loopBodyOr(owner); body != owner {
		if body.Scope == scope {
			return owner
		}
		// What a loop's body writes is scoped under the loop and owned by its body.
		owner = body
	}
	nodes := m.statementNodes(owner)
	// A node written as its statement (`action lp for i in s { … }`) is registered as its body's owner.
	if node, ok := m.statementScopes[scope]; ok {
		return node
	}
	for _, node := range nodes {
		if node.Scope == scope {
			return node
		}
		if _, ok := node.Decl.(*ast.IfActionNode); ok {
			for _, branch := range m.statementNodes(node) {
				if branch.Scope == scope {
					return branch
				}
			}
		}
	}
	return nil
}

// IsStatementNode reports whether node is a statement the reflective layer
// synthesizes an action node for.
func IsStatementNode(node ast.Node) bool {
	switch node.(type) {
	case *ast.InitialNode:
		// `first a then b;` is the SuccessionAsUsage the builder declares; `first a;` is the body's start marker.
		return false
	case *ast.IfActionNode, *ast.IfBranchNode, *ast.WhileLoopActionNode, *ast.AssignmentActionNode,
		*ast.TerminateStatement, *ast.PerformActionNode, *ast.ActionExecutionNode:
		return true
	case *ast.ControlFlowEdge:
		// A decision's branch (`if g then b;`, `else b;`) is a transition usage
		// owning its guard and a succession (SysML.xtext TargetTransitionUsage).
		return true
	}
	return false
}

// IsLoopVariable reports whether sym is a `for` loop's variable, which declares
// no node of its own: its symbol is keyed by the loop.
func IsLoopVariable(sym *symbols.Symbol) bool {
	if sym == nil || sym.Kind != symbols.SymbolReferenceUsage {
		return false
	}
	_, loop := sym.Decl.(*ast.WhileLoopActionNode)
	return loop
}

// statementBodyOf lists the members a declaration's body writes, where its
// statements stand.
func statementBodyOf(decl ast.Node) []ast.Node {
	switch d := decl.(type) {
	case *ast.Usage:
		return withSubactionActions(d.Members)
	case *ast.Definition:
		return withSubactionActions(d.Members)
	case *ast.IfActionNode:
		out := make([]ast.Node, 0, 2)
		for _, branch := range d.Branches() {
			out = append(out, branch)
		}
		return out
	case *ast.IfBranchNode:
		return d.Body
	case *ast.WhileLoopActionNode:
		return d.Body
	}
	return ast.NodeBodyMembers(decl)
}

// statementOperands are the expressions an action node statement writes at its
// own slots, named as the conversion mints them (condition, target, value, ...).
func (m *Model) statementOperands(sym *symbols.Symbol) []*symbols.Symbol {
	if m == nil || sym == nil || sym.Implicit != nil || sym.Expr != nil || sym.Recorded() || IsLoopVariable(sym) {
		return nil
	}
	if cached, ok := m.statementOperandSymbols[sym]; ok {
		return cached
	}
	var out []*symbols.Symbol
	add := func(slot string, node ast.Node) {
		if node == nil {
			return
		}
		out = append(out, &symbols.Symbol{
			Kind:       symbols.SymbolExpression,
			Decl:       node,
			DeclSpan:   node.Span(),
			DocName:    sym.DocName,
			Visibility: sym.Visibility,
			Expr:       &symbols.ExpressionElement{Owner: sym, Role: symbols.ExpressionOperand, Slot: slot},
		})
	}
	decl := sym.Decl
	if statement := nodeStatementOf(decl); statement != nil {
		// `action a assign x := y;` is the assignment: its operands are the
		// usage's own.
		decl = statement
	}
	switch d := decl.(type) {
	case *ast.IfActionNode:
		add("condition", d.Condition)
	case *ast.WhileLoopActionNode:
		switch d.Kind {
		case ast.LoopFor:
			// The collection is the value of the loop's `seq` parameter (controlPartsOf).
		case ast.LoopUntil:
			// `loop { … } until c;` tests its condition after each iteration: it is the until condition.
			add("untilCondition", d.Condition)
		default:
			add("whileCondition", d.Condition)
			add("untilCondition", d.Until)
		}
	case *ast.AssignmentActionNode:
		// The value is that of the assignment's `replacementValues` parameter (controlPartsOf).
		add("target", d.Target)
	case *ast.TerminateStatement:
		add("expression", d.Target)
	case *ast.PerformActionNode:
		add("expression", d.ActionRef)
	case *ast.ActionExecutionNode:
		if d.ActionRef != nil {
			add("expression", d.ActionRef)
		} else {
			add("expression", d.Expression)
		}
	}
	if m.statementOperandSymbols == nil {
		m.statementOperandSymbols = make(map[*symbols.Symbol][]*symbols.Symbol)
	}
	journal(m, m.statementOperandSymbols, sym, sym.Decl)
	m.statementOperandSymbols[sym] = out
	return out
}

// NodeStatementOf is the statement an action node is written as (`action a assign …`,
// `action a : A if …`; SysML v2 §8.2.2.17 ActionNode), the rule the conversion reads (naming.NodeStatement).
func NodeStatementOf(n *ast.Usage) ast.Node {
	if n == nil || n.PrefixKeyword != "" {
		return nil
	}
	switch s := symbols.ActionNodeStatementOf(n).(type) {
	case *ast.SendStatement, *ast.AssignmentActionNode, *ast.IfActionNode, *ast.WhileLoopActionNode:
		return s
	}
	return nil
}

// nodeStatementOf is NodeStatementOf for any declaration.
func nodeStatementOf(decl ast.Node) ast.Node {
	if u, ok := decl.(*ast.Usage); ok {
		return NodeStatementOf(u)
	}
	return nil
}

// isBodySuccessionNode reports whether node is a succession written as a statement
// of a body: `then b;` or `first a then b;`.
func isBodySuccessionNode(node ast.Node) bool {
	switch node := node.(type) {
	case *ast.SuccessionEdge:
		return true
	case *ast.InitialNode:
		return node.Successor != nil
	}
	return false
}

// statementParts are the operands of the statement sym with the ReferenceUsage parameters an
// assignment or loop states beside them (controlPartsOf), in the order they are written.
func (m *Model) statementParts(sym *symbols.Symbol) []*symbols.Symbol {
	operands := m.statementOperands(sym)
	if sym == nil {
		return operands
	}
	switch statementOf(sym).(type) {
	case *ast.AssignmentActionNode, *ast.WhileLoopActionNode:
	default:
		return operands
	}
	out := make([]*symbols.Symbol, 0, len(operands)+2)
	for _, param := range m.controlParameters(sym) {
		if param.Expr == nil || param.Expr.Role != symbols.ExpressionPayload || param.Kind != symbols.SymbolReferenceUsage {
			continue
		}
		for len(operands) > 0 && operands[0].DeclSpan.Offset < param.DeclSpan.Offset {
			out, operands = append(out, operands[0]), operands[1:]
		}
		out = append(out, param)
	}
	return append(out, operands...)
}

// statementOf is the statement sym is: the node synthesized for it, or the usage written as it
// (its declaration's typing and prefix included); nil where the statement is a node nested in the usage, and for anything else.
func statementOf(sym *symbols.Symbol) ast.Node {
	if sym == nil || sym.Expr != nil || sym.Implicit != nil || IsLoopVariable(sym) {
		return nil
	}
	if _, usage := sym.Decl.(*ast.Usage); usage {
		return nodeStatementOf(sym.Decl)
	}
	if IsStatementNode(sym.Decl) {
		return sym.Decl
	}
	return nil
}
