package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// evalDeclaredExpr evaluates expr as the value of an attribute declared beside src.
func evalDeclaredExpr(t *testing.T, src, expr string) (*Context, Value, error) {
	t.Helper()
	ctx, scope, decl := declaredExpr(t, src, expr)
	got, err := newEvalContext(ctx, scope).Eval(decl)
	return ctx, got, err
}

// declaredExpr is expr as the value of an attribute declared beside src, with
// the context and scope to evaluate it in, so one context can evaluate it twice.
func declaredExpr(t *testing.T, src, expr string) (*Context, *symbols.Scope, ast.Node) {
	t.Helper()
	full := src + "\npackage probe {\n\tattribute result = " + expr + ";\n}"
	model, resolver, root := parseAndBuildLibraryModel(t, full)
	pkg, ok := root.LookupLocal("probe")
	if !ok || pkg == nil || pkg.Scope == nil {
		t.Fatal("package probe has no scope")
	}
	sym, ok := pkg.Scope.LookupLocal("result")
	if !ok || sym == nil {
		t.Fatal("attribute result not found")
	}
	decl, ok := sym.Decl.(*ast.Usage)
	if !ok {
		t.Fatalf("result declares %T, want a usage", sym.Decl)
	}
	return NewContext(typedModel(model, resolver), 10000), pkg.Scope, decl.Value
}

// TestInfinityValue evaluates `*`: its own scalar value, printed as written.
func TestInfinityValue(t *testing.T) {
	_, got, err := evalDeclaredExpr(t, "package test {}", "*")
	if err != nil {
		t.Fatalf("* failed: %v", err)
	}
	if got.Kind != ValConst || !got.Const.IsUnbounded() {
		t.Fatalf("* evaluated to %v (%v), want the unbounded constant", got.Kind, got.Const.Kind)
	}
	if text := FormatValue(got); text != "*" {
		t.Errorf("FormatValue = %q, want %q", text, "*")
	}
	if text := FormatTraceValue(got); text != "*" {
		t.Errorf("FormatTraceValue = %q, want %q", text, "*")
	}
}

// TestInfinityComparison orders `*` above every finite number and equal to itself.
func TestInfinityComparison(t *testing.T) {
	cases := []struct {
		expr string
		want bool
	}{
		{"* > 1", true},
		{"* > 1.5", true},
		{"* > -100", true},
		{"1 < *", true},
		{"* >= 7", true},
		{"* <= 1", false},
		{"* < 1", false},
		{"* == *", true},
		{"* >= *", true},
		{"* <= *", true},
		{"* > *", false},
		{"* < *", false},
		{"* != *", false},
		{"* == 1", false},
		{"* != 1", true},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			_, got, err := evalDeclaredExpr(t, "package test {}", tc.expr)
			if err != nil {
				t.Fatalf("%s failed: %v", tc.expr, err)
			}
			if got.Kind != ValConst || got.Const.Kind != semantics.ValBool {
				t.Fatalf("%s evaluated to %v, want a Boolean", tc.expr, got.Kind)
			}
			if got.Const.Bool != tc.want {
				t.Errorf("%s = %v, want %v", tc.expr, got.Const.Bool, tc.want)
			}
		})
	}
}

// TestInfinityArithmetic answers arithmetic over `*` and `-*` in the affinely extended reals KerML
// 9.3.2.2.9 makes Real: the limit where one exists, in operator, function and aggregation forms alike.
func TestInfinityArithmetic(t *testing.T) {
	cases := []struct{ expr, want string }{
		{"-*", "-*"}, {"-(-*)", "*"}, {"+(-*)", "-*"},
		{"* + 1", "*"}, {"1 + *", "*"}, {"* + 1.5", "*"}, {"* + *", "*"},
		{"* - 1", "*"}, {"1 - *", "-*"}, {"-* - *", "-*"},
		{"2 * *", "*"}, {"-2 * *", "-*"}, {"* * *", "*"}, {"(-*) * *", "-*"},
		{"* / 2", "*"}, {"* / -2", "-*"}, {"2 / *", "0.0"}, {"2 / -*", "0.0"},
		{"* ** 2", "*"}, {"* ^ 2", "*"}, {"* ** -1", "0.0"}, {"* ** *", "*"}, {"* ** -*", "0.0"},
		{"2 ** *", "*"}, {"2 ** -*", "0.0"}, {"0.5 ** *", "0.0"}, {"0 ** *", "0"},
		{"(-*) ** 2", "*"}, {"(-*) ** 3", "-*"}, {"(-*) ** -3", "0.0"},
		{"NaturalFunctions::'+'(*, 1)", "*"}, {"RationalFunctions::rat(*, 2)", "*"}, {"RationalFunctions::rat(2, -*)", "0.0"},
		{"2 / * hastype ScalarValues::Rational", "true"},
		{"OpenSysMLMathFunctions::log(*, 2)", "*"}, {"OpenSysMLMathFunctions::log(2, *)", "0.0"},
		{"OpenSysMLMathFunctions::atan2(1, *)", "0.0"},
		{"OpenSysMLMathFunctions::quotient(*, -2)", "-*"}, {"OpenSysMLMathFunctions::quotient(2, *)", "0"},
		{"IntegerFunctions::'-'(*)", "-*"},
		{"RealFunctions::'*'(-*, 2.0)", "-*"},
		{"NaturalFunctions::'/'(*, 2)", "*"},
		{"NaturalFunctions::'/'(4, *)", "0"},
		{"IntegerFunctions::sum((1, *))", "*"},
		{"RealFunctions::sum((1.5, -*, 2))", "-*"},
		{"IntegerFunctions::product((2, *))", "*"},
		{"IntegerFunctions::product((-2, *))", "-*"},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			_, got, err := evalDeclaredExpr(t, "package test {}", tc.expr)
			if err != nil {
				t.Fatalf("%s failed: %v", tc.expr, err)
			}
			if text := FormatValue(got); text != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, text, tc.want)
			}
		})
	}
}

// TestInfinityArithmeticUndefined: an indeterminate form of the extended reals is a typed domain error,
// a zero divisor a division by zero, an operand outside the operation's declared type a type mismatch.
func TestInfinityArithmeticUndefined(t *testing.T) {
	cases := []struct {
		expr string
		kind error
		want string
	}{
		{"* - *", semantics.ErrArithmeticDomain, "* - * is undefined in the extended reals"},
		{"-* + *", semantics.ErrArithmeticDomain, "-* + * is undefined in the extended reals"},
		{"* * 0", semantics.ErrArithmeticDomain, "* * 0 is undefined in the extended reals"},
		{"0.0 * -*", semantics.ErrArithmeticDomain, "is undefined in the extended reals"},
		{"* / *", semantics.ErrArithmeticDomain, "* / * is undefined in the extended reals"},
		{"* / 0", ErrDivisionByZero, "division by zero"},
		{"-* / 0.0", ErrDivisionByZero, "division by zero"},
		{"* % 2", semantics.ErrArithmeticDomain, "* % 2 is undefined in the extended reals"},
		{"2 % -*", semantics.ErrArithmeticDomain, "2 % -* is undefined in the extended reals"},
		{"* ** 0", semantics.ErrArithmeticDomain, "* ** 0 is undefined in the extended reals"},
		{"1 ** *", semantics.ErrArithmeticDomain, "1 ** * is undefined in the extended reals"},
		{"0 ** -*", semantics.ErrArithmeticDomain, "is undefined in the extended reals"},
		{"(-2) ** *", semantics.ErrArithmeticDomain, "is undefined in the extended reals"},
		{"(-*) ** 0.5", semantics.ErrArithmeticDomain, "is undefined in the extended reals"},
		{"NaturalFunctions::'/'(*, *)", semantics.ErrArithmeticDomain, "* / * is undefined in the extended reals"},
		{"IntegerFunctions::'%'(*, 2)", semantics.ErrArithmeticDomain, "* % 2 is undefined in the extended reals"},
		{"IntegerFunctions::product((0, *))", semantics.ErrArithmeticDomain, "0 * * is undefined in the extended reals"},
		{"RealFunctions::sum((*, -*))", semantics.ErrArithmeticDomain, "* + -* is undefined in the extended reals"},
		{"NaturalFunctions::'+'(-*, 1)", ErrTypeMismatch, "requires a Natural value, got -*"},
		{"1..*", semantics.ErrArithmeticDomain, "'..' requires finite bounds, got * for upper"},
		{"-*..1", semantics.ErrArithmeticDomain, "'..' requires finite bounds, got -* for lower"},
		{"NaturalFunctions::ToString(-*)", ErrTypeMismatch, "requires a Natural value, got -*"},
		{"* + true", ErrTypeMismatch, "operator '+' requires numeric operands, got * and true"},
		{"(-*) ** -0.5", semantics.ErrArithmeticDomain, "is undefined in the extended reals"},
		{"OpenSysMLMathFunctions::atan2(*, -*)", semantics.ErrArithmeticDomain, "is undefined at (*, -*)"},
		{"OpenSysMLMathFunctions::log(*, *)", semantics.ErrArithmeticDomain, "is undefined at (*, *)"},
		{"OpenSysMLMathFunctions::quotient(*, 0)", ErrDivisionByZero, "division by zero"},
		{`* > "a"`, ErrTypeMismatch, "'>' is not defined"},
		{`-* + "a"`, ErrTypeMismatch, ""},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			_, _, err := evalDeclaredExpr(t, "package test {}", tc.expr)
			if err == nil {
				t.Fatalf("%s succeeded, want a typed error", tc.expr)
			}
			if !errors.Is(err, tc.kind) {
				t.Errorf("%s: error %v, want %v", tc.expr, err, tc.kind)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("%s: error %q, want it to contain %q", tc.expr, err, tc.want)
			}
			if strings.Contains(err.Error(), "∞") || strings.Contains(err.Error(), "Inf") {
				t.Errorf("%s: error %q spells the infinity other than as `*`", tc.expr, err)
			}
		})
	}
}

// TestNegativeInfinity orders `-*` below every number, types it as the Integer,
// Rational and Real it is and as no Natural, and takes it through the library.
func TestNegativeInfinity(t *testing.T) {
	cases := []struct{ expr, want string }{
		{"-* < *", "true"}, {"-* < -1", "true"}, {"-1.5 > -*", "true"}, {"-* <= -*", "true"},
		{"-* == -*", "true"}, {"-* == *", "false"}, {"-* != *", "true"}, {"-* > 0", "false"},
		{"RealFunctions::'<'(-*, 2.0)", "true"},
		{"-* istype ScalarValues::Real", "true"}, {"-* istype ScalarValues::Rational", "true"},
		{"-* istype ScalarValues::Integer", "true"}, {"-* istype ScalarValues::Natural", "false"},
		{"* istype ScalarValues::Natural", "true"},
		{"NumericalFunctions::max(-*, 3)", "3"}, {"RealFunctions::min(-*, 3.0)", "-*"},
		{"RealFunctions::min(-*, *)", "-*"}, {"RealFunctions::max(-*, *)", "*"},
		{"RealFunctions::abs(-*)", "*"}, {"IntegerFunctions::abs(-*)", "*"},
		{"RealFunctions::sqrt(*)", "*"}, {"RealFunctions::floor(-*)", "-*"}, {"RealFunctions::round(*)", "*"},
		{"OpenSysMLMathFunctions::exp(-*)", "0.0"}, {"OpenSysMLMathFunctions::exp(*)", "*"},
		{"OpenSysMLMathFunctions::ln(*)", "*"},
		{"NumericalFunctions::isZero(-*)", "false"},
		{"RealFunctions::ToString(-*)", `"-*"`}, {"IntegerFunctions::ToString(-*)", `"-*"`},
		{"RealFunctions::ToInteger(-*)", "-*"}, {"RealFunctions::ToRational(-*)", "-*"},
		{"IntegerFunctions::ToNatural(*)", "*"},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			_, got, err := evalDeclaredExpr(t, "package test {}", tc.expr)
			if err != nil {
				t.Fatalf("%s failed: %v", tc.expr, err)
			}
			if text := FormatValue(got); text != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, text, tc.want)
			}
		})
	}
	undefined := []struct {
		expr string
		kind error
	}{
		{"RealFunctions::sqrt(-*)", semantics.ErrArithmeticDomain},
		{"OpenSysMLMathFunctions::ln(-*)", semantics.ErrArithmeticDomain},
		{"TrigFunctions::sin(*)", semantics.ErrArithmeticDomain},
		{"RationalFunctions::gcd(*, 2)", semantics.ErrArithmeticDomain},
		{"IntegerFunctions::ToNatural(-*)", semantics.ErrArithmeticDomain},
		{"NaturalFunctions::max(-*, 3)", ErrTypeMismatch},
		{"NaturalFunctions::ToString(-*)", ErrTypeMismatch},
	}
	for _, tc := range undefined {
		t.Run(tc.expr, func(t *testing.T) {
			if _, _, err := evalDeclaredExpr(t, "package test {}", tc.expr); !errors.Is(err, tc.kind) {
				t.Errorf("%s: error %v, want %v", tc.expr, err, tc.kind)
			}
		})
	}
}

// TestInfinityOrderingFunctions answers what the ordering of `*` decides (KerML
// 1.0 §8.2.5.8.4: a Positive greater than all the integers) in the function
// notation as in the operator notation: the comparisons and equality, `max`,
// `min` and `abs` in every numeric library package, `ToString`, and unary `+`.
func TestInfinityOrderingFunctions(t *testing.T) {
	cases := []struct {
		expr string
		want string
	}{
		{"+*", "*"},
		{"NaturalFunctions::'+'(*)", "*"},
		{"NaturalFunctions::'>'(*, 1)", "true"},
		{"IntegerFunctions::'<'(*, -1)", "false"},
		{"RealFunctions::'>='(*, 1.5)", "true"},
		{"RationalFunctions::'<='(*, *)", "true"},
		{"NumericalFunctions::'>'(2, *)", "false"},
		{"NaturalFunctions::'=='(*, 1)", "false"},
		{"IntegerFunctions::'=='(*, *)", "true"},
		{"BaseFunctions::'!='(*, 1)", "true"},
		{"NaturalFunctions::max(*, 1)", "*"},
		{"NaturalFunctions::min(*, 1)", "1"},
		{"IntegerFunctions::max(-3, *)", "*"},
		{"IntegerFunctions::min(-3, *)", "-3"},
		{"IntegerFunctions::max(*, *)", "*"},
		{"IntegerFunctions::min(*, *)", "*"},
		{"RationalFunctions::min(*, 2.5)", "2.5"},
		{"RealFunctions::max(*, 2)", "*"},
		{"RealFunctions::min(*, 2)", "2.0"},
		{"NumericalFunctions::max(2.5, *)", "*"},
		{"NumericalFunctions::min(2.5, *)", "2.5"},
		{"ScalarFunctions::max(*, 3)", "*"},
		{"DataFunctions::min(3, *)", "3"},
		{"IntegerFunctions::abs(*)", "*"},
		{"RationalFunctions::abs(*)", "*"},
		{"RealFunctions::abs(*)", "*"},
		{"NumericalFunctions::abs(*)", "*"},
		{"BaseFunctions::ToString(*)", `"*"`},
		{"NaturalFunctions::ToString(*)", `"*"`},
		{"IntegerFunctions::ToString(*)", `"*"`},
		{"RationalFunctions::ToString(*)", `"*"`},
		{"RealFunctions::ToString(*)", `"*"`},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			_, got, err := evalDeclaredExpr(t, "package test {}", tc.expr)
			if err != nil {
				t.Fatalf("%s failed: %v", tc.expr, err)
			}
			if text := FormatValue(got); text != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, text, tc.want)
			}
		})
	}
}

// TestInfinityOrderingFunctionsKeepTheirDomains checks the finite operand of
// `max`, `min` and `abs` beside `*` as the function checks it alone.
func TestInfinityOrderingFunctionsKeepTheirDomains(t *testing.T) {
	cases := []struct {
		expr string
		want string
	}{
		{"IntegerFunctions::min(*, 2.5)", "requires an Integer argument, got a Rational"},
		{"NaturalFunctions::max(*, -1)", "requires Natural arguments, got -1"},
		{`NaturalFunctions::max(*, "a")`, "requires a numeric value, got string"},
		{"IntegerFunctions::abs((1, *))", "requires a numeric value, got sequence"},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			_, _, err := evalDeclaredExpr(t, "package test {}", tc.expr)
			if err == nil {
				t.Fatalf("%s succeeded, want a typed error", tc.expr)
			}
			if !errors.Is(err, ErrTypeMismatch) {
				t.Errorf("%s: error %v, want a type mismatch", tc.expr, err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("%s: error %q, want it to contain %q", tc.expr, err, tc.want)
			}
		})
	}
}

// TestInfinityDirectType classifies `*` as the Positive it is typed as in
// expression position, and so as a Natural, but as nothing unrelated.
func TestInfinityDirectType(t *testing.T) {
	cases := map[string]string{
		"* istype ScalarValues::Positive":  "true",
		"* hastype ScalarValues::Positive": "true",
		"* istype ScalarValues::Natural":   "true",
		// hastype demands the direct type itself, which Natural is only above.
		"* hastype ScalarValues::Natural": "false",
		"* istype ScalarValues::String":   "false",
		"* hastype ScalarValues::Boolean": "false",
		"* @ ScalarValues::Positive":      "true",
		"* @ ScalarValues::String":        "false",
	}
	for expr, want := range cases {
		_, got, err := evalDeclaredExpr(t, "package test {}", expr)
		if err != nil {
			t.Errorf("%s failed: %v", expr, err)
			continue
		}
		if text := FormatValue(got); text != want {
			t.Errorf("%s = %s, want %s", expr, text, want)
		}
	}
}

// TestUnboundedAgainstDeclaredSubtype judges `*` against a subtype a model declares
// of Positive as it judges `4` against `Even :> Integer`: a feature declared of the
// subtype holds it and affirms `istype`, the bare value leaves a cast undecided, and
// a scalar type beside Positive still excludes it.
func TestUnboundedAgainstDeclaredSubtype(t *testing.T) {
	const src = `package test {
		private import ScalarValues::*;
		attribute def Boundless :> Positive;
		attribute def Cost :> Real;
		attribute def Boolish :> Boolean;
		attribute limit : Boundless = *;
		attribute cost : Cost = *;
	}`
	cases := map[string]string{
		"test::limit":                         "*",
		"test::cost":                          "*",
		"test::limit istype test::Boundless":  "true",
		"test::limit hastype test::Boundless": "false",
		"test::limit @ test::Boundless":       "true",
		"test::cost istype test::Cost":        "true",
		"* istype test::Boundless":            "false",
		"* istype test::Cost":                 "false",
		"* as test::Boolish":                  "[]",
		"* as ScalarValues::String":           "[]",
		"test::limit as test::Boundless":      "*",
	}
	for expr, want := range cases {
		_, got, err := evalDeclaredExpr(t, src, expr)
		if err != nil {
			t.Errorf("%s failed: %v", expr, err)
			continue
		}
		if text := FormatValue(got); text != want {
			t.Errorf("%s = %s, want %s", expr, text, want)
		}
	}
	for _, expr := range []string{"* as test::Boundless", "* as test::Cost", "(1, *) as test::Boundless"} {
		if _, _, err := evalDeclaredExpr(t, src, expr); !errors.Is(err, errUndecidedClassification) {
			t.Errorf("%s error = %v, want errUndecidedClassification", expr, err)
		}
	}
}

// TestInfinityOrderingOverOpenOperands: beside a feature the model leaves open the maximum with `*` is
// `*`; the minimum, the comparisons and a sum stay open in both notations; a remainder is undefined.
func TestInfinityOrderingOverOpenOperands(t *testing.T) {
	const src = "package test {\n\tprivate import ScalarValues::*;\n\tattribute u : Integer;\n}"
	for _, expr := range []string{"IntegerFunctions::max(*, test::u)", "IntegerFunctions::max(test::u, *)"} {
		t.Run(expr, func(t *testing.T) {
			_, got, err := evalDeclaredExpr(t, src, expr)
			if err != nil {
				t.Fatalf("%s failed: %v", expr, err)
			}
			if text := FormatValue(got); text != "*" {
				t.Errorf("%s = %s, want *", expr, text)
			}
		})
	}
	open := []string{
		"IntegerFunctions::min(*, test::u)", "IntegerFunctions::min(test::u, *)",
		"test::u < *", "* > test::u", "test::u <= *", "test::u == *",
		"IntegerFunctions::'<'(test::u, *)", "RealFunctions::'>='(*, test::u)",
	}
	for _, expr := range open {
		t.Run(expr, func(t *testing.T) {
			_, got, err := evalDeclaredExpr(t, src, expr)
			if err != nil {
				t.Fatalf("%s failed: %v", expr, err)
			}
			if got.Kind != ValUndetermined {
				t.Errorf("%s = %s, want an undetermined value", expr, FormatValue(got))
			}
		})
	}
	// The feature may hold the opposite infinity, so the sum stays open too.
	for _, expr := range []string{"test::u + *", "* - test::u", "IntegerFunctions::'+'(test::u, *)"} {
		t.Run(expr, func(t *testing.T) {
			_, got, err := evalDeclaredExpr(t, src, expr)
			if err != nil {
				t.Fatalf("%s failed: %v", expr, err)
			}
			if got.Kind != ValUndetermined {
				t.Errorf("%s = %s, want an undetermined value", expr, FormatValue(got))
			}
		})
	}
	t.Run("test::u % *", func(t *testing.T) {
		if _, _, err := evalDeclaredExpr(t, src, "test::u % *"); !errors.Is(err, semantics.ErrArithmeticDomain) {
			t.Errorf("test::u %% *: error %v, want a domain error", err)
		}
	})
}
