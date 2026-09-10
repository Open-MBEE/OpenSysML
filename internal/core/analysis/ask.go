package analysis

import (
	"context"
	"errors"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
)

// Held is the model as a surface reaches it through a context it holds: one
// execution runs in ctx, and fresh runs are made where fresh says.
func Held(ctx *runtime.Context, fresh func() (*runtime.Context, error)) *Model {
	return &Model{Context: func() (*runtime.Context, error) { return ctx, nil }, Fresh: fresh}
}

// Perform puts one execution (call, in the model's context) to the registry under auto;
// answer says what it established. A dispatch fault or refusal is the execution's error.
func Perform[T any](
	ctx context.Context,
	r *Registry,
	model *Model,
	subject string,
	schedule runtime.SchedulePolicy,
	budget Budget,
	call func(*runtime.Context) (T, error),
	answer func(T, error) Answer,
) (T, error) {
	var out T
	var err error
	plan, fault := r.Answer(ctx, model, Question{
		Kind:     Evaluate,
		Subject:  subject,
		Schedule: schedule,
		Perform: func(rctx *runtime.Context) (Answer, error) {
			out, err = call(rctx)
			return answer(out, err), nil
		},
	}, budget)
	if fault != nil {
		return out, fault
	}
	if refused := plan.Refused(); refused != nil {
		return out, refused
	}
	return out, err
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

// Explore puts a behavior's outcomes to the registry under auto: run performs
// it once per linearization policy asks for, each in a context the model makes.
func (r *Registry) Explore(
	ctx context.Context,
	model *Model,
	subject string,
	policy runtime.SchedulePolicy,
	run Linearization,
	budget Budget,
) (*runtime.Exploration, error) {
	plan, err := r.Answer(ctx, model, Question{
		Kind:      Outcomes,
		Subject:   subject,
		Schedule:  policy,
		Free:      FreeSchedule,
		Linearize: run,
	}, budget)
	if err != nil {
		return nil, err
	}
	if refused := plan.Refused(); refused != nil {
		return nil, refused
	}
	return plan.Result.Exploration(), nil
}

// Sweep puts a domain to the registry under auto: row runs the subject once per
// row of the plan in the model's context, and the table is the rows in plan order.
func (r *Registry) Sweep(
	ctx context.Context,
	model *Model,
	subject string,
	schedule runtime.SchedulePolicy,
	plan runtime.SweepPlan,
	row runtime.SweepRun,
	budget Budget,
) (runtime.SweepTable, error) {
	answered, err := r.Answer(ctx, model, Question{
		Kind:     Sweep,
		Subject:  subject,
		Schedule: schedule,
		Sweep:    &SweepAsk{Plan: plan, Row: row},
	}, budget)
	if err != nil {
		return runtime.SweepTable{}, err
	}
	if refused := answered.Refused(); refused != nil {
		return runtime.SweepTable{}, refused
	}
	return answered.Result.Table(), nil
}

// Solve puts an element's condition sets to the registry under auto, ask being the
// operation made of each, and returns the answers in order; the error is an absent solver.
func (r *Registry) Solve(ctx context.Context, subject string, queries []*solve.Query, ask Asking, budget Budget) ([]Evaluation, error) {
	plan, err := r.Answer(ctx, nil, Question{
		Kind:    Satisfiable,
		Subject: subject,
		Free:    FreeInputs,
		Solve:   &SolveAsk{Queries: queries, Ask: ask},
	}, budget)
	if err != nil {
		return nil, err
	}
	if refused := plan.Refused(); refused != nil {
		return nil, refused
	}
	return plan.Result.Values, nil
}
