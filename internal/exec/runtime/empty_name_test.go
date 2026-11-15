package runtime

import "testing"

// A result parameter with the empty name is a named one: the tool answers it under that name
// and the calculation returns the answer.
func TestToolCalcReturnsAnEmptyNamedResult(t *testing.T) {
	ctx, scope := analysisFixture(t, `package test {
	private import ScalarValues::*;
	private import AnalysisTooling::*;

	calc def Bare {
		metadata ToolExecution { toolName = "Thermo"; uri = "u"; }
		in a : Real[1] { @ToolVariable { name = "a"; } }
		return '' : Real[1];
	}

	calc def Varied {
		metadata ToolExecution { toolName = "Thermo"; uri = "u"; }
		in a : Real[1] { @ToolVariable { name = "a"; } }
		return '' : Real[1] { @ToolVariable { name = "T"; } }
	}
}`)
	for calc, variable := range map[string]string{"Bare": "", "Varied": "T"} {
		ctx.SetToolRunner(&recordingRunner{answer: map[string]ToolValue{variable: {Value: toolReal(7)}}})
		result, err := ctx.InvokeCalc(calcNamed(t, scope, calc), []Value{realOf(1)}, scope)
		if err != nil {
			t.Fatalf("%s: InvokeCalc: %v", calc, err)
		}
		if got := FormatValue(result); got != "7.0" {
			t.Errorf("%s: result = %s, want 7.0", calc, got)
		}
	}
}
