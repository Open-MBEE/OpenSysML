package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestBooleanOperandErrorDescribesTheOperand checks a non-Boolean operand of a
// logical operator or `if` is named by its type, never by its value kind.
func TestBooleanOperandErrorDescribesTheOperand(t *testing.T) {
	cases := []struct {
		expr string
		want string
	}{
		{"true and 3", "right operand of 'and' must be Boolean, got an Integer"},
		{"3 and true", "left operand of 'and' must be Boolean, got an Integer"},
		{"false or 2.5", "right operand of 'or' must be Boolean, got a Rational"},
		{`true xor "a"`, "right operand of 'xor' must be Boolean, got a string"},
		{"true implies 3", "right operand of 'implies' must be Boolean, got an Integer"},
		{"true & 3", "right operand of '&' must be Boolean, got an Integer"},
		{"false | 3", "right operand of '|' must be Boolean, got an Integer"},
		{"if 3 ? 1 else 2", "condition of 'if' must be Boolean, got an Integer"},
		{"true and (1, 2)", "right operand of 'and' must be Boolean, got a sequence"},
		{"null and true", "left operand of 'and' must be Boolean, got null"},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			_, _, err := evalDeclaredExpr(t, "package test {}", tc.expr)
			if !errors.Is(err, ErrTypeMismatch) {
				t.Fatalf("%s: error %v, want a type mismatch", tc.expr, err)
			}
			if !strings.HasSuffix(err.Error(), tc.want) {
				t.Errorf("%s: error %q, want it to end with %q", tc.expr, err, tc.want)
			}
		})
	}
}
