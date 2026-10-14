package grpc

import (
	"context"
	"math"
	"strings"
	"testing"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// tankModel performs an action on a part whose constraint one schedule of the
// action breaks, so a property of the performer is what the check engine judges.
const tankModel = `package Plant {
    private import ScalarValues::*;
    part def Tank {
        attribute level : Integer = 0;
        attribute capacity : Integer = 5;
        constraint low { level < 2 }
        constraint capped { level <= capacity }
        action fill {
            first start;
            fork split;
            action a { assign level := 1; }
            action b { assign level := 2; }
            join sync;
            done;
            succession first start then split;
            succession first split then a;
            succession first split then b;
            succession first a then sync;
            succession first b then sync;
            succession first sync then done;
        }
    }
    part tank : Tank;
}
`

func TestCheckBehaviorWitnessesAViolationAndADivergence(t *testing.T) {
	ctx := context.Background()
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, tankModel, "check-tank")

	resp, err := srv.CheckBehavior(ctx, &pb.CheckBehaviorRequest{
		ModelHash: hash, ActionSymbolId: "Plant::Tank::fill", PerformerSymbolId: "Plant::tank",
		CheckProperty: []string{"Plant::Tank::low"}, CheckDiverge: []string{"this.level"}, CheckWitness: true,
	})
	if err != nil || resp.GetError() != "" {
		t.Fatalf("CheckBehavior: %v %q", err, resp.GetError())
	}
	if resp.Verdict != "fails" {
		t.Fatalf("verdict %q, want fails; lines %v", resp.Verdict, resp.Lines)
	}
	if len(resp.Lines) == 0 || !strings.HasPrefix(resp.Lines[0], "✗ Action Plant::Tank::fill: violation") {
		t.Errorf("lines %v, want the command's violation report first", resp.Lines)
	}
	if resp.Search == nil || resp.Search.States != 11 || resp.Search.Moves != 10 || resp.Search.MaxDepth != 6 || len(resp.Search.BoundsHit) != 0 {
		t.Errorf("search %v, want 11 states, 10 moves, depth 6, exhaustive", resp.Search)
	}
	if len(resp.Violations) != 1 {
		t.Fatalf("violations %v, want one", resp.Violations)
	}
	v := resp.Violations[0]
	if v.Kind != "property" || v.Property != "Plant::Tank::low" || v.Depth != 3 || v.Error != "" {
		t.Errorf("violation %v, want Plant::Tank::low false after 3 moves", v)
	}
	if v.Witness == nil || len(v.Witness.Choices) == 0 || v.Witness.Schedule == "" {
		t.Errorf("violation witness %v, want its choices and replay schedule", v.Witness)
	}
	if v.Witness.GetFileName() == "" || !strings.Contains(v.Witness.GetFileContent(), "\n") {
		t.Errorf("violation witness file %q %q, want the written witness carried in the answer", v.Witness.GetFileName(), v.Witness.GetFileContent())
	}
	if len(resp.Divergences) != 1 || resp.Divergences[0].Feature != "this.level" || len(resp.Divergences[0].Values) != 2 {
		t.Fatalf("divergences %v, want this.level ending as two values", resp.Divergences)
	}
	for _, value := range resp.Divergences[0].Values {
		if value.Witness == nil || value.Witness.GetFileName() == "" {
			t.Errorf("divergent value %v carries no witness file", value)
		}
	}
	if resp.Engine != "check" || resp.Strength == "" {
		t.Errorf("standing %q/%q, want the check engine's", resp.Engine, resp.Strength)
	}

	held, err := srv.CheckBehavior(ctx, &pb.CheckBehaviorRequest{
		ModelHash: hash, ActionSymbolId: "Plant::Tank::fill", PerformerSymbolId: "Plant::tank",
		CheckProperty: []string{"Plant::Tank::capped"}, CheckDiverge: []string{"this.capacity"},
	})
	if err != nil || held.GetError() != "" {
		t.Fatalf("CheckBehavior held: %v %q", err, held.GetError())
	}
	if held.Verdict != "holds" || len(held.Violations) != 0 || len(held.Divergences) != 0 {
		t.Errorf("held verdict %q with %v / %v, want holds and nothing witnessed; lines %v", held.Verdict, held.Violations, held.Divergences, held.Lines)
	}
	if len(held.Finals) == 0 {
		t.Errorf("held check reports no final outcomes; lines %v", held.Lines)
	}
}

func TestCheckBehaviorBoundsTheSearch(t *testing.T) {
	ctx := context.Background()
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, tankModel, "check-tank-bounded")
	resp, err := srv.CheckBehavior(ctx, &pb.CheckBehaviorRequest{
		ModelHash: hash, ActionSymbolId: "Plant::Tank::fill", PerformerSymbolId: "Plant::tank",
		CheckProperty: []string{"Plant::Tank::low"}, CheckDepth: 2,
	})
	if err != nil || resp.GetError() != "" {
		t.Fatalf("CheckBehavior: %v %q", err, resp.GetError())
	}
	if resp.Verdict != "undecided" || resp.Search == nil || len(resp.Search.BoundsHit) == 0 {
		t.Errorf("verdict %q search %v, want undecided with the depth bound hit", resp.Verdict, resp.Search)
	}
}

func TestGRPCRobustnessCheckBehavior(t *testing.T) {
	ctx := context.Background()
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, tankModel, "check-tank-robust")
	invalid := []struct {
		name string
		req  *pb.CheckBehaviorRequest
	}{
		{"no behavior", &pb.CheckBehaviorRequest{ModelHash: hash}},
		{"two behaviors", &pb.CheckBehaviorRequest{ModelHash: hash, ActionSymbolId: "Plant::Tank::fill", StateMachineSymbolId: "Plant::Tank::fill"}},
		{"negative depth", &pb.CheckBehaviorRequest{ModelHash: hash, ActionSymbolId: "Plant::Tank::fill", CheckDepth: -1}},
		{"negative states", &pb.CheckBehaviorRequest{ModelHash: hash, ActionSymbolId: "Plant::Tank::fill", CheckStates: -1}},
		{"NaN timeout", &pb.CheckBehaviorRequest{ModelHash: hash, ActionSymbolId: "Plant::Tank::fill", CheckTimeout: math.NaN()}},
		{"negative timeout", &pb.CheckBehaviorRequest{ModelHash: hash, ActionSymbolId: "Plant::Tank::fill", CheckTimeout: -1}},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			_, err := srv.CheckBehavior(ctx, tc.req)
			if connect.CodeOf(err) != connect.CodeInvalidArgument {
				t.Fatalf("err = %v, want INVALID_ARGUMENT", err)
			}
		})
	}
	t.Run("unknown model", func(t *testing.T) {
		_, err := srv.CheckBehavior(ctx, &pb.CheckBehaviorRequest{ModelHash: "nope", ActionSymbolId: "Plant::Tank::fill"})
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("err = %v, want NOT_FOUND", err)
		}
	})
	inBand := []struct {
		name string
		req  *pb.CheckBehaviorRequest
		want string
	}{
		{"unknown behavior", &pb.CheckBehaviorRequest{ModelHash: hash, ActionSymbolId: "Plant::Tank::drain"}, "behavior not found"},
		{"unknown property", &pb.CheckBehaviorRequest{ModelHash: hash, ActionSymbolId: "Plant::Tank::fill", CheckProperty: []string{"Plant::Tank::nope"}}, "not found"},
		{"property is not a condition", &pb.CheckBehaviorRequest{ModelHash: hash, ActionSymbolId: "Plant::Tank::fill", CheckProperty: []string{"Plant::Tank::level"}}, "not a constraint or requirement"},
	}
	for _, tc := range inBand {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := srv.CheckBehavior(ctx, tc.req)
			if err != nil {
				t.Fatalf("err = %v, want an in-band refusal", err)
			}
			if resp.Verdict != "undecided" || !strings.Contains(resp.Error, tc.want) {
				t.Fatalf("verdict %q error %q, want undecided with %q", resp.Verdict, resp.Error, tc.want)
			}
		})
	}
}
