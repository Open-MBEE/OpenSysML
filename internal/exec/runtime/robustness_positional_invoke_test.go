package runtime

import (
	"errors"
	"testing"
)

// TestRuntimeRobustnessPositionalInvoke exercises the failure modes of an operation
// invoked with a positional argument list: a surplus argument, a list mixing the
// positional and the named form, and a list no same-named operation takes.
func TestRuntimeRobustnessPositionalInvoke(t *testing.T) {
	t.Run("surplus_positional_argument", testPositionalInvokeSurplus)
	t.Run("surplus_over_an_out_parameter", testPositionalInvokeSurplusOverOut)
	t.Run("positional_and_named_mixed", testPositionalInvokeMixed)
	t.Run("required_parameter_left_unbound", testPositionalInvokeTooFew)
	t.Run("no_overload_of_that_arity", testPositionalInvokeNoOverload)
}

// positionalInvokeSource declares operations with a defaulted, an `out` and an
// `inout` parameter, and two calcs of one name that differ in arity.
const positionalInvokeSource = `
	package test {
		private import ScalarValues::*;
		part def Adder {
			attribute total : Integer = 0;
			action add { in addend : Integer; in times : Integer = 1; out sum : Integer;
				first apply; action apply { assign total := total + addend * times; assign sum := total; } }
			action moveTo { in amount : Integer; inout sink : Integer;
				first apply; action apply { assign sink := sink + amount; } }
			calc scaled { in factor : Integer; return : Integer = total * factor; }
			calc scaled { in factor : Integer; in offset : Integer; return : Integer = total * factor + offset; }
		}
	}`

func positionalInvokeObject(t *testing.T) (*Context, *Instance) {
	t.Helper()
	ctx, inst, err := instantiateWithLibraries(t, positionalInvokeSource, "test::Adder")
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	return ctx, inst
}

// testPositionalInvokeSurplus: a third argument to an operation of two input
// parameters is an arity error, not a value bound nowhere.
func testPositionalInvokeSurplus(t *testing.T) {
	ctx, inst := positionalInvokeObject(t)
	_, err := ctx.InvokeOperationWith(inst, "add", OperationArguments{
		Positional: []Value{intArgument(1), intArgument(2), intArgument(3)},
	})
	if !errors.Is(err, ErrOperationArity) {
		t.Fatalf("add(1, 2, 3): %v, want ErrOperationArity", err)
	}
}

// testPositionalInvokeSurplusOverOut: an `out` parameter takes no position, so an
// argument aimed at it is a surplus.
func testPositionalInvokeSurplusOverOut(t *testing.T) {
	ctx, inst := positionalInvokeObject(t)
	_, err := ctx.InvokeOperationWith(inst, "moveTo", OperationArguments{
		Positional: []Value{intArgument(1), intArgument(2), intArgument(3)},
	})
	if !errors.Is(err, ErrOperationArity) {
		t.Fatalf("moveTo(1, 2, 3): %v, want ErrOperationArity", err)
	}
}

// testPositionalInvokeMixed: an argument list is positional or named, never both,
// even when the two would bind different parameters.
func testPositionalInvokeMixed(t *testing.T) {
	ctx, inst := positionalInvokeObject(t)
	_, err := ctx.InvokeOperationWith(inst, "add", OperationArguments{
		Positional: []Value{intArgument(1)},
		Named:      map[string]Value{"times": intArgument(2)},
	})
	if !errors.Is(err, ErrMixedArguments) {
		t.Fatalf("add(1, times=2): %v, want ErrMixedArguments", err)
	}
	if fv, err := inst.GetFeatureValue(ctx, "total"); err != nil || fv.HeldValue().Const.Int != 0 {
		t.Fatalf("total after a refused invocation = %v, %v, want 0", fv, err)
	}
}

// testPositionalInvokeTooFew: an empty positional list leaves the default-less
// first parameter unbound; only the trailing defaulted one may be omitted.
func testPositionalInvokeTooFew(t *testing.T) {
	ctx, inst := positionalInvokeObject(t)
	_, err := ctx.InvokeOperationWith(inst, "moveTo", OperationArguments{
		Positional: []Value{intArgument(1)},
	})
	if !errors.Is(err, ErrUnboundParameter) {
		t.Fatalf("moveTo(1): %v, want ErrUnboundParameter", err)
	}
}

// testPositionalInvokeNoOverload: with two same-named calcs taking one and two
// arguments, three arguments select neither and report the arity.
func testPositionalInvokeNoOverload(t *testing.T) {
	ctx, inst := positionalInvokeObject(t)
	_, err := ctx.InvokeOperationWith(inst, "scaled", OperationArguments{
		Positional: []Value{intArgument(1), intArgument(2), intArgument(3)},
	})
	if !errors.Is(err, ErrOperationArity) {
		t.Fatalf("scaled(1, 2, 3): %v, want ErrOperationArity", err)
	}
}
