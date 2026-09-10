package repl

import (
	"context"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
)

// budgetFor is the session's bounds as a question under policy states them.
func (s *Session) budgetFor(policy runtime.SchedulePolicy) analysis.Budget {
	return analysis.BudgetOf(s.budgets, policy)
}

// evaluate puts one execution in ctx to the session's engines under auto.
func evaluate[T any](s *Session, subject string, ctx *runtime.Context, call func(*runtime.Context) (T, error), answer func(T, error) analysis.Answer) (T, error) {
	schedule := s.drivenSchedule()
	return analysis.Perform(context.Background(), s.engines, analysis.Held(ctx, nil), subject, schedule, s.budgetFor(schedule), call, answer)
}

// check puts one constraint, requirement or satisfaction check to the engines.
func (s *Session) check(subject string, ctx *runtime.Context, call func(*runtime.Context) (runtime.CheckResult, error)) (runtime.CheckResult, error) {
	return evaluate(s, subject, ctx, call, analysis.CheckAnswer)
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
func (s *Session) advanceFor(subject string, contexts []*runtime.Context, duration float64) (advanceOutcome, []string, error) {
	type drained struct {
		moved  advanceOutcome
		failed []string
	}
	done, err := evaluate(s, subject, contexts[0], func(*runtime.Context) (drained, error) {
		moved, failed, err := s.advanceContexts(contexts, duration)
		return drained{moved: moved, failed: failed}, err
	}, func(done drained, err error) analysis.Answer {
		if err == nil && done.moved.stopped != nil {
			return analysis.Answer{Err: done.moved.stopped}
		}
		return behaviorAnswer(true, "", nil, err)
	})
	return done.moved, done.failed, err
}

// runCase puts one analysis case run to the engines.
func (s *Session) runCase(subject string, ctx *runtime.Context, call func(*runtime.Context) (runtime.AnalysisResult, error)) (runtime.AnalysisResult, error) {
	return evaluate(s, subject, ctx, call, analysis.CaseAnswer)
}

// runVerification puts one verification case run to the engines.
func (s *Session) runVerification(subject string, ctx *runtime.Context, call func(*runtime.Context) (runtime.VerificationResult, error)) (runtime.VerificationResult, error) {
	return evaluate(s, subject, ctx, call, analysis.VerificationAnswer)
}

// explore puts a behavior's outcomes to the engines under auto: run performs it
// once per linearization, each in a context fresh makes.
func (s *Session) explore(subject string, policy runtime.SchedulePolicy, fresh func() (*runtime.Context, error), run analysis.Linearization) (*runtime.Exploration, error) {
	return s.engines.Explore(context.Background(), &analysis.Model{Fresh: fresh}, subject, policy, run, s.budgetFor(policy))
}

// sweep puts a domain to the engines under auto: row runs the target once per
// row of the plan in ctx.
func (s *Session) sweep(target string, ctx *runtime.Context, plan runtime.SweepPlan, row runtime.SweepRun) (runtime.SweepTable, error) {
	schedule := s.drivenSchedule()
	return s.engines.Sweep(context.Background(), analysis.Held(ctx, nil), target, schedule, plan, row, s.budgetFor(schedule))
}

// solveWith puts an element's condition sets to the engines under auto, ask
// being the operation made of each, and returns the solver's answer to each in
// order. The error is the one for a solver that is absent.
func (s *Session) solveWith(subject string, queries []*solve.Query, ask analysis.Asking) ([]analysis.Evaluation, error) {
	return s.engines.Solve(context.Background(), subject, queries, ask, s.budgetFor(s.drivenSchedule()))
}
