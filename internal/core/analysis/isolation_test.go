package analysis

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// workers records every worker a fixture built for a model, and how many times it was asked.
type workers struct {
	mu    sync.Mutex
	built []*Worker
	asked int
}

// recording is the fixture's building model with every worker it makes recorded.
func (f *fixture) recording(w *workers) *Model {
	return &Model{
		Semantics: func() (*runtime.Model, error) {
			w.mu.Lock()
			w.asked++
			w.mu.Unlock()
			return f.semantics()
		},
		Fresh: func(worker *Worker) (*runtime.Context, error) {
			w.mu.Lock()
			w.built = append(w.built, worker)
			w.mu.Unlock()
			return f.fresh(worker)
		},
	}
}

// distinct is how many distinct resolvers and semantic models the runs were made on.
func (w *workers) distinct() (resolvers, models int) {
	seenR := make(map[*resolve.Resolver]bool)
	seenM := make(map[*semantics.Model]bool)
	for _, worker := range w.built {
		seenR[worker.Model.Resolver()] = true
		seenM[worker.Model.Semantics()] = true
	}
	return len(seenR), len(seenM)
}

// Two plans on one model, on two goroutines, each build a worker of their own and make every
// run on it; the model handed in is never written. Run under -race, this is the isolation test.
func TestPlansOnOneModelHaveWorkersOfTheirOwn(t *testing.T) {
	f := parseFixture(t)
	w := &workers{}
	model := f.recording(w)
	q := Question{Kind: Outcomes, Subject: "test::race", Schedule: policy(t, "explore"), Free: FreeSchedule, Linearize: raceRun(t, f)}
	plans := make([]Plan, 2)
	results := make([]Result, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			plan, err := Default().Answer(context.Background(), model, q, Budget{})
			plans[i], results[i], errs[i] = plan, plan.Result, err
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("plan %d: %v", i, err)
		}
		x := results[i].Exploration()
		if x == nil || !x.Complete() || len(x.Outcomes) != 3 {
			t.Fatalf("plan %d explored %+v, want the 3 outcomes", i, x)
		}
		if results[i].Workers != 1 {
			t.Fatalf("plan %d built %d workers, want one", i, results[i].Workers)
		}
		if plans[i].Workers != 1 || plans[i].Warming != results[i].Warming {
			t.Fatalf("plan %d reports %d workers warming %s, want the result's one warming %s", i, plans[i].Workers, plans[i].Warming, results[i].Warming)
		}
	}
	resolvers, models := w.distinct()
	if w.asked != 2 || resolvers != 2 || models != 2 || len(w.built) != 12 {
		t.Fatalf("%d workers built, %d resolvers and %d semantic models over %d runs; want 2, 2, 2 and 12", w.asked, resolvers, models, len(w.built))
	}
	if model.workers != nil {
		t.Fatal("the caller's model was given a worker, want the plan's copy to hold it")
	}
}

// A run-owned context takes Steps and Memory from the budget at construction, as the
// evaluation-step and element limits alone; a zero field, and every other bound, is what
// the surface built.
func TestRunOwnedContextTakesTheBudgetAtConstruction(t *testing.T) {
	f := parseFixture(t)
	surface := f.context(t).Budgets()
	var seen runtime.Budgets
	q := Question{Kind: Evaluate, Subject: "test::Double", Schedule: runtime.DefaultSchedulePolicy,
		Perform: func(rctx *runtime.Context) (Answer, error) {
			seen = rctx.Budgets()
			return ValuesAnswer(nil, nil), nil
		},
	}
	want := surface
	want.MaxSteps, want.MaxElements = 7, 9
	result := answered(t, Default(), f.building(), q, Budget{Steps: 7, Memory: 9}).Result
	if seen != want {
		t.Fatalf("run's limits %+v, want %+v: the budget's steps and memory, the rest the surface's", seen, want)
	}
	if steps, ok := result.Bounds.Limit("steps"); !ok || steps != 7 {
		t.Fatalf("bounds %s, want steps=7", result.Bounds)
	}
	if result.Workers != 1 {
		t.Fatalf("workers %d, want the one the plan built", result.Workers)
	}

	result = answered(t, Default(), f.building(), q, Budget{}).Result
	if seen != surface {
		t.Fatalf("run's limits under a zero budget %+v, want the surface's %+v", seen, surface)
	}
	if steps, ok := result.Bounds.Limit("steps"); !ok || steps != fixtureSteps {
		t.Fatalf("bounds %s, want steps=%d", result.Bounds, fixtureSteps)
	}
}

// Every run of an exploration is a context of its own under the budget.
func TestExploreBuildsEachRunUnderTheBudget(t *testing.T) {
	f := parseFixture(t)
	surface := f.context(t).Budgets()
	var mu sync.Mutex
	var seen []runtime.Budgets
	run := raceRun(t, f)
	q := Question{Kind: Outcomes, Subject: "test::race", Schedule: policy(t, "explore"), Free: FreeSchedule,
		Linearize: func(rctx *runtime.Context) (runtime.Outcome, error) {
			mu.Lock()
			seen = append(seen, rctx.Budgets())
			mu.Unlock()
			return run(rctx)
		},
	}
	want := surface
	want.MaxSteps, want.MaxElements = 7, 9
	result := answered(t, Default(), f.building(), q, Budget{Steps: 7, Memory: 9}).Result
	if len(seen) != 6 || result.Workers != 1 {
		t.Fatalf("%d runs on %d workers, want 6 on one", len(seen), result.Workers)
	}
	for i, limits := range seen {
		if limits != want {
			t.Fatalf("run %d's limits %+v, want %+v", i, limits, want)
		}
	}
	seen = nil
	answered(t, Default(), f.building(), q, Budget{})
	for i, limits := range seen {
		if limits != surface {
			t.Fatalf("run %d's limits under a zero budget %+v, want the surface's %+v", i, limits, surface)
		}
	}
}

// The surface's own context is not a run's: its limits stand whatever the budget says,
// and no worker is built to run in it. Sweep rows never run there: each is a context of
// its own under the budget on the plan's worker, and solve builds none.
func TestTheSurfacesContextKeepsItsLimits(t *testing.T) {
	f := parseFixture(t)
	ctx := f.context(t)
	before := ctx.Budgets()
	q := Question{Kind: Evaluate, Subject: "test::Double", Schedule: runtime.DefaultSchedulePolicy,
		Perform: func(rctx *runtime.Context) (Answer, error) {
			if rctx != ctx {
				t.Fatal("the run was not in the surface's context")
			}
			return ValuesAnswer(nil, nil), nil
		},
	}
	result := answered(t, Default(), Held(ctx), q, Budget{Steps: 7, Memory: 9}).Result
	if ctx.Budgets() != before {
		t.Fatalf("the surface's limits %+v became %+v", before, ctx.Budgets())
	}
	if result.Workers != 0 || result.Warming != 0 {
		t.Fatalf("workers %d warming %s, want none for a run in the surface's context", result.Workers, result.Warming)
	}

	var mu sync.Mutex
	var rows []runtime.Budgets
	double := doubleRow(t, f)
	sweep := Question{Kind: Sweep, Subject: "test::Double", Schedule: ctx.Schedule(), Sweep: &SweepAsk{Plan: doublePlan(t, f, ctx),
		Row: func(rctx *runtime.Context, bindings []runtime.SweepBinding) (runtime.SweepRunResult, error) {
			if rctx == ctx {
				t.Error("a sweep row ran in the surface's context")
			}
			mu.Lock()
			rows = append(rows, rctx.Budgets())
			mu.Unlock()
			return double(rctx, bindings)
		}}}
	if _, err := Default().Answer(context.Background(), Held(ctx), sweep, Budget{Steps: 7, Memory: 9}); !errors.Is(err, ErrNoRuntime) {
		t.Fatalf("sweep on a held context alone: %v, want ErrNoRuntime", err)
	}
	result = answered(t, Default(), f.building(), sweep, Budget{Steps: 7, Memory: 9, Jobs: 2}).Result
	want := before
	want.MaxSteps, want.MaxElements = 7, 9
	if ctx.Budgets() != before || result.Workers < 1 || result.Workers > 2 || len(rows) != 3 {
		t.Fatalf("after a sweep, limits %+v, %d workers and %d rows; want %+v, one or two and 3", ctx.Budgets(), result.Workers, len(rows), before)
	}
	for i, limits := range rows {
		if limits != want {
			t.Fatalf("row %d's limits %+v, want %+v", i, limits, want)
		}
	}

	requireSolver(t)
	if result = satisfiable(t, intQuery("C", 5, 2)).Result; result.Workers != 0 || result.Warming != 0 {
		t.Fatalf("solve built %d workers warming %s, want none", result.Workers, result.Warming)
	}
}

// An engine needing a context the model does not build faults with a typed error.
func TestARunNeedsAModelThatBuildsItsContext(t *testing.T) {
	f := parseFixture(t)
	ctx := f.context(t)
	evaluate := Question{Kind: Evaluate, Subject: "test::Double", Schedule: runtime.DefaultSchedulePolicy,
		Perform: func(*runtime.Context) (Answer, error) { return ValuesAnswer(nil, nil), nil }}
	if _, err := Default().Answer(context.Background(), nil, evaluate, Budget{}); !errors.Is(err, ErrNoRuntime) {
		t.Fatalf("run on no model: %v, want ErrNoRuntime", err)
	}
	outcomes := Question{Kind: Outcomes, Subject: "test::race", Schedule: policy(t, "explore"), Free: FreeSchedule, Linearize: raceRun(t, f)}
	if _, err := Default().Answer(context.Background(), Held(ctx), outcomes, Budget{}); !errors.Is(err, ErrNoRuntime) {
		t.Fatalf("explore on a held context alone: %v, want ErrNoRuntime", err)
	}
	sweep := Question{Kind: Sweep, Subject: "test::Double", Schedule: ctx.Schedule(), Sweep: &SweepAsk{Plan: doublePlan(t, f, ctx), Row: doubleRow(t, f)}}
	if _, err := Default().Answer(context.Background(), nil, sweep, Budget{}); !errors.Is(err, ErrNoRuntime) {
		t.Fatalf("sweep on no model: %v, want ErrNoRuntime", err)
	}
	var typed *NoRuntimeError
	if _, err := (&Model{}).Worker(); !errors.Is(err, ErrNoRuntime) || errors.As(err, &typed) {
		t.Fatalf("worker of a model building none: %v, want the bare ErrNoRuntime", err)
	}
}
