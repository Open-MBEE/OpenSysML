package grpc

import (
	"context"
	"fmt"
	"slices"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
)

// TestToolPreviewLeavesTheHeldPopulationAsItWas verifies that a subjectless tool dry run
// leaves the held population as it found it: no preview object held, the prior runner back on.
func TestToolPreviewLeavesTheHeldPopulationAsItWas(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, heldPopulationModel, "held-population-preview")
	ctx := context.Background()
	if made, err := srv.Instantiate(ctx, &pb.InstantiateRequest{ModelHash: hash, SymbolId: "test::fleet"}); err != nil || made.Error != "" {
		t.Fatalf("Instantiate(test::fleet): %v %q", err, made.GetError())
	}
	cached, _ := srv.cache.Get(hash)
	rt := srv.objects(cached).rt
	plain := &pb.RunAnalysisRequest{ModelHash: hash, SymbolId: "test::Definition::costTest"}
	first, err := srv.RunAnalysis(ctx, plain)
	if err != nil || first.Error != "" {
		t.Fatalf("RunAnalysis before the preview: %v %q", err, first.GetError())
	}
	// A run keeps the performance it made, as the REPL's does; a preview keeps nothing.
	before := rt.InstanceCount()

	resp, err := srv.RunAnalysis(ctx, &pb.RunAnalysisRequest{ModelHash: hash, SymbolId: "test::Definition::costTest", ToolDryRun: true})
	if err != nil {
		t.Fatalf("RunAnalysis tool dry run: %v", err)
	}
	if resp.Error != msgNoToolReached {
		t.Errorf("preview of a case calling no tool reported %q, want %q", resp.Error, msgNoToolReached)
	}
	if got := rt.InstanceCount(); got != before {
		t.Errorf("held after the preview = %d objects, want the %d held before it", got, before)
	}
	if _, dry := rt.ToolRunner().(*analysis.DryRunner); dry {
		t.Error("the preview's dry runner is still on the held population's runtime")
	}

	// The run after the preview is the run before it: a run, not a preview, on the same objects.
	run, err := srv.RunAnalysis(ctx, plain)
	if err != nil {
		t.Fatalf("RunAnalysis after the preview: %v", err)
	}
	if run.Error != "" || run.ToolDryRun != nil {
		t.Errorf("run after the preview reported %q with preview %v, want a run of the case", run.Error, run.ToolDryRun)
	}
	spell := func(verdicts []*pb.VerificationVerdict) []string {
		out := make([]string, len(verdicts))
		for i, v := range verdicts {
			out[i] = fmt.Sprintf("%s=%s on #%d", v.CaseId, v.Kind, v.InstanceId)
		}
		return out
	}
	if got, want := spell(run.VerificationVerdicts), spell(first.VerificationVerdicts); !slices.Equal(got, want) {
		t.Errorf("run after the preview = %v, want the run before it, %v", got, want)
	}
}
