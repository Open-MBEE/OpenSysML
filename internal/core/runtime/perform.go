package runtime

import (
	"errors"
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// QueuedEvent is one occurrence a driven state performance queues before it
// runs: a signal by type, carrying arguments or one bare payload, or an
// operation call carrying arguments.
type QueuedEvent struct {
	Signal string
	Call   string
	Args   map[string]Value
	Value  *Value
}

// ErrMalformedEvent is the typed error Enqueue returns for an event that is
// not exactly one signal or one call.
var ErrMalformedEvent = errors.New("malformed queued event")

// Enqueue queues one event on the executor as a case declares it.
func (e *StateExecutor) Enqueue(event QueuedEvent) error {
	switch {
	case event.Call != "" && event.Signal != "":
		return fmt.Errorf("%w: event declares both signal %q and call %q", ErrMalformedEvent, event.Signal, event.Call)
	case event.Value != nil && (event.Call != "" || len(event.Args) > 0):
		return fmt.Errorf("%w: event %s%s carries a bare value beside its arguments", ErrMalformedEvent, event.Signal, event.Call)
	case event.Value != nil && event.Signal == "":
		return fmt.Errorf("%w: event carries a bare value but declares no signal", ErrMalformedEvent)
	case event.Call != "":
		e.InvokeOperation(event.Call, event.Args)
	case event.Value != nil:
		value := *event.Value
		e.enqueueSignal(Message{SignalType: event.Signal, Value: &value})
	case event.Signal != "":
		e.SendSignal(event.Signal, event.Args)
	default:
		return fmt.Errorf("%w: event declares neither a signal nor a call", ErrMalformedEvent)
	}
	return nil
}

// PerformState drives one performance of a state machine, by self or by no
// object: it enters the initial state, queues the events, and runs through the
// executor's own loop to completion or suspension, returning the executor for
// its outcome. The conformance harness and the referees drive machines this way.
func (ctx *Context) PerformState(stateMachine *symbols.Symbol, self *Instance, events []QueuedEvent) (*StateExecutor, error) {
	exec, err := ctx.CreateStateExecutorFor(stateMachine, self)
	if err != nil {
		return nil, err
	}
	for _, event := range events {
		if err := exec.Enqueue(event); err != nil {
			exec.Release()
			return nil, err
		}
	}
	if err := exec.RunToCompletion(); err != nil {
		exec.Release()
		return nil, err
	}
	return exec, nil
}
