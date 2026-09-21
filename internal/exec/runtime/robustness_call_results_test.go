package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessCallResults exercises the failure modes of a synchronous
// call (StateExecutor.Call): held, untaken, empty, repeated and erroring calls,
// and the caller released at its own step's end, ahead of later events and timers.
func TestRuntimeRobustnessCallResults(t *testing.T) {
	t.Run("call_left_deferred", testCallResultsLeftDeferred)
	t.Run("deferred_call_returns_once_recalled", testCallResultsDeferredRecalled)
	t.Run("call_left_queued_behind_termination", testCallResultsLeftQueued)
	t.Run("call_nothing_takes_returns_empty", testCallResultsNothingTakes)
	t.Run("results_do_not_carry_over", testCallResultsDoNotCarryOver)
	t.Run("dispatch_error_reaches_the_caller", testCallResultsDispatchError)
	t.Run("released_before_the_completion_step", testCallResultsReleasedBeforeCompletion)
	t.Run("timer_after_the_step_is_not_drained", testCallResultsTimerNotDrained)
	t.Run("queued_signal_is_dispatched_before_the_call", testCallResultsQueuedSignalFirst)
	t.Run("declared_operation_returns_its_parameters_only", testCallResultsDeclaredReturns)
	t.Run("inout_argument_returns_as_passed_unless_written", testCallResultsInoutArgument)
	t.Run("overloaded_operation_returns_the_selected_declaration", testCallResultsOverloaded)
	t.Run("arguments_must_bind_the_declared_inputs", testCallResultsArguments)
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

// callDeferredModel: the step taking `Hold` sends `Release` behind the call and
// enters `holding`, which defers `ask` until `Release` returns the machine to `idle`.
const callDeferredModel = `package test {
	private import ScalarValues::*;
	attribute def Hold;
	attribute def Release;
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
		entry; then idle;
		state idle;
		state holding { defer ask(); }
		state answered;
		transition first idle accept Hold do send new Release() then holding;
		transition first holding accept Release then idle;
		transition first idle accept ask() do perform Answer then answered;
	}
}`

// callBoundaryModel: the step answering `ask` enters `answered`, whose completion
// transition leads to `later`, whose timer fires an effect that fails.
const callBoundaryModel = `package test {
	private import SI::*;
	private import ScalarValues::*;
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
		attribute poked : Boolean = false;
		entry; then idle;
		state idle;
		state answered;
		state later;
		transition first idle accept ask() do perform Answer then answered;
		transition first idle accept Poke do assign poked := true then idle;
		transition first answered then later;
		transition first later accept after 1 [s] do assign result := result / divisor then done;
	}
}`

// callOwnerModel: `Owner` declares `ask` returning `result` alone and `nudge`
// carrying `x` in and out; its machine answers `ask` and the undeclared `tell`
// with a helper that also writes `log`, takes `nudge` in `idle` without writing
// `x`, in `answered` with a helper that increments it, and never in `told`.
const callOwnerModel = `package P {
	private import ScalarValues::*;
	part def Owner {
		attribute log : String = "";
		attribute result : Integer = 0;
		action def ask { out result : Integer; }
		action def nudge { inout x : Integer; }
		action def Answer {
			inout log : String;
			out result : Integer;
			first start;
			action answering { assign result := 42; assign log := "answered"; }
			done;
			succession first start then answering;
			succession first answering then done;
		}
		action def Increment {
			inout x : Integer;
			first start;
			action incrementing { assign x := x + 1; }
			done;
			succession first start then incrementing;
			succession first incrementing then done;
		}
		exhibit state sm {
			entry; then idle;
			state idle;
			state answered;
			state told;
			transition first idle accept nudge(x) then idle;
			transition first idle accept ask() do action : Answer { inout log = log; } then answered;
			transition first answered accept nudge(x) do action : Increment { inout x = x; } then answered;
			transition first answered accept tell() do action : Answer { inout log = log; } then told;
		}
	}
	part owner : Owner;
}`

// callOverloadedModel: the owner sees two operations named `compute`, told apart
// by their input's type and declaring different outputs; the step answering the
// call writes both outputs.
const callOverloadedModel = `package P {
	private import ScalarValues::*;
	part def Owner {
		action def compute { in x : Integer; out n : Integer; }
		action def compute { in x : String; out s : String; }
		action def Both {
			out n : Integer;
			out s : String;
			first start;
			action writing { assign n := 1; assign s := "one"; }
			done;
			succession first start then writing;
			succession first writing then done;
		}
		exhibit state sm {
			entry; then idle;
			state idle;
			transition first idle accept compute(x) do perform Both then idle;
		}
	}
	part owner : Owner;
}`

// testCallResultsOverloaded: a call of an overloaded operation releases the
// outputs of the declaration its arguments select, and one telling the
// declarations apart by nothing is refused as ambiguous before it is queued.
func testCallResultsOverloaded(t *testing.T) {
	exec := callMachineOwnedBy(t, callOverloadedModel)
	got, err := exec.Call("compute", map[string]Value{"x": strValue("text")})
	if err != nil {
		t.Fatal(err)
	}
	if s, ok := got["s"]; !ok || s.Str() != "one" || len(got) != 1 {
		t.Errorf("compute(\"text\") returned %v, want s = \"one\" alone", got)
	}
	got, err = exec.Call("compute", map[string]Value{"x": constInt(3)})
	if err != nil {
		t.Fatal(err)
	}
	if n, ok := got["n"]; !ok || n.Const.Int != 1 || len(got) != 1 {
		t.Errorf("compute(3) returned %v, want n = 1 alone", got)
	}
	got, err = exec.Call("compute", nil)
	if !errors.Is(err, ErrAmbiguousInvocation) {
		t.Fatalf("compute() = %v, %v; want ErrAmbiguousInvocation", got, err)
	}
	if n := exec.eventQueue.Len(); n != 0 {
		t.Errorf("%d event(s) queued, want the ambiguous call refused before it is queued", n)
	}
}

const callArgumentsModel = `package P {
	private import ScalarValues::*;
	part def Owner {
		action def compute { in x : Integer; out n : Integer; }
		action def One {
			out n : Integer;
			first start;
			action writing { assign n := 1; }
			done;
			succession first start then writing;
			succession first writing then done;
		}
		exhibit state sm {
			entry; then idle;
			state idle;
			transition first idle accept compute() do perform One then idle;
		}
	}
	part owner : Owner;
}`

// testCallResultsArguments: a call of a declared operation is checked against
// its inputs as an invocation is — an unbound or unknown argument is refused
// before the call is queued, even where a trigger would accept the bare call.
func testCallResultsArguments(t *testing.T) {
	exec := callMachineOwnedBy(t, callArgumentsModel)
	for name, args := range map[string]map[string]Value{
		"none":    nil,
		"unknown": {"x": constInt(3), "y": constInt(4)},
	} {
		got, err := exec.Call("compute", args)
		if !errors.Is(err, ErrUnboundParameter) {
			t.Errorf("compute with %s argument(s) = %v, %v; want ErrUnboundParameter", name, got, err)
		}
		if n := exec.eventQueue.Len(); n != 0 {
			t.Errorf("%d event(s) queued after the %s call, want it refused before it is queued", n, name)
		}
	}
	got, err := exec.Call("compute", map[string]Value{"x": constInt(3)})
	if err != nil {
		t.Fatal(err)
	}
	if n, ok := got["n"]; !ok || n.Const.Int != 1 || len(got) != 1 {
		t.Errorf("compute(3) returned %v, want n = 1 alone", got)
	}
}

// callOwnerMachine creates an executor of callOwnerModel's machine on its owner.
func callOwnerMachine(t *testing.T) *StateExecutor {
	t.Helper()
	return callMachineOwnedBy(t, callOwnerModel)
}

// callMachineOwnedBy creates an executor of `P::Owner::sm` on `P::owner` of model.
func callMachineOwnedBy(t *testing.T, model string) *StateExecutor {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, model))
	owner, err := ctx.Instantiate(oneSymbol(t, idx, "P::owner"))
	if err != nil {
		t.Fatalf("instantiate owner: %v", err)
	}
	exec, err := ctx.CreateStateExecutorFor(oneSymbol(t, idx, "P::Owner::sm"), owner)
	if err != nil {
		t.Fatalf("create state executor: %v", err)
	}
	t.Cleanup(exec.Release)
	return exec
}

// callResultsMachine creates an executor of the model's machine on a fresh context.
func callResultsMachine(t *testing.T) *StateExecutor {
	t.Helper()
	return callMachineOf(t, callResultsModel)
}

// callMachineOf creates an executor of the model's `Machine` on a fresh context.
func callMachineOf(t *testing.T, model string) *StateExecutor {
	t.Helper()
	m := parseLibraryModel(t, model)
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

// testCallResultsDeferredRecalled: a call the active state defers holds its
// caller through the machine's later steps and returns once it is recalled.
func testCallResultsDeferredRecalled(t *testing.T) {
	exec := callMachineOf(t, callDeferredModel)
	if err := exec.Enqueue(QueuedEvent{Signal: "Hold"}); err != nil {
		t.Fatal(err)
	}
	results, err := exec.Call("ask", nil)
	if err != nil {
		t.Fatalf("Call = %v, want the caller released once the deferred call was recalled", err)
	}
	if got, ok := results["result"]; !ok || got.Const.Int != 42 {
		t.Errorf("Call returned %v, want result = 42", results)
	}
	if got := exec.Outcome().FinalState; got != "answered" {
		t.Errorf("final state %q, want answered", got)
	}
	if n := len(exec.DeferredEvents()); n != 0 {
		t.Errorf("%d deferred events, want none left", n)
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

// testCallResultsReleasedBeforeCompletion: the caller is released once the step
// dispatching its call ends; the completion event that step queued is a later step.
func testCallResultsReleasedBeforeCompletion(t *testing.T) {
	exec := callMachineOf(t, callBoundaryModel)
	results, err := exec.Call("ask", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := results["result"]; !ok || got.Const.Int != 42 {
		t.Fatalf("Call returned %v, want result = 42", results)
	}
	if got := exec.Outcome().FinalState; got != "answered" {
		t.Errorf("state after the call %q, want answered with its completion still queued", got)
	}
	if err := exec.RunToQuiescence(); err != nil {
		t.Fatal(err)
	}
	if got := exec.Outcome().FinalState; got != "later" {
		t.Errorf("state after the next run %q, want later", got)
	}
}

// testCallResultsTimerNotDrained: a timer the call's step arms does not run under
// the call; its later failure reaches the run that advances the clock, not the caller.
func testCallResultsTimerNotDrained(t *testing.T) {
	exec := callMachineOf(t, callBoundaryModel)
	results, err := exec.Call("ask", nil)
	if err != nil {
		t.Fatalf("Call = %v, want the result ahead of the timer's failure", err)
	}
	if got, ok := results["result"]; !ok || got.Const.Int != 42 {
		t.Fatalf("Call returned %v, want result = 42", results)
	}
	err = exec.RunToCompletion()
	if err == nil || !strings.Contains(err.Error(), "division by zero") {
		t.Fatalf("RunToCompletion = %v, want the timer's division by zero", err)
	}
}

// testCallResultsQueuedSignalFirst: a signal queued ahead of the call is dispatched
// first, in its own step, and the call still returns its step's results.
func testCallResultsQueuedSignalFirst(t *testing.T) {
	exec := callMachineOf(t, callBoundaryModel)
	if err := exec.Enqueue(QueuedEvent{Signal: "Poke"}); err != nil {
		t.Fatal(err)
	}
	results, err := exec.Call("ask", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := results["result"]; !ok || got.Const.Int != 42 {
		t.Fatalf("Call returned %v, want result = 42", results)
	}
	if len(results) != 1 {
		t.Errorf("Call returned %v, want the call's own result alone", results)
	}
	if got := exec.StateData()["poked"]; got.Kind != ValConst || !got.Const.Bool {
		t.Errorf("poked = %v, want the queued signal dispatched before the call", got)
	}
	if got := exec.Outcome().FinalState; got != "answered" {
		t.Errorf("state after the call %q, want answered", got)
	}
}

// testCallResultsDeclaredReturns: a call of an operation the owner declares
// releases its out parameters alone, so the helper's `log` stays the object's;
// a call the owner declares nothing for releases every output the step returned.
func testCallResultsDeclaredReturns(t *testing.T) {
	exec := callOwnerMachine(t)
	owner, ctx := exec.self, exec.ctx
	asked, err := exec.Call("ask", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := asked["result"]; !ok || got.Const.Int != 42 || len(asked) != 1 {
		t.Fatalf("ask returned %v, want result = 42 alone", asked)
	}
	if log, err := owner.GetFeatureValue(ctx, "log"); err != nil || log.Value.Str() != "answered" {
		t.Errorf("log = %v, %v; want the helper's write kept by the object", log, err)
	}
	told, err := exec.Call("tell", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := told["log"]; !ok || got.Str() != "answered" || len(told) != 2 {
		t.Errorf("tell returned %v, want both outputs of the undeclared operation", told)
	}
}

// testCallResultsInoutArgument: a declared inout goes back to the caller as it
// was passed when the step writes nothing to it, as the step's value when a
// behavior writes it, and not at all when no transition takes the call.
func testCallResultsInoutArgument(t *testing.T) {
	exec := callOwnerMachine(t)
	nudged, err := exec.Call("nudge", map[string]Value{"x": constInt(7)})
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := nudged["x"]; !ok || got.Const.Int != 7 || len(nudged) != 1 {
		t.Errorf("nudge in idle returned %v, want x = 7 as passed", nudged)
	}
	if _, err := exec.Call("ask", nil); err != nil {
		t.Fatal(err)
	}
	nudged, err = exec.Call("nudge", map[string]Value{"x": constInt(7)})
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := nudged["x"]; !ok || got.Const.Int != 8 || len(nudged) != 1 {
		t.Errorf("nudge in answered returned %v, want x = 8 from the helper", nudged)
	}
	if _, err := exec.Call("tell", nil); err != nil {
		t.Fatal(err)
	}
	nudged, err = exec.Call("nudge", map[string]Value{"x": constInt(7)})
	if err != nil {
		t.Fatal(err)
	}
	if len(nudged) != 0 {
		t.Errorf("nudge in told returned %v, want nothing for a call no transition takes", nudged)
	}
	if got := exec.Outcome().FinalState; got != "told" {
		t.Errorf("final state %q, want told", got)
	}
}
