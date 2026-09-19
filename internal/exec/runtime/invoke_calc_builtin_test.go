package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// directBuiltinModel declares calcs that reach built-ins through invocation
// expressions, and bodies a direct invocation can hand to an `expr` parameter.
const directBuiltinModel = `package test {
	private import ScalarValues::*;
	calc def Then { return : expr = { 10 }; }
	calc def Boom { return : expr = { 1 / 0 }; }
	calc def IfTrue { return : Integer = ControlFunctions::'if'(true, 1, 2); }
	calc def IfFalse { return : Integer = ControlFunctions::'if'(false, 1, 2); }
	calc def Third { return : Integer = BaseFunctions::'#'((4, 5, 6), 3); }
	calc def Total { return : Integer = NumericalFunctions::sum0((1, 2, 3), 0); }
	calc def Nothing { return : Integer = NumericalFunctions::sum0(zero = 0); }
	calc def Size { return : Integer = (7, 8)->SequenceFunctions::size(); }
}`

// A direct invocation of a library declaration the built-ins implement — an
// abstract control function, an indexing operator, or one the library also
// gives a body — computes what the invocation expression computes.
func TestInvokeCalcDispatchesBuiltins(t *testing.T) {
	ctx, idx := libraryModelContext(t, directBuiltinModel)
	seq := func(vals ...int64) Value {
		s := NewSequence()
		for _, v := range vals {
			s.Append(constInt(v))
		}
		return NewSequenceValue(s)
	}
	cases := []struct {
		fqn  string
		args []Value
		via  string // the model calc reaching the same built-in by expression
	}{
		{"ControlFunctions::if", []Value{constBool(true), constInt(1), constInt(2)}, "IfTrue"},
		{"ControlFunctions::if", []Value{constBool(false), constInt(1), constInt(2)}, "IfFalse"},
		{"BaseFunctions::#", []Value{seq(4, 5, 6), constInt(3)}, "Third"},
		{"NumericalFunctions::sum0", []Value{seq(1, 2, 3), constInt(0)}, "Total"},
		{"SequenceFunctions::size", []Value{seq(7, 8)}, "Size"},
	}
	for _, tc := range cases {
		sym := lookupOne(t, idx, tc.fqn)
		if _, ok := ctx.builtinFor(sym); !ok {
			t.Fatalf("%s is not a built-in", tc.fqn)
		}
		direct, err := ctx.InvokeCalc(sym, tc.args, nil)
		if err != nil {
			t.Fatalf("InvokeCalc(%s) = error %v", tc.fqn, err)
		}
		expr, err := ctx.InvokeCalc(lookupOne(t, idx, "test::"+tc.via), nil, nil)
		if err != nil {
			t.Fatalf("%s = error %v", tc.via, err)
		}
		if !valueIdentical(direct, expr) {
			t.Errorf("InvokeCalc(%s) = %s, but %s = %s", tc.fqn, FormatValue(direct), tc.via, FormatValue(expr))
		}
	}
}

// Named arguments bind a built-in's declared parameters directly as they do in
// an expression: an omitted optional parameter is null, and an unknown, doubly
// bound or missing required one is reported.
func TestInvokeCalcNamedDispatchesBuiltins(t *testing.T) {
	ctx, idx := libraryModelContext(t, directBuiltinModel)
	sum0, cond := lookupOne(t, idx, "NumericalFunctions::sum0"), lookupOne(t, idx, "ControlFunctions::if")

	got, err := ctx.InvokeCalcNamed(sum0, map[string]Value{"zero": constInt(0)}, nil)
	if err != nil || !valueIdentical(got, constInt(0)) {
		t.Errorf("sum0(zero = 0) = %s, %v; want 0", FormatValue(got), err)
	}
	expr, err := ctx.InvokeCalc(lookupOne(t, idx, "test::Nothing"), nil, nil)
	if err != nil || !valueIdentical(got, expr) {
		t.Errorf("sum0(zero = 0) by expression = %s, %v; want %s", FormatValue(expr), err, FormatValue(got))
	}
	got, err = ctx.InvokeCalcNamed(cond, map[string]Value{"elseValue": constInt(2), "test": constBool(false)}, nil)
	if err != nil || !valueIdentical(got, constInt(2)) {
		t.Errorf("'if'(elseValue = 2, test = false) = %s, %v; want 2", FormatValue(got), err)
	}
	got, err = ctx.InvokeCalcNamed(cond, map[string]Value{"test": constBool(true)}, nil)
	if err != nil || got.Kind != ValNull {
		t.Errorf("'if'(test = true) = %s, %v; want null", FormatValue(got), err)
	}

	if _, err := ctx.InvokeCalcNamed(cond, map[string]Value{"cond": constBool(true)}, nil); !errors.Is(err, ErrUnknownParameter) {
		t.Errorf("'if'(cond = true) = %v, want %v", err, ErrUnknownParameter)
	}
	if _, err := ctx.InvokeCalcNamed(sum0, map[string]Value{"collection": nullValue()}, nil); !errors.Is(err, ErrCalcArity) {
		t.Errorf("sum0(collection = null) = %v, want %v", err, ErrCalcArity)
	}
	if _, err := ctx.InvokeCalc(cond, []Value{constBool(true), constInt(1), constInt(2), constInt(3)}, nil); !errors.Is(err, ErrCalcArity) {
		t.Errorf("'if' with four arguments = %v, want %v", err, ErrCalcArity)
	}
}

// A body value handed directly to an `expr` parameter is applied only when the
// control function selects it, so an unselected failing branch never runs.
func TestInvokeCalcDefersBodyArguments(t *testing.T) {
	ctx, idx := libraryModelContext(t, directBuiltinModel)
	body := func(calc string) Value {
		val, err := ctx.InvokeCalc(lookupOne(t, idx, "test::"+calc), nil, nil)
		if err != nil || val.Kind != ValExpr {
			t.Fatalf("%s = %s, %v; want a body", calc, FormatValue(val), err)
		}
		return val
	}
	then, boom := body("Then"), body("Boom")
	cond := lookupOne(t, idx, "ControlFunctions::if")

	got, err := ctx.InvokeCalc(cond, []Value{constBool(true), then, boom}, nil)
	if err != nil || !valueIdentical(got, constInt(10)) {
		t.Errorf("'if'(true, {10}, {1/0}) = %s, %v; want 10", FormatValue(got), err)
	}
	if _, err := ctx.InvokeCalc(cond, []Value{constBool(false), then, boom}, nil); !errors.Is(err, ErrDivisionByZero) {
		t.Errorf("'if'(false, {10}, {1/0}) = %v, want %v", err, ErrDivisionByZero)
	}
	got, err = ctx.InvokeCalc(lookupOne(t, idx, "ControlFunctions::??"), []Value{nullValue(), then}, nil)
	if err != nil || !valueIdentical(got, constInt(10)) {
		t.Errorf("'??'(null, {10}) = %s, %v; want 10", FormatValue(got), err)
	}
}

// A direct invocation may omit a `[0..*]` body over an empty collection, which
// answers the operation's empty result; over any element the body is required.
func TestInvokeCalcOmittedBodyOverEmptyCollection(t *testing.T) {
	ctx, idx := libraryModelContext(t, directBuiltinModel)
	empty, one := NewSequenceValue(NewSequence()), constInt(1)
	for _, tc := range []struct {
		fqn  string
		want string
	}{
		{"ControlFunctions::select", "[]"},
		{"ControlFunctions::reject", "[]"},
		{"ControlFunctions::collect", "[]"},
		{"ControlFunctions::reduce", "null"},
		{"ControlFunctions::selectOne", "null"},
		{"ControlFunctions::forAll", "true"},
		{"ControlFunctions::exists", "false"},
	} {
		sym := lookupOne(t, idx, tc.fqn)
		for _, args := range [][]Value{nil, {empty}, {empty, nullValue()}, {empty, empty}} {
			got, err := ctx.InvokeCalc(sym, args, nil)
			if err != nil || FormatValue(got) != tc.want {
				t.Errorf("InvokeCalc(%s, %d args) = %s, %v; want %s", tc.fqn, len(args), FormatValue(got), err, tc.want)
			}
		}
		got, err := ctx.InvokeCalcNamed(sym, map[string]Value{"collection": empty}, nil)
		if err != nil || FormatValue(got) != tc.want {
			t.Errorf("InvokeCalcNamed(%s, collection = ()) = %s, %v; want %s", tc.fqn, FormatValue(got), err, tc.want)
		}
		if _, err := ctx.InvokeCalc(sym, []Value{one}, nil); !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("InvokeCalc(%s, 1) = %v, want %v", tc.fqn, err, ErrTypeMismatch)
		}
	}
}

// A model's own declaration under a built-in's qualified name is the model's,
// so a direct invocation evaluates it rather than the library implementation.
func TestInvokeCalcNeverAnswersAModelDeclarationWithABuiltin(t *testing.T) {
	ctx, idx := libraryModelContext(t, `package NumericalFunctions {
	function sum0 { in collection : Integer[*]; in zero : Integer; return : Integer; }
}`)
	var sym *symbols.Symbol
	for _, s := range idx.LookupQualified("NumericalFunctions::sum0") {
		if !ctx.libraryDeclared(s) {
			sym = s
		}
	}
	if sym == nil {
		t.Fatal("the model's NumericalFunctions::sum0 was not indexed")
	}
	if _, ok := ctx.builtinFor(sym); ok {
		t.Fatal("the model's NumericalFunctions::sum0 dispatched to the built-in")
	}
	if _, err := ctx.InvokeCalc(sym, []Value{constInt(0)}, nil); !errors.Is(err, ErrNoResultExpression) {
		t.Errorf("InvokeCalc(model sum0) = %v, want %v", err, ErrNoResultExpression)
	}
}
