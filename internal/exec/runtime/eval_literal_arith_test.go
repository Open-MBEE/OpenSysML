package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
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

// TestIntegerArithmeticBeyondInt64IsExact requires an arithmetic result past
// int64 to be the exact Integer, as KerML's unbounded Integers are, rather than
// the wrapped number the int64 arithmetic would give or an overflow.
func TestIntegerArithmeticBeyondInt64IsExact(t *testing.T) {
	for expr, want := range map[string]string{
		"9223372036854775807 + 1":  "9223372036854775808",
		"-9223372036854775807 - 2": "-9223372036854775809",
		"9223372036854775807 * 2":  "18446744073709551614",
	} {
		t.Run(expr, func(t *testing.T) {
			value, err := evalStringExpr(t, expr)
			if err != nil {
				t.Fatalf("eval %s: %v", expr, err)
			}
			if FormatValue(value) != want {
				t.Fatalf("eval %s = %s, want %s", expr, FormatValue(value), want)
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

// TestFeatureArithmeticOverflowIsNoFalseVerdict pins what a holds proof means:
// an overflowed intermediate — 1e200*1e200 read through feature values, which no
// constant folding can absorb — is an evaluation error, not a false comparison.
func TestFeatureArithmeticOverflowIsNoFalseVerdict(t *testing.T) {
	src := `
package test {
	private import ScalarValues::*;
	attribute power : Real = 1.0e200;
	attribute d : Real = 1.0e200;
	attribute efficiency : Real = 0.0;
	attribute result = power * d * efficiency <= power * d;
}`
	model, resolver, root := parseAndBuildLibraryModel(t, src)
	pkg, ok := root.LookupLocal("test")
	if !ok || pkg == nil || pkg.Scope == nil {
		t.Fatal("package test has no scope")
	}
	sym, ok := pkg.Scope.LookupLocal("result")
	if !ok || sym == nil {
		t.Fatal("attribute result not found")
	}
	decl, ok := sym.Decl.(*ast.Usage)
	if !ok {
		t.Fatalf("result declares %T, want a usage", sym.Decl)
	}
	ec := NewEvalContext(NewContext(typedModel(model, resolver), 10000), pkg.Scope)
	got, err := ec.Eval(decl.Value)
	if err == nil {
		t.Fatalf("eval = %v, want an overflow error", got)
	}
	if !errors.Is(err, semantics.ErrArithmeticOverflow) {
		t.Fatalf("eval: %v, want an overflow error", err)
	}
	if got.Kind == ValConst && got.Const.Kind == semantics.ValBool && !got.Const.Bool {
		t.Error("eval answered false — an overflowing evaluation is no verdict, never a violation")
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
