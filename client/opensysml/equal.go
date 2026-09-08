package opensysml

import "slices"

// Equal reports whether two values are the same value: the same kind holding
// the same contents. An Int is never a Real, a Sequence's order counts, a
// Set's does not, a Null is one whatever its reason, and a nil Value equals
// only another nil.
func Equal(a, b Value) bool {
	switch x := a.(type) {
	case nil:
		return b == nil
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
		return ok && slices.Equal(x, y)
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

func quantityEqual(a, b Quantity) bool {
	return a.Magnitude == b.Magnitude && a.Unit == b.Unit && unitTermEqual(a.Term, b.Term)
}

func unitTermEqual(a, b *UnitTerm) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.ScaleNum == b.ScaleNum && a.ScaleDen == b.ScaleDen && slices.Equal(a.Factors, b.Factors)
}
