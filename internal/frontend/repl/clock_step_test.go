package repl

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
)

// %clock-step shows and sets the step the runs' clock ticks by: under a step the
// waits of a %runs table come due on the ticks, 0 restores the continuous clock,
// and a step that is no finite, non-negative number is refused and leaves it.
func TestClockStepTicksTheRunsClock(t *testing.T) {
	s := runsSession(t)
	wants(t, run(t, s, "%clock-step"), "clock step: none (a continuous clock)")
	wants(t, sweepTable(run(t, s, "%runs 2 7 MC::acquire clock")), "24.59324592607634 [s]", "4.848060164697906 [s]")
	wants(t, run(t, s, "%clock-step 1"), "clock step: 1.0 s")
	if s.ClockStep() != 1 {
		t.Errorf("ClockStep() = %v, want 1", s.ClockStep())
	}
	wants(t, sweepTable(run(t, s, "%runs 2 7 MC::acquire clock")), "runs MC::acquire — 2 run(s), seed 7", "| 25.0 [s]", "| 5.0 [s]")
	wants(t, run(t, s, "%clock-step 0.5"), "clock step: 0.5 s")
	wants(t, sweepTable(run(t, s, "%runs 2 7 MC::acquire clock")), "| 25.0 [s]", "| 5.0 [s]")
	wants(t, run(t, s, "%clock-step -1"), "invalid clock step: a clock steps by a finite, non-negative number of seconds, not -1.0")
	wants(t, run(t, s, "%clock-step soon"), `invalid clock step: "soon" is not a number of seconds`)
	wants(t, run(t, s, "%clock-step 1 2"), "usage: %clock-step [<seconds>]")
	if s.ClockStep() != 0.5 {
		t.Errorf("ClockStep() = %v after a refused %%clock-step, want 0.5 still", s.ClockStep())
	}
	wants(t, run(t, s, "%clock-step 0"), "clock step: none (a continuous clock)")
	wants(t, sweepTable(run(t, s, "%runs 2 7 MC::acquire clock")), "24.59324592607634 [s]")
	if err := s.SetClockStep(-2); !errors.Is(err, runtime.ErrClockStep) {
		t.Errorf("SetClockStep(-2) = %v, want ErrClockStep", err)
	}
}

// The clock step reaches the debugger's run too: under a step of 10 s a wait of
// 2 s parks its token until t=10, and once the step is cleared, on the clock the
// session keeps at t=10, until t=12.
func TestClockStepAppliesToTheDebugger(t *testing.T) {
	s := runsSession(t)
	wants(t, run(t, s, "%clock-step 10"), "clock step: 10.0 s")
	wants(t, run(t, s, "%action MC::timed"), "Started action executor")
	wants(t, stepToTheWait(t, s), "Use %advance 10.0 to move the clock from t=0.0 to the earliest wait")
	wants(t, run(t, s, "%advance 10"), "Advanced to 10.0", "Action completed")

	wants(t, run(t, s, "%clock-step 0"), "clock step: none (a continuous clock)")
	wants(t, run(t, s, "%action MC::timed"), "Started action executor")
	wants(t, stepToTheWait(t, s), "Use %advance 2.0 to move the clock from t=10.0 to the earliest wait")
	wants(t, run(t, s, "%advance 2"), "Advanced to 12.0", "Action completed")
}

// stepToTheWait steps the action under debug until its token waits on the clock.
func stepToTheWait(t *testing.T, s *Session) string {
	t.Helper()
	for i := 0; i < 8; i++ {
		out := run(t, s, "%step")
		if strings.Contains(out, "waits on the clock") {
			return out
		}
	}
	t.Fatal("the action never waited on the clock")
	return ""
}
