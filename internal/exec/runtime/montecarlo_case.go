package runtime

import (
	"errors"
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// A Simulation::MonteCarlo case is an analysis of repeated runs, each on a fresh subject
// reading `observed`; runs, mean, deviation and outOfSpec are bound over the sample.

// MonteCarloCaseFQN names the library analysis of repeated runs.
const MonteCarloCaseFQN = "Simulation::MonteCarlo"

// The features Simulation::MonteCarlo declares, by the name each is declared under;
// the outputs are exported for the comparison of a case's statistics with a tool's.
const (
	monteCarloObserved        = "observed"
	MonteCarloRunsOutput      = "runs"
	MonteCarloMeanOutput      = "mean"
	MonteCarloDeviationOutput = "deviation"
	MonteCarloOutOfSpecOutput = "outOfSpec"
)

// ErrNotMonteCarlo reports repeating a case that is no Simulation::MonteCarlo analysis.
var ErrNotMonteCarlo = errors.New("not a Simulation::MonteCarlo analysis")

// ErrMonteCarloObserved reports a run whose `observed` the statistics cannot be taken of.
var ErrMonteCarloObserved = errors.New("invalid observation")

// IsMonteCarloCase reports whether sym declares an analysis case specializing
// Simulation::MonteCarlo, matched by identity so a case merely named alike is none.
func (ctx *Context) IsMonteCarloCase(sym *symbols.Symbol) bool {
	return sym != nil && RequireAnalysis(sym) == nil && ctx.specializesLibraryType(sym, MonteCarloCaseFQN)
}

// RequireMonteCarloCase reports ErrNotMonteCarlo for a symbol that is not an
// analysis case specializing Simulation::MonteCarlo, describing what it is instead.
func (ctx *Context) RequireMonteCarloCase(sym *symbols.Symbol) error {
	if err := ctx.RequireAnalysisCase(sym); err != nil {
		return err
	}
	if !ctx.IsMonteCarloCase(sym) {
		return fmt.Errorf("%w: %s specializes no %s, so its runs have no statistics to bind",
			ErrNotMonteCarlo, ctx.qualifiedSymbolName(sym), MonteCarloCaseFQN)
	}
	return nil
}

// MonteCarloRun is one run of a Simulation::MonteCarlo case: its steps performed
// and `observed` read, its statistics unbound until ConcludeMonteCarlo binds the sample's.
type MonteCarloRun struct {
	ctx   *Context
	sym   *symbols.Symbol
	scope *symbols.Scope
	run   *calcRun
	log   *evaluationLog
	// results are the returns ending the case's steps, evaluated by ConcludeMonteCarlo.
	results []lower.Statement
	// left marks the checks this run alone left undecided, for the sample to decide.
	left []bool

	// Case is the qualified name of the case that ran; Subject the object it ran on.
	Case    string
	Subject *Instance

	// Number is the run's number in its Monte Carlo, the row it was drawn as; a
	// sample names a run by it, whatever earlier runs failed.
	Number int64

	// Observed is the value the run's `observed` came to, null when the run left it unbound.
	Observed Value

	// Verdicts are the case's checks over this run: on its own until ConcludeMonteCarlo
	// settles them over the sample, which keeps the checks that are the sample's alone.
	Verdicts []AnalysisVerdict
}

// Context is the context the run was made in, which its values are read through.
func (r *MonteCarloRun) Context() *Context { return r.ctx }

// ObserveMonteCarlo makes one run of a Simulation::MonteCarlo case as RunAnalysis would,
// reading `observed` and the checks in place of the outputs, which ConcludeMonteCarlo evaluates.
func (ctx *Context) ObserveMonteCarlo(sym *symbols.Symbol, args AnalysisArgs, scope *symbols.Scope, self *Instance) (*MonteCarloRun, error) {
	defer ctx.beginRun()()

	if err := ctx.RequireMonteCarloCase(sym); err != nil {
		return nil, err
	}
	if err := ctx.checkCalcTyping(sym); err != nil {
		return nil, err
	}
	shape, err := ctx.calcShapeOf(sym)
	if err != nil {
		return nil, err
	}
	asUsage := args.Subject == nil && len(args.Positional) == 0 && len(args.Named) == 0 && isCalcUsageSymbol(sym)
	var calcArgs calcArgs
	if !asUsage {
		if calcArgs, err = shape.analysisArgs(args); err != nil {
			return nil, err
		}
	}
	reader := NewEvalContextIn(ctx, scope, self)
	log := ctx.beginEvaluationLog(sym)
	defer ctx.endEvaluationLog(log)

	var run *calcRun
	if asUsage {
		run, err = ctx.calcUsageObservation(reader, sym)
	} else {
		run, err = ctx.analysisRun(shape, reader, calcArgs, true)
	}
	if err != nil {
		return nil, err
	}
	observed, err := ctx.monteCarloObserved(run)
	if err != nil {
		return nil, err
	}
	// The run's environment outlives the invocation: the conclusion reads it once the sample is in.
	run = run.detached()
	_, results := shape.observationSteps()
	verdicts := ctx.analysisVerdicts(run, sym, scope)
	left := make([]bool, len(verdicts))
	for i, v := range verdicts {
		left[i] = v.Status == VerdictUndecided
	}
	return &MonteCarloRun{
		ctx: ctx, sym: sym, scope: scope, run: run, log: log, results: results, left: left,
		Case:     shape.Name,
		Subject:  run.boundSubject(ctx),
		Observed: observed,
		Verdicts: verdicts,
	}, nil
}

// calcUsageObservation runs a case usage as calcUsageRun does, its results deferred.
func (ctx *Context) calcUsageObservation(reader *EvalContext, sym *symbols.Symbol) (*calcRun, error) {
	start, run, err := ctx.beginCalcUsage(reader, sym)
	if err != nil || run != nil {
		return run, err
	}
	start.deferResults = true
	return ctx.finishCalcUsage(start)
}

// monteCarloObserved reads the run's `observed`: the feature the library declares,
// under whichever name the case redefines it.
func (ctx *Context) monteCarloObserved(run *calcRun) (Value, error) {
	name, ok := ctx.monteCarloMember(run.shape, monteCarloObserved)
	if !ok {
		return Value{}, fmt.Errorf("%w: %s declares no %s::%s to observe",
			ErrMonteCarloObserved, run.shape.Label, MonteCarloCaseFQN, monteCarloObserved)
	}
	if value, ok := run.env.lookup(name); ok {
		return value, nil
	}
	return Value{Kind: ValNull}, nil
}

// monteCarloMember is the name the case's run binds the library feature under.
func (ctx *Context) monteCarloMember(shape *calcShape, feature string) (string, bool) {
	if ctx.model.resolver == nil || ctx.model.resolver.Index() == nil {
		return "", false
	}
	for _, sym := range ctx.model.resolver.Index().LookupQualified(MonteCarloCaseFQN + "::" + feature) {
		if name, ok := shape.memberName(ctx, sym); ok {
			return name, true
		}
	}
	return "", false
}

// MonteCarloStatistics are the statistics of a sample of runs, as
// Simulation::MonteCarlo declares them.
type MonteCarloStatistics struct {
	// Runs is the number of runs observed.
	Runs int64
	// Mean is the arithmetic mean of the observations.
	Mean float64
	// Deviation is their sample standard deviation, which under two runs there is none of.
	Deviation float64
	// OutOfSpec is the number of runs a check of the runs did not hold in, counted by
	// ConcludeMonteCarlo once the runs' checks are settled; MonteCarloSample leaves it 0.
	OutOfSpec int64
	// Unit is the unit quantity observations were taken in, which Mean and Deviation
	// are expressed in; nil when the observations were bare numbers.
	Unit *Unit
}

// statistic is Mean or Deviation as a value: a Real, or a quantity in the sample's unit.
func (s MonteCarloStatistics) statistic(x float64) Value {
	if s.Unit == nil {
		return constValue(drawnReal(x))
	}
	return NewQuantityValue(&Quantity{Num: drawnReal(x), Unit: s.Unit.Clone()})
}

// MonteCarloSample is the statistics of runs, each of which observed a number or a
// quantity, the quantities expressed in the first run's unit; one observing none,
// no number, or a quantity of another dimension refuses the sample, named by its Number.
func MonteCarloSample(runs []*MonteCarloRun) (MonteCarloStatistics, error) {
	if len(runs) == 0 {
		return MonteCarloStatistics{}, fmt.Errorf("%w: no run observed anything", ErrMonteCarloObserved)
	}
	numbers := make([]semantics.Value, 0, len(runs))
	var unit *Unit
	first := runs[0]
	for i, run := range runs {
		observed := soleElement(run.Observed)
		number, ok := MagnitudeValue(observed)
		if !ok {
			return MonteCarloStatistics{}, fmt.Errorf("%w: run %d of %s observed %s, not a number",
				ErrMonteCarloObserved, run.Number, run.Case, describeObserved(observed))
		}
		q := observed.Quantity()
		switch {
		case i == 0 && q != nil:
			u := q.Unit.Clone()
			unit = &u
		case (q == nil) != (unit == nil):
			return MonteCarloStatistics{}, fmt.Errorf("%w: run %d of %s observed %s where run %d observed %s; a sample is of numbers or of quantities, not both",
				ErrMonteCarloObserved, run.Number, run.Case, describeObserved(observed), first.Number, describeObserved(soleElement(first.Observed)))
		case q != nil:
			magnitude, err := q.ConvertTo(*unit)
			if err != nil {
				return MonteCarloStatistics{}, fmt.Errorf("%w: run %d of %s: %w", ErrMonteCarloObserved, run.Number, run.Case, err)
			}
			number = drawnReal(magnitude)
		}
		numbers = append(numbers, number)
	}
	d := Distribute(numbers)
	return MonteCarloStatistics{Runs: int64(d.Count), Mean: d.Mean, Deviation: d.Deviation, Unit: unit}, nil
}

// describeObserved words an observation as the sample's refusal names it.
func describeObserved(value Value) string {
	if value.Kind == ValNull {
		return "no value"
	}
	return FormatValue(value)
}

// ConcludeMonteCarlo settles every run's checks over the sample's runs, mean and deviation,
// counts outOfSpec over them, and concludes the case in the last run with the sample's checks.
func ConcludeMonteCarlo(runs []*MonteCarloRun, stats MonteCarloStatistics) (AnalysisResult, error) {
	if len(runs) == 0 {
		return AnalysisResult{}, fmt.Errorf("%w: no run to conclude", ErrMonteCarloObserved)
	}
	for _, r := range runs {
		r.settle(stats)
	}
	sample := sampleChecks(runs)
	for _, r := range runs {
		r.Verdicts = verdictsOutside(r.Verdicts, sample)
		if r.failsOwn() {
			stats.OutOfSpec++
		}
	}
	return runs[len(runs)-1].conclude(stats, sample)
}

// settle binds the statistics of the observations in the run and decides over them the
// checks the run left to the sample; a statistic that cannot be bound leaves them undecided.
func (r *MonteCarloRun) settle(stats MonteCarloStatistics) {
	ctx := r.ctx
	defer ctx.beginRun()()
	r.log.enclosing = ctx.evaluations
	ctx.evaluations = r.log
	defer ctx.endEvaluationLog(r.log)

	var settled []AnalysisVerdict
	if err := r.bindObservationStatistics(stats); err != nil {
		settled = ctx.undecidedVerdicts(r.sym, r.scope, err)
	} else {
		settled = ctx.analysisVerdicts(r.run, r.sym, r.scope)
	}
	if len(settled) != len(r.Verdicts) {
		return
	}
	for i, v := range settled {
		if r.left[i] {
			r.Verdicts[i] = v
		}
	}
}

// bindObservationStatistics binds runs, mean and deviation (empty under two runs).
func (r *MonteCarloRun) bindObservationStatistics(stats MonteCarloStatistics) error {
	deviation := Value{Kind: ValNull}
	if stats.Runs >= 2 {
		deviation = stats.statistic(stats.Deviation)
	}
	bound := []struct {
		feature string
		value   Value
	}{
		{MonteCarloRunsOutput, constValue(drawnInt(stats.Runs))},
		{MonteCarloMeanOutput, stats.statistic(stats.Mean)},
		{MonteCarloDeviationOutput, deviation},
	}
	for _, b := range bound {
		if err := r.bindStatistic(b.feature, b.value); err != nil {
			return err
		}
	}
	return nil
}

// sampleChecks marks the checks that are the sample's: every run left them and settled
// them alike. A check some run decided, or the runs settled apart, is a check of the runs.
func sampleChecks(runs []*MonteCarloRun) []bool {
	last := runs[len(runs)-1]
	sample := make([]bool, len(last.Verdicts))
	for i := range sample {
		sample[i] = true
		for _, r := range runs {
			if len(r.Verdicts) != len(sample) || !r.left[i] || !sameVerdict(r.Verdicts[i], last.Verdicts[i]) {
				sample[i] = false
				break
			}
		}
	}
	return sample
}

func sameVerdict(a, b AnalysisVerdict) bool {
	return a.Status == b.Status && a.Detail == b.Detail
}

// failsOwn reports a run some check of the runs did not hold in.
func (r *MonteCarloRun) failsOwn() bool {
	for _, v := range r.Verdicts {
		if v.Status == VerdictNotSatisfied {
			return true
		}
	}
	return false
}

// verdictsOutside keeps the verdicts of the checks not marked, in order.
func verdictsOutside(verdicts []AnalysisVerdict, marked []bool) []AnalysisVerdict {
	kept := make([]AnalysisVerdict, 0, len(verdicts))
	for i, v := range verdicts {
		if i >= len(marked) || !marked[i] {
			kept = append(kept, v)
		}
	}
	return kept
}

// verdictsWithin keeps the verdicts of the checks marked, in order.
func verdictsWithin(verdicts []AnalysisVerdict, marked []bool) []AnalysisVerdict {
	kept := make([]AnalysisVerdict, 0, len(verdicts))
	for i, v := range verdicts {
		if i < len(marked) && marked[i] {
			kept = append(kept, v)
		}
	}
	return kept
}

// conclude binds outOfSpec over the settled run, then reports the case's outputs and
// the verdicts of the sample's checks, judged over the results and all four statistics.
func (r *MonteCarloRun) conclude(stats MonteCarloStatistics, sample []bool) (AnalysisResult, error) {
	ctx := r.ctx
	defer ctx.beginRun()()
	r.log.enclosing = ctx.evaluations
	ctx.evaluations = r.log
	defer ctx.endEvaluationLog(r.log)

	result := AnalysisResult{Case: r.Case, Subject: r.Subject}
	if err := r.bindObservationStatistics(stats); err != nil {
		return result, err
	}
	if err := r.bindStatistic(MonteCarloOutOfSpecOutput, constValue(drawnInt(stats.OutOfSpec))); err != nil {
		return result, err
	}
	if err := r.returnResults(); err != nil {
		result.Verdicts = verdictsWithin(ctx.undecidedVerdicts(r.sym, r.scope, err), sample)
		result.Evaluations = r.log.evaluations(Value{}, false)
		return result, err
	}
	outputs, err := r.run.outputValues(ctx)
	result.Outputs = outputs
	if err != nil {
		result.Verdicts = verdictsWithin(ctx.undecidedVerdicts(r.sym, r.scope, err), sample)
		result.Evaluations = r.log.evaluations(Value{}, false)
		return result, err
	}
	result.Verdicts = verdictsWithin(ctx.analysisVerdicts(r.run, r.sym, r.scope), sample)
	result.Evaluations = r.log.evaluations(r.run.caseResult(r.run.bindingsFrame(ctx).vars))
	return result, nil
}

// bindStatistic gives the library's output feature its value, under the name the
// case binds it, checked against the declaration as a binding's value is. The run's
// frame holds it too, so the results read it as they read any value the body bound.
func (r *MonteCarloRun) bindStatistic(feature string, value Value) error {
	name, ok := r.ctx.monteCarloMember(r.run.shape, feature)
	if !ok {
		return fmt.Errorf("%w: %s declares no %s::%s to bind",
			ErrMonteCarloObserved, r.run.shape.Label, MonteCarloCaseFQN, feature)
	}
	out, ok := r.run.shape.output(name)
	if !ok {
		return fmt.Errorf("%w: %s of %s is no output", ErrMonteCarloObserved, name, r.run.shape.Label)
	}
	if err := out.Decl.check(r.ctx, &value, func() string {
		return fmt.Sprintf("%s: output %s", r.run.shape.Label, name)
	}); err != nil {
		return err
	}
	r.run.outputs[name] = value
	r.run.env.set(name, value)
	return nil
}

// returnResults runs the deferred results over the run's frame with the statistics
// bound, as the body's end would: a `return` on any path, nested or not, yields the result.
func (r *MonteCarloRun) returnResults() error {
	ctx, run := r.ctx, r.run
	host := &calcStmtHost{ctx: ctx, shape: run.shape, self: run.self}
	var enclosing []frame
	if run.outer != nil {
		enclosing = run.shape.bodyEnclosing(run.outer.enclosingRun(run.shape))
	}
	engine := newStmtEngineIn(ctx, host, run.env, enclosing)
	defer engine.finish()
	host.attachPerformances(engine)
	result, returned, err := runCalcSteps(engine, host, r.results)
	if err != nil {
		return calcFrame(run.shape.Kind, run.shape.Name, fmt.Errorf("result: %w", err))
	}
	if !returned {
		return nil
	}
	if out := run.shape.resultOutput(); out != nil && out.Name != "" {
		run.outputs[out.Name] = result
	} else if _, anonymous := run.shape.anonymousResult(); anonymous {
		run.outputs[resultOutputName] = result
	}
	run.result, run.returned = result, true
	return nil
}

// monteCarloUnconcluded says why one run of a Monte Carlo case reads a value that
// is unbound: its statistics are of repeated runs, which a single run has none of.
func (ctx *Context) monteCarloUnconcluded(sym *symbols.Symbol, err error) error {
	if !errors.Is(err, ErrMultiplicityViolation) || !ctx.IsMonteCarloCase(sym) {
		return err
	}
	return fmt.Errorf("%w; the statistics of a %s case are of repeated runs, which one run leaves unbound: ask for them as -runs <n>", err, MonteCarloCaseFQN)
}
