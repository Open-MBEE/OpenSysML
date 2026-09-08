package semantics

import "github.com/Open-MBEE/OpenSysML/internal/core/symbols"

// TypeClassification is how far the types a value is of settle whether a target
// type classifies it, the question a CastExpression asks (KerML 1.1 §8.3.4.9).
type TypeClassification uint8

const (
	// ClassifiesNone is a target disjoint from every type the value is of, so no
	// value of those types is one of the target's.
	ClassifiesNone TypeClassification = iota
	// ClassifiesAll is a type of the value specializing the target, so every
	// value of that type is one of the target's.
	ClassifiesAll
	// ClassifiesSome is a target specializing a type of the value: some values of
	// that type are the target's and the value itself decides which.
	ClassifiesSome
)

// ClassifiesTypes reports how target classifies a value known to be of types.
func (m *Model) ClassifiesTypes(types []*symbols.Symbol, target *symbols.Symbol) TypeClassification {
	if m == nil || target == nil {
		return ClassifiesNone
	}
	verdict := ClassifiesNone
	for _, typ := range types {
		if typ == nil {
			continue
		}
		if m.Classifies(target, typ) {
			return ClassifiesAll
		}
		if m.Conforms(target, typ) {
			verdict = ClassifiesSome
		}
	}
	return verdict
}

// Classifies reports whether every value of typ is one of target's: typ conforms
// to target, or target unions a type typ conforms to.
func (m *Model) Classifies(target, typ *symbols.Symbol) bool {
	if m == nil {
		return false
	}
	return m.Conforms(typ, target) || m.unionsAType(target, typ, nil)
}

// unionsAType reports whether target unions a type typ conforms to, so every
// value of typ is one of target's (KerML 1.0 §8.3.3); unioning guards a cycle.
func (m *Model) unionsAType(target, typ *symbols.Symbol, unioning map[*symbols.Symbol]bool) bool {
	unions := m.UnioningTypes(target)
	if len(unions) == 0 || unioning[target] {
		return false
	}
	if unioning == nil {
		unioning = make(map[*symbols.Symbol]bool)
	}
	unioning[target] = true
	defer delete(unioning, target)
	for _, u := range unions {
		if m.Conforms(typ, u) || m.unionsAType(u, typ, unioning) {
			return true
		}
	}
	return false
}
