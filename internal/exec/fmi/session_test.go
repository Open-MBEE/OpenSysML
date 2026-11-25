package fmi

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
)

// decayFMU is the fixture of the known ODE y' = -k*y, y(0) = y0: y0 and k are
// bound once at init, y is the state a step reads back.
func decayFMU(t *testing.T) string {
	return writeFMU(t, "decay.fmu", `<?xml version="1.0" encoding="UTF-8"?>
<fmiModelDescription fmiVersion="2.0" modelName="Decay" guid="{decay}">
  <CoSimulation modelIdentifier="Decay"/>
  <ModelVariables>
    <ScalarVariable name="y0" valueReference="1" causality="input"><Real start="1.0"/></ScalarVariable>
    <ScalarVariable name="k" valueReference="2" causality="parameter"><Real start="1.0"/></ScalarVariable>
    <ScalarVariable name="y" valueReference="3" causality="output"><Real/></ScalarVariable>
  </ModelVariables>
</fmiModelDescription>`, "x86_64-linux")
}

// decayDriver is the model the session's engine seam uses: the same annotated
// calc the importer writes, with the four reserved experiment inputs.
const decayDriver = `package Drive {
	private import AnalysisTooling::*;
	private import ScalarValues::*;

	calc def Decay {
		metadata ToolExecution { toolName = "fmi"; uri = "%s"; }
		in y0 : Real = 1.0 { @ToolVariable { name = "y0"; } }
		in k : Real = 1.0 { @ToolVariable { name = "k"; } }
		in startTime : Real = 0.0 { @ToolVariable { name = "fmi:startTime"; } }
		in stopTime : Real = 3.0 { @ToolVariable { name = "fmi:stopTime"; } }
		return y : Real[1] { @ToolVariable { name = "y"; } }
	}

	calc decay : Decay { }
}`

// steppedDriver adds the step input and one calc usage per time: the in-model
// analysis leg sharing one caller's session.
func steppedDriver(uri string, times []float64) string {
	var b strings.Builder
	b.WriteString("package Drive {\n\tprivate import AnalysisTooling::*;\n\tprivate import ScalarValues::*;\n\n")
	fmt.Fprintf(&b, "\tcalc def Decay {\n\t\tmetadata ToolExecution { toolName = \"fmi\"; uri = %q; }\n", uri)
	b.WriteString("\t\tin stepTime : Real = 0.0 { @ToolVariable { name = \"fmi:stepTime\"; } }\n")
	b.WriteString("\t\tin y0 : Real = 1.0 { @ToolVariable { name = \"y0\"; } }\n")
	b.WriteString("\t\tin k : Real = 1.0 { @ToolVariable { name = \"k\"; } }\n")
	b.WriteString("\t\tin startTime : Real = 0.0 { @ToolVariable { name = \"fmi:startTime\"; } }\n")
	b.WriteString("\t\tin stopTime : Real = 3.0 { @ToolVariable { name = \"fmi:stopTime\"; } }\n")
	b.WriteString("\t\treturn y : Real[1] { @ToolVariable { name = \"y\"; } }\n\t}\n")
	for i, t := range times {
		fmt.Fprintf(&b, "\n\tcalc d%d : Decay { in stepTime = %v; }\n", i, t)
	}
	b.WriteString("}\n")
	return b.String()
}

// wholeDriver is the one-shot model of one whole run to a given stop time.
func wholeDriver(uri string, stop float64) string {
	text := strings.Replace(decayDriver, "in stopTime : Real = 3.0",
		fmt.Sprintf("in stopTime : Real = %v", stop), 1)
	return fmt.Sprintf(text, uri)
}

var stepTimes = []float64{0.5, 1.0, 2.0, 3.0}

func real(f float64) runtime.ToolValue {
	return runtime.ToolValue{Value: semantics.Value{Kind: semantics.ValReal, Real: f}}
}

// numberOf reads the plain Real a bound output carries; a non-number is NaN so
// the caller's tolerance check fails instead of reading an unrelated field.
func numberOf(v runtime.Value) float64 {
	if v.Kind != runtime.ValConst || v.Const.Kind != semantics.ValReal {
		return math.NaN()
	}
	return v.Const.Real
}

// decayCall builds the init call; a later one can change stopTime for the
// whole-run comparison without changing the session key.
func decayCall(t *testing.T, p *probe, uri string, stop float64) *runtime.ToolCall {
	sym, ok := p.pkg.LookupLocal("Decay")
	if !ok {
		t.Fatal("Decay not indexed")
	}
	return &runtime.ToolCall{Action: sym, ToolName: "fmi", URI: uri,
		Inputs: []runtime.ToolInput{
			{Variable: "y0", Parameter: "y0", Value: real(1)},
			{Variable: "k", Parameter: "k", Value: real(1)},
			{Variable: "fmi:startTime", Parameter: "startTime", Value: real(0)},
			{Variable: "fmi:stopTime", Parameter: "stopTime", Value: real(stop)},
		},
		Outputs: []runtime.ToolOutput{{Variable: "y", Parameter: "y"}}}
}

// testManager is the session seam with a fixed short timeout and output bound.
func testManager(t *testing.T, config SessionConfig) *SessionManager {
	t.Helper()
	m := newSessionManager(func() (string, error) { return runner(t), nil },
		func() time.Duration { return 20 * time.Second }, func() int { return 1 << 20 }, config)
	t.Cleanup(func() { _ = m.Close() })
	return m
}

// The session tests give their runner a private scratch directory so parallel
// test binaries cannot share mode, reply or counting files.
func scratchPath(name string) string {
	dir := strings.TrimSpace(os.Getenv("FMIRUNNER_SCRATCH"))
	if dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "fmirunner-"+name)
}

func writeScratch(t *testing.T, name, contents string) {
	t.Helper()
	if err := os.WriteFile(scratchPath(name), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

// count reads one of the stand-in's counting files.
func count(t *testing.T, name string) int {
	t.Helper()
	data, err := os.ReadFile(scratchPath(name))
	if err != nil {
		t.Fatal(err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatalf("count %q: %v", name, err)
	}
	return n
}

// resetModes gives one test a private scratch directory and a known state.
func resetModes(t *testing.T, mode string) {
	t.Helper()
	t.Setenv("FMIRUNNER_SCRATCH", t.TempDir())
	t.Setenv(analysis.ToolEnvPassthroughEnv, "FMIRUNNER_SCRATCH")
	writeScratch(t, "mode.txt", mode)
	writeScratch(t, "session-fault.txt", "")
	writeScratch(t, "start-count.txt", "0")
	writeScratch(t, "init-count.txt", "0")
}

// One initialize plus N steps answer the same values as N whole runs, but the
// runner starts once instead of N times.
func TestSessionStepsMatchWholeRuns(t *testing.T) {
	fmu := decayFMU(t)
	uri := fileURI(fmu)
	p := parseProbe(t, fmt.Sprintf(decayDriver, uri))
	call := decayCall(t, p, uri, 3)

	resetModes(t, "session")
	m := testManager(t, SessionConfig{})
	ctx := context.Background()
	s, err := m.Open(ctx, "alice", call)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got := s.Time(); got != 0 {
		t.Fatalf("initial time = %v, want 0", got)
	}
	for _, at := range stepTimes {
		if err := s.Step(ctx, at); err != nil {
			t.Fatalf("Step(%v): %v", at, err)
		}
		values, err := s.Read(ctx, []string{"y"})
		if err != nil {
			t.Fatalf("Read at %v: %v", at, err)
		}
		got := values["y"].Value.Real
		want := math.Exp(-at)
		if math.Abs(got-want) > 1e-9 {
			t.Errorf("session y(%v) = %v, want %v", at, got, want)
		}
	}
	if got := count(t, "start-count.txt"); got != 1 {
		t.Fatalf("the session started the runner %d times, want 1", got)
	}
	if got := count(t, "init-count.txt"); got != 1 {
		t.Fatalf("the session initialized %d times, want 1", got)
	}

	// The whole-run path uses the unchanged one-shot engine, one process each.
	resetModes(t, "ode")
	e := engineFor(t)
	for _, at := range stepTimes {
		whole := parseProbe(t, wholeDriver(uri, at))
		result, err := invokeCalcNamed(t, whole, "Decay", e)
		if err != nil {
			t.Fatalf("whole run at %v: %v", at, err)
		}
		if got := numberOf(result); math.Abs(got-math.Exp(-at)) > 1e-9 {
			t.Errorf("whole run y(%v) = %v, want %v", at, got, math.Exp(-at))
		}
	}
	if got := count(t, "start-count.txt"); got != len(stepTimes) {
		t.Fatalf("the whole runs started the runner %d times, want %d", got, len(stepTimes))
	}
}

// The additive fmi/2 wire shape: init, step, read. The fmi/1 request bytes are
// untouched.
func TestSessionProtocolCommands(t *testing.T) {
	fmu := decayFMU(t)
	uri := fileURI(fmu)
	p := parseProbe(t, fmt.Sprintf(decayDriver, uri))
	resetModes(t, "session")
	m := testManager(t, SessionConfig{})
	ctx := context.Background()
	s, err := m.Open(ctx, "alice", decayCall(t, p, uri, 3))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := s.Step(ctx, 1.5); err != nil {
		t.Fatalf("Step: %v", err)
	}
	if _, err := s.Read(ctx, []string{"y", "fmi:time"}); err != nil {
		t.Fatalf("Read: %v", err)
	}
	data, err := os.ReadFile(scratchPath("commands.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("commands =\n%s\nwant init, step and read", data)
	}
	wantInit := fmt.Sprintf(`{"protocol":2,"action":"init","fmu":%q,"interface":"coSimulation","experiment":{"startTime":0,"stopTime":3},"start":{"k":1,"y0":1}}`, fmu)
	if lines[0] != wantInit {
		t.Errorf("init =\n%s\nwant\n%s", lines[0], wantInit)
	}
	if want := `{"protocol":2,"action":"step","time":1.5}`; lines[1] != want {
		t.Errorf("step =\n%s\nwant\n%s", lines[1], want)
	}
	if want := `{"protocol":2,"action":"read","outputs":["y"]}`; lines[2] != want {
		t.Errorf("read =\n%s\nwant\n%s", lines[2], want)
	}
}

// TestSessionIsolatesCallers proves that two principals driving the same FMU
// keep separate state, and that re-opening one returns the same session.
func TestSessionIsolatesCallers(t *testing.T) {
	fmu := decayFMU(t)
	uri := fileURI(fmu)
	p := parseProbe(t, fmt.Sprintf(decayDriver, uri))
	call := decayCall(t, p, uri, 3)
	resetModes(t, "session")
	m := testManager(t, SessionConfig{})
	ctx := context.Background()
	alice, err := m.Open(ctx, "alice", call)
	if err != nil {
		t.Fatalf("Open alice: %v", err)
	}
	bob, err := m.Open(ctx, "bob", call)
	if err != nil {
		t.Fatalf("Open bob: %v", err)
	}
	again, err := m.Open(ctx, "alice", call)
	if err != nil {
		t.Fatalf("Open alice again: %v", err)
	}
	if again != alice {
		t.Fatal("re-opening alice did not return her session")
	}
	if err := alice.Step(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if err := bob.Step(ctx, 2.5); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		s    *Session
		want float64
	}{
		{alice, math.Exp(-1)},
		{bob, math.Exp(-2.5)},
	} {
		values, err := tc.s.Read(ctx, []string{"y"})
		if err != nil {
			t.Fatal(err)
		}
		if got := values["y"].Value.Real; math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("caller %s y = %v, want %v", tc.s.Caller(), got, tc.want)
		}
	}
	if got := count(t, "start-count.txt"); got != 2 {
		t.Fatalf("two callers started %d runners, want 2", got)
	}
	if got := alice.Time(); got != 1 {
		t.Errorf("alice's time = %v, want 1", got)
	}
	if got := bob.Time(); got != 2.5 {
		t.Errorf("bob's time = %v, want 2.5", got)
	}
}

// TestSessionBounds covers the manager's three limits: sessions open, steps
// taken, and idle lifetime. Every refusal is a typed error.
func TestSessionBounds(t *testing.T) {
	fmu := decayFMU(t)
	uri := fileURI(fmu)
	p := parseProbe(t, fmt.Sprintf(decayDriver, uri))
	call := decayCall(t, p, uri, 3)
	ctx := context.Background()

	resetModes(t, "session")
	m := testManager(t, SessionConfig{MaxSessions: 1, MaxSteps: 2})
	if _, err := m.Open(ctx, "alice", call); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Open(ctx, "bob", call); !errors.Is(err, ErrSessionLimit) {
		t.Fatalf("the second caller: %v, want a session limit error", err)
	}
	s, err := m.Open(ctx, "alice", call)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Step(ctx, 0.5); err != nil {
		t.Fatal(err)
	}
	if err := s.Step(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if err := s.Step(ctx, 1.5); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("the third step: %v, want a session expiry error", err)
	}

	resetModes(t, "session")
	idle := testManager(t, SessionConfig{Idle: 60 * time.Millisecond})
	short, err := idle.Open(ctx, "alice", call)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if err := short.Step(ctx, 0.5); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("a step on an idle session: %v, want a session expiry error", err)
	}
}

// TestSessionRefusesABackwardStep is the failure mode of stepping outside the
// experiment: a typed StepError, never a silent rewind or a runner call.
func TestSessionRefusesABackwardStep(t *testing.T) {
	fmu := decayFMU(t)
	uri := fileURI(fmu)
	p := parseProbe(t, fmt.Sprintf(decayDriver, uri))
	resetModes(t, "session")
	m := testManager(t, SessionConfig{})
	s, err := m.Open(context.Background(), "alice", decayCall(t, p, uri, 1))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Step(context.Background(), 0.5); err != nil {
		t.Fatal(err)
	}
	err = s.Step(context.Background(), 0.25)
	var step *StepError
	if !errors.As(err, &step) || !strings.Contains(err.Error(), "backwards") {
		t.Fatalf("a backward step: %v, want a StepError naming backwards", err)
	}
	err = s.Step(context.Background(), 2)
	if !errors.As(err, &step) || !strings.Contains(err.Error(), "stops at 1") {
		t.Fatalf("a step past stopTime: %v, want a StepError naming the stop", err)
	}
	if got := count(t, "start-count.txt"); got != 1 {
		t.Fatalf("refused steps started %d runners, want 1", got)
	}
}

// TestSessionRefusesAnUnknownOutput is the negative control of Read: a name
// the FMU does not declare is refused before the runner is asked.
func TestSessionRefusesAnUnknownOutput(t *testing.T) {
	fmu := decayFMU(t)
	uri := fileURI(fmu)
	p := parseProbe(t, fmt.Sprintf(decayDriver, uri))
	resetModes(t, "session")
	m := testManager(t, SessionConfig{})
	s, err := m.Open(context.Background(), "alice", decayCall(t, p, uri, 3))
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Read(context.Background(), []string{"missing"})
	var variable *VariableError
	if !errors.As(err, &variable) || !errors.Is(err, ErrVariable) {
		t.Fatalf("Read of an unknown variable: %v, want a VariableError", err)
	}
	_, err = s.Read(context.Background(), []string{"k"})
	if !errors.As(err, &variable) || !strings.Contains(err.Error(), "not an output") {
		t.Fatalf("Read of an input: %v, want a VariableError naming direction", err)
	}
}

// The runner's own error, a process exit and a malformed answer are typed and
// named, never mistaken for a value.
func TestSessionRunnerFaults(t *testing.T) {
	fmu := decayFMU(t)
	uri := fileURI(fmu)
	p := parseProbe(t, fmt.Sprintf(decayDriver, uri))
	ctx := context.Background()
	for name, tc := range map[string]struct {
		fault   string
		wantErr error
		text    string
	}{
		"runner error": {"error:the FMU did not converge", nil, "did not converge"},
		"process exit": {"exit:license server unreachable", ErrSession, "license server unreachable"},
		"not json":     {"garbage", ErrSession, "JSON"},
	} {
		t.Run(name, func(t *testing.T) {
			resetModes(t, "session")
			writeScratch(t, "session-fault.txt", tc.fault)
			m := testManager(t, SessionConfig{})
			_, err := m.Open(ctx, "alice", decayCall(t, p, uri, 3))
			if err == nil || !strings.Contains(err.Error(), tc.text) {
				t.Fatalf("Open: %v, want an error naming %q", err, tc.text)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("Open = %v, want it to match %v", err, tc.wantErr)
			}
			if tc.fault == "error:the FMU did not converge" {
				var runnerErr *RunnerError
				if !errors.As(err, &runnerErr) {
					t.Fatalf("Open = %v, want a RunnerError", err)
				}
			}
		})
	}
}

// A runner that exits successfully before answering fails with a typed error,
// not a panic on the nil wait error of a clean exit.
func TestSessionCleanExitIsTyped(t *testing.T) {
	fmu := decayFMU(t)
	uri := fileURI(fmu)
	p := parseProbe(t, fmt.Sprintf(decayDriver, uri))
	resetModes(t, "session-exit-clean")
	m := testManager(t, SessionConfig{})

	_, err := m.Open(context.Background(), "alice", decayCall(t, p, uri, 3))
	if err == nil {
		t.Fatal("Open accepted a runner that exited without answering")
	}
	var sessionErr *SessionError
	var toolErr *runtime.ToolError
	if !errors.As(err, &sessionErr) || !errors.As(err, &toolErr) {
		t.Fatalf("Open = %v, want a SessionError wrapping a ToolError", err)
	}
	if !errors.Is(err, ErrSession) || !errors.Is(err, runtime.ErrTool) {
		t.Fatalf("Open = %v, want it to match the session and tool sentinels", err)
	}
	if toolErr.Kind != runtime.ToolProcessFailed {
		t.Fatalf("Open = %v, want a ToolError of kind process failed", err)
	}
	if !strings.Contains(err.Error(), "the session runner exited before answering") {
		t.Fatalf("Open = %v, want it to name the clean exit", err)
	}
}

// An init bigger than a pipe's capacity still misses its deadline when the
// runner never reads stdin: the write and answer share one cancellation.
func TestInitDeadlineWithBlockedWrite(t *testing.T) {
	fmu := writeFMU(t, "payload.fmu", `<?xml version="1.0" encoding="UTF-8"?>
<fmiModelDescription fmiVersion="2.0" modelName="Payload" guid="{payload}">
  <CoSimulation modelIdentifier="Payload"/>
  <ModelVariables>
    <ScalarVariable name="payload" valueReference="1" causality="input"><String/></ScalarVariable>
    <ScalarVariable name="y" valueReference="2" causality="output"><Real/></ScalarVariable>
  </ModelVariables>
</fmiModelDescription>`, "x86_64-linux")
	uri := fileURI(fmu)
	resetModes(t, "session-deaf")
	m := newSessionManager(func() (string, error) { return runner(t), nil },
		func() time.Duration { return 250 * time.Millisecond }, func() int { return 1 << 20 }, SessionConfig{})
	t.Cleanup(func() { _ = m.Close() })
	call := &runtime.ToolCall{ToolName: "fmi", URI: uri, Inputs: []runtime.ToolInput{
		{Variable: "payload", Parameter: "payload", Value: runtime.ToolValue{Text: strings.Repeat("x", 2<<20)}},
		{Variable: "fmi:startTime", Parameter: "startTime", Value: real(0)},
		{Variable: "fmi:stopTime", Parameter: "stopTime", Value: real(3)},
	}, Outputs: []runtime.ToolOutput{{Variable: "y", Parameter: "y"}}}

	result := make(chan error, 1)
	go func() { _, err := m.Open(context.Background(), "alice", call); result <- err }()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("Open initialized a runner that never read stdin")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Open = %v, want a deadline error", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the blocked init missed its deadline")
	}
}

// TestSessionStepFaultIsTyped drives a failure after a successful init through
// the fault file, so the step's error is distinguished from an init failure.
func TestSessionStepFaultIsTyped(t *testing.T) {
	fmu := decayFMU(t)
	uri := fileURI(fmu)
	p := parseProbe(t, fmt.Sprintf(decayDriver, uri))
	resetModes(t, "session")
	m := testManager(t, SessionConfig{})
	s, err := m.Open(context.Background(), "alice", decayCall(t, p, uri, 3))
	if err != nil {
		t.Fatal(err)
	}
	writeScratch(t, "session-fault.txt", "error:the solver diverged")
	err = s.Step(context.Background(), 1)
	var sessionErr *SessionError
	var runnerErr *RunnerError
	if !errors.As(err, &sessionErr) || !errors.As(err, &runnerErr) {
		t.Fatalf("Step = %v, want a SessionError wrapping a RunnerError", err)
	}
	if !strings.Contains(err.Error(), "the solver diverged") {
		t.Fatalf("Step = %v, want it to name the runner's fault", err)
	}
}

// sessionContext builds a runtime over the probe's model with a caller's tool
// runner attached, as a surface would for an analysis leg.
func sessionContext(p *probe, runner runtime.ToolRunner) *runtime.Context {
	resolver := resolve.New(p.idx)
	model := runtime.NewModel(passes.NewTypedModel(resolver), resolver)
	model.SetExpressionParser(lower.ExpressionReader(parser.ParseOneExpression, model.Resolver()))
	ctx := runtime.NewContext(model, 10_000)
	ctx.SetToolRunner(runner)
	return ctx
}

// Several calc usages read mid-experiment values through one session-backed
// runner, with the counting runner invoked once.
func TestSessionToolRunnerInModel(t *testing.T) {
	fmu := decayFMU(t)
	uri := fileURI(fmu)
	p := parseProbe(t, steppedDriver(uri, stepTimes))
	resetModes(t, "session")
	m := testManager(t, SessionConfig{})

	delegate := analysis.NewRegistry()
	runner := delegate.ToolRunner(context.Background(), nil, analysis.Budget{}, analysis.Only("tool:fmi"))
	ctx := sessionContext(p, m.ToolRunner("alice", runner))

	for i, at := range stepTimes {
		name := fmt.Sprintf("d%d", i)
		sym, ok := p.pkg.LookupLocal(name)
		if !ok {
			t.Fatalf("%s not indexed", name)
		}
		outputs, err := ctx.CalcUsageOutputs(sym, p.pkg, nil)
		if err != nil {
			t.Fatalf("CalcUsageOutputs(%s): %v", name, err)
		}
		var found bool
		for _, out := range outputs {
			if out.Name == "y" {
				found = true
				got := numberOf(out.Value)
				want := math.Exp(-at)
				if math.Abs(got-want) > 1e-9 {
					t.Errorf("%s y = %v, want %v", name, got, want)
				}
			}
		}
		if !found {
			t.Fatalf("%s answered no y", name)
		}
	}
	if got := count(t, "start-count.txt"); got != 1 {
		t.Fatalf("the in-model leg started the runner %d times, want 1", got)
	}
	if got := count(t, "init-count.txt"); got != 1 {
		t.Fatalf("the in-model leg initialized %d times, want 1", got)
	}
}

// TestSessionToolRunnerDelegatesOneShot proves the adapter additive: a call
// without StepVariable goes to the delegate unchanged, speaking fmi/1.
func TestSessionToolRunnerDelegatesOneShot(t *testing.T) {
	fmu := decayFMU(t)
	uri := fileURI(fmu)
	p := parseProbe(t, fmt.Sprintf(decayDriver, uri))
	resetModes(t, "ode")
	m := testManager(t, SessionConfig{})
	// The model has no step input, so every call goes to the delegate unchanged.
	holder := sessionContext(p, nil)
	delegate := registryWith(t, engineFor(t)).ToolRunner(context.Background(), holder, analysis.Budget{}, analysis.Only("tool:fmi"))
	holder.SetToolRunner(m.ToolRunner("alice", delegate))
	sym, ok := p.pkg.LookupLocal("Decay")
	if !ok {
		t.Fatal("Decay not indexed")
	}
	result, err := holder.InvokeCalc(sym, nil, p.pkg)
	if err != nil {
		t.Fatalf("InvokeCalc: %v", err)
	}
	got := numberOf(result)
	if math.Abs(got-math.Exp(-3)) > 1e-9 {
		t.Fatalf("one-shot y = %v, want %v", got, math.Exp(-3))
	}
	if got := count(t, "start-count.txt"); got != 1 {
		t.Fatalf("the one-shot call started %d runners, want 1", got)
	}
	if got := count(t, "init-count.txt"); got != 0 {
		t.Fatalf("the one-shot call initialized a session %d times, want 0", got)
	}
}

// A stepped call reaching the one-shot engine is refused, never answered with
// the value at stopTime.
func TestOneShotRefusesASteppedCall(t *testing.T) {
	fmu := decayFMU(t)
	uri := fileURI(fmu)
	p := parseProbe(t, fmt.Sprintf(decayDriver, uri))
	call := decayCall(t, p, uri, 3)
	call.Inputs = append(call.Inputs, runtime.ToolInput{Variable: StepVariable, Parameter: "unused", Value: real(0.5)})
	e := engineFor(t)
	q := analysis.Question{Kind: analysis.Compute, Compute: &analysis.ComputeAsk{Call: call}}
	cov := e.Covers(nil, q)
	var required *SessionRequiredError
	if cov.Covered || !errors.As(cov.Refusal, &required) {
		t.Fatalf("Covers = %+v, want a SessionRequiredError", cov)
	}
	_, err := e.Run(context.Background(), nil, q, analysis.Budget{})
	if !errors.As(err, &required) {
		t.Fatalf("Run = %v, want a SessionRequiredError", err)
	}
}

// An unconfigured manager and session refuse with typed errors, not panics.
func TestSessionManagerZeroValue(t *testing.T) {
	var m SessionManager
	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	_, err := m.Open(context.Background(), "alice", nil)
	var sessionErr *SessionError
	if !errors.As(err, &sessionErr) {
		t.Fatalf("Open = %v, want a SessionError", err)
	}
	runner := m.ToolRunner("alice", nil)
	_, err = runner.RunTool(&runtime.ToolCall{ToolName: analysis.ToolEngineName("fmi")})
	if err == nil {
		t.Fatal("a zero manager answered a session call")
	}
	var zero Session
	if err := zero.Step(context.Background(), 1); !errors.Is(err, ErrSession) {
		t.Fatalf("a zero session stepped: %v", err)
	}
	if _, err := zero.Read(context.Background(), []string{"y"}); !errors.Is(err, ErrSession) {
		t.Fatalf("a zero session read: %v", err)
	}
}

// TestSessionReadsTheLastAllowedStep pins the step budget: the last allowed
// step's value is readable, and only the next step is refused.
func TestSessionReadsTheLastAllowedStep(t *testing.T) {
	fmu := decayFMU(t)
	uri := fileURI(fmu)
	p := parseProbe(t, fmt.Sprintf(decayDriver, uri))
	resetModes(t, "session")
	m := testManager(t, SessionConfig{MaxSteps: 1})
	ctx := context.Background()
	s, err := m.Open(ctx, "alice", decayCall(t, p, uri, 3))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := s.Step(ctx, 0.5); err != nil {
		t.Fatalf("the last allowed Step: %v", err)
	}
	values, err := s.Read(ctx, []string{"y"})
	if err != nil {
		t.Fatalf("Read after the last allowed Step: %v", err)
	}
	if got, want := values["y"].Value.Real, math.Exp(-0.5); math.Abs(got-want) > 1e-9 {
		t.Fatalf("y = %v, want %v", got, want)
	}
	if err := s.Step(ctx, 1); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("a step past the budget: %v, want a session expiry error", err)
	}
}

// TestSessionBudgetReadsEveryStep reads after each of several allowed steps,
// including the last, so the budget cannot hide the final value.
func TestSessionBudgetReadsEveryStep(t *testing.T) {
	fmu := decayFMU(t)
	uri := fileURI(fmu)
	p := parseProbe(t, fmt.Sprintf(decayDriver, uri))
	resetModes(t, "session")
	m := testManager(t, SessionConfig{MaxSteps: len(stepTimes)})
	ctx := context.Background()
	s, err := m.Open(ctx, "alice", decayCall(t, p, uri, 3))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	for i, at := range stepTimes {
		if err := s.Step(ctx, at); err != nil {
			t.Fatalf("Step %d: %v", i, err)
		}
		values, err := s.Read(ctx, []string{"y"})
		if err != nil {
			t.Fatalf("Read %d: %v", i, err)
		}
		if got, want := values["y"].Value.Real, math.Exp(-at); math.Abs(got-want) > 1e-9 {
			t.Errorf("y at step %d = %v, want %v", i, got, want)
		}
	}
}

// An idle reclaim races Step's reads of the session's close state and process
// handle. A race detector flags the old unlocked calls here.
func TestSessionJanitorReclaimWhileStepping(t *testing.T) {
	fmu := decayFMU(t)
	uri := fileURI(fmu)
	p := parseProbe(t, fmt.Sprintf(decayDriver, uri))
	resetModes(t, "session")
	m := testManager(t, SessionConfig{Idle: 40 * time.Millisecond})
	ctx := context.Background()
	s, err := m.Open(ctx, "alice", decayCall(t, p, uri, 3))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := s.Step(ctx, 0.2); err != nil {
		t.Fatal(err)
	}
	time.Sleep(45 * time.Millisecond)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 25; i++ {
			m.evict(time.Now())
			time.Sleep(time.Millisecond)
		}
	}()
	at := 0.2
	for i := 0; i < 25; i++ {
		at += 0.01
		err := s.Step(ctx, at)
		if errors.Is(err, ErrSessionExpired) {
			break
		}
		if err != nil {
			t.Fatalf("Step while the janitor reclaimed: %v", err)
		}
		time.Sleep(time.Millisecond)
	}
	<-done
}

// With init outstanding, the janitor and a second caller both proceed: the
// manager's lock is not held while the runner answers.
func TestHungInitDoesNotBlockManager(t *testing.T) {
	fmu := decayFMU(t)
	uri := fileURI(fmu)
	p := parseProbe(t, fmt.Sprintf(decayDriver, uri))
	resetModes(t, "session-hang")
	m := newSessionManager(func() (string, error) { return runner(t), nil },
		func() time.Duration { return 300 * time.Millisecond }, func() int { return 1 << 20 }, SessionConfig{})
	t.Cleanup(func() { _ = m.Close() })
	ctx := context.Background()
	call := decayCall(t, p, uri, 3)

	first := make(chan error, 1)
	go func() { _, err := m.Open(ctx, "alice", call); first <- err }()
	deadline := time.Now().Add(2 * time.Second)
	for {
		m.mu.Lock()
		open := len(m.opening)
		m.mu.Unlock()
		if open > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the open never registered as in flight")
		}
		time.Sleep(time.Millisecond)
	}
	second := make(chan error, 1)
	go func() { _, err := m.Open(ctx, "alice", call); second <- err }()
	evicted := make(chan struct{})
	go func() { m.evict(time.Now()); close(evicted) }()
	select {
	case <-evicted:
	case <-time.After(time.Second):
		t.Fatal("the janitor was blocked by an unanswered init")
	}
	for name, result := range map[string]<-chan error{"first": first, "second": second} {
		select {
		case err := <-result:
			if err == nil {
				t.Fatalf("the %s open answered a hung init", name)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("the %s open did not return", name)
		}
	}
	m.mu.Lock()
	stored := len(m.sessions)
	m.mu.Unlock()
	if stored != 0 {
		t.Fatalf("a hung init stored %d sessions, want none", stored)
	}
}

// A same-key caller joins an in-flight init even when the manager is at
// capacity; a different key still gets the limit error.
func TestSameKeyJoinsInFlightAtCapacity(t *testing.T) {
	fmu := decayFMU(t)
	uri := fileURI(fmu)
	p := parseProbe(t, fmt.Sprintf(decayDriver, uri))
	resetModes(t, "session-hang")
	m := newSessionManager(func() (string, error) { return runner(t), nil },
		func() time.Duration { return 300 * time.Millisecond }, func() int { return 1 << 20 },
		SessionConfig{MaxSessions: 1})
	t.Cleanup(func() { _ = m.Close() })
	ctx := context.Background()
	call := decayCall(t, p, uri, 3)

	first := make(chan error, 1)
	go func() { _, err := m.Open(ctx, "alice", call); first <- err }()
	deadline := time.Now().Add(2 * time.Second)
	for {
		m.mu.Lock()
		open := len(m.opening)
		m.mu.Unlock()
		if open > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the open never registered as in flight")
		}
		time.Sleep(time.Millisecond)
	}

	second := make(chan error, 1)
	go func() { _, err := m.Open(ctx, "alice", call); second <- err }()
	_, limitErr := m.Open(ctx, "bob", call)
	var limit *SessionLimitError
	if !errors.As(limitErr, &limit) {
		t.Fatalf("a different key at capacity: %v, want a SessionLimitError", limitErr)
	}

	for name, result := range map[string]<-chan error{"first": first, "second": second} {
		select {
		case err := <-result:
			if err == nil {
				t.Fatalf("the %s open answered a hung init", name)
			}
			var joinedLimit *SessionLimitError
			if errors.As(err, &joinedLimit) {
				t.Fatalf("the %s open hit the limit instead of joining the init", name)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("the %s open did not return", name)
		}
	}
}
