package semantics

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// ImpliedSpecializations returns the generals of the implied Specializations sym owns once
// they are included, after the redundancy rules (KerML §8.4.2, Tables 8 and 10; SysML Tables 31, 32).
func (m *Model) ImpliedSpecializations(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil {
		return nil
	}
	if sym.Inclusion != nil {
		return m.inclusionGenerals(sym)
	}
	defer m.own(sym).LeaveDoc()
	if cached, ok := m.impliedSpecializations[sym]; ok {
		return cached
	}
	m.resolver.Enter()
	// The candidates are what inheritance reads for the kind and ImplicitSubsettings for the position.
	candidates := slices.Clone(m.ImplicitGenerals(sym))
	add := func(generals []*symbols.Symbol) {
		for _, general := range generals {
			if general != nil && general != sym && !slices.Contains(candidates, general) {
				candidates = append(candidates, general)
			}
		}
	}
	add(m.impliedFeatureGenerals(sym))
	// The chain a value or a chain expression's result is reached through stands for the feature it ends at.
	if chain := m.impliedSubsettedChain(sym); chain != nil {
		ends := m.DirectSupertypes(chain)
		reached := append(slices.Clone(ends), m.FeatureValueGeneral(sym))
		reached = append(reached, m.expressionElementGenerals(sym)...)
		candidates = slices.DeleteFunc(candidates, func(general *symbols.Symbol) bool { return slices.Contains(reached, general) })
		candidates = append(candidates, chain)
	}
	// checkFeatureCrossingSpecialization: the CrossSubsetting of an end owning a cross feature.
	if chain := m.ImpliedCrossedChain(sym); chain != nil {
		candidates = append(candidates, chain)
	}
	// checkTransitionUsageSuccessionSourceSpecialization: the ReferenceSubsetting of the source end.
	if source := m.impliedTransitionSourceEnd(sym); source != nil {
		candidates = append(candidates, source)
	}
	// What is left of the supertypes is what the declaration states.
	declared := slices.DeleteFunc(slices.Clone(m.DirectSupertypes(sym)), func(super *symbols.Symbol) bool {
		return slices.Contains(candidates, super)
	})
	add(m.ImplicitSubsettings(sym))
	out := m.withoutRedundant(sym, candidates, declared)
	if m.resolver.Leave() {
		journal(m, m.impliedSpecializations, sym, sym.Decl)
		m.impliedSpecializations[sym] = out
	}
	return out
}

// impliedGeneralsOf returns the generals sym's kind implies, as DirectSupertypes derived them.
func (m *Model) impliedGeneralsOf(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil {
		return nil
	}
	defer m.own(sym).LeaveDoc()
	// A record names them among its supertypes, so a library symbol answers from its facts.
	if sym.Facts != nil && (sym.Facts.Supers != nil || sym.Facts.Recorded) {
		if cached, ok := m.impliedSupers[sym]; ok {
			return cached
		}
		out := m.recordedElements(sym, sym.Facts.ImpliedSupers)
		journal(m, m.impliedSupers, sym, sym.Decl)
		m.impliedSupers[sym] = out
		return out
	}
	m.DirectSupertypes(sym)
	return m.impliedSupers[sym]
}

// deriveImpliedGenerals returns the generals sym's kind implies beside declared, the ones
// it has otherwise; a declared chain that reaches a kind's base suppresses that base.
func (m *Model) deriveImpliedGenerals(sym *symbols.Symbol, declared []*symbols.Symbol, fromMetadata bool) []*symbols.Symbol {
	var implied []*symbols.Symbol
	add := func(generals []*symbols.Symbol) {
		for _, general := range generals {
			if general != nil && general != sym && !slices.Contains(declared, general) && !slices.Contains(implied, general) {
				implied = append(implied, general)
			}
		}
	}
	// A metadata keyword supplies the kind itself, so its baseType stands in for the kind's
	// bases; an individual definition is a Life whatever its kind (SysML v2 §8.4 Table 31).
	bases := m.implicitBases(sym)
	if fromMetadata {
		bases = slices.DeleteFunc(slices.Clone(bases), func(base *symbols.Symbol) bool {
			return m.resolver.Index().GetFQN(base) != lifeFQN
		})
	}
	add(bases)
	add(m.implicitBaseFeatures(sym, slices.Concat(declared, implied)))
	return implied
}

// withoutRedundant leaves out each candidate general another specialization of sym, owned
// or implied, makes redundant by having the same general or a subtype of it (KerML §8.4.2).
func (m *Model) withoutRedundant(sym *symbols.Symbol, candidates, declared []*symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	for i, general := range candidates {
		subsumes := func(other *symbols.Symbol) bool {
			return other != nil && other != sym && (symbols.SameElement(other, general) || m.Conforms(other, general) ||
				m.subsetsByPosition(other, general))
		}
		// A general that reaches sym again is in a cycle with it and makes nothing redundant:
		// each member of the cycle would otherwise leave the base to the next.
		if slices.ContainsFunc(declared, func(other *symbols.Symbol) bool {
			return subsumes(other) && !slices.Contains(m.AllSupertypes(other), sym)
		}) {
			continue
		}
		redundant := false
		for j, other := range candidates {
			// Of two candidates that are subtypes of each other the earlier is kept.
			if i != j && subsumes(other) && (j < i || !m.Conforms(general, other)) {
				redundant = true
				break
			}
		}
		if !redundant {
			out = append(out, general)
		}
	}
	return out
}

// subsetsByPosition reports whether sym or a supertype of it subsets general, or a subtype
// of it, by its position: the implied Specializations inheritance does not traverse.
func (m *Model) subsetsByPosition(sym, general *symbols.Symbol) bool {
	for super := range m.eachSelfAndSupertype(sym) {
		if slices.ContainsFunc(m.ImplicitSubsettings(super), func(feature *symbols.Symbol) bool {
			return symbols.SameElement(feature, general) || m.Conforms(feature, general)
		}) {
			return true
		}
	}
	return false
}

// impliedFeatureGenerals returns the generals DirectSupertypes derives for a feature from where it stands
// (KerML Table 9; SysML Table 32): an end's participant, a cross feature's types, a value, a variation.
func (m *Model) impliedFeatureGenerals(sym *symbols.Symbol) []*symbols.Symbol {
	redefines := len(m.implicitEndRedefinitions(sym)) > 0 || declaresRedefinition(sym)
	out := []*symbols.Symbol{m.implicitEndParticipant(sym, redefines)}
	out = append(out, m.implicitCrossFeatureGenerals(sym)...)
	out = append(out, m.FeatureValueGeneral(sym), m.VariationPointOwning(sym))
	return append(out, m.expressionElementGenerals(sym)...)
}
