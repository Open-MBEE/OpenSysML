package runtime

import (
	"errors"
	"testing"
)

// A behavior writing the features it declares judges their count by the feature's
// effective multiplicity alone: a feature stating none and inheriting none holds what is
// written under the default 0..* (KerML 1.1 §7.4.12), a stated bound or the implicit
// [1..1] of a usage (SysML v2 §7.6.3) refuses a count outside it, as a write from
// outside the behavior does.
func TestRuntimeRobustnessWriteDefaultMultiplicity(t *testing.T) {
	t.Run("defaulted_subset_holds_many", testWriteDefaultedSubsetHoldsMany)
	t.Run("implicit_one_refuses_two", testWriteImplicitOneRefusesTwo)
	t.Run("stated_bound_judged", testWriteStatedBoundJudged)
}

func writeOutcome(t *testing.T, decls, write string) (Value, error) {
	t.Helper()
	ctx, action := loadAction(t, `package test {
		private import ScalarValues::Real;
		action def A {
			`+decls+`
			assign x := `+write+`;
		}
	}`, "A")
	out, err := ctx.ExecuteAction(action)
	return out["x"], err
}

func testWriteDefaultedSubsetHoldsMany(t *testing.T) {
	for _, tc := range []struct{ name, decls string }{
		{"subsetting an unbounded feature", "attribute many : Real[0..*]; attribute x : Real :> many;"},
		{"stating an unbounded multiplicity", "attribute x : Real[0..*];"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x, err := writeOutcome(t, tc.decls, "(3.0, 4.0)")
			if err != nil {
				t.Fatalf("ExecuteAction: %v", err)
			}
			if got := FormatValue(x); got != "[3.0, 4.0]" {
				t.Fatalf("x = %s, want [3.0, 4.0]", got)
			}
		})
	}
}

func testWriteImplicitOneRefusesTwo(t *testing.T) {
	_, err := writeOutcome(t, "attribute x : Real;", "(3.0, 4.0)")
	if !errors.Is(err, ErrMultiplicityViolation) {
		t.Fatalf("two values into an implicit [1..1] attribute = %v, want ErrMultiplicityViolation", err)
	}
}

func testWriteStatedBoundJudged(t *testing.T) {
	if _, err := writeOutcome(t, "attribute x : Real[1..2];", "(3.0, 4.0, 5.0)"); !errors.Is(err, ErrMultiplicityViolation) {
		t.Fatalf("three values into [1..2] = %v, want ErrMultiplicityViolation", err)
	}
	if _, err := writeOutcome(t, "attribute x : Real[1..2];", "()"); !errors.Is(err, ErrMultiplicityViolation) {
		t.Fatalf("no value into [1..2] = %v, want ErrMultiplicityViolation", err)
	}
	x, err := writeOutcome(t, "attribute x : Real[1..2];", "(3.0, 4.0)")
	if err != nil {
		t.Fatalf("two values into [1..2]: %v", err)
	}
	if got := FormatValue(x); got != "[3.0, 4.0]" {
		t.Fatalf("x = %s, want [3.0, 4.0]", got)
	}
}
