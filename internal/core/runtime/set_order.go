package runtime

import (
	"cmp"
	"math"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// canonicalLess is the total order a set enumerates its elements in, so that
// every operation consuming a set — a trace rendering it, `collect` over it, a
// sequence it flows into — sees equal sets alike. Values order by class first
// (null, Booleans, numbers, complex numbers, strings, quantities, enumeration
// literals, objects, then every other kind), then within a class by their own
// order where they have one (numeric, lexicographic, dimension then magnitude,
// declaration, object identity), then by their trace rendering, and finally by
// their elements, so that only equal values compare as neither before nor after.
func canonicalLess(a, b Value) bool {
	return canonicalCompare(a, b) < 0
}

// canonicalCompare orders a before b as negative, after as positive, and equal
// values — valueEqual ones, whose position a set never depends on — as zero.
func canonicalCompare(a, b Value) int {
	ca, cb := canonicalClass(a), canonicalClass(b)
	if ca != cb {
		return cmp.Compare(ca, cb)
	}
	switch ca {
	case classNull:
		return 0
	case classBool:
		return compareBool(a.Const.Bool, b.Const.Bool)
	case classNumber:
		return compareNumbers(a.Const, b.Const)
	case classComplex:
		x, y := a.Complex(), b.Complex()
		if c := cmp.Compare(real(x), real(y)); c != 0 {
			return c
		}
		return cmp.Compare(imag(x), imag(y))
	case classString:
		return strings.Compare(a.Str(), b.Str())
	case classQuantity:
		qa, qb := a.Quantity(), b.Quantity()
		if c := strings.Compare(qa.Unit.Term.DimensionKey(), qb.Unit.Term.DimensionKey()); c != 0 {
			return c
		}
		if c, err := semantics.CompareMagnitudes(*qa, *qb); err == nil {
			return c
		}
	case classEnumLiteral:
		return compareSymbols(a.Literal(), b.Literal())
	case classObject:
		if a.Kind != b.Kind {
			return cmp.Compare(a.Kind, b.Kind)
		}
		if a.Kind == ValVariant {
			return compareSymbols(a.Variant(), b.Variant())
		}
		return cmp.Compare(a.Instance, b.Instance)
	}
	if c := strings.Compare(FormatTraceValue(a), FormatTraceValue(b)); c != 0 {
		return c
	}
	if a.Kind != b.Kind {
		return cmp.Compare(a.Kind, b.Kind)
	}
	switch a.Kind {
	case ValSequence, ValSet:
		return compareElements(elementsOf(a), elementsOf(b))
	case ValArray:
		return compareElements(a.Array().Elements, b.Array().Elements)
	}
	return 0
}

// compareElements orders two element lists lexicographically, a shorter prefix first.
func compareElements(a, b []Value) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if c := canonicalCompare(a[i], b[i]); c != 0 {
			return c
		}
	}
	return cmp.Compare(len(a), len(b))
}

// compareSymbols orders declarations by qualified name, then by the document
// and position declaring them, so same-named literals of different
// enumerations still order deterministically.
func compareSymbols(a, b *symbols.Symbol) int {
	if a == b {
		return 0
	}
	if a == nil || b == nil {
		return compareBool(a != nil, b != nil)
	}
	if c := strings.Compare(symbols.FQNOf(a), symbols.FQNOf(b)); c != 0 {
		return c
	}
	if c := strings.Compare(a.DocName, b.DocName); c != 0 {
		return c
	}
	return cmp.Compare(a.DeclSpan.Offset, b.DeclSpan.Offset)
}

func compareBool(a, b bool) int {
	if a == b {
		return 0
	}
	if !a {
		return -1
	}
	return 1
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

// compareNumbers orders the numeric constants, infinity above every finite number.
func compareNumbers(a, b semantics.Value) int {
	if a.Kind == semantics.ValInt && b.Kind == semantics.ValInt {
		return cmp.Compare(a.Int, b.Int)
	}
	return cmp.Compare(numberOf(a), numberOf(b))
}

func numberOf(v semantics.Value) float64 {
	if v.Kind == semantics.ValInfinity {
		return math.Inf(1)
	}
	return v.AsReal()
}
