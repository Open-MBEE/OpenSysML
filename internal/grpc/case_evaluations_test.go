package grpc

import (
	"context"
	"slices"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

const tradeStudyModelSource = `package Trade {
	private import ScalarValues::*;
	private import TradeStudies::*;

	part def Engine { attribute mass : Real; attribute cylinders : Integer; }
	part a : Engine { attribute :>> mass = 30.0; attribute :>> cylinders = 6; }
	part b : Engine { attribute :>> mass = 10.0; attribute :>> cylinders = 4; }
	part c : Engine { attribute :>> mass = 10.0; attribute :>> cylinders = 0; }

	analysis lightest : TradeStudy {
		subject : Engine[1..*] = (a, b, c);
		objective : MinimizeObjective;
		calc :>> evaluationFunction {
			in part e :>> alternative : Engine;
			return :>> result : Real = e.mass;
		}
		return part :>> selectedAlternative : Engine;
	}

	analysis perCylinder : TradeStudy {
		subject : Engine[1..*] = (a, c);
		objective : MinimizeObjective;
		calc :>> evaluationFunction {
			in part e :>> alternative : Engine;
			return :>> result : Real = e.mass / e.cylinders;
		}
		return part :>> selectedAlternative : Engine;
	}

	analysis weighted : TradeStudy {
		subject : Engine[1..*] = (a, b);
		in attribute cylinderWeight : Real;
		objective : MaximizeObjective;
		calc :>> evaluationFunction {
			in part e :>> alternative : Engine;
			return :>> result : Real = e.cylinders * cylinderWeight - e.mass;
		}
		return part :>> selectedAlternative : Engine;
	}

	analysis perOffset : TradeStudy {
		subject : Engine[1..*] = (a, b);
		in attribute offset : Integer;
		objective : MinimizeObjective;
		calc :>> evaluationFunction {
			in part e :>> alternative : Engine;
			return :>> result : Real = e.mass / (e.cylinders - offset);
		}
		return part :>> selectedAlternative : Engine;
	}
}
`

// TestRunAnalysisReportsCaseEvaluations verifies a trade study's run crosses the
// wire with each alternative's evaluation in subject order, the selected one
// and the one tied with it marked, and every alternative among the instances.
func TestRunAnalysisReportsCaseEvaluations(t *testing.T) {
	srv := mustNewService(t, 10)
	info, err := srv.GetServerInfo(context.Background(), &pb.ServerInfoRequest{})
	if err != nil {
		t.Fatalf("GetServerInfo: %v", err)
	}
	if !slices.Contains(info.Capabilities, CapabilityCaseEvaluations) {
		t.Fatalf("capabilities = %v, want %q among them", info.Capabilities, CapabilityCaseEvaluations)
	}
	hash := mustVerifyModel(t, srv, tradeStudyModelSource, "trade-study")

	resp := runAnalysis(t, srv, &pb.RunAnalysisRequest{ModelHash: hash, SymbolId: "Trade::lightest"})
	if resp.Error != "" {
		t.Fatalf("RunAnalysis reported %q", resp.Error)
	}
	if len(resp.Outputs) != 1 || resp.Outputs[0].Name != "selectedAlternative" {
		t.Fatalf("outputs = %v, want selectedAlternative alone", resp.Outputs)
	}
	selectedID := resp.Outputs[0].Value.GetInstanceId()
	if selectedID == 0 {
		t.Fatalf("selectedAlternative = %v, want an instance", resp.Outputs[0].Value)
	}
	if len(resp.Verdicts) != 1 || !resp.Verdicts[0].Holds || resp.Verdicts[0].Element != "tradeStudyObjective" {
		t.Errorf("verdicts = %v, want objective tradeStudyObjective satisfied", resp.Verdicts)
	}
	if len(resp.Evaluations) != 3 {
		t.Fatalf("got %d evaluations, want one per alternative: %v", len(resp.Evaluations), resp.Evaluations)
	}
	typeOf := func(id int64) string {
		for _, inst := range resp.Instances {
			if inst.Id == id {
				return inst.TypeSymbolId
			}
		}
		t.Errorf("instance %d is not among the reported instances %v", id, resp.Instances)
		return ""
	}
	wantAlternatives := []string{"Trade::a", "Trade::b", "Trade::c"}
	wantResults := []float64{30.0, 10.0, 10.0}
	for i, e := range resp.Evaluations {
		if e.FunctionId != "Trade::lightest::evaluationFunction" {
			t.Errorf("evaluation %d applies %q, want the case's evaluationFunction", i, e.FunctionId)
		}
		if len(e.Arguments) != 1 || typeOf(e.Arguments[0].GetInstanceId()) != wantAlternatives[i] {
			t.Errorf("evaluation %d arguments = %v, want %s", i, e.Arguments, wantAlternatives[i])
		}
		if e.Error != "" || e.Result.GetRealValue() != wantResults[i] {
			t.Errorf("evaluation %d = %v (%q), want %v", i, e.Result, e.Error, wantResults[i])
		}
		wantSelected, wantTied := i == 1, i == 2
		if e.Selected != wantSelected || e.Tied != wantTied {
			t.Errorf("evaluation %d selected=%v tied=%v, want selected=%v tied=%v", i, e.Selected, e.Tied, wantSelected, wantTied)
		}
	}
	if resp.Evaluations[1].Arguments[0].GetInstanceId() != selectedID {
		t.Errorf("selected evaluation is of instance %d, selectedAlternative is %d", resp.Evaluations[1].Arguments[0].GetInstanceId(), selectedID)
	}
}

// TestRunAnalysisKeepsEvaluationsOfFailedRun verifies a run one alternative's
// evaluation fails still reports every evaluation made — the failed one carrying
// its error — beside the run's error and the objective undecided.
func TestRunAnalysisKeepsEvaluationsOfFailedRun(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, tradeStudyModelSource, "trade-study-failed")

	resp := runAnalysis(t, srv, &pb.RunAnalysisRequest{ModelHash: hash, SymbolId: "Trade::perCylinder"})
	if !strings.Contains(resp.Error, "division by zero") || resp.FailureReason != pb.FailureReason_FAILURE_REASON_EVALUATION {
		t.Fatalf("error = %q (%v), want an evaluation failure by division by zero", resp.Error, resp.FailureReason)
	}
	if len(resp.Outputs) != 0 {
		t.Errorf("outputs = %v, want none: nothing was selected", resp.Outputs)
	}
	if len(resp.Verdicts) != 1 || resp.Verdicts[0].Holds || !strings.Contains(resp.Verdicts[0].Error, "division by zero") {
		t.Errorf("verdicts = %v, want the objective undecided by the division by zero", resp.Verdicts)
	}
	if len(resp.Evaluations) != 2 {
		t.Fatalf("got %d evaluations, want one per alternative: %v", len(resp.Evaluations), resp.Evaluations)
	}
	if first := resp.Evaluations[0]; first.Error != "" || first.Result.GetRealValue() != 5.0 || first.Selected || first.Tied {
		t.Errorf("first evaluation = %v, want 5.0, neither selected nor tied", first)
	}
	if second := resp.Evaluations[1]; !strings.Contains(second.Error, "division by zero") || second.Result != nil {
		t.Errorf("second evaluation = %v, want its error and no result", second)
	}
	for _, e := range resp.Evaluations {
		id := e.Arguments[0].GetInstanceId()
		if !slices.ContainsFunc(resp.Instances, func(inst *pb.Instance) bool { return inst.Id == id }) {
			t.Errorf("alternative %d is not among the reported instances %v", id, resp.Instances)
		}
	}
}

// TestRunAnalysisWithoutCaseEvaluations verifies a service without the
// capability answers as before it: no evaluations, and a failed run as its
// error alone.
func TestRunAnalysisWithoutCaseEvaluations(t *testing.T) {
	srv := mustNewServiceWithout(t, CapabilityCaseEvaluations)
	hash := mustVerifyModel(t, srv, tradeStudyModelSource, "trade-study-legacy")

	resp := runAnalysis(t, srv, &pb.RunAnalysisRequest{ModelHash: hash, SymbolId: "Trade::lightest"})
	if resp.Error != "" || len(resp.Outputs) != 1 || len(resp.Evaluations) != 0 {
		t.Errorf("response = %v, want the output alone without evaluations", resp)
	}
	failed := runAnalysis(t, srv, &pb.RunAnalysisRequest{ModelHash: hash, SymbolId: "Trade::perCylinder"})
	if failed.Error == "" || len(failed.Verdicts) != 0 || len(failed.Evaluations) != 0 || len(failed.Instances) != 0 {
		t.Errorf("failed response = %v, want its error alone", failed)
	}
}

// TestRunSweepReportsCaseEvaluations verifies each row of a swept trade study
// carries that run's evaluations, the selected alternative marked, and that
// every alternative a row names is among the table's instances.
func TestRunSweepReportsCaseEvaluations(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, tradeStudyModelSource, "trade-study-sweep")

	resp := runSweep(t, srv, &pb.RunSweepRequest{
		ModelHash: hash, SymbolId: "Trade::weighted",
		Ranges: []*pb.SweepRange{{Parameter: "cylinderWeight", Start: realProto(0), End: realProto(20), Step: realProto(10)}},
	})
	if resp.Error != "" {
		t.Fatalf("RunSweep reported %q", resp.Error)
	}
	if len(resp.Rows) != 3 {
		t.Fatalf("ran %d row(s); want 3: %s", len(resp.Rows), rowText(resp))
	}
	// a: 6 cylinders, 30 kg; b: 4 cylinders, 10 kg. Weighting cylinders at 0 or
	// 10 favours b, at 20 a.
	wantResults := [][]float64{{-30, -10}, {30, 30}, {90, 70}}
	wantSelected := []int{1, 0, 0}
	for i, row := range resp.Rows {
		if row.Error != "" || len(row.Outputs) != 1 {
			t.Fatalf("row %d = %v; want selectedAlternative and no error", i, row)
		}
		if len(row.Evaluations) != 2 {
			t.Fatalf("row %d has %d evaluations; want one per alternative: %v", i, len(row.Evaluations), row.Evaluations)
		}
		for j, e := range row.Evaluations {
			if e.FunctionId != "Trade::weighted::evaluationFunction" || e.Error != "" || e.Result.GetRealValue() != wantResults[i][j] {
				t.Errorf("row %d evaluation %d = %v; want %v", i, j, e, wantResults[i][j])
			}
			if e.Selected != (j == wantSelected[i]) {
				t.Errorf("row %d evaluation %d selected=%v; want alternative %d selected", i, j, e.Selected, wantSelected[i])
			}
			id := e.Arguments[0].GetInstanceId()
			if !slices.ContainsFunc(resp.Instances, func(inst *pb.Instance) bool { return inst.Id == id }) {
				t.Errorf("row %d: alternative %d is not among the reported instances", i, id)
			}
		}
		if selected := row.Evaluations[wantSelected[i]]; selected.Arguments[0].GetInstanceId() != row.Outputs[0].Value.GetInstanceId() {
			t.Errorf("row %d selected evaluation is of %v, selectedAlternative is %v", i, selected.Arguments[0], row.Outputs[0].Value)
		}
	}
	// The second row is a tie: both alternatives evaluate to 30, and the library
	// selects the first, the second tied with it.
	if tie := resp.Rows[1].Evaluations; !tie[1].Tied || tie[0].Tied {
		t.Errorf("tied row evaluations = %v; want the second marked tied", tie)
	}
}

// TestRunSweepKeepsEvaluationsOfFailedRow verifies a row whose run failed on
// one alternative keeps the evaluations made before it and its undecided
// objective beside the error, and a service without the capability reports
// the error alone as before.
func TestRunSweepKeepsEvaluationsOfFailedRow(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, tradeStudyModelSource, "trade-study-sweep-failed")
	req := &pb.RunSweepRequest{
		ModelHash: hash, SymbolId: "Trade::perOffset",
		Ranges: []*pb.SweepRange{intRange("offset", 3, 4)},
	}

	resp := runSweep(t, srv, req)
	if resp.Error != "" || len(resp.Rows) != 2 {
		t.Fatalf("RunSweep = %q with %d row(s); want two rows: %s", resp.Error, len(resp.Rows), rowText(resp))
	}
	ok := resp.Rows[0]
	if ok.Error != "" || len(ok.Evaluations) != 2 || !ok.Evaluations[0].Selected || ok.Evaluations[1].Selected {
		t.Errorf("completed row = %v; want a (10.0) selected over b (10.0 tied)", ok)
	}
	failed := resp.Rows[1]
	if !strings.Contains(failed.Error, "division by zero") || failed.FailureReason != pb.FailureReason_FAILURE_REASON_EVALUATION {
		t.Fatalf("failed row = %q (%v); want an evaluation failure by division by zero", failed.Error, failed.FailureReason)
	}
	if len(failed.Outputs) != 0 {
		t.Errorf("failed row outputs = %v; want none: nothing was selected", failed.Outputs)
	}
	if len(failed.Verdicts) != 1 || failed.Verdicts[0].Holds || !strings.Contains(failed.Verdicts[0].Error, "division by zero") {
		t.Errorf("failed row verdicts = %v; want the objective undecided", failed.Verdicts)
	}
	if len(failed.Evaluations) != 2 || failed.Evaluations[0].Error != "" || failed.Evaluations[0].Result.GetRealValue() != 15.0 ||
		!strings.Contains(failed.Evaluations[1].Error, "division by zero") || failed.Evaluations[1].Result != nil {
		t.Errorf("failed row evaluations = %v; want a = 15.0 then b's division by zero", failed.Evaluations)
	}

	legacy := mustNewServiceWithout(t, CapabilityCaseEvaluations)
	req.ModelHash = mustVerifyModel(t, legacy, tradeStudyModelSource, "trade-study-sweep-legacy")
	resp = runSweep(t, legacy, req)
	if resp.Error != "" || len(resp.Rows) != 2 {
		t.Fatalf("legacy RunSweep = %q with %d row(s); want two rows", resp.Error, len(resp.Rows))
	}
	if ok := resp.Rows[0]; ok.Error != "" || len(ok.Outputs) != 1 || len(ok.Evaluations) != 0 {
		t.Errorf("legacy completed row = %v; want its output without evaluations", ok)
	}
	if failed := resp.Rows[1]; failed.Error == "" || len(failed.Verdicts) != 0 || len(failed.Evaluations) != 0 || len(failed.Outputs) != 0 {
		t.Errorf("legacy failed row = %v; want its error alone", failed)
	}
}
