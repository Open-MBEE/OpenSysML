package grpc

import (
	"context"
	"encoding/json"
	"slices"
	"sync"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis/modelform"
)

const inheritedBehaviorModel = `
package Demo {
    part def Plant {
        attribute runs : ScalarValues::Integer = 0;
        action fill { assign runs := runs + 1; }
        state machine { entry; then idle; state idle; }
    }
    part def BigPlant :> Plant { }
    part def HugePlant :> BigPlant { }
    part def RedefPlant :> Plant {
        action fill :>> fill { assign runs := runs + 10; }
    }
    part p : BigPlant;
    part h : HugePlant;
    part r : RedefPlant;
}
`

// ExecuteAction names an inherited action by the subtype's qualified name too,
// runs it on the performer, and a redefinition in the subtype is the one it
// runs: its own step beside the step it inherits, not the inherited action alone.
func TestExecuteAction_InheritedActionBySubtypesQualifiedName(t *testing.T) {
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: inheritedBehaviorModel},
	})
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	for _, tc := range []struct {
		action, performer string
		runs              int64
	}{
		{"Demo::BigPlant::fill", "Demo::p", 1},
		{"Demo::HugePlant::fill", "Demo::h", 1},
		{"Demo::RedefPlant::fill", "Demo::r", 11},
	} {
		resp, err := srv.ExecuteAction(context.Background(), &pb.ExecuteActionRequest{
			ModelHash:         parsed.ModelHash,
			ActionSymbolId:    tc.action,
			PerformerSymbolId: tc.performer,
		})
		if err != nil {
			t.Fatalf("ExecuteAction(%s) failed: %v", tc.action, err)
		}
		if resp.Error != "" {
			t.Fatalf("ExecuteAction(%s): %s", tc.action, resp.Error)
		}
		if got, held := resp.PerformerAttributes["this.runs"]; !held || got.GetIntValue() != tc.runs {
			t.Errorf("ExecuteAction(%s) left runs = %v, want %d", tc.action, got, tc.runs)
		}
	}

	resp, err := srv.ExecuteAction(context.Background(), &pb.ExecuteActionRequest{
		ModelHash:         parsed.ModelHash,
		ActionSymbolId:    "Demo::BigPlant::fil",
		PerformerSymbolId: "Demo::p",
	})
	if err != nil {
		t.Fatalf("ExecuteAction failed: %v", err)
	}
	if resp.Error != "action not found: Demo::BigPlant::fil" {
		t.Errorf("a miss answered %q", resp.Error)
	}
}

// ExecuteState names an inherited state machine by the subtype's qualified name too.
func TestExecuteState_InheritedMachineBySubtypesQualifiedName(t *testing.T) {
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: inheritedBehaviorModel},
	})
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	resp, err := srv.ExecuteState(context.Background(), &pb.ExecuteStateRequest{
		ModelHash:            parsed.ModelHash,
		StateMachineSymbolId: "Demo::HugePlant::machine",
		PerformerSymbolId:    "Demo::h",
	})
	if err != nil {
		t.Fatalf("ExecuteState failed: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("ExecuteState: %s", resp.Error)
	}
	if !slices.Contains(resp.StatesVisited, "idle") {
		t.Errorf("states visited = %v, want idle", resp.StatesVisited)
	}

	resp, err = srv.ExecuteState(context.Background(), &pb.ExecuteStateRequest{
		ModelHash:            parsed.ModelHash,
		StateMachineSymbolId: "Demo::HugePlant::machin",
	})
	if err != nil {
		t.Fatalf("ExecuteState failed: %v", err)
	}
	if resp.Error != "state machine not found: Demo::HugePlant::machin" {
		t.Errorf("a miss answered %q", resp.Error)
	}
}

// ExportGraphs takes the subtype's qualified name for an inherited behavior too.
func TestExportGraphs_InheritedActionBySubtypesQualifiedName(t *testing.T) {
	srv := mustNewService(t, 10)
	t.Cleanup(srv.Close)
	parsed, _ := parseContent(t, srv, inheritedBehaviorModel)

	resp, err := srv.ExportGraphs(context.Background(), &pb.ExportGraphsRequest{ModelHash: parsed.ModelHash, Subject: "Demo::HugePlant::fill"})
	if err != nil {
		t.Fatalf("ExportGraphs: %v", err)
	}
	var graphs modelform.Graphs
	if err := json.Unmarshal([]byte(resp.Content), &graphs); err != nil {
		t.Fatalf("content is not graphs JSON: %v", err)
	}
	if len(graphs.Actions) != 1 || graphs.Actions[0].Name != "Demo::Plant::fill" {
		t.Errorf("graphs = %+v, want the inherited action alone", graphs.Actions)
	}

	_, err = srv.ExportGraphs(context.Background(), &pb.ExportGraphsRequest{ModelHash: parsed.ModelHash, Subject: "Demo::HugePlant::fil"})
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("a miss answered %v, want NOT_FOUND", err)
	}
}

// Concurrent requests naming inherited members share one cached model, whose
// semantic model memoizes the member walk: the lookups must serialize on it.
func TestExecuteAction_ConcurrentInheritedLookupsShareTheModel(t *testing.T) {
	srv := mustNewService(t, 10)
	parsed, _ := parseContent(t, srv, inheritedBehaviorModel)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		for _, tc := range []struct{ action, performer string }{
			{"Demo::BigPlant::fill", "Demo::p"},
			{"Demo::HugePlant::fill", "Demo::h"},
		} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				resp, err := srv.ExecuteAction(context.Background(), &pb.ExecuteActionRequest{
					ModelHash: parsed.ModelHash, ActionSymbolId: tc.action, PerformerSymbolId: tc.performer,
				})
				if err != nil || resp.Error != "" {
					t.Errorf("ExecuteAction(%s): %v %s", tc.action, err, resp.GetError())
				}
			}()
		}
	}
	wg.Wait()
}
