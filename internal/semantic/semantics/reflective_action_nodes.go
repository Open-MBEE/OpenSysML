package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// statementNodes are the anonymous action nodes (SysML v2 §7.16.8) sym's body writes as statements,
// synthesized here because the symbol table scopes their bodies but registers no symbol for them.
func (m *Model) statementNodes(sym *symbols.Symbol) []*symbols.Symbol {
	if m == nil || sym == nil || sym.Implicit != nil || sym.Expr != nil || sym.Recorded() {
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
	for _, member := range statementBodyOf(sym.Decl) {
		if wrapper, ok := member.(*ast.Membership); ok {
			member = wrapper.Member
		}
		if !IsStatementNode(member) || nodeStatementOf(sym.Decl) == member {
			continue
		}
		node := &symbols.Symbol{
			Kind:       symbols.SymbolActionUsage,
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
		m.statementOwners[node] = sym
		if node.Scope != nil {
			m.statementScopes[node.Scope] = node
		}
		out = append(out, node)
	}
	if m.statementSymbols == nil {
		m.statementSymbols = make(map[*symbols.Symbol][]*symbols.Symbol)
	}
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
	return m.statementScopeOwner(sym.OwnerScope)
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
	for _, node := range m.statementNodes(owner) {
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

// statementBodyOf lists the members a declaration's body writes, where its
// statements stand.
func statementBodyOf(decl ast.Node) []ast.Node {
	switch d := decl.(type) {
	case *ast.Usage:
		return d.Members
	case *ast.Definition:
		return d.Members
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
	if m == nil || sym == nil || sym.Implicit != nil || sym.Expr != nil || sym.Recorded() {
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
		if d.Kind == ast.LoopFor {
			add("collection", d.Collection)
		} else {
			add("whileCondition", d.Condition)
			add("untilCondition", d.Until)
		}
	case *ast.AssignmentActionNode:
		add("target", d.Target)
		add("value", d.Value)
	case *ast.SendStatement:
		add("payload", d.Message)
		add("receiver", d.Target)
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
	m.statementOperandSymbols[sym] = out
	return out
}

// NodeStatementOf is the send or assignment a plain-headed action node is (`action a assign x := y;`,
// SysML.xtext SendNode/AssignmentNode); nil for any other usage, which keeps the action it declares.
func NodeStatementOf(n *ast.Usage) ast.Node {
	if n == nil || n.Kind != ast.UsageAction || !n.IsActionNode || len(n.Members) != 1 || !PlainNodeHead(n) {
		return nil
	}
	statement := n.Members[0]
	if m, ok := statement.(*ast.Membership); ok {
		statement = m.Member
	}
	switch s := statement.(type) {
	case *ast.SendStatement:
		if !s.HasBody {
			return s
		}
	case *ast.AssignmentActionNode:
		return s
	}
	return nil
}

// PlainNodeHead reports whether an action node declares nothing but its visibility
// and name: no prefix, modifier, specialization, multiplicity or value.
func PlainNodeHead(n *ast.Usage) bool {
	return len(n.Prefixes) == 0 && n.PrefixKeyword == "" && len(n.Relationships) == 0 &&
		n.Multiplicity == nil && n.CrossFeature == nil && n.Value == nil &&
		!n.IsAbstract && !n.IsVariation && !n.IsVariant && !n.IsReference && !n.IsVariable &&
		!n.IsIndividual && n.Portion == ast.PortionNone && !n.IsPortion && n.Direction == ast.DirNone &&
		!n.IsEnd && !n.IsConstant && !n.IsDerived && !n.IsOrdered && !n.IsNonunique && !n.IsParallel
}

// nodeStatementOf is NodeStatementOf for any declaration.
func nodeStatementOf(decl ast.Node) ast.Node {
	if u, ok := decl.(*ast.Usage); ok {
		return NodeStatementOf(u)
	}
	return nil
}
