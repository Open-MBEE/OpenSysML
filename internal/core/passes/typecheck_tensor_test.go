package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// tensorPrelude declares a 2×2 TensorMeasurementReference with the imports it needs.
const tensorPrelude = `private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;
	private import MeasurementReferences::*;
	private import Quantities::*;
	private import TensorCalculations::*;
	attribute stressRef : TensorMeasurementReference {
		:>> dimensions = (2, 2);
		:>> mRefs = (Pa, Pa, Pa, Pa);
	}
`

func tensorDiags(t *testing.T, body string) []diag.Diagnostic {
	t.Helper()
	return libraryTypeDiags(t, "package P {\n"+tensorPrelude+body+"\n}")
}

// TestTensorCalculationsBindToTheirDeclaredTypes: every TensorCalculations result,
// a vector or scalar quantity, and each declared tensor member binds without a diagnostic.
func TestTensorCalculationsBindToTheirDeclaredTypes(t *testing.T) {
	if diags := tensorDiags(t, `
	attribute stress : TensorQuantityValue = TensorCalculations::'['((1.0, 2.0, 3.0, 4.0), stressRef);
	attribute sum : TensorQuantityValue = TensorCalculations::'+'(stress, stress);
	attribute difference : TensorQuantityValue = TensorCalculations::'-'(stress, stress);
	attribute doubled : TensorQuantityValue = scalarTensorMult(2, stress);
	attribute doubledRight : TensorQuantityValue = TensorScalarMult(stress, 2.0);
	attribute stretched : TensorQuantityValue = scalarQuantityTensorMult(3 [m], stress);
	attribute stretchedRight : TensorQuantityValue = TensorScalarQuantityMult(stress, 3 [m]);
	attribute scalar : TensorQuantityValue = 3 [m];
	attribute vector : TensorQuantityValue = VectorFunctions::VectorOf((1.0, 2.0)) [m];
	attribute zero : Boolean = isZeroTensorQuantity(stress);
	attribute unit : Boolean = isUnitTensorQuantity(stress);
	attribute dims : Positive[*] ordered = stress.dimensions;
	attribute order : Natural = stress.order;
	attribute flattenedSize : Natural = stress.flattenedSize;
	attribute num : Number[*] ordered = stress.num;
	attribute bound : Boolean = stress.isBound;
	attribute mRef : TensorMeasurementReference = stress.mRef;
	attribute component : ScalarQuantityValue = stress#(1, 2);`); len(diags) != 0 {
		t.Fatalf("expected no type diagnostics, got %v", diags)
	}
}

// TestTensorCalculationsRejectUnrelatedTypes: a tensor result binds to no
// Integer, String or Boolean feature, as the runtime refuses the value.
func TestTensorCalculationsRejectUnrelatedTypes(t *testing.T) {
	diags := tensorDiags(t, `
	attribute count : Integer = TensorCalculations::'['((1.0, 2.0, 3.0, 4.0), stressRef);
	attribute label : String = scalarTensorMult(2, TensorCalculations::'['((1.0, 2.0, 3.0, 4.0), stressRef));
	attribute flag : Boolean = TensorCalculations::'+'(TensorCalculations::'['((1.0, 2.0, 3.0, 4.0), stressRef), TensorCalculations::'['((1.0, 2.0, 3.0, 4.0), stressRef));
	attribute number : Real = isZeroTensorQuantity(TensorCalculations::'['((1.0, 2.0, 3.0, 4.0), stressRef));`)
	want := []string{
		"cannot bind a value of type TensorQuantityValue to a feature typed by Integer",
		"cannot bind a value of type TensorQuantityValue to a feature typed by String",
		"cannot bind a value of type TensorQuantityValue to a feature typed by Boolean",
		"cannot bind Boolean value to a feature typed by Real",
	}
	if len(diags) != len(want) {
		t.Fatalf("want %d type diagnostics, got %v", len(want), diags)
	}
	for i, w := range want {
		if !strings.Contains(diags[i].Message, w) {
			t.Errorf("diagnostic %d = %q, want %q", i, diags[i].Message, w)
		}
	}
}
