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
	if m.ownedByIndividualDefinition(sym) {
		return zeroOrOneFQN, true
	}
	return naturalsFQN, true
}

// ownedByIndividualDefinition reports whether sym is the empty Multiplicity an individual
// OccurrenceDefinition is parsed with (its `multiplicity`), not a member written in its body.
func (m *Model) ownedByIndividualDefinition(sym *symbols.Symbol) bool {
	if !IsDefaultMultiplicity(sym) || sym.OwnerScope == nil {
		return false
	}
	owner := sym.OwnerScope.OwningElement()
	return IsIndividualDefinition(owner) && !m.isKerMLDoc(owner) && m.reflectiveMetaclassConforms(owner, "OccurrenceDefinition")
}

// implicitMultiplicityBase returns the library multiplicity sym implicitly subsets
// (MultiplicityBaseFQN), or nil when sym is no multiplicity or a subsetting it declares reaches it.
func (m *Model) implicitMultiplicityBase(sym *symbols.Symbol) *symbols.Symbol {
	fqn, ok := m.MultiplicityBaseFQN(sym)
	if !ok || m.resolver == nil || m.resolver.Index() == nil {
		return nil
	}
	return m.kindBaseSymbol(m.multiplicityBase, sym, func() *symbols.Symbol {
		if m.declaredGeneralizationReaches(sym, fqn, nil) {
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

// multiplicityBaseIs reports whether sym is a multiplicity whose implied base is want,
// so a declared chain reaching sym reaches want as a chain reaching a usage of the kind does.
func (m *Model) multiplicityBaseIs(sym *symbols.Symbol, want string) bool {
	fqn, ok := m.MultiplicityBaseFQN(sym)
	return ok && fqn == want
}
