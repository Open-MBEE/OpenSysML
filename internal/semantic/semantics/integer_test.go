package semantics

import (
	"errors"
	"math"
	"math/big"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func TestIntegerPromotionAndDemotion(t *testing.T) {
	sum, err := IntArith(ast.OpAdd, intVal(math.MaxInt64), intVal(1), DefaultMaxIntegerBits)
	if err != nil || !sum.IsBigInt() || sum.FormatInt() != "9223372036854775808" {
		t.Fatalf("MaxInt64 + 1 = %s (big %v), %v", sum.FormatInt(), sum.IsBigInt(), err)
	}
	back, err := IntArith(ast.OpSub, sum, intVal(1), DefaultMaxIntegerBits)
	if err != nil || back.IsBigInt() || back != intVal(math.MaxInt64) {
		t.Fatalf("(MaxInt64 + 1) - 1 = %+v, %v; want the int64 MaxInt64", back, err)
	}
	if neg := IntNeg(intVal(math.MinInt64)); neg.FormatInt() != "9223372036854775808" {
		t.Errorf("-(MinInt64) = %s", neg.FormatInt())
	}
	if neg := IntNeg(IntNeg(intVal(math.MinInt64))); neg != intVal(math.MinInt64) {
		t.Errorf("-(-(MinInt64)) = %+v, want the int64 MinInt64", neg)
	}
	if q, _ := IntDivTrunc(intVal(math.MinInt64), intVal(-1)); q.FormatInt() != "9223372036854775808" {
		t.Errorf("MinInt64 div -1 = %s", q.FormatInt())
	}
	if v := BigIntValue(big.NewInt(42)); v.IsBigInt() || v != intVal(42) {
		t.Errorf("BigIntValue(42) = %+v, want the int64 42", v)
	}
}

func TestIntegerEqualityAcrossRepresentations(t *testing.T) {
	a := bigVal("1180591620717411303424")
	b, err := IntPow(intVal(2), intVal(70), DefaultMaxIntegerBits)
	if err != nil {
		t.Fatal(err)
	}
	if !a.Equal(b) || CompareInt(a, b) != 0 {
		t.Errorf("2**70 parsed and computed differ")
	}
	if eq, ok := EvalBinary(ast.OpEq, a, b); !ok || !eq.Bool {
		t.Errorf("2**70 == 2**70 = %+v", eq)
	}
	c := bigVal("1180591620717411303425")
	if eq, _ := EvalBinary(ast.OpEq, a, c); eq.Bool {
		t.Errorf("2**70 == 2**70 + 1 holds; neighbours beyond 2^53 must stay distinct")
	}
	if lt, _ := EvalBinary(ast.OpLt, a, c); !lt.Bool {
		t.Errorf("2**70 < 2**70 + 1 fails")
	}
	if eq, _ := EvalBinary(ast.OpEq, intVal(9007199254740993), intVal(9007199254740992)); eq.Bool {
		t.Errorf("2^53 + 1 == 2^53 holds between Integers")
	}
	if CompareInt(bigVal("-1180591620717411303424"), intVal(math.MinInt64)) != -1 {
		t.Errorf("-2**70 is not below MinInt64")
	}
}

func TestIntegerFormatAndParse(t *testing.T) {
	for _, text := range []string{"0", "-9223372036854775808", "9223372036854775808", "-123456789012345678901234567890"} {
		v, ok := ParseInteger(text)
		if !ok || v.FormatInt() != text || FormatConst(v) != text {
			t.Errorf("ParseInteger(%q) = %+v, %v; formats as %q", text, v, ok, v.FormatInt())
		}
	}
	for _, text := range []string{"", "-", "1_000", "0x10", "1.0", "1e3"} {
		if v, ok := ParseInteger(text); ok {
			t.Errorf("ParseInteger(%q) = %+v, want rejected", text, v)
		}
	}
	v, ok := evalExpr(t, "123456789012345678901234567890")
	if !ok || v.FormatInt() != "123456789012345678901234567890" {
		t.Errorf("big literal folds to %+v ok=%v", v, ok)
	}
}

func TestIntegerToReal(t *testing.T) {
	if got := bigVal("9223372036854775809").AsReal(); got != 9223372036854775808.0 {
		t.Errorf("2^63+1 as Real = %v", got)
	}
	// 2^1024 is beyond the float64 range: the nearest is the infinity of its sign.
	huge, _ := IntPow(intVal(-2), intVal(1025), DefaultMaxIntegerBits)
	if got := huge.AsReal(); !math.IsInf(got, -1) {
		t.Errorf("-2^1025 as Real = %v, want -Inf", got)
	}
	if v, ok := IntegerOfReal(1e30); !ok || v.FormatInt() != "1000000000000000019884624838656" {
		t.Errorf("IntegerOfReal(1e30) = %s, %v", v.FormatInt(), ok)
	}
}

func TestIntegerQuotientAndModulo(t *testing.T) {
	p70, _ := IntPow(intVal(2), intVal(70), DefaultMaxIntegerBits)
	p69, _ := IntPow(intVal(2), intVal(69), DefaultMaxIntegerBits)
	if q, err := RatArith(ast.OpDiv, p70, p69, DefaultMaxIntegerBits); err != nil || CompareRat(q, intVal(2)) != 0 || q.Kind != ValRational {
		t.Errorf("2**70 / 2**69 = %s, %v, want the Rational 2", q.FormatRational(), err)
	}
	if r, ok := IntRem(p70, intVal(7)); !ok || r != intVal(2) {
		t.Errorf("2**70 mod 7 = %+v", r)
	}
	if r, _ := IntRem(IntNeg(p70), intVal(7)); r != intVal(-2) {
		t.Errorf("-(2**70) mod 7 = %+v, want -2 (truncated, sign of the dividend)", r)
	}
	if _, ok := IntRem(p70, intVal(0)); ok {
		t.Errorf("2**70 mod 0 succeeded")
	}
}

func TestIntegerSizeBudget(t *testing.T) {
	_, err := IntArith(ast.OpMul, bigVal("1180591620717411303424"), bigVal("1180591620717411303424"), 100)
	if !errors.Is(err, ErrIntegerSizeLimit) {
		t.Errorf("2**70 * 2**70 under a 100-bit budget = %v, want %v", err, ErrIntegerSizeLimit)
	}
	if _, err := IntPow(intVal(3), intVal(1<<40), DefaultMaxIntegerBits); !errors.Is(err, ErrIntegerSizeLimit) {
		t.Errorf("3 ** 2^40 = %v, want %v", err, ErrIntegerSizeLimit)
	}
	if v, err := IntPow(intVal(2), intVal(DefaultMaxIntegerBits-1), DefaultMaxIntegerBits); err != nil || v.IntBitLen() != DefaultMaxIntegerBits {
		t.Errorf("2 ** (budget-1) = %d bits, %v", v.IntBitLen(), err)
	}
}
