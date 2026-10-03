package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessFunctionValues covers the failure modes of function
// values chosen at run time and of the closures a body declares: a runaway
// closure, a function given where a String or number is needed, and a closure
// failing over what it captured.
func TestRuntimeRobustnessFunctionValues(t *testing.T) {
	t.Run("runaway_closure_spends_the_step_budget", testFunctionValueRunawayClosure)
	t.Run("function_has_no_string_notation", testFunctionValueToString)
	t.Run("function_is_not_a_number", testFunctionValueArithmetic)
	t.Run("closure_fails_over_its_capture", testFunctionValueClosureCaptureFails)
}

const functionValueModel = `
	package test {
		private import ScalarValues::*;
		calc def Sq { in v : Real; return : Real = v * v; }
		calc def Half { in v : Real; return : Real = v / 2.0; }
		calc sqU : Sq;
		calc halfU : Half;
		calc def Apply { in calc f { in v : Real; return : Real; } in a : Real; return : Real = f(a); }
		calc def Pick { in b : Boolean; return r = if b ? sqU else halfU; }
		calc def Runaway { in k : Real; calc s { in v : Real; attribute x : Real = v; while true { assign x := x + k; } return : Real = x; } return : Real = Apply(s, 1.0); }
		calc def Text { in b : Boolean; return : String = BaseFunctions::ToString(Pick(b)); }
		calc def Plus { in b : Boolean; return r = Pick(b) + 1; }
		calc def Cut { in s : String; calc f { in n : Integer; return : String = StringFunctions::Substring(s, 1, n); } return : String = f(5); }
	}
`

// invokeFunctionValueCalc runs the named calc of functionValueModel on args.
func invokeFunctionValueCalc(t *testing.T, name string, args ...Value) (Value, error) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, functionValueModel))
	root := idx.DocumentRoot("<test>")
	sym := findSymbolByName(root, name, ast.DefCalc)
	if sym == nil {
		t.Fatalf("calc %s not found", name)
	}
	return ctx.InvokeCalc(sym, args, root)
}

// A closure that never returns ends with the step budget's error, not a hang.
func testFunctionValueRunawayClosure(t *testing.T) {
	got, err := invokeFunctionValueCalc(t, "Runaway", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 1}})
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Fatalf("Runaway(1.0) = %+v, %v; want ErrStepLimitExceeded", got, err)
	}
}

// BaseFunctions::ToString of a function value is a type mismatch: a function has no String notation.
func testFunctionValueToString(t *testing.T) {
	got, err := invokeFunctionValueCalc(t, "Text", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: true}})
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("Text(true) = %+v, %v; want ErrTypeMismatch", got, err)
	}
}

// A function value chosen at run time is no operand of arithmetic.
func testFunctionValueArithmetic(t *testing.T) {
	got, err := invokeFunctionValueCalc(t, "Plus", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: false}})
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("Plus(false) = %+v, %v; want ErrTypeMismatch", got, err)
	}
}

// A closure reading a captured String past its end fails with the library's typed error.
func testFunctionValueClosureCaptureFails(t *testing.T) {
	got, err := invokeFunctionValueCalc(t, "Cut", NewStringValue("hi"))
	if !errors.Is(err, ErrIndexOutOfRange) {
		t.Fatalf("Cut(\"hi\") = %+v, %v; want ErrIndexOutOfRange", got, err)
	}
}
