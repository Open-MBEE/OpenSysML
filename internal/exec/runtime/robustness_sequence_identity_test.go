package runtime

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// TestRuntimeRobustnessSequenceIdentity covers the edges of `===` over
// sequences: element-wise identity answers in bounded time whatever the size,
// sequences of other sizes or of no value compare without failing, and the
// function form, declared over [0..1] operands, still refuses a sequence with a
// typed multiplicity error rather than answering.
func TestRuntimeRobustnessSequenceIdentity(t *testing.T) {
	cases := []struct {
		name, expr string
		want       bool
	}{
		{"mixed_kinds", "(1, 2.5) === (1.0, 2.5)", false},
		{"same_kinds", "(1, 2.5) === (1, 2.5)", true},
		{"negated", "(1, 2.5) !== (1.0, 2.5)", true},
		{"nested", "((1, 2), 2.5) === ((1, 2.0), 2.5)", false},
		{"other_size", "(1, 2) === (1, 2, 3)", false},
		{"empty_and_null", "() === null", true},
		{"sequence_and_null", "(1, 2) === null", false},
		{"strings", `("a", 1) === ("a", 1.0)`, false},
		{"equality_unchanged", "(1, 2.5) == (1.0, 2.5)", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := evalCollectionExpr(t, c.expr)
			if err != nil {
				t.Fatalf("%s: %v", c.expr, err)
			}
			if got.Kind != ValConst || got.Const.Bool != c.want {
				t.Fatalf("%s = %s, want %v", c.expr, FormatValue(got), c.want)
			}
		})
	}
	t.Run("large_sequences", func(t *testing.T) {
		start := time.Now()
		got, err := evalCollectionExprBounded(t, "(1..100000) === (1..100000)", 1_000_000)
		if err != nil || got.Kind != ValConst || !got.Const.Bool {
			t.Fatalf("(1..100000) === (1..100000) = %s, %v; want true", FormatValue(got), err)
		}
		if elapsed := time.Since(start); elapsed > 5*time.Second {
			t.Fatalf("comparing 100000 elements took %v", elapsed)
		}
	})
	t.Run("function_form_over_a_sequence", func(t *testing.T) {
		_, err := evalCollectionExpr(t, "BaseFunctions::'==='((1, 2.5), (1, 2.5))")
		if !errors.Is(err, ErrMultiplicityViolation) || !strings.Contains(err.Error(), "'==='") {
			t.Fatalf("BaseFunctions::'===' over two-element sequences = %v, want %v naming '==='", err, ErrMultiplicityViolation)
		}
	})
}
