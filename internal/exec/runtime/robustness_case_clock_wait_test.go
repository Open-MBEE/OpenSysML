package runtime

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestRuntimeRobustnessCaseClockWait exercises the failure modes of a case
// body's step waiting on the clock (`accept after`): a wait for a message
// nothing posts, a wait under a behavior that holds the clock, the clock
// letting go of the case's flow once its run ends, the same for a case an
// action body performs as a step, whose wait the action lists among its own,
// and the refusal of a wait under a case read as a feature of an expression.
func TestRuntimeRobustnessCaseClockWait(t *testing.T) {
	t.Run("message_wait_still_deadlocks", testCaseClockWaitMessageStillDeadlocks)
	t.Run("wait_under_a_held_clock", testCaseClockWaitUnderHeldClock)
	t.Run("clock_lets_go_of_the_case_after_its_run", testCaseClockWaitDetachesAfterRun)
	t.Run("failing_run_lets_go_of_the_case", testCaseClockWaitDetachesAfterFailure)
	t.Run("performing_action_lists_the_wait_of_its_case_step", testCaseClockWaitListedByPerformingAction)
	t.Run("message_wait_of_a_case_step_deadlocks_the_performer", testCaseClockWaitOfStepDeadlocksPerformer)
	t.Run("case_read_as_a_feature_cannot_wait", testCaseClockWaitRefusedUnderRead)
}

// caseWaitingModel declares a case whose steps wait on the clock, and one whose
// step waits for a message no behavior posts.
const caseWaitingModel = `
	package test {
		private import ScalarValues::*;
		private import SI::*;
		attribute def Ping;
		part def Ship { attribute log : Real default = 0.0; }
		part ship : Ship;
		analysis def Voyage {
			subject boat : Ship;
			action sail accept after 60.0 [s];
			then action arrive { assign boat.log := boat.log + 1.0; }
			return total : Real = boat.log;
		}
		analysis def Stranded {
			subject boat : Ship;
			action sail accept after 60.0 [s];
			then action hail accept p : Ping;
			then action arrive { assign boat.log := boat.log + 1.0; }
			return total : Real = boat.log;
		}
		analysis def Broken {
			subject boat : Ship;
			action sail accept after 60.0 [s];
			then action arrive { assign boat.log := boat.log / 0.0; }
			return total : Real = boat.log;
		}
	}`

// testCaseClockWaitMessageStillDeadlocks: advancing the clock to a step's instant
// does not stand in for a message; a step waiting for one nothing posts deadlocks.
func testCaseClockWaitMessageStillDeadlocks(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, caseWaitingModel))
	ship := instanceOfUsage(t, ctx, idx, "test::ship")
	_, err := ctx.RunAnalysis(oneSymbol(t, idx, "test::Stranded"), AnalysisArgs{Subject: ship}, nil, nil)
	if !errors.Is(err, ErrAcceptDeadlock) {
		t.Fatalf("error = %v, want ErrAcceptDeadlock", err)
	}
	if !strings.Contains(err.Error(), "test::Stranded") || !strings.Contains(err.Error(), "Ping") {
		t.Errorf("error %q does not name the case and the message it waits for", err)
	}
	if ctx.clock.now != 60.0 {
		t.Errorf("clock = %v after the deadlock, want 60.0: the timed step ran first", ctx.clock.now)
	}
}

// testCaseClockWaitUnderHeldClock: a state's entry behavior holds the clock, so a
// case it reads that waits on the clock is refused, not advanced.
func testCaseClockWaitUnderHeldClock(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		private import SI::*;
		part def Ship { attribute log : Real default = 0.0; }
		analysis def Pause {
			subject boat : Ship;
			action sail accept after 60.0 [s];
			then action arrive { assign boat.log := boat.log + 1.0; }
			return total : Real = boat.log;
		}
		part def Station {
			part boat : Ship;
			attribute seen : Real default = -1.0;
			analysis pause : Pause { subject boat = boat; }
			exhibit state run {
				entry; then observing;
				state observing { entry assign seen := pause.total; }
			}
		}
	}`
	ctx, _, err := instantiateWithLibraries(t, src, "test::Station")
	if !errors.Is(err, ErrStateBehaviorWaits) {
		t.Fatalf("error = %v, want ErrStateBehaviorWaits", err)
	}
	if !strings.Contains(err.Error(), "test::Station::pause") || !strings.Contains(err.Error(), "t=60.0") {
		t.Errorf("error %q does not name the case and the instant it waits for", err)
	}
	if ctx.clock.now != 0 {
		t.Errorf("clock = %v, want 0: a held clock never advances", ctx.clock.now)
	}
}

// testCaseClockWaitDetachesAfterRun: the clock drives the case's flow while the
// case runs and lets go of it afterwards, so a later run finds no stale wait.
func testCaseClockWaitDetachesAfterRun(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, caseWaitingModel))
	ship := instanceOfUsage(t, ctx, idx, "test::ship")
	waiters := len(ctx.clock.waiters)
	for i := 1; i <= 2; i++ {
		res, err := ctx.RunAnalysis(oneSymbol(t, idx, "test::Voyage"), AnalysisArgs{Subject: ship}, nil, nil)
		if err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
		if want := fmt.Sprintf("%d.0", i); len(res.Outputs) != 1 || FormatValue(res.Outputs[0].Value) != want {
			t.Errorf("run %d: outputs = %+v, want total = %s", i, res.Outputs, want)
		}
		if got := len(ctx.clock.waiters); got != waiters {
			t.Errorf("run %d: the clock drives %d executor(s) after the run, want %d", i, got, waiters)
		}
	}
	if ctx.clock.now != 120.0 {
		t.Errorf("clock = %v after two runs, want 120.0", ctx.clock.now)
	}
}

// caseStepModel declares actions performing, as a step, a verification case whose
// steps wait on the clock, and one whose step waits for a message nothing posts.
const caseStepModel = `
	package test {
		private import ScalarValues::*;
		private import SI::*;
		private import VerificationCases::*;
		attribute def Ping;
		part def Ship { attribute log : Real default = 0.0; }
		verification def Voyage {
			subject boat : Ship;
			action sail accept after 60.0 [s];
			then action arrive { assign boat.log := boat.log + 1.0; }
			return verdict : VerdictKind = VerdictKind::pass;
		}
		verification def Stranded {
			subject boat : Ship;
			action sail accept after 60.0 [s];
			then action hail accept p : Ping;
			return verdict : VerdictKind = VerdictKind::pass;
		}
		action def Cruise {
			part ship : Ship;
			out ended : Real;
			verification run : Voyage { subject boat = ship; }
			then action after { assign ended := localClock.currentTime; }
		}
		action def Lost {
			part ship : Ship;
			verification run : Stranded { subject boat = ship; }
		}
	}`

// testCaseClockWaitListedByPerformingAction: an action performing a case as a
// step lists the case's wait among its own while the step pauses, the clock
// advances to it, and the run leaves the clock driving no more than before.
func testCaseClockWaitListedByPerformingAction(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, caseStepModel))
	waiters := len(ctx.clock.waiters)
	out, err := ctx.ExecuteAction(oneSymbol(t, idx, "test::Cruise"))
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if got := FormatValue(out["ended"]); got != "60.0" {
		t.Errorf("ended = %s, want 60.0: the step after the case runs at the case's instant", got)
	}
	if ctx.clock.now != 60.0 {
		t.Errorf("clock = %v, want 60.0", ctx.clock.now)
	}
	if got := len(ctx.clock.waiters); got != waiters {
		t.Errorf("the clock drives %d executor(s) after the run, want %d", got, waiters)
	}
}

// testCaseClockWaitOfStepDeadlocksPerformer: a case step waiting for a message
// nothing posts deadlocks the action performing it, after its timed step ran.
func testCaseClockWaitOfStepDeadlocksPerformer(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, caseStepModel))
	waiters := len(ctx.clock.waiters)
	_, err := ctx.ExecuteAction(oneSymbol(t, idx, "test::Lost"))
	if !errors.Is(err, ErrAcceptDeadlock) {
		t.Fatalf("error = %v, want ErrAcceptDeadlock", err)
	}
	if !strings.Contains(err.Error(), "Lost") {
		t.Errorf("error %q does not name the performing action", err)
	}
	if ctx.clock.now != 60.0 {
		t.Errorf("clock = %v after the deadlock, want 60.0: the timed step ran first", ctx.clock.now)
	}
	if got := len(ctx.clock.waiters); got != waiters {
		t.Errorf("the clock drives %d executor(s) after the deadlock, want %d", got, waiters)
	}
}

// caseReadModel declares an action reading, in an expression, an output of a case
// whose steps wait on the clock.
const caseReadModel = `
	package test {
		private import ScalarValues::*;
		private import SI::*;
		part def Ship { attribute log : Real default = 0.0; }
		analysis def Voyage {
			subject boat : Ship;
			action sail accept after 60.0 [s];
			then action arrive { assign boat.log := boat.log + 1.0; }
			return total : Real = boat.log;
		}
		part def Station {
			part boat : Ship;
			analysis voyage : Voyage { subject boat = boat; }
		}
		action def Read {
			part station : Station;
			out total : Real;
			action go { assign total := station.voyage.total; }
		}
	}`

// testCaseClockWaitRefusedUnderRead: an expression reading a case's output takes
// the case whole, so a wait under it is refused, naming the wait, and leaves
// the clock where it was and the case neither running nor on the calc stack.
func testCaseClockWaitRefusedUnderRead(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, caseReadModel))
	waiters := len(ctx.clock.waiters)
	_, err := ctx.ExecuteAction(oneSymbol(t, idx, "test::Read"))
	if !errors.Is(err, ErrCaseReadWaits) {
		t.Fatalf("error = %v, want ErrCaseReadWaits", err)
	}
	for _, want := range []string{"test::Station::voyage", "t=60.0"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
	if ctx.clock.now != 0.0 {
		t.Errorf("clock = %v, want 0.0: the read does not advance the clock", ctx.clock.now)
	}
	if got := len(ctx.clock.waiters); got != waiters {
		t.Errorf("the clock drives %d executor(s) after the refusal, want %d", got, waiters)
	}
	if ctx.calcDepth != 0 || len(ctx.calcUsageRunning) != 0 {
		t.Errorf("calc depth %d, %d usage(s) running after the refusal, want none", ctx.calcDepth, len(ctx.calcUsageRunning))
	}
}

// testCaseClockWaitDetachesAfterFailure: a run its step fails ends too, and the
// clock lets go of the case's flow all the same.
func testCaseClockWaitDetachesAfterFailure(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, caseWaitingModel))
	ship := instanceOfUsage(t, ctx, idx, "test::ship")
	waiters := len(ctx.clock.waiters)
	_, err := ctx.RunAnalysis(oneSymbol(t, idx, "test::Broken"), AnalysisArgs{Subject: ship}, nil, nil)
	if err == nil {
		t.Fatal("a step dividing by zero ran to completion")
	}
	if !strings.Contains(err.Error(), "test::Broken") {
		t.Errorf("error %q does not name the case", err)
	}
	if got := len(ctx.clock.waiters); got != waiters {
		t.Errorf("the clock drives %d executor(s) after the failed run, want %d", got, waiters)
	}
}
