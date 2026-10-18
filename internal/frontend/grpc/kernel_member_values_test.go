package grpc

import (
	"context"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

const kernelMemberModel = `package Probe {
	part def P1 { part pa; part pb; attribute x : ScalarValues::Integer = 1; }
	part p1 : P1;
}`

// A session reads an object's Kernel Semantic Library members by name as an
// expression does: `self` is the object, an open required collection is sent as
// the undetermined value rather than an empty list, and a name the type lacks is
// refused naming the type.
func TestSessionFeatureValueReadsKernelMembers(t *testing.T) {
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: kernelMemberModel},
		ContentHash: "kernel-member-values",
	})
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	ss, err := srv.OpenSession(parsed.ModelHash)
	if err != nil {
		t.Fatalf("OpenSession failed: %v", err)
	}
	id, err := ss.Instantiate("Probe::P1")
	if err != nil {
		t.Fatalf("Instantiate failed: %v", err)
	}

	self, err := ss.FeatureValue(id, "self")
	if err != nil || self.GetValue() == nil {
		t.Fatalf("FeatureValue(self) = %v, %v; want the object", self, err)
	}
	snapshots, err := ss.FeatureValue(id, "snapshots")
	if err != nil {
		t.Fatalf("FeatureValue(snapshots) failed: %v", err)
	}
	if _, ok := snapshots.GetValue().GetKind().(*pb.Value_Undetermined); !ok || len(snapshots.GetValues()) != 0 {
		kind, value := describeValue(snapshots.GetValue())
		t.Errorf("snapshots: %s %v with %d values, want an undetermined value", kind, value, len(snapshots.GetValues()))
	}
	predecessors, err := ss.FeatureValue(id, "predecessors")
	if err != nil || predecessors.GetValue() != nil || len(predecessors.GetValues()) != 0 {
		t.Errorf("predecessors = %v, %v; want an empty collection", predecessors, err)
	}
	if _, err := ss.FeatureValue(id, "ownedParts"); err == nil || !strings.Contains(err.Error(), "has no member ownedParts") {
		t.Errorf("FeatureValue(ownedParts) = %v, want a refusal naming the type", err)
	}
}
