package runtime

import (
	"errors"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// steppedModel waits three times and reads the clock after each wait, so the
// instants a run reads show the clock it ran on.
const steppedModel = `
package test {
	private import ScalarValues::*;
	private import SI::*;
	private import Time::*;
	action def Stepped {
		attribute six : TimeInstantValue = 6.0 [s];
		out t1 : Real;
		out t2 : Real;
		out t3 : Real;
		action w1 accept after 2.3 [s];
		then action r1 { assign t1 := localClock.currentTime; }
		then action w2 accept after 0.4 [s];
		then action r2 { assign t2 := localClock.currentTime; }
		then action w3 accept at six;
		then action r3 { assign t3 := localClock.currentTime; }
	}
}`

// runUnderStep runs name on a fresh context whose clock steps by step.
func runUnderStep(t *testing.T, m *exploreModel, name string, step float64) (*Context, map[string]Value, error) {
	t.Helper()
	ctx, _ := m.fresh()
	if err := ctx.SetClockStep(step); err != nil {
		t.Fatal(err)
	}
	out, err := ctx.ExecuteAction(m.action(t, name))
	return ctx, out, err
}

func realOutputs(t *testing.T, out map[string]Value, names ...string) []float64 {
	t.Helper()
	got := make([]float64, len(names))
	for i, name := range names {
		v := out[name]
		if v.Kind != ValConst || v.Const.Kind != semantics.ValReal {
			t.Fatalf("%s = %v, want a Real", name, v)
		}
		got[i] = v.Const.Real
	}
	return got
}

// A continuous clock (the default) reads a wait due exactly when it ends: the
// instants are the sums of the durations, and an absolute wait is its instant.
func TestContinuousClockReadsWaitsExactly(t *testing.T) {
	m := parseLibraryModel(t, steppedModel)
	ctx, out, err := runUnderStep(t, m, "Stepped", 0)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.ClockStep() != 0 {
		t.Errorf("ClockStep() = %v, want 0", ctx.ClockStep())
	}
	got := realOutputs(t, out, "t1", "t2", "t3")
	for i, want := range []float64{2.3, 2.7, 6.0} {
		if math.Abs(got[i]-want) > 1e-12 {
			t.Errorf("t%d = %v, want %v", i+1, got[i], want)
		}
	}
}

// Under a step, a wait comes due at the first tick not before it ends — a
// relative wait from the tick it was set on, an absolute one at its instant when
// that is a tick — so a run reads only ticks.
func TestSteppedClockReadsWaitsOnTicks(t *testing.T) {
	m := parseLibraryModel(t, steppedModel)
	for _, tc := range []struct {
		step float64
		want []float64
	}{
		{1.0, []float64{3.0, 4.0, 6.0}},
		{0.5, []float64{2.5, 3.0, 6.0}},
		{0.1, []float64{2.3, 2.7, 6.0}},
		{4.0, []float64{4.0, 8.0, 8.0}},
	} {
		ctx, out, err := runUnderStep(t, m, "Stepped", tc.step)
		if err != nil {
			t.Fatalf("step %v: %v", tc.step, err)
		}
		if ctx.ClockStep() != tc.step {
			t.Errorf("step %v: ClockStep() = %v", tc.step, ctx.ClockStep())
		}
		got := realOutputs(t, out, "t1", "t2", "t3")
		for i, want := range tc.want {
			if math.Abs(got[i]-want) > 1e-9 {
				t.Errorf("step %v: t%d = %v, want %v", tc.step, i+1, got[i], want)
			}
		}
		if now := ctx.Clock().Now(); math.Abs(now-tc.want[2]) > 1e-9 {
			t.Errorf("step %v: the clock ended at %v, want %v", tc.step, now, tc.want[2])
		}
	}
}

// onTick lands an instant within floating-point rounding of a tick on that tick
// rather than the next, so a sum of steps does not drift a tick late.
func TestClockStepAbsorbsRounding(t *testing.T) {
	sum := 0.0
	for i := 0; i < 10; i++ {
		sum += 0.1
	}
	if sum == 1.0 {
		t.Fatal("ten 0.1 add to exactly 1.0 here; the case needs a rounded sum")
	}
	if got := onTick(sum, 0.1); got != 1.0 {
		t.Errorf("onTick(%v) = %v, want 1.0", sum, got)
	}
	if got := onTick(1.0+1e-6, 0.1); math.Abs(got-1.1) > 1e-12 {
		t.Errorf("onTick(1.000001) = %v, want 1.1", got)
	}
	if got := onTick(0, 0.1); got != 0 {
		t.Errorf("onTick(0) = %v, want 0", got)
	}
	// Far along the clock the rounding allowed stays a sliver of a tick: an instant
	// well past a tick is not pulled back onto it.
	if got := onTick(1_000_000_000.4, 1); got != 1_000_000_001 {
		t.Errorf("onTick(1000000000.4) = %v, want 1000000001", got)
	}
	if got := onTick(1_000_000_000, 1); got != 1_000_000_000 {
		t.Errorf("onTick(1000000000) = %v, want 1000000000", got)
	}
	if got := onTick(1_000_000_000+1e-7, 1); got != 1_000_000_000 {
		t.Errorf("onTick(1000000000+1e-7) = %v, want 1000000000", got)
	}
}

// CheckClockStep admits every finite, non-negative step and refuses the rest
// with ErrClockStep naming the value.
func TestCheckClockStep(t *testing.T) {
	for _, step := range []float64{0, 1e-9, 0.01, 1, 3600} {
		if err := CheckClockStep(step); err != nil {
			t.Errorf("CheckClockStep(%v) = %v", step, err)
		}
	}
	for _, step := range []float64{-1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		err := CheckClockStep(step)
		if !errors.Is(err, ErrClockStep) {
			t.Errorf("CheckClockStep(%v) = %v, want ErrClockStep", step, err)
		}
		if !strings.Contains(err.Error(), "finite, non-negative number of seconds") {
			t.Errorf("CheckClockStep(%v) = %q, want the rule", step, err)
		}
	}
}

// A witness of a run on a stepped clock names the step, reads back, and replays
// on that clock whatever the replaying context's own step is; a continuous run's
// witness names none. A step line out of place, doubled, unreadable or naming a
// continuous clock is refused with a typed error.
func TestWitnessCarriesTheClockStep(t *testing.T) {
	m := parseLibraryModel(t, steppedModel)
	ctx, out, err := runUnderStep(t, m, "Stepped", 1)
	if err != nil {
		t.Fatal(err)
	}
	w := Witness{DrawPolicy: ctx.DrawPolicyTaken(), ClockStep: ctx.ClockStepTaken(), Draws: ctx.DrawsTaken(), Choices: ctx.ChoicesTaken()}
	text := w.String()
	if !strings.HasPrefix(text, "clock steps by 1.0\n") {
		t.Fatalf("the witness does not open with the clock step:\n%s", text)
	}
	parsed, err := ParseWitness(text)
	if err != nil {
		t.Fatalf("the witness does not read back: %v\n%s", err, text)
	}
	if parsed.ClockStep != 1 {
		t.Fatalf("the witness reads back stepping by %v, want 1", parsed.ClockStep)
	}
	replay, _ := m.fresh()
	mustSchedule(t, replay, ReplayOf(parsed))
	if got := replay.ClockStepTaken(); got != 1 {
		t.Fatalf("the replay's clock steps by %v, want the witness's 1", got)
	}
	got, err := replay.ExecuteAction(m.action(t, "Stepped"))
	if err == nil {
		err = replay.Unfollowed()
	}
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if want, have := realOutputs(t, out, "t1", "t2", "t3"), realOutputs(t, got, "t1", "t2", "t3"); !slices.Equal(want, have) {
		t.Errorf("the replay read %v, want the stepped run's %v", have, want)
	}
	if replay.ClockStep() != 0 {
		t.Errorf("the replay changed the context's own clock step to %v", replay.ClockStep())
	}

	continuous, _, err := runUnderStep(t, m, "Stepped", 0)
	if err != nil {
		t.Fatal(err)
	}
	if text := (Witness{ClockStep: continuous.ClockStepTaken(), Choices: continuous.ChoicesTaken()}).String(); strings.Contains(text, clockStepPrefix) {
		t.Errorf("a continuous run's witness names a clock step:\n%s", text)
	}

	var parse *ClockStepParseError
	for _, bad := range []struct{ text, reason string }{
		{"draw uniform(0.0, 1.0) = 1.0\nclock steps by 1.0\nno choice points\n", "before the draws"},
		{"clock steps by 1.0\nclock steps by 0.5\nno choice points\n", "named twice"},
		{"clock steps by 0.0\nno choice points\n", "continuous clock"},
		{"clock steps by -1\nno choice points\n", "non-negative"},
		{"clock steps by soon\nno choice points\n", "not a number"},
	} {
		_, err := ParseWitness(bad.text)
		if !errors.As(err, &parse) || !errors.Is(err, ErrClockStep) || !strings.Contains(err.Error(), bad.reason) || parse.Line == 0 {
			t.Errorf("%q parsed as %v, want a ClockStepParseError on its line saying %q", bad.text, err, bad.reason)
		}
	}
}
