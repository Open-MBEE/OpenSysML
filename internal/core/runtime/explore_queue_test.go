package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// exploreJobs is the job count the determinism tests run beside one job.
const exploreJobs = 8

// exploreWorkers builds a context per run on a resolver, semantic model and runtime Model per
// job over one index, which is what a plan's worker fleet gives ExploreWith.
type exploreWorkers struct {
	idx     *symbols.Index
	mu      sync.Mutex
	workers map[int]*Model
}

func newExploreWorkers(idx *symbols.Index) *exploreWorkers {
	return &exploreWorkers{idx: idx, workers: make(map[int]*Model)}
}

func (w *exploreWorkers) fresh(job int) (*Context, error) {
	w.mu.Lock()
	m, ok := w.workers[job]
	if !ok {
		resolver := resolve.New(w.idx)
		m = NewModel(semantics.NewModel(resolver), resolver)
		w.workers[job] = m
	}
	w.mu.Unlock()
	return NewContext(m, 10000), nil
}

// renderExploration spells everything an exploration reports, so two are compared whole.
func renderExploration(x *Exploration) string {
	var b strings.Builder
	fmt.Fprintf(&b, "runs %d; budget %+v; hit %v; status %s\n", x.Runs, x.Budget, x.BudgetsHit, x.Status())
	for _, o := range x.Outcomes {
		fmt.Fprintf(&b, "%s | %s | %d linearizations | witness run %d: %s\n",
			o.Outcome, o.Outcome.identity(), o.Linearizations, o.WitnessRun, FormatChoices(o.Witness))
	}
	return b.String()
}

// exploreBoth explores the same runs on one job and on exploreJobs and fails unless the
// two explorations are the same; it returns the one-job exploration.
func exploreBoth(t *testing.T, budget ExploreBudget, fresh func(int) (*Context, error), run func(*Context) (Outcome, error)) *Exploration {
	t.Helper()
	policy, err := ExplorePolicy(budget)
	if err != nil {
		t.Fatal(err)
	}
	sequential, err := ExploreWith(context.Background(), policy, 1, fresh, run)
	if err != nil {
		t.Fatalf("explore on one job: %v", err)
	}
	want := renderExploration(sequential)
	for i := 0; i < 8; i++ {
		parallel, err := ExploreWith(context.Background(), policy, exploreJobs, fresh, run)
		if err != nil {
			t.Fatalf("explore on %d jobs: %v", exploreJobs, err)
		}
		if got := renderExploration(parallel); got != want {
			t.Fatalf("exploration on %d jobs differs from one job's\n got:\n%s\nwant:\n%s", exploreJobs, got, want)
		}
	}
	return sequential
}

// parseIndex parses text into an index of its own.
func parseIndex(t *testing.T, text string) (*symbols.Index, string) {
	t.Helper()
	return indexAt(t, filepath.Join(t.TempDir(), "explore.sysml"), []byte(text))
}

// indexFile parses the model at path into an index of its own.
func indexFile(t *testing.T, path string) (*symbols.Index, string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return indexAt(t, path, data)
}

func indexAt(t *testing.T, path string, data []byte) (*symbols.Index, string) {
	t.Helper()
	p := parser.New(source.New(path, data))
	file := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse %s: %v", path, p.Diagnostics)
	}
	idx := symbols.NewIndex()
	idx.AddDocument(path, file)
	return idx, path
}

// The determinism fixtures: a violation a later, wider subtree reaches faster than the
// first prefix, which the conformance harness cannot hold since it admits no erroring
// outcome; and a slow first prefix beside wide siblings, which is a conformance case.
var (
	laterPrefixViolatesFasterPath = filepath.Join("testdata", "later_prefix_violates_faster.sysml")
	slowFirstWriterPath           = filepath.Join("testdata", "conformance", "action_explore_slow_first_writer.sysml")
)

// actionRun performs the action named in test:: by no object.
func actionRun(t *testing.T, idx *symbols.Index, path, name string) func(*Context) (Outcome, error) {
	t.Helper()
	sym := namedOrFoundSymbol(t, idx, "test::"+name, idx.DocumentRoot(path), ast.DefAction, ast.UsageAction)
	return func(ctx *Context) (Outcome, error) {
		outputs, err := ctx.ExecuteAction(sym)
		if err != nil {
			return Outcome{}, err
		}
		return ctx.ActionOutcome(outputs), nil
	}
}

// Every conformance case with an admissible set explores the same on eight jobs as on one.
func TestExploreWithIsExploreOverTheConformanceCorpus(t *testing.T) {
	conformanceDir := filepath.Join("testdata", "conformance")
	knownFailures := loadKnownFailures(t, conformanceDir)
	entries, err := os.ReadDir(conformanceDir)
	if err != nil {
		t.Fatal(err)
	}
	explored := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".expected.json") {
			continue
		}
		caseName := strings.TrimSuffix(entry.Name(), ".expected.json")
		if knownFailures[caseName] {
			continue
		}
		expectedData, err := os.ReadFile(filepath.Join(conformanceDir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var expected ExpectedOutcome
		if err := json.Unmarshal(expectedData, &expected); err != nil {
			t.Fatalf("%s: %v", entry.Name(), err)
		}
		if len(expected.Outcomes) == 0 {
			continue
		}
		explored++
		t.Run(caseName, func(t *testing.T) {
			t.Parallel()
			sysmlPath := filepath.Join(conformanceDir, caseName+".sysml")
			sysmlData, err := os.ReadFile(sysmlPath)
			if err != nil {
				t.Fatal(err)
			}
			p := parser.New(source.New(sysmlPath, sysmlData))
			file := p.ParseFile()
			idx := symbols.NewIndex()
			if expected.Libraries {
				idx = libs.NewModelIndex()
			}
			idx.AddDocument(sysmlPath, file)
			if expected.Libraries {
				idx.ExpandWildcardImports()
			}
			run := conformanceRun(t, idx, sysmlPath, expected)
			exploreBoth(t, expected.ExploreBudget.budget(), newExploreWorkers(idx).fresh, run)
		})
	}
	if explored == 0 {
		t.Fatal("no conformance case with an admissible set")
	}
}

// failing is the exploration's failing outcome, which the fixture reaches by division by zero.
func failing(t *testing.T, x *Exploration) ExploredOutcome {
	t.Helper()
	var found []ExploredOutcome
	for _, o := range x.Outcomes {
		if o.Outcome.Err != nil {
			found = append(found, o)
		}
	}
	if len(found) != 1 || !errors.Is(found[0].Outcome.Err, ErrDivisionByZero) {
		t.Fatalf("failing outcomes %v, want the one division by zero", found)
	}
	return found[0]
}

// The failure a later, wider subtree reaches in a few steps has its witness at the slow
// first prefix on eight jobs as on one, and the table is the same.
func TestExploreWithKeepsTheLeastWitnessOfALaterFasterViolation(t *testing.T) {
	idx, path := indexFile(t, laterPrefixViolatesFasterPath)
	x := exploreBoth(t, DefaultExploreBudget, newExploreWorkers(idx).fresh, actionRun(t, idx, path, "race"))
	if !x.Complete() || x.Runs != 7 {
		t.Fatalf("status %q after %d runs, want complete after the slow branch and six orders of the quick one", x.Status(), x.Runs)
	}
	failed := failing(t, x)
	if failed.Linearizations != 3 || failed.WitnessRun != 1 {
		t.Fatalf("failure reached by %d linearizations with witness run %d, want 3 and the first run", failed.Linearizations, failed.WitnessRun)
	}
	if len(failed.Witness) != 1 || failed.Witness[0].Taken != 0 {
		t.Fatalf("witness %s, want the first alternative of the decision alone", FormatChoices(failed.Witness))
	}
}

// With the runs cut just past the witness, the speculative runs the wider subtree feeds
// eight jobs are discarded and charged to nothing: the cut is the one job's.
func TestExploreWithCutsRunsJustAboveTheWitness(t *testing.T) {
	idx, path := indexFile(t, laterPrefixViolatesFasterPath)
	x := exploreBoth(t, ExploreBudget{Runs: 2, Depth: 64}, newExploreWorkers(idx).fresh, actionRun(t, idx, path, "race"))
	if x.Complete() || x.Runs != 2 || strings.Join(x.BudgetsHit, ",") != "runs" {
		t.Fatalf("status %q after %d runs hitting %v, want incomplete at the runs cut after 2", x.Status(), x.Runs, x.BudgetsHit)
	}
	if failed := failing(t, x); failed.WitnessRun != 1 {
		t.Fatalf("failure witnessed by run %d, want the first", failed.WitnessRun)
	}
}

// A slow first prefix beside siblings fanning out never costs more than runs + jobs
// executions, and what it reports is the one job's.
func TestExploreWithNeverRunsMoreThanRunsPlusJobs(t *testing.T) {
	idx, path := indexFile(t, slowFirstWriterPath)
	run := actionRun(t, idx, path, "race")
	fresh := newExploreWorkers(idx).fresh
	for _, runs := range []int{1, 2, 3, 5, 6, 1024} {
		budget := ExploreBudget{Runs: runs, Depth: 64}
		x := exploreBoth(t, budget, fresh, run)
		if x.Runs != min(runs, 6) {
			t.Fatalf("runs %d under a budget of %d, want %d", x.Runs, runs, min(runs, 6))
		}
		policy, err := ExplorePolicy(budget)
		if err != nil {
			t.Fatal(err)
		}
		for _, jobs := range []int{1, 2, 3, exploreJobs} {
			for i := 0; i < 4; i++ {
				var executions atomic.Int64
				counted := func(ctx *Context) (Outcome, error) {
					executions.Add(1)
					return run(ctx)
				}
				if _, err := ExploreWith(context.Background(), policy, jobs, fresh, counted); err != nil {
					t.Fatal(err)
				}
				if got, most := executions.Load(), int64(runs+jobs); got > most {
					t.Fatalf("%d executions under a budget of %d runs on %d jobs, want at most %d", got, runs, jobs, most)
				}
			}
		}
	}
}

// A run's context is built by fresh for the job running it, and no job goes beyond jobs.
func TestExploreWithBuildsEachRunOnItsJob(t *testing.T) {
	idx, path := parseIndex(t, threeWritersModel)
	workers := newExploreWorkers(idx)
	var mu sync.Mutex
	seen := make(map[int]int)
	fresh := func(job int) (*Context, error) {
		mu.Lock()
		seen[job]++
		mu.Unlock()
		return workers.fresh(job)
	}
	policy := mustPolicy(t, "explore")
	x, err := ExploreWith(context.Background(), policy, 3, fresh, actionRun(t, idx, path, "race"))
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for job, n := range seen {
		if job < 0 || job >= 3 {
			t.Errorf("fresh built a context for job %d, want 0 to 2", job)
		}
		total += n
	}
	if total < x.Runs || total > x.Runs+3 {
		t.Errorf("fresh built %d contexts for %d runs on 3 jobs, want one per run and at most 3 discarded", total, x.Runs)
	}
}

// A job count past the runs budget puts no more jobs to work than the budget admits runs:
// a vast count starts no fleet of that size.
func TestExploreWithPutsNoMoreJobsToWorkThanRuns(t *testing.T) {
	idx, path := parseIndex(t, threeWritersModel)
	workers := newExploreWorkers(idx)
	var mu sync.Mutex
	seen := make(map[int]bool)
	fresh := func(job int) (*Context, error) {
		mu.Lock()
		seen[job] = true
		mu.Unlock()
		return workers.fresh(job)
	}
	policy, err := ExplorePolicy(ExploreBudget{Runs: 2, Depth: 64})
	if err != nil {
		t.Fatal(err)
	}
	x, err := ExploreWith(context.Background(), policy, 1<<30, fresh, actionRun(t, idx, path, "race"))
	if err != nil {
		t.Fatal(err)
	}
	if x.Runs != 2 || strings.Join(x.BudgetsHit, ",") != "runs" {
		t.Fatalf("%d runs hitting %v, want 2 at the runs cut", x.Runs, x.BudgetsHit)
	}
	for job := range seen {
		if job < 0 || job >= 2 {
			t.Errorf("fresh built a context for job %d, want 0 or 1", job)
		}
	}
}

// A caller that goes away ends the exploration with its error once the runs in flight are done.
func TestExploreWithStopsWhenTheCallerGoesAway(t *testing.T) {
	idx, path := parseIndex(t, threeWritersModel)
	stop, cancel := context.WithCancel(context.Background())
	var started atomic.Int64
	run := actionRun(t, idx, path, "race")
	x, err := ExploreWith(stop, mustPolicy(t, "explore"), 2, newExploreWorkers(idx).fresh, func(ctx *Context) (Outcome, error) {
		if started.Add(1) == 1 {
			cancel()
		}
		return run(ctx)
	})
	if !errors.Is(err, context.Canceled) || x != nil {
		t.Fatalf("exploration %v, %v; want nil and the caller's cancellation", x, err)
	}
}

// A context fresh cannot build fails the exploration at the run that asked for it.
func TestExploreWithFailsWhenFreshFails(t *testing.T) {
	idx, path := parseIndex(t, threeWritersModel)
	workers := newExploreWorkers(idx)
	broken := errors.New("no context")
	var built atomic.Int64
	fresh := func(job int) (*Context, error) {
		if built.Add(1) == 3 {
			return nil, broken
		}
		return workers.fresh(job)
	}
	x, err := ExploreWith(context.Background(), mustPolicy(t, "explore"), exploreJobs, fresh, actionRun(t, idx, path, "race"))
	if !errors.Is(err, broken) || x != nil {
		t.Fatalf("exploration %v, %v; want nil and fresh's error", x, err)
	}
}

// A run that does not reach the choice points its prefix planned is a divergence, reported
// at the run's position in plan order whatever job it ran on.
func TestExploreWithReportsADivergedReplayAtItsRun(t *testing.T) {
	idx, path := parseIndex(t, threeWritersModel)
	run := actionRun(t, idx, path, "race")
	var runs atomic.Int64
	diverging := func(ctx *Context) (Outcome, error) {
		if runs.Add(1) > 1 {
			return Outcome{ctx: ctx}, nil
		}
		return run(ctx)
	}
	for _, jobs := range []int{1, exploreJobs} {
		runs.Store(0)
		x, err := ExploreWith(context.Background(), mustPolicy(t, "explore"), jobs, newExploreWorkers(idx).fresh, diverging)
		if !errors.Is(err, ErrExplorationDiverged) || x != nil {
			t.Fatalf("%d jobs: exploration %v, %v; want nil and a divergence", jobs, x, err)
		}
		if !strings.Contains(err.Error(), "run 2:") {
			t.Fatalf("%d jobs: %v; want the divergence at run 2", jobs, err)
		}
	}
}
