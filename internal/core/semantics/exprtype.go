package semantics

import (
	"math"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// PrimType classifies a symbol against the stdlib scalar value types
// (`ScalarValues`). It is the lattice the expression type checker reasons over;
// anything outside it (parts, items, enumerations, collections, unresolved
// names) is PrimUnknown and suppresses checking.
type PrimType int

const (
	PrimUnknown PrimType = iota
	PrimBoolean
	PrimString
	PrimNatural
	PrimInteger
	PrimRational
	PrimReal
	PrimComplex
	PrimNumber
	// PrimExpression is a body `{ … }` written as a value: the Expression itself,
	// known and no scalar, unlike PrimUnknown which suppresses checking.
	PrimExpression
)

var primNames = map[PrimType]string{
	PrimUnknown:    "unknown",
	PrimBoolean:    "Boolean",
	PrimString:     "String",
	PrimNatural:    "Natural",
	PrimInteger:    "Integer",
	PrimRational:   "Rational",
	PrimReal:       "Real",
	PrimComplex:    "Complex",
	PrimNumber:     "Number",
	PrimExpression: "Expression",
}

// String returns the stdlib name of the type, or "unknown".
func (p PrimType) String() string {
	if n, ok := primNames[p]; ok {
		return n
	}
	return "unknown"
}

// numericRank orders the numeric tower: a value of lower rank conforms to a
// type of higher rank (Natural ⊑ Integer ⊑ Rational ⊑ Real ⊑ Complex ⊑ Number).
var numericRank = map[PrimType]int{
	PrimNatural:  0,
	PrimInteger:  1,
	PrimRational: 2,
	PrimReal:     3,
	PrimComplex:  4,
	PrimNumber:   5,
}

// IsNumeric reports whether p is part of the numeric tower.
func (p PrimType) IsNumeric() bool {
	_, ok := numericRank[p]
	return ok
}

// PrimConforms reports whether a value of type from is acceptable where to is
// expected. PrimUnknown on either side conforms, so partial information never
// produces a diagnostic.
func PrimConforms(from, to PrimType) bool {
	if from == PrimUnknown || to == PrimUnknown {
		return true
	}
	if from == to {
		return true
	}
	if from.IsNumeric() && to.IsNumeric() {
		return numericRank[from] <= numericRank[to]
	}
	return false
}

// PrimWiden returns the more general of two numeric types, or PrimUnknown if
// either is not numeric.
func PrimWiden(a, b PrimType) PrimType {
	if !a.IsNumeric() || !b.IsNumeric() {
		return PrimUnknown
	}
	if numericRank[a] >= numericRank[b] {
		return a
	}
	return b
}

// PrimTypeOfValue returns the narrowest scalar type a constant is a value of;
// a whole number is an Integer (or Natural) whatever kind computed it.
func PrimTypeOfValue(v Value) PrimType {
	switch v.Kind {
	case ValBool:
		return PrimBoolean
	case ValInt, ValReal:
		if whole, ok := v.WholeNumber(); ok {
			if whole >= 0 {
				return PrimNatural
			}
			return PrimInteger
		}
		// A finite float is a ratio; an infinity is a Real only.
		switch {
		case math.IsNaN(v.Real):
			return PrimUnknown
		case math.IsInf(v.Real, 0):
			return PrimReal
		}
		return PrimRational
	}
	return PrimUnknown
}

// scalarFQNs maps stdlib scalar qualified names to their lattice element.
var scalarFQNs = map[string]PrimType{
	"ScalarValues::Boolean":  PrimBoolean,
	"ScalarValues::String":   PrimString,
	"ScalarValues::Positive": PrimNatural,
	"ScalarValues::Natural":  PrimNatural,
	"ScalarValues::Integer":  PrimInteger,
	"ScalarValues::Rational": PrimRational,
	"ScalarValues::Real":     PrimReal,
	"ScalarValues::Complex":  PrimComplex,
	"ScalarValues::Number":   PrimNumber,
}

// scalarTable resolves the stdlib scalar symbols once per model, by identity,
// so a user-declared type merely named "Integer" is never mistaken for one.
func (m *Model) scalarTable() map[*symbols.Symbol]PrimType {
	if m.scalars != nil {
		return m.scalars
	}
	table := make(map[*symbols.Symbol]PrimType, len(scalarFQNs))
	if m.resolver != nil && m.resolver.Index() != nil {
		for fqn, prim := range scalarFQNs {
			for _, sym := range m.resolver.Index().LookupQualified(fqn) {
				if sym != nil {
					table[sym] = prim
				}
			}
		}
	}
	m.scalars = table
	return table
}

// ScalarLatticeElement is the lattice element sym is, as opposed to one it
// specializes; false for any type outside ScalarValues.
func (m *Model) ScalarLatticeElement(sym *symbols.Symbol) (PrimType, bool) {
	if m == nil || sym == nil {
		return PrimUnknown, false
	}
	prim, ok := m.scalarTable()[sym]
	return prim, ok
}

// ScalarSymbol returns the library definition a lattice element stands for
// (`ScalarValues::Natural` for PrimNatural), or nil when none is loaded.
func (m *Model) ScalarSymbol(prim PrimType) *symbols.Symbol {
	fqn, ok := scalarDefFQNs[prim]
	if !ok || m == nil || m.resolver == nil || m.resolver.Index() == nil {
		return nil
	}
	for _, sym := range m.resolver.Index().LookupQualified(fqn) {
		if sym != nil {
			return sym
		}
	}
	return nil
}

// scalarDefFQNs names the definition each lattice element stands for.
var scalarDefFQNs = map[PrimType]string{
	PrimBoolean:    "ScalarValues::Boolean",
	PrimString:     "ScalarValues::String",
	PrimNatural:    "ScalarValues::Natural",
	PrimInteger:    "ScalarValues::Integer",
	PrimRational:   "ScalarValues::Rational",
	PrimReal:       "ScalarValues::Real",
	PrimComplex:    "ScalarValues::Complex",
	PrimNumber:     "ScalarValues::Number",
	PrimExpression: fqnEvaluation,
}

// PrimTypeOf classifies sym against the scalar lattice. A definition is
// classified by itself or its nearest scalar supertype; a usage by the type it
// is typed by (typing is a generalization edge, so the same walk covers both).
// Symbols with no scalar ancestor are PrimUnknown.
func (m *Model) PrimTypeOf(sym *symbols.Symbol) PrimType {
	if m == nil || sym == nil {
		return PrimUnknown
	}
	if cached, ok := m.primTypes[sym]; ok {
		return cached
	}
	table := m.scalarTable()
	prim := PrimUnknown
	if p, ok := table[sym]; ok {
		prim = p
	} else {
		// AllSupertypes is breadth-first over declaration order, so the nearest
		// scalar ancestor (the most specific one) is found first.
		for _, super := range m.AllSupertypes(sym) {
			if p, ok := table[super]; ok {
				prim = p
				break
			}
		}
	}
	if m.primTypes == nil {
		m.primTypes = make(map[*symbols.Symbol]PrimType)
	}
	m.primTypes[sym] = prim
	return prim
}

// DeclaresResolvedType reports whether sym resolves a declared type — directly, via
// subsetting/redefinition, or an implicit kind base no prim value can be of (Part, not Anything).
func (m *Model) DeclaresResolvedType(sym *symbols.Symbol, prim PrimType) bool {
	if m == nil || sym == nil {
		return false
	}
	seen := map[*symbols.Symbol]bool{sym: true}
	queue := []*symbols.Symbol{sym}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		bases := m.implicitBases(cur)
		for _, sup := range m.DirectSupertypes(cur) {
			if seen[sup] {
				continue
			}
			if slices.Contains(bases, sup) {
				if m.kindBaseExcludes(sup, prim) {
					return true
				}
				continue
			}
			if !sup.IsFeature() {
				return true
			}
			seen[sup] = true
			queue = append(queue, sup)
		}
		if m.kindBaseExcludes(m.implicitKerMLFeatureBase(cur), prim) {
			return true
		}
	}
	return false
}

// kindBaseExcludes reports whether no value of prim is of the type a kind base implies.
func (m *Model) kindBaseExcludes(base *symbols.Symbol, prim PrimType) bool {
	scalar := m.ScalarSymbol(prim)
	if scalar == nil || base == nil {
		return false
	}
	typ := base
	if base.IsFeature() {
		if typ = m.nearestDeclaredType(base); typ == nil {
			return false
		}
	}
	return !m.Conforms(scalar, typ)
}

// CouldHold reports whether a feature of the given type could hold a value of
// the scalar type prim: either the type conforms to it, or it conforms to the
// type, as every value does to `Anything`.
func (m *Model) CouldHold(sym *symbols.Symbol, prim PrimType) bool {
	if m == nil || sym == nil || prim == PrimUnknown {
		return true
	}
	if p := m.PrimTypeOf(sym); p != PrimUnknown {
		return PrimConforms(p, prim)
	}
	for scalar, p := range m.scalarTable() {
		if p != prim {
			continue
		}
		if scalar == sym {
			return true
		}
		for _, super := range m.AllSupertypes(scalar) {
			if super == sym {
				return true
			}
		}
	}
	return false
}
