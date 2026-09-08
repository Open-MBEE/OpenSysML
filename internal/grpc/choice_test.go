package grpc

import (
	"context"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// choiceDiagnostics keeps the choice-point diagnostics of a response.
func choiceDiagnostics(diags []*pb.Diagnostic) []*pb.Diagnostic {
	var out []*pb.Diagnostic
	for _, d := range diags {
		if strings.HasPrefix(d.Message, "choice point: ") {
			out = append(out, d)
		}
	}
	return out
}

// A run that chose among unordered alternatives reports each choice as an
// informational diagnostic located at the declaration it was made at; a run
// with one possible order reports none.
func TestExecuteAction_ChoicePointDiagnostics(t *testing.T) {
	srv := mustNewService(t, 10)

	content := `
package Test {
  action tally {
    attribute leftCount : Integer = 0;
    attribute rightCount : Integer = 0;

    first start;
    fork split;
    action left { assign leftCount := leftCount + 1; }
    action right { assign rightCount := rightCount + 10; }
    join sync;
    done;

    succession first start then split;
    succession first split then left;
    succession first split then right;
    succession first left then sync;
    succession first right then sync;
    succession first sync then done;
  }

  action single {
    attribute n : Integer = 0;
    first start;
    then action one { assign n := 1; }
    then done;
  }
}
`
	parseResp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: content},
		ContentHash: "test-execute-action-choices",
	})
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	resp, err := srv.ExecuteAction(context.Background(), &pb.ExecuteActionRequest{
		ModelHash:      parseResp.ModelHash,
		ActionSymbolId: "Test::tally",
	})
	if err != nil {
		t.Fatalf("ExecuteAction failed: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("execution error: %s", resp.Error)
	}
	choices := choiceDiagnostics(resp.Diagnostics)
	if len(choices) != 1 {
		t.Fatalf("choice diagnostics = %v, want one", resp.Diagnostics)
	}
	d := choices[0]
	if d.Severity != "info" {
		t.Errorf("severity = %q, want info", d.Severity)
	}
	want := "choice point: step 3: tokens 2@left, 3@right (unordered; took 3@right first)"
	if d.Message != want {
		t.Errorf("message = %q, want %q", d.Message, want)
	}
	if d.Span == nil || d.Span.StartLine == 0 {
		t.Errorf("choice diagnostic carries no location: %v", d.Span)
	}

	resp, err = srv.ExecuteAction(context.Background(), &pb.ExecuteActionRequest{
		ModelHash:      parseResp.ModelHash,
		ActionSymbolId: "Test::single",
	})
	if err != nil {
		t.Fatalf("ExecuteAction failed: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("execution error: %s", resp.Error)
	}
	if got := choiceDiagnostics(resp.Diagnostics); len(got) != 0 {
		t.Errorf("a run with one order reported choices: %v", got)
	}
}

// An analysis whose performed action forked reports the order the executor
// took among the branches and between their writes, with the case's outputs.
func TestRunAnalysis_ChoicePointDiagnostics(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, `
package An {
	private import ScalarValues::*;

	action def Tally {
		out r : Integer = 0;
		first start;
		fork split;
		action left { assign r := r + 1; }
		action right { assign r := r + 10; }
		join sync;
		done;
		succession first start then split;
		succession first split then left;
		succession first split then right;
		succession first left then sync;
		succession first right then sync;
		succession first sync then done;
	}

	analysis forked {
		out r : Integer;
		perform action tally : Tally;
		return : Integer = r;
	}
}
`, "analysis-choices")

	resp := runAnalysis(t, srv, &pb.RunAnalysisRequest{ModelHash: hash, SymbolId: "An::forked"})
	if resp.Error != "" {
		t.Fatalf("RunAnalysis reported %q", resp.Error)
	}
	choices := choiceDiagnostics(resp.Diagnostics)
	want := []string{
		"choice point: step 3: writes r := 11 by token 2, r := 10 by token 3 (unordered; r := 11 by token 2 stood)",
		"choice point: step 3: tokens 2@left, 3@right (unordered; took 3@right first)",
	}
	if len(choices) != len(want) {
		t.Fatalf("choice diagnostics = %v, want %d", resp.Diagnostics, len(want))
	}
	for i, d := range choices {
		if d.Message != want[i] || d.Severity != "info" {
			t.Errorf("diagnostic %d = %s %q, want info %q", i, d.Severity, d.Message, want[i])
		}
	}
}

// Two transitions out of one state enabled by one event are reported on the
// state response; the first declared still fires.
func TestExecuteState_ChoicePointDiagnostics(t *testing.T) {
	srv := mustNewService(t, 10)

	content := `
package Test {
  state Dispatcher {
    attribute level : Integer = 8;
    entry; then idle;
    state idle;
    state low;
    state high;
    transition first idle accept Go if level > 5 then low;
    transition first idle accept Go if level > 7 then high;
  }
}
`
	parseResp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: content},
		ContentHash: "test-execute-state-choices",
	})
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	resp, err := srv.ExecuteState(context.Background(), &pb.ExecuteStateRequest{
		ModelHash:            parseResp.ModelHash,
		StateMachineSymbolId: "Test::Dispatcher",
		Events:               []string{"Go"},
	})
	if err != nil {
		t.Fatalf("ExecuteState failed: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("execution error: %s", resp.Error)
	}
	if got := strings.Join(resp.StatesVisited, ","); got != "idle,low" {
		t.Fatalf("states visited %q, want idle,low", got)
	}
	choices := choiceDiagnostics(resp.Diagnostics)
	if len(choices) != 1 {
		t.Fatalf("choice diagnostics = %v, want one", resp.Diagnostics)
	}
	want := "choice point: state idle on accept Go: transitions 1->low, 2->high (unordered; took 1->low)"
	if choices[0].Message != want || choices[0].Severity != "info" {
		t.Errorf("diagnostic = %s %q, want info %q", choices[0].Severity, choices[0].Message, want)
	}
}

// A guard the run read only to report a choice and could not evaluate is an
// informational diagnostic naming the decision, the branch and the failure; the
// run itself is unchanged, and a first guard's failure still fails it.
func TestExecuteAction_UnevaluableGuardDiagnostics(t *testing.T) {
	srv := mustNewService(t, 10)

	content := `
package Test {
  action route {
    attribute level : Integer = 75;
    attribute handler : Integer = 0;
    first start;
    then decide select;
      if level > 50 then warn;
      if 1 / (level - 75) > 0 then alarm;
    action warn { assign handler := 1; }
    then done;
    action alarm { assign handler := 2; }
    then done;
  }
  action broken {
    attribute level : Integer = 75;
    first start;
    then decide select;
      if 1 / (level - 75) > 0 then alarm;
      if level > 50 then warn;
    action warn;
    then done;
    action alarm;
    then done;
  }
}
`
	parseResp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: content},
		ContentHash: "test-execute-action-unevaluable-guard",
	})
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	resp, err := srv.ExecuteAction(context.Background(), &pb.ExecuteActionRequest{
		ModelHash:      parseResp.ModelHash,
		ActionSymbolId: "Test::route",
	})
	if err != nil {
		t.Fatalf("ExecuteAction failed: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("execution error: %s", resp.Error)
	}
	if got := choiceDiagnostics(resp.Diagnostics); len(got) != 0 {
		t.Errorf("an unevaluable guard was reported as a choice: %v", got)
	}
	var unevaluable []*pb.Diagnostic
	for _, d := range resp.Diagnostics {
		if strings.HasPrefix(d.Message, "guard not evaluable: ") {
			unevaluable = append(unevaluable, d)
		}
	}
	if len(unevaluable) != 1 {
		t.Fatalf("unevaluable-guard diagnostics = %v, want one", resp.Diagnostics)
	}
	want := "guard not evaluable: step 2: decision select branch 2->alarm: division by zero (not selected)"
	if unevaluable[0].Message != want || unevaluable[0].Severity != "info" {
		t.Errorf("diagnostic = %s %q, want info %q", unevaluable[0].Severity, unevaluable[0].Message, want)
	}
	if unevaluable[0].Span == nil || unevaluable[0].Span.StartLine != 9 {
		t.Errorf("diagnostic is not located at the guard: %v", unevaluable[0].Span)
	}

	resp, err = srv.ExecuteAction(context.Background(), &pb.ExecuteActionRequest{
		ModelHash:      parseResp.ModelHash,
		ActionSymbolId: "Test::broken",
	})
	if err != nil {
		t.Fatalf("ExecuteAction failed: %v", err)
	}
	if !strings.Contains(resp.Error, "division by zero") {
		t.Fatalf("error = %q, want the first guard's evaluation failure", resp.Error)
	}
	for _, d := range resp.Diagnostics {
		if strings.HasPrefix(d.Message, "guard not evaluable: ") {
			t.Errorf("the first guard's failure was reported as unevaluable: %q", d.Message)
		}
	}
}
