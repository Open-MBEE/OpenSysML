package semantics

import "github.com/Open-MBEE/OpenSysML/internal/core/symbols"

// MembersOf returns the members visible on sym: those declared directly in its
// owned scope plus members inherited from what it specializes and what it
// reference-subsets. Two maskings apply: a member declared closer to sym hides
// an inherited member of the same name, and a feature redefined by one of sym's
// features is not inherited at all (see masking.go).
// Results are deterministic: local members first (declaration order), then
// contributed members in MemberSources order.
func (m *Model) MembersOf(sym *symbols.Symbol) []*symbols.Symbol {
	return m.membersOf(sym, memberViewEffective, nil)
}

// MembersOfIncludingRedefined is MembersOf without redefinition masking: the
// members a type would have if none of its features redefined anything. A
// redefinition shares its target's feature value, so the runtime shape needs
// both.
func (m *Model) MembersOfIncludingRedefined(sym *symbols.Symbol) []*symbols.Symbol {
	return m.membersOf(sym, memberViewUnmasked, nil)
}

// MembersOfDeclaring returns the members of sym as a declaration being written
// in it sees them: that declaration is not yet a member of its own owner and
// the redefinition it carries masks nothing, so its target stays resolvable
// (KerML 8.3.3.3.6). Sym's other declarations, and the masks they cause, are
// present. A nil declaring — the caller cannot tell which declaration is being
// written — stands for every redefinition sym declares.
func (m *Model) MembersOfDeclaring(sym, declaring *symbols.Symbol) []*symbols.Symbol {
	return m.membersOf(sym, memberViewDeclaring, declaring)
}

// memberView selects which of a type's members MembersOf reports.
type memberView int

const (
	// memberViewEffective is what the type actually has: declarations plus
	// unmasked inherited members.
	memberViewEffective memberView = iota
	// memberViewUnmasked applies no redefinition mask.
	memberViewUnmasked
	// memberViewDeclaring is the view a declaration being written in the type
	// has: itself absent, and the mask it causes suspended.
	memberViewDeclaring
)

// memberKey keys a memoized members answer: the declaring view is asked per
// declaration and never memoized.
type memberKey struct {
	sym  *symbols.Symbol
	view memberView
}

func (m *Model) membersOf(sym *symbols.Symbol, view memberView, declaring *symbols.Symbol) []*symbols.Symbol {
	if sym == nil {
		return nil
	}
	if m.resolver != nil {
		if target, ok := m.resolver.ResolveAliasTarget(sym); ok {
			sym = target
		}
	}
	key := memberKey{sym: sym, view: view}
	if view != memberViewDeclaring {
		if cached, ok := m.members[key]; ok {
			return cached
		}
	}
	out := m.collectMembers(sym, view, declaring)
	// Memoized once the sources are complete and no redefinition is mid-resolution,
	// the same condition MemberSources and the constructor slots memoize under.
	if view != memberViewDeclaring && m.MemberSourcesStable(sym) && m.computingRedefinedFeatures == 0 {
		m.members[key] = out
	}
	return out
}

func (m *Model) collectMembers(sym *symbols.Symbol, view memberView, declaring *symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	m.eachMember(sym, view, declaring, func(s *symbols.Symbol) bool {
		out = append(out, s)
		return true
	})
	return out
}

// HasMember reports whether member is among MembersOf(sym), without
// enumerating the members past it.
func (m *Model) HasMember(sym, member *symbols.Symbol) bool {
	if sym == nil || member == nil {
		return false
	}
	if m.resolver != nil {
		if target, ok := m.resolver.ResolveAliasTarget(sym); ok {
			sym = target
		}
	}
	if cached, ok := m.members[memberKey{sym: sym, view: memberViewEffective}]; ok {
		return containsSymbol(cached, member)
	}
	found := false
	m.eachMember(sym, memberViewEffective, nil, func(s *symbols.Symbol) bool {
		found = s == member
		return !found
	})
	return found
}

// eachMember yields the members of sym in MembersOf order until yield returns false.
func (m *Model) eachMember(sym *symbols.Symbol, view memberView, declaring *symbols.Symbol, yield func(*symbols.Symbol) bool) {
	seenName := make(map[string]bool)
	seenSym := make(map[*symbols.Symbol]bool)
	// One mask per enumeration: it depends only on sym and declaring.
	mask := m.viewMask(sym, view, declaring)

	stopped := false
	collect := func(scope *symbols.Scope, inherited bool) {
		if scope == nil || stopped {
			return
		}
		for _, key := range scope.MemberNames() {
			if seenName[key] {
				continue // masked by a closer declaration
			}
			for _, s := range scope.LookupLocalAll(key) {
				if !m.resolver.BindsName(s) {
					continue // a derived name its target does not supply
				}
				if !inherited && view == memberViewDeclaring && NotYetMember(s, declaring) {
					continue // a feature being declared is not yet a member
				}
				if inherited && m.maskedBy(mask, s) {
					continue // redefined by a feature of sym
				}
				if !seenSym[s] {
					seenSym[s] = true
					if !yield(s) {
						stopped = true
						return
					}
				}
			}
		}
		// Mark this scope's names only after processing it, so a short+primary
		// pair in the same scope does not mask its own second key. A name no
		// kept member binds masks nothing.
		for _, key := range scope.MemberNames() {
			for _, s := range scope.LookupLocalAll(key) {
				if seenSym[s] {
					seenName[key] = true
					break
				}
			}
		}
	}

	collect(sym.Scope, false)
	for _, src := range m.MemberSources(sym) {
		if stopped {
			return
		}
		collect(src.Scope, true)
	}
}

// LookupMember returns the member sym has under name, as MembersOf reports them:
// its own declaration, else an inherited member it does not redefine, else the
// feature of sym redefining that member and so answering to its name.
func (m *Model) LookupMember(sym *symbols.Symbol, name string) (*symbols.Symbol, bool) {
	if sym == nil || name == "" {
		return nil, false
	}
	if target, ok := m.resolver.ResolveAliasTarget(sym); ok {
		sym = target
	}
	// Local first.
	if s, ok := m.resolver.LocalBinding(sym.Scope, name); ok {
		return s, true
	}
	// If no scope (cached stdlib symbol), query index for direct children
	if sym.Scope == nil {
		for _, child := range m.resolver.Index().LookupDirectChildrenNamed(sym.Name, name) {
			if leafName(child.Name) == name {
				return child, true
			}
		}
	}
	var found *symbols.Symbol
	m.eachContributedMember(sym, name, func(s *symbols.Symbol) bool {
		if !m.InheritanceMasked(sym, s) {
			found = s
		} else {
			found = m.NamingRedefiner(sym, s)
		}
		return found == nil
	})
	return found, found != nil
}

// LookupContributedMember is LookupMember without sym's own declarations: only
// the members contributed by what sym specializes, is typed by or
// reference-subsets. Callers that must not see a local binding — resolving a
// reference subsetting's target past the borrowed name it binds itself — ask
// for the contributed member instead.
func (m *Model) LookupContributedMember(sym *symbols.Symbol, name string) (*symbols.Symbol, bool) {
	var found *symbols.Symbol
	m.eachContributedMember(sym, name, func(s *symbols.Symbol) bool {
		found = s
		return false
	})
	return found, found != nil
}

// LookupContributedMembers is LookupContributedMember collecting the member
// each source contributes under name, in source order, without duplicates.
func (m *Model) LookupContributedMembers(sym *symbols.Symbol, name string) []*symbols.Symbol {
	var out []*symbols.Symbol
	m.eachContributedMember(sym, name, func(s *symbols.Symbol) bool {
		if !containsSymbol(out, s) {
			out = append(out, s)
		}
		return true
	})
	return out
}

// eachContributedMember calls yield with the member each of sym's member
// sources holds under name and passes on to sym, in name-lookup order, until
// yield returns false. A member a type on the way redefines stops there.
func (m *Model) eachContributedMember(sym *symbols.Symbol, name string, yield func(*symbols.Symbol) bool) {
	if sym == nil || name == "" {
		return
	}
	if target, ok := m.resolver.ResolveAliasTarget(sym); ok {
		sym = target
	}
	for _, src := range m.lookupSources(sym) {
		sup := src.sym
		if sup.Scope != nil {
			for _, s := range m.resolver.LocalBindings(sup.Scope, name) {
				if m.inheritedAlong(s, src.via) && !yield(s) {
					return
				}
			}
			continue
		}
		// A cached source with no scope is read from the index.
		for _, child := range m.resolver.Index().LookupDirectChildrenNamed(sup.Name, name) {
			if leafName(child.Name) == name && m.inheritedAlong(child, src.via) && !yield(child) {
				return
			}
		}
	}
}
