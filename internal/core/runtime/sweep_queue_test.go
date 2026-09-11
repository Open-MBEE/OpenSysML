package runtime

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// sweepJobs is the job count the sweep determinism tests run beside one job.
const sweepJobs = 8

// parallelSweepModel is what the parallel sweep tests run over: a calc whose work grows
// steeply with its input, so rows of a descending range finish out of plan order, and a
// case whose body writes a feature of its subject.
const parallelSweepModel = `
	package test {
		private import ScalarValues::*;
		calc def Fib {
			in n : Integer;
			return : Integer = if n < 2 ? n else Fib(n - 1) + Fib(n - 2);
		}
		part def Ship { attribute cost : Real = 5.0; }
		part ship : Ship;
		analysis def Bump {
			subject s : Ship;
			in tax : Real;
			action raise { assign s.cost := s.cost + tax; }
			out total : Real = s.cost;
		}
	}
`

// sweepJobsFixture indexes the model with the library and returns a builder of run-owned
// contexts, each over a resolver and semantic model of its own as a worker's is, with the
// test package.
func sweepJobsFixture(t *testing.T) (func(job int) (*Context, error), *symbols.Scope) {
	t.Helper()
	idx := libs.NewModelIndex()
	idx.AddDocument("<test>", parseAndBuild(t, parallelSweepModel))
	idx.ExpandWildcardImports()
	fresh := func(int) (*Context, error) {
		resolver := resolve.New(idx)
		return NewContext(NewModel(semantics.NewModel(resolver), resolver), 50_000_000), nil
	}
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	return fresh, pkg.Scope
}

// firstOf is the context the first row runs in, built as any other job's is.
func firstOf(t *testing.T, fresh func(job int) (*Context, error)) *Context {
	t.Helper()
	ctx, err := fresh(0)
	if err != nil {
		t.Fatal(err)
	}
	return ctx
}

// arrivals records the plan index of each row as it finishes, and the context it ran in;
// rows are told apart by their inputs.
type arrivals struct {
	mu       sync.Mutex
	index    map[string]int
	order    []int
	contexts []*Context
}

func arrivalsOf(planned SweepTable) *arrivals {
	index := make(map[string]int, len(planned.Rows))
	for i, inputs := range inputsOf(planned) {
		index[inputs] = i
	}
	return &arrivals{index: index}
}

// recording wraps run to note each row's arrival.
func (a *arrivals) recording(run SweepRun) SweepRun {
	return func(ctx *Context, bindings []SweepBinding) (SweepRunResult, error) {
		result, err := run(ctx, bindings)
		a.mu.Lock()
		defer a.mu.Unlock()
		a.order = append(a.order, a.index[inputsOf(SweepTable{Rows: []SweepRow{{Bindings: bindings}}})[0]])
		a.contexts = append(a.contexts, ctx)
		return result, err
	}
}

// Rows of a descending range over Fib arrive out of plan order on eight jobs, and the table
// is the one job's all the same: plan order, the same values, no row read through another's
// context.
func TestRunSweepWithTablesRowsInPlanOrderWhateverTheirArrival(t *testing.T) {
	fresh, scope := sweepJobsFixture(t)
	first := firstOf(t, fresh)
	sym := calcNamed(t, scope, "Fib")
	plan, err := first.ResolveSweepPlan(sym, SweepPlan{Ranges: []SweepRange{steppedRange("n", intOf(22), intOf(12), intOf(-1))}}, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	run := sweepCalcRun(sym, scope)
	sequential, err := RunSweepWith(context.Background(), first, "test::Fib", plan, 0, 1, fresh, run)
	if err != nil {
		t.Fatalf("sweep on one job: %v", err)
	}
	want := tableText(sequential)
	if !strings.HasPrefix(want, "n=22 result -> 17711\n") || !strings.HasSuffix(want, "n=12 result -> 144") {
		t.Fatalf("one job's table:\n%s", want)
	}
	inPlanOrder := make([]int, len(sequential.Rows))
	for i := range inPlanOrder {
		inPlanOrder[i] = i
	}
	outOfOrder := false
	for attempt := 0; attempt < 8; attempt++ {
		a := arrivalsOf(sequential)
		parallel, err := RunSweepWith(context.Background(), firstOf(t, fresh), "test::Fib", plan, 0, sweepJobs, fresh, a.recording(run))
		if err != nil {
			t.Fatalf("sweep on %d jobs: %v", sweepJobs, err)
		}
		if got := tableText(parallel); got != want {
			t.Fatalf("table on %d jobs differs from one job's\n got:\n%s\nwant:\n%s", sweepJobs, got, want)
		}
		for i, row := range parallel.Rows {
			if row.Context == nil || row.Context != a.contexts[slices.Index(a.order, i)] {
				t.Fatalf("row %d carries a context other than the one it ran in", i)
			}
			for j := range parallel.Rows {
				if j != i && parallel.Rows[j].Context == row.Context {
					t.Fatalf("rows %d and %d share a context", i, j)
				}
			}
		}
		if !slices.Equal(a.order, inPlanOrder) {
			outOfOrder = true
		}
	}
	if !outOfOrder {
		t.Fatalf("on %d jobs, the rows arrived in plan order on every attempt: the fixture proves nothing", sweepJobs)
	}
}

// bumpRun runs Bump on a Ship instantiated in the row's context and reports its total.
func bumpRun(t *testing.T, scope *symbols.Scope) SweepRun {
	t.Helper()
	sym, ok := scope.LookupLocal("Bump")
	if !ok {
		t.Fatal("Bump not indexed")
	}
	ship := calcNamed(t, scope, "ship")
	return func(ctx *Context, bindings []SweepBinding) (SweepRunResult, error) {
		subject, err := ctx.Instantiate(ship)
		if err != nil {
			return SweepRunResult{}, err
		}
		named := make(map[string]Value, len(bindings))
		for _, b := range bindings {
			named[b.Param] = b.Value
		}
		result, err := ctx.RunAnalysis(sym, AnalysisArgs{Named: named, Subject: subject}, scope, nil)
		return SweepRunResult{Outputs: result.Outputs, Subject: result.Subject}, err
	}
}

// A case writing a feature of its subject sees the declaration's value on every row: no row
// observes another's write, on one job or on eight, and the row's subject carries the write
// in the row's context alone.
func TestRunSweepWithKeepsEachRowsWritesToItself(t *testing.T) {
	fresh, scope := sweepJobsFixture(t)
	first := firstOf(t, fresh)
	sym, _ := scope.LookupLocal("Bump")
	plan, err := first.ResolveSweepPlan(sym, SweepPlan{Ranges: []SweepRange{steppedRange("tax", realOf(1), realOf(4), realOf(1))}}, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		"tax=1.0 total -> 6.0",
		"tax=2.0 total -> 7.0",
		"tax=3.0 total -> 8.0",
		"tax=4.0 total -> 9.0",
	}, "\n")
	for _, jobs := range []int{1, sweepJobs} {
		table, err := RunSweepWith(context.Background(), firstOf(t, fresh), "test::Bump", plan, 0, jobs, fresh, bumpRun(t, scope))
		if err != nil {
			t.Fatalf("sweep on %d jobs: %v", jobs, err)
		}
		if got := tableText(table); got != want {
			t.Fatalf("on %d jobs the rows read each other's writes:\n%s\nwant\n%s", jobs, got, want)
		}
		for i, row := range table.Rows {
			cost, ok := row.Subject.FeatureValues["cost"]
			if !ok || !cost.Written || FormatValue(cost.Value) != FormatValue(row.Outputs[0].Value) {
				t.Fatalf("row %d's subject in its context: cost %+v, want the row's total written", i, cost)
			}
			if err := row.Context.Pristine(row.Subject); err == nil {
				t.Fatalf("row %d's subject reads as pristine after its body wrote it", i)
			}
		}
	}
}

// A deadline met mid-sweep ends the sweep with context.DeadlineExceeded and no table: no row
// starts once the deadline is visible, the rows in flight finish and are discarded — an
// absence, never a partial answer — as one job reports when it meets the deadline between
// two rows. The rows the jobs take first wait out the deadline, so on any job count the
// rows started are exactly the jobs' first ones.
func TestRunSweepWithStopsAtTheDeadline(t *testing.T) {
	fresh, scope := sweepJobsFixture(t)
	first := firstOf(t, fresh)
	sym := calcNamed(t, scope, "Fib")
	plan, err := first.ResolveSweepPlan(sym, SweepPlan{Ranges: []SweepRange{rangeOf("n", intOf(1), intOf(12))}}, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	double := sweepCalcRun(sym, scope)
	for _, jobs := range []int{1, sweepJobs} {
		stop, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		var started, finished, late atomic.Int64
		waiting := func(ctx *Context, bindings []SweepBinding) (SweepRunResult, error) {
			started.Add(1)
			if stop.Err() != nil {
				late.Add(1)
			}
			if bindings[0].Value.Const.Int <= int64(sweepJobs) {
				<-stop.Done()
			}
			result, err := double(ctx, bindings)
			finished.Add(1)
			return result, err
		}
		table, err := RunSweepWith(stop, firstOf(t, fresh), "test::Fib", plan, 0, jobs, fresh, waiting)
		cancel()
		if !errors.Is(err, context.DeadlineExceeded) || len(table.Rows) != 0 || table.Target != "" {
			t.Fatalf("on %d jobs: table of %d rows and %v, want no table and context.DeadlineExceeded", jobs, len(table.Rows), err)
		}
		if late.Load() != 0 || started.Load() != int64(jobs) || finished.Load() != int64(jobs) {
			t.Fatalf("on %d jobs: %d rows started (%d after the deadline), %d finished; want one per job, none after the deadline, every one finished",
				jobs, started.Load(), late.Load(), finished.Load())
		}
	}

	// A deadline already met starts no row at all.
	stop, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	ran := 0
	_, err = RunSweepWith(stop, first, "test::Fib", plan, 0, sweepJobs, fresh, func(ctx *Context, bindings []SweepBinding) (SweepRunResult, error) {
		ran++
		return double(ctx, bindings)
	})
	if !errors.Is(err, context.DeadlineExceeded) || ran != 0 {
		t.Fatalf("under a met deadline: %v after %d rows, want context.DeadlineExceeded after none", err, ran)
	}
}

// A job whose context cannot be built ends the sweep with that error, and the first row's
// context is the one handed in.
func TestRunSweepWithFailsWhenAJobHasNoContext(t *testing.T) {
	fresh, scope := sweepJobsFixture(t)
	first := firstOf(t, fresh)
	sym := calcNamed(t, scope, "Fib")
	plan, err := first.ResolveSweepPlan(sym, SweepPlan{Ranges: []SweepRange{rangeOf("n", intOf(1), intOf(4))}}, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	var firstRow *Context
	run := func(ctx *Context, bindings []SweepBinding) (SweepRunResult, error) {
		if FormatValue(bindings[0].Value) == "1" {
			firstRow = ctx
		}
		return sweepCalcRun(sym, scope)(ctx, bindings)
	}
	table, err := RunSweepWith(context.Background(), first, "test::Fib", plan, 0, 1, fresh, run)
	if err != nil || firstRow != first {
		t.Fatalf("on one job: %v; first row in the context handed in: %v", err, firstRow == first)
	}
	for i := 1; i < len(table.Rows); i++ {
		if table.Rows[i].Context == first {
			t.Fatalf("row %d ran in the first row's context", i)
		}
	}
	broken := errors.New("no worker")
	_, err = RunSweepWith(context.Background(), first, "test::Fib", plan, 0, 1, func(int) (*Context, error) { return nil, broken }, run)
	if !errors.Is(err, broken) {
		t.Fatalf("with no context to build: %v, want the builder's error", err)
	}
}
