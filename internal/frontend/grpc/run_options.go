package grpc

import (
	"errors"
	"fmt"
	"math"

	"connectrpc.com/connect"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// ErrNotCompleted is the typed error an action run for a duration that has not
// completed by its end fails with; the message names where it stopped.
var ErrNotCompleted = errors.New("action did not complete")

// runOptions are the options of one behavior run a request carries beside the
// schedule, as `sysml -advance`, `-seed`, `-draws` and `-clock-step` carry them.
type runOptions struct {
	// advance is the simulated time to run for; nil runs to completion.
	advance *float64
	// seed is the seed the model's own draws come from; nil leaves them to the schedule.
	seed      *uint64
	draws     runtime.DrawPolicy
	clockStep float64
}

// readRunOptions reads a request's run options, each refused as INVALID_ARGUMENT
// where the CLI refuses the flag.
func readRunOptions(advance *float64, seed *uint64, draws string, clockStep float64) (runOptions, error) {
	opts := runOptions{advance: advance, seed: seed, clockStep: clockStep}
	if advance != nil && (math.IsNaN(*advance) || math.IsInf(*advance, 0) || *advance < 0) {
		return opts, statusErrorf(connect.CodeInvalidArgument, "advance takes a finite, non-negative duration to run for, not %s", semantics.FormatReal(*advance))
	}
	if draws != "" {
		policy, err := runtime.ParseDrawPolicy(draws)
		if err != nil {
			return opts, statusError(connect.CodeInvalidArgument, err.Error())
		}
		opts.draws = policy
	}
	if err := runtime.CheckClockStep(clockStep); err != nil {
		return opts, statusError(connect.CodeInvalidArgument, err.Error())
	}
	return opts, nil
}

// modelSeed is the seed as a question to the engines carries it.
func (o runOptions) modelSeed() analysis.ModelSeed {
	if o.seed == nil {
		return analysis.ModelSeed{}
	}
	return analysis.ModelSeed{Seed: *o.seed, Set: true}
}

// apply gives rt the options, as the REPL gives its runtime the session's.
func (o runOptions) apply(rt *runtime.Context) {
	if o.seed != nil {
		rt.SetModelSeed(*o.seed)
	} else {
		rt.ClearModelSeed()
	}
	rt.SetDrawPolicy(o.draws)
	_ = rt.SetClockStep(o.clockStep) // checked by readRunOptions
}

// runAction runs action performed by self on rt: to completion, or for the
// duration asked, in which case one that has not completed is ErrNotCompleted.
func (o runOptions) runAction(rt *runtime.Context, name string, action *symbols.Symbol, self *runtime.Instance, inputs map[string]runtime.Value) (actionRun, error) {
	if o.advance == nil {
		outputs, performer, err := rt.ExecuteActionReportingPerformer(action, self, inputs)
		return actionRun{outputs: outputs, performer: performer}, err
	}
	exec, release, err := o.startAction(rt, name, action, self, inputs)
	if err != nil {
		return actionRun{}, err
	}
	defer release()
	outputs, err := exec.ResultsWithError()
	if err != nil {
		return actionRun{}, err
	}
	performer := (&runtime.Invocation{Actions: []*runtime.ActionExecutor{exec}}).PerformerAttributes()
	return actionRun{outputs: outputs, performer: performer}, nil
}

// actionOutcome is runAction as an exploration's run observes it.
func (o runOptions) actionOutcome(rt *runtime.Context, name string, action *symbols.Symbol, self *runtime.Instance, inputs map[string]runtime.Value) (runtime.Outcome, error) {
	if o.advance == nil {
		return rt.ActionOutcomePerformedBy(action, self, inputs)
	}
	exec, _, err := o.startAction(rt, name, action, self, inputs)
	if err != nil {
		return runtime.Outcome{}, err
	}
	if _, err := exec.ResultsWithError(); err != nil {
		return runtime.Outcome{}, err
	}
	return (&runtime.Invocation{Actions: []*runtime.ActionExecutor{exec}}).Outcome(), nil
}

// startAction starts the performance a run to completion would run and moves the clock by
// the duration asked, as `-action X -advance T` does; release lets go of one started for the call.
func (o runOptions) startAction(rt *runtime.Context, name string, action *symbols.Symbol, self *runtime.Instance, inputs map[string]runtime.Value) (*runtime.ActionExecutor, func(), error) {
	exec, reused, err := rt.StartActionPerformedBy(action, self, inputs)
	if err != nil {
		return nil, nil, err
	}
	release := exec.Release
	if reused {
		release = func() {}
	}
	if _, err := rt.Advance(*o.advance); err != nil {
		release()
		return nil, nil, err
	}
	if !exec.State().Ended() {
		err := fmt.Errorf("%w: action %s stopped at %s at simulation time %s without completing",
			ErrNotCompleted, name, exec.State(), semantics.FormatReal(rt.Clock().Now()))
		if next, ok := exec.NextWait(); ok {
			err = fmt.Errorf("%w; the clock must reach t=%s for it to go on", err, semantics.FormatReal(next))
		}
		release()
		return nil, nil, err
	}
	return exec, release, nil
}

// stateOutcome runs the machine self performs (the one it exhibits, where it does) over
// events on rt: to quiescence, or sending the events and then moving the clock by the duration asked.
func (o runOptions) stateOutcome(rt *runtime.Context, stateMachine *symbols.Symbol, self *runtime.Instance, events []string) (runtime.Outcome, error) {
	if o.advance == nil {
		return rt.StateOutcomePerformedBy(stateMachine, self, events)
	}
	exec, reused, err := rt.StartStatePerformedBy(stateMachine, self)
	if err != nil {
		return runtime.Outcome{}, err
	}
	if !reused {
		defer exec.Release()
	}
	for _, event := range events {
		exec.SendSignal(event, nil)
	}
	if _, err := rt.Advance(*o.advance); err != nil {
		return runtime.Outcome{}, err
	}
	if _, err := exec.StateDataWithError(); err != nil {
		return runtime.Outcome{}, err
	}
	return (&runtime.Invocation{States: []*runtime.StateExecutor{exec}}).Outcome(), nil
}
