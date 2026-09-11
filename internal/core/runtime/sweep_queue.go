package runtime

import (
	"context"
	"sync"
)

// RunSweepWith makes one run per row of the plan with up to jobs going concurrently, each
// row in a context of its own — first for the first row, the one the rows are enumerated in,
// and fresh building every other for the job making it — and reports the table in plan order
// whatever order the rows finish in; the table does not depend on jobs. A run that failed
// is that row's typed error; the table is completed either way. A plan of more rows than
// runs allows (first's SweepRunBudget when runs is zero) is refused before any run is made.
// A caller that goes away starts no further row and takes the table with it: the rows in
// flight finish and are discarded, and the caller's error is the sweep's, as it is when one
// job meets it between two rows.
func RunSweepWith(stop context.Context, first *Context, target string, plan SweepPlan, runs int64, jobs int, fresh func(job int) (*Context, error), run SweepRun) (SweepTable, error) {
	rows, err := first.sweepBindings(plan, first.sweepRunLimit(runs))
	if err != nil {
		return SweepTable{}, err
	}
	table := NewSweepTable(target, plan)
	table.Rows = make([]SweepRow, len(rows))
	q := &sweepQueue{stop: stop, rows: rows, table: table.Rows, fresh: fresh, run: run}
	var wg sync.WaitGroup
	i, ok := q.take()
	wg.Add(1)
	go func() {
		defer wg.Done()
		q.work(0, first, i, ok)
	}()
	for job := 1; job < min(jobs, len(rows)); job++ {
		wg.Add(1)
		go func(job int) {
			defer wg.Done()
			i, ok := q.take()
			q.work(job, nil, i, ok)
		}(job)
	}
	wg.Wait()
	if q.err != nil {
		return SweepTable{}, q.err
	}
	return table, nil
}

// sweepQueue hands the rows of one sweep to the jobs in plan order and tables each where
// the plan puts it.
type sweepQueue struct {
	mu    sync.Mutex
	stop  context.Context
	rows  [][]SweepBinding
	table []SweepRow
	fresh func(job int) (*Context, error)
	run   SweepRun
	next  int   // rows[:next] are started
	err   error // the caller went away, or a job's context could not be built
}

// work is one job: it runs row i if ok, then rows as the queue hands them out until the
// queue is over, each in a context of its own, ctx being the one already built for the first.
func (q *sweepQueue) work(job int, ctx *Context, i int, ok bool) {
	for ; ok; i, ok = q.take() {
		if ctx == nil {
			built, err := q.fresh(job)
			if err != nil {
				q.fail(err)
				return
			}
			ctx = built
		}
		q.table[i] = runSweepRow(ctx, q.rows[i], q.run)
		ctx = nil
	}
}

// take hands a job the next row in plan order, or reports that the queue is over: every row
// is started, the caller went away, or the sweep failed.
func (q *sweepQueue) take() (int, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.err != nil || q.next == len(q.rows) {
		return 0, false
	}
	if err := q.stop.Err(); err != nil {
		q.err = err
		return 0, false
	}
	q.next++
	return q.next - 1, true
}

// fail ends the sweep with err, the first met.
func (q *sweepQueue) fail(err error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.err == nil {
		q.err = err
	}
}
