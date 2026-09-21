package runtime

import (
	"errors"
	"fmt"
	"maps"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
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
		payload, err := e.callPayload(event.Call, event.Args)
		if err != nil {
			return err
		}
		e.queueCall(payload)
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
// machine has returned to its caller so far, the names the operation's
// declaration lets out (nil when the machine's owner declares no such operation)
// and the inout arguments a taken call returns as they are until a behavior
// writes them.
type pendingCall struct {
	id      int64
	outputs map[string]Value
	returns map[string]bool
	inouts  map[string]Value
	taken   bool
}

// clone copies the call where it stands, its maps by value, for a capture.
func (call *pendingCall) clone() *pendingCall {
	if call == nil {
		return nil
	}
	copied := *call
	copied.outputs, copied.returns, copied.inouts = maps.Clone(call.outputs), maps.Clone(call.returns), maps.Clone(call.inouts)
	return &copied
}

// Call queues the call event, runs the machine through the run-to-completion
// step dispatching it and releases the caller with the outputs the behaviors that
// step fired returned, by name (PSSM 8.5.9): the operation's out and inout
// parameters when the machine's owner declares it, an inout unwritten by the step
// as the caller passed it, every output otherwise. Events that step queued and
// timers it armed stay for the machine's later runs; the clock does not move.
func (e *StateExecutor) Call(operation string, args map[string]Value) (map[string]Value, error) {
	payload, err := e.callPayload(operation, args)
	if err != nil {
		return nil, err
	}
	call, err := e.newPendingCall(payload)
	if err != nil {
		return nil, err
	}
	e.pendingCall = call
	defer func() { e.pendingCall = nil }()
	e.queueCall(payload)
	if err := e.RunToQuiescence(); err != nil {
		return nil, err
	}
	if held := e.eventDisposition(call.id); held != "" {
		return nil, fmt.Errorf("%w: %s is still %s", ErrCallNotReturned, operation, held)
	}
	// A step undone within the run restored the call as it stood when captured.
	if e.pendingCall != nil {
		call = e.pendingCall
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

// callReleased reports whether the pending call's event has been dispatched, so
// its run-to-completion step is done; a deferred call still holds its caller.
func (e *StateExecutor) callReleased() bool {
	return e.pendingCall != nil && e.eventDisposition(e.pendingCall.id) == ""
}

// callOwner is the type whose members a called operation is looked up among: the
// machine's owner, the machine itself when it stands alone.
func (e *StateExecutor) callOwner() *symbols.Symbol {
	if e.self != nil && e.self.Type != nil {
		return e.self.Type
	}
	return e.stateMachine
}

// callTriggerOperations reads the declared operations a call trigger names, as a
// UML call event names one: the owner's behavior members of the trigger's name
// declaring an input for each trigger parameter — those declaring exactly the
// trigger's parameters when any does — memoized per trigger.
func (e *StateExecutor) callTriggerOperations(trigger *ast.CallEvent) []*symbols.Symbol {
	if named, memoized := e.callTriggers[trigger]; memoized {
		return named
	}
	name := ast.SimpleName(trigger.Operation)
	var loose, exact []*symbols.Symbol
	for _, member := range e.ctx.model.semantics.MembersOf(e.callOwner()) {
		if member.Name != name || !isActionSymbol(member) && !isCalcSymbol(member) && !isConstraintSymbol(member) {
			continue
		}
		inputs := make(map[string]bool)
		for _, param := range e.ctx.model.semantics.SignatureParametersOf(member) {
			inputs[param.Name] = true
		}
		declared := 0
		for _, param := range trigger.Parameters {
			if inputs[param.Text] {
				declared++
			}
		}
		if declared < len(trigger.Parameters) {
			continue
		}
		loose = append(loose, member)
		if len(inputs) == len(trigger.Parameters) {
			exact = append(exact, member)
		}
	}
	named := loose
	if len(exact) > 0 {
		named = exact
	}
	if e.callTriggers == nil {
		e.callTriggers = make(map[*ast.CallEvent][]*symbols.Symbol)
	}
	e.callTriggers[trigger] = named
	return named
}

// callPayload builds the call event's payload: the operation's declaration as a
// behavior member of the machine's owner — among several so named, the one the
// arguments select as a call in the model would — and the arguments bound to its
// inputs as an invocation binds them; an operation no member of that name
// declares as a behavior carries the arguments as given and no declaration.
func (e *StateExecutor) callPayload(operation string, args map[string]Value) (Call, error) {
	member, err := e.ctx.memberCalled(e.callOwner(), e.self, operation, OperationArguments{Named: args})
	if err != nil {
		return Call{}, err
	}
	if !isActionSymbol(member) && !isCalcSymbol(member) && !isConstraintSymbol(member) {
		return Call{Operation: operation, Args: args}, nil
	}
	inputs, err := e.callInputs(member, operation, args)
	if err != nil {
		return Call{}, err
	}
	return Call{Operation: operation, Declared: member, Args: inputs}, nil
}

// newPendingCall takes the names the call returns: an action's out and inout
// parameters, with the inout values the call carries in, a calc's or constraint's
// result; a call of no declared operation returns every output.
func (e *StateExecutor) newPendingCall(payload Call) (*pendingCall, error) {
	call := &pendingCall{id: e.nextEventID, outputs: make(map[string]Value)}
	member, inputs := payload.Declared, payload.Args
	if member == nil {
		return call, nil
	}
	call.returns = make(map[string]bool)
	switch {
	case isActionSymbol(member):
		for _, param := range e.ctx.actionParametersOf(member) {
			switch param.Direction {
			case ast.DirOut:
				call.returns[param.Name] = true
			case ast.DirInOut:
				call.returns[param.Name] = true
				if value, ok := inputs[param.Name]; ok {
					if call.inouts == nil {
						call.inouts = make(map[string]Value)
					}
					call.inouts[param.Name] = value
				}
			}
		}
	case isCalcSymbol(member):
		shape, err := e.ctx.calcShapeOf(member)
		if err != nil {
			return nil, err
		}
		call.returns[shape.resultName()] = true
	default:
		call.returns["result"] = true
	}
	return call, nil
}

// callInputs binds the arguments to the operation's inputs as InvokeOperation
// binds them — by name, each value checked against its parameter's declaration, a
// parameter no argument binds holding its default — so the event carries a value
// for each parameter, as PSSM 8.5.9's call event execution does.
func (e *StateExecutor) callInputs(member *symbols.Symbol, operation string, args map[string]Value) (map[string]Value, error) {
	ctx := e.ctx
	inputs, err := operationInputs(ctx.model.semantics.SignatureParametersOf(member), operation, OperationArguments{Named: args})
	if err != nil {
		return nil, err
	}
	scope := DeclScope(member)
	ec := NewEvalContextIn(ctx, scope, e.self)
	defer ec.beginStep()()
	if isCalcSymbol(member) {
		shape, err := ctx.calcShapeOf(member)
		if err != nil {
			return nil, err
		}
		bound := mapFrame(make(map[string]Value, len(shape.Params)))
		ec.frames = []frame{bound}
		if err := ctx.bindCalcParameters(shape, ec, calcArgs{named: inputs}, scope, bound, nil); err != nil {
			return nil, err
		}
		return bound.vars, nil
	}
	ec.frames = []frame{mapFrame(inputs)}
	where := "call " + operation
	for _, param := range ctx.model.semantics.BehaviorParametersOf(member) {
		if param.Symbol == nil || param.Direction != ast.DirIn && param.Direction != ast.DirInOut {
			continue
		}
		name := param.Symbol.Name
		value, bound := inputs[name]
		if !bound {
			expr, declared := ctx.model.semantics.ParameterDefault(param.Symbol)
			if expr == nil {
				continue
			}
			if value, err = ec.evalIn(declared).Eval(expr); err != nil {
				return nil, fmt.Errorf("%s: eval default of %s: %w", where, name, err)
			}
		}
		if err := ctx.checkNamedWrite(scope, where, name, &value); err != nil {
			return nil, err
		}
		inputs[name] = value
	}
	return inputs, nil
}

// callTaken notes that a transition fired on the pending call's event, so an inout
// argument no behavior of the step wrote goes back to the caller as it came.
func (e *StateExecutor) callTaken(event *Event) {
	call := e.pendingCall
	if call == nil || call.taken || event == nil || event.ID != call.id {
		return
	}
	call.taken = true
	for name, value := range call.inouts {
		if _, written := call.outputs[name]; !written {
			call.outputs[name] = value
		}
	}
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
