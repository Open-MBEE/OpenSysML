package grpc

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"google.golang.org/protobuf/proto"
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

	calc def Toggle {
		in on : Boolean;
		return : Boolean = not on;
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

// TestRunSweepBindsInTheParameterType verifies a range is typed by the
// parameter it sweeps: Integer literals over a Real parameter reach each row
// as Reals, real literals over an Integer parameter as Integers, and a Real
// parameter samples reals.
func TestRunSweepBindsInTheParameterType(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, sweepModelSource, "sweep-typed")

	resp := runSweep(t, srv, &pb.RunSweepRequest{
		ModelHash: hash, SymbolId: "Sw::Ratio",
		NamedArguments: map[string]*pb.Value{"b": realProto(2)},
		Ranges:         []*pb.SweepRange{{Parameter: "a", Start: intProto(1), End: intProto(4), Step: intProto(1)}},
	})
	if resp.Error != "" {
		t.Fatalf("RunSweep reported %q", resp.Error)
	}
	want := strings.Join([]string{
		"a=1.0 result -> 0.5",
		"a=2.0 result -> 1.0",
		"a=3.0 result -> 1.5",
		"a=4.0 result -> 2.0",
	}, "\n")
	if got := rowText(resp); got != want {
		t.Errorf("rows are\n%s\nwant\n%s", got, want)
	}
	for _, row := range resp.Rows {
		if _, ok := row.Inputs[0].Value.GetKind().(*pb.Value_RealValue); !ok {
			t.Errorf("row binds %v; want a real_value", row.Inputs[0].Value)
		}
	}

	resp = runSweep(t, srv, &pb.RunSweepRequest{
		ModelHash: hash, SymbolId: "Sw::Twice",
		Ranges: []*pb.SweepRange{{Parameter: "n", Start: realProto(1), End: realProto(3), Step: realProto(1)}},
	})
	if resp.Error != "" {
		t.Fatalf("RunSweep reported %q", resp.Error)
	}
	if got, want := rowText(resp), "n=1 result -> 2\nn=2 result -> 4\nn=3 result -> 6"; got != want {
		t.Errorf("rows are\n%s\nwant\n%s", got, want)
	}
	for _, row := range resp.Rows {
		if _, ok := row.Inputs[0].Value.GetKind().(*pb.Value_IntValue); !ok {
			t.Errorf("row binds %v; want an int_value", row.Inputs[0].Value)
		}
	}

	resp = runSweep(t, srv, &pb.RunSweepRequest{
		ModelHash: hash, SymbolId: "Sw::Ratio",
		NamedArguments: map[string]*pb.Value{"b": realProto(1)},
		Ranges:         []*pb.SweepRange{intRange("a", 1, 4)},
		Samples:        4, Seed: 7,
	})
	if resp.Error != "" || len(resp.Rows) != 4 {
		t.Fatalf("RunSweep reported %q with %d row(s); want 4 rows", resp.Error, len(resp.Rows))
	}
	for _, row := range resp.Rows {
		drawn, ok := row.Inputs[0].Value.GetKind().(*pb.Value_RealValue)
		if !ok || drawn.RealValue < 1 || drawn.RealValue >= 4 {
			t.Errorf("drew %v; want a real_value in [1.0, 4.0)", row.Inputs[0].Value)
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
					Parameter: "a", Start: realProto(0.5), End: realProto(1),
				}}},
			wants: "step",
		},
		{
			name: "a fractional step over an Integer parameter",
			req: &pb.RunSweepRequest{ModelHash: hash, SymbolId: "Sw::Twice",
				Ranges: []*pb.SweepRange{{
					Parameter: "n", Start: realProto(1), End: realProto(3), Step: realProto(0.5),
				}}},
			wants: "n : Integer",
		},
		{
			name: "a fractional endpoint sampled over an Integer parameter",
			req: &pb.RunSweepRequest{ModelHash: hash, SymbolId: "Sw::Twice",
				Ranges:  []*pb.SweepRange{{Parameter: "n", Start: realProto(1.5), End: realProto(3)}},
				Samples: 2, Seed: 1},
			wants: "n : Integer",
		},
		{
			name: "a range over a Boolean parameter",
			req: &pb.RunSweepRequest{ModelHash: hash, SymbolId: "Sw::Toggle",
				Ranges: []*pb.SweepRange{intRange("on", 0, 1)}},
			wants: "typed by Boolean",
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

// TestRunSweepCanceledCallerFailsTheCall verifies a caller that has gone away
// fails the call rather than being reported as a table of failed rows.
func TestRunSweepCanceledCallerFailsTheCall(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, sweepModelSource, "sweep-canceled")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	resp, err := srv.RunSweep(ctx, &pb.RunSweepRequest{
		ModelHash: hash, SymbolId: "Sw::Twice",
		Ranges: []*pb.SweepRange{intRange("n", 1, 8)},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v; want context.Canceled", err)
	}
	if resp != nil {
		t.Errorf("a canceled call reported %d row(s); want no response", len(resp.Rows))
	}
}

// TestRunSweepVerdictInstancesResolve verifies every row verdict names an
// object the table carries, whose feature values a client can read.
func TestRunSweepVerdictInstancesResolve(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, sweepModelSource, "sweep-instances")

	resp := runSweep(t, srv, &pb.RunSweepRequest{
		ModelHash: hash, SymbolId: "Sw::CostAnalysis", SubjectSymbolId: "Sw::ship",
		Ranges: []*pb.SweepRange{{
			Parameter: "limit", Start: realProto(2), End: realProto(30), Step: realProto(14),
		}},
	})
	if resp.Error != "" {
		t.Fatalf("RunSweep reported %q", resp.Error)
	}
	byID := make(map[int64]*pb.Instance, len(resp.Instances))
	for _, inst := range resp.Instances {
		byID[inst.Id] = inst
	}
	for i, row := range resp.Rows {
		for _, verdict := range row.Verdicts {
			if verdict.InstanceId == 0 {
				t.Fatalf("row %d verdict is about no object; want the subject", i)
			}
			inst, ok := byID[verdict.InstanceId]
			if !ok {
				t.Fatalf("row %d verdict names object %d, which the table does not carry",
					i, verdict.InstanceId)
			}
			cost, ok := inst.FeatureValues["cost"]
			if !ok || cost.Value.GetRealValue() != 5.0 {
				t.Errorf("row %d subject holds cost = %v; want 5", i, cost)
			}
		}
	}
}

// TestRunSweepRefusesAnalysisInputAnArgumentBinds verifies a case's positional
// argument binds the input its run binds, the subject skipped, so sweeping that
// input is refused rather than run twice over.
func TestRunSweepRefusesAnalysisInputAnArgumentBinds(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, sweepModelSource, "sweep-collision")

	resp := runSweep(t, srv, &pb.RunSweepRequest{
		ModelHash: hash, SymbolId: "Sw::CostAnalysis", SubjectSymbolId: "Sw::ship",
		Arguments: []*pb.Value{realProto(3)},
		Ranges:    []*pb.SweepRange{{Parameter: "limit", Start: realProto(2), End: realProto(6), Step: realProto(2)}},
	})
	if !strings.Contains(resp.Error, "limit") || len(resp.Rows) != 0 {
		t.Errorf("error = %q with %d row(s); want a refusal naming limit", resp.Error, len(resp.Rows))
	}
}

// TestRunSweepRefusesSweepingTheSubject verifies a case's subject is an object
// an instantiation binds, which no range of values stands for.
func TestRunSweepRefusesSweepingTheSubject(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, sweepModelSource, "sweep-subject-range")

	resp := runSweep(t, srv, &pb.RunSweepRequest{
		ModelHash: hash, SymbolId: "Sw::CostAnalysis", SubjectSymbolId: "Sw::ship",
		Ranges: []*pb.SweepRange{intRange("s", 1, 3)},
	})
	if !strings.Contains(resp.Error, "subject") || len(resp.Rows) != 0 {
		t.Errorf("error = %q with %d row(s); want a refusal naming the subject", resp.Error, len(resp.Rows))
	}
}

// TestRunSweepRefusesNonFiniteRanges verifies a range whose endpoint or step is
// not a finite number states no run to make and is refused as a whole.
func TestRunSweepRefusesNonFiniteRanges(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, sweepModelSource, "sweep-non-finite")

	cases := []struct {
		name  string
		start float64
		end   float64
		step  float64
	}{
		{"start is not a number", math.NaN(), 4, 1},
		{"end is not a number", 0, math.NaN(), 1},
		{"step is not a number", 0, 4, math.NaN()},
		{"end is infinite", 0, math.Inf(1), 1},
		{"start is infinite", math.Inf(-1), 4, 1},
		{"step is infinite", 0, 4, math.Inf(1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := runSweep(t, srv, &pb.RunSweepRequest{
				ModelHash: hash, SymbolId: "Sw::Ratio",
				NamedArguments: map[string]*pb.Value{"b": realProto(2)},
				Ranges: []*pb.SweepRange{{
					Parameter: "a", Start: realProto(tc.start),
					End: realProto(tc.end), Step: realProto(tc.step),
				}},
			})
			if !strings.Contains(resp.Error, "finite") || len(resp.Rows) != 0 {
				t.Errorf("error = %q with %d row(s); want a refusal naming a non-finite number",
					resp.Error, len(resp.Rows))
			}
		})
	}
}

// TestRunSweepRefusesSamplingANonFiniteRange verifies a drawn range is refused
// for the same reason a stepped one is: no draw lies in it.
func TestRunSweepRefusesSamplingANonFiniteRange(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, sweepModelSource, "sweep-non-finite-samples")

	resp := runSweep(t, srv, &pb.RunSweepRequest{
		ModelHash: hash, SymbolId: "Sw::Ratio",
		NamedArguments: map[string]*pb.Value{"b": realProto(2)},
		Ranges: []*pb.SweepRange{{
			Parameter: "a", Start: realProto(0), End: realProto(math.Inf(1)),
		}},
		Samples: 3, Seed: 7,
	})
	if !strings.Contains(resp.Error, "finite") || len(resp.Rows) != 0 {
		t.Errorf("error = %q with %d row(s); want a refusal naming a non-finite number",
			resp.Error, len(resp.Rows))
	}
}

// TestRunSweepAnswersAlikeOnOneJobAndOnEight verifies every sweep the service
// answers — a calc over a range, a failing row, a product, a sample, a case on a
// held object, trade studies and a case whose outputs nest objects — is the same
// response on eight jobs as on one apart from the time each row took: rows in plan
// order, the same inputs, outputs, verdicts, evaluations, errors and instances.
func TestRunSweepAnswersAlikeOnOneJobAndOnEight(t *testing.T) {
	srv := mustNewService(t, 10)
	t.Cleanup(srv.Close)
	sweep := mustVerifyModel(t, srv, sweepModelSource, "sweep-jobs")
	trade := mustVerifyModel(t, srv, tradeStudyModelSource, "sweep-jobs-trade")
	nested := mustVerifyModel(t, srv, nestedObjectsModelSource, "sweep-jobs-nested")

	requests := []*pb.RunSweepRequest{
		{ModelHash: sweep, SymbolId: "Sw::Twice", Ranges: []*pb.SweepRange{intRange("n", 1, 12)}},
		{ModelHash: sweep, SymbolId: "Sw::Ratio", NamedArguments: map[string]*pb.Value{"a": realProto(4)},
			Ranges: []*pb.SweepRange{intRange("b", -1, 1)}},
		{ModelHash: sweep, SymbolId: "Sw::Plus", Ranges: []*pb.SweepRange{intRange("a", 1, 3), intRange("b", 10, 12)}},
		{ModelHash: sweep, SymbolId: "Sw::Twice", Ranges: []*pb.SweepRange{intRange("n", 0, 1_000_000)}, Samples: 8, Seed: 7},
		{ModelHash: sweep, SymbolId: "Sw::CostAnalysis", SubjectSymbolId: "Sw::ship",
			Ranges: []*pb.SweepRange{{Parameter: "limit", Start: realProto(2), End: realProto(30), Step: realProto(4)}}},
		{ModelHash: trade, SymbolId: "Trade::weighted",
			Ranges: []*pb.SweepRange{{Parameter: "cylinderWeight", Start: realProto(0), End: realProto(20), Step: realProto(5)}}},
		{ModelHash: trade, SymbolId: "Trade::perOffset", Ranges: []*pb.SweepRange{intRange("offset", 3, 4)}},
		{ModelHash: nested, SymbolId: "Nested::grouping",
			Ranges: []*pb.SweepRange{{Parameter: "k", Start: realProto(1), End: realProto(4), Step: realProto(1)}}},
	}
	for _, req := range requests {
		t.Run(req.SymbolId, func(t *testing.T) {
			answer := func(jobs int) *pb.RunSweepResponse {
				srv.jobs = jobs
				resp := runSweep(t, srv, req)
				for _, row := range resp.Rows {
					row.ElapsedMicros = 0
				}
				return resp
			}
			one, eight := answer(1), answer(8)
			if one.Error != "" || len(one.Rows) == 0 {
				t.Fatalf("one job answered %q with %d row(s)", one.Error, len(one.Rows))
			}
			if !proto.Equal(one, eight) {
				t.Errorf("one job answered\n%v\neight jobs answered\n%v", one, eight)
			}
		})
	}
}

// sweepWritesModelSource is a case whose body writes its subject and reports it,
// through a value, a verdict, a function read off it and a sequence holding it.
const sweepWritesModelSource = `package Rows {
	private import ScalarValues::*;
	part def Ship {
		attribute cost : Real = 5.0;
		calc weigh { in x : Real; return : Real = cost * x; }
	}
	part ship : Ship;
	analysis def Bump {
		subject s : Ship;
		in tax : Real;
		action raise { assign s.cost := s.cost + tax; }
		out total : Real = s.cost;
		out who : Ship = s;
		out scale = s.weigh;
		out crew : Ship[*] nonunique = (s, s);
		objective cheap { require constraint { total <= 7.0 } }
	}
}
`

// TestRunSweepRowsNameTheirOwnObjects verifies a table whose rows ran in contexts
// of their own, each numbering its objects from 1, still names every object once:
// each row's outputs, verdict, function and sequence resolve to the object that
// row wrote, not to another row's, and the ids differ from row to row.
func TestRunSweepRowsNameTheirOwnObjects(t *testing.T) {
	for _, jobs := range []int{1, 8} {
		t.Run(strconv.Itoa(jobs)+" jobs", func(t *testing.T) {
			srv := mustNewService(t, 10)
			srv.jobs = jobs
			hash := mustVerifyModel(t, srv, sweepWritesModelSource, "sweep-writes")
			resp := runSweep(t, srv, &pb.RunSweepRequest{
				ModelHash: hash, SymbolId: "Rows::Bump", SubjectSymbolId: "Rows::ship",
				Ranges: []*pb.SweepRange{{Parameter: "tax", Start: realProto(1), End: realProto(3), Step: realProto(1)}},
			})
			if resp.Error != "" || len(resp.Rows) != 3 {
				t.Fatalf("RunSweep = %q with %d row(s); want three rows: %s", resp.Error, len(resp.Rows), rowText(resp))
			}
			byID := make(map[int64]*pb.Instance, len(resp.Instances))
			for _, inst := range resp.Instances {
				if byID[inst.Id] != nil {
					t.Fatalf("object %d is in the table twice", inst.Id)
				}
				byID[inst.Id] = inst
			}
			if len(resp.Instances) != 3 {
				t.Errorf("the table carries %d object(s), want one ship per row", len(resp.Instances))
			}
			seen := make(map[int64]int)
			for i, row := range resp.Rows {
				want := 6.0 + float64(i)
				var ids []int64
				for _, out := range row.Outputs {
					switch out.Name {
					case "total":
						if got := out.GetValue().GetRealValue(); got != want {
							t.Errorf("row %d total = %v, want %v", i, got, want)
						}
					case "who", "scale", "crew":
						ids = append(ids, idsOf(out.GetValue())...)
					}
				}
				if len(row.Verdicts) != 1 || row.Verdicts[0].InstanceId == 0 {
					t.Fatalf("row %d verdicts = %v, want one about the subject", i, row.Verdicts)
				}
				if row.Verdicts[0].Holds != (want <= 7.0) {
					t.Errorf("row %d verdict holds = %v on total %v", i, row.Verdicts[0].Holds, want)
				}
				ids = append(ids, row.Verdicts[0].InstanceId)
				if len(ids) != 5 {
					t.Fatalf("row %d names %d object(s) %v, want its ship five times", i, len(ids), ids)
				}
				for _, id := range ids {
					if id != ids[0] {
						t.Fatalf("row %d names objects %v, want one ship", i, ids)
					}
				}
				inst := byID[ids[0]]
				if inst == nil {
					t.Fatalf("row %d names object %d, which the table does not carry", i, ids[0])
				}
				if got := inst.FeatureValues["cost"].GetValue().GetRealValue(); got != want {
					t.Errorf("row %d resolves to a ship of cost %v, want %v", i, got, want)
				}
				if prior, ok := seen[ids[0]]; ok {
					t.Errorf("rows %d and %d name the same object %d", prior, i, ids[0])
				}
				seen[ids[0]] = i
			}
		})
	}
}
