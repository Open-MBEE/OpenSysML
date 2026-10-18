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
	attribute misplacedOptional = (1, 2, 3)->collect VectorFunctions::sum0;
}
`

const bodyFunctionReferenceBudgetModel = `
package test {
	private import ScalarValues::*;
	attribute large : Integer = (1..10000)->reduce '+';
}
`

// TestRuntimeRobustnessBodyFunctionReference pins what a collection operation
// refuses in place of its body: a function of the wrong arity (an optional input
// before a required one included), a value that is no function; that a function
// over no elements is not applied; and that each application of a library function
// is a step of the run, so a large reduction stops at the step budget.
func TestRuntimeRobustnessBodyFunctionReference(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, bodyFunctionReferenceRobustnessModel))
	root := idx.DocumentRoot("<test>")
	for _, tc := range []struct {
		name, expr string
		want       error
	}{
		{"function_of_the_wrong_arity", "test::unary", ErrBodyArity},
		{"value_that_is_no_function", "test::notAFunction", ErrTypeMismatch},
		{"optional_input_before_a_required_one", "test::misplacedOptional", ErrBodyArity},
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
	t.Run("library_function_applications_are_steps", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<budget>", parseAndBuild(t, bodyFunctionReferenceBudgetModel))
		ctx.maxSteps = 100
		_, err := evalIn(t, ctx, idx.DocumentRoot("<budget>"), "test::large")
		if !errors.Is(err, ErrStepLimitExceeded) {
			t.Fatalf("test::large under 100 steps: %v, want %v", err, ErrStepLimitExceeded)
		}
	})
}
