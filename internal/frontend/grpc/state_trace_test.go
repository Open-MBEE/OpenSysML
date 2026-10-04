package grpc

import (
	"context"
	"math"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

const stateTraceMachine = `
package Trace {
  item def Go;
  state Machine {
    entry; then idle;
    state idle;
    state done;
    transition first idle accept Go then done;
  }
}
`

func TestTraceDroppedCountToInt32(t *testing.T) {
	tests := []struct {
		name    string
		dropped int64
		want    int32
	}{
		{name: "within range", dropped: 42, want: 42},
		{name: "negative", dropped: -1, want: 0},
		{name: "overflow", dropped: int64(math.MaxInt32) + 1, want: math.MaxInt32},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := traceDroppedCountToInt32(test.dropped); got != test.want {
				t.Fatalf("traceDroppedCountToInt32(%d) = %d, want %d", test.dropped, got, test.want)
			}
		})
	}
}

func parseStateTraceModel(t *testing.T, service *Service, content string) string {
	t.Helper()
	response, err := service.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: content},
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if errors := errorDiagnostics(response.Diagnostics); len(errors) != 0 {
		t.Fatalf("ParseFile diagnostics: %v", errors)
	}
	return response.ModelHash
}

func TestExecuteStateTraceOrderAndOptIn(t *testing.T) {
	service := mustNewService(t, 10)
	modelHash := parseStateTraceModel(t, service, stateTraceMachine)
	request := &pb.ExecuteStateRequest{
		ModelHash:            modelHash,
		StateMachineSymbolId: "Trace::Machine",
		Events:               []string{"Go"},
	}
	plain, err := service.ExecuteState(context.Background(), request)
	if err != nil {
		t.Fatalf("ExecuteState without trace: %v", err)
	}
	if len(plain.Trace) != 0 || plain.TraceDropped != 0 {
		t.Fatalf("unrequested trace = %v dropped %d", plain.Trace, plain.TraceDropped)
	}

	request.Trace = true
	traced, err := service.ExecuteState(context.Background(), request)
	if err != nil {
		t.Fatalf("ExecuteState with trace: %v", err)
	}
	if traced.Error != "" {
		t.Fatalf("execution error: %s", traced.Error)
	}
	wantKinds := []string{"entry", "accept", "exit", "entry", "transition"}
	if len(traced.Trace) != len(wantKinds) {
		t.Fatalf("trace has %d records, want %d: %v", len(traced.Trace), len(wantKinds), traced.Trace)
	}
	for i, want := range wantKinds {
		if got := traced.Trace[i].Kind; got != want {
			t.Errorf("trace[%d].kind = %q, want %q", i, got, want)
		}
	}
	if traced.Trace[0].State != "idle" ||
		traced.Trace[1].Event != "Go" ||
		traced.Trace[2].State != "idle" ||
		traced.Trace[3].State != "done" ||
		traced.Trace[4].From != "idle" ||
		traced.Trace[4].To != "done" ||
		traced.Trace[4].Event != "accept Go" {
		t.Fatalf("unexpected event details: %v", traced.Trace)
	}
}

func TestExecuteStateTraceReportsChoice(t *testing.T) {
	service := mustNewService(t, 10)
	modelHash := parseStateTraceModel(t, service, `
package Trace {
  item def Go;
  state Machine {
    entry; then idle;
    state idle;
    state low;
    state high;
    transition first idle accept Go then low;
    transition first idle accept Go then high;
  }
}
`)
	response, err := service.ExecuteState(context.Background(), &pb.ExecuteStateRequest{
		ModelHash:            modelHash,
		StateMachineSymbolId: "Trace::Machine",
		Events:               []string{"Go"},
		Schedule:             "seed:3",
		Trace:                true,
	})
	if err != nil {
		t.Fatalf("ExecuteState: %v", err)
	}
	if response.Error != "" {
		t.Fatalf("execution error: %s", response.Error)
	}
	for _, event := range response.Trace {
		if event.Kind != "choice" {
			continue
		}
		if len(event.Alternatives) != 2 || event.Taken == "" {
			t.Fatalf("choice trace = alternatives %v, taken %q", event.Alternatives, event.Taken)
		}
		if !strings.Contains(strings.Join(event.Alternatives, ","), event.Taken) {
			t.Fatalf("taken choice %q not in %v", event.Taken, event.Alternatives)
		}
		return
	}
	t.Fatalf("trace has no choice record: %v", response.Trace)
}

func TestExecuteStateTraceCapabilityAndExplore(t *testing.T) {
	_, err := mustNewServiceWithout(t, CapabilityStateTrace).ExecuteState(context.Background(), &pb.ExecuteStateRequest{Trace: true})
	if connect.CodeOf(err) != connect.CodeUnimplemented || !strings.Contains(err.Error(), CapabilityStateTrace) {
		t.Fatalf("without capability: %v, want UNIMPLEMENTED naming %q", err, CapabilityStateTrace)
	}
	_, err = mustNewService(t, 10).ExecuteState(context.Background(), &pb.ExecuteStateRequest{
		Schedule: "explore", Trace: true,
	})
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("explore with trace status = %s, want INVALID_ARGUMENT: %v", connect.CodeOf(err), err)
	}
}

func TestExecuteStateTraceRetainsFailuresAndReportsDroppedRecords(t *testing.T) {
	service := mustNewService(t, 10)
	modelHash := parseStateTraceModel(t, service, `
package Trace {
  item def Go;
  item def Again;
  state Machine {
    entry; then idle;
    state idle;
    state middle;
    state done;
    transition first idle accept Go then middle;
    transition first middle accept Again then done;
  }
}
`)
	service.budgets.MaxStateEvents = 1
	failed, err := service.ExecuteState(context.Background(), &pb.ExecuteStateRequest{
		ModelHash:            modelHash,
		StateMachineSymbolId: "Trace::Machine",
		Events:               []string{"Go", "Again"},
		Trace:                true,
	})
	if err != nil {
		t.Fatalf("ExecuteState: %v", err)
	}
	if failed.Error == "" || len(failed.Trace) == 0 {
		t.Fatalf("failed run response = error %q, trace %v; want partial trace", failed.Error, failed.Trace)
	}

	bounded := mustNewService(t, 10)
	bounded.maxHeldEvents = 2
	modelHash = parseStateTraceModel(t, bounded, stateTraceMachine)
	response, err := bounded.ExecuteState(context.Background(), &pb.ExecuteStateRequest{
		ModelHash:            modelHash,
		StateMachineSymbolId: "Trace::Machine",
		Events:               []string{"Go"},
		Trace:                true,
	})
	if err != nil {
		t.Fatalf("ExecuteState: %v", err)
	}
	if len(response.Trace) != 2 || response.TraceDropped == 0 {
		t.Fatalf("bounded trace = %d records, dropped %d; want 2 records and drops", len(response.Trace), response.TraceDropped)
	}
}
