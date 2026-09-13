package grpc

import (
	"context"
	"errors"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

const unsetFeatureValueModel = `
package Demo {
  private import ScalarValues::*;
  part def Engine {
    attribute power : Real = 120.0;
  }
  part def Vehicle {
    attribute d : Real;
    attribute ds : Real[2];
    attribute k : Real = 2.0;
    part engine : Engine;
  }
}
`

// A feature value that holds no value is sent as such, so a client reads "no value" rather
// than the id of an object with nothing in it. A valued feature value and an object-valued
// one are unaffected.
func TestInstantiate_ValuelessValueTypedFeatureValueIsSentUnset(t *testing.T) {
	resp := instantiate(t, unsetFeatureValueModel, "unset-feature-value", "Demo::Vehicle")
	fvs := resp.GetInstance().GetFeatureValues()

	d := fvs["d"]
	if !d.GetMaterialized() {
		t.Errorf("feature value d: materialized = false, want true: the object is created either way")
	}
	if _, ok := d.GetValue().GetKind().(*pb.Value_Unset); !ok {
		kind, value := describeValue(d.GetValue())
		t.Errorf("feature value d: %s %v, want unset", kind, value)
	}

	for i, elem := range fvs["ds"].GetValues() {
		if _, ok := elem.GetKind().(*pb.Value_Unset); !ok {
			kind, value := describeValue(elem)
			t.Errorf("feature value ds[%d]: %s %v, want unset", i, kind, value)
		}
	}

	if fvs["k"].GetValue().GetRealValue() != 2.0 {
		kind, value := describeValue(fvs["k"].GetValue())
		t.Errorf("feature value k: %s %v, want real 2", kind, value)
	}
	if _, ok := fvs["engine"].GetValue().GetKind().(*pb.Value_InstanceId); !ok {
		kind, value := describeValue(fvs["engine"].GetValue())
		t.Errorf("feature value engine: %s %v, want an instance id", kind, value)
	}
}

// The empty object a valueless value-typed feature materializes is not reachable
// as a value, so it is not sent as one of the graph's instances either.
func TestInstantiate_UnsetFeatureValueContributesNoInstance(t *testing.T) {
	resp := instantiate(t, unsetFeatureValueModel, "unset-feature-value-graph", "Demo::Vehicle")
	if len(resp.Instances) != 2 {
		ids := make([]int64, 0, len(resp.Instances))
		for _, inst := range resp.Instances {
			ids = append(ids, inst.Id)
		}
		t.Errorf("instances = %v, want the object and its engine", ids)
	}
}

// Unset says what a feature value holds, which is something to read and not to supply, so
// a caller sending it is told so rather than having it read as some value.
func TestProtoToValue_RejectsUnset(t *testing.T) {
	_, err := ProtoToValueIn(&pb.Value{Kind: &pb.Value_Unset{Unset: true}}, nil, nil)
	if !errors.Is(err, ErrUnsetNotAccepted) {
		t.Errorf("err = %v, want %v", err, ErrUnsetNotAccepted)
	}

	seq := &pb.Value{Kind: &pb.Value_Sequence{Sequence: &pb.ValueSequence{
		Elements: []*pb.Value{{Kind: &pb.Value_IntValue{IntValue: 1}}, {Kind: &pb.Value_Unset{Unset: true}}},
	}}}
	if _, err := ProtoToValueIn(seq, nil, nil); !errors.Is(err, ErrUnsetNotAccepted) {
		t.Errorf("in a sequence: err = %v, want %v", err, ErrUnsetNotAccepted)
	}
}

// A calc answers with the same spelling as every other RPC: a result or output
// the model gives no value is sent undetermined rather than as an object reference.
func TestEvaluateCalc_ValuelessOutputIsSentUndetermined(t *testing.T) {
	srv := mustNewService(t, 10)
	source := `package Demo {
	private import ScalarValues::*;
	part def Q {
		attribute d : Real;
	}
	part q : Q;
	calc def Read {
		out r = q.d;
	}
	calc reading : Read;
}
`
	hash := mustVerifyModel(t, srv, source, "verify-calc-unset")

	resp, err := srv.EvaluateCalc(context.Background(), &pb.EvaluateCalcRequest{
		ModelHash: hash,
		SymbolId:  "Demo::reading",
	})
	if err != nil {
		t.Fatalf("EvaluateCalc: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("EvaluateCalc reported %q", resp.Error)
	}
	if len(resp.Outputs) != 1 {
		t.Fatalf("got %d outputs, want r: %v", len(resp.Outputs), resp.Outputs)
	}
	if _, ok := resp.Outputs[0].GetValue().GetKind().(*pb.Value_Undetermined); !ok {
		kind, value := describeValue(resp.Outputs[0].GetValue())
		t.Errorf("output r: %s %v, want undetermined", kind, value)
	}

	invoked, err := srv.EvaluateCalc(context.Background(), &pb.EvaluateCalcRequest{
		ModelHash: hash,
		SymbolId:  "Demo::Read",
	})
	if err != nil {
		t.Fatalf("EvaluateCalc of the definition: %v", err)
	}
	if invoked.Error != "" {
		t.Fatalf("EvaluateCalc of the definition reported %q", invoked.Error)
	}
	if _, ok := invoked.GetResult().GetKind().(*pb.Value_Undetermined); !ok {
		kind, value := describeValue(invoked.GetResult())
		t.Errorf("result: %s %v, want undetermined", kind, value)
	}
}

// The unset arm is a wire-visible addition, so it is advertised as a capability
// a client can require rather than left for a client to discover.
func TestCapabilities_IncludeUnsetValue(t *testing.T) {
	for _, name := range Capabilities() {
		if name == CapabilityUnsetValue {
			return
		}
	}
	t.Errorf("capabilities = %v, want one named %q", Capabilities(), CapabilityUnsetValue)
}
