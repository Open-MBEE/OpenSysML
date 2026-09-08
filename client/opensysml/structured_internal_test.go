package opensysml

import (
	"context"
	"reflect"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"google.golang.org/protobuf/proto"
)

// A structured value is refused before it leaves the client when the service
// lacks structured_values, since such a service would read it as null: whether
// the service predates the capability or the GetServerInfo RPC itself.
func TestStructuredInputIsNotSentWithoutStructuredValues(t *testing.T) {
	ctx := context.Background()
	model := &Model{Hash: "h"}
	vector := Vector{Real(3), Real(4)}
	for name, old := range map[string]*oldCaller{
		"predates structured_values": {t: t, capabilities: []string{CapabilityFeatureValues, CapabilityComplexValues}},
		"predates GetServerInfo":     {t: t, infoErr: &StatusError{Code: CodeUnimplemented, Message: "unknown method"}},
	} {
		t.Run(name, func(t *testing.T) {
			old.t = t
			c := &client{caller: old}
			for label, input := range map[string]Value{
				"vector":          vector,
				"array":           Array{Dimensions: []int64{1}, Elements: []Value{Int(1)}},
				"vector quantity": VectorQuantity{{Magnitude: Real(1), Unit: "m"}},
				"nested":          Sequence{Int(1), Sequence{vector}},
				"in an array":     Array{Dimensions: []int64{1}, Elements: []Value{vector}},
			} {
				_, err := c.ExecuteAction(ctx, model, "A", map[string]Value{"x": input})
				wantUnimplemented(t, "ExecuteAction "+label, err)
				_, err = c.EvaluateCalc(ctx, model, "f", Int(1), input)
				wantUnimplemented(t, "EvaluateCalc "+label, err)
			}
		})
	}
}

// An array carrying a Complex needs complex_values, as a sequence does.
func TestComplexInArrayNeedsComplexValues(t *testing.T) {
	old := &oldCaller{t: t, capabilities: []string{CapabilityStructuredValues}}
	c := &client{caller: old}
	input := Array{Dimensions: []int64{1}, Elements: []Value{Complex(complex(1, 2))}}
	_, err := c.ExecuteAction(context.Background(), &Model{Hash: "h"}, "A", map[string]Value{"z": input})
	wantUnimplemented(t, "ExecuteAction array of complex", err)
}

// A malformed structured value in an answer reads as an unsupported null
// naming the fault, never as an Array, Vector or VectorQuantity that
// contradicts its own contract.
func TestMalformedStructuredAnswersAreNullsNamingTheFault(t *testing.T) {
	one := &pb.Value{Kind: &pb.Value_IntValue{IntValue: 1}}
	metre := &pb.Quantity{Unit: "m", Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 1}}
	noMagnitude := &pb.Quantity{Unit: "m"}
	array := func(dims []int64, elements ...*pb.Value) *pb.Value {
		return &pb.Value{Kind: &pb.Value_Array{Array: &pb.Array{Dimensions: dims, Elements: elements}}}
	}
	vectorQuantity := func(components ...*pb.Quantity) *pb.Value {
		return &pb.Value{Kind: &pb.Value_VectorQuantity{VectorQuantity: &pb.VectorQuantity{Components: components}}}
	}
	text := &pb.Value{Kind: &pb.Value_StringValue{StringValue: "x"}}
	for name, tc := range map[string]struct {
		value *pb.Value
		want  string
	}{
		"zero dimension":           {array([]int64{0}), "not positive"},
		"negative dimension":       {array([]int64{2, -3}, one), "not positive"},
		"too few elements":         {array([]int64{2, 3}, one, one, one, one, one), "do not fill"},
		"too many elements":        {array([]int64{2}, one, one, one), "do not fill"},
		"rank 0 without one":       {array(nil), "do not fill"},
		"rank 0 with two":          {array(nil, one, one), "do not fill"},
		"overflowing shape":        {array([]int64{1 << 40, 1 << 40}, one), "exceeds the Integer range"},
		"non-numeric vector":       {&pb.Value{Kind: &pb.Value_Vector{Vector: &pb.Vector{Components: []*pb.Value{one, text}}}}, "non-numeric"},
		"empty vector quantity":    {vectorQuantity(), "without components"},
		"magnitude-less component": {vectorQuantity(metre, noMagnitude), "without a magnitude"},
		"magnitude-less quantity":  {&pb.Value{Kind: &pb.Value_Quantity{Quantity: noMagnitude}}, "without a magnitude"},
	} {
		t.Run(name, func(t *testing.T) {
			got := valueFromProto(tc.value)
			null, ok := got.(Null)
			if !ok || !strings.HasPrefix(string(null), "unsupported: ") || !strings.Contains(string(null), tc.want) {
				t.Fatalf("read as %#v, want an unsupported Null containing %q", got, tc.want)
			}
		})
	}
	well := array([]int64{2, 1}, one, one)
	if got, ok := valueFromProto(well).(Array); !ok || len(got.Elements) != 2 {
		t.Fatalf("well-formed array read as %#v", valueFromProto(well))
	}
	nested := valueFromProto(array([]int64{1}, &pb.Value{Kind: &pb.Value_Quantity{Quantity: noMagnitude}}))
	if got, ok := nested.(Array); !ok || len(got.Elements) != 1 || got.Elements[0] != Null("unsupported: quantity without a magnitude") {
		t.Fatalf("array of a magnitude-less quantity read as %#v", nested)
	}
}

// A measurement reference is refused before it leaves the client when the
// service lacks measurement_refs, however deeply nested.
func TestMeasurementRefInputIsNotSentWithoutMeasurementRefs(t *testing.T) {
	ctx := context.Background()
	model := &Model{Hash: "h"}
	metre := MeasurementRef{Unit: "m", Term: &UnitTerm{ScaleNum: 1, ScaleDen: 1, Factors: []UnitFactor{{UnitID: "SI::metre", Exponent: 1}}}}
	old := &oldCaller{t: t, capabilities: []string{CapabilityFeatureValues, CapabilityComplexValues, CapabilityStructuredValues}}
	c := &client{caller: old}
	for label, input := range map[string]Value{
		"reference":   metre,
		"nested":      Sequence{Int(1), Sequence{metre}},
		"in an array": Array{Dimensions: []int64{1}, Elements: []Value{metre}},
	} {
		_, err := c.ExecuteAction(ctx, model, "A", map[string]Value{"x": input})
		wantUnimplemented(t, "ExecuteAction "+label, err)
		_, err = c.EvaluateCalc(ctx, model, "f", Int(1), input)
		wantUnimplemented(t, "EvaluateCalc "+label, err)
	}
}

// A set or a tensor quantity is refused before it leaves the client when the
// service lacks set_values or tensor_values, however deeply nested.
func TestSetAndTensorInputsAreNotSentWithoutTheirCapabilities(t *testing.T) {
	ctx := context.Background()
	model := &Model{Hash: "h"}
	metre := Quantity{Magnitude: Real(1), Unit: "m"}
	old := &oldCaller{t: t, capabilities: []string{CapabilityFeatureValues, CapabilityComplexValues, CapabilityStructuredValues, CapabilityMeasurementRefs}}
	c := &client{caller: old}
	for label, input := range map[string]Value{
		"set":                Set{Int(1)},
		"set nested":         Sequence{Int(1), Sequence{Set{}}},
		"set in an array":    Array{Dimensions: []int64{1}, Elements: []Value{Set{Int(1)}}},
		"tensor":             TensorQuantity{Dimensions: []int64{1}, Components: []Quantity{metre}},
		"tensor nested":      Sequence{TensorQuantity{Dimensions: []int64{1}, Components: []Quantity{metre}}},
		"tensor in a set":    Set{TensorQuantity{Dimensions: []int64{1}, Components: []Quantity{metre}}},
		"tensor in an array": Array{Dimensions: []int64{1}, Elements: []Value{TensorQuantity{Dimensions: []int64{1}, Components: []Quantity{metre}}}},
	} {
		_, err := c.ExecuteAction(ctx, model, "A", map[string]Value{"x": input})
		wantUnimplemented(t, "ExecuteAction "+label, err)
		_, err = c.EvaluateCalc(ctx, model, "f", Int(1), input)
		wantUnimplemented(t, "EvaluateCalc "+label, err)
	}
}

// A malformed tensor quantity in an answer reads as an unsupported null naming
// the fault; a set reads as its elements, whatever they are.
func TestMalformedTensorAnswersAreNullsNamingTheFault(t *testing.T) {
	one := &pb.Quantity{Magnitude: &pb.Quantity_IntMagnitude{IntMagnitude: 1}, Unit: "m"}
	tensor := func(dimensions []int64, components ...*pb.Quantity) *pb.Value {
		return &pb.Value{Kind: &pb.Value_TensorQuantity{TensorQuantity: &pb.TensorQuantity{Dimensions: dimensions, Components: components}}}
	}
	for name, tc := range map[string]struct {
		value *pb.Value
		want  string
	}{
		"too few components":  {tensor([]int64{2, 2}, one, one, one), "do not fill"},
		"too many components": {tensor([]int64{2}, one, one, one), "do not fill"},
		"zero dimension":      {tensor([]int64{0}), "not positive"},
		"negative dimension":  {tensor([]int64{-1}, one), "not positive"},
		"no magnitude":        {tensor([]int64{1}, &pb.Quantity{Unit: "m"}), "without a magnitude"},
	} {
		t.Run(name, func(t *testing.T) {
			got := valueFromProto(tc.value)
			null, ok := got.(Null)
			if !ok || !strings.HasPrefix(string(null), "unsupported: ") || !strings.Contains(string(null), tc.want) {
				t.Fatalf("read as %#v, want an unsupported Null containing %q", got, tc.want)
			}
		})
	}
	set := &pb.Value{Kind: &pb.Value_Set{Set: &pb.ValueSet{Elements: []*pb.Value{
		{Kind: &pb.Value_IntValue{IntValue: 1}},
		{Kind: &pb.Value_Set{Set: &pb.ValueSet{}}},
	}}}}
	if got, want := valueFromProto(set), (Set{Int(1), Set{}}); !reflect.DeepEqual(got, want) {
		t.Errorf("set read as %#v, want %#v", got, want)
	}
	sent, err := valueToProto(Set{Int(1), Set{}})
	if err != nil || !proto.Equal(sent, set) {
		t.Errorf("set sent as %v (%v), want %v", sent, err, set)
	}
}

// A malformed measurement reference in an answer reads as an unsupported null
// naming the fault; a well-formed one reads as itself, reduction and identity
// intact.
func TestMalformedMeasurementRefAnswersAreNullsNamingTheFault(t *testing.T) {
	ref := func(pm *pb.MeasurementRef) *pb.Value {
		return &pb.Value{Kind: &pb.Value_MeasurementRef{MeasurementRef: pm}}
	}
	for name, tc := range map[string]struct {
		value *pb.Value
		want  string
	}{
		"empty":                {ref(&pb.MeasurementRef{}), "naming no unit"},
		"unreduced by text":    {ref(&pb.MeasurementRef{Unit: "m"}), "without its reduction"},
		"unreduced by unit_id": {ref(&pb.MeasurementRef{UnitId: "SI::metre"}), "without its reduction"},
	} {
		t.Run(name, func(t *testing.T) {
			got := valueFromProto(tc.value)
			null, ok := got.(Null)
			if !ok || !strings.HasPrefix(string(null), "unsupported: ") || !strings.Contains(string(null), tc.want) {
				t.Fatalf("read as %#v, want an unsupported Null containing %q", got, tc.want)
			}
		})
	}
	term := &pb.UnitTerm{ScaleNum: 1000, ScaleDen: 1, Factors: []*pb.UnitFactor{{UnitId: "SI::metre", Exponent: 1}}}
	got := valueFromProto(ref(&pb.MeasurementRef{Unit: "km", UnitTerm: term, UnitId: "SI::kilometre"}))
	want := MeasurementRef{Unit: "km", Term: &UnitTerm{ScaleNum: 1000, ScaleDen: 1, Factors: []UnitFactor{{UnitID: "SI::metre", Exponent: 1}}}, UnitID: "SI::kilometre"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("well-formed reference read as %#v, want %#v", got, want)
	}
	sent, err := valueToProto(want)
	if err != nil {
		t.Fatalf("valueToProto: %v", err)
	}
	if !proto.Equal(sent, ref(&pb.MeasurementRef{Unit: "km", UnitTerm: term, UnitId: "SI::kilometre"})) {
		t.Fatalf("marshalled as %v", sent)
	}
}

func TestAValueArmThisClientPredatesIsANullNamingIt(t *testing.T) {
	// A newer service's arm parses as an unknown field: a Value with no kind.
	got := valueFromProto(&pb.Value{})
	null, ok := got.(Null)
	if !ok || !strings.Contains(string(null), "does not know") {
		t.Fatalf("unknown arm read as %#v, want a Null naming it", got)
	}
	if plain := valueFromProto(&pb.Value{Kind: &pb.Value_Null{}}); plain != Null("") {
		t.Fatalf("plain null read as %#v", plain)
	}
}
