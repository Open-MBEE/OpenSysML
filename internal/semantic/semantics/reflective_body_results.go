package semantics

import (
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// IsFilterMembership reports whether sym reflects the ElementFilterMembership of a filter package.
func IsFilterMembership(sym *symbols.Symbol) bool {
	if sym == nil || sym.Implicit == nil {
		return false
	}
	return sym.Implicit.Form == symbols.ImplicitMembership && IsFilterPackage(sym.Implicit.Owner)
}

// FilterMemberOf is the `filter <cond>;` member that states condition, the member
// of an ElementFilterMembership (KerML 8.2.4); nil for another symbol.
func FilterMemberOf(condition *symbols.Symbol) *ast.FilterMember {
	if condition == nil || condition.Expr == nil || condition.Expr.Role != symbols.ExpressionFilterCondition || condition.Expr.Owner == nil {
		return nil
	}
	filter, _ := filterMemberStating(condition.Expr.Owner, condition.Decl)
	return filter
}

// filterMemberStating is the `filter <cond>;` member of owner whose condition is the
// node, with the visibility written ahead of it (MemberPrefix); nil when owner states none.
func filterMemberStating(owner *symbols.Symbol, condition ast.Node) (*ast.FilterMember, ast.Visibility) {
	for _, member := range bodyMembers(owner) {
		visibility := ast.VisibilityDefault
		if wrapper, ok := member.(*ast.Membership); ok {
			member, visibility = wrapper.Member, wrapper.Visibility
		}
		if filter, ok := member.(*ast.FilterMember); ok && filter.Condition == condition {
			return filter, visibility
		}
	}
	return nil, ast.VisibilityDefault
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
