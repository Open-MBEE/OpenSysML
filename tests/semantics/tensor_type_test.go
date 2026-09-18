package semantics_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// tensorModel binds a feature to each TensorCalculations invocation.
func tensorModel(t *testing.T) (*semantics.Model, *symbols.Index) {
	t.Helper()
	return shapeModel(t, `package T {
		private import ISQ::*;
		private import SI::*;
		private import MeasurementReferences::*;
		private import Quantities::*;
		private import TensorCalculations::*;
		attribute stressRef : TensorMeasurementReference {
			:>> dimensions = (2, 2);
			:>> mRefs = (Pa, Pa, Pa, Pa);
		}
		attribute stress = TensorCalculations::'['((1.0, 2.0, 3.0, 4.0), stressRef);
		attribute sum = TensorCalculations::'+'(stress, stress);
		attribute difference = TensorCalculations::'-'(stress, stress);
		attribute doubled = scalarTensorMult(2, stress);
		attribute doubledRight = TensorScalarMult(stress, 2);
		attribute stretched = scalarQuantityTensorMult(3 [m], stress);
		attribute stretchedRight = TensorScalarQuantityMult(stress, 3 [m]);
		attribute zero = isZeroTensorQuantity(stress);
		attribute unit = isUnitTensorQuantity(stress);
		attribute product = tensorTensorMult(stress, stress);
		attribute operatorSum = stress + stress;
		attribute operatorScaled = 2 * stress;
	}`)
}

// TestTensorCalculationResultTypes: each TensorCalculations invocation is typed by
// its declared result; the operator forms keep the Kernel function's DataValue.
func TestTensorCalculationResultTypes(t *testing.T) {
	m, idx := tensorModel(t)
	tensor := dimensionSymbol(t, idx, "Quantities::TensorQuantityValue")
	boolean := dimensionSymbol(t, idx, "ScalarValues::Boolean")
	dataValue := dimensionSymbol(t, idx, "Base::DataValue")
	for _, tc := range []struct {
		feature string
		want    *symbols.Symbol
	}{
		{"T::stress", tensor},
		{"T::sum", tensor},
		{"T::difference", tensor},
		{"T::doubled", tensor},
		{"T::doubledRight", tensor},
		{"T::stretched", tensor},
		{"T::stretchedRight", tensor},
		{"T::product", tensor},
		{"T::zero", boolean},
		{"T::unit", boolean},
		{"T::operatorSum", dataValue},
		{"T::operatorScaled", dataValue},
	} {
		sym := dimensionSymbol(t, idx, tc.feature)
		u, ok := sym.Decl.(*ast.Usage)
		if !ok || u.Value == nil {
			t.Fatalf("%s is not a usage with a value", tc.feature)
		}
		if got := m.ExprResultType(sym.OwnerScope, u.Value); got != tc.want {
			t.Errorf("result type of %s = %v, want %v", tc.feature, got, tc.want)
		}
	}
}

// TestTensorQuantityValueSubtypeChain: a vector or scalar quantity is a tensor
// quantity by the library's chain; a tensor is neither of them.
func TestTensorQuantityValueSubtypeChain(t *testing.T) {
	m, idx := tensorModel(t)
	tensor := dimensionSymbol(t, idx, "Quantities::TensorQuantityValue")
	vector := dimensionSymbol(t, idx, "Quantities::VectorQuantityValue")
	scalar := dimensionSymbol(t, idx, "Quantities::ScalarQuantityValue")
	array := dimensionSymbol(t, idx, "Collections::Array")
	for _, tc := range []struct {
		sub, super *symbols.Symbol
		want       bool
	}{
		{vector, tensor, true},
		{scalar, tensor, true},
		{scalar, vector, true},
		{tensor, array, true},
		{tensor, vector, false},
		{tensor, scalar, false},
	} {
		if got := m.Conforms(tc.sub, tc.super); got != tc.want {
			t.Errorf("%s conforms to %s = %v, want %v", tc.sub.Name, tc.super.Name, got, tc.want)
		}
	}
}
