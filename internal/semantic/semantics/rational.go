package semantics

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"math/bits"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// ErrRationalSizeLimit reports a Rational result whose numerator and
// denominator together would need more bits than the evaluation's numeric size
// budget allows. A Rational is never rounded to fit.
var ErrRationalSizeLimit = errors.New("rational size limit exceeded")

// A Rational (KerML 1.0 §9.3.2.2.8) is held exactly, in lowest terms with a
// positive denominator. One whose terms fit int64 is held in Int and den, with
// rat nil; any other is held as an immutable big.Rat, so each Rational has one
// representation. A whole Rational stays a Rational: `6 / 3` is the Rational 2.

// RatValue is the Rational r, normalized. The Value takes r over: the caller
// must not modify it afterwards.
func RatValue(r *big.Rat) Value {
	if r.Num().IsInt64() && r.Num().Int64() != math.MinInt64 && r.Denom().IsUint64() && r.Denom().Uint64() <= math.MaxUint32 {
		return Value{Kind: ValRational, Int: r.Num().Int64(), den: uint32(r.Denom().Uint64())}
	}
	return Value{Kind: ValRational, ext: r}
}

// FracValue is the Rational num/den in lowest terms; ok=false for a zero den.
func FracValue(num, den int64) (Value, bool) {
	if den == 0 {
		return Value{}, false
	}
	if v, ok := smallRat(num, den); ok {
		return v, true
	}
	return RatValue(new(big.Rat).SetFrac64(num, den)), true
}

// smallRat normalizes num/den within int64; ok=false when that would overflow.
func smallRat(num, den int64) (Value, bool) {
	if num == math.MinInt64 || den == math.MinInt64 {
		return Value{}, false
	}
	if den < 0 {
		num, den = -num, -den
	}
	if g := gcd64(abs64(num), den); g > 1 {
		num, den = num/g, den/g
	}
	if den > math.MaxUint32 {
		return Value{}, false
	}
	return Value{Kind: ValRational, Int: num, den: uint32(den)}, true
}

func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func gcd64(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	if a == 0 {
		return 1
	}
	return a
}

// IsRational reports whether v is an exact Rational (not an Integer).
func (v Value) IsRational() bool { return v.Kind == ValRational }

// IsExact reports whether v is an exact number: an Integer or a Rational.
func (v Value) IsExact() bool { return v.Kind == ValInt || v.Kind == ValRational }

// ratView is the exact number v (Integer or Rational) as a big.Rat that must
// not be modified.
func (v Value) ratView() *big.Rat {
	switch {
	case v.ext != nil:
		return v.ext
	case v.Kind == ValRational:
		return new(big.Rat).SetFrac64(v.Int, int64(v.den))
	}
	return new(big.Rat).SetInt64(v.Int)
}

// Rat returns the exact number v (Integer or Rational) as a fresh big.Rat the
// caller may modify.
func (v Value) Rat() *big.Rat { return new(big.Rat).Set(v.ratView()) }

// small reports v's terms when v is an Integer within int64 or a Rational held
// in int64 terms.
func (v Value) small() (num, den int64, ok bool) {
	switch {
	case v.Kind == ValRational && v.ext == nil:
		return v.Int, int64(v.den), true
	case v.Kind == ValInt && v.ext == nil && v.Int != math.MinInt64:
		return v.Int, 1, true
	}
	return 0, 0, false
}

// RatNumer is the numerator, in lowest terms, of the exact number v.
func (v Value) RatNumer() Value {
	if num, _, ok := v.small(); ok {
		return IntValue(num)
	}
	if v.Kind == ValInt {
		return v
	}
	return BigIntValue(new(big.Int).Set(v.ext.Num()))
}

// RatDenom is the positive denominator, in lowest terms, of the exact number v.
func (v Value) RatDenom() Value {
	if _, den, ok := v.small(); ok {
		return IntValue(den)
	}
	if v.Kind == ValInt {
		return IntValue(1)
	}
	return BigIntValue(new(big.Int).Set(v.ext.Denom()))
}

// RatSign is -1, 0 or 1 by the sign of the exact number v.
func (v Value) RatSign() int {
	if v.Kind == ValInt {
		return v.IntSign()
	}
	if v.ext != nil {
		return v.ext.Sign()
	}
	switch {
	case v.Int < 0:
		return -1
	case v.Int > 0:
		return 1
	}
	return 0
}

// RatBitLen is the size of the exact number v: the bits of its numerator's
// magnitude plus the bits of its denominator.
func (v Value) RatBitLen() int64 {
	if num, den, ok := v.small(); ok {
		return int64(bits.Len64(uint64(abs64(num))) + bits.Len64(uint64(den)))
	}
	r := v.ratView()
	return int64(r.Num().BitLen() + r.Denom().BitLen())
}

// RatIsWhole reports whether the exact number v has no fractional part.
func (v Value) RatIsWhole() bool {
	if v.Kind == ValInt {
		return true
	}
	if v.ext != nil {
		return v.ext.IsInt()
	}
	return v.den == 1
}

// ratToFloat is the exact number v rounded to the nearest float64, ties to
// even; one beyond the float64 range rounds to the infinity of its sign.
func ratToFloat(v Value) float64 {
	if num, den, ok := v.small(); ok && abs64(num) <= 1<<53 && den <= 1<<53 {
		return float64(num) / float64(den)
	}
	f, _ := v.ratView().Float64()
	return f
}

// RationalOfReal is the Rational a finite Real exactly is.
func RationalOfReal(r float64) (Value, bool) {
	if math.IsNaN(r) || math.IsInf(r, 0) {
		return Value{}, false
	}
	if r == math.Trunc(r) && math.Abs(r) < 1<<53 {
		return Value{Kind: ValRational, Int: int64(r), den: 1}, true
	}
	return RatValue(new(big.Rat).SetFloat64(r)), true
}

// RationalSizeExceeded reports a Rational result of bits numerator-plus-
// denominator bits beyond the maxBits an evaluation allows.
func RationalSizeExceeded(bits, maxBits int64) error {
	return fmt.Errorf("%w: the result needs at least %d bits of numerator and denominator, beyond the %d-bit size budget", ErrRationalSizeLimit, bits, maxBits)
}

// sizedRat is the Rational r, refused when its terms exceed maxBits.
func sizedRat(r *big.Rat, maxBits int64) (Value, error) {
	if bits := int64(r.Num().BitLen() + r.Denom().BitLen()); bits > maxBits {
		return Value{}, RationalSizeExceeded(bits, maxBits)
	}
	return RatValue(r), nil
}

// RatOf is the exact number v (Integer or Rational) as a Rational.
func RatOf(v Value) Value {
	if v.Kind == ValRational {
		return v
	}
	if v.ext == nil && v.Int != math.MinInt64 {
		return Value{Kind: ValRational, Int: v.Int, den: 1}
	}
	if v.ext == nil {
		return RatValue(new(big.Rat).SetInt64(v.Int))
	}
	return Value{Kind: ValRational, ext: v.ext}
}

// RatArith is exact addition, subtraction, multiplication, division and
// truncated remainder over Integers and Rationals, as RationalFunctions
// declares them: the result is a Rational, whole or not. A zero divisor is
// ErrDivisionByZero; a result beyond maxBits is ErrRationalSizeLimit.
func RatArith(op ast.OperatorKind, a, b Value, maxBits int64) (Value, error) {
	if (op == ast.OpDiv || op == ast.OpMod) && b.RatSign() == 0 {
		return Value{}, ErrDivisionByZero
	}
	if an, ad, ok := a.small(); ok {
		if bn, bd, ok := b.small(); ok {
			if res, ok := smallRatArith(op, an, ad, bn, bd); ok {
				if bits := res.RatBitLen(); bits > maxBits {
					return Value{}, RationalSizeExceeded(bits, maxBits)
				}
				return res, nil
			}
		}
	}
	x, y := a.ratView(), b.ratView()
	switch op {
	case ast.OpAdd:
		return sizedRat(new(big.Rat).Add(x, y), maxBits)
	case ast.OpSub:
		return sizedRat(new(big.Rat).Sub(x, y), maxBits)
	case ast.OpMul:
		return sizedRat(new(big.Rat).Mul(x, y), maxBits)
	case ast.OpDiv:
		return sizedRat(new(big.Rat).Quo(x, y), maxBits)
	case ast.OpMod:
		q := new(big.Rat).Quo(x, y)
		t := new(big.Int).Quo(q.Num(), q.Denom())
		r := new(big.Rat).Sub(x, new(big.Rat).Mul(y, new(big.Rat).SetInt(t)))
		return sizedRat(r, maxBits)
	}
	return Value{}, fmt.Errorf("%w: '%s' is no Rational arithmetic operator", ErrArithmeticDomain, op)
}

// smallRatArith is RatArith within int64 terms; ok=false when an intermediate
// would overflow, so the caller computes it over math/big.
func smallRatArith(op ast.OperatorKind, an, ad, bn, bd int64) (Value, bool) {
	switch op {
	case ast.OpAdd, ast.OpSub:
		if op == ast.OpSub {
			if bn == math.MinInt64 {
				return Value{}, false
			}
			bn = -bn
		}
		g := gcd64(ad, bd)
		l, ok1 := mulInt(an, bd/g)
		r, ok2 := mulInt(bn, ad/g)
		d, ok3 := mulInt(ad, bd/g)
		if !ok1 || !ok2 || !ok3 {
			return Value{}, false
		}
		n, ok := int64Arith(ast.OpAdd, l, r)
		if !ok {
			return Value{}, false
		}
		return smallRat(n, d)
	case ast.OpMul:
		g1, g2 := gcd64(abs64(an), bd), gcd64(abs64(bn), ad)
		n, ok1 := mulInt(an/g1, bn/g2)
		d, ok2 := mulInt(ad/g2, bd/g1)
		if !ok1 || !ok2 {
			return Value{}, false
		}
		return smallRat(n, d)
	case ast.OpDiv:
		if bn == math.MinInt64 {
			return Value{}, false
		}
		if bn < 0 {
			an, bn = -an, -bn
		}
		return smallRatArith(ast.OpMul, an, ad, bd, bn)
	}
	return Value{}, false
}

// CompareExactReal orders the exact number v against the non-NaN Real r
// exactly, neither rounded to the other.
func CompareExactReal(v Value, r float64) int {
	if v.Kind == ValInt {
		return CompareIntReal(v, r)
	}
	switch {
	case math.IsInf(r, 1):
		return -1
	case math.IsInf(r, -1):
		return 1
	}
	x, _ := RationalOfReal(r)
	return CompareRat(v, x)
}

// RatNeg is the negation of the exact number v, of v's kind.
func RatNeg(v Value) Value {
	if v.Kind == ValInt {
		return IntNeg(v)
	}
	if v.ext == nil {
		return Value{Kind: ValRational, Int: -v.Int, den: v.den}
	}
	return RatValue(new(big.Rat).Neg(v.ext))
}

// RatAbs is the magnitude of the exact number v, of v's kind.
func RatAbs(v Value) Value {
	if v.RatSign() < 0 {
		return RatNeg(v)
	}
	return v
}

// CompareRat orders two exact numbers (Integers or Rationals) exactly.
func CompareRat(a, b Value) int {
	if a.Kind == ValInt && b.Kind == ValInt {
		return CompareInt(a, b)
	}
	if an, ad, ok := a.small(); ok {
		if bn, bd, ok := b.small(); ok {
			l, ok1 := mulInt(an, bd)
			r, ok2 := mulInt(bn, ad)
			if ok1 && ok2 {
				switch {
				case l < r:
					return -1
				case l > r:
					return 1
				}
				return 0
			}
		}
	}
	return a.ratView().Cmp(b.ratView())
}

// RatFloor is the greatest Integer not above the exact number v.
func RatFloor(v Value) Value {
	if v.Kind == ValInt {
		return v
	}
	if v.ext == nil {
		d := int64(v.den)
		q := v.Int / d
		if v.Int%d != 0 && v.Int < 0 {
			q--
		}
		return IntValue(q)
	}
	q, _ := new(big.Int).DivMod(v.ext.Num(), v.ext.Denom(), new(big.Int))
	return BigIntValue(q)
}

// RatRound is the Integer nearest the exact number v, a half rounding away
// from zero as RealFunctions::round does over a Real.
func RatRound(v Value) Value {
	if v.Kind == ValInt {
		return v
	}
	half := Value{Kind: ValRational, Int: 1, den: 2}
	shifted, err := RatArith(ast.OpAdd, RatAbs(v), half, math.MaxInt64)
	if err != nil {
		return Value{}
	}
	n := RatFloor(shifted)
	if v.RatSign() < 0 {
		return IntNeg(n)
	}
	return n
}

// RatTrunc is the exact number v rounded toward zero, as an Integer.
func RatTrunc(v Value) Value {
	if v.RatSign() < 0 {
		return IntNeg(RatFloor(RatNeg(v)))
	}
	return RatFloor(v)
}

// RatPow is base**n for an exact base and an Integer exponent, the Rational
// RationalFunctions::'**' declares. A negative exponent of zero is
// ErrArithmeticDomain; a result beyond maxBits is ErrRationalSizeLimit,
// refused before it is computed.
func RatPow(base, n Value, maxBits int64) (Value, error) {
	sign := n.IntSign()
	if sign == 0 {
		return Value{Kind: ValRational, Int: 1, den: 1}, nil
	}
	if base.RatSign() == 0 {
		if sign < 0 {
			return Value{}, fmt.Errorf("%w: 0 ** %s is undefined (negative exponent)", ErrArithmeticDomain, n.FormatInt())
		}
		return RatOf(base), nil
	}
	r := base.ratView()
	num, den := r.Num(), r.Denom()
	if num.CmpAbs(den) == 0 && den.Cmp(big.NewInt(1)) == 0 {
		if num.Sign() > 0 || n.bigView().Bit(0) == 0 {
			return Value{Kind: ValRational, Int: 1, den: 1}, nil
		}
		return Value{Kind: ValRational, Int: -1, den: 1}, nil
	}
	e := new(big.Int).Abs(n.bigView())
	// |num| and den are coprime, so the power's terms need at least
	// (bitlen-1)*e+1 bits each, summed over the terms that are not 1.
	lower := new(big.Int)
	for _, t := range []*big.Int{num, den} {
		if t.CmpAbs(big.NewInt(1)) != 0 {
			term := new(big.Int).Mul(big.NewInt(int64(t.BitLen()-1)), e)
			lower.Add(lower, term.Add(term, big.NewInt(1)))
		}
	}
	if !lower.IsInt64() || lower.Int64() > maxBits {
		b := int64(math.MaxInt64)
		if lower.IsInt64() {
			b = lower.Int64()
		}
		return Value{}, RationalSizeExceeded(b, maxBits)
	}
	pn := new(big.Int).Exp(num, e, nil)
	pd := new(big.Int).Exp(den, e, nil)
	if sign < 0 {
		pn, pd = pd, pn
	}
	return sizedRat(new(big.Rat).SetFrac(pn, pd), maxBits)
}

// ParseRational reads decimal notation — an optional sign, digits with an
// optional fraction and an optional decimal exponent — as the exact Rational
// it denotes: `0.1` is 1/10, `1.5e3` is 1500. Other notation is
// ErrRealNotation; a value whose terms exceed maxBits is ErrRationalSizeLimit.
func ParseRational(text string, maxBits int64) (Value, error) {
	if !isRealNotation(text) {
		return Value{}, ErrRealNotation
	}
	mantissa, exponent := text, ""
	if i := strings.IndexAny(text, "eE"); i >= 0 {
		mantissa, exponent = text[:i], text[i+1:]
	}
	neg := false
	switch {
	case strings.HasPrefix(mantissa, "-"):
		neg, mantissa = true, mantissa[1:]
	case strings.HasPrefix(mantissa, "+"):
		mantissa = mantissa[1:]
	}
	whole, frac, _ := strings.Cut(mantissa, ".")
	digits := strings.TrimLeft(whole+frac, "0")
	if digits == "" {
		return Value{Kind: ValRational, Int: 0, den: 1}, nil
	}
	scale := -int64(len(frac))
	if exponent != "" {
		e, err := strconv.ParseInt(exponent, 10, 64)
		if err != nil || e > math.MaxInt64/4 || e < math.MinInt64/4 {
			return Value{}, RationalSizeExceeded(math.MaxInt64, maxBits)
		}
		scale += e
	}
	trimmed := strings.TrimRight(digits, "0")
	scale += int64(len(digits) - len(trimmed))
	digits = trimmed
	// 10^k needs more than 3.32*k bits; the significand needs 3.32 per digit, less one.
	if lower := int64(float64(len(digits)-1)*math.Log2(10)) + int64(float64(abs64(scale))*math.Log2(10)); lower > maxBits {
		return Value{}, RationalSizeExceeded(lower, maxBits)
	}
	if scale >= -18 && scale <= 0 && len(digits) <= 18 {
		n, _ := strconv.ParseInt(digits, 10, 64)
		if neg {
			n = -n
		}
		v, _ := FracValue(n, pow10(-scale))
		return v, nil
	}
	n, _ := new(big.Int).SetString(digits, 10)
	if neg {
		n.Neg(n)
	}
	p := new(big.Int).Exp(big.NewInt(10), big.NewInt(abs64(scale)), nil)
	r := new(big.Rat)
	if scale >= 0 {
		r.SetInt(n.Mul(n, p))
	} else {
		r.SetFrac(n, p)
	}
	return sizedRat(r, maxBits)
}

func pow10(k int64) int64 {
	p := int64(1)
	for ; k > 0; k-- {
		p *= 10
	}
	return p
}

// ParseRationalText reads a Rational as ToRational does: decimal notation, or
// the `numer/denom` form a non-terminating Rational prints as, each term an
// optionally signed decimal Integer and the denominator nonzero.
func ParseRationalText(text string, maxBits int64) (Value, error) {
	if n, d, ok := strings.Cut(text, "/"); ok {
		num, okN := ParseInteger(n)
		den, okD := ParseInteger(d)
		if !okN || !okD {
			return Value{}, ErrRealNotation
		}
		if den.IntSign() == 0 {
			return Value{}, ErrDivisionByZero
		}
		return RatArith(ast.OpDiv, num, den, maxBits)
	}
	return ParseRational(text, maxBits)
}

// FormatRational renders the Rational v exactly. One whose denominator has no
// prime factor but 2 and 5 is a terminating decimal and prints as one, as a
// Real does (`0.1`, `2.0`, `1.5e-05`); any other prints as `numer/denom`
// (`1/3`), since no finite decimal is it.
func (v Value) FormatRational() string {
	num, den := v.RatNumer(), v.RatDenom()
	d := den.BigInt()
	twos := d.TrailingZeroBits()
	d.Rsh(d, twos)
	fives := uint(0)
	five, ten := big.NewInt(5), big.NewInt(10)
	for m := new(big.Int); ; fives++ {
		q, r := new(big.Int).QuoRem(d, five, m)
		if r.Sign() != 0 {
			break
		}
		d = q
	}
	if d.Cmp(big.NewInt(1)) != 0 {
		return num.FormatInt() + "/" + den.FormatInt()
	}
	k := max(twos, fives)
	scaled := num.BigInt()
	scaled.Mul(scaled, new(big.Int).Exp(ten, big.NewInt(int64(k)), nil))
	scaled.Quo(scaled, den.bigView())
	return formatDecimal(scaled, int64(k))
}

// formatDecimal renders n·10^-k in FormatReal's layout: positional between
// 1e-4 and 1e21, exponent notation outside, with a ".0" on a whole value.
func formatDecimal(n *big.Int, k int64) string {
	sign := ""
	if n.Sign() < 0 {
		sign = "-"
	}
	digits := new(big.Int).Abs(n).String()
	if digits == "0" {
		return "0.0"
	}
	trimmed := strings.TrimRight(digits, "0")
	k -= int64(len(digits) - len(trimmed))
	digits = trimmed
	exp := int64(len(digits)) - k - 1
	if exp < -4 || exp >= 21 {
		mantissa := digits[:1]
		if len(digits) > 1 {
			mantissa += "." + digits[1:]
		}
		esign := "+"
		if exp < 0 {
			esign, exp = "-", -exp
		}
		e := strconv.FormatInt(exp, 10)
		if len(e) < 2 {
			e = "0" + e
		}
		return sign + mantissa + "e" + esign + e
	}
	switch {
	case k <= 0:
		return sign + digits + strings.Repeat("0", int(-k)) + ".0"
	case int64(len(digits)) > k:
		cut := int64(len(digits)) - k
		return sign + digits[:cut] + "." + digits[cut:]
	}
	return sign + "0." + strings.Repeat("0", int(k-int64(len(digits)))) + digits
}

// BinaryExact is the binary64 a Rational is exactly, when one is: 0.5 and
// 1800.0 are, 0.1 and 1 / 3 are not.
func (v Value) BinaryExact() (float64, bool) {
	if v.Kind != ValRational {
		return 0, false
	}
	f, exact := v.Rat().Float64()
	return f, exact && !math.IsInf(f, 0)
}

// CanonicalRational reads the Rational a wire encoding spells as two decimals:
// a numerator over a positive denominator sharing no factor with it, and no
// binary64 holds exactly, which crosses as a double instead; so no Rational has
// two spellings.
func CanonicalRational(numerator, denominator string) (Value, bool) {
	num, okNum := ParseInteger(numerator)
	den, okDen := ParseInteger(denominator)
	if !okNum || !okDen || CompareInt(den, IntValue(1)) < 0 {
		return Value{}, false
	}
	n, d := num.BigInt(), den.BigInt()
	if new(big.Int).GCD(nil, nil, new(big.Int).Abs(n), d).Cmp(big.NewInt(1)) != 0 {
		return Value{}, false
	}
	v := RatValue(new(big.Rat).SetFrac(n, d))
	if _, exact := v.BinaryExact(); exact {
		return Value{}, false
	}
	return v, true
}
