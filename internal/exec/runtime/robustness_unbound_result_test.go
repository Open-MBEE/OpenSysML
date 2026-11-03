package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessUnboundResult requires a calc result nothing binds to be
// reported as no value, exactly as an `out` the body never assigns is: the spec
// says only how a result is bound (KerML 7.4.8.2, SysML 7.19.2), so a result
// left unbound has no value asserted and is not a distinct kind of failure.
func TestRuntimeRobustnessUnboundResult(t *testing.T) {
	t.Run("result_never_bound_reads_as_an_unassigned_out", testUnboundResultReadsAsUnassignedOut)
	t.Run("body_falls_through_is_no_value", testBodyFallsThroughIsNoValue)
	t.Run("no_result_expression_keeps_the_hint", testNoResultExpressionKeepsTheHint)
	t.Run("empty_sequence_result_is_a_value", testEmptySequenceResultIsAValue)
	t.Run("unevaluable_result_is_not_no_value", testUnevaluableResultIsNotNoValue)
}

// testUnboundResultReadsAsUnassignedOut: a calc usage whose `return` is never
// bound and one whose `out` is never assigned fail the same way.
func testUnboundResultReadsAsUnassignedOut(t *testing.T) {
	src := `
		package test {
			calc def Unbound {
				in n : Integer;
				return r : Integer;
			}
			calc def Unassigned {
				in n : Integer;
				out o : Integer;
				return : Integer = n;
			}
			calc u : Unbound { in n = 5; }
			calc a : Unassigned { in n = 5; }
		}
	`
	result := calcUsageOutputInSource(t, src, "u", "r", 10000)
	output := calcUsageOutputInSource(t, src, "a", "o", 10000)
	for name, err := range map[string]error{"result": result, "output": output} {
		if !errors.Is(err, ErrNoValue) {
			t.Errorf("%s never bound = %v, want %v", name, err, ErrNoValue)
		}
	}
	if !strings.Contains(result.Error(), "result parameter r binds no value") {
		t.Errorf("unbound result = %v, want it to name the result parameter", result)
	}
	if !errors.Is(output, errOutputNotAssigned) {
		t.Errorf("unassigned output = %v, want %v", output, errOutputNotAssigned)
	}
}

// testBodyFallsThroughIsNoValue: a body that runs to its end on a path with no
// `return` leaves the result unbound, which is no value, not a null result.
func testBodyFallsThroughIsNoValue(t *testing.T) {
	src := `
		package test {
			calc maybe {
				in n : Integer;
				if n < 0 {
					return : Integer = n;
				}
			}
		}
	`
	err := invokeCalcInSource(t, src, "maybe", 5, 10000)
	if !errors.Is(err, ErrNoValue) {
		t.Fatalf("fall-through = %v, want %v", err, ErrNoValue)
	}
	if !strings.Contains(err.Error(), "test::maybe ended without a return") {
		t.Errorf("fall-through = %v, want it to say which calc ended without a return", err)
	}
}

// testNoResultExpressionKeepsTheHint: the no-value error of a calc with no
// result expression still says how to state one.
func testNoResultExpressionKeepsTheHint(t *testing.T) {
	src := `
		package test {
			calc plain {
				in n : Integer;
			}
			calc declared {
				in n : Integer;
				attribute h : Integer = n;
				return h;
			}
		}
	`
	plain := invokeCalcInSource(t, src, "plain", 5, 10000)
	if !errors.Is(plain, ErrNoValue) || plain.Error() != "no value: calc test::plain has no return expression" {
		t.Errorf("no result expression = %v, want the no-value error naming the calc", plain)
	}
	declared := invokeCalcInSource(t, src, "declared", 5, 10000)
	if !errors.Is(declared, ErrNoValue) {
		t.Errorf("unbound result parameter = %v, want %v", declared, ErrNoValue)
	}
	for _, detail := range []string{"result parameter h binds no value", "trailing expression `h`", "return : Integer = h;"} {
		if !strings.Contains(declared.Error(), detail) {
			t.Errorf("unbound result parameter = %v, want the hint %q", declared, detail)
		}
	}
}

// testEmptySequenceResultIsAValue: returning an empty sequence binds the result
// to nothing-many, which is a value; only a result never bound is no value.
func testEmptySequenceResultIsAValue(t *testing.T) {
	src := `
		package test {
			calc none {
				in n : Integer;
				return : Integer[0..*] = ();
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	sym, scope := calcByName(t, idx.DocumentRoot("<test>"), "test", "none")
	result, err := ctx.InvokeCalc(sym, []Value{constInt(1)}, scope)
	if err != nil {
		t.Fatalf("InvokeCalc(none) = %v, want the empty sequence", err)
	}
	if result.Kind != ValSequence || result.Sequence().Size() != 0 {
		t.Errorf("InvokeCalc(none) = %s, want an empty sequence", FormatTraceValue(result))
	}
}

// testUnevaluableResultIsNotNoValue: a result the evaluator cannot compute is
// reported as that failure; it is bound, so it is not no value.
func testUnevaluableResultIsNotNoValue(t *testing.T) {
	src := `
		package test {
			calc divide {
				in n : Integer;
				return : Integer = n / 0;
			}
		}
	`
	err := invokeCalcInSource(t, src, "divide", 5, 10000)
	if err == nil || errors.Is(err, ErrNoValue) {
		t.Errorf("division by zero = %v, want its own error, not no value", err)
	}
}
