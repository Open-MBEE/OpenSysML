package grpc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

const exploreModel = `
package Race {
  private import ScalarValues::*;

  action race {
    attribute x : Integer = 0;
    first start;
    fork split;
    action a { assign x := 1; }
    action b { assign x := 2; }
    action c { assign x := 3; }
    join sync;
    done;
    succession first start then split;
    succession first split then a;
    succession first split then b;
    succession first split then c;
    succession first a then sync;
    succession first b then sync;
    succession first c then sync;
    succession first sync then done;
  }

  action straight {
    attribute n : Integer = 0;
    first start;
    action inc { assign n := n + 1; }
    done;
    succession first start then inc;
    succession first inc then done;
  }

  action def Race {
    out r : Integer = 0;
    first start;
    fork split;
    action a { assign r := 1; }
    action b { assign r := 2; }
    join sync;
    done;
    succession first start then split;
    succession first split then a;
    succession first split then b;
    succession first a then sync;
    succession first b then sync;
    succession first sync then done;
  }

  analysis raced {
    out r : Integer;
    perform action race : Race;
    return : Integer = r;
  }
}
`

func outcomeInts(t *testing.T, outcomes []*pb.Outcome, name string) []int64 {
	t.Helper()
	got := make([]int64, 0, len(outcomes))
	for _, o := range outcomes {
		val, ok := o.Outputs[name]
		if !ok {
			t.Fatalf("outcome %v carries no %q", o, name)
		}
		got = append(got, val.GetIntValue())
	}
	return got
}

// Under explore an action answers with every distinct outcome, its linearization
// count and a witness, in canonical order, and an ordinary run's fields stay empty.
func TestExploreActionOverTheWire(t *testing.T) {
	ctx := context.Background()
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, exploreModel, "explore-action")

	var first *pb.ExecuteActionResponse
	for i := 0; i < 2; i++ {
		resp, err := srv.ExecuteAction(ctx, &pb.ExecuteActionRequest{ModelHash: hash, ActionSymbolId: "Race::race", Schedule: "explore"})
		if err != nil || resp.Error != "" {
			t.Fatalf("ExecuteAction under explore: %v %q", err, resp.GetError())
		}
		if first == nil {
			first = resp
		} else if resp.String() != first.String() {
			t.Fatalf("second exploration answered\n%v\nthe first\n%v", resp, first)
		}
	}
	if len(first.Outputs) != 0 || len(first.Diagnostics) != 0 {
		t.Errorf("an explored run answered a single run's outputs %v / diagnostics %v", first.Outputs, first.Diagnostics)
	}
	x := first.Exploration
	if x == nil || !x.Complete || x.Runs != 6 || len(x.BudgetsHit) != 0 || x.RunsBudget != 1024 || x.DepthBudget != 64 {
		t.Fatalf("exploration status %v, want complete after 6 runs under the default budget", x)
	}
	if got := outcomeInts(t, first.Outcomes, "x"); len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("outcomes x = %v, want [1 2 3]", got)
	}
	for _, o := range first.Outcomes {
		if o.Linearizations != 2 {
			t.Errorf("outcome x=%d reached by %d linearizations, want 2", o.Outputs["x"].GetIntValue(), o.Linearizations)
		}
		if len(o.Witness) == 0 || !strings.Contains(o.Witness[0], "first of") {
			t.Errorf("outcome x=%d witness %v, want token-order choices", o.Outputs["x"].GetIntValue(), o.Witness)
		}
		if o.Error != "" || o.FinalState != "" || len(o.StatesVisited) != 0 {
			t.Errorf("action outcome carries state or error fields: %v", o)
		}
		if choices := choiceDiagnostics(o.Diagnostics); len(choices) != len(o.Witness) {
			t.Errorf("outcome reports %d choice diagnostics for %d witness choices", len(choices), len(o.Witness))
		}
	}
}

// A behavior with no choice point explores in one run; a budget hit is reported
// as incomplete, naming the budget, never as an error.
func TestExploreBudgetsOverTheWire(t *testing.T) {
	ctx := context.Background()
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, exploreModel, "explore-budget")

	resp, err := srv.ExecuteAction(ctx, &pb.ExecuteActionRequest{ModelHash: hash, ActionSymbolId: "Race::straight", Schedule: "explore"})
	if err != nil || resp.Error != "" {
		t.Fatalf("ExecuteAction straight: %v %q", err, resp.GetError())
	}
	if x := resp.Exploration; !x.Complete || x.Runs != 1 || len(resp.Outcomes) != 1 || len(resp.Outcomes[0].Witness) != 0 {
		t.Errorf("no-choice exploration: %v, outcomes %v; want one complete run with an empty witness", x, resp.Outcomes)
	}

	for _, test := range []struct {
		schedule, budget string
		runs             int32
	}{
		{"explore:runs=2", "runs", 2},
		{"explore:depth=0", "depth", 1},
		{"explore:depth=1,runs=1", "runs,depth", 1},
	} {
		resp, err := srv.ExecuteAction(ctx, &pb.ExecuteActionRequest{ModelHash: hash, ActionSymbolId: "Race::race", Schedule: test.schedule})
		if err != nil || resp.Error != "" {
			t.Fatalf("ExecuteAction under %q: %v %q", test.schedule, err, resp.GetError())
		}
		x := resp.Exploration
		if x.Complete || x.Runs != test.runs || strings.Join(x.BudgetsHit, ",") != test.budget {
			t.Errorf("under %q status %v, want incomplete on %s after %d runs", test.schedule, x, test.budget, test.runs)
		}
		if len(resp.Outcomes) == 0 {
			t.Errorf("under %q no outcome of the runs made was reported", test.schedule)
		}
	}
}

// A state machine's outcomes carry the state it rests in and the states entered.
func TestExploreStateOverTheWire(t *testing.T) {
	ctx := context.Background()
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, scheduleModel, "explore-state")

	resp, err := srv.ExecuteState(ctx, &pb.ExecuteStateRequest{
		ModelHash: hash, StateMachineSymbolId: "Sched::Dispatcher", Events: []string{"Go"}, Schedule: "explore",
	})
	if err != nil || resp.Error != "" {
		t.Fatalf("ExecuteState under explore: %v %q", err, resp.GetError())
	}
	if x := resp.Exploration; !x.Complete || x.Runs != 2 {
		t.Fatalf("exploration status %v, want complete after 2 runs", x)
	}
	if len(resp.FinalContext) != 0 || len(resp.StatesVisited) != 0 || len(resp.Diagnostics) != 0 {
		t.Errorf("an explored run answered a single run's fields: %v", resp)
	}
	want := []struct{ final, visited, witness string }{
		{"high", "idle,high", "state idle on accept Go -> 2->high"},
		{"low", "idle,low", "state idle on accept Go -> 1->low"},
	}
	if len(resp.Outcomes) != len(want) {
		t.Fatalf("outcomes %v, want %d", resp.Outcomes, len(want))
	}
	for i, o := range resp.Outcomes {
		if o.FinalState != want[i].final || strings.Join(o.StatesVisited, ",") != want[i].visited || o.Linearizations != 1 {
			t.Errorf("outcome %d = %v, want final %s visiting %s once", i, o, want[i].final, want[i].visited)
		}
		if strings.Join(o.Witness, "; ") != want[i].witness {
			t.Errorf("outcome %d witness %q, want %q", i, strings.Join(o.Witness, "; "), want[i].witness)
		}
		if o.Outputs["level"].GetIntValue() != 8 {
			t.Errorf("outcome %d outputs %v, want level 8", i, o.Outputs)
		}
	}
}

// An analysis case's outcomes carry its outputs and verdicts.
func TestExploreAnalysisOverTheWire(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, exploreModel, "explore-analysis")

	resp := runAnalysis(t, srv, &pb.RunAnalysisRequest{ModelHash: hash, SymbolId: "Race::raced", Schedule: "explore"})
	if resp.Error != "" {
		t.Fatalf("RunAnalysis under explore reported %q", resp.Error)
	}
	if x := resp.Exploration; !x.Complete || x.Runs != 2 {
		t.Fatalf("exploration status %v, want complete after 2 runs", x)
	}
	if len(resp.Outputs) != 0 || len(resp.Verdicts) != 0 || resp.Instances != nil {
		t.Errorf("an explored run answered a single run's fields: %v", resp)
	}
	if got := outcomeInts(t, resp.Outcomes, "r"); len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("outcomes r = %v, want [1 2]", got)
	}
}

// A run that fails during exploration is an outcome of its own, not a failure
// of the exploration.
func TestExploreReportsAFailingRunAsAnOutcome(t *testing.T) {
	ctx := context.Background()
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, `
package Fail {
  private import ScalarValues::*;
  action divide {
    attribute d : Integer = 1;
    attribute q : Integer = 0;
    first start;
    fork split;
    action zero { assign d := 0; }
    action one { assign d := 1; }
    join sync;
    action use { assign q := 10 / d; }
    done;
    succession first start then split;
    succession first split then zero;
    succession first split then one;
    succession first zero then sync;
    succession first one then sync;
    succession first sync then use;
    succession first use then done;
  }
}
`, "explore-error")

	resp, err := srv.ExecuteAction(ctx, &pb.ExecuteActionRequest{ModelHash: hash, ActionSymbolId: "Fail::divide", Schedule: "explore"})
	if err != nil || resp.Error != "" {
		t.Fatalf("ExecuteAction under explore: %v %q", err, resp.GetError())
	}
	if !resp.Exploration.Complete || resp.Exploration.Runs != 2 || len(resp.Outcomes) != 2 {
		t.Fatalf("exploration %v with outcomes %v, want 2 outcomes over 2 runs", resp.Exploration, resp.Outcomes)
	}
	var failed, succeeded int
	for _, o := range resp.Outcomes {
		if o.Error != "" {
			failed++
			if !strings.Contains(o.Error, "action execution failed") || len(o.Outputs) != 0 {
				t.Errorf("error outcome %v, want the run's failure and no outputs", o)
			}
		} else {
			succeeded++
			if o.Outputs["q"].GetRealValue() != 10 {
				t.Errorf("outcome %v, want q = 10", o)
			}
		}
	}
	if failed != 1 || succeeded != 1 {
		t.Errorf("%d failed and %d succeeded outcomes, want one each: %v", failed, succeeded, resp.Outcomes)
	}
}

// Exploring needs the schedule_explore capability on every RPC carrying the
// field; the other policies need only schedule.
func TestExploreIsUnimplementedWithoutItsCapability(t *testing.T) {
	ctx := context.Background()
	srv := mustNewServiceWithout(t, CapabilityScheduleExplore)
	hash := mustVerifyModel(t, srv, scheduleModel, "explore-capability")

	calls := map[string]func(schedule string) error{
		"ExecuteAction": func(schedule string) error {
			_, err := srv.ExecuteAction(ctx, &pb.ExecuteActionRequest{ModelHash: hash, ActionSymbolId: "Sched::tally", Schedule: schedule})
			return err
		},
		"ExecuteState": func(schedule string) error {
			_, err := srv.ExecuteState(ctx, &pb.ExecuteStateRequest{ModelHash: hash, StateMachineSymbolId: "Sched::Dispatcher", Schedule: schedule})
			return err
		},
		"RunAnalysis": func(schedule string) error {
			_, err := srv.RunAnalysis(ctx, &pb.RunAnalysisRequest{ModelHash: hash, SymbolId: "Sched::forked", Schedule: schedule})
			return err
		},
	}
	for name, call := range calls {
		for _, schedule := range []string{"explore", "explore:runs=3"} {
			err := call(schedule)
			if connect.CodeOf(err) != connect.CodeUnimplemented || !strings.Contains(err.Error(), CapabilityScheduleExplore) {
				t.Errorf("%s under %q without %s: %v, want UNIMPLEMENTED naming it", name, schedule, CapabilityScheduleExplore, err)
			}
		}
		if err := call("seed:2"); err != nil {
			t.Errorf("%s under seed:2 without %s: %v", name, CapabilityScheduleExplore, err)
		}
	}
	info, err := srv.GetServerInfo(ctx, &pb.ServerInfoRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range info.Capabilities {
		if c == CapabilityScheduleExplore {
			t.Errorf("capabilities %v advertise %s, which the service withholds", info.Capabilities, c)
		}
	}
}

// A caller that has gone away fails the call with its own error, whether the
// behavior was to run once or be explored; neither is a failed run or an unmet
// precondition.
func TestExecuteActionCanceledCallerFailsTheCall(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, exploreModel, "explore-canceled")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, schedule := range []string{"", "explore"} {
		resp, err := srv.ExecuteAction(ctx, &pb.ExecuteActionRequest{ModelHash: hash, ActionSymbolId: "Race::race", Schedule: schedule})
		if !errors.Is(err, context.Canceled) {
			t.Errorf("schedule %q: err = %v; want context.Canceled", schedule, err)
		}
		if resp != nil {
			t.Errorf("schedule %q: a canceled call answered %v; want no response", schedule, resp)
		}
	}
}
