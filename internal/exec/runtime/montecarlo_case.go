package runtime

import (
	"errors"
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A Simulation::MonteCarlo case is an analysis of repeated runs: each run performs
// the case's steps on a fresh subject and reads `observed`; the case's statistics
// — runs, mean, deviation, outOfSpec — are of the sample, bound once it is complete.

// MonteCarloCaseFQN names the library analysis of repeated runs.
const MonteCarloCaseFQN = "Simulation::MonteCarlo"

// The features Simulation::MonteCarlo declares, by the name each is declared under.
const (
	monteCarloObserved  = "observed"
	monteCarloRuns      = "runs"
	monteCarloMean      = "mean"
	monteCarloDeviation = "deviation"
	monteCarloOutOfSpec = "outOfSpec"
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
// and `observed` read, its statistics unbound until Conclude binds the sample's.
type MonteCarloRun struct {
	ctx   *Context
	sym   *symbols.Symbol
	scope *symbols.Scope
	run   *calcRun
	log   *evaluationLog
	// results are the returns ending the case's steps, evaluated by Conclude.
	results []lower.Statement

	// Case is the qualified name of the case that ran; Subject the object it ran on.
	Case    string
	Subject *Instance

	// Observed is the value the run's `observed` came to, null when the run left it unbound.
	Observed Value

	// Verdicts are the case's objectives and assertions checked over this run alone;
	// one reading a statistic of the sample is undecided here and decided by Conclude.
	Verdicts []AnalysisVerdict
}

// Context is the context the run was made in, which its values are read through.
func (r *MonteCarloRun) Context() *Context { return r.ctx }

// OutOfSpec reports a run some required check of which did not hold.
func (r *MonteCarloRun) OutOfSpec() bool {
	for _, v := range r.Verdicts {
		if v.Status == VerdictNotSatisfied {
			return true
		}
	}
	return false
}

// ObserveMonteCarlo makes one run of a Simulation::MonteCarlo case as RunAnalysis
// would, binding its subject and inputs from args, but reads `observed` and the
// checks in place of the outputs: the statistics they read are of the whole
// sample, which Conclude binds. self, when non-null, is the object a usage is a feature of.
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
	// The run's environment outlives the invocation: Conclude reads it once the sample is in.
	run = run.detached()
	_, results := shape.observationSteps()
	return &MonteCarloRun{
		ctx: ctx, sym: sym, scope: scope, run: run, log: log, results: results,
		Case:     shape.Name,
		Subject:  run.boundSubject(ctx),
		Observed: observed,
		Verdicts: ctx.analysisVerdicts(run, sym, scope),
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
	// Deviation is their sample standard deviation; unbound under two runs.
	Deviation float64
	// OutOfSpec is the number of runs a required check of the case did not hold in.
	OutOfSpec int64
}

// MonteCarloSample is the statistics of runs, each of which observed a number;
// one observing none, or no number, refuses the sample.
func MonteCarloSample(runs []*MonteCarloRun) (MonteCarloStatistics, error) {
	if len(runs) == 0 {
		return MonteCarloStatistics{}, fmt.Errorf("%w: no run observed anything", ErrMonteCarloObserved)
	}
	numbers := make([]semantics.Value, 0, len(runs))
	var outOfSpec int64
	for i, run := range runs {
		number, ok := observedNumber(run.Observed)
		if !ok {
			return MonteCarloStatistics{}, fmt.Errorf("%w: run %d of %s observed %s, not a number",
				ErrMonteCarloObserved, i+1, run.Case, describeObserved(run.Observed))
		}
		numbers = append(numbers, number)
		if run.OutOfSpec() {
			outOfSpec++
		}
	}
	d := Distribute(numbers)
	return MonteCarloStatistics{Runs: int64(d.Count), Mean: d.Mean, Deviation: d.Deviation, OutOfSpec: outOfSpec}, nil
}

// observedNumber is the number an observation holds: a scalar Integer or Real,
// or the number of a quantity value.
func observedNumber(value Value) (semantics.Value, bool) {
	value = soleElement(value)
	if value.Kind != ValConst {
		return semantics.Value{}, false
	}
	switch value.Const.Kind {
	case semantics.ValInt, semantics.ValReal:
		return value.Const, true
	}
	return semantics.Value{}, false
}

// describeObserved words an observation that is no number.
func describeObserved(value Value) string {
	if value.Kind == ValNull {
		return "no value"
	}
	return value.Kind.String()
}

// Conclude binds the sample's statistics to the case's runs, mean, deviation and
// outOfSpec, then reports the run as RunAnalysis reports one: every output the case
// declares, evaluated over the statistics, and the verdict of each objective and assertion.
func (r *MonteCarloRun) Conclude(stats MonteCarloStatistics) (AnalysisResult, error) {
	ctx := r.ctx
	defer ctx.beginRun()()
	r.log.enclosing = ctx.evaluations
	ctx.evaluations = r.log
	defer ctx.endEvaluationLog(r.log)

	bound := []struct {
		feature string
		value   Value
		bind    bool
	}{
		{monteCarloRuns, constValue(drawnInt(stats.Runs)), true},
		{monteCarloMean, constValue(drawnReal(stats.Mean)), true},
		{monteCarloDeviation, constValue(drawnReal(stats.Deviation)), stats.Runs >= 2},
		{monteCarloOutOfSpec, constValue(drawnInt(stats.OutOfSpec)), true},
	}
	for _, b := range bound {
		if !b.bind {
			continue
		}
		if err := r.bindStatistic(b.feature, b.value); err != nil {
			return AnalysisResult{Case: r.Case, Subject: r.Subject}, err
		}
	}

	result := AnalysisResult{Case: r.Case, Subject: r.Subject}
	if err := r.returnResults(); err != nil {
		result.Verdicts = ctx.undecidedVerdicts(r.sym, r.scope, err)
		result.Evaluations = r.log.evaluations(Value{}, false)
		return result, err
	}
	outputs, err := r.run.outputValues(ctx)
	result.Outputs = outputs
	if err != nil {
		result.Verdicts = ctx.undecidedVerdicts(r.sym, r.scope, err)
		result.Evaluations = r.log.evaluations(Value{}, false)
		return result, err
	}
	result.Verdicts = ctx.analysisVerdicts(r.run, r.sym, r.scope)
	result.Evaluations = r.log.evaluations(r.run.caseResult(r.run.bindingsFrame(ctx).vars))
	return result, nil
}

// bindStatistic gives the library's output feature its value, under the name the
// case binds it, checked against the declaration as a binding's value is.
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
	return nil
}

// returnResults evaluates the returns the run deferred: a `return expr` yields the
// result as the body would have; a bound result parameter is read as an output.
func (r *MonteCarloRun) returnResults() error {
	for _, stmt := range r.results {
		ret := stmt.(lower.Return)
		if _, bound := ret.Node.(*ast.Usage); bound {
			continue
		}
		value, err := r.run.bindingEnv(r.ctx, r.run.shape.BodyOwner).Eval(ret.Value)
		if err != nil {
			return calcFrame(r.run.shape.Kind, r.run.shape.Name, fmt.Errorf("result: %w", err))
		}
		if out := r.run.shape.resultOutput(); out != nil {
			if err := out.Decl.check(r.ctx, &value, func() string { return "result" }); err != nil {
				return err
			}
			if out.Name != "" {
				r.run.outputs[out.Name] = value
			}
		}
		r.run.result, r.run.returned = value, true
	}
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
