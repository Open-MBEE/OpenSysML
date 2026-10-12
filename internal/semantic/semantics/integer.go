package semantics

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// DefaultMaxIntegerBits bounds the magnitude of one Integer an evaluation
// computes, in bits: 2^20 bits is 315,653 decimal digits and 128KiB of memory.
// The mathematical Integers have no bound; this one stops an exponentiation or
// a product from exhausting memory or time instead of reporting.
const DefaultMaxIntegerBits int64 = 1 << 20

// MinMaxIntegerBits is the least Integer size budget: every int64 fits within
// it, so machine-word arithmetic never consults the budget.
const MinMaxIntegerBits int64 = 64

// ErrIntegerSizeLimit reports an Integer result whose magnitude would need more
// bits than the evaluation's Integer size budget allows.
var ErrIntegerSizeLimit = errors.New("integer size limit exceeded")

// ErrIntegerUnaddressable is returned when an Integer serves as a position,
// count or bound of something held, and is beyond the 64-bit range any of
// those can take: Integers are unbounded, what they address is not.
var ErrIntegerUnaddressable = errors.New("integer beyond the addressable range")

// IntValue is the Integer i.
func IntValue(i int64) Value { return Value{Kind: ValInt, Int: i} }

// BigIntValue is the Integer b, held in Int when it fits int64. The Value takes
// b over: the caller must not modify it afterwards.
func BigIntValue(b *big.Int) Value {
	if b.IsInt64() {
		return Value{Kind: ValInt, Int: b.Int64()}
	}
	return Value{Kind: ValInt, ext: new(big.Rat).SetInt(b)}
}

// ParseInteger reads decimal Integer notation of any magnitude.
func ParseInteger(text string) (Value, bool) {
	if i, err := strconv.ParseInt(text, 10, 64); err == nil {
		return IntValue(i), true
	}
	if !isIntegerNotation(text) {
		return Value{}, false
	}
	b, ok := new(big.Int).SetString(text, 10)
	if !ok {
		return Value{}, false
	}
	return BigIntValue(b), true
}

// isIntegerNotation reports whether text is an optionally signed run of decimal
// digits, the only notation ParseInteger accepts (big.Int alone also reads "_").
func isIntegerNotation(text string) bool {
	if text != "" && (text[0] == '+' || text[0] == '-') {
		text = text[1:]
	}
	if text == "" {
		return false
	}
	for i := 0; i < len(text); i++ {
		if !isDigit(text[i]) {
			return false
		}
	}
	return true
}

// IsBigInt reports whether v is an Integer outside the int64 range.
func (v Value) IsBigInt() bool { return v.Kind == ValInt && v.ext != nil }

// Int64 returns v as an int64 when it is an Integer within that range.
func (v Value) Int64() (int64, bool) {
	if v.Kind != ValInt || v.ext != nil {
		return 0, false
	}
	return v.Int, true
}

// BigInt returns the Integer v as a fresh big.Int the caller may modify.
func (v Value) BigInt() *big.Int {
	if v.ext != nil {
		return new(big.Int).Set(v.ext.Num())
	}
	return big.NewInt(v.Int)
}

// BigIntView is the Integer v's big.Int when it is beyond int64, nil otherwise;
// the Value shares it, so the caller must not modify it.
func (v Value) BigIntView() *big.Int {
	if v.ext == nil {
		return nil
	}
	return v.ext.Num()
}

// bigView is the Integer v as a big.Int that must not be modified.
func (v Value) bigView() *big.Int {
	if v.ext != nil {
		return v.ext.Num()
	}
	return big.NewInt(v.Int)
}

// IntSign is -1, 0 or 1 by the sign of the Integer v.
func (v Value) IntSign() int {
	if v.ext != nil {
		return v.ext.Num().Sign()
	}
	switch {
	case v.Int < 0:
		return -1
	case v.Int > 0:
		return 1
	}
	return 0
}

// IntBitLen is the number of bits of the Integer v's magnitude.
func (v Value) IntBitLen() int64 {
	if v.ext != nil {
		return int64(v.ext.Num().BitLen())
	}
	if v.Int == math.MinInt64 {
		return 64
	}
	n := v.Int
	if n < 0 {
		n = -n
	}
	return int64(big.NewInt(n).BitLen())
}

// FormatInt renders the Integer v in full decimal notation.
func (v Value) FormatInt() string {
	if v.ext != nil {
		return v.ext.Num().String()
	}
	return strconv.FormatInt(v.Int, 10)
}

// Equal reports whether v and w are the same constant: of one kind, with equal
// payloads. An Integer equals another Integer of the same value however held.
func (v Value) Equal(w Value) bool {
	if v.Kind == ValInt && w.Kind == ValInt {
		return CompareInt(v, w) == 0
	}
	if v.Kind == ValRational && w.Kind == ValRational {
		return CompareRat(v, w) == 0
	}
	return v == w
}

// CompareInt orders two Integers exactly: -1, 0 or 1.
func CompareInt(a, b Value) int {
	if a.ext == nil && b.ext == nil {
		switch {
		case a.Int < b.Int:
			return -1
		case a.Int > b.Int:
			return 1
		}
		return 0
	}
	return a.bigView().Cmp(b.bigView())
}

// CompareIntReal orders the Integer a against the Real r exactly, neither rounded
// to the other: -1, 0 or 1. A NaN r is unordered; the caller decides it.
func CompareIntReal(a Value, r float64) int {
	switch {
	case math.IsInf(r, 1):
		return -1
	case math.IsInf(r, -1):
		return 1
	}
	if a.ext == nil && a.Int >= -1<<53 && a.Int <= 1<<53 {
		f := float64(a.Int)
		switch {
		case f < r:
			return -1
		case f > r:
			return 1
		}
		return 0
	}
	return new(big.Float).SetInt(a.bigView()).Cmp(big.NewFloat(r))
}

// intToFloat is the Integer v rounded to the nearest float64, ties to even; an
// Integer beyond the float64 range rounds to the infinity of its sign.
func intToFloat(v Value) float64 {
	if v.ext == nil {
		return float64(v.Int)
	}
	f, _ := new(big.Float).SetInt(v.ext.Num()).Float64()
	return f
}

// IntegerOfReal is the Integer a finite whole Real is, of any magnitude.
func IntegerOfReal(r float64) (Value, bool) {
	if math.IsNaN(r) || math.IsInf(r, 0) || r != math.Trunc(r) {
		return Value{}, false
	}
	if r >= -float64(math.MinInt64) || r < math.MinInt64 {
		b, _ := new(big.Float).SetFloat64(r).Int(nil)
		return BigIntValue(b), true
	}
	return IntValue(int64(r)), true
}

// IntegerSizeExceeded reports an Integer result of bits magnitude bits beyond
// the maxBits an evaluation allows.
func IntegerSizeExceeded(bits, maxBits int64) error {
	return fmt.Errorf("%w: the result needs at least %d bits, beyond the %d-bit Integer size budget", ErrIntegerSizeLimit, bits, maxBits)
}

// intCell is a fresh big.Int an Integer Value holds without a copy: the numerator
// of a big.Rat over one.
func intCell() (*big.Rat, *big.Int) {
	r := new(big.Rat)
	return r, r.Num()
}

// sizedInt is the Integer cell r holds, refused when its magnitude exceeds maxBits.
func sizedInt(r *big.Rat, maxBits int64) (Value, error) {
	n := r.Num()
	if bits := int64(n.BitLen()); bits > maxBits {
		return Value{}, IntegerSizeExceeded(bits, maxBits)
	}
	if n.IsInt64() {
		return IntValue(n.Int64()), nil
	}
	return Value{Kind: ValInt, ext: r}, nil
}

// IntArith is Integer addition, subtraction and multiplication, shared by the
// folder, the operators and the library functions. A result within int64 is
// computed there; any other is computed exactly, refused only when it would
// exceed maxBits.
func IntArith(op ast.OperatorKind, a, b Value, maxBits int64) (Value, error) {
	if a.ext == nil && b.ext == nil {
		if res, ok := int64Arith(op, a.Int, b.Int); ok {
			return IntValue(res), nil
		}
	}
	return bigIntArith(op, a, b, maxBits)
}

// bigIntArith is IntArith over math/big, for a result or operand beyond int64.
func bigIntArith(op ast.OperatorKind, a, b Value, maxBits int64) (Value, error) {
	x, y := a.bigView(), b.bigView()
	r, n := intCell()
	switch op {
	case ast.OpAdd:
		n.Add(x, y)
		return sizedInt(r, maxBits)
	case ast.OpSub:
		n.Sub(x, y)
		return sizedInt(r, maxBits)
	case ast.OpMul:
		if x.Sign() == 0 || y.Sign() == 0 {
			return IntValue(0), nil
		}
		// The product has at least bitlen(x)+bitlen(y)-1 bits.
		if bits := int64(x.BitLen()) + int64(y.BitLen()) - 1; bits > maxBits {
			return Value{}, IntegerSizeExceeded(bits, maxBits)
		}
		n.Mul(x, y)
		return sizedInt(r, maxBits)
	}
	return Value{}, fmt.Errorf("%w: '%s' is no Integer arithmetic operator", ErrArithmeticDomain, op)
}

// int64Arith is IntArith within int64, reporting ok=false when the result leaves it.
func int64Arith(op ast.OperatorKind, a, b int64) (int64, bool) {
	switch op {
	case ast.OpAdd:
		res := a + b
		return res, (b <= 0 || res > a) && (b >= 0 || res < a)
	case ast.OpSub:
		res := a - b
		return res, (b >= 0 || res > a) && (b <= 0 || res < a)
	case ast.OpMul:
		return mulInt(a, b)
	}
	return 0, false
}

// IntNeg is the negation of the Integer v.
func IntNeg(v Value) Value {
	if v.ext == nil && v.Int != math.MinInt64 {
		return IntValue(-v.Int)
	}
	return BigIntValue(new(big.Int).Neg(v.bigView()))
}

// IntAbs is the magnitude of the Integer v.
func IntAbs(v Value) Value {
	if v.IntSign() < 0 {
		return IntNeg(v)
	}
	return v
}

// IntRem is the remainder of a truncated division of a by b, with a's sign as
// `mod` has it; ok=false on a zero divisor.
func IntRem(a, b Value) (Value, bool) {
	if b.IntSign() == 0 {
		return Value{}, false
	}
	if a.ext == nil && b.ext == nil {
		return IntValue(a.Int % b.Int), true
	}
	return BigIntValue(new(big.Int).Rem(a.bigView(), b.bigView())), true
}

// IntDivTrunc is the quotient of a by b rounded toward zero; ok=false on a zero divisor.
func IntDivTrunc(a, b Value) (Value, bool) {
	if b.IntSign() == 0 {
		return Value{}, false
	}
	if a.ext == nil && b.ext == nil && (a.Int != math.MinInt64 || b.Int != -1) {
		return IntValue(a.Int / b.Int), true
	}
	return BigIntValue(new(big.Int).Quo(a.bigView(), b.bigView())), true
}

// IntPow is a**n for an Integer base and a non-negative Integer exponent,
// refused with ErrIntegerSizeLimit before it is computed when the result would
// need more than maxBits.
func IntPow(a, n Value, maxBits int64) (Value, error) {
	if a.ext == nil && n.ext == nil {
		if res, ok := intPow(a.Int, n.Int); ok {
			return IntValue(res), nil
		}
	}
	switch {
	case n.IntSign() == 0:
		return IntValue(1), nil
	case a.ext == nil && (a.Int == 0 || a.Int == 1):
		return a, nil
	case a.ext == nil && a.Int == -1:
		if n.bigView().Bit(0) == 0 {
			return IntValue(1), nil
		}
		return a, nil
	}
	// |a| >= 2, so a**n has at least (bitlen(a)-1)*n+1 bits.
	lowerBits := new(big.Int).Mul(big.NewInt(a.IntBitLen()-1), n.bigView())
	lowerBits.Add(lowerBits, big.NewInt(1))
	if !lowerBits.IsInt64() || lowerBits.Int64() > maxBits {
		bits := int64(math.MaxInt64)
		if lowerBits.IsInt64() {
			bits = lowerBits.Int64()
		}
		return Value{}, IntegerSizeExceeded(bits, maxBits)
	}
	r, p := intCell()
	p.Exp(a.bigView(), n.bigView(), nil)
	return sizedInt(r, maxBits)
}
