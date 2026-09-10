package analysis

import (
	"context"
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// raceRun is one run of the fixture's racing action.
func raceRun(t *testing.T, f *fixture) Linearization {
	t.Helper()
	race := f.symbol(t, "race")
	return func(ctx *runtime.Context) (runtime.Outcome, error) {
		outputs, err := ctx.ExecuteAction(race)
		if err != nil {
			return runtime.Outcome{}, err
		}
		return ctx.ActionOutcome(outputs), nil
	}
}

// outcomes asks the default registry for the racing action's outcomes under the policy.
func outcomes(t *testing.T, f *fixture, spelling string, budget Budget) Plan {
	t.Helper()
	q := Question{Kind: Outcomes, Subject: "test::race", Schedule: policy(t, spelling), Free: FreeSchedule, Linearize: raceRun(t, f)}
	return answered(t, Default(), &Model{Fresh: f.fresh}, q, budget)
}

func TestExploreProvesACompleteOutcomeSet(t *testing.T) {
	f := parseFixture(t)
	plan := outcomes(t, f, "explore", Budget{})
	result := plan.Result
	if result.Engine != ExploreEngineName || result.Claim != ClaimOutcomes || result.Strength != Proved {
		t.Fatalf("result %+v, want explore's proved outcomes", result)
	}
	x := result.Exploration()
	if x == nil || !x.Complete() || x.Runs != 6 || len(x.Outcomes) != 3 {
		t.Fatalf("exploration %+v, want 6 runs reaching 3 outcomes", x)
	}
	if result.Bounds.Reached() {
		t.Fatalf("bounds %s, want none reached", result.Bounds)
	}
	if runs, ok := result.Bounds.Limit("runs"); !ok || runs != int64(x.Budget.Runs) {
		t.Fatalf("bounds %s, want the policy's runs budget", result.Bounds)
	}
	if !sameNames(stepNames(plan), []string{ExploreEngineName}) {
		t.Fatalf("steps %v, want explore alone", stepNames(plan))
	}
}

func TestExploreObservesAnIncompleteOutcomeSet(t *testing.T) {
	f := parseFixture(t)
	result := outcomes(t, f, "explore:runs=1", Budget{}).Result
	if result.Claim != ClaimOutcomes || result.Strength != Observed {
		t.Fatalf("result %+v, want observed outcomes", result)
	}
	x := result.Exploration()
	if x == nil || x.Complete() || x.Runs != 1 {
		t.Fatalf("exploration %+v, want 1 run short of complete", x)
	}
	for _, b := range result.Bounds {
		if b.Name == "runs" && (b.Limit != 1 || !b.Reached) {
			t.Fatalf("bounds %s, want runs=1 reached", result.Bounds)
		}
		if b.Name == "depth" && b.Reached {
			t.Fatalf("bounds %s, want depth unreached", result.Bounds)
		}
	}
}

func TestExploreTakesTheBudgetsRunsAndDepth(t *testing.T) {
	f := parseFixture(t)
	x := outcomes(t, f, "explore", Budget{Runs: 2}).Result.Exploration()
	if x == nil || x.Budget.Runs != 2 || x.Runs != 2 || x.Complete() {
		t.Fatalf("exploration %+v, want the budget's 2 runs hit", x)
	}
	x = outcomes(t, f, "explore", Budget{Depth: 0}).Result.Exploration()
	if x == nil || !x.Complete() {
		t.Fatalf("exploration %+v, want a zero depth left to the policy", x)
	}
}

func TestRegistryExploreIsTheRuntimesExploration(t *testing.T) {
	f := parseFixture(t)
	x, err := Default().Explore(context.Background(), &Model{Fresh: f.fresh}, "test::race", policy(t, "explore"), raceRun(t, f), Budget{})
	if err != nil {
		t.Fatalf("explore: %v", err)
	}
	if !x.Complete() || len(x.Outcomes) != 3 {
		t.Fatalf("exploration %+v, want the 3 outcomes", x)
	}
	if _, err = Default().Explore(context.Background(), &Model{Fresh: f.fresh}, "test::race", runtime.DefaultSchedulePolicy, raceRun(t, f), Budget{}); !errors.Is(err, runtime.ErrNotExploring) {
		t.Fatalf("explore under a fixed schedule: %v, want the refusal", err)
	}
}

func TestExploreRefusesWhatItCannotRun(t *testing.T) {
	e := NewExplore()
	run := func(*runtime.Context) (runtime.Outcome, error) { return runtime.Outcome{}, nil }
	if c := e.Covers(nil, Question{Kind: Evaluate}); c.Covered || !errors.Is(c.Refusal, ErrNotAsked) {
		t.Fatalf("evaluate: %+v, want not asked", c)
	}
	if c := e.Covers(nil, Question{Kind: Outcomes, Free: FreeSchedule | FreeInputs, Linearize: run}); c.Covered || !errors.Is(c.Refusal, ErrFreedom) {
		t.Fatalf("free inputs: %+v, want a freedom refusal", c)
	}
	if c := e.Covers(nil, Question{Kind: Outcomes, Free: FreeSchedule}); c.Covered || !errors.Is(c.Refusal, ErrMalformedQuestion) {
		t.Fatalf("no linearize: %+v, want malformed", c)
	}
	if c := e.Covers(nil, Question{Kind: Outcomes, Free: FreeSchedule, Linearize: run, Schedule: runtime.DefaultSchedulePolicy}); c.Covered || !errors.Is(c.Refusal, runtime.ErrNotExploring) {
		t.Fatalf("fixed schedule: %+v, want not exploring", c)
	}
	if c := e.Covers(nil, Question{Kind: Outcomes, Free: FreeSchedule, Linearize: run, Schedule: policy(t, "explore")}); !c.Covered {
		t.Fatalf("exploring: %+v, want covered", c)
	}
}
