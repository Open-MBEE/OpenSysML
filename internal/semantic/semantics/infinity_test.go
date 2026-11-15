package semantics

import (
	"errors"
	"math"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestInfinityValues: `*` and `-*` are distinct infinities, only `*` is the
// unbounded multiplicity bound, and both survive packing and print as written.
func TestInfinityValues(t *testing.T) {
	pos, neg := PositiveInfinity(), NegativeInfinity()
	if !pos.IsInfinite() || !neg.IsInfinite() || pos.IsNumeric() || neg.IsNumeric() {
		t.Fatalf("infinities misclassified: %+v %+v", pos, neg)
	}
	if !pos.IsUnbounded() || neg.IsUnbounded() || pos.IsNegativeInfinity() || !neg.IsNegativeInfinity() {
		t.Errorf("sign misread: %+v %+v", pos, neg)
	}
	if pos.Sign() != 1 || neg.Sign() != -1 {
		t.Errorf("Sign() = %d, %d; want 1, -1", pos.Sign(), neg.Sign())
	}
	if got := FormatConst(pos); got != "*" {
		t.Errorf("FormatConst(*) = %q", got)
	}
	if got := FormatConst(neg); got != "-*" {
		t.Errorf("FormatConst(-*) = %q", got)
	}
	if got := ExtendedReal(math.Inf(1)); !got.IsUnbounded() {
		t.Errorf("ExtendedReal(+Inf) = %+v, want *", got)
	}
	if got := ExtendedReal(math.Inf(-1)); !got.IsNegativeInfinity() {
		t.Errorf("ExtendedReal(-Inf) = %+v, want -*", got)
	}
	if got := ExtendedReal(2.5); got.Kind != ValReal || got.Real != 2.5 {
		t.Errorf("ExtendedReal(2.5) = %+v", got)
	}
}

// TestInfinityOrder orders `-*` below and `*` above every finite number.
func TestInfinityOrder(t *testing.T) {
	pos, neg := PositiveInfinity(), NegativeInfinity()
	big := Value{Kind: ValReal, Real: math.MaxFloat64}
	cases := []struct {
		l, r Value
		want int
	}{
		{pos, pos, 0}, {neg, neg, 0}, {neg, pos, -1}, {pos, neg, 1},
		{pos, big, 1}, {big, pos, -1}, {neg, IntValue(math.MinInt64), -1}, {IntValue(0), neg, 1},
	}
	for _, tc := range cases {
		got, ok := InfinityOrder(tc.l, tc.r)
		if !ok || got != tc.want {
			t.Errorf("InfinityOrder(%s, %s) = %d, %v; want %d", FormatConst(tc.l), FormatConst(tc.r), got, ok, tc.want)
		}
	}
	if _, ok := InfinityOrder(pos, Value{Kind: ValBool}); ok {
		t.Error("InfinityOrder(*, false) ordered a Boolean")
	}
}

// TestInfinityArith answers the limits of the affinely extended reals and
// reports each indeterminate form.
func TestInfinityArith(t *testing.T) {
	pos, neg := PositiveInfinity(), NegativeInfinity()
	real := func(f float64) Value { return Value{Kind: ValReal, Real: f} }
	cases := []struct {
		op   ast.OperatorKind
		l, r Value
		want string
	}{
		{ast.OpAdd, pos, IntValue(1), "*"}, {ast.OpAdd, real(-1e308), neg, "-*"}, {ast.OpAdd, pos, pos, "*"},
		{ast.OpSub, IntValue(1), pos, "-*"}, {ast.OpSub, neg, pos, "-*"}, {ast.OpSub, pos, neg, "*"},
		{ast.OpMul, IntValue(-2), pos, "-*"}, {ast.OpMul, neg, neg, "*"}, {ast.OpMul, real(0.5), neg, "-*"},
		{ast.OpDiv, pos, IntValue(-2), "-*"}, {ast.OpDiv, IntValue(2), neg, "0"}, {ast.OpDiv, real(2), pos, "0.0"},
		{ast.OpDiv, Value{Kind: ValRational, Int: 5, den: 2}, neg, "0.0"},
		{ast.OpPow, pos, IntValue(2), "*"}, {ast.OpPow, neg, IntValue(2), "*"}, {ast.OpPow, neg, IntValue(3), "-*"},
		{ast.OpPow, neg, real(3), "-*"}, {ast.OpPow, pos, IntValue(-1), "0"}, {ast.OpPow, pos, real(0.5), "*"},
		{ast.OpPow, IntValue(2), pos, "*"}, {ast.OpPow, IntValue(2), neg, "0"}, {ast.OpPow, real(0.5), pos, "0.0"},
		{ast.OpPow, real(0.5), neg, "*"}, {ast.OpPow, IntValue(0), pos, "0"}, {ast.OpPow, pos, pos, "*"}, {ast.OpPow, pos, neg, "0"},
	}
	for _, tc := range cases {
		got, err := InfinityArith(tc.op, tc.l, tc.r)
		if err != nil {
			t.Errorf("%s %s %s failed: %v", FormatConst(tc.l), tc.op, FormatConst(tc.r), err)
			continue
		}
		if text := FormatConst(got); text != tc.want {
			t.Errorf("%s %s %s = %s, want %s", FormatConst(tc.l), tc.op, FormatConst(tc.r), text, tc.want)
		}
	}
	undefined := []struct {
		op   ast.OperatorKind
		l, r Value
		want error
	}{
		{ast.OpAdd, pos, neg, ErrArithmeticDomain}, {ast.OpSub, pos, pos, ErrArithmeticDomain},
		{ast.OpMul, IntValue(0), pos, ErrArithmeticDomain}, {ast.OpMul, neg, real(0), ErrArithmeticDomain},
		{ast.OpDiv, pos, neg, ErrArithmeticDomain}, {ast.OpDiv, pos, IntValue(0), ErrDivisionByZero},
		{ast.OpMod, pos, IntValue(2), ErrArithmeticDomain}, {ast.OpMod, IntValue(2), neg, ErrArithmeticDomain},
		{ast.OpPow, pos, IntValue(0), ErrArithmeticDomain}, {ast.OpPow, IntValue(1), pos, ErrArithmeticDomain},
		{ast.OpPow, IntValue(-1), neg, ErrArithmeticDomain}, {ast.OpPow, IntValue(-2), pos, ErrArithmeticDomain},
		{ast.OpPow, IntValue(0), neg, ErrArithmeticDomain}, {ast.OpPow, neg, real(0.5), ErrArithmeticDomain},
		{ast.OpPow, neg, pos, ErrArithmeticDomain},
		{ast.OpAdd, pos, Value{Kind: ValBool}, ErrArithmeticDomain},
	}
	for _, tc := range undefined {
		if got, err := InfinityArith(tc.op, tc.l, tc.r); !errors.Is(err, tc.want) {
			t.Errorf("%s %s %s = %s, %v; want %v", FormatConst(tc.l), tc.op, FormatConst(tc.r), FormatConst(got), err, tc.want)
		}
	}
}

// TestInfinityConstantFolding folds `-*` and arithmetic over it as the runtime answers it.
func TestInfinityConstantFolding(t *testing.T) {
	neg, ok := EvalUnary(ast.OpNeg, PositiveInfinity())
	if !ok || !neg.IsNegativeInfinity() {
		t.Fatalf("-* = %+v, %v", neg, ok)
	}
	if back, ok := EvalUnary(ast.OpNeg, neg); !ok || !back.IsUnbounded() {
		t.Errorf("-(-*) = %+v, %v", back, ok)
	}
	if got, err := Pow(neg, IntValue(3), DefaultMaxIntegerBits); err != nil || !got.IsNegativeInfinity() {
		t.Errorf("(-*) ** 3 = %+v, %v", got, err)
	}
}
