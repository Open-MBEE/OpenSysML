package runtime

import (
	"errors"
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
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

// ErrCallNotReturned is the typed error Call returns for an invocation the
// machine ran to completion without dispatching: it is still queued or deferred,
// so a synchronous caller would still be waiting on it.
var ErrCallNotReturned = errors.New("call not returned")

// pendingCall is the synchronous call Call is waiting on and the outputs the
// machine has returned to its caller so far.
type pendingCall struct {
	id      int64
	outputs map[string]Value
}

// Call invokes operation on the machine as a synchronous caller does: the call
// event is queued, the machine runs to completion, and the caller is released
// with the outputs the behaviors the event triggered (effects, entries, exits,
// do actions) returned to the machine, by name, the last returned under a name
// being its value (PSSM EventTriggeredExecution). A call the run left queued or
// deferred reports ErrCallNotReturned.
func (e *StateExecutor) Call(operation string, args map[string]Value) (map[string]Value, error) {
	call := &pendingCall{id: e.nextEventID, outputs: make(map[string]Value)}
	e.pendingCall = call
	defer func() { e.pendingCall = nil }()
	e.InvokeOperation(operation, args)
	if err := e.RunToCompletion(); err != nil {
		return nil, err
	}
	if held := e.eventDisposition(call.id); held != "" {
		return nil, fmt.Errorf("%w: %s is still %s", ErrCallNotReturned, operation, held)
	}
	return call.outputs, nil
}

// ErrNoSuchAttribute is the typed error WriteAttribute returns for a name the
// machine declares no attribute for.
var ErrNoSuchAttribute = errors.New("no such attribute")

// WriteAttribute writes a value to an attribute the machine declares, as an
// object outside the machine does between its steps: a driver appending to a
// log the machine's own behaviors also write. The write is the same one an
// assignment in the machine's behavior makes, so the value is checked the same way.
func (e *StateExecutor) WriteAttribute(name string, value Value) error {
	if !e.declaresAttribute(name) {
		return fmt.Errorf("%w: state machine %s declares no attribute %q", ErrNoSuchAttribute, symbolText(e.stateMachine), name)
	}
	return e.assignAttribute(name, value)
}

// recordCallOutput keeps an output a behavior returned to the machine while the
// pending call's event is being dispatched, for Call to release the caller with.
func (e *StateExecutor) recordCallOutput(name string, value Value) {
	call := e.pendingCall
	if call == nil || e.firingEvent == nil || e.firingEvent.ID != call.id {
		return
	}
	call.outputs[name] = value
}

// eventDisposition is "queued" or "deferred" for an event the machine still
// holds, empty once it was dispatched.
func (e *StateExecutor) eventDisposition(id int64) string {
	for _, event := range e.eventQueue.Events() {
		if event.ID == id {
			return "queued"
		}
	}
	for _, event := range e.deferred {
		if event.ID == id {
			return "deferred"
		}
	}
	return ""
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
