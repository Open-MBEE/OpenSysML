package runtime

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestRuntimeRobustnessCaseClockWait exercises the failure modes of a case
// body's step waiting on the clock (`accept after`): a wait for a message
// nothing posts, a wait under a behavior that holds the clock, and the clock
// letting go of the case's flow once its run ends.
func TestRuntimeRobustnessCaseClockWait(t *testing.T) {
	t.Run("message_wait_still_deadlocks", testCaseClockWaitMessageStillDeadlocks)
	t.Run("wait_under_a_held_clock", testCaseClockWaitUnderHeldClock)
	t.Run("clock_lets_go_of_the_case_after_its_run", testCaseClockWaitDetachesAfterRun)
	t.Run("failing_run_lets_go_of_the_case", testCaseClockWaitDetachesAfterFailure)
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
