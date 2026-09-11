package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// The system of units whose base units compose a dimension's coherent unit.
const (
	fqnSystemOfUnitsSI = "SI::si"
	memberBaseUnits    = "baseUnits"
)

// CoherentUnit is the coherent unit of dim in the library's system of units: the
// product of SI::si's base units raised to dim's exponents (`kg` for mass,
// `kg*m/s**2` for force). A dimensionless dim measures in the unit one. False when
// the library declares no base unit for one of dim's base quantities.
func (m *Model) CoherentUnit(dim Dimension) (Unit, bool) {
	if m == nil {
		return Unit{}, false
	}
	if dim.Term.Dimensionless() {
		return UnitOne(), true
	}
	bases := m.systemBaseUnits()
	product, term := UnitProduct{}, UnitTerm{Scale: UnitScale(1)}
	for _, factor := range dim.Term.Factors {
		unit, ok := bases[factor.Unit]
		if !ok {
			return Unit{}, false
		}
		reduced, err := m.UnitTermOf(unit)
		if err != nil {
			return Unit{}, false
		}
		product = product.Times(NamedUnitProduct(unit, unitShortName(unit), false).Pow(factor.Exponent))
		term = term.Times(reduced.Pow(factor.Exponent))
	}
	return Unit{Text: product.String(), Product: product, Term: term}, true
}

// CoherentUnitFor is the coherent unit of dim spelt by the declared unit the
// feature's type admits (`SI::m`, `SI::'m/s'`), else over the base units.
func (m *Model) CoherentUnitFor(dim Dimension, declared *symbols.Symbol) (Unit, bool) {
	coherent, ok := m.CoherentUnit(dim)
	if !ok {
		return Unit{}, false
	}
	if synonym, ok := m.coherentSynonym(coherent.Term, declared); ok {
		return synonym, true
	}
	return coherent, true
}

// systemBaseUnits maps each base quantity to the base unit SI::si measures it in,
// read once from the library's `baseUnits` list.
func (m *Model) systemBaseUnits() map[*symbols.Symbol]*symbols.Symbol {
	if m.baseUnits != nil {
		return m.baseUnits
	}
	m.baseUnits = make(map[*symbols.Symbol]*symbols.Symbol)
	system := m.libSymbol(fqnSystemOfUnitsSI)
	listed, ok := m.LookupMember(system, memberBaseUnits)
	if !ok || m.resolver == nil {
		return m.baseUnits
	}
	value := usageValue(listed)
	if value == nil {
		return m.baseUnits
	}
	scope := scopeOf(listed)
	for _, ref := range sequenceElements(value) {
		unit, ok := m.resolver.ResolveTarget(scope, ref)
		if !ok || unit == nil {
			continue
		}
		term, ok := m.dimensionOf(unit)
		if !ok || len(term.Factors) != 1 || term.Factors[0].Exponent != 1 {
			continue
		}
		m.baseUnits[term.Factors[0].Unit] = unit
	}
	return m.baseUnits
}

// unitShortName is the symbol a unit is written by (`kg`), or its name when it
// declares none.
func unitShortName(unit *symbols.Symbol) string {
	if unit.ShortName != "" {
		return unit.ShortName
	}
	return unit.Name
}
