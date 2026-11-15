package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// ConstraintResultCondition is the closing bare condition of sym's own body, the result
// the constraint evaluates (SysML.xtext ConstraintBody); nil when the body ends otherwise.
func ConstraintResultCondition(sym *symbols.Symbol) *ast.ConstraintMember {
	if sym == nil || !FunctionLike(sym) {
		return nil
	}
	members := declMembers(sym)
	if len(members) == 0 {
		return nil
	}
	last := members[len(members)-1]
	if m, ok := last.(*ast.Membership); ok {
		last = m.Member
	}
	c, ok := last.(*ast.ConstraintMember)
	if !ok || c.Keyword != "" || c.Name != "" || c.Body != nil || c.Expression == nil {
		return nil
	}
	return c
}

// ConstraintResultMember is the expression element sym's closing bare condition declares,
// owned through a ResultExpressionMembership as the conversion writes it; nil when sym states none.
func (m *Model) ConstraintResultMember(sym *symbols.Symbol) *symbols.Symbol {
	if m == nil || sym == nil || sym.Implicit != nil || sym.Expr != nil {
		return nil
	}
	if cached, ok := m.constraintResults[sym]; ok {
		return cached
	}
	c := ConstraintResultCondition(sym)
	if c == nil {
		return nil
	}
	if m.constraintResults == nil {
		m.constraintResults = make(map[*symbols.Symbol]*symbols.Symbol)
	}
	journal(m, m.constraintResults, sym, sym.Decl)
	result := &symbols.Symbol{
		Kind:       symbols.SymbolExpression,
		DocName:    sym.DocName,
		Decl:       c.Expression,
		DeclSpan:   c.Expression.Span(),
		Visibility: sym.Visibility,
		Expr:       &symbols.ExpressionElement{Owner: sym, Role: symbols.ExpressionBodyResult},
	}
	m.constraintResults[sym] = result
	return result
}

// withConstraintResult adds the result expression element sym's closing bare
// condition declares after members, the body's last member.
func (m *Model) withConstraintResult(sym *symbols.Symbol, members []*symbols.Symbol) []*symbols.Symbol {
	result := m.ConstraintResultMember(sym)
	if result == nil {
		return members
	}
	return append(append([]*symbols.Symbol{}, members...), result)
}
