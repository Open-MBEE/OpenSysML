package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// expectedValue is the fixture encoding of a pb.Value: the oneof field name
// plus its JSON literal.
type expectedValue struct {
	Kind  string      `json:"kind"`
	Value interface{} `json:"value"`
}

// expectedFeatureValue is the fixture encoding of a pb.FeatureValue.
type expectedFeatureValue struct {
	Materialized bool        `json:"materialized"`
	ValueKind    string      `json:"value_kind"`
	Value        interface{} `json:"value"`
	// Error, when set, is a substring the feature value's error must contain.
	Error string `json:"error"`
}

// expectedAttribute is the fixture encoding of a pb.AttributeInfo.
type expectedAttribute struct {
	Type      string      `json:"type"`
	ValueKind string      `json:"value_kind"`
	Value     interface{} `json:"value"`
	Unit      string      `json:"unit"`
}

// conformanceCase is the schema of a <name>.expected.json fixture. See
// testdata/conformance/README.md.
type conformanceCase struct {
	RPC string `json:"rpc"`

	// ApplyEdits
	Operations      []conformanceEditOperation `json:"operations,omitempty"`
	ExpectedContent string                     `json:"expected_content,omitempty"`

	// Evaluate
	Expression      string `json:"expression,omitempty"`
	ContextSymbolID string `json:"context_symbol_id,omitempty"`
	SubjectSymbolID string `json:"subject_symbol_id,omitempty"`

	// GetSymbol, Instantiate, ExecuteAction, ExecuteState
	SymbolID string `json:"symbol_id,omitempty"`

	// ExecuteAction
	Inputs map[string]expectedValue `json:"inputs,omitempty"`

	// ExecuteState
	Events []string `json:"events,omitempty"`

	ExpectedResult        *expectedValue                  `json:"expected_result,omitempty"`
	ExpectedFeatureValues map[string]expectedFeatureValue `json:"expected_feature_values,omitempty"`
	ExpectedInstanceCount int                             `json:"expected_instance_count,omitempty"`
	ExpectedOutputs       map[string]expectedValue        `json:"expected_outputs,omitempty"`
	ExpectedStatesVisited []string                        `json:"expected_states_visited,omitempty"`
	ExpectedFinalContext  map[string]expectedValue        `json:"expected_final_context,omitempty"`

	// GetSymbol
	ExpectedAttributeNames []string                     `json:"expected_attribute_names,omitempty"`
	ExpectedAttributes     map[string]expectedAttribute `json:"expected_attributes,omitempty"`
	// MinWithheldLibraryAttributes requires the response to state at least this
	// many library-inherited attributes withheld from the list.
	MinWithheldLibraryAttributes int32 `json:"min_withheld_library_attributes,omitempty"`

	// ExpectedError, when set, requires the RPC to report an in-band error
	// containing this substring.
	ExpectedError string `json:"expected_error,omitempty"`
}

type conformanceEditOperation struct {
	Kind         string   `json:"kind"`
	Target       string   `json:"target,omitempty"`
	Value        string   `json:"value,omitempty"`
	NewName      string   `json:"new_name,omitempty"`
	Owner        string   `json:"owner,omitempty"`
	MemberKind   string   `json:"member_kind,omitempty"`
	MemberName   string   `json:"member_name,omitempty"`
	Type         string   `json:"type,omitempty"`
	Multiplicity string   `json:"multiplicity,omitempty"`
	Specializes  []string `json:"specializes,omitempty"`
	Cascade      bool     `json:"cascade,omitempty"`
}

// TestGRPCConformance is the AGENTS.md §5.2 Layer 2 contract for the gRPC
// service: every runtime RPC is driven end-to-end from a real model file
// (<name>.sysml) through ParseFile and the RPC under test, and its response is
// compared against <name>.expected.json.
func TestGRPCConformance(t *testing.T) {
	conformanceDir := filepath.Join("testdata", "conformance")

	entries, err := os.ReadDir(conformanceDir)
	if err != nil {
		t.Fatalf("read conformance dir: %v", err)
	}

	cases := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".expected.json") || strings.HasSuffix(entry.Name(), ".check.expected.json") {
			continue
		}
		caseName := strings.TrimSuffix(entry.Name(), ".expected.json")
		cases++
		t.Run(caseName, func(t *testing.T) {
			runGRPCConformanceCase(t, conformanceDir, caseName)
		})
	}

	if cases == 0 {
		t.Fatalf("no conformance cases in %s", conformanceDir)
	}
}

func runGRPCConformanceCase(t *testing.T, dir, caseName string) {
	t.Helper()

	expectedData, err := os.ReadFile(filepath.Join(dir, caseName+".expected.json"))
	if err != nil {
		t.Fatalf("read expectation: %v", err)
	}
	var tc conformanceCase
	if err := json.Unmarshal(expectedData, &tc); err != nil {
		t.Fatalf("parse expectation: %v", err)
	}

	modelData, err := os.ReadFile(filepath.Join(dir, caseName+".sysml"))
	if err != nil {
		t.Fatalf("read model: %v", err)
	}

	srv := mustNewService(t, 4)
	ctx := context.Background()

	parseResp, err := srv.ParseFile(ctx, &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: string(modelData)},
		ContentHash: caseName,
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	for _, diag := range parseResp.Diagnostics {
		if diag.Severity == "error" {
			t.Fatalf("model has a diagnostic error: %s", diag.Message)
		}
	}

	switch tc.RPC {
	case "GetSymbol":
		runGetSymbolCase(t, srv, ctx, parseResp.ModelHash, tc)
	case "Evaluate":
		runEvaluateCase(t, srv, ctx, parseResp.ModelHash, tc)
	case "Instantiate":
		runInstantiateCase(t, srv, ctx, parseResp.ModelHash, tc)
	case "ExecuteAction":
		runExecuteActionCase(t, srv, ctx, parseResp.ModelHash, tc)
	case "ExecuteState":
		runExecuteStateCase(t, srv, ctx, parseResp.ModelHash, tc)
	case "ApplyEdits":
		runApplyEditsCase(t, srv, ctx, parseResp.ModelHash, tc)
	default:
		t.Fatalf("unknown rpc %q", tc.RPC)
	}
}

func runApplyEditsCase(t *testing.T, srv *Service, ctx context.Context, modelHash string, tc conformanceCase) {
	t.Helper()

	operations := make([]*pb.EditOperation, 0, len(tc.Operations))
	for _, op := range tc.Operations {
		switch op.Kind {
		case "add_member":
			operations = append(operations, &pb.EditOperation{
				Operation: &pb.EditOperation_AddMember{AddMember: &pb.AddMemberEdit{
					Owner: op.Owner, Kind: op.MemberKind, Name: op.MemberName,
					Type: op.Type, Multiplicity: op.Multiplicity,
					Value: op.Value, Specializes: op.Specializes,
				}},
			})
		case "delete":
			operations = append(operations, &pb.EditOperation{
				Operation: &pb.EditOperation_Delete{Delete: &pb.DeleteEdit{
					Target: op.Target, Cascade: op.Cascade,
				}},
			})
		case "set_value":
			operations = append(operations, &pb.EditOperation{
				Operation: &pb.EditOperation_SetValue{SetValue: &pb.SetValueEdit{
					Target: op.Target, Value: op.Value,
				}},
			})
		case "rename":
			operations = append(operations, &pb.EditOperation{
				Operation: &pb.EditOperation_Rename{Rename: &pb.RenameEdit{
					Target: op.Target, NewName: op.NewName,
				}},
			})
		default:
			t.Fatalf("unknown edit operation %q", op.Kind)
		}
	}

	resp, err := srv.ApplyEdits(ctx, &pb.ApplyEditsRequest{
		ModelHash: modelHash, Operations: operations,
	})
	if err != nil {
		t.Fatalf("ApplyEdits: %v", err)
	}
	if checkExpectedError(t, tc, resp.Error) {
		return
	}
	if resp.Content != tc.ExpectedContent {
		t.Errorf("content =\n%s\nwant\n%s", resp.Content, tc.ExpectedContent)
	}
}

func runEvaluateCase(t *testing.T, srv *Service, ctx context.Context, modelHash string, tc conformanceCase) {
	t.Helper()

	resp, err := srv.Evaluate(ctx, &pb.EvaluateRequest{
		ModelHash:       modelHash,
		Expression:      tc.Expression,
		ContextSymbolId: tc.ContextSymbolID,
		SubjectSymbolId: tc.SubjectSymbolID,
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if checkExpectedError(t, tc, resp.Error) {
		return
	}
	if tc.ExpectedResult != nil {
		checkValue(t, "result", *tc.ExpectedResult, resp.Result)
	}
}

// runGetSymbolCase pins the static facts a symbol is reported with, and is how
// the attribute set is kept from regressing to empty.
func runGetSymbolCase(t *testing.T, srv *Service, ctx context.Context, modelHash string, tc conformanceCase) {
	t.Helper()

	resp, err := srv.GetSymbol(ctx, &pb.GetSymbolRequest{
		ModelHash: modelHash,
		SymbolId:  tc.SymbolID,
	})
	if err != nil {
		t.Fatalf("GetSymbol: %v", err)
	}
	if checkExpectedError(t, tc, resp.Error) {
		return
	}
	if resp.Symbol == nil {
		t.Fatal("expected a symbol")
	}

	attrs := resp.Symbol.GetAttributes()
	if tc.ExpectedAttributeNames != nil {
		var got []string
		for _, attr := range attrs {
			got = append(got, attr.GetName())
		}
		if strings.Join(got, ",") != strings.Join(tc.ExpectedAttributeNames, ",") {
			t.Errorf("attributes = %v, want %v", got, tc.ExpectedAttributeNames)
		}
	}
	if want := tc.MinWithheldLibraryAttributes; want > 0 {
		if got := resp.Symbol.GetWithheldLibraryAttributes(); got < want {
			t.Errorf("withheld library attributes = %d, want at least %d", got, want)
		}
	}

	byName := make(map[string]*pb.AttributeInfo, len(attrs))
	for _, attr := range attrs {
		byName[attr.GetName()] = attr
	}
	for name, want := range tc.ExpectedAttributes {
		got, ok := byName[name]
		if !ok {
			t.Errorf("missing attribute %q", name)
			continue
		}
		if got.GetType() != want.Type {
			t.Errorf("attribute %q: type = %q, want %q", name, got.GetType(), want.Type)
		}
		if got.GetUnit() != want.Unit {
			t.Errorf("attribute %q: unit = %q, want %q", name, got.GetUnit(), want.Unit)
		}
		if want.ValueKind == "" {
			if got.GetValue() != nil {
				kind, value := describeValue(got.GetValue())
				t.Errorf("attribute %q: unexpected value %s %v", name, kind, value)
			}
			continue
		}
		checkValue(t, "attribute "+name, expectedValue{Kind: want.ValueKind, Value: want.Value}, got.GetValue())
	}
}

func runInstantiateCase(t *testing.T, srv *Service, ctx context.Context, modelHash string, tc conformanceCase) {
	t.Helper()

	resp, err := srv.Instantiate(ctx, &pb.InstantiateRequest{
		ModelHash: modelHash,
		SymbolId:  tc.SymbolID,
	})
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if checkExpectedError(t, tc, resp.Error) {
		return
	}
	if resp.Instance == nil {
		t.Fatal("expected an instance")
	}
	if resp.Instance.TypeSymbolId != tc.SymbolID {
		t.Errorf("type_symbol_id = %q, want %q", resp.Instance.TypeSymbolId, tc.SymbolID)
	}
	for name, want := range tc.ExpectedFeatureValues {
		fv, ok := resp.Instance.FeatureValues[name]
		if !ok {
			t.Errorf("missing feature value %q", name)
			continue
		}
		if fv.Materialized != want.Materialized {
			t.Errorf("feature value %q: materialized = %v, want %v", name, fv.Materialized, want.Materialized)
		}
		if want.Error == "" {
			if fv.Error != "" {
				t.Errorf("feature value %q: unexpected error %q", name, fv.Error)
			}
		} else if !strings.Contains(fv.Error, want.Error) {
			t.Errorf("feature value %q: error = %q, want it to contain %q", name, fv.Error, want.Error)
		}
		if want.ValueKind != "" {
			checkValue(t, "feature value "+name, expectedValue{Kind: want.ValueKind, Value: want.Value}, fv.Value)
		}
	}
	if tc.ExpectedInstanceCount != 0 && len(resp.Instances) != tc.ExpectedInstanceCount {
		t.Errorf("instances = %d, want %d", len(resp.Instances), tc.ExpectedInstanceCount)
	}
}

func runExecuteActionCase(t *testing.T, srv *Service, ctx context.Context, modelHash string, tc conformanceCase) {
	t.Helper()

	var inputs map[string]*pb.Value
	if len(tc.Inputs) > 0 {
		inputs = make(map[string]*pb.Value, len(tc.Inputs))
		for name, val := range tc.Inputs {
			inputs[name] = toProtoValue(t, val)
		}
	}

	resp, err := srv.ExecuteAction(ctx, &pb.ExecuteActionRequest{
		ModelHash:      modelHash,
		ActionSymbolId: tc.SymbolID,
		Inputs:         inputs,
	})
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if checkExpectedError(t, tc, resp.Error) {
		return
	}
	for name, want := range tc.ExpectedOutputs {
		got, ok := resp.Outputs[name]
		if !ok {
			t.Errorf("missing output %q", name)
			continue
		}
		checkValue(t, "output "+name, want, got)
	}
}

func runExecuteStateCase(t *testing.T, srv *Service, ctx context.Context, modelHash string, tc conformanceCase) {
	t.Helper()

	resp, err := srv.ExecuteState(ctx, &pb.ExecuteStateRequest{
		ModelHash:            modelHash,
		StateMachineSymbolId: tc.SymbolID,
		Events:               tc.Events,
	})
	if err != nil {
		t.Fatalf("ExecuteState: %v", err)
	}
	if checkExpectedError(t, tc, resp.Error) {
		return
	}
	if tc.ExpectedStatesVisited != nil {
		if len(resp.StatesVisited) != len(tc.ExpectedStatesVisited) {
			t.Fatalf("states_visited = %v, want %v", resp.StatesVisited, tc.ExpectedStatesVisited)
		}
		for i, want := range tc.ExpectedStatesVisited {
			if resp.StatesVisited[i] != want {
				t.Fatalf("states_visited = %v, want %v", resp.StatesVisited, tc.ExpectedStatesVisited)
			}
		}
	}
	for name, want := range tc.ExpectedFinalContext {
		got, ok := resp.FinalContext[name]
		if !ok {
			t.Errorf("missing final_context entry %q", name)
			continue
		}
		checkValue(t, "final_context "+name, want, got)
	}
}

// checkExpectedError validates the in-band error field and reports whether the
// case is a negative one, in which case no further assertions apply.
func checkExpectedError(t *testing.T, tc conformanceCase, actual string) bool {
	t.Helper()

	if tc.ExpectedError == "" {
		if actual != "" {
			t.Fatalf("unexpected error: %s", actual)
		}
		return false
	}
	if !strings.Contains(actual, tc.ExpectedError) {
		t.Fatalf("error = %q, want it to contain %q", actual, tc.ExpectedError)
	}
	return true
}

// toProtoValue converts a fixture value to its pb.Value oneof.
func toProtoValue(t *testing.T, ev expectedValue) *pb.Value {
	t.Helper()

	switch ev.Kind {
	case "int_value":
		return &pb.Value{Kind: &pb.Value_IntValue{IntValue: int64(mustFloat(t, ev))}}
	case "real_value":
		return &pb.Value{Kind: &pb.Value_RealValue{RealValue: mustFloat(t, ev)}}
	case "bool_value":
		b, ok := ev.Value.(bool)
		if !ok {
			t.Fatalf("bool_value: %v is not a boolean", ev.Value)
		}
		return &pb.Value{Kind: &pb.Value_BoolValue{BoolValue: b}}
	case "string_value":
		s, ok := ev.Value.(string)
		if !ok {
			t.Fatalf("string_value: %v is not a string", ev.Value)
		}
		return &pb.Value{Kind: &pb.Value_StringValue{StringValue: s}}
	case "null":
		return &pb.Value{Kind: &pb.Value_Null{Null: ""}}
	default:
		t.Fatalf("unknown value kind %q", ev.Kind)
		return nil
	}
}

// checkValue compares a pb.Value against its fixture encoding, including which
// oneof arm is set.
func checkValue(t *testing.T, label string, want expectedValue, got *pb.Value) {
	t.Helper()

	if got == nil {
		t.Errorf("%s: value is nil, want %s %v", label, want.Kind, want.Value)
		return
	}

	gotKind, gotValue := describeValue(got)
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
	case "null", "instance_id", "unset":
		// The oneof arm carries all the information; ids are runtime-assigned.
	default:
		if fmt.Sprint(gotValue) != fmt.Sprint(want.Value) {
			t.Errorf("%s: value = %v, want %v", label, gotValue, want.Value)
		}
	}
}

// describeValue reports the set oneof arm of a pb.Value and its payload.
func describeValue(v *pb.Value) (string, interface{}) {
	switch k := v.Kind.(type) {
	case *pb.Value_IntValue:
		return "int_value", k.IntValue
	case *pb.Value_RealValue:
		return "real_value", k.RealValue
	case *pb.Value_BoolValue:
		return "bool_value", k.BoolValue
	case *pb.Value_StringValue:
		return "string_value", k.StringValue
	case *pb.Value_InstanceId:
		return "instance_id", k.InstanceId
	case *pb.Value_Sequence:
		return "sequence", k.Sequence
	case *pb.Value_Quantity:
		return "quantity", describeQuantity(k.Quantity)
	case *pb.Value_Null:
		return "null", nil
	case *pb.Value_Unset:
		return "unset", nil
	default:
		return "no arm", nil
	}
}

// describeQuantity renders a quantity as "<magnitude> [<unit as written>] =
// <reduction>", which is every part of it a fixture needs to pin.
func describeQuantity(q *pb.Quantity) string {
	if q == nil {
		return ""
	}
	magnitude := "unset"
	switch m := q.GetMagnitude().(type) {
	case *pb.Quantity_IntMagnitude:
		magnitude = strconv.FormatInt(m.IntMagnitude, 10)
	case *pb.Quantity_RealMagnitude:
		magnitude = strconv.FormatFloat(m.RealMagnitude, 'g', -1, 64)
	}
	return fmt.Sprintf("%s [%s] = %s", magnitude, q.GetUnit(), describeUnitTerm(q.GetUnitTerm()))
}

func mustFloat(t *testing.T, ev expectedValue) float64 {
	t.Helper()

	f, ok := ev.Value.(float64)
	if !ok {
		t.Fatalf("%s: %v is not a number", ev.Kind, ev.Value)
	}
	return f
}
