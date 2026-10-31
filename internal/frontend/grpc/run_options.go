package grpc

import (
	"math"

	"connectrpc.com/connect"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

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

// runAction runs action performed by self on rt, posting events as the run waits: to completion,
// or for the duration asked, in which case one that has not completed is runtime.ErrNotCompleted.
func (o runOptions) runAction(rt *runtime.Context, action *symbols.Symbol, self *runtime.Instance, inputs map[string]runtime.Value, events []runtime.Message) (actionRun, error) {
	var run actionRun
	var err error
	if o.advance == nil {
		run.outputs, run.performer, err = rt.ExecuteActionWithEvents(action, self, inputs, events)
	} else {
		run.outputs, run.performer, err = rt.ExecuteActionAdvancedBy(action, self, inputs, events, *o.advance)
	}
	if err != nil {
		return actionRun{}, err
	}
	return run, nil
}

// actionOutcome is runAction as an exploration's run observes it.
func (o runOptions) actionOutcome(rt *runtime.Context, action *symbols.Symbol, self *runtime.Instance, inputs map[string]runtime.Value) (runtime.Outcome, error) {
	if o.advance == nil {
		return rt.ActionOutcomePerformedBy(action, self, inputs)
	}
	return rt.ActionOutcomeAdvancedBy(action, self, inputs, *o.advance)
}

// stateOutcome runs the machine self performs (the one it exhibits, where it does) over
// events on rt: to quiescence, or sending the events and then moving the clock by the duration asked.
func (o runOptions) stateOutcome(rt *runtime.Context, stateMachine *symbols.Symbol, self *runtime.Instance, events []string) (runtime.Outcome, error) {
	if o.advance == nil {
		return rt.StateOutcomePerformedBy(stateMachine, self, events)
	}
	return rt.StateOutcomeAdvancedBy(stateMachine, self, events, *o.advance)
}
