package semantics

import (
	"maps"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// coherentUnit is a declared unit a coherent result may be spelt by, ranked by
// where it is declared: the library first.
type coherentUnit struct {
	sym  *symbols.Symbol
	rank int
}

// Composed reports whether the unit is a product, quotient or power of named
// units (`N/kg`, `km**0.5`) rather than one named unit (`N`, `'m/s'`).
func (u Unit) Composed() bool {
	switch len(u.Product.Powers) {
	case 0:
		return false
	case 1:
		return u.Product.Powers[0].Exponent != 1
	}
	return true
}

// CoherentQuantity folds a composed unit's scale into the magnitude and spells it by
// the coherent unit of its dimension, else over the base units; an unreducible unit is kept.
func (m *Model) CoherentQuantity(q Quantity, declared *symbols.Symbol) (Quantity, error) {
	if !q.Unit.Composed() {
		return q, nil
	}
	coherent, ok := m.coherentUnitOf(q.Unit, declared, true)
	if !ok {
		return q, nil
	}
	num := q.Num
	if !q.Unit.Term.Same(coherent.Term) {
		var err error
		if num, err = RealResult(ConvertMagnitude(q.Num.AsReal(), q.Unit.Term.Scale, coherent.Term.Scale)); err != nil {
			return Quantity{}, err
		}
	}
	return Quantity{Num: num, Unit: coherent}, nil
}

// CoherentSpelling respells an already coherent unit by the unit the declared type's
// measurement reference admits; a named unit it admits is kept, the magnitude never changes.
func (m *Model) CoherentSpelling(q Quantity, declared *symbols.Symbol) Quantity {
	if m == nil || declared == nil {
		return q
	}
	if !q.Unit.Composed() {
		if len(q.Unit.Product.Powers) == 0 || q.Unit.Product.Powers[0].Unit == nil || m.measuredBy(q.Unit.Product.Powers[0].Unit, declared) {
			return q
		}
	}
	coherent, ok := m.coherentUnitOf(q.Unit, declared, false)
	if !ok || !q.Unit.Term.Same(coherent.Term) {
		return q
	}
	return Quantity{Num: q.Num, Unit: coherent}
}

// coherentUnitOf is the coherent unit a unit is re-expressed in: a declared
// synonym, else the base-unit product when baseFallback says so.
func (m *Model) coherentUnitOf(unit Unit, declared *symbols.Symbol, baseFallback bool) (Unit, bool) {
	if m == nil || len(unit.Product.Powers) == 0 || unit.Term.Dimensionless() {
		return Unit{}, false
	}
	// A dimension-one factor carries meaning its reduction drops; an opaque one
	// (declared nowhere in the model) has no reduction the model can vouch for.
	for _, power := range unit.Product.Powers {
		if power.DimensionOne || power.Unit == nil {
			return Unit{}, false
		}
	}
	dim, ok := m.DimensionOfUnit(unit.Term)
	if !ok {
		return Unit{}, false
	}
	coherent, ok := m.CoherentUnit(dim)
	if !ok || !coherent.Term.Commensurable(unit.Term) {
		return Unit{}, false
	}
	if synonym, ok := m.coherentSynonym(coherent.Term, declared); ok {
		return synonym, true
	}
	return coherent, baseFallback
}

// coherentSynonym is the declared unit spelling the coherent term: library over model,
// then shortest; where candidates measure different kinds (`J`, `'N⋅m'`), the base-unit-defined ones decide.
func (m *Model) coherentSynonym(coherent UnitTerm, declared *symbols.Symbol) (Unit, bool) {
	var candidates []coherentUnit
	for _, candidate := range m.coherentUnitsFor(coherent) {
		if !m.measuredBy(candidate.sym, declared) {
			continue
		}
		if len(candidates) > 0 && candidate.rank != candidates[0].rank {
			break
		}
		candidates = append(candidates, candidate)
	}
	if len(candidates) > 0 && !m.shareUnitType(candidates) {
		candidates = slices.DeleteFunc(candidates, func(c coherentUnit) bool { return !m.definedOverBaseUnits(c.sym) })
	}
	if len(candidates) == 0 || !m.shareUnitType(candidates) {
		return Unit{}, false
	}
	slices.SortStableFunc(candidates, func(a, b coherentUnit) int {
		if c := utf8.RuneCountInString(unitShortName(a.sym)) - utf8.RuneCountInString(unitShortName(b.sym)); c != 0 {
			return c
		}
		if c := strings.Compare(unitShortName(a.sym), unitShortName(b.sym)); c != 0 {
			return c
		}
		return strings.Compare(symbols.FQNOf(a.sym), symbols.FQNOf(b.sym))
	})
	chosen := candidates[0].sym
	product := NamedUnitProduct(chosen, qualifiedUnitName(chosen), false)
	return Unit{Text: product.String(), Product: product, Term: coherent}, true
}

// qualifiedUnitName spells a unit by its symbol, qualified by where it is declared
// (`SI::'m/s'`), so the spelling reads back in any scope.
func qualifiedUnitName(unit *symbols.Symbol) string {
	name := unitNameSpelling(unitShortName(unit))
	if unit.OwnerScope == nil || unit.OwnerScope.Owner() == nil {
		return name
	}
	if owner := symbols.FQNOf(unit.OwnerScope.Owner()); owner != "" {
		return owner + "::" + name
	}
	return name
}

// coherentUnitsFor lists the declared units reducing to exactly the coherent term,
// library ones first, each rank in declaration order.
func (m *Model) coherentUnitsFor(coherent UnitTerm) []coherentUnit {
	if m.coherentUnits == nil {
		m.coherentUnits = m.indexCoherentUnits()
	}
	var out []coherentUnit
	for _, candidate := range m.coherentUnits[coherent.DimensionKey()] {
		if term, err := m.UnitTermOf(candidate.sym); err == nil && term.Same(coherent) {
			out = append(out, candidate)
		}
	}
	return out
}

// indexCoherentUnits gathers, by reduced factors, the measurement units the
// system of units' document and the workspace declare in their packages.
func (m *Model) indexCoherentUnits() map[string][]coherentUnit {
	out := make(map[string][]coherentUnit)
	if m.resolver == nil || m.resolver.Index() == nil {
		return out
	}
	idx := m.resolver.Index()
	var roots []*symbols.Scope
	if system := m.libSymbol(fqnSystemOfUnitsSI); system != nil {
		roots = append(roots, rootScopeOf(system.OwnerScope))
	}
	for _, doc := range idx.WorkspaceDocuments() {
		roots = append(roots, idx.DocumentRoot(doc))
	}
	for rank, root := range roots {
		if root == nil {
			continue
		}
		m.gatherCoherentUnits(root, min(rank, 1), out)
	}
	return out
}

// gatherCoherentUnits walks a scope tree's packages for the units they declare, leaving
// out those whose declared kind disagrees with their definition or that compose a dimension-one unit.
func (m *Model) gatherCoherentUnits(scope *symbols.Scope, rank int, out map[string][]coherentUnit) {
	for _, sym := range scope.Members() {
		switch sym.Kind {
		case symbols.SymbolPackage, symbols.SymbolNamespace:
			if sym.Scope != nil {
				m.gatherCoherentUnits(sym.Scope, rank, out)
			}
		case symbols.SymbolAttributeUsage:
			if !m.IsMeasurementUnit(sym) {
				continue
			}
			term, err := m.UnitTermOf(sym)
			if err != nil || term.Dimensionless() {
				continue
			}
			if slices.ContainsFunc(m.definitionPowers(sym), func(p UnitPower) bool { return p.DimensionOne }) {
				continue
			}
			if declared, ok := m.dimensionOf(sym); ok {
				if reduced, ok := m.dimensionOfUnitTerm(term); !ok || !declared.Commensurable(reduced) {
					continue
				}
			}
			out[term.DimensionKey()] = append(out[term.DimensionKey()], coherentUnit{sym: sym, rank: rank})
		}
	}
}

// definedOverBaseUnits reports whether a derived unit is defined over the system's
// base units alone (`N = kg*m/s^2`), as SI states its coherent derived units.
func (m *Model) definedOverBaseUnits(sym *symbols.Symbol) bool {
	powers := m.definitionPowers(sym)
	if len(powers) == 0 {
		return false
	}
	bases := slices.Collect(maps.Values(m.systemBaseUnits()))
	return !slices.ContainsFunc(powers, func(p UnitPower) bool { return p.Unit == nil || !slices.Contains(bases, p.Unit) })
}

// definitionPowers lists the unit powers a derived unit's definition names, from
// its `unitPowerFactors` or the expression it is written as (`kg*m/s^2`).
func (m *Model) definitionPowers(sym *symbols.Symbol) []UnitPower {
	var out []UnitPower
	collect := func(named *symbols.Symbol) {
		if product, err := m.UnitProductOfExpr(scopeOf(named), usageValue(named)); err == nil {
			out = append(out, product.Powers...)
		}
	}
	if factors, declared := m.unitPowerFactorSymbols(sym); declared {
		for _, factor := range factors {
			if named, ok := m.LookupMember(factor, memberUnit); ok {
				collect(named)
			}
		}
	} else if usageValue(sym) != nil {
		collect(sym)
	}
	return out
}

// rootScopeOf is the document root a scope belongs to.
func rootScopeOf(scope *symbols.Scope) *symbols.Scope {
	for scope != nil && scope.Parent() != nil {
		scope = scope.Parent()
	}
	return scope
}

// measuredBy reports whether the declared type's measurement reference admits the
// unit; a type declaring none, or one typed by no unit definition, admits every unit.
func (m *Model) measuredBy(unit, declared *symbols.Symbol) bool {
	if declared == nil {
		return true
	}
	if m.resolver != nil {
		if alias, ok := m.resolver.ResolveAliasTarget(declared); ok {
			declared = alias
		}
	}
	typ := m.quantityValueTypeIn(append([]*symbols.Symbol{declared}, m.AllSupertypes(declared)...))
	if typ == nil {
		return true
	}
	mRef, ok := m.LookupMember(typ, memberMRef)
	if !ok {
		return true
	}
	typed := false
	for _, ref := range m.DirectSupertypes(mRef) {
		if ref.Kind != symbols.SymbolAttributeDef {
			continue
		}
		typed = true
		if m.Conforms(unit, ref) {
			return true
		}
	}
	return !typed
}

// shareUnitType reports whether every candidate is declared by one unit definition
// in common (`'m⋅s⁻¹'` and `'m/s'` are SpeedUnits; `Hz` and `Bq` share none).
func (m *Model) shareUnitType(candidates []coherentUnit) bool {
	common := m.unitTypesOf(candidates[0].sym)
	for _, candidate := range candidates[1:] {
		types := m.unitTypesOf(candidate.sym)
		common = slices.DeleteFunc(common, func(typ *symbols.Symbol) bool { return !slices.Contains(types, typ) })
	}
	return len(common) > 0
}

// unitTypesOf lists the unit definitions a unit is declared by.
func (m *Model) unitTypesOf(unit *symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, super := range m.DirectSupertypes(unit) {
		if super.Kind == symbols.SymbolAttributeDef {
			out = append(out, super)
		}
	}
	return out
}
