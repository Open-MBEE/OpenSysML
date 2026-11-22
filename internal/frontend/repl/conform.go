package repl

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/exec/objref"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
)

// ConformTrace checks typed external observations against one state machine.
// It resolves the behavior as -state does but runs in an isolated context.
func (s *Session) ConformTrace(machine Behavior, rows []runtime.TraceRow, budget runtime.CheckBudget, timeout time.Duration) (runtime.TraceReport, error) {
	defer s.enter()()
	inv, unresolved := s.resolveInvocation(nil, []Behavior{machine}, nil)
	if inv == nil {
		if len(unresolved) > 0 {
			return runtime.TraceReport{}, &runtime.SetupError{Err: errors.New(strings.Join(unresolved[0].Lines, "\n"))}
		}
		return runtime.TraceReport{}, &runtime.SetupError{Err: errors.New("no state machine resolved")}
	}
	model := s.freshModel()
	ctx := s.planContext()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	s.state.Unlock()
	defer s.state.Lock()
	return runtime.MatchTrace(ctx, func() (*runtime.Context, error) {
		return model.NewContext(analysis.Budget{})
	}, func(ctx *runtime.Context) (*runtime.Invocation, error) {
		run, err := inv.start(ctx)
		if err != nil {
			// A bad performer selection is invalid input, unlike a model
			// evaluation that happens to fail while resolving that performer.
			var path *objref.PathError
			var exhibitors *ExhibitorsError
			var ambiguous *AmbiguousMachineError
			if (errors.As(err, &path) && path.Err == nil) ||
				errors.As(err, &exhibitors) || errors.As(err, &ambiguous) {
				return nil, &runtime.SetupError{Err: err}
			}
			return nil, err
		}
		// There is no simulated-time input in v1. The matcher advances only
		// untimed behavior to quiescence, not the ordinary -state horizon.
		run.Horizon = runtime.Horizon{}
		return run, nil
	}, rows, budget)
}
