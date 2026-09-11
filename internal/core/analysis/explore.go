package analysis

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// ExploreEngineName is the name of the engine that runs every linearization.
const ExploreEngineName = "explore"

// exploreEngine answers Outcomes questions by running the interpreter once per linearization
// within a runs and depth budget, each in a context of its own on one of the plan's workers.
type exploreEngine struct{}

// NewExplore returns the explore engine.
func NewExplore() Engine { return exploreEngine{} }

// Name is `explore`.
func (exploreEngine) Name() string { return ExploreEngineName }

// Describe: every schedule for the inputs as written, so a complete
// exploration proves its outcome set; each outcome's witness is a run it made.
func (exploreEngine) Describe() Description {
	return Description{
		Questions: []Kind{Outcomes},
		Bounds:    []string{"runs", "depth"},
		Replays:   true,
		Authority: Proved,
	}
}

// Covers takes an Outcomes question over concrete inputs whose policy explores;
// it runs concrete values only, so free inputs are refused.
func (e exploreEngine) Covers(_ *Model, q Question) Coverage {
	if q.Kind != Outcomes {
		return refused(&NotAskedError{Engine: e.Name(), Kind: q.Kind})
	}
	if q.Free.Has(FreeInputs) {
		return refused(&FreedomError{Engine: e.Name(), Free: FreeInputs})
	}
	if q.Linearize == nil {
		return refused(&MalformedQuestionError{Kind: q.Kind, Missing: "a Linearize"})
	}
	if _, ok := q.Schedule.Exploration(); !ok {
		return refused(fmt.Errorf("%w: %s", runtime.ErrNotExploring, q.Schedule))
	}
	return covered
}

// Run explores under the budget's runs and depth (else the policy's own) on the budget's jobs,
// each run in a context of its own under the budget on the worker of the job making it:
// complete is proved, incomplete observed naming the budget hit. A model that builds no
// context of a run's own is the typed fault NoRuntimeError.
func (e exploreEngine) Run(ctx context.Context, model *Model, q Question, budget Budget) (Result, error) {
	policy, err := explorePolicy(q.Schedule, budget)
	if err != nil {
		return Result{}, err
	}
	if !model.builds() {
		return Result{}, &NoRuntimeError{Engine: e.Name()}
	}
	fresh := func(job int) (*runtime.Context, error) { return model.NewContextOn(job, budget) }
	started := time.Now()
	x, err := runtime.ExploreWith(ctx, policy, budget.Jobs, fresh, q.Linearize)
	if err != nil {
		return Result{}, err
	}
	result := Result{
		Question: q,
		Engine:   e.Name(),
		Claim:    ClaimOutcomes,
		Strength: Observed,
		Bounds: Bounds{
			{Name: "runs", Limit: int64(x.Budget.Runs), Reached: slices.Contains(x.BudgetsHit, "runs")},
			{Name: "depth", Limit: int64(x.Budget.Depth), Reached: slices.Contains(x.BudgetsHit, "depth")},
		},
		Values:  []Evaluation{{Name: q.Subject, Explored: x}},
		Elapsed: time.Since(started),
	}
	if x.Complete() {
		result.Strength = Proved
	}
	return result, nil
}

// explorePolicy is the question's policy with the budget's runs and depth where
// the budget names them.
func explorePolicy(policy runtime.SchedulePolicy, budget Budget) (runtime.SchedulePolicy, error) {
	limits, ok := policy.Exploration()
	if !ok {
		return policy, fmt.Errorf("%w: %s", runtime.ErrNotExploring, policy)
	}
	if budget.Runs > 0 {
		limits.Runs = budget.Runs
	}
	if budget.Depth > 0 {
		limits.Depth = budget.Depth
	}
	return runtime.ExplorePolicy(limits)
}
