package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestArithmeticRejectsBooleanOperands checks an arithmetic operator over a
// Boolean is a type mismatch naming both operand types, never a number.
func TestArithmeticRejectsBooleanOperands(t *testing.T) {
	cases := []struct {
		expr string
		want string
	}{
		{"(1 > 0) * 5", "operator '*' is not defined for a Boolean and an Integer"},
		{"true + 1", "operator '+' is not defined for a Boolean and an Integer"},
		{"1 + true", "operator '+' is not defined for an Integer and a Boolean"},
		{"true - true", "operator '-' is not defined for a Boolean and a Boolean"},
		{"true / 2", "operator '/' is not defined for a Boolean and an Integer"},
		{"true % 2", "operator '%' is not defined for a Boolean and an Integer"},
		{"true ** 2", "operator '**' is not defined for a Boolean and an Integer"},
		{"2 ** false", "operator '**' is not defined for an Integer and a Boolean"},
		{"1.5 * true", "operator '*' is not defined for a Rational and a Boolean"},
		{"2 ** 0.5 - false", "operator '-' is not defined for a Real and a Boolean"},
		{"* + true", "operator '+' is not defined for an infinity and a Boolean"},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			_, got, err := evalDeclaredExpr(t, "package test {}", tc.expr)
			if err == nil {
				t.Fatalf("%s = %v, want a type mismatch", tc.expr, got)
			}
			if !errors.Is(err, ErrTypeMismatch) {
				t.Fatalf("%s: error %v, want a type mismatch", tc.expr, err)
			}
			if !strings.HasSuffix(err.Error(), tc.want) {
				t.Errorf("%s: error %q, want it to end with %q", tc.expr, err, tc.want)
			}
		})
	}
}
