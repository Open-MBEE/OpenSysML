package runtime

import (
	"errors"
	"testing"
)

// TestRuntimeRobustnessToolCalc exercises the failure modes of a calc annotated
// ToolExecution: every one is the typed error of its kind, never a panic, a hang
// or a body the calc was not to run.
func TestRuntimeRobustnessToolCalc(t *testing.T) {
	t.Run("no_runner", func(t *testing.T) {
		ctx, scope := analysisFixture(t, toolCalcModel)
		_, err := ctx.InvokeCalc(calcNamed(t, scope, "Thermal"), thermalArgs(t, ctx, scope), scope)
		if !errors.Is(err, ErrToolNotRegistered) {
			t.Fatalf("InvokeCalc = %v, want ErrToolNotRegistered", err)
		}
	})
	t.Run("no_runner_usage", func(t *testing.T) {
		ctx, scope := analysisFixture(t, toolCalcModel)
		_, err := ctx.CalcUsageOutputs(calcNamed(t, scope, "t"), scope, nil)
		if !errors.Is(err, ErrToolNotRegistered) {
			t.Fatalf("CalcUsageOutputs = %v, want ErrToolNotRegistered", err)
		}
	})
	t.Run("runner_fault_usage", func(t *testing.T) {
		ctx, scope := analysisFixture(t, toolCalcModel)
		fault := &ToolError{Tool: "Thermo", Kind: ToolProcessFailed, Detail: "exit 2"}
		ctx.SetToolRunner(&recordingRunner{err: fault})
		_, err := ctx.CalcUsageOutputs(calcNamed(t, scope, "t"), scope, nil)
		if !errors.Is(err, fault) {
			t.Fatalf("CalcUsageOutputs = %v, want %v", err, fault)
		}
	})
	t.Run("unbound_input", func(t *testing.T) {
		// A required input no argument and no usage binding supplies is
		// ErrUnboundParameter, as a body-run calc reports it, and no tool runs.
		ctx, scope := analysisFixture(t, toolCalcRobustnessModel)
		runner := &recordingRunner{answer: map[string]ToolValue{}}
		ctx.SetToolRunner(runner)
		_, err := ctx.CalcUsageOutputs(calcNamed(t, scope, "tbare"), scope, nil)
		if !errors.Is(err, ErrUnboundParameter) {
			t.Fatalf("CalcUsageOutputs = %v, want ErrUnboundParameter", err)
		}
		if len(runner.calls) != 0 {
			t.Fatalf("tool ran %d times for an unbound input", len(runner.calls))
		}
	})
	t.Run("ambiguous_variable", func(t *testing.T) {
		ctx, scope := analysisFixture(t, toolCalcRobustnessModel)
		runner := &recordingRunner{answer: map[string]ToolValue{"y": {Value: toolReal(1)}}}
		ctx.SetToolRunner(runner)
		_, err := ctx.InvokeCalc(calcNamed(t, scope, "Colliding"), []Value{realOf(1), realOf(2)}, scope)
		var failure *ToolError
		if !errors.As(err, &failure) || failure.Kind != ToolAmbiguousVariable {
			t.Fatalf("InvokeCalc = %v, want ToolAmbiguousVariable", err)
		}
		if len(runner.calls) != 0 {
			t.Fatalf("tool ran %d times under an ambiguous variable", len(runner.calls))
		}
	})
	t.Run("ambiguous_result", func(t *testing.T) {
		// Two bare outs answered by the tool designate no result, as two bound
		// outputs do for a body that returned nothing.
		ctx, scope := analysisFixture(t, toolCalcModel)
		ctx.SetToolRunner(&recordingRunner{answer: map[string]ToolValue{
			"ok": {Value: semanticsBool(true)},
			"n":  {Value: toolReal(3)},
		}})
		_, err := ctx.InvokeCalc(calcNamed(t, scope, "TwoOuts"), []Value{realOf(1)}, scope)
		if !errors.Is(err, ErrAmbiguousResult) {
			t.Fatalf("InvokeCalc = %v, want ErrAmbiguousResult", err)
		}
	})
	t.Run("unanswered_output", func(t *testing.T) {
		// A reply the tool bound before the run ended reaches no binding: the
		// output read asks the run, which answers the reply's failure as a typed
		// error rather than evaluating the calc's bindings.
		ctx, scope := analysisFixture(t, toolCalcRobustnessModel)
		ctx.SetToolRunner(dodgyCalcRunner{})
		_, err := ctx.CalcUsageOutput(calcNamed(t, scope, "tw"), "warn", scope, nil)
		var failure *ToolError
		if !errors.As(err, &failure) || failure.Kind != ToolMissingOutput {
			t.Fatalf("CalcUsageOutput = %v, want ToolMissingOutput", err)
		}
	})
}

// dodgyCalcRunner answers the call's outputs except one, bypassing Bind's check,
// as a runner is free to do.
type dodgyCalcRunner struct{}

func (dodgyCalcRunner) RunTool(call *ToolCall) (ToolAnswer, error) {
	return ToolAnswer{Outputs: map[string]Value{"result": {Kind: ValConst, Const: toolReal(1)}}}, nil
}

// toolCalcRobustnessModel is the failure-mode calcs: one whose usage binds no
// input, one whose inputs share a ToolVariable name, and one a runner can leave
// an output of unanswered.
const toolCalcRobustnessModel = `package test {
	private import ScalarValues::*;
	private import AnalysisTooling::*;
	private import ISQ::*;

	calc def Needing {
		metadata ToolExecution { toolName = "Thermo"; uri = "u"; }
		in m : MassValue { @ToolVariable { name = "mass"; } }
		return : TemperatureValue { @ToolVariable { name = "Tmax"; } }
	}
	calc tbare : Needing;

	calc def Colliding {
		metadata ToolExecution { toolName = "Thermo"; uri = "u"; }
		in a : Real  { @ToolVariable { name = "x"; } }
		in b : Real  { @ToolVariable { name = "x"; } }
		out y : Real { @ToolVariable { name = "y"; } }
	}

	calc def Tw {
		metadata ToolExecution { toolName = "Thermo"; uri = "u"; }
		out warn : Boolean { @ToolVariable { name = "warn"; } }
		return : Real { @ToolVariable { name = "r"; } }
	}
	calc tw : Tw;
}`
