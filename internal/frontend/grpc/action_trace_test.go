package grpc

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

const actionTraceModel = `
package Trace {
  private import ScalarValues::*;
  item def Go;
  action def Flow {
    in budget : Integer;
    attribute remaining : Integer = budget;
    out landed : Boolean = false;
    first start;
    then action burn assign remaining := remaining - 3;
    then decide route;
      if remaining >= 5 then waiting;
      else finish;
    action waiting accept go : Go;
    then action land assign landed := true;
    then finish;
    action finish;
    then done;
  }
  part def Craft;
  part craft : Craft;
}
`

func traceTexts(trace []*pb.DocumentEvent) []string {
	texts := make([]string, len(trace))
	for i, event := range trace {
		texts[i] = event.Text
	}
	return texts
}

func TestExecuteActionTraceRecordsTheRun(t *testing.T) {
	service := mustNewService(t, 10)
	modelHash := parseStateTraceModel(t, service, actionTraceModel)
	request := &pb.ExecuteActionRequest{
		ModelHash:      modelHash,
		ActionSymbolId: "Trace::Flow",
		Inputs:         map[string]*pb.Value{"budget": {Kind: &pb.Value_IntValue{IntValue: 2}}},
	}
	plain, err := service.ExecuteAction(context.Background(), request)
	if err != nil {
		t.Fatalf("ExecuteAction without trace: %v", err)
	}
	if plain.Error != "" || len(plain.Trace) != 0 || plain.TraceDropped != 0 {
		t.Fatalf("unrequested trace = %v dropped %d error %q", plain.Trace, plain.TraceDropped, plain.Error)
	}

	request.Trace = true
	traced, err := service.ExecuteAction(context.Background(), request)
	if err != nil {
		t.Fatalf("ExecuteAction with trace: %v", err)
	}
	if traced.Error != "" {
		t.Fatalf("execution error: %s", traced.Error)
	}
	want := []string{
		"step start",
		"step start -> burn",
		"assign remaining = -1",
		"step burn -> route",
		"decide route -> 2->finish",
		"step route -> finish",
		"step finish -> done",
		"complete",
	}
	if got := traceTexts(traced.Trace); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("trace:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	assign := traced.Trace[2]
	if assign.Kind != "assign" || assign.Feature != "remaining" || assign.Token != 1 || assign.Step != 2 || assign.State != "burn" {
		t.Fatalf("assign record = %v", assign)
	}
	if value, ok := assign.Value.Kind.(*pb.Value_IntValue); !ok || value.IntValue != -1 {
		t.Fatalf("assign value = %v, want -1", assign.Value)
	}
	decide := traced.Trace[4]
	if decide.Kind != "decide" || decide.State != "route" || decide.Taken != "2->finish" ||
		strings.Join(decide.Alternatives, ",") != "1->waiting,2->finish" {
		t.Fatalf("decide record = %v", decide)
	}
	if step := traced.Trace[1]; step.Kind != "step" || step.From != "start" || step.To != "burn" || step.Token != 1 || step.Step != 1 {
		t.Fatalf("step record = %v", step)
	}
}

func TestExecuteActionEventsReachAWaitingAccept(t *testing.T) {
	service := mustNewService(t, 10)
	modelHash := parseStateTraceModel(t, service, actionTraceModel)
	request := &pb.ExecuteActionRequest{
		ModelHash:      modelHash,
		ActionSymbolId: "Trace::Flow",
		Inputs:         map[string]*pb.Value{"budget": {Kind: &pb.Value_IntValue{IntValue: 10}}},
		Trace:          true,
	}
	_, err := service.ExecuteAction(context.Background(), &pb.ExecuteActionRequest{
		ModelHash: modelHash, ActionSymbolId: "Trace::Flow", Events: []string{"Go"},
	})
	if connect.CodeOf(err) != connect.CodeInvalidArgument || !strings.Contains(err.Error(), "performer_symbol_id") {
		t.Fatalf("events without a performer: %v, want INVALID_ARGUMENT naming performer_symbol_id", err)
	}
	request.PerformerSymbolId = "Trace::craft"
	waiting, err := service.ExecuteAction(context.Background(), request)
	if err != nil {
		t.Fatalf("ExecuteAction without events: %v", err)
	}
	if !strings.Contains(waiting.Error, "accept deadlock") || !strings.Contains(waiting.Error, "Go") {
		t.Fatalf("error without the event = %q, want an accept deadlock naming Go", waiting.Error)
	}
	texts := traceTexts(waiting.Trace)
	if len(texts) == 0 || texts[len(texts)-1] != "wait Go" {
		t.Fatalf("trace without the event ends in %v, want a wait for Go", texts)
	}

	request.Events = []string{"Go"}
	landed, err := service.ExecuteAction(context.Background(), request)
	if err != nil {
		t.Fatalf("ExecuteAction with the event: %v", err)
	}
	if landed.Error != "" {
		t.Fatalf("execution error: %s", landed.Error)
	}
	if got := landed.Outputs["landed"].String(); !strings.Contains(got, "bool_value:true") {
		t.Fatalf("landed = %s, want true", got)
	}
	texts = traceTexts(landed.Trace)
	joined := strings.Join(texts, "\n")
	for _, want := range []string{"wait Go\n", "send Go\n", "accept Go\n", "step waiting -> land\n", "assign landed = [true]\n", "complete"} {
		if !strings.Contains(joined+"\n", want) {
			t.Errorf("trace lacks %q:\n%s", strings.TrimSpace(want), joined)
		}
	}
	if strings.Index(joined, "wait Go") > strings.Index(joined, "send Go") || strings.Index(joined, "send Go") > strings.Index(joined, "accept Go") {
		t.Fatalf("the event was not posted after the wait and taken after the post:\n%s", joined)
	}

	request.Inputs = map[string]*pb.Value{"budget": {Kind: &pb.Value_IntValue{IntValue: 2}}}
	unawaited, err := service.ExecuteAction(context.Background(), request)
	if err != nil {
		t.Fatalf("ExecuteAction with an unawaited event: %v", err)
	}
	if !strings.Contains(unawaited.Error, "event Go was posted after the action completed") {
		t.Fatalf("error for an event after completion = %q", unawaited.Error)
	}
}

func TestExecuteActionTraceCapabilityAndExplore(t *testing.T) {
	_, err := mustNewServiceWithout(t, CapabilityActionTrace).ExecuteAction(context.Background(), &pb.ExecuteActionRequest{Trace: true})
	if connect.CodeOf(err) != connect.CodeUnimplemented || !strings.Contains(err.Error(), CapabilityActionTrace) {
		t.Fatalf("without capability: %v, want UNIMPLEMENTED naming %q", err, CapabilityActionTrace)
	}
	service := mustNewService(t, 10)
	_, err = service.ExecuteAction(context.Background(), &pb.ExecuteActionRequest{Schedule: "explore", Trace: true})
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("trace under explore: %v, want INVALID_ARGUMENT", err)
	}
	_, err = service.ExecuteAction(context.Background(), &pb.ExecuteActionRequest{Schedule: "explore", Events: []string{"Go"}})
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("events under explore: %v, want INVALID_ARGUMENT", err)
	}
}
