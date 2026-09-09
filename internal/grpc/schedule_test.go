package grpc

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

const scheduleModel = `
package Sched {
  private import ScalarValues::*;

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

  state Dispatcher {
    attribute level : Integer = 8;
    entry; then idle;
    state idle;
    state low;
    state high;
    transition first idle accept Go if level > 5 then low;
    transition first idle accept Go if level > 7 then high;
  }

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
`

// The schedule field selects the order a run resolves its choice points in,
// and the choice it reports is the one that policy took; empty is the default.
func TestScheduleFieldSelectsThePolicyOnEveryRunRPC(t *testing.T) {
	ctx := context.Background()
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, scheduleModel, "schedule-field")

	tokenChoice := func(first string) string {
		return "choice point: step 3: tokens 2@left, 3@right (unordered; took " + first + " first)"
	}
	for _, test := range []struct{ schedule, want string }{
		{"", tokenChoice("3@right")},
		{"reverse", tokenChoice("3@right")},
		{"declared", tokenChoice("2@left")},
	} {
		resp, err := srv.ExecuteAction(ctx, &pb.ExecuteActionRequest{
			ModelHash: hash, ActionSymbolId: "Sched::tally", Schedule: test.schedule,
		})
		if err != nil || resp.Error != "" {
			t.Fatalf("ExecuteAction under %q: %v %q", test.schedule, err, resp.GetError())
		}
		if choices := choiceDiagnostics(resp.Diagnostics); len(choices) != 1 || choices[0].Message != test.want {
			t.Errorf("ExecuteAction under %q reported %v, want %q", test.schedule, choices, test.want)
		}
	}

	transitionChoice := func(took string) string {
		return "choice point: state idle on accept Go: transitions 1->low, 2->high (unordered; took " + took + ")"
	}
	for _, test := range []struct{ schedule, visited, want string }{
		{"", "idle,low", transitionChoice("1->low")},
		{"declared", "idle,low", transitionChoice("1->low")},
		{"seed:3", "idle,high", transitionChoice("2->high")},
	} {
		resp, err := srv.ExecuteState(ctx, &pb.ExecuteStateRequest{
			ModelHash: hash, StateMachineSymbolId: "Sched::Dispatcher", Events: []string{"Go"}, Schedule: test.schedule,
		})
		if err != nil || resp.Error != "" {
			t.Fatalf("ExecuteState under %q: %v %q", test.schedule, err, resp.GetError())
		}
		if got := strings.Join(resp.StatesVisited, ","); got != test.visited {
			t.Errorf("ExecuteState under %q visited %q, want %q", test.schedule, got, test.visited)
		}
		if choices := choiceDiagnostics(resp.Diagnostics); len(choices) != 1 || choices[0].Message != test.want {
			t.Errorf("ExecuteState under %q reported %v, want %q", test.schedule, choices, test.want)
		}
	}

	for _, test := range []struct{ schedule, want string }{
		{"", "choice point: step 3: tokens 2@left, 3@right (unordered; took 3@right first)"},
		{"declared", "choice point: step 3: tokens 2@left, 3@right (unordered; took 2@left first)"},
	} {
		resp := runAnalysis(t, srv, &pb.RunAnalysisRequest{ModelHash: hash, SymbolId: "Sched::forked", Schedule: test.schedule})
		if resp.Error != "" {
			t.Fatalf("RunAnalysis under %q reported %q", test.schedule, resp.Error)
		}
		var found bool
		for _, d := range choiceDiagnostics(resp.Diagnostics) {
			found = found || d.Message == test.want
		}
		if !found {
			t.Errorf("RunAnalysis under %q reported %v, want %q", test.schedule, resp.Diagnostics, test.want)
		}
	}
}

// The same seed orders the same run the same way, request after request.
func TestSeededScheduleIsReproducibleOverTheWire(t *testing.T) {
	ctx := context.Background()
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, scheduleModel, "schedule-seed")

	var first []string
	for i := 0; i < 3; i++ {
		resp, err := srv.ExecuteAction(ctx, &pb.ExecuteActionRequest{
			ModelHash: hash, ActionSymbolId: "Sched::tally", Schedule: "seed:7",
		})
		if err != nil || resp.Error != "" {
			t.Fatalf("ExecuteAction: %v %q", err, resp.GetError())
		}
		var messages []string
		for _, d := range choiceDiagnostics(resp.Diagnostics) {
			messages = append(messages, d.Message)
		}
		if first == nil {
			first = messages
		} else if strings.Join(first, "\n") != strings.Join(messages, "\n") {
			t.Fatalf("run %d under seed:7 chose %v, the first chose %v", i, messages, first)
		}
	}
	if len(first) != 1 {
		t.Fatalf("choices under seed:7 = %v, want one", first)
	}
}

// A spelling that names no policy is INVALID_ARGUMENT before anything runs,
// on every RPC carrying the field, whether or not the model exists.
func TestAnUnknownScheduleIsInvalidArgument(t *testing.T) {
	ctx := context.Background()
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, scheduleModel, "schedule-invalid")

	for _, spelling := range []string{"random", "seed", "seed:", "seed:-1", "seed:abc", "Declared", " declared", "explore:", "explore:runs=x"} {
		calls := map[string]func(hash string) error{
			"ExecuteAction": func(hash string) error {
				_, err := srv.ExecuteAction(ctx, &pb.ExecuteActionRequest{ModelHash: hash, ActionSymbolId: "Sched::tally", Schedule: spelling})
				return err
			},
			"ExecuteState": func(hash string) error {
				_, err := srv.ExecuteState(ctx, &pb.ExecuteStateRequest{ModelHash: hash, StateMachineSymbolId: "Sched::Dispatcher", Schedule: spelling})
				return err
			},
			"RunAnalysis": func(hash string) error {
				_, err := srv.RunAnalysis(ctx, &pb.RunAnalysisRequest{ModelHash: hash, SymbolId: "Sched::forked", Schedule: spelling})
				return err
			},
		}
		for name, call := range calls {
			for _, h := range []string{hash, "missing"} {
				err := call(h)
				if connect.CodeOf(err) != connect.CodeInvalidArgument {
					t.Errorf("%s schedule %q on model %q: status %s, want INVALID_ARGUMENT: %v", name, spelling, h, connect.CodeOf(err), err)
					continue
				}
				if !strings.Contains(err.Error(), spelling) {
					t.Errorf("%s schedule %q: message %q does not name the spelling", name, spelling, err.Error())
				}
			}
		}
	}
}

// The schedule field is read before the capability the rest of the request
// needs, so a malformed spelling is INVALID_ARGUMENT on a service that withholds
// verification too; a valid one there is still refused for verification.
func TestAnUnknownScheduleIsInvalidArgumentWithoutVerification(t *testing.T) {
	ctx := context.Background()
	srv := mustNewServiceWithout(t, CapabilityVerification)
	_, err := srv.RunAnalysis(ctx, &pb.RunAnalysisRequest{ModelHash: "missing", SymbolId: "Sched::forked", Schedule: "seed:abc"})
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("RunAnalysis schedule seed:abc without verification: status %s, want INVALID_ARGUMENT: %v", connect.CodeOf(err), err)
	}
	_, err = srv.RunAnalysis(ctx, &pb.RunAnalysisRequest{ModelHash: "missing", SymbolId: "Sched::forked", Schedule: "seed:1"})
	if connect.CodeOf(err) != connect.CodeUnimplemented || !strings.Contains(err.Error(), CapabilityVerification) {
		t.Errorf("RunAnalysis schedule seed:1 without verification: status %s, want UNIMPLEMENTED naming %s: %v", connect.CodeOf(err), CapabilityVerification, err)
	}
}
