package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// toolCalcModel is a tool-computed calc definition and a calc usage of it, a calc
// whose body must never run, one naming no tool, a part reading the tool's result
// as a derived attribute, and a driver invoking it positionally.
const toolCalcModel = `package test {
	private import ScalarValues::*;
	private import AnalysisTooling::*;
	private import ISQ::*;

	calc def Thermal {
		metadata ToolExecution { toolName = "Thermo"; uri = "thermo://x"; }
		in m : MassValue   { @ToolVariable { name = "mass"; } }
		in p : PowerValue  { @ToolVariable { name = "power"; } }
		out warn : Boolean { @ToolVariable { name = "warn"; } }
		out rating : PowerValue { @ToolVariable { name = "rating"; } }
		return : TemperatureValue { @ToolVariable { name = "Tmax"; } }
	}

	calc def Poisoned {
		metadata ToolExecution { toolName = "Thermo"; uri = "u"; }
		in a : Real { @ToolVariable { name = "a"; } }
		return : Real = 1 / 0;
	}

	calc def Nameless {
		metadata ToolExecution { toolName = ""; uri = "u"; }
		in a : Real { @ToolVariable { name = "a"; } }
		return : Real;
	}

	calc t : Thermal {
		in m = 2 [SI::kg];
		in p = 10 [SI::W];
	}

	calc def Warned {
		metadata ToolExecution { toolName = "Thermo"; uri = "w"; }
		in m : MassValue   { @ToolVariable { name = "mass"; } }
		in p : PowerValue  { @ToolVariable { name = "power"; } }
		out warn : Boolean { @ToolVariable { name = "warn"; } }
		out rating : PowerValue { @ToolVariable { name = "rating"; } }
	}

	calc w : Warned {
		in m = 2 [SI::kg];
		in p = 10 [SI::W];
	}

	part def Board {
		attribute mass : MassValue = 2 [SI::kg];
		attribute power : PowerValue = 10 [SI::W];
		attribute Tmax : TemperatureValue = Thermal(mass, power);
	}

	calc def Driven {
		return : TemperatureValue = Thermal(2 [SI::kg], 10 [SI::W]);
	}
}`

// evalText evaluates one expression in scope, for building invocation arguments.
func evalText(t *testing.T, ctx *Context, scope *symbols.Scope, text string) Value {
	t.Helper()
	expr, ok, err := ctx.model.parseOneExpression("<test>", text)
	if err != nil || !ok {
		t.Fatalf("parsing %q: %v", text, err)
	}
	value, err := NewEvalContextIn(ctx, scope, nil).Eval(expr)
	if err != nil {
		t.Fatalf("evaluating %q: %v", text, err)
	}
	return value
}

// thermalArgs is the invocation's two quantities, as a caller evaluates them.
func thermalArgs(t *testing.T, ctx *Context, scope *symbols.Scope) []Value {
	t.Helper()
	return []Value{evalText(t, ctx, scope, "2 [SI::kg]"), evalText(t, ctx, scope, "10 [SI::W]")}
}

// A calc annotated ToolExecution is computed by the tool: the inputs arrive under
// their ToolVariable names, and the result parameter is bound from the answer — the
// calculation's return — while the calc states no body at all.
func TestToolCalcBindsTheResultFromTheTool(t *testing.T) {
	ctx, scope := analysisFixture(t, toolCalcModel)
	runner := &recordingRunner{answer: map[string]ToolValue{
		"Tmax":   {Value: toolReal(340), Unit: "K"},
		"warn":   {Value: semanticsBool(true)},
		"rating": {Value: toolReal(30), Unit: "W"},
	}}
	ctx.SetToolRunner(runner)

	result, err := ctx.InvokeCalc(calcNamed(t, scope, "Thermal"), thermalArgs(t, ctx, scope), scope)
	if err != nil {
		t.Fatalf("InvokeCalc: %v", err)
	}
	if got := FormatValue(result); got != "340.0 [SI::K]" {
		t.Fatalf("result = %s, want 340.0 [SI::K]", got)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("tool invoked %d times, want once", len(runner.calls))
	}
	call := runner.calls[0]
	if call.ToolName != "Thermo" || call.URI != "thermo://x" {
		t.Fatalf("call names %q at %q", call.ToolName, call.URI)
	}
	if call.Action == nil || call.Action.Name != "Thermal" {
		t.Fatalf("call.Action = %v, want the Thermal definition", call.Action)
	}
	inputs := map[string]ToolValue{}
	for _, in := range call.Inputs {
		inputs[in.Variable] = in.Value
	}
	if got, ok := inputs["mass"]; !ok || got.Unit != "kg" || !nearly(got.Value, toolReal(2)) {
		t.Fatalf("inputs %v, want mass = 2 kg", inputs)
	}
	if got, ok := inputs["power"]; !ok || got.Unit != "W" || !nearly(got.Value, toolReal(10)) {
		t.Fatalf("inputs %v, want power = 10 W", inputs)
	}
	var outputs []string
	for _, out := range call.Outputs {
		outputs = append(outputs, out.Variable)
	}
	if strings.Join(outputs, ",") != "Tmax,rating,warn" {
		t.Fatalf("outputs %v, want Tmax, rating and warn", outputs)
	}
}

func semanticsBool(b bool) semantics.Value {
	return semantics.Value{Kind: semantics.ValBool, Bool: b}
}

// A calc usage of the annotated definition reads its outputs from the tool's
// answer: `warn` under its own name, the anonymous result under `result`, and the
// usage's own input bindings are what the tool was sent.
func TestToolCalcUsageReadsItsOutputsFromTheTool(t *testing.T) {
	ctx, scope := analysisFixture(t, toolCalcModel)
	runner := &recordingRunner{answer: map[string]ToolValue{
		"Tmax":   {Value: toolReal(340), Unit: "K"},
		"warn":   {Value: semanticsBool(false)},
		"rating": {Value: toolReal(30), Unit: "W"},
	}}
	ctx.SetToolRunner(runner)

	outputs, err := ctx.CalcUsageOutputs(calcNamed(t, scope, "t"), scope, nil)
	if err != nil {
		t.Fatalf("CalcUsageOutputs: %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("tool invoked %d times, want once", len(runner.calls))
	}
	if len(runner.calls[0].Inputs) != 2 {
		t.Fatalf("inputs %+v, want mass and power", runner.calls[0].Inputs)
	}
	got := map[string]string{}
	for _, out := range outputs {
		got[out.Name] = FormatValue(out.Value)
	}
	if got["warn"] != "false" || got["rating"] != "30.0 [SI::W]" {
		t.Fatalf("outputs %v, want warn = false and rating = 30 W", got)
	}
	result, err := ctx.CalcUsageOutput(calcNamed(t, scope, "t"), "result", scope, nil)
	if err != nil {
		t.Fatalf("CalcUsageOutput(result): %v", err)
	}
	if FormatValue(result) != "340.0 [SI::K]" {
		t.Fatalf("result = %s, want 340.0 [SI::K]", FormatValue(result))
	}
}

// A calc declaring only `out` parameters asks the tool for those alone — no
// `result` output is demanded — and its usage reads them as any usage's.
func TestToolCalcOutOnlyAsksNoResult(t *testing.T) {
	ctx, scope := analysisFixture(t, toolCalcModel)
	runner := &recordingRunner{answer: map[string]ToolValue{
		"warn":   {Value: semanticsBool(true)},
		"rating": {Value: toolReal(30), Unit: "W"},
	}}
	ctx.SetToolRunner(runner)

	outputs, err := ctx.CalcUsageOutputs(calcNamed(t, scope, "w"), scope, nil)
	if err != nil {
		t.Fatalf("CalcUsageOutputs: %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("tool invoked %d times, want once", len(runner.calls))
	}
	for _, out := range runner.calls[0].Outputs {
		if out.Variable == "result" {
			t.Fatalf("call demands a result output from an out-only calc: %+v", runner.calls[0].Outputs)
		}
	}
	got := map[string]string{}
	for _, out := range outputs {
		got[out.Name] = FormatValue(out.Value)
	}
	if got["warn"] != "true" || got["rating"] != "30.0 [SI::W]" {
		t.Fatalf("outputs %v, want warn = true and rating = 30 W", got)
	}
}

// The answer's unit is converted to the result parameter's coherent unit; one of
// another dimension is the reply's failure, a malformed ToolError.
func TestToolCalcConvertsTheAnsweredUnit(t *testing.T) {
	t.Run("converted", func(t *testing.T) {
		ctx, scope := analysisFixture(t, toolCalcModel)
		ctx.SetToolRunner(&recordingRunner{answer: map[string]ToolValue{
			"Tmax":   {Value: toolReal(340), Unit: "K"},
			"warn":   {Value: semanticsBool(true)},
			"rating": {Value: toolReal(3), Unit: "kW"},
		}})
		outputs, err := ctx.CalcUsageOutputs(calcNamed(t, scope, "t"), scope, nil)
		if err != nil {
			t.Fatalf("CalcUsageOutputs: %v", err)
		}
		got := map[string]string{}
		for _, out := range outputs {
			got[out.Name] = FormatValue(out.Value)
		}
		if got["rating"] != "3000.0 [SI::W]" {
			t.Fatalf("rating = %s, want 3000.0 [SI::W]", got["rating"])
		}
	})
	t.Run("wrong dimension", func(t *testing.T) {
		ctx, scope := analysisFixture(t, toolCalcModel)
		ctx.SetToolRunner(&recordingRunner{answer: map[string]ToolValue{
			"Tmax":   {Value: toolReal(340), Unit: "kg"},
			"warn":   {Value: semanticsBool(true)},
			"rating": {Value: toolReal(30), Unit: "W"},
		}})
		_, err := ctx.InvokeCalc(calcNamed(t, scope, "Thermal"), thermalArgs(t, ctx, scope), scope)
		var failure *ToolError
		if !errors.As(err, &failure) || failure.Kind != ToolMalformed {
			t.Fatalf("InvokeCalc = %v, want a malformed ToolError", err)
		}
	})
}

// A derived attribute bound to the tool-computed calc reads the tool's answer:
// the attribute's evaluation invokes the calc, which goes to the tool.
func TestToolCalcThroughADerivedAttribute(t *testing.T) {
	ctx, scope := analysisFixture(t, toolCalcModel)
	runner := &recordingRunner{answer: map[string]ToolValue{
		"Tmax":   {Value: toolReal(340), Unit: "K"},
		"warn":   {Value: semanticsBool(true)},
		"rating": {Value: toolReal(30), Unit: "W"},
	}}
	ctx.SetToolRunner(runner)

	inst, err := ctx.Instantiate(calcNamed(t, scope, "Board"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	fv, err := inst.GetFeatureValue(ctx, "Tmax")
	if err != nil {
		t.Fatalf("GetFeatureValue(Tmax): %v", err)
	}
	if got := FormatValue(fv.HeldValue()); got != "340.0 [SI::K]" {
		t.Fatalf("Tmax = %s, want 340.0 [SI::K]", got)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("tool invoked %d times, want once", len(runner.calls))
	}
}

// The body is never evaluated: the calc stating a result expression that cannot
// evaluate is computed by the tool all the same.
func TestToolCalcNeverRunsTheBody(t *testing.T) {
	ctx, scope := analysisFixture(t, toolCalcModel)
	runner := &recordingRunner{answer: map[string]ToolValue{"result": {Value: toolReal(4)}}}
	ctx.SetToolRunner(runner)

	result, err := ctx.InvokeCalc(calcNamed(t, scope, "Poisoned"), []Value{realOf(1)}, scope)
	if err != nil {
		t.Fatalf("InvokeCalc: %v", err)
	}
	if got := FormatValue(result); got != "4.0" {
		t.Fatalf("result = %s, want the tool's 4.0, not the body's 1 / 0", got)
	}
	if len(runner.calls) != 1 || len(runner.calls[0].Inputs) != 1 || runner.calls[0].Inputs[0].Variable != "a" {
		t.Fatalf("calls %+v, want one sending a", runner.calls)
	}
}

// Every refusal is the performance's unchanged: no runner or an empty toolName is
// not registered, the runner's own failure propagates as is, and a reply missing
// or adding an output is a ToolError of its kind.
func TestToolCalcRefusesLikeAPerformance(t *testing.T) {
	args := func(t *testing.T, ctx *Context, scope *symbols.Scope) []Value {
		t.Helper()
		return thermalArgs(t, ctx, scope)
	}
	t.Run("no runner", func(t *testing.T) {
		ctx, scope := analysisFixture(t, toolCalcModel)
		_, err := ctx.InvokeCalc(calcNamed(t, scope, "Thermal"), args(t, ctx, scope), scope)
		var refusal *ToolNotRegisteredError
		if !errors.As(err, &refusal) || !errors.Is(err, ErrToolNotRegistered) || refusal.Tool != "Thermo" {
			t.Fatalf("InvokeCalc = %v, want ToolNotRegisteredError", err)
		}
	})
	t.Run("empty tool name", func(t *testing.T) {
		ctx, scope := analysisFixture(t, toolCalcModel)
		runner := &recordingRunner{answer: map[string]ToolValue{"result": {Value: toolReal(1)}}}
		ctx.SetToolRunner(runner)
		_, err := ctx.InvokeCalc(calcNamed(t, scope, "Nameless"), []Value{realOf(1)}, scope)
		var refusal *ToolNotRegisteredError
		if !errors.As(err, &refusal) || refusal.Tool != "" {
			t.Fatalf("InvokeCalc = %v, want tool '' not registered", err)
		}
		if len(runner.calls) != 0 {
			t.Fatalf("tool invoked %d times for an empty toolName", len(runner.calls))
		}
	})
	t.Run("runner failure", func(t *testing.T) {
		ctx, scope := analysisFixture(t, toolCalcModel)
		fault := &ToolError{Tool: "Thermo", Kind: ToolTimeout, Detail: "10s"}
		ctx.SetToolRunner(&recordingRunner{err: fault})
		_, err := ctx.InvokeCalc(calcNamed(t, scope, "Thermal"), args(t, ctx, scope), scope)
		if !errors.Is(err, fault) {
			t.Fatalf("InvokeCalc = %v, want %v", err, fault)
		}
	})
	t.Run("missing output", func(t *testing.T) {
		ctx, scope := analysisFixture(t, toolCalcModel)
		ctx.SetToolRunner(&recordingRunner{answer: map[string]ToolValue{
			"warn":   {Value: semanticsBool(true)},
			"rating": {Value: toolReal(30), Unit: "W"},
		}})
		_, err := ctx.InvokeCalc(calcNamed(t, scope, "Thermal"), args(t, ctx, scope), scope)
		var failure *ToolError
		if !errors.As(err, &failure) || failure.Kind != ToolMissingOutput {
			t.Fatalf("InvokeCalc = %v, want ToolMissingOutput", err)
		}
	})
	t.Run("unknown output", func(t *testing.T) {
		ctx, scope := analysisFixture(t, toolCalcModel)
		ctx.SetToolRunner(&recordingRunner{answer: map[string]ToolValue{
			"Tmax":   {Value: toolReal(340), Unit: "K"},
			"warn":   {Value: semanticsBool(true)},
			"rating": {Value: toolReal(30), Unit: "W"},
			"z":      {Value: toolReal(1)},
		}})
		_, err := ctx.InvokeCalc(calcNamed(t, scope, "Thermal"), args(t, ctx, scope), scope)
		var failure *ToolError
		if !errors.As(err, &failure) || failure.Kind != ToolUnknownOutput {
			t.Fatalf("InvokeCalc = %v, want ToolUnknownOutput", err)
		}
	})
}

// A runner reporting unequal answers for equal inputs leaves the run a
// ToolDivergence, as a performance's does.
func TestToolCalcNotesDivergence(t *testing.T) {
	ctx, scope := analysisFixture(t, toolCalcModel)
	ctx.SetToolRunner(divergingCalcRunner{})
	if _, err := ctx.InvokeCalc(calcNamed(t, scope, "Thermal"), thermalArgs(t, ctx, scope), scope); err != nil {
		t.Fatalf("InvokeCalc: %v", err)
	}
	notes := ctx.Notes()
	if len(notes) != 1 {
		t.Fatalf("notes = %v, want one divergence", notes)
	}
	d, ok := notes[0].(ToolDivergence)
	if !ok || d.Tool != "Thermo" || !strings.Contains(d.Describe(), "answered differently for equal inputs") {
		t.Fatalf("note = %#v", notes[0])
	}
}

// divergingCalcRunner answers the calc's call and reports the answer as changed.
type divergingCalcRunner struct{}

func (divergingCalcRunner) RunTool(call *ToolCall) (ToolAnswer, error) {
	outputs, err := call.Bind(map[string]ToolValue{
		"Tmax":   {Value: toolReal(340), Unit: "K"},
		"warn":   {Value: semanticsBool(true)},
		"rating": {Value: toolReal(30), Unit: "W"},
	})
	return ToolAnswer{Outputs: outputs, Diverged: true}, err
}
