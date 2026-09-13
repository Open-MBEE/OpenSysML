package analysis

import (
	"context"
	"errors"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
)

// Held is the model as a surface reaches it through a context it holds: an
// execution runs in ctx, under its limits, and no run of its own is built.
func Held(ctx *runtime.Context) *Model {
	return &Model{Context: func() (*runtime.Context, error) { return ctx, nil }}
}

// Request is what every question put to the registry carries: the model it is
// asked of, the subject and schedule it runs under, and the budget and selection it is answered with.
type Request struct {
	Model     *Model
	Subject   string
	Schedule  runtime.SchedulePolicy
	Budget    Budget
	Selection Selection
}

// ask puts q, about the request's subject under its schedule, to the registry
// under the request's budget and selection; a dispatch fault or refusal is the error.
func (r *Registry) ask(ctx context.Context, req Request, q Question) (Plan, error) {
	q.Subject, q.Schedule = req.Subject, req.Schedule
	return refusedOr(r.AnswerWith(ctx, req.Model, q, req.Budget, req.Selection))
}

// refusedOr is a plan and how it failed: a dispatch fault, else the refusal it holds.
func refusedOr(plan Plan, err error) (Plan, error) {
	if err != nil {
		return plan, err
	}
	if refused := plan.Refused(); refused != nil {
		return plan, refused
	}
	return plan, nil
}

// Perform puts one execution (call, in the request's model's context) to the registry;
// answer says what it established. A dispatch fault or refusal is the
// execution's error, and the plan is how it was answered.
func Perform[T any](
	ctx context.Context,
	r *Registry,
	req Request,
	call func(*runtime.Context) (T, error),
	answer func(T, error) Answer,
) (T, Plan, error) {
	var out T
	var err error
	plan, fault := r.ask(ctx, req, Question{
		Kind: Evaluate,
		Perform: func(rctx *runtime.Context) (Answer, error) {
			out, err = call(rctx)
			return answer(out, err), nil
		},
	})
	if fault != nil {
		return out, plan, fault
	}
	return out, plan, err
}

// CheckAnswer is what a constraint, requirement or satisfaction check established. A
// false verdict arrives as an error unwrapping to runtime.ErrViolated and is a violation.
func CheckAnswer(result runtime.CheckResult, err error) Answer {
	switch {
	case errors.Is(err, runtime.ErrViolated):
		return Answer{Claim: ClaimViolated, Reason: err.Error()}
	case err != nil:
		return Answer{Err: err}
	case result.Holds:
		return Answer{Claim: ClaimHolds}
	}
	return Answer{Claim: ClaimViolated}
}

// ValuesAnswer is what a run producing values established: the values, or
// nothing when it failed.
func ValuesAnswer(values []Evaluation, err error) Answer {
	if err != nil {
		return Answer{Err: err, Values: values}
	}
	return Answer{Claim: ClaimValue, Values: values}
}

// OutputValues is a case's or calculation's outputs as evaluations.
func OutputValues(outputs []runtime.CalcOutputValue) []Evaluation {
	values := make([]Evaluation, len(outputs))
	for i, out := range outputs {
		values[i] = Evaluation{Name: out.Name, Value: out.Value}
	}
	return values
}

// ValuesOf is a behavior's outputs or state data as evaluations, in name order.
func ValuesOf(held map[string]runtime.Value) []Evaluation {
	names := make([]string, 0, len(held))
	for name := range held {
		names = append(names, name)
	}
	slices.Sort(names)
	values := make([]Evaluation, len(names))
	for i, name := range names {
		values[i] = Evaluation{Name: name, Value: held[name]}
	}
	return values
}

// CaseAnswer is what one analysis case run established: its outputs.
func CaseAnswer(result runtime.AnalysisResult, err error) Answer {
	return ValuesAnswer(OutputValues(result.Outputs), err)
}

// VerificationAnswer is what one verification case run established: pass holds,
// fail is violated, and an inconclusive or erroring body establishes nothing.
func VerificationAnswer(result runtime.VerificationResult, err error) Answer {
	answer := Answer{Err: err, Reason: result.Verdict.Detail, Values: OutputValues(result.Run.Outputs)}
	if err != nil {
		return answer
	}
	switch result.Verdict.Kind {
	case runtime.VerdictPass:
		answer.Claim = ClaimHolds
	case runtime.VerdictFail:
		answer.Claim = ClaimViolated
	}
	return answer
}

// Explore puts a behavior's outcomes to the registry: run performs it once per
// linearization the request's schedule asks for, each in a context its model makes.
// The plan's result holds the Exploration reached.
func (r *Registry) Explore(ctx context.Context, req Request, run Linearization) (Plan, error) {
	return r.ask(ctx, req, Question{
		Kind:      Outcomes,
		Free:      FreeSchedule,
		Linearize: run,
	})
}

// Check puts an action's schedules to the registry as a question of kind, free being
// what the asker leaves open beside them: check is the explicit-state search's ask,
// holds the symbolic one, and run performs the action once for an engine exploring it.
func (r *Registry) Check(ctx context.Context, req Request, kind Kind, free Freedom, check *CheckAsk, holds *HoldsAsk, run Linearization) (Plan, error) {
	return r.ask(ctx, req, Question{
		Kind:      kind,
		Free:      FreeSchedule | free,
		Linearize: run,
		Check:     check,
		Holds:     holds,
	})
}

// CheckKind is what a check asks: Holds once a property or condition is stated, else Outcomes.
func CheckKind(check *CheckAsk, holds *HoldsAsk) Kind {
	if check != nil && len(check.Properties) > 0 || holds != nil && len(holds.Conditions) > 0 {
		return Holds
	}
	return Outcomes
}

// Sweep puts a domain to the registry: row runs the subject once per row of the
// plan, each in a context of its own the request's model builds; the answered
// plan's result tables the rows.
func (r *Registry) Sweep(ctx context.Context, req Request, plan runtime.SweepPlan, row runtime.SweepRun) (Plan, error) {
	return r.ask(ctx, req, Question{
		Kind:  Sweep,
		Sweep: &SweepAsk{Plan: plan, Row: row},
	})
}

// Solve puts an element's condition sets to the registry under the selection, ask being
// the operation made of each; the plan's result holds the answers in order, and the
// error is an absent solver.
func (r *Registry) Solve(ctx context.Context, subject string, queries []*solve.Query, ask Asking, budget Budget, selection Selection) (Plan, error) {
	return refusedOr(r.AnswerWith(ctx, nil, Question{
		Kind:    Satisfiable,
		Subject: subject,
		Free:    FreeInputs,
		Solve:   &SolveAsk{Queries: queries, Ask: ask},
	}, budget, selection))
}
