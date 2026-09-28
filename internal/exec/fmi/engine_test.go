package fmi

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
	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
	"github.com/Open-MBEE/OpenSysML/tests/testutil/gobuild"
)

var (
	runnerOnce sync.Once
	runnerPath string
	runnerErr  error
)

// runner builds the stand-in FMI runner once per test binary and returns its path.
func runner(t *testing.T) string {
	t.Helper()
	runnerOnce.Do(func() {
		dir, err := os.MkdirTemp("", "fmirunner")
		if err != nil {
			runnerErr = err
			return
		}
		runnerPath = filepath.Join(dir, "fmirunner")
		build := exec.Command("go", gobuild.Args(runnerPath)...)
		build.Dir = filepath.Join("testdata", "fmirunner")
		if out, err := build.CombinedOutput(); err != nil {
			runnerErr = fmt.Errorf("go build: %v\n%s", err, out)
		}
	})
	if runnerErr != nil {
		t.Fatalf("building the stand-in runner: %v", runnerErr)
	}
	return runnerPath
}

// scratch writes a file the stand-in reads from TMPDIR, and returns its path.
func scratch(t *testing.T, name, contents string) string {
	t.Helper()
	path := filepath.Join(os.TempDir(), "fmirunner-"+name)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(path) })
	return path
}

// engineFor registers the stand-in runner as the engine's process.
func engineFor(t *testing.T) *engine {
	t.Helper()
	return New(func() (string, error) { return runner(t), nil }).(*engine)
}

// driver is the model one test's calc def comes from: the same shape the
// importer writes, with %s where the FMU's file URI belongs.
const driver = `package Drive {
	private import AnalysisTooling::*;
	private import ScalarValues::*;
	private import ISQ::*;

	calc def BB {
		metadata ToolExecution { toolName = "fmi"; uri = "%s"; }
		in g : Real = -9.81 { @ToolVariable { name = "g"; } }
		in e : Real = 0.7 { @ToolVariable { name = "e"; } }
		in startTime : Real = 0.0 { @ToolVariable { name = "fmi:startTime"; } }
		in stopTime : Real = 3.0 { @ToolVariable { name = "fmi:stopTime"; } }
		in stepSize : Real = 0.01 { @ToolVariable { name = "fmi:stepSize"; } }
		return h : LengthValue { @ToolVariable { name = "h"; } }
		out v : SpeedValue { @ToolVariable { name = "v"; } }
	}

	calc bb : BB { }
}`

// probe is the driver's index and package scope.
type probe struct {
	idx *symbols.Index
	pkg *symbols.Scope
}

func parseProbe(t *testing.T, text string) *probe {
	t.Helper()
	idx := libs.NewModelIndex()
	name := filepath.Join(t.TempDir(), "drive.sysml")
	p := parser.New(source.New(name, []byte(text)))
	file := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse: %v", p.Diagnostics)
	}
	idx.AddDocument(name, file)
	idx.ExpandWildcardImports()
	pkg, ok := idx.DocumentRoot(name).LookupLocal("Drive")
	if !ok || pkg.Scope == nil {
		t.Fatal("Drive package not indexed")
	}
	return &probe{idx: idx, pkg: pkg.Scope}
}

// context builds a runtime over the probe's model, with the given registry's
// tool runner attached.
func (p *probe) context(r *analysis.Registry) *runtime.Context {
	resolver := resolve.New(p.idx)
	model := runtime.NewModel(passes.NewTypedModel(resolver), resolver)
	model.SetExpressionParser(parser.ParseOneExpression)
	ctx := runtime.NewContext(model, 10000)
	ctx.SetToolRunner(r.ToolRunner(context.Background(), ctx, analysis.Budget{}, analysis.Only("tool:fmi")))
	return ctx
}

func (p *probe) invoke(t *testing.T, r *analysis.Registry) (runtime.Value, error) {
	t.Helper()
	sym, ok := p.pkg.LookupLocal("BB")
	if !ok {
		t.Fatal("BB not indexed")
	}
	return p.context(r).InvokeCalc(sym, nil, p.pkg)
}

// registryWith is a fresh registry holding only the engine given.
func registryWith(t *testing.T, e analysis.Engine) *analysis.Registry {
	t.Helper()
	r := analysis.NewRegistry()
	if err := r.Register(e); err != nil {
		t.Fatal(err)
	}
	return r
}

func ballFMU(t *testing.T) string {
	return writeFMU(t, "ball.fmu", fixtureXML(t, "bouncingball-2.0.xml"), "x86_64-linux")
}

func fileURI(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return "file://" + filepath.ToSlash(abs)
}

// TestEngineRunDrivesTheRunner is the whole path end to end: a calc computed by
// the runner, the request asserted byte-for-byte, the reply bound to the calc's
// outputs.
func TestEngineRunDrivesTheRunner(t *testing.T) {
	fmu := ballFMU(t)
	uri := fileURI(fmu)
	scratch(t, "mode.txt", "answer")
	scratch(t, "reply.json", `{"protocol": 1, "time": 3.0, "outputs": {"h": 0.0123, "v": -0.15}}`)
	p := parseProbe(t, fmt.Sprintf(driver, uri))
	e := engineFor(t)
	result, err := p.invoke(t, registryWith(t, e))
	if err != nil {
		t.Fatalf("InvokeCalc: %v", err)
	}
	if got := runtime.FormatValue(result); got != "0.0123 [SI::m]" {
		t.Fatalf("result = %s, want 0.0123 [SI::m]", got)
	}
	request, err := os.ReadFile(filepath.Join(os.TempDir(), "fmirunner-request.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf(`{"protocol":1,"fmu":%q,"interface":"coSimulation","experiment":{"startTime":0,"stopTime":3,"stepSize":0.01,"tolerance":0.0001},"start":{"e":0.7,"g":-9.81},"outputs":["h","v"]}`, fmu)
	if strings.TrimSpace(string(request)) != want {
		t.Fatalf("request =\n%s\nwant\n%s", request, want)
	}
}

// TestEngineCoversRefusals walks the Covers order: kind, tool name, runner,
// uri, the variables' direction.
func TestEngineCoversRefusals(t *testing.T) {
	e := engineFor(t)
	q := analysis.Question{Kind: analysis.Evaluate}
	if cov := e.Covers(nil, q); cov.Covered {
		t.Fatal("an evaluate question should be refused")
	}
	fmu := ballFMU(t)
	p := parseProbe(t, fmt.Sprintf(driver, fileURI(fmu)))
	sym, _ := p.pkg.LookupLocal("BB")

	// The runner is absent: a look that fails refuses every compute question.
	absent := New(func() (string, error) { return "", fmt.Errorf("no runner") })
	call := &runtime.ToolCall{Action: sym, ToolName: "fmi", URI: fileURI(fmu)}
	q = analysis.Question{Kind: analysis.Compute, Compute: &analysis.ComputeAsk{Call: call}}
	if cov := absent.Covers(nil, q); cov.Covered {
		t.Fatal("no runner should refuse")
	}

	call.ToolName = "other"
	if cov := e.Covers(nil, q); cov.Covered {
		t.Fatal("a call naming another tool should be refused")
	}
	call.ToolName = "fmi"

	call.URI = "missing-variable.fmu"
	if cov := e.Covers(nil, q); cov.Covered {
		t.Fatal("an unreadable FMU should be refused")
	}
	call.URI = fileURI(fmu)

	if cov := e.Covers(nil, q); !cov.Covered {
		t.Fatalf("the well-formed call should be covered, refused with %v", cov.Refusal)
	}
}

// TestVariableDirectionErrors is the misnamed/misdirected variable refusal.
func TestVariableDirectionErrors(t *testing.T) {
	fmu := ballFMU(t)
	uri := fileURI(fmu)
	scratch(t, "mode.txt", "answer")
	scratch(t, "reply.json", `{"protocol": 1, "time": 0.0, "outputs": {}}`)
	e := engineFor(t)

	var cases = []struct {
		name     string
		inputs   []string
		outputs  []string
		wantText string
	}{
		{"unknown input", []string{"gg"}, nil, `has no variable "gg"`},
		{"output as input", []string{"h"}, nil, "not an input"},
		{"unknown output", nil, []string{"w"}, `has no variable "w"`},
		{"input as output", nil, []string{"g"}, "not an output"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var b strings.Builder
			b.WriteString(`package Drive {
	private import AnalysisTooling::*;
	private import ScalarValues::*;
	calc def BB {
		metadata ToolExecution { toolName = "fmi"; uri = "` + uri + `"; }
`)
			for _, name := range tc.inputs {
				fmt.Fprintf(&b, "\t\tin %s : Real = 1.0 { @ToolVariable { name = %q; } }\n", sanitizeIdent(name), name)
			}
			b.WriteString("\t\tin startTime : Real = 0.0 { @ToolVariable { name = \"fmi:startTime\"; } }\n")
			for _, name := range tc.outputs {
				fmt.Fprintf(&b, "\t\tout %s : Real { @ToolVariable { name = %q; } }\n", sanitizeIdent(name), name)
			}
			b.WriteString("\t\treturn h : Real { @ToolVariable { name = \"h\"; } }\n\t}\n}\n")
			p := parseProbe(t, b.String())
			sym, _ := p.pkg.LookupLocal("BB")
			call := &runtime.ToolCall{Action: sym, ToolName: "fmi", URI: uri}
			for _, name := range tc.inputs {
				call.Inputs = append(call.Inputs, runtime.ToolInput{Variable: name, Parameter: name})
			}
			for _, name := range append(append([]string(nil), tc.outputs...), "h") {
				call.Outputs = append(call.Outputs, runtime.ToolOutput{Variable: name, Parameter: name})
			}
			q := analysis.Question{Kind: analysis.Compute, Compute: &analysis.ComputeAsk{Call: call}}
			cov := e.Covers(nil, q)
			if cov.Covered {
				t.Fatalf("call with %s should be refused", tc.name)
			}
			var verr *VariableError
			if !errors.As(cov.Refusal, &verr) || !strings.Contains(cov.Refusal.Error(), tc.wantText) {
				t.Fatalf("refusal = %v, want a VariableError naming %q", cov.Refusal, tc.wantText)
			}
		})
	}
}

func sanitizeIdent(name string) string {
	name = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' {
			return r
		}
		return '_'
	}, name)
	if name == "" {
		return "x"
	}
	return name
}

// TestProcessAbsentWhenUnset is the operator-grant refusal: no runner variable,
// no process.
func TestProcessAbsentWhenUnset(t *testing.T) {
	t.Setenv(RunnerEnv, "")
	e := New(nil)
	_, err := e.Process()
	var absent *analysis.ProcessAbsentError
	if !errors.As(err, &absent) {
		t.Fatalf("Process = %v, want ProcessAbsentError", err)
	}
	if !strings.Contains(err.Error(), RunnerEnv) {
		t.Fatalf("Process error %q should name %s", err, RunnerEnv)
	}
}

// TestProcessFindsTheRunner is the env lookup path used in production.
func TestProcessFindsTheRunner(t *testing.T) {
	t.Setenv(RunnerEnv, runner(t))
	e := New(nil)
	got, err := e.Process()
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if !strings.Contains(got, runner(t)) {
		t.Fatalf("Process = %q, want the runner path", got)
	}
}

// TestRunnerFaults is the reply's error member and the process's own failures.
func TestRunnerFaults(t *testing.T) {
	fmu := ballFMU(t)
	p := parseProbe(t, fmt.Sprintf(driver, fileURI(fmu)))
	cases := []struct {
		mode     string
		wantText string
	}{
		{"error:the FMU did not converge", "did not converge"},
		{"exit:license server unreachable", "license server unreachable"},
	}
	for _, tc := range cases {
		t.Run(tc.mode, func(t *testing.T) {
			scratch(t, "mode.txt", tc.mode)
			_, err := p.invoke(t, registryWith(t, engineFor(t)))
			if err == nil || !strings.Contains(err.Error(), tc.wantText) {
				t.Fatalf("InvokeCalc = %v, want an error naming %q", err, tc.wantText)
			}
		})
	}
}

// TestRunnerErrorIsTyped: the reply's error member answers a *RunnerError.
func TestRunnerErrorIsTyped(t *testing.T) {
	fmu := ballFMU(t)
	scratch(t, "mode.txt", "error:blew up")
	p := parseProbe(t, fmt.Sprintf(driver, fileURI(fmu)))
	_, err := p.invoke(t, registryWith(t, engineFor(t)))
	var runnerErr *RunnerError
	if !errors.As(err, &runnerErr) {
		t.Fatalf("InvokeCalc = %v, want RunnerError", err)
	}
}

// TestProtocolBreaks: a reply that is not the protocol fails the performance.
func TestProtocolBreaks(t *testing.T) {
	fmu := ballFMU(t)
	p := parseProbe(t, fmt.Sprintf(driver, fileURI(fmu)))
	scratch(t, "mode.txt", "answer")
	for name, reply := range map[string]string{
		"not json":       `hello`,
		"wrong protocol": `{"protocol": 9, "outputs": {"h": 1.0, "v": 0.0}}`,
		"missing output": `{"protocol": 1, "time": 3.0, "outputs": {"h": 1.0}}`,
		"no time":        `{"protocol": 1, "outputs": {"h": 1.0, "v": 0.0}}`,
		"wrong type":     `{"protocol": 1, "time": 3.0, "outputs": {"h": "high", "v": 0.0}}`,
	} {
		t.Run(name, func(t *testing.T) {
			scratch(t, "reply.json", reply)
			_, err := p.invoke(t, registryWith(t, engineFor(t)))
			var fault *runtime.ToolError
			if !errors.As(err, &fault) || fault.Kind != runtime.ToolMalformed {
				t.Fatalf("InvokeCalc = %v, want a ToolError of kind malformed", err)
			}
		})
	}
}

// TestEngineRefusesANullOutput: null answered for a String variable is a
// protocol break, not an empty string.
func TestEngineRefusesANullOutput(t *testing.T) {
	fmu := writeFMU(t, "strings.fmu", `<?xml version="1.0" encoding="UTF-8"?>
<fmiModelDescription fmiVersion="2.0" modelName="Strings" guid="{strings}">
  <CoSimulation modelIdentifier="Strings"/>
  <ModelVariables>
    <ScalarVariable name="s" valueReference="1" causality="output"><String/></ScalarVariable>
  </ModelVariables>
</fmiModelDescription>`, "x86_64-linux")
	scratch(t, "mode.txt", "answer")
	scratch(t, "reply.json", `{"protocol": 1, "time": 1.0, "outputs": {"s": null}}`)
	p := parseProbe(t, fmt.Sprintf(`package Drive {
	private import AnalysisTooling::*;
	private import ScalarValues::*;

	calc def S {
		metadata ToolExecution { toolName = "fmi"; uri = "%s"; }
		return s : String { @ToolVariable { name = "s"; } }
	}

	calc s : S { }
}`, fileURI(fmu)))
	_, err := invokeCalcNamed(t, p, "S", engineFor(t))
	var fault *runtime.ToolError
	if !errors.As(err, &fault) || fault.Kind != runtime.ToolMalformed {
		t.Fatalf("InvokeCalc = %v, want a ToolError of kind malformed", err)
	}
}

// TestEngineConvertsExperimentTimes: a measured experiment time reaches the FMU
// in seconds, and a measured tolerance is refused — it is a plain number.
func TestEngineConvertsExperimentTimes(t *testing.T) {
	fmu := writeFMU(t, "units.fmu", unitsFMU, "x86_64-linux")
	uri := fileURI(fmu)
	scratch(t, "mode.txt", "answer")
	scratch(t, "reply.json", `{"protocol": 1, "time": 60.0, "outputs": {"h": 2.0}}`)
	p := parseProbe(t, fmt.Sprintf(unitsDriver, uri,
		`in stopTime : DurationValue = 1 [min] { @ToolVariable { name = "fmi:stopTime"; } }`))
	if _, err := invokeCalcNamed(t, p, "Conv", engineFor(t)); err != nil {
		t.Fatalf("InvokeCalc: %v", err)
	}
	request, err := os.ReadFile(filepath.Join(os.TempDir(), "fmirunner-request.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf(`{"protocol":1,"fmu":%q,"interface":"coSimulation","experiment":{"startTime":0,"stopTime":60},"start":{},"outputs":["h"]}`, fmu)
	if strings.TrimSpace(string(request)) != want {
		t.Fatalf("request =\n%s\nwant\n%s", request, want)
	}
	p = parseProbe(t, fmt.Sprintf(unitsDriver, uri,
		`in tolerance : LengthValue = 0.5 [m] { @ToolVariable { name = "fmi:tolerance"; } }`))
	_, err = invokeCalcNamed(t, p, "Conv", engineFor(t))
	var fault *runtime.ToolError
	if !errors.As(err, &fault) || fault.Kind != runtime.ToolUnsentInput {
		t.Fatalf("InvokeCalc = %v, want a ToolError of kind unsent input", err)
	}
}

// TestTimeout: a runner that never answers is cut off.
func TestTimeout(t *testing.T) {
	t.Setenv(analysis.ToolTimeoutEnv, "150ms")
	fmu := ballFMU(t)
	p := parseProbe(t, fmt.Sprintf(driver, fileURI(fmu)))
	scratch(t, "mode.txt", "hang")
	_, err := p.invoke(t, registryWith(t, engineFor(t)))
	var fault *runtime.ToolError
	if !errors.As(err, &fault) || fault.Kind != runtime.ToolTimeout {
		t.Fatalf("InvokeCalc = %v, want a ToolError of kind timeout", err)
	}
}

// unitsFMU describes one measured input and output in metres, and one unitless
// input.
const unitsFMU = `<?xml version="1.0" encoding="UTF-8"?>
<fmiModelDescription fmiVersion="2.0" modelName="Units" guid="{units}">
  <CoSimulation modelIdentifier="Units"/>
  <UnitDefinitions>
    <Unit name="m"><BaseUnit m="1"/></Unit>
  </UnitDefinitions>
  <ModelVariables>
    <ScalarVariable name="dist" valueReference="1" causality="input"><Real start="0.0" unit="m"/></ScalarVariable>
    <ScalarVariable name="h" valueReference="2" causality="output"><Real unit="m"/></ScalarVariable>
    <ScalarVariable name="e" valueReference="3" causality="input"><Real start="0.5"/></ScalarVariable>
  </ModelVariables>
</fmiModelDescription>`

// unitsDriver is the model the conversion tests use: %s where the FMU's file
// URI belongs and %s for the input's declaration line.
const unitsDriver = `package Drive {
	private import AnalysisTooling::*;
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;

	calc def Conv {
		metadata ToolExecution { toolName = "fmi"; uri = "%s"; }
		%s
		return h : LengthValue { @ToolVariable { name = "h"; } }
	}

	calc conv : Conv { }
}`

// TestEngineConvertsMeasuredInputs: an input sent in kilometres reaches the FMU
// in metres, and the reply in metres binds to the ISQ-typed parameter.
func TestEngineConvertsMeasuredInputs(t *testing.T) {
	fmu := writeFMU(t, "units.fmu", unitsFMU, "x86_64-linux")
	uri := fileURI(fmu)
	scratch(t, "mode.txt", "answer")
	scratch(t, "reply.json", `{"protocol": 1, "time": 1.0, "outputs": {"h": 2.0}}`)
	p := parseProbe(t, fmt.Sprintf(unitsDriver, uri,
		`in dist : LengthValue = 5.0 [km] { @ToolVariable { name = "dist"; } }`))
	result, err := invokeCalcNamed(t, p, "Conv", engineFor(t))
	if err != nil {
		t.Fatalf("InvokeCalc: %v", err)
	}
	if got := runtime.FormatValue(result); got != "2.0 [SI::m]" {
		t.Fatalf("result = %s, want 2.0 [SI::m]", got)
	}
	request, err := os.ReadFile(filepath.Join(os.TempDir(), "fmirunner-request.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf(`{"protocol":1,"fmu":%q,"interface":"coSimulation","experiment":{"startTime":0,"stopTime":1},"start":{"dist":5000},"outputs":["h"]}`, fmu)
	if strings.TrimSpace(string(request)) != want {
		t.Fatalf("request =\n%s\nwant\n%s", request, want)
	}
}

// TestEngineRefusesMeasuredInputs: a measured input fails before the runner
// starts when the variable resolves no coherent unit, or its own dimension
// does not match the value's.
func TestEngineRefusesMeasuredInputs(t *testing.T) {
	fmu := writeFMU(t, "units.fmu", unitsFMU, "x86_64-linux")
	uri := fileURI(fmu)
	scratch(t, "mode.txt", "answer")
	scratch(t, "reply.json", `{"protocol": 1, "time": 1.0, "outputs": {"h": 2.0}}`)
	for name, decl := range map[string]string{
		"dimension mismatch": `in dist : MassValue = 5.0 [kg] { @ToolVariable { name = "dist"; } }`,
		"unitless variable":  `in e : LengthValue = 0.5 [m] { @ToolVariable { name = "e"; } }`,
	} {
		t.Run(name, func(t *testing.T) {
			p := parseProbe(t, fmt.Sprintf(unitsDriver, uri, decl))
			_, err := invokeCalcNamed(t, p, "Conv", engineFor(t))
			var fault *runtime.ToolError
			if !errors.As(err, &fault) || fault.Kind != runtime.ToolUnsentInput {
				t.Fatalf("InvokeCalc = %v, want a ToolError of kind unsent input", err)
			}
		})
	}
}

// TestEngineRunsArrays: a one-dimensional array input is sent as a JSON list in
// the variable's unit, and an array reply binds item by item.
func TestEngineRunsArrays(t *testing.T) {
	fmu := writeFMU(t, "arrays.fmu", `<?xml version="1.0" encoding="UTF-8"?>
<fmiModelDescription fmiVersion="3.0" modelName="Arrays" instantiationToken="{arrays}">
  <CoSimulation modelIdentifier="Arrays"/>
  <UnitDefinitions>
    <Unit name="m"><BaseUnit m="1"/></Unit>
  </UnitDefinitions>
  <ModelVariables>
    <Float64 name="u" valueReference="1" causality="input" unit="m" start="1 2"><Dimension start="2"/></Float64>
    <Float64 name="y" valueReference="2" causality="output" unit="m"><Dimension start="2"/></Float64>
  </ModelVariables>
</fmiModelDescription>`, "x86_64-linux")
	uri := fileURI(fmu)
	scratch(t, "mode.txt", "answer")
	scratch(t, "reply.json", `{"protocol": 1, "time": 1.0, "outputs": {"y": [3.0, 4.0]}}`)
	p := parseProbe(t, fmt.Sprintf(`package Drive {
	private import AnalysisTooling::*;
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;

	calc def A {
		metadata ToolExecution { toolName = "fmi"; uri = "%s"; }
		in u : Real[2] = (1.0, 2.0) { @ToolVariable { name = "u"; } }
		return y : Real[2] { @ToolVariable { name = "y"; } }
	}

	calc a : A { }
}`, uri))
	result, err := invokeCalcNamed(t, p, "A", engineFor(t))
	if err != nil {
		t.Fatalf("InvokeCalc: %v", err)
	}
	if got := runtime.FormatValue(result); got != "[3.0, 4.0]" {
		t.Fatalf("result = %s, want [3.0, 4.0]", got)
	}
	request, err := os.ReadFile(filepath.Join(os.TempDir(), "fmirunner-request.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf(`{"protocol":1,"fmu":%q,"interface":"coSimulation","experiment":{"startTime":0,"stopTime":1},"start":{"u":[1,2]},"outputs":["y"]}`, fmu)
	if strings.TrimSpace(string(request)) != want {
		t.Fatalf("request =\n%s\nwant\n%s", request, want)
	}
}

// invokeCalcNamed runs the named calc def of p's model under a registry holding e.
func invokeCalcNamed(t *testing.T, p *probe, name string, e analysis.Engine) (runtime.Value, error) {
	t.Helper()
	sym, ok := p.pkg.LookupLocal(name)
	if !ok {
		t.Fatalf("%s not indexed", name)
	}
	return p.context(registryWith(t, e)).InvokeCalc(sym, nil, p.pkg)
}

// TestEngineRefusesArrayLengthMismatch: a sequence of the wrong length fails
// before the runner starts.
func TestEngineRefusesArrayLengthMismatch(t *testing.T) {
	fmu := writeFMU(t, "arrays.fmu", `<?xml version="1.0" encoding="UTF-8"?>
<fmiModelDescription fmiVersion="3.0" modelName="Arrays" instantiationToken="{arrays}">
  <CoSimulation modelIdentifier="Arrays"/>
  <ModelVariables>
    <Float64 name="u" valueReference="1" causality="input" start="1 2"><Dimension start="2"/></Float64>
    <Float64 name="y" valueReference="2" causality="output"><Dimension start="2"/></Float64>
  </ModelVariables>
</fmiModelDescription>`, "x86_64-linux")
	p := parseProbe(t, fmt.Sprintf(`package Drive {
	private import AnalysisTooling::*;
	private import ScalarValues::*;

	calc def A {
		metadata ToolExecution { toolName = "fmi"; uri = "%s"; }
		in u : Real[3] nonunique = (1.0, 2.0, 3.0) { @ToolVariable { name = "u"; } }
		return y : Real[2] nonunique { @ToolVariable { name = "y"; } }
	}

	calc a : A { }
}`, fileURI(fmu)))
	_, err := invokeCalcNamed(t, p, "A", engineFor(t))
	var fault *runtime.ToolError
	if !errors.As(err, &fault) || fault.Kind != runtime.ToolUnsentInput {
		t.Fatalf("InvokeCalc = %v, want a ToolError of kind unsent input", err)
	}
	if !strings.Contains(fault.Detail, "dimension holds 2") {
		t.Fatalf("refusal = %q, want it to name the sent and expected lengths", fault.Detail)
	}
}
