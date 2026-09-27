package runtime

import (
	"testing"
)

// inheritedSequenceModel redefines a multi-valued output without restating its bound: the
// tool action's samples inherits [0..*] from the general action's.
const inheritedSequenceModel = `package test {
	private import ScalarValues::Real;
	private import AnalysisTooling::*;

	action def Sampling { out samples : Real[0..*]; first start; then done; }
	action def Sample : Sampling {
		metadata ToolExecution { toolName = "MC"; uri = "u"; }
		out :>> samples { @ToolVariable { name = "samples"; } }
	}
	action def RunSample {
		out samples : Real[0..*];
		action s : Sample {}
		bind samples = s.samples;
	}
}`

// A redefinition stating no bound governs by the one it inherits; a sequence binds to it.
func TestToolOutputSequenceToAnInheritedMultiplicity(t *testing.T) {
	ctx, scope := analysisFixture(t, inheritedSequenceModel)
	ctx.SetToolRunner(&recordingRunner{answer: map[string]ToolValue{
		"samples": toolSeq(ToolValue{Value: toolReal(1)}, ToolValue{Value: toolReal(2)}),
	}})
	out, err := ctx.ExecuteAction(calcNamed(t, scope, "RunSample"))
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if got := FormatValue(out["samples"]); got != "[1.0, 2.0]" {
		t.Fatalf("samples = %s, want [1.0, 2.0]", got)
	}
}
