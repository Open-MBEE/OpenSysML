package grpc_test

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/grpc"
)

// expectedBinding is the fixture encoding of a pb.DocumentQueryBinding: the
// parameter and the document values bound to it, each an expectedValue whose
// kind is a pb.DocumentValue arm.
type expectedBinding struct {
	Parameter string          `json:"parameter"`
	Values    []expectedValue `json:"values"`
}

// expectedRow is the fixture encoding of a pb.DocumentQueryRow: the row's own
// value and its cells, one list of values per column.
type expectedRow struct {
	Element expectedValue     `json:"element"`
	Cells   [][]expectedValue `json:"cells"`
}

// runRunDocumentQueryCase instantiates the case's objects, runs its query and
// compares columns and rows, or the status error a failing case pins.
func runRunDocumentQueryCase(t *testing.T, srv *grpc.Service, ctx context.Context, modelHash string, tc conformanceCase) {
	t.Helper()

	for _, sym := range tc.Instantiate {
		resp, err := srv.Instantiate(ctx, &pb.InstantiateRequest{ModelHash: modelHash, SymbolId: sym})
		if err != nil {
			t.Fatalf("Instantiate %s: %v", sym, err)
		}
		if resp.Error != "" {
			t.Fatalf("Instantiate %s: %s", sym, resp.Error)
		}
	}
	bindings := make([]*pb.DocumentQueryBinding, 0, len(tc.Bindings))
	for _, b := range tc.Bindings {
		values := make([]*pb.DocumentValue, 0, len(b.Values))
		for _, v := range b.Values {
			values = append(values, toDocumentValue(t, v))
		}
		bindings = append(bindings, &pb.DocumentQueryBinding{Parameter: b.Parameter, Values: values})
	}
	resp, err := srv.RunDocumentQuery(ctx, &pb.RunDocumentQueryRequest{
		ModelHash: modelHash, QueryId: tc.QueryID, Bindings: bindings,
	})
	if tc.ExpectedError != "" {
		if err == nil {
			t.Fatalf("RunDocumentQuery answered %d rows, want an error containing %q", len(resp.Rows), tc.ExpectedError)
		}
		if !strings.Contains(err.Error(), tc.ExpectedError) {
			t.Fatalf("error = %q, want it to contain %q", err.Error(), tc.ExpectedError)
		}
		return
	}
	if err != nil {
		t.Fatalf("RunDocumentQuery: %v", err)
	}
	columns := make([]string, 0, len(resp.Columns))
	for _, column := range resp.Columns {
		columns = append(columns, column.Name)
	}
	if strings.Join(columns, ",") != strings.Join(tc.ExpectedColumns, ",") {
		t.Errorf("columns = %v, want %v", columns, tc.ExpectedColumns)
	}
	if len(resp.Rows) != len(tc.ExpectedRows) {
		t.Fatalf("rows = %d, want %d:\n%s", len(resp.Rows), len(tc.ExpectedRows), describeRows(resp.Rows))
	}
	for i, row := range resp.Rows {
		want := tc.ExpectedRows[i]
		checkDocumentValue(t, fmt.Sprintf("row %d", i+1), want.Element, row.Element)
		if len(row.Cells) != len(want.Cells) {
			t.Errorf("row %d: cells = %d, want %d", i+1, len(row.Cells), len(want.Cells))
			continue
		}
		for j, cell := range row.Cells {
			if len(cell.Values) != len(want.Cells[j]) {
				t.Errorf("row %d cell %d: values = %d, want %d", i+1, j+1, len(cell.Values), len(want.Cells[j]))
				continue
			}
			for k, value := range cell.Values {
				checkDocumentValue(t, fmt.Sprintf("row %d cell %d value %d", i+1, j+1, k+1), want.Cells[j][k], value)
			}
		}
	}
}

// toDocumentValue converts a fixture value to the pb.DocumentValue it binds: an
// element by qualified name, an object by `{"instance_id": n}`, `{"path": p}`
// or both, and the scalar arms by literal.
func toDocumentValue(t *testing.T, ev expectedValue) *pb.DocumentValue {
	t.Helper()

	switch ev.Kind {
	case "element_id":
		return &pb.DocumentValue{Kind: &pb.DocumentValue_ElementId{ElementId: fmt.Sprint(ev.Value)}}
	case "object":
		fields, ok := ev.Value.(map[string]interface{})
		if !ok {
			t.Fatalf("object: %v is not an {instance_id, path} object", ev.Value)
		}
		obj := &pb.DocumentObject{}
		if id, ok := fields["instance_id"].(float64); ok {
			obj.InstanceId = int64(id)
		}
		if path, ok := fields["path"].(string); ok {
			obj.Path = path
		}
		return &pb.DocumentValue{Kind: &pb.DocumentValue_Object{Object: obj}}
	case "int_value":
		return &pb.DocumentValue{Kind: &pb.DocumentValue_IntValue{IntValue: int64(mustFloat(t, ev))}}
	case "real_value":
		return &pb.DocumentValue{Kind: &pb.DocumentValue_RealValue{RealValue: mustFloat(t, ev)}}
	case "bool_value":
		b, ok := ev.Value.(bool)
		if !ok {
			t.Fatalf("bool_value: %v is not a boolean", ev.Value)
		}
		return &pb.DocumentValue{Kind: &pb.DocumentValue_BoolValue{BoolValue: b}}
	case "string_value":
		return &pb.DocumentValue{Kind: &pb.DocumentValue_StringValue{StringValue: fmt.Sprint(ev.Value)}}
	default:
		t.Fatalf("unsupported bound document value kind %q", ev.Kind)
		return nil
	}
}

// checkDocumentValue asserts a document value's arm and payload, as the
// fixture spells them (see testdata/conformance/README.md).
func checkDocumentValue(t *testing.T, label string, want expectedValue, got *pb.DocumentValue) {
	t.Helper()

	if got == nil {
		t.Errorf("%s: value is nil, want %s %v", label, want.Kind, want.Value)
		return
	}
	gotKind, gotValue := describeDocumentValue(got)
	if gotKind != want.Kind {
		t.Errorf("%s: kind = %s, want %s", label, gotKind, want.Kind)
		return
	}
	switch want.Kind {
	case "int_value":
		if got.GetIntValue() != int64(mustFloat(t, want)) {
			t.Errorf("%s: value = %d, want %v", label, got.GetIntValue(), want.Value)
		}
	case "real_value":
		if got.GetRealValue() != mustFloat(t, want) {
			t.Errorf("%s: value = %v, want %v", label, got.GetRealValue(), want.Value)
		}
	case "infinity":
	default:
		if fmt.Sprint(gotValue) != fmt.Sprint(want.Value) {
			t.Errorf("%s: value = %v, want %v", label, gotValue, want.Value)
		}
	}
}

// describeDocumentValue names a document value's arm and renders its payload
// in the fixture spelling: an element as "<fqn> (<type>)", an object as
// "<path> (#<id>) : <usage fqn> (<type>)", a verdict as "<text> on <path>: <verdict>",
// a state as "<path>.<machine> in <state path> (<region>)", an event as "<kind> at <time>: <text>".
func describeDocumentValue(v *pb.DocumentValue) (string, interface{}) {
	switch k := v.Kind.(type) {
	case *pb.DocumentValue_ElementId:
		return "element_id", describeElement(v)
	case *pb.DocumentValue_StringValue:
		return "string_value", k.StringValue
	case *pb.DocumentValue_IntValue:
		return "int_value", k.IntValue
	case *pb.DocumentValue_RealValue:
		return "real_value", k.RealValue
	case *pb.DocumentValue_BoolValue:
		return "bool_value", k.BoolValue
	case *pb.DocumentValue_Infinity:
		return "infinity", nil
	case *pb.DocumentValue_Quantity:
		return "quantity", describeQuantity(k.Quantity)
	case *pb.DocumentValue_Verdict:
		return "verdict", fmt.Sprintf("%s on %s: %s", k.Verdict.GetText(), k.Verdict.GetPath(), k.Verdict.GetVerdict())
	case *pb.DocumentValue_Object:
		return "object", fmt.Sprintf("%s (#%d) : %s", k.Object.GetPath(), k.Object.GetInstanceId(), describeElement(k.Object.GetElement()))
	case *pb.DocumentValue_State:
		return "state", fmt.Sprintf("%s.%s in %s (%s)", k.State.GetObject().GetPath(), k.State.GetMachine(), k.State.GetStatePath(), k.State.GetRegion())
	case *pb.DocumentValue_Event:
		_, at := describeDocumentValue(k.Event.GetTime())
		return "event", fmt.Sprintf("%s at %v: %s", k.Event.GetKind(), at, k.Event.GetText())
	default:
		return "no arm", nil
	}
}

// describeElement renders an element value as "<fqn> (<type>)".
func describeElement(v *pb.DocumentValue) string {
	if v.GetElementType() == "" {
		return v.GetElementId()
	}
	return fmt.Sprintf("%s (%s)", v.GetElementId(), v.GetElementType())
}

// describeRows renders answered rows one per line, for a failing case's message.
func describeRows(rows []*pb.DocumentQueryRow) string {
	var b strings.Builder
	for i, row := range rows {
		kind, value := describeDocumentValue(row.Element)
		fmt.Fprintf(&b, "%s: %s %v\n", strconv.Itoa(i+1), kind, value)
	}
	return b.String()
}
