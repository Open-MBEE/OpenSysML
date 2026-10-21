package grpc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
)

const timedActionModel = `
package Test {
  private import ScalarValues::*;
  action delayed {
    attribute count : Integer = 0;
    first start;
    then action wait accept after 5 [SI::s];
    then action tick assign count := count + 1;
    then done;
  }
  state Timer {
    attribute fired : Integer = 0;
    entry; then armed;
    state armed;
    transition armed then done accept after 3 [SI::s] do assign fired := fired + 1;
  }
}
`

const drawingActionModel = `
package Test {
  private import ScalarValues::*;
  private import RandomFunctions::*;
  action draw {
    out d : Real = uniform(0.0, 1.0);
    out n : Integer = uniformInteger(1, 6);
  }
}
`

func f64(v float64) *float64 { return &v }
func u64(v uint64) *uint64   { return &v }

// -advance over the wire: the action is run for the duration given, so one
// whose wait ends later has not completed and says so, while one run long
// enough reports its outputs and the clock it ended at.
func TestExecuteActionAdvanceRunsForTheDuration(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParse(t, srv, timedActionModel)

	short, err := srv.ExecuteAction(context.Background(), &pb.ExecuteActionRequest{
		ModelHash: hash, ActionSymbolId: "Test::delayed", Advance: f64(2),
	})
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if !strings.Contains(short.Error, "without completing") || !strings.Contains(short.Error, "t=5") {
		t.Errorf("advance 2 error = %q, want the action stopped short of its wait at t=5", short.Error)
	}
	if short.FinalTime != 2 {
		t.Errorf("advance 2 final_time = %g, want 2", short.FinalTime)
	}

	long, err := srv.ExecuteAction(context.Background(), &pb.ExecuteActionRequest{
		ModelHash: hash, ActionSymbolId: "Test::delayed", Advance: f64(10),
	})
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if long.Error != "" {
		t.Fatalf("advance 10 error: %s", long.Error)
	}
	if got := long.Outputs["count"].GetIntValue(); got != 1 {
		t.Errorf("count = %d, want 1", got)
	}
	if long.FinalTime != 10 {
		t.Errorf("advance 10 final_time = %g, want 10: the clock moved the whole duration", long.FinalTime)
	}
}

// A state machine run for a duration takes the transitions due within it.
func TestExecuteStateAdvanceRunsForTheDuration(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParse(t, srv, timedActionModel)
	for _, tc := range []struct {
		advance float64
		fired   int64
	}{{1, 0}, {3, 1}} {
		resp, err := srv.ExecuteState(context.Background(), &pb.ExecuteStateRequest{
			ModelHash: hash, StateMachineSymbolId: "Test::Timer", Advance: f64(tc.advance),
		})
		if err != nil {
			t.Fatalf("ExecuteState: %v", err)
		}
		if resp.Error != "" {
			t.Fatalf("advance %g: %s", tc.advance, resp.Error)
		}
		if got := resp.FinalContext["fired"].GetIntValue(); got != tc.fired {
			t.Errorf("advance %g: fired = %d, want %d", tc.advance, got, tc.fired)
		}
		if resp.FinalTime != tc.advance {
			t.Errorf("advance %g: final_time = %g", tc.advance, resp.FinalTime)
		}
	}
}

// -seed and -draws over the wire: the same seed draws the same run, a fixed
// policy needs none and takes each call's bound.
func TestExecuteActionSeedAndDrawsResolveTheModelsDraws(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParse(t, srv, drawingActionModel)
	run := func(req *pb.ExecuteActionRequest) *pb.ExecuteActionResponse {
		t.Helper()
		req.ModelHash, req.ActionSymbolId = hash, "Test::draw"
		resp, err := srv.ExecuteAction(context.Background(), req)
		if err != nil {
			t.Fatalf("ExecuteAction: %v", err)
		}
		if resp.Error != "" {
			t.Fatalf("ExecuteAction %v: %s", req, resp.Error)
		}
		return resp
	}
	first, again := run(&pb.ExecuteActionRequest{Seed: u64(7)}), run(&pb.ExecuteActionRequest{Seed: u64(7)})
	if soleElement(t, "d", first.Outputs["d"]).GetRealValue() != soleElement(t, "d", again.Outputs["d"]).GetRealValue() || soleElement(t, "n", first.Outputs["n"]).GetIntValue() != soleElement(t, "n", again.Outputs["n"]).GetIntValue() {
		t.Errorf("seed 7 drew %v then %v", first.Outputs, again.Outputs)
	}
	other := run(&pb.ExecuteActionRequest{Seed: u64(8)})
	if soleElement(t, "d", first.Outputs["d"]).GetRealValue() == soleElement(t, "d", other.Outputs["d"]).GetRealValue() {
		t.Errorf("seeds 7 and 8 drew the same %v", first.Outputs)
	}
	for policy, want := range map[string]struct {
		d float64
		n int64
	}{"min": {0, 1}, "max": {1, 6}} {
		resp := run(&pb.ExecuteActionRequest{Draws: policy})
		if soleElement(t, "d", resp.Outputs["d"]).GetRealValue() != want.d || soleElement(t, "n", resp.Outputs["n"]).GetIntValue() != want.n {
			t.Errorf("draws %s = %v, want d=%g n=%d", policy, resp.Outputs, want.d, want.n)
		}
	}
}

// -clock-step over the wire: a wait comes due at the first multiple of the
// step not before it ends.
func TestExecuteActionClockStepRoundsTheWaits(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParse(t, srv, timedActionModel)
	resp, err := srv.ExecuteAction(context.Background(), &pb.ExecuteActionRequest{
		ModelHash: hash, ActionSymbolId: "Test::delayed", ClockStep: 4,
	})
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("ExecuteAction: %s", resp.Error)
	}
	if resp.FinalTime != 8 {
		t.Errorf("final_time = %g, want 8: the 5 s wait comes due on the 4 s tick after it", resp.FinalTime)
	}
}

// -disable-lint and -enable-lint over the wire: the diagnostics follow the
// request's selection, the parse's cached default selection untouched.
func TestGetDiagnosticsSelectsLints(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParse(t, srv, `package P {
	private import ScalarValues::*;
	attribute x : Real = 0.1;
	state def M { entry; then a; state a; state b; transition first a when Pnig then b; }
}`)
	codes := func(req *pb.DiagnosticsRequest) map[string]bool {
		t.Helper()
		req.ModelHash = hash
		resp, err := srv.GetDiagnostics(context.Background(), req)
		if err != nil {
			t.Fatalf("GetDiagnostics %v: %v", req, err)
		}
		got := map[string]bool{}
		for _, d := range resp.Diagnostics {
			got[d.GetCode()] = true
		}
		return got
	}
	if got := codes(&pb.DiagnosticsRequest{}); !got[passes.CodeUndeclaredSignal] || got[passes.CodeRoundedRealLiteral] {
		t.Errorf("default selection reports %v", got)
	}
	if got := codes(&pb.DiagnosticsRequest{EnableLint: []string{passes.CodeRoundedRealLiteral}}); !got[passes.CodeRoundedRealLiteral] || !got[passes.CodeUndeclaredSignal] {
		t.Errorf("enable-lint rounded-real-literal reports %v", got)
	}
	if got := codes(&pb.DiagnosticsRequest{DisableLint: []string{passes.CodeUndeclaredSignal}}); got[passes.CodeUndeclaredSignal] {
		t.Errorf("disable-lint undeclared-signal reports %v", got)
	}
	both := &pb.DiagnosticsRequest{EnableLint: []string{passes.CodeRoundedRealLiteral}, DisableLint: []string{passes.CodeRoundedRealLiteral}}
	if got := codes(both); got[passes.CodeRoundedRealLiteral] {
		t.Errorf("disable-lint wins over enable-lint, got %v", got)
	}
	if got := codes(&pb.DiagnosticsRequest{}); got[passes.CodeRoundedRealLiteral] {
		t.Errorf("a selection changed the default answer: %v", got)
	}
}

// A model parsed from a lineage keeps its lint findings for a later selection.
func TestGetDiagnosticsSelectsLintsOnALineageModel(t *testing.T) {
	srv := mustNewService(t, 10)
	doc := func(x string) *pb.SourceDocument {
		return &pb.SourceDocument{Name: "p.sysml", Source: &pb.SourceDocument_Content{Content: "package P { private import ScalarValues::*; attribute x : Real = " + x + "; }"}}
	}
	first, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{Documents: []*pb.SourceDocument{doc("0.1")}})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	second, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{Documents: []*pb.SourceDocument{doc("0.2")}, BaseModelHash: first.ModelHash})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	resp, err := srv.GetDiagnostics(context.Background(), &pb.DiagnosticsRequest{ModelHash: second.ModelHash, EnableLint: []string{passes.CodeRoundedRealLiteral}})
	if err != nil {
		t.Fatalf("GetDiagnostics: %v", err)
	}
	for _, d := range resp.Diagnostics {
		if d.GetCode() == passes.CodeRoundedRealLiteral {
			return
		}
	}
	t.Errorf("the opt-in lint is not reported on the lineage model: %v", resp.Diagnostics)
}

// -engines -probe over the wire: the built-in engines are listed as without it.
func TestListEnginesProbeListsTheBuiltInEngines(t *testing.T) {
	srv := mustNewService(t, 10)
	plain, err := srv.ListEngines(context.Background(), &pb.ListEnginesRequest{})
	if err != nil {
		t.Fatalf("ListEngines: %v", err)
	}
	probed, err := srv.ListEngines(context.Background(), &pb.ListEnginesRequest{Probe: true})
	if err != nil {
		t.Fatalf("ListEngines probe: %v", err)
	}
	if len(plain.Engines) != len(probed.Engines) || len(probed.Engines) == 0 {
		t.Fatalf("probe lists %d engines, plain %d", len(probed.Engines), len(plain.Engines))
	}
	for i := range plain.Engines {
		if plain.Engines[i].Name != probed.Engines[i].Name || plain.Engines[i].Ready != probed.Engines[i].Ready {
			t.Errorf("probe lists %v, plain %v", probed.Engines[i], plain.Engines[i])
		}
	}
}

// -tool-dry-run over the wire: the case runs to its first external tool call,
// which is previewed rather than made; a case with no tool call says so.
func TestRunAnalysisToolDryRunPreviewsTheToolCall(t *testing.T) {
	demo := filepath.Join("..", "..", "..", "examples", "external-tool-demo")
	// The manifest directory must be the test's own: the registry refuses one
	// writable by others, which a checkout may be.
	tools := t.TempDir()
	for _, name := range []string{"thermal.json", "thermal.py"} {
		data, err := os.ReadFile(filepath.Join(demo, "tools", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tools, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(analysis.ToolsEnv, tools)
	t.Setenv(analysis.EnginesEnv, "")
	srv := mustNewService(t, 10)
	t.Cleanup(srv.Close)
	model, err := os.ReadFile(filepath.Join(demo, "thermal.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	hash := mustParse(t, srv, string(model))

	resp, err := srv.RunAnalysis(context.Background(), &pb.RunAnalysisRequest{ModelHash: hash, SymbolId: "ThermalDemo::heating", ToolDryRun: true})
	if err != nil {
		t.Fatalf("RunAnalysis: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("dry run error: %s", resp.Error)
	}
	dry := resp.GetToolDryRun()
	if dry == nil {
		t.Fatal("no tool_dry_run in the response")
	}
	if dry.Tool != "ThermalSolver" || dry.Version != "1.0" || dry.Executable == "" || dry.Stdin != "none" || dry.ReplySource != "stdout" {
		t.Errorf("preview = %v", dry)
	}
	if !strings.Contains(dry.Action, "Solve") {
		t.Errorf("action = %q, want the ToolExecution-annotated action", dry.Action)
	}
	if got := strings.Join(dry.Args, " "); !strings.Contains(got, "--mass 12.5") {
		t.Errorf("args = %q, want the bound mass rendered", got)
	}
	inputs := map[string]string{}
	for _, in := range dry.Inputs {
		inputs[in.Variable] = in.Value
	}
	if inputs["power"] != "250.0" || inputs["ambient"] != "290.0" {
		t.Errorf("inputs = %v", inputs)
	}
	if len(dry.Outputs) != 2 || len(dry.Lines) == 0 || dry.Lines[len(dry.Lines)-1] != "the process was not started" {
		t.Errorf("outputs %v, lines %v", dry.Outputs, dry.Lines)
	}
	if len(resp.Outputs)+len(resp.Verdicts)+len(resp.Instances) != 0 {
		t.Errorf("a dry run kept the run's results: %v %v", resp.Outputs, resp.Verdicts)
	}

	plain := mustParse(t, srv, timedActionModel+"\npackage Cases { analysis def Nothing { return : ScalarValues::Integer = 1; } }")
	none, err := srv.RunAnalysis(context.Background(), &pb.RunAnalysisRequest{ModelHash: plain, SymbolId: "Cases::Nothing", ToolDryRun: true})
	if err != nil {
		t.Fatalf("RunAnalysis: %v", err)
	}
	if none.Error != msgNoToolReached || none.ToolDryRun != nil {
		t.Errorf("a case without a tool call answered %q, %v", none.Error, none.ToolDryRun)
	}
}
