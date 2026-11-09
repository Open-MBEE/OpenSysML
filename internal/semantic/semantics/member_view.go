package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// Member is one member of a type as KerML Type::member reports it: the symbol,
// whether sym inherits it rather than declares it, and the type whose owned
// member it is (sym itself for an own member).
type Member struct {
	Symbol    *symbols.Symbol
	Inherited bool
	Source    *symbols.Symbol
}

// Members returns the members of sym, named and anonymous, with their
// provenance: its own members in declaration order, then the members it
// inherits in MemberSources order (KerML 8.3.3.1 Type::member, the owned
// memberships followed by the inherited ones). An inherited member is one its
// supertypes own and sym does not redefine (Type::inheritedMembership, a
// redefinition masking what it redefines, masking.go), a closer declaration of
// the same name hiding a farther one (MembersOf), and a private member of a
// supertype is not inherited at all (Type::inheritableMemberships). A member
// owned through an Annotation (a `#M` prefix) is no member of the element.
func (m *Model) Members(sym *symbols.Symbol) []Member {
	if sym == nil {
		return nil
	}
	named := m.MembersOf(sym)
	namedSet := make(map[*symbols.Symbol]bool, len(named))
	for _, s := range named {
		namedSet[s] = true
	}
	var out []Member
	seen := make(map[*symbols.Symbol]bool, len(named))
	mask := m.redefinitionMask(sym, true)
	collect := func(source *symbols.Symbol, inherited bool) {
		scope := source.Scope
		if scope == nil {
			return
		}
		anonymous := make(map[*symbols.Symbol]bool)
		for _, s := range scope.AnonymousMembers() {
			anonymous[s] = true
		}
		scope.ForEachMember(func(s *symbols.Symbol) bool {
			if s.Kind == symbols.SymbolAlias || seen[s] || ownedThroughAnnotation(source, s) ||
				(!anonymous[s] && !namedSet[s] && !(s.Kind == symbols.SymbolDocumentation && s.Name == "")) ||
				(inherited && (m.maskedBy(mask, s) || !Inheritable(s))) {
				return true
			}
			seen[s] = true
			out = append(out, Member{Symbol: s, Inherited: inherited, Source: source})
			return true
		})
	}
	collect(sym, false)
	for _, src := range m.MemberSources(sym) {
		collect(src, true)
	}
	// A named member MembersOf reports from a scope no source owns — a
	// cached library symbol without a scope — is still a member.
	for _, s := range named {
		if seen[s] || !Inheritable(s) {
			continue
		}
		seen[s] = true
		out = append(out, Member{Symbol: s, Inherited: true, Source: s.OwnerScope.Owner()})
	}
	return out
}

// Inheritable reports whether a member may be inherited by what specializes
// its owner: every membership but a private one (KerML 8.3.3.1
// Type::inheritableMemberships; a private member is visible only inside the
// namespace declaring it, 8.2.3.5).
func Inheritable(member *symbols.Symbol) bool {
	return member != nil && member.Visibility != ast.VisibilityPrivate
}
