package grpc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/tests/testutil/gobuild"
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
		{"check", "bounded", []string{"outcomes", "holds", "sensitive"}},
		{"explore", "proved", []string{"outcomes"}},
		{"run", "observed", []string{"evaluate"}},
		{"smt", "proved", []string{"holds", "sensitive"}},
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
		if got.Name != "solve" && got.Name != "smt" && (!got.Ready || got.Process != "" || got.Unavailable != "") {
			t.Errorf("in-process engine %s = %v, want ready with no process", got.Name, got)
		}
	}
	for _, external := range []*pb.EngineInfo{resp.Engines[3], resp.Engines[4]} {
		if external.Process == "" || external.Ready == (external.Unavailable != "") {
			t.Errorf("%s = %v, want a process and ready or a reason", external.Name, external)
		}
		if external.Ready && external.ProcessFound == "" {
			t.Errorf("%s is ready but names no found process: %v", external.Name, external)
		}
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
		"ValidateInstance": func() error {
			_, err := srv.ValidateInstance(ctx, &pb.ValidateInstanceRequest{ModelHash: hash, SymbolId: "Demo::sedan", Engine: engine})
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

var (
	standinOnce sync.Once
	standinPath string
	standinErr  error
)

// engineStandin builds the analysis package's stand-in engine once per test binary.
func engineStandin(t *testing.T) string {
	t.Helper()
	standinOnce.Do(func() {
		dir, err := os.MkdirTemp("", "enginestandin")
		if err != nil {
			standinErr = err
			return
		}
		standinPath = filepath.Join(dir, "enginestandin")
		build := exec.Command("go", gobuild.Args(standinPath)...)
		build.Dir = filepath.Join("..", "core", "analysis", "testdata", "enginestandin")
		if out, err := build.CombinedOutput(); err != nil {
			standinErr = fmt.Errorf("go build: %v\n%s", err, out)
		}
	})
	if standinErr != nil {
		t.Fatalf("building the stand-in engine: %v", standinErr)
	}
	return standinPath
}

// standinManifest points OPENSYSML_ENGINES at a manifest registering the stand-in as `standin`.
func standinManifest(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	entry := `{"kind":"engine","name":"standin","version":"1.0.0","command":["` + engineStandin(t) + `"],` +
		`"protocol":1,"answers":["holds"],"model":["sources"],"witness":"schedule","authority":"bounded"}`
	file := filepath.Join(dir, "standin.json")
	if err := os.WriteFile(file, []byte(entry), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(analysis.ToolsEnv, "")
	t.Setenv(analysis.EnginesEnv, dir)
	return file
}

// standinInfo is the stand-in's entry in ListEngines.
func standinInfo(t *testing.T, srv *Service) *pb.EngineInfo {
	t.Helper()
	resp, err := srv.ListEngines(context.Background(), &pb.ListEnginesRequest{})
	if err != nil {
		t.Fatalf("ListEngines: %v", err)
	}
	for _, e := range resp.Engines {
		if e.Name == "standin" {
			return e
		}
	}
	t.Fatalf("standin is not listed in %v", resp.Engines)
	return nil
}

// A service started without -serve-external-engines lists a manifest engine with its
// origin and served false, refuses a request naming it with FAILED_PRECONDITION, keeps auto
// away from it and does not advertise engines_external.
func TestManifestEnginesAreListedButNotServedByDefault(t *testing.T) {
	ctx := context.Background()
	file := standinManifest(t)
	srv := mustNewService(t, 10)
	t.Cleanup(srv.Close)

	info := standinInfo(t, srv)
	if info.Served || info.Ready || info.Kind != "engine" || info.Protocol != "stdio/1" || info.Source != file ||
		info.Command != engineStandin(t) || info.Version != "1.0.0" || info.Authority != "bounded" ||
		strings.Join(info.Answers, ",") != "holds" || info.Unavailable != "" {
		t.Errorf("standin = %v, want its origin, not served and not ready with no fault", info)
	}
	resp, err := srv.ListEngines(ctx, &pb.ListEnginesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range resp.Engines {
		if e.Name != "standin" && (e.Kind != "built-in" || e.Protocol != "-" || e.Source != "" || !e.Served) {
			t.Errorf("built-in %s = %v, want kind built-in, no protocol or source, served", e.Name, e)
		}
	}

	server, err := srv.GetServerInfo(ctx, &pb.ServerInfoRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(server.Capabilities, CapabilityEnginesExternal) {
		t.Errorf("a service serving no manifest engine advertises %s", CapabilityEnginesExternal)
	}

	hash := mustVerifyModel(t, srv, verifyModelSource, "engines-withheld-standin")
	for name, call := range engineCalls(ctx, srv, hash, "standin") {
		err := call()
		if connect.CodeOf(err) != connect.CodeFailedPrecondition || !strings.Contains(err.Error(), "engine 'standin' is not served by this service") {
			t.Errorf("%s with engine standin: %v, want FAILED_PRECONDITION naming the withholding", name, err)
		}
	}
	verdict, err := srv.VerifyConstraint(ctx, &pb.VerifyConstraintRequest{ModelHash: hash, SymbolId: "Demo::Vehicle::massPositive", Engine: "all"})
	if err != nil || verdict.Verdict == nil || verdict.Verdict.Engine != "run" {
		t.Errorf("engine all = %v %v, want the built-ins alone consulted", err, verdict)
	}
}

// Started with -serve-external-engines naming it, the service lists the engine served,
// advertises engines_external and puts a request naming it to the engine; a name that is not a
// manifest engine fails construction.
func TestServeExternalEnginesRunsTheNamedManifestEngines(t *testing.T) {
	ctx := context.Background()
	standinManifest(t)
	srv, err := NewService(10, "test", ServeExternalEngines("standin"))
	if err != nil {
		t.Fatalf("NewService serving standin: %v", err)
	}
	t.Cleanup(srv.Close)

	if info := standinInfo(t, srv); !info.Served || !info.Ready || info.Unavailable != "" {
		t.Errorf("standin = %v, want served and ready", info)
	}
	server, err := srv.GetServerInfo(ctx, &pb.ServerInfoRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(server.Capabilities, CapabilityEnginesExternal) {
		t.Errorf("a service serving standin does not advertise %s", CapabilityEnginesExternal)
	}

	hash := mustVerifyModel(t, srv, verifyModelSource, "engines-served-standin")
	resp, err := srv.VerifyConstraint(ctx, &pb.VerifyConstraintRequest{ModelHash: hash, SymbolId: "Demo::Vehicle::massPositive", Engine: "standin"})
	if err != nil || resp.Verdict == nil {
		t.Fatalf("VerifyConstraint engine standin: %v %v", err, resp)
	}
	if v := resp.Verdict; v.Holds || v.Strength != "not covered" || !strings.Contains(v.Error, "standin does not answer evaluate questions") {
		t.Errorf("verdict = %v, want the stand-in's own refusal of an evaluate question", v)
	}

	if _, err := NewService(10, "test", ServeExternalEngines("run")); !errors.Is(err, analysis.ErrNotExternal) {
		t.Errorf("NewService serving run: %v, want the typed refusal of a built-in", err)
	}
	all, err := NewService(10, "test", ServeExternalEngines(analysis.ServeAll))
	if err != nil {
		t.Fatalf("NewService serving all: %v", err)
	}
	t.Cleanup(all.Close)
	if info := standinInfo(t, all); !info.Served {
		t.Errorf("standin under all = %v, want served", info)
	}
}
