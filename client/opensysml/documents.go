package opensysml

import (
	"context"
	"strconv"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// Cell is one typed document-query value: Element, Object, String, Int, Real,
// Bool, Quantity, Infinity, DocumentVerdict, DocumentState or DocumentEvent. A
// type switch over them is exhaustive.
type Cell interface {
	isCell()
}

// Element is a model element a document query selected or was bound to, named
// by qualified name.
type Element struct {
	// ID is the element's qualified name, the identity a binding names it by.
	ID string
	// Type is its metamodel type name ("PartUsage", …); answered, never bound.
	Type string
}

// Object is an object the service holds for the model, created by Instantiate.
// Bound, it names the object by Path when set and by ID otherwise (both must
// then name one object); answered, it carries ID, Path and Element.
type Object struct {
	// ID is the object's id, as Instantiate answered it.
	ID int64
	// Path is the object by the label a session reaches it under: the qualified
	// name it was instantiated as ("Garage::car"), its id ("#2"), or a path
	// through feature values of either ("Garage::car.wheels[2]", "#2.wheels[2]";
	// indexes count from 1).
	Path string
	// Element is the usage the object is held under, its definition or usage;
	// answered, ignored when bound.
	Element Element
}

// ObjectByID names a held object by the id Instantiate answered.
func ObjectByID(id int64) Object { return Object{ID: id} }

// ObjectByPath names a held object by name, id or path ("car", "#2",
// "car.wheels[2]").
func ObjectByPath(path string) Object { return Object{Path: path} }

// Infinity is an unbounded multiplicity. It is answered, never bound.
type Infinity struct{}

// DocumentVerdict is a row a `Verdicts` query answered: an assertion checked
// on the object at Path. It is answered, never bound.
type DocumentVerdict struct {
	// Assertion is the constraint, requirement, satisfy usage or verification
	// case checked; its ID is empty when the assertion is anonymous.
	Assertion Element
	// Kind is "constraint", "requirement", "satisfaction" or "verification".
	Kind string
	// Text is the assertion as written ("assert constraint massKnown").
	Text string
	// Path names the object checked from the element the query was bound to
	// ("Garage::car.wheels[2]").
	Path string
	// Status is "holds", "violated" or "undecided".
	Status string
	// Condition is the condition that evaluated to false, as written; empty otherwise.
	Condition string
	// Reason is why the assertion is violated or undecided; empty when it holds.
	Reason string
	// Verification is the verdict kinds of the verification cases verifying the
	// requirement the row is about; a verification row's own kind.
	Verification []string
}

// DocumentState is a row a `States` query answered: one active leaf state of
// the state machine Object exhibits as Machine. It is answered, never bound.
type DocumentState struct {
	// Object is the object whose state the row reports.
	Object Object
	// Machine is the exhibited state usage's name ("lp"), or the state
	// definition's when the object performs one directly.
	Machine string
	// Name is the active leaf state's name ("dim").
	Name string
	// Path is the leaf's dotted path from the machine's top level ("on.dim").
	Path string
	// State is the leaf's declaration; its ID is empty for an anonymous state.
	State Element
	// Region is the orthogonal region declaring the leaf ("light"); empty for a
	// leaf outside any region.
	Region string
	// Enclosing are the active composite states above the leaf, outermost first.
	Enclosing []string
}

// DocumentEvent is a row an `Events` query answered: one record of a session's
// trace. It is answered, never bound.
type DocumentEvent struct {
	// Kind is "accept", "send", "transition", "entry", "exit", "do", "choice"
	// or "guard".
	Kind string
	// Time is the run's clock when the record was made: a Quantity when the
	// clock carries a unit, a Real otherwise.
	Time Cell
	// Object is the object the record is about; nil for a record of none.
	Object *Object
	// Machine is the state machine the record is about, as DocumentState.Machine names it.
	Machine string
	// State is the state entered, exited or performing, by path; From and To
	// are a transition's source and target.
	State, From, To string
	// Target is the object a send was addressed to; nil for any other record.
	Target *Object
	// Event is the signal accepted or sent.
	Event string
	// Payload is the accepted signal's arguments as "name = value", by name.
	Payload []string
	// Alternatives are a choice's candidates as offered; Taken is the one drawn.
	Alternatives []string
	Taken        string
	// Text is the record as the trace prints it.
	Text string
}

func (Element) isCell()         { /* marker: closed Cell set */ }
func (Object) isCell()          { /* marker: closed Cell set */ }
func (Infinity) isCell()        { /* marker: closed Cell set */ }
func (DocumentVerdict) isCell() { /* marker: closed Cell set */ }
func (DocumentState) isCell()   { /* marker: closed Cell set */ }
func (DocumentEvent) isCell()   { /* marker: closed Cell set */ }
func (String) isCell()          { /* marker: closed Cell set */ }
func (Int) isCell()             { /* marker: closed Cell set */ }
func (Real) isCell()            { /* marker: closed Cell set */ }
func (Bool) isCell()            { /* marker: closed Cell set */ }
func (Quantity) isCell()        { /* marker: closed Cell set */ }

// String is the element as a binding names it, its qualified name.
func (e Element) String() string { return e.ID }

// String is the object as a session names it: its path when it has one, else
// its id ("#2").
func (o Object) String() string {
	if o.Path != "" {
		return o.Path
	}
	return "#" + strconv.FormatInt(o.ID, 10)
}

// String reports an unbounded multiplicity as the notation writes it.
func (Infinity) String() string { return "*" }

// String is the verdict in one line, as the CLI reports it:
// "assert constraint inflated on Garage::car.wheels[2]: violated".
func (v DocumentVerdict) String() string {
	if v.Path == "" {
		return v.Text + ": " + v.Status
	}
	return v.Text + " on " + v.Path + ": " + v.Status
}

// String is the state in one line: "lamp1.lp in on.dim".
func (s DocumentState) String() string {
	return s.Object.String() + "." + s.Machine + " in " + s.Path
}

// String is the event as the trace prints it, prefixed by its time:
// "1 s: accept Dim".
func (e DocumentEvent) String() string {
	if e.Time == nil {
		return e.Text
	}
	return CellText(e.Time) + ": " + e.Text
}

// Binding binds one entry parameter of a document query. Several values bind a
// nonscalar parameter.
type Binding struct {
	Parameter string
	Values    []Cell
}

// Bind is a binding of one parameter to the values given.
func Bind(parameter string, values ...Cell) Binding {
	return Binding{Parameter: parameter, Values: values}
}

// Rows is a document query's answer: its projected columns, and one row per
// selected element, both in the order the engine reports.
type Rows struct {
	// Columns are the projected properties in projection order.
	Columns []string
	// Rows are the selected elements and their cells.
	Rows []Row
}

// Row is one selected element and its projected cells, one per column.
type Row struct {
	// Element is the element the row is about: for an object row, the usage the
	// object is held under; for a row a `Verdicts` query answered, the assertion
	// checked.
	Element Element
	// Object is the object a row over held objects is about — one an `Objects`
	// query enumerated or a bound object's part — nil for any other row.
	Object *Object
	// Verdict is the verdict a row a `Verdicts` query answered carries; nil for
	// any other row.
	Verdict *DocumentVerdict
	// State is the state a row a `States` query answered carries; nil for any
	// other row.
	State *DocumentState
	// Event is the trace record a row an `Events` query answered carries; nil
	// for any other row.
	Event *DocumentEvent
	// Cells holds each column's values, in column order.
	Cells [][]Cell
}

func (c *client) RunDocumentQuery(
	ctx context.Context,
	model *Model,
	queryID string,
	bindings ...Binding,
) (*Rows, error) {
	hash, err := c.call(model)
	if err != nil {
		return nil, err
	}
	req := &pb.RunDocumentQueryRequest{ModelHash: hash, QueryId: queryID}
	for _, binding := range bindings {
		bound := &pb.DocumentQueryBinding{Parameter: binding.Parameter}
		for _, value := range binding.Values {
			sent, err := cellToProto(value)
			if err != nil {
				return nil, err
			}
			bound.Values = append(bound.Values, sent)
		}
		req.Bindings = append(req.Bindings, bound)
	}
	resp, err := c.caller.runDocumentQuery(ctx, req)
	if err != nil {
		return nil, err
	}
	rows := &Rows{Columns: make([]string, 0, len(resp.Columns)), Rows: make([]Row, 0, len(resp.Rows))}
	for _, column := range resp.Columns {
		rows.Columns = append(rows.Columns, column.Name)
	}
	for _, row := range resp.Rows {
		converted := Row{Cells: make([][]Cell, 0, len(row.Cells))}
		switch selected := cellFromProto(row.Element).(type) {
		case Element:
			converted.Element = selected
		case Object:
			converted.Element = selected.Element
			converted.Object = &selected
		case DocumentVerdict:
			converted.Element = selected.Assertion
			converted.Verdict = &selected
		case DocumentState:
			converted.Element = selected.Object.Element
			converted.Object = &selected.Object
			converted.State = &selected
		case DocumentEvent:
			if selected.Object != nil {
				converted.Element = selected.Object.Element
				converted.Object = selected.Object
			}
			converted.Event = &selected
		}
		for _, cell := range row.Cells {
			values := make([]Cell, 0, len(cell.Values))
			for _, value := range cell.Values {
				values = append(values, cellFromProto(value))
			}
			converted.Cells = append(converted.Cells, values)
		}
		rows.Rows = append(rows.Rows, converted)
	}
	return rows, nil
}

func (c *client) RenderDocument(ctx context.Context, model *Model, documentID string) (string, error) {
	hash, err := c.call(model)
	if err != nil {
		return "", err
	}
	resp, err := c.caller.renderDocument(ctx, &pb.RenderDocumentRequest{ModelHash: hash, DocumentId: documentID})
	if err != nil {
		return "", err
	}
	return resp.Markdown, nil
}

// cellToProto marshals a bound value; infinity and the verdict, state and event
// rows are refused here as the service refuses them: queries answer, nothing binds.
func cellToProto(cell Cell) (*pb.DocumentValue, error) {
	switch value := cell.(type) {
	case nil:
		return nil, &StatusError{Code: CodeInvalidArgument, Message: "a binding carries no value"}
	case Element:
		return &pb.DocumentValue{Kind: &pb.DocumentValue_ElementId{ElementId: value.ID}}, nil
	case Object:
		if value.ID == 0 && value.Path == "" {
			return nil, &StatusError{Code: CodeInvalidArgument, Message: "an object is bound by id or by path; neither was given"}
		}
		return &pb.DocumentValue{Kind: &pb.DocumentValue_Object{
			Object: &pb.DocumentObject{InstanceId: value.ID, Path: value.Path},
		}}, nil
	case String:
		return &pb.DocumentValue{Kind: &pb.DocumentValue_StringValue{StringValue: string(value)}}, nil
	case Int:
		return &pb.DocumentValue{Kind: &pb.DocumentValue_IntValue{IntValue: int64(value)}}, nil
	case Real:
		return &pb.DocumentValue{Kind: &pb.DocumentValue_RealValue{RealValue: float64(value)}}, nil
	case Bool:
		return &pb.DocumentValue{Kind: &pb.DocumentValue_BoolValue{BoolValue: bool(value)}}, nil
	case Quantity:
		quantity, err := quantityToProto(value)
		if err != nil {
			return nil, err
		}
		return &pb.DocumentValue{Kind: &pb.DocumentValue_Quantity{Quantity: quantity}}, nil
	case Infinity:
		return nil, &StatusError{
			Code:    CodeInvalidArgument,
			Message: "infinity is answered by queries, not bound to them",
		}
	case DocumentVerdict:
		return nil, &StatusError{
			Code:    CodeInvalidArgument,
			Message: "a verdict is answered by queries, not bound to them",
		}
	case DocumentState:
		return nil, &StatusError{
			Code:    CodeInvalidArgument,
			Message: "a state row is answered by queries, not bound to them",
		}
	case DocumentEvent:
		return nil, &StatusError{
			Code:    CodeInvalidArgument,
			Message: "an event row is answered by queries, not bound to them",
		}
	default:
		return nil, &StatusError{Code: CodeInvalidArgument, Message: "unknown document value kind"}
	}
}

func cellFromProto(value *pb.DocumentValue) Cell {
	switch kind := value.GetKind().(type) {
	case *pb.DocumentValue_ElementId:
		return Element{ID: kind.ElementId, Type: value.GetElementType()}
	case *pb.DocumentValue_StringValue:
		return String(kind.StringValue)
	case *pb.DocumentValue_IntValue:
		return Int(kind.IntValue)
	case *pb.DocumentValue_RealValue:
		return Real(kind.RealValue)
	case *pb.DocumentValue_BoolValue:
		return Bool(kind.BoolValue)
	case *pb.DocumentValue_Quantity:
		quantity, ok := quantityFromProto(kind.Quantity)
		if !ok {
			return nil
		}
		return quantity
	case *pb.DocumentValue_Object:
		return objectFromProto(kind.Object)
	case *pb.DocumentValue_Infinity:
		return Infinity{}
	case *pb.DocumentValue_Verdict:
		verdict := DocumentVerdict{
			Kind:         kind.Verdict.GetKind(),
			Text:         kind.Verdict.GetText(),
			Path:         kind.Verdict.GetPath(),
			Status:       kind.Verdict.GetVerdict(),
			Condition:    kind.Verdict.GetCondition(),
			Reason:       kind.Verdict.GetReason(),
			Verification: append([]string(nil), kind.Verdict.GetVerification()...),
		}
		if assertion, ok := cellFromProto(kind.Verdict.GetAssertion()).(Element); ok {
			verdict.Assertion = assertion
		}
		return verdict
	case *pb.DocumentValue_State:
		state := DocumentState{
			Object:    objectFromProto(kind.State.GetObject()),
			Machine:   kind.State.GetMachine(),
			Name:      kind.State.GetName(),
			Path:      kind.State.GetStatePath(),
			Region:    kind.State.GetRegion(),
			Enclosing: append([]string(nil), kind.State.GetEnclosing()...),
		}
		if declaration, ok := cellFromProto(kind.State.GetState()).(Element); ok {
			state.State = declaration
		}
		return state
	case *pb.DocumentValue_Event:
		event := DocumentEvent{
			Kind:         kind.Event.GetKind(),
			Time:         cellFromProto(kind.Event.GetTime()),
			Machine:      kind.Event.GetMachine(),
			State:        kind.Event.GetState(),
			From:         kind.Event.GetFrom(),
			To:           kind.Event.GetTo(),
			Event:        kind.Event.GetEvent(),
			Payload:      append([]string(nil), kind.Event.GetPayload()...),
			Alternatives: append([]string(nil), kind.Event.GetAlternatives()...),
			Taken:        kind.Event.GetTaken(),
			Text:         kind.Event.GetText(),
		}
		if kind.Event.GetObject() != nil {
			object := objectFromProto(kind.Event.GetObject())
			event.Object = &object
		}
		if kind.Event.GetTarget() != nil {
			target := objectFromProto(kind.Event.GetTarget())
			event.Target = &target
		}
		return event
	default:
		return nil
	}
}

func objectFromProto(object *pb.DocumentObject) Object {
	out := Object{ID: object.GetInstanceId(), Path: object.GetPath()}
	if element, ok := cellFromProto(object.GetElement()).(Element); ok {
		out.Element = element
	}
	return out
}

// CellText renders one cell value as a report writes it, the way the CLI's
// document tables do.
func CellText(cell Cell) string {
	switch value := cell.(type) {
	case nil:
		return ""
	case Element:
		return value.ID
	case Object:
		return value.String()
	case String:
		return string(value)
	case Int:
		return strconv.FormatInt(int64(value), 10)
	case Real:
		return strconv.FormatFloat(float64(value), 'g', -1, 64)
	case Bool:
		return strconv.FormatBool(bool(value))
	case Quantity:
		return value.String()
	case Infinity:
		return "*"
	case DocumentVerdict:
		return value.String()
	case DocumentState:
		return value.String()
	case DocumentEvent:
		return value.String()
	default:
		return ""
	}
}
