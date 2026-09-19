package queryexec

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/ir/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/query"
)

// Event rows read the session's trace as a relation: one row per typed record
// an object's behavior made, in the order the run made them.

// eventFQN declares the event properties a column expression may reference.
const eventFQN = "DocumentQueries::Event"

// Event properties, beside the metadata every row answers.
const (
	propertyTime         = "time"
	propertyFrom         = "from"
	propertyTo           = "to"
	propertyTarget       = "target"
	propertyEvent        = "event"
	propertyPayload      = "payload"
	propertyAlternatives = "alternatives"
	propertyTaken        = "taken"
	propertyText         = "text"
)

// eventKinds are the record kinds Events reports, in the `kind` argument's vocabulary.
var eventKinds = map[string]runtime.TraceKind{
	"accept":     runtime.TraceAccept,
	"send":       runtime.TraceSend,
	"transition": runtime.TraceTransition,
	"entry":      runtime.TraceEntry,
	"exit":       runtime.TraceExit,
	"do":         runtime.TraceDo,
	"choice":     runtime.TraceChoice,
	"guard":      runtime.TraceGuard,
}

// evaluateEvents lists the trace's records of the requested kinds, made by the
// source objects (any object when unbound) at an instant in [since, before).
func (e *executor) evaluateEvents(expression queryplan.Expression) (sequence, error) {
	if err := e.requireRuntime(expression); err != nil {
		return sequence{}, err
	}
	trace := e.context.Runtime.Trace()
	if trace == nil {
		return sequence{}, &Error{
			Kind:      ErrorNoTrace,
			Query:     e.definition.Name(),
			Operation: expression.Operation(),
			Origin:    expression.Origin(),
		}
	}
	kinds, err := e.eventKindArgument(expression)
	if err != nil {
		return sequence{}, err
	}
	from, hasFrom, err := e.instantArgument(expression, "since")
	if err != nil {
		return sequence{}, err
	}
	to, hasTo, err := e.instantArgument(expression, "before")
	if err != nil {
		return sequence{}, err
	}
	if hasFrom && hasTo && to <= from {
		return sequence{}, &Error{
			Kind:      ErrorInvalidInterval,
			Query:     e.definition.Name(),
			Operation: expression.Operation(),
			Actual:    fmt.Sprintf("before = %s is not after since = %s", formatInstant(to), formatInstant(from)),
			Origin:    expression.Origin(),
		}
	}
	if dropped, upTo := trace.Dropped(); dropped > 0 && (!hasFrom || from <= upTo) {
		return sequence{}, &Error{
			Kind:      ErrorTraceTruncated,
			Query:     e.definition.Name(),
			Operation: expression.Operation(),
			Actual:    fmt.Sprintf("%d records up to t = %s dropped", dropped, formatInstant(upTo)),
			Origin:    expression.Origin(),
		}
	}
	var only map[int64]struct{}
	if hasArgument(expression, "source") {
		source, err := e.objectArgument(expression, "source")
		if err != nil {
			return sequence{}, err
		}
		only = make(map[int64]struct{}, len(source.values))
		for _, row := range source.values {
			inst, _, _ := row.Object()
			only[inst.ID] = struct{}{}
		}
	}
	labels, err := e.sessionLabels(expression)
	if err != nil {
		return sequence{}, err
	}
	var result sequence
	for index, record := range trace.Records() {
		if _, wanted := kinds[record.Kind]; !wanted {
			continue
		}
		at := record.Origin.At
		if (hasFrom && at < from) || (hasTo && at >= to) {
			continue
		}
		object := record.Origin.Object
		if only != nil {
			if object == nil {
				continue
			}
			if _, wanted := only[object.ID]; !wanted {
				continue
			}
		}
		if !e.consumeVisit() {
			return sequence{}, e.budgetError(expression)
		}
		result.values = append(result.values, EventValue(Event{
			record:  record,
			at:      at,
			time:    instantValue(e.context.Runtime, at),
			object:  object,
			label:   labels.label(object),
			machine: eventMachine(record),
			target:  labels.label(record.Target),
			index:   index,
		}))
	}
	return result, nil
}

// instantValue is instant t as `time` answers it: a quantity in the clock's
// unit when the library defines one, else a bare real of clock units.
func instantValue(ctx *runtime.Context, t float64) Value {
	value := ctx.InstantValue(t)
	if quantity := value.Quantity(); quantity != nil {
		return QuantityValue(*quantity)
	}
	return RealValue(t)
}

// eventKindArgument reads `kind`: `all` (the default), or record kinds
// separated by `,`.
func (e *executor) eventKindArgument(expression queryplan.Expression) (map[runtime.TraceKind]struct{}, error) {
	text := "all"
	if hasArgument(expression, "kind") {
		var err error
		if text, err = e.stringArgument(expression, "kind"); err != nil {
			return nil, err
		}
	}
	kinds := make(map[runtime.TraceKind]struct{})
	for _, part := range strings.Split(text, ",") {
		part = strings.TrimSpace(part)
		if part == "all" {
			for _, kind := range eventKinds {
				kinds[kind] = struct{}{}
			}
			continue
		}
		kind, ok := eventKinds[part]
		if !ok {
			return nil, e.invalidArgument(expression, "kind", text)
		}
		kinds[kind] = struct{}{}
	}
	return kinds, nil
}

// instantArgument reads a bound of the interval as clock units: a bare number
// is one already, a quantity converts through the clock's unit. Unbound is open.
func (e *executor) instantArgument(expression queryplan.Expression, name string) (float64, bool, error) {
	if !hasArgument(expression, name) {
		return 0, false, nil
	}
	value, err := e.argument(expression, name)
	if err != nil {
		return 0, false, err
	}
	if len(value.values) == 0 {
		return 0, false, nil
	}
	if len(value.values) > 1 {
		return 0, false, e.invalidArgument(expression, name, fmt.Sprintf("%d values", len(value.values)))
	}
	bound := value.values[0]
	var instant float64
	switch {
	case bound.kind == ValueInteger:
		instant = float64(bound.integer)
	case bound.kind == ValueReal:
		instant = bound.real
	case bound.kind == ValueQuantity:
		quantity := bound.quantity.Clone()
		magnitude, err := e.context.Runtime.ClockMagnitude(runtime.NewQuantityValue(&quantity), name)
		if err != nil {
			return 0, false, &Error{
				Kind:      ErrorInvalidInterval,
				Query:     e.definition.Name(),
				Operation: expression.Operation(),
				Parameter: name,
				Actual:    quantity.String(),
				Cause:     err,
				Origin:    expression.Origin(),
			}
		}
		instant = magnitude
	default:
		return 0, false, e.invalidArgument(expression, name, string(bound.kind))
	}
	if math.IsNaN(instant) || math.IsInf(instant, 0) {
		return 0, false, e.invalidArgument(expression, name, formatInstant(instant))
	}
	return instant, true, nil
}

func formatInstant(t float64) string {
	return strconv.FormatFloat(t, 'g', -1, 64)
}

// sessionLabelMap names the session's objects by the labels Objects reaches them
// under; an object the roots do not reach is named by its identity.
type sessionLabelMap map[int64]string

func (l sessionLabelMap) label(inst *runtime.Instance) string {
	if inst == nil {
		return ""
	}
	if label, ok := l[inst.ID]; ok {
		return label
	}
	return "#" + strconv.FormatInt(inst.ID, 10)
}

// sessionLabels walks the session's objects once for their labels.
func (e *executor) sessionLabels(expression queryplan.Expression) (sessionLabelMap, error) {
	labels := make(sessionLabelMap)
	err := e.eachSessionObject(expression, func(row Value) {
		inst, label, _ := row.Object()
		labels[inst.ID] = label
	})
	return labels, err
}

// eventPropertyValues reads a property of an event row: the record's fields,
// then the metadata of the behavior that made it.
func (e *executor) eventPropertyValues(row Value, property string) ([]Value, bool, error) {
	event, _ := row.Event()
	record := event.record
	origin := row.Origin()
	text := func(value string) []Value {
		if value == "" {
			return nil
		}
		return []Value{valueAt(StringValue(value), origin)}
	}
	switch property {
	case propertyKind:
		return text(event.Kind()), true, nil
	case propertyTime:
		return []Value{valueAt(event.time, origin)}, true, nil
	case propertyObject:
		if event.object == nil {
			return nil, true, nil
		}
		return []Value{valueAt(ObjectValue(event.object, event.label), origin)}, true, nil
	case propertyPath:
		return text(event.label), true, nil
	case propertyMachine:
		return text(event.machine), true, nil
	case propertyState:
		return text(record.State), true, nil
	case propertyFrom:
		return text(record.From), true, nil
	case propertyTo:
		return text(record.To), true, nil
	case propertyTarget:
		if record.Target == nil {
			return nil, true, nil
		}
		return []Value{valueAt(ObjectValue(record.Target, event.target), origin)}, true, nil
	case propertyEvent:
		return text(record.Event), true, nil
	case query.PropertyName:
		return text(eventName(record)), true, nil
	case propertyPayload:
		out := make([]Value, 0, len(record.Payload))
		for _, cell := range payloadTexts(record) {
			out = append(out, valueAt(StringValue(cell), origin))
		}
		return out, true, nil
	case propertyAlternatives:
		alternatives := event.Alternatives()
		out := make([]Value, 0, len(alternatives))
		for _, alt := range alternatives {
			out = append(out, valueAt(StringValue(alt), origin))
		}
		return out, true, nil
	case propertyTaken:
		return text(event.Taken()), true, nil
	case propertyText:
		return text(record.Text()), true, nil
	}
	if record.Origin.Behavior == nil {
		return nil, false, nil
	}
	return e.propertyValues(ElementValue(record.Origin.Behavior), property)
}

// eventMachine names the behavior a record came from as its object exhibits it,
// or as declared when it is anonymous or the record has no object.
func eventMachine(record runtime.TraceRecord) string {
	behavior := record.Origin.Behavior
	if behavior == nil {
		return ""
	}
	if record.Origin.Object != nil {
		for _, b := range record.Origin.Object.Behaviors() {
			if b.Symbol == behavior || (b.State != nil && b.State.StateMachineSymbol() == behavior) {
				if b.Name != "" {
					return b.Name
				}
				break
			}
		}
	}
	return behavior.Name
}

// eventName is what a record is about: the event accepted or sent, the state
// entered, exited or run, a transition's target, the choice kind or the guarded alternative.
func eventName(record runtime.TraceRecord) string {
	switch record.Kind {
	case runtime.TraceAccept, runtime.TraceSend:
		return record.Event
	case runtime.TraceTransition:
		return record.To
	case runtime.TraceEntry, runtime.TraceExit, runtime.TraceDo:
		return record.State
	case runtime.TraceChoice:
		if choice, ok := record.Note.(runtime.ChoicePoint); ok {
			return choice.Kind.String()
		}
	case runtime.TraceGuard:
		if guard, ok := record.Note.(runtime.UnevaluableGuard); ok {
			return guard.Alternative
		}
	}
	return ""
}

// payloadTexts renders an accept's or send's payload, one `name = value`
// entry per parameter in name order, in the runtime's notation.
func payloadTexts(record runtime.TraceRecord) []string {
	names := make([]string, 0, len(record.Payload))
	for name := range record.Payload {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, name+" = "+runtime.FormatValue(record.Payload[name]))
	}
	return out
}
