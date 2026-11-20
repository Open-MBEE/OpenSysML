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
