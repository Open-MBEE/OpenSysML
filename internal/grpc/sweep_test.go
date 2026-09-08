package grpc

import (
	"context"
	"strconv"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

const sweepModelSource = `package Sw {
	private import ScalarValues::*;

	part def Ship {
		attribute cost : Real default = 5.0;
	}

	calc def Twice {
		in n : Integer;
		return : Integer = n * 2;
	}

	calc def Plus {
		in a : Integer;
		in b : Integer;
		return : Integer = a + b;
	}

	calc def Ratio {
		in a : Real;
		in b : Real;
		return : Real = a / b;
	}

	analysis def CostAnalysis {
		subject s : Ship;
		in limit : Real = 20.0;
		out total : Real = s.cost * limit;
		objective affordable {
			require constraint { total <= 100.0 }
		}
	}

	part ship : Ship;
}
`

func runSweep(t *testing.T, srv *Service, req *pb.RunSweepRequest) *pb.RunSweepResponse {
	t.Helper()
	resp, err := srv.RunSweep(context.Background(), req)
	if err != nil {
		t.Fatalf("RunSweep: %v", err)
	}
	return resp
}

// intRange is a range between two Integers, stating no step.
func intRange(param string, from, to int64) *pb.SweepRange {
	return &pb.SweepRange{Parameter: param, Start: intProto(from), End: intProto(to)}
}

func intProto(n int64) *pb.Value {
	return &pb.Value{Kind: &pb.Value_IntValue{IntValue: n}}
}

func realProto(f float64) *pb.Value {
	return &pb.Value{Kind: &pb.Value_RealValue{RealValue: f}}
}

// rowText renders a response as one line per row — its inputs, its outputs and
// its failure — so one comparison covers the rows, their order and their values.
func rowText(resp *pb.RunSweepResponse) string {
	lines := make([]string, 0, len(resp.Rows))
	for _, row := range resp.Rows {
		parts := make([]string, 0, len(row.Inputs)+len(row.Outputs)+1)
		for _, in := range row.Inputs {
			parts = append(parts, in.Name+"="+valueText(in.Value))
		}
		for _, out := range row.Outputs {
			parts = append(parts, out.Name+" -> "+valueText(out.Value))
		}
		if row.Error != "" {
			parts = append(parts, "error: "+row.Error)
		}
		lines = append(lines, strings.Join(parts, " "))
	}
	return strings.Join(lines, "\n")
}

// valueText renders the kinds of value a sweep binds and returns.
func valueText(val *pb.Value) string {
	switch val.GetKind().(type) {
	case *pb.Value_IntValue:
		return strconv.FormatInt(val.GetIntValue(), 10)
	case *pb.Value_RealValue:
		return semantics.FormatReal(val.GetRealValue())
	}
	return val.String()
}

// TestRunSweepIntegerRangeStepsByOne verifies a range between Integers runs one
// ordinary calc invocation per value, from its start through its end.
func TestRunSweepIntegerRangeStepsByOne(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, sweepModelSource, "sweep-integers")

	resp := runSweep(t, srv, &pb.RunSweepRequest{
		ModelHash: hash, SymbolId: "Sw::Twice",
		Ranges: []*pb.SweepRange{intRange("n", 1, 4)},
	})
	if resp.Error != "" {
		t.Fatalf("RunSweep reported %q", resp.Error)
	}
	got := rowText(resp)
	want := strings.Join([]string{
		"n=1 result -> 2",
		"n=2 result -> 4",
		"n=3 result -> 6",
		"n=4 result -> 8",
	}, "\n")
	if got != want {
		t.Errorf("rows are\n%s\nwant\n%s", got, want)
	}
	if len(resp.Parameters) != 1 || resp.Parameters[0] != "n" || resp.Sampled {
		t.Errorf("response reports parameters %v, sampled %v; want [n], false", resp.Parameters, resp.Sampled)
	}
	for i, row := range resp.Rows {
		if row.ElapsedMicros < 0 {
			t.Errorf("row %d took %d micros", i, row.ElapsedMicros)
		}
	}
}

// TestRunSweepCartesianProduct verifies several ranges run their product, the
// first range given varying slowest.
func TestRunSweepCartesianProduct(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, sweepModelSource, "sweep-product")

	resp := runSweep(t, srv, &pb.RunSweepRequest{
		ModelHash: hash, SymbolId: "Sw::Plus",
		Ranges: []*pb.SweepRange{intRange("a", 1, 2), intRange("b", 10, 11)},
	})
	got := rowText(resp)
	want := strings.Join([]string{
		"a=1 b=10 result -> 11",
		"a=1 b=11 result -> 12",
		"a=2 b=10 result -> 12",
		"a=2 b=11 result -> 13",
	}, "\n")
	if got != want {
		t.Errorf("rows are\n%s\nwant\n%s", got, want)
	}
	if len(resp.Parameters) != 2 || resp.Parameters[0] != "a" || resp.Parameters[1] != "b" {
		t.Errorf("parameters = %v, want [a b] in the order the ranges were given", resp.Parameters)
	}
}

// TestRunSweepStatedStep verifies a range advances by the step it states, and
// includes the end where the step lands on it.
func TestRunSweepStatedStep(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, sweepModelSource, "sweep-step")

	resp := runSweep(t, srv, &pb.RunSweepRequest{
		ModelHash: hash, SymbolId: "Sw::Twice",
		Ranges: []*pb.SweepRange{{
			Parameter: "n", Start: intProto(0), End: intProto(6), Step: intProto(3),
		}},
	})
	if got, want := rowText(resp), "n=0 result -> 0\nn=3 result -> 6\nn=6 result -> 12"; got != want {
		t.Errorf("rows are\n%s\nwant\n%s", got, want)
	}
}

// TestRunSweepAnalysisCaseOnSubject verifies an analysis case runs against the
// object named once per row, reporting each row's outputs and verdict.
func TestRunSweepAnalysisCaseOnSubject(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, sweepModelSource, "sweep-analysis")

	resp := runSweep(t, srv, &pb.RunSweepRequest{
		ModelHash: hash, SymbolId: "Sw::CostAnalysis", SubjectSymbolId: "Sw::ship",
		Ranges: []*pb.SweepRange{{
			Parameter: "limit", Start: realProto(2), End: realProto(30), Step: realProto(14),
		}},
	})
	if resp.Error != "" {
		t.Fatalf("RunSweep reported %q", resp.Error)
	}
	if len(resp.Rows) != 3 {
		t.Fatalf("ran %d row(s); want 3: %s", len(resp.Rows), rowText(resp))
	}
	holds := make([]bool, 0, len(resp.Rows))
	for _, row := range resp.Rows {
		if len(row.Verdicts) != 1 || row.Verdicts[0].Kind != "objective" {
			t.Fatalf("row verdicts = %v; want the objective's", row.Verdicts)
		}
		if row.Verdicts[0].InstanceTypeId != "Sw::ship" {
			t.Errorf("verdict is about %q; want the subject named", row.Verdicts[0].InstanceTypeId)
		}
		holds = append(holds, row.Verdicts[0].Holds)
	}
	if want := []bool{true, true, false}; holds[0] != want[0] || holds[1] != want[1] || holds[2] != want[2] {
		t.Errorf("objective held %v; want %v", holds, want)
	}
}

// TestRunSweepFailedRunIsARow verifies a run that failed is that row's typed
// error, and that the rows after it are run all the same.
func TestRunSweepFailedRunIsARow(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, sweepModelSource, "sweep-failed-row")

	resp := runSweep(t, srv, &pb.RunSweepRequest{
		ModelHash: hash, SymbolId: "Sw::Ratio",
		NamedArguments: map[string]*pb.Value{"a": realProto(4)},
		Ranges:         []*pb.SweepRange{intRange("b", -1, 1)},
	})
	if resp.Error != "" {
		t.Fatalf("RunSweep reported %q; want the failure on its row", resp.Error)
	}
	if len(resp.Rows) != 3 {
		t.Fatalf("ran %d row(s); want 3: %s", len(resp.Rows), rowText(resp))
	}
	failed := resp.Rows[1]
	if !strings.Contains(failed.Error, "division by zero") {
		t.Errorf("row error = %q; want a division by zero", failed.Error)
	}
	if failed.FailureReason != pb.FailureReason_FAILURE_REASON_EVALUATION {
		t.Errorf("row failure reason = %v; want EVALUATION", failed.FailureReason)
	}
	if len(failed.Outputs) != 0 {
		t.Errorf("failed row reported outputs %v; want none", failed.Outputs)
	}
	for _, i := range []int{0, 2} {
		if resp.Rows[i].Error != "" || len(resp.Rows[i].Outputs) != 1 {
			t.Errorf("row %d = %v; want one output and no error", i, resp.Rows[i])
		}
	}
}

// TestRunSweepSamplesAreReproducible verifies a sampled request draws the same
// rows from the same seed, different rows from another, and echoes the seed.
func TestRunSweepSamplesAreReproducible(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, sweepModelSource, "sweep-samples")

	draw := func(seed uint64) *pb.RunSweepResponse {
		return runSweep(t, srv, &pb.RunSweepRequest{
			ModelHash: hash, SymbolId: "Sw::Twice",
			Ranges:  []*pb.SweepRange{intRange("n", 0, 1_000_000)},
			Samples: 8, Seed: seed,
		})
	}
	first, again, other := draw(7), draw(7), draw(8)
	if !first.Sampled || first.Seed != 7 || len(first.Rows) != 8 {
		t.Fatalf("response reports sampled %v, seed %d, %d row(s); want true, 7, 8",
			first.Sampled, first.Seed, len(first.Rows))
	}
	if rowText(first) != rowText(again) {
		t.Errorf("seed 7 drew\n%s\nthen\n%s", rowText(first), rowText(again))
	}
	if rowText(first) == rowText(other) {
		t.Errorf("seed 8 drew what seed 7 did:\n%s", rowText(first))
	}
}

// TestRunSweepFailures verifies the in-band refusals: a target of another kind,
// a calc given a subject, a parameter the target does not declare, one the
// request already binds, a range no values follow from, a request naming no
// range, and a sweep beyond its budget.
func TestRunSweepFailures(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, sweepModelSource, "sweep-failures")

	cases := []struct {
		name    string
		req     *pb.RunSweepRequest
		wants   string
		reason  pb.FailureReason
		checked bool
	}{
		{
			name: "another kind of element",
			req: &pb.RunSweepRequest{ModelHash: hash, SymbolId: "Sw::ship",
				Ranges: []*pb.SweepRange{intRange("n", 1, 2)}},
			wants: "Sw::ship", reason: pb.FailureReason_FAILURE_REASON_WRONG_KIND, checked: true,
		},
		{
			name: "a calc given a subject",
			req: &pb.RunSweepRequest{ModelHash: hash, SymbolId: "Sw::Twice",
				SubjectSymbolId: "Sw::ship", Ranges: []*pb.SweepRange{intRange("n", 1, 2)}},
			wants: "subject",
		},
		{
			name: "a parameter the target does not declare",
			req: &pb.RunSweepRequest{ModelHash: hash, SymbolId: "Sw::Twice",
				Ranges: []*pb.SweepRange{intRange("nope", 1, 2)}},
			wants: "nope",
		},
		{
			name: "a parameter the request binds",
			req: &pb.RunSweepRequest{ModelHash: hash, SymbolId: "Sw::Twice",
				NamedArguments: map[string]*pb.Value{"n": intProto(3)},
				Ranges:         []*pb.SweepRange{intRange("n", 1, 2)}},
			wants: "both an argument",
		},
		{
			name: "a real range stating no step",
			req: &pb.RunSweepRequest{ModelHash: hash, SymbolId: "Sw::Ratio",
				NamedArguments: map[string]*pb.Value{"b": realProto(1)},
				Ranges: []*pb.SweepRange{{
					Parameter: "a", Start: realProto(0), End: realProto(1),
				}}},
			wants: "step",
		},
		{
			name: "a step of zero",
			req: &pb.RunSweepRequest{ModelHash: hash, SymbolId: "Sw::Twice",
				Ranges: []*pb.SweepRange{{
					Parameter: "n", Start: intProto(1), End: intProto(4), Step: intProto(0),
				}}},
			wants: "zero",
		},
		{
			name: "an endpoint left unstated",
			req: &pb.RunSweepRequest{ModelHash: hash, SymbolId: "Sw::Twice",
				Ranges: []*pb.SweepRange{{Parameter: "n", Start: intProto(1)}}},
			wants: "endpoint",
		},
		{
			name:  "no range at all",
			req:   &pb.RunSweepRequest{ModelHash: hash, SymbolId: "Sw::Twice"},
			wants: "no sweep range",
		},
		{
			name: "more runs than the budget allows",
			req: &pb.RunSweepRequest{ModelHash: hash, SymbolId: "Sw::Twice",
				Ranges: []*pb.SweepRange{intRange("n", 1, 100_000)}},
			wants: "OPENSYSML_MAX_SWEEP_RUNS",
		},
		{
			name: "fewer samples than one",
			req: &pb.RunSweepRequest{ModelHash: hash, SymbolId: "Sw::Twice",
				Ranges: []*pb.SweepRange{intRange("n", 1, 9)}, Samples: -1},
			wants: "sample",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := runSweep(t, srv, tc.req)
			if !strings.Contains(resp.Error, tc.wants) {
				t.Errorf("error = %q; want it to name %q", resp.Error, tc.wants)
			}
			if len(resp.Rows) != 0 {
				t.Errorf("a refused request ran %d row(s)", len(resp.Rows))
			}
			if tc.checked && resp.FailureReason != tc.reason {
				t.Errorf("failure reason = %v; want %v", resp.FailureReason, tc.reason)
			}
		})
	}
}

// TestRunSweepUnknownModel verifies an evicted or unknown model fails the call
// the way every other model-scoped RPC does.
func TestRunSweepUnknownModel(t *testing.T) {
	srv := mustNewService(t, 10)
	_, err := srv.RunSweep(context.Background(), &pb.RunSweepRequest{
		ModelHash: "nope", SymbolId: "Sw::Twice",
		Ranges: []*pb.SweepRange{intRange("n", 1, 2)},
	})
	if err == nil {
		t.Fatal("RunSweep on an unknown model should fail the call")
	}
}
