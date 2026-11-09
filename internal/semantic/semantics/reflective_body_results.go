package semantics

import (
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// resultExpressionMembers are the expressions a function-like type's body states as results
// (`calc def C { x + 1 }`), each the member of a ResultExpressionMembership (KerML 8.3.4.6); synthesized once.
func (m *Model) resultExpressionMembers(sym *symbols.Symbol) []*symbols.Symbol {
	if m == nil || sym == nil || sym.Implicit != nil || sym.Expr != nil || sym.Recorded() {
		return nil
	}
	if cached, ok := m.resultExpressionSymbols[sym]; ok {
		return cached
	}
	var out []*symbols.Symbol
	if !FunctionLike(sym) {
		return nil
	}
	for _, member := range bodyMembers(sym) {
		if wrapper, ok := member.(*ast.Membership); ok {
			member = wrapper.Member
		}
		if !IsResultExpression(member) {
			continue
		}
		if condition, ok := member.(*ast.ConstraintMember); ok {
			if condition.Expression == nil || scopeDeclaresNode(sym.Scope, member) {
				// A declared constraint member is a member of its own.
				continue
			}
			member = condition.Expression
		}
		out = append(out, &symbols.Symbol{
			Kind:       symbols.SymbolExpression,
			Decl:       member,
			DeclSpan:   member.Span(),
			DocName:    sym.DocName,
			Visibility: sym.Visibility,
			OwnerScope: sym.Scope,
			Expr:       &symbols.ExpressionElement{Owner: sym, Role: symbols.ExpressionBodyResult},
		})
	}
	if m.resultExpressionSymbols == nil {
		m.resultExpressionSymbols = make(map[*symbols.Symbol][]*symbols.Symbol)
	}
	m.resultExpressionSymbols[sym] = out
	return out
}

// filterMemberships are the ElementFilterMemberships a namespace's `filter <cond>;` members write
// (KerML 8.2.4), each owning its condition, minted as the conversion writes them; synthesized once.
func (m *Model) filterMemberships(sym *symbols.Symbol) []*symbols.Symbol {
	if m == nil || sym == nil || sym.Implicit != nil || sym.Expr != nil || sym.Recorded() {
		return nil
	}
	if cached, ok := m.filterMembershipSymbols[sym]; ok {
		return cached
	}
	var out []*symbols.Symbol
	for _, member := range bodyMembers(sym) {
		if wrapper, ok := member.(*ast.Membership); ok {
			member = wrapper.Member
		}
		filter, ok := member.(*ast.FilterMember)
		if !ok || filter.Condition == nil {
			continue
		}
		membership := &symbols.Symbol{
			Kind:       symbols.SymbolRelationship,
			DocName:    sym.DocName,
			DeclSpan:   filter.Span(),
			Visibility: ast.VisibilityPrivate,
			Implicit:   &symbols.ImplicitRelationship{Owner: sym, Form: symbols.ImplicitMembership, Decl: filter},
		}
		membership.Implicit.Member = &symbols.Symbol{
			Kind:       symbols.SymbolExpression,
			Decl:       filter.Condition,
			DeclSpan:   filter.Condition.Span(),
			DocName:    sym.DocName,
			Visibility: ast.VisibilityPrivate,
			OwnerScope: sym.Scope,
			Expr:       &symbols.ExpressionElement{Owner: membership, Role: symbols.ExpressionOperand, Slot: "filter"},
		}
		out = append(out, membership)
	}
	if m.filterMembershipSymbols == nil {
		m.filterMembershipSymbols = make(map[*symbols.Symbol][]*symbols.Symbol)
	}
	m.filterMembershipSymbols[sym] = out
	return out
}

// IsFilterMembership reports whether sym reflects a `filter <cond>;` member.
func IsFilterMembership(sym *symbols.Symbol) bool {
	if sym == nil || sym.Implicit == nil {
		return false
	}
	if _, ok := sym.Implicit.Decl.(*ast.FilterMember); ok {
		return true
	}
	return sym.Implicit.Form == symbols.ImplicitMembership && IsFilterPackage(sym.Implicit.Owner)
}

// mergeMemberships places extra memberships among out in source order and renumbers
// the ordinals of every synthesized membership.
func mergeMemberships(out, extra []*symbols.Symbol) []*symbols.Symbol {
	if len(extra) == 0 {
		return out
	}
	out = append(append([]*symbols.Symbol{}, out...), extra...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].DeclSpan.Offset < out[j].DeclSpan.Offset })
	for i, membership := range out {
		if membership.Implicit != nil {
			membership.Implicit.Ordinal = i
		}
	}
	return out
}

// bodyMembers are the nodes a declaration's body writes, a package's included.
func bodyMembers(sym *symbols.Symbol) []ast.Node {
	if pkg, ok := sym.Decl.(*ast.Package); ok {
		return pkg.Members
	}
	return declMembers(sym)
}

// scopeDeclaresNode reports whether scope holds a symbol declared by node.
func scopeDeclaresNode(scope *symbols.Scope, node ast.Node) bool {
	if scope == nil {
		return false
	}
	found := false
	scope.ForEachMember(func(member *symbols.Symbol) bool {
		if member != nil && member.Decl == node {
			found = true
			return false
		}
		return true
	})
	return found
}
