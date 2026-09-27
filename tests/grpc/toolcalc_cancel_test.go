package grpc_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
)

// A request cancelled while a tool-computed calc's tool is running ends the
// tool rather than running its timeout out: the runner is bound to the
// request's context, not the background.
func TestGRPCToolCalcHonoursACancelledRequest(t *testing.T) {
	t.Setenv("TOOL_CALC_MODE", "hang")
	t.Setenv(analysis.ToolTimeoutEnv, "10s")
	t.Setenv(analysis.ToolsEnv, conformanceToolManifest(t, []string{"toolcalc"}))

	srv, err := grpc.NewService(4, "test")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	modelData, err := os.ReadFile(filepath.Join("testdata", "conformance", "evaluate_calc_tool.sysml"))
	if err != nil {
		t.Fatalf("read model: %v", err)
	}
	parseResp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: string(modelData)},
		ContentHash: "toolcalc-cancel",
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	for _, diag := range parseResp.Diagnostics {
		if diag.Severity == "error" {
			t.Fatalf("model has a diagnostic error: %s", diag.Message)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	resp, err := srv.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{
		ModelHash: parseResp.ModelHash,
		SymbolId:  "thermoCase::Thermal",
		Arguments: []*pb.Value{
			{Kind: &pb.Value_RealValue{RealValue: 2}},
			{Kind: &pb.Value_RealValue{RealValue: 10}},
		},
	})
	elapsed := time.Since(start)
	if elapsed > 5*time.Second {
		t.Fatalf("EvaluateCalc took %s on a cancelled request, want well under the 10s tool timeout", elapsed)
	}
	var failure string
	switch {
	case err != nil:
		failure = err.Error()
	case resp != nil:
		failure = resp.Error
	}
	if !strings.Contains(failure, "cancel") {
		t.Fatalf("EvaluateCalc failure = %q, want a cancellation error", failure)
	}
}
