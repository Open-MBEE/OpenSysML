package grpc

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// TestGRPCRobustnessHeldPopulation: a subjectless sweep that would pass the held-objects
// bound is RESOURCE_EXHAUSTED, the population unchanged, a named-subject check unaffected.
func TestGRPCRobustnessHeldPopulation(t *testing.T) {
	t.Setenv(HeldObjectsEnvVar, "3")
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, heldPopulationModel, "held-population-robustness")
	ctx := context.Background()
	if made, err := srv.Instantiate(ctx, &pb.InstantiateRequest{ModelHash: hash, SymbolId: "test::fleet"}); err != nil || made.Error != "" {
		t.Fatalf("Instantiate(test::fleet): %v %q", err, made.GetError())
	}
	exhausted := func(t *testing.T, rpc string, err error) {
		t.Helper()
		if connect.CodeOf(err) != connect.CodeResourceExhausted {
			t.Fatalf("%s at the bound: code = %v, want RESOURCE_EXHAUSTED: %v", rpc, connect.CodeOf(err), err)
		}
		for _, text := range []string{"holds 3 objects", HeldObjectsEnvVar} {
			if !strings.Contains(err.Error(), text) {
				t.Errorf("%s error %q lacks %q", rpc, err.Error(), text)
			}
		}
	}

	t.Run("requirement sweep at the bound", func(t *testing.T) {
		_, err := srv.VerifyRequirement(ctx, &pb.VerifyRequirementRequest{ModelHash: hash, SymbolId: "test::costReq"})
		exhausted(t, "VerifyRequirement", err)
	})
	t.Run("case run at the bound", func(t *testing.T) {
		_, err := srv.RunAnalysis(ctx, &pb.RunAnalysisRequest{ModelHash: hash, SymbolId: "test::Definition::costTest"})
		exhausted(t, "RunAnalysis", err)
	})
	t.Run("satisfaction sweep at the bound", func(t *testing.T) {
		_, err := srv.VerifySatisfaction(ctx, &pb.VerifySatisfactionRequest{ModelHash: hash})
		exhausted(t, "VerifySatisfaction", err)
	})
	t.Run("population unchanged", func(t *testing.T) {
		cached, _ := srv.cache.Get(hash)
		if got := srv.objects(cached).rt.InstanceCount(); got != 3 {
			t.Errorf("held after the refused sweeps = %d, want 3", got)
		}
	})
	t.Run("named subject on its own worker", func(t *testing.T) {
		resp, err := srv.VerifyRequirement(ctx, &pb.VerifyRequirementRequest{ModelHash: hash, SymbolId: "test::costReq", SubjectSymbolId: "test::fleet"})
		if err != nil {
			t.Fatalf("VerifyRequirement on fleet: %v", err)
		}
		wantVerdicts(t, "VerifyRequirement on fleet", resp.VerificationVerdicts, "test::Definition::costTest=pass", "test::Definition::costTest=pass")
	})
}
