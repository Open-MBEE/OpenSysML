package runtime

import (
	"math"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// canonicalLess is the total order a set enumerates its elements in, so that
// every operation consuming a set — a trace rendering it, `collect` over it, a
// sequence it flows into — sees equal sets alike. Values order by class first
// (null, Booleans, numbers, complex numbers, strings, quantities, enumeration
// literals, objects, then every other kind), then within a class by their own
// order where they have one (numeric, lexicographic, dimension then magnitude,
// object identity) and by their trace rendering otherwise.
func canonicalLess(a, b Value) bool {
	ca, cb := canonicalClass(a), canonicalClass(b)
	if ca != cb {
		return ca < cb
	}
	switch ca {
	case classBool:
		return !a.Const.Bool && b.Const.Bool
	case classNumber:
		return numberLess(a.Const, b.Const)
	case classComplex:
		x, y := a.Complex(), b.Complex()
		if real(x) != real(y) {
			return real(x) < real(y)
		}
		if imag(x) != imag(y) {
			return imag(x) < imag(y)
		}
	case classString:
		return a.Str() < b.Str()
	case classQuantity:
		qa, qb := a.Quantity(), b.Quantity()
		if da, db := qa.Unit.Term.DimensionKey(), qb.Unit.Term.DimensionKey(); da != db {
			return da < db
		}
		if c, err := semantics.CompareMagnitudes(*qa, *qb); err == nil && c != 0 {
			return c < 0
		}
	case classObject:
		if a.Instance != b.Instance {
			return a.Instance < b.Instance
		}
	}
	return FormatTraceValue(a) < FormatTraceValue(b)
}

const (
	classNull = iota
	classBool
	classNumber
	classComplex
	classString
	classQuantity
	classEnumLiteral
	classObject
	classOther
)

// canonicalClass groups values so that only values with an order of their own
// are compared by it.
func canonicalClass(v Value) int {
	switch v.Kind {
	case ValNull, ValInvalid:
		return classNull
	case ValConst:
		if v.Const.Kind == semantics.ValBool {
			return classBool
		}
		return classNumber
	case ValComplex:
		return classComplex
	case ValString:
		return classString
	case ValQuantity:
		if v.Quantity() != nil {
			return classQuantity
		}
	case ValEnumLiteral:
		return classEnumLiteral
	case ValInstance, ValVariant:
		return classObject
	}
	return classOther
}

// numberLess orders the numeric constants, infinity above every finite number.
func numberLess(a, b semantics.Value) bool {
	if a.Kind == semantics.ValInt && b.Kind == semantics.ValInt {
		return a.Int < b.Int
	}
	return numberOf(a) < numberOf(b)
}

func numberOf(v semantics.Value) float64 {
	if v.Kind == semantics.ValInfinity {
		return math.Inf(1)
	}
	return v.AsReal()
}
