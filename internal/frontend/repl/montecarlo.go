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

// A Simulation::MonteCarlo case runs as an action does under %runs: one seeded run per
// row on a fresh subject, then the case concluded once over the sample.

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

// RunMonteCarlo runs the named case count times on fresh subjects, each run seeded from
// seed (the session's when nil), and reports the table, distribution and conclusion.
func (s *Session) RunMonteCarlo(invocation string, count int64, seed *uint64) Verdict {
	defer s.enter()()
	inv, err := splitAnalysisArgs(invocation)
	if err != nil {
		return s.withTrace(unresolvedVerdict(invocation, err.Error()))
	}
	return s.withTrace(s.monteCarloVerdict(inv, count, seed))
}

// monteCarloVerdict reports the runs as an action's table with the concluded case after it;
// a failed run or unsatisfied concluding check fails it, a failed in-run check only counts.
func (s *Session) monteCarloVerdict(inv analysisInvocation, count int64, seed *uint64) Verdict {
	label := "runs " + inv.name
	if inv.argText != "" {
		label += "(" + strings.TrimSpace(inv.argText) + ")"
	}
	sample, answered, err := s.monteCarloSample(inv, count, seed)
	if err != nil {
		return standing(unresolvedVerdict(label, err.Error()), answered)
	}
	var concluded runtime.AnalysisResult
	if sample.unconcluded == nil {
		concluded, err = sample.conclude()
	}
	status, rows := sweepStatus(sample.table)
	if status == VerdictFails && !failedRun(sample.table) {
		status = VerdictHolds
	}
	lines := append(sweepTraces(sample.table), sweepTableLines(sample.table)...)
	lines = append(lines, distributionLines(sample.table)...)
	verdict := Verdict{Subject: label, Status: status, Values: sweepValues(sample.table, rows), Rows: rows}
	if sample.unconcluded != nil {
		if len(sample.completed) > 0 {
			verdict.Status = worsened(verdict.Status, VerdictUnresolved)
		}
		verdict.Lines = append(lines, fmt.Sprintf("%s %s: %s", statusMark(verdict.Status), inv.name, sample.unconcluded.Error()))
		return standing(verdict, answered)
	}
	if err != nil {
		verdict.Status = worsened(verdict.Status, VerdictUnresolved)
		verdict.Lines = append(lines, fmt.Sprintf("%s %s: %s", statusMark(verdict.Status), inv.name, err.Error()))
		reportCaseRunIn(sample.last().Context(), &verdict, concluded)
		return standing(verdict, answered)
	}
	for _, v := range concluded.Verdicts {
		switch v.Status {
		case runtime.VerdictNotSatisfied:
			verdict.Status = worsened(verdict.Status, VerdictFails)
		case runtime.VerdictUndecided:
			verdict.Status = worsened(verdict.Status, VerdictUnresolved)
		}
	}
	verdict.Lines = append(lines, fmt.Sprintf("%s %s over %d run(s)", statusMark(verdict.Status), inv.name, sample.stats.Runs))
	reportCaseRunIn(sample.last().Context(), &verdict, concluded)
	return standing(verdict, answered)
}

// worsened is the table's status once the conclusion adds its own: a failed run
// fails the table whatever the conclusion, and only a holding table is unsettled.
func worsened(status, by VerdictStatus) VerdictStatus {
	if status == VerdictHolds {
		return by
	}
	return status
}

// statusMark is the mark a status's conclusion line opens with.
func statusMark(status VerdictStatus) string {
	switch status {
	case VerdictFails:
		return "✗"
	case VerdictUnresolved:
		return "?"
	}
	return "✓"
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
// completed ones with the statistics of their sample, and the case's conclusion.
type monteCarloRuns struct {
	table runtime.SweepTable
	stats runtime.MonteCarloStatistics
	// completed are the runs of the table's rows that completed, in row order; the
	// case is concluded in the last of them.
	completed []*runtime.MonteCarloRun
	// unconcluded says why the case is not concluded over the table: no run completed,
	// or the sample of the completed ones was refused.
	unconcluded error
}

// last is the completed run the conclusion is read through.
func (m *monteCarloRuns) last() *runtime.MonteCarloRun {
	return m.completed[len(m.completed)-1]
}

// conclude settles every completed run's checks over the sample and concludes the case in
// the last; each row then carries the run's settled checks, the sample's own left to the conclusion.
func (m *monteCarloRuns) conclude() (runtime.AnalysisResult, error) {
	concluded, err := runtime.ConcludeMonteCarlo(m.completed, m.stats)
	byNumber := make(map[int64]*runtime.MonteCarloRun, len(m.completed))
	for _, run := range m.completed {
		byNumber[run.Number] = run
	}
	for k, row := range m.table.Rows {
		if i, ok := runtime.RunNumber(row.Bindings); ok && byNumber[i] != nil {
			m.table.Rows[k].Verdicts = byNumber[i].Verdicts
		}
	}
	return concluded, err
}

// monteCarloSample makes count runs of the invocation, each in its own context on objects
// made from their declarations; the plan is returned beside a refusal made after an engine ran.
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

	// The rows run concurrently; each keeps its run by number for the conclusion. The
	// sweep validates the plan before the first row, so nothing is sized by count here.
	var mu sync.Mutex
	made := make(map[int64]*runtime.MonteCarloRun)
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
		observed.Number = i
		mu.Lock()
		made[i] = observed
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
	for _, row := range table.Rows {
		if i, ok := runtime.RunNumber(row.Bindings); ok && made[i] != nil {
			completed = append(completed, made[i])
		}
	}
	if len(completed) == 0 {
		return &monteCarloRuns{table: table, unconcluded: errors.New("no run completed, so the case is not concluded")}, &answered, nil
	}
	stats, err := runtime.MonteCarloSample(completed)
	if err != nil {
		return &monteCarloRuns{table: table, completed: completed, unconcluded: err}, &answered, nil
	}
	return &monteCarloRuns{table: table, stats: stats, completed: completed}, &answered, nil
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
// of the sample is undecided until the sample is in, when the conclusion settles it.
func decidedVerdicts(verdicts []runtime.AnalysisVerdict) []runtime.AnalysisVerdict {
	decided := make([]runtime.AnalysisVerdict, 0, len(verdicts))
	for _, v := range verdicts {
		if v.Status != runtime.VerdictUndecided {
			decided = append(decided, v)
		}
	}
	return decided
}
