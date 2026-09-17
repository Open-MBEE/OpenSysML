package queryexec

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/provenance"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Event is one row of Events: a typed record of the session's trace, at the
// instant it was made. Immutable once built.
type Event struct {
	record runtime.TraceRecord
	at     float64
	object *runtime.Instance
	label  string
	// target labels the object a send was addressed to, "" for none.
	target string
	index  int
}

// Kind names the record's kind as its `kind` property does: `accept`, `send`,
// `transition`, `entry`, `exit`, `do`, `choice`, `guard`.
func (ev Event) Kind() string { return ev.record.Kind.String() }

// At is the clock instant the record was made at, in clock units.
func (ev Event) At() float64 { return ev.at }

// Object is the object whose behavior made the record, nil for one made from
// outside the run, with the label a session reaches it by.
func (ev Event) Object() (*runtime.Instance, string) { return ev.object, ev.label }

// Target is the object a send was addressed to, nil for any other record,
// with the label a session reaches it by.
func (ev Event) Target() (*runtime.Instance, string) { return ev.record.Target, ev.target }

// Behavior is the state machine or action the record was made under.
func (ev Event) Behavior() *symbols.Symbol { return ev.record.Origin.Behavior }

// Record is the trace record the row reads.
func (ev Event) Record() runtime.TraceRecord { return ev.record }

// Text is the record's trace text.
func (ev Event) Text() string { return ev.record.Text() }

// Label names the row: its kind and text.
func (ev Event) Label() string {
	return ev.Kind() + " " + ev.Text()
}

// EventValue constructs an event value; its provenance is the object's declaration.
func EventValue(event Event) Value {
	value := Value{kind: ValueEvent, event: &event}
	if event.object != nil {
		value.origin = provenance.Symbol(objectDeclaration(event.object))
	} else if event.record.Origin.Behavior != nil {
		value.origin = provenance.Symbol(event.record.Origin.Behavior)
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
