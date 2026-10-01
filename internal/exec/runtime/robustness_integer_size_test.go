package runtime

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessIntegerSize covers the failure modes of Integers beyond
// int64: a result larger than the integer size budget is refused with a typed
// error rather than computed at unbounded cost, and an Integer beyond int64
// used where it must address something — an index, a range materialized past
// the element budget — is a typed error naming it.
func TestRuntimeRobustnessIntegerSize(t *testing.T) {
	t.Run("power_beyond_the_size_budget", testPowerBeyondTheSizeBudget)
	t.Run("product_beyond_a_lowered_size_budget", testProductBeyondALoweredSizeBudget)
	t.Run("constant_beyond_a_lowered_size_budget", testConstantBeyondALoweredSizeBudget)
	t.Run("range_with_bounds_beyond_int64", testRangeWithBoundsBeyondInt64)
	t.Run("range_beyond_the_element_budget", testRangeBeyondTheElementBudget)
	t.Run("index_beyond_int64", testIndexBeyondInt64)
}

// 2 ** 2^20 needs one bit more than the default budget allows: refused at once,
// naming the budget, instead of computing a megabit Integer.
func testPowerBeyondTheSizeBudget(t *testing.T) {
	start := time.Now()
	_, err := evalCollectionExpr(t, "2 ** 1048576")
	if !errors.Is(err, ErrIntegerSizeLimit) {
		t.Fatalf("2 ** 1048576 = %v, want %v", err, ErrIntegerSizeLimit)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("refusing 2 ** 1048576 took %v", elapsed)
	}
	got, err := evalCollectionExpr(t, "2 ** 1048575 - 2 ** 1048575")
	if err != nil || got.Kind != ValConst || !got.Const.Equal(semantics.IntValue(0)) {
		t.Fatalf("2 ** 1048575 - 2 ** 1048575 = %s, %v; want 0", FormatValue(got), err)
	}
}

// The budget is the run's: lowered to 100 bits, a product of 120 bits is
// refused while one of 80, past int64 but within it, is computed exactly.
func testProductBeyondALoweredSizeBudget(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			calc square {
				in x : Integer;
				return : Integer = x * x;
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	budgets := DefaultBudgets()
	budgets.MaxIntegerBits = 100
	if err := ctx.SetBudgets(budgets); err != nil {
		t.Fatal(err)
	}
	root := idx.DocumentRoot("<test>")
	sym := findSymbolByName(root, "square", ast.DefCalc)
	if sym == nil {
		t.Fatal("square calc not found")
	}
	square := func(x semantics.Value) (Value, error) {
		return ctx.InvokeCalc(sym, []Value{{Kind: ValConst, Const: x}}, root)
	}
	if _, err := square(semantics.IntValue(1 << 60)); !errors.Is(err, ErrIntegerSizeLimit) {
		t.Fatalf("square(2^60) under a 100-bit budget = %v, want %v", err, ErrIntegerSizeLimit)
	}
	got, err := square(semantics.IntValue(1 << 40))
	if err != nil || got.Const.FormatInt() != "1208925819614629174706176" {
		t.Fatalf("square(2^40) = %s, %v; want 2^80", FormatValue(got), err)
	}
}

// A constant expression is held to the run's budget too: lowered to 64 bits,
// 2 ** 70 and an intermediate 2 ** 70 whose final result fits are refused,
// while the default budget computes them exactly.
func testConstantBeyondALoweredSizeBudget(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			calc power { return : Integer = 2 ** 70; }
			calc cancelled { return : Integer = 2 ** 70 - 2 ** 70 + 1; }
		}
	`
	for _, tc := range []struct {
		calc string
		want string
	}{
		{calc: "power", want: "1180591620717411303424"},
		{calc: "cancelled", want: "1"},
	} {
		for _, maxBits := range []int64{64, DefaultMaxIntegerBits} {
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
			budgets := DefaultBudgets()
			budgets.MaxIntegerBits = maxBits
			if err := ctx.SetBudgets(budgets); err != nil {
				t.Fatal(err)
			}
			root := idx.DocumentRoot("<test>")
			sym := findSymbolByName(root, tc.calc, ast.DefCalc)
			if sym == nil {
				t.Fatalf("%s calc not found", tc.calc)
			}
			got, err := ctx.InvokeCalc(sym, nil, root)
			if maxBits == 64 {
				if !errors.Is(err, ErrIntegerSizeLimit) {
					t.Errorf("%s under a 64-bit budget = %s, %v; want %v", tc.calc, FormatValue(got), err, ErrIntegerSizeLimit)
				}
				continue
			}
			if err != nil || got.Kind != ValConst || got.Const.FormatInt() != tc.want {
				t.Errorf("%s under the default budget = %s, %v; want %s", tc.calc, FormatValue(got), err, tc.want)
			}
		}
	}
}

// A range whose bounds are beyond int64 has as many elements as they span.
func testRangeWithBoundsBeyondInt64(t *testing.T) {
	got, err := evalCollectionExpr(t, "(2 ** 70 .. 2 ** 70 + 2)")
	if err != nil {
		t.Fatal(err)
	}
	if text := FormatValue(got); text != "[1180591620717411303424, 1180591620717411303425, 1180591620717411303426]" {
		t.Fatalf("2 ** 70 .. 2 ** 70 + 2 = %s", text)
	}
}

// A range of 2^70 elements is refused by the element budget once it is
// charged past it, under a step budget wide enough not to stop it first.
func testRangeBeyondTheElementBudget(t *testing.T) {
	_, err := evalCollectionExprBounded(t, "size(1 .. 2 ** 70)", 1<<40)
	if !errors.Is(err, ErrElementLimitExceeded) {
		t.Fatalf("size(1 .. 2 ** 70) = %v, want %v", err, ErrElementLimitExceeded)
	}
}

// An index beyond int64 addresses no element of any sequence.
func testIndexBeyondInt64(t *testing.T) {
	_, err := evalCollectionExpr(t, "xs#(2 ** 70)")
	if !errors.Is(err, ErrIndexOutOfRange) || !strings.Contains(err.Error(), "1180591620717411303424") {
		t.Fatalf("xs#(2 ** 70) = %v, want %v naming the index", err, ErrIndexOutOfRange)
	}
}
