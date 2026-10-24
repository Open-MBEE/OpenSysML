package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
)

// TestRuntimeRobustnessRandomDraws covers weighted decisions and random draws: mixed weights,
// undecidable weights and exhausted draw policies.
func TestRuntimeRobustnessRandomDraws(t *testing.T) {
	t.Run("weighted_decision_whose_weights_do_not_sum_to_one", testWeightedDecisionWhoseWeightsDoNotSumToOne)
	t.Run("weighted_decision_with_a_weight_outside_zero_to_one", testWeightedDecisionWithAWeightOutsideZeroToOne)
	t.Run("decision_mixing_weighted_and_unweighted_successions", testDecisionMixingWeightedAndUnweightedSuccessions)
	t.Run("weighted_decision_whose_read_weight_is_no_probability", testWeightedDecisionWhoseReadWeightIsNoProbability)
	t.Run("random_draw_without_a_seed", testRandomDrawWithoutASeed)
	t.Run("random_bounds_reversed", testRandomBoundsReversed)
	t.Run("random_duration_without_a_seed", testRandomDurationWithoutASeed)
}

// testWeightedDecisionWhoseWeightsDoNotSumToOne: constant weights out of one
// decision must sum to 1; the lowering refuses the action before it runs.
func testWeightedDecisionWhoseWeightsDoNotSumToOne(t *testing.T) {
	err := weightedActionError(t, `
		first start; then decide d;
		first d then fast { @Probability { p = 0.6; } }
		first d then slow { @Probability { p = 0.6; } }
		action fast; then done;
		action slow; then done;`)
	if !errors.Is(err, lower.ErrProbability) || !strings.Contains(err.Error(), "sum to 1.2") {
		t.Fatalf("error = %v, want the Probability weights' sum refused", err)
	}
}

// testWeightedDecisionWithAWeightOutsideZeroToOne: a weight is a probability,
// so one above 1 or below 0 is refused where it is declared.
func testWeightedDecisionWithAWeightOutsideZeroToOne(t *testing.T) {
	err := weightedActionError(t, `
		first start; then decide d;
		first d then fast { @Probability { p = 1.2; } }
		first d then slow { @Probability { p = -0.2; } }
		action fast; then done;
		action slow; then done;`)
	if !errors.Is(err, lower.ErrProbability) || !strings.Contains(err.Error(), "outside 0.0..1.0") {
		t.Fatalf("error = %v, want the out-of-range weight refused", err)
	}
}

// testDecisionMixingWeightedAndUnweightedSuccessions: every succession out of a
// decision is weighted or none is; a mix has no reading and is refused.
func testDecisionMixingWeightedAndUnweightedSuccessions(t *testing.T) {
	err := weightedActionError(t, `
		first start; then decide d;
		first d then fast { @Probability { p = 1.0; } }
		first d then slow;
		action fast; then done;
		action slow; then done;`)
	if !errors.Is(err, lower.ErrProbability) || !strings.Contains(err.Error(), "weights 1 of its 2 successions") {
		t.Fatalf("error = %v, want the mixed decision refused", err)
	}
}

// testWeightedDecisionWhoseReadWeightIsNoProbability: a weight read from a
// feature is checked when read, whether one branch holds or several.
func testWeightedDecisionWhoseReadWeightIsNoProbability(t *testing.T) {
	cases := []struct{ name, w, slowGuard, want string }{
		{"sole branch weighs zero", "0.0", "if not ready", "no holding branch has a positive weight"},
		{"sole branch weighs over one", "1.5", "if not ready", "branch 0 weighs 1.5, not a probability in [0, 1]"},
		{"sole branch weighs a negative", "-0.5", "if not ready", "branch 0 weighs -0.5, not a probability in [0, 1]"},
		{"sole branch weighs a boolean", "false", "if not ready", "weight of 1->fast is a Boolean, not a number"},
		{"both branches weigh zero", "0.0", "if ready", "no holding branch has a positive weight"},
		{"both branches weigh over one", "1.5", "if ready", "branch 0 weighs 1.5, not a probability in [0, 1]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := weightedActionError(t, `
				attribute ready : Boolean = true;
				attribute w = `+tc.w+`;
				attribute taken : Integer = 0;
				first start; then decide select;
				first select then fast { @Probability { p = w; } }
				first select `+tc.slowGuard+` then slow { @Probability { p = w; } }
				action fast { assign taken := 1; } then done;
				action slow { assign taken := 2; } then done;`, 3)
			if !errors.Is(err, errBranchWeights) {
				t.Fatalf("error = %v, want errBranchWeights", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want it to say %q", err, tc.want)
			}
		})
	}
}

// testRandomDrawWithoutASeed: a random function under no model seed and no
// `seed:<n>` schedule refuses to draw, naming the call and the seed to give.
func testRandomDrawWithoutASeed(t *testing.T) {
	err := weightedActionError(t, `
		attribute d : Real = uniform(0.0, 1.0);
		first start; then done;`)
	var unseeded *unseededDrawError
	if !errors.Is(err, ErrUnseededDraw) || !errors.As(err, &unseeded) {
		t.Fatalf("error = %v, want ErrUnseededDraw", err)
	}
	if msg := err.Error(); !strings.Contains(msg, "uniform(0.0, 1.0)") || !strings.Contains(msg, "seed:<n>") {
		t.Fatalf("error = %q, want it to name the call and `seed:<n>`", msg)
	}
}

// testRandomBoundsReversed: uniform(hi, lo) has an empty support and is refused
// before any draw, under a seed or not.
func testRandomBoundsReversed(t *testing.T) {
	for _, call := range []string{"uniform(1.0, 0.0)", "uniformInteger(6, 1)", "triangular(1.0, 0.5, 0.0)", "normal(0.0, -1.0)"} {
		err := weightedActionError(t, "attribute d = "+call+";\n first start; then done;", 3)
		if !errors.Is(err, errRandomDomain) {
			t.Errorf("%s: error = %v, want errRandomDomain", call, err)
		}
	}
}

// testRandomDurationWithoutASeed: a wait whose duration is drawn refuses to park
// unseeded, with the same refusal a random value gets.
func testRandomDurationWithoutASeed(t *testing.T) {
	err := weightedActionError(t, `
		first start;
		then accept after uniform(1, 80) [s];
		then done;`)
	if !errors.Is(err, ErrUnseededDraw) {
		t.Fatalf("error = %v, want ErrUnseededDraw", err)
	}
}
