package repl

import (
	"context"
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
)

// budgetFor is the session's bounds as a question of kind under policy states them.
func (s *Session) budgetFor(policy runtime.SchedulePolicy, kind analysis.Kind) analysis.Budget {
	return analysis.BudgetOf(s.budgets, policy, kind)
}

// Engine returns the engine selection every question the session asks is made under.
func (s *Session) Engine() analysis.Selection {
	defer s.reading()()
	return s.engine
}

// SetEngine selects the engine questions asked from here on are put to: `auto`,
// `all`, or one engine by name. A name no engine is registered under is a typed error.
func (s *Session) SetEngine(text string) error {
	defer s.enter()()
	selection, err := s.engines.Select(text)
	if err != nil {
		return err
	}
	return s.setEngine(selection)
}

// setEngine records the selection and moves the session's context to the schedule it
// drives, which changes when the selection makes runs explore.
func (s *Session) setEngine(selection analysis.Selection) error {
	s.engine = selection
	if s.rtCtx != nil {
		return s.rtCtx.SetSchedule(s.drivenSchedule())
	}
	return nil
}

// SetEngines replaces the registry the session's questions are put to, keeping the
// selection where the registry still knows it. A nil registry is refused.
func (s *Session) SetEngines(engines *analysis.Registry) error {
	defer s.enter()()
	if engines == nil {
		return &NoEnginesError{}
	}
	selection, err := engines.Select(s.engine.String())
	if err != nil {
		return err
	}
	s.engines = engines
	return s.setEngine(selection)
}

// NoEnginesError reports a session given no registry to put its questions to.
type NoEnginesError struct{}

func (e *NoEnginesError) Error() string { return "no engine registry to put questions to" }

// Engines lists the registered engines with their authority, the questions each
// answers and the state of its process, as `%engines` prints them.
func (s *Session) Engines() []analysis.Listing {
	defer s.reading()()
	return s.engines.Listings()
}

// doEngine shows the engine selection, or sets it when one is named. `explore` is
// refused as `%schedule explore` is: the prompt's debuggers step one run.
func (s *Session) doEngine(args []string) []string {
	if len(args) > 0 {
		selection, err := s.engines.Select(args[0])
		if err != nil {
			return []string{errPrefix + err.Error()}
		}
		if policy, explores := analysis.Explores(selection, runtime.DefaultSchedulePolicy); explores {
			return []string{errPrefix + (&ExploreAtPromptError{Policy: policy}).Error()}
		}
		if err := s.setEngine(selection); err != nil {
			return []string{errPrefix + err.Error()}
		}
	}
	return []string{fmt.Sprintf("engine: %s", s.engine)}
}

// doEngines lists the registered engines.
func (s *Session) doEngines() []string {
	return analysis.Lines(s.engines.Listings())
}

// standingPrefix opens the line that follows a verdict with its standing.
const standingPrefix = "  standing: "

// standing attaches to a verdict the plan that answered it and the line
// reporting the standing of the answer; a verdict no engine was asked for keeps its lines.
func standing(v Verdict, plan *analysis.Plan) Verdict {
	if plan == nil {
		return v
	}
	v.Plan = plan
	v.Lines = append(v.Lines, standingPrefix+plan.Standing())
	return v
}

// execution is how one run is made: put to the engines under the session's selection,
// or, inside a linearization an engine is already running, made directly in its context.
type execution struct {
	s      *Session
	direct bool
}

// dispatched is the execution the prompt makes: a question to the engines.
func (s *Session) dispatched() execution { return execution{s: s} }

// direct is the execution a linearization makes, which an engine is already answering.
func (s *Session) direct() execution { return execution{s: s, direct: true} }

// evaluate makes one execution in ctx; the plan is nil for a direct one.
func evaluate[T any](x execution, subject string, ctx *runtime.Context, call func(*runtime.Context) (T, error), answer func(T, error) analysis.Answer) (T, *analysis.Plan, error) {
	if x.direct {
		out, err := call(ctx)
		return out, nil, err
	}
	s := x.s
	schedule := s.drivenSchedule()
	out, plan, err := analysis.Perform(context.Background(), s.engines, analysis.Held(ctx), subject, schedule, s.budgetFor(schedule, analysis.Evaluate), s.engine, call, answer)
	return out, &plan, err
}

// check puts one constraint, requirement or satisfaction check to the engines.
func (s *Session) check(subject string, ctx *runtime.Context, call func(*runtime.Context) (runtime.CheckResult, error)) (runtime.CheckResult, *analysis.Plan, error) {
	return evaluate(s.dispatched(), subject, ctx, call, analysis.CheckAnswer)
}

// behaviorAnswer is what a behavior's run established: its values when it ran
// to where it was asked to, nothing when it failed or stopped short.
func behaviorAnswer(reached bool, reason string, values map[string]runtime.Value, err error) analysis.Answer {
	switch {
	case err != nil:
		return analysis.Answer{Err: err}
	case !reached:
		return analysis.Answer{Reason: reason}
	}
	return analysis.Answer{Claim: analysis.ClaimValue, Values: analysis.ValuesOf(values)}
}

// performed is what one behavior run at the prompt reported: its lines and values.
type performed struct {
	lines  []string
	values []NamedValue
}

// advanceFor puts the run of the behaviors named in subject, their clocks
// advanced by duration, to the engines; a budget stop is a run that fell short.
func (s *Session) advanceFor(subject string, contexts []*runtime.Context, duration float64) (advanceOutcome, []string, *analysis.Plan, error) {
	type drained struct {
		moved  advanceOutcome
		failed []string
	}
	done, plan, err := evaluate(s.dispatched(), subject, contexts[0], func(*runtime.Context) (drained, error) {
		moved, failed, err := s.advanceContexts(contexts, duration)
		return drained{moved: moved, failed: failed}, err
	}, func(done drained, err error) analysis.Answer {
		if err == nil && done.moved.stopped != nil {
			return analysis.Answer{Err: done.moved.stopped}
		}
		return behaviorAnswer(true, "", nil, err)
	})
	return done.moved, done.failed, plan, err
}

// runCase makes one analysis case run.
func (x execution) runCase(subject string, ctx *runtime.Context, call func(*runtime.Context) (runtime.AnalysisResult, error)) (runtime.AnalysisResult, *analysis.Plan, error) {
	return evaluate(x, subject, ctx, call, analysis.CaseAnswer)
}

// runVerification makes one verification case run.
func (x execution) runVerification(subject string, ctx *runtime.Context, call func(*runtime.Context) (runtime.VerificationResult, error)) (runtime.VerificationResult, *analysis.Plan, error) {
	return evaluate(x, subject, ctx, call, analysis.VerificationAnswer)
}

// explore puts a behavior's outcomes to the engines under selection: run performs
// it once per linearization, each in a context of the plan's own over model.
func (s *Session) explore(subject string, policy runtime.SchedulePolicy, selection analysis.Selection, model *analysis.Model, run analysis.Linearization) (analysis.Plan, error) {
	return s.engines.Explore(context.Background(), model, subject, policy, run, s.budgetFor(policy, analysis.Outcomes), selection)
}

// sweep puts a domain to the engines under the session's selection: row runs the
// target once per row of the plan in ctx.
func (s *Session) sweep(target string, ctx *runtime.Context, plan runtime.SweepPlan, row runtime.SweepRun) (analysis.Plan, error) {
	schedule := s.drivenSchedule()
	return s.engines.Sweep(context.Background(), analysis.Held(ctx), target, schedule, plan, row, s.budgetFor(schedule, analysis.Sweep), s.engine)
}

// solveWith puts an element's condition sets to the engines under the session's
// selection, ask being the operation made of each; the plan's result answers each
// in order. The error is a refusal: no solver, or a selected engine that does not solve.
func (s *Session) solveWith(subject string, queries []*solve.Query, ask analysis.Asking) (analysis.Plan, error) {
	return s.engines.Solve(context.Background(), subject, queries, ask, s.budgetFor(s.drivenSchedule(), analysis.Satisfiable), s.engine)
}

// solveReports renders one report per query from the plan's answers, every report
// carrying the plan and the last followed by its standing.
func solveReports(name string, queries []*solve.Query, plan analysis.Plan, err error, report func(string, *solve.Query, analysis.Evaluation) SolveReport) []SolveReport {
	if err != nil {
		return []SolveReport{withStanding(unavailableReport(name, err.Error()), &plan)}
	}
	if len(plan.Result.Values) != len(queries) {
		return []SolveReport{withStanding(unavailableReport(name, plan.Result.Standing()), &plan)}
	}
	reports := make([]SolveReport, 0, len(queries))
	for i, q := range queries {
		r := report(name, q, plan.Result.Values[i])
		r.Plan = &plan
		reports = append(reports, r)
	}
	reports[len(reports)-1] = withStanding(reports[len(reports)-1], &plan)
	return reports
}

func withStanding(r SolveReport, plan *analysis.Plan) SolveReport {
	r.Plan = plan
	r.Lines = append(r.Lines, standingPrefix+plan.Standing())
	return r
}
