package grpc

import (
	"context"
	"errors"
	"math"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// TestGRPCRobustnessRunOptions gives the run options bad values: a negative or
// non-finite advance, an unknown draw policy, a negative clock step, a lint
// code naming no lint, and a tool dry run under explore each refuse the call as
// INVALID_ARGUMENT, before any model is looked up, and nothing panics.
func TestGRPCRobustnessRunOptions(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParse(t, srv, timedActionModel)
	nan := math.NaN()
	for name, call := range map[string]func() error{
		"negative advance": func() error {
			_, err := srv.ExecuteAction(context.Background(), &pb.ExecuteActionRequest{ModelHash: hash, ActionSymbolId: "Test::delayed", Advance: f64(-1)})
			return err
		},
		"nan advance on a state": func() error {
			_, err := srv.ExecuteState(context.Background(), &pb.ExecuteStateRequest{ModelHash: hash, StateMachineSymbolId: "Test::Timer", Advance: &nan})
			return err
		},
		"unknown draws": func() error {
			_, err := srv.ExecuteAction(context.Background(), &pb.ExecuteActionRequest{ModelHash: hash, ActionSymbolId: "Test::delayed", Draws: "median"})
			return err
		},
		"negative clock step": func() error {
			_, err := srv.ExecuteState(context.Background(), &pb.ExecuteStateRequest{ModelHash: hash, StateMachineSymbolId: "Test::Timer", ClockStep: -1})
			return err
		},
		"unknown lint": func() error {
			_, err := srv.GetDiagnostics(context.Background(), &pb.DiagnosticsRequest{ModelHash: hash, DisableLint: []string{"no-such-lint"}})
			return err
		},
		"unknown lint on a missing model": func() error {
			_, err := srv.GetDiagnostics(context.Background(), &pb.DiagnosticsRequest{ModelHash: "missing", EnableLint: []string{"no-such-lint"}})
			return err
		},
		"dry run under explore": func() error {
			_, err := srv.RunAnalysis(context.Background(), &pb.RunAnalysisRequest{ModelHash: hash, SymbolId: "Test::delayed", Schedule: "explore", ToolDryRun: true})
			return err
		},
	} {
		err := call()
		var cerr *connect.Error
		if !errors.As(err, &cerr) || cerr.Code() != connect.CodeInvalidArgument {
			t.Errorf("%s: err = %v, want INVALID_ARGUMENT", name, err)
		}
	}
}
