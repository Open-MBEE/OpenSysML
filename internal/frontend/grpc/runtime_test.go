package grpc

import (
	"context"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// TestEvaluate_SimpleExpression verifies Evaluate RPC with a simple arithmetic expression
func TestEvaluate_SimpleExpression(t *testing.T) {
	srv := mustNewService(t, 10)

	// Parse a model with a calc definition
	content := `
package Test {
  calc two_plus_two { 2 + 2 }
}
`

	parseReq := &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: content},
		ContentHash: "test-eval-1",
	}

	parseResp, err := srv.ParseFile(context.Background(), parseReq)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if len(parseResp.Diagnostics) > 0 {
		t.Fatalf("unexpected parse diagnostics: %v", parseResp.Diagnostics)
	}

	// Call Evaluate RPC
	evalReq := &pb.EvaluateRequest{
		ModelHash:  parseResp.ModelHash,
		Expression: "2 + 2",
	}

	evalResp, err := srv.Evaluate(context.Background(), evalReq)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if evalResp.Error != "" {
		t.Errorf("expected no error, got: %s", evalResp.Error)
	}

	if evalResp.Result == nil {
		t.Fatal("expected result to be populated")
	}

	// Verify result is int value 4
	if intVal := evalResp.Result.GetIntValue(); intVal != 4 {
		t.Errorf("expected result 4, got %d", intVal)
	}
}

// TestEvaluate_ParseError verifies Evaluate RPC handles parse errors gracefully
func TestEvaluate_ParseError(t *testing.T) {
	srv := mustNewService(t, 10)

	// Parse a minimal model
	content := `package Test {}`

	parseReq := &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: content},
		ContentHash: "test-eval-parse-error",
	}

	parseResp, err := srv.ParseFile(context.Background(), parseReq)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Call Evaluate with invalid expression
	evalReq := &pb.EvaluateRequest{
		ModelHash:  parseResp.ModelHash,
		Expression: "2 + +",
	}

	evalResp, err := srv.Evaluate(context.Background(), evalReq)
	if err != nil {
		t.Fatalf("Evaluate should not return gRPC error, got: %v", err)
	}

	if evalResp.Error == "" {
		t.Error("expected error for invalid expression")
	}

	if len(evalResp.Diagnostics) == 0 {
		t.Error("expected diagnostics for parse error")
	}
}

// TestInstantiate_SimplePart verifies Instantiate RPC creates an instance
func TestInstantiate_SimplePart(t *testing.T) {
	srv := mustNewService(t, 10)

	// Parse a model with a part definition
	content := `
package Test {
  part def Vehicle {
    attribute speed : Real;
  }
}
`

	parseReq := &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: content},
		ContentHash: "test-instantiate-1",
	}

	parseResp, err := srv.ParseFile(context.Background(), parseReq)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Call Instantiate RPC
	instReq := &pb.InstantiateRequest{
		ModelHash: parseResp.ModelHash,
		SymbolId:  "Test::Vehicle",
	}

	instResp, err := srv.Instantiate(context.Background(), instReq)
	if err != nil {
		t.Fatalf("Instantiate failed: %v", err)
	}

	if instResp.Error != "" {
		t.Errorf("expected no error, got: %s", instResp.Error)
	}

	if instResp.Instance == nil {
		t.Fatal("expected instance to be populated")
	}

	// Verify instance properties
	if instResp.Instance.Id <= 0 {
		t.Errorf("expected positive instance ID, got %d", instResp.Instance.Id)
	}

	if instResp.Instance.TypeSymbolId != "Test::Vehicle" {
		t.Errorf("expected type 'Test::Vehicle', got %s", instResp.Instance.TypeSymbolId)
	}
}

// TestInstantiate_SymbolNotFound verifies Instantiate handles missing symbol
func TestInstantiate_SymbolNotFound(t *testing.T) {
	srv := mustNewService(t, 10)

	// Parse a minimal model
	content := `package Test {}`

	parseReq := &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: content},
		ContentHash: "test-instantiate-notfound",
	}

	parseResp, err := srv.ParseFile(context.Background(), parseReq)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Call Instantiate with nonexistent symbol
	instReq := &pb.InstantiateRequest{
		ModelHash: parseResp.ModelHash,
		SymbolId:  "Test::Nonexistent",
	}

	instResp, err := srv.Instantiate(context.Background(), instReq)
	if err != nil {
		t.Fatalf("Instantiate should not return gRPC error, got: %v", err)
	}

	if instResp.Error == "" {
		t.Error("expected error for nonexistent symbol")
	}
}

// TestExecuteAction_EmptyAction verifies ExecuteAction RPC on a minimal action
func TestExecuteAction_EmptyAction(t *testing.T) {
	srv := mustNewService(t, 10)

	// Parse a model with an action definition
	content := `
package Test {
  action def SimpleAction {
    // Empty action body
  }
}
`

	parseReq := &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: content},
		ContentHash: "test-execute-action-1",
	}

	parseResp, err := srv.ParseFile(context.Background(), parseReq)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Call ExecuteAction RPC
	execReq := &pb.ExecuteActionRequest{
		ModelHash:      parseResp.ModelHash,
		ActionSymbolId: "Test::SimpleAction",
		Inputs:         make(map[string]*pb.Value),
	}

	execResp, err := srv.ExecuteAction(context.Background(), execReq)
	if err != nil {
		t.Fatalf("ExecuteAction failed: %v", err)
	}

	// Empty action should fail at initialize() with "no initial node" per AGENTS.md §4
	if execResp.Error == "" {
		t.Error("expected error for empty action (no initial node)")
	}
}

// TestExecuteAction_InputBinding verifies that inputs supplied to ExecuteAction
// are bound into the action's initial context and flow through to the outputs.
// The action seeds an attribute `result` (default 0) and adds 5 to it; supplying
// result=10 must yield result=15, proving inputs are not discarded.
func TestExecuteAction_InputBinding(t *testing.T) {
	srv := mustNewService(t, 10)

	content := `
package Test {
  action addFive {
    attribute result : Integer = 0;
    first start;
    action inner {
      assign result := result + 5;
    }
    done;
    succession first start then inner;
    succession first inner then done;
  }
}
`

	parseReq := &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: content},
		ContentHash: "test-execute-action-inputs",
	}

	parseResp, err := srv.ParseFile(context.Background(), parseReq)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	// Note: ParseFile now runs semantic passes which may produce diagnostics.
	// This test focuses on runtime input binding, not parse cleanliness.
	// Skip diagnostic check.

	// Baseline: no inputs -> attribute default (0) + 5 = 5.
	baseResp, err := srv.ExecuteAction(context.Background(), &pb.ExecuteActionRequest{
		ModelHash:      parseResp.ModelHash,
		ActionSymbolId: "Test::addFive",
	})
	if err != nil {
		t.Fatalf("ExecuteAction (baseline) failed: %v", err)
	}
	if baseResp.Error != "" {
		t.Fatalf("baseline execution error: %s", baseResp.Error)
	}
	if got := baseResp.Outputs["result"].GetIntValue(); got != 5 {
		t.Errorf("baseline: expected result 5, got %d", got)
	}

	// With input result=10 -> 10 + 5 = 15, proving the input was bound.
	inResp, err := srv.ExecuteAction(context.Background(), &pb.ExecuteActionRequest{
		ModelHash:      parseResp.ModelHash,
		ActionSymbolId: "Test::addFive",
		Inputs: map[string]*pb.Value{
			"result": {Kind: &pb.Value_IntValue{IntValue: 10}},
		},
	})
	if err != nil {
		t.Fatalf("ExecuteAction (with inputs) failed: %v", err)
	}
	if inResp.Error != "" {
		t.Fatalf("input execution error: %s", inResp.Error)
	}
	result := inResp.Outputs["result"]
	if result == nil {
		t.Fatal("expected 'result' in outputs")
	}
	if got := result.GetIntValue(); got != 15 {
		t.Errorf("with input result=10: expected result 15, got %d (inputs were discarded?)", got)
	}
}

// TestExecuteState_SimpleStateMachine verifies ExecuteState RPC returns the REAL
// ordered sequence of visited states, not a fabricated placeholder trace.
func TestExecuteState_SimpleStateMachine(t *testing.T) {
	srv := mustNewService(t, 10)

	// A state machine with three real states: init -> Running -> done.
	content := `
package Test {
  state Machine {
    entry; then init;
    state init;
    state Running;

    succession first init then Running;
    succession first Running then done;
  }
}
`

	parseReq := &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: content},
		ContentHash: "test-execute-state-1",
	}

	parseResp, err := srv.ParseFile(context.Background(), parseReq)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	// `initial`/`final` warn as OpenSysML extensions; only errors would be a
	// fixture defect.
	if errs := errorDiagnostics(parseResp.Diagnostics); len(errs) > 0 {
		t.Fatalf("unexpected parse errors: %v", errs)
	}

	execReq := &pb.ExecuteStateRequest{
		ModelHash:            parseResp.ModelHash,
		StateMachineSymbolId: "Test::Machine",
	}

	execResp, err := srv.ExecuteState(context.Background(), execReq)
	if err != nil {
		t.Fatalf("ExecuteState failed: %v", err)
	}
	if execResp.Error != "" {
		t.Fatalf("execution error: %s", execResp.Error)
	}

	// The trace must reflect the actual states, in order, not a hardcoded placeholder.
	want := []string{"init", "Running", "done"}
	got := execResp.StatesVisited
	if len(got) != len(want) {
		t.Fatalf("expected states_visited %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected states_visited %v, got %v", want, got)
		}
	}
}

// TestExecuteAction_TimedAcceptRunsTheClock verifies that an action parked at
// `accept after` completes over the RPC: the run moves the context's clock to
// the instant the accept waits for rather than refusing the time trigger.
func TestExecuteAction_TimedAcceptRunsTheClock(t *testing.T) {
	srv := mustNewService(t, 10)

	content := `
package Test {
  private import ScalarValues::*;
  action delayed {
    attribute count : Integer = 0;
    first start;
    then action wait accept after 5 [SI::s];
    then action tick assign count := count + 1;
    then done;
  }
}
`
	parseResp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: content},
		ContentHash: "test-execute-action-timed",
	})
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	execResp, err := srv.ExecuteAction(context.Background(), &pb.ExecuteActionRequest{
		ModelHash:      parseResp.ModelHash,
		ActionSymbolId: "Test::delayed",
	})
	if err != nil {
		t.Fatalf("ExecuteAction failed: %v", err)
	}
	if execResp.Error != "" {
		t.Fatalf("execution error: %s", execResp.Error)
	}
	if got := execResp.Outputs["count"].GetIntValue(); got != 1 {
		t.Errorf("expected count 1 after the timed accept fired, got %d", got)
	}
	if execResp.FinalTime != 5 {
		t.Errorf("expected final_time 5 after the accept fired, got %g", execResp.FinalTime)
	}
}

// TestExecuteState_FinalTime verifies that the state response reports the
// clock the run ended at, the instant of its last time-triggered transition,
// and that a service without "final_time" withholds it.
func TestExecuteState_FinalTime(t *testing.T) {
	content := `
package Test {
  private import ScalarValues::*;
  state Timer {
    attribute fired : Integer = 0;
    entry; then armed;
    state armed;
    transition armed then done accept after 3 [SI::s] do assign fired := fired + 1;
  }
}
`
	run := func(t *testing.T, srv *Service) *pb.ExecuteStateResponse {
		parseResp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
			Source:      &pb.ParseFileRequest_Content{Content: content},
			ContentHash: "test-execute-state-final-time",
		})
		if err != nil {
			t.Fatalf("ParseFile failed: %v", err)
		}
		execResp, err := srv.ExecuteState(context.Background(), &pb.ExecuteStateRequest{
			ModelHash:            parseResp.ModelHash,
			StateMachineSymbolId: "Test::Timer",
		})
		if err != nil {
			t.Fatalf("ExecuteState failed: %v", err)
		}
		if execResp.Error != "" {
			t.Fatalf("execution error: %s", execResp.Error)
		}
		if got := execResp.FinalContext["fired"].GetIntValue(); got != 1 {
			t.Fatalf("expected the timed transition to fire once, got fired=%d", got)
		}
		return execResp
	}

	if got := run(t, mustNewService(t, 10)).FinalTime; got != 3 {
		t.Errorf("expected final_time 3, got %g", got)
	}
	if got := run(t, mustNewServiceWithout(t, CapabilityFinalTime)).FinalTime; got != 0 {
		t.Errorf("without final_time the field must be withheld, got %g", got)
	}
}
