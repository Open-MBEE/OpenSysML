package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessAcceptAfterParameter exercises `accept after` over a
// parameter rather than a literal: a parameter bound to one duration times the
// wait, one holding no duration or several is a typed error naming the count,
// and a bound non-duration is still refused as before. Never a panic or a hang.
func TestRuntimeRobustnessAcceptAfterParameter(t *testing.T) {
	t.Run("bound_parameter_times_the_wait", testBoundParameterTimesTheWait)
	t.Run("unbound_parameter_is_refused", testUnboundParameterIsRefused)
	t.Run("multivalued_parameter_is_refused", testMultivaluedParameterIsRefused)
	t.Run("string_parameter_is_refused", testStringParameterIsRefused)
}

const acceptAfterParameterModel = `
	package test {
		private import ScalarValues::*;
		private import SI::*;
		private import ISQ::*;
		action def Wait {
			in howLong : DurationValue;
			out ended : Real = 0.0;
			first start;
			then action w accept after howLong;
			then assign ended := localClock.currentTime;
			then done;
		}
		action Bound {
			out ended : Real = 0.0;
			first start;
			then action w : Wait { in howLong = 3.0 [s]; }
			then assign ended := w.ended;
			then done;
		}
		action Unbound {
			first start;
			then action w : Wait;
			then done;
		}
		action Many {
			first start;
			then action w : Wait { in howLong = (1.0 [s], 2.0 [s]); }
			then done;
		}
		action Text {
			in word : String = "soon";
			first start;
			then action w accept after word;
			then done;
		}
	}`

// testBoundParameterTimesTheWait: the duration a performing node binds to the
// parameter is the wait, so the clock reads it when the accept fires.
func testBoundParameterTimesTheWait(t *testing.T) {
	m := parseLibraryModel(t, acceptAfterParameterModel)
	_, out, err := runUnderStep(t, m, "Bound", 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := soleRealOutputs(t, out, "ended"); got[0] != 3.0 {
		t.Errorf("ended = %v, want 3", got[0])
	}
}

// testUnboundParameterIsRefused: a parameter holding no duration cannot time a
// wait; the accept is refused as a typed error naming the empty value.
func testUnboundParameterIsRefused(t *testing.T) {
	m := parseLibraryModel(t, acceptAfterParameterModel)
	_, _, err := runUnderStep(t, m, "Unbound", 0)
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("err = %v, want ErrTypeMismatch", err)
	}
	if !strings.Contains(err.Error(), "must be one value, got 0") {
		t.Errorf("err = %v, want it to name the empty value", err)
	}
}

// testMultivaluedParameterIsRefused: two durations bound to one parameter time
// no single wait; the accept is refused naming how many it holds.
func testMultivaluedParameterIsRefused(t *testing.T) {
	m := parseLibraryModel(t, acceptAfterParameterModel)
	_, _, err := runUnderStep(t, m, "Many", 0)
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("err = %v, want ErrTypeMismatch", err)
	}
	if !strings.Contains(err.Error(), "must be one value, got 2") {
		t.Errorf("err = %v, want it to count the values", err)
	}
}

// testStringParameterIsRefused: unwrapping a parameter's single value does not
// admit a non-duration; a string is refused as it always was.
func testStringParameterIsRefused(t *testing.T) {
	m := parseLibraryModel(t, acceptAfterParameterModel)
	_, _, err := runUnderStep(t, m, "Text", 0)
	if err == nil {
		t.Fatal("expected an error for a string duration")
	}
	if !strings.Contains(err.Error(), "must be") {
		t.Errorf("err = %v, want a refusal of the string", err)
	}
}
