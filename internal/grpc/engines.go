package grpc

import (
	"context"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// perform puts one execution on the request's runtime to the service's engines
// under auto, under the schedule that runtime was set.
func perform[T any](ctx context.Context, v *verifyContext, subject string, call func(*runtime.Context) (T, error), answer func(T, error) analysis.Answer) (T, error) {
	return performOn(ctx, v.service, v.runtime, subject, call, answer)
}

// check puts one constraint, requirement or satisfaction check to the engines.
func (v *verifyContext) check(ctx context.Context, subject string, call func(*runtime.Context) (runtime.CheckResult, error)) (runtime.CheckResult, error) {
	return perform(ctx, v, subject, call, analysis.CheckAnswer)
}

// performOn puts one execution on rt to the service's engines under auto, under
// the schedule rt was set.
func performOn[T any](ctx context.Context, s *Service, rt *runtime.Context, subject string, call func(*runtime.Context) (T, error), answer func(T, error) analysis.Answer) (T, error) {
	schedule := rt.Schedule()
	return analysis.Perform(ctx, s.engines, analysis.Held(rt), subject, schedule, analysis.BudgetOf(s.budgets, schedule, analysis.Evaluate), call, answer)
}

// callerGone is the caller's own error when a run failed because the caller went
// away, which fails the call rather than being reported as the run's failure.
func callerGone(ctx context.Context, err error) error {
	if err != nil {
		return ctx.Err()
	}
	return nil
}

// heldAnswer is what a behavior run established: the values it left, or nothing.
func heldAnswer(held map[string]runtime.Value, err error) analysis.Answer {
	return analysis.ValuesAnswer(analysis.ValuesOf(held), err)
}

// stateRun is what one state machine run left: its final data and the states it visited.
type stateRun struct {
	final   map[string]runtime.Value
	visited []string
}
