package sweep

import (
	"errors"
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// A Monte Carlo plan is a sweep over nothing but the seed: one ordinary run of a
// behavior per row, each drawing its modeled randomness from a seed of its own
// derived from the one given.

// ErrSweepRuns reports a Monte Carlo that runs nothing or states a range too.
var ErrSweepRuns = errors.New("invalid run request")

// RunParam names the column a Monte Carlo table numbers its runs in.
const RunParam = "run"

// MonteCarloPlan is the plan of runs runs seeded from seed: one row per run and
// no ranges.
func MonteCarloPlan(runs int64, seed uint64) SweepPlan {
	return SweepPlan{Runs: runs, Seed: seed, MonteCarlo: true}
}

// SeedlessMonteCarloPlan is the plan of runs runs seeded from nothing: a fixed draw
// policy resolves every RandomFunctions call, and a weighted decision is an unseeded draw.
func SeedlessMonteCarloPlan(runs int64) SweepPlan {
	return SweepPlan{Runs: runs, MonteCarlo: true, Seedless: true}
}

// runBindings is one row per run, binding RunParam to the run's number from 1;
// the run's seed is RunSeed of the plan's, which the run derives when it starts.
func runBindings(ctx *runtime.Context, plan SweepPlan, limit int64) ([][]SweepBinding, error) {
	if len(plan.Ranges) > 0 {
		return nil, fmt.Errorf("%w: a Monte Carlo runs one behavior as declared; it states no range", ErrSweepRuns)
	}
	if plan.Sampled {
		return nil, fmt.Errorf("%w: a Monte Carlo draws its seeds from the seed given; it samples no range", ErrSweepRuns)
	}
	if plan.Runs < 1 {
		return nil, fmt.Errorf("%w: %d run(s) runs nothing; ask for at least one", ErrSweepRuns, plan.Runs)
	}
	if plan.Runs > limit {
		return nil, sweepBudgetError(ctx, limit)
	}
	rows := make([][]SweepBinding, 0, plan.Runs)
	for i := int64(1); i <= plan.Runs; i++ {
		number := runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: i}}
		rows = append(rows, []SweepBinding{{Param: RunParam, Value: number}})
	}
	return rows, nil
}

// RunNumber is the number of the run a Monte Carlo row is, from its bindings.
func RunNumber(bindings []SweepBinding) (int64, bool) {
	for _, b := range bindings {
		if b.Param == RunParam && b.Value.Kind == runtime.ValConst && b.Value.Const.Kind == semantics.ValInt {
			return b.Value.Const.Int64()
		}
	}
	return 0, false
}
