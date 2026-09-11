package runtime

import (
	"context"
	"fmt"
	"slices"
	"sync"
)

// ExploreWith explores as Explore does with up to jobs runs going concurrently, fresh building
// each run's context for the job making it; the result does not depend on jobs.
//
// The prefixes form a work queue in plan order, the order one job visits them, and the runs
// budget is a cut in that order. A run is committed once every prefix before it has completed;
// one started earlier is speculative, and at most jobs of those are discarded in all.
func ExploreWith(stop context.Context, policy SchedulePolicy, jobs int, fresh func(job int) (*Context, error), run func(*Context) (Outcome, error)) (*Exploration, error) {
	budget, ok := policy.Exploration()
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotExploring, policy)
	}
	if jobs < 1 {
		jobs = 1
	}
	q := &exploreQueue{
		stop:    stop,
		policy:  policy,
		budget:  budget,
		jobs:    jobs,
		result:  &Exploration{Budget: budget},
		reached: make(map[string]int),
	}
	q.wake = sync.NewCond(&q.mu)
	q.insert(0, []*explorePrefix{{}})
	var wg sync.WaitGroup
	for job := 0; job < jobs; job++ {
		wg.Add(1)
		go func(job int) {
			defer wg.Done()
			q.work(job, fresh, run)
		}(job)
	}
	q.watch(&wg)
	if q.err != nil {
		return nil, q.err
	}
	if q.beyond {
		q.result.BudgetsHit = append(q.result.BudgetsHit, "runs")
	}
	if q.depthHit {
		q.result.BudgetsHit = append(q.result.BudgetsHit, "depth")
	}
	sortOutcomes(q.result.Outcomes)
	return q.result, nil
}

// prefixState is where a prefix on the queue stands.
type prefixState int

const (
	prefixQueued  prefixState = iota
	prefixRunning             // a job is running it
	prefixDone                // run, its outcome waiting to be folded in plan order
	prefixDropped             // past the runs cut; a run of it is discarded
)

// explorePrefix is one prefix on the queue: its position in plan order and, once run, the result.
type explorePrefix struct {
	prefix   []exploreSlot
	index    int
	state    prefixState
	replay   *exploreRun // nil when fresh failed
	outcome  Outcome
	identity string
	err      error // fresh failed, or the run diverged
}

// exploreQueue coordinates the jobs over the prefixes discovered within the runs cut.
type exploreQueue struct {
	mu   sync.Mutex
	wake *sync.Cond

	stop   context.Context
	policy SchedulePolicy
	budget ExploreBudget
	jobs   int

	order     []*explorePrefix // in plan order; never longer than the runs budget
	frontier  int              // order[:frontier] are folded into the result
	running   int              // runs in flight, dropped ones included
	pending   []*explorePrefix // started, neither folded nor dropped
	discarded int              // speculative runs dropped past the cut
	beyond    bool             // a prefix was discovered past the runs cut
	done      bool             // every prefix within the cut is folded, or the exploration failed

	result   *Exploration
	reached  map[string]int
	depthHit bool
	err      error
}

// work is one job: it runs prefixes as the queue hands them out until the queue is over.
func (q *exploreQueue) work(job int, fresh func(int) (*Context, error), run func(*Context) (Outcome, error)) {
	for {
		p := q.next()
		if p == nil {
			return
		}
		ctx, err := fresh(job)
		if err != nil {
			q.finish(p, nil, Outcome{}, err)
			continue
		}
		replay := &exploreRun{prefix: p.prefix, depth: q.budget.Depth}
		ctx.beginExploration(q.policy, replay)
		outcome, runErr := run(ctx)
		if runErr != nil {
			outcome = Outcome{Err: runErr, ctx: ctx}
		}
		q.finish(p, replay, outcome, nil)
	}
}

// watch waits for the jobs to finish, waking the idle ones when the caller goes away.
func (q *exploreQueue) watch(wg *sync.WaitGroup) {
	finished := make(chan struct{})
	go func() {
		wg.Wait()
		close(finished)
	}()
	select {
	case <-finished:
	case <-q.stop.Done():
		q.mu.Lock()
		q.wake.Broadcast()
		q.mu.Unlock()
		<-finished
	}
}

// next hands a job the least prefix it may start, waiting until there is one; nil once the
// queue is over, which is after every run in flight has completed.
func (q *exploreQueue) next() *explorePrefix {
	q.mu.Lock()
	defer q.mu.Unlock()
	for {
		if q.done {
			return nil
		}
		if q.frontier == len(q.order) {
			if q.running == 0 {
				q.done = true
				q.wake.Broadcast()
				return nil
			}
			q.wake.Wait()
			continue
		}
		if err := q.stop.Err(); err != nil {
			q.fail(err)
			return nil
		}
		if p := q.startable(); p != nil {
			p.state = prefixRunning
			q.running++
			q.pending = append(q.pending, p)
			return p
		}
		q.wake.Wait()
	}
}

// startable is the least queued prefix a job may start: the one at the frontier, or a
// speculative one while the discards it could add stay within jobs.
func (q *exploreQueue) startable() *explorePrefix {
	for i := q.frontier; i < len(q.order); i++ {
		p := q.order[i]
		if p.state != prefixQueued {
			continue
		}
		if i == q.frontier || q.discarded+q.speculative() < q.jobs {
			return p
		}
		return nil
	}
	return nil
}

// speculative counts the runs started past the frontier, whose position may still move.
func (q *exploreQueue) speculative() int {
	n := 0
	for _, p := range q.pending {
		if p.index > q.frontier {
			n++
		}
	}
	return n
}

// finish records a run's result, queues the prefixes it leaves right after it and folds
// what is now committed; a run dropped meanwhile is discarded.
func (q *exploreQueue) finish(p *explorePrefix, replay *exploreRun, outcome Outcome, err error) {
	if err == nil {
		if err = replay.followed(); err == nil {
			p.identity = outcome.identity()
		}
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	defer q.wake.Broadcast()
	q.running--
	if p.state == prefixDropped || q.done {
		return
	}
	p.state, p.replay, p.outcome, p.err = prefixDone, replay, outcome, err
	if err == nil {
		children := replay.unexplored()
		next := make([]*explorePrefix, len(children))
		for i, prefix := range children {
			next[i] = &explorePrefix{prefix: prefix}
		}
		q.insert(p.index+1, next)
	}
	q.fold()
}

// insert queues prefixes at position at, moving what follows back; whatever moves past the
// runs cut is dropped, a started run among it discarded.
func (q *exploreQueue) insert(at int, prefixes []*explorePrefix) {
	if len(prefixes) == 0 {
		return
	}
	q.order = slices.Insert(q.order, at, prefixes...)
	if len(q.order) > q.budget.Runs {
		q.beyond = true
		for _, p := range q.order[q.budget.Runs:] {
			if p.state == prefixRunning || p.state == prefixDone {
				q.discarded++
				q.forget(p)
			}
			p.state = prefixDropped
		}
		q.order = q.order[:q.budget.Runs]
	}
	for i := at; i < len(q.order); i++ {
		q.order[i].index = i
	}
}

// fold takes every done prefix at the frontier into the result in plan order; the first that
// failed fails the exploration at its run.
func (q *exploreQueue) fold() {
	for q.frontier < len(q.order) && q.order[q.frontier].state == prefixDone {
		p := q.order[q.frontier]
		q.forget(p)
		q.frontier++
		q.result.Runs = q.frontier
		if p.err != nil {
			if p.replay != nil {
				p.err = fmt.Errorf("%w: run %d: %v", ErrExplorationDiverged, q.frontier, p.err)
			}
			q.fail(p.err)
			return
		}
		q.depthHit = q.depthHit || p.replay.depthHit
		if i, seen := q.reached[p.identity]; seen {
			if !p.replay.duplicate {
				q.result.Outcomes[i].Linearizations++
			}
			continue
		}
		q.reached[p.identity] = len(q.result.Outcomes)
		q.result.Outcomes = append(q.result.Outcomes, ExploredOutcome{
			Outcome:        p.outcome,
			Linearizations: 1,
			Witness:        p.replay.choices(),
			WitnessRun:     q.frontier,
		})
	}
}

// forget takes a prefix off the pending list.
func (q *exploreQueue) forget(p *explorePrefix) {
	if i := slices.Index(q.pending, p); i >= 0 {
		q.pending = slices.Delete(q.pending, i, i+1)
	}
}

// fail ends the exploration with err: the first failure in plan order, or the caller's.
func (q *exploreQueue) fail(err error) {
	if q.err == nil {
		q.err = err
	}
	q.done = true
	q.wake.Broadcast()
}
