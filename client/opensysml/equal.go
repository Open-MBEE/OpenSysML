package opensysml

import (
	"math"
	"slices"
)

// Equal reports whether two values are the same value to the model, as the
// service judges a Set's membership: numbers by value, so a whole Real is the
// Int of its value and a Complex on the real axis is its real part, exactly
// across the whole Int range; a Sequence's order counts and a Set's does not;
// a Quantity is one in its unit as written; a Null is one whatever its reason;
// and a nil Value equals only another nil.
func Equal(a, b Value) bool {
	switch x := a.(type) {
	case nil:
		return b == nil
	case Int, Real, Complex:
		return numbersEqual(a, b)
	case Null:
		_, ok := b.(Null)
		return ok
	case Sequence:
		y, ok := b.(Sequence)
		return ok && slices.EqualFunc(x, y, Equal)
	case Set:
		y, ok := b.(Set)
		if !ok || len(x) != len(y) {
			return false
		}
		for _, e := range x {
			if !y.Contains(e) {
				return false
			}
		}
		return true
	case Array:
		y, ok := b.(Array)
		return ok && slices.Equal(x.Dimensions, y.Dimensions) && slices.EqualFunc(x.Elements, y.Elements, Equal)
	case Vector:
		y, ok := b.(Vector)
		return ok && slices.EqualFunc(x, y, func(m, n Number) bool { return numbersEqual(m, n) })
	case VectorQuantity:
		y, ok := b.(VectorQuantity)
		return ok && slices.EqualFunc(x, y, quantityEqual)
	case TensorQuantity:
		y, ok := b.(TensorQuantity)
		return ok && slices.Equal(x.Dimensions, y.Dimensions) && slices.EqualFunc(x.Components, y.Components, quantityEqual)
	case Quantity:
		y, ok := b.(Quantity)
		return ok && quantityEqual(x, y)
	case MeasurementRef:
		y, ok := b.(MeasurementRef)
		return ok && x.Unit == y.Unit && x.UnitID == y.UnitID && unitTermEqual(x.Term, y.Term)
	default:
		return a == b
	}
}

// Contains reports whether value is a member of the set.
func (s Set) Contains(value Value) bool {
	return slices.ContainsFunc(s, func(e Value) bool { return Equal(e, value) })
}

// numbersEqual compares an Int, Real or Complex with any value by numeric
// value; a Complex off the real axis equals only the same Complex.
func numbersEqual(a, b Value) bool {
	if z, ok := a.(Complex); ok {
		if imag(z) != 0 {
			return a == b
		}
		a = Real(real(z))
	}
	if z, ok := b.(Complex); ok {
		if imag(z) != 0 {
			return false
		}
		b = Real(real(z))
	}
	switch x := a.(type) {
	case Int:
		switch y := b.(type) {
		case Int:
			return x == y
		case Real:
			return realIsInt(float64(y), int64(x))
		}
	case Real:
		switch y := b.(type) {
		case Int:
			return realIsInt(float64(x), int64(y))
		case Real:
			return x == y
		}
	}
	return false
}

// realIsInt reports whether r is exactly the integer n, never rounding n.
func realIsInt(r float64, n int64) bool {
	return r == math.Trunc(r) && r >= math.MinInt64 && r < -math.MinInt64 && int64(r) == n
}

func quantityEqual(a, b Quantity) bool {
	return numbersEqual(a.Magnitude, b.Magnitude) && a.Unit == b.Unit && unitTermEqual(a.Term, b.Term)
}

func unitTermEqual(a, b *UnitTerm) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.ScaleNum == b.ScaleNum && a.ScaleDen == b.ScaleDen && slices.Equal(a.Factors, b.Factors)
}
