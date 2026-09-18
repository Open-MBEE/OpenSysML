package runtime

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// TestRuntimeRobustnessIntegerQuotient covers the failure modes of
// OpenSysMLMathFunctions::quotient: zero divisor, overflow and a non-Integer operand.
func TestRuntimeRobustnessIntegerQuotient(t *testing.T) {
	t.Run("quotient_by_zero", testIntegerQuotientByZero)
	t.Run("quotient_of_the_least_integer_by_minus_one", testIntegerQuotientOfLeastIntegerByMinusOne)
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

// MinInt64 / -1 is 2^63, outside the Integer range: an overflow, never a wrap.
func testIntegerQuotientOfLeastIntegerByMinusOne(t *testing.T) {
	_, divide := quotientCalc(t, "Integer", "Integer")
	got, err := divide(semantics.Value{Kind: semantics.ValInt, Int: math.MinInt64}, semantics.Value{Kind: semantics.ValInt, Int: -1})
	if !errors.Is(err, semantics.ErrArithmeticOverflow) || !strings.Contains(err.Error(), "exceeds the Integer range") {
		t.Fatalf("quotient(MinInt64, -1) = %+v, %v; want an overflow error", got, err)
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

// A Real operand is a type mismatch, not a truncated quotient.
func testIntegerQuotientOfAReal(t *testing.T) {
	_, divide := quotientCalc(t, "Real", "Integer")
	got, err := divide(semantics.Value{Kind: semantics.ValReal, Real: 7.5}, semantics.Value{Kind: semantics.ValInt, Int: 2})
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("quotient(7.5, 2) = %+v, %v; want ErrTypeMismatch", got, err)
	}
}
