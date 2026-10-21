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
	t.Run("one_element_tool_input_is_sent_as_its_element", testPinShapeToolInputSendsElement)
	t.Run("one_element_sequence_has_its_element_magnitude", testPinShapeMagnitude)
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

func testPinShapeToolInputSendsElement(t *testing.T) {
	one := sequenceOf([]Value{integerValue(7)})
	sent, err := toolInput("probe", &symbols.Symbol{Name: "xs"}, one)
	if err != nil {
		t.Fatalf("one-element input: %v", err)
	}
	if sent.Items != nil || sent.Value.Int != 7 {
		t.Fatalf("a parameter holding [7] sends %+v, want the scalar 7", sent)
	}
	two := sequenceOf([]Value{integerValue(7), integerValue(8)})
	sent, err = toolInput("probe", &symbols.Symbol{Name: "xs"}, two)
	if err != nil {
		t.Fatalf("two-element input: %v", err)
	}
	if len(sent.Items) != 2 {
		t.Fatalf("a parameter holding [7, 8] sends %+v, want two Items", sent)
	}
}

func testPinShapeMagnitude(t *testing.T) {
	n, ok := MagnitudeValue(sequenceOf([]Value{integerValue(7)}))
	if !ok || n.Int != 7 {
		t.Fatalf("MagnitudeValue([7]) = %v, %v; want 7", n, ok)
	}
	if _, ok := MagnitudeValue(sequenceOf([]Value{integerValue(7), integerValue(8)})); ok {
		t.Fatal("MagnitudeValue([7, 8]) has a magnitude; two values are no one magnitude")
	}
	metre := Unit{Text: "m", Term: semantics.UnitTerm{Scale: semantics.UnitScale(1)}}
	q := sequenceOf([]Value{NewQuantityValue(&Quantity{Num: integerValue(3).Const, Unit: metre})})
	if n, unit, ok := Magnitude(q); !ok || n.AsReal() != 3 || unit == "" {
		t.Fatalf("Magnitude([3 [m]]) = %v, %q, %v; want 3 with its unit", n, unit, ok)
	}
}
