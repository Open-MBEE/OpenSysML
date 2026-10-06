package semantics

import (
	"errors"
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func frac(t *testing.T, num, den int64) Value {
	t.Helper()
	v, ok := FracValue(num, den)
	if !ok {
		t.Fatalf("FracValue(%d, %d) refused", num, den)
	}
	return v
}

func parsed(t *testing.T, text string) Value {
	t.Helper()
	v, err := ParseRational(text, DefaultMaxIntegerBits)
	if err != nil {
		t.Fatalf("ParseRational(%q): %v", text, err)
	}
	return v
}

func TestRationalNormalization(t *testing.T) {
	if v := frac(t, 2, -4); v.Kind != ValRational || v.FormatRational() != "-0.5" || v.RatDenom() != intVal(2) || v.RatNumer() != intVal(-1) {
		t.Errorf("2/-4 = %s over %s", v.RatNumer().FormatInt(), v.RatDenom().FormatInt())
	}
	if _, ok := FracValue(1, 0); ok {
		t.Error("1/0 accepted")
	}
	// One Rational, one representation: a big.Rat whose terms fit is held inline.
	wide := RatValue(new(big.Rat).SetFrac64(1, 1<<33))
	inline := RatValue(new(big.Rat).SetFrac64(6, 9))
	if inline != frac(t, 2, 3) {
		t.Errorf("RatValue(6/9) = %+v, want the inline 2/3", inline)
	}
	if CompareRat(wide, frac(t, 1, 1<<33)) != 0 || wide.FormatRational() != "1.16415321826934814453125e-10" {
		t.Errorf("1/2^33 = %s", wide.FormatRational())
	}
	if v := frac(t, math.MinInt64, 2); v.RatNumer().FormatInt() != "-4611686018427387904" || v.RatDenom() != intVal(1) {
		t.Errorf("MinInt64/2 = %s", v.FormatRational())
	}
	if v := frac(t, 1, math.MinInt64); v.RatNumer() != intVal(-1) || v.RatDenom().FormatInt() != "9223372036854775808" {
		t.Errorf("1/MinInt64 = %s", v.FormatRational())
	}
	if v := RatOf(intVal(6)); v.Kind != ValRational || !v.RatIsWhole() || CompareRat(v, intVal(6)) != 0 {
		t.Errorf("RatOf(6) = %+v", v)
	}
}

func TestParseRationalIsExact(t *testing.T) {
	for _, c := range []struct{ text, want string }{
		{"0.1", "0.1"}, {".5", "0.5"}, {"1.5e3", "1500.0"}, {"25E-3", "0.025"}, {"-0.0", "0.0"},
		{"+2.50", "2.5"}, {"1.0e400", "1e+400"}, {"3e-7", "3e-07"}, {"123456789012345678901234567890.5", "1.234567890123456789012345678905e+29"}, {"12345678901234567890.25", "12345678901234567890.25"},
	} {
		if got := parsed(t, c.text); got.Kind != ValRational || got.FormatRational() != c.want {
			t.Errorf("ParseRational(%q) = %s, want %s", c.text, got.FormatRational(), c.want)
		}
	}
	if v := parsed(t, "0.1"); v.RatNumer() != intVal(1) || v.RatDenom() != intVal(10) {
		t.Errorf("0.1 = %s/%s, want 1/10", v.RatNumer().FormatInt(), v.RatDenom().FormatInt())
	}
	for _, text := range []string{"1e100000", "1e-100000", "1e9223372036854775807", strings.Repeat("7", 100) + ".1"} {
		if _, err := ParseRational(text, 128); !errors.Is(err, ErrRationalSizeLimit) {
			t.Errorf("ParseRational(%.20q, 128) = %v, want ErrRationalSizeLimit", text, err)
		}
	}
	for _, text := range []string{"0.123456789012345678", "-123456789.123456789", "0.123456789012345678e0"} {
		if _, err := ParseRational(text, 64); !errors.Is(err, ErrRationalSizeLimit) {
			t.Errorf("ParseRational(%q, 64) = %v, want ErrRationalSizeLimit", text, err)
		}
	}
	for _, text := range []string{"", "abc", "1.2.3", "NaN", "Inf", "0x1p3", "1e"} {
		if _, err := ParseRational(text, DefaultMaxIntegerBits); !errors.Is(err, ErrRealNotation) {
			t.Errorf("ParseRational(%q) = %v, want ErrRealNotation", text, err)
		}
	}
	if v, err := ParseRationalText("2/6", DefaultMaxIntegerBits); err != nil || CompareRat(v, frac(t, 1, 3)) != 0 {
		t.Errorf("ParseRationalText(2/6) = %s, %v", v.FormatRational(), err)
	}
	if _, err := ParseRationalText("1/0", DefaultMaxIntegerBits); !errors.Is(err, ErrDivisionByZero) {
		t.Errorf("ParseRationalText(1/0) = %v", err)
	}
	if _, err := ParseRationalText("1/x", DefaultMaxIntegerBits); !errors.Is(err, ErrRealNotation) {
		t.Errorf("ParseRationalText(1/x) = %v", err)
	}
}

func TestRationalArithmeticIsExact(t *testing.T) {
	arith := func(op ast.OperatorKind, a, b Value) Value {
		t.Helper()
		v, err := RatArith(op, a, b, DefaultMaxIntegerBits)
		if err != nil {
			t.Fatalf("%s %s %s: %v", a.FormatRational(), op, b.FormatRational(), err)
		}
		return v
	}
	if sum := arith(ast.OpAdd, parsed(t, "0.1"), parsed(t, "0.2")); CompareRat(sum, parsed(t, "0.3")) != 0 {
		t.Errorf("0.1 + 0.2 = %s", sum.FormatRational())
	}
	third := arith(ast.OpDiv, intVal(1), intVal(3))
	if third.Kind != ValRational || third.FormatRational() != "1/3" {
		t.Errorf("1 / 3 = %s", third.FormatRational())
	}
	if whole := arith(ast.OpMul, third, intVal(3)); whole.Kind != ValRational || !whole.RatIsWhole() || CompareRat(whole, intVal(1)) != 0 {
		t.Errorf("1 / 3 * 3 = %+v, want the Rational 1", whole)
	}
	if rem := arith(ast.OpMod, frac(t, -7, 2), intVal(1)); CompareRat(rem, frac(t, -1, 2)) != 0 {
		t.Errorf("-7/2 %% 1 = %s, want -0.5 (truncated)", rem.FormatRational())
	}
	// Past int64 the terms move to big.Rat and stay exact.
	wide := arith(ast.OpAdd, frac(t, math.MaxInt64, 1), frac(t, 1, 2))
	if wide.FormatRational() != "9223372036854775807.5" {
		t.Errorf("MaxInt64 + 1/2 = %s", wide.FormatRational())
	}
	if back := arith(ast.OpSub, wide, frac(t, 1, 2)); back != frac(t, math.MaxInt64, 1) {
		t.Errorf("(MaxInt64 + 1/2) - 1/2 = %+v, want the inline MaxInt64", back)
	}
	if _, err := RatArith(ast.OpDiv, third, intVal(0), DefaultMaxIntegerBits); !errors.Is(err, ErrDivisionByZero) {
		t.Errorf("1/3 / 0 = %v", err)
	}
	if _, err := RatArith(ast.OpMul, frac(t, 1, 1<<40), frac(t, 1, 1<<40), 64); !errors.Is(err, ErrRationalSizeLimit) {
		t.Errorf("2^-40 * 2^-40 under a 64-bit budget = %v, want ErrRationalSizeLimit", err)
	}
	if n := RatNeg(third); n.FormatRational() != "-1/3" || RatAbs(n) != third {
		t.Errorf("-(1/3) = %s", n.FormatRational())
	}
}

func TestRationalPower(t *testing.T) {
	pow := func(base Value, n int64) (Value, error) { return RatPow(base, intVal(n), DefaultMaxIntegerBits) }
	for _, c := range []struct {
		base Value
		n    int64
		want string
	}{
		{frac(t, 2, 3), 3, "8/27"}, {frac(t, 2, 3), -2, "2.25"}, {frac(t, -1, 1), 3, "-1.0"}, {frac(t, -1, 1), 4, "1.0"},
		{frac(t, 5, 7), 0, "1.0"}, {frac(t, 0, 1), 5, "0.0"},
	} {
		if v, err := pow(c.base, c.n); err != nil || v.Kind != ValRational || v.FormatRational() != c.want {
			t.Errorf("(%s) ** %d = %s, %v, want %s", c.base.FormatRational(), c.n, v.FormatRational(), err, c.want)
		}
	}
	if _, err := pow(frac(t, 0, 1), -1); !errors.Is(err, ErrArithmeticDomain) {
		t.Errorf("0 ** -1 = %v", err)
	}
	// Refused from a lower bound, before 3^(2^62) is computed.
	if _, err := RatPow(frac(t, 2, 3), intVal(1<<62), 1024); !errors.Is(err, ErrRationalSizeLimit) {
		t.Errorf("(2/3) ** 2^62 = %v, want ErrRationalSizeLimit", err)
	}
}

func TestRationalComparisonAndRounding(t *testing.T) {
	tenth := parsed(t, "0.1")
	if CompareRat(frac(t, 1, 3), parsed(t, "0.3334")) >= 0 || CompareRat(frac(t, 2, 6), frac(t, 1, 3)) != 0 {
		t.Error("exact order")
	}
	if CompareRat(frac(t, math.MaxInt64, 3), frac(t, math.MaxInt64-1, 3)) <= 0 {
		t.Error("order of terms whose cross products overflow int64")
	}
	// A Rational meets a Real as its nearest binary64, ±Inf past the range; an Integer exactly.
	huge, _ := ParseRational("1e400", 4096)
	if CompareReal(tenth, 0.1) != 0 || CompareReal(frac(t, 1, 3), 1.0/3.0) != 0 || CompareReal(frac(t, 1, 2), 0.5) != 0 ||
		CompareReal(tenth, math.Inf(1)) != -1 || CompareReal(tenth, math.Inf(-1)) != 1 ||
		CompareReal(huge, math.MaxFloat64) != 1 || CompareReal(huge, math.Inf(1)) != 0 || CompareReal(RatNeg(huge), math.Inf(-1)) != 0 {
		t.Error("order of a Rational against a Real")
	}
	if CompareReal(intVal(1<<53+1), 1<<53) != 1 {
		t.Error("an Integer orders against a Real exactly")
	}
	for _, c := range []struct {
		v                      Value
		floor, round, truncate int64
	}{
		{frac(t, -7, 2), -4, -4, -3}, {frac(t, 5, 2), 2, 3, 2}, {frac(t, -5, 2), -3, -3, -2}, {frac(t, 1, 3), 0, 0, 0}, {frac(t, 4, 1), 4, 4, 4},
	} {
		if f, r, tr := RatFloor(c.v), RatRound(c.v), RatTrunc(c.v); f != intVal(c.floor) || r != intVal(c.round) || tr != intVal(c.truncate) {
			t.Errorf("floor/round/trunc(%s) = %s %s %s", c.v.FormatRational(), f.FormatInt(), r.FormatInt(), tr.FormatInt())
		}
	}
}

func TestRationalFormatting(t *testing.T) {
	for _, c := range []struct {
		v    Value
		want string
	}{
		{frac(t, 1, 3), "1/3"}, {frac(t, -25, 12), "-25/12"}, {frac(t, 3, 10), "0.3"}, {frac(t, 2, 1), "2.0"},
		{frac(t, 3, 200000), "1.5e-05"}, {frac(t, 1, 10000), "0.0001"}, {frac(t, 0, 7), "0.0"},
	} {
		if got := c.v.FormatRational(); got != c.want {
			t.Errorf("FormatRational(%d/%d) = %s, want %s", c.v.RatNumer().Int, c.v.RatDenom().Int, got, c.want)
		}
	}
	// A Rational binary64 holds prints as that Real prints.
	for _, f := range []float64{0.5, 1.5, 2.0, 1e21, 0x1p-16, 1800} {
		v, _ := RationalOfReal(f)
		if v.FormatRational() != FormatReal(f) {
			t.Errorf("FormatRational(%v) = %s, FormatReal = %s", f, v.FormatRational(), FormatReal(f))
		}
	}
}

func TestRationalBinaryBoundary(t *testing.T) {
	if f, ok := frac(t, 1, 2).BinaryExact(); !ok || f != 0.5 {
		t.Error("1/2 is a binary64")
	}
	if _, ok := parsed(t, "0.1").BinaryExact(); ok {
		t.Error("1/10 is no binary64")
	}
	if _, ok := parsed(t, "1e400").BinaryExact(); ok {
		t.Error("1e400 is no finite binary64")
	}
	if v, ok := RationalOfReal(0.1); !ok || v.RatNumer() != intVal(3602879701896397) || v.RatDenom() != intVal(36028797018963968) {
		t.Errorf("RationalOfReal(0.1) = %s", v.FormatRational())
	}
	if _, ok := RationalOfReal(math.NaN()); ok {
		t.Error("NaN has no Rational")
	}
	if v, ok := CanonicalRational("1", "3"); !ok || CompareRat(v, frac(t, 1, 3)) != 0 {
		t.Error("1/3 is canonical")
	}
	for _, c := range [][2]string{{"2", "6"}, {"1", "-3"}, {"1", "0"}, {"1", "2"}, {"3", "1"}, {"x", "3"}} {
		if _, ok := CanonicalRational(c[0], c[1]); ok {
			t.Errorf("CanonicalRational(%s, %s) accepted a noncanonical encoding", c[0], c[1])
		}
	}
}
