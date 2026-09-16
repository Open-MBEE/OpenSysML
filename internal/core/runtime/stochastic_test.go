package runtime

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// weightedRouteModel decides between two weighted branches, the less probable
// declared first, and records which one ran.
const weightedRouteModel = `
package test {
	private import ScalarValues::*;
	private import Stochastic::*;
	action route {
		attribute taken : Integer = 0;
		first start;
		then decide select;
		first select then slow { @Probability { p = 0.3; } }
		first select then fast { @Probability { p = 0.7; } }
		action slow { assign taken := 2; }
		then done;
		action fast { assign taken := 1; }
		then done;
	}
}`

// drawingModel draws a Real and an Integer as it starts.
const drawingModel = `
package test {
	private import ScalarValues::*;
	private import RandomFunctions::*;
	action draw {
		attribute d : Real = uniform(0.0, 1.0);
		attribute n : Integer = uniformInteger(1, 6);
		first start; then done;
	}
}`

// runAction runs the action named to completion on a fresh context under policy,
// with the model seed when one is given.
func runAction(t *testing.T, m *exploreModel, name, policy string, seed ...uint64) (*Context, map[string]Value, error) {
	t.Helper()
	ctx, _ := m.fresh()
	if policy != "" {
		mustSchedule(t, ctx, mustPolicy(t, policy))
	}
	if len(seed) > 0 {
		ctx.SetModelSeed(seed[0])
	}
	out, err := ctx.ExecuteAction(m.action(t, name))
	return ctx, out, err
}

// takenInt is the Integer an output holds.
func takenInt(t *testing.T, out map[string]Value, name string) int64 {
	t.Helper()
	v, ok := out[name]
	if !ok || v.Const.Kind != semantics.ValInt {
		t.Fatalf("%s = %v, want an Integer", name, v)
	}
	return v.Const.Int
}

// A weighted decision under a seed draws a branch; the same seed draws the same
// branch, and the trace records the weights and the draw.
func TestWeightedDecisionDrawsUnderASeed(t *testing.T) {
	m := parseLibraryModel(t, weightedRouteModel)
	picks := map[int64]int{}
	for seed := uint64(1); seed <= 40; seed++ {
		policy := fmt.Sprintf("seed:%d", seed)
		_, first, err := runAction(t, m, "route", policy)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		_, again, err := runAction(t, m, "route", policy)
		if err != nil {
			t.Fatalf("seed %d again: %v", seed, err)
		}
		if a, b := takenInt(t, first, "taken"), takenInt(t, again, "taken"); a != b {
			t.Fatalf("seed %d took %d then %d", seed, a, b)
		}
		picks[takenInt(t, first, "taken")]++
	}
	if picks[1] == 0 || picks[2] == 0 {
		t.Errorf("40 seeds took only one branch: %v", picks)
	}
	if picks[1] < picks[2] {
		t.Errorf("the 0.7 branch was taken %d times against %d for the 0.3 one", picks[1], picks[2])
	}

	ctx, _, err := runAction(t, m, "route", "seed:7")
	if err != nil {
		t.Fatal(err)
	}
	choices := ctx.Choices()
	if len(choices) != 1 || !choices[0].Weighted() || !choices[0].Drawn {
		t.Fatalf("choices %v, want one weighted, drawn decision", choices)
	}
	desc := choices[0].Describe()
	if !strings.Contains(desc, "1->slow p=0.3, 2->fast p=0.7") || !strings.Contains(desc, "weighted; drew ") {
		t.Errorf("trace line %q, want the weights and the draw", desc)
	}
	taken := ctx.ChoicesTaken()
	if len(taken) != 1 || !taken[0].Weighted() || taken[0].Weights[0] != 0.3 || taken[0].Weights[1] != 0.7 {
		t.Errorf("witness %v, want the weights carried", taken)
	}
}

// guardedRouteModel weights three branches, the most probable of them guarded off.
const guardedRouteModel = `
package test {
	private import ScalarValues::*;
	private import Stochastic::*;
	action route {
		attribute ready : Boolean = false;
		attribute taken : Integer = 0;
		first start;
		then decide select;
		first select if ready then quick { @Probability { p = 0.6; } }
		first select then slow { @Probability { p = 0.1; } }
		first select then fast { @Probability { p = 0.3; } }
		action quick { assign taken := 3; }
		then done;
		action slow { assign taken := 2; }
		then done;
		action fast { assign taken := 1; }
		then done;
	}
}`

// A weighted branch whose guard does not hold is out of the draw: the holding
// branches are drawn by their weights renormalized, and the most probable of
// them is taken unseeded.
func TestWeightedDecisionDrawsAmongTheHoldingBranches(t *testing.T) {
	m := parseLibraryModel(t, guardedRouteModel)
	picks := map[int64]int{}
	for seed := uint64(1); seed <= 40; seed++ {
		ctx, out, err := runAction(t, m, "route", fmt.Sprintf("seed:%d", seed))
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		picks[takenInt(t, out, "taken")]++
		if choices := ctx.Choices(); len(choices) != 1 || !strings.Contains(choices[0].Describe(), "2->slow p=0.1, 3->fast p=0.3 hold") {
			t.Fatalf("seed %d: choices %v, want the two holding weighted branches", seed, choices)
		}
	}
	if picks[3] != 0 {
		t.Errorf("the guarded-off branch was taken %d times", picks[3])
	}
	if picks[1] == 0 || picks[2] == 0 || picks[1] < picks[2] {
		t.Errorf("picks %v, want both holding branches taken, the 0.3 one oftener", picks)
	}
	_, out, err := runAction(t, m, "route", "declared")
	if err != nil {
		t.Fatal(err)
	}
	if got := takenInt(t, out, "taken"); got != 1 {
		t.Errorf("unseeded took %d, want the most probable holding branch", got)
	}
}

// A model seed fixes the draws under any policy, `seed:<n>` alone seeds the
// modeled stream with n, and a model seed set overrides the schedule's.
// A weighted branch left holding alone is taken without a choice or a draw, its
// weight read all the same.
func TestSoleHoldingWeightedBranchIsTakenWithoutADraw(t *testing.T) {
	m := parseLibraryModel(t, `
package test {
	private import ScalarValues::*;
	private import Stochastic::*;
	action route {
		attribute w : Real = 0.2;
		attribute taken : Integer = 0;
		first start;
		then decide select;
		first select if w < 0.5 then slow { @Probability { p = w; } }
		first select if w >= 0.5 then fast { @Probability { p = 1.0 - w; } }
		action slow { assign taken := 2; }
		then done;
		action fast { assign taken := 1; }
		then done;
	}
}`)
	for _, policy := range []string{"declared", "seed:3"} {
		ctx, out, err := runAction(t, m, "route", policy)
		if err != nil {
			t.Fatalf("%s: %v", policy, err)
		}
		if got := takenInt(t, out, "taken"); got != 2 {
			t.Errorf("%s took %d, want the holding branch", policy, got)
		}
		if choices := ctx.Choices(); len(choices) != 0 {
			t.Errorf("%s: choices %v, want none for one holding branch", policy, choices)
		}
		if draws := ctx.DrawsTaken(); len(draws) != 0 {
			t.Errorf("%s drew %v", policy, draws)
		}
	}
}

func TestModelSeedIsIndependentOfTheScheduleSeed(t *testing.T) {
	m := parseLibraryModel(t, drawingModel)
	_, a, err := runAction(t, m, "draw", "seed:1", 7)
	if err != nil {
		t.Fatal(err)
	}
	_, b, err := runAction(t, m, "draw", "seed:2", 7)
	if err != nil {
		t.Fatal(err)
	}
	_, c, err := runAction(t, m, "draw", "declared", 7)
	if err != nil {
		t.Fatal(err)
	}
	if a["d"].Const.Real != b["d"].Const.Real || a["d"].Const.Real != c["d"].Const.Real {
		t.Errorf("model seed 7 drew %v, %v, %v under three schedules", a["d"], b["d"], c["d"])
	}
	_, d, err := runAction(t, m, "draw", "seed:7")
	if err != nil {
		t.Fatal(err)
	}
	if d["d"].Const.Real != a["d"].Const.Real {
		t.Errorf("`seed:7` drew %v, model seed 7 %v; want one modeled stream per seed", d["d"], a["d"])
	}
}

// Unseeded, a declared or reverse run takes the most probable branch, whatever
// the declaration order, and never draws.
func TestWeightedDecisionTakesTheMostProbableBranchUnseeded(t *testing.T) {
	m := parseLibraryModel(t, weightedRouteModel)
	for _, policy := range []string{"declared", "reverse"} {
		ctx, out, err := runAction(t, m, "route", policy)
		if err != nil {
			t.Fatalf("%s: %v", policy, err)
		}
		if got := takenInt(t, out, "taken"); got != 1 {
			t.Errorf("%s took %d, want the 0.7 branch", policy, got)
		}
		if choices := ctx.Choices(); len(choices) != 1 || choices[0].Drawn {
			t.Errorf("%s: choices %v, want one undrawn decision", policy, choices)
		}
		if draws := ctx.DrawsTaken(); len(draws) != 0 {
			t.Errorf("%s drew %v", policy, draws)
		}
	}
}

// weightedCasesModel weights the decision among the steps of a verification case
// and of an analysis case, the more probable branch written first each time: read
// unweighted, the last unguarded succession would be taken as the else branch.
const weightedCasesModel = `
package test {
	private import ScalarValues::*;
	private import Stochastic::*;

	part def Sensor {
		attribute reading : Integer default = 0;
	}
	part zeroed : Sensor;
	part drifted : Sensor {
		attribute :>> reading = 4;
	}

	verification def ZeroCheck {
		subject sensor : Sensor;
		VerificationCases::PassIf(sensor.reading == 0)
	}

	verification plan {
		subject sensor = zeroed;
		action start;
		then decide route;
		first route then checkZeroed { @Probability { p = 0.8; } }
		first route then checkDrifted { @Probability { p = 0.2; } }
		verification checkZeroed : ZeroCheck {
			subject sensor = test::zeroed;
		}
		verification checkDrifted : ZeroCheck {
			subject sensor = test::drifted;
		}
	}

	analysis sizing {
		subject s = zeroed;
		attribute taken : Integer = 0;
		action start;
		then decide route;
		first route then fast { @Probability { p = 0.8; } }
		first route then slow { @Probability { p = 0.2; } }
		action fast { assign taken := 1; }
		action slow { assign taken := 2; }
		return : Integer = taken;
	}
}`

// A weighted decision among a case's steps is weighted as an action's is: unseeded
// it takes the most probable branch whatever the schedule order, and under a seed
// it is drawn, the same seed drawing the same branch.
func TestWeightedCaseStepsKeepTheirWeights(t *testing.T) {
	verify := func(t *testing.T, policy string) (string, ChoicePoint) {
		t.Helper()
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, weightedCasesModel))
		mustSchedule(t, ctx, mustPolicy(t, policy))
		result, err := ctx.RunVerification(oneSymbol(t, idx, "test::plan"), AnalysisArgs{}, nil, nil)
		if err != nil {
			t.Fatalf("RunVerification: %v", err)
		}
		if len(result.Subcases) != 1 {
			t.Fatalf("subcases = %+v, want the one branch performed", result.Subcases)
		}
		choices := ctx.Choices()
		if len(choices) != 1 || !choices[0].Weighted() {
			t.Fatalf("choices %v, want one weighted decision", choices)
		}
		return result.Subcases[0].Case, choices[0]
	}
	analyze := func(t *testing.T, policy string) (int64, ChoicePoint) {
		t.Helper()
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, weightedCasesModel))
		mustSchedule(t, ctx, mustPolicy(t, policy))
		run, err := ctx.RunAnalysis(oneSymbol(t, idx, "test::sizing"), AnalysisArgs{}, nil, nil)
		if err != nil {
			t.Fatalf("RunAnalysis: %v", err)
		}
		if len(run.Outputs) != 1 || run.Outputs[0].Value.Const.Kind != semantics.ValInt {
			t.Fatalf("outputs = %+v, want the one Integer result", run.Outputs)
		}
		choices := ctx.Choices()
		if len(choices) != 1 || !choices[0].Weighted() {
			t.Fatalf("choices %v, want one weighted decision", choices)
		}
		return run.Outputs[0].Value.Const.Int, choices[0]
	}

	for _, policy := range []string{"declared", "reverse"} {
		sub, choice := verify(t, policy)
		if !strings.HasSuffix(sub, "checkZeroed") || choice.Drawn {
			t.Errorf("%s performed %s (drawn %v), want the 0.8 branch undrawn", policy, sub, choice.Drawn)
		}
		taken, choice := analyze(t, policy)
		if taken != 1 || choice.Drawn {
			t.Errorf("%s computed %d (drawn %v), want the 0.8 branch's 1 undrawn", policy, taken, choice.Drawn)
		}
	}
	sub, choice := verify(t, "seed:3")
	if again, _ := verify(t, "seed:3"); again != sub || !choice.Drawn {
		t.Errorf("seed:3 performed %s then %s (drawn %v), want one drawn branch reproduced", sub, again, choice.Drawn)
	}
	taken, choice := analyze(t, "seed:3")
	if again, _ := analyze(t, "seed:3"); again != taken || !choice.Drawn {
		t.Errorf("seed:3 computed %d then %d (drawn %v), want one drawn branch reproduced", taken, again, choice.Drawn)
	}
}

// Explore still enumerates every branch of a weighted decision, weights notwithstanding.
func TestExploreEnumeratesWeightedBranches(t *testing.T) {
	m := parseLibraryModel(t, weightedRouteModel)
	x := m.exploreAction(t, "explore", "route")
	if !x.Complete() || len(x.Outcomes) != 2 {
		t.Fatalf("explore found %v, want both branches", outcomeTexts(x))
	}
	for _, o := range x.Outcomes {
		if len(o.Witness) != 1 || !o.Witness[0].Weighted() {
			t.Errorf("witness %v, want the weighted decision", o.Witness)
		}
	}
}

// A random function call without a seed, model or schedule, is a typed refusal
// naming the call and the seed it needs; a probe of it draws nothing.
func TestRandomFunctionRefusesToDrawUnseeded(t *testing.T) {
	m := parseLibraryModel(t, drawingModel)
	for _, policy := range []string{"", "declared", "reverse"} {
		ctx, _, err := runAction(t, m, "draw", policy)
		var unseeded *UnseededDrawError
		if !errors.As(err, &unseeded) || !errors.Is(err, ErrUnseededDraw) {
			t.Fatalf("%q: error %T %v, want an UnseededDrawError", policy, err, err)
		}
		if unseeded.What != "uniform(0.0, 1.0)" || !strings.Contains(err.Error(), "seed:<n>") {
			t.Errorf("%q: error %q, want it to name the call and the seed", policy, err)
		}
		if len(ctx.DrawsTaken()) != 0 {
			t.Errorf("%q: draws %v recorded by a refused run", policy, ctx.DrawsTaken())
		}
	}
}

// Every random function refuses arguments bounding no distribution with a typed
// error, before any draw is made.
func TestRandomFunctionsRefuseAnEmptyDomain(t *testing.T) {
	cases := []struct{ name, expr, want string }{
		{"uniform reversed", "uniform(80.0, 1.0)", "lo exceeds hi"},
		{"uniformInteger reversed", "uniformInteger(6, 1)", "lo exceeds hi"},
		{"triangular mode outside", "triangular(0.0, 2.0, 1.0)", "needs lo <= mode <= hi"},
		{"triangular flat", "triangular(1.0, 1.0, 1.0)", "needs lo <= mode <= hi with lo < hi"},
		{"normal negative sd", "normal(0.0, -1.0)", "sd is negative"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := parseLibraryModel(t, `
				package test {
					private import ScalarValues::*;
					private import RandomFunctions::*;
					action draw {
						attribute d : Real = `+tc.expr+`;
						first start; then done;
					}
				}`)
			ctx, _, err := runAction(t, m, "draw", "seed:1")
			if !errors.Is(err, ErrRandomDomain) {
				t.Fatalf("error %v, want ErrRandomDomain", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q, want it to say %q", err, tc.want)
			}
			if len(ctx.DrawsTaken()) != 0 {
				t.Errorf("draws %v made by a refused call", ctx.DrawsTaken())
			}
		})
	}
}

// A bound that is not a finite number, which no expression the evaluator accepts
// produces but a library caller may hand in, is refused before any draw.
func TestRandomFunctionsRefuseANonFiniteBound(t *testing.T) {
	m := parseLibraryModel(t, drawingModel)
	ctx, _ := m.fresh()
	ctx.SetModelSeed(1)
	ctx.run.scheduler = ctx.schedulerUnder(ctx.schedule)
	for _, bad := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		args := []semantics.Value{drawnReal(0), drawnReal(bad)}
		if _, err := drawUniform(ctx, RandomFunctionsFQN+"::uniform", args); !errors.Is(err, ErrRandomDomain) || !strings.Contains(err.Error(), "is not a finite number") {
			t.Errorf("uniform(0, %v) = %v, want ErrRandomDomain naming the bound", bad, err)
		}
		if _, err := drawNormal(ctx, RandomFunctionsFQN+"::normal", args); !errors.Is(err, ErrRandomDomain) {
			t.Errorf("normal(0, %v) = %v, want ErrRandomDomain", bad, err)
		}
		if _, err := drawTriangular(ctx, RandomFunctionsFQN+"::triangular", []semantics.Value{drawnReal(0), drawnReal(bad), drawnReal(1)}); !errors.Is(err, ErrRandomDomain) {
			t.Errorf("triangular(0, %v, 1) = %v, want ErrRandomDomain", bad, err)
		}
	}
	if len(ctx.DrawsTaken()) != 0 {
		t.Errorf("draws %v made by refused calls", ctx.DrawsTaken())
	}
}

// uniformInteger draws both ends of its range, exactly the one value of a
// zero-width range, and the whole Integer range without overflowing.
func TestUniformIntegerCoversItsRangeInclusively(t *testing.T) {
	draw := func(t *testing.T, call string, seeds int) []int64 {
		t.Helper()
		m := parseLibraryModel(t, `
			package test {
				private import ScalarValues::*;
				private import RandomFunctions::*;
				action draw {
					attribute n : Integer = `+call+`;
					first start; then done;
				}
			}`)
		values := make([]int64, 0, seeds)
		for seed := 1; seed <= seeds; seed++ {
			_, out, err := runAction(t, m, "draw", "declared", uint64(seed))
			if err != nil {
				t.Fatalf("%s under seed %d: %v", call, seed, err)
			}
			values = append(values, takenInt(t, out, "n"))
		}
		return values
	}

	seen := map[int64]bool{}
	for _, n := range draw(t, "uniformInteger(1, 3)", 60) {
		if n < 1 || n > 3 {
			t.Fatalf("uniformInteger(1, 3) drew %d", n)
		}
		seen[n] = true
	}
	if !seen[1] || !seen[2] || !seen[3] {
		t.Errorf("60 draws of uniformInteger(1, 3) saw %v, want both ends", seen)
	}
	for _, n := range draw(t, "uniformInteger(5, 5)", 3) {
		if n != 5 {
			t.Errorf("uniformInteger(5, 5) drew %d", n)
		}
	}
	for _, n := range draw(t, "uniformInteger(-9223372036854775807 - 1, 9223372036854775807)", 8) {
		_ = n // any Integer is in range; the draw must not fail or overflow
	}
	negative := draw(t, "uniformInteger(-3, -1)", 20)
	for _, n := range negative {
		if n < -3 || n > -1 {
			t.Errorf("uniformInteger(-3, -1) drew %d", n)
		}
	}
	if a, b := draw(t, "uniformInteger(1, 1000000)", 1), draw(t, "uniformInteger(1, 1000000)", 1); a[0] != b[0] {
		t.Errorf("seed 1 drew %d then %d", a[0], b[0])
	}
}

// Each random function draws within its support and, under a fixed seed, the same value.
func TestRandomFunctionsDrawWithinTheirSupport(t *testing.T) {
	m := parseLibraryModel(t, `
		package test {
			private import ScalarValues::*;
			private import RandomFunctions::*;
			action draw {
				attribute u : Real = uniform(2.0, 3.0);
				attribute tri : Real = triangular(0.0, 1.0, 4.0);
				attribute flat : Real = normal(5.0, 0.0);
				attribute g : Real = normal(0.0, 1.0);
				first start; then done;
			}
		}`)
	var last map[string]Value
	for seed := uint64(1); seed <= 30; seed++ {
		_, out, err := runAction(t, m, "draw", "declared", seed)
		if err != nil {
			t.Fatal(err)
		}
		if u := out["u"].Const.Real; u < 2 || u >= 3 {
			t.Errorf("uniform(2.0, 3.0) drew %v", u)
		}
		if x := out["tri"].Const.Real; x < 0 || x > 4 {
			t.Errorf("triangular(0.0, 1.0, 4.0) drew %v", x)
		}
		if x := out["flat"].Const.Real; x != 5 {
			t.Errorf("normal(5.0, 0.0) drew %v, want the mean", x)
		}
		if x := out["g"].Const.Real; math.IsNaN(x) || math.IsInf(x, 0) {
			t.Errorf("normal(0.0, 1.0) drew %v", x)
		}
		last = out
	}
	_, again, err := runAction(t, m, "draw", "declared", 30)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"u", "tri", "g"} {
		if again[name].Const.Real != last[name].Const.Real {
			t.Errorf("seed 30 drew %s = %v then %v", name, last[name], again[name])
		}
	}
}

// normal never draws what its witness would refuse: with parameters near the
// largest Real the raw draw overflows to infinity on some seeds, and each such
// run still yields a finite value that replays.
func TestNormalDrawsStayFiniteNearTheLargestReal(t *testing.T) {
	m := parseLibraryModel(t, `
		package test {
			private import ScalarValues::*;
			private import RandomFunctions::*;
			action draw {
				attribute x : Real = normal(1.0e308, 1.0e308);
				first start; then done;
			}
		}`)
	overflowed := 0
	for seed := uint64(1); seed <= 40; seed++ {
		if raw := 1.0e308 + 1.0e308*newModeledSource(seed).rng.NormFloat64(); math.IsInf(raw, 0) {
			overflowed++
		}
		ctx, out, err := runAction(t, m, "draw", "declared", seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		x := out["x"].Const.Real
		if math.IsInf(x, 0) || math.IsNaN(x) {
			t.Fatalf("seed %d drew normal(1.0e308, 1.0e308) = %v", seed, x)
		}
		w := Witness{Draws: ctx.DrawsTaken()}
		replay, _ := m.fresh()
		mustSchedule(t, replay, ReplayOf(w))
		got, err := replay.ExecuteAction(m.action(t, "draw"))
		if err == nil {
			err = replay.Unfollowed()
		}
		if err != nil {
			t.Fatalf("seed %d: replay: %v\n%s", seed, err, w)
		}
		if got["x"].Const.Real != x {
			t.Errorf("seed %d: replay gave x = %v, want %v", seed, got["x"].Const.Real, x)
		}
	}
	if overflowed == 0 {
		t.Fatal("no seed's raw draw overflowed; the test exercises nothing")
	}
}

// Bounds a float apart — where hi-lo, or the products the triangular inverse
// forms, overflow — still bound the draws: every one is finite, within the
// interval, spread over it rather than piled at hi, and replays.
func TestWideBoundsStillBoundTheDraws(t *testing.T) {
	m := parseLibraryModel(t, `
		package test {
			private import ScalarValues::*;
			private import RandomFunctions::*;
			action draw {
				attribute u : Real = uniform(-1.0e308, 1.0e308);
				attribute t : Real = triangular(-1.0e308, 0.0, 1.0e308);
				attribute s : Real = triangular(0.0, 1.0e200, 1.0e200);
				first start; then done;
			}
		}`)
	bounds := map[string][2]float64{
		"u": {-1.0e308, 1.0e308},
		"t": {-1.0e308, 1.0e308},
		"s": {0, 1.0e200},
	}
	signs := map[string]map[bool]int{"u": {}, "t": {}}
	for seed := uint64(1); seed <= 40; seed++ {
		ctx, out, err := runAction(t, m, "draw", "declared", seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		for name, bound := range bounds {
			x := out[name].Const.Real
			if math.IsInf(x, 0) || math.IsNaN(x) || x < bound[0] || x > bound[1] || (name == "u" && x == bound[1]) {
				t.Fatalf("seed %d drew %s = %v, outside its bounds %v", seed, name, x, bound)
			}
			if counts, spread := signs[name]; spread {
				counts[x < 0]++
			}
		}
		w := Witness{Draws: ctx.DrawsTaken()}
		replay, _ := m.fresh()
		mustSchedule(t, replay, ReplayOf(w))
		got, err := replay.ExecuteAction(m.action(t, "draw"))
		if err == nil {
			err = replay.Unfollowed()
		}
		if err != nil {
			t.Fatalf("seed %d: replay: %v\n%s", seed, err, w)
		}
		for name := range bounds {
			if got[name].Const.Real != out[name].Const.Real {
				t.Errorf("seed %d: replay gave %s = %v, want %v", seed, name, got[name].Const.Real, out[name].Const.Real)
			}
		}
	}
	for name, counts := range signs {
		if counts[true] == 0 || counts[false] == 0 {
			t.Errorf("%s drew %d negative and %d non-negative values over 40 seeds: not spread over its interval", name, counts[true], counts[false])
		}
	}
}

// between and fractionOf are the arithmetic the wide draws rest on: exact at the
// ends, finite and within the interval wherever the span itself is not a float.
func TestBetweenAndFractionOfWideSpans(t *testing.T) {
	lo, hi := -math.MaxFloat64, math.MaxFloat64
	for _, tc := range []struct {
		t, want float64
	}{{0, lo}, {1, hi}, {0.5, 0}} {
		if got := between(lo, hi, tc.t); got != tc.want {
			t.Errorf("between(-max, max, %v) = %v, want %v", tc.t, got, tc.want)
		}
	}
	if q := between(lo, hi, 0.25); math.IsInf(q, 0) || q >= 0 || q <= lo {
		t.Errorf("between(-max, max, 0.25) = %v, want a finite negative", q)
	}
	if got := between(1, 80, 0.5); got != 1+0.5*79 {
		t.Errorf("between(1, 80, 0.5) = %v, want the plain interpolation %v", got, 1+0.5*79)
	}
	for _, tc := range []struct {
		lo, x, hi, want float64
	}{{lo, 0, hi, 0.5}, {lo, lo, hi, 0}, {lo, hi, hi, 1}, {0, 1, 4, 0.25}, {0, 1.0e200, 1.0e200, 1}} {
		if got := fractionOf(tc.lo, tc.x, tc.hi); got != tc.want {
			t.Errorf("fractionOf(%v, %v, %v) = %v, want %v", tc.lo, tc.x, tc.hi, got, tc.want)
		}
	}
}

// A run's witness records its draws, and replaying it reproduces the run: the same
// values, the same branch, every draw consumed, no generator consulted.
func TestReplayReproducesTheDrawsAndTheWeightedBranch(t *testing.T) {
	m := parseLibraryModel(t, `
		package test {
			private import ScalarValues::*;
			private import Stochastic::*;
			private import RandomFunctions::*;
			action route {
				attribute taken : Integer = 0;
				attribute d : Real = uniform(0.0, 10.0);
				attribute n : Integer = uniformInteger(1, 6);
				first start;
				then decide select;
				first select then slow { @Probability { p = 0.3; } }
				first select then fast { @Probability { p = 0.7; } }
				action slow { assign taken := 2; }
				then done;
				action fast { assign taken := 1; }
				then done;
			}
		}`)
	for seed := uint64(1); seed <= 6; seed++ {
		ctx, out, err := runAction(t, m, "route", fmt.Sprintf("seed:%d", seed))
		if err != nil {
			t.Fatal(err)
		}
		w := Witness{Draws: ctx.DrawsTaken(), Choices: ctx.ChoicesTaken()}
		if len(w.Draws) != 2 || w.Draws[0].What != "uniform(0.0, 10.0)" || w.Draws[1].What != "uniformInteger(1, 6)" {
			t.Fatalf("seed %d recorded %v, want the two draws in order", seed, w.Draws)
		}
		parsed, err := ParseWitness(w.String())
		if err != nil {
			t.Fatalf("seed %d: the witness does not read back: %v\n%s", seed, err, w)
		}
		if len(w.Choices) != 1 || !w.Choices[0].Drawn || !reflect.DeepEqual(parsed.Choices, w.Choices) {
			t.Fatalf("seed %d: the witness reads back as %+v, want the drawn weighted choice %+v", seed, parsed.Choices, w.Choices)
		}
		replay, _ := m.fresh()
		mustSchedule(t, replay, ReplayOf(parsed))
		got, err := replay.ExecuteAction(m.action(t, "route"))
		if err == nil {
			err = replay.Unfollowed()
		}
		if err != nil {
			t.Fatalf("seed %d: replay: %v\n%s", seed, err, w)
		}
		for _, name := range []string{"taken", "d", "n"} {
			if got[name].Const != out[name].Const {
				t.Errorf("seed %d: replay gave %s = %v, want %v", seed, name, got[name], out[name])
			}
		}
		if again := replay.DrawsTaken(); len(again) != 2 || again[0] != w.Draws[0] || again[1] != w.Draws[1] {
			t.Errorf("seed %d: the replay recorded %v, want the witness's draws", seed, again)
		}
	}
}

// A witness whose draws the run cannot consume is refused with a typed error:
// one drawn for another call, one missing, one left over, one the call could not
// have drawn, and one spelt unreadably.
func TestReplayRefusesDrawsItCannotConsume(t *testing.T) {
	m := parseLibraryModel(t, drawingModel)
	ctx, _, err := runAction(t, m, "draw", "seed:3")
	if err != nil {
		t.Fatal(err)
	}
	good := Witness{Draws: ctx.DrawsTaken()}
	real := func(x float64) semantics.Value { return semantics.Value{Kind: semantics.ValReal, Real: x} }
	integer := func(n int64) semantics.Value { return semantics.Value{Kind: semantics.ValInt, Int: n} }
	cases := []struct {
		name  string
		draws []DrawTaken
		draw  int
		want  string
	}{
		{"another call", []DrawTaken{{What: "normal(0.0, 1.0)", Value: real(0.5)}, good.Draws[1]}, 1, "the run drew uniform(0.0, 1.0) instead"},
		{"missing", good.Draws[:1], 0, "records no draw left for it"},
		{"left over", append(append([]DrawTaken{}, good.Draws...), DrawTaken{What: "uniform(0.0, 1.0)", Value: real(0.25)}), 3, "the run ended without drawing it"},
		{"none at all", nil, 0, "records no draw left for it"},
		{"real above hi", []DrawTaken{{What: good.Draws[0].What, Value: real(2)}, good.Draws[1]}, 1, "records 2.0, which the call cannot draw"},
		{"real below lo", []DrawTaken{{What: good.Draws[0].What, Value: real(-0.5)}, good.Draws[1]}, 1, "records -0.5, which the call cannot draw"},
		{"real for an integer", []DrawTaken{good.Draws[0], {What: good.Draws[1].What, Value: real(3.5)}}, 2, "records 3.5, which the call cannot draw"},
		{"integer past hi", []DrawTaken{good.Draws[0], {What: good.Draws[1].What, Value: integer(7)}}, 2, "records 7, which the call cannot draw"},
		{"integer for a real", []DrawTaken{{What: good.Draws[0].What, Value: integer(0)}, good.Draws[1]}, 1, "records 0, which the call cannot draw"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			replay, _ := m.fresh()
			mustSchedule(t, replay, ReplayOf(Witness{Draws: tc.draws}))
			_, err := replay.ExecuteAction(m.action(t, "draw"))
			if err == nil {
				err = replay.Unfollowed()
			}
			var refused *WitnessDrawError
			if !errors.As(err, &refused) || !errors.Is(err, ErrWitnessDraw) {
				t.Fatalf("error %T %v, want a WitnessDrawError", err, err)
			}
			if refused.Draw != tc.draw || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = draw %d %q, want draw %d saying %q", refused.Draw, err, tc.draw, tc.want)
			}
		})
	}

	for _, line := range []string{"draw uniform(0.0, 1.0)", "draw = 0.5", "draw uniform(0.0, 1.0) = half", "drew uniform(0.0, 1.0) = 0.5"} {
		if _, err := ParseDraw(line); !errors.Is(err, ErrInvalidDraw) {
			t.Errorf("ParseDraw(%q) = %v, want ErrInvalidDraw", line, err)
		}
	}
	if _, err := ParseWitness("draw uniform(0.0, 1.0) = half\n"); !errors.Is(err, ErrInvalidDraw) {
		t.Errorf("a witness with a malformed draw parsed: %v", err)
	}
	for _, d := range good.Draws {
		back, err := ParseDraw(d.String())
		if err != nil || back != d {
			t.Errorf("ParseDraw(%q) = %v, %v; want the draw back", d, back, err)
		}
	}
}

// A replayed draw must lie where its call's distribution puts it: a triangular draw
// within its bounds, a normal draw finite and, at zero deviation, at the mean.
func TestReplayRefusesDrawsOutsideTheCallsDistribution(t *testing.T) {
	m := parseLibraryModel(t, `
		package test {
			private import ScalarValues::*;
			private import RandomFunctions::*;
			action draw {
				attribute a : Real = triangular(0.0, 1.0, 2.0);
				attribute b : Real = normal(5.0, 0.0);
				attribute c : Real = normal(0.0, 1.0);
				attribute u : Real = uniform(0.0, 1.0);
				attribute z : Real = uniform(3.0, 3.0);
				first start; then done;
			}
		}`)
	ctx, _, err := runAction(t, m, "draw", "seed:3")
	if err != nil {
		t.Fatal(err)
	}
	good := ctx.DrawsTaken()
	if len(good) != 5 {
		t.Fatalf("recorded %v, want five draws", good)
	}
	real := func(x float64) semantics.Value { return semantics.Value{Kind: semantics.ValReal, Real: x} }
	with := func(i int, v semantics.Value) []DrawTaken {
		draws := append([]DrawTaken{}, good...)
		draws[i].Value = v
		return draws
	}
	cases := []struct {
		name  string
		draws []DrawTaken
		ok    bool
	}{
		{"triangular at lo", with(0, real(0)), true},
		{"triangular at hi", with(0, real(2)), true},
		{"triangular past hi", with(0, real(2.5)), false},
		{"zero deviation at the mean", with(1, real(5)), true},
		{"zero deviation off the mean", with(1, real(5.1)), false},
		{"normal far out", with(2, real(-40)), true},
		{"normal infinite", with(2, real(math.Inf(1))), false},
		{"uniform at lo", with(3, real(0)), true},
		{"uniform just below hi", with(3, real(math.Nextafter(1, 0))), true},
		{"uniform at hi", with(3, real(1)), false},
		{"uniform below lo", with(3, real(-0.1)), false},
		{"zero-width uniform at its one value", with(4, real(3)), true},
		{"zero-width uniform off its one value", with(4, real(3.1)), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			replay, _ := m.fresh()
			mustSchedule(t, replay, ReplayOf(Witness{Draws: tc.draws}))
			_, err := replay.ExecuteAction(m.action(t, "draw"))
			if err == nil {
				err = replay.Unfollowed()
			}
			if tc.ok {
				if err != nil {
					t.Fatalf("replay refused a draw the call could make: %v", err)
				}
				return
			}
			if !errors.Is(err, ErrWitnessDraw) || !strings.Contains(err.Error(), "which the call cannot draw") {
				t.Fatalf("error = %v, want a WitnessDrawError naming the draw the call cannot make", err)
			}
		})
	}
}

// A replayed witness's weighted choice is followed as recorded, even where a seed
// would draw the other branch, and a choice not among the holding branches is refused.
func TestReplayFollowsTheRecordedWeightedBranch(t *testing.T) {
	m := parseLibraryModel(t, weightedRouteModel)
	x := m.exploreAction(t, "explore", "route")
	for _, o := range x.Outcomes {
		replay, _ := m.fresh()
		mustSchedule(t, replay, ReplayOf(Witness{Choices: o.Witness}))
		replay.SetModelSeed(7)
		out, err := replay.ExecuteAction(m.action(t, "route"))
		if err == nil {
			err = replay.Unfollowed()
		}
		if err != nil {
			t.Fatalf("replay of %s: %v", FormatChoices(o.Witness), err)
		}
		if got := fmt.Sprintf("taken = %d", takenInt(t, out, "taken")); got != o.Outcome.String() {
			t.Errorf("replay of %s reached %s, want %s", FormatChoices(o.Witness), got, o.Outcome)
		}
		if choices := replay.Choices(); len(choices) != 1 || choices[0].Drawn {
			t.Errorf("the replay drew a branch: %v", choices)
		}
	}
}

// A weighted witness is followed only where it fits the decision the run faces: the
// same branches weighed the same, and a recorded draw that selects the branch it took.
// One weighed otherwise, drawn impossibly, or drawn for the other branch is refused
// as a stale witness is, whatever the model seed would draw.
func TestReplayRefusesAWeightedMoveTheDecisionDoesNotFit(t *testing.T) {
	m := parseLibraryModel(t, weightedRouteModel)
	ctx, _, err := runAction(t, m, "route", "declared", 7)
	if err != nil {
		t.Fatal(err)
	}
	good := ctx.ChoicesTaken()
	if len(good) != 1 || !good[0].Drawn || good[0].Took != "2->fast" {
		t.Fatalf("seed 7 recorded %v, want one drawn choice of 2->fast", FormatChoices(good))
	}
	replayOf := func(w ChoiceTaken) error {
		replay, _ := m.fresh()
		mustSchedule(t, replay, ReplayOf(Witness{Choices: []ChoiceTaken{w}}))
		replay.SetModelSeed(7)
		if _, err := replay.ExecuteAction(m.action(t, "route")); err != nil {
			return err
		}
		return replay.Unfollowed()
	}
	if err := replayOf(good[0]); err != nil {
		t.Fatalf("the run's own witness is refused: %v", err)
	}
	reweighed := func(alter func(w *ChoiceTaken)) ChoiceTaken {
		w := good[0]
		w.Among, w.Weights = slices.Clone(w.Among), slices.Clone(w.Weights)
		alter(&w)
		return w
	}
	cases := []struct {
		name  string
		move  ChoiceTaken
		faced string
	}{
		{"a weight changed", reweighed(func(w *ChoiceTaken) { w.Weights[0], w.Weights[1] = 0.6, 0.4 }), "1->slow weighs p=0.3, not p=0.6"},
		{"a branch fewer", reweighed(func(w *ChoiceTaken) {
			w.Among, w.Weights, w.Alternatives, w.Taken = w.Among[1:], w.Weights[1:], 1, 0
		}), "the run's decision weighs every branch: 1->slow p=0.3, 2->fast p=0.7"},
		{"no weights recorded", reweighed(func(w *ChoiceTaken) { w.Weights, w.Drawn = nil, false }), "the run's decision weighs every branch"},
		{"a branch weighed twice", reweighed(func(w *ChoiceTaken) {
			w.Among[0], w.Weights[0] = "2->fast", 0.7
		}), "the move weighs 2->fast twice: 2->fast, 2->fast"},
		{"a branch the run does not face", reweighed(func(w *ChoiceTaken) {
			w.Among[0] = "3->never"
		}), "3->never is not enabled (enabled: 1->slow, 2->fast)"},
		{"a draw of one", reweighed(func(w *ChoiceTaken) { w.Drew = 1 }), "the draw 1 is no unit draw in [0, 1)"},
		{"a negative draw", reweighed(func(w *ChoiceTaken) { w.Drew = -0.5 }), "the draw -0.5 is no unit draw in [0, 1)"},
		{"a draw that is not a number", reweighed(func(w *ChoiceTaken) { w.Drew = math.NaN() }), "is no unit draw in [0, 1)"},
		{"a draw selecting the other branch", reweighed(func(w *ChoiceTaken) { w.Drew = 0.1 }), "the draw 0.1 selects 1->slow, not 2->fast"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := replayOf(c.move)
			var refused *ReplayError
			if !errors.As(err, &refused) || !errors.Is(err, ErrReplayRefused) {
				t.Fatalf("error %T %v, want a ReplayError", err, err)
			}
			if refused.Move != 1 || !strings.Contains(err.Error(), c.faced) {
				t.Errorf("error %q, want move 1 refused as %q", err, c.faced)
			}
		})
	}
	unweighted, err := ParseChoices(fmt.Sprintf("step %d: decision select -> 1->slow", good[0].Step))
	if err != nil {
		t.Fatal(err)
	}
	if err = replayOf(unweighted[0]); !errors.Is(err, ErrReplayRefused) || !strings.Contains(err.Error(), "the run's decision weighs every branch") {
		t.Fatalf("an unweighted move at the weighted decision: %v, want it refused", err)
	}
}

// A replayed weighted move carries the witness's draw, so the replay's own witness
// reads as the run's did.
func TestReplayCarriesTheWitnessedDraw(t *testing.T) {
	m := parseLibraryModel(t, weightedRouteModel)
	ctx, _, err := runAction(t, m, "route", "declared", 7)
	if err != nil {
		t.Fatal(err)
	}
	w := Witness{Choices: ctx.ChoicesTaken()}
	replay, _ := m.fresh()
	mustSchedule(t, replay, ReplayOf(w))
	if _, err := replay.ExecuteAction(m.action(t, "route")); err != nil {
		t.Fatal(err)
	}
	if err := replay.Unfollowed(); err != nil {
		t.Fatal(err)
	}
	if again := FormatChoices(replay.ChoicesTaken()); again != FormatChoices(w.Choices) {
		t.Errorf("the replay recorded %s, want the witness %s", again, FormatChoices(w.Choices))
	}
}

// A probe of a run that draws — a checker's or explorer's preview — leaves the
// modeled stream where it was: the run after it draws what it would have.
func TestProbeRestoresTheModeledStream(t *testing.T) {
	m := parseLibraryModel(t, drawingModel)
	ctx, _ := m.fresh()
	mustSchedule(t, ctx, mustPolicy(t, "seed:5"))
	ctx.SetModelSeed(11)
	ctx.run.scheduler = ctx.schedulerUnder(ctx.schedule)
	unit := distribution{
		draw:   func(rng *rand.Rand) semantics.Value { return drawnReal(rng.Float64()) },
		admits: realWithin(0, 1),
	}
	peek := func() semantics.Value {
		saved := *ctx.run.scheduler.modeled.pcg
		v, err := ctx.run.scheduler.draw("peek", unit)
		if err != nil {
			t.Fatal(err)
		}
		*ctx.run.scheduler.modeled.pcg = saved
		return v
	}
	before := peek()

	end := ctx.beginProbe()
	if _, err := ctx.draw("probe", unit); err != nil {
		t.Fatal(err)
	}
	if len(ctx.DrawsTaken()) != 0 {
		t.Errorf("a probe's draw was recorded: %v", ctx.DrawsTaken())
	}
	end()
	if after := peek(); after != before {
		t.Errorf("the probe moved the modeled stream: %v then %v", before, after)
	}
}

// Under check, a weighted decision is searched like any other: both branches
// are reached and reported as finals, the weights only recorded in the witness.
func TestCheckSearchesWeightedBranchesAsASet(t *testing.T) {
	m := parseLibraryModel(t, weightedRouteModel)
	report := checkModel(t, m, "route", CheckBudget{}, CheckOptions{})
	if len(report.Finals) != 2 {
		t.Fatalf("check found %d final(s), want the two branches: %s", len(report.Finals), report.Status())
	}
	for _, final := range report.Finals {
		choices := final.Witness.Choices
		if len(choices) != 1 || !choices[0].Weighted() {
			t.Errorf("final %v carries the witness %v, want the weighted decision", final.Outcome, choices)
		}
	}
}

// Two check paths that consume different numbers of draws before reaching the
// same configuration and values are different states: the seeded stream stands
// elsewhere on each, so what the run draws next differs.
func TestCheckKeepsPathsApartByTheirModeledStream(t *testing.T) {
	m := parseLibraryModel(t, `
package test {
	private import ScalarValues::*;
	private import RandomFunctions::*;
	state def Machine {
		attribute x : Real = 0.0;
		attribute seen : Real = 0.0;
		entry; then a;
		state a;
		transition first a if true do { assign x := uniform(0.0, 1.0); } then b;
		transition first a if true do { assign x := 0.0; } then b;
		state b { entry { assign x := 0.0; } }
		transition first b if true do { assign seen := uniform(0.0, 1.0); } then c;
		state c;
	}
}`)
	seeded := func() (*Context, error) {
		ctx, err := m.fresh()
		if err == nil {
			ctx.SetModelSeed(3)
		}
		return ctx, err
	}
	start := invocationOf(nil, []*symbols.Symbol{m.state(t, "Machine")})
	report, err := Check(context.Background(), seeded, start, CheckBudget{}, CheckOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Violations) > 0 {
		t.Fatalf("check failed: %v", report.Violations)
	}
	if got := divergentValues(report, "seen"); len(got) != 2 || got[0] == got[1] {
		t.Fatalf("seen takes %v over the two paths, want a value per stream position: %s", got, report.Status())
	}
	if len(report.Finals) != 2 {
		t.Errorf("check found %d final(s), want one per path", len(report.Finals))
	}
}
