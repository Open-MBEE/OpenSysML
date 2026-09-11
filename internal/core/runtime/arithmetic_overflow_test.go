package runtime

import (
	"errors"
	"math"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// sumModel and productModel are calcs over parameters, so their arithmetic
// cannot be folded and is evaluated by the runtime.
const (
	sumModel     = `calc def sum { in a : Real; in b : Real; return : Real = a + b; }`
	productModel = `calc def product { in a : Real; in b : Real; return : Real = a * b; }`
)

// TestIntegerArithmeticReportsOverflow: a sum, difference or product outside the
// Integer range is an error at evaluation, not a value wrapped around.
func TestIntegerArithmeticReportsOverflow(t *testing.T) {
	cases := []struct {
		name  string
		model string
		calc  string
		args  []Value
	}{
		{"sum", sumModel, "sum", []Value{constInt(math.MaxInt64), constInt(1)}},
		{"difference", sumModel, "sum", []Value{constInt(math.MinInt64), constInt(-1)}},
		{"product", productModel, "product", []Value{constInt(math.MaxInt64), constInt(2)}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model, resolver, root := parseAndBuildModel(t, tc.model)
			ctx := NewContext(NewModel(model, resolver), 1000)
			got, err := ctx.InvokeCalc(resolveSymbol(t, root, tc.calc), tc.args, root)
			if !errors.Is(err, semantics.ErrArithmeticOverflow) {
				t.Fatalf("%s%v = %+v, %v; want ErrArithmeticOverflow", tc.name, tc.args, got, err)
			}
		})
	}
}

// TestFoldedIntegerArithmeticReportsOverflow: the folder and the runtime agree,
// so a literal sum outside the Integer range is reported the same way.
func TestFoldedIntegerArithmeticReportsOverflow(t *testing.T) {
	model, resolver, _ := parseAndBuildModel(t, sumModel)
	ctx := NewContext(NewModel(model, resolver), 1000)

	for _, src := range []string{
		"9223372036854775807 + 1",
		"-9223372036854775807 - 2",
		"9223372036854775807 * 2",
	} {
		got, err := evalLiteral(t, ctx, src)
		if !errors.Is(err, semantics.ErrArithmeticOverflow) {
			t.Fatalf("%s = %+v, %v; want ErrArithmeticOverflow", src, got, err)
		}
	}
}

// TestLiteralOutsideItsRangeIsReported: a literal no Integer or Real holds is an
// error, not the nearest value or an infinity.
func TestLiteralOutsideItsRangeIsReported(t *testing.T) {
	model, resolver, _ := parseAndBuildModel(t, sumModel)
	ctx := NewContext(NewModel(model, resolver), 1000)

	for _, src := range []string{
		"9223372036854775808",
		"1e400",
		"1e400 + 1.0",
		"1e308 + 1e308",
		"1e200 * 1e200",
	} {
		got, err := evalLiteral(t, ctx, src)
		if !errors.Is(err, semantics.ErrArithmeticOverflow) {
			t.Fatalf("%s = %+v, %v; want ErrArithmeticOverflow", src, got, err)
		}
	}
}

// TestLeastIntegerLiteralIsRead: -9223372036854775808 is an Integer even though
// its magnitude written alone is not.
func TestLeastIntegerLiteralIsRead(t *testing.T) {
	model, resolver, _ := parseAndBuildModel(t, sumModel)
	ctx := NewContext(NewModel(model, resolver), 1000)

	got, err := evalLiteral(t, ctx, "-9223372036854775808")
	if err != nil {
		t.Fatalf("evaluating the least Integer: %v", err)
	}
	if got.Const.Kind != semantics.ValInt || got.Const.Int != math.MinInt64 {
		t.Fatalf("-9223372036854775808 = %+v, want %d", got.Const, int64(math.MinInt64))
	}
}

// TestNegatingTheLeastIntegerIsReported: the least Integer has no negation in
// range. Dividing it by -1 is not an overflow: a quotient is a Rational, so it
// answers the Real the negated magnitude rounds to.
func TestNegatingTheLeastIntegerIsReported(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, sumModel)
	ctx := NewContext(NewModel(model, resolver), 1000)
	sum := resolveSymbol(t, root, "sum")

	got, err := evalLiteral(t, ctx, "-(-9223372036854775808)")
	if !errors.Is(err, semantics.ErrArithmeticOverflow) {
		t.Errorf("-(-9223372036854775808) = %+v, %v; want ErrArithmeticOverflow", got, err)
	}

	got, err = evalLiteral(t, ctx, "-9223372036854775808 / -1")
	if err != nil {
		t.Fatalf("-9223372036854775808 / -1: %v", err)
	}
	if got.Const.Kind != semantics.ValReal || got.Const.Real != -float64(math.MinInt64) {
		t.Errorf("-9223372036854775808 / -1 = %+v, want the Real %v", got.Const, -float64(math.MinInt64))
	}

	args := []Value{constInt(math.MinInt64), constInt(0)}
	if _, err := ctx.InvokeCalc(sum, args, root); err != nil {
		t.Fatalf("the least Integer is a value the runtime carries: %v", err)
	}
}

// TestRealArithmeticReportsNonFiniteResult: a product no Real holds is reported
// rather than answered as an infinity.
func TestRealArithmeticReportsNonFiniteResult(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, productModel)
	ctx := NewContext(NewModel(model, resolver), 1000)
	product := resolveSymbol(t, root, "product")

	args := []Value{constReal(math.MaxFloat64), constReal(2)}
	got, err := ctx.InvokeCalc(product, args, root)
	if !errors.Is(err, semantics.ErrArithmeticOverflow) {
		t.Fatalf("product%v = %+v, %v; want ErrArithmeticOverflow", args, got, err)
	}
}
