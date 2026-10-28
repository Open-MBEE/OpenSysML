package grpc_test

import (
	"context"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
)

// expectedVerificationVerdict is the fixture encoding of a pb.VerificationVerdict:
// the case, the verdict kind its body answered, the detail of one that decided
// nothing, and the type of the object it ran on (empty for a run on none).
type expectedVerificationVerdict struct {
	Case         string `json:"case"`
	Kind         string `json:"kind"`
	Detail       string `json:"detail"`
	InstanceType string `json:"instance_type"`
}

// runVerifyRequirementCase instantiates the case's objects, verifies the requirement
// about the subject named (none when empty) and compares the requirement's status
// with the body verdicts of the cases verifying it, each about its own object.
func runVerifyRequirementCase(t *testing.T, srv *grpc.Service, ctx context.Context, modelHash string, tc conformanceCase) {
	t.Helper()
	instantiateAll(t, srv, ctx, modelHash, tc.Instantiate)

	resp, err := srv.VerifyRequirement(ctx, &pb.VerifyRequirementRequest{
		ModelHash:       modelHash,
		SymbolId:        tc.SymbolID,
		SubjectSymbolId: tc.SubjectSymbolID,
	})
	if err != nil {
		t.Fatalf("VerifyRequirement: %v", err)
	}
	if checkExpectedError(t, tc, resp.Error) {
		return
	}
	if resp.Verdict.Status != tc.ExpectedStatus {
		t.Errorf("status = %q (%s), want %q", resp.Verdict.Status, resp.Verdict.Error, tc.ExpectedStatus)
	}
	if len(resp.VerificationVerdicts) != len(tc.ExpectedVerificationVerdicts) {
		t.Fatalf("verification verdicts = %v, want %d", resp.VerificationVerdicts, len(tc.ExpectedVerificationVerdicts))
	}
	for i, want := range tc.ExpectedVerificationVerdicts {
		got := resp.VerificationVerdicts[i]
		if got.CaseId != want.Case || got.Kind != want.Kind || got.Detail != want.Detail || got.InstanceTypeId != want.InstanceType {
			t.Errorf("verification verdict %d = %v, want %+v", i, got, want)
		}
		if (got.InstanceId != 0) != (want.InstanceType != "") {
			t.Errorf("verification verdict %d names object #%d, want an object: %v", i, got.InstanceId, want.InstanceType != "")
		}
	}
}

// instantiateAll creates an object of each symbol named, in order, so the
// request that follows is about the objects the model holds.
func instantiateAll(t *testing.T, srv *grpc.Service, ctx context.Context, modelHash string, symbols []string) {
	t.Helper()
	for _, sym := range symbols {
		resp, err := srv.Instantiate(ctx, &pb.InstantiateRequest{ModelHash: modelHash, SymbolId: sym})
		if err != nil {
			t.Fatalf("Instantiate %s: %v", sym, err)
		}
		if resp.Error != "" {
			t.Fatalf("Instantiate %s: %s", sym, resp.Error)
		}
	}
}
