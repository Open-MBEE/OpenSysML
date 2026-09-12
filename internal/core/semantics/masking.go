package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Redefinition masking (KerML 7.4.7, 8.3.3.3): a redefined feature is not
// inherited by the type owning the redefining feature, so none of its names
// are visible there. The mask is keyed by element, not by name, so a chain of
// redefinitions masks every link and an inherited namesake nobody redefines
// stays visible.

// RedefinedFeatures returns the features sym redefines: the resolved target of
// each explicit `redefines`/`:>>` clause. Alias targets are resolved through, so
// the result names elements rather than the bindings that reach them. The
// implicit redefinitions of parameters and connector ends are matched by
// position rather than declared, and are reported by DirectSupertypes only.
// Memoized.
func (m *Model) RedefinedFeatures(sym *symbols.Symbol) []*symbols.Symbol {
	if m == nil || sym == nil {
		return nil
	}
	if cached, ok := m.redefined[sym]; ok {
		return cached
	}
	m.redefined[sym] = nil // re-entrancy guard for cyclic declarations
	m.computingRedefinedFeatures++
	defer func() {
		m.computingRedefinedFeatures--
	}()

	var out []*symbols.Symbol
	seen := make(map[*symbols.Symbol]bool)
	add := func(target *symbols.Symbol) {
		if target == nil || target == sym || seen[target] {
			return
		}
		seen[target] = true
		out = append(out, target)
	}
	for _, rel := range RelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelRedefines || rel.Target == nil {
			continue
		}
		add(m.redefinitionTarget(sym, rel.Target))
	}

	m.redefined[sym] = out
	return out
}

// AllRedefinedFeatures returns every feature sym redefines, directly or through
// the features those redefine: explicit clauses and the implicit redefinitions
// of parameters, connector ends and case roles alike. sym itself is excluded.
func (m *Model) AllRedefinedFeatures(sym *symbols.Symbol) []*symbols.Symbol {
	if m == nil || sym == nil {
		return nil
	}
	var out []*symbols.Symbol
	seen := map[*symbols.Symbol]bool{sym: true}
	var visit func(*symbols.Symbol)
	visit = func(s *symbols.Symbol) {
		for _, target := range m.directRedefinedFeatures(s) {
			if target == nil || seen[target] {
				continue
			}
			seen[target] = true
			out = append(out, target)
			visit(target)
		}
	}
	visit(sym)
	return out
}

// directRedefinedFeatures returns the features sym redefines by clause, by
// position, or by name in a metadata body.
func (m *Model) directRedefinedFeatures(sym *symbols.Symbol) []*symbols.Symbol {
	explicit := m.RedefinedFeatures(sym)
	out := make([]*symbols.Symbol, 0, len(explicit))
	out = append(out, explicit...)
	out = append(out, m.ImplicitParameterRedefinitions(sym)...)
	out = append(out, m.implicitEndRedefinitions(sym)...)
	out = append(out, m.ImplicitRoleRedefinitions(sym)...)
	return append(out, m.implicitMetadataBodyRedefinitions(sym)...)
}

// implicitMetadataBodyRedefinitions is the metadata type's feature a body
// declaration without a `:>>` clause redefines by its name (KerML 7.4.7).
func (m *Model) implicitMetadataBodyRedefinitions(sym *symbols.Symbol) []*symbols.Symbol {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || m.resolver == nil || declaresRedefinitionAST(usage) {
		return nil
	}
	owner := m.resolver.MetadataBodyOwner(sym.OwnerScope)
	if owner == nil {
		return nil
	}
	target := symbols.MetadataBodyTarget(m, owner, usage.Ident)
	if target == nil || target == sym {
		return nil
	}
	return []*symbols.Symbol{target}
}

// maskingRedefinedFeatures returns the features sym redefines by clause or by
// case role, which its owner does not inherit.
func (m *Model) maskingRedefinedFeatures(sym *symbols.Symbol) []*symbols.Symbol {
	explicit := m.RedefinedFeatures(sym)
	roles := m.ImplicitRoleRedefinitions(sym)
	if len(roles) == 0 {
		return explicit
	}
	out := make([]*symbols.Symbol, 0, len(explicit)+len(roles))
	return append(append(out, explicit...), roles...)
}

// EffectiveNameOf is Element::effectiveName: the declared name, or the one of
// the feature an unnamed feature takes its identifiers from (KerML 1.1 §8.2.4).
func (m *Model) EffectiveNameOf(sym *symbols.Symbol) string {
	return inheritedIdentifier(sym, declaresIdentifier, declaredName, m.namingFeature)
}

// EffectiveShortNameOf is Element::shortName: the declared short name, or, for
// a feature declaring neither identifier, that of the feature it takes its
// identifiers from (KerML 1.1 §8.2.4, §8.3.3.3).
func (m *Model) EffectiveShortNameOf(sym *symbols.Symbol) string {
	return inheritedIdentifier(sym, declaresIdentifier, declaredShortName, m.namingFeature)
}

func declaredName(sym *symbols.Symbol) string      { return sym.Name }
func declaredShortName(sym *symbols.Symbol) string { return sym.ShortName }

// declaresIdentifier reports whether sym's declaration states a name or a short
// name of its own; a symbol named by its naming feature (KerML 7.3.4.5) declares neither.
func declaresIdentifier(sym *symbols.Symbol) bool {
	return sym.Naming == symbols.NamedByDeclaration && (sym.Name != "" || sym.ShortName != "")
}

// inheritedIdentifier reads identifier id from the first of sym and the
// features from leads to that declares one; "" past a cycle or the chain's end.
func inheritedIdentifier(
	sym *symbols.Symbol,
	declares func(*symbols.Symbol) bool,
	id func(*symbols.Symbol) string,
	from func(*symbols.Symbol) *symbols.Symbol,
) string {
	seen := make(map[*symbols.Symbol]bool)
	for sym != nil && !seen[sym] {
		if declares(sym) {
			return id(sym)
		}
		seen[sym] = true
		sym = from(sym)
	}
	return ""
}

// namingFeature is Feature::namingFeature, the feature an unnamed feature takes
// its identifiers from: what a member named by its reference references, else
// the first feature it redefines, by clause or by position (KerML 1.1 §8.3.3.3;
// SysML ConstraintUsage::namingFeature for assume/require/verify members).
func (m *Model) namingFeature(sym *symbols.Symbol) *symbols.Symbol {
	if m == nil || sym == nil {
		return nil
	}
	if byReference, referenceOnly := ast.DeclNamedByReference(sym.Decl); byReference {
		if ref := m.ReferencedFeature(sym); ref != nil {
			return ref
		}
		if referenceOnly {
			return nil
		}
	}
	if redefinesChainFirst(sym) {
		return nil
	}
	for _, candidate := range m.directRedefinedFeatures(sym) {
		if candidate != nil && candidate != sym {
			return candidate
		}
	}
	return nil
}

// redefinesChainFirst reports whether sym's first `:>>` clause names a feature
// chain, a nameless feature of its own that gives sym no name.
func redefinesChainFirst(sym *symbols.Symbol) bool {
	for _, rel := range RelationshipsOf(sym) {
		if rel != nil && rel.Kind == ast.RelRedefines {
			return ast.IsFeatureChain(rel.Target)
		}
	}
	return false
}

// DeclaresRedefinition reports whether sym carries a `redefines`/`:>>` clause,
// whether or not its target resolves.
func DeclaresRedefinition(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	for _, rel := range RelationshipsOf(sym) {
		if rel != nil && rel.Kind == ast.RelRedefines {
			return true
		}
	}
	return false
}

// NotYetMember reports whether a member of a namespace a declaration is being
// written in is that declaration: the one identified, or, when the caller cannot
// tell which it is, any redefinition the namespace declares (KerML 8.3.3.3.6).
func NotYetMember(sym, declaring *symbols.Symbol) bool {
	if declaring != nil {
		return sym == declaring
	}
	return DeclaresRedefinition(sym)
}

// redefinitionTarget resolves one redefinition target reference. A single-segment
// name denotes the feature the owner inherits under it, not the declaration that
// borrowed the name in the owner's own scope (KerML 7.3.4.5).
func (m *Model) redefinitionTarget(sym *symbols.Symbol, target ast.Node) *symbols.Symbol {
	if ref, ok := target.(*ast.FeatureReference); ok {
		target = ref.Name
	}
	switch node := target.(type) {
	case *ast.QualifiedName:
		if len(node.Parts) == 1 {
			if inherited := m.inheritedFeature(sym, node); inherited != nil {
				// An alias names its target: what is redefined is the element.
				if resolved, aliasOK := m.resolver.ResolveAliasTarget(inherited); aliasOK {
					return resolved
				}
				return inherited
			}
		}
		found, ok := m.resolver.ResolveRedefinitionTarget(sym.OwnerScope, sym.Decl, node)
		if !ok || found == nil {
			return nil
		}
		if resolved, aliasOK := m.resolver.ResolveAliasTarget(found); aliasOK {
			return resolved
		}
	case *ast.FeatureChainExpr:
		if found, ok := m.resolver.ResolveTarget(sym.OwnerScope, node); ok {
			return found
		}
	}
	return nil
}

// redefinitionMask returns the elements sym does not inherit because a feature
// of sym — owned or itself inherited — redefines them. Members sym declares are
// never masked: only inheritance is affected. When declared is false the
// features sym declares mask nothing, which is how a declaration written in sym
// sees what sym inherits (KerML 8.3.3.3.6). Memoized.
func (m *Model) redefinitionMask(sym *symbols.Symbol, declared bool) map[*symbols.Symbol]bool {
	if m == nil || sym == nil {
		return nil
	}
	cache := m.redefMask
	if !declared {
		cache = m.redefMaskInherited
	}
	if cached, ok := cache[sym]; ok {
		return cached
	}
	cache[sym] = nil // re-entrancy guard: a nested query sees no mask
	mask := m.buildMaskFromCandidates(sym, func(yield func(*symbols.Symbol) bool) {
		m.forEachMaskCandidate(sym, declared, yield)
	})
	// A redefinition mid-resolution contributes nothing yet, so its mask is not final.
	if m.computingRedefinedFeatures == 0 {
		cache[sym] = mask
	} else {
		delete(cache, sym)
	}
	return mask
}

// redefinitionMaskExcluding is the mask on sym with one of its own declarations
// left out: the declaration being written, whose redefinition must not hide the
// feature it names (KerML 8.3.3.3.6). Not memoized — it is asked per edit.
func (m *Model) redefinitionMaskExcluding(sym, exclude *symbols.Symbol) map[*symbols.Symbol]bool {
	if m == nil || sym == nil {
		return nil
	}
	return m.buildMaskFromCandidates(sym, func(yield func(*symbols.Symbol) bool) {
		m.forEachMaskCandidate(sym, true, func(candidate *symbols.Symbol) bool {
			if candidate == exclude {
				return true
			}
			return yield(candidate)
		})
	})
}

// buildMask closes candidates' redefinitions transitively into the set of
// elements sym does not inherit.
func (m *Model) buildMask(sym *symbols.Symbol, candidates []*symbols.Symbol) map[*symbols.Symbol]bool {
	mask := make(map[*symbols.Symbol]bool)
	visited := make(map[*symbols.Symbol]bool)
	pending := candidates
	for i := 0; i < len(pending); i++ {
		redefining := pending[i]
		for _, target := range m.RedefinedFeatures(redefining) {
			if target == sym {
				continue
			}
			if !redefinesSibling(redefining, target) {
				mask[target] = true
			}
			if !visited[target] {
				visited[target] = true
				pending = append(pending, target) // a redefined feature's own targets are masked too
			}
		}
	}
	// A local declaration is present whatever it redefines.
	if sym.Scope != nil {
		sym.Scope.ForEachMember(func(local *symbols.Symbol) bool {
			delete(mask, local)
			return true
		})
	}
	if len(mask) == 0 {
		return nil
	}
	return mask
}

// buildMaskFromCandidates unions cached candidate closures, falling back to the
// exact expansion when a closure is cyclic or reaches the owner.
func (m *Model) buildMaskFromCandidates(
	sym *symbols.Symbol,
	iterate func(func(*symbols.Symbol) bool),
) map[*symbols.Symbol]bool {
	mask := make(map[*symbols.Symbol]bool)
	fallback := false
	iterate(func(candidate *symbols.Symbol) bool {
		if candidate == nil {
			return true
		}
		closure, cyclic := m.redefinitionClosure(candidate)
		if cyclic || closure[sym] {
			fallback = true
			return false
		}
		for target := range closure {
			mask[target] = true
		}
		return true
	})
	if fallback {
		candidates := make([]*symbols.Symbol, 0, 8)
		iterate(func(candidate *symbols.Symbol) bool {
			candidates = append(candidates, candidate)
			return true
		})
		return m.buildMask(sym, candidates)
	}
	if sym.Scope != nil {
		sym.Scope.ForEachMember(func(local *symbols.Symbol) bool {
			delete(mask, local)
			return true
		})
	}
	if len(mask) == 0 {
		return nil
	}
	return mask
}

// redefinesSibling reports whether target is declared beside redefining: a type
// never inherits its own members, so redefining one of them removes nothing.
func redefinesSibling(redefining, target *symbols.Symbol) bool {
	return target.OwnerScope != nil && target.OwnerScope == redefining.OwnerScope
}

// redefinitionClosure returns what candidate's redefinitions, by clause or by
// position, remove from a type inheriting it, transitively; a sibling edge
// removes nothing but is followed.
func (m *Model) redefinitionClosure(candidate *symbols.Symbol) (map[*symbols.Symbol]bool, bool) {
	if candidate == nil {
		return nil, false
	}
	if cached, ok := m.redefClosure[candidate]; ok {
		return cached, false
	}
	if m.computingRedefClosure[candidate] {
		return nil, true
	}
	m.computingRedefClosure[candidate] = true
	out := make(map[*symbols.Symbol]bool)
	cyclic := false
	for _, target := range m.maskingRedefinedFeatures(candidate) {
		if !redefinesSibling(candidate, target) {
			out[target] = true
		}
		child, childCyclic := m.redefinitionClosure(target)
		if childCyclic {
			cyclic = true
		}
		for nested := range child {
			out[nested] = true
		}
	}
	delete(m.computingRedefClosure, candidate)
	if cyclic {
		return out, true
	}
	if m.computingRedefinedFeatures == 0 {
		m.redefClosure[candidate] = out
	}
	return out, false
}

// forEachMaskCandidate visits features whose redefinitions can mask something
// on sym, preserving declaration and inherited source order.
func (m *Model) forEachMaskCandidate(sym *symbols.Symbol, declared bool, yield func(*symbols.Symbol) bool) {
	if sym == nil || yield == nil {
		return
	}
	if declared && sym.Scope != nil {
		stopped := false
		sym.Scope.ForEachMember(func(candidate *symbols.Symbol) bool {
			if !yield(candidate) {
				stopped = true
				return false
			}
			return true
		})
		if stopped {
			return
		}
	}
	for _, src := range m.MemberSources(sym) {
		if src == nil || src.Scope == nil {
			continue
		}
		stopped := false
		src.Scope.ForEachMember(func(candidate *symbols.Symbol) bool {
			if !yield(candidate) {
				stopped = true
				return false
			}
			return true
		})
		if stopped {
			return
		}
	}
}

// InheritanceMasked reports whether sym does not inherit candidate because one
// of its features redefines it. Callers enumerating or resolving inherited
// members ask here; resolving a redefinition target itself must not, as that
// reference names the masked feature (KerML 7.3.4.5).
func (m *Model) InheritanceMasked(sym, candidate *symbols.Symbol) bool {
	return m.maskedBy(m.redefinitionMask(sym, true), candidate)
}

// InheritanceMaskedRedefining is InheritanceMasked as a redefinition written in
// sym sees it: the redefinitions sym itself declares mask nothing, so the target
// it names stays resolvable, while inherited redefinitions still mask
// (KerML 8.3.3.3.6). Every other reference sees InheritanceMasked.
func (m *Model) InheritanceMaskedRedefining(sym, candidate *symbols.Symbol) bool {
	return m.maskedBy(m.redefinitionMask(sym, false), candidate)
}

// NamingRedefiner returns the feature sym has in place of masked, which it does
// not inherit: the feature, own or inherited, named by redefining masked, and so
// answering to masked's identifiers (KerML 8.2.4). Nil when there is none.
func (m *Model) NamingRedefiner(sym, masked *symbols.Symbol) *symbols.Symbol {
	return m.namingRedefiner(sym, masked, true)
}

// NamingRedefinerRedefining is NamingRedefiner as a redefinition written in sym
// sees it: only inherited redefinitions mask, so only they substitute.
func (m *Model) NamingRedefinerRedefining(sym, masked *symbols.Symbol) *symbols.Symbol {
	return m.namingRedefiner(sym, masked, false)
}

func (m *Model) namingRedefiner(sym, masked *symbols.Symbol, declared bool) *symbols.Symbol {
	if m == nil || sym == nil || masked == nil {
		return nil
	}
	if target, ok := m.resolver.ResolveAliasTarget(masked); ok {
		masked = target
	}
	mask := m.redefinitionMask(sym, declared)
	var found *symbols.Symbol
	m.forEachMaskCandidate(sym, declared, func(candidate *symbols.Symbol) bool {
		if declaresIdentifier(candidate) || m.maskedBy(mask, candidate) {
			return true
		}
		if m.namedThrough(candidate, masked) {
			found = candidate
			return false
		}
		return true
	})
	return found
}

// namedThrough reports whether sym takes its identifiers from target, directly
// or through features themselves so named.
func (m *Model) namedThrough(sym, target *symbols.Symbol) bool {
	seen := make(map[*symbols.Symbol]bool)
	for cur := m.namingFeature(sym); cur != nil && !seen[cur]; cur = m.namingFeature(cur) {
		if cur == target {
			return true
		}
		seen[cur] = true
		if cur.Naming == symbols.NamedByDeclaration {
			return false
		}
	}
	return false
}

// viewMask returns the elements sym does not inherit under the given view.
func (m *Model) viewMask(sym *symbols.Symbol, view memberView, declaring *symbols.Symbol) map[*symbols.Symbol]bool {
	switch {
	case view == memberViewUnmasked:
		return nil
	case view == memberViewDeclaring && declaring == nil:
		return m.redefinitionMask(sym, false)
	case view == memberViewDeclaring:
		return m.redefinitionMaskExcluding(sym, declaring)
	}
	return m.redefinitionMask(sym, true)
}

func (m *Model) maskedBy(mask map[*symbols.Symbol]bool, candidate *symbols.Symbol) bool {
	if len(mask) == 0 || candidate == nil {
		return false
	}
	if mask[candidate] {
		return true
	}
	target, ok := m.resolver.ResolveAliasTarget(candidate)
	return ok && mask[target]
}
