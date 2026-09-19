package runtime

import (
	"errors"
	"testing"
)

// TestIntegerRange pins `..` as the ordered sequence of integers the library
// declares it to be (IntegerFunctions::'..' returns `Integer[0..*] ordered`), so
// the operations over a sequence apply to it unchanged.
func TestIntegerRange(t *testing.T) {
	tests := []struct {
		expr string
		want []int64
	}{
		{"1..5", []int64{1, 2, 3, 4, 5}},
		{"3..3", []int64{3}},
		{"0..2", []int64{0, 1, 2}},
		{"(0 - 2)..1", []int64{-2, -1, 0, 1}},
		// A descending range is empty: `..` counts up from lower to upper, and
		// the library gives it a `[0..*]` result, so no element is not an error.
		{"5..1", nil},
		{"1..factor", []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}},
		{"(1..5)->collect{in i; i * i}", []int64{1, 4, 9, 16, 25}},
		{"(1..5)->select{in i; i > 3}", []int64{4, 5}},
		{"IntegerFunctions::'..'(2, 4)", []int64{2, 3, 4}},
		{"(1..4)->SequenceFunctions::subsequence(2, 3)", []int64{2, 3}},
		{"xs->SequenceFunctions::subsequence(1, 2)", []int64{1, 2}},
	}
	for _, tt := range tests {
		got, err := evalCollectionExpr(t, tt.expr)
		if err != nil {
			t.Errorf("%s: %v", tt.expr, err)
			continue
		}
		if ints := intsOf(t, got); !equalInts(ints, tt.want) {
			t.Errorf("%s = %v, want %v", tt.expr, ints, tt.want)
		}
	}
}

// TestIntegerRangeSequenceOperations requires the sequence operations to see a
// range as the sequence it is, rather than as a value of their own kind.
func TestIntegerRangeSequenceOperations(t *testing.T) {
	tests := []struct {
		expr string
		want int64
	}{
		{"(1..5)->SequenceFunctions::size()", 5},
		{"(5..1)->SequenceFunctions::size()", 0},
		{"(1..5)#(2)", 2},
		{"(1..4)->NumericalFunctions::sum()", 10},
		{"(1..3)->ControlFunctions::reduce{in a; in b; a + b}", 6},
	}
	for _, tt := range tests {
		got, err := evalCollectionExpr(t, tt.expr)
		if err != nil {
			t.Errorf("%s: %v", tt.expr, err)
			continue
		}
		if got.Kind != ValConst || got.Const.Int != tt.want {
			t.Errorf("%s = %s, want %d", tt.expr, FormatTraceValue(got), tt.want)
		}
	}
}

// TestIntegerRangeNonIntegerBound requires a bound that is not an Integer to be
// reported as the type mismatch it is: `IntegerFunctions::'..'` declares
// `in lower: Integer[1]`.
func TestIntegerRangeNonIntegerBound(t *testing.T) {
	for _, expr := range []string{"1.5..3", "1..2.5", `"a".."c"`, "true..false"} {
		_, err := evalCollectionExpr(t, expr)
		if err == nil {
			t.Errorf("%s: want a type mismatch, got a value", expr)
			continue
		}
		if !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("%s: error = %v, want ErrTypeMismatch", expr, err)
		}
	}
}

// TestIntegerRangeSpendsTheStepBudget requires each element a range generates to
// cost a step, so a range too large to hold fails the run rather than exhausting
// memory.
func TestIntegerRangeSpendsTheStepBudget(t *testing.T) {
	_, err := evalCollectionExprBounded(t, "1..1000000", 1000)
	if err == nil {
		t.Fatal("want the step budget's error, got a value")
	}
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("error = %v, want ErrStepLimitExceeded", err)
	}
}

// TestIntegerRangeExtremeBounds requires a range whose width overflows, or
// exceeds what can be held at all, to report the step budget rather than panic.
func TestIntegerRangeExtremeBounds(t *testing.T) {
	for _, expr := range []string{
		"1..9223372036854775807",
		"(0 - 9223372036854775807)..9223372036854775807",
		"(1..9223372036854775807)->collect{in i; i * i}",
	} {
		_, err := evalCollectionExprBounded(t, expr, 1000)
		if err == nil {
			t.Errorf("%s: want the step budget's error, got a value", expr)
			continue
		}
		if !errors.Is(err, ErrStepLimitExceeded) {
			t.Errorf("%s: error = %v, want ErrStepLimitExceeded", expr, err)
		}
	}
}

// TestForOverIntegerRange requires `for i in 1..n` to iterate the range, which is
// the standard way to write an index loop.
func TestForOverIntegerRange(t *testing.T) {
	src := `
		package test {
			calc def Total {
				in n : Integer;
				attribute acc : Integer = 0;
				for i in 1..n {
					assign acc := acc + i;
				}
				acc
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	total, scope := calcByName(t, idx.DocumentRoot("<test>"), "test", "Total")

	got, err := ctx.InvokeCalc(total, []Value{constInt(4)}, scope)
	if err != nil {
		t.Fatalf("Total(4): %v", err)
	}
	if got.Const.Int != 10 {
		t.Errorf("Total(4) = %s, want 10", FormatTraceValue(got))
	}
}
