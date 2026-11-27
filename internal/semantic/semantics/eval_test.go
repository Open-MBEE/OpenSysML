package semantics

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// evalExpr parses src as a standalone expression and evaluates it.
func evalExpr(t *testing.T, src string) (Value, bool) {
	t.Helper()
	p := parser.New(source.New("<e>", []byte(src)))
	expr := p.ParseExpression()
	if expr == nil {
		t.Fatalf("failed to parse expression %q", src)
	}
	m := NewModel(nil)
	return m.Eval(expr)
}

func TestEvalIntLiteralsAndArithmetic(t *testing.T) {
	cases := map[string]int64{
		"42":          42,
		"1 + 2":       3,
		"2 * 3 + 4":   10,
		"10 - 3":      7,
		"10 % 3":      1,
		"-5":          -5,
		"2 * (3 + 4)": 14,
	}
	for src, want := range cases {
		v, ok := evalExpr(t, src)
		if !ok || v.Kind != ValInt || v.Int != want {
			t.Fatalf("%q = %+v ok=%v, want int %d", src, v, ok, want)
		}
	}
}

// ParseReal reads every representable decimal notation, a zero however
// written and keeping its sign, and reports a magnitude float64 cannot hold
// instead of rounding it to 0 or Inf.
func TestParseReal(t *testing.T) {
	for text, want := range map[string]float64{
		"1.5":      1.5,
		"7":        7,
		"+7":       7,
		"7.":       7,
		".5":       0.5,
		"-2e3":     -2000,
		"2E+3":     2000,
		"1e-3":     0.001,
		"0.0":      0,
		"-0.000":   math.Copysign(0, -1),
		"0e-400":   0,
		"4.9e-324": math.SmallestNonzeroFloat64,
		"1e-320":   1e-320,
		"1e308":    1e308,
	} {
		got, err := ParseReal(text)
		if err != nil || got != want || math.Signbit(got) != math.Signbit(want) {
			t.Errorf("ParseReal(%q) = (%v, %v), want %v", text, got, err, want)
		}
	}
	for _, text := range []string{"1e400", "-1e400", "1e-400", "-2e-324", "0." + strings.Repeat("0", 330) + "1"} {
		if got, err := ParseReal(text); !errors.Is(err, ErrArithmeticOverflow) {
			t.Errorf("ParseReal(%q) = (%v, %v), want %v", text, got, err, ErrArithmeticOverflow)
		}
	}
	if v, ok := evalExpr(t, "1e-400"); !ok || v.Kind != ValRational || v.FormatRational() != "1e-400" {
		t.Errorf("1e-400 folded to %+v ok=%v, want the exact Rational", v, ok)
	}
}

// ParseReal reads decimal notation only: what strconv.ParseFloat admits beyond
// it (NaN, infinities, hexadecimal floats, underscores, blanks) is not a Real.
func TestParseRealRejectsNonDecimalNotation(t *testing.T) {
	for _, text := range []string{
		"NaN", "nan", "Inf", "-Inf", "+Infinity", "infinity",
		"0x1p-2", "0X1.8P3", "-0x10", "0x1p99999",
		"1_000.5", " 1.5", "1.5 ", "", "+", "-", ".", "e5", "1e", "1e+", "1.5e2.0", "1,5", "abc", "1.5f",
	} {
		if got, err := ParseReal(text); !errors.Is(err, ErrRealNotation) {
			t.Errorf("ParseReal(%q) = (%v, %v), want %v", text, got, err, ErrRealNotation)
		}
	}
}

// An Integer quotient is the exact Rational IntegerFunctions::'/' declares.
func TestEvalRealArithmetic(t *testing.T) {
	v, ok := evalExpr(t, "10 / 4")
	if !ok || v.Kind != ValRational || v.FormatRational() != "2.5" {
		t.Fatalf("10/4 = %+v ok=%v, want the Rational 5/2", v, ok)
	}
}

// A quotient of operands beyond 2^53 folds exactly, a whole one staying a
// Rational, the same answer the runtime computes.
func TestEvalFoldsExactQuotientBeyondFloatRange(t *testing.T) {
	v, ok := evalExpr(t, "9007199254740993 / 3")
	if !ok || v.Kind != ValRational || v.FormatRational() != "3002399751580331.0" {
		t.Fatalf("9007199254740993/3 = %+v ok=%v, want the Rational 3002399751580331", v, ok)
	}
}

func TestEvalBooleanLogic(t *testing.T) {
	cases := map[string]bool{
		"true and false": false,
		"true or false":  true,
		"not true":       false,
		"1 < 2":          true,
		"3 <= 3":         true,
		"2 == 2":         true,
		"2 != 3":         true,
		"5 > 10":         false,
	}
	for src, want := range cases {
		v, ok := evalExpr(t, src)
		if !ok || v.Kind != ValBool || v.Bool != want {
			t.Fatalf("%q = %+v ok=%v, want bool %v", src, v, ok, want)
		}
	}
}

func TestEvalConditional(t *testing.T) {
	v, ok := evalExpr(t, "if 1 < 2 ? 10 else 20")
	if !ok || v.Kind != ValInt || v.Int != 10 {
		t.Fatalf("conditional = %+v ok=%v, want 10", v, ok)
	}
}

func TestEvalInfinityLiteral(t *testing.T) {
	m := NewModel(nil)
	v, ok := m.Eval(&ast.LiteralInfinity{})
	if !ok || v.Kind != ValInfinity {
		t.Fatalf("infinity = %+v ok=%v", v, ok)
	}
}

func TestEvalNonEvaluableReference(t *testing.T) {
	// A feature reference is not a model-level constant.
	if _, ok := evalExpr(t, "x + 1"); ok {
		t.Fatalf("expected feature reference to be non-evaluable")
	}
}

func TestEvalDivByZeroNotEvaluable(t *testing.T) {
	if _, ok := evalExpr(t, "1 / 0"); ok {
		t.Fatalf("division by zero should be non-evaluable")
	}
}

// WholeNumber converts a whole-valued finite real exactly and refuses the rest.
func TestWholeNumberRefusesFractionsAndNonFinite(t *testing.T) {
	if n, ok := (Value{Kind: ValReal, Real: 2.0}).WholeNumber(); !ok || n != 2 {
		t.Errorf("2.0 = %d, %v; want 2, true", n, ok)
	}
	for _, r := range []float64{1.5, math.NaN(), math.Inf(1), math.Inf(-1), 1e19, -1e19} {
		if n, ok := (Value{Kind: ValReal, Real: r}).WholeNumber(); ok {
			t.Errorf("%v: WholeNumber = %d, true; want false", r, n)
		}
	}
	if _, ok := (Value{Kind: ValBool, Bool: true}).WholeNumber(); ok {
		t.Error("a Boolean is no whole number")
	}
}

// TestEvalConstUnnestedAgreesWithEvalConst: the evaluation that keeps a stack of its own
// gives the value, or the refusal, the recursive one gives.
func TestEvalConstUnnestedAgreesWithEvalConst(t *testing.T) {
	for _, src := range []string{
		"-5", "+2", "not true", "not 1", "-true",
		"1 + 2 * 3 - 4", "2 ** 3 ** 2", "(1 + 2) * (3 - 4) / 5", "7 % 0", "1 / 0 + 1",
		"1 < 2 and 3 >= 3", "true xor false implies 1 == 1.0", "1 and true", "1 + true",
		"if true ? 1 else 2", "if false ? 1 else x", "if true ? 1 else x", "if 1 ? 1 else 2",
		"if x ? 1 else 2", "if 1 < 2 ? (if false ? 3 else 4) + 1 else 5",
		"(if true ? 1 else 2) + (if false ? 1 else 2)", "not (if true ? false else 1)",
		"1 ?? 2", "1 + x", "x + 1", "-(x)", "(1, 2)", "1..3", "1 + (2, 3)",
	} {
		expr := parser.New(source.New("<e>", []byte(src))).ParseExpression()
		if expr == nil {
			t.Fatalf("failed to parse expression %q", src)
		}
		want, wantOK := evalConst(expr, DefaultMaxIntegerBits)
		e, isOperator := expr.(*ast.OperatorExpr)
		if !isOperator {
			continue
		}
		got := evalConstUnnested(expr, DefaultMaxIntegerBits)[e]
		if got.ok != wantOK || !reflect.DeepEqual(got.value, want) {
			t.Errorf("%q = %+v ok=%v, want %+v ok=%v", src, got.value, got.ok, want, wantOK)
		}
	}
}

// TestEvalConstUnnestedFollowsNestingOfAnyDepth: a chain far past what a filter condition
// may nest folds to its value.
func TestEvalConstUnnestedFollowsNestingOfAnyDepth(t *testing.T) {
	const n = 100_000
	for src, want := range map[string]int64{
		"1" + strings.Repeat(" + 1", n):                                 n + 1,
		strings.Repeat("if false ? 0 else ", n) + "7":                   7,
		strings.Repeat("-(", n) + "3" + strings.Repeat(")", n):          3,
		strings.Repeat("(1 + ", n/10) + "1" + strings.Repeat(")", n/10): n/10 + 1,
	} {
		expr := parser.New(source.New("<e>", []byte(src))).ParseExpression()
		if expr == nil {
			t.Fatalf("failed to parse a chain of %d", n)
		}
		got := evalConstUnnested(expr, DefaultMaxIntegerBits)[expr.(*ast.OperatorExpr)]
		if !got.ok || got.value.Kind != ValInt || got.value.Int != want {
			t.Errorf("%.20q… = %+v ok=%v, want int %d", src, got.value, got.ok, want)
		}
	}
}
