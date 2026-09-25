package analysis

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
	"github.com/Open-MBEE/OpenSysML/tests/testutil/gobuild"
)

// replyDriver performs Thermal, a tool-computed action with five outputs, and Probe and
// Exit, single-output actions for the exitcode reply.
const replyDriver = `package ProbeR {
	private import AnalysisTooling::*;
	private import ScalarValues::*;
	private import ISQ::*;

	action def Thermal {
		metadata ToolExecution {
			toolName = "Thermal";
			uri = "thermal://host/solve";
		}
		in mass : MassValue          { @ToolVariable { name = "mass"; } }
		out T_max : TemperatureValue { @ToolVariable { name = "T_max"; } }
		out v_out : SpeedValue       { @ToolVariable { name = "v_out"; } }
		out ok : Boolean             { @ToolVariable { name = "done"; } }
		out code : Integer           { @ToolVariable { name = "code"; } }
		out note : String            { @ToolVariable { name = "note"; } }
	}

	action def Once {
		out T : TemperatureValue;
		out v : SpeedValue;
		out ok : Boolean;
		out code : Integer;
		out note : String;
		action step : Thermal {
			in mass = 1500 [SI::kg];
		}
		bind T = step.T_max;
		bind v = step.v_out;
		bind ok = step.ok;
		bind code = step.code;
		bind note = step.note;
	}

	action def Twice {
		out T1 : TemperatureValue;
		out T2 : TemperatureValue;
		first start;
		then action stepA : Thermal { in mass = 1500 [SI::kg]; }
		then action stepB : Thermal { in mass = 1500 [SI::kg]; }
		bind T1 = stepA.T_max;
		bind T2 = stepB.T_max;
	}

	action def Probe {
		metadata ToolExecution {
			toolName = "Thermal";
			uri = "thermal://host/solve";
		}
		out ok : Boolean { @ToolVariable { name = "done"; } }
	}

	action def ProbeOnce {
		out ok : Boolean;
		action step : Probe;
		bind ok = step.ok;
	}

	action def Exit {
		metadata ToolExecution {
			toolName = "Thermal";
			uri = "thermal://host/solve";
		}
		out code : Integer { @ToolVariable { name = "code"; } }
	}

	action def ExitOnce {
		out code : Integer;
		action step : Exit;
		bind code = step.code;
	}

	action def Signal {
		metadata ToolExecution {
			toolName = "Thermal";
			uri = "thermal://host/solve";
		}
		in mass : MassValue { @ToolVariable { name = "mass"; } }
	}

	action def SignalOnce {
		action step : Signal { in mass = 1500 [SI::kg]; }
	}
}`

var replyVariables = []string{"mass", "T_max", "v_out", "done", "code", "note"}

var (
	replyOnce    sync.Once
	replyBinPath string
	replyBinErr  error
)

// toolreply builds the reply stand-in once per test binary and returns its path.
func toolreply(t *testing.T) string {
	t.Helper()
	replyOnce.Do(func() {
		dir, err := os.MkdirTemp("", "toolreply")
		if err != nil {
			replyBinErr = err
			return
		}
		replyBinPath = filepath.Join(dir, "toolreply")
		build := exec.Command("go", gobuild.Args(replyBinPath)...)
		build.Dir = filepath.Join("testdata", "toolreply")
		if out, err := build.CombinedOutput(); err != nil {
			replyBinErr = fmt.Errorf("go build: %v\n%s", err, out)
		}
	})
	if replyBinErr != nil {
		t.Fatalf("building the reply stand-in: %v", replyBinErr)
	}
	return replyBinPath
}

// rprobe is the reply driver indexed over the standard libraries.
type rprobe struct {
	idx *symbols.Index
	pkg *symbols.Scope
}

func parseRProbe(t *testing.T) *rprobe {
	t.Helper()
	idx := libs.NewModelIndex()
	p := parser.New(source.New("rprobe.sysml", []byte(replyDriver)))
	file := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse: %v", p.Diagnostics)
	}
	idx.AddDocument("rprobe.sysml", file)
	idx.ExpandWildcardImports()
	pkg, ok := idx.DocumentRoot("rprobe.sysml").LookupLocal("ProbeR")
	if !ok || pkg.Scope == nil {
		t.Fatal("ProbeR package not indexed")
	}
	return &rprobe{idx: idx, pkg: pkg.Scope}
}

func (p *rprobe) context() *runtime.Context {
	resolver := resolve.New(p.idx)
	model := runtime.NewModel(passes.NewTypedModel(resolver), resolver)
	model.SetExpressionParser(parser.ParseOneExpression)
	return runtime.NewContext(model, fixtureSteps)
}

// perform runs one of the driver's actions through the registry and returns its outputs.
func (p *rprobe) perform(t *testing.T, r *Registry, name string) (map[string]runtime.Value, *runtime.Context, error) {
	t.Helper()
	action, ok := p.pkg.LookupLocal(name)
	if !ok {
		t.Fatalf("%s not indexed", name)
	}
	ctx := p.context()
	out, _, err := Perform(context.Background(), r, request(Held(ctx), "ProbeR::"+name, runtime.DefaultSchedulePolicy),
		func(rctx *runtime.Context) (map[string]runtime.Value, error) { return rctx.ExecuteAction(action) },
		func(out map[string]runtime.Value, err error) Answer {
			if err != nil {
				return Answer{Err: err}
			}
			return Answer{Claim: ClaimValue, Values: ValuesOf(out)}
		})
	return out, ctx, err
}

// thermalEntry is a manifest entry for the stand-in: the invocation's one mode argument
// and the reply block given.
func thermalEntry(executable string, args []string, reply *Reply) ToolEntry {
	return ToolEntry{ToolName: "Thermal", Executable: executable, Variables: replyVariables,
		Invocation: &Invocation{Args: args}, Reply: reply}
}

// wantReply checks the driver's four fixed outputs as the stand-in answered them; the
// `note` string and `ok` differ per mode and each test checks them itself.
func wantReply(t *testing.T, out map[string]runtime.Value) {
	t.Helper()
	wantValues(t, out, map[string]string{
		"T":    "341.2 [SI::K]",
		"v":    "10.0 [SI::'m/s']",
		"code": "7",
	})
}

var csvReplyOutputs = map[string]*ReplyOutput{
	"T_max": {Column: &Column{Name: "T_max"}, UnitColumn: &Column{Name: "U"}},
	"v_out": {Column: &Column{Name: "v_out"}, UnitColumn: &Column{Name: "VU"}},
	"done":  {Column: &Column{Name: "done"}, Type: TypeBoolean},
	"code":  {Column: &Column{Name: "code"}, Type: TypeInteger},
	"note":  {Column: &Column{Name: "note"}, Type: TypeString},
}

// A CSV reply on standard output is read cell by cell; the unit column's `km/h` is
// converted to the declared SpeedValue's coherent `m/s` by Bind.
func TestToolReplyReadsCSVFromStdout(t *testing.T) {
	p := parseRProbe(t)
	entry := thermalEntry(toolreply(t), []string{"csv-stdout"}, &Reply{Format: ReplyCSV, Outputs: csvReplyOutputs})
	dir := manifestDir(t, entry)
	out, _, err := p.perform(t, toolRegistry(t, dir), "Once")
	if err != nil {
		t.Fatalf("perform: %v", err)
	}
	wantReply(t, out)
	if runtime.FormatValue(out["ok"]) != "false" {
		t.Errorf("ok = %s", runtime.FormatValue(out["ok"]))
	}
	if out["note"].Str() != "second" {
		t.Errorf("note = %q", out["note"].Str())
	}
}

// A JSON reply read from a file the tool wrote into {outputDir}; the chatter on standard
// output is not parsed, and the invocation's directory is removed once it is read.
func TestToolReplyReadsJSONFromAFile(t *testing.T) {
	p := parseRProbe(t)
	entry := thermalEntry(toolreply(t), []string{"json-file", "{outputDir}"}, &Reply{
		Format: ReplyJSON, Source: "file:{outputDir}/result.json", ErrorPath: "/error",
		Outputs: map[string]*ReplyOutput{
			"T_max": {Path: jsonPath("/results/0/T_max"), UnitPath: "/units/T_max"},
			"v_out": {Path: jsonPath("/results/0/v_out"), UnitPath: "/units/v_out"},
			"done":  {Path: jsonPath("/results/0/done"), Type: TypeBoolean},
			"code":  {Path: jsonPath("/results/0/code"), Type: TypeInteger},
			"note":  {Path: jsonPath("/results/0/note"), Type: TypeString},
		}})
	out, _, err := p.perform(t, toolRegistry(t, manifestDir(t, entry)), "Once")
	if err != nil {
		t.Fatalf("perform: %v", err)
	}
	want := map[string]string{"T": "341.2 [SI::K]", "v": "10.0 [SI::'m/s']", "ok": "true", "code": "7"}
	for name, spelling := range want {
		if got := runtime.FormatValue(out[name]); got != spelling {
			t.Errorf("%s = %s, want %s", name, got, spelling)
		}
	}
	outDir := out["note"].Str()
	if _, err := os.Stat(outDir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("output directory %s still present: %v", outDir, err)
	}
}

// A key=value reply tolerates the tool's chatter lines; under regex each output is the
// named group of the one line matching it.
func TestToolReplyReadsLines(t *testing.T) {
	p := parseRProbe(t)
	plain := &Reply{Format: ReplyLines, Outputs: map[string]*ReplyOutput{
		"T_max": {Type: TypeReal, Unit: "K"},
		"v_out": {Unit: "km/h"},
		"done":  {Type: TypeBoolean},
		"code":  {Type: TypeInteger},
		"note":  {Type: TypeString},
	}}
	out, _, err := p.perform(t, toolRegistry(t, manifestDir(t, thermalEntry(toolreply(t), []string{"lines"}, plain))), "Once")
	if err != nil {
		t.Fatalf("perform: %v", err)
	}
	wantReply(t, out)
	if runtime.FormatValue(out["ok"]) != "true" {
		t.Errorf("ok = %s", runtime.FormatValue(out["ok"]))
	}
	if out["note"].Str() != "it ran" {
		t.Errorf("note = %q", out["note"].Str())
	}

	regex := &Reply{Format: ReplyLines, Regex: `T=(?P<T_max>[0-9.]+)K v=(?P<v_out>[0-9.]+)km/h done=(?P<done>[A-Z]+) code=(?P<code>[0-9]+) note=(?P<note>.+)`,
		Outputs: map[string]*ReplyOutput{
			"T_max": {Type: TypeReal, Unit: "K"},
			"v_out": {Unit: "km/h"},
			"done":  {Type: TypeBoolean},
			"code":  {Type: TypeInteger},
			"note":  {Type: TypeString},
		}}
	out, _, err = p.perform(t, toolRegistry(t, manifestDir(t, thermalEntry(toolreply(t), []string{"lines-regex"}, regex))), "Once")
	if err != nil {
		t.Fatalf("perform regex: %v", err)
	}
	wantReply(t, out)
	if runtime.FormatValue(out["ok"]) != "true" {
		t.Errorf("ok = %s", runtime.FormatValue(out["ok"]))
	}
	if out["note"].Str() != "it ran" {
		t.Errorf("note = %q", out["note"].Str())
	}
}

// Under exitcode the status is the reply: a status among `success` is true, another is
// false, and an `integer` output is the status itself.
func TestToolReplyReadsExitCode(t *testing.T) {
	p := parseRProbe(t)
	entry := func(mode string, reply *Reply) *Registry {
		e := thermalEntry(toolreply(t), []string{mode}, reply)
		e.Variables = keysOf(reply.Outputs)
		return toolRegistry(t, manifestDir(t, e))
	}
	boolean := &Reply{Format: ReplyExitCode, Success: []int{0, 3}, Outputs: map[string]*ReplyOutput{"done": {}}}
	out, _, err := p.perform(t, entry("exit3", boolean), "ProbeOnce")
	if err != nil || runtime.FormatValue(out["ok"]) != "true" {
		t.Fatalf("exit 3 with success [0,3] = %v, %v; want true", out["ok"], err)
	}
	out, _, err = p.perform(t, entry("exit1", boolean), "ProbeOnce")
	if err != nil || runtime.FormatValue(out["ok"]) != "false" {
		t.Fatalf("exit 1 = %v, %v; want false", out["ok"], err)
	}
	integer := &Reply{Format: ReplyExitCode, Outputs: map[string]*ReplyOutput{"code": {Type: TypeInteger}}}
	out, _, err = p.perform(t, entry("exit1", integer), "ExitOnce")
	if err != nil || runtime.FormatValue(out["code"]) != "1" {
		t.Fatalf("exit 1 as integer = %v, %v; want 1", out["code"], err)
	}
}

// A performance declaring one of the mapped outputs reads only it: the manifest maps all
// five but Probe asks for `done` alone, and nothing else is bound or faulted for.
func TestToolReplyReadsOnlyRequestedOutputs(t *testing.T) {
	p := parseRProbe(t)
	entry := thermalEntry(toolreply(t), []string{"csv-stdout"}, &Reply{Format: ReplyCSV, Outputs: csvReplyOutputs})
	out, _, err := p.perform(t, toolRegistry(t, manifestDir(t, entry)), "ProbeOnce")
	if err != nil || runtime.FormatValue(out["ok"]) != "false" {
		t.Fatalf("ProbeOnce over a csv reply = %v, %v; want false", out["ok"], err)
	}
}

// A performance asking for no outputs runs the tool and binds nothing: the exit status
// is data only to an output the action declares.
func TestToolReplyReadsNoRequestedOutputs(t *testing.T) {
	p := parseRProbe(t)
	entry := ToolEntry{ToolName: "Thermal", Executable: toolreply(t), Variables: []string{"mass", "done"},
		Invocation: &Invocation{Args: []string{"csv-stdout"}},
		Reply:      &Reply{Format: ReplyExitCode, Outputs: map[string]*ReplyOutput{"done": {}}}}
	out, _, err := p.perform(t, toolRegistry(t, manifestDir(t, entry)), "SignalOnce")
	if err != nil {
		t.Fatalf("SignalOnce over an exitcode reply: %v", err)
	}
	_, echoed := out["step.mass"]
	if len(out) != 1 || !echoed {
		t.Fatalf("outputs = %+v, want only the step's input echoed", out)
	}
}

// An entry built in code, not read from a manifest, has its reply checked by NewTool: a
// sound one reads the exit status, a faulty one refuses every question with the fault.
func TestNewToolChecksAProgrammaticReply(t *testing.T) {
	p := parseRProbe(t)
	sound := NewTool(ToolEntry{ToolName: "Thermal", Executable: toolreply(t), Variables: []string{"done"},
		Invocation: &Invocation{Args: []string{"exit3"}},
		Reply:      &Reply{Format: ReplyExitCode, Success: []int{0, 3}, Outputs: map[string]*ReplyOutput{"done": {}}}})
	out, _, err := p.perform(t, registered(t, NewRun(), sound), "ProbeOnce")
	if err != nil || runtime.FormatValue(out["ok"]) != "true" {
		t.Fatalf("programmatic exitcode reply = %v, %v; want true", out["ok"], err)
	}

	faulty := NewTool(ToolEntry{ToolName: "Thermal", Executable: toolreply(t), Variables: []string{"done"},
		Reply: &Reply{Format: "yaml", Outputs: map[string]*ReplyOutput{"done": {}}}})
	if _, err := faulty.Process(); !errors.Is(err, ErrManifest) {
		t.Errorf("process %v, want the ManifestError, so listings show the tool unavailable", err)
	}
	question := Question{Kind: Compute, Compute: &ComputeAsk{Call: &runtime.ToolCall{ToolName: "Thermal"}}}
	if c := faulty.Covers(nil, question); c.Covered || !errors.Is(c.Refusal, ErrManifest) {
		t.Errorf("%+v, want the refusal a ManifestError naming tool:Thermal", c)
	}
}

// keysOf is the output names a reply declares, for a manifest's variables.
func keysOf(outputs map[string]*ReplyOutput) []string {
	var names []string
	for name := range outputs {
		names = append(names, name)
	}
	return names
}

// The bounds hold through a non-object reply: a tool that never answers is a timeout.
func TestToolReplyTimesOut(t *testing.T) {
	p := parseRProbe(t)
	t.Setenv(ToolTimeoutEnv, "200ms")
	entry := thermalEntry(toolreply(t), []string{"sleep"}, &Reply{Format: ReplyCSV, Outputs: csvReplyOutputs})
	_, _, err := p.perform(t, toolRegistry(t, manifestDir(t, entry)), "Once")
	var fault *runtime.ToolError
	if !errors.As(err, &fault) || fault.Kind != runtime.ToolTimeout {
		t.Fatalf("perform = %v, want a timeout", err)
	}
}

// A file source the tool did not write is malformed, the file named relative to
// {outputDir}.
func TestToolReplyMissingFile(t *testing.T) {
	p := parseRProbe(t)
	entry := thermalEntry(toolreply(t), []string{"json-file", "{outputDir}"}, &Reply{
		Format: ReplyJSON, Source: "file:{outputDir}/missing.json",
		Outputs: map[string]*ReplyOutput{"T_max": {Path: jsonPath("/T_max")}}})
	_, _, err := p.perform(t, toolRegistry(t, manifestDir(t, entry)), "Once")
	var fault *runtime.ToolError
	if !errors.As(err, &fault) || fault.Kind != runtime.ToolMalformed || !strings.Contains(err.Error(), "wrote no file missing.json") {
		t.Fatalf("perform = %v, want malformed naming the missing file", err)
	}
}

// Two performances of equal inputs whose CSV answers differ set the divergence note on
// the parsed values, as under the object protocol.
func TestToolReplyReportsDivergence(t *testing.T) {
	p := parseRProbe(t)
	t.Setenv(ToolEnvPassthroughEnv, "TOOLREPLY_COUNTER")
	t.Setenv("TOOLREPLY_COUNTER", filepath.Join(t.TempDir(), "count"))
	entry := thermalEntry(toolreply(t), []string{"diverge"}, &Reply{Format: ReplyCSV, Outputs: csvReplyOutputs})
	out, ctx, err := p.perform(t, toolRegistry(t, manifestDir(t, entry)), "Twice")
	if err != nil {
		t.Fatalf("perform: %v", err)
	}
	if got := runtime.FormatValue(out["T1"]); got != "340.0 [SI::K]" {
		t.Errorf("T1 = %s", got)
	}
	if got := runtime.FormatValue(out["T2"]); got != "341.0 [SI::K]" {
		t.Errorf("T2 = %s", got)
	}
	notes := ctx.Notes()
	if len(notes) != 1 {
		t.Fatalf("notes %v, want the one divergence", notes)
	}
	if d, ok := notes[0].(runtime.ToolDivergence); !ok || d.Tool != "Thermal" {
		t.Fatalf("note %v, want Thermal diverging", notes[0])
	}
}
