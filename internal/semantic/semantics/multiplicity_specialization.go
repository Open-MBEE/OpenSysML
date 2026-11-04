package semantics

import "github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"

const (
	naturalsFQN  = "Base::naturals"
	zeroOrOneFQN = "Base::zeroOrOne"
)

// MultiplicityBaseFQN returns the library feature a Multiplicity implicitly subsets:
// Base::naturals (KerML 1.1 §8.3.3.1.9 checkMultiplicitySpecialization), or Base::zeroOrOne for
// an individual OccurrenceDefinition's (SysML v2 §8.3.9.3 checkOccurrenceDefinitionMultiplicitySpecialization).
func (m *Model) MultiplicityBaseFQN(sym *symbols.Symbol) (string, bool) {
	if m == nil || sym == nil || sym.Kind != symbols.SymbolMultiplicity {
		return "", false
	}
	if IsIndividualMultiplicity(sym) {
		return zeroOrOneFQN, true
	}
	return naturalsFQN, true
}

// implicitMultiplicityBase returns the library multiplicity sym implicitly subsets
// (MultiplicityBaseFQN), or nil when sym is no multiplicity or a subsetting it declares reaches it.
func (m *Model) implicitMultiplicityBase(sym *symbols.Symbol) *symbols.Symbol {
	fqn, ok := m.MultiplicityBaseFQN(sym)
	if !ok || m.resolver == nil || m.resolver.Index() == nil {
		return nil
	}
	return m.kindBaseSymbol(m.multiplicityBase, sym, func() *symbols.Symbol {
		if m.declaredMultiplicityReaches(sym, fqn, nil) {
			return nil
		}
		for _, base := range m.resolver.Index().LookupQualified(fqn) {
			if base != nil && base != sym && !enclosedBy(sym, base) {
				return base
			}
		}
		return nil
	})
}

// declaredMultiplicityReaches reports whether the multiplicity sym declares it subsets
// (`multiplicity m subsets n`) is or reaches want, so sym needs no implied subsetting of it.
func (m *Model) declaredMultiplicityReaches(sym *symbols.Symbol, want string, visiting map[*symbols.Symbol]bool) bool {
	target := m.SubsettedMultiplicity(sym)
	if target == nil || target == sym || visiting[target] {
		return false
	}
	// Every Multiplicity specializes Base::naturals, by declaration or by implication.
	if symbols.HasFQN(target, want) || (want == naturalsFQN && target.Kind == symbols.SymbolMultiplicity) {
		return true
	}
	if visiting == nil {
		visiting = make(map[*symbols.Symbol]bool)
	}
	visiting[sym] = true
	return m.declaredGeneralizationReaches(target, want, nil) || m.declaredMultiplicityReaches(target, want, visiting)
}
