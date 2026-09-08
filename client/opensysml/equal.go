package opensysml

import (
	"math"
	"math/big"
	"slices"
)

// Equal reports whether two values are the same value to the model, as the
// service judges a Set's membership: numbers by value, so a whole Real is the
// Int of its value and a Complex on the real axis is its real part, exactly
// across the whole Int range; a Sequence's order counts and a Set's does not,
// nor does a member it lists twice; a Quantity is one with any commensurable
// quantity of the same magnitude over their base units, so 1 m is 100 cm,
// while one carrying no reduction is compared in its unit as written; a
// MeasurementRef is one reduction at one scale however spelt, except that a
// named unit of dimension one is only its own declaration (rad is not sr); an
// EnumLiteral is its LiteralID, whatever else describes it; a Null is one
// whatever its reason; and a nil Value equals only another nil.
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
		return ok && x.subsetOf(y) && y.subsetOf(x)
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
		return ok && measurementRefEqual(x, y)
	case EnumLiteral:
		y, ok := b.(EnumLiteral)
		return ok && x.LiteralID == y.LiteralID
	default:
		return a == b
	}
}

// Contains reports whether value is a member of the set.
func (s Set) Contains(value Value) bool {
	return slices.ContainsFunc(s, func(e Value) bool { return Equal(e, value) })
}

func (s Set) subsetOf(t Set) bool {
	for _, e := range s {
		if !t.Contains(e) {
			return false
		}
	}
	return true
}

// repeated returns the first member the set lists twice, if any.
func (s Set) repeated() (Value, bool) {
	for i, e := range s {
		if s[:i].Contains(e) {
			return e, true
		}
	}
	return nil, false
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
	if a.Term == nil || b.Term == nil {
		return numbersEqual(a.Magnitude, b.Magnitude) && a.Unit == b.Unit && unitTermEqual(a.Term, b.Term)
	}
	if !a.Term.commensurable(*b.Term) || a.Term.zeroScale() || b.Term.zeroScale() {
		return false
	}
	if x, y, ok := exactBaseMagnitudes(a, b); ok {
		return x.Cmp(y) == 0
	}
	return a.baseMagnitude() == b.baseMagnitude()
}

// exactBaseMagnitudes expresses two Int magnitudes over their base units as
// rationals, which is exact only while both scales are whole.
func exactBaseMagnitudes(a, b Quantity) (*big.Rat, *big.Rat, bool) {
	x, xok := a.Magnitude.(Int)
	y, yok := b.Magnitude.(Int)
	if !xok || !yok || !a.Term.whole() || !b.Term.whole() {
		return nil, nil, false
	}
	return a.Term.scaled(int64(x)), b.Term.scaled(int64(y)), true
}

func (q Quantity) baseMagnitude() float64 {
	var m float64
	switch x := q.Magnitude.(type) {
	case Int:
		m = float64(x)
	case Real:
		m = float64(x)
	}
	return m * q.Term.ScaleNum / q.Term.ScaleDen
}

// measurementRefEqual holds for one reduction at one scale (SI::'m/s' is m/s, km/m
// is m/mm); a named unit reducing to nothing is only the declaration it names.
func measurementRefEqual(a, b MeasurementRef) bool {
	if a.Term == nil || b.Term == nil {
		return a.Unit == b.Unit && a.UnitID == b.UnitID && unitTermEqual(a.Term, b.Term)
	}
	if !a.Term.same(*b.Term) {
		return false
	}
	if len(a.Term.exponents()) == 0 && (a.UnitID != "" || b.UnitID != "") {
		return a.UnitID == b.UnitID
	}
	return true
}

// same reports one reduction: commensurable at one scale, however the ratio is written.
func (t UnitTerm) same(other UnitTerm) bool {
	return t.commensurable(other) && !t.zeroScale() && !other.zeroScale() &&
		t.ScaleNum*other.ScaleDen == other.ScaleNum*t.ScaleDen
}

// exponents sums the term's factors by base unit, dropping those that cancel,
// so two reductions over the same base units compare however they are listed.
func (t UnitTerm) exponents() map[string]float64 {
	totals := make(map[string]float64, len(t.Factors))
	for _, f := range t.Factors {
		totals[f.UnitID] += f.Exponent
	}
	for id, exponent := range totals {
		if exponent == 0 {
			delete(totals, id)
		}
	}
	return totals
}

// commensurable reports whether a magnitude in t converts into other.
func (t UnitTerm) commensurable(other UnitTerm) bool {
	x, y := t.exponents(), other.exponents()
	if len(x) != len(y) {
		return false
	}
	for id, exponent := range x {
		if y[id] != exponent {
			return false
		}
	}
	return true
}

func (t UnitTerm) zeroScale() bool { return t.ScaleNum == 0 || t.ScaleDen == 0 }

func (t UnitTerm) whole() bool { return isWhole(t.ScaleNum) && isWhole(t.ScaleDen) }

func isWhole(f float64) bool { return f == math.Trunc(f) && !math.IsInf(f, 0) }

// scaled is magnitude times the term's scale, exactly; the scale must be whole.
func (t UnitTerm) scaled(magnitude int64) *big.Rat {
	num, den := new(big.Rat).SetFloat64(t.ScaleNum), new(big.Rat).SetFloat64(t.ScaleDen)
	m := new(big.Rat).SetInt64(magnitude)
	return m.Mul(m, num.Quo(num, den))
}

func unitTermEqual(a, b *UnitTerm) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.ScaleNum == b.ScaleNum && a.ScaleDen == b.ScaleDen && slices.Equal(a.Factors, b.Factors)
}
