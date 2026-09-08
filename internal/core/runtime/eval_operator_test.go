package runtime

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// TestUnimplementedOperatorReportsWhy requires an operator the runtime does not
// evaluate to say what it would need, rather than failing as "unsupported".
func TestUnimplementedOperatorReportsWhy(t *testing.T) {
	const src = `calc def complement { in n : Integer; return : Integer = ~n; }`

	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 1000)
	complement := resolveSymbol(t, root, "complement")

	_, err := ctx.InvokeCalc(complement, []Value{constInt(1)}, root)
	if !errors.Is(err, ErrUnsupportedOperator) {
		t.Fatalf("InvokeCalc: got %v, want ErrUnsupportedOperator", err)
	}
	if !strings.Contains(err.Error(), "function library") {
		t.Fatalf("InvokeCalc: %v does not say what the complement would need", err)
	}
}

func TestTypeClassificationOperators(t *testing.T) {
	tests := []struct {
		expr string
		want bool
	}{
		{"n7 istype Integer", true},
		{"n7 hastype Integer", true},
		{"n7 istype Real", true},
		{"n7 hastype Real", false},
		{"n7 istype Natural", false},
		{"nat3 istype Integer", true},
		{"nat3 hastype Integer", true},
		{"test::car hastype Car", true},
		{"r25 istype Integer", false},
		{"test::car hastype Vehicle", false},
		{`str istype String`, true},
		{"n7 istype String", false},
		{"3 istype Integer", true},
		{"3 istype Real", true},
		{"seqInt istype Integer", true},
		{`(1, "a") istype Integer`, false},
		{"test::none istype Integer", true},
		{"test::car istype Car", true},
		{"test::car istype Vehicle", true},
		{"test::sys istype Car", false},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			got, err := evalTypeClassificationExpr(t, tt.expr)
			if err != nil {
				t.Fatalf("%s: %v", tt.expr, err)
			}
			if got != tt.want {
				t.Errorf("%s = %v, want %v", tt.expr, got, tt.want)
			}
		})
	}
}

func evalTypeClassificationExpr(t *testing.T, expr string) (bool, error) {
	t.Helper()
	const prefix = `package test {
		item def Real;
		item def Integer :> Real;
		item def Natural :> Integer;
		item def Boolean;
		item def String;
		part def Vehicle;
		part def Car :> Vehicle;
		part def Sys;
		attribute n7 : Integer = 7;
		attribute nat3 : Natural = 3;
		attribute r25 : Real = 2.5;
		attribute str : String = "s";
		attribute seqInt : Integer[*] = (1, 2, 3);
		attribute none : Integer[0..1];
		part car : Car;
		part sys : Sys;
		attribute result = `
	model, resolver, root := parseAndBuildModel(t, prefix+expr+`; }`)
	pkg := resolveSymbol(t, root, "test")
	result := resolveSymbol(t, pkg.Scope, "result")
	decl := result.Decl.(*ast.Usage)
	value, err := NewEvalContext(NewContext(model, resolver, 10000), pkg.Scope).Eval(decl.Value)
	if err != nil {
		return false, err
	}
	if value.Kind != ValConst || value.Const.Kind != semantics.ValBool {
		t.Fatalf("%s = %v, want Boolean", expr, value)
	}
	return value.Const.Bool, nil
}

func TestTypeClassificationFollowsSelectedVariant(t *testing.T) {
	const src = `
		part def Vehicle;
		variation part choice : Vehicle {
			variant part car : Vehicle;
		}
		part def Garage {
			part chosen : Vehicle = choice::car;
		}
		part garage : Garage;
		attribute exact = garage.chosen hastype choice::car;
		attribute declared = garage.chosen hastype Vehicle;
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 10000)
	for _, tt := range []struct {
		name string
		want bool
	}{
		{name: "exact", want: true},
		{name: "declared", want: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			sym := resolveSymbol(t, root, tt.name)
			value, err := ctx.Eval(sym.Decl.(*ast.Usage).Value)
			if err != nil {
				t.Fatalf("Eval: %v", err)
			}
			if value.Kind != ValConst || value.Const.Kind != semantics.ValBool {
				t.Fatalf("value = %v, want Boolean", value)
			}
			if value.Const.Bool != tt.want {
				t.Errorf("value = %v, want %v", value.Const.Bool, tt.want)
			}
		})
	}
}

// strValue is a String runtime value, the representation of a string literal.
func strValue(s string) Value { return NewStringValue(s) }

// evalStringExpr evaluates expr against string-valued attributes, on its own
// goroutine so an evaluation that neither answers nor fails fails the test.
func evalStringExpr(t *testing.T, expr string) (Value, error) {
	t.Helper()
	src := `
package test {
	private import SequenceFunctions::*;
	attribute s = "abc";
	attribute empty = "";
	attribute accented = "héllo";
	attribute three = 3;
	attribute result = ` + expr + `;
}`
	model, resolver, root := parseAndBuildLibraryModel(t, src)
	pkg, ok := root.LookupLocal("test")
	if !ok || pkg == nil || pkg.Scope == nil {
		t.Fatal("package test has no scope")
	}
	sym, ok := pkg.Scope.LookupLocal("result")
	if !ok || sym == nil {
		t.Fatal("attribute result not found")
	}
	decl, ok := sym.Decl.(*ast.Usage)
	if !ok {
		t.Fatalf("result declares %T, want a usage", sym.Decl)
	}
	ec := NewEvalContext(NewContext(model, resolver, 10000), pkg.Scope)

	type outcome struct {
		value Value
		err   error
	}
	done := make(chan outcome, 1)
	go func() {
		got, err := ec.Eval(decl.Value)
		done <- outcome{got, err}
	}()
	select {
	case got := <-done:
		return got.value, got.err
	case <-time.After(10 * time.Second):
		t.Fatalf("%s did not terminate", expr)
		return Value{}, nil
	}
}

// TestStringOperators pins the operators StringFunctions declares: '+'
// concatenates, the four comparisons order strings by their characters, and
// '==' specializes DataFunctions::'==', which answers for any pair of values.
func TestStringOperators(t *testing.T) {
	tests := []struct {
		expr string
		want Value
	}{
		{`s + "d"`, strValue("abcd")},
		{`s + empty`, strValue("abc")},
		{`empty + empty`, strValue("")},
		{`accented + "!"`, strValue("héllo!")},
		{`s + "d" + "e"`, strValue("abcde")},
		{`s < "b"`, boolValue(true)},
		{`s < "abc"`, boolValue(false)},
		{`s <= "abc"`, boolValue(true)},
		{`s > "abb"`, boolValue(true)},
		{`s >= "abd"`, boolValue(false)},
		{`empty < s`, boolValue(true)},
		// A multi-byte character orders by its code point, above every ASCII one.
		{`accented > "hello"`, boolValue(true)},
		{`s == "abc"`, boolValue(true)},
		{`s == "abd"`, boolValue(false)},
		{`s != "abd"`, boolValue(true)},
		// '==' is DataFunctions' equality over any values, so a String and an
		// Integer are unequal rather than an error, and neither is coerced.
		{`s == three`, boolValue(false)},
		{`s != three`, boolValue(true)},
		// Equality of strings is by characters, which is what a sequence
		// membership test asks of them.
		{`("a", "b")->includes("b")`, boolValue(true)},
		{`("a", "b")->includes("c")`, boolValue(false)},
		{`("a", "b")->equals(("a", "b"))`, boolValue(true)},
		{`StringFunctions::'+'(s, "d")`, strValue("abcd")},
		{`StringFunctions::'<'(s, "b")`, boolValue(true)},
		{`StringFunctions::'>='(s, "abc")`, boolValue(true)},
		{`StringFunctions::'=='(s, "abc")`, boolValue(true)},
		{`StringFunctions::ToString(s)`, strValue("abc")},
	}
	for _, tt := range tests {
		got, err := evalStringExpr(t, tt.expr)
		if err != nil {
			t.Errorf("%s: %v", tt.expr, err)
			continue
		}
		if !valueEqual(got, tt.want) {
			t.Errorf("%s = %v, want %v", tt.expr, got, tt.want)
		}
	}
}

// TestStringOperatorErrors pins that a string is not coerced: an operator the
// library declares over two Strings reports an operand of another type.
func TestStringOperatorErrors(t *testing.T) {
	tests := []string{
		`s + three`,
		`three + s`,
		`s < three`,
		`three < s`,
		`s <= three`,
		`s > three`,
		`s >= three`,
		`s - "a"`,
		`s * "a"`,
		`StringFunctions::'+'(s, three)`,
		`StringFunctions::'<'(three, s)`,
	}
	for _, expr := range tests {
		got, err := evalStringExpr(t, expr)
		if !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("%s = %v, %v, want ErrTypeMismatch", expr, got, err)
		}
	}
}

// TestRealDivisionByZeroIsReported: a real quotient or remainder by zero has no
// value and is reported as such, as the integer and quantity ones are, rather
// than answering an infinity a condition would then read as a number.
func TestRealDivisionByZeroIsReported(t *testing.T) {
	const src = `
package test {
	calc def quotient { in a : Real; in b : Real; return : Real = a / b; }
	calc def remainder { in a : Real; in b : Real; return : Real = a % b; }
}`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 1000)
	for _, name := range []string{"quotient", "remainder"} {
		sym := resolveSymbol(t, root, "test")
		if sym.Scope == nil {
			t.Fatal("package test has no scope")
		}
		calc, ok := sym.Scope.LookupLocal(name)
		if !ok || calc == nil {
			t.Fatalf("calc %s not found", name)
		}
		got, err := ctx.InvokeCalc(calc, []Value{constReal(2), constReal(0)}, sym.Scope)
		if !errors.Is(err, ErrDivisionByZero) {
			t.Errorf("%s(2.0, 0.0) = (%v, %v), want ErrDivisionByZero", name, got, err)
		}
	}
}
