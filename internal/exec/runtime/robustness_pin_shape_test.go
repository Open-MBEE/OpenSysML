package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// TestRuntimeRobustnessPinShape exercises the shape a performed action's pin holds its
// values in: a [1] pin refuses more than one value wherever the values come from, a bare
// pin holds one value as a one-element sequence that any consumer of one value accepts,
// and two values in a bare pin are not one operand.
func TestRuntimeRobustnessPinShape(t *testing.T) {
	t.Run("scalar_pin_refuses_two_values", testPinShapeScalarRefusesTwoValues)
	t.Run("scalar_pin_assigned_from_a_two_element_pin_is_refused", testPinShapeScalarAssignedTwoElements)
	t.Run("bare_pin_holding_one_object_addresses_a_send", testPinShapeBarePinAddressesSend)
	t.Run("two_values_in_a_bare_pin_are_no_operand", testPinShapeTwoValuesAreNoOperand)
	t.Run("undetermined_count_survives_a_multi_valued_target", testPinShapeUndeterminedSurvives)
	t.Run("multi_valued_tool_input_keeps_its_one_element_sequence", testPinShapeToolInputKeepsSequence)
}

func pinShapeModel(pins, body string) string {
	return `package test {
		private import ScalarValues::*;
		action def Step {
			` + pins + `
			first start;
			then action s { ` + body + ` }
			then done;
		}
		action def Probe {
			out result : Integer[1] = 0;
			first start;
			then action step : Step { in value = (1, 2); }
			then action read { assign result := step.out1; }
			then done;
		}
	}`
}

func testPinShapeScalarRefusesTwoValues(t *testing.T) {
	_, err := executeActionSource(t, "Probe", pinShapeModel(`in value : Integer[1]; out out1 : Integer[1];`, `assign out1 := value;`))
	if !errors.Is(err, ErrMultiplicityViolation) || !strings.Contains(err.Error(), "value") {
		t.Fatalf("error = %v, want ErrMultiplicityViolation naming the pin", err)
	}
}

func testPinShapeScalarAssignedTwoElements(t *testing.T) {
	_, err := executeActionSource(t, "Probe", pinShapeModel(`in value : Integer; out out1 : Integer[1];`, `assign out1 := value;`))
	if !errors.Is(err, ErrMultiplicityViolation) || !strings.Contains(err.Error(), "out1") {
		t.Fatalf("error = %v, want ErrMultiplicityViolation naming the [1] output", err)
	}
}

func testPinShapeTwoValuesAreNoOperand(t *testing.T) {
	_, err := executeActionSource(t, "Probe", pinShapeModel(`in value : Integer; out out1 : Integer;`, `assign out1 := value + 1;`))
	if !errors.Is(err, ErrTypeMismatch) || !strings.Contains(err.Error(), "sequence") {
		t.Fatalf("error = %v, want ErrTypeMismatch over the two-element pin", err)
	}
}

func testPinShapeBarePinAddressesSend(t *testing.T) {
	outputs, err := executeActionSource(t, "Notifier", `package test {
		private import ScalarValues::*;
		attribute def Go { attribute level : Integer; }
		part def Target;
		action def Notifier {
			out sent : Integer[1] = 0;
			first start;
			then action make { out target : Target = new Target(); }
			then action notify { in target : Target; send new Go(level = 4) to target; assign sent := 4; }
			then done;
			flow make.target to notify.target;
		}
	}`)
	if err != nil {
		t.Fatalf("a bare pin holding one object is that object to a send: %v", err)
	}
	if got := FormatValue(outputs["sent"]); got != "4" {
		t.Fatalf("sent = %s, want 4", got)
	}
}

func testPinShapeUndeterminedSurvives(t *testing.T) {
	known := func(n int64) semantics.Bound { return semantics.Bound{Value: n, Known: true} }
	count := semantics.Range{Lower: known(2), Upper: known(4)}
	value := NewUndeterminedValue(noValueReason("xs"), count)
	target := &writeTarget{mult: count, countJudged: true}
	if err := emptyRuntimeContext().shapeForTarget(target, &value); err != nil {
		t.Fatalf("shapeForTarget: %v", err)
	}
	if value.Kind != ValUndetermined {
		t.Fatalf("an undetermined value shaped for a [2..4] target is %s, want it kept undetermined", value.Kind)
	}
	if got := unknownCountOf(value); got != count {
		t.Fatalf("count = %v, want %v kept", got, count)
	}
}

func testPinShapeToolInputKeepsSequence(t *testing.T) {
	one := sequenceOf([]Value{integerValue(7)})
	multi, err := toolInput("probe", &symbols.Symbol{Name: "xs"}, one, false)
	if err != nil {
		t.Fatalf("multi-valued input: %v", err)
	}
	if len(multi.Items) != 1 || multi.Items[0].Value.Int != 7 {
		t.Fatalf("a multi-valued parameter holding [7] sends %+v, want Items [7]", multi)
	}
	single, err := toolInput("probe", &symbols.Symbol{Name: "x"}, one, true)
	if err != nil {
		t.Fatalf("single-valued input: %v", err)
	}
	if single.Items != nil || single.Value.Int != 7 {
		t.Fatalf("a single-valued parameter holding [7] sends %+v, want Value 7", single)
	}
}
