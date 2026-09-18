package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessDrawPolicy exercises what a draw policy cannot resolve: a
// call with no point of the kind the policy names, a witness whose draws its
// policy could not have made, a policy that reads as none, and a run whose draws
// the policy fixes but whose weighted decision still needs a seed. Each is a typed
// error, never a panic, a silent default or a hang.
func TestRuntimeRobustnessDrawPolicy(t *testing.T) {
	t.Run("normal_has_no_min_or_max", testNormalHasNoMinOrMax)
	t.Run("degenerate_normal_is_its_mean_under_every_policy", testDegenerateNormalIsItsMeanUnderEveryPolicy)
	t.Run("degenerate_normal_draws_nothing_at_random", testDegenerateNormalDrawsNothingAtRandom)
	t.Run("policy_set_mid_run_waits_for_the_next_run", testPolicySetMidRunWaitsForTheNextRun)
	t.Run("unbounded_timer_stops_the_run_where_it_parks", testUnboundedTimerStopsTheRunWhereItParks)
	t.Run("unknown_policy_is_refused", testUnknownPolicyIsRefused)
	t.Run("witness_draw_the_policy_cannot_make", testWitnessDrawThePolicyCannotMake)
	t.Run("witness_policy_over_a_call_it_cannot_resolve", testWitnessPolicyOverACallItCannotResolve)
	t.Run("fixed_policy_still_checks_the_domain", testFixedPolicyStillChecksTheDomain)
	t.Run("fixed_policy_leaves_unseeded_decisions_most_probable", testFixedPolicyLeavesUnseededDecisionsMostProbable)
}

// normalModel draws one normal as it starts.
const normalModel = `
package test {
	private import ScalarValues::*;
	private import RandomFunctions::*;
	action draw {
		attribute g : Real = normal(12.0, 3.0);
		first start; then done;
	}
}`

// testNormalHasNoMinOrMax: a normal under min or max is a DrawUnboundedError
// naming the call and the policy, seeded or not; under average it is its mean.
func testNormalHasNoMinOrMax(t *testing.T) {
	m := parseLibraryModel(t, normalModel)
	for _, policy := range []DrawPolicy{DrawMin, DrawMax} {
		for _, seed := range [][]uint64{nil, {7}} {
			_, _, err := runUnderDraws(t, m, "draw", policy, seed...)
			var unbounded *DrawUnboundedError
			if !errors.As(err, &unbounded) || !errors.Is(err, ErrDrawUnbounded) {
				t.Fatalf("%s: error %T %v, want a DrawUnboundedError", policy, err, err)
			}
			if unbounded.What != "normal(12.0, 3.0)" || unbounded.Policy != policy {
				t.Errorf("%s: the error names %s under %s", policy, unbounded.What, unbounded.Policy)
			}
			if !strings.Contains(err.Error(), "normal(12.0, 3.0) under "+policy.String()+": the distribution is unbounded") {
				t.Errorf("%s: error %q does not name the call and the policy", policy, err)
			}
		}
	}
	_, out, err := runUnderDraws(t, m, "draw", DrawAverage)
	if err != nil {
		t.Fatal(err)
	}
	if got := realOut(t, out, "g"); got != 12 {
		t.Errorf("g = %v under average, want the mean", got)
	}
}

// testDegenerateNormalIsItsMeanUnderEveryPolicy: normal(mean, 0) draws mean and
// nothing else, so min, max and average resolve it to mean, as a witness under min records.
func testDegenerateNormalIsItsMeanUnderEveryPolicy(t *testing.T) {
	m := parseLibraryModel(t, `
		package test {
			private import ScalarValues::*;
			private import RandomFunctions::*;
			action draw {
				attribute g : Real = normal(5.0, 0.0);
				first start; then done;
			}
		}`)
	for _, policy := range []DrawPolicy{DrawRandom, DrawMin, DrawMax, DrawAverage} {
		ctx, out, err := runUnderDraws(t, m, "draw", policy, 7)
		if err != nil {
			t.Fatalf("%s: %v", policy, err)
		}
		if got := realOut(t, out, "g"); got != 5 {
			t.Errorf("g = %v under %s, want the mean", got, policy)
		}
		if draws := ctx.DrawsTaken(); len(draws) != 1 || draws[0].String() != "draw normal(5.0, 0.0) = 5.0" {
			t.Errorf("%s recorded %v, want the one draw of the mean", policy, draws)
		}
	}
	w, err := ParseWitness("draws by min\ndraw normal(5.0, 0.0) = 5.0\nno choice points\n")
	if err != nil {
		t.Fatal(err)
	}
	replay, _ := m.fresh()
	mustSchedule(t, replay, ReplayOf(w))
	if _, err := replay.ExecuteAction(m.action(t, "draw")); err != nil {
		t.Errorf("replaying the witness under min: %v", err)
	}
}

// testDegenerateNormalDrawsNothingAtRandom: normal(mean, 0) makes no random choice,
// so at random it needs no seed and leaves the seeded stream where the next draw finds it.
func testDegenerateNormalDrawsNothingAtRandom(t *testing.T) {
	m := parseLibraryModel(t, `
		package test {
			private import ScalarValues::*;
			private import RandomFunctions::*;
			action unseeded {
				attribute g : Real = normal(5.0, 0.0);
				first start; then done;
			}
			action alone {
				attribute u : Real = uniform(2.0, 6.0);
				first start; then done;
			}
			action after {
				attribute g : Real = normal(5.0, 0.0);
				attribute u : Real = uniform(2.0, 6.0);
				first start; then done;
			}
		}`)
	ctx, out, err := runUnderDraws(t, m, "unseeded", DrawRandom)
	if err != nil {
		t.Fatalf("normal(5.0, 0.0) at random without a seed: %v", err)
	}
	if got := realOut(t, out, "g"); got != 5 {
		t.Errorf("g = %v, want the mean", got)
	}
	if draws := formatDraws(ctx.DrawsTaken()); draws != "draw normal(5.0, 0.0) = 5.0\n" {
		t.Errorf("recorded %q, want the one draw of the mean", draws)
	}
	_, alone, err := runUnderDraws(t, m, "alone", DrawRandom, 7)
	if err != nil {
		t.Fatal(err)
	}
	_, after, err := runUnderDraws(t, m, "after", DrawRandom, 7)
	if err != nil {
		t.Fatal(err)
	}
	if alone["u"].Const != after["u"].Const {
		t.Errorf("u = %v after normal(5.0, 0.0), want %v: the degenerate normal consumed the stream", after["u"], alone["u"])
	}
}

// testPolicySetMidRunWaitsForTheNextRun: a run keeps the policy it started under
// while paused, its witness names that policy, and the next run takes the new one.
func testPolicySetMidRunWaitsForTheNextRun(t *testing.T) {
	m := parseLibraryModel(t, `
		package test {
			private import ScalarValues::*;
			private import RandomFunctions::*;
			action draw {
				attribute u : Real = uniform(2.0, 6.0);
				attribute v : Real;
				first start;
				then assign v := uniform(2.0, 6.0);
				then done;
			}
		}`)
	ctx, _ := m.fresh()
	ctx.SetDrawPolicy(DrawMax)
	exec, err := newActionExecutor(ctx, m.action(t, "draw"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := exec.initialize(); err != nil {
		t.Fatal(err)
	}
	if draws := formatDraws(ctx.DrawsTaken()); draws != "draw uniform(2.0, 6.0) = 6.0\n" {
		t.Fatalf("initializing drew\n%swant u at max", draws)
	}
	ctx.SetDrawPolicy(DrawMin)
	for !exec.State().Ended() {
		if err := exec.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if draws := formatDraws(ctx.DrawsTaken()); draws != "draw uniform(2.0, 6.0) = 6.0\ndraw uniform(2.0, 6.0) = 6.0\n" {
		t.Errorf("the paused run drew\n%swant both draws at max, the policy it started under", draws)
	}
	if taken := ctx.DrawPolicyTaken(); taken != DrawMax {
		t.Errorf("the run's witness would name %s, want max", taken)
	}
	if ctx.DrawPolicy() != DrawMin {
		t.Errorf("the next run's policy is %s, want min", ctx.DrawPolicy())
	}
	out, err := ctx.ExecuteAction(m.action(t, "draw"))
	if err != nil {
		t.Fatal(err)
	}
	if u, v := realOut(t, out, "u"), realOut(t, out, "v"); u != 2 || v != 2 {
		t.Errorf("the next run drew u = %v, v = %v, want both at min", u, v)
	}
	if taken := ctx.DrawPolicyTaken(); taken != DrawMin {
		t.Errorf("the next run drew by %s, want min", taken)
	}
}

// testUnboundedTimerStopsTheRunWhereItParks: a timer drawn from a normal under
// max fails as the wait arms, not at the start, so the steps before it ran.
func testUnboundedTimerStopsTheRunWhereItParks(t *testing.T) {
	m := parseLibraryModel(t, `
		package test {
			private import ScalarValues::*;
			private import SI::*;
			private import RandomFunctions::*;
			action timed {
				attribute reached : Integer = 0;
				first start;
				then action mark assign reached := 1;
				then action wait accept after normal(30.0, 5.0) [s];
				then action late assign reached := 2;
				then done;
			}
		}`)
	ctx, _ := m.fresh()
	ctx.SetDrawPolicy(DrawMax)
	_, err := ctx.ExecuteAction(m.action(t, "timed"))
	if !errors.Is(err, ErrDrawUnbounded) {
		t.Fatalf("error %v, want ErrDrawUnbounded", err)
	}
	if ctx.Clock().Now() != 0 {
		t.Errorf("the clock advanced to %v past a wait that never armed", ctx.Clock().Now())
	}
	if reached := ctx.Clock().Waits(); len(reached) != 0 {
		t.Errorf("a wait is armed past the refusal: %v", reached)
	}
}

// testUnknownPolicyIsRefused: a spelling that is no policy is ErrDrawPolicy, case
// included, and returns the random policy beside its error.
func testUnknownPolicyIsRefused(t *testing.T) {
	for _, text := range []string{"Max", "mean", "minimum", "seed:3", "random,min"} {
		policy, err := ParseDrawPolicy(text)
		if !errors.Is(err, ErrDrawPolicy) {
			t.Errorf("ParseDrawPolicy(%q) = %v, %v; want ErrDrawPolicy", text, policy, err)
		}
		if policy != DrawRandom {
			t.Errorf("ParseDrawPolicy(%q) returned %s beside its error", text, policy)
		}
	}
}

// testWitnessDrawThePolicyCannotMake: a witness under max whose draw is not the
// call's max is a WitnessDrawError naming the draw and the policy, at the draw.
func testWitnessDrawThePolicyCannotMake(t *testing.T) {
	m := parseLibraryModel(t, drawingModel)
	w, err := ParseWitness("draws by max\ndraw uniform(0.0, 1.0) = 0.25\ndraw uniformInteger(1, 6) = 6\nno choice points\n")
	if err != nil {
		t.Fatal(err)
	}
	replay, _ := m.fresh()
	mustSchedule(t, replay, ReplayOf(w))
	_, err = replay.ExecuteAction(m.action(t, "draw"))
	var refused *WitnessDrawError
	if !errors.As(err, &refused) {
		t.Fatalf("error %T %v, want a WitnessDrawError", err, err)
	}
	if refused.Draw != 1 || !strings.Contains(err.Error(), "records 0.25, which the call cannot draw under max") {
		t.Errorf("error %q, want draw 1 refused under max", err)
	}
}

// testWitnessPolicyOverACallItCannotResolve: a witness under min recording a draw
// of a normal is refused — no min draw of a normal exists to have recorded.
func testWitnessPolicyOverACallItCannotResolve(t *testing.T) {
	m := parseLibraryModel(t, normalModel)
	w, err := ParseWitness("draws by min\ndraw normal(12.0, 3.0) = 12.0\nno choice points\n")
	if err != nil {
		t.Fatal(err)
	}
	replay, _ := m.fresh()
	mustSchedule(t, replay, ReplayOf(w))
	_, err = replay.ExecuteAction(m.action(t, "draw"))
	var refused *WitnessDrawError
	if !errors.As(err, &refused) || !strings.Contains(err.Error(), "under min") {
		t.Fatalf("error %T %v, want a WitnessDrawError under min", err, err)
	}
}

// testFixedPolicyStillChecksTheDomain: an empty or non-finite domain is refused
// under a fixed policy as under random, before any point is taken.
func testFixedPolicyStillChecksTheDomain(t *testing.T) {
	m := parseLibraryModel(t, `
		package test {
			private import ScalarValues::*;
			private import RandomFunctions::*;
			action inverted {
				attribute u : Real = uniform(6.0, 2.0);
				first start; then done;
			}
			action narrowed {
				attribute n : Integer = uniformInteger(9, 3);
				first start; then done;
			}
		}`)
	for _, name := range []string{"inverted", "narrowed"} {
		for _, policy := range []DrawPolicy{DrawMin, DrawMax, DrawAverage} {
			ctx, _, err := runUnderDraws(t, m, name, policy)
			if !errors.Is(err, ErrRandomDomain) {
				t.Errorf("%s under %s: error %v, want ErrRandomDomain", name, policy, err)
			}
			if draws := ctx.DrawsTaken(); len(draws) != 0 {
				t.Errorf("%s under %s recorded %v past its refusal", name, policy, draws)
			}
		}
	}
}

// testFixedPolicyLeavesUnseededDecisionsMostProbable: a fixed policy resolves the
// durations, not the decision: unseeded, the most probable branch is taken and no
// draw is recorded for it, while the durations are fixed.
func testFixedPolicyLeavesUnseededDecisionsMostProbable(t *testing.T) {
	m := parseLibraryModel(t, `
		package test {
			private import ScalarValues::*;
			private import SI::*;
			private import Stochastic::*;
			private import RandomFunctions::*;
			action route {
				attribute taken : Integer = 0;
				first start;
				then decide select;
				first select then slow { @Probability { p = 0.3; } }
				first select then fast { @Probability { p = 0.7; } }
				action slow { assign taken := 2; }
				then action slowWait accept after uniform(10, 20) [s];
				then done;
				action fast { assign taken := 1; }
				then action fastWait accept after uniform(1, 5) [s];
				then done;
			}
		}`)
	ctx, out, err := runUnderDraws(t, m, "route", DrawMax)
	if err != nil {
		t.Fatal(err)
	}
	if got := takenInt(t, out, "taken"); got != 1 {
		t.Errorf("took %d, want the 0.7 branch", got)
	}
	if draws := ctx.DrawsTaken(); len(draws) != 1 || draws[0].String() != "draw uniform(1, 5) = 5.0" {
		t.Errorf("recorded %v, want the one fixed duration", draws)
	}
	if ctx.Clock().Now() != 5 {
		t.Errorf("the clock ended at %v, want 5", ctx.Clock().Now())
	}
}
