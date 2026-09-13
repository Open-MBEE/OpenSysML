package grpc

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"google.golang.org/protobuf/proto"
)

// ListEngines names every registered engine in name order, with its authority,
// the questions it answers and whether it can run here.
func TestListEnginesNamesEveryEngine(t *testing.T) {
	srv := mustNewService(t, 10)
	t.Cleanup(srv.Close)
	resp, err := srv.ListEngines(context.Background(), &pb.ListEnginesRequest{})
	if err != nil {
		t.Fatalf("ListEngines: %v", err)
	}
	want := []struct {
		name, authority string
		answers         []string
	}{
		{"check", "bounded", []string{"outcomes", "holds"}},
		{"explore", "proved", []string{"outcomes"}},
		{"run", "observed", []string{"evaluate"}},
		{"solve", "proved", []string{"satisfiable"}},
		{"sweep", "observed", []string{"sweep"}},
	}
	if len(resp.Engines) != len(want) {
		t.Fatalf("engines = %v, want %d", resp.Engines, len(want))
	}
	for i, w := range want {
		got := resp.Engines[i]
		if got.Name != w.name || got.Authority != w.authority || strings.Join(got.Answers, ",") != strings.Join(w.answers, ",") {
			t.Errorf("engine %d = %v, want %s %s %v", i, got, w.name, w.authority, w.answers)
		}
		if got.Name != "solve" && (!got.Ready || got.Process != "" || got.Unavailable != "") {
			t.Errorf("in-process engine %s = %v, want ready with no process", got.Name, got)
		}
	}
	solve := resp.Engines[3]
	if solve.Process == "" || solve.Ready == (solve.Unavailable != "") {
		t.Errorf("solve = %v, want a process and ready or a reason", solve)
	}
	if solve.Ready && solve.ProcessFound == "" {
		t.Errorf("solve is ready but names no found process: %v", solve)
	}
}

// ListEngines and the engine field need the engines capability; an unset field
// is auto and needs nothing, though the response then withholds the standing.
func TestEngineFieldNeedsTheEnginesCapability(t *testing.T) {
	ctx := context.Background()
	srv := mustNewServiceWithout(t, CapabilityEngines)
	hash := mustVerifyModel(t, srv, verifyModelSource, "engines-withheld")

	_, err := srv.ListEngines(ctx, &pb.ListEnginesRequest{})
	if connect.CodeOf(err) != connect.CodeUnimplemented || !strings.Contains(err.Error(), CapabilityEngines) {
		t.Errorf("ListEngines without engines: %v, want UNIMPLEMENTED naming %s", err, CapabilityEngines)
	}
	for name, call := range engineCalls(ctx, srv, hash, "run") {
		if err := call(); connect.CodeOf(err) != connect.CodeUnimplemented || !strings.Contains(err.Error(), CapabilityEngines) {
			t.Errorf("%s with engine run without engines: %v, want UNIMPLEMENTED naming %s", name, err, CapabilityEngines)
		}
	}
	resp, err := srv.VerifyConstraint(ctx, &pb.VerifyConstraintRequest{ModelHash: hash, SymbolId: "Demo::Vehicle::massPositive"})
	if err != nil || resp.Error != "" || resp.Verdict == nil || !resp.Verdict.Holds {
		t.Fatalf("VerifyConstraint with engine unset: %v %v", err, resp)
	}
	if v := resp.Verdict; v.Engine != "" || v.Strength != "" || len(v.Bounds) != 0 {
		t.Errorf("verdict carries a standing the service withholds: %v", v)
	}
}

// A spelling that names no engine is INVALID_ARGUMENT on every RPC carrying the
// field, before the model is looked up.
func TestAnUnknownEngineIsInvalidArgument(t *testing.T) {
	ctx := context.Background()
	srv := mustNewService(t, 10)
	t.Cleanup(srv.Close)
	hash := mustVerifyModel(t, srv, verifyModelSource, "engines-unknown")
	for _, h := range []string{hash, "missing"} {
		for name, call := range engineCalls(ctx, srv, h, "nope") {
			err := call()
			if connect.CodeOf(err) != connect.CodeInvalidArgument || !strings.Contains(err.Error(), `"nope"`) {
				t.Errorf("%s with engine nope on %q: %v, want INVALID_ARGUMENT naming the spelling", name, h, err)
			}
		}
	}
}

// engineCalls is every RPC carrying the engine field, made with the spelling.
func engineCalls(ctx context.Context, srv *Service, hash, engine string) map[string]func() error {
	return map[string]func() error{
		"VerifyConstraint": func() error {
			_, err := srv.VerifyConstraint(ctx, &pb.VerifyConstraintRequest{ModelHash: hash, SymbolId: "Demo::Vehicle::massPositive", Engine: engine})
			return err
		},
		"VerifyRequirement": func() error {
			_, err := srv.VerifyRequirement(ctx, &pb.VerifyRequirementRequest{ModelHash: hash, SymbolId: "Demo::Vehicle::lightEnough", Engine: engine})
			return err
		},
		"VerifySatisfaction": func() error {
			_, err := srv.VerifySatisfaction(ctx, &pb.VerifySatisfactionRequest{ModelHash: hash, Engine: engine})
			return err
		},
		"EvaluateCalc": func() error {
			_, err := srv.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{ModelHash: hash, SymbolId: "Demo::add", Arguments: []*pb.Value{intProto(1), intProto(2)}, Engine: engine})
			return err
		},
		"RunAnalysis": func() error {
			_, err := srv.RunAnalysis(ctx, &pb.RunAnalysisRequest{ModelHash: hash, SymbolId: "Demo::analysis", Engine: engine})
			return err
		},
		"RunSweep": func() error {
			_, err := srv.RunSweep(ctx, &pb.RunSweepRequest{ModelHash: hash, SymbolId: "Demo::add", Ranges: []*pb.SweepRange{intRange("x", 1, 2)}, NamedArguments: map[string]*pb.Value{"y": intProto(1)}, Engine: engine})
			return err
		},
	}
}

// unreached reports whether bounds name the run engine's limits, none reached.
func unreached(bounds []*pb.Bound) bool {
	if len(bounds) == 0 {
		return false
	}
	for _, b := range bounds {
		if b.Reached || b.Name == "" || b.Limit <= 0 {
			return false
		}
	}
	return true
}

// Every verification response names the engine that answered it, the strength
// it earned and the bounds it ran under; unset, auto, run and all answer alike
// where run is the one covering engine. A violation one run saw is witnessed.
func TestVerdictsCarryEngineStrengthAndBounds(t *testing.T) {
	ctx := context.Background()
	srv := mustNewService(t, 10)
	t.Cleanup(srv.Close)
	hash := mustVerifyModel(t, srv, verifyModelSource, "engines-standing")

	for _, engine := range []string{"", "auto", "run", "all"} {
		resp, err := srv.VerifyConstraint(ctx, &pb.VerifyConstraintRequest{ModelHash: hash, SymbolId: "Demo::Vehicle::massPositive", Engine: engine})
		if err != nil || resp.Error != "" || resp.Verdict == nil {
			t.Fatalf("VerifyConstraint engine %q: %v %v", engine, err, resp)
		}
		if v := resp.Verdict; !v.Holds || v.Engine != "run" || v.Strength != "observed" || !unreached(v.Bounds) {
			t.Errorf("VerifyConstraint engine %q verdict = %v, want holds by run, observed, within its bounds", engine, v)
		}

		req, err := srv.VerifyRequirement(ctx, &pb.VerifyRequirementRequest{ModelHash: hash, SymbolId: "Demo::Vehicle::tiny", Engine: engine})
		if err != nil || req.Error != "" || req.Verdict == nil {
			t.Fatalf("VerifyRequirement engine %q: %v %v", engine, err, req)
		}
		if v := req.Verdict; v.Holds || v.Engine != "run" || v.Strength != "witnessed" {
			t.Errorf("VerifyRequirement engine %q verdict = %v, want violated by run, witnessed", engine, v)
		}

		sat, err := srv.VerifySatisfaction(ctx, &pb.VerifySatisfactionRequest{ModelHash: hash, Engine: engine})
		if err != nil || sat.Error != "" || len(sat.Verdicts) != 2 {
			t.Fatalf("VerifySatisfaction engine %q: %v %v", engine, err, sat)
		}
		for _, v := range sat.Verdicts {
			want := "observed"
			if !v.Holds {
				want = "witnessed"
			}
			if v.Engine != "run" || v.Strength != want {
				t.Errorf("VerifySatisfaction engine %q verdict = %v, want by run, %s", engine, v, want)
			}
		}

		calc, err := srv.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{ModelHash: hash, SymbolId: "Demo::add", Arguments: []*pb.Value{intProto(1), intProto(2)}, Engine: engine})
		if err != nil || calc.Error != "" {
			t.Fatalf("EvaluateCalc engine %q: %v %v", engine, err, calc)
		}
		if calc.Engine != "run" || calc.Strength != "observed" || !unreached(calc.Bounds) {
			t.Errorf("EvaluateCalc engine %q = %v, want by run, observed, within its bounds", engine, calc)
		}
	}
	for _, engine := range []string{"", "auto", "sweep", "all"} {
		sweep := runSweep(t, srv, &pb.RunSweepRequest{ModelHash: hash, SymbolId: "Demo::add", Ranges: []*pb.SweepRange{intRange("x", 1, 2)}, NamedArguments: map[string]*pb.Value{"y": intProto(1)}, Engine: engine})
		if sweep.Error != "" {
			t.Fatalf("RunSweep engine %q reported %q", engine, sweep.Error)
		}
		if sweep.Engine != "sweep" || sweep.Strength != "observed" || len(sweep.Rows) != 2 {
			t.Errorf("RunSweep engine %q = %s %s over %d rows, want by sweep, observed, 2 rows", engine, sweep.Engine, sweep.Strength, len(sweep.Rows))
		}
	}
}

// A named engine that does not answer the question is the answer: the verdict
// is its refusal, not covered, and no other engine is tried.
func TestNamedEngineRefusalIsFinalOverTheWire(t *testing.T) {
	ctx := context.Background()
	srv := mustNewService(t, 10)
	t.Cleanup(srv.Close)
	hash := mustVerifyModel(t, srv, verifyModelSource, "engines-refusal")

	resp, err := srv.VerifyConstraint(ctx, &pb.VerifyConstraintRequest{ModelHash: hash, SymbolId: "Demo::Vehicle::massPositive", Engine: "sweep"})
	if err != nil || resp.Verdict == nil {
		t.Fatalf("VerifyConstraint engine sweep: %v %v", err, resp)
	}
	v := resp.Verdict
	if v.Holds || !strings.Contains(v.Error, "sweep does not answer evaluate questions") || v.Strength != "not covered" {
		t.Errorf("verdict = %v, want the refusal, not covered", v)
	}

	sweep := runSweep(t, srv, &pb.RunSweepRequest{ModelHash: hash, SymbolId: "Demo::add", Ranges: []*pb.SweepRange{intRange("x", 1, 2)}, NamedArguments: map[string]*pb.Value{"y": intProto(1)}, Engine: "run"})
	if !strings.Contains(sweep.Error, "run does not answer sweep questions") || len(sweep.Rows) != 0 || sweep.Strength != "not covered" {
		t.Errorf("RunSweep engine run = %v, want the refusal, not covered", sweep)
	}
}

// engine=explore asks what schedule=explore asks: the same outcomes, the
// standing of the explore engine; without the schedule-explore capability it is
// refused as that schedule is.
func TestEngineExploreIsScheduleExplore(t *testing.T) {
	srv := mustNewService(t, 10)
	t.Cleanup(srv.Close)
	hash := mustVerifyModel(t, srv, exploreModel, "engines-explore")

	bySchedule := runAnalysis(t, srv, &pb.RunAnalysisRequest{ModelHash: hash, SymbolId: "Race::raced", Schedule: "explore"})
	byEngine := runAnalysis(t, srv, &pb.RunAnalysisRequest{ModelHash: hash, SymbolId: "Race::raced", Engine: "explore"})
	if bySchedule.Error != "" || byEngine.Error != "" {
		t.Fatalf("explore reported %q / %q", bySchedule.Error, byEngine.Error)
	}
	if !proto.Equal(bySchedule, byEngine) {
		t.Errorf("engine explore answered\n%v\nschedule explore answered\n%v", byEngine, bySchedule)
	}
	if byEngine.Engine != "explore" || byEngine.Strength != "proved" || len(byEngine.Bounds) == 0 {
		t.Errorf("engine explore standing = %s %s %v, want explore, proved, with its bounds", byEngine.Engine, byEngine.Strength, byEngine.Bounds)
	}
	for _, b := range byEngine.Bounds {
		if b.Reached {
			t.Errorf("a complete exploration reached bound %v", b)
		}
	}

	without := mustNewServiceWithout(t, CapabilityScheduleExplore)
	hash = mustVerifyModel(t, without, exploreModel, "engines-explore-withheld")
	_, err := without.RunAnalysis(context.Background(), &pb.RunAnalysisRequest{ModelHash: hash, SymbolId: "Race::raced", Engine: "explore"})
	if connect.CodeOf(err) != connect.CodeUnimplemented || !strings.Contains(err.Error(), CapabilityScheduleExplore) {
		t.Errorf("engine explore without %s: %v, want UNIMPLEMENTED naming it", CapabilityScheduleExplore, err)
	}
}
