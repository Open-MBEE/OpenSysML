package runtime

import (
	"errors"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessExactRational covers the failure modes of exact
// Rationals: a denominator that grows past the size budget, a power or a
// literal beyond it, and a zero divisor are typed errors, never a rounded
// value, a hang or a panic.
func TestRuntimeRobustnessExactRational(t *testing.T) {
	t.Run("denominator_growth_beyond_a_lowered_budget", testDenominatorGrowthBeyondALoweredBudget)
	t.Run("power_beyond_the_size_budget", testRationalPowerBeyondTheSizeBudget)
	t.Run("literal_beyond_the_size_budget", testRationalLiteralBeyondTheSizeBudget)
	t.Run("conversion_beyond_a_lowered_budget", testRationalConversionBeyondALoweredBudget)
	t.Run("division_by_zero", testRationalDivisionByZero)
}

// The harmonic sum's denominator is lcm(1..n): within a 64-bit budget ten
// terms are exact (7381/2520) and two hundred are refused, not rounded.
func testDenominatorGrowthBeyondALoweredBudget(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			private import ControlFunctions::*;
			calc harmonic {
				in n : Integer;
				return : Rational = RationalFunctions::sum((1..n)->collect { in i : Integer; 1 / i });
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	budgets := DefaultBudgets()
	budgets.MaxIntegerBits = MinMaxIntegerBits
	if err := ctx.SetBudgets(budgets); err != nil {
		t.Fatal(err)
	}
	root := idx.DocumentRoot("<test>")
	sym := findSymbolByName(root, "harmonic", ast.DefCalc)
	if sym == nil {
		t.Fatal("harmonic calc not found")
	}
	got, err := ctx.InvokeCalc(sym, []Value{{Kind: ValConst, Const: semantics.IntValue(10)}}, root)
	if err != nil || got.Kind != ValConst || got.Const.Kind != semantics.ValRational || got.Const.FormatRational() != "7381/2520" {
		t.Fatalf("harmonic(10) = %s, %v; want 7381/2520", FormatValue(got), err)
	}
	start := time.Now()
	if got, err := ctx.InvokeCalc(sym, []Value{{Kind: ValConst, Const: semantics.IntValue(200)}}, root); !errors.Is(err, semantics.ErrRationalSizeLimit) {
		t.Fatalf("harmonic(200) under a 64-bit budget = %s, %v; want %v", FormatValue(got), err, semantics.ErrRationalSizeLimit)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("refusing harmonic(200) took %v", elapsed)
	}
}

// (1/3) ** 2000000 needs about 3.2 million bits: refused at once.
func testRationalPowerBeyondTheSizeBudget(t *testing.T) {
	start := time.Now()
	if got, err := evalCollectionExpr(t, "(1 / 3) ** 2000000"); !errors.Is(err, semantics.ErrRationalSizeLimit) {
		t.Fatalf("(1 / 3) ** 2000000 = %s, %v; want %v", FormatValue(got), err, semantics.ErrRationalSizeLimit)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("refusing (1 / 3) ** 2000000 took %v", elapsed)
	}
	got, err := evalCollectionExpr(t, "(2 / 3) ** -3")
	if err != nil || got.Kind != ValConst || got.Const.FormatRational() != "3.375" {
		t.Fatalf("(2 / 3) ** -3 = %s, %v; want 3.375", FormatValue(got), err)
	}
}

// A literal is the exact Rational it spells, so one past the budget is refused
// rather than read as the nearest double, zero or infinity.
func testRationalLiteralBeyondTheSizeBudget(t *testing.T) {
	for _, src := range []string{"1.0e400000", "1.0e-400000"} {
		if got, err := evalCollectionExpr(t, src); !errors.Is(err, semantics.ErrRationalSizeLimit) {
			t.Errorf("%s = %s, %v; want %v", src, FormatValue(got), err, semantics.ErrRationalSizeLimit)
		}
	}
}

// ToRational reads its text under the run's budget, however few digits spell
// it: eighteen fractional digits need 117 bits, past a 64-bit budget.
func testRationalConversionBeyondALoweredBudget(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			calc read {
				in s : String;
				return : Rational = RationalFunctions::ToRational(s);
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	budgets := DefaultBudgets()
	budgets.MaxIntegerBits = MinMaxIntegerBits
	if err := ctx.SetBudgets(budgets); err != nil {
		t.Fatal(err)
	}
	root := idx.DocumentRoot("<test>")
	sym := findSymbolByName(root, "read", ast.DefCalc)
	if sym == nil {
		t.Fatal("read calc not found")
	}
	got, err := ctx.InvokeCalc(sym, []Value{NewStringValue("0.25")}, root)
	if err != nil || got.Kind != ValConst || got.Const.FormatRational() != "0.25" {
		t.Fatalf("ToRational(\"0.25\") = %s, %v; want 0.25", FormatValue(got), err)
	}
	if got, err := ctx.InvokeCalc(sym, []Value{NewStringValue("0.123456789012345678")}, root); !errors.Is(err, semantics.ErrRationalSizeLimit) {
		t.Fatalf("ToRational of 18 digits under a 64-bit budget = %s, %v; want %v", FormatValue(got), err, semantics.ErrRationalSizeLimit)
	}
}

// A zero divisor of an exact quotient is a typed error, for a Rational and an
// Integer dividend alike.
func testRationalDivisionByZero(t *testing.T) {
	for _, src := range []string{"(1 / 3) / 0", "0.1 / 0.0", "RationalFunctions::rat(1, 0)"} {
		if got, err := evalCollectionExpr(t, src); !errors.Is(err, ErrDivisionByZero) {
			t.Errorf("%s = %s, %v; want %v", src, FormatValue(got), err, ErrDivisionByZero)
		}
	}
}
