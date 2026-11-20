package runtime

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

// TestRuntimeRobustnessAdvanceFailure covers a type-bound behavior failing while the clock
// runs it: it ends with the failure recorded, and the advance that ran it reports that failure.
func TestRuntimeRobustnessAdvanceFailure(t *testing.T) {
	t.Run("transition_effect_failing_on_a_message_fails_the_advance", testAdvanceReportsTransitionEffectFailure)
	t.Run("entry_action_failing_on_a_timed_transition_fails_the_advance", testAdvanceReportsEntryActionFailure)
	t.Run("the_failure_is_an_error_outcome_of_an_exploration", testAdvanceFailureIsExploredOutcome)
	t.Run("the_failed_behavior_fails_no_later_advance", testAdvanceFailureReportedOnce)
}

// relayedFailureModel is a worker whose machine indexes an empty sequence in the
// effect of a transition a sibling's timed send triggers through a port.
const relayedFailureModel = `package test {
	private import ScalarValues::*;
	private import SI::*;
	item def Ask;
	port def P { out item ask : Ask; }
	part def W {
		port p : ~P;
		attribute xs : Integer[0..*];
		attribute n : Integer = 0;
		exhibit state run {
			entry; then idle;
			state idle;
			transition first idle accept Ask via p do action { assign n := n + xs#(1); } then idle;
		}
	}
	part def S {
		port a : P;
		exhibit state run {
			entry; then s0;
			state s0;
			transition first s0 accept after 1 [SI::s] do action { send new Ask() via a; } then s0;
		}
	}
	part sys { part s : S; part w : W; connect s.a to w.p; }
}`

// timedEntryFailureModel is a machine whose timed transition enters a state
// whose entry action indexes an empty sequence.
const timedEntryFailureModel = `package test {
	private import ScalarValues::*;
	private import SI::*;
	part def W {
		attribute xs : Integer[0..*];
		attribute n : Integer = 0;
		exhibit state run {
			entry; then idle;
			state idle;
			state bad { entry action { assign n := n + xs#(1); } }
			transition first idle accept after 1 [SI::s] then bad;
		}
	}
	part sys { part w : W; }
}`

// workerMachine is the machine `run` the W of the object made exhibits.
func workerMachine(t *testing.T, ctx *Context) *ObjectBehavior {
	t.Helper()
	for _, b := range ctx.objectBehaviors {
		if b.Name == "run" && b.Object.Type != nil && b.Object.Type.Name == "W" {
			return b
		}
	}
	t.Fatal("no object of W exhibits run")
	return nil
}

// advanceFailing makes test::sys of src, which must start cleanly, and advances
// it by three seconds, returning the worker's machine and the advance's error.
func advanceFailing(t *testing.T, src string) (*Context, *ObjectBehavior, error) {
	t.Helper()
	ctx, _, err := instantiateWithLibraries(t, src, "test::sys")
	if err != nil {
		t.Fatalf("instantiate sys: %v", err)
	}
	machine := workerMachine(t, ctx)
	if machine.Err != nil || machine.State == nil {
		t.Fatalf("run before the clock moves: Err = %v, want a running machine", machine.Err)
	}
	_, err = ctx.Advance(3)
	return ctx, machine, err
}

func checkAdvanceFailure(t *testing.T, ctx *Context, machine *ObjectBehavior, err error, path string) {
	t.Helper()
	if !errors.Is(err, ErrIndexOutOfRange) || !strings.Contains(err.Error(), path) {
		t.Fatalf("Advance(3) = %v, want %v through %q", err, ErrIndexOutOfRange, path)
	}
	if !errors.Is(machine.Err, ErrIndexOutOfRange) || machine.Err.Error() != err.Error() {
		t.Errorf("machine.Err = %v, want the advance's error %v", machine.Err, err)
	}
	if l, _ := ctx.OccurrenceLife(machine.State.occurrence.ID); l.Ended == 0 {
		t.Error("the failed machine's performance life is un-ended")
	}
	if slices.Contains(ctx.clock.waiters, clockWaiter(machine.State)) {
		t.Error("the failed machine is still a clock waiter")
	}
}

func testAdvanceReportsTransitionEffectFailure(t *testing.T) {
	ctx, machine, err := advanceFailing(t, relayedFailureModel)
	checkAdvanceFailure(t, ctx, machine, err, "fire transition out of idle: transition effect")
}

func testAdvanceReportsEntryActionFailure(t *testing.T) {
	ctx, machine, err := advanceFailing(t, timedEntryFailureModel)
	checkAdvanceFailure(t, ctx, machine, err, "entry action")
}

func testAdvanceFailureIsExploredOutcome(t *testing.T) {
	m := parseLibraryModel(t, relayedFailureModel)
	sys := oneSymbol(t, m.idx, "test::sys")
	exploration, err := Explore(context.Background(), mustPolicy(t, "explore"), m.fresh, func(ctx *Context) (Outcome, error) {
		if _, err := instantiateReporting(ctx, sys); err != nil {
			return Outcome{}, err
		}
		if _, err := ctx.Advance(3); err != nil {
			return Outcome{}, err
		}
		return Outcome{}, nil
	})
	if err != nil {
		t.Fatalf("Explore: %v; want the failure as an outcome", err)
	}
	if exploration == nil || len(exploration.Outcomes) != 1 || exploration.FailedLinearizations() != 1 {
		t.Fatalf("exploration = %v; want one error outcome", exploration)
	}
	if got := exploration.Outcomes[0].Outcome.Err; !errors.Is(got, ErrIndexOutOfRange) {
		t.Fatalf("outcome error = %v, want %v", got, ErrIndexOutOfRange)
	}
}

func testAdvanceFailureReportedOnce(t *testing.T) {
	ctx, _, err := advanceFailing(t, relayedFailureModel)
	if !errors.Is(err, ErrIndexOutOfRange) {
		t.Fatalf("Advance(3) = %v, want %v", err, ErrIndexOutOfRange)
	}
	if _, err := ctx.Advance(3); err != nil {
		t.Fatalf("second Advance(3) = %v, want nil: the failed machine runs no further", err)
	}
}
