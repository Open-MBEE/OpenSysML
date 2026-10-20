package grpc

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// recordModelSource declares an analysis case over a part, for recording.
const recordModelSource = `package A {
	private import ScalarValues::*;
	part def P { attribute m : Real = 2.0; }
	part p : P;
	analysis def Cost { subject s : P; return total : Real = s.m * 3.0; }
}`

func recordRun(t *testing.T, srv *Service, req *pb.RecordRunRequest) *pb.RecordRunResponse {
	t.Helper()
	resp, err := srv.RecordRun(context.Background(), req)
	if err != nil {
		t.Fatalf("RecordRun: %v", err)
	}
	return resp
}

// TestRecordRunRecordsOneRun verifies one run of a case is recorded into a
// Records package beside the case's own, that the recorded model is cached
// under the answered hash, and that a second recording joins the first.
func TestRecordRunRecordsOneRun(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, recordModelSource, "record-one")

	resp := recordRun(t, srv, &pb.RecordRunRequest{ModelHash: hash, AnalysisSymbolId: "A::Cost", SubjectSymbolId: "A::p"})
	if resp.Error != "" {
		t.Fatalf("RecordRun reported %q", resp.Error)
	}
	if resp.Run == nil || resp.Run.Error != "" || len(resp.Run.Outputs) != 1 || soleElement(t, "total", resp.Run.Outputs[0].Value).GetRealValue() != 6 {
		t.Fatalf("run = %v; want total 6", resp.Run)
	}
	if resp.Package != "Records" || resp.Definition != "Records::CostRun" || strings.Join(resp.Records, ",") != "Records::Cost_run1" {
		t.Errorf("recorded %s %s %v; want Records::CostRun and Records::Cost_run1", resp.Package, resp.Definition, resp.Records)
	}
	if !strings.Contains(resp.Source, "Cost_run1") || resp.ModelHash == "" || resp.ModelHash == hash {
		t.Errorf("source %q, model hash %q; want the records and a new model", resp.Source, resp.ModelHash)
	}
	if len(resp.Documents) != 1 || resp.Documents[0].Name != "Records.sysml" || resp.Documents[0].Content != resp.Source || resp.Content != "" {
		t.Errorf("documents = %v, content %q; want the records as a document of their own", resp.Documents, resp.Content)
	}

	again := recordRun(t, srv, &pb.RecordRunRequest{ModelHash: resp.ModelHash, AnalysisSymbolId: "A::Cost", SubjectSymbolId: "A::p", AcceptDocuments: true})
	if again.Error != "" {
		t.Fatalf("second RecordRun reported %q", again.Error)
	}
	if strings.Join(again.Records, ",") != "Records::Cost_run2" || len(again.Documents) != 1 ||
		!strings.Contains(again.Documents[0].Content, "Cost_run1") || !strings.Contains(again.Documents[0].Content, "Cost_run2") {
		t.Errorf("second recording made %v in %v; want Cost_run2 beside Cost_run1", again.Records, again.Documents)
	}
	sym, err := srv.GetSymbol(context.Background(), &pb.GetSymbolRequest{ModelHash: again.ModelHash, SymbolId: "Records::Cost_run2"})
	if err != nil || sym.GetError() != "" {
		t.Errorf("Records::Cost_run2 in the recorded model: %v %v", err, sym)
	}
}

// TestRecordRunRecordsASweep verifies each completed row of a sweep is a record.
func TestRecordRunRecordsASweep(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, sweepModelSource, "record-sweep")

	resp := recordRun(t, srv, &pb.RecordRunRequest{
		ModelHash: hash, AnalysisSymbolId: "Sw::CostAnalysis", SubjectSymbolId: "Sw::ship",
		Ranges: []*pb.SweepRange{{Parameter: "limit", Start: realProto(2), End: realProto(30), Step: realProto(14)}},
	})
	if resp.Error != "" {
		t.Fatalf("RecordRun reported %q", resp.Error)
	}
	if resp.Sweep == nil || len(resp.Sweep.Rows) != 3 || len(resp.Records) != 3 || resp.Skipped != 0 {
		t.Fatalf("sweep %v, records %v, skipped %d; want 3 rows recorded", resp.Sweep, resp.Records, resp.Skipped)
	}
}

// TestRecordRunRecordsASample verifies a Monte Carlo case's completed runs
// and the sample's conclusion are recorded.
func TestRecordRunRecordsASample(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, runsModelSource, "record-sample")

	resp := recordRun(t, srv, &pb.RecordRunRequest{
		ModelHash: hash, AnalysisSymbolId: "MC::Mc", SubjectSymbolId: "MC::probe", Runs: 3, Seed: seedOf(1),
	})
	if resp.Error != "" {
		t.Fatalf("RecordRun reported %q", resp.Error)
	}
	if resp.Sample == nil || len(resp.Sample.Rows) != 3 || resp.Sample.Unconcluded != "" {
		t.Fatalf("sample = %v; want 3 concluded rows", resp.Sample)
	}
	if len(resp.Records) != 4 || len(resp.Reasons) != 0 {
		t.Errorf("records %v, reasons %v; want three runs and the sample", resp.Records, resp.Reasons)
	}
}

// TestGRPCRobustnessRecordRun verifies the typed refusals of RecordRun.
func TestGRPCRobustnessRecordRun(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, recordModelSource, "record-robust")
	invalid := []struct {
		name string
		req  *pb.RecordRunRequest
	}{
		{"no case", &pb.RecordRunRequest{ModelHash: hash}},
		{"negative runs", &pb.RecordRunRequest{ModelHash: hash, AnalysisSymbolId: "A::Cost", Runs: -1}},
		{"runs and ranges", &pb.RecordRunRequest{ModelHash: hash, AnalysisSymbolId: "A::Cost", Runs: 2, Seed: seedOf(1), Ranges: []*pb.SweepRange{{Parameter: "x"}}}},
		{"runs without seed", &pb.RecordRunRequest{ModelHash: hash, AnalysisSymbolId: "A::Cost", Runs: 2}},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			_, err := srv.RecordRun(context.Background(), tc.req)
			if connect.CodeOf(err) != connect.CodeInvalidArgument {
				t.Fatalf("got %v; want INVALID_ARGUMENT", err)
			}
		})
	}
	t.Run("unknown model", func(t *testing.T) {
		_, err := srv.RecordRun(context.Background(), &pb.RecordRunRequest{ModelHash: "nope", AnalysisSymbolId: "A::Cost"})
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("got %v; want NOT_FOUND", err)
		}
	})
	t.Run("unknown case", func(t *testing.T) {
		resp := recordRun(t, srv, &pb.RecordRunRequest{ModelHash: hash, AnalysisSymbolId: "A::Nope"})
		if !strings.Contains(resp.Error, "A::Nope") || len(resp.Records) != 0 || resp.ModelHash != "" {
			t.Fatalf("unknown case: %v; want the error alone", resp)
		}
	})
	t.Run("not a case", func(t *testing.T) {
		resp := recordRun(t, srv, &pb.RecordRunRequest{ModelHash: hash, AnalysisSymbolId: "A::P"})
		if resp.Error == "" || resp.FailureReason != pb.FailureReason_FAILURE_REASON_WRONG_KIND {
			t.Fatalf("part def as case: error %q, reason %v; want WRONG_KIND", resp.Error, resp.FailureReason)
		}
	})
	t.Run("into a non-package", func(t *testing.T) {
		resp := recordRun(t, srv, &pb.RecordRunRequest{ModelHash: hash, AnalysisSymbolId: "A::Cost", SubjectSymbolId: "A::p", RecordInto: "A::P"})
		if !strings.Contains(resp.Error, "not a package") || resp.ModelHash != "" {
			t.Fatalf("error = %q; want the target refused", resp.Error)
		}
	})
	t.Run("several documents unaccepted", func(t *testing.T) {
		two := mustParseTwoDocuments(t, srv)
		_, err := srv.RecordRun(context.Background(), &pb.RecordRunRequest{ModelHash: two, AnalysisSymbolId: "A::Cost"})
		if connect.CodeOf(err) != connect.CodeFailedPrecondition {
			t.Fatalf("got %v; want FAILED_PRECONDITION", err)
		}
	})
}
