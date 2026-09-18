package TensorQuantities {
    private import ScalarValues::*;
    private import ISQ::*;
    private import SI::*;
    private import MeasurementReferences::*;
    private import Quantities::*;
    private import TensorCalculations::*;

    // A 2×2 stress tensor over a model-declared TensorMeasurementReference;
    // every TensorCalculations call the runtime evaluates, and the ones it does not.
    attribute stressRef : TensorMeasurementReference {
        :>> dimensions = (2, 2);
        :>> mRefs = (Pa, Pa, Pa, Pa);
    }
    attribute stress : TensorQuantityValue = TensorCalculations::'['((1.0, 2.0, 3.0, 4.0), stressRef);
    attribute identity : TensorQuantityValue = TensorCalculations::'['((1.0, 0.0, 0.0, 1.0), stressRef);
    attribute dims = stress.dimensions;
    attribute size = stress.flattenedSize;
    attribute corner = stress#(2, 1);
    attribute sum = stress + stress;
    attribute doubled = scalarTensorMult(2, stress);
    attribute stretched = scalarQuantityTensorMult(3 [m], stress);
    attribute zero = isZeroTensorQuantity(stress);
    attribute unit = isUnitTensorQuantity(identity);
    attribute product = tensorTensorMult(stress, stress);
    attribute outerProduct = VectorCalculations::outer(3 [m], 4 [m]);
}
