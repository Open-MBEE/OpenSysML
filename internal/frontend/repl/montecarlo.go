package repl

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// A Simulation::MonteCarlo analysis case is run many times as an action is: one
// run per row, each on a fresh subject seeded from the seed given, tabling what
// each observed; then the case is concluded once over the sample, its statistics
// bound and its outputs and checks evaluated over them.

// monteCarloObservable names the column the runs' observations are tabled in.
const monteCarloObservable = "observed"

// namesAnalysisCase reports whether %runs' tail names an analysis case rather than
// an action, so the prompt tells the two apart before either is run.
func (s *Session) namesAnalysisCase(tail string) bool {
	inv, err := splitAnalysisArgs(tail)
	if err != nil || inv.name == "" {
		return false
	}
	sym, _, err := s.lookupSymbolOfKinds(inv.name,
		symbols.SymbolAnalysisCaseDef, symbols.SymbolAnalysisCaseUsage)
	return err == nil && runtime.RequireAnalysis(sym) == nil
}

// RunMonteCarlo runs the analysis case the invocation names count times, each run
// drawing its modeled randomness from a seed of its own derived from seed — the
// session's when seed is nil, and none when it has none, as a fixed draw policy
// allows — on a fresh object of the subject named, and reports the table of what
// the runs observed, the distribution, then the case concluded over the sample.
func (s *Session) RunMonteCarlo(invocation string, count int64, seed *uint64) Verdict {
	defer s.enter()()
	inv, err := splitAnalysisArgs(invocation)
	if err != nil {
		return s.withTrace(unresolvedVerdict(invocation, err.Error()))
	}
	return s.withTrace(s.monteCarloVerdict(inv, count, seed))
}

// monteCarloVerdict makes the runs and reports them as a Monte Carlo of an action
// reports its table, the concluded case after the distribution. A run that failed
// fails the table; a check the conclusion left unsatisfied fails it, an undecided
// one leaves it unresolved. A check that did not hold in a run is counted, as
// outOfSpec, not held against the table.
func (s *Session) monteCarloVerdict(inv analysisInvocation, count int64, seed *uint64) Verdict {
	label := "runs " + inv.name
	if inv.argText != "" {
		label += "(" + strings.TrimSpace(inv.argText) + ")"
	}
	sample, answered, err := s.monteCarloSample(inv, count, seed)
	if err != nil {
		return standing(unresolvedVerdict(label, err.Error()), answered)
	}
	status, rows := sweepStatus(sample.table)
	if status == VerdictFails && !failedRun(sample.table) {
		status = VerdictHolds
	}
	lines := append(sweepTraces(sample.table), sweepTableLines(sample.table)...)
	lines = append(lines, distributionLines(sample.table)...)
	verdict := Verdict{Subject: label, Status: status, Values: sweepValues(sample.table, rows), Rows: rows}

	concluded, err := sample.conclude()
	if err != nil {
		verdict.Status = VerdictUnresolved
		verdict.Lines = append(lines, fmt.Sprintf("? %s: %s", inv.name, err.Error()))
		reportCaseRunIn(sample.last.Context(), &verdict, concluded)
		return standing(verdict, answered)
	}
	for _, v := range concluded.Verdicts {
		switch v.Status {
		case runtime.VerdictNotSatisfied:
			if verdict.Status == VerdictHolds {
				verdict.Status = VerdictFails
			}
		case runtime.VerdictUndecided:
			verdict.Status = VerdictUnresolved
		}
	}
	mark := "✓"
	switch verdict.Status {
	case VerdictFails:
		mark = "✗"
	case VerdictUnresolved:
		mark = "?"
	}
	verdict.Lines = append(lines, fmt.Sprintf("%s %s over %d run(s)", mark, inv.name, sample.stats.Runs))
	reportCaseRunIn(sample.last.Context(), &verdict, concluded)
	return standing(verdict, answered)
}

// failedRun reports whether any run of the table failed.
func failedRun(table runtime.SweepTable) bool {
	for _, row := range table.Rows {
		if row.Err != nil {
			return true
		}
	}
	return false
}

// monteCarloRuns are the runs of a Monte Carlo case: the table of them, the
// statistics of the completed ones, and the run the case is concluded in.
type monteCarloRuns struct {
	table runtime.SweepTable
	stats runtime.MonteCarloStatistics
	// last is the last completed run, whose context the conclusion is read through.
	last *runtime.MonteCarloRun
}

// conclude binds the statistics of the sample in the last completed run and
// evaluates the case's outputs and checks over them.
func (m *monteCarloRuns) conclude() (runtime.AnalysisResult, error) {
	return m.last.Conclude(m.stats)
}

// monteCarloSample resolves the invocation once at the prompt and makes count runs of
// it, each in a context of its own on objects made there from their declarations, and
// takes the statistics of what the completed runs observed; the plan that answered is
// returned beside a refusal made after an engine ran, nil for one made before.
func (s *Session) monteCarloSample(inv analysisInvocation, count int64, seed *uint64) (*monteCarloRuns, *analysis.Plan, error) {
	doc := s.ws.Document(docName)
	if doc == nil || doc.Scope == nil {
		return nil, nil, errors.New("no declarations loaded")
	}
	if _, replaying := s.drivenSchedule().Replay(); replaying {
		return nil, nil, ErrRunsReplay
	}
	sym, fqn, err := s.analysisSymbol(inv)
	if err != nil {
		return nil, nil, err
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, nil, err
	}
	if err := ctx.RequireMonteCarloCase(sym); err != nil {
		return nil, nil, err
	}
	parsed, err := parseAnalysisArgs(inv.argText)
	if err != nil {
		return nil, nil, err
	}
	args, err := s.sweptArgs(ctx, s.promptScope(), parsed)
	if err != nil {
		return nil, nil, err
	}
	var subject, owner freshRef
	if inv.object != "" {
		if subject, err = s.sweptObject(ctx, inv.object); err != nil {
			return nil, nil, err
		}
	}
	if isNestedCase(sym) {
		if owner, err = s.sweptOwner(ctx, fqn); err != nil {
			return nil, nil, err
		}
	}
	if subject, err = declaredFresh(subject); err != nil {
		return nil, nil, err
	}
	if owner, err = declaredFresh(owner); err != nil {
		return nil, nil, err
	}
	for id, ref := range args.objects {
		if args.objects[id], err = declaredFresh(ref); err != nil {
			return nil, nil, err
		}
	}
	runScope := declaringScope(sym, doc.Scope)

	if seed == nil && s.modelSeed.set {
		session := s.modelSeed.value
		seed = &session
	}
	plan := runtime.SeedlessMonteCarloPlan(count)
	if seed != nil {
		plan = runtime.MonteCarloPlan(count, *seed)
	} else if !s.draws.Fixed() {
		return nil, nil, fmt.Errorf("%w: the runs draw at random; name the seed they draw from, or fix the draws as %%draws min|max|average", runtime.ErrSweepRuns)
	}

	// The rows run concurrently; each keeps its run by number for the conclusion.
	var mu sync.Mutex
	made := make([]*runtime.MonteCarloRun, count)
	run := func(rt *runtime.Context, bindings []runtime.SweepBinding) (runtime.SweepRunResult, error) {
		i, ok := runtime.RunNumber(bindings)
		if !ok {
			return runtime.SweepRunResult{}, fmt.Errorf("%w: the row numbers no run", runtime.ErrSweepRuns)
		}
		if seed != nil {
			rt.SetModelSeed(runtime.RunSeed(*seed, i))
		} else {
			rt.ClearModelSeed()
		}
		row, err := s.rowObjects(rt, args.objects, nil)
		if err != nil {
			return runtime.SweepRunResult{}, err
		}
		positional, named, err := args.in(row)
		if err != nil {
			return runtime.SweepRunResult{}, err
		}
		caseArgs := runtime.AnalysisArgs{Positional: positional, Named: named}
		if caseArgs.Subject, err = row.object(subject); err != nil {
			return runtime.SweepRunResult{}, err
		}
		self, err := row.object(owner)
		if err != nil {
			return runtime.SweepRunResult{}, err
		}
		observed, err := rt.ObserveMonteCarlo(sym, caseArgs, runScope, self)
		if err != nil {
			return runtime.SweepRunResult{}, err
		}
		mu.Lock()
		made[i-1] = observed
		mu.Unlock()
		return runtime.SweepRunResult{
			Outputs:  []runtime.CalcOutputValue{{Name: monteCarloObservable, Value: observed.Observed}},
			Verdicts: decidedVerdicts(observed.Verdicts),
			Subject:  observed.Subject,
		}, nil
	}

	model := s.freshModel()
	s.state.Unlock()
	answered, err := s.sweep(fqn, model, plan, run, s.draws, s.clockStep)
	s.state.Lock()
	if err != nil {
		return nil, &answered, err
	}
	table := answered.Result.Table()
	completed := make([]*runtime.MonteCarloRun, 0, len(made))
	for _, observed := range made {
		if observed != nil {
			completed = append(completed, observed)
		}
	}
	stats, err := runtime.MonteCarloSample(completed)
	if err != nil {
		return nil, &answered, err
	}
	last := completed[len(completed)-1]
	return &monteCarloRuns{table: table, stats: stats, last: last}, &answered, nil
}

// declaredFresh makes the reference one each run makes from its declaration, as its
// draws are the run's; an object no declaration makes, named by `#id`, has no fresh copy.
func declaredFresh(ref freshRef) (freshRef, error) {
	if ref.held == 0 {
		return ref, nil
	}
	if ref.sym == nil {
		return freshRef{}, fmt.Errorf("%w: %s is no declaration's object; each run of a Monte Carlo makes its objects from their declarations, so name the object by the declaration it is reached from", runtime.ErrSweepRuns, ref.name)
	}
	ref.imaged = false
	return ref, nil
}

// decidedVerdicts are the checks a run decided on its own; one reading a statistic
// of the sample is undecided until the conclusion, which reports it.
func decidedVerdicts(verdicts []runtime.AnalysisVerdict) []runtime.AnalysisVerdict {
	decided := make([]runtime.AnalysisVerdict, 0, len(verdicts))
	for _, v := range verdicts {
		if v.Status != runtime.VerdictUndecided {
			decided = append(decided, v)
		}
	}
	return decided
}
