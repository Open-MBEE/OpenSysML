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

// ErrCallNotReturned reports a call the run left queued or deferred, so a
// synchronous caller would still be waiting on it.
var ErrCallNotReturned = errors.New("call not returned")

// pendingCall is the synchronous call Call is waiting on, the outputs the
// machine has returned to its caller so far and the names the operation's
// declaration lets out (nil when the machine's owner declares no such operation).
type pendingCall struct {
	id      int64
	outputs map[string]Value
	returns map[string]bool
}

// Call queues the call event, runs the machine through the run-to-completion
// step dispatching it and releases the caller with the outputs the behaviors that
// step fired returned, by name (PSSM 8.5.9): the operation's out parameters when
// the machine's owner declares it, every output otherwise. Events that step
// queued and timers it armed stay for the machine's later runs; the clock does
// not move.
func (e *StateExecutor) Call(operation string, args map[string]Value) (map[string]Value, error) {
	call := &pendingCall{id: e.nextEventID, outputs: make(map[string]Value), returns: e.callReturns(operation)}
	e.pendingCall = call
	defer func() { e.pendingCall = nil }()
	e.InvokeOperation(operation, args)
	if err := e.RunToQuiescence(); err != nil {
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

// WriteAttribute assigns an attribute the machine declares from outside it,
// between its steps, with the checks an assignment in its behavior gets.
func (e *StateExecutor) WriteAttribute(name string, value Value) error {
	if !e.declaresAttribute(name) {
		return fmt.Errorf("%w: state machine %s declares no attribute %q", ErrNoSuchAttribute, symbolText(e.stateMachine), name)
	}
	return e.assignAttribute(name, value)
}

// callReleased reports whether the pending call's event has left the queue, so
// its run-to-completion step is done (or it is deferred) and the caller may go.
func (e *StateExecutor) callReleased() bool {
	return e.pendingCall != nil && e.eventDisposition(e.pendingCall.id) != "queued"
}

// callReturns is the set of out and inout parameters the operation declares as a
// member of the machine's owner (of the machine itself when it stands alone);
// nil when no member of that name states a behavior.
func (e *StateExecutor) callReturns(operation string) map[string]bool {
	owner := e.stateMachine
	if e.self != nil && e.self.Type != nil {
		owner = e.self.Type
	}
	for _, member := range e.ctx.model.semantics.MembersOf(owner) {
		if member.Name != operation || !isActionSymbol(member) {
			continue
		}
		_, out := parameterNames(e.ctx.actionParametersOf(member))
		returns := make(map[string]bool, len(out))
		for _, name := range out {
			returns[name] = true
		}
		return returns
	}
	return nil
}

// recordCallOutput keeps an output a behavior returned to the machine while the
// pending call's event is being dispatched, for Call to release the caller with;
// a name the declared operation does not return stays the machine's own.
func (e *StateExecutor) recordCallOutput(name string, value Value) {
	call := e.pendingCall
	if call == nil || e.firingEvent == nil || e.firingEvent.ID != call.id {
		return
	}
	if call.returns != nil && !call.returns[name] {
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
