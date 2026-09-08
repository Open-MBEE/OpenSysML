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
		if m.Conforms(typ, target) {
			return ClassifiesAll
		}
		if m.Conforms(target, typ) {
			verdict = ClassifiesSome
		}
	}
	return verdict
}
