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
	// errLifelineUnbound is a lifeline, or the usage of a nested interaction, holding no object.
	errLifelineUnbound = errors.New("holds no object")
	// errNestedRecursion is an interaction using itself, directly or through another.
	errNestedRecursion = errors.New("the interaction uses itself")
	// errUnboundPayload is a message whose payload cannot be built from its `of` clause.
	errUnboundPayload = errors.New("message payload is unbound")
	// errTransferLost is a transfer posted onto the bus that could not be taken off it again.
	errTransferLost = errors.New("transfer was not delivered")
)

// ExecuteOccurrence performs an occurrence definition or usage as the interaction
// its body states (SysML v2.1 §7.16.1: a message "may be realized … using
// transfers from send actions to accept actions"): the occurrence is materialized,
// and its nodes are visited in the order the events admit. At an event the
// messages reaching it are taken off the bus by the lifeline object holding it,
// the actions the event performs are performed once on that object, the
// constraints it asserts are checked there, and the messages leaving it are
// posted, typed by their payloads, to the objects holding their target events;
// at an occurrence usage of a nested interaction, that interaction is performed
// on the object the usage holds. The trace records each transfer as a send and
// an accept.
func (ctx *Context) ExecuteOccurrence(sym *symbols.Symbol) (*OccurrenceRun, error) {
	graph, err := lower.ToSequenceGraph(sym, ctx.Resolver())
	if err != nil {
		return nil, err
	}
	defer ctx.beginRun()()
	inst, err := ctx.Instantiate(sym)
	if err != nil {
		return nil, fmt.Errorf("materialize %s: %w", SymbolText(sym), err)
	}
	run := &OccurrenceRun{Occurrence: inst}
	if err := ctx.runSequence(sym, inst, graph, run, nil, map[*symbols.Symbol]bool{sym: true}); err != nil {
		return run, err
	}
	return run, nil
}

// runSequence realizes a lowered interaction on the object holding its lifelines.
// outer holds the lifeline objects of the interaction using this one by name: a
// lifeline the use leaves unbound is the outer lifeline of its name, as an
// interaction use covers the lifelines of the interaction using it. active holds
// the occurrences being performed, so a use of one of them is refused rather than
// recursed into.
func (ctx *Context) runSequence(sym *symbols.Symbol, inst *Instance, graph *lower.SequenceGraph, run *OccurrenceRun, outer map[string]*Instance, active map[*symbols.Symbol]bool) error {
	objects := make(map[*symbols.Symbol]*Instance, len(graph.Lifelines))
	byName := make(map[string]*Instance, len(graph.Lifelines))
	for _, line := range graph.Lifelines {
		if o := outer[line.Name]; o != nil && !declaresMember(graph.Scope, line.Name) {
			// The use leaves the lifeline unbound: it is the outer lifeline of its name.
			if err := inst.BindFeatureValue(ctx, line.Name, Value{Kind: ValInstance, Instance: o.ID}); err != nil {
				return fmt.Errorf("lifeline %s %w: %w", line.Name, errLifelineUnbound, err)
			}
		}
		obj, err := ctx.heldUsageObject(inst, line, "lifeline", outer[line.Name])
		if err != nil {
			return err
		}
		objects[line] = obj
		byName[line.Name] = obj
	}
	posted := make(map[*lower.Transfer]uint64, len(graph.Transfers))
	post := func(t *lower.Transfer) error {
		serial, err := ctx.postTransfer(sym, inst, t, objects[t.From.Lifeline], objects[t.To.Lifeline])
		if err != nil {
			return err
		}
		posted[t] = serial
		return nil
	}
	for _, ev := range graph.Order {
		if ev.Use {
			if err := ctx.performUse(inst, ev.Symbol, run, byName, active); err != nil {
				return err
			}
			continue
		}
		obj := objects[ev.Lifeline]
		// A message an event sends to itself is posted before the event takes it.
		for _, t := range ev.Sends {
			if t.To == ev {
				if err := post(t); err != nil {
					return err
				}
			}
		}
		first := len(run.Transfers)
		for _, t := range ev.Receives {
			performed, err := ctx.takeTransfer(sym, t, posted[t], objects[t.From.Lifeline], obj)
			if err != nil {
				return err
			}
			run.Transfers = append(run.Transfers, performed)
		}
		for _, perform := range ev.Performs {
			exec, err := ctx.performAtEvent(inst, ev, perform, obj)
			if err != nil {
				return err
			}
			for i := first; i < len(run.Transfers); i++ {
				if run.Transfers[i].Performed == nil {
					run.Transfers[i].Performed = exec
				}
			}
		}
		for _, a := range ev.Asserts {
			if err := ctx.assertAtEvent(ev, a, obj); err != nil {
				return err
			}
		}
		for _, t := range ev.Sends {
			if t.To != ev {
				if err := post(t); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// performUse performs a nested interaction at its place in the order: the object
// the occurrence usage holds runs the interaction its type states, with its
// lifelines bound as the usage binds them.
func (ctx *Context) performUse(inst *Instance, use *symbols.Symbol, run *OccurrenceRun, outer map[string]*Instance, active map[*symbols.Symbol]bool) error {
	if active[use] {
		return fmt.Errorf("%w: %s", errNestedRecursion, SymbolText(use))
	}
	graph, err := lower.ToSequenceGraph(use, ctx.Resolver())
	if err != nil {
		return fmt.Errorf("occurrence %s: %w", use.Name, err)
	}
	obj, err := ctx.heldUsageObject(inst, use, "occurrence", nil)
	if err != nil {
		return err
	}
	if ctx.trace != nil {
		ctx.trace.line("perform occurrence " + use.Name)
	}
	active[use] = true
	defer delete(active, use)
	return ctx.runSequence(use, obj, graph, run, outer, active)
}

// performAtEvent performs an action the event states, on the lifeline object: the
// event is on the lifeline's timeline, so the action reads and writes that
// object's features. On an object of the lifeline usage, the event occurrence it
// holds holds the performance (`m.spinReceived.spinCall.result` reads its
// result); an object standing for the lifeline without being its usage's own (an
// outer interaction's object a use covers) holds nothing of the event. The `in`
// bindings are evaluated on the interaction as arguments.
func (ctx *Context) performAtEvent(inst *Instance, ev *lower.SequenceEvent, perform *symbols.Symbol, obj *Instance) (*ActionExecutor, error) {
	fail := func(err error) (*ActionExecutor, error) {
		return nil, fmt.Errorf("%s performs %s: %w", ev.Text(), SymbolText(perform), err)
	}
	var exec *ActionExecutor
	if objectOf(obj, ev.Lifeline) {
		event, err := ctx.heldUsageObject(obj, ev.Symbol, "event", nil)
		if err != nil {
			return nil, err
		}
		var decl classifierBehaviorDecl
		for _, candidate := range ctx.classifierBehaviorsOf(ev.Symbol) {
			if candidate.member == perform {
				decl = candidate
			}
		}
		if decl.member == nil {
			return fail(errUnsupportedClassifierBehavior)
		}
		ctx.attachBehavior(event, decl.member)
		behavior, err := ctx.attachClassifierBehaviorWith(event, obj, decl, 0, func() (map[string]Value, error) {
			return ctx.classifierBehaviorArguments(inst, decl)
		})
		ctx.behaviorAttached(event, decl.member)
		if err != nil {
			return fail(err)
		}
		exec = behavior.Action
		if err := ctx.runPerformance(exec, ctx.runDepth == 0); err != nil {
			return fail(err)
		}
	} else {
		inputs, err := ctx.performInputs(perform, inst)
		if err != nil {
			return fail(err)
		}
		if exec, err = ctx.performAction(perform, obj, inputs); err != nil {
			return fail(err)
		}
	}
	if ctx.trace != nil {
		ctx.trace.line(fmt.Sprintf("perform %s at %s", SymbolText(perform), ev.Text()))
	}
	return exec, nil
}

// objectOf reports whether the object is one of the usage's own.
func objectOf(obj *Instance, usage *symbols.Symbol) bool {
	for _, typ := range obj.types() {
		if typ == usage {
			return true
		}
	}
	return false
}

// declaresMember reports whether a body declares a member of the name.
func declaresMember(scope *symbols.Scope, name string) bool {
	if scope == nil {
		return false
	}
	for _, member := range scope.Members() {
		if member.Name == name {
			return true
		}
	}
	return false
}

// assertAtEvent checks a constraint the event asserts holds on the lifeline object
// holding the event, whose features the constraint's names read; a violation is
// the run's error.
func (ctx *Context) assertAtEvent(ev *lower.SequenceEvent, a lower.Assert, obj *Instance) error {
	if a.Sym == nil {
		return fmt.Errorf("%w: %s: the assertion is not declared where it was written", ErrNoConditions, ev.Text())
	}
	check := conditionCheck{sym: a.Sym, kind: "constraint", what: "assertion", self: obj, negated: a.Node.IsNegated}
	if _, err := ctx.evaluateConditions(check, ctx.conditionsOf(a.Sym, ctx.chainMembers(a.Sym, a.Scope))); err != nil {
		return fmt.Errorf("%s: %w", ev.Text(), err)
	}
	if ctx.trace != nil {
		ctx.trace.line("assert at " + ev.Text())
	}
	return nil
}

// heldUsageObject is the object a usage of the occurrence holds: a lifeline, or the
// occurrence usage of a nested interaction. One holding nothing is bound to the
// outer object given, else to a fresh object of its type.
func (ctx *Context) heldUsageObject(inst *Instance, usage *symbols.Symbol, what string, outer *Instance) (*Instance, error) {
	fv, err := inst.GetFeatureValue(ctx, usage.Name)
	if err != nil {
		return nil, fmt.Errorf("%s %s %w: %w", what, usage.Name, errLifelineUnbound, err)
	}
	value := fv.Value
	if _, ok := value.Object(); !ok {
		if held := heldElements(fv.Values); len(held) > 0 {
			value = held[0]
		}
	}
	id, ok := value.Object()
	if !ok && outer != nil {
		if err := inst.BindFeatureValue(ctx, usage.Name, Value{Kind: ValInstance, Instance: outer.ID}); err != nil {
			return nil, fmt.Errorf("%s %s %w: %w", what, usage.Name, errLifelineUnbound, err)
		}
		return outer, nil
	}
	if !ok {
		// A referenced lifeline the occurrence's usages bind (`ref part :>> c = ctrl`)
		// holds nothing in a run of the def alone: it stands for an object of its type.
		return ctx.standIn(inst, usage, fv, what)
	}
	obj, ok := ctx.Instance(id)
	if !ok {
		return nil, fmt.Errorf("%s %s %w: object #%d is not in the context", what, usage.Name, errLifelineUnbound, id)
	}
	return obj, nil
}

// standIn materializes the object a referenced usage stands for in a run of the
// occurrence alone, and binds the usage to it.
func (ctx *Context) standIn(inst *Instance, usage *symbols.Symbol, fv *FeatureValue, what string) (*Instance, error) {
	typ := usage
	if fv.Feature != nil && fv.Feature.Type != nil {
		typ = fv.Feature.Type
	}
	obj, err := ctx.Instantiate(typ)
	if err != nil {
		return nil, fmt.Errorf("%s %s %w: %w", what, usage.Name, errLifelineUnbound, err)
	}
	if err := inst.BindFeatureValue(ctx, usage.Name, Value{Kind: ValInstance, Instance: obj.ID}); err != nil {
		return nil, fmt.Errorf("%s %s %w: %w", what, usage.Name, errLifelineUnbound, err)
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
		ctx.trace.line(fmt.Sprintf("send %s from %s", t.Name, t.From.Text()))
	}
	return ctx.messageSerial, nil
}

// takeTransfer realizes the receiving half of a transfer at its target event: the
// message is taken off the bus by the target lifeline object.
func (ctx *Context) takeTransfer(sym *symbols.Symbol, t *lower.Transfer, serial uint64, from, to *Instance) (PerformedTransfer, error) {
	performed := PerformedTransfer{Name: t.Name, From: from, To: to, FromEvent: t.From.Text(), ToEvent: t.To.Text()}
	taken, ok := ctx.takeMessage(func(m Message) bool { return m.Serial == serial })
	if !ok {
		return performed, fmt.Errorf("%w: message %s", errTransferLost, t.Name)
	}
	performed.Message = taken
	if ctx.trace != nil {
		ctx.trace.RecordAccept(TraceOrigin{At: ctx.clock.now, Object: to, Behavior: sym}, taken.Serial, acceptedEventName(taken), taken.Payload)
		ctx.trace.line(fmt.Sprintf("accept %s at %s", t.Name, performed.ToEvent))
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
