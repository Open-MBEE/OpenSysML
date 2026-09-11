package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// toolModel is the pilot's AnalysisAnnotation fixture, which imports no units, with a driver that performs the
// annotated action with fixed inputs and exposes what it answered.
const toolModel = `package test {
	private import ScalarValues::Real;
	private import AnalysisTooling::*;
	private import ISQ::*;

	action def ComputeDynamics {
		metadata ToolExecution {
			toolName = "ModelCenter";
			uri = "aserv://localhost/Vehicle/Equation1";
		}

		in dt : TimeValue             { @ToolVariable { name = "deltaT"; } }
		in whlpwr : PowerValue        { @ToolVariable { name = "power"; } }
		in Cd : Real                  { @ToolVariable { name = "C_D"; } }
		in Cf: Real                   { @ToolVariable { name = "C_F"; } }
		in tm : MassValue             { @ToolVariable { name = "mass"; } }
		in v_in : SpeedValue          { @ToolVariable { name = "v0"; } }
		in x_in : LengthValue         { @ToolVariable { name = "x0"; } }

		out a_out : AccelerationValue { @ToolVariable { name = "a"; } }
		out v_out : SpeedValue          { @ToolVariable { name = "v"; } }
		out x_out : LengthValue         { @ToolVariable { name = "x"; } }
	}

	action def Drive {
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
}`

// recordingRunner answers every call from a table, keeping the calls it saw.
type recordingRunner struct {
	calls  []*ToolCall
	answer map[string]ToolValue
	err    error
}

func (r *recordingRunner) RunTool(call *ToolCall) (ToolAnswer, error) {
	r.calls = append(r.calls, call)
	if r.err != nil {
		return ToolAnswer{}, r.err
	}
	outputs, err := call.Bind(r.answer)
	return ToolAnswer{Outputs: outputs}, err
}

func toolReal(x float64) semantics.Value { return semantics.Value{Kind: semantics.ValReal, Real: x} }

// The annotated action's performance goes to the tool: inputs in the coherent unit of their
// parameter's dimension under their ToolVariable names, outputs converted to the parameters'
// declared units and propagated to the caller; its body is never run.
func TestToolExecutionPerformsThroughTheRunner(t *testing.T) {
	ctx, scope := analysisFixture(t, toolModel)
	runner := &recordingRunner{answer: map[string]ToolValue{
		"a": {Value: toolReal(2), Unit: "m/s**2"},
		"v": {Value: toolReal(43.2), Unit: "km/h"},
		"x": {Value: toolReal(11000), Unit: "cm"},
	}}
	ctx.SetToolRunner(runner)
	drive := calcNamed(t, scope, "Drive")

	out, err := ctx.ExecuteAction(drive)
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("tool invoked %d times, want once", len(runner.calls))
	}
	call := runner.calls[0]
	if call.ToolName != "ModelCenter" || call.URI != "aserv://localhost/Vehicle/Equation1" {
		t.Fatalf("call names %q at %q", call.ToolName, call.URI)
	}
	inputs := map[string]ToolValue{}
	for _, in := range call.Inputs {
		inputs[in.Variable] = in.Value
	}
	want := map[string]ToolValue{
		"deltaT": {Value: toolReal(1), Unit: "s"},
		"power":  {Value: toolReal(2), Unit: "kW"},
		"C_D":    {Value: toolReal(0.3)},
		"C_F":    {Value: toolReal(0.01)},
		"mass":   {Value: toolReal(1500), Unit: "kg"},
		"v0":     {Value: toolReal(36), Unit: "km/h"},
		"x0":     {Value: toolReal(100), Unit: "m"},
	}
	for name, w := range want {
		got, ok := inputs[name]
		if !ok {
			t.Fatalf("input %s not sent; sent %v", name, inputs)
		}
		if got.Unit != w.Unit || !nearly(got.Value, w.Value) {
			t.Errorf("input %s = %v [%s], want %v [%s]", name, got.Value, got.Unit, w.Value, w.Unit)
		}
	}
	if len(call.Outputs) != 3 {
		t.Fatalf("outputs asked: %v", call.Outputs)
	}
	for name, w := range map[string]string{"a": "2.0 [SI::'m⋅s⁻²']", "v": "12.0 [SI::'m/s']", "x": "110.0 [SI::m]"} {
		got, ok := out[name]
		if !ok {
			t.Fatalf("output %s not propagated; got %v", name, out)
		}
		if FormatValue(got) != w {
			t.Errorf("%s = %s, want %s", name, FormatValue(got), w)
		}
	}
}

// nearly compares two numbers as the protocol carries them.
func nearly(a, b semantics.Value) bool {
	af, bf := a.Real, b.Real
	if a.Kind == semantics.ValInt {
		af = float64(a.Int)
	}
	if b.Kind == semantics.ValInt {
		bf = float64(b.Int)
	}
	d := af - bf
	return d < 1e-9 && d > -1e-9
}

// A context with no runner refuses the performance as the registry would: the named
// tool is not registered, and the body is not run in its place.
func TestToolExecutionWithoutRunnerIsNotRegistered(t *testing.T) {
	ctx, scope := analysisFixture(t, toolModel)
	_, err := ctx.ExecuteAction(calcNamed(t, scope, "Drive"))
	var refusal *ToolNotRegisteredError
	if !errors.As(err, &refusal) || !errors.Is(err, ErrToolNotRegistered) {
		t.Fatalf("ExecuteAction = %v, want ToolNotRegisteredError", err)
	}
	if want := "tool 'ModelCenter' is not registered; set OPENSYSML_TOOLS"; !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q does not carry %q", err, want)
	}
}

// What the tool answers is checked against the action: an output missing, one no
// ToolVariable receives, or one in a unit the parameter does not measure is a ToolError
// of its kind, and the runner's own failure is the performance's.
func TestToolExecutionRefusesBadAnswers(t *testing.T) {
	good := map[string]ToolValue{
		"a": {Value: toolReal(2), Unit: "m/s**2"},
		"v": {Value: toolReal(12), Unit: "m/s"},
		"x": {Value: toolReal(110), Unit: "m"},
	}
	cases := []struct {
		name   string
		answer map[string]ToolValue
		kind   ToolErrorKind
	}{
		{"missing", map[string]ToolValue{"a": good["a"], "v": good["v"]}, ToolMissingOutput},
		{"unknown", map[string]ToolValue{"a": good["a"], "v": good["v"], "x": good["x"], "y": good["x"]}, ToolUnknownOutput},
		{"wrong dimension", map[string]ToolValue{"a": {Value: toolReal(2), Unit: "kg"}, "v": good["v"], "x": good["x"]}, ToolMalformed},
		{"not a unit", map[string]ToolValue{"a": {Value: toolReal(2), Unit: "m/s**"}, "v": good["v"], "x": good["x"]}, ToolMalformed},
		{"text measured", map[string]ToolValue{"a": {Text: "fast", Unit: "m"}, "v": good["v"], "x": good["x"]}, ToolMalformed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, scope := analysisFixture(t, toolModel)
			ctx.SetToolRunner(&recordingRunner{answer: tc.answer})
			_, err := ctx.ExecuteAction(calcNamed(t, scope, "Drive"))
			var failure *ToolError
			if !errors.As(err, &failure) || !errors.Is(err, ErrTool) {
				t.Fatalf("ExecuteAction = %v, want ToolError", err)
			}
			if failure.Kind != tc.kind || failure.Tool != "ModelCenter" {
				t.Fatalf("ToolError = %v, want kind %s", failure, tc.kind)
			}
		})
	}
	t.Run("runner failure", func(t *testing.T) {
		ctx, scope := analysisFixture(t, toolModel)
		fault := &ToolError{Tool: "ModelCenter", Kind: ToolTimeout, Detail: "10s"}
		ctx.SetToolRunner(&recordingRunner{err: fault})
		_, err := ctx.ExecuteAction(calcNamed(t, scope, "Drive"))
		if !errors.Is(err, fault) {
			t.Fatalf("ExecuteAction = %v, want %v", err, fault)
		}
	})
}

// A runner reporting that equal inputs were answered differently leaves the run a note.
func TestToolExecutionNotesDivergence(t *testing.T) {
	ctx, scope := analysisFixture(t, toolModel)
	ctx.SetToolRunner(divergingRunner{})
	if _, err := ctx.ExecuteAction(calcNamed(t, scope, "Drive")); err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	notes := ctx.Notes()
	if len(notes) != 1 {
		t.Fatalf("notes = %v, want one divergence", notes)
	}
	d, ok := notes[0].(ToolDivergence)
	if !ok || d.Tool != "ModelCenter" || !strings.Contains(d.Describe(), "answered differently for equal inputs") {
		t.Fatalf("note = %#v", notes[0])
	}
	if diag := d.Diagnostic(); diag.Code != ToolDivergenceCode {
		t.Fatalf("diagnostic code %q", diag.Code)
	}
}

// divergingRunner answers well, spelling units as the run prints them, and reports the
// answer as having changed.
type divergingRunner struct{}

func (divergingRunner) RunTool(call *ToolCall) (ToolAnswer, error) {
	outputs, err := call.Bind(map[string]ToolValue{
		"a": {Value: toolReal(2), Unit: "SI::'m⋅s⁻²'"},
		"v": {Value: toolReal(12), Unit: "SI::'m/s'"},
		"x": {Value: toolReal(110), Unit: "m"},
	})
	return ToolAnswer{Outputs: outputs, Diverged: true}, err
}

// scaleModel is a tool-computed action with a required and an optional input, driven with
// and without each, and variants whose ToolVariable names collide.
const scaleModel = `package test {
	private import ScalarValues::Real;
	private import AnalysisTooling::*;

	action def Scale {
		metadata ToolExecution { toolName = "MC"; uri = "u"; }
		in k : Real          { @ToolVariable { name = "k"; } }
		in bias : Real [0..1] { @ToolVariable { name = "bias"; } }
		out y : Real         { @ToolVariable { name = "y"; } }
	}
	action def Scaled { out y : Real; action s : Scale { in k = 2.0; } bind y = s.y; }
	action def Biased { out y : Real; action s : Scale { in k = 2.0; in bias = 1.0; } bind y = s.y; }
	action def Unscaled { out y : Real; action s : Scale { in bias = 1.0; } bind y = s.y; }

	action def TwoInputs {
		metadata ToolExecution { toolName = "MC"; uri = "u"; }
		in a : Real  { @ToolVariable { name = "x"; } }
		in b : Real  { @ToolVariable { name = "x"; } }
		out y : Real { @ToolVariable { name = "y"; } }
	}
	action def TwoOutputs {
		metadata ToolExecution { toolName = "MC"; uri = "u"; }
		in a : Real  { @ToolVariable { name = "x"; } }
		out y : Real { @ToolVariable { name = "y"; } }
		out z : Real { @ToolVariable { name = "y"; } }
	}
	action def InAndOut {
		metadata ToolExecution { toolName = "MC"; uri = "u"; }
		in a : Real  { @ToolVariable { name = "x"; } }
		out y : Real { @ToolVariable { name = "x"; } }
	}
	action def Collides { out y : Real; action s : InAndOut { in a = 1.0; } bind y = s.y; }

	action def Doubling {
		metadata ToolExecution { toolName = "MC"; uri = "u"; }
		in k : Real = 2.0 { @ToolVariable { name = "k"; } }
		out y : Real     { @ToolVariable { name = "y"; } }
	}
}`

// An input carrying a ToolVariable that no argument binds is ErrUnboundParameter before
// any tool runs, as it is for an action run by its body; an optional one is omitted from
// the call, never sent as an invented value.
func TestToolCallInputsFollowTheParameters(t *testing.T) {
	t.Run("optional omitted", func(t *testing.T) {
		ctx, scope := analysisFixture(t, scaleModel)
		runner := &recordingRunner{answer: map[string]ToolValue{"y": {Value: toolReal(4)}}}
		ctx.SetToolRunner(runner)
		if _, err := ctx.ExecuteAction(calcNamed(t, scope, "Scaled")); err != nil {
			t.Fatalf("ExecuteAction: %v", err)
		}
		if len(runner.calls) != 1 || len(runner.calls[0].Inputs) != 1 || runner.calls[0].Inputs[0].Variable != "k" {
			t.Fatalf("calls %+v, want one sending k alone", runner.calls)
		}
	})
	t.Run("optional sent", func(t *testing.T) {
		ctx, scope := analysisFixture(t, scaleModel)
		runner := &recordingRunner{answer: map[string]ToolValue{"y": {Value: toolReal(4)}}}
		ctx.SetToolRunner(runner)
		if _, err := ctx.ExecuteAction(calcNamed(t, scope, "Biased")); err != nil {
			t.Fatalf("ExecuteAction: %v", err)
		}
		if len(runner.calls) != 1 || len(runner.calls[0].Inputs) != 2 {
			t.Fatalf("calls %+v, want one sending k and bias", runner.calls)
		}
	})
	t.Run("required unbound", func(t *testing.T) {
		ctx, scope := analysisFixture(t, scaleModel)
		runner := &recordingRunner{answer: map[string]ToolValue{"y": {Value: toolReal(4)}}}
		ctx.SetToolRunner(runner)
		_, err := ctx.ExecuteAction(calcNamed(t, scope, "Unscaled"))
		if !errors.Is(err, ErrUnboundParameter) || !strings.Contains(err.Error(), "input parameter k is bound by no argument") {
			t.Fatalf("ExecuteAction = %v, want ErrUnboundParameter for k", err)
		}
		if len(runner.calls) != 0 {
			t.Fatalf("tool ran %d times for an action missing a required input", len(runner.calls))
		}
	})
}

// Two parameters carrying one ToolVariable name would share a key on the wire: the
// performance is a ToolError naming both, and the tool is not run.
func TestToolCallRefusesAmbiguousVariables(t *testing.T) {
	cases := map[string]struct {
		action, detail string
		inputs         map[string]Value
	}{
		"two inputs":   {"TwoInputs", "x names both a and b", map[string]Value{"a": realOf(1), "b": realOf(2)}},
		"two outputs":  {"TwoOutputs", "y names both y and z", map[string]Value{"a": realOf(1)}},
		"in and out":   {"InAndOut", "x names both a and y", map[string]Value{"a": realOf(1)}},
		"via a driver": {"Collides", "x names both a and y", nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, scope := analysisFixture(t, scaleModel)
			runner := &recordingRunner{answer: map[string]ToolValue{"y": {Value: toolReal(1)}}}
			ctx.SetToolRunner(runner)
			_, err := ctx.ExecuteActionWithInputs(calcNamed(t, scope, tc.action), tc.inputs)
			var fault *ToolError
			if !errors.As(err, &fault) || fault.Kind != ToolAmbiguousVariable || !strings.Contains(fault.Detail, tc.detail) {
				t.Fatalf("ExecuteAction = %v, want ToolError %q", err, tc.detail)
			}
			if len(runner.calls) != 0 {
				t.Fatalf("tool ran %d times under an ambiguous variable", len(runner.calls))
			}
		})
	}
}

// A debugger's executor of a tool-computed action is the same performance: created, the tool
// has run once and the executor is completed with its outputs, with no flow to step; without
// a runner its creation is the same refusal a run's performance is.
func TestToolExecutionThroughTheDebuggerExecutor(t *testing.T) {
	ctx, scope := analysisFixture(t, scaleModel)
	runner := &recordingRunner{answer: map[string]ToolValue{"y": {Value: toolReal(4)}}}
	ctx.SetToolRunner(runner)
	scale := calcNamed(t, scope, "Doubling")

	exec, err := ctx.CreateActionExecutor(scale)
	if err != nil {
		t.Fatalf("CreateActionExecutor: %v", err)
	}
	defer exec.Release()
	if exec.State() != StateCompleted || len(exec.Tokens()) != 0 {
		t.Fatalf("state %s with %d tokens, want completed with none", exec.State(), len(exec.Tokens()))
	}
	if len(runner.calls) != 1 {
		t.Fatalf("tool invoked %d times, want once", len(runner.calls))
	}
	if got := FormatValue(exec.Results()["y"]); got != "4.0" {
		t.Fatalf("y = %s, want 4.0", got)
	}
	if err := exec.Step(); err != nil || len(runner.calls) != 1 {
		t.Fatalf("stepping a completed executor: %v, %d invocations", err, len(runner.calls))
	}

	bare, _ := analysisFixture(t, scaleModel)
	_, err = bare.CreateActionExecutor(scale)
	if !errors.Is(err, ErrToolNotRegistered) {
		t.Fatalf("CreateActionExecutor without a runner = %v, want ErrToolNotRegistered", err)
	}
}

// annotatedUsageModel carries ToolExecution on performed usages rather than definitions,
// on a definition whose toolName is empty, and on the action an object performs.
const annotatedUsageModel = `package test {
	private import ScalarValues::Real;
	private import AnalysisTooling::*;

	action def Plain { in k : Real; out y : Real; }
	action def Stepped { in k : Real; out y : Real; first start; then done; }

	action def UsageOfPlain {
		out y : Real;
		action s : Plain {
			metadata ToolExecution { toolName = "MC"; uri = "u"; }
			in k :>> k = 2.0 { @ToolVariable { name = "k"; } }
			out y :>> y      { @ToolVariable { name = "y"; } }
		}
		bind y = s.y;
	}
	action def UsageOfStepped {
		out y : Real;
		action s : Stepped {
			metadata ToolExecution { toolName = "MC"; uri = "u"; }
			in k :>> k = 2.0 { @ToolVariable { name = "k"; } }
			out y :>> y      { @ToolVariable { name = "y"; } }
		}
		bind y = s.y;
	}

	action def Nameless {
		metadata ToolExecution { toolName = ""; uri = "u"; }
		out y : Real = 1.0;
		first start; then done;
	}

	action def Doubling {
		metadata ToolExecution { toolName = "MC"; uri = "u"; }
		in k : Real = 2.0 { @ToolVariable { name = "k"; } }
		out y : Real     { @ToolVariable { name = "y"; } }
	}
	part def Rig { perform action scale : Doubling; }
	part def AnnotatedRig {
		perform action scale : Stepped {
			metadata ToolExecution { toolName = "MC"; uri = "u"; }
			in k :>> k = 2.0 { @ToolVariable { name = "k"; } }
			out y :>> y      { @ToolVariable { name = "y"; } }
		}
	}
}`

// A ToolExecution on a usage typed by a definition holds whether the definition states a
// body or not: the usage's own parameters name the variables, and the body never runs.
func TestToolExecutionOnATypedUsage(t *testing.T) {
	for _, driver := range []string{"UsageOfPlain", "UsageOfStepped"} {
		t.Run(driver, func(t *testing.T) {
			ctx, scope := analysisFixture(t, annotatedUsageModel)
			runner := &recordingRunner{answer: map[string]ToolValue{"y": {Value: toolReal(4)}}}
			ctx.SetToolRunner(runner)
			out, err := ctx.ExecuteAction(calcNamed(t, scope, driver))
			if err != nil {
				t.Fatalf("ExecuteAction: %v", err)
			}
			if len(runner.calls) != 1 {
				t.Fatalf("tool invoked %d times, want once", len(runner.calls))
			}
			call := runner.calls[0]
			if call.Action.Name != "s" || len(call.Inputs) != 1 || call.Inputs[0].Variable != "k" || !nearly(call.Inputs[0].Value.Value, toolReal(2)) {
				t.Fatalf("call %+v, want s sending k = 2.0", call)
			}
			if got := FormatValue(out["y"]); got != "4.0" {
				t.Fatalf("y = %s, want 4.0", got)
			}

			bare, bareScope := analysisFixture(t, annotatedUsageModel)
			if _, err := bare.ExecuteAction(calcNamed(t, bareScope, driver)); !errors.Is(err, ErrToolNotRegistered) {
				t.Fatalf("ExecuteAction without a runner = %v, want ErrToolNotRegistered", err)
			}
		})
	}
}

// A ToolExecution naming no tool is still one: the action is refused as not registered,
// with or without a runner, and its body does not stand in.
func TestToolExecutionWithAnEmptyNameRunsNoBody(t *testing.T) {
	for _, withRunner := range []bool{true, false} {
		ctx, scope := analysisFixture(t, annotatedUsageModel)
		runner := &recordingRunner{answer: map[string]ToolValue{}}
		if withRunner {
			ctx.SetToolRunner(runner)
		}
		out, err := ctx.ExecuteAction(calcNamed(t, scope, "Nameless"))
		var refused *ToolNotRegisteredError
		if !errors.As(err, &refused) || refused.Tool != "" {
			t.Fatalf("runner attached %v: ExecuteAction = %v, %v; want tool '' not registered", withRunner, out, err)
		}
		if len(runner.calls) != 0 {
			t.Fatalf("tool invoked %d times for an empty toolName", len(runner.calls))
		}
	}
}

// An object performing an annotated action performs it by the tool, the annotation on the
// definition it names or on the performed usage itself; without a runner it is not created.
func TestToolExecutionOfAPerformedAction(t *testing.T) {
	for _, rig := range []string{"Rig", "AnnotatedRig"} {
		t.Run(rig, func(t *testing.T) {
			ctx, scope := analysisFixture(t, annotatedUsageModel)
			runner := &recordingRunner{answer: map[string]ToolValue{"y": {Value: toolReal(4)}}}
			ctx.SetToolRunner(runner)
			inst, err := ctx.Instantiate(calcNamed(t, scope, rig))
			if err != nil {
				t.Fatalf("Instantiate: %v", err)
			}
			if len(runner.calls) != 1 {
				t.Fatalf("tool invoked %d times, want once", len(runner.calls))
			}
			behavior, ok := inst.Behavior("scale")
			if !ok || behavior.Action == nil || behavior.Action.State() != StateCompleted {
				t.Fatalf("scale is not a completed performance: %v", inst.Behaviors())
			}
			if got := FormatValue(performedFeature(t, ctx, inst, "scale", "y")); got != "4.0" {
				t.Fatalf("scale.y = %s, want 4.0", got)
			}

			bare, bareScope := analysisFixture(t, annotatedUsageModel)
			if _, err := bare.Instantiate(calcNamed(t, bareScope, rig)); !errors.Is(err, ErrToolNotRegistered) {
				t.Fatalf("Instantiate without a runner = %v, want ErrToolNotRegistered", err)
			}
		})
	}
}
