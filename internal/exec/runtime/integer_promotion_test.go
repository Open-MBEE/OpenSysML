package runtime

import (
	"errors"
	"math"
	"math/big"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// sumModel and productModel are calcs over parameters, so their arithmetic
// cannot be folded and is evaluated by the runtime.
const (
	sumModel     = `calc def sum { in a : Real; in b : Real; return : Real = a + b; }`
	productModel = `calc def product { in a : Real; in b : Real; return : Real = a * b; }`
)

// bigConst is the Integer the decimal text spells, of any magnitude.
func bigConst(t *testing.T, text string) Value {
	t.Helper()
	n, ok := semantics.ParseInteger(text)
	if !ok {
		t.Fatalf("%q is no Integer", text)
	}
	return Value{Kind: ValConst, Const: n}
}

// wantInteger fails unless got is the Integer the decimal text spells, held as
// int64 exactly when it fits.
func wantInteger(t *testing.T, what string, got Value, want string) {
	t.Helper()
	if got.Kind != ValConst || got.Const.Kind != semantics.ValInt || got.Const.FormatInt() != want {
		t.Fatalf("%s = %s, want %s", what, FormatValue(got), want)
	}
	if _, fits := got.Const.Int64(); fits == got.Const.IsBigInt() {
		t.Fatalf("%s = %s is held as big %v, fitting int64 %v", what, want, got.Const.IsBigInt(), fits)
	}
}

// TestIntegerArithmeticPromotesBeyondInt64: KerML Integers are unbounded, so a
// sum, difference or product past int64 is the exact Integer, never a value
// wrapped around and never an overflow.
func TestIntegerArithmeticPromotesBeyondInt64(t *testing.T) {
	cases := []struct {
		name  string
		model string
		calc  string
		args  []Value
		want  string
	}{
		{"sum", sumModel, "sum", []Value{constInt(math.MaxInt64), constInt(1)}, "9223372036854775808"},
		{"difference", sumModel, "sum", []Value{constInt(math.MinInt64), constInt(-1)}, "-9223372036854775809"},
		{"product", productModel, "product", []Value{constInt(math.MaxInt64), constInt(2)}, "18446744073709551614"},
		{"demoted sum", sumModel, "sum", []Value{bigConst(t, "9223372036854775808"), constInt(-1)}, "9223372036854775807"},
		{"demoted product", productModel, "product", []Value{bigConst(t, "-9223372036854775808"), constInt(1)}, "-9223372036854775808"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model, resolver, root := parseAndBuildModel(t, tc.model)
			ctx := NewContext(typedModel(model, resolver), 1000)
			got, err := ctx.InvokeCalc(resolveSymbol(t, root, tc.calc), tc.args, root)
			if err != nil {
				t.Fatalf("%s%v: %v", tc.name, tc.args, err)
			}
			wantInteger(t, tc.name, got, tc.want)
		})
	}
}

// TestFoldedIntegerArithmeticIsExact: the folder and the runtime agree, so a
// literal sum past int64 folds to the same exact Integer.
func TestFoldedIntegerArithmeticIsExact(t *testing.T) {
	model, resolver, _ := parseAndBuildModel(t, sumModel)
	ctx := NewContext(typedModel(model, resolver), 1000)

	for src, want := range map[string]string{
		"9223372036854775807 + 1":                     "9223372036854775808",
		"-9223372036854775807 - 2":                    "-9223372036854775809",
		"9223372036854775807 * 2":                     "18446744073709551614",
		"2 ** 70":                                     "1180591620717411303424",
		"(2 ** 70) - (2 ** 70) + 1":                   "1",
		"9223372036854775808 - 1":                     "9223372036854775807",
		"(2 ** 70) % 7":                               "2",
		"-(2 ** 70) % 7":                              "-2",
		"99999999999999999999 * 99999999999999999999": "9999999999999999999800000000000000000001",
	} {
		got, err := evalLiteral(t, ctx, src)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		wantInteger(t, src, got, want)
	}
}

// TestLiteralBeyondInt64IsRead: an Integer literal of any magnitude is the
// Integer it spells; a Real literal no float64 holds is still an error rather
// than an infinity.
func TestLiteralBeyondInt64IsRead(t *testing.T) {
	model, resolver, _ := parseAndBuildModel(t, sumModel)
	ctx := NewContext(typedModel(model, resolver), 1000)

	for _, src := range []string{"9223372036854775808", "-9223372036854775809", "123456789012345678901234567890"} {
		got, err := evalLiteral(t, ctx, src)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		wantInteger(t, src, got, src)
	}
	for src, want := range map[string]string{
		"1e400":         "1e400",
		"1e400 + 1.0":   "1e400",
		"1e308 + 1e308": "2e308",
		"1e200 * 1e200": "1e400",
	} {
		got, err := evalLiteral(t, ctx, src)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		exact, _ := semantics.ParseRationalText(want, semantics.DefaultMaxIntegerBits)
		if src == "1e400 + 1.0" {
			exact, _ = semantics.RatArith(ast.OpAdd, exact, semantics.IntValue(1), semantics.DefaultMaxIntegerBits)
		}
		if got.Const.Kind != semantics.ValRational || semantics.CompareRat(got.Const, exact) != 0 {
			t.Fatalf("%s = %s, want the exact Rational", src, FormatValue(got))
		}
	}
}

// TestLeastInt64LiteralIsRead: -9223372036854775808 is held as int64.
func TestLeastInt64LiteralIsRead(t *testing.T) {
	model, resolver, _ := parseAndBuildModel(t, sumModel)
	ctx := NewContext(typedModel(model, resolver), 1000)

	got, err := evalLiteral(t, ctx, "-9223372036854775808")
	if err != nil {
		t.Fatalf("evaluating the least int64: %v", err)
	}
	if got.Const.Kind != semantics.ValInt || got.Const.Int != math.MinInt64 || got.Const.IsBigInt() {
		t.Fatalf("-9223372036854775808 = %+v, want %d", got.Const, int64(math.MinInt64))
	}
}

// TestNegatingTheLeastInt64: the negation of the least int64 is 2^63, an
// Integer beyond int64. Dividing it by -1 is a quotient, so it answers the
// Rational 2^63.
func TestNegatingTheLeastInt64(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, sumModel)
	ctx := NewContext(typedModel(model, resolver), 1000)
	sum := resolveSymbol(t, root, "sum")

	got, err := evalLiteral(t, ctx, "-(-9223372036854775808)")
	if err != nil {
		t.Fatalf("-(-9223372036854775808): %v", err)
	}
	wantInteger(t, "-(-9223372036854775808)", got, "9223372036854775808")

	got, err = evalLiteral(t, ctx, "-9223372036854775808 / -1")
	if err != nil {
		t.Fatalf("-9223372036854775808 / -1: %v", err)
	}
	if got.Const.Kind != semantics.ValRational || got.Const.FormatRational() != "9223372036854775808.0" {
		t.Errorf("-9223372036854775808 / -1 = %s, want the Rational 2^63", FormatValue(got))
	}

	args := []Value{constInt(math.MinInt64), constInt(0)}
	if _, err := ctx.InvokeCalc(sum, args, root); err != nil {
		t.Fatalf("the least int64 is a value the runtime carries: %v", err)
	}
}

// TestIntegerEqualityIsRepresentationIndependent: Integers numerically equal
// are equal, key alike and order alike whether held as int64 or big.
func TestIntegerEqualityIsRepresentationIndependent(t *testing.T) {
	model, resolver, _ := parseAndBuildModel(t, sumModel)
	ctx := NewContext(typedModel(model, resolver), 1000)
	for src, want := range map[string]bool{
		"9223372036854775807 + 1 == 9223372036854775808":           true,
		"(2 ** 70) / (2 ** 69) == 2":                               true,
		"(2 ** 64) - (2 ** 64) == 0":                               true,
		"2 ** 70 > 2 ** 69":                                        true,
		"-(2 ** 70) < -9223372036854775808":                        true,
		"9223372036854775808 == 9223372036854775807":               false,
		"9223372036854775808 == 9223372036854775808.0":             true,
		"9223372036854775807 == 9223372036854775808.0":             false,
		"2 ** 70 != 1180591620717411303424":                        false,
		"(9223372036854775808, 1) == (9223372036854775807 + 1, 1)": true,
	} {
		got, err := evalLiteral(t, ctx, src)
		if err != nil || got.Kind != ValConst || got.Const.Kind != semantics.ValBool || got.Const.Bool != want {
			t.Errorf("%s = %s, %v; want %v", src, FormatValue(got), err, want)
		}
	}
	small := constInt(5)
	demoted := Value{Kind: ValConst, Const: semantics.BigIntValue(big.NewInt(5))}
	if demoted.Const.IsBigInt() || !valueEqual(small, demoted) || valueKeyFunc(small) != valueKeyFunc(demoted) {
		t.Errorf("5 and a big 5 differ: %+v, %+v", valueKeyFunc(small), valueKeyFunc(demoted))
	}
	set := NewSet()
	set.Add(bigConst(t, "9223372036854775808"))
	if sum, err := evalLiteral(t, ctx, "9223372036854775807 + 1"); err != nil || !set.Contains(sum) {
		t.Errorf("a set of 2^63 does not hold the computed 2^63: %v", err)
	}
}

// TestRealArithmeticReportsNonFiniteResult: a product no Real holds is reported
// rather than answered as an infinity.
func TestRealArithmeticReportsNonFiniteResult(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, productModel)
	ctx := NewContext(typedModel(model, resolver), 1000)
	product := resolveSymbol(t, root, "product")

	args := []Value{constReal(math.MaxFloat64), constReal(2)}
	got, err := ctx.InvokeCalc(product, args, root)
	if !errors.Is(err, semantics.ErrArithmeticOverflow) {
		t.Fatalf("product%v = %+v, %v; want ErrArithmeticOverflow", args, got, err)
	}
}
