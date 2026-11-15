package semantics

import (
	"cmp"
	"fmt"
	"math"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// PositiveInfinity is `*`, the infinity above every finite number, and
// NegativeInfinity the one below them all: the two values KerML 9.3.2.2.9 adds to Real.
func PositiveInfinity() Value { return Value{Kind: ValInfinity} }

// NegativeInfinity is `-*`. See PositiveInfinity.
func NegativeInfinity() Value { return Value{Kind: ValInfinity, Int: -1} }

// signedInfinity is the infinity of the given non-zero sign.
func signedInfinity(sign int) Value {
	if sign < 0 {
		return NegativeInfinity()
	}
	return PositiveInfinity()
}

// IsInfinite reports whether the value is either infinity.
func (v Value) IsInfinite() bool { return v.Kind == ValInfinity }

// IsNegativeInfinity reports whether the value is `-*`.
func (v Value) IsNegativeInfinity() bool { return v.Kind == ValInfinity && v.Int < 0 }

// IsUnbounded reports whether the value is the unbounded `*`, the positive infinity.
func (v Value) IsUnbounded() bool { return v.Kind == ValInfinity && v.Int >= 0 }

// IsExtendedNumeric reports whether the value is a number or an infinity: a
// value of the extended Real.
func (v Value) IsExtendedNumeric() bool { return v.IsNumeric() || v.IsInfinite() }

// Sign is -1, 0 or 1 as an extended number is negative, zero or positive; a NaN
// and a value that is no number give 0.
func (v Value) Sign() int {
	switch v.Kind {
	case ValInt:
		return v.IntSign()
	case ValRational:
		return v.RatSign()
	case ValReal:
		switch {
		case v.Real > 0:
			return 1
		case v.Real < 0:
			return -1
		}
	case ValInfinity:
		if v.Int < 0 {
			return -1
		}
		return 1
	}
	return 0
}

// ExtendedReal is the Real a binary64 value denotes, its two infinities being
// the extended reals' own: `*` and `-*`.
func ExtendedReal(f float64) Value {
	switch {
	case math.IsInf(f, 1):
		return PositiveInfinity()
	case math.IsInf(f, -1):
		return NegativeInfinity()
	}
	return Value{Kind: ValReal, Real: f}
}

// InfinityOrder compares l and r where at least one is an infinity: `*`
// exceeds every finite number, `-*` is below them all, and each equals itself.
// ok is false when an operand is neither a number nor an infinity.
func InfinityOrder(l, r Value) (order int, ok bool) {
	if !l.IsExtendedNumeric() || !r.IsExtendedNumeric() {
		return 0, false
	}
	ordinal := func(v Value) int {
		if v.IsInfinite() {
			return v.Sign()
		}
		return 0
	}
	lo, ro := ordinal(l), ordinal(r)
	switch {
	case lo == ro:
		return 0, true
	case lo > ro:
		return 1, true
	default:
		return -1, true
	}
}

// zeroLike is the zero an operation over finite and infinite operands yields:
// a Real zero beside a Real operand, an exact Rational zero beside a Rational
// one, the Integer zero otherwise, as the finite operation promotes.
func zeroLike(l, r Value) Value {
	switch {
	case l.Kind == ValReal || r.Kind == ValReal:
		return Value{Kind: ValReal}
	case l.Kind == ValRational || r.Kind == ValRational:
		return Value{Kind: ValRational, den: 1}
	}
	return IntValue(0)
}

func undefinedOverInfinity(op ast.OperatorKind, l, r Value) error {
	return fmt.Errorf("%w: %s %s %s is undefined in the extended reals",
		ErrArithmeticDomain, FormatConst(l), op, FormatConst(r))
}

// InfinityArith is op over extended numbers of which at least one is an
// infinity, in the affinely extended reals: a sum or product the limit
// determines is that limit, and an indeterminate form (`* - *`, `0 * *`,
// `* / *`, `* ** 0`, `1 ** *`, any `%`) is ErrArithmeticDomain. A zero divisor
// is ErrDivisionByZero as it is for a finite dividend.
func InfinityArith(op ast.OperatorKind, l, r Value) (Value, error) {
	if !l.IsExtendedNumeric() || !r.IsExtendedNumeric() {
		return Value{}, fmt.Errorf("%w: %s requires numeric operands", ErrArithmeticDomain, op)
	}
	ls, rs := l.Sign(), r.Sign()
	switch op {
	case ast.OpAdd, ast.OpSub:
		if op == ast.OpSub {
			rs = -rs
		}
		if l.IsInfinite() && r.IsInfinite() && ls != rs {
			return Value{}, undefinedOverInfinity(op, l, r)
		}
		if l.IsInfinite() {
			return signedInfinity(ls), nil
		}
		return signedInfinity(rs), nil
	case ast.OpMul:
		if ls == 0 || rs == 0 {
			return Value{}, undefinedOverInfinity(op, l, r)
		}
		return signedInfinity(ls * rs), nil
	case ast.OpDiv:
		switch {
		case l.IsInfinite() && r.IsInfinite():
			return Value{}, undefinedOverInfinity(op, l, r)
		case r.IsInfinite():
			return zeroLike(l, r), nil
		case rs == 0:
			return Value{}, ErrDivisionByZero
		}
		return signedInfinity(ls * rs), nil
	case ast.OpPow:
		return infinityPow(l, r)
	}
	return Value{}, undefinedOverInfinity(op, l, r)
}

// infinityPow is l ** r with an infinite base or exponent, where the limit exists.
func infinityPow(l, r Value) (Value, error) {
	undefined := undefinedOverInfinity(ast.OpPow, l, r)
	if r.IsInfinite() {
		// An infinite exponent: the base's magnitude against one decides.
		var magnitude int
		switch {
		case l.IsInfinite():
			magnitude = 1
		case l.IsExact():
			magnitude = CompareRat(RatAbs(l), IntValue(1))
		default:
			magnitude = cmp.Compare(math.Abs(l.Real), 1)
		}
		grows := magnitude > 0
		if r.Sign() < 0 {
			grows = magnitude < 0
		}
		switch {
		case magnitude == 0, grows && l.Sign() <= 0:
			return Value{}, undefined
		case grows:
			return PositiveInfinity(), nil
		}
		return zeroLike(l, r), nil
	}
	// An infinite base under a finite exponent.
	switch rs := r.Sign(); {
	case rs == 0:
		return Value{}, undefined
	case rs < 0:
		return zeroLike(l, r), nil
	case l.Sign() > 0:
		return PositiveInfinity(), nil
	}
	// `-*` raised to a positive exponent: defined for a whole one, by its parity.
	n, whole := wholeExponent(r)
	if !whole {
		return Value{}, undefined
	}
	if n.bigView().Bit(0) == 1 {
		return NegativeInfinity(), nil
	}
	return PositiveInfinity(), nil
}

// wholeExponent is the Integer a finite exponent equals, if it is whole.
func wholeExponent(r Value) (Value, bool) {
	switch r.Kind {
	case ValInt:
		return r, true
	case ValRational:
		if r.RatIsWhole() {
			return r.RatNumer(), true
		}
	case ValReal:
		return IntegerOfReal(r.Real)
	}
	return Value{}, false
}
