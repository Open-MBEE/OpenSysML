package analysis

import (
	"context"
	"fmt"
	"time"
)

// SweepEngineName is the name of the engine that runs a domain row by row.
const SweepEngineName = "sweep"

// sweepEngine answers Sweep questions with one run per row of the plan, the
// rows in the order the plan draws them.
type sweepEngine struct{}

// NewSweep returns the sweep engine.
func NewSweep() Engine { return sweepEngine{} }

// Name is `sweep`.
func (sweepEngine) Name() string { return SweepEngineName }

// Describe: concrete runs over a stated domain, so every row is observed.
func (sweepEngine) Describe() Description {
	return Description{
		Questions: []Kind{Sweep},
		Bounds:    []string{"runs"},
		Authority: Observed,
	}
}

// Covers takes a Sweep question that fixes every choice and says how one row runs.
func (e sweepEngine) Covers(_ *Model, q Question) Coverage {
	if q.Kind != Sweep {
		return refused(&NotAskedError{Engine: e.Name(), Kind: q.Kind})
	}
	if q.Free != FreeNothing {
		return refused(&FreedomError{Engine: e.Name(), Free: q.Free})
	}
	if q.Sweep == nil || q.Sweep.Row == nil {
		return refused(&MalformedQuestionError{Kind: q.Kind, Missing: "a Sweep with a Row"})
	}
	return covered
}

// Run tables the plan in the surface's context within the budget's runs (else the context's),
// one evaluation per row (a failed row carrying its error); a plan of more rows than that,
// or a caller that went away, is the error. Row takes no context, so the rows are the surface's
// closures over its own and a model holding none is the typed fault NoRuntimeError.
func (e sweepEngine) Run(ctx context.Context, model *Model, q Question, budget Budget) (Result, error) {
	if !model.holds() {
		return Result{}, &NoRuntimeError{Engine: e.Name()}
	}
	rctx, err := model.running(e.Name(), budget)
	if err != nil {
		return Result{}, err
	}
	runs := int64(budget.Runs)
	if runs <= 0 {
		runs = rctx.SweepRunBudget()
	}
	started := time.Now()
	table, err := rctx.RunSweep(ctx, q.Subject, q.Sweep.Plan, runs, q.Sweep.Row)
	if err != nil {
		return Result{}, err
	}
	values := make([]Evaluation, len(table.Rows))
	for i := range table.Rows {
		row := &table.Rows[i]
		values[i] = Evaluation{Name: fmt.Sprintf("row %d", i+1), Row: row, Err: row.Err}
	}
	return Result{
		Question: q,
		Engine:   e.Name(),
		Claim:    ClaimTable,
		Strength: Observed,
		Bounds:   Bounds{{Name: "runs", Limit: runs}},
		Values:   values,
		Elapsed:  time.Since(started),
	}, nil
}
