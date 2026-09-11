package semantics

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
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
	if v, ok := evalExpr(t, "1e-400"); ok {
		t.Errorf("1e-400 folded to %+v, want not evaluable", v)
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

func TestEvalRealArithmetic(t *testing.T) {
	v, ok := evalExpr(t, "10 / 4")
	if !ok || v.Kind != ValReal || v.Real != 2.5 {
		t.Fatalf("10/4 = %+v ok=%v, want real 2.5", v, ok)
	}
}

// A quotient of operands beyond 2^53 folds as the exact ratio rounded once,
// the same answer the runtime computes.
func TestEvalFoldsExactQuotientBeyondFloatRange(t *testing.T) {
	v, ok := evalExpr(t, "9007199254740993 / 3")
	if !ok || v.Kind != ValReal || v.Real != 3002399751580331 {
		t.Fatalf("9007199254740993/3 = %+v ok=%v, want real 3002399751580331", v, ok)
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
