package grpc

import (
	"testing"

	"google.golang.org/protobuf/proto"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// Instance.slots was the pre-0.1.0 spelling of feature_values. Its field number
// is reserved, so an object is sent under the one spelling and nothing occupies
// field 3 on the wire.
func TestInstantiate_SendsFeatureValuesAndNoSlots(t *testing.T) {
	resp := instantiate(t, unsetFeatureValueModel, "feature-values", "Demo::Vehicle")

	fields := (&pb.Instance{}).ProtoReflect().Descriptor().Fields()
	if f := fields.ByNumber(3); f != nil {
		t.Errorf("field 3 is %q, want it reserved", f.Name())
	}

	for _, inst := range append([]*pb.Instance{resp.GetInstance()}, resp.GetInstances()...) {
		if len(inst.GetFeatureValues()) == 0 {
			t.Fatalf("instance %d carries no feature values", inst.GetId())
		}
		wire, err := proto.Marshal(inst)
		if err != nil {
			t.Fatalf("marshalling instance %d: %v", inst.GetId(), err)
		}
		var got pb.Instance
		if err := proto.Unmarshal(wire, &got); err != nil {
			t.Fatalf("unmarshalling instance %d: %v", inst.GetId(), err)
		}
		if n := len(got.ProtoReflect().GetUnknown()); n != 0 {
			t.Errorf("instance %d sent %d bytes of unknown fields, want none", inst.GetId(), n)
		}
	}
}

// The field is wire-visible, so a client can require it rather than discover it.
func TestCapabilities_IncludeFeatureValues(t *testing.T) {
	for _, name := range Capabilities() {
		if name == CapabilityFeatureValues {
			return
		}
	}
	t.Errorf("capabilities = %v, want one named %q", Capabilities(), CapabilityFeatureValues)
}
