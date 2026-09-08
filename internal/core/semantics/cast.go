package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

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
	return m.classifiesTypes(types, target, nil)
}

// Classifies reports whether every value of typ is one of target's.
func (m *Model) Classifies(target, typ *symbols.Symbol) bool {
	return m.ClassifiesTypes([]*symbols.Symbol{typ}, target) == ClassifiesAll
}

// classifiesTypes answers ClassifiesTypes; composing holds the composed targets
// being read, so a composition naming itself does not recur.
func (m *Model) classifiesTypes(
	types []*symbols.Symbol, target *symbols.Symbol, composing map[*symbols.Symbol]bool,
) TypeClassification {
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
	composed, ok := m.classifiesComposed(types, target, composing)
	if !ok || composed == ClassifiesNone {
		return verdict
	}
	if composed == ClassifiesAll || verdict == ClassifiesNone {
		return composed
	}
	return verdict
}

// classifiesComposed reads a target composed of other types — a union, an
// intersection or a difference (KerML 1.0 §8.3.3) — as those types classify: the
// values of a union are those of any of its types, of an intersection those of
// every type, of a difference those of the first that are none of the rest.
func (m *Model) classifiesComposed(
	types []*symbols.Symbol, target *symbols.Symbol, composing map[*symbols.Symbol]bool,
) (TypeClassification, bool) {
	unions, intersects, differences := m.UnioningTypes(target),
		m.IntersectingTypes(target), m.DifferencingTypes(target)
	if len(unions)+len(intersects)+len(differences) == 0 || composing[target] {
		return ClassifiesNone, false
	}
	if composing == nil {
		composing = make(map[*symbols.Symbol]bool)
	}
	composing[target] = true
	defer delete(composing, target)

	// Every composition of a type constrains its values, so all of them must hold.
	constraints := make([]TypeClassification, 0, 3)
	if len(unions) > 0 {
		constraints = append(constraints, classifiesAny(m.classifiesEach(types, unions, composing)))
	}
	if len(intersects) > 0 {
		constraints = append(constraints, classifiesAll(m.classifiesEach(types, intersects, composing)))
	}
	if len(differences) > 0 {
		constraints = append(constraints, classifiesExcept(m.classifiesEach(types, differences, composing)))
	}
	return classifiesAll(constraints), true
}

// classifiesEach classifies types by each of a composition's operands in order.
func (m *Model) classifiesEach(
	types, operands []*symbols.Symbol, composing map[*symbols.Symbol]bool,
) []TypeClassification {
	out := make([]TypeClassification, 0, len(operands))
	for _, operand := range operands {
		out = append(out, m.classifiesTypes(types, operand, composing))
	}
	return out
}

// classifiesAny is how a union of the classified types classifies: a value one of
// them classifies is one of the union's.
func classifiesAny(verdicts []TypeClassification) TypeClassification {
	out := ClassifiesNone
	for _, v := range verdicts {
		if v == ClassifiesAll {
			return ClassifiesAll
		}
		if v == ClassifiesSome {
			out = ClassifiesSome
		}
	}
	return out
}

// classifiesAll is how an intersection of the classified types classifies: only a
// value every one of them classifies is one of the intersection's.
func classifiesAll(verdicts []TypeClassification) TypeClassification {
	out := ClassifiesAll
	for _, v := range verdicts {
		if v == ClassifiesNone {
			return ClassifiesNone
		}
		if v == ClassifiesSome {
			out = ClassifiesSome
		}
	}
	return out
}

// classifiesExcept is how a difference of the classified types classifies: a value
// of the first that none of the rest classifies.
func classifiesExcept(verdicts []TypeClassification) TypeClassification {
	if len(verdicts) == 0 || verdicts[0] == ClassifiesNone {
		return ClassifiesNone
	}
	out := verdicts[0]
	for _, v := range verdicts[1:] {
		if v == ClassifiesAll {
			return ClassifiesNone
		}
		if v == ClassifiesSome {
			out = ClassifiesSome
		}
	}
	return out
}

// MayShareValues reports whether a cast from a value of typ to target can select
// anything: either type classifies values of the other, or one is composed of a
// type that does — a value of `Wheeled unions Car, Truck` may well be a Car.
func (m *Model) MayShareValues(target, typ *symbols.Symbol) bool {
	return m.mayShareValues(target, typ, nil)
}

func (m *Model) mayShareValues(target, typ *symbols.Symbol, reading map[*symbols.Symbol]bool) bool {
	if m == nil || target == nil || typ == nil || reading[typ] {
		return false
	}
	if m.ClassifiesTypes([]*symbols.Symbol{typ}, target) != ClassifiesNone {
		return true
	}
	if reading == nil {
		reading = make(map[*symbols.Symbol]bool)
	}
	reading[typ] = true
	defer delete(reading, typ)
	for _, kind := range []ast.RelationshipKind{ast.RelUnions, ast.RelIntersects, ast.RelDifferences} {
		for _, operand := range m.composedOperands(typ, kind) {
			if m.mayShareValues(target, operand, reading) {
				return true
			}
		}
	}
	return false
}
