package runtime

import (
	"errors"
	"strings"
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
	t.Run("function_value_call_of_a_non_function", testFunctionValueCallOfANonFunction)
	t.Run("function_value_bound_to_a_non_function", testFunctionValueBoundToANonFunction)
	t.Run("function_value_arity_mismatch", testFunctionValueArityMismatch)
	t.Run("function_value_unknown_named_argument", testFunctionValueUnknownNamedArgument)
	t.Run("function_value_unbound_calc_parameter", testFunctionValueUnboundCalcParameter)
	t.Run("function_value_of_a_wrong_typed_calc", testFunctionValueOfAWrongTypedCalc)
	t.Run("function_value_of_a_built_in", testFunctionValueOfABuiltIn)
	t.Run("function_value_applied_to_itself_forever", testFunctionValueAppliedToItselfForever)
	t.Run("function_value_inherited_body_outside_the_closure", testFunctionValueInheritedBodyOutsideTheClosure)
	t.Run("function_value_nested_calc_outside_its_run", testFunctionValueNestedCalcOutsideItsRun)
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

// testFunctionValueCallOfANonFunction: a scalar passed where a calc-typed parameter
// is declared is refused when it is bound, before the body calls it.
func testFunctionValueCallOfANonFunction(t *testing.T) {
	err := invokeCalcExpecting(t, `package test {`+functionValueFixture+`}`, "test::Fn(3.0, 3.0)")
	if !errors.Is(err, errNotAFunction) || !strings.Contains(err.Error(), `parameter "f"`) {
		t.Fatalf("error = %v, want errNotAFunction naming f", err)
	}
}

// testFunctionValueBoundToANonFunction: a named argument binding a calc-typed
// parameter to an object is refused the same way.
func testFunctionValueBoundToANonFunction(t *testing.T) {
	src := `package test {` + functionValueFixture + `
		part def Box;
		part box : Box;
	}`
	err := invokeCalcExpecting(t, src, "test::Fn(a = 3.0, f = test::box)")
	if !errors.Is(err, errNotAFunction) {
		t.Fatalf("error = %v, want errNotAFunction", err)
	}
}

// testFunctionValueArityMismatch: applying a function value to more arguments
// than its calc declares is a calc arity error naming the calc the value is of.
func testFunctionValueArityMismatch(t *testing.T) {
	src := `package test {` + functionValueFixture + `
		calc def Two { in calc f { in v : Real; return : Real; } return : Real = f(1.0, 2.0); }
	}`
	err := invokeCalcExpecting(t, src, "test::Two(test::Sq)")
	if !errors.Is(err, errCalcArity) || !strings.Contains(err.Error(), "test::Sq") {
		t.Fatalf("error = %v, want errCalcArity for test::Sq", err)
	}
}

// testFunctionValueUnknownNamedArgument: a named argument the applied calc does
// not declare is reported against that calc, not the parameter it was passed through.
func testFunctionValueUnknownNamedArgument(t *testing.T) {
	src := `package test {` + functionValueFixture + `
		calc def Named { in calc f { in v : Real; return : Real; } return : Real = f(w = 1.0); }
	}`
	err := invokeCalcExpecting(t, src, "test::Named(test::Sq)")
	if !errors.Is(err, errUnknownParameter) || !strings.Contains(err.Error(), "test::Sq") {
		t.Fatalf("error = %v, want errUnknownParameter for test::Sq", err)
	}
}

// testFunctionValueUnboundCalcParameter: a calc-typed parameter no argument binds
// is reported as unbound when the calc is invoked, and applying a function value
// to fewer arguments than its calc needs is reported the same way.
func testFunctionValueUnboundCalcParameter(t *testing.T) {
	src := `package test {` + functionValueFixture + `
		calc def Partial { in calc f { in x : Real; in y : Real[1]; return : Real; } return : Real = f(1.0); }
	}`
	err := invokeCalcExpecting(t, src, "test::Fn(a = 3.0)")
	if !errors.Is(err, errUnboundParameter) || !strings.Contains(err.Error(), `parameter "f"`) {
		t.Fatalf("error = %v, want errUnboundParameter naming f", err)
	}
	err = invokeCalcExpecting(t, src, "test::Partial(test::Add)")
	if !errors.Is(err, errUnboundParameter) || !strings.Contains(err.Error(), `parameter "y"`) {
		t.Fatalf("error = %v, want errUnboundParameter naming y", err)
	}
}

// testFunctionValueOfAWrongTypedCalc: a calc-typed parameter typed by a calc def
// refuses a function value of an unrelated calc.
func testFunctionValueOfAWrongTypedCalc(t *testing.T) {
	src := `package test {` + functionValueFixture + `
		calc def Typed { in calc f : Sq; return : Real = f(2.0); }
	}`
	err := invokeCalcExpecting(t, src, "test::Typed(test::Add)")
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("error = %v, want ErrTypeMismatch", err)
	}
}

// testFunctionValueOfABuiltIn: a library function the runtime binds unevaluated
// has no value to pass on, and says so.
func testFunctionValueOfABuiltIn(t *testing.T) {
	src := `package test {` + functionValueFixture + `
		private import ControlFunctions::*;
		calc def PassIf { return : Real = Fn(ControlFunctions::'if', 3.0); }
	}`
	err := invokeCalcExpecting(t, src, "test::PassIf()")
	if !errors.Is(err, errNotAFunction) {
		t.Fatalf("error = %v, want errNotAFunction", err)
	}
}

// testFunctionValueAppliedToItselfForever: a calc passing itself as a function
// value to itself without end spends the recursion budget rather than hanging.
func testFunctionValueAppliedToItselfForever(t *testing.T) {
	src := `package test {` + functionValueFixture + `
		calc def Loop { in calc f { in v : Real; return : Real; } in v : Real; return : Real = Loop(f, f(v)); }
	}`
	err := invokeCalcExpecting(t, src, "test::Loop(test::Sq, 1.0)")
	if !errors.Is(err, ErrCalcRecursionLimit) && !errors.Is(err, ErrStepLimitExceeded) {
		t.Fatalf("error = %v, want the recursion or step budget spent", err)
	}
}

// testFunctionValueInheritedBodyOutsideTheClosure: a usage nested in a calc body
// closes over that body only for the code written there; the body it inherits from
// a calc declared outside reads no binding of the enclosing run, however it is applied.
func testFunctionValueInheritedBodyOutsideTheClosure(t *testing.T) {
	src := `package test {` + functionValueFixture + `
		calc def Leaky { in v : Real; return : Real = v * k; }
		calc def Bare { in k : Real; calc inner : Leaky { in v = 2.0; } return : Real = inner; }
		calc def Called { in k : Real; calc inner : Leaky; return : Real = inner(2.0); }
		calc def Passed { in k : Real; calc inner : Leaky; return : Real = Fn(inner, 2.0); }
	}`
	for _, expr := range []string{"test::Bare(3.0)", "test::Called(3.0)", "test::Passed(3.0)"} {
		err := invokeCalcExpecting(t, src, expr)
		if !errors.Is(err, ErrNoValue) && !errors.Is(err, ErrUnresolvedReference) {
			t.Fatalf("%s: error = %v, want k unresolved in Leaky's body", expr, err)
		}
	}
}

// testFunctionValueNestedCalcOutsideItsRun: a calc nested in another calc's body
// closes over a run of that calc alone; applied from a calc that binds the same
// parameter name while no such run is active, it reads no binding of the caller's.
func testFunctionValueNestedCalcOutsideItsRun(t *testing.T) {
	src := `package test {` + functionValueFixture + `
		calc def Outer { in k : Real[1]; calc inner { in v : Real[1]; return : Real = v * k; } return : Real = inner(1.0); }
		calc def Called { in k : Real[1]; return : Real = Outer::inner(2.0); }
		calc def Passed { in k : Real[1]; return : Real = Fn(Outer::inner, 2.0); }
	}`
	for _, expr := range []string{"test::Called(3.0)", "test::Passed(3.0)"} {
		err := invokeCalcExpecting(t, src, expr)
		if !errors.Is(err, ErrNoValue) && !errors.Is(err, ErrUnresolvedReference) {
			t.Fatalf("%s: error = %v, want k unresolved in inner's body", expr, err)
		}
	}
}
