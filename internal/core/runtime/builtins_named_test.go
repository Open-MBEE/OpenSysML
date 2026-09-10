package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// evalLibraryCall evaluates expr as an attribute value of a model that loads
// the standard libraries, which is how a function-call form reaches a builtin.
func evalLibraryCall(t *testing.T, expr string) (Value, error) {
	t.Helper()
	return evalNamedAttribute(t, `package test {
	private import NumericalFunctions::*;
	private import ControlFunctions::*;
	attribute xs = (1, 2, 3);
	attribute result = `+expr+`;
}`, "result")
}

// The function-call form of a control function keeps the library's
// short-circuit semantics: a branch it does not select is never evaluated.
func TestControlFunctionCallForms(t *testing.T) {
	cases := []struct {
		expr string
		want string
	}{
		{"ControlFunctions::'if'(true, 1, 2)", "1"},
		{"ControlFunctions::'if'(false, 1, 2)", "2"},
		{"ControlFunctions::'if'(false, 1)", "null"},
		{"ControlFunctions::'if'(true, 1, 1 / 0)", "1"},
		{"ControlFunctions::'if'(false, 1 / 0, 2)", "2"},
		{"ControlFunctions::'if'(true, {1 + 1}, 2)", "2"},
		{"ControlFunctions::'and'(false, 1 / 0 == 0)", "false"},
		{"ControlFunctions::'and'(true, true)", "true"},
		{"ControlFunctions::'and'(true, false)", "false"},
		{"ControlFunctions::'or'(true, 1 / 0 == 0)", "true"},
		{"ControlFunctions::'or'(false, true)", "true"},
		{"ControlFunctions::'or'(false, false)", "false"},
		{"ControlFunctions::'implies'(false, 1 / 0 == 0)", "true"},
		{"ControlFunctions::'implies'(true, false)", "false"},
		{"ControlFunctions::'implies'(true, true)", "true"},
		{"ControlFunctions::'??'(null, 5)", "5"},
		{"ControlFunctions::'??'((), 5)", "5"},
		{"ControlFunctions::'??'(test::xs->select {in x; x > 3}, 5)", "5"},
		{"ControlFunctions::'??'((), {1 + 1})", "2"},
		{"ControlFunctions::'??'((1, 2), 1 / 0)", "[1, 2]"},
		{"ControlFunctions::'??'(3, 1 / 0)", "3"},
		{"ControlFunctions::'if'(false)", "null"},
		{"ControlFunctions::'if'(test = false, thenValue = 1)", "null"},
		{"ControlFunctions::'??'()", "null"},
		{"ControlFunctions::'??'(null)", "null"},
		{"ControlFunctions::'and'(false)", "false"},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			got, err := evalLibraryCall(t, tc.expr)
			if err != nil {
				t.Fatalf("%s = error %v", tc.expr, err)
			}
			if FormatValue(got) != tc.want {
				t.Fatalf("%s = %s, want %s", tc.expr, FormatValue(got), tc.want)
			}
		})
	}
}

// A control function reports a non-Boolean test and an error in the branch
// it does select.
func TestControlFunctionCallFormErrors(t *testing.T) {
	cases := []struct {
		expr string
		want error
	}{
		{"ControlFunctions::'if'(1, 1, 2)", ErrTypeMismatch},
		{"ControlFunctions::'if'(false, 1, 1 / 0)", ErrDivisionByZero},
		{"ControlFunctions::'and'(true, 1 / 0 == 0)", ErrDivisionByZero},
		{"ControlFunctions::'and'(true, 1)", ErrTypeMismatch},
		{"ControlFunctions::'or'(1, true)", ErrTypeMismatch},
		{"ControlFunctions::'??'(null, 1 / 0)", ErrDivisionByZero},
		{"ControlFunctions::'??'((), 1 / 0)", ErrDivisionByZero},
		{"ControlFunctions::'if'()", ErrCalcArity},
		{"ControlFunctions::'if'(true, 1, 2, 3)", ErrCalcArity},
		{"ControlFunctions::'and'(true)", ErrMultiplicityViolation},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			_, err := evalLibraryCall(t, tc.expr)
			if !errors.Is(err, tc.want) {
				t.Fatalf("%s error = %v, want %v", tc.expr, err, tc.want)
			}
		})
	}
}

// sum0 and product1 fold a collection and answer the identity given for an
// empty one, in the kind of the elements or of the identity.
func TestAggregationsWithIdentity(t *testing.T) {
	cases := []struct {
		expr string
		want string
	}{
		{"NumericalFunctions::sum0((1, 2, 3), 0)", "6"},
		{"NumericalFunctions::sum0((1.5, 2), 0)", "3.5"},
		{"NumericalFunctions::sum0((), 0)", "0"},
		{"NumericalFunctions::sum0((), 0.0)", "0.0"},
		{"NumericalFunctions::sum0(test::xs, 0.0)", "6"},
		{"NumericalFunctions::product1((2, 3, 4), 1)", "24"},
		{"NumericalFunctions::product1((0.5, 4), 1)", "2.0"},
		{"NumericalFunctions::product1((), 1)", "1"},
		{"NumericalFunctions::product1((), 1.0)", "1.0"},
		{"sum0((1, 2), 0)", "3"},
		{"product1((2, 5), 1)", "10"},
		{"NumericalFunctions::sum0(zero = 0)", "0"},
		{"NumericalFunctions::sum()", "0"},
		{"NumericalFunctions::product()", "1"},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			got, err := evalLibraryCall(t, tc.expr)
			if err != nil {
				t.Fatalf("%s = error %v", tc.expr, err)
			}
			if FormatValue(got) != tc.want {
				t.Fatalf("%s = %s, want %s", tc.expr, FormatValue(got), tc.want)
			}
		})
	}
}

// The identity argument must be the identity the library's invariant asserts,
// the elements must be numbers, and an overflowing fold is reported.
func TestAggregationsWithIdentityErrors(t *testing.T) {
	cases := []struct {
		expr string
		want error
		text string
	}{
		{"NumericalFunctions::sum0((1, 2), 1)", ErrTypeMismatch, "isZero(zero)"},
		{"NumericalFunctions::product1((1, 2), 0)", ErrTypeMismatch, "isUnit(one)"},
		{`NumericalFunctions::sum0(("a", "b"), 0)`, ErrTypeMismatch, "numeric elements"},
		{"NumericalFunctions::sum0((9223372036854775807, 1), 0)", semantics.ErrArithmeticOverflow, "Integer range"},
		{"NumericalFunctions::product1((1e200, 1e200), 1)", semantics.ErrArithmeticOverflow, "finite"},
		{"NumericalFunctions::sum0((1, 2))", ErrCalcArity, "sum0"},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			_, err := evalLibraryCall(t, tc.expr)
			if !errors.Is(err, tc.want) {
				t.Fatalf("%s error = %v, want %v", tc.expr, err, tc.want)
			}
			if !strings.Contains(err.Error(), tc.text) {
				t.Fatalf("%s error %q does not mention %q", tc.expr, err, tc.text)
			}
		})
	}
}

// A quantity identity is judged by its magnitude, as QuantityCalculations::isZero
// is NumericalFunctions::isZero(x.num), and answers the empty fold in its unit.
func TestAggregationsWithQuantityIdentity(t *testing.T) {
	eval := func(t *testing.T, expr string) (Value, error) {
		t.Helper()
		return evalNamedAttribute(t, `package test {
	private import SI::*;
	attribute result = `+expr+`;
}`, "result")
	}
	for _, tc := range []struct {
		expr string
		want string
	}{
		{"NumericalFunctions::sum0((1.0 [m], 2.0 [m]), 0.0 [m])", "3.0 [m]"},
		{"NumericalFunctions::sum0((), 0.0 [m])", "0.0 [m]"},
		{"NumericalFunctions::sum0((), 0 [m])", "0 [m]"},
		{"NumericalFunctions::product1((2.0 [m], 3.0 [m]), 1.0 [m])", "6.0 [SI::'m²']"},
		{"NumericalFunctions::product1((), 1.0 [m])", "1.0 [m]"},
	} {
		t.Run(tc.expr, func(t *testing.T) {
			got, err := eval(t, tc.expr)
			if err != nil {
				t.Fatalf("%s = error %v", tc.expr, err)
			}
			if FormatValue(got) != tc.want {
				t.Fatalf("%s = %s, want %s", tc.expr, FormatValue(got), tc.want)
			}
		})
	}
	for _, tc := range []struct {
		expr string
		text string
	}{
		{"NumericalFunctions::sum0((1.0 [m]), 5.0 [m])", "isZero(zero)"},
		{"NumericalFunctions::sum0((), 1.0 [m])", "isZero(zero)"},
		{"NumericalFunctions::product1((2.0 [m]), 0.0 [m])", "isUnit(one)"},
		{"NumericalFunctions::product1((), 2 [m])", "isUnit(one)"},
	} {
		t.Run(tc.expr, func(t *testing.T) {
			_, err := eval(t, tc.expr)
			if !errors.Is(err, ErrTypeMismatch) || !strings.Contains(err.Error(), tc.text) {
				t.Fatalf("%s error = %v, want %v mentioning %q", tc.expr, err, ErrTypeMismatch, tc.text)
			}
		})
	}
}

// A `[0..*]` body may be omitted or empty over an empty collection, which
// answers the operation's empty result; over any element the body is required.
// A deferred argument that is not a body literal is evaluated only where an
// element needs it, so one that would fail is harmless over no elements.
func TestCollectionOperationsWithOmittedBody(t *testing.T) {
	for _, tc := range []struct {
		expr string
		want string
	}{
		{"ControlFunctions::select()", "[]"},
		{"ControlFunctions::select(())", "[]"},
		{"ControlFunctions::select(collection = ())", "[]"},
		{"ControlFunctions::reject(())", "[]"},
		{"ControlFunctions::collect(())", "[]"},
		{"ControlFunctions::reduce(())", "null"},
		{"ControlFunctions::selectOne(())", "null"},
		{"ControlFunctions::forAll(())", "true"},
		{"ControlFunctions::exists(())", "false"},
		{"ControlFunctions::select(test::xs->select {in x; x > 3})", "[]"},
		{"ControlFunctions::select((), ())", "[]"},
		{"ControlFunctions::reject((), null)", "[]"},
		{"ControlFunctions::collect((), test::xs->select {in x; x > 3})", "[]"},
		{"ControlFunctions::reduce((), ())", "null"},
		{"ControlFunctions::selectOne((), ())", "null"},
		{"ControlFunctions::forAll(collection = (), test = ())", "true"},
		{"ControlFunctions::exists((), ())", "false"},
		{"ControlFunctions::select((), 1 / 0)", "[]"},
		{"ControlFunctions::reject((), 1 / 0)", "[]"},
		{"ControlFunctions::collect((), 1 / 0)", "[]"},
		{"ControlFunctions::reduce((), 1 / 0)", "null"},
		{"ControlFunctions::selectOne((), 1 / 0)", "null"},
		{"ControlFunctions::forAll((), 1 / 0)", "true"},
		{"ControlFunctions::exists(collection = (), test = 1 / 0)", "false"},
		{"ControlFunctions::select((), 5)", "[]"},
		{"test::xs->select {in x; x > 3}->select (1 / 0)", "[]"},
	} {
		t.Run(tc.expr, func(t *testing.T) {
			got, err := evalLibraryCall(t, tc.expr)
			if err != nil {
				t.Fatalf("%s = error %v", tc.expr, err)
			}
			if FormatValue(got) != tc.want {
				t.Fatalf("%s = %s, want %s", tc.expr, FormatValue(got), tc.want)
			}
		})
	}
	for _, tc := range []struct {
		expr string
		want error
	}{
		{"ControlFunctions::select(test::xs)", ErrTypeMismatch},
		{"ControlFunctions::reject(collection = (1))", ErrTypeMismatch},
		{"ControlFunctions::collect(test::xs)", ErrTypeMismatch},
		{"ControlFunctions::reduce(test::xs)", ErrTypeMismatch},
		{"ControlFunctions::selectOne(test::xs)", ErrTypeMismatch},
		{"ControlFunctions::forAll(test::xs)", ErrTypeMismatch},
		{"ControlFunctions::exists(test::xs)", ErrTypeMismatch},
		{"ControlFunctions::select(test::xs, ())", ErrTypeMismatch},
		{"ControlFunctions::select(test::xs, 5)", ErrTypeMismatch},
		{"ControlFunctions::select(test::xs, 1 / 0)", ErrDivisionByZero},
		{"ControlFunctions::reduce(test::xs, 1 / 0)", ErrDivisionByZero},
		{"ControlFunctions::forAll(test::xs, 1 / 0)", ErrDivisionByZero},
		{"ControlFunctions::forAll(test::xs, null)", ErrTypeMismatch},
		{"ControlFunctions::reduce((), {in a; a})", ErrBodyArity},
		{"ControlFunctions::minimize(())", ErrMultiplicityViolation},
		{"ControlFunctions::maximize(test::xs)", ErrTypeMismatch},
	} {
		t.Run(tc.expr, func(t *testing.T) {
			_, err := evalLibraryCall(t, tc.expr)
			if !errors.Is(err, tc.want) {
				t.Fatalf("%s error = %v, want %v", tc.expr, err, tc.want)
			}
		})
	}
}

// The sequence and range operators called by name answer as the notation does.
func TestSequenceOperatorCallForms(t *testing.T) {
	cases := []struct {
		expr string
		want string
	}{
		{"ScalarFunctions::'..'(1, 3)", "[1, 2, 3]"},
		{"DataFunctions::'..'(3, 1)", "[]"},
		{"IntegerFunctions::'..'(-1, 1)", "[-1, 0, 1]"},
		{"BaseFunctions::'#'((10, 20, 30), 2)", "20"},
		{"BaseFunctions::'#'((10, 20, 30), (3))", "30"},
		{"BaseFunctions::'#'(index = 1, seq = test::xs)", "1"},
		{"CollectionFunctions::'#'(test::xs, 3)", "3"},
		{"BaseFunctions::','((1, 2), (3))", "[1, 2, 3]"},
		{"CollectionFunctions::'=='((1, 2), (1, 2))", "true"},
		{"CollectionFunctions::'=='((1, 2), (2, 1))", "false"},
		{"SequenceFunctions::size()", "0"},
		{"SequenceFunctions::isEmpty()", "true"},
		{"SequenceFunctions::subsequence(test::xs, 2)", "[2, 3]"},
		{"SequenceFunctions::subsequence(startIndex = 2, seq = test::xs)", "[2, 3]"},
		{"SequenceFunctions::excludingAt(test::xs, 2)", "[1, 3]"},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			got, err := evalLibraryCall(t, tc.expr)
			if err != nil {
				t.Fatalf("%s = error %v", tc.expr, err)
			}
			if FormatValue(got) != tc.want {
				t.Fatalf("%s = %s, want %s", tc.expr, FormatValue(got), tc.want)
			}
		})
	}
}

// BaseFunctions::'#' takes `Positive[1..*]` indexes: several address an Array,
// so a flat sequence takes one, and none violates the multiplicity; the
// scalar-index forms name themselves in their errors.
func TestIndexCallFormErrors(t *testing.T) {
	cases := []struct {
		expr string
		want error
		text string
	}{
		{"BaseFunctions::'#'((1, 2, 3, 4), (2, 2))", ErrTypeMismatch, "BaseFunctions::'#': 2 indexes address an Array, got sequence"},
		{"BaseFunctions::'#'((1, 2), ())", ErrMultiplicityViolation, "BaseFunctions::'#' requires at least one index"},
		{"BaseFunctions::'#'((1, 2), 3)", ErrIndexOutOfRange, "BaseFunctions::'#' index 3"},
		{`BaseFunctions::'#'((1, 2), "1")`, ErrTypeMismatch, "BaseFunctions::'#' requires an Integer index"},
		{"CollectionFunctions::'#'((1, 2), 3)", ErrIndexOutOfRange, "CollectionFunctions::'#' index 3"},
		{"SequenceFunctions::'#'((1, 2), (1, 2))", ErrTypeMismatch, "SequenceFunctions::'#' requires an Integer index"},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			_, err := evalLibraryCall(t, tc.expr)
			if !errors.Is(err, tc.want) {
				t.Fatalf("%s error = %v, want %v", tc.expr, err, tc.want)
			}
			if !strings.Contains(err.Error(), tc.text) {
				t.Fatalf("%s error = %q, want it to mention %q", tc.expr, err, tc.text)
			}
		})
	}
}
