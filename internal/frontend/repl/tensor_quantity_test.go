package repl

import "testing"

// tensorSession declares a 2×2 stress tensor over a model-declared
// TensorMeasurementReference, with the tensor calculations in scope.
func tensorSession(t *testing.T) *Session {
	t.Helper()
	s := NewSession()
	res := s.Submit(`package Stress {
		private import ScalarValues::*;
		private import ISQ::*;
		private import SI::*;
		private import MeasurementReferences::*;
		private import Quantities::*;
		private import TensorCalculations::*;
		attribute stressRef : TensorMeasurementReference {
			:>> dimensions = (2, 2);
			:>> mRefs = (Pa, Pa, Pa, Pa);
		}
		part def Plate {
			attribute stress : TensorQuantityValue = TensorCalculations::'['((1.0, 2.0, 3.0, 4.0), stressRef);
			attribute pressure : PressureValue = 2 [Pa];
		}
		part plate : Plate;
	}`)
	if len(errorDiagnostics(res.Diagnostics)) > 0 {
		t.Fatalf("fixture has diagnostics: %v", res.Diagnostics)
	}
	return s
}

// A tensor renders as its shape, its row-major components and their unit; its
// structural members, indexing and the componentwise calculations evaluate.
func TestEvalTensorQuantities(t *testing.T) {
	s := tensorSession(t)
	wants(t, run(t, s, "%eval plate.stress"), "✓", "= Tensor(2, 2)[1.0, 2.0, 3.0, 4.0] [Pa]")
	wants(t, run(t, s, "%eval plate.stress.dimensions"), "= [2, 2]")
	wants(t, run(t, s, "%eval plate.stress.order"), "= 2")
	wants(t, run(t, s, "%eval plate.stress.flattenedSize"), "= 4")
	wants(t, run(t, s, "%eval plate.stress.isBound"), "= false")
	wants(t, run(t, s, "%eval plate.stress.mRef.mRefs"), "= [Pa, Pa, Pa, Pa]")
	wants(t, run(t, s, "%eval plate.stress#(2, 1)"), "= 3.0 [Pa]")
	wants(t, run(t, s, "%eval plate.stress + plate.stress"), "= Tensor(2, 2)[2.0, 4.0, 6.0, 8.0] [Pa]")
	wants(t, run(t, s, "%eval TensorCalculations::scalarTensorMult(2, plate.stress)"), "= Tensor(2, 2)[2.0, 4.0, 6.0, 8.0] [Pa]")
	wants(t, run(t, s, "%eval TensorCalculations::TensorScalarQuantityMult(plate.stress, 2 [m])"), "= Tensor(2, 2)[2.0, 4.0, 6.0, 8.0] [SI::'kg⋅s⁻²']")
	wants(t, run(t, s, "%eval TensorCalculations::isZeroTensorQuantity(plate.stress - plate.stress)"), "= true")
	wants(t, run(t, s, "%eval TensorCalculations::isZeroTensorQuantity(plate.pressure)"), "= false")
}

// What the library leaves undetermined about a tensor is a typed failure that
// quotes the declaration, never a made-up value.
func TestEvalTensorQuantityLimitsAreTyped(t *testing.T) {
	s := tensorSession(t)
	wants(t, run(t, s, "%eval TensorCalculations::tensorTensorMult(plate.stress, plate.stress)"),
		"error:", "library function is not evaluable", "TensorCalculations::tensorTensorMult", "which indices contract")
	wants(t, run(t, s, "%eval TensorCalculations::isUnitTensorQuantity(TensorCalculations::'['((1.0, 2.0, 3.0), stressRef))"),
		"error:", "multiplicity violation", "3 elements for a reference of dimensions [2, 2]", "`n = mRef.flattenedSize` = 4")
	wants(t, run(t, s, "%eval plate.stress.covariantOrder"),
		"error:", "library function is not evaluable", "covariantOrder", "orderSum")
	wants(t, run(t, s, "%eval plate.stress + plate.pressure"),
		"error:", "multiplicity violation", "dimensions [2, 2] and [] differ")
}

// %features lists a tensor-valued attribute as its tensor value.
func TestFeaturesListTensorQuantities(t *testing.T) {
	s := tensorSession(t)
	run(t, s, "%instantiate Stress::plate")
	wants(t, run(t, s, "%features Stress::plate"), "\n  stress = Tensor(2, 2)[1.0, 2.0, 3.0, 4.0] [Pa]", "\n  pressure = 2 [Pa]")
}

// A tensor quantity binds nothing to a query parameter: the refusal names it.
func TestRunQueryRefusesATensorQuantity(t *testing.T) {
	s := docQuerySession(t)
	if res := s.Submit(`package Stress {
		private import SI::*;
		private import MeasurementReferences::*;
		attribute stressRef : TensorMeasurementReference { :>> dimensions = (2, 2); :>> mRefs = (Pa, Pa, Pa, Pa); }
	}`); len(errorDiagnostics(res.Diagnostics)) > 0 {
		t.Fatalf("fixture has diagnostics: %v", res.Diagnostics)
	}
	wants(t, run(t, s, `%run-query NamedSubsystems root=TensorCalculations::'['((1.0, 2.0, 3.0, 4.0), Stress::stressRef) pattern="unit"`),
		"error:", "binding root: a tensor quantity Tensor(2, 2)[1.0, 2.0, 3.0, 4.0] [Pa] cannot be bound to a query parameter")
}
