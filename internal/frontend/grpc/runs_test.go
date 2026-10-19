package grpc

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// runsModelSource declares an action drawing a Real and a Simulation::MonteCarlo
// case over a part whose behavior draws one.
const runsModelSource = `package MC {
	private import ScalarValues::*;
	private import RandomFunctions::*;
	action def Settle { out t : Real; first start; then assign t := uniform(1.0, 5.0); then done; }
	part def Probe {
		attribute t : Real;
		action settle { first start; then assign t := uniform(1.0, 5.0); then done; }
	}
	individual def probe :> Probe;
	analysis def Mc :> Simulation::MonteCarlo {
		subject analysed : Probe;
		perform action run ::> analysed.settle;
		attribute :>> observed : Real = analysed.t;
		return Mean : Real = mean;
		out N : Natural = runs;
	}
}`

func runRuns(t *testing.T, srv *Service, req *pb.RunRunsRequest) *pb.RunRunsResponse {
	t.Helper()
	resp, err := srv.RunRuns(context.Background(), req)
	if err != nil {
		t.Fatalf("RunRuns: %v", err)
	}
	return resp
}

func seedOf(n uint64) *uint64 { return &n }

// TestRunRunsActionTablesTheObservables verifies the runs of an action make one
// row per run, in run order, each observing the features asked for and the
// clock, and that every observable's distribution is summarised.
func TestRunRunsActionTablesTheObservables(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, runsModelSource, "runs-action")

	resp := runRuns(t, srv, &pb.RunRunsRequest{
		ModelHash: hash, ActionSymbolId: "MC::Settle", Runs: 5, Seed: seedOf(7), Observe: []string{"t"},
	})
	if resp.Error != "" {
		t.Fatalf("RunRuns reported %q", resp.Error)
	}
	if len(resp.Rows) != 5 {
		t.Fatalf("ran %d row(s); want 5", len(resp.Rows))
	}
	if resp.Seed == nil || *resp.Seed != 7 {
		t.Errorf("seed = %v; want 7", resp.Seed)
	}
	if strings.Join(resp.Observables, ",") != "t" {
		t.Errorf("observables = %v; want [t]", resp.Observables)
	}
	for i, row := range resp.Rows {
		if row.Error != "" {
			t.Fatalf("row %d failed: %s", i, row.Error)
		}
		if len(row.Inputs) != 1 || row.Inputs[0].Name != "run" {
			t.Fatalf("row %d inputs = %v; want the run number", i, row.Inputs)
		}
		if len(row.Outputs) != 1 || row.Outputs[0].Name != "t" {
			t.Fatalf("row %d outputs = %v; want t", i, row.Outputs)
		}
	}
	if len(resp.Distributions) != 1 {
		t.Fatalf("distributions = %v; want one", resp.Distributions)
	}
	d := resp.Distributions[0]
	if d.Name != "t" || !d.Numeric || d.Count != 5 || d.Min == nil || d.Max == nil || d.Mean == nil || len(d.Percentiles) != 2 {
		t.Errorf("distribution of t = %v; want 5 numbers with extremes, mean and percentiles", d)
	}
	if len(resp.Lines) == 0 || !strings.HasPrefix(resp.Lines[0], "t: 5 run(s), min ") {
		t.Errorf("lines = %q; want the distribution line first", resp.Lines)
	}

	again := runRuns(t, srv, &pb.RunRunsRequest{
		ModelHash: hash, ActionSymbolId: "MC::Settle", Runs: 5, Seed: seedOf(7), Observe: []string{"t"},
	})
	for i := range resp.Rows {
		if resp.Rows[i].Outputs[0].Value.GetRealValue() != again.Rows[i].Outputs[0].Value.GetRealValue() {
			t.Fatalf("row %d drew differently under the same seed", i)
		}
	}
}

// TestRunRunsFixedDrawsNeedNoSeed verifies the runs are seedless under a fixed
// draw policy and that an observable no run produced is the table's error.
func TestRunRunsFixedDrawsNeedNoSeed(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, runsModelSource, "runs-fixed")

	resp := runRuns(t, srv, &pb.RunRunsRequest{ModelHash: hash, ActionSymbolId: "MC::Settle", Runs: 2, Draws: "average"})
	if resp.Error != "" || len(resp.Rows) != 2 || resp.Seed != nil {
		t.Fatalf("seedless average runs: error %q, %d row(s), seed %v", resp.Error, len(resp.Rows), resp.Seed)
	}
	if got := soleElement(t, "t", resp.Rows[0].Outputs[0].Value).GetRealValue(); got != 3 {
		t.Errorf("average draw of uniform(1, 5) = %v; want 3", got)
	}
	if strings.Join(resp.Observables, ",") != "t,clock" {
		t.Errorf("observables = %v; want every feature and the clock", resp.Observables)
	}

	missing := runRuns(t, srv, &pb.RunRunsRequest{ModelHash: hash, ActionSymbolId: "MC::Settle", Runs: 2, Draws: "min", Observe: []string{"nope"}})
	if !strings.Contains(missing.Error, "nope") || missing.FailureReason == pb.FailureReason_FAILURE_REASON_UNSPECIFIED {
		t.Errorf("unobserved feature: error %q, reason %v; want a typed error naming it", missing.Error, missing.FailureReason)
	}
}

// TestRunRunsMonteCarloCaseConcludes verifies a Simulation::MonteCarlo case's
// runs report its observed value per row and the conclusion over the sample.
func TestRunRunsMonteCarloCaseConcludes(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, runsModelSource, "runs-case")

	resp := runRuns(t, srv, &pb.RunRunsRequest{
		ModelHash: hash, AnalysisSymbolId: "MC::Mc", SubjectSymbolId: "MC::probe", Runs: 4, Seed: seedOf(3),
	})
	if resp.Error != "" || resp.Unconcluded != "" {
		t.Fatalf("RunRuns reported error %q, unconcluded %q", resp.Error, resp.Unconcluded)
	}
	if len(resp.Rows) != 4 || strings.Join(resp.Observables, ",") != "observed" {
		t.Fatalf("%d row(s) of %v; want 4 of observed", len(resp.Rows), resp.Observables)
	}
	outputs := make(map[string]*pb.Value)
	for _, out := range resp.Outputs {
		outputs[out.Name] = out.Value
	}
	if outputs["Mean"] == nil || outputs["N"] == nil {
		t.Fatalf("conclusion outputs = %v; want Mean and N", resp.Outputs)
	}
	if got := outputs["N"].GetIntValue(); got != 4 {
		t.Errorf("N = %d; want 4", got)
	}
	if len(resp.Lines) == 0 || resp.Lines[len(resp.Lines)-1] != "✓ Mc over 4 run(s)" {
		t.Errorf("lines = %q; want the conclusion last", resp.Lines)
	}
	// The conclusion is drawn in the last run's context: its objects are the last
	// row's, numbered as that row was, and no object is listed twice.
	ids := make(map[int64]bool, len(resp.Instances))
	for _, inst := range resp.Instances {
		if ids[inst.Id] {
			t.Errorf("object %d listed twice", inst.Id)
		}
		ids[inst.Id] = true
	}
	lastRow := resp.Rows[len(resp.Rows)-1]
	rowObjects := make(map[int64]bool)
	for _, v := range lastRow.Verdicts {
		rowObjects[v.InstanceId] = true
	}
	for _, v := range resp.Verdicts {
		if v.InstanceId == 0 {
			continue
		}
		if !ids[v.InstanceId] {
			t.Errorf("conclusion verdict names object %d, which no instance carries", v.InstanceId)
		}
		if len(rowObjects) > 0 && !rowObjects[v.InstanceId] {
			t.Errorf("conclusion verdict names object %d, not the last row's %v", v.InstanceId, rowObjects)
		}
	}
}

// TestGRPCRobustnessRunRuns verifies the typed refusals of RunRuns: an
// unreadable request is INVALID_ARGUMENT, an unknown behavior an in-band error.
func TestGRPCRobustnessRunRuns(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, runsModelSource, "runs-robust")
	invalid := []struct {
		name string
		req  *pb.RunRunsRequest
	}{
		{"no behavior", &pb.RunRunsRequest{ModelHash: hash, Runs: 1, Seed: seedOf(1)}},
		{"both behaviors", &pb.RunRunsRequest{ModelHash: hash, ActionSymbolId: "MC::Settle", AnalysisSymbolId: "MC::Mc", Runs: 1, Seed: seedOf(1)}},
		{"no runs", &pb.RunRunsRequest{ModelHash: hash, ActionSymbolId: "MC::Settle", Seed: seedOf(1)}},
		{"random without seed", &pb.RunRunsRequest{ModelHash: hash, ActionSymbolId: "MC::Settle", Runs: 1}},
		{"unknown draws", &pb.RunRunsRequest{ModelHash: hash, ActionSymbolId: "MC::Settle", Runs: 1, Draws: "sometimes"}},
		{"negative clock step", &pb.RunRunsRequest{ModelHash: hash, ActionSymbolId: "MC::Settle", Runs: 1, Seed: seedOf(1), ClockStep: -1}},
		{"case observes", &pb.RunRunsRequest{ModelHash: hash, AnalysisSymbolId: "MC::Mc", Runs: 1, Seed: seedOf(1), Observe: []string{"t"}}},
		{"action with arguments", &pb.RunRunsRequest{ModelHash: hash, ActionSymbolId: "MC::Settle", Runs: 1, Seed: seedOf(1), Arguments: []*pb.Value{realProto(1)}}},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			_, err := srv.RunRuns(context.Background(), tc.req)
			if connect.CodeOf(err) != connect.CodeInvalidArgument {
				t.Fatalf("got %v; want INVALID_ARGUMENT", err)
			}
		})
	}
	t.Run("unknown model", func(t *testing.T) {
		_, err := srv.RunRuns(context.Background(), &pb.RunRunsRequest{ModelHash: "nope", ActionSymbolId: "MC::Settle", Runs: 1, Seed: seedOf(1)})
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("got %v; want NOT_FOUND", err)
		}
	})
	t.Run("unknown action", func(t *testing.T) {
		resp := runRuns(t, srv, &pb.RunRunsRequest{ModelHash: hash, ActionSymbolId: "MC::Nope", Runs: 1, Seed: seedOf(1)})
		if !strings.Contains(resp.Error, "MC::Nope") {
			t.Fatalf("error = %q; want the action named", resp.Error)
		}
	})
	t.Run("not a Monte Carlo case", func(t *testing.T) {
		resp := runRuns(t, srv, &pb.RunRunsRequest{ModelHash: hash, AnalysisSymbolId: "MC::Probe", Runs: 1, Seed: seedOf(1)})
		if resp.Error == "" || resp.FailureReason != pb.FailureReason_FAILURE_REASON_WRONG_KIND {
			t.Fatalf("error %q, reason %v; want WRONG_KIND", resp.Error, resp.FailureReason)
		}
	})
	t.Run("cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := srv.RunRuns(ctx, &pb.RunRunsRequest{ModelHash: hash, ActionSymbolId: "MC::Settle", Runs: 3, Seed: seedOf(1)})
		if err == nil {
			t.Log("a cancelled context answered before the runs noticed; the runs end all the same")
		}
	})
}
