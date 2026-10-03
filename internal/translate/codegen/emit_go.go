package codegen

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

const goAssign = "%s = %s"

// goPrelude is the support library of a generated Go program. It mirrors
// cPrelude: the same checks, the same errors, the same output format.
const goPrelude = `package main

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"
)

type sysmlError struct{ msg string }

func (e sysmlError) Error() string { return e.msg }

func sysmlFail(msg string) { panic(sysmlError{msg}) }

func sysmlFailf(format string, args ...any) { sysmlFail(fmt.Sprintf(format, args...)) }

var sysmlDepth int

func sysmlEnter() {
	if sysmlDepth >= sysmlMaxCalcDepth {
		sysmlFail("calc recursion limit exceeded")
	}
	sysmlDepth++
}

func sysmlLeave() { sysmlDepth-- }

var (
	sysmlSteps    int64
	sysmlMaxSteps int64 = sysmlDefaultMaxSteps
)

// sysmlStep spends n evaluation steps of the run's budget.
func sysmlStep(n int64) struct{} {
	if sysmlSteps += n; sysmlSteps > sysmlMaxSteps {
		sysmlStepFail()
	}
	return struct{}{}
}

func sysmlStepFail() {
	sysmlSteps = sysmlMaxSteps + 1
	sysmlFailf("evaluation step limit exceeded (%d steps; raise OPENSYSML_MAX_STEPS to allow more)", sysmlMaxSteps)
}

// sysmlAfter is x, evaluated after the call giving its first argument.
func sysmlAfter[T any](_ struct{}, x T) T { return x }

// sysmlReadBudget is the positive budget the variable env sets, def when unset.
func sysmlReadBudget(env, counts string, def int64) int64 {
	raw, ok := os.LookupEnv(env)
	if !ok || strings.TrimSpace(raw) == "" {
		return def
	}
	n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s=%q is not an integer: set it to a positive number of %s (default %d)\n", env, raw, counts, def)
		os.Exit(2)
	}
	if n <= 0 {
		fmt.Fprintf(os.Stderr, "%s=%q must be greater than zero: the budget is what stops a runaway run (default %d)\n", env, raw, def)
		os.Exit(2)
	}
	return n
}

// sysmlInt is an Integer, unbounded as KerML's are: small holds one within
// int64 and big is nil, else big holds it. A result within int64 is always
// small, so each Integer has one representation.
type sysmlInt struct {
	small int64
	big   *big.Int
}

var sysmlMaxIntegerBits int64 = sysmlDefaultMaxIntegerBits

func sysmlI(v int64) sysmlInt { return sysmlInt{small: v} }

func (a sysmlInt) toBig() *big.Int {
	if a.big != nil {
		return a.big
	}
	return big.NewInt(a.small)
}

func (a sysmlInt) String() string {
	if a.big != nil {
		return a.big.String()
	}
	return strconv.FormatInt(a.small, 10)
}

func (a sysmlInt) sign() int {
	if a.big != nil {
		return a.big.Sign()
	}
	switch {
	case a.small < 0:
		return -1
	case a.small > 0:
		return 1
	}
	return 0
}

// sysmlWrap is b as an Integer, demoted to int64 when it fits.
func sysmlWrap(b *big.Int) sysmlInt {
	if b.IsInt64() {
		return sysmlInt{small: b.Int64()}
	}
	return sysmlInt{big: b}
}

func sysmlSizeFail(bits int64) {
	sysmlFailf("integer size limit exceeded: the result needs at least %d bits, beyond the %d-bit Integer size budget (raise OPENSYSML_MAX_INTEGER_BITS to allow more)", bits, sysmlMaxIntegerBits)
}

// sysmlSized is sysmlWrap of a computed result, refused beyond the size budget.
func sysmlSized(b *big.Int) sysmlInt {
	if bits := int64(b.BitLen()); bits > sysmlMaxIntegerBits {
		sysmlSizeFail(bits)
	}
	return sysmlWrap(b)
}

func sysmlBigLit(digits string) sysmlInt {
	b, _ := new(big.Int).SetString(digits, 10)
	return sysmlWrap(b)
}

func sysmlAdd(a, b sysmlInt) sysmlInt {
	if a.big == nil && b.big == nil {
		if r := a.small + b.small; (r > a.small) == (b.small > 0) {
			return sysmlInt{small: r}
		}
	}
	return sysmlSized(new(big.Int).Add(a.toBig(), b.toBig()))
}

func sysmlSub(a, b sysmlInt) sysmlInt {
	if a.big == nil && b.big == nil {
		if r := a.small - b.small; (r < a.small) == (b.small > 0) {
			return sysmlInt{small: r}
		}
	}
	return sysmlSized(new(big.Int).Sub(a.toBig(), b.toBig()))
}

func sysmlMulOK(a, b int64) (int64, bool) {
	if a == 0 || b == 0 {
		return 0, true
	}
	if (a == math.MinInt64 && b == -1) || (b == math.MinInt64 && a == -1) {
		return 0, false
	}
	r := a * b
	return r, r/b == a
}

func sysmlMul(a, b sysmlInt) sysmlInt {
	if a.big == nil && b.big == nil {
		if r, ok := sysmlMulOK(a.small, b.small); ok {
			return sysmlInt{small: r}
		}
	}
	x, y := a.toBig(), b.toBig()
	if x.Sign() == 0 || y.Sign() == 0 {
		return sysmlInt{}
	}
	// The product has at least bitlen(x)+bitlen(y)-1 bits.
	if bits := int64(x.BitLen()) + int64(y.BitLen()) - 1; bits > sysmlMaxIntegerBits {
		sysmlSizeFail(bits)
	}
	return sysmlSized(new(big.Int).Mul(x, y))
}

func sysmlNeg(a sysmlInt) sysmlInt {
	if a.big == nil && a.small != math.MinInt64 {
		return sysmlInt{small: -a.small}
	}
	return sysmlWrap(new(big.Int).Neg(a.toBig()))
}

func sysmlMod(a, b sysmlInt) sysmlInt {
	if b.sign() == 0 {
		sysmlFail("division by zero")
	}
	if a.big == nil && b.big == nil {
		return sysmlInt{small: a.small % b.small}
	}
	return sysmlWrap(new(big.Int).Rem(a.toBig(), b.toBig()))
}

func sysmlQuot(a, b sysmlInt) float64 {
	if b.sign() == 0 {
		sysmlFail("division by zero")
	}
	var q float64
	if a.big == nil && b.big == nil {
		q, _ = new(big.Rat).SetFrac64(a.small, b.small).Float64()
	} else {
		q, _ = new(big.Rat).SetFrac(a.toBig(), b.toBig()).Float64()
	}
	return q
}

func sysmlICmp(a, b sysmlInt) int {
	if a.big == nil && b.big == nil {
		switch {
		case a.small < b.small:
			return -1
		case a.small > b.small:
			return 1
		}
		return 0
	}
	return a.toBig().Cmp(b.toBig())
}

// sysmlCmpIR orders the Integer a against the finite Real r exactly, neither
// rounded to the other.
func sysmlCmpIR(a sysmlInt, r float64) int {
	if a.big == nil && a.small > -1<<53 && a.small < 1<<53 {
		switch f := float64(a.small); {
		case f < r:
			return -1
		case f > r:
			return 1
		}
		return 0
	}
	return new(big.Float).SetInt(a.toBig()).Cmp(big.NewFloat(r))
}

// sysmlToReal is the binary64 nearest a, ties to even; beyond the binary64
// range it is an infinity, which the arithmetic it feeds reports.
// sysmlToRealExact is the Integer a in exact Rational arithmetic, which
// compiled code computes in binary64 only over values binary64 holds.
func sysmlToRealExact(a sysmlInt) float64 {
	if a.big != nil || a.small > 1<<53 || a.small < -(1<<53) {
		sysmlFail("unsupported: exact Rational arithmetic over an Integer beyond 2^53, which binary64 does not hold exactly")
	}
	return float64(a.small)
}

// sysmlWhole is the exact whole number r, which binary64 holds below 2^53.
func sysmlWhole(r float64) float64 {
	if r >= 1<<53 || r <= -(1<<53) {
		sysmlFail("unsupported: exact Rational arithmetic reaching 2^53, beyond which binary64 does not hold a whole number exactly")
	}
	return sysmlUnsignedZero(r)
}

// sysmlRatQuot is the exact Integer quotient a/b.
func sysmlRatQuot(a, b sysmlInt) *big.Rat {
	if b.sign() == 0 {
		sysmlFail("division by zero")
	}
	return new(big.Rat).SetFrac(a.toBig(), b.toBig())
}

// sysmlCmpQR orders the exact quotient q against the finite Real r.
func sysmlCmpQR(q *big.Rat, r float64) int { return q.Cmp(new(big.Rat).SetFloat64(r)) }

// sysmlCmpQI orders the exact quotient q against the Integer i.
func sysmlCmpQI(q *big.Rat, i sysmlInt) int { return q.Cmp(new(big.Rat).SetInt(i.toBig())) }

// sysmlUnsignedZero is r with an exact Rational's unsigned zero.
func sysmlUnsignedZero(r float64) float64 {
	if r == 0 {
		return 0
	}
	return r
}

func sysmlToReal(a sysmlInt) float64 {
	if a.big == nil {
		return float64(a.small)
	}
	f, _ := new(big.Float).SetInt(a.big).Float64()
	return f
}

// sysmlNum is a value of a Real-typed feature: an Integer unless real is set.
type sysmlNum struct {
	i    sysmlInt
	r    float64
	real bool
}

func sysmlNI(i sysmlInt) sysmlNum { return sysmlNum{i: i} }
func sysmlNR(r float64) sysmlNum  { return sysmlNum{r: r, real: true} }

func (n sysmlNum) toReal() float64 {
	if n.real {
		return n.r
	}
	return sysmlToReal(n.i)
}

// sysmlNCmp orders two numbers exactly, whatever their kinds.
func sysmlNCmp(a, b sysmlNum) int {
	switch {
	case !a.real && !b.real:
		return sysmlICmp(a.i, b.i)
	case !a.real:
		return sysmlCmpIR(a.i, b.r)
	case !b.real:
		return -sysmlCmpIR(b.i, a.r)
	case a.r < b.r:
		return -1
	case a.r > b.r:
		return 1
	}
	return 0
}

// sysmlNSame is '===' of two numbers: the same kind and value.
func sysmlNSame(a, b sysmlNum) bool { return a.real == b.real && sysmlNCmp(a, b) == 0 }

func sysmlParseNum(s, name string) sysmlNum {
	if digits := strings.TrimLeft(s, "+-"); len(s)-len(digits) <= 1 && digits != "" && strings.Trim(digits, "0123456789") == "" {
		return sysmlNI(sysmlParseInt(s, name))
	}
	return sysmlNR(sysmlParseReal(s, name))
}

func sysmlAtLeast(v sysmlInt, lo int64, typ string) sysmlInt {
	if sysmlICmp(v, sysmlI(lo)) < 0 {
		sysmlFail(fmt.Sprintf("type mismatch: cannot write %s (an Integer) to a feature typed by %s", v, typ))
	}
	return v
}

func sysmlFinite(r float64) float64 {
	if math.IsNaN(r) || math.IsInf(r, 0) {
		sysmlFail("arithmetic overflow: result is not a finite Real")
	}
	return r
}

// Library functions: a NaN result is a domain error, an infinity an overflow.
func sysmlLibReal(r float64) float64 {
	if math.IsNaN(r) {
		sysmlFail("arithmetic domain error: argument outside the function's domain")
	}
	if math.IsInf(r, 0) {
		sysmlFail("arithmetic overflow: result is not a finite Real")
	}
	return r
}

// sysmlLibInt is the Integer a whole Real is, exactly.
func sysmlLibInt(x float64) sysmlInt {
	if math.IsNaN(x) {
		sysmlFail("arithmetic domain error: argument outside the function's domain")
	}
	if x < 9223372036854775808.0 && x >= -9223372036854775808.0 {
		return sysmlInt{small: int64(x)}
	}
	b, _ := new(big.Float).SetFloat64(x).Int(nil)
	return sysmlWrap(b)
}

func sysmlIAbs(a sysmlInt) sysmlInt {
	if a.sign() < 0 {
		return sysmlNeg(a)
	}
	return a
}

func sysmlIMax(a, b sysmlInt) sysmlInt {
	if sysmlICmp(a, b) > 0 {
		return a
	}
	return b
}

func sysmlIMin(a, b sysmlInt) sysmlInt {
	if sysmlICmp(a, b) < 0 {
		return a
	}
	return b
}

func sysmlNaturalArg(v sysmlInt) sysmlInt {
	if v.sign() < 0 {
		sysmlFail("type mismatch: requires Natural arguments")
	}
	return v
}

func sysmlNaturalString(v sysmlInt) string {
	if v.sign() < 0 {
		sysmlFailf("type mismatch: function NaturalFunctions::ToString parameter \"x\" requires a Natural value, got %s", v)
	}
	return v.String()
}

func sysmlLength(x string) sysmlInt { return sysmlI(int64(utf8.RuneCountInString(x))) }

// sysmlPosition is a Substring position as an int64, which any Integer naming a character is.
func sysmlPosition(v sysmlInt, param string) int64 {
	if v.big != nil {
		sysmlFailf("integer beyond the addressable range: StringFunctions::Substring parameter %q is %s", param, v)
	}
	return v.small
}

// sysmlSubstring is StringFunctions::Substring: characters lower to upper,
// counting code points from 1; empty when lower > upper.
func sysmlSubstring(x string, lower, upper sysmlInt) string {
	chars := []rune(x)
	lo, hi := sysmlPosition(lower, "lower"), sysmlPosition(upper, "upper")
	if lo < 1 {
		sysmlFailf("index out of range: function StringFunctions::Substring lower character %d is outside 1..%d", lo, len(chars))
	}
	if lo > hi {
		return ""
	}
	if hi > int64(len(chars)) {
		sysmlFailf("index out of range: function StringFunctions::Substring upper character %d is outside 1..%d", hi, len(chars))
	}
	return string(chars[lo-1 : hi])
}

func sysmlTan(t float64) float64 { return sysmlLibReal(math.Sin(t) / math.Cos(t)) }
func sysmlCot(t float64) float64 { return sysmlLibReal(math.Cos(t) / math.Sin(t)) }

func sysmlLn(x float64) float64 {
	if x <= 0 {
		sysmlFail("arithmetic domain error: the logarithm of a non-positive argument is not a Real (requires x > 0.0)")
	}
	return sysmlLibReal(math.Log(x))
}

func sysmlLog(x, base float64) float64 {
	switch {
	case x <= 0:
		sysmlFail("arithmetic domain error: the logarithm of a non-positive argument is not a Real (requires x > 0.0)")
	case base <= 0:
		sysmlFail("arithmetic domain error: a non-positive base has no logarithm (requires base > 0.0)")
	case base == 1:
		sysmlFail("arithmetic domain error: base 1.0 has no logarithm")
	case base == 10:
		return sysmlLibReal(math.Log10(x))
	case base == 2:
		return sysmlLibReal(math.Log2(x))
	}
	return sysmlLibReal(math.Log(x) / math.Log(base))
}

func sysmlAtan2(y, x float64) float64 {
	if y == 0 && x == 0 {
		sysmlFail("arithmetic domain error: atan2(0.0, 0.0) has no angle")
	}
	return sysmlLibReal(math.Atan2(y, x))
}

func sysmlRDiv(a, b float64) float64 {
	if b == 0 {
		sysmlFail("division by zero")
	}
	return sysmlFinite(a / b)
}

func sysmlRMod(a, b float64) float64 {
	if b == 0 {
		sysmlFail("division by zero")
	}
	return sysmlFinite(math.Mod(a, b))
}

func sysmlIPow64(a, n int64) (int64, bool) {
	res := int64(1)
	for n > 0 {
		var ok bool
		if n&1 == 1 {
			if res, ok = sysmlMulOK(res, a); !ok {
				return 0, false
			}
		}
		if n >>= 1; n == 0 {
			break
		}
		if a, ok = sysmlMulOK(a, a); !ok {
			return 0, false
		}
	}
	return res, true
}

// sysmlIPow is a**n for a non-negative n, refused before it is computed when
// the result would exceed the size budget.
func sysmlIPow(a, n sysmlInt) sysmlInt {
	if a.big == nil && n.big == nil {
		if r, ok := sysmlIPow64(a.small, n.small); ok {
			return sysmlInt{small: r}
		}
	}
	nb := n.toBig()
	switch {
	case nb.Sign() == 0:
		return sysmlI(1)
	case a.big == nil && (a.small == 0 || a.small == 1):
		return a
	case a.big == nil && a.small == -1:
		if nb.Bit(0) == 0 {
			return sysmlI(1)
		}
		return a
	}
	ab := a.toBig()
	// |a| >= 2, so a**n has at least (bitlen(a)-1)*n+1 bits.
	lower := new(big.Int).Mul(big.NewInt(int64(ab.BitLen())-1), nb)
	lower.Add(lower, big.NewInt(1))
	if !lower.IsInt64() || lower.Int64() > sysmlMaxIntegerBits {
		bits := int64(math.MaxInt64)
		if lower.IsInt64() {
			bits = lower.Int64()
		}
		sysmlSizeFail(bits)
	}
	return sysmlSized(new(big.Int).Exp(ab, nb, nil))
}

func sysmlRPow(base, exp float64) float64 {
	switch {
	case base == 0 && exp < 0:
		sysmlFail("arithmetic domain: 0 ** negative exponent is undefined")
	case base < 0 && exp != math.Trunc(exp):
		sysmlFail("arithmetic domain: negative base with fractional exponent is not a Real")
	}
	return sysmlFinite(math.Pow(base, exp))
}

func sysmlParseInt(s, name string) sysmlInt {
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		return sysmlInt{small: v}
	}
	b, ok := new(big.Int).SetString(s, 10)
	if !ok {
		fmt.Fprintf(os.Stderr, "argument %s: %s is not an Integer\n", name, s)
		os.Exit(2)
	}
	return sysmlWrap(b)
}

func sysmlReadMaxIntegerBits() {
	raw, ok := os.LookupEnv("OPENSYSML_MAX_INTEGER_BITS")
	if !ok || strings.TrimSpace(raw) == "" {
		return
	}
	n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "OPENSYSML_MAX_INTEGER_BITS=%q is not an integer: set it to a positive number of bits of one Integer (default %d)\n", raw, sysmlDefaultMaxIntegerBits)
		os.Exit(2)
	}
	if n <= 0 {
		fmt.Fprintf(os.Stderr, "OPENSYSML_MAX_INTEGER_BITS=%q must be greater than zero: the budget is what stops a runaway run (default %d)\n", raw, sysmlDefaultMaxIntegerBits)
		os.Exit(2)
	}
	if n < sysmlMinMaxIntegerBits {
		fmt.Fprintf(os.Stderr, "OPENSYSML_MAX_INTEGER_BITS=%q must be at least %d: every machine-word Integer fits within it (default %d)\n", raw, sysmlMinMaxIntegerBits, sysmlDefaultMaxIntegerBits)
		os.Exit(2)
	}
	sysmlMaxIntegerBits = n
}

func sysmlParseReal(s, name string) float64 {
	if !sysmlRealNotation(s) {
		fmt.Fprintf(os.Stderr, "argument %s: %s is not a finite Real in decimal notation\n", name, s)
		os.Exit(2)
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v == 0 && sysmlNonzeroNotation(s) {
		fmt.Fprintf(os.Stderr, "argument %s: arithmetic overflow: %s is outside the Real range\n", name, s)
		os.Exit(1)
	}
	return sysmlUnsignedZero(v)
}

// sysmlRealNotation reports whether s is decimal Real notation: an optional
// sign, digits with an optional fraction, and an optional decimal exponent.
func sysmlRealNotation(s string) bool {
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	digits := 0
	for i < len(s) && '0' <= s[i] && s[i] <= '9' {
		i, digits = i+1, digits+1
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && '0' <= s[i] && s[i] <= '9' {
			i, digits = i+1, digits+1
		}
	}
	if digits == 0 {
		return false
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		exponent := 0
		for i < len(s) && '0' <= s[i] && s[i] <= '9' {
			i, exponent = i+1, exponent+1
		}
		if exponent == 0 {
			return false
		}
	}
	return i == len(s)
}

// sysmlNonzeroNotation reports whether the significand of decimal notation s
// has a nonzero digit, so a zero it parsed to is an underflow.
func sysmlNonzeroNotation(s string) bool {
	significand, _, _ := strings.Cut(strings.ToLower(s), "e")
	return strings.ContainsAny(significand, "123456789")
}

func sysmlParseBool(s, name string) bool {
	switch s {
	case "true":
		return true
	case "false":
		return false
	}
	fmt.Fprintf(os.Stderr, "argument %s: %s is not a Boolean\n", name, s)
	os.Exit(2)
	return false
}

// sysmlParseString reads a String argument written as a KerML string literal.
func sysmlParseString(s, name string) string {
	bad := func() string {
		fmt.Fprintf(os.Stderr, "argument %s: %s is not a String literal\n", name, s)
		os.Exit(2)
		return ""
	}
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' || !utf8.ValidString(s) {
		return bad()
	}
	body := s[1 : len(s)-1]
	var b strings.Builder
	for i := 0; i < len(body); i++ {
		c := body[i]
		if c == '"' {
			return bad()
		}
		if c != '\\' {
			b.WriteByte(c)
			continue
		}
		if i++; i == len(body) {
			return bad()
		}
		switch body[i] {
		case 'b':
			b.WriteByte('\b')
		case 't':
			b.WriteByte('\t')
		case 'n':
			b.WriteByte('\n')
		case 'f':
			b.WriteByte('\f')
		case 'r':
			b.WriteByte('\r')
		case '"', '\'', '\\':
			b.WriteByte(body[i])
		default:
			return bad()
		}
	}
	return b.String()
}

func sysmlFormat(v any) string {
	if t, ok := v.(string); ok {
		return strconv.Quote(t)
	}
	if l, ok := v.(sysmlEnum); ok {
		return sysmlLiterals[l]
	}
	if f, ok := v.(sysmlFn); ok {
		return sysmlFnNames[f.c]
	}
	if n, ok := v.(sysmlNum); ok {
		if n.real {
			v = n.r
		} else {
			v = n.i
		}
	}
	if i, ok := v.(sysmlInt); ok {
		return i.String()
	}
	if r, ok := v.(float64); ok {
		format := byte('f')
		if abs := math.Abs(r); r != 0 && (abs < 1e-4 || abs >= 1e21) {
			format = 'g'
		}
		text := strconv.FormatFloat(r, format, -1, 64)
		if !strings.ContainsAny(text, ".eEnN") {
			text += ".0"
		}
		return text
	}
	return fmt.Sprint(v)
}

// sysmlRun invokes fn, converting a failed check into an error.
func sysmlRun(fn func()) (err error) {
	sysmlDepth, sysmlSteps = 0, 0
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(sysmlError); ok {
				err = errors.New(e.msg)
				return
			}
			panic(r)
		}
	}()
	fn()
	return nil
}
`

// EmitGo writes program as a self-contained Go main package whose command line
// and output match the C program's.
func EmitGo(w io.Writer, p *Program) error {
	e := &goEmitter{w: w, collections: p.Collections}
	e.raw(goPrelude)
	e.raw(goEnumPrelude)
	e.raw(goEnumTables(p))
	e.raw(goFnPrelude)
	e.raw(goFnTables(p))
	e.raw(fmt.Sprintf("const sysmlMaxCalcDepth = %d\n\nconst sysmlDefaultMaxSteps = %d\n\n", runtime.DefaultMaxCalcDepth, runtime.DefaultMaxSteps))
	e.raw(fmt.Sprintf("const sysmlDefaultMaxIntegerBits = %d\n\nconst sysmlMinMaxIntegerBits = %d\n\n", runtime.DefaultMaxIntegerBits, runtime.MinMaxIntegerBits))
	if p.Collections {
		e.raw(fmt.Sprintf("const sysmlDefaultMaxElements = %d\n", runtime.DefaultMaxElements))
		e.raw(goSeqPrelude)
	}
	for _, fn := range p.Funcs {
		e.function(fn)
	}
	e.main(p.Entry)
	e.literals()
	return e.err
}

type goEmitter struct {
	w      io.Writer
	err    error
	indent int
	// resultRange is checked on each return of the function being emitted.
	resultRange Range
	// collections brackets every statement with the element budget's release.
	collections bool
	temps       int
	// bigLits are the Integer literals beyond int64, each parsed once into a
	// package variable.
	bigLits []string
}

func (e *goEmitter) raw(s string) {
	if e.err != nil {
		return
	}
	_, e.err = io.WriteString(e.w, s)
}

func (e *goEmitter) linef(format string, args ...any) {
	e.raw(strings.Repeat("\t", e.indent) + fmt.Sprintf(format, args...) + "\n")
}

func goType(t Type) string {
	if t.IsEnum() || t.IsFn() {
		if t.Many() {
			return goSeqType(t)
		}
		if t.IsFn() {
			return "sysmlFn"
		}
		return "sysmlEnum"
	}
	if t == TypeRun {
		return "int64"
	}
	switch t {
	case TypeInt:
		return "sysmlInt"
	case TypeReal:
		return "float64"
	case TypeBool:
		return "bool"
	case TypeNum:
		return "sysmlNum"
	case TypeString:
		return "string"
	case TypeSeqInt, TypeSeqReal, TypeSeqBool, TypeSeqNum, TypeSeqString:
		return goSeqType(t)
	}
	return "struct{}"
}

func goLocal(name string) string { return cLocal(name) }

// goNarrowed checks v against the range of the feature it is written to.
func goNarrowed(v string, r Range) string {
	if r == RangeAny {
		return v
	}
	return fmt.Sprintf("sysmlAtLeast(%s, %d, %q)", v, r.Lower(), r.String())
}

func goParams(fn *Func) string {
	parts := make([]string, len(fn.Params))
	for i, p := range fn.Params {
		parts[i] = goLocal(p.Name) + " " + goType(p.Type)
	}
	return strings.Join(parts, ", ")
}

func (e *goEmitter) function(fn *Func) {
	e.raw("\n")
	e.linef("func %s(%s) %s {", fn.Ident, goParams(fn), goType(fn.Result))
	e.indent++
	e.linef("sysmlEnter()")
	e.linef("defer sysmlLeave()")
	if fn.Run != "" {
		e.linef("%s := sysmlNextRun()", goLocal(fn.Run))
		e.linef("_ = %s", goLocal(fn.Run))
	}
	for _, p := range fn.Params {
		switch {
		case p.Type.Many():
			if p.Mult != MultAny || p.Range != RangeAny || p.Unique {
				v := e.checked(Checked{X: Var{Name: p.Name, T: p.Type}, M: p.Mult, R: p.Range, Unique: p.Unique, Where: paramWhere(p.Name)})
				e.linef(goAssign, goLocal(p.Name), v)
			}
		case p.Range != RangeAny:
			e.linef(goAssign, goLocal(p.Name), goNarrowed(goLocal(p.Name), p.Range))
		}
	}
	e.resultRange = fn.ResultRange
	e.block(fn.Body)
	e.linef("sysmlFail(%s)", strconv.Quote("calc "+fn.Name+" completed without returning a value"))
	e.linef("panic(\"unreachable\")")
	e.indent--
	e.linef("}")
}

// block emits statements; with collections, the elements a statement
// materializes are released when it ends, as the interpreter's step does.
func (e *goEmitter) block(stmts []Stmt) {
	releases := false
	for _, s := range stmts {
		if _, ok := s.(Return); !ok {
			releases = true
		}
	}
	if !e.collections || !releases {
		for _, s := range stmts {
			e.stmt(s)
		}
		return
	}
	e.temps++
	held := fmt.Sprintf("sysmlH%d", e.temps)
	e.linef("%s := sysmlElements", held)
	for _, s := range stmts {
		e.stmt(s)
		if _, ok := s.(Return); !ok {
			e.linef("sysmlElements = %s", held)
		}
	}
}

func (e *goEmitter) stmt(s Stmt) {
	switch s := s.(type) {
	case Declare:
		e.linef("var %s %s = %s", goLocal(s.Name), goType(s.T), goNarrowed(e.declInit(s), s.Range))
		e.linef("_ = %s", goLocal(s.Name))
	case Assign:
		e.linef(goAssign, goLocal(s.Name), goNarrowed(e.expr(s.Value), s.Range))
	case If:
		e.linef("if %s {", e.expr(s.Cond))
		e.indent++
		e.block(s.Then)
		e.indent--
		if len(s.Else) > 0 {
			e.linef("} else {")
			e.indent++
			e.block(s.Else)
			e.indent--
		}
		e.linef("}")
	case While:
		e.linef("for sysmlAfter(sysmlStep(1), %s) {", e.expr(s.Cond))
		e.indent++
		e.block(s.Body)
		if s.Until != nil {
			e.linef("if %s {", e.expr(s.Until))
			e.linef("\tbreak")
			e.linef("}")
		}
		e.indent--
		e.linef("}")
	case ForEach:
		e.forEach(s)
	case Sample:
		e.linef("%s", e.sample(s))
	case Return:
		e.linef("return %s", goNarrowed(e.expr(s.Value), e.resultRange))
	default:
		e.err = fmt.Errorf("codegen: Go emitter has no case for %T", s)
	}
}

func (e *goEmitter) expr(x Expr) string {
	switch x := x.(type) {
	case IntLit:
		if x.Big != nil {
			e.bigLits = append(e.bigLits, x.Big.String())
			return fmt.Sprintf("sysmlLit%d", len(e.bigLits)-1)
		}
		if x.Value == math.MinInt64 {
			return "sysmlI(math.MinInt64)"
		}
		return fmt.Sprintf("sysmlI(%d)", x.Value)
	case RealLit:
		return "float64(" + cReal(x.Value) + ")"
	case BoolLit:
		return strconv.FormatBool(x.Value)
	case StrLit:
		return strconv.Quote(x.Value)
	case EnumLit:
		return fmt.Sprintf("sysmlEnum(%d)", x.T.Enum.Base+x.I)
	case EnumText:
		return "sysmlLiterals[" + e.expr(x.X) + "]"
	case Refusal:
		return e.refusal(x)
	case Var:
		return goLocal(x.Name)
	case ToReal:
		if x.X.Type().Many() {
			return "sysmlWiden(" + e.expr(x.X) + ")"
		}
		if x.X.Type() == TypeNum {
			return e.expr(x.X) + ".toReal()"
		}
		if x.Exact {
			return "sysmlToRealExact(" + e.expr(x.X) + ")"
		}
		return "sysmlToReal(" + e.expr(x.X) + ")"
	case ToNum:
		switch x.X.Type() {
		case TypeInt:
			return "sysmlNI(" + e.expr(x.X) + ")"
		case TypeReal:
			return "sysmlNR(" + e.expr(x.X) + ")"
		}
		return "sysmlNums(" + e.expr(x.X) + ")"
	case AsInt:
		return e.expr(x.X) + ".i"
	case NumSplit:
		ints := make([]string, len(x.Nums))
		for i, v := range x.Nums {
			ints[i] = "!" + goLocal(v.Name) + ".real"
		}
		return fmt.Sprintf("func() %s { if %s { return %s }; return %s }()", goType(x.T), strings.Join(ints, " && "), e.expr(x.Int), e.expr(x.Real))
	case Unary:
		operand := e.expr(x.X)
		switch x.Op {
		case ast.OpNot:
			return "(!" + operand + ")"
		case ast.OpPos:
			return operand
		case ast.OpNeg:
			if x.T == TypeInt {
				return "sysmlNeg(" + operand + ")"
			}
			if x.Exact {
				return "(0 - " + operand + ")"
			}
			return "(-" + operand + ")"
		}
	case Binary:
		return e.binary(x)
	case Cond:
		return fmt.Sprintf("func() %s { if %s { return %s }; return %s }()", goType(x.T), e.expr(x.C), e.expr(x.Then), e.expr(x.Else))
	case Call:
		return e.call(x.Args, len(x.Fn.Params), goType(x.Fn.Result), func(operands []string) string {
			return fmt.Sprintf("%s(%s)", x.Fn.Ident, strings.Join(operands, ", "))
		})
	case LibCall:
		return e.call(x.Args, len(x.Op.Operands()), goType(x.Op.Result()), x.Op.goExpr)
	case Steps:
		return fmt.Sprintf("sysmlAfter(sysmlStep(%d), %s)", x.N, e.expr(x.X))
	}
	if s, ok := e.seqExpr(x); ok {
		return s
	}
	if s, ok := e.fnExpr(x); ok {
		return s
	}
	e.err = fmt.Errorf("codegen: Go emitter has no case for %T", x)
	return "0"
}

// call emits a call; Go evaluates operands left to right, so only arguments
// written out of parameter order need temporaries to keep source order.
func (e *goEmitter) call(args []Arg, nParams int, result string, apply func(operands []string) string) string {
	names := make([]string, len(args))
	for i, a := range args {
		names[i] = e.expr(a.Value)
	}
	if inParamOrder(args, nParams) {
		return apply(names)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "func() %s { ", result)
	for i := range args {
		fmt.Fprintf(&b, "t%d := %s; _ = t%d; ", i, names[i], i)
		names[i] = fmt.Sprintf("t%d", i)
	}
	fmt.Fprintf(&b, "return %s }()", apply(callOperands(args, nParams, names)))
	return b.String()
}

func (e *goEmitter) binary(x Binary) string {
	if c, ok := e.mixedComparison(x); ok {
		return c
	}
	l, r := e.expr(x.L), e.expr(x.R)
	if x.L.Type().IsFn() {
		if x.Op == ast.OpNeq {
			return fmt.Sprintf("(!sysmlFnEq(%s, %s))", l, r)
		}
		return fmt.Sprintf("sysmlFnEq(%s, %s)", l, r)
	}
	if x.L.Type() == TypeNum {
		switch x.Op {
		case ast.OpEqEqEq:
			return fmt.Sprintf("sysmlNSame(%s, %s)", l, r)
		case ast.OpNeqEqEq:
			return fmt.Sprintf("(!sysmlNSame(%s, %s))", l, r)
		}
		return fmt.Sprintf("(sysmlNCmp(%s, %s) %s 0)", l, r, cOperator(x.Op))
	}
	if x.L.Type() == TypeString {
		return fmt.Sprintf("(%s %s %s)", l, cOperator(x.Op), r)
	}
	ints := x.L.Type() == TypeInt
	switch x.Op {
	case ast.OpAdd, ast.OpSub:
		if ints {
			return fmt.Sprintf("sysml%s(%s, %s)", map[ast.OperatorKind]string{ast.OpAdd: "Add", ast.OpSub: "Sub"}[x.Op], l, r)
		}
		return goUnsignedZero(x, fmt.Sprintf("sysmlFinite(%s %s %s)", l, cOperator(x.Op), r))
	case ast.OpMul:
		if ints {
			return fmt.Sprintf("sysmlMul(%s, %s)", l, r)
		}
		return goUnsignedZero(x, fmt.Sprintf("sysmlFinite(%s * %s)", l, r))
	case ast.OpDiv:
		if ints {
			return fmt.Sprintf("sysmlQuot(%s, %s)", l, r)
		}
		return goUnsignedZero(x, fmt.Sprintf("sysmlRDiv(%s, %s)", l, r))
	case ast.OpMod:
		if ints {
			return fmt.Sprintf("sysmlMod(%s, %s)", l, r)
		}
		return goUnsignedZero(x, fmt.Sprintf("sysmlRMod(%s, %s)", l, r))
	case ast.OpPow:
		if x.T == TypeInt {
			return fmt.Sprintf("sysmlIPow(%s, %s)", l, r)
		}
		return fmt.Sprintf("sysmlRPow(%s, %s)", l, r)
	case ast.OpLt, ast.OpLe, ast.OpGt, ast.OpGe, ast.OpEq, ast.OpNeq:
		if ints {
			return fmt.Sprintf("(sysmlICmp(%s, %s) %s 0)", l, r, cOperator(x.Op))
		}
		return fmt.Sprintf("(%s %s %s)", l, cOperator(x.Op), r)
	case ast.OpAnd, ast.OpConditionalAnd:
		return fmt.Sprintf("(%s && %s)", l, r)
	case ast.OpOr, ast.OpConditionalOr:
		return fmt.Sprintf("(%s || %s)", l, r)
	case ast.OpXor:
		return fmt.Sprintf("(%s != %s)", l, r)
	case ast.OpImplies:
		return fmt.Sprintf("(!%s || %s)", l, r)
	}
	e.err = fmt.Errorf("codegen: Go emitter has no binary case for %s", x.Op)
	return "0"
}

// goUnsignedZero gives an exact Rational operation's zero result no sign.
func goUnsignedZero(x Binary, r string) string {
	if x.Whole && x.Guard {
		return "sysmlWhole(" + r + ")"
	}
	if x.Exact {
		return "sysmlUnsignedZero(" + r + ")"
	}
	return r
}

// mixedComparison orders an Integer against a Real exactly, as the
// interpreter does, rather than comparing the Integer's binary64 rounding.
func (e *goEmitter) mixedComparison(x Binary) (string, bool) {
	if !isComparison(x.Op) {
		return "", false
	}
	if q, ok := exactQuotient(x.L); ok {
		if i, ok := widenedInt(x.R); ok {
			return fmt.Sprintf("(sysmlCmpQI(sysmlRatQuot(%s, %s), %s) %s 0)", e.expr(q.L), e.expr(q.R), e.expr(i), cOperator(x.Op)), true
		}
		return fmt.Sprintf("(sysmlCmpQR(sysmlRatQuot(%s, %s), %s) %s 0)", e.expr(q.L), e.expr(q.R), e.expr(x.R), cOperator(x.Op)), true
	}
	if q, ok := exactQuotient(x.R); ok {
		if i, ok := widenedInt(x.L); ok {
			return fmt.Sprintf("func() bool { l := %s; return -sysmlCmpQI(sysmlRatQuot(%s, %s), l) %s 0 }()", e.expr(i), e.expr(q.L), e.expr(q.R), cOperator(x.Op)), true
		}
		return fmt.Sprintf("func() bool { l := %s; return -sysmlCmpQR(sysmlRatQuot(%s, %s), l) %s 0 }()", e.expr(x.L), e.expr(q.L), e.expr(q.R), cOperator(x.Op)), true
	}
	if i, ok := widenedInt(x.L); ok && !isWidenedInt(x.R) {
		return fmt.Sprintf("(sysmlCmpIR(%s, %s) %s 0)", e.expr(i), e.expr(x.R), cOperator(x.Op)), true
	}
	if i, ok := widenedInt(x.R); ok && !isWidenedInt(x.L) {
		return fmt.Sprintf("func() bool { l := %s; return -sysmlCmpIR(%s, l) %s 0 }()", e.expr(x.L), e.expr(i), cOperator(x.Op)), true
	}
	return "", false
}

func (e *goEmitter) main(fn *Func) {
	e.raw("\n")
	e.linef("func main() {")
	e.indent++
	e.linef("args := os.Args[1:]")
	e.linef("repeat, badRepeat := 1, false")
	e.linef("if len(args) >= 2 && args[0] == \"--repeat\" {")
	e.linef("\tn, err := strconv.Atoi(args[1])")
	e.linef("\trepeat, badRepeat = n, err != nil || n < 1")
	e.linef("\targs = args[2:]")
	e.linef("}")
	e.linef("if badRepeat || len(args) != %d {", len(fn.Params))
	e.linef("\tfmt.Fprintf(os.Stderr, \"usage: %%s [--repeat N]%s\\n\", os.Args[0])", cUsage(fn))
	e.linef("\tos.Exit(2)")
	e.linef("}")
	e.linef("sysmlReadMaxIntegerBits()")
	e.linef("sysmlMaxSteps = sysmlReadBudget(\"OPENSYSML_MAX_STEPS\", \"evaluation steps\", sysmlDefaultMaxSteps)")
	if e.collections {
		e.linef("sysmlMaxElements = sysmlReadBudget(\"OPENSYSML_MAX_ELEMENTS\", \"collection elements\", sysmlDefaultMaxElements)")
	}
	args := make([]string, len(fn.Params))
	for i, p := range fn.Params {
		parser := map[Type]string{TypeInt: "sysmlParseInt", TypeReal: "sysmlParseReal", TypeBool: "sysmlParseBool", TypeNum: "sysmlParseNum", TypeString: "sysmlParseString"}[p.Type.Elem()]
		if p.Type.IsEnum() {
			parser = goEnumParser(p.Type)
		}
		if p.Type.Many() {
			parser = fmt.Sprintf("sysmlParseSeq[%s](args[%d], %q, %s)", goElem(p.Type), i, p.Name, parser)
		} else {
			parser = fmt.Sprintf("%s(args[%d], %q)", parser, i, p.Name)
		}
		e.linef("%s := %s", goLocal(p.Name), parser)
		args[i] = goLocal(p.Name)
	}
	e.linef("var result %s", goType(fn.Result))
	reset := ""
	if e.collections {
		// A run holds its sequence arguments throughout, as the interpreter
		// holds the sequence literals it evaluated them from.
		reset = "sysmlElements = 0; "
		for i, p := range fn.Params {
			if p.Type.Many() {
				reset += fmt.Sprintf("if %s.shape == sysmlMany { sysmlCharge(int64(len(%s.data))) }; ", args[i], args[i])
			}
		}
	}
	e.linef("for i := 0; i < repeat; i++ {")
	e.linef("\tif err := sysmlRun(func() { %sresult = %s(%s) }); err != nil {", reset, fn.Ident, strings.Join(args, ", "))
	e.linef("\t\tfmt.Fprintf(os.Stderr, \"%s: %%s\\n\", err)", fn.Name)
	e.linef("\t\tos.Exit(1)")
	e.linef("\t}")
	e.linef("}")
	if fn.Result.Many() {
		e.linef("fmt.Println(sysmlFormatSeq(result))")
	} else {
		e.linef("fmt.Println(sysmlFormat(result))")
	}
	e.indent--
	e.linef("}")
}

// literals declares the Integer literals beyond int64 the program reads.
func (e *goEmitter) literals() {
	for i, digits := range e.bigLits {
		e.linef("\nvar sysmlLit%d = sysmlBigLit(%q)", i, digits)
	}
}
