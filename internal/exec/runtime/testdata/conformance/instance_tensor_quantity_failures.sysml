// What TensorCalculations does not determine fails by name. `'['` refuses an
// element count that does not fill the reference's dimensions, and a reference
// stating one mRef for four components is refused as the Array it is (mRefs
// redefines elements, so one reference does not broadcast); `+` refuses tensors of different
// dimensions and components whose units are incommensurable; the unit predicate
// is stated for a square tensor of order two only; the tensor-vector,
// vector-tensor and tensor-tensor products name no contraction; `outer`
// declares a result no outer product is; and `transform` needs a coordinate
// frame the runtime holds no value of. A covariance the model never set, and the
// boundness of a bodiless calculation's result, are unstated rather than made up.
package test {
	public import ISQ::*;
	public import SI::*;
	public import MeasurementReferences::*;
	public import Quantities::*;
	public import TensorCalculations::*;

	attribute stressRef : TensorMeasurementReference {
		:>> dimensions = (2, 2);
		:>> mRefs = (Pa, Pa, Pa, Pa);
	}
	attribute oneRef : TensorMeasurementReference {
		:>> dimensions = (2, 2);
		:>> mRefs = (Pa);
	}
	attribute clockRef : TensorMeasurementReference {
		:>> dimensions = (2, 2);
		:>> mRefs = (Pa, s, Pa, Pa);
	}
	attribute wideRef : TensorMeasurementReference {
		:>> dimensions = (2, 3);
		:>> mRefs = (Pa, Pa, Pa, Pa, Pa, Pa);
	}

	part def Block {
		attribute stress : TensorQuantityValue = TensorCalculations::'['((1.0, 2.0, 3.0, 4.0), stressRef);
		attribute wide : TensorQuantityValue = TensorCalculations::'['((1.0, 2.0, 3.0, 4.0, 5.0, 6.0), wideRef);
		attribute clock : TensorQuantityValue = TensorCalculations::'['((1.0, 2.0, 3.0, 4.0), clockRef);

		attribute short = TensorCalculations::'['((1.0, 2.0, 3.0), stressRef);
		attribute broadcast = TensorCalculations::'['((1.0, 2.0, 3.0, 4.0), oneRef);
		attribute notAReference = TensorCalculations::'['((1.0, 2.0, 3.0, 4.0), 4);
		attribute shapes = stress + wide;
		attribute incommensurable = stress + clock;
		attribute notATensor = isZeroTensorQuantity(true);
		attribute unitOfWide = isUnitTensorQuantity(wide);
		attribute unitOfVector = isUnitTensorQuantity(VectorFunctions::VectorOf((1.0, 1.0)) [m]);
		attribute tooManyIndexes = stress#(1, 1, 1);
		attribute outOfRange = stress#(3, 1);
		attribute covariance = stress.covariantOrder;
		attribute sumBound = (stress + stress).isBound;

		attribute byVector = tensorVectorMult(stress, VectorFunctions::VectorOf((1.0, 2.0)) [m]);
		attribute vectorBy = vectorTensorMult(VectorFunctions::VectorOf((1.0, 2.0)) [m], stress);
		attribute byTensor = tensorTensorMult(stress, stress);
		attribute spread = VectorCalculations::outer(VectorFunctions::VectorOf((1.0, 2.0)) [m], VectorFunctions::VectorOf((3.0, 4.0)) [m]);
		attribute turned = transform(VectorFunctions::VectorOf((1.0, 2.0)) [m], stress);
	}
}
