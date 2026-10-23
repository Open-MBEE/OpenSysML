package runtime

import (
	"errors"
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// OccurrenceRun is what performing an occurrence definition as an interaction
// did: the object the occurrence materialized to, whose parts are the lifelines,
// and the transfers its messages were realized as, in the order performed.
type OccurrenceRun struct {
	Occurrence *Instance
	Transfers  []PerformedTransfer
}

// PerformedTransfer is one message realized: posted from the lifeline object
// holding the event it leaves to the one holding the event it reaches, and the
// action the receiving event performed on it, nil for none.
type PerformedTransfer struct {
	Name      string
	From, To  *Instance
	FromEvent string
	ToEvent   string
	Message   Message
	Performed *ActionExecutor
}

var (
	// errLifelineUnbound is a lifeline whose feature value holds no object to send from or to.
	errLifelineUnbound = errors.New("lifeline holds no object")
	// errUnboundPayload is a message whose payload cannot be built from its `of` clause.
	errUnboundPayload = errors.New("message payload is unbound")
	// errTransferLost is a transfer posted onto the bus that could not be taken off it again.
	errTransferLost = errors.New("transfer was not delivered")
)

// ExecuteOccurrence performs an occurrence definition or usage as the interaction
// its body states (SysML v2.1 §7.16.1: a message "may be realized … using
// transfers from send actions to accept actions"): the occurrence is materialized,
// and each message, in the order the events admit, is posted as a message typed by
// its payload from the lifeline object holding its source event to the one holding
// its target event, taken there, and the action the receiving event performs is
// performed on that object. The trace records each transfer as a send and an accept.
func (ctx *Context) ExecuteOccurrence(sym *symbols.Symbol) (*OccurrenceRun, error) {
	graph, err := lower.ToSequenceGraph(sym.Decl, DeclScope(sym), ctx.Resolver())
	if err != nil {
		return nil, err
	}
	defer ctx.beginRun()()
	inst, err := ctx.Instantiate(sym)
	if err != nil {
		return nil, fmt.Errorf("materialize %s: %w", SymbolText(sym), err)
	}
	objects := make(map[*symbols.Symbol]*Instance, len(graph.Lifelines))
	for _, line := range graph.Lifelines {
		obj, err := ctx.lifelineObject(inst, line)
		if err != nil {
			return nil, err
		}
		objects[line] = obj
	}
	run := &OccurrenceRun{Occurrence: inst}
	posted := make(map[*lower.Transfer]uint64, len(graph.Transfers))
	for _, ev := range graph.Order {
		for _, t := range ev.Sends {
			serial, err := ctx.postTransfer(sym, inst, t, objects[t.From.Lifeline], objects[t.To.Lifeline])
			if err != nil {
				return run, err
			}
			posted[t] = serial
		}
		for _, t := range ev.Receives {
			performed, err := ctx.takeTransfer(sym, inst, t, posted[t], objects[t.From.Lifeline], objects[t.To.Lifeline])
			if err != nil {
				return run, err
			}
			run.Transfers = append(run.Transfers, performed)
		}
	}
	return run, nil
}

// eventText names an event as a lifeline's member.
func eventText(ev *lower.SequenceEvent) string {
	return ev.Lifeline.Name + "." + ev.Symbol.Name
}

// lifelineObject is the object a lifeline usage of the occurrence holds.
func (ctx *Context) lifelineObject(inst *Instance, line *symbols.Symbol) (*Instance, error) {
	fv, err := inst.GetFeatureValue(ctx, line.Name)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", errLifelineUnbound, line.Name, err)
	}
	value := fv.Value
	if _, ok := value.Object(); !ok {
		if held := heldElements(fv.Values); len(held) > 0 {
			value = held[0]
		}
	}
	id, ok := value.Object()
	if !ok {
		// A referenced lifeline the occurrence's usages bind (`ref part :>> c = ctrl`)
		// holds nothing in a run of the def alone: it stands for an object of its type.
		return ctx.standInLifeline(inst, line, fv)
	}
	obj, ok := ctx.Instance(id)
	if !ok {
		return nil, fmt.Errorf("%w: %s holds object #%d, which the context does not", errLifelineUnbound, line.Name, id)
	}
	return obj, nil
}

// standInLifeline materializes the object a referenced lifeline stands for in a
// run of the occurrence alone, and binds the lifeline to it.
func (ctx *Context) standInLifeline(inst *Instance, line *symbols.Symbol, fv *FeatureValue) (*Instance, error) {
	typ := line
	if fv.Feature != nil && fv.Feature.Type != nil {
		typ = fv.Feature.Type
	}
	obj, err := ctx.Instantiate(typ)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", errLifelineUnbound, line.Name, err)
	}
	if err := inst.BindFeatureValue(ctx, line.Name, Value{Kind: ValInstance, Instance: obj.ID}); err != nil {
		return nil, fmt.Errorf("%w: %s: %w", errLifelineUnbound, line.Name, err)
	}
	return obj, nil
}

// postTransfer realizes the sending half of a transfer at its source event: the message,
// typed by the payload, is posted onto the bus from the source lifeline object.
func (ctx *Context) postTransfer(sym *symbols.Symbol, inst *Instance, t *lower.Transfer, from, to *Instance) (uint64, error) {
	msg, err := ctx.transferMessage(sym, inst, t, to)
	if err != nil {
		return 0, err
	}
	ctx.postFrom(msg, from, sym)
	if ctx.trace != nil {
		ctx.trace.line(fmt.Sprintf("send %s from %s", t.Name, eventText(t.From)))
	}
	return ctx.messageSerial, nil
}

// takeTransfer realizes the receiving half of a transfer at its target event: the message
// is taken off the bus by the target lifeline object, which performs the action the
// event states on it.
func (ctx *Context) takeTransfer(sym *symbols.Symbol, inst *Instance, t *lower.Transfer, serial uint64, from, to *Instance) (PerformedTransfer, error) {
	performed := PerformedTransfer{Name: t.Name, From: from, To: to, FromEvent: eventText(t.From), ToEvent: eventText(t.To)}
	taken, ok := ctx.takeMessage(func(m Message) bool { return m.Serial == serial })
	if !ok {
		return performed, fmt.Errorf("%w: message %s", errTransferLost, t.Name)
	}
	performed.Message = taken
	if ctx.trace != nil {
		ctx.trace.RecordAccept(TraceOrigin{At: ctx.clock.now, Object: to, Behavior: sym}, taken.Serial, acceptedEventName(taken), taken.Payload)
		ctx.trace.line(fmt.Sprintf("accept %s at %s", t.Name, performed.ToEvent))
	}
	if t.To.Perform != nil {
		inputs, err := ctx.performInputs(t.To.Perform, inst)
		if err != nil {
			return performed, fmt.Errorf("message %s: %s performs %s: %w", t.Name, performed.ToEvent, SymbolText(t.To.Perform), err)
		}
		exec, err := ctx.performAction(t.To.Perform, to, inputs)
		if err != nil {
			return performed, fmt.Errorf("message %s: %s performs %s: %w", t.Name, performed.ToEvent, SymbolText(t.To.Perform), err)
		}
		performed.Performed = exec
	}
	return performed, nil
}

// performInputs are the arguments a receiving event's perform binds to its
// action's `in` parameters (`in rpm[1] = 30.0;`), evaluated on the occurrence.
func (ctx *Context) performInputs(perform *symbols.Symbol, inst *Instance) (map[string]Value, error) {
	if perform.Scope == nil {
		return nil, nil
	}
	inputs := map[string]Value{}
	for _, member := range perform.Scope.Members() {
		usage, ok := member.Decl.(*ast.Usage)
		if !ok || usage.Value == nil || usage.Direction != ast.DirIn && usage.Direction != ast.DirInOut {
			continue
		}
		value, err := ctx.EvalWithScopeOn(usage.Value, perform.Scope, inst)
		if err != nil {
			return nil, fmt.Errorf("%w: argument %s: %w", errUnboundPayload, member.Name, err)
		}
		inputs[member.Name] = value
	}
	return inputs, nil
}

// transferMessage builds the message a transfer posts: typed by the payload its
// `of` clause states (a declared payload's type, else the named type), named as
// the message is, and carrying the payload's value evaluated on the occurrence.
func (ctx *Context) transferMessage(sym *symbols.Symbol, inst *Instance, t *lower.Transfer, to *Instance) (Message, error) {
	msg := NamedSignalMessage(t.Name, to)
	msg.EventName = t.Name
	if t.Payload == nil || t.Payload.Payload == nil {
		return msg, nil
	}
	scope := t.Message.OwnerScope
	if t.Payload.PayloadDecl != nil {
		decl := t.Payload.PayloadDecl
		if typ := ctx.payloadType(t.Message, decl); typ != nil {
			msg.SignalType, msg.Signal = typ.Name, typ
		}
		if decl.Value != nil {
			value, err := ctx.EvalWithScopeOn(decl.Value, scope, inst)
			if err != nil {
				return msg, fmt.Errorf("%w: message %s: %w", errUnboundPayload, t.Name, err)
			}
			msg.Value = &value
			if obj, ok := ctx.payloadObject(value); ok {
				msg.Payload = ctx.payloadFeatures(obj)
				if msg.Signal == nil && obj.Type != nil {
					msg.SignalType, msg.Signal = obj.Type.Name, obj.Type
				}
			}
		}
		return msg, nil
	}
	if typ, ok := ctx.Resolver().ResolveTarget(scope, t.Payload.Payload); ok {
		msg.SignalType, msg.Signal = typ.Name, typ
	}
	return msg, nil
}

// payloadType is the definition a declared payload is typed by, nil for none.
func (ctx *Context) payloadType(message *symbols.Symbol, decl *ast.Usage) *symbols.Symbol {
	payload := message.Scope.MemberDeclaring(decl)
	if payload == nil {
		return nil
	}
	for _, feat := range ctx.FeaturesOf(message) {
		if feat.Symbol == payload {
			return feat.Type
		}
	}
	return nil
}

// payloadObject is the object a payload value holds, if it holds one.
func (ctx *Context) payloadObject(value Value) (*Instance, bool) {
	id, ok := value.Object()
	if !ok {
		return nil, false
	}
	return ctx.Instance(id)
}

// payloadFeatures are the feature values a payload object holds, by name, as a
// send's arguments bind them.
func (ctx *Context) payloadFeatures(obj *Instance) map[string]Value {
	out := map[string]Value{}
	for name, fv := range obj.FeatureValues {
		switch {
		case fv.Value.Kind != ValNull && fv.Value.Kind != ValInvalid:
			out[name] = fv.Value
		case fv.Values.Kind != ValNull && fv.Values.Kind != ValInvalid:
			out[name] = fv.Values
		}
	}
	return out
}
