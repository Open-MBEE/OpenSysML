package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// quotientModel divides parameters, so the quotient is the runtime's and not
// the constant folder's.
const quotientModel = `calc def quotient { in a : Integer; in b : Integer; return : Rational = a / b; }`

// TestWholeNumberQuotientIsRational: the quotient of two whole numbers is the exact
// Rational for every sign combination, whole or not, never truncated.
func TestWholeNumberQuotientIsRational(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, quotientModel)
	ctx := NewContext(typedModel(model, resolver), 1000)
	quotient := resolveSymbol(t, root, "quotient")

	cases := []struct {
		a, b int64
		want string
	}{
		{5, 2, "2.5"},
		{9, 3, "3"},
		{-5, -1, "5"},
		{-7, 2, "-3.5"},
		{7, -2, "-3.5"},
		{10, 4, "2.5"},
		{7, 2, "3.5"},
		{0, -3, "0"},
	}
	for _, tc := range cases {
		got, err := ctx.InvokeCalc(quotient, []Value{constInt(tc.a), constInt(tc.b)}, root)
		if err != nil {
			t.Fatalf("%d / %d: %v", tc.a, tc.b, err)
		}
		if want := constRat(tc.want).Const; got.Const.Kind != semantics.ValRational || semantics.CompareRat(got.Const, want) != 0 {
			t.Errorf("%d / %d = %s, want the Rational %s", tc.a, tc.b, FormatValue(got), tc.want)
		}
	}
}

// TestQuotientBeyondFloatExactRangeIsExact: operands above 2^53 divide as exact
// rationals, not through rounded float64 operands or a rounded quotient.
func TestQuotientBeyondFloatExactRangeIsExact(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, quotientModel)
	ctx := NewContext(typedModel(model, resolver), 1000)
	quotient := resolveSymbol(t, root, "quotient")

	cases := []struct {
		a, b int64
		want string
	}{
		{9007199254740993, 3, "3002399751580331"},         // 2^53+1: rounding a first loses the exact thirds
		{-9007199254740993, 3, "-3002399751580331"},       // the sign does not change the quotient
		{9007199254740993, 1, "9007199254740993"},         // 2^53+1 itself is not a float64
		{9007199254740993, 2, "9007199254740993/2"},       // no half is rounded
		{-9223372036854775808, -1, "9223372036854775808"}, // MinInt64 / -1 is the Rational 2^63
	}
	for _, tc := range cases {
		got, err := ctx.InvokeCalc(quotient, []Value{constInt(tc.a), constInt(tc.b)}, root)
		if err != nil {
			t.Fatalf("%d / %d: %v", tc.a, tc.b, err)
		}
		if want := constRat(tc.want).Const; got.Const.Kind != semantics.ValRational || semantics.CompareRat(got.Const, want) != 0 {
			t.Errorf("%d / %d = %s, want the Rational %s", tc.a, tc.b, FormatValue(got), tc.want)
		}
	}
}

// TestWholeNumberDivisionByZeroIsReported: a Rational quotient does not change
// what a zero divisor is — an error, not an infinity.
func TestWholeNumberDivisionByZeroIsReported(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, quotientModel)
	ctx := NewContext(typedModel(model, resolver), 1000)
	quotient := resolveSymbol(t, root, "quotient")

	got, err := ctx.InvokeCalc(quotient, []Value{constInt(7), constInt(0)}, root)
	if !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("7 / 0 = %+v, %v; want ErrDivisionByZero", got, err)
	}
}

// TestRemainderStaysInteger: `%` keeps the Integer truncating-toward-zero
// contract the quotient no longer has.
func TestRemainderStaysInteger(t *testing.T) {
	const remainderModel = `calc def remainder { in a : Integer; in b : Integer; return : Integer = a % b; }`
	model, resolver, root := parseAndBuildModel(t, remainderModel)
	ctx := NewContext(typedModel(model, resolver), 1000)
	remainder := resolveSymbol(t, root, "remainder")

	cases := []struct {
		a, b, want int64
	}{
		{7, 2, 1},
		{-7, 2, -1},
		{7, -2, 1},
		{-7, -2, -1},
	}
	for _, tc := range cases {
		got, err := ctx.InvokeCalc(remainder, []Value{constInt(tc.a), constInt(tc.b)}, root)
		if err != nil {
			t.Fatalf("%d %% %d: %v", tc.a, tc.b, err)
		}
		if got.Const.Kind != semantics.ValInt || got.Const.Int != tc.want {
			t.Errorf("%d %% %d = %+v, want the Integer %d", tc.a, tc.b, got.Const, tc.want)
		}
	}
}
