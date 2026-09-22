package runtime

import (
	"errors"
	"math"
	"testing"
)

// TestRuntimeRobustnessClockStep exercises what a stepped clock refuses or must
// not change: a step that is no finite, non-negative number, a negative wait
// under a step, a step changed while waits are pending, an instant already past,
// a step too fine for the instant, and a tick past the last instant. Each is a
// typed error or the continuous clock's rule, never a panic or a wait that never
// comes due.
func TestRuntimeRobustnessClockStep(t *testing.T) {
	t.Run("step_that_is_no_number_is_refused", testStepThatIsNoNumberIsRefused)
	t.Run("negative_wait_is_refused_under_a_step", testNegativeWaitIsRefusedUnderAStep)
	t.Run("step_set_mid_run_applies_to_the_waits_set_after_it", testStepSetMidRunAppliesToTheWaitsSetAfterIt)
	t.Run("past_instant_fires_at_once_under_a_step", testPastInstantFiresAtOnceUnderAStep)
	t.Run("past_instant_fires_at_once_off_the_grid", testPastInstantFiresAtOnceOffTheGrid)
	t.Run("step_too_fine_to_tell_apart_leaves_the_wait_finite", testStepTooFineToTellApartLeavesTheWaitFinite)
	t.Run("tick_past_the_last_instant_is_refused", testTickPastTheLastInstantIsRefused)
}

// testStepThatIsNoNumberIsRefused: a negative, NaN or infinite step is ErrClockStep
// and leaves the clock's step as it was.
func testStepThatIsNoNumberIsRefused(t *testing.T) {
	m := parseLibraryModel(t, steppedModel)
	ctx, _ := m.fresh()
	if err := ctx.SetClockStep(0.5); err != nil {
		t.Fatal(err)
	}
	for _, step := range []float64{-0.5, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if err := ctx.SetClockStep(step); !errors.Is(err, ErrClockStep) {
			t.Errorf("SetClockStep(%v) = %v, want ErrClockStep", step, err)
		}
		if ctx.ClockStep() != 0.5 {
			t.Errorf("SetClockStep(%v) left the step at %v, want 0.5", step, ctx.ClockStep())
		}
	}
}

// testNegativeWaitIsRefusedUnderAStep: a step does not round a negative duration
// up to a tick; the wait stays ErrNegativeDuration.
func testNegativeWaitIsRefusedUnderAStep(t *testing.T) {
	m := parseLibraryModel(t, `
		package test {
			private import ScalarValues::*;
			private import SI::*;
			action wrong {
				attribute back : Real = -0.2;
				first start;
				then action w accept after back [s];
				then done;
			}
		}`)
	_, _, err := runUnderStep(t, m, "wrong", 1.0)
	if !errors.Is(err, ErrNegativeDuration) {
		t.Fatalf("err = %v, want ErrNegativeDuration", err)
	}
}

// testStepSetMidRunAppliesToTheWaitsSetAfterIt: a wait already on the clock keeps
// the instant it was set for; the waits set after the change come due on the new
// grid.
func testStepSetMidRunAppliesToTheWaitsSetAfterIt(t *testing.T) {
	m := parseLibraryModel(t, steppedModel)
	ctx, _ := m.fresh()
	exec, err := newActionExecutor(ctx, m.action(t, "Stepped"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := exec.initialize(); err != nil {
		t.Fatal(err)
	}
	for len(ctx.Clock().Waits()) == 0 {
		if err := exec.Step(); err != nil && len(ctx.Clock().Waits()) == 0 {
			t.Fatal(err)
		}
	}
	if waits := ctx.Clock().Waits(); len(waits) != 1 || math.Abs(waits[0].Due-2.3) > 1e-12 {
		t.Fatalf("the first wait is %+v, want one due at 2.3 on the continuous clock", waits)
	}
	if err := ctx.SetClockStep(1.0); err != nil {
		t.Fatal(err)
	}
	if _, err := ctx.Advance(10); err != nil {
		t.Fatal(err)
	}
	if !exec.State().Ended() {
		t.Fatalf("the run is %v after 10 s, want ended", exec.State())
	}
	got := realOutputs(t, exec.Results(), "t1", "t2", "t3")
	for i, want := range []float64{2.3, 3.0, 6.0} {
		if math.Abs(got[i]-want) > 1e-9 {
			t.Errorf("t%d = %v, want %v: the pending wait keeps 2.3, the next tick from there is 3.0", i+1, got[i], want)
		}
	}
}

// testPastInstantFiresAtOnceUnderAStep: an absolute instant the clock has passed
// is not waited for under a step either; the clock stays on the tick it reached.
func testPastInstantFiresAtOnceUnderAStep(t *testing.T) {
	m := parseLibraryModel(t, `
		package test {
			private import ScalarValues::*;
			private import SI::*;
			private import Time::*;
			action def Past {
				out at : Real;
				attribute early : TimeInstantValue = 1.0 [s];
				action w1 accept after 2.5 [s];
				then action w2 accept at early;
				then action r { assign at := localClock.currentTime; }
			}
		}`)
	ctx, out, err := runUnderStep(t, m, "Past", 2.0)
	if err != nil {
		t.Fatal(err)
	}
	if got := realOutputs(t, out, "at")[0]; got != 4.0 {
		t.Errorf("at = %v, want 4.0: 2.5 rounds up to the tick 4.0 and the past instant waits no further", got)
	}
	if now := ctx.Clock().Now(); now != 4.0 {
		t.Errorf("the clock is at %v, want 4.0", now)
	}
}

// testPastInstantFiresAtOnceOffTheGrid: a step set once the clock stands between
// two of its ticks does not put a passed absolute instant off to the next tick;
// only the waits for instants ahead come due on the grid.
func testPastInstantFiresAtOnceOffTheGrid(t *testing.T) {
	m := parseLibraryModel(t, `
		package test {
			private import ScalarValues::*;
			private import SI::*;
			private import Time::*;
			action def Past {
				out at : Real;
				out onward : Real;
				attribute early : TimeInstantValue = 1.0 [s];
				attribute later : TimeInstantValue = 2.5 [s];
				action w1 accept after 2.3 [s];
				then action w2 accept at early;
				then action r1 { assign at := localClock.currentTime; }
				then action w3 accept at later;
				then action r2 { assign onward := localClock.currentTime; }
			}
		}`)
	ctx, _ := m.fresh()
	exec, err := newActionExecutor(ctx, m.action(t, "Past"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := exec.initialize(); err != nil {
		t.Fatal(err)
	}
	for len(ctx.Clock().Waits()) == 0 {
		if err := exec.Step(); err != nil && len(ctx.Clock().Waits()) == 0 {
			t.Fatal(err)
		}
	}
	if err := ctx.SetClockStep(1.0); err != nil {
		t.Fatal(err)
	}
	if _, err := ctx.Advance(10); err != nil {
		t.Fatal(err)
	}
	if !exec.State().Ended() {
		t.Fatalf("the run is %v after 10 s, want ended", exec.State())
	}
	got := realOutputs(t, exec.Results(), "at", "onward")
	for i, want := range []float64{2.3, 3.0} {
		if math.Abs(got[i]-want) > 1e-9 {
			t.Errorf("%s = %v, want %v: the passed instant 1.0 waits no further at 2.3, and 2.5 ahead rounds up to the tick 3.0", []string{"at", "onward"}[i], got[i], want)
		}
	}
}

// testStepTooFineToTellApartLeavesTheWaitFinite: a positive step so small that the
// instant counts more ticks than a float64 holds leaves the wait due at its instant,
// never at infinity.
func testStepTooFineToTellApartLeavesTheWaitFinite(t *testing.T) {
	m := parseLibraryModel(t, steppedModel)
	for _, step := range []float64{1e-320, 1e-300, 1e-17} {
		ctx, out, err := runUnderStep(t, m, "Stepped", step)
		if err != nil {
			t.Fatalf("step %v: %v", step, err)
		}
		got := realOutputs(t, out, "t1", "t2", "t3")
		for i, want := range []float64{2.3, 2.7, 6.0} {
			if math.Abs(got[i]-want) > 1e-9 {
				t.Errorf("step %v: t%d = %v, want %v", step, i+1, got[i], want)
			}
		}
		if now := ctx.Clock().Now(); math.IsInf(now, 0) || math.IsNaN(now) {
			t.Errorf("step %v: the clock is at %v, want a finite instant", step, now)
		}
	}
}

// testTickPastTheLastInstantIsRefused: an instant whose next tick lies past the
// last float64 is refused as ErrNegativeDuration rather than waited for at infinity.
func testTickPastTheLastInstantIsRefused(t *testing.T) {
	m := parseLibraryModel(t, `
		package test {
			private import ScalarValues::*;
			private import SI::*;
			private import Time::*;
			action def Far {
				attribute last : TimeInstantValue = 1.79e308 [s];
				action w accept at last;
				then done;
			}
		}`)
	ctx, _, err := runUnderStep(t, m, "Far", 1e307)
	if !errors.Is(err, ErrNegativeDuration) {
		t.Fatalf("err = %v, want ErrNegativeDuration", err)
	}
	if waits := ctx.Clock().Waits(); len(waits) != 0 {
		t.Errorf("waits = %+v, want none queued", waits)
	}
}
