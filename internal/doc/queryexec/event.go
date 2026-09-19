package queryexec

import (
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// Event is one row of Events: a typed record of the session's trace, at the
// instant it was made. Immutable once built.
type Event struct {
	record runtime.TraceRecord
	at     float64
	// time is the instant as the clock reports it: a quantity in the clock's
	// unit when the library defines one, else a bare real.
	time   Value
	object *runtime.Instance
	label  string
	// machine names the behavior as the object exhibits it, else as declared.
	machine string
	// target labels the object a send was addressed to, "" for none.
	target string
	index  int
}

// Kind names the record's kind as its `kind` property does: `accept`, `send`,
// `transition`, `entry`, `exit`, `do`, `choice`, `guard`.
func (ev Event) Kind() string { return ev.record.Kind.String() }

// At is the clock instant the record was made at, in clock units.
func (ev Event) At() float64 { return ev.at }

// Time is the instant as its `time` property answers it.
func (ev Event) Time() Value { return ev.time }

// Object is the object whose behavior made the record, nil for one made from
// outside the run, with the label a session reaches it by.
func (ev Event) Object() (*runtime.Instance, string) { return ev.object, ev.label }

// Target is the object a send was addressed to, nil for any other record,
// with the label a session reaches it by.
func (ev Event) Target() (*runtime.Instance, string) { return ev.record.Target, ev.target }

// Behavior is the state machine or action the record was made under.
func (ev Event) Behavior() *symbols.Symbol { return ev.record.Origin.Behavior }

// Machine names the behavior as its `machine` property does, "" for none.
func (ev Event) Machine() string { return ev.machine }

// Payload is an accept's or send's payload, `name = value` per parameter in
// name order, in the runtime's notation.
func (ev Event) Payload() []string { return payloadTexts(ev.record) }

// Alternatives are a choice's alternatives as offered, nil for any other record.
func (ev Event) Alternatives() []string {
	if choice, ok := ev.record.Note.(runtime.ChoicePoint); ok {
		return append([]string(nil), choice.Alternatives...)
	}
	return nil
}

// Taken is the alternative a choice took, "" for any other record.
func (ev Event) Taken() string {
	choice, ok := ev.record.Note.(runtime.ChoicePoint)
	if !ok || choice.Taken < 0 || choice.Taken >= len(choice.Alternatives) {
		return ""
	}
	return choice.Alternatives[choice.Taken]
}

// Record is the trace record the row reads.
func (ev Event) Record() runtime.TraceRecord { return ev.record }

// Text is the record's trace text.
func (ev Event) Text() string { return ev.record.Text() }

// Label names the row: the machine it came from and its text (`lamp.lp: enter: on`);
// a record with no object reads as its text alone.
func (ev Event) Label() string {
	if ev.label == "" {
		return ev.Text()
	}
	if ev.machine == "" {
		return ev.label + ": " + ev.Text()
	}
	return ev.label + "." + ev.machine + ": " + ev.Text()
}

// Summary is the event in one line: its instant in clock units, then its label.
func (ev Event) Summary() string {
	return "t=" + strconv.FormatFloat(ev.at, 'g', -1, 64) + " " + ev.Label()
}

// EventValue constructs an event value; its provenance is the object's declaration.
func EventValue(event Event) Value {
	value := Value{kind: ValueEvent, event: &event}
	if event.object != nil {
		value.origin = objectDeclaration(event.object).Origin()
	} else if event.record.Origin.Behavior != nil {
		value.origin = event.record.Origin.Behavior.Origin()
	}
	return value
}

// Event returns the value's event and whether it is an event value.
func (v Value) Event() (Event, bool) {
	if v.kind != ValueEvent || v.event == nil {
		return Event{}, false
	}
	return *v.event, true
}
