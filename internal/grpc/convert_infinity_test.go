package grpc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// infinityWireModel yields the unbounded value as a feature value, alone and
// nested in a sequence, beside the ordinary string that prints the same way.
const infinityWireModel = `
package U {
  private import ScalarValues::*;

  attribute unbounded = *;
  attribute star : String = "*";
  attribute bounds [*] = (1, *);

  action pass {
    in x;
    out y;
    first start;
    action inner { assign y := x; }
    then done;
    succession first start then inner;
  }
}
`

func infinityValue() *pb.Value {
	return &pb.Value{Kind: &pb.Value_Infinity{Infinity: true}}
}

// The unbounded value crosses as its own arm, distinct from the string "*" a
// client would otherwise read as text, and reads back as the same value.
func TestInfinityRoundTrip(t *testing.T) {
	idx := symbols.NewIndex()
	unbounded := runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInfinity}}

	pv := ValueToProto(unbounded, idx)
	if _, ok := pv.GetKind().(*pb.Value_Infinity); !ok {
		t.Fatalf("* crossed as %T, want the infinity arm", pv.GetKind())
	}

	back, err := ProtoToValueIn(pv, idx, nil)
	if err != nil {
		t.Fatalf("ProtoToValueIn: %v", err)
	}
	if back.Kind != runtime.ValConst || !back.Const.IsUnbounded() {
		t.Errorf("round trip of * = %s (%v)", back.Kind, back)
	}

	// The string "*" stays a string in both directions.
	str := ValueToProto(runtime.NewStringValue("*"), idx)
	if str.GetStringValue() != "*" {
		t.Errorf(`"*" crossed as %T, want a string`, str.GetKind())
	}
	strBack, err := ProtoToValueIn(str, idx, nil)
	if err != nil {
		t.Fatalf(`ProtoToValueIn("*"): %v`, err)
	}
	if strBack.Kind != runtime.ValString || strBack.Str() != "*" {
		t.Errorf(`round trip of "*" = %s (%v)`, strBack.Kind, strBack)
	}

	seq := runtime.NewSequence()
	seq.Append(runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}})
	seq.Append(unbounded)
	elements := ValueToProto(runtime.NewSequenceValue(seq), idx).GetSequence().GetElements()
	if len(elements) != 2 {
		t.Fatalf("(1, *) crossed as %d elements", len(elements))
	}
	if _, ok := elements[1].GetKind().(*pb.Value_Infinity); !ok {
		t.Errorf("(1, *)#2 crossed as %T, want the infinity arm", elements[1].GetKind())
	}
}

// Evaluating the model reports the unbounded value on the arm and the ordinary
// string as text, so a client can tell them apart.
func TestInfinityEvaluate(t *testing.T) {
	srv := mustNewService(t, 4)
	modelHash := mustParse(t, srv, infinityWireModel)

	if pv := mustEvaluate(t, srv, modelHash, "U::unbounded"); !pv.GetInfinity() {
		t.Errorf("U::unbounded = %v, want the infinity arm", pv)
	}
	if pv := mustEvaluate(t, srv, modelHash, "U::star"); pv.GetStringValue() != "*" {
		t.Errorf(`U::star = %v, want the string "*"`, pv)
	}
	bounds := mustEvaluate(t, srv, modelHash, "U::bounds").GetSequence().GetElements()
	if len(bounds) != 2 || bounds[0].GetIntValue() != 1 || !bounds[1].GetInfinity() {
		t.Errorf("U::bounds = %v, want (1, *)", bounds)
	}
}

// The arm is advertised under its own capability, so a client can require it; a
// service withholding it reports the value as unsupported — nested in a
// structured value included — and refuses one sent to it rather than reading it
// as another value. The string "*" is unaffected.
func TestInfinityCapability(t *testing.T) {
	ctx := context.Background()
	found := false
	for _, c := range Capabilities() {
		found = found || c == CapabilityInfinityValue
	}
	if !found {
		t.Errorf("capabilities %v do not include %q", Capabilities(), CapabilityInfinityValue)
	}

	withheld := mustNewServiceWithout(t, CapabilityInfinityValue)
	modelHash := mustParse(t, withheld, infinityWireModel)

	const want = "unsupported: unbounded value *"
	got := mustEvaluate(t, withheld, modelHash, "U::unbounded")
	if got.GetInfinity() || got.GetNull() != want {
		t.Errorf("U::unbounded without %s = %v, want null %q", CapabilityInfinityValue, got, want)
	}
	if pv := mustEvaluate(t, withheld, modelHash, "U::star"); pv.GetStringValue() != "*" {
		t.Errorf(`U::star without %s = %v, want the string "*"`, CapabilityInfinityValue, pv)
	}
	bounds := mustEvaluate(t, withheld, modelHash, "U::bounds").GetSequence().GetElements()
	if len(bounds) != 2 || bounds[1].GetNull() != want {
		t.Errorf("U::bounds without %s = %v, want the element withheld", CapabilityInfinityValue, bounds)
	}

	nested := &pb.Value{Kind: &pb.Value_Sequence{Sequence: &pb.ValueSequence{Elements: []*pb.Value{infinityValue()}}}}
	for name, input := range map[string]*pb.Value{"unbounded": infinityValue(), "nested": nested} {
		_, err := withheld.ExecuteAction(ctx, &pb.ExecuteActionRequest{
			ModelHash:      modelHash,
			ActionSymbolId: "U::pass",
			Inputs:         map[string]*pb.Value{"x": input},
		})
		if connect.CodeOf(err) != connect.CodeUnimplemented || !strings.Contains(err.Error(), CapabilityInfinityValue) {
			t.Errorf("ExecuteAction with %s input without %s: err = %v, want UNIMPLEMENTED naming the capability", name, CapabilityInfinityValue, err)
		}
	}
}

// TestProtoToValueRefusesAFalseInfinityArm refuses the infinity arm sent as
// false, directly and nested: the arm is the unbounded value itself, so false
// states no value and must not be read as one.
func TestProtoToValueRefusesAFalseInfinityArm(t *testing.T) {
	false_ := &pb.Value{Kind: &pb.Value_Infinity{Infinity: false}}
	for _, tc := range []struct {
		name string
		sent *pb.Value
	}{
		{"directly", false_},
		{"in a sequence", &pb.Value{Kind: &pb.Value_Sequence{Sequence: &pb.ValueSequence{Elements: []*pb.Value{false_}}}}},
		{"in an array", &pb.Value{Kind: &pb.Value_Array{Array: &pb.Array{Dimensions: []int64{1}, Elements: []*pb.Value{false_}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ProtoToValueIn(tc.sent, nil, nil); !errors.Is(err, ErrInfinityNotAsserted) {
				t.Errorf("error %v, want the infinity arm refused", err)
			}
		})
	}
	if got, err := ProtoToValueIn(infinityValue(), nil, nil); err != nil || !got.Const.IsUnbounded() {
		t.Errorf("infinity: true read as %v (%v), want the unbounded value", got.Kind, err)
	}
}
