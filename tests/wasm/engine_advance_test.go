package wasm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/engine"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/jsonrpc"
)

// timedActionSource is an action whose wait ends at t=5, so a run advanced by less stops short.
const timedActionSource = `
package Timed {
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

// advanceFixture parses sources on both the service and the engine and answers the
// shared model hash, with a comparator that fails where the two answers diverge.
func advanceFixture(t *testing.T, docs map[string]string) (hash string, svc *sysmlgrpc.Service, eng *engine.Engine, equal func(method, params string, grpcOut []byte) []byte) {
	t.Helper()
	ctx := context.Background()
	svc, err := sysmlgrpc.NewService(4, "parity")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	eng, err = engine.New()
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	documents := []map[string]string{}
	for name, content := range docs {
		documents = append(documents, map[string]string{"name": name, "content": content})
	}
	body, err := json.Marshal(map[string]any{"documents": documents})
	if err != nil {
		t.Fatalf("rendering the ParseSources request: %v", err)
	}
	grpcResp, err := svc.ParseSources(ctx, mustUnmarshal[pb.ParseSourcesRequest](t, string(body)))
	if err != nil {
		t.Fatalf("grpc ParseSources: %v", err)
	}
	var engineResp struct {
		ModelHash string `json:"modelHash"`
	}
	if err := json.Unmarshal(mustCall(t, eng, "ParseSources", string(body)), &engineResp); err != nil {
		t.Fatalf("decoding the engine's ParseSources response: %v", err)
	}
	if grpcResp.GetModelHash() != engineResp.ModelHash {
		t.Fatalf("modelHash: grpc %s, engine %s", grpcResp.GetModelHash(), engineResp.ModelHash)
	}
	equal = func(method, params string, grpcOut []byte) []byte {
		t.Helper()
		engineOut := mustCall(t, eng, method, params)
		var grpcJSON, engineJSON any
		if err := json.Unmarshal(grpcOut, &grpcJSON); err != nil {
			t.Fatalf("%s: the grpc answer is not JSON: %v\n%s", method, err, grpcOut)
		}
		if err := json.Unmarshal(engineOut, &engineJSON); err != nil {
			t.Fatalf("%s: the engine answer is not JSON: %v\n%s", method, err, engineOut)
		}
		if !reflect.DeepEqual(grpcJSON, engineJSON) {
			t.Errorf("%s diverges:\ngrpc:   %s\nengine: %s", method, grpcOut, engineOut)
		}
		return engineOut
	}
	return grpcResp.GetModelHash(), svc, eng, equal
}

// stateAnswer is the part of an ExecuteState answer a clocked run is judged by.
type stateAnswer struct {
	Error         string   `json:"error"`
	StatesVisited []string `json:"statesVisited"`
	FinalTime     float64  `json:"finalTime"`
	Trace         []struct {
		Kind string `json:"kind"`
		From string `json:"from"`
		To   string `json:"to"`
		Time struct {
			Quantity struct {
				RealMagnitude float64 `json:"realMagnitude"`
				Unit          string  `json:"unit"`
			} `json:"quantity"`
		} `json:"time"`
	} `json:"trace"`
}

// TestEngineStateAdvance runs the mission's clocked phases for a duration, as `sysml -state
// MissionSequence::mission -advance T` does, and matches the service answer for answer.
func TestEngineStateAdvance(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "examples", "runtime-showcase", "mission-sequence.sysml"))
	if err != nil {
		t.Fatalf("reading the mission fixture: %v", err)
	}
	hash, svc, _, equal := advanceFixture(t, map[string]string{"mission-sequence.sysml": string(source)})
	ctx := context.Background()

	for _, tc := range []struct {
		advance     int
		wantVisited []string
		wantFired   map[string]float64
	}{
		{400000, []string{"prelaunch", "translunarCoast", "lunarOrbit", "onSurface"},
			map[string]float64{"translunarCoast": 9000, "lunarOrbit": 279000, "onSurface": 379000}},
		{800000, []string{"prelaunch", "translunarCoast", "lunarOrbit", "onSurface", "transEarthCoast", "recovered", "done"},
			map[string]float64{"translunarCoast": 9000, "lunarOrbit": 279000, "onSurface": 379000, "transEarthCoast": 457000, "recovered": 702000, "done": 702000}},
	} {
		t.Run(fmt.Sprintf("advance %d", tc.advance), func(t *testing.T) {
			params := fmt.Sprintf(`{"modelHash":%q,"stateMachineSymbolId":"MissionSequence::MissionPhases","performerSymbolId":"MissionSequence::mission","advance":%d,"trace":true}`, hash, tc.advance)
			grpcRes, err := svc.ExecuteState(ctx, mustUnmarshal[pb.ExecuteStateRequest](t, params))
			if err != nil {
				t.Fatalf("grpc ExecuteState: %v", err)
			}
			var got stateAnswer
			if err := json.Unmarshal(equal("ExecuteState", params, mustMarshal(t, grpcRes)), &got); err != nil {
				t.Fatalf("decoding the engine answer: %v", err)
			}
			if got.Error != "" {
				t.Fatalf("ExecuteState error = %q", got.Error)
			}
			if !reflect.DeepEqual(got.StatesVisited, tc.wantVisited) {
				t.Errorf("statesVisited = %v, want %v", got.StatesVisited, tc.wantVisited)
			}
			if got.FinalTime != float64(tc.advance) {
				t.Errorf("finalTime = %g, want %d", got.FinalTime, tc.advance)
			}
			fired := map[string]float64{}
			for _, record := range got.Trace {
				if record.Kind != "transition" {
					continue
				}
				if record.Time.Quantity.Unit != "s" {
					t.Errorf("transition to %s is timed in %q, want seconds", record.To, record.Time.Quantity.Unit)
				}
				fired[record.To] = record.Time.Quantity.RealMagnitude
			}
			if !reflect.DeepEqual(fired, tc.wantFired) {
				t.Errorf("transition instants = %v, want %v", fired, tc.wantFired)
			}
		})
	}

	t.Run("quiescence without advance", func(t *testing.T) {
		// Without advance the machine runs through its clock, as the service does.
		params := fmt.Sprintf(`{"modelHash":%q,"stateMachineSymbolId":"MissionSequence::MissionPhases","performerSymbolId":"MissionSequence::mission"}`, hash)
		grpcRes, err := svc.ExecuteState(ctx, mustUnmarshal[pb.ExecuteStateRequest](t, params))
		if err != nil {
			t.Fatalf("grpc ExecuteState: %v", err)
		}
		var got stateAnswer
		if err := json.Unmarshal(equal("ExecuteState", params, mustMarshal(t, grpcRes)), &got); err != nil {
			t.Fatalf("decoding the engine answer: %v", err)
		}
		if got.StatesVisited[len(got.StatesVisited)-1] != "done" || got.FinalTime != 702000 {
			t.Errorf("without advance: statesVisited %v at t=%g, want the machine run through its clock to done at t=702000", got.StatesVisited, got.FinalTime)
		}
	})
}

// TestEngineActionAdvance runs an action for a duration: one stopped short of its wait
// says so and names the instant it needs, one run long enough completes, as on the service.
func TestEngineActionAdvance(t *testing.T) {
	hash, svc, eng, equal := advanceFixture(t, map[string]string{"timed.sysml": timedActionSource})
	ctx := context.Background()
	var short struct {
		Error     string  `json:"error"`
		FinalTime float64 `json:"finalTime"`
	}
	shortParams := fmt.Sprintf(`{"modelHash":%q,"actionSymbolId":"Timed::delayed","advance":2}`, hash)
	grpcShort, err := svc.ExecuteAction(ctx, mustUnmarshal[pb.ExecuteActionRequest](t, shortParams))
	if err != nil {
		t.Fatalf("grpc ExecuteAction: %v", err)
	}
	if err := json.Unmarshal(equal("ExecuteAction", shortParams, mustMarshal(t, grpcShort)), &short); err != nil {
		t.Fatalf("decoding the engine answer: %v", err)
	}
	if !strings.Contains(short.Error, "without completing") || !strings.Contains(short.Error, "t=5") || short.FinalTime != 2 {
		t.Errorf("advance 2 = %q at t=%g, want the action stopped short of its wait at t=5 with the clock at 2", short.Error, short.FinalTime)
	}

	longParams := fmt.Sprintf(`{"modelHash":%q,"actionSymbolId":"Timed::delayed","advance":10}`, hash)
	grpcLong, err := svc.ExecuteAction(ctx, mustUnmarshal[pb.ExecuteActionRequest](t, longParams))
	if err != nil {
		t.Fatalf("grpc ExecuteAction: %v", err)
	}
	long := string(equal("ExecuteAction", longParams, mustMarshal(t, grpcLong)))
	if strings.Contains(long, `"error"`) || !strings.Contains(long, `"finalTime":10`) {
		t.Errorf("advance 10 = %s, want a completed run with the clock at 10", long)
	}

	for method, params := range map[string]string{
		"ExecuteAction": fmt.Sprintf(`{"modelHash":%q,"actionSymbolId":"Timed::delayed","advance":-1}`, hash),
		"ExecuteState":  fmt.Sprintf(`{"modelHash":%q,"stateMachineSymbolId":"Timed::delayed","advance":-1}`, hash),
	} {
		_, err := eng.Call(ctx, method, []byte(params))
		var callErr *jsonrpc.Error
		if !errors.As(err, &callErr) || callErr.Code != 3 || !strings.Contains(callErr.Message, "finite, non-negative") {
			t.Errorf("%s with advance -1: Call error = %v, want InvalidArgument (3) refusing the duration", method, err)
		}
	}
}
