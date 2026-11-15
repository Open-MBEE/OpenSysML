package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// Redefinition masking (KerML 7.4.7, 8.3.3.3): a redefined feature is not
// inherited by the type owning the redefining feature, so none of its names
// are visible there. The mask is keyed by element, not by name, so a chain of
// redefinitions masks every link and an inherited namesake nobody redefines
// stays visible.

// RedefinedFeatures returns the features sym redefines: the resolved target of
// each explicit `redefines`/`:>>` clause. Alias targets are resolved through, so
// the result names elements rather than the bindings that reach them. The
// implicit redefinitions of parameters, connector ends and metadata body
// declarations are not declared, and are reported by DirectSupertypes only.
// Memoized.
func (m *Model) RedefinedFeatures(sym *symbols.Symbol) []*symbols.Symbol {
	if m == nil || sym == nil {
		return nil
	}
	defer m.own(sym).LeaveDoc()
	if cached, ok := m.redefined[sym]; ok {
		// The seed answers a re-entrant query with nothing, cutting that query short.
		if depth := m.computingRedefined[sym]; depth != 0 {
			m.resolver.CutShort(depth)
		}
		return cached
	}
	journal(m, m.redefined, sym, sym.Decl)
	m.redefined[sym] = nil // re-entrancy guard for cyclic declarations
	m.computingRedefined[sym] = m.resolver.Enter()
	defer delete(m.computingRedefined, sym)
	m.computingRedefinedFeatures++
	defer func() {
		m.computingRedefinedFeatures--
	}()

	var out []*symbols.Symbol
	if sym.Recorded() {
		out = m.recordedElements(sym, sym.Facts.Redefines)
	}
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

	// An answer derived while a guard cut a query short may have missed a target,
	// so it is recomputed on the next query, not memoized.
	if !m.resolver.Leave() {
		delete(m.redefined, sym)
		return out
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
	defer m.own(sym).LeaveDoc()
	if cached, ok := m.allRedefined[sym]; ok {
		return cached
	}
	m.resolver.Enter()
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
	// A closure walked while a guard cut a query short may lack a target, so it
	// is recomputed on the next query, not memoized.
	if !m.resolver.Leave() {
		return out
	}
	journal(m, m.allRedefined, sym, sym.Decl)
	m.allRedefined[sym] = out
	return out
}

// DirectRedefinedFeatures returns the features sym itself redefines: by clause, then by
// position (parameter, connector end, case role), then by name in a metadata body.
func (m *Model) DirectRedefinedFeatures(sym *symbols.Symbol) []*symbols.Symbol {
	if m == nil || sym == nil {
		return nil
	}
	return m.directRedefinedFeatures(sym)
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

// RestatesMetadataBodyFeature reports whether sym is a metadata body's `mission = …`:
// a feature declaring no name of its own, named by the definition's feature it redefines.
func (m *Model) RestatesMetadataBodyFeature(sym *symbols.Symbol) bool {
	return m != nil && sym != nil && len(m.implicitMetadataBodyRedefinitions(sym)) > 0
}

// implicitMetadataBodyRedefinitions is the metadata type's feature a body
// declaration without a `:>>` clause redefines by its name (KerML 7.4.7).
func (m *Model) implicitMetadataBodyRedefinitions(sym *symbols.Symbol) []*symbols.Symbol {
	if _, ok := sym.UsageKind(); !ok || m.resolver == nil || DeclaresRedefinition(sym) ||
		sym.Naming != symbols.NamedByDeclaration {
		return nil
	}
	owner := m.resolver.MetadataBodyOwner(sym.OwnerScope)
	if owner == nil {
		return nil
	}
	target := symbols.MetadataBodyTarget(m, owner, ast.Identification{Name: sym.Name, ShortName: sym.ShortName})
	if target == nil || target == sym {
		return nil
	}
	return []*symbols.Symbol{target}
}

// MaskingRedefinedFeatures returns the features sym redefines by clause or by
// case role: the ones its owner does not inherit (KerML 7.4.7).
func (m *Model) MaskingRedefinedFeatures(sym *symbols.Symbol) []*symbols.Symbol {
	if m == nil || sym == nil {
		return nil
	}
	return m.maskingRedefinedFeatures(sym)
}

// maskingRedefinedFeatures returns the features sym redefines by clause, by parameter
// position (KerML 8.4.4.6) or by case role, which its owner does not inherit. A library role (a view's `render`,
// a state's `entry` action) leaves the library name reachable, as in the pilot.
func (m *Model) maskingRedefinedFeatures(sym *symbols.Symbol) []*symbols.Symbol {
	explicit := m.RedefinedFeatures(sym)
	parameters := m.MaskedParameterRedefinitions(sym)
	roles := m.caseRoleRedefinitions(sym)
	if len(parameters) == 0 && len(roles) == 0 {
		return explicit
	}
	out := make([]*symbols.Symbol, 0, len(explicit)+len(parameters)+len(roles))
	return append(append(append(out, explicit...), parameters...), roles...)
}

// EffectiveNameOf is Element::effectiveName: the declared name, or the one of
// the feature an unnamed feature takes its identifiers from (KerML 1.1 §8.2.4).
func (m *Model) EffectiveNameOf(sym *symbols.Symbol) string {
	name, _ := m.EffectiveNamed(sym)
	return name
}

// EffectiveNamed is EffectiveNameOf with whether there is such a name, which
// tells the empty unrestricted name from none (KerML §8.2.2.3).
func (m *Model) EffectiveNamed(sym *symbols.Symbol) (string, bool) {
	from := identifierSource(sym, m.declaresOwnIdentifier, m.namingFeature)
	if from == nil {
		return "", false
	}
	return from.Name, from.Named()
}

// EffectiveShortNameOf is Element::shortName: the declared short name, or, for
// a feature declaring neither identifier, that of the feature it takes its
// identifiers from (KerML 1.1 §8.2.4, §8.3.3.3).
func (m *Model) EffectiveShortNameOf(sym *symbols.Symbol) string {
	name, _ := m.EffectiveShortNamed(sym)
	return name
}

// EffectiveShortNamed is EffectiveShortNameOf with whether there is such a short name.
func (m *Model) EffectiveShortNamed(sym *symbols.Symbol) (string, bool) {
	from := identifierSource(sym, m.declaresOwnIdentifier, m.namingFeature)
	if from == nil {
		return "", false
	}
	return from.ShortName, from.HasShortName()
}

// declaresOwnIdentifier is declaresIdentifier for a symbol that is not a metadata
// body restatement: `m = …` writes the redefined feature's name, not one of its own.
func (m *Model) declaresOwnIdentifier(sym *symbols.Symbol) bool {
	return declaresIdentifier(sym) && !m.RestatesMetadataBodyFeature(sym)
}

// declaresIdentifier reports whether sym's declaration states a name or a short
// name of its own; a symbol named by its naming feature (KerML 7.3.4.5) declares neither.
func declaresIdentifier(sym *symbols.Symbol) bool {
	return sym.Naming == symbols.NamedByDeclaration && (sym.Named() || sym.HasShortName())
}

// identifierSource is the first of sym and the features from leads to that
// declares an identifier; nil past a cycle or the chain's end.
func identifierSource(
	sym *symbols.Symbol,
	declares func(*symbols.Symbol) bool,
	from func(*symbols.Symbol) *symbols.Symbol,
) *symbols.Symbol {
	seen := make(map[*symbols.Symbol]bool)
	for sym != nil && !seen[sym] {
		if declares(sym) {
			return sym
		}
		seen[sym] = true
		sym = from(sym)
	}
	return nil
}

// namingFeature is Feature::namingFeature, the feature an unnamed feature takes
// its identifiers from: what a member named by its reference references, else
// the first feature it redefines, by clause or by position (KerML 1.1 §8.3.3.3;
// SysML ConstraintUsage::namingFeature for assume/require/verify members).
// NamingFeatureOf is the feature sym takes its name and short name from when it
// declares neither (KerML 8.3.3.3 namingFeature): referenced, else first redefined.
func (m *Model) NamingFeatureOf(sym *symbols.Symbol) *symbols.Symbol {
	return m.namingFeature(sym)
}

// NamingFeature returns the feature that gives sym its effective name.
func (m *Model) NamingFeature(sym *symbols.Symbol) *symbols.Symbol {
	return m.namingFeature(sym)
}

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

// redefinitionTarget resolves one redefinition target reference as the resolver
// reads it: from the owner's generals, then the enclosing namespaces (KerML 8.2.3.5.2).
func (m *Model) redefinitionTarget(sym *symbols.Symbol, target ast.Node) *symbols.Symbol {
	found, ok := m.resolver.ProbeRedefinitionTarget(sym.OwnerScope, sym.Decl, target)
	if !ok || found == nil {
		return nil
	}
	if resolved, aliasOK := m.resolver.ResolveAliasTarget(found); aliasOK {
		return resolved
	}
	return nil
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
	defer m.own(candidate).LeaveDoc()
	if cached, ok := m.redefClosure[candidate]; ok {
		return cached, false
	}
	if m.computingRedefClosure[candidate] {
		return nil, true
	}
	m.computingRedefClosure[candidate] = true
	m.resolver.Enter()
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
	settled := m.resolver.Leave()
	if cyclic {
		return out, true
	}
	if settled && m.computingRedefinedFeatures == 0 {
		journal(m, m.redefClosure, candidate, candidate.Decl)
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

// OwnRedefinitionMasked reports whether sym's own features keep its
// specializations from inheriting candidate through sym (KerML 8.3.3.1.10, condition 2).
func (m *Model) OwnRedefinitionMasked(sym, candidate *symbols.Symbol) bool {
	return m.ownRedefinitions(sym, nil).masks(m, m.aliased(candidate))
}

// OwnNamingRedefiner returns the feature sym declares in place of masked, named
// by redefining it, which sym's specializations inherit under masked's names.
func (m *Model) OwnNamingRedefiner(sym, masked *symbols.Symbol) *symbols.Symbol {
	if m == nil || sym == nil || sym.Scope == nil {
		return nil
	}
	own := m.ownRedefinitions(sym, nil)
	return m.namingRedefinerAmong(masked, func(candidate *symbols.Symbol) bool {
		return own.masks(m, m.aliased(candidate))
	}, sym.Scope.ForEachMember)
}

func (m *Model) namingRedefiner(sym, masked *symbols.Symbol, declared bool) *symbols.Symbol {
	if m == nil || sym == nil {
		return nil
	}
	mask := m.redefinitionMask(sym, declared)
	return m.namingRedefinerAmong(masked, func(candidate *symbols.Symbol) bool {
		return m.maskedBy(mask, candidate)
	}, func(yield func(*symbols.Symbol) bool) {
		m.forEachMaskCandidate(sym, declared, yield)
	})
}

// namingRedefinerAmong finds, among the candidates iterate yields, the feature
// sym keeps that is named by redefining masked. An alias of masked is a name of
// its membership, not an identifier a redefinition takes (KerML 8.3.3.3.4 effectiveName).
func (m *Model) namingRedefinerAmong(masked *symbols.Symbol, lost func(*symbols.Symbol) bool, iterate func(func(*symbols.Symbol) bool)) *symbols.Symbol {
	if masked == nil || masked.Kind == symbols.SymbolAlias {
		return nil
	}
	var found *symbols.Symbol
	iterate(func(candidate *symbols.Symbol) bool {
		if declaresIdentifier(candidate) || lost(candidate) {
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
	return mask[candidate] || mask[m.aliased(candidate)]
}

// aliased is the element an alias membership names, else sym itself.
func (m *Model) aliased(sym *symbols.Symbol) *symbols.Symbol {
	if target, ok := m.resolver.ResolveAliasTarget(sym); ok {
		return target
	}
	return sym
}
