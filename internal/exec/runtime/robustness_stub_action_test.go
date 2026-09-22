package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessStubAction exercises a declared action that computes
// nothing — parameters and no body, as a v1 call action naming no behavior
// migrates to — whose outputs a flow reads after it fires.
func TestRuntimeRobustnessStubAction(t *testing.T) {
	t.Run("optional_output_flows_no_value", testStubActionOptionalOutputFlowsNoValue)
	t.Run("required_output_never_written", testStubActionRequiredOutputNeverWritten)
	t.Run("required_target_gets_no_value", testStubActionRequiredTargetGetsNoValue)
	t.Run("optional_output_read_by_path", testStubActionOptionalOutputReadByPath)
	t.Run("optional_result_read_as_value", testStubActionOptionalResultReadAsValue)
	t.Run("required_result_read_as_value", testStubActionRequiredResultReadAsValue)
}

// executeChain builds src with the standard libraries and executes its action `chain`.
func executeChain(t *testing.T, src string) (map[string]Value, error) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "chain", ast.DefAction)
	if sym == nil {
		t.Fatal("action chain not found")
	}
	return ctx.ExecuteAction(sym)
}

// stubChain is an action performing a stub with `out reading : Integer` of the
// given multiplicity ahead of a consumer with `in value : Integer` of another, the
// stub's output flowing to the consumer's input.
func stubChain(outMult, inMult, consumerBody string) string {
	return stubChainNamed("reading", outMult, inMult, consumerBody)
}

// stubChainNamed is stubChain with the stub's output named out; `result` makes
// it what the stub stands for when read as a value.
func stubChainNamed(out, outMult, inMult, consumerBody string) string {
	return `package test {
		private import SequenceFunctions::*;
		action chain {
			attribute seen : Integer = -1;
			action stub {
				in gain : Integer = 2;
				out ` + out + ` : Integer` + outMult + `;
			}
			action consumer {
				in value : Integer` + inMult + `;
				` + consumerBody + `
			}
			succession first start then stub;
			succession first stub then consumer;
			succession first consumer then done;
			flow stub.` + out + ` to consumer.value;
		}
	}`
}

// An output declared admitting no value that the stub never writes flows nothing:
// the consumer runs with the input empty and the action completes.
func testStubActionOptionalOutputFlowsNoValue(t *testing.T) {
	outputs, err := executeChain(t, stubChain("[0..1]", "[0..1]",
		"assign seen := value->size();"))
	if err != nil {
		t.Fatalf("ExecuteAction failed: %v", err)
	}
	if got := outputs["seen"]; got.Kind != ValConst || got.Const.Int != 0 {
		t.Fatalf("seen = %v, want 0 from an empty input", got)
	}
}

// An output declared holding a value that the stub never writes is a flow source
// with nothing to carry, which the run reports rather than passing silently.
func testStubActionRequiredOutputNeverWritten(t *testing.T) {
	_, err := executeChain(t, stubChain("", "[0..1]",
		"assign seen := value->size();"))
	if !errors.Is(err, ErrFlowSource) || !strings.Contains(err.Error(), "node stub produced no value at reading") {
		t.Fatalf("error = %v, want ErrFlowSource from an output never written", err)
	}
}

// A consumer input declared holding a value gets none when the stub's optional
// output is empty, which reading it reports as no value rather than as empty.
func testStubActionRequiredTargetGetsNoValue(t *testing.T) {
	_, err := executeChain(t, stubChain("[0..1]", "",
		"assign seen := value;"))
	var noValue *NoValueError
	if !errors.As(err, &noValue) || noValue.Feature != "value" {
		t.Fatalf("error = %v, want NoValueError for value", err)
	}
}

// A sibling reading the stub's empty optional output by path, `stub.reading`,
// reads the empty sequence rather than failing.
func testStubActionOptionalOutputReadByPath(t *testing.T) {
	outputs, err := executeChain(t, stubChain("[0..1]", "[0..1]",
		"assign seen := stub.reading->size() + value->size();"))
	if err != nil {
		t.Fatalf("ExecuteAction failed: %v", err)
	}
	if got := outputs["seen"]; got.Kind != ValConst || got.Const.Int != 0 {
		t.Fatalf("seen = %v, want 0 from two empty reads", got)
	}
}

// Reading the stub itself as a value reads its result: an optional result left
// empty reads as the empty sequence, just as `stub.result` does.
func testStubActionOptionalResultReadAsValue(t *testing.T) {
	outputs, err := executeChain(t, stubChainNamed("result", "[0..1]", "[0..1]",
		"assign seen := stub->size() + stub.result->size();"))
	if err != nil {
		t.Fatalf("ExecuteAction failed: %v", err)
	}
	if got := outputs["seen"]; got.Kind != ValConst || got.Const.Int != 0 {
		t.Fatalf("seen = %v, want 0 from two empty reads of the result", got)
	}
}

// A required result left empty is still no value when read through the stub itself.
func testStubActionRequiredResultReadAsValue(t *testing.T) {
	_, err := executeChain(t, `package test {
		private import SequenceFunctions::*;
		action chain {
			attribute seen : Integer = -1;
			action stub { out result : Integer; }
			action consumer { assign seen := stub->size(); }
			succession first start then stub;
			succession first stub then consumer;
			succession first consumer then done;
		}
	}`)
	var noValue *NoValueError
	if !errors.As(err, &noValue) || !strings.HasSuffix(noValue.Feature, "stub.result") {
		t.Fatalf("error = %v, want NoValueError for stub.result", err)
	}
}
