package runtime

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessRationalRealComparison covers the edges of a Rational
// compared at Real precision with a binary64 Real: NaN, the infinities, a
// magnitude past the binary64 range, set membership, and a Real-typed
// accumulation that stays a double rather than growing an exact denominator.
func TestRuntimeRobustnessRationalRealComparison(t *testing.T) {
	t.Run("against_nan", testRationalAgainstNaN)
	t.Run("against_the_infinities", testRationalAgainstTheInfinities)
	t.Run("beyond_the_binary64_range", testRationalBeyondTheBinary64Range)
	t.Run("set_membership", testRationalRealSetMembership)
	t.Run("real_accumulation_under_a_lowered_budget", testRealAccumulationUnderALoweredBudget)
}

const rationalRealComparisonSrc = `
	package test {
		private import ScalarValues::*;
		calc tenth { in r; return : Boolean[6] ordered nonunique = (0.1 == r, 0.1 != r, 0.1 < r, 0.1 <= r, 0.1 > r, 0.1 >= r); }
		calc huge { in r; return : Boolean[6] ordered nonunique = (1.0e400 == r, 1.0e400 != r, 1.0e400 < r, 1.0e400 <= r, 1.0e400 > r, 1.0e400 >= r); }
		calc tiny { in r; return : Boolean[6] ordered nonunique = (-1.0e400 == r, -1.0e400 != r, -1.0e400 < r, -1.0e400 <= r, -1.0e400 > r, -1.0e400 >= r); }
	}
`

// compareRationalWithReal invokes the named calc of rationalRealComparisonSrc on r
// and returns its six comparisons: ==, !=, <, <=, >, >=.
func compareRationalWithReal(t *testing.T, calc string, r float64) [6]bool {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, rationalRealComparisonSrc))
	root := idx.DocumentRoot("<test>")
	sym := findSymbolByName(root, calc, ast.DefCalc)
	if sym == nil {
		t.Fatalf("%s calc not found", calc)
	}
	got, err := ctx.InvokeCalc(sym, []Value{realConst(r)}, root)
	if err != nil {
		t.Fatalf("%s(%v): %v", calc, r, err)
	}
	elements := elementsOf(got)
	if len(elements) != 6 {
		t.Fatalf("%s(%v) = %s; want six Booleans", calc, r, FormatValue(got))
	}
	var out [6]bool
	for i, e := range elements {
		if e.Kind != ValConst || e.Const.Kind != semantics.ValBool {
			t.Fatalf("%s(%v)[%d] = %s; want a Boolean", calc, r, i, FormatValue(e))
		}
		out[i] = e.Const.Bool
	}
	return out
}

// A Real feature admits no NaN, but an untyped parameter holds one: a Rational
// meets it as a Real does, unequal and neither above nor below it.
func testRationalAgainstNaN(t *testing.T) {
	for _, calc := range []string{"tenth", "huge", "tiny"} {
		if got, want := compareRationalWithReal(t, calc, math.NaN()), [6]bool{false, true, false, false, false, false}; got != want {
			t.Errorf("%s against NaN = %v; want %v", calc, got, want)
		}
	}
}

// A finite Rational lies strictly between the infinities.
func testRationalAgainstTheInfinities(t *testing.T) {
	if got, want := compareRationalWithReal(t, "tenth", math.Inf(1)), [6]bool{false, true, true, true, false, false}; got != want {
		t.Errorf("0.1 against +Inf = %v; want %v", got, want)
	}
	if got, want := compareRationalWithReal(t, "tenth", math.Inf(-1)), [6]bool{false, true, false, false, true, true}; got != want {
		t.Errorf("0.1 against -Inf = %v; want %v", got, want)
	}
}

// A Rational no binary64 holds compares as the infinity of its sign: above the
// largest double, and equal to the infinity itself.
func testRationalBeyondTheBinary64Range(t *testing.T) {
	cases := []struct {
		calc string
		r    float64
		want [6]bool
	}{
		{"huge", math.MaxFloat64, [6]bool{false, true, false, false, true, true}},
		{"huge", math.Inf(1), [6]bool{true, false, false, true, false, true}},
		{"huge", math.Inf(-1), [6]bool{false, true, false, false, true, true}},
		{"tiny", -math.MaxFloat64, [6]bool{false, true, true, true, false, false}},
		{"tiny", math.Inf(-1), [6]bool{true, false, false, true, false, true}},
	}
	for _, c := range cases {
		if got := compareRationalWithReal(t, c.calc, c.r); got != c.want {
			t.Errorf("%s against %v = %v; want %v", c.calc, c.r, got, c.want)
		}
	}
}

// A Rational and the Real it rounds to are one member of a set, and two
// Rationals stay distinct however close.
func testRationalRealSetMembership(t *testing.T) {
	third := rationalConst(t, 1, 3)
	nearest := rationalConst(t, 6004799503160661, 18014398509481984)
	tenth := rationalConst(t, 1, 10)
	s := NewSet()
	s.Add(tenth)
	s.Add(realConst(0.1))
	if s.Size() != 1 || !s.Contains(realConst(0.1)) || !s.Contains(tenth) {
		t.Errorf("{1/10, 0.1} has %d members; want one", s.Size())
	}
	s = NewSet()
	s.Add(third)
	s.Add(nearest)
	if s.Size() != 2 {
		t.Errorf("{1/3, %s} has %d members; want two", FormatValue(nearest), s.Size())
	}
	if !s.Contains(realConst(1.0 / 3.0)) {
		t.Error("a Real 1/3 is not a member of a set holding the Rational 1/3")
	}
}

// A Real feature holds each step's sum as a double, so ten thousand steps of
// an exact 0.01 stay fast under the smallest budget and never reach it.
func testRealAccumulationUnderALoweredBudget(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			action accumulate {
				out attribute total : Real = 0.0;
				first start;
				then action sum {
					for i in 1..10000 {
						assign total := total + 0.01;
					}
				}
				then done;
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	budgets := DefaultBudgets()
	budgets.MaxIntegerBits = MinMaxIntegerBits
	if err := ctx.SetBudgets(budgets); err != nil {
		t.Fatal(err)
	}
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "accumulate", ast.DefAction)
	if sym == nil {
		t.Fatal("accumulate action not found")
	}
	start := time.Now()
	out, err := ctx.ExecuteAction(sym)
	if errors.Is(err, semantics.ErrRationalSizeLimit) {
		t.Fatalf("a Real accumulation reached the Rational size budget: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	total := out["total"]
	if total.Kind != ValConst || total.Const.Kind != semantics.ValReal {
		t.Fatalf("total = %s; want a Real", FormatValue(total))
	}
	if math.Abs(total.Const.Real-100) > 1e-9 || total.Const.Real == 100 {
		t.Errorf("total = %v; want the binary64 sum near 100", total.Const.Real)
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Errorf("ten thousand Real steps took %v", elapsed)
	}
}

func rationalConst(t *testing.T, num, den int64) Value {
	t.Helper()
	r, ok := semantics.FracValue(num, den)
	if !ok {
		t.Fatalf("%d/%d is not a Rational", num, den)
	}
	return Value{Kind: ValConst, Const: r}
}
