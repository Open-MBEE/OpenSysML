package grpc

import (
	"context"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// performerAdvanceModel has a part performing an action with a declaration-bound
// `in ref` parameter and a wait, so a bounded run can stop it short; a vehicle
// exhibiting a timed machine; and a twin performing one definition twice over.
const performerAdvanceModel = `
package P {
  private import ScalarValues::*;
  action def Run {
    in ref ctx : Robot[1];
    out n : Integer[1];
  }
  part def Robot {
    attribute steps : Integer = 0;
    perform action run : Run {
      in ref :>> ctx = this;
      first start;
      then action wait accept after 5 [SI::s];
      then action tick assign steps := steps + 1;
      then action yield assign n := steps;
      then done;
    }
  }
  action def Ping;
  part def Twin { perform action morning : Ping; perform action evening : Ping; }
  part def Vehicle {
    attribute moved : Integer = 0;
    exhibit state modes {
      entry; then idle;
      state idle;
      transition idle then moving accept after 3 [SI::s] do assign moved := moved + 1;
      state moving;
    }
  }
  part robot : Robot;
  part twin : Twin;
  part vehicle : Vehicle;
}
`

func executeActionOn(t *testing.T, srv *Service, hash, action, performer string, advance *float64, inputs map[string]*pb.Value) *pb.ExecuteActionResponse {
	t.Helper()
	resp, err := srv.ExecuteAction(context.Background(), &pb.ExecuteActionRequest{
		ModelHash: hash, ActionSymbolId: action, PerformerSymbolId: performer, Advance: advance, Inputs: inputs,
	})
	if err != nil {
		t.Fatalf("ExecuteAction %s on %s (advance %v): %v", action, performer, advance, err)
	}
	return resp
}

// An action run on the object performing it runs that object's own performance
// whether or not the run is bounded by advance: the duration decides how far it
// proceeds, not which performance runs, so the object is written once and the
// outputs and performer attributes are the same in both modes.
func TestExecuteActionAdvanceRunsThePerformersPerformance(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParse(t, srv, performerAdvanceModel)
	for _, action := range []string{"P::Robot::run", "P::Run"} {
		whole := executeActionOn(t, srv, hash, action, "P::robot", nil, nil)
		bounded := executeActionOn(t, srv, hash, action, "P::robot", f64(10), nil)
		for label, resp := range map[string]*pb.ExecuteActionResponse{"to completion": whole, "advance 10": bounded} {
			if resp.Error != "" {
				t.Fatalf("%s %s: %s", action, label, resp.Error)
			}
			if got := resp.PerformerAttributes["this.steps"].GetIntValue(); got != 1 {
				t.Errorf("%s %s: this.steps = %d, want 1: the robot's own performance ran, once", action, label, got)
			}
			if got := resp.Outputs["run.n"].GetIntValue(); got != 1 {
				t.Errorf("%s %s: outputs = %v, want the performed usage's run.n = 1", action, label, resp.Outputs)
			}
		}
		if whole.FinalTime != 5 || bounded.FinalTime != 10 {
			t.Errorf("%s: final_time %g to completion and %g under advance 10, want 5 and 10", action, whole.FinalTime, bounded.FinalTime)
		}
	}

	short := executeActionOn(t, srv, hash, "P::Robot::run", "P::robot", f64(2), nil)
	if !strings.Contains(short.Error, "stopped at Waiting at simulation time 2.0 without completing") || !strings.Contains(short.Error, "t=5") {
		t.Errorf("advance 2 error = %q, want the robot's performance stopped short of its wait at t=5", short.Error)
	}
	if short.FinalTime != 2 {
		t.Errorf("advance 2 final_time = %g, want 2", short.FinalTime)
	}
}

// Inputs for an action the object performs are refused in both modes alike, as
// its declaration binds its arguments; so is an object performing the
// definition under two usages, which names no one performance.
func TestExecuteActionAdvanceRefusesPerformedInputsAndAmbiguityAlike(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParse(t, srv, performerAdvanceModel)
	conflicting := map[string]*pb.Value{"n": {Kind: &pb.Value_IntValue{IntValue: 3}}}
	for label, advance := range map[string]*float64{"to completion": nil, "advance 10": f64(10)} {
		resp := executeActionOn(t, srv, hash, "P::Robot::run", "P::robot", advance, conflicting)
		if want := "inputs for a performed action: the object performs run already, with the arguments its declaration binds"; !strings.Contains(resp.Error, want) {
			t.Errorf("%s with n=3: error = %q, want %q", label, resp.Error, want)
		}
		resp = executeActionOn(t, srv, hash, "P::Ping", "P::twin", advance, nil)
		if want := "ambiguous action: the object performs Ping as morning and evening"; !strings.Contains(resp.Error, want) {
			t.Errorf("%s on the twin: error = %q, want %q", label, resp.Error, want)
		}
	}
}

// An explored run bounded by advance runs the performer's performance too: every
// outcome carries the robot written once.
func TestExecuteActionExploreAdvanceRunsThePerformersPerformance(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParse(t, srv, performerAdvanceModel)
	resp, err := srv.ExecuteAction(context.Background(), &pb.ExecuteActionRequest{
		ModelHash: hash, ActionSymbolId: "P::Robot::run", PerformerSymbolId: "P::robot", Advance: f64(10), Schedule: "explore",
	})
	if err != nil || resp.Error != "" {
		t.Fatalf("ExecuteAction under explore: %v %q", err, resp.GetError())
	}
	if got := outcomeInts(t, resp.Outcomes, "this.steps"); len(got) != 1 || got[0] != 1 {
		t.Errorf("explored outcomes this.steps = %v, want [1]: one performance per run", got)
	}
}

// A machine run on the object exhibiting it runs that exhibited machine whether
// or not the run is bounded by advance: the transition due fires once.
func TestExecuteStateAdvanceRunsTheExhibitedMachine(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParse(t, srv, performerAdvanceModel)
	for label, advance := range map[string]*float64{"to quiescence": nil, "advance 3": f64(3), "advance 10": f64(10)} {
		resp, err := srv.ExecuteState(context.Background(), &pb.ExecuteStateRequest{
			ModelHash: hash, StateMachineSymbolId: "P::Vehicle::modes", PerformerSymbolId: "P::vehicle", Advance: advance,
		})
		if err != nil {
			t.Fatalf("ExecuteState %s: %v", label, err)
		}
		if resp.Error != "" {
			t.Fatalf("ExecuteState %s: %s", label, resp.Error)
		}
		if got := resp.FinalContext["this.moved"].GetIntValue(); got != 1 {
			t.Errorf("%s: this.moved = %d, want 1: the vehicle's own machine took the transition, once", label, got)
		}
		if got := strings.Join(resp.StatesVisited, " "); got != "idle moving" {
			t.Errorf("%s: states visited %q, want \"idle moving\"", label, got)
		}
	}
}

const heldActionModel = `
package H {
  private import ScalarValues::*;
  part def Pair {
    attribute marks : Integer = 0;
    perform action earlier { first start; then action w accept after 5 [SI::s]; then done; }
    perform action later { first start; then action mark assign marks := marks + 1; then done; }
    first earlier then later;
  }
  part pair : Pair;
}
`

// A performed action a succession order still holds back is refused with the
// order's violation in both modes, rather than performed anew beside the held one.
func TestExecuteActionOnAPerformerHoldingItBySuccessionIsRefusedAlike(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParse(t, srv, heldActionModel)
	for label, advance := range map[string]*float64{"to completion": nil, "advance 2": f64(2), "advance 10": f64(10)} {
		resp := executeActionOn(t, srv, hash, "H::Pair::later", "H::pair", advance, nil)
		if want := "succession-order-violated: succession first-then succession orders earlier of object #1 before later of object #1, but earlier of object #1 has already begun"; !strings.Contains(resp.Error, want) {
			t.Errorf("%s: error = %q, want %q", label, resp.Error, want)
		}
	}
	resp := executeActionOn(t, srv, hash, "H::Pair::earlier", "H::pair", f64(10), nil)
	if resp.Error != "" {
		t.Fatalf("earlier under advance 10: %s", resp.Error)
	}
	if got := resp.PerformerAttributes["this.marks"].GetIntValue(); got != 1 {
		t.Errorf("this.marks = %d after earlier ran its course, want 1: the clock released later once", got)
	}
}
