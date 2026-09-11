package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/testutil/gobuild"
)

// pilotFixture is the pilot corpus file declaring ComputeDynamics, the action ToolExecution
// hands to ModelCenter; pilotRequireEnv turns its absence into a failure, as CI does.
const (
	pilotFixture    = "../../../examples/pilot-corpora/sysml-examples/Analysis Examples/AnalysisAnnotation.sysml"
	pilotRequireEnv = "OPENSYSML_REQUIRE_PILOT_CORPORA"
)

// pilotDriver performs the fixture's action with fixed inputs and adopts its outputs; Twice
// performs it twice with equal inputs, so a tool answering differently is seen in one run;
// Alike asks the same of it through two actions naming their parameters differently; Bodied
// states a body no token flow can run, which its tool never sees; Swept takes the drag
// coefficient as its input, the parameter a sweep ranges over.
const pilotDriver = `package Drive {
	private import AnalysisAnnotation::ComputeDynamics;
	private import AnalysisTooling::*;
	private import ScalarValues::Real;
	private import ISQ::*;

	action def Once {
		out a : AccelerationValue;
		out v : SpeedValue;
		out x : LengthValue;
		action step : ComputeDynamics {
			in dt = 1 [SI::s];
			in whlpwr = 2 [SI::kW];
			in Cd = 0.3;
			in Cf = 0.01;
			in tm = 1500 [SI::kg];
			in v_in = 36 [SI::km / SI::h];
			in x_in = 100 [SI::m];
		}
		bind a = step.a_out;
		bind v = step.v_out;
		bind x = step.x_out;
	}

	action def Twice {
		out a1 : AccelerationValue;
		out a2 : AccelerationValue;
		first start;
		then action stepA : ComputeDynamics {
			in dt = 1 [SI::s]; in whlpwr = 2 [SI::kW]; in Cd = 0.3; in Cf = 0.01;
			in tm = 1500 [SI::kg]; in v_in = 36 [SI::km / SI::h]; in x_in = 100 [SI::m];
		}
		then action stepB : ComputeDynamics {
			in dt = 1 [SI::s]; in whlpwr = 2 [SI::kW]; in Cd = 0.3; in Cf = 0.01;
			in tm = 1500 [SI::kg]; in v_in = 36 [SI::km / SI::h]; in x_in = 100 [SI::m];
		}
		bind a1 = stepA.a_out;
		bind a2 = stepB.a_out;
	}

	action def SameDynamics {
		metadata ToolExecution {
			toolName = "ModelCenter";
			uri = "aserv://localhost/Vehicle/Equation1";
		}
		in deltaT : TimeValue         { @ToolVariable { name = "deltaT"; } }
		in power : PowerValue         { @ToolVariable { name = "power"; } }
		in dragC : Real               { @ToolVariable { name = "C_D"; } }
		in frictionC : Real           { @ToolVariable { name = "C_F"; } }
		in mass : MassValue           { @ToolVariable { name = "mass"; } }
		in speed0 : SpeedValue        { @ToolVariable { name = "v0"; } }
		in position0 : LengthValue    { @ToolVariable { name = "x0"; } }
		out acceleration : AccelerationValue { @ToolVariable { name = "a"; } }
		out speed : SpeedValue        { @ToolVariable { name = "v"; } }
		out position : LengthValue    { @ToolVariable { name = "x"; } }
	}

	action def Alike {
		out a1 : AccelerationValue;
		out a2 : AccelerationValue;
		first start;
		then action stepA : ComputeDynamics {
			in dt = 1 [SI::s]; in whlpwr = 2 [SI::kW]; in Cd = 0.3; in Cf = 0.01;
			in tm = 1500 [SI::kg]; in v_in = 36 [SI::km / SI::h]; in x_in = 100 [SI::m];
		}
		then action stepB : SameDynamics {
			in deltaT = 1 [SI::s]; in power = 2 [SI::kW]; in dragC = 0.3; in frictionC = 0.01;
			in mass = 1500 [SI::kg]; in speed0 = 36 [SI::km / SI::h]; in position0 = 100 [SI::m];
		}
		bind a1 = stepA.a_out;
		bind a2 = stepB.acceleration;
	}

	action def Bodied {
		metadata ToolExecution {
			toolName = "ModelCenter";
			uri = "aserv://localhost/Vehicle/Equation1";
		}
		in deltaT : TimeValue         { @ToolVariable { name = "deltaT"; } }
		in power : PowerValue         { @ToolVariable { name = "power"; } }
		in dragC : Real               { @ToolVariable { name = "C_D"; } }
		in frictionC : Real           { @ToolVariable { name = "C_F"; } }
		in mass : MassValue           { @ToolVariable { name = "mass"; } }
		in speed0 : SpeedValue        { @ToolVariable { name = "v0"; } }
		in position0 : LengthValue    { @ToolVariable { name = "x0"; } }
		out acceleration : AccelerationValue { @ToolVariable { name = "a"; } }
		out speed : SpeedValue        { @ToolVariable { name = "v"; } }
		out position : LengthValue    { @ToolVariable { name = "x"; } }
		assign speed := speed0;
	}

	action def Embodied {
		out a : AccelerationValue;
		out v : SpeedValue;
		action step : Bodied {
			in deltaT = 1 [SI::s]; in power = 2 [SI::kW]; in dragC = 0.3; in frictionC = 0.01;
			in mass = 1500 [SI::kg]; in speed0 = 36 [SI::km / SI::h]; in position0 = 100 [SI::m];
		}
		bind a = step.acceleration;
		bind v = step.speed;
	}

	analysis def Swept {
		in drag : Real;
		action step : ComputeDynamics {
			in dt = 1 [SI::s]; in whlpwr = 2 [SI::kW]; in Cd = drag; in Cf = 0.01;
			in tm = 1500 [SI::kg]; in v_in = 36 [SI::km / SI::h]; in x_in = 100 [SI::m];
		}
		out a : AccelerationValue = step.a_out;
	}
}`

// standinEnv are the stand-in's own variables, from testdata/toolstandin.
const (
	standinMode    = "TOOL_STANDIN_MODE"
	standinRecord  = "TOOL_STANDIN_RECORD"
	standinCounter = "TOOL_STANDIN_COUNTER"
)

var (
	standinOnce sync.Once
	standinPath string
	standinErr  error
)

// standin builds the stand-in tool once per test binary and returns its path.
func standin(t *testing.T) string {
	t.Helper()
	standinOnce.Do(func() {
		dir, err := os.MkdirTemp("", "toolstandin")
		if err != nil {
			standinErr = err
			return
		}
		standinPath = filepath.Join(dir, "toolstandin")
		build := exec.Command("go", gobuild.Args(standinPath)...)
		build.Dir = filepath.Join("testdata", "toolstandin")
		if out, err := build.CombinedOutput(); err != nil {
			standinErr = fmt.Errorf("go build: %v\n%s", err, out)
		}
	})
	if standinErr != nil {
		t.Fatalf("building the stand-in tool: %v", standinErr)
	}
	return standinPath
}

// pilotEntry is the manifest entry the fixture's ToolExecution resolves to, answered by
// the stand-in.
func pilotEntry(executable string) ToolEntry {
	return ToolEntry{ToolName: "ModelCenter", Version: "stand-in", Executable: executable,
		Variables: []string{"deltaT", "power", "C_D", "C_F", "mass", "v0", "x0", "a", "v", "x"}}
}

// manifestDir writes one JSON file per entry into a directory of the test's own.
func manifestDir(t *testing.T, entries ...ToolEntry) string {
	t.Helper()
	dir := t.TempDir()
	for i, entry := range entries {
		data, err := json.MarshalIndent(entry, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		name := fmt.Sprintf("%02d-%s%s", i, strings.ToLower(entry.ToolName), ManifestExt)
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// toolRegistry is the default registry with the manifest's tools, as the environment adds them.
func toolRegistry(t *testing.T, dir string) *Registry {
	t.Helper()
	t.Setenv(ToolsEnv, dir)
	r, err := DefaultFromEnv()
	if err != nil {
		t.Fatalf("%s=%s: %v", ToolsEnv, dir, err)
	}
	return r
}

// pilot is the fixture and its driver, indexed over the standard libraries.
type pilot struct {
	idx *symbols.Index
	pkg *symbols.Scope
}

// parsePilot loads the fixture and the driver; without the corpus the test is skipped,
// unless the corpus is required.
func parsePilot(t *testing.T) *pilot {
	t.Helper()
	data, err := os.ReadFile(pilotFixture)
	if err != nil {
		if os.Getenv(pilotRequireEnv) != "" {
			t.Fatalf("%s is set but the pilot corpus is absent: %v", pilotRequireEnv, err)
		}
		t.Skip("pilot corpora absent; run ./scripts/download-pilot-corpora.sh")
	}
	idx := libs.NewModelIndex()
	for _, doc := range []struct{ path, text string }{{pilotFixture, string(data)}, {"driver.sysml", pilotDriver}} {
		p := parser.New(source.New(doc.path, []byte(doc.text)))
		file := p.ParseFile()
		if len(p.Diagnostics) > 0 {
			t.Fatalf("parse %s: %v", doc.path, p.Diagnostics)
		}
		idx.AddDocument(doc.path, file)
	}
	idx.ExpandWildcardImports()
	pkg, ok := idx.DocumentRoot("driver.sysml").LookupLocal("Drive")
	if !ok || pkg.Scope == nil {
		t.Fatal("driver package not indexed")
	}
	return &pilot{idx: idx, pkg: pkg.Scope}
}

func (p *pilot) semantics() (*runtime.Model, error) {
	resolver := resolve.New(p.idx)
	return runtime.NewModel(semantics.NewModel(resolver), resolver), nil
}

func (p *pilot) fresh(w *Worker) (*runtime.Context, error) {
	return runtime.NewContext(w.Model, fixtureSteps), nil
}

// building is the model as a surface holding no context supplies it.
func (p *pilot) building() *Model { return &Model{Semantics: p.semantics, Fresh: p.fresh} }

// context is a runtime a surface would hold over the fixture.
func (p *pilot) context() *runtime.Context {
	model, _ := p.semantics()
	return runtime.NewContext(model, fixtureSteps)
}

func (p *pilot) action(t *testing.T, name string) *symbols.Symbol {
	t.Helper()
	sym, ok := p.pkg.LookupLocal(name)
	if !ok {
		t.Fatalf("%s not indexed", name)
	}
	return sym
}

// perform puts one performance of the driver's action to the registry in a held context.
func (p *pilot) perform(t *testing.T, r *Registry, ctx *runtime.Context, name string) (map[string]runtime.Value, Plan, error) {
	t.Helper()
	action := p.action(t, name)
	return Perform(context.Background(), r, Held(ctx), "Drive::"+name, runtime.DefaultSchedulePolicy, Budget{}, Auto(),
		func(rctx *runtime.Context) (map[string]runtime.Value, error) { return rctx.ExecuteAction(action) },
		func(out map[string]runtime.Value, err error) Answer {
			if err != nil {
				return Answer{Err: err}
			}
			return Answer{Claim: ClaimValue, Values: ValuesOf(out)}
		})
}

// wantValues checks the outputs against their canonical spellings.
func wantValues(t *testing.T, out map[string]runtime.Value, want map[string]string) {
	t.Helper()
	for name, spelling := range want {
		got, ok := out[name]
		if !ok {
			t.Fatalf("output %s not propagated; got %v", name, out)
		}
		if runtime.FormatValue(got) != spelling {
			t.Errorf("%s = %s, want %s", name, runtime.FormatValue(got), spelling)
		}
	}
}

// requests reads every request the stand-in recorded.
func requests(t *testing.T, record string) []map[string]json.RawMessage {
	t.Helper()
	data, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("the stand-in recorded no request: %v", err)
	}
	var seen []map[string]json.RawMessage
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var req map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			t.Fatalf("request %q is not one JSON object: %v", line, err)
		}
		seen = append(seen, req)
	}
	return seen
}

// The fixture's performance goes to the stand-in as one process with one request: the tool
// and URI as annotated, the inputs under their ToolVariable names with their units; the
// outputs come back converted to the parameters' units and the enclosing action adopts them.
func TestPilotFixtureRunsAgainstTheStandIn(t *testing.T) {
	p := parsePilot(t)
	record := filepath.Join(t.TempDir(), "requests.jsonl")
	t.Setenv(standinRecord, record)
	r := toolRegistry(t, manifestDir(t, pilotEntry(standin(t))))

	out, plan, err := p.perform(t, r, p.context(), "Once")
	if err != nil {
		t.Fatalf("perform: %v", err)
	}
	if plan.Result.Engine != RunEngineName || plan.Result.Strength != Observed {
		t.Fatalf("plan result %+v, want the run engine's observed value", plan.Result)
	}
	wantValues(t, out, map[string]string{"a": "3.0 [SI::'m⋅s⁻²']", "v": "12.0 [SI::'m/s']", "x": "110.0 [SI::m]"})

	seen := requests(t, record)
	if len(seen) != 1 {
		t.Fatalf("stand-in ran %d times, want once", len(seen))
	}
	req := seen[0]
	if string(req["toolName"]) != `"ModelCenter"` || string(req["uri"]) != `"aserv://localhost/Vehicle/Equation1"` {
		t.Errorf("request names %s at %s, want the annotation as written", req["toolName"], req["uri"])
	}
	var inputs map[string]struct {
		Value json.RawMessage `json:"value"`
		Unit  string          `json:"unit"`
	}
	if err := json.Unmarshal(req["inputs"], &inputs); err != nil {
		t.Fatalf("inputs: %v", err)
	}
	want := map[string][2]string{"deltaT": {"1", "s"}, "power": {"2", "kW"}, "C_D": {"0.3", ""}, "C_F": {"0.01", ""},
		"mass": {"1500", "kg"}, "v0": {"36", "km/h"}, "x0": {"100", "m"}}
	if len(inputs) != len(want) {
		t.Fatalf("inputs %v, want the seven ToolVariables", inputs)
	}
	for name, w := range want {
		got, ok := inputs[name]
		if !ok || string(got.Value) != w[0] || got.Unit != w[1] {
			t.Errorf("input %s = %s [%s], want %s [%s]", name, got.Value, got.Unit, w[0], w[1])
		}
	}
}

// Each way the stand-in can fail is the typed error of its kind, the performance fails
// with it, no output is invented for the enclosing action, and the run engine's answer is
// that nothing was established, for the fault's reason.
func TestPilotFixtureFailsWithTheToolsFault(t *testing.T) {
	cases := []struct {
		mode string
		kind runtime.ToolErrorKind
		text string
	}{
		{"missing-output", runtime.ToolMissingOutput, "x"},
		{"unknown-output", runtime.ToolUnknownOutput, "y"},
		{"duplicate-output", runtime.ToolMalformed, "twice"},
		{"malformed", runtime.ToolMalformed, "JSON"},
		{"flood", runtime.ToolMalformed, "wrote more than"},
		{"string-unit", runtime.ToolMalformed, "unit"},
		{"wrong-unit", runtime.ToolMalformed, "kg"},
		{"wrong-type", runtime.ToolMalformed, "true"},
		{"error", runtime.ToolRefused, "equation did not converge"},
		{"exit", runtime.ToolProcessFailed, "license server unreachable"},
	}
	p := parsePilot(t)
	r := toolRegistry(t, manifestDir(t, pilotEntry(standin(t))))
	for _, tc := range cases {
		t.Run(tc.mode, func(t *testing.T) {
			t.Setenv(standinMode, tc.mode)
			out, plan, err := p.perform(t, r, p.context(), "Once")
			var fault *runtime.ToolError
			if !errors.As(err, &fault) || fault.Kind != tc.kind || fault.Tool != "ModelCenter" {
				t.Fatalf("perform = %v, want a ToolError of kind %s", err, tc.kind)
			}
			if !strings.Contains(err.Error(), tc.text) {
				t.Errorf("error %q does not carry %q", err, tc.text)
			}
			if len(out) != 0 {
				t.Errorf("outputs %v, want none from a failed tool", out)
			}
			wantFaulted(t, plan, err)
		})
	}
}

// wantFaulted checks a plan whose one run failed: the run engine answered alone, claiming
// nothing, with the failure as its reason.
func wantFaulted(t *testing.T, plan Plan, fault error) {
	t.Helper()
	if len(plan.Steps) != 1 || plan.Steps[0].Engine != RunEngineName || plan.Steps[0].Result == nil {
		t.Fatalf("plan %+v, want the run engine's one step", plan.Steps)
	}
	if r := plan.Result; r.Covered() || r.Claim != ClaimNone || r.Strength != NotCovered || r.Reason != fault.Error() {
		t.Errorf("result %+v, want nothing claimed for %q", r, fault)
	}
}

// A stand-in that never answers is stopped at OPENSYSML_TOOL_TIMEOUT and the performance
// fails with the timeout.
func TestPilotFixtureTimesOut(t *testing.T) {
	p := parsePilot(t)
	t.Setenv(standinMode, "hang")
	t.Setenv(ToolTimeoutEnv, "200ms")
	r := toolRegistry(t, manifestDir(t, pilotEntry(standin(t))))
	_, _, err := p.perform(t, r, p.context(), "Once")
	var fault *runtime.ToolError
	if !errors.As(err, &fault) || fault.Kind != runtime.ToolTimeout {
		t.Fatalf("perform = %v, want a timeout", err)
	}
	if !strings.Contains(err.Error(), "200ms") || !strings.Contains(err.Error(), ToolTimeoutEnv) {
		t.Errorf("timeout %q does not name the limit and its variable", err)
	}
}

// With OPENSYSML_TOOLS unset, or naming a manifest without the tool, the performance is
// refused as not registered: the body is not run and nothing completes silently.
func TestPilotFixtureRefusesAnUnregisteredTool(t *testing.T) {
	p := parsePilot(t)
	other := ToolEntry{ToolName: "Other", Executable: standin(t), Variables: []string{"a"}}
	for name, r := range map[string]*Registry{"unset": Default(), "another tool": toolRegistry(t, manifestDir(t, other))} {
		t.Run(name, func(t *testing.T) {
			out, plan, err := p.perform(t, r, p.context(), "Once")
			var refusal *runtime.ToolNotRegisteredError
			if !errors.As(err, &refusal) || refusal.Tool != "ModelCenter" {
				t.Fatalf("perform = %v, want ToolNotRegisteredError", err)
			}
			if want := "tool 'ModelCenter' is not registered; set OPENSYSML_TOOLS"; !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not carry %q", err, want)
			}
			if len(out) != 0 {
				t.Errorf("outputs %v, want none", out)
			}
			wantFaulted(t, plan, err)
		})
	}
}

// A sweep's rows run in contexts of their own, on jobs of their own, and every one carries
// the plan's tool runner: the tool-computed performance inside the swept action goes to the
// stand-in once per row, and the rows table its answers in plan order. Without the tool the
// rows fail as the held performance does, so the rows are seen to reach the tool path.
func TestPilotFixtureSweepsThroughTheToolOnEveryJob(t *testing.T) {
	p := parsePilot(t)
	record := filepath.Join(t.TempDir(), "requests.jsonl")
	t.Setenv(standinRecord, record)
	swept := p.action(t, "Swept")
	ctx := p.context()
	plan, err := ctx.ResolveSweepPlan(swept, runtime.SweepPlan{Ranges: []runtime.SweepRange{{Param: "drag", From: intOf(1), To: intOf(4)}}}, 0, nil)
	if err != nil {
		t.Fatalf("resolve the plan: %v", err)
	}
	row := func(rctx *runtime.Context, bindings []runtime.SweepBinding) (runtime.SweepRunResult, error) {
		inputs := make(map[string]runtime.Value, len(bindings))
		for _, b := range bindings {
			inputs[b.Param] = b.Value
		}
		result, err := rctx.RunAnalysis(swept, runtime.AnalysisArgs{Named: inputs}, p.pkg, nil)
		return runtime.SweepRunResult{Outputs: result.Outputs}, err
	}
	sweep := func(r *Registry) runtime.SweepTable {
		t.Helper()
		answered, err := r.Sweep(context.Background(), p.building(), "Drive::Swept", runtime.DefaultSchedulePolicy, plan, row, Budget{Jobs: 8}, Auto())
		if err != nil {
			t.Fatalf("sweep: %v", err)
		}
		table := answered.Result.Table()
		if len(table.Rows) != 4 {
			t.Fatalf("result %+v, want a table of 4 rows", answered.Result)
		}
		if answered.Workers < 2 || answered.Workers > 4 {
			t.Fatalf("plan built %d workers, want the rows on two to four jobs", answered.Workers)
		}
		return table
	}

	table := sweep(toolRegistry(t, manifestDir(t, pilotEntry(standin(t)))))
	seen := make(map[*runtime.Context]bool, len(table.Rows))
	for i, row := range table.Rows {
		if row.Err != nil {
			t.Fatalf("row %d: %v", i, row.Err)
		}
		if want := fmt.Sprintf("%d.0 [SI::'m⋅s⁻²']", 10*(i+1)); len(row.Outputs) != 1 || runtime.FormatValue(row.Outputs[0].Value) != want {
			t.Fatalf("row %d = %+v, want a = %s in plan order", i, row.Outputs, want)
		}
		if row.Context == nil || row.Context == ctx || seen[row.Context] {
			t.Fatalf("row %d ran in %p, want a context of its own", i, row.Context)
		}
		seen[row.Context] = true
	}
	if seen := requests(t, record); len(seen) != 4 {
		t.Fatalf("stand-in ran %d times, want once per row", len(seen))
	}

	for i, row := range sweep(Default()).Rows {
		var refusal *runtime.ToolNotRegisteredError
		if !errors.As(row.Err, &refusal) || refusal.Tool != "ModelCenter" {
			t.Fatalf("row %d without the tool = %v, want ToolNotRegisteredError", i, row.Err)
		}
	}
}

// A manifest entry whose executable is absent is registered, listed with its status, and
// refuses the performance with the absence rather than running the body.
func TestPilotFixtureRefusesAnAbsentExecutable(t *testing.T) {
	p := parsePilot(t)
	missing := filepath.Join(t.TempDir(), "modelcenter")
	r := toolRegistry(t, manifestDir(t, pilotEntry(missing)))
	listing := r.Listings()[len(r.Listings())-1]
	if listing.Engine != "tool:ModelCenter" || listing.Ready() || !strings.HasPrefix(listing.StatusText(), "unavailable: tool 'ModelCenter': executable "+missing) {
		t.Fatalf("listing %+v, want tool:ModelCenter unavailable", listing)
	}
	out, _, err := p.perform(t, r, p.context(), "Once")
	if !errors.Is(err, ErrProcessAbsent) || !errors.Is(err, ErrToolAbsent) {
		t.Fatalf("perform = %v, want the executable's absence", err)
	}
	if len(out) != 0 {
		t.Errorf("outputs %v, want none", out)
	}
}

// Two actions asking the tool the same request and binding its one answer under different
// parameter names are not a divergence: the replies compare as the tool wrote them.
func TestPilotFixtureComparesRepliesNotBindings(t *testing.T) {
	p := parsePilot(t)
	r := toolRegistry(t, manifestDir(t, pilotEntry(standin(t))))
	ctx := p.context()
	out, _, err := p.perform(t, r, ctx, "Alike")
	if err != nil {
		t.Fatalf("perform: %v", err)
	}
	wantValues(t, out, map[string]string{"a1": "3.0 [SI::'m⋅s⁻²']", "a2": "3.0 [SI::'m⋅s⁻²']"})
	if notes := ctx.Notes(); len(notes) != 0 {
		t.Fatalf("notes %v, want none: equal replies bound under different names", notes)
	}
}

// An annotated action's body is never run, so one no token flow can be lowered from does
// not keep the tool from performing the action; the tool's answer is what it outputs.
func TestPilotFixturePerformsAnUnlowerableBodyByTool(t *testing.T) {
	p := parsePilot(t)
	bodied := p.action(t, "Bodied")
	if _, err := lower.ToActionGraph(bodied.Decl, bodied.Scope); !errors.Is(err, lower.ErrStatementOutsideFlow) {
		t.Fatalf("Bodied lowers to a flow (%v); its body should not", err)
	}
	r := toolRegistry(t, manifestDir(t, pilotEntry(standin(t))))
	out, plan, err := p.perform(t, r, p.context(), "Embodied")
	if err != nil {
		t.Fatalf("perform: %v", err)
	}
	wantValues(t, out, map[string]string{"a": "3.0 [SI::'m⋅s⁻²']", "v": "12.0 [SI::'m/s']"})
	if plan.Result.Claim != ClaimValue || plan.Result.Strength != Observed {
		t.Fatalf("result %+v, want the tool's answer observed", plan.Result)
	}
}

// A tool answering equal inputs differently is observed as it answered, each time, and the
// run notes the divergence at the second performance; explore sees the same.
func TestPilotFixtureReportsANonDeterministicTool(t *testing.T) {
	p := parsePilot(t)
	t.Setenv(standinMode, "varying")
	t.Setenv(standinCounter, filepath.Join(t.TempDir(), "count"))
	r := toolRegistry(t, manifestDir(t, pilotEntry(standin(t))))

	ctx := p.context()
	out, _, err := p.perform(t, r, ctx, "Twice")
	if err != nil {
		t.Fatalf("perform: %v", err)
	}
	wantValues(t, out, map[string]string{"a1": "1.0 [SI::'m⋅s⁻²']", "a2": "2.0 [SI::'m⋅s⁻²']"})
	notes := ctx.Notes()
	if len(notes) != 1 {
		t.Fatalf("notes %v, want the one divergence", notes)
	}
	diverged, ok := notes[0].(runtime.ToolDivergence)
	if !ok || diverged.Tool != "ModelCenter" || diverged.Action != "AnalysisAnnotation::ComputeDynamics" {
		t.Fatalf("note %v, want ModelCenter diverging at ComputeDynamics", notes[0])
	}
	if d := diverged.Diagnostic(); d.Code != runtime.ToolDivergenceCode || !strings.Contains(d.Message, "equal inputs") {
		t.Errorf("diagnostic %+v, want %s naming equal inputs", d, runtime.ToolDivergenceCode)
	}

	// Under explore each linearization is a run of its own; the tool's answers are
	// observed as distinct outcomes, never reconciled.
	once := p.action(t, "Once")
	plan, err := r.Explore(context.Background(), p.building(), "Drive::Once", policy(t, "explore:runs=2"),
		func(rctx *runtime.Context) (runtime.Outcome, error) {
			outputs, err := rctx.ExecuteAction(once)
			if err != nil {
				return runtime.Outcome{}, err
			}
			return rctx.ActionOutcome(outputs), nil
		}, Budget{}, Auto())
	if err != nil {
		t.Fatalf("explore: %v", err)
	}
	x := plan.Result.Exploration()
	if x == nil || x.Runs != 1 || len(x.Outcomes) != 1 || x.Outcomes[0].Outcome.Err != nil {
		t.Fatalf("exploration %+v, want the one linearization observed", x)
	}
	if got := runtime.FormatValue(x.Outcomes[0].Outcome.Outputs["a"]); got != "3.0 [SI::'m⋅s⁻²']" {
		t.Errorf("explored a = %s, want the third answer as the tool gave it", got)
	}
}

// -engine tool:<name> on a question the tool does not answer stops with its refusal, and
// auto never advances past a tool failure: the tool's engine is the only one asked for the
// computation, so its fault is the performance's, and the run claims nothing.
func TestToolEngineDispatch(t *testing.T) {
	p := parsePilot(t)
	r := toolRegistry(t, manifestDir(t, pilotEntry(standin(t))))
	once := p.action(t, "Once")
	call := func(rctx *runtime.Context) (map[string]runtime.Value, error) { return rctx.ExecuteAction(once) }
	answer := func(out map[string]runtime.Value, err error) Answer {
		if err != nil {
			return Answer{Err: err}
		}
		return Answer{Claim: ClaimValue, Values: ValuesOf(out)}
	}
	named, err := r.Select("tool:ModelCenter")
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	_, plan, err := Perform(context.Background(), r, Held(p.context()), "Drive::Once", runtime.DefaultSchedulePolicy, Budget{}, named, call, answer)
	var refused *RefusedError
	if !errors.As(err, &refused) || len(plan.Steps) != 1 || plan.Steps[0].Engine != "tool:ModelCenter" || !errors.Is(plan.Steps[0].Refusal, ErrNotAsked) {
		t.Fatalf("named tool: %v, plan %+v; want its refusal to stop the plan", err, plan.Steps)
	}

	t.Setenv(standinMode, "exit")
	_, plan, err = Perform(context.Background(), r, Held(p.context()), "Drive::Once", runtime.DefaultSchedulePolicy, Budget{}, Auto(), call, answer)
	if !errors.Is(err, runtime.ErrTool) {
		t.Fatalf("auto: %v, want the tool's fault", err)
	}
	wantFaulted(t, plan, err)
}

// The computation an annotated action asks for inside a run is put to the registry under
// the selection the run itself was asked under: `all` composes the tool's answer as it does
// the run's, and a named engine that does not answer compute refuses the computation, so
// `-engine run` never reaches the tool.
func TestToolEngineDispatchKeepsTheSelection(t *testing.T) {
	p := parsePilot(t)
	r := toolRegistry(t, manifestDir(t, pilotEntry(standin(t))))
	once := p.action(t, "Once")
	call := func(rctx *runtime.Context) (map[string]runtime.Value, error) { return rctx.ExecuteAction(once) }
	answer := func(out map[string]runtime.Value, err error) Answer {
		if err != nil {
			return Answer{Err: err}
		}
		return Answer{Claim: ClaimValue, Values: ValuesOf(out)}
	}
	want := map[string]string{"a": "3.0 [SI::'m⋅s⁻²']", "v": "12.0 [SI::'m/s']", "x": "110.0 [SI::m]"}

	out, plan, err := Perform(context.Background(), r, Held(p.context()), "Drive::Once", runtime.DefaultSchedulePolicy, Budget{}, All(), call, answer)
	if err != nil {
		t.Fatalf("all: %v, plan %+v", err, plan.Steps)
	}
	wantValues(t, out, want)

	run, err := r.Select("run")
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	out, plan, err = Perform(context.Background(), r, Held(p.context()), "Drive::Once", runtime.DefaultSchedulePolicy, Budget{}, run, call, answer)
	if err == nil || !errors.Is(err, ErrNotAsked) || len(out) != 0 {
		t.Fatalf("run alone: %v, outputs %v; want the run engine's refusal of the computation", err, out)
	}
	if got := plan.Refused(); got != nil {
		t.Fatalf("run alone refused %v; want the run to answer with the refusal as its fault", got)
	}
	wantFaulted(t, plan, err)
}
