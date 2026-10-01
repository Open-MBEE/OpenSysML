package semantics

import (
	"errors"
	"math"
	"testing"
)

func intVal(i int64) Value { return Value{Kind: ValInt, Int: i} }
func bigVal(decimal string) Value {
	v, ok := ParseInteger(decimal)
	if !ok {
		panic("not an Integer: " + decimal)
	}
	return v
}

func realVal(f float64) Value {
	return Value{Kind: ValReal, Real: f}
}

func TestPowValues(t *testing.T) {
	cases := []struct {
		name string
		l, r Value
		want Value
	}{
		{"integer base and exponent stay Integer", intVal(2), intVal(10), intVal(1024)},
		{"zero exponent is one", intVal(7), intVal(0), intVal(1)},
		{"negative integer base, even exponent", intVal(-3), intVal(2), intVal(9)},
		{"negative integer base, odd exponent", intVal(-3), intVal(3), intVal(-27)},
		{"negative exponent gives a Real", intVal(2), intVal(-1), realVal(0.5)},
		{"real base and exponent", realVal(2), realVal(0.5), realVal(math.Sqrt2)},
		{"mixed operands widen to Real", intVal(9), realVal(0.5), realVal(3)},
		{"negative base with whole real exponent", realVal(-2), realVal(3), realVal(-8)},
		{"zero to a positive power", realVal(0), realVal(2), realVal(0)},
		{"integer result beyond int64 is exact", intVal(2), intVal(70), bigVal("1180591620717411303424")},
		{"big base", bigVal("-36893488147419103232"), intVal(3), bigVal("-50216813883093446110686315385661331328818843555712276103168")},
		{"one to a big power", intVal(1), bigVal("100000000000000000000"), intVal(1)},
		{"minus one to an odd big power", intVal(-1), bigVal("100000000000000000001"), intVal(-1)},
		{"zero to a big power", intVal(0), bigVal("100000000000000000000"), intVal(0)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Pow(tc.l, tc.r, DefaultMaxIntegerBits)
			if err != nil {
				t.Fatalf("Pow(%+v, %+v) = error %v", tc.l, tc.r, err)
			}
			if !got.Equal(tc.want) {
				t.Fatalf("Pow(%+v, %+v) = %+v, want %+v", tc.l, tc.r, got, tc.want)
			}
		})
	}
}

func TestPowErrors(t *testing.T) {
	cases := []struct {
		name string
		l, r Value
		want error
	}{
		{"zero to a negative power", realVal(0), realVal(-1), ErrArithmeticDomain},
		{"integer zero to a negative power", intVal(0), intVal(-1), ErrArithmeticDomain},
		{"negative base, fractional exponent", realVal(-2), realVal(0.5), ErrArithmeticDomain},
		{"non-numeric operand", Value{Kind: ValBool, Bool: true}, intVal(2), ErrArithmeticDomain},
		{"infinity operand", Value{Kind: ValInfinity}, intVal(2), ErrArithmeticDomain},
		{"integer beyond the size budget", intVal(2), intVal(DefaultMaxIntegerBits), ErrIntegerSizeLimit},
		{"integer to a big power", intVal(2), bigVal("100000000000000000000"), ErrIntegerSizeLimit},
		{"real overflow", realVal(1e300), realVal(2), ErrArithmeticOverflow},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Pow(tc.l, tc.r, DefaultMaxIntegerBits)
			if !errors.Is(err, tc.want) {
				t.Fatalf("Pow(%+v, %+v) = %+v, %v; want error %v", tc.l, tc.r, got, err, tc.want)
			}
		})
	}
}

// The folder declines rather than folding to a NaN, an infinity or a wrapped
// integer, so the unsupported case is reported where the expression is evaluated.
func TestFoldExponentiation(t *testing.T) {
	folded := map[string]Value{
		"2 ** 10":                  intVal(1024),
		"2 ^ 3":                    intVal(8),
		"2.0 ** 0.5":               realVal(math.Sqrt2),
		"9 ** 0.5":                 realVal(3),
		"2 ** -2":                  realVal(0.25),
		"9223372036854775807 ** 2": bigVal("85070591730234615847396907784232501249"),
	}
	for src, want := range folded {
		v, ok := evalExpr(t, src)
		if !ok || !v.Equal(want) {
			t.Errorf("%q = %+v ok=%v, want %+v", src, v, ok, want)
		}
	}

	declined := []string{
		"0.0 ** -1.0",
		"(0 - 2.0) ** 0.5",
		"2 ** 1048576",
	}
	for _, src := range declined {
		if v, ok := evalExpr(t, src); ok {
			t.Errorf("%q folded to %+v, want declined", src, v)
		}
	}
}
