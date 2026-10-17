package explore

import (
	"context"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
)

// ExploreQueued runs an exploration through a queue of the given width and
// reports, per run in plan order, whether the queue still holds the run's
// replay, context or outputs after folding it.
func ExploreQueued(stop context.Context, policy runtime.SchedulePolicy, budget runtime.ExploreBudget, jobs int, fresh func(int) (*runtime.Context, error), run func(*runtime.Context) (runtime.Outcome, error)) (*Exploration, []bool, error) {
	q := newExploreQueue(policy, budget, jobs)
	x, err := q.explore(stop, fresh, run)
	held := make([]bool, len(q.order))
	for i, p := range q.order {
		held[i] = p.replay != nil || p.outcome.Context() != nil || p.outcome.Outputs != nil
	}
	return x, held, err
}
