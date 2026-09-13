package semantics

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// ReferencedFeature returns the feature sym reference-subsets: the target of
// the single `references` / `::>` edge its declaration may carry (KerML
// 8.3.3.3.9, "A Feature can have at most one ownedReferenceSubsetting"), or
// nil when it has none or the target does not resolve.
//
// The `perform` and `event` shorthands declare that edge without the keyword:
// `perform providePower.generateTorque` is a perform action usage whose
// performed action is related to it by reference subsetting (SysML 7.17.6).
//
// The result is memoized. Resolution of the target runs member lookup, which
// consults this relation in turn, so a symbol already being resolved yields nil
// rather than recursing.
func (m *Model) ReferencedFeature(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil {
		return nil
	}
	if cached, ok := m.referenced[sym]; ok {
		return cached
	}
	if m.resolvingRef[sym] {
		return nil
	}
	m.resolvingRef[sym] = true
	defer delete(m.resolvingRef, sym)

	var out *symbols.Symbol
	if node := referenceSubsettingTarget(sym); node != nil {
		if target, ok := m.resolver.ResolveReferenceTarget(referenceScope(sym), sym.Decl, node); ok && target != sym {
			out = target
		}
	} else if u, isUsage := sym.Decl.(*ast.Usage); isUsage && u.IsVariantReference() && sym.Name != "" {
		// A bare `variant X` is a VariantReference (SysML.xtext:642): a
		// reference usage subsetting the like-named feature visible outside.
		if named, ok := m.resolver.LookupNameExcluding(sym.OwnerScope, sym.Name, sym.Decl); ok && named != sym {
			out = named
		}
	}
	// A result computed while another symbol's reference is in flight saw a
	// truncated member view (that symbol's own reference was hidden), so it is
	// provisional and must not be cached.
	if len(m.resolvingRef) == 1 {
		m.referenced[sym] = out
	}
	return out
}

// referenceSubsettingTarget returns the node naming the feature sym
// reference-subsets, or nil when it has no such clause. A connector end carries
// that clause outside its relationship list when it is written with the
// `references` keyword, so it is asked for its own.
func referenceSubsettingTarget(sym *symbols.Symbol) ast.Node {
	switch decl := sym.Decl.(type) {
	case *ast.ConnectorEnd:
		return decl.ReferencedTarget()
	case *ast.Usage:
		if rel := decl.ReferenceSubsetting(); rel != nil {
			return rel.Target
		}
		return nil
	}
	if ref := ast.ConstraintReferenceOf(sym.Decl); ref != nil {
		return ref
	}
	for _, rel := range RelationshipsOf(sym) {
		if rel != nil && rel.Kind.ReferenceSubsets() && rel.Target != nil {
			return rel.Target
		}
	}
	return nil
}

// referenceScope returns the scope sym's reference-subsetting target is written
// in. A connector end is a member of the connector, but what it attaches to is a
// feature of the connector's owner, so it resolves one scope out — the same
// scope the document walk uses (resolve/document.go).
func referenceScope(sym *symbols.Symbol) *symbols.Scope {
	if _, ok := sym.Decl.(*ast.ConnectorEnd); ok && sym.OwnerScope != nil {
		return sym.OwnerScope.Parent()
	}
	return sym.OwnerScope
}

// MemberSources returns the symbols whose scopes contribute members to sym, in
// deterministic breadth-first order and excluding sym itself: what sym
// specializes (DirectSupertypes) and what it reference-subsets
// (ReferencedFeature), transitively.
//
// Reference subsetting is a kind of subsetting, and subsetting is a kind of
// specialization (KerML 8.3.3.3.9), so a referencing feature inherits the
// referenced feature's features: `perform action takePhoto references
// takePicture` makes takePicture's members reachable as `takePhoto.focus`.
// It is kept out of DirectSupertypes because that relation also drives
// conformance and implicit typing, which this implementation does not yet
// derive from reference subsetting — see docs/project/spec-compliance.md.
func (m *Model) MemberSources(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil {
		return nil
	}
	if cached, ok := m.memberSources[sym]; ok {
		return cached
	}

	var order []*symbols.Symbol
	visited := map[*symbols.Symbol]bool{sym: true}
	queue := slices.Clone(m.contributors(sym))
	provisional := m.supersUnstable(sym)
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == nil || visited[cur] {
			continue
		}
		visited[cur] = true
		order = append(order, cur)
		queue = append(queue, m.contributors(cur)...)
		provisional = provisional || m.supersUnstable(cur)
	}
	// An answer computed while a reference target is being resolved, or while a
	// supertype query it depends on is itself unresolved, is provisional: those
	// guards report fewer sources than the finished model has, so it is not cached.
	if len(m.resolvingRef) == 0 && !provisional {
		m.memberSources[sym] = order
	}
	return order
}

// lookupSource is one member source in name-lookup order together with the
// types the search passed through to reach it, whose own redefinitions decide
// which of its members are inherited along that path.
type lookupSource struct {
	sym *symbols.Symbol
	via []*symbols.Symbol
}

// lookupSources is MemberSources in name-lookup order: each contributor and
// everything it inherits before the next contributor, as a name search through
// generals visits them (KerML 8.2.3.5.2). A source reached by several paths is
// listed once, under the first.
func (m *Model) lookupSources(sym *symbols.Symbol) []lookupSource {
	if sym == nil {
		return nil
	}
	if cached, ok := m.lookupOrder[sym]; ok {
		return cached
	}

	var order []lookupSource
	visited := map[*symbols.Symbol]bool{sym: true}
	provisional := m.supersUnstable(sym)
	var walk func(cur *symbols.Symbol, via []*symbols.Symbol)
	walk = func(cur *symbols.Symbol, via []*symbols.Symbol) {
		for _, next := range m.contributors(cur) {
			if next == nil || visited[next] {
				continue
			}
			visited[next] = true
			order = append(order, lookupSource{sym: next, via: via})
			provisional = provisional || m.supersUnstable(next)
			walk(next, append(via[:len(via):len(via)], next))
		}
	}
	walk(sym, nil)
	if len(m.resolvingRef) == 0 && !provisional {
		m.lookupOrder[sym] = order
	}
	return order
}

// inheritedAlong reports whether member, declared by the last of via, reaches
// the type the search started from: no type on the way redefines it.
func (m *Model) inheritedAlong(member *symbols.Symbol, via []*symbols.Symbol) bool {
	for _, t := range via {
		if m.OwnRedefinitionMasked(t, member) {
			return false
		}
	}
	return true
}

// MemberSourcesStable reports whether the last MemberSources answer for sym was
// complete and memoized, so a caller may memoize what it derived from it.
func (m *Model) MemberSourcesStable(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if m.resolver != nil {
		if target, ok := m.resolver.ResolveAliasTarget(sym); ok {
			sym = target
		}
	}
	_, ok := m.memberSources[sym]
	return ok
}

// DirectMemberSources returns the symbols that contribute members to sym in one
// step, deduplicated: the edges MemberSources takes the closure of. A caller
// enumerating names needs the steps, not the closure, to tell which types a
// derivation traversed (KerML 8.2.3.5, inheritedMemberships' excluded types).
func (m *Model) DirectMemberSources(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil {
		return nil
	}
	seen := map[*symbols.Symbol]bool{sym: true}
	var out []*symbols.Symbol
	for _, src := range m.contributors(sym) {
		if src == nil || seen[src] {
			continue
		}
		seen[src] = true
		out = append(out, src)
	}
	return out
}

// contributors returns the direct member-contributing neighbours of sym: its
// supertypes, the base usage every usage element subsets, the base feature a
// KerML feature keyword implies, then the feature it reference-subsets. The two
// bases contribute members only, not conformance.
func (m *Model) contributors(sym *symbols.Symbol) []*symbols.Symbol {
	if cached, ok := m.contributed[sym]; ok {
		return cached
	}
	out := m.collectContributors(sym)
	// Memoized under the condition MemberSources memoizes its closure: no reference
	// mid-resolution and sym's own supertypes settled.
	if len(m.resolvingRef) == 0 && !m.supersUnstable(sym) {
		m.contributed[sym] = out
	}
	return out
}

func (m *Model) collectContributors(sym *symbols.Symbol) []*symbols.Symbol {
	supers := m.DirectSupertypes(sym)
	out := make([]*symbols.Symbol, 0, len(supers)+2)
	out = append(out, supers...)
	if base := m.implicitBaseUsage(sym); base != nil {
		out = append(out, base)
	}
	if base := m.implicitKerMLFeatureBase(sym); base != nil {
		out = append(out, base)
	}
	if ref := m.ReferencedFeature(sym); ref != nil {
		out = append(out, ref)
	}
	return out
}
