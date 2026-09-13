package runtime

import (
	"errors"
	"math"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// powerModel is a calc over parameters, so its `**` cannot be folded and is
// evaluated by the runtime.
const powerModel = `calc def power { in b : Real; in e : Real; return : Real = b ** e; }`

// evalLiteral evaluates src as a standalone expression, which the folder handles
// because every operand is a literal.
func evalLiteral(t *testing.T, ctx *Context, src string) (Value, error) {
	t.Helper()
	expr := parser.New(source.New("<e>", []byte(src))).ParseExpression()
	if expr == nil {
		t.Fatalf("failed to parse %q", src)
	}
	return ctx.Eval(expr)
}

// TestExponentiationRuntimeMatchesFolding evaluates each case twice — once over
// literals, which the constant folder computes, and once over calc parameters,
// which the runtime computes — and requires the same value from both paths.
func TestExponentiationRuntimeMatchesFolding(t *testing.T) {
	cases := []struct {
		src  string
		args []Value
		want semantics.Value
	}{
		{"2 ** 10", []Value{constInt(2), constInt(10)}, semantics.Value{Kind: semantics.ValInt, Int: 1024}},
		{"2 ^ 3", []Value{constInt(2), constInt(3)}, semantics.Value{Kind: semantics.ValInt, Int: 8}},
		{"7 ** 0", []Value{constInt(7), constInt(0)}, semantics.Value{Kind: semantics.ValInt, Int: 1}},
		{"2 ** -1", []Value{constInt(2), constInt(-1)}, semantics.Value{Kind: semantics.ValReal, Real: 0.5}},
		{"2.0 ** 0.5", []Value{constReal(2), constReal(0.5)}, semantics.Value{Kind: semantics.ValReal, Real: math.Sqrt2}},
		{"9 ** 0.5", []Value{constInt(9), constReal(0.5)}, semantics.Value{Kind: semantics.ValReal, Real: 3}},
		{"2.5 ** 2", []Value{constReal(2.5), constInt(2)}, semantics.Value{Kind: semantics.ValReal, Real: 6.25}},
	}

	model, resolver, root := parseAndBuildModel(t, powerModel)
	ctx := NewContext(NewModel(model, resolver), 1000)
	power := resolveSymbol(t, root, "power")

	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			folded, err := evalLiteral(t, ctx, tc.src)
			if err != nil {
				t.Fatalf("folding %q: %v", tc.src, err)
			}
			if folded.Kind != ValConst || folded.Const != tc.want {
				t.Fatalf("folded %q = %+v, want %+v", tc.src, folded, tc.want)
			}

			evaluated, err := ctx.InvokeCalc(power, tc.args, root)
			if err != nil {
				t.Fatalf("evaluating %q: %v", tc.src, err)
			}
			if evaluated.Const != folded.Const {
				t.Fatalf("evaluated %q = %+v, folded = %+v", tc.src, evaluated.Const, folded.Const)
			}
		})
	}
}

// The folder declines the cases with no value to fold, so the runtime is where
// they are reported.
func TestExponentiationErrorsAtEvaluation(t *testing.T) {
	cases := []struct {
		name string
		args []Value
		want error
	}{
		{"zero to a negative power", []Value{constReal(0), constReal(-1)}, semantics.ErrArithmeticDomain},
		{"negative base, fractional exponent", []Value{constReal(-2), constReal(0.5)}, semantics.ErrArithmeticDomain},
		{"integer overflow", []Value{constInt(math.MaxInt64), constInt(2)}, semantics.ErrArithmeticOverflow},
		{"real overflow", []Value{constReal(1e300), constReal(2)}, semantics.ErrArithmeticOverflow},
		{"boolean operand", []Value{boolValue(true), constInt(2)}, semantics.ErrArithmeticDomain},
		{"string operand", []Value{NewStringValue("2"), constInt(2)}, ErrTypeMismatch},
	}

	model, resolver, root := parseAndBuildModel(t, powerModel)
	ctx := NewContext(NewModel(model, resolver), 1000)
	power := resolveSymbol(t, root, "power")

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ctx.InvokeCalc(power, tc.args, root)
			if !errors.Is(err, tc.want) {
				t.Fatalf("power%v = %+v, %v; want error %v", tc.args, got, err, tc.want)
			}
		})
	}
}
