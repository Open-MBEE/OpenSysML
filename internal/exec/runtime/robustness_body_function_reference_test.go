package runtime

import (
	"errors"
	"testing"
)

const bodyFunctionReferenceRobustnessModel = `
package test {
	private import ScalarValues::*;
	private import ControlFunctions::*;
	calc def Neg { in a : Integer; return : Integer = -a; }
	attribute unary : Integer = (1, 2, 3)->reduce Neg;
	attribute notAFunction : Integer = (1, 2, 3)->reduce 5;
	attribute overNone = ()->reduce '+';
}
`

// TestRuntimeRobustnessBodyFunctionReference pins what a collection operation
// refuses in place of its body: a function of the wrong arity, a value that is no
// function; and that a function over no elements is not applied.
func TestRuntimeRobustnessBodyFunctionReference(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, bodyFunctionReferenceRobustnessModel))
	root := idx.DocumentRoot("<test>")
	for _, tc := range []struct {
		name, expr string
		want       error
	}{
		{"function_of_the_wrong_arity", "test::unary", ErrBodyArity},
		{"value_that_is_no_function", "test::notAFunction", ErrTypeMismatch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := evalIn(t, ctx, root, tc.expr)
			if !errors.Is(err, tc.want) {
				t.Fatalf("%s: %v, want %v", tc.expr, err, tc.want)
			}
		})
	}
	t.Run("function_over_no_elements_is_not_applied", func(t *testing.T) {
		val, err := evalIn(t, ctx, root, "test::overNone")
		if err != nil || val.Kind != ValNull {
			t.Fatalf("test::overNone = %v, %v; want null", val, err)
		}
	})
}
