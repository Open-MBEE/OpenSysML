package fmi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// seriesFMU describes one scalar output T, sampled into a trajectory.
const seriesFMU = `<?xml version="1.0" encoding="UTF-8"?>
<fmiModelDescription fmiVersion="2.0" modelName="Series" guid="{series}">
  <CoSimulation modelIdentifier="Series"/>
  <ModelVariables>
    <ScalarVariable name="T" valueReference="1" causality="output"><Real/></ScalarVariable>
  </ModelVariables>
</fmiModelDescription>`

const seriesReply = `{"protocol": 1, "time": 3.0, "outputs": {"T": 99.0},
  "series": {"times": [0.0, 0.75, 1.5, 2.25, 3.0], "outputs": {"T": [1.0, 2.0, 3.0, 4.0, 5.0]}}}`

// seriesModel reads the committed fixture, filling in the FMU's file URI.
func seriesModel(t *testing.T, uri string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "series.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf(string(data), uri)
}

func seriesFixture(t *testing.T) (*probe, string) {
	t.Helper()
	fmu := writeFMU(t, "series.fmu", seriesFMU, "x86_64-linux")
	scratch(t, "mode.txt", "answer")
	scratch(t, "reply.json", seriesReply)
	return parseProbe(t, seriesModel(t, fileURI(fmu))), fmu
}

func seriesRequest(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(os.TempDir(), "fmirunner-request.json"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(data))
}

func seriesSymbol(t *testing.T, p *probe, name string) *symbols.Symbol {
	t.Helper()
	sym, ok := p.pkg.LookupLocal(name)
	if !ok {
		t.Fatalf("%s not indexed", name)
	}
	return sym
}

func seriesInvoke(t *testing.T, p *probe, r *analysis.Registry) (runtime.Value, error) {
	t.Helper()
	sym := seriesSymbol(t, p, "Traj")
	return p.context(r).InvokeCalc(sym, nil, p.pkg)
}

func seriesCalc(t *testing.T, p *probe, name string) (runtime.Value, error) {
	t.Helper()
	return invokeCalcNamed(t, p, name, engineFor(t))
}

// TestSeriesReturnsTheTrajectory binds one sampled run and decides a
// max-over-trajectory requirement from it.
func TestSeriesReturnsTheTrajectory(t *testing.T) {
	p, fmu := seriesFixture(t)
	series, err := seriesInvoke(t, p, registryWith(t, engineFor(t)))
	if err != nil {
		t.Fatalf("InvokeCalc: %v", err)
	}
	want := "[1.0, 2.0, 3.0, 4.0, 5.0]"
	if got := runtime.FormatValue(series); got != want {
		t.Fatalf("series = %s, want %s", got, want)
	}
	request := seriesRequest(t)
	if !strings.Contains(request, `"series":{"samples":5,"outputs":["T"]}`) {
		t.Fatalf("request does not ask for the series:\n%s", request)
	}
	if !strings.Contains(request, fmt.Sprintf(`"fmu":%q`, fmu)) {
		t.Fatalf("request lost the FMU:\n%s", request)
	}
	var sent runnerRequest
	if err := json.Unmarshal([]byte(request), &sent); err != nil {
		t.Fatalf("request is not JSON: %v", err)
	}
	if len(sent.Outputs) != 0 {
		t.Fatalf("the series variable was also requested as a scalar output: %v", sent.Outputs)
	}

	ctx := p.context(registryWith(t, engineFor(t)))
	peak := seriesSymbol(t, p, "Peak")
	failed, err := ctx.CheckRequirementWith(peak, p.pkg, nil,
		runtime.CheckArgs{Named: map[string]runtime.Value{"temps": series, "limit": realValue(4)}})
	if err != nil && !errors.Is(err, runtime.ErrViolated) {
		t.Fatalf("CheckRequirementWith (limit 4): %v", err)
	}
	if failed.Holds {
		t.Fatal("max 5 <= limit 4 should not hold")
	}
	passed, err := ctx.CheckRequirementWith(peak, p.pkg, nil,
		runtime.CheckArgs{Named: map[string]runtime.Value{"temps": series, "limit": realValue(5)}})
	if err != nil {
		t.Fatalf("CheckConstraintWith (limit 5): %v", err)
	}
	if !passed.Holds {
		t.Fatal("max 5 <= limit 5 should hold")
	}
}

func realValue(n float64) runtime.Value {
	return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: n}}
}

// TestSeriesRequirementDecidesInOneRun decides a requirement in one run:
// the condition invokes the tool calc and takes the trajectory's maximum.
func TestSeriesRequirementDecidesInOneRun(t *testing.T) {
	p, _ := seriesFixture(t)
	ctx := p.context(registryWith(t, engineFor(t)))
	peak := seriesSymbol(t, p, "PeakFromRun")
	failed, err := ctx.CheckRequirementWith(peak, p.pkg, nil,
		runtime.CheckArgs{Named: map[string]runtime.Value{"limit": realValue(4)}})
	if err != nil && !errors.Is(err, runtime.ErrViolated) {
		t.Fatalf("CheckRequirementWith (limit 4): %v", err)
	}
	if failed.Holds {
		t.Fatal("peak 5 <= limit 4 should not hold")
	}
	passed, err := ctx.CheckRequirementWith(peak, p.pkg, nil,
		runtime.CheckArgs{Named: map[string]runtime.Value{"limit": realValue(5)}})
	if err != nil {
		t.Fatalf("CheckRequirementWith (limit 5): %v", err)
	}
	if !passed.Holds {
		t.Fatal("peak 5 <= limit 5 should hold")
	}
	request := seriesRequest(t)
	if !strings.Contains(request, `"series":{"samples":5,"outputs":["T"]}`) {
		t.Fatalf("the requirement did not drive a series run:\n%s", request)
	}
}

// TestSeriesDefaultSampleCount proves a caller need not declare a sampling
// input: a plain series output still gets a real trajectory, not one value.
func TestSeriesDefaultSampleCount(t *testing.T) {
	_, fmu := seriesFixture(t)
	src := strings.Replace(seriesModel(t, fileURI(fmu)), `in samples : Integer = 5 { @ToolVariable { name = "fmi:samples"; } }`, "", 1)
	p := parseProbe(t, src)
	reply := strings.Replace(seriesReply, "[0.0, 0.75, 1.5, 2.25, 3.0]", "[0.0, 0.3, 0.6, 0.9, 1.2, 1.5, 1.8, 2.1, 2.4, 2.7, 3.0]", 1)
	reply = strings.Replace(reply, "[1.0, 2.0, 3.0, 4.0, 5.0]", "[1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0, 11.0]", 1)
	scratch(t, "reply.json", reply)
	series, err := seriesInvoke(t, p, registryWith(t, engineFor(t)))
	if err != nil {
		t.Fatalf("InvokeCalc: %v", err)
	}
	if got := runtime.FormatValue(series); got != "[1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0, 11.0]" {
		t.Fatalf("series = %s, want eleven samples", got)
	}
	if !strings.Contains(seriesRequest(t), `"series":{"samples":11,"outputs":["T"]}`) {
		t.Fatalf("request did not use the default sample count:\n%s", seriesRequest(t))
	}
}

// TestSeriesCarriesUnitsAndTimes checks units, sample times and a measured
// interval reaching the runner in seconds.
func TestSeriesCarriesUnitsAndTimes(t *testing.T) {
	fmu := writeFMU(t, "units.fmu", unitsFMU, "x86_64-linux")
	scratch(t, "mode.txt", "answer")
	scratch(t, "reply.json", `{"protocol": 1, "time": 2.0, "outputs": {},
		"series": {"times": [0.0, 1.0, 2.0], "outputs": {"h": [1.0, 2.0, 3.0]}}}`)
	const measured = `package Drive {
	private import AnalysisTooling::*;
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;

	calc def Series {
		metadata ToolExecution { toolName = "fmi"; uri = "%s"; }
		in samples : Integer = 3 { @ToolVariable { name = "fmi:samples"; } }
		in stopTime : Real = 2.0 { @ToolVariable { name = "fmi:stopTime"; } }
		out times : DurationValue[1..*] { @ToolVariable { name = "fmi:seriesTime"; } }
		return h : LengthValue[1..*] { @ToolVariable { name = "fmi:series:h"; } }
	}
}`
	p := parseProbe(t, fmt.Sprintf(measured, fileURI(fmu)))
	series, err := seriesCalc(t, p, "Series")
	if err != nil {
		t.Fatalf("InvokeCalc: %v", err)
	}
	if got := runtime.FormatValue(series); !strings.Contains(got, "[SI::m]") || !strings.Contains(got, "3.0") {
		t.Fatalf("series = %s, want measured samples in metres", got)
	}
	if !strings.Contains(seriesRequest(t), `"series":{"samples":3,"outputs":["h"]}`) {
		t.Fatalf("request lost the series:\n%s", seriesRequest(t))
	}

	const timed = `package Drive {
	private import AnalysisTooling::*;
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;

	calc def Series {
		metadata ToolExecution { toolName = "fmi"; uri = "%s"; }
		in interval : DurationValue = 1 [min] { @ToolVariable { name = "fmi:sampleInterval"; } }
		in stopTime : Real = 120.0 { @ToolVariable { name = "fmi:stopTime"; } }
		return times : DurationValue[1..*] { @ToolVariable { name = "fmi:seriesTime"; } }
	}
}`
	scratch(t, "reply.json", `{"protocol": 1, "time": 120.0, "outputs": {},
		"series": {"times": [0.0, 60.0, 120.0], "outputs": {}}}`)
	p = parseProbe(t, fmt.Sprintf(timed, fileURI(fmu)))
	times, err := seriesCalc(t, p, "Series")
	if err != nil {
		t.Fatalf("InvokeCalc (sample times): %v", err)
	}
	if got := runtime.FormatValue(times); !strings.Contains(got, "[SI::s]") || !strings.Contains(got, "120.0") {
		t.Fatalf("sample times = %s, want seconds", got)
	}
	if !strings.Contains(seriesRequest(t), `"series":{"samples":3,"times":[0,60,120],"outputs":[]}`) {
		t.Fatalf("the interval did not resolve to three instants:\n%s", seriesRequest(t))
	}
}

// TestSeriesIntervalSpacingIsHonoured is the non-dividing-interval regression:
// instants at 0 s and 2 s are requested and answered, never respaced at stopTime.
func TestSeriesIntervalSpacingIsHonoured(t *testing.T) {
	fmu := writeFMU(t, "series.fmu", seriesFMU, "x86_64-linux")
	scratch(t, "mode.txt", "answer")
	scratch(t, "reply.json", `{"protocol": 1, "time": 3.0, "outputs": {},
		"series": {"times": [0.0, 2.0], "outputs": {"T": [1.0, 2.0]}}}`)
	const model = `package Drive {
	private import AnalysisTooling::*;
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;

	calc def Times {
		metadata ToolExecution { toolName = "fmi"; uri = "%s"; }
		in interval : DurationValue = 2 [s] { @ToolVariable { name = "fmi:sampleInterval"; } }
		in stopTime : Real = 3.0 { @ToolVariable { name = "fmi:stopTime"; } }
		return times : DurationValue[1..*] { @ToolVariable { name = "fmi:seriesTime"; } }
	}

	calc def Temps {
		metadata ToolExecution { toolName = "fmi"; uri = "%s"; }
		in interval : DurationValue = 2 [s] { @ToolVariable { name = "fmi:sampleInterval"; } }
		in stopTime : Real = 3.0 { @ToolVariable { name = "fmi:stopTime"; } }
		return temps : Real[1..*] { @ToolVariable { name = "fmi:series:T"; } }
	}
}`
	p := parseProbe(t, fmt.Sprintf(model, fileURI(fmu), fileURI(fmu)))
	times, err := seriesCalc(t, p, "Times")
	if err != nil {
		t.Fatalf("InvokeCalc (times): %v", err)
	}
	if got := runtime.FormatValue(times); !strings.Contains(got, "2.0 [SI::s]") || strings.Contains(got, "3.0 [SI::s]") {
		t.Fatalf("sample times = %s, want 0 s and 2 s", got)
	}
	if !strings.Contains(seriesRequest(t), `"series":{"samples":2,"times":[0,2],"outputs":[]}`) {
		t.Fatalf("request did not carry the requested instants:\n%s", seriesRequest(t))
	}
	temps, err := seriesCalc(t, p, "Temps")
	if err != nil {
		t.Fatalf("InvokeCalc (temps): %v", err)
	}
	if got := runtime.FormatValue(temps); got != "[1.0, 2.0]" {
		t.Fatalf("temps = %s, want the two requested samples", got)
	}

	scratch(t, "reply.json", `{"protocol": 1, "time": 3.0, "outputs": {},
		"series": {"times": [0.0, 3.0], "outputs": {"T": [1.0, 2.0]}}}`)
	_, err = seriesCalc(t, p, "Times")
	var fault *runtime.ToolError
	if !errors.As(err, &fault) || fault.Kind != runtime.ToolMalformed {
		t.Fatalf("InvokeCalc = %v, want a malformed reply", err)
	}
	if !strings.Contains(fault.Detail, "expected 2") {
		t.Fatalf("refusal = %q, want it to name the requested instant", fault.Detail)
	}
}

// TestSeriesIsNeverACollapsedStopTimeValue is the negative control: a scalar
// answer is refused where a trajectory was requested.
func TestSeriesIsNeverACollapsedStopTimeValue(t *testing.T) {
	p, _ := seriesFixture(t)
	scratch(t, "reply.json", `{"protocol": 1, "time": 3.0, "outputs": {"T": 99.0}}`)
	_, err := seriesInvoke(t, p, registryWith(t, engineFor(t)))
	var fault *runtime.ToolError
	if !errors.As(err, &fault) || fault.Kind != runtime.ToolMalformed {
		t.Fatalf("InvokeCalc = %v, want a malformed reply", err)
	}
	if !strings.Contains(fault.Detail, "answered no series") {
		t.Fatalf("refusal = %q, want it to name the missing series", fault.Detail)
	}
}

// TestSeriesRejectsAWrongSampleCount guards sample count, time ordering,
// finiteness and per-variable lengths.
func TestSeriesRejectsAWrongSampleCount(t *testing.T) {
	p, _ := seriesFixture(t)
	for name, reply := range map[string]string{
		"short samples":           `{"protocol": 1, "time": 3.0, "outputs": {}, "series": {"times": [0.0, 1.0], "outputs": {"T": [1.0, 2.0]}}}`,
		"short times":             `{"protocol": 1, "time": 3.0, "outputs": {}, "series": {"times": [0.0, 1.0, 2.0], "outputs": {"T": [1.0, 2.0]}}}`,
		"missing series variable": `{"protocol": 1, "time": 3.0, "outputs": {}, "series": {"times": [0.0, 1.0, 2.0, 3.0, 4.0], "outputs": {}}}`,
		"unordered times":         `{"protocol": 1, "time": 3.0, "outputs": {}, "series": {"times": [0.0, 2.0, 1.0, 3.0, 4.0], "outputs": {"T": [1.0, 2.0, 3.0, 4.0, 5.0]}}}`,
		"non-finite time":         `{"protocol": 1, "time": 3.0, "outputs": {}, "series": {"times": [0.0, 1.0, "NaN", 3.0, 4.0], "outputs": {"T": [1.0, 2.0, 3.0, 4.0, 5.0]}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			scratch(t, "reply.json", reply)
			_, err := seriesInvoke(t, p, registryWith(t, engineFor(t)))
			var fault *runtime.ToolError
			if !errors.As(err, &fault) || fault.Kind != runtime.ToolMalformed {
				t.Fatalf("InvokeCalc = %v, want a malformed reply", err)
			}
		})
	}
}

// TestSeriesRefusesAScalarParameter proves the guard also holds on the caller's
// side: binding a trajectory into a single-valued parameter is a typed refusal.
func TestSeriesRefusesAScalarParameter(t *testing.T) {
	_, fmu := seriesFixture(t)
	src := strings.Replace(seriesModel(t, fileURI(fmu)), "return temps : Real[1..*] nonunique", "return temps : Real[1]", 1)
	p := parseProbe(t, src)
	_, err := seriesInvoke(t, p, registryWith(t, engineFor(t)))
	var fault *runtime.ToolError
	if !errors.As(err, &fault) || fault.Kind != runtime.ToolMalformed {
		t.Fatalf("InvokeCalc = %v, want a malformed reply", err)
	}
	if !strings.Contains(fault.Detail, "holds at most one value") {
		t.Fatalf("refusal = %q, want it to name the multiplicity", fault.Detail)
	}
}

// TestSeriesVariableErrors covers the variable a series names: it must exist,
// be readable as an output and be a scalar rather than an array.
func TestSeriesVariableErrors(t *testing.T) {
	for name, tc := range map[string]struct {
		xml, output, want string
	}{
		"unknown": {seriesFMU, "fmi:series:missing", `has no variable "missing"`},
		"input": {`<?xml version="1.0" encoding="UTF-8"?>
<fmiModelDescription fmiVersion="2.0" modelName="Series" guid="{series}">
  <CoSimulation modelIdentifier="Series"/>
  <ModelVariables>
    <ScalarVariable name="T" valueReference="1" causality="input"><Real/></ScalarVariable>
  </ModelVariables>
</fmiModelDescription>`, "fmi:series:T", "not an output"},
		"array": {`<?xml version="1.0" encoding="UTF-8"?>
<fmiModelDescription fmiVersion="3.0" modelName="Series" instantiationToken="{series}">
  <CoSimulation modelIdentifier="Series"/>
  <ModelVariables>
    <Float64 name="T" valueReference="1" causality="output"><Dimension start="2"/></Float64>
  </ModelVariables>
</fmiModelDescription>`, "fmi:series:T", "cannot be sampled"},
	} {
		t.Run(name, func(t *testing.T) {
			fmu := writeFMU(t, "series.fmu", tc.xml, "x86_64-linux")
			src := strings.Replace(seriesModel(t, fileURI(fmu)), "fmi:series:T", tc.output, 1)
			p := parseProbe(t, src)
			sym := seriesSymbol(t, p, "Traj")
			call := &runtime.ToolCall{Action: sym, ToolName: "fmi", URI: fileURI(fmu)}
			call.Outputs = append(call.Outputs, runtime.ToolOutput{Variable: tc.output, Parameter: "temps"})
			q := analysis.Question{Kind: analysis.Compute, Compute: &analysis.ComputeAsk{Call: call}}
			cov := engineFor(t).Covers(nil, q)
			if cov.Covered {
				t.Fatal("a series naming an invalid variable should be refused")
			}
			var verr *VariableError
			if !errors.As(cov.Refusal, &verr) || !strings.Contains(cov.Refusal.Error(), tc.want) {
				t.Fatalf("refusal = %v, want a VariableError naming %q", cov.Refusal, tc.want)
			}
		})
	}
}

// TestLiteralNameCollidingWithSeriesSelectorBindsAtStopTime proves an output named
// `fmi:series:y` reads that variable at stopTime, even when the FMU also declares `y`.
func TestLiteralNameCollidingWithSeriesSelectorBindsAtStopTime(t *testing.T) {
	fmu := writeFMU(t, "collide.fmu", `<?xml version="1.0" encoding="UTF-8"?>
<fmiModelDescription fmiVersion="2.0" modelName="Collide" guid="{collide}">
  <CoSimulation modelIdentifier="Collide"/>
  <ModelVariables>
    <ScalarVariable name="y" valueReference="1" causality="output"><Real/></ScalarVariable>
    <ScalarVariable name="fmi:series:y" valueReference="2" causality="output"><Real/></ScalarVariable>
  </ModelVariables>
</fmiModelDescription>`, "x86_64-linux")
	scratch(t, "mode.txt", "answer")
	scratch(t, "reply.json", `{"protocol": 1, "time": 3.0, "outputs": {"fmi:series:y": 42.0}}`)
	p := parseProbe(t, fmt.Sprintf(`package Drive {
	private import AnalysisTooling::*;
	private import ScalarValues::*;

	calc def Literal {
		metadata ToolExecution { toolName = "fmi"; uri = "%s"; }
		return literal : Real[1] { @ToolVariable { name = "fmi:series:y"; } }
	}
}`, fileURI(fmu)))
	value, err := invokeCalcNamed(t, p, "Literal", engineFor(t))
	if err != nil {
		t.Fatalf("InvokeCalc: %v", err)
	}
	if got := runtime.FormatValue(value); got != "42.0" {
		t.Fatalf("value = %s, want the literal stopTime value 42.0", got)
	}
	want := fmt.Sprintf(`{"protocol":1,"fmu":%q,"interface":"coSimulation","experiment":{"startTime":0,"stopTime":1},"start":{},"outputs":["fmi:series:y"]}`, fmu)
	if got := seriesRequest(t); got != want {
		t.Fatalf("request =\n%s\nwant\n%s", got, want)
	}
}

// TestSampleCountValidatesTheRequest is the typed refusal of contradictory or
// unbounded sampling inputs, checked before the runner is started.
func TestSampleCountValidatesTheRequest(t *testing.T) {
	three, zero, fractional, big := 3.0, 0.0, 2.5, float64(maxSeriesSamples+1)
	two, interval, wide, negative := 2.0, 1.0, 100.0, -1.0
	for name, tc := range map[string]struct {
		samples, interval *float64
		want              string
	}{
		"one sample":        {&zero, nil, "not an integer from 2"},
		"fractional count":  {&fractional, nil, "not an integer from 2"},
		"both sent":         {&two, &interval, "both sent"},
		"too many samples":  {&big, nil, "not an integer from 2"},
		"negative interval": {nil, &negative, "positive finite"},
		"interval too wide": {nil, &wide, "no more than one point"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := sampleCount(0, float64(three), tc.samples, tc.interval)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("sampleCount = %v, want an error naming %q", err, tc.want)
			}
			var fault *runtime.ToolError
			if !errors.As(err, &fault) || fault.Kind != runtime.ToolUnsentInput {
				t.Fatalf("sampleCount = %v, want a ToolError of kind unsent input", err)
			}
		})
	}
}

// recordingToolRunner forwards nothing; it only proves which surface a call reached.
type recordingToolRunner struct{ calls []*runtime.ToolCall }

func (r *recordingToolRunner) RunTool(call *runtime.ToolCall) (runtime.ToolAnswer, error) {
	r.calls = append(r.calls, call)
	return runtime.ToolAnswer{}, nil
}

// TestLiteralStepInputIsSetAsAVariable proves an FMU input actually named
// fmi:stepTime is set on the FMU, not routed to a session as a step control.
func TestLiteralStepInputIsSetAsAVariable(t *testing.T) {
	fmu := writeFMU(t, "literal-step.fmu", `<?xml version="1.0" encoding="UTF-8"?>
<fmiModelDescription fmiVersion="2.0" modelName="LiteralStep" guid="{literal-step}">
  <CoSimulation modelIdentifier="LiteralStep"/>
  <ModelVariables>
    <ScalarVariable name="fmi:stepTime" valueReference="1" causality="input"><Real/></ScalarVariable>
    <ScalarVariable name="y" valueReference="2" causality="output"><Real/></ScalarVariable>
  </ModelVariables>
</fmiModelDescription>`, "x86_64-linux")
	uri := fileURI(fmu)
	d, err := Read(fmu)
	if err != nil {
		t.Fatal(err)
	}
	call := &runtime.ToolCall{ToolName: "fmi", URI: uri,
		Inputs:  []runtime.ToolInput{{Variable: StepVariable, Parameter: "stepTime", Value: real(0.7)}},
		Outputs: []runtime.ToolOutput{{Variable: "y", Parameter: "y"}}}
	if namesStep(call, d) {
		t.Fatal("an FMU-declared fmi:stepTime must not be a session step")
	}
	if _, stepped, err := stepTimeOf(call, d); err != nil || stepped {
		t.Fatalf("stepTimeOf = %v, %v; want no step", stepped, err)
	}
	fields, _, err := requestFields(call, d, fmu)
	if err != nil {
		t.Fatalf("requestFields: %v", err)
	}
	if got := fields.Start[StepVariable]; got != 0.7 {
		t.Fatalf("start[%s] = %v, want the literal value 0.7", StepVariable, got)
	}
	e := engineFor(t)
	q := analysis.Question{Kind: analysis.Compute, Compute: &analysis.ComputeAsk{Call: call}}
	if cov := e.Covers(nil, q); !cov.Covered {
		t.Fatalf("Covers refused a literal step input: %v", cov.Refusal)
	}

	delegate := &recordingToolRunner{}
	m := testManager(t, SessionConfig{})
	if _, err := m.ToolRunner("alice", delegate).RunTool(call); err != nil {
		t.Fatalf("RunTool: %v", err)
	}
	if len(delegate.calls) != 1 {
		t.Fatalf("the session runner forwarded %d calls, want the one-shot delegate", len(delegate.calls))
	}
}

// TestRealStepAndSeriesAreRefusedTogether keeps the typed refusal for a genuine
// step control combined with a trajectory selector on all three surfaces.
func TestRealStepAndSeriesAreRefusedTogether(t *testing.T) {
	fmu := writeFMU(t, "series.fmu", seriesFMU, "x86_64-linux")
	d, err := Read(fmu)
	if err != nil {
		t.Fatal(err)
	}
	call := &runtime.ToolCall{ToolName: "fmi", URI: fileURI(fmu),
		Inputs:  []runtime.ToolInput{{Variable: StepVariable, Parameter: "t", Value: real(0.5)}},
		Outputs: []runtime.ToolOutput{{Variable: seriesOutputPrefix + "T", Parameter: "temps"}}}
	if !namesStep(call, d) || !namesSeries(call, d) {
		t.Fatal("a genuine step and series selector must both be detected")
	}
	if _, _, err := requestFields(call, d, fmu); !errors.Is(err, ErrSessionSeries) {
		t.Fatalf("requestFields = %v, want a typed step+series refusal", err)
	}
	e := engineFor(t)
	q := analysis.Question{Kind: analysis.Compute, Compute: &analysis.ComputeAsk{Call: call}}
	var refusal *SessionSeriesError
	if cov := e.Covers(nil, q); cov.Covered || !errors.As(cov.Refusal, &refusal) {
		t.Fatalf("Covers = %+v, want *SessionSeriesError", cov)
	}
	if _, err := e.Run(context.Background(), nil, q, analysis.Budget{}); !errors.As(err, &refusal) {
		t.Fatalf("Run = %v, want *SessionSeriesError", err)
	}
}

// TestSampleCountKeepsTheDecimalEndpoint proves a 0.1 s step over a 0.3 s run
// includes 0.3, and that the bound and non-dividing spans still behave.
func TestSampleCountKeepsTheDecimalEndpoint(t *testing.T) {
	step := 0.1
	for bounds, want := range map[struct{ start, stop float64 }]int{
		{0, 0.3}:   4, // 0, 0.1, 0.2, 0.3
		{0, 0.35}:  4, // the last step before stopTime
		{0.1, 0.3}: 3,
	} {
		got, err := sampleCount(bounds.start, bounds.stop, nil, &step)
		if err != nil || got != want {
			t.Fatalf("sampleCount(%v, %v, 0.1) = %d, %v; want %d", bounds.start, bounds.stop, got, err, want)
		}
	}
}

// TestDecimalIntervalRequestCarriesTheEndpoint pins the exact instants the
// engine sends: a decimal interval must not drop the final sample at stopTime.
func TestDecimalIntervalRequestCarriesTheEndpoint(t *testing.T) {
	fmu := writeFMU(t, "units.fmu", unitsFMU, "x86_64-linux")
	scratch(t, "mode.txt", "answer")
	scratch(t, "reply.json", `{"protocol": 1, "time": 0.3, "outputs": {},
		"series": {"times": [0.0, 0.1, 0.2, 0.3], "outputs": {}}}`)
	const model = `package Drive {
	private import AnalysisTooling::*;
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;

	calc def Times {
		metadata ToolExecution { toolName = "fmi"; uri = "%s"; }
		in interval : DurationValue = 0.1 [s] { @ToolVariable { name = "fmi:sampleInterval"; } }
		in stopTime : Real = 0.3 { @ToolVariable { name = "fmi:stopTime"; } }
		return times : DurationValue[1..*] { @ToolVariable { name = "fmi:seriesTime"; } }
	}
}`
	p := parseProbe(t, fmt.Sprintf(model, fileURI(fmu)))
	times, err := seriesCalc(t, p, "Times")
	if err != nil {
		t.Fatalf("InvokeCalc: %v", err)
	}
	if got := runtime.FormatValue(times); !strings.Contains(got, "0.3") {
		t.Fatalf("sample times = %s, want the 0.3 s endpoint", got)
	}
	if request := seriesRequest(t); !strings.Contains(request, `"series":{"samples":4,"times":[0,0.1,0.2,0.3],"outputs":[]}`) {
		t.Fatalf("request lost the decimal endpoint:\n%s", request)
	}
}

// TestOutOfRangeSampleIsATypedRunnerError proves a runner refusing an
// unobserved instant surfaces as the typed RunnerError, not a bound value.
func TestOutOfRangeSampleIsATypedRunnerError(t *testing.T) {
	fmu := writeFMU(t, "series.fmu", seriesFMU, "x86_64-linux")
	scratch(t, "mode.txt", "error:sample instant 4 lies outside the simulated run 0 to 2")
	const model = `package Drive {
	private import AnalysisTooling::*;
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;

	calc def Interval {
		metadata ToolExecution { toolName = "fmi"; uri = "%s"; }
		in interval : DurationValue = 2 [s] { @ToolVariable { name = "fmi:sampleInterval"; } }
		in stopTime : Real = 4.0 { @ToolVariable { name = "fmi:stopTime"; } }
		return times : DurationValue[1..*] { @ToolVariable { name = "fmi:seriesTime"; } }
	}
}`
	p := parseProbe(t, fmt.Sprintf(model, fileURI(fmu)))
	_, err := invokeCalcNamed(t, p, "Interval", engineFor(t))
	var runnerErr *RunnerError
	if !errors.As(err, &runnerErr) {
		t.Fatalf("InvokeCalc = %v, want a typed RunnerError", err)
	}
	if !strings.Contains(err.Error(), "outside the simulated run") {
		t.Fatalf("error = %q, want the runner's range refusal", err)
	}
}

// TestScalarOutputsRemainUnchanged pins the old request shape byte for byte
// beside a series request: adding a series must not perturb a scalar-only call.
func TestScalarOutputsRemainUnchanged(t *testing.T) {
	fmu := ballFMU(t)
	p := parseProbe(t, fmt.Sprintf(driver, fileURI(fmu)))
	scratch(t, "mode.txt", "answer")
	scratch(t, "reply.json", `{"protocol": 1, "time": 3.0, "outputs": {"h": 0.0123, "v": -0.15}}`)
	result, err := p.invoke(t, registryWith(t, engineFor(t)))
	if err != nil {
		t.Fatalf("InvokeCalc: %v", err)
	}
	if got := runtime.FormatValue(result); got != "0.0123 [SI::m]" {
		t.Fatalf("result = %s, want 0.0123 [SI::m]", got)
	}
	request := seriesRequest(t)
	want := fmt.Sprintf(`{"protocol":1,"fmu":%q,"interface":"coSimulation","experiment":{"startTime":0,"stopTime":3,"stepSize":0.01,"tolerance":0.0001},"start":{"e":0.7,"g":-9.81},"outputs":["h","v"]}`, fmu)
	if request != want {
		t.Fatalf("request =\n%s\nwant\n%s", request, want)
	}
}
