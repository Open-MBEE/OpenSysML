package export_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
)

// setTensorModel values a Set from a repeating sequence and builds a rank-three
// tensor: what the runtime holds as a set and a tensor is, in the model, the
// expression each feature is written with.
const setTensorModel = `package P {
	private import ScalarValues::*;
	private import Collections::*;
	private import Quantities::*;
	private import MeasurementReferences::*;
	private import SI::*;
	attribute s : Set { :>> elements = (3, 1, 2, 2, 3); }
	attribute e : Set { :>> elements = (); }
	attribute nested : Set { :>> elements = (s, e, s); }
	attribute cubeRef : TensorMeasurementReference {
		:>> dimensions = (2, 2, 2);
		:>> mRefs = (Pa, Pa, Pa, Pa, Pa, Pa, Pa, Pa);
	}
	attribute cube : TensorQuantityValue = TensorCalculations::'['((1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0), cubeRef);
	attribute corner : Real = cube#(2, 1, 2);
}
`

// A set or a tensor has no literal form in RDF: the graph states the expression
// the feature is written with, and the runtime evaluates it again after the
// hop. The trip must therefore be exact, with and without the source text.
func TestSetAndTensorValuesRoundTripAsExpressions(t *testing.T) {
	turtle := roundTripsExactly(t, setTensorModel)
	text := string(turtle)
	for _, want := range []string{
		`sysml:redefines "elements"`,
		"a sysml:OperatorExpression ;\n    sysx:sourceText \"(3, 1, 2, 2, 3)\"",
		`sysx:sourceText "()"`,
		"a sysml:InvocationExpression ;\n    sysx:sourceText \"TensorCalculations::'['((1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0), cubeRef)\"",
		`sysml:function "TensorCalculations::["`,
		`sysx:sourceText "cube#(2, 1, 2)"`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("graph lacks %q:\n%s", want, text)
		}
	}
	for _, never := range []string{"Set{", "Tensor(", "xsd:integer\", \"", "urn:opensysml:set", "urn:opensysml:tensor"} {
		if strings.Contains(text, never) {
			t.Errorf("graph states an evaluated value %q, which the mapping does not define:\n%s", never, text)
		}
	}

	stripped := withoutSourceText(t, turtle)
	back, err := export.Convert("m.ttl", stripped, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation from the expression trees alone: %v", err)
	}
	for _, want := range []string{
		"redefines elements = (3, 1, 2, 2, 3);",
		"redefines elements = null;",
		"redefines elements = (s, e, s);",
		"redefines dimensions = (2, 2, 2);",
		"= TensorCalculations::'['((1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0), cubeRef);",
		"= cube#(2, 1, 2);",
	} {
		if !strings.Contains(string(back), want) {
			t.Errorf("the expression trees alone should spell %q\n--- notation ---\n%s", want, back)
		}
	}
	again, err := export.Convert("m.sysml", []byte(back), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle again: %v", err)
	}
	if lost, gained := tripleSetDiff(t, stripped, withoutSourceText(t, again)); len(lost)+len(gained) > 0 {
		t.Errorf("the expression trees alone changed the graph\n--- lost ---\n%s\n--- gained ---\n%s",
			strings.Join(lost, "\n"), strings.Join(gained, "\n"))
	}
}
