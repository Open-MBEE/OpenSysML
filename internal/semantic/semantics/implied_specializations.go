package semantics

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// impliedGenerals is what DirectSupertypes derives of a symbol's implied Specializations.
type impliedGenerals struct {
	// inherited are the generals the symbol's kind implies, which DirectSupertypes ends with.
	inherited []*symbols.Symbol
	// included are those and the feature of its owner the symbol subsets by its position,
	// less the ones another specialization of the symbol makes redundant (KerML §8.4.2).
	included []*symbols.Symbol
}

// ImpliedSpecializations returns the generals of the implied Specializations sym owns once
// they are included: those its kind and position imply (KerML Tables 8 to 10, SysML Tables
// 31 and 32) less the ones the redundancy rules of KerML §8.4.2 leave out. It selects among
// what DirectSupertypes appends for the kind and ImplicitSubsettings answers for the
// position, so it states nothing that inheritance and execution do not read.
func (m *Model) ImpliedSpecializations(sym *symbols.Symbol) []*symbols.Symbol {
	return m.impliedGeneralsOf(sym).included
}

// impliedGeneralsOf returns what DirectSupertypes derived for sym, deriving it when needed.
func (m *Model) impliedGeneralsOf(sym *symbols.Symbol) impliedGenerals {
	if sym == nil {
		return impliedGenerals{}
	}
	defer m.own(sym).LeaveDoc()
	// A record names the generals its kind implied among its supertypes, so a library
	// symbol answers from its facts in every model instead of deriving them.
	if sym.Facts != nil && (sym.Facts.Supers != nil || sym.Facts.Recorded) {
		if cached, ok := m.impliedSupers[sym]; ok {
			return cached
		}
		inherited := m.recordedElements(sym, sym.Facts.ImpliedSupers)
		declared := slices.DeleteFunc(slices.Clone(m.DirectSupertypes(sym)), func(super *symbols.Symbol) bool {
			return slices.Contains(inherited, super)
		})
		out := impliedGenerals{inherited: inherited, included: m.includedGenerals(sym, inherited, declared)}
		journal(m, m.impliedSupers, sym, sym.Decl)
		m.impliedSupers[sym] = out
		return out
	}
	// DirectSupertypes derives them as it appends them, so the two cannot disagree.
	m.DirectSupertypes(sym)
	return m.impliedSupers[sym]
}

// deriveImpliedGenerals returns the generals sym's kind and position imply. declared holds
// the generals sym has otherwise, written or implied by a Redefinition.
func (m *Model) deriveImpliedGenerals(sym *symbols.Symbol, declared []*symbols.Symbol, fromMetadata bool) impliedGenerals {
	var inherited []*symbols.Symbol
	add := func(generals []*symbols.Symbol) {
		for _, general := range generals {
			if general != nil && general != sym && !slices.Contains(declared, general) && !slices.Contains(inherited, general) {
				inherited = append(inherited, general)
			}
		}
	}
	// A metadata keyword supplies the kind itself, so its baseType stands in for the kind's bases.
	if !fromMetadata {
		add(m.implicitBases(sym))
	}
	add(m.implicitBaseFeatures(sym, slices.Concat(declared, inherited)))
	return impliedGenerals{inherited: inherited, included: m.includedGenerals(sym, inherited, declared)}
}

// includedGenerals joins the generals sym's kind implies with the one its position does
// and leaves out the redundant ones.
func (m *Model) includedGenerals(sym *symbols.Symbol, inherited, declared []*symbols.Symbol) []*symbols.Symbol {
	candidates := slices.Clone(inherited)
	for _, general := range m.ImplicitSubsettings(sym) {
		if !slices.Contains(candidates, general) {
			candidates = append(candidates, general)
		}
	}
	return m.withoutRedundant(sym, candidates, declared)
}

// withoutRedundant applies the redundancy rules of KerML §8.4.2 to the candidate generals
// of sym's implied Specializations: one is left out when another specialization of sym, owned
// or implied, has the same general or a subtype of it.
func (m *Model) withoutRedundant(sym *symbols.Symbol, candidates, declared []*symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	for i, general := range candidates {
		subsumes := func(other *symbols.Symbol) bool {
			return other != nil && other != sym && (symbols.SameElement(other, general) || m.Conforms(other, general))
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
