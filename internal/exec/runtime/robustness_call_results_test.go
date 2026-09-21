package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessCallResults exercises the failure modes of a synchronous
// call (StateExecutor.Call): held, untaken, empty, repeated and erroring calls.
func TestRuntimeRobustnessCallResults(t *testing.T) {
	t.Run("call_left_deferred", testCallResultsLeftDeferred)
	t.Run("call_left_queued_behind_termination", testCallResultsLeftQueued)
	t.Run("call_nothing_takes_returns_empty", testCallResultsNothingTakes)
	t.Run("results_do_not_carry_over", testCallResultsDoNotCarryOver)
	t.Run("dispatch_error_reaches_the_caller", testCallResultsDispatchError)
}

const callResultsModel = `package test {
	action def Answer {
		out result : Integer;
		first start;
		action answering { assign result := 42; }
		done;
		succession first start then answering;
		succession first answering then done;
	}
	state def Machine {
		attribute result : Integer = 0;
		attribute divisor : Integer = 0;
		entry; then idle;
		state idle;
		state holding { defer ask(); }
		state answered;
		state quiet;
		transition first idle accept ask() do perform Answer then answered;
		transition first idle accept Hold then holding;
		transition first idle accept Finish then done;
		transition first answered accept ask() then quiet;
		transition first quiet accept ask() do assign result := result / divisor then done;
	}
}`

// callResultsMachine creates an executor of the model's machine on a fresh context.
func callResultsMachine(t *testing.T) *StateExecutor {
	t.Helper()
	m := parseExploreModel(t, callResultsModel)
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	exec, err := ctx.CreateStateExecutorFor(m.state(t, "Machine"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(exec.Release)
	return exec
}

// testCallResultsLeftDeferred: a call the active state defers holds its caller,
// so Call refuses to return and says the call is deferred.
func testCallResultsLeftDeferred(t *testing.T) {
	exec := callResultsMachine(t)
	if err := exec.Enqueue(QueuedEvent{Signal: "Hold"}); err != nil {
		t.Fatal(err)
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatal(err)
	}
	_, err := exec.Call("ask", nil)
	if !errors.Is(err, ErrCallNotReturned) || !strings.Contains(err.Error(), "ask is still deferred") {
		t.Fatalf("Call = %v, want ErrCallNotReturned naming the deferred call", err)
	}
	if n := len(exec.DeferredEvents()); n != 1 {
		t.Errorf("%d deferred events, want the call held", n)
	}
}

// testCallResultsLeftQueued: a call queued behind an event that terminates the
// machine is never dispatched, and Call says it is still queued.
func testCallResultsLeftQueued(t *testing.T) {
	exec := callResultsMachine(t)
	if err := exec.Enqueue(QueuedEvent{Signal: "Finish"}); err != nil {
		t.Fatal(err)
	}
	_, err := exec.Call("ask", nil)
	if !errors.Is(err, ErrCallNotReturned) || !strings.Contains(err.Error(), "ask is still queued") {
		t.Fatalf("Call = %v, want ErrCallNotReturned naming the queued call", err)
	}
	if got := exec.Outcome().FinalState; got != "done" {
		t.Errorf("final state %q, want done", got)
	}
}

// testCallResultsNothingTakes: a call no transition of the configuration takes
// is consumed and dropped; its caller is released with no results.
func testCallResultsNothingTakes(t *testing.T) {
	exec := callResultsMachine(t)
	results, err := exec.Call("other", nil)
	if err != nil {
		t.Fatalf("Call = %v, want the caller released", err)
	}
	if results == nil || len(results) != 0 {
		t.Errorf("results = %v, want an empty map", results)
	}
	if got := exec.Outcome().FinalState; got != "idle" {
		t.Errorf("final state %q, want idle", got)
	}
}

// testCallResultsDoNotCarryOver: the results of one call belong to it alone; a
// later call whose dispatch returns nothing gets nothing, though the machine's
// attribute still holds the earlier value.
func testCallResultsDoNotCarryOver(t *testing.T) {
	exec := callResultsMachine(t)
	first, err := exec.Call("ask", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := first["result"]; !ok || got.Const.Int != 42 {
		t.Fatalf("first call returned %v, want result = 42", first)
	}
	second, err := exec.Call("ask", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 0 {
		t.Errorf("second call returned %v, want nothing", second)
	}
	if got := exec.Outcome().FinalState; got != "quiet" {
		t.Errorf("final state %q, want quiet", got)
	}
	if got, ok := exec.Outcome().Outputs["result"]; !ok || got.Const.Int != 42 {
		t.Errorf("machine result = %v, want 42 kept from the first call", got)
	}
}

// testCallResultsDispatchError: an error the effect of the call's transition
// raises ends the run and reaches the caller instead of any results.
func testCallResultsDispatchError(t *testing.T) {
	exec := callResultsMachine(t)
	for range 2 {
		if _, err := exec.Call("ask", nil); err != nil {
			t.Fatal(err)
		}
	}
	results, err := exec.Call("ask", nil)
	if err == nil || !strings.Contains(err.Error(), "division by zero") {
		t.Fatalf("Call = (%v, %v), want the division by zero", results, err)
	}
}
