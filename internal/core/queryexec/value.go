// Package queryexec executes immutable document-query plans.
package queryexec

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// ValueKind classifies one scalar query value.
type ValueKind string

const (
	ValueElement ValueKind = "element"
	// ValueObject is a runtime object a session holds, `car.wheels[2]`, as opposed
	// to the model element declaring it.
	ValueObject ValueKind = "object"
	// ValueVerdict is an assertion checked about an object, carried as a row whose
	// declaration is the asserting element.
	ValueVerdict ValueKind = "verdict"
	// ValueState is one active state of an object's state machine, carried as a
	// row whose declaration is the state.
	ValueState ValueKind = "state"
	// ValueEvent is one record of a session's trace, carried as a row whose
	// declaration is the behavior that made it.
	ValueEvent    ValueKind = "event"
	ValueString   ValueKind = "string"
	ValueInteger  ValueKind = "integer"
	ValueReal     ValueKind = "real"
	ValueBoolean  ValueKind = "boolean"
	ValueInfinity ValueKind = "infinity"
	// ValueQuantity is a magnitude in a measurement unit, `2290000 [kg]`.
	ValueQuantity ValueKind = "quantity"
)

// Value is one immutable scalar carried through query execution.
type Value struct {
	kind     ValueKind
	element  *symbols.Symbol
	object   *runtime.Instance
	verdict  *Verdict
	state    *State
	event    *Event
	text     string
	integer  int64
	real     float64
	boolean  bool
	quantity *semantics.Quantity
	origin   symbols.Origin
}

// ElementValue constructs an element value with declaration provenance.
func ElementValue(sym *symbols.Symbol) Value {
	return Value{kind: ValueElement, element: sym, origin: sym.Origin()}
}

// ObjectValue constructs a runtime object value under the label a session
// reaches it by (`car.wheels[2]`, `#7`), with its declaration as provenance.
func ObjectValue(inst *runtime.Instance, label string) Value {
	value := Value{kind: ValueObject, object: inst, text: label}
	if inst != nil {
		value.origin = objectDeclaration(inst).Origin()
	}
	return value
}

// StringValue constructs a string value.
func StringValue(value string) Value {
	return Value{kind: ValueString, text: value}
}

// IntegerValue constructs an integer value.
func IntegerValue(value int64) Value {
	return Value{kind: ValueInteger, integer: value}
}

// RealValue constructs a real value.
func RealValue(value float64) Value {
	return Value{kind: ValueReal, real: value}
}

// BooleanValue constructs a Boolean value.
func BooleanValue(value bool) Value {
	return Value{kind: ValueBoolean, boolean: value}
}

// QuantityValue constructs a quantity value from a magnitude in a unit; the
// quantity is deep-copied, so the value stays immutable.
func QuantityValue(quantity semantics.Quantity) Value {
	clone := quantity.Clone()
	return Value{kind: ValueQuantity, quantity: &clone}
}

// constantValue converts a folded semantic constant to the query value of the
// same kind.
func constantValue(constant semantics.Value) (Value, bool) {
	switch constant.Kind {
	case semantics.ValInt:
		return IntegerValue(constant.Int), true
	case semantics.ValReal:
		return RealValue(constant.Real), true
	case semantics.ValBool:
		return BooleanValue(constant.Bool), true
	case semantics.ValInfinity:
		return Value{kind: ValueInfinity}, true
	default:
		return Value{}, false
	}
}

func valueAt(value Value, origin symbols.Origin) Value {
	value.origin = origin
	return value
}

// Kind returns the value's scalar kind.
func (v Value) Kind() ValueKind { return v.kind }

// Element returns the value's element and whether it is an element value.
func (v Value) Element() (*symbols.Symbol, bool) {
	return v.element, v.kind == ValueElement && v.element != nil
}

// Declaration returns the element a value is declared by: an element itself, the
// usage or definition an object stands for, the assertion a verdict is about,
// the state a state row is of, the behavior an event row came from (its object's
// declaration when the record names none), and nil for a scalar.
func (v Value) Declaration() *symbols.Symbol {
	if inst, _, ok := v.Object(); ok {
		return objectDeclaration(inst)
	}
	if verdict, ok := v.Verdict(); ok {
		return verdict.assertion
	}
	if state, ok := v.State(); ok {
		return state.Declaration()
	}
	if event, ok := v.Event(); ok {
		if behavior := event.Behavior(); behavior != nil {
			return behavior
		}
		if inst, _ := event.Object(); inst != nil {
			return objectDeclaration(inst)
		}
		return nil
	}
	sym, _ := v.Element()
	return sym
}

// Object returns the value's runtime object, the label it is reached by, and
// whether it is an object value.
func (v Value) Object() (*runtime.Instance, string, bool) {
	if v.kind != ValueObject || v.object == nil {
		return nil, "", false
	}
	return v.object, v.text, true
}

// String returns the value's string and whether it is a string value.
func (v Value) String() (string, bool) { return v.text, v.kind == ValueString }

// Integer returns the value's integer and whether it is an integer value.
func (v Value) Integer() (int64, bool) { return v.integer, v.kind == ValueInteger }

// Real returns the value's real and whether it is a real value.
func (v Value) Real() (float64, bool) { return v.real, v.kind == ValueReal }

// Boolean returns the value's Boolean and whether it is a Boolean value.
func (v Value) Boolean() (bool, bool) { return v.boolean, v.kind == ValueBoolean }

// Quantity returns an independent copy of the value's quantity and whether it
// is a quantity value.
func (v Value) Quantity() (semantics.Quantity, bool) {
	if v.kind != ValueQuantity || v.quantity == nil {
		return semantics.Quantity{}, false
	}
	return v.quantity.Clone(), true
}

// Magnitude returns a quantity value's magnitude as the bare integer or real
// value it is, and whether the value is a quantity.
func (v Value) Magnitude() (Value, bool) {
	quantity, ok := v.Quantity()
	if !ok {
		return Value{}, false
	}
	magnitude, ok := constantValue(quantity.Num)
	return valueAt(magnitude, v.origin), ok
}

// Origin returns the source declaration behind the value.
func (v Value) Origin() symbols.Origin { return v.origin }

// Bindings supplies named values to an entry query.
type Bindings map[string][]Value

// Column describes one ordered projected property.
type Column struct {
	name   string
	origin symbols.Origin
}

// Name returns the projected property name.
func (c Column) Name() string { return c.name }

// Origin returns the query expression that projected the column.
func (c Column) Origin() symbols.Origin { return c.origin }

// Cell is one immutable projected value sequence.
type Cell struct {
	values []Value
	origin symbols.Origin
}

// Values returns an independent copy of the cell values.
func (c Cell) Values() []Value { return append([]Value(nil), c.values...) }

// Origin returns the selected model element behind the cell.
func (c Cell) Origin() symbols.Origin { return c.origin }

// isRow reports whether a value can be a query row: an element, an object, a
// verdict, a state or an event.
func (v Value) isRow() bool {
	switch v.kind {
	case ValueElement:
		return v.element != nil
	case ValueObject:
		return v.object != nil
	case ValueVerdict:
		return v.verdict != nil
	case ValueState:
		return v.state != nil
	case ValueEvent:
		return v.event != nil
	}
	return false
}

// Row retains the selected element, object, verdict, state or event and its
// ordered projected cells.
type Row struct {
	element Value
	cells   []Cell
}

// Element returns the selected value: a model element, a runtime object when
// the query ran over a session's objects, a verdict about one, a state one is
// in, or an event of the session's trace.
func (r Row) Element() Value { return r.element }

// Cells returns an independent copy of the row's projected cells.
func (r Row) Cells() []Cell {
	out := make([]Cell, len(r.cells))
	for i, cell := range r.cells {
		out[i] = Cell{values: cell.Values(), origin: cell.origin}
	}
	return out
}

// Origin returns the selected element's declaration provenance.
func (r Row) Origin() symbols.Origin { return r.element.origin }

// RowSet is an immutable ordered query result.
type RowSet struct {
	columns []Column
	rows    []Row
	origin  symbols.Origin
}

// Columns returns an independent copy of the projected columns.
func (r *RowSet) Columns() []Column {
	if r == nil {
		return nil
	}
	return append([]Column(nil), r.columns...)
}

// Rows returns an independent copy of the result rows.
func (r *RowSet) Rows() []Row {
	if r == nil {
		return nil
	}
	out := make([]Row, len(r.rows))
	for i, row := range r.rows {
		out[i] = Row{element: row.element, cells: row.Cells()}
	}
	return out
}

// Origin returns the entry query declaration.
func (r *RowSet) Origin() symbols.Origin {
	if r == nil {
		return symbols.Origin{}
	}
	return r.origin
}
