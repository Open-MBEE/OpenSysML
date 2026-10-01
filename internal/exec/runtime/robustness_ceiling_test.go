package runtime

import (
	"errors"
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessCeiling covers OpenSysMLMathFunctions::ceiling: a result
// beyond int64, an infinity, a non-numeric operand, and the least int64 as a value.
func TestRuntimeRobustnessCeiling(t *testing.T) {
	t.Run("ceiling_beyond_int64", testCeilingBeyondInt64)
	t.Run("ceiling_of_an_infinity", testCeilingOfAnInfinity)
	t.Run("ceiling_of_the_least_integer", testCeilingOfTheLeastInteger)
	t.Run("ceiling_of_a_boolean", testCeilingOfABoolean)
}

// ceilingCalc builds a calc that applies ceiling to a parameter of the given type.
func ceilingCalc(t *testing.T, xType string) func(x semantics.Value) (Value, error) {
	src := `
		package test {
			private import ScalarValues::*;
			private import OpenSysMLMathFunctions::*;
			calc whole {
				in x : ` + xType + `;
				return : Integer = ceiling(x);
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "whole", ast.DefCalc)
	if sym == nil {
		t.Fatal("whole calc not found")
	}
	return func(x semantics.Value) (Value, error) {
		return ctx.InvokeCalc(sym, []Value{{Kind: ValConst, Const: x}}, rootScope)
	}
}

// A ceiling at or beyond 2^63, or below -2^63, is the Integer it is, in full:
// Integers are unbounded, so it is never a wrap and never an overflow.
func testCeilingBeyondInt64(t *testing.T) {
	whole := ceilingCalc(t, "Real")
	for _, tc := range []struct {
		x    float64
		want string
	}{
		{-float64(math.MinInt64), "9223372036854775808"},
		{1e20, "100000000000000000000"},
		{-1e20, "-100000000000000000000"},
		{math.Ldexp(1, 200), new(big.Int).Lsh(big.NewInt(1), 200).String()},
	} {
		got, err := whole(semantics.Value{Kind: semantics.ValReal, Real: tc.x})
		if err != nil || got.Const.Kind != semantics.ValInt || got.Const.FormatInt() != tc.want {
			t.Fatalf("ceiling(%v) = %+v, %v; want %s", tc.x, got, err, tc.want)
		}
	}
}

// An infinity is no Integer: its ceiling is a typed overflow, not a value.
func testCeilingOfAnInfinity(t *testing.T) {
	whole := ceilingCalc(t, "Real")
	for _, x := range []float64{math.Inf(1), math.Inf(-1)} {
		got, err := whole(semantics.Value{Kind: semantics.ValReal, Real: x})
		if !errors.Is(err, semantics.ErrArithmeticOverflow) || !strings.Contains(err.Error(), "is no Integer") {
			t.Fatalf("ceiling(%v) = %+v, %v; want an overflow error", x, got, err)
		}
	}
}

// The least Integer and the values just above it have a ceiling, which a
// negated floor would lose to the overflow of positive 2^63.
func testCeilingOfTheLeastInteger(t *testing.T) {
	whole := ceilingCalc(t, "Real")
	for _, tc := range []struct {
		x    float64
		want int64
	}{
		{math.MinInt64, math.MinInt64},
		{-9223372036854774784, -9223372036854774784},
		{9223372036854774784, 9223372036854774784},
		{-0.5, 0},
	} {
		got, err := whole(semantics.Value{Kind: semantics.ValReal, Real: tc.x})
		if err != nil || got.Const.Kind != semantics.ValInt || got.Const.Int != tc.want {
			t.Fatalf("ceiling(%v) = %+v, %v; want %d", tc.x, got, err, tc.want)
		}
	}
}

// A Boolean operand is a type mismatch, not a ceiling of 0 or 1.
func testCeilingOfABoolean(t *testing.T) {
	whole := ceilingCalc(t, "Boolean")
	got, err := whole(semantics.Value{Kind: semantics.ValBool, Bool: true})
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("ceiling(true) = %+v, %v; want ErrTypeMismatch", got, err)
	}
}
