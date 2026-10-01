package runtime

import (
	"errors"
	"math"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessIntegerQuotient covers the failure modes of
// OpenSysMLMathFunctions::quotient: zero divisor, a quotient beyond int64 and a
// non-Integer operand.
func TestRuntimeRobustnessIntegerQuotient(t *testing.T) {
	t.Run("quotient_by_zero", testIntegerQuotientByZero)
	t.Run("quotient_of_the_least_int64_by_minus_one", testIntegerQuotientOfLeastInt64ByMinusOne)
	t.Run("quotient_of_big_integers", testIntegerQuotientOfBigIntegers)
	t.Run("quotient_of_a_real", testIntegerQuotientOfAReal)
}

// quotientCalc builds a calc that applies quotient to parameters of the given types.
func quotientCalc(t *testing.T, xType, yType string) (*Context, func(x, y semantics.Value) (Value, error)) {
	src := `
		package test {
			private import ScalarValues::*;
			private import OpenSysMLMathFunctions::*;
			calc divide {
				in x : ` + xType + `;
				in y : ` + yType + `;
				return : Integer = quotient(x, y);
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "divide", ast.DefCalc)
	if sym == nil {
		t.Fatal("divide calc not found")
	}
	return ctx, func(x, y semantics.Value) (Value, error) {
		return ctx.InvokeCalc(sym, []Value{{Kind: ValConst, Const: x}, {Kind: ValConst, Const: y}}, rootScope)
	}
}

// A zero divisor is ErrDivisionByZero, as for `/`.
func testIntegerQuotientByZero(t *testing.T) {
	_, divide := quotientCalc(t, "Integer", "Integer")
	got, err := divide(semantics.Value{Kind: semantics.ValInt, Int: 7}, semantics.Value{Kind: semantics.ValInt, Int: 0})
	if !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("quotient(7, 0) = %+v, %v; want ErrDivisionByZero", got, err)
	}
}

// MinInt64 / -1 is 2^63, an Integer beyond int64: the value, never a wrap.
func testIntegerQuotientOfLeastInt64ByMinusOne(t *testing.T) {
	_, divide := quotientCalc(t, "Integer", "Integer")
	got, err := divide(semantics.IntValue(math.MinInt64), semantics.IntValue(-1))
	if err != nil || got.Const.Kind != semantics.ValInt || got.Const.FormatInt() != "9223372036854775808" {
		t.Fatalf("quotient(MinInt64, -1) = %+v, %v; want 9223372036854775808", got, err)
	}
	for _, tc := range []struct{ x, y, want int64 }{
		{math.MinInt64, 1, math.MinInt64},
		{math.MinInt64 + 1, -1, math.MaxInt64},
		{math.MaxInt64, -1, -math.MaxInt64},
	} {
		got, err := divide(semantics.Value{Kind: semantics.ValInt, Int: tc.x}, semantics.Value{Kind: semantics.ValInt, Int: tc.y})
		if err != nil || got.Const.Kind != semantics.ValInt || got.Const.Int != tc.want {
			t.Fatalf("quotient(%d, %d) = %+v, %v; want %d", tc.x, tc.y, got, err, tc.want)
		}
	}
}

// The quotient of Integers beyond int64 truncates toward zero exactly, and
// demotes to int64 when it fits.
func testIntegerQuotientOfBigIntegers(t *testing.T) {
	_, divide := quotientCalc(t, "Integer", "Integer")
	big70, _ := semantics.ParseInteger("1180591620717411303424")
	big69, _ := semantics.ParseInteger("590295810358705651712")
	for _, tc := range []struct {
		x, y semantics.Value
		want string
	}{
		{big70, big69, "2"},
		{big70, semantics.IntValue(7), "168655945816773043346"},
		{semantics.IntNeg(big70), semantics.IntValue(7), "-168655945816773043346"},
		{semantics.IntValue(7), big70, "0"},
	} {
		got, err := divide(tc.x, tc.y)
		if err != nil || got.Const.Kind != semantics.ValInt || got.Const.FormatInt() != tc.want {
			t.Fatalf("quotient(%s, %s) = %+v, %v; want %s", tc.x.FormatInt(), tc.y.FormatInt(), got, err, tc.want)
		}
		if tc.want == "2" && got.Const.IsBigInt() {
			t.Errorf("quotient(%s, %s) did not demote to int64", tc.x.FormatInt(), tc.y.FormatInt())
		}
	}
}

// A Real operand is a type mismatch, not a truncated quotient.
func testIntegerQuotientOfAReal(t *testing.T) {
	_, divide := quotientCalc(t, "Real", "Integer")
	got, err := divide(semantics.Value{Kind: semantics.ValReal, Real: 7.5}, semantics.Value{Kind: semantics.ValInt, Int: 2})
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("quotient(7.5, 2) = %+v, %v; want ErrTypeMismatch", got, err)
	}
}
