package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessCalcUsage covers calculation usages and outputs: unbound inputs,
// unknown or cyclic outputs, outputs assigned twice or never, and nested usages.
func TestRuntimeRobustnessCalcUsage(t *testing.T) {
	t.Run("calc_non_boolean_condition", testCalcNonBooleanCondition)
	t.Run("calc_usage_unbound_input", testCalcUsageUnboundInput)
	t.Run("calc_usage_unknown_output", testCalcUsageUnknownOutput)
	t.Run("calc_usage_cyclic_outputs", testCalcUsageCyclicOutputs)
	t.Run("calc_usage_specializes_a_non_calc", testCalcUsageSpecializesANonCalc)
	t.Run("calc_usage_step_budget", testCalcUsageStepBudget)
	t.Run("calc_usage_output_without_a_value", testCalcUsageOutputWithoutAValue)
	t.Run("calc_output_never_assigned_by_the_body", testCalcOutputNeverAssignedByTheBody)
	t.Run("calc_output_assigned_in_a_branch_not_taken", testCalcOutputAssignedInABranchNotTaken)
	t.Run("calc_output_valued_and_assigned", testCalcOutputValuedAndAssigned)
	t.Run("calc_output_assigned_twice", testCalcOutputAssignedTwice)
	t.Run("calc_output_binding_violates_declared_type", testCalcOutputBindingViolatesDeclaredType)
	t.Run("operation_constraint_body_cannot_be_evaluated", testOperationConstraintBodyCannotBeEvaluated)
	t.Run("nested_calc_usage_unbound_input", testNestedCalcUsageUnboundInput)
	t.Run("nested_calc_usage_unknown_output", testNestedCalcUsageUnknownOutput)
	t.Run("nested_calc_usage_self_cycle", testNestedCalcUsageSelfCycle)
	t.Run("nested_calc_usage_recursion_depth", testNestedCalcUsageRecursionDepth)
	t.Run("nested_calc_usage_step_budget", testNestedCalcUsageStepBudget)
	t.Run("multiple_outputs_invoked_as_an_expression", testMultipleOutputsInvokedAsAnExpression)
}

// testCalcNonBooleanCondition: a condition that is not Boolean is a type error
// the typecheck pass reports; an execution that reaches one anyway says so
// rather than coercing the value.
func testCalcNonBooleanCondition(t *testing.T) {
	src := `
		package test {
			calc counting {
				in n: Integer;
				while n {
					return : Integer = 1;
				}
				return : Integer = 0;
			}
		}
	`
	err := invokeCalcInSource(t, src, "counting", 3, 10000)
	if err == nil || !strings.Contains(err.Error(), "must evaluate to a Boolean") {
		t.Errorf("expected a non-Boolean condition error, got: %v", err)
	}
}

// testCalcUsageUnboundInput: a usage that leaves an input of its calc with
// neither a value nor a default computes nothing, since a usage passes no
// arguments to stand in for one.
func testCalcUsageUnboundInput(t *testing.T) {
	src := `
		package test {
			calc def Two {
				in n : Integer[1];
				out a = n + 1;
				out b = n * 2;
			}
			calc c : Two;
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "a", 10000)
	if !errors.Is(err, errUnboundParameter) {
		t.Errorf("expected errUnboundParameter, got: %v", err)
	}
}

// testCalcUsageUnknownOutput: a name the calc declares no output for is a
// modeling error, not an empty value.
func testCalcUsageUnknownOutput(t *testing.T) {
	src := `
		package test {
			calc def Two {
				in n : Integer[1];
				out a = n + 1;
				out b = n * 2;
			}
			calc c : Two { in n = 5; }
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "nope", 10000)
	if !errors.Is(err, errUnknownOutput) {
		t.Errorf("expected errUnknownOutput, got: %v", err)
	}
	if err != nil && !strings.Contains(err.Error(), "a, b") {
		t.Errorf("error should name the outputs the calc does declare, got: %v", err)
	}
}

// testCalcUsageCyclicOutputs: outputs valued from each other have no value to
// compute, which is reported as the cycle it is rather than spending the step
// budget or hanging.
func testCalcUsageCyclicOutputs(t *testing.T) {
	src := `
		package test {
			calc def Knot {
				in n : Integer;
				out a = b + 1;
				out b = a + n;
			}
			calc c : Knot { in n = 1; }
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "a", 10000)
	if !errors.Is(err, errCyclicOutput) {
		t.Errorf("expected errCyclicOutput, got: %v", err)
	}
}

// testCalcUsageSpecializesANonCalc: a calc usage typed by something that is not
// a calc inherits no parameters, outputs or body from it, so the specialization
// is reported rather than the outputs it appears to be missing.
func testCalcUsageSpecializesANonCalc(t *testing.T) {
	src := `
		package test {
			part def Chassis {
				attribute mass : Integer = 4;
			}
			calc c : Chassis;
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "mass", 10000)
	if !errors.Is(err, ErrNotACalc) {
		t.Errorf("expected ErrNotACalc, got: %v", err)
	}
}

// testCalcUsageStepBudget: a usage whose body never terminates spends the step
// budget of the run reading its output, so the read fails instead of hanging
// whoever drove it.
func testCalcUsageStepBudget(t *testing.T) {
	src := `
		package test {
			calc def Spin {
				in n : Integer;
				attribute i : Integer = 0;
				while i >= 0 {
					assign i := i + 1;
				}
				out reached = i;
			}
			calc c : Spin { in n = 1; }
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "reached", 20)
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("expected ErrStepLimitExceeded, got: %v", err)
	}
}

// testCalcUsageOutputWithoutAValue: an output the calc declares but binds no
// value to computes nothing, so reading it says so rather than answering null.
func testCalcUsageOutputWithoutAValue(t *testing.T) {
	src := `
		package test {
			calc def Half {
				in n : Integer;
				out a = n + 1;
				out b : Integer;
			}
			calc c : Half { in n = 5; }
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "b", 10000)
	if !errors.Is(err, ErrNoValue) {
		t.Errorf("expected ErrNoValue, got: %v", err)
	}
}

// testCalcOutputNeverAssignedByTheBody: a body that assigns one of two declared
// outputs leaves the other unbound, and the read says that rather than blaming a
// missing result expression.
func testCalcOutputNeverAssignedByTheBody(t *testing.T) {
	src := `
		package test {
			calc def Two {
				in n : Integer;
				out a : Integer;
				out b : Integer;
				assign a := n + 1;
			}
			calc c : Two { in n = 5; }
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "b", 10000)
	if !errors.Is(err, errOutputNotAssigned) {
		t.Errorf("expected errOutputNotAssigned, got: %v", err)
	}
	if errors.Is(err, errNoResultExpression) {
		t.Errorf("expected no result-expression blame, got: %v", err)
	}
}

// testCalcOutputAssignedInABranchNotTaken: an output only a branch that does not
// run would assign is unbound for that activation.
func testCalcOutputAssignedInABranchNotTaken(t *testing.T) {
	src := `
		package test {
			calc def Branch {
				in n : Integer;
				out a : Integer;
				if n > 10 {
					assign a := n;
				}
			}
			calc c : Branch { in n = 5; }
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "a", 10000)
	if !errors.Is(err, errOutputNotAssigned) {
		t.Errorf("expected errOutputNotAssigned, got: %v", err)
	}
}

// testCalcOutputValuedAndAssigned: an output given a value two ways is reported
// rather than silently picking one (the precedent of #127/#131).
func testCalcOutputValuedAndAssigned(t *testing.T) {
	src := `
		package test {
			calc def Both {
				in n : Integer;
				out a : Integer = n;
				assign a := n + 1;
			}
			calc c : Both { in n = 5; }
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "a", 10000)
	if !errors.Is(err, errConflictingOutput) {
		t.Errorf("expected errConflictingOutput, got: %v", err)
	}
}

// testCalcOutputBindingViolatesDeclaredType: an output whose declaration binds
// it a value of another type is rejected where a write of one is, the input it
// reads being untyped so only the run time can judge it.
func testCalcOutputBindingViolatesDeclaredType(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			calc def Bound {
				in n;
				out a : Integer = n;
			}
			calc c : Bound { in n = "seven"; }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	usage := oneSymbol(t, idx, "test::c")
	_, err := ctx.calcUsageOutput(usage, "a", idx.DocumentRoot("<test>"), nil)
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("error = %v, want ErrTypeMismatch", err)
	}
	if !strings.Contains(err.Error(), "Integer") {
		t.Errorf("error = %v, want it to name the declared type", err)
	}
}

// testCalcOutputAssignedTwice: a body assigning an output more than once leaves
// it bound to the last assignment that ran, the same as a body local, so an
// output may be initialized and then accumulated into.
func testCalcOutputAssignedTwice(t *testing.T) {
	src := `
		package test {
			calc def Twice {
				in n : Integer;
				out a : Integer;
				assign a := n + 1;
				assign a := a + 1;
			}
			calc c : Twice { in n = 5; }
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	ctx.maxSteps = 10000
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "c", ast.DefCalc)
	if sym == nil {
		t.Fatal("calc usage c not found")
	}

	value, err := ctx.calcUsageOutput(sym, "a", sym.OwnerScope, nil)
	if err != nil {
		t.Fatalf("reading output a of c: %v", err)
	}
	if got := FormatTraceValue(value); got != "7" {
		t.Errorf("output a = %s, want 7", got)
	}
}

// testMultipleOutputsInvokedAsAnExpression: an invocation of a function yields
// exactly one result (KerML 7.4.9), so invoking a calc that computes several
// outputs and designates no result is reported rather than answered with
// whichever output happens to come first.
func testMultipleOutputsInvokedAsAnExpression(t *testing.T) {
	src := `
		package test {
			calc def Two {
				in n : Integer[1];
				out a = n + 1;
				out b = n * 2;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "Two", ast.DefCalc)
	if sym == nil {
		t.Fatal("Two calc not found")
	}

	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 5}}
	result, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
	if err == nil {
		t.Fatalf("expected the invocation to be rejected, it answered %s", FormatTraceValue(result))
	}
	if !errors.Is(err, errAmbiguousResult) {
		t.Errorf("expected errAmbiguousResult, got: %v", err)
	}
	// The diagnostic has to teach the spelling that does work.
	if err != nil && !strings.Contains(err.Error(), "calc c : test::Two { in n = ...; } then read c.a") {
		t.Errorf("error should spell out the calc usage to declare, got: %v", err)
	}
}

// testNestedCalcUsageUnboundInput: a usage nested in a calc binds its inputs
// from the enclosing evaluation, and an input nothing there values is reported.
func testNestedCalcUsageUnboundInput(t *testing.T) {
	src := `
		package test {
			calc def Two {
				in n : Integer[1];
				out a = n + 1;
				out b = n * 2;
			}
			calc def Outer {
				in m : Integer[1];
				calc inner : Two;
				out d = inner.a;
			}
			calc c : Outer { in m = 5; }
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "d", 10000)
	if !errors.Is(err, errUnboundParameter) {
		t.Errorf("expected errUnboundParameter, got: %v", err)
	}
}

// testNestedCalcUsageUnknownOutput: reading a name the nested calc declares no
// output for is a modeling error wherever the usage is declared.
func testNestedCalcUsageUnknownOutput(t *testing.T) {
	src := `
		package test {
			calc def Two {
				in n : Integer[1];
				out a = n + 1;
				out b = n * 2;
			}
			calc def Outer {
				in m : Integer[1];
				calc inner : Two { in n = m; }
				out d = inner.nope;
			}
			calc c : Outer { in m = 5; }
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "d", 10000)
	if !errors.Is(err, errUnknownOutput) {
		t.Errorf("expected errUnknownOutput, got: %v", err)
	}
}

// testNestedCalcUsageSelfCycle: an input of a nested usage valued from its own
// name with nothing outside to resolve to stays the cycle it is, rather than
// being read as the shadowing binding it looks like.
func testNestedCalcUsageSelfCycle(t *testing.T) {
	src := `
		package test {
			calc def Two {
				in n : Integer[1];
				out a = n + 1;
				out b = n * 2;
			}
			calc def Outer {
				calc inner : Two { in n = n; }
				out d = inner.a;
			}
			calc c : Outer;
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "d", 10000)
	if !errors.Is(err, ErrCyclicFeatureValue) {
		t.Errorf("expected ErrCyclicFeatureValue, got: %v", err)
	}
}

// testNestedCalcUsageRecursionDepth: a calc whose nested usage is of itself
// never bottoms out, so the depth budget reports it instead of hanging. A usage
// frame costs far more than an invocation, so the case states a shallow budget.
func testNestedCalcUsageRecursionDepth(t *testing.T) {
	src := `
		package test {
			calc def Down {
				in n : Integer;
				calc next : Down { in n = n - 1; }
				out a = next.a;
				out b = n;
			}
			calc c : Down { in n = 3; }
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "a", 1000000,
		func(ctx *Context) { ctx.maxCalcDepth = nestingProbeDepth })
	if !errors.Is(err, ErrCalcRecursionLimit) {
		t.Errorf("expected ErrCalcRecursionLimit, got: %v", err)
	}
}

// testNestedCalcUsageStepBudget: the body of a nested usage spends the budget of
// the run reading it, so a body that never terminates fails the read.
func testNestedCalcUsageStepBudget(t *testing.T) {
	src := `
		package test {
			calc def Spin {
				in n : Integer;
				attribute i : Integer = 0;
				while i >= 0 {
					assign i := i + 1;
				}
				out reached = i;
			}
			calc def Outer {
				in m : Integer[1];
				calc inner : Spin { in n = m; }
				out d = inner.reached;
			}
			calc c : Outer { in m = 1; }
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "d", 20)
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("expected ErrStepLimitExceeded, got: %v", err)
	}
}

// testOperationConstraintBodyCannotBeEvaluated: invoking a constraint whose
// assertion names no feature returns its evaluation error without panicking.
func testOperationConstraintBodyCannotBeEvaluated(t *testing.T) {
	src := `
	package test {
		part def Tank {
			constraint broken { missing > 0 }
		}
	}`
	ctx, inst, err := instantiateInSource(t, src, "test::Tank")
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	_, err = ctx.invokeOperation(inst, "broken", nil)
	if err == nil {
		t.Fatal("expected an error for an unevaluable constraint body")
	}
	if !errors.Is(err, ErrUnresolvedReference) {
		t.Fatalf("error = %v, want ErrUnresolvedReference", err)
	}
}
