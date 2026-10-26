package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// TestRuntimeRobustnessOperatorEmptyOperand covers a binary operator given an
// empty operand: `+` and `-` admit an empty second one as the library's `y`
// [0..1] does (KerML 1.1 §8.4.4.9.6), every other operand is [1] and refused.
func TestRuntimeRobustnessOperatorEmptyOperand(t *testing.T) {
	t.Run("empty_second_operand_is_the_unary_sign", testEmptySecondOperandIsTheUnarySign)
	t.Run("empty_required_operand_is_refused", testEmptyRequiredOperandIsRefused)
}

const emptyOperandModel = `
	package test {
		private import ScalarValues::*;
		private import NumericalFunctions::*;
		private import SequenceFunctions::*;
		private import ISQ::*;
		private import SI::*;
		attribute none : Integer[0..1];
		attribute xs : Integer[*] = (1, 2);
		attribute side : LengthValue = 2 [m];
	}
`

func emptyOperandContext(t *testing.T) (*Context, *symbols.Scope) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, emptyOperandModel))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	return ctx, pkg.Scope
}

// testEmptySecondOperandIsTheUnarySign: the operator form answers what the
// function form answers, whatever spelling of no value the second operand is.
func testEmptySecondOperandIsTheUnarySign(t *testing.T) {
	ctx, scope := emptyOperandContext(t)
	for _, tt := range []struct{ operator, function string }{
		{"1 + null", "'+'(1, null)"},
		{"3 - null", "'-'(3, null)"},
		{"3 - ()", "'-'(3)"},
		{"3 - none", "'-'(3)"},
		{"3 - xs->excluding(1)->excluding(2)", "'-'(3)"},
		{"1.5 - null", "'-'(1.5)"},
		{"side - null", "-side"},
	} {
		got, err := evalIn(t, ctx, scope, tt.operator)
		if err != nil {
			t.Errorf("%s: %v", tt.operator, err)
			continue
		}
		want, err := evalIn(t, ctx, scope, tt.function)
		if err != nil {
			t.Fatalf("%s: %v", tt.function, err)
		}
		if FormatValue(got) != FormatValue(want) {
			t.Errorf("%s = %s, want %s as %s", tt.operator, FormatValue(got), FormatValue(want), tt.function)
		}
	}
}

// testEmptyRequiredOperandIsRefused: a [1] parameter given no value is refused
// as a typed error, never evaluated as if the operand were omitted.
func testEmptyRequiredOperandIsRefused(t *testing.T) {
	ctx, scope := emptyOperandContext(t)
	for _, expr := range []string{
		"null + 1", "none - 3", "2 * null", "2 / ()", "7 % null", "2 ** none", "2 ^ null",
		"1 < null", "null >= 1", "side * null", "\"a\" + null",
	} {
		got, err := evalIn(t, ctx, scope, expr)
		var opErr *OperandTypeError
		if !errors.As(err, &opErr) || !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("%s = (%s, %v), want an OperandTypeError wrapping %v", expr, FormatValue(got), err, ErrTypeMismatch)
		}
	}
	if _, err := evalIn(t, ctx, scope, "NumericalFunctions::'+'(null, 1)"); !errors.Is(err, ErrMultiplicityViolation) {
		t.Errorf("'+'(null, 1) = %v, want %v naming parameter x", err, ErrMultiplicityViolation)
	}
}
