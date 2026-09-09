package grpc

import (
	"context"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

const analysisModelSource = `package An {
	private import ScalarValues::*;

	part def Ship {
		attribute cost : Real default = 5.0;
		attribute other : Real default = 7.0;
		attribute cap : Real;
	}

	calc def Sum {
		in a : Real;
		in b : Real;
		return : Real = a + b;
	}

	analysis def CostAnalysis {
		subject s : Ship;
		in limit : Real = 20.0;
		out total : Real = Sum(s.cost, s.other);
		objective affordable {
			require constraint { total <= limit }
		}
	}

	part ship : Ship;
	part barge : Ship {
		attribute :>> cost = 30.0;
	}

	analysis shipCost : CostAnalysis {
		subject s = ship;
	}

	analysis plain {
		out x : Real = 1.0 + 2.0;
	}

	analysis undecided {
		subject s = ship;
		out total : Real = Sum(s.cost, s.other);
		objective obj {
			require constraint { total <= s.cap }
		}
	}
}
`

func runAnalysis(t *testing.T, srv *Service, req *pb.RunAnalysisRequest) *pb.RunAnalysisResponse {
	t.Helper()
	resp, err := srv.RunAnalysis(context.Background(), req)
	if err != nil {
		t.Fatalf("RunAnalysis: %v", err)
	}
	return resp
}

func realOutput(t *testing.T, resp *pb.RunAnalysisResponse, name string) float64 {
	t.Helper()
	for _, out := range resp.Outputs {
		if out.Name == name {
			return out.Value.GetRealValue()
		}
	}
	t.Fatalf("no output %q in %v", name, resp.Outputs)
	return 0
}

// TestRunAnalysisUsageBindsItsOwnSubject verifies a usage whose subject is bound
// in the model runs with no request subject, reporting its outputs and the
// verdict of its objective.
func TestRunAnalysisUsageBindsItsOwnSubject(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, analysisModelSource, "analysis-usage")

	resp := runAnalysis(t, srv, &pb.RunAnalysisRequest{ModelHash: hash, SymbolId: "An::shipCost"})
	if resp.Error != "" {
		t.Fatalf("RunAnalysis reported %q", resp.Error)
	}
	if got := realOutput(t, resp, "total"); got != 12.0 {
		t.Errorf("total = %v, want 12", got)
	}
	if len(resp.Verdicts) != 1 {
		t.Fatalf("got %d verdicts, want the objective's: %v", len(resp.Verdicts), resp.Verdicts)
	}
	v := resp.Verdicts[0]
	if v.Kind != "objective" || v.Element != "affordable" || v.ElementId != "An::CostAnalysis::affordable" {
		t.Errorf("verdict names %s %q (%s), want objective affordable (An::CostAnalysis::affordable)", v.Kind, v.Element, v.ElementId)
	}
	if !v.Holds || v.Error != "" || v.Condition != "" {
		t.Errorf("verdict = %v, want it to hold", v)
	}
	if v.InstanceId == 0 || v.InstanceTypeId != "An::ship" {
		t.Errorf("verdict is about instance %d of %q, want the ship the usage binds", v.InstanceId, v.InstanceTypeId)
	}
	if len(resp.Instances) == 0 || resp.Instances[0].Id != v.InstanceId {
		t.Errorf("instances = %v, want the bound subject first", resp.Instances)
	}
}

// TestRunAnalysisWithoutObjectiveReportsOutputsAlone verifies a case stating no
// objective answers with its outputs and no verdict.
func TestRunAnalysisWithoutObjectiveReportsOutputsAlone(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, analysisModelSource, "analysis-plain")

	resp := runAnalysis(t, srv, &pb.RunAnalysisRequest{ModelHash: hash, SymbolId: "An::plain"})
	if resp.Error != "" {
		t.Fatalf("RunAnalysis reported %q", resp.Error)
	}
	if got := realOutput(t, resp, "x"); got != 3.0 {
		t.Errorf("x = %v, want 3", got)
	}
	if len(resp.Verdicts) != 0 || len(resp.Instances) != 0 {
		t.Errorf("verdicts = %v, instances = %d; want none", resp.Verdicts, len(resp.Instances))
	}
}

// TestRunAnalysisOnSubject verifies a definition runs against the object named,
// that a violated objective names its condition, and that the subject's graph
// is reported with the verdict.
func TestRunAnalysisOnSubject(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, analysisModelSource, "analysis-subject")

	resp := runAnalysis(t, srv, &pb.RunAnalysisRequest{
		ModelHash:       hash,
		SymbolId:        "An::CostAnalysis",
		SubjectSymbolId: "An::barge",
	})
	if resp.Error != "" {
		t.Fatalf("RunAnalysis reported %q", resp.Error)
	}
	if got := realOutput(t, resp, "total"); got != 37.0 {
		t.Errorf("total = %v, want 37", got)
	}
	if len(resp.Verdicts) != 1 {
		t.Fatalf("got %d verdicts, want 1: %v", len(resp.Verdicts), resp.Verdicts)
	}
	v := resp.Verdicts[0]
	if v.Holds || v.Error != "" {
		t.Errorf("verdict = %v, want it not to hold with no error", v)
	}
	if v.Condition != "total <= limit" {
		t.Errorf("condition = %q, want the violated constraint", v.Condition)
	}
	if v.InstanceId == 0 || v.InstanceTypeId != "An::barge" {
		t.Errorf("verdict is about instance %d of %q, want the barge", v.InstanceId, v.InstanceTypeId)
	}
	if len(resp.Instances) == 0 || resp.Instances[0].Id != v.InstanceId {
		t.Errorf("instances = %v, want the subject first", resp.Instances)
	}
}

// TestRunAnalysisArgumentsBindInputs verifies positional and named arguments
// bind the case's inputs, the subject excluded from the positional ones.
func TestRunAnalysisArgumentsBindInputs(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, analysisModelSource, "analysis-args")

	named := runAnalysis(t, srv, &pb.RunAnalysisRequest{
		ModelHash:       hash,
		SymbolId:        "An::CostAnalysis",
		SubjectSymbolId: "An::barge",
		NamedArguments:  map[string]*pb.Value{"limit": {Kind: &pb.Value_RealValue{RealValue: 50.0}}},
	})
	if named.Error != "" || len(named.Verdicts) != 1 || !named.Verdicts[0].Holds {
		t.Errorf("limit = 50 by name: error %q, verdicts %v; want the objective to hold", named.Error, named.Verdicts)
	}

	positional := runAnalysis(t, srv, &pb.RunAnalysisRequest{
		ModelHash:       hash,
		SymbolId:        "An::CostAnalysis",
		SubjectSymbolId: "An::barge",
		Arguments:       []*pb.Value{{Kind: &pb.Value_RealValue{RealValue: 10.0}}},
	})
	if positional.Error != "" || len(positional.Verdicts) != 1 || positional.Verdicts[0].Holds {
		t.Errorf("limit = 10 positionally: error %q, verdicts %v; want the objective violated", positional.Error, positional.Verdicts)
	}

	unknown := runAnalysis(t, srv, &pb.RunAnalysisRequest{
		ModelHash:       hash,
		SymbolId:        "An::CostAnalysis",
		SubjectSymbolId: "An::barge",
		NamedArguments:  map[string]*pb.Value{"nope": {Kind: &pb.Value_RealValue{RealValue: 1.0}}},
	})
	if !strings.Contains(unknown.Error, "nope") || unknown.FailureReason != pb.FailureReason_FAILURE_REASON_EVALUATION {
		t.Errorf("unknown argument: error %q (%v), want it named as an evaluation failure", unknown.Error, unknown.FailureReason)
	}
}

// TestRunAnalysisUndecidedObjective verifies an objective that cannot be
// evaluated is reported as an error on its verdict, not as a failed run.
func TestRunAnalysisUndecidedObjective(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, analysisModelSource, "analysis-undecided")

	resp := runAnalysis(t, srv, &pb.RunAnalysisRequest{ModelHash: hash, SymbolId: "An::undecided"})
	if resp.Error != "" {
		t.Fatalf("RunAnalysis reported %q", resp.Error)
	}
	if got := realOutput(t, resp, "total"); got != 12.0 {
		t.Errorf("total = %v, want 12", got)
	}
	if len(resp.Verdicts) != 1 {
		t.Fatalf("got %d verdicts, want 1: %v", len(resp.Verdicts), resp.Verdicts)
	}
	v := resp.Verdicts[0]
	if v.Holds || v.Error == "" || v.FailureReason != pb.FailureReason_FAILURE_REASON_EVALUATION {
		t.Errorf("verdict = %v, want an undecided one carrying its reason", v)
	}
	if !strings.Contains(v.Error, "cap") {
		t.Errorf("error = %q, want it to name the valueless feature", v.Error)
	}
}

// TestRunAnalysisFailures verifies the in-band failures: an unbound subject and
// an unknown symbol are evaluation failures, another kind of element is a
// wrong request.
func TestRunAnalysisFailures(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, analysisModelSource, "analysis-failures")

	unbound := runAnalysis(t, srv, &pb.RunAnalysisRequest{ModelHash: hash, SymbolId: "An::CostAnalysis"})
	if !strings.Contains(unbound.Error, "subject") || unbound.FailureReason != pb.FailureReason_FAILURE_REASON_EVALUATION {
		t.Errorf("unbound subject: error %q (%v), want an evaluation failure naming the subject", unbound.Error, unbound.FailureReason)
	}
	if len(unbound.Outputs) != 0 || len(unbound.Evaluations) != 0 {
		t.Errorf("unbound subject answered %v %v, want no output and no evaluation", unbound.Outputs, unbound.Evaluations)
	}
	// A run that never started decided nothing: its objective is reported undecided, not omitted.
	if len(unbound.Verdicts) != 1 || unbound.Verdicts[0].Holds || !strings.Contains(unbound.Verdicts[0].Error, "subject") {
		t.Errorf("unbound subject verdicts = %v, want the objective undecided by the unbound subject", unbound.Verdicts)
	}

	wrongKind := runAnalysis(t, srv, &pb.RunAnalysisRequest{ModelHash: hash, SymbolId: "An::ship"})
	if wrongKind.FailureReason != pb.FailureReason_FAILURE_REASON_WRONG_KIND || !strings.Contains(wrongKind.Error, "An::ship") {
		t.Errorf("part usage: error %q (%v), want WRONG_KIND naming it", wrongKind.Error, wrongKind.FailureReason)
	}

	unknown := runAnalysis(t, srv, &pb.RunAnalysisRequest{ModelHash: hash, SymbolId: "An::NoSuchCase"})
	if !strings.Contains(unknown.Error, "NoSuchCase") {
		t.Errorf("error = %q, want it to name the missing symbol", unknown.Error)
	}

	badSubject := runAnalysis(t, srv, &pb.RunAnalysisRequest{
		ModelHash:       hash,
		SymbolId:        "An::CostAnalysis",
		SubjectSymbolId: "An::NoSuchPart",
	})
	if !strings.Contains(badSubject.Error, "NoSuchPart") {
		t.Errorf("error = %q, want it to name the missing subject", badSubject.Error)
	}
}

// TestRunAnalysisUnknownModel verifies an evicted or unknown model fails the
// call the way every other model-scoped RPC does.
func TestRunAnalysisUnknownModel(t *testing.T) {
	srv := mustNewService(t, 10)
	_, err := srv.RunAnalysis(context.Background(), &pb.RunAnalysisRequest{ModelHash: "nope", SymbolId: "An::plain"})
	if err == nil {
		t.Fatal("RunAnalysis on an unknown model should fail the call")
	}
}
