package runtime

import (
	"fmt"
	"math"
	"math/cmplx"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// realConst is the Real runtime value of a float64.
func realConst(x float64) Value {
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: x}}
}

// realPart is the Real a complex number on the real axis is, reporting whether
// the value is one: a ValComplex whose imaginary part is zero.
func (v Value) realPart() (float64, bool) {
	if v.Kind != ValComplex || imag(v.Complex()) != 0 {
		return 0, false
	}
	return real(v.Complex()), true
}

// holdsComplex reports whether any element is a ValComplex.
func holdsComplex(elements []Value) bool {
	for _, elem := range elements {
		if elem.Kind == ValComplex {
			return true
		}
	}
	return false
}

// complexOf reads a value as a complex number: a ValComplex, or a numeric
// constant, which ScalarValues declares a Complex (Real :> Complex) with a zero
// imaginary part. Nothing else is one.
func complexOf(v Value) (complex128, bool) {
	switch v.Kind {
	case ValComplex:
		return v.Complex(), true
	case ValConst:
		if v.Const.IsNumeric() {
			return complex(asReal(v.Const), 0), true
		}
	}
	return 0, false
}

// complexResult wraps a computed complex number, reporting a part that is not a
// finite number as the Real functions report one.
func complexResult(z complex128) (Value, error) {
	for _, part := range [2]float64{real(z), imag(z)} {
		if _, err := semantics.RealResult(part); err != nil {
			return Value{}, err
		}
	}
	return NewComplex(z), nil
}

// complexEqual compares two values as complex numbers: a Complex equals the Real
// on the real axis it is, and neither equals a value that is no number.
func complexEqual(a, b Value) bool {
	x, ok := complexOf(a)
	if !ok {
		return false
	}
	y, ok := complexOf(b)
	return ok && x == y
}

// complexOperands reads a binary operator's operands as complex numbers when one
// of them is a Complex and the other is a number, so the operator is
// ComplexFunctions'. Any other pairing is not, and is reported by the caller.
func complexOperands(left, right Value) (x, y complex128, ok bool) {
	if left.Kind != ValComplex && right.Kind != ValComplex {
		return 0, 0, false
	}
	if x, ok = complexOf(left); !ok {
		return 0, 0, false
	}
	if y, ok = complexOf(right); !ok {
		return 0, 0, false
	}
	return x, y, true
}

// complexArithmetic applies an arithmetic operator to two complex numbers as the
// ComplexFunctions declaration of the same name does; `%` has none.
func complexArithmetic(op ast.OperatorKind, x, y complex128, left, right Value, span source.Span) (Value, error) {
	switch op {
	case ast.OpAdd:
		return complexResult(x + y)
	case ast.OpSub:
		return complexResult(x - y)
	case ast.OpMul:
		return complexResult(x * y)
	case ast.OpDiv:
		if y == 0 {
			return Value{}, ErrDivisionByZero
		}
		return complexResult(x / y)
	case ast.OpPow:
		return complexPow(x, y)
	}
	return Value{}, &OperandTypeError{
		Op:    op.String(),
		Left:  describeOperand(left),
		Right: describeOperand(right),
		Span:  span,
	}
}

// complexPow is x to the power y over the complex numbers. Zero has no value
// raised to a power whose real part is not positive, which is reported.
func complexPow(x, y complex128) (Value, error) {
	if x == 0 && real(y) <= 0 {
		return Value{}, fmt.Errorf(
			"%w: no value for %s to the power %s",
			semantics.ErrArithmeticDomain, FormatComplex(x), FormatComplex(y),
		)
	}
	if n, ok := smallWholeExponent(y); ok {
		return complexResult(complexIntPow(x, n))
	}
	return complexResult(cmplx.Pow(x, y))
}

// maxExactExponent bounds the whole exponents raised by repeated squaring, which
// keeps `i ** 2` exactly -1 where exp(y*log(x)) would carry rounding noise.
const maxExactExponent = 1 << 10

// smallWholeExponent reads y as the whole number it is, within maxExactExponent.
func smallWholeExponent(y complex128) (int64, bool) {
	re := real(y)
	if imag(y) != 0 || re != math.Trunc(re) || math.Abs(re) > maxExactExponent {
		return 0, false
	}
	return int64(re), true
}

// complexIntPow is x to a whole power by repeated squaring, the reciprocal for a
// negative one.
func complexIntPow(x complex128, n int64) complex128 {
	if n < 0 {
		return 1 / complexIntPow(x, -n)
	}
	acc := complex(1, 0)
	for ; n > 0; n >>= 1 {
		if n&1 == 1 {
			acc *= x
		}
		x *= x
	}
	return acc
}
