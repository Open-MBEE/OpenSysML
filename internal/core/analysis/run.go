package analysis

import (
	"context"
	"errors"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// RunEngineName is the name of the engine that runs the interpreter once.
const RunEngineName = "run"

// runEngine answers Evaluate questions with one execution of the interpreter
// under the question's scheduling policy, in the surface's own context.
type runEngine struct{}

// NewRun returns the run engine.
func NewRun() Engine { return runEngine{} }

// Name is `run`.
func (runEngine) Name() string { return RunEngineName }

// Describe: one execution, so its evidence is at most observed; a violation it
// finds is the replay of itself.
func (runEngine) Describe() Description {
	return Description{
		Questions: []Kind{Evaluate},
		Bounds:    []string{"steps", "elements"},
		Replays:   true,
		Authority: Observed,
	}
}

// Covers takes any Evaluate question that fixes every choice and says how to perform it.
func (e runEngine) Covers(_ *Model, q Question) Coverage {
	if q.Kind != Evaluate {
		return refused(&NotAskedError{Engine: e.Name(), Kind: q.Kind})
	}
	if q.Free != FreeNothing {
		return refused(&FreedomError{Engine: e.Name(), Free: q.Free})
	}
	if q.Perform == nil {
		return refused(&MalformedQuestionError{Kind: q.Kind, Missing: "a Perform"})
	}
	return covered
}

// Run performs the question once in the model's context: a violation is witnessed, any
// other claim observed, and a failed execution claims nothing, naming the budget it hit.
// The one execution is the unit of work, so a ctx already done is its error unperformed.
func (e runEngine) Run(ctx context.Context, model *Model, q Question, _ Budget) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	rctx, err := model.Context()
	if err != nil {
		return Result{}, err
	}
	started := time.Now()
	answer, err := q.Perform(rctx)
	if err != nil {
		return Result{}, err
	}
	result := Result{
		Question: q,
		Engine:   e.Name(),
		Claim:    answer.Claim,
		Reason:   answer.Reason,
		Values:   answer.Values,
		Bounds:   runBounds(rctx.Budgets(), answer.Err),
		Elapsed:  time.Since(started),
	}
	switch answer.Claim {
	case ClaimNone:
		result.Strength = NotCovered
		if result.Reason == "" && answer.Err != nil {
			result.Reason = answer.Err.Error()
		}
	case ClaimViolated:
		result.Strength = Witnessed
		result.Witness = &Witness{Schedule: q.Schedule}
	default:
		result.Strength = Observed
	}
	return result, nil
}

// runBounds is the step and element limits a run had, and which one the
// execution's failure reached.
func runBounds(limits runtime.Budgets, failure error) Bounds {
	steps := errors.Is(failure, runtime.ErrStepLimitExceeded) ||
		errors.Is(failure, runtime.ErrActionStepLimitExceeded) ||
		errors.Is(failure, runtime.ErrStateEventLimitExceeded) ||
		errors.Is(failure, runtime.ErrDoStepLimitExceeded)
	elements := errors.Is(failure, runtime.ErrElementLimitExceeded)
	return Bounds{
		{Name: "steps", Limit: limits.MaxSteps, Reached: steps},
		{Name: "elements", Limit: limits.MaxElements, Reached: elements},
	}
}
