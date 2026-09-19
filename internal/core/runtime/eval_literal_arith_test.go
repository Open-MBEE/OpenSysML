package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// TestStringLiteralEscapes evaluates the escapes KerML §8.2.2 defines: a literal
// stands for the characters it escapes, not for the backslashes it is written
// with.
func TestStringLiteralEscapes(t *testing.T) {
	tests := []struct {
		expr string
		want string
	}{
		{`"a\nb"`, "a\nb"},
		{`"a\tb"`, "a\tb"},
		{`"a\rb"`, "a\rb"},
		{`"a\bb"`, "a\bb"},
		{`"a\fb"`, "a\fb"},
		{`"say \"hi\""`, `say "hi"`},
		{`"a\\b"`, `a\b`},
		{`"it\'s"`, "it's"},
		{`"héllo 🚗"`, "héllo 🚗"},
		{`"a\nb" + "\tc"`, "a\nb\tc"},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			got, err := evalStringExpr(t, tt.expr)
			if err != nil {
				t.Fatalf("eval %s: %v", tt.expr, err)
			}
			if got.Kind != ValString || got.Str() != tt.want {
				t.Errorf("eval %s = %#v, want %q", tt.expr, got, tt.want)
			}
		})
	}
}

// TestIntegerArithmeticOverflowIsReported requires an arithmetic result outside
// the Integer range to be an error, as the Integer functions report it, rather
// than the wrapped number the int64 arithmetic would give.
func TestIntegerArithmeticOverflowIsReported(t *testing.T) {
	for _, expr := range []string{
		"9223372036854775807 + 1",
		"-9223372036854775807 - 2",
		"9223372036854775807 * 2",
	} {
		t.Run(expr, func(t *testing.T) {
			value, err := evalStringExpr(t, expr)
			if err == nil {
				t.Fatalf("eval %s = %v, want an overflow error", expr, value)
			}
			if !errors.Is(err, semantics.ErrArithmeticOverflow) {
				t.Fatalf("eval %s: %v, want an overflow error", expr, err)
			}
			if !strings.Contains(err.Error(), "Integer range") {
				t.Errorf("eval %s: %v does not say what range was left", expr, err)
			}
		})
	}
}

// TestRealArithmeticOverflowIsReported requires an arithmetic result that is no
// finite Real to be an error rather than an infinity.
func TestRealArithmeticOverflowIsReported(t *testing.T) {
	value, err := evalStringExpr(t, "1.0e308 * 10.0")
	if err == nil {
		t.Fatalf("eval = %v, want an overflow error", value)
	}
	if !errors.Is(err, semantics.ErrArithmeticOverflow) {
		t.Fatalf("eval: %v, want an overflow error", err)
	}
}

// TestIntegerArithmeticInRangeIsUnchanged keeps the arithmetic that fits
// answering, including the extremes of the Integer range.
func TestIntegerArithmeticInRangeIsUnchanged(t *testing.T) {
	tests := []struct {
		expr string
		want int64
	}{
		{"2 + 3", 5},
		{"9223372036854775806 + 1", 9223372036854775807},
		{"0 - 9223372036854775807 - 1", -9223372036854775808},
		{"4611686018427387903 * 2", 9223372036854775806},
		{"7 % 2", 1},
		{"(0 - 9223372036854775807 - 1) % (0 - 1)", 0},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			got, err := evalStringExpr(t, tt.expr)
			if err != nil {
				t.Fatalf("eval %s: %v", tt.expr, err)
			}
			if got.Kind != ValConst || got.Const.Kind != semantics.ValInt || got.Const.Int != tt.want {
				t.Errorf("eval %s = %#v, want %d", tt.expr, got, tt.want)
			}
		})
	}
}
