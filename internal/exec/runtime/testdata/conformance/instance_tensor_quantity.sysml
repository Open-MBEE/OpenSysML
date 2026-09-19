// The TensorCalculations the library defines over what a TensorQuantityValue
// holds: `'['` pairs numbers with a TensorMeasurementReference's mRefs in
// row-major order over its dimensions and keeps its isBound; its dimensions,
// order, flattenedSize, num and mRef read; `#` indexes as Array does; `+` and `-` convert each component
// to the left operand's unit; the four scalar multiplications scale every
// component, a scalar quantity's unit composing with each component's; the
// zero and unit predicates answer, the latter over a square tensor of order two.
// A vector or scalar quantity is a tensor of order one or zero to them.
package test {
	public import ScalarValues::*;
	public import ISQ::*;
	public import SI::*;
	public import MeasurementReferences::*;
	public import Quantities::*;
	public import TensorCalculations::*;

	attribute <kPa> kilopascal : PressureUnit { :>> unitConversion : ConversionByPrefix { :>> prefix = SIPrefixes::kilo; :>> referenceUnit = Pa; } }
	attribute stressRef : TensorMeasurementReference {
		:>> dimensions = (2, 2);
		:>> mRefs = (Pa, Pa, Pa, Pa);
	}
	attribute boundRef : TensorMeasurementReference {
		:>> dimensions = (2, 2);
		:>> mRefs = (Pa, Pa, Pa, Pa);
		:>> isBound = true;
	}
	attribute mixedRef : TensorMeasurementReference {
		:>> dimensions = (2, 2);
		:>> mRefs = (Pa, kPa, Pa, kPa);
	}
	attribute inertiaRef : TensorMeasurementReference {
		:>> dimensions = (3, 3);
		:>> mRefs = (kg*m**2, kg*m**2, kg*m**2, kg*m**2, kg*m**2, kg*m**2, kg*m**2, kg*m**2, kg*m**2);
	}

	part def Block {
		attribute stress : TensorQuantityValue = TensorCalculations::'['((1.0, 2.0, 3.0, 4.0), stressRef);
		attribute bound : TensorQuantityValue = TensorCalculations::'['((1.0, 2.0, 3.0, 4.0), boundRef);
		attribute mixed : TensorQuantityValue = TensorCalculations::'['((1.0, 2.0, 3.0, 4.0), mixedRef);
		attribute inertia : TensorQuantityValue = TensorCalculations::'['((1, 0, 0, 0, 1, 0, 0, 0, 1), inertiaRef);

		attribute dims : Positive[*] ordered nonunique = stress.dimensions;
		attribute order = stress.order;
		attribute flattenedSize = stress.flattenedSize;
		attribute num : Number[*] ordered = stress.num;
		attribute unbound = stress.isBound;
		attribute isBound = bound.isBound;
		attribute mRef = stress.mRef;
		attribute mRefDims : Positive[*] ordered nonunique = stress.mRef.dimensions;
		attribute row1col2 = stress#(1, 2);
		attribute row2col1 = stress#(2, 1);
		attribute corner = inertia#(3, 3);

		attribute sum = stress + mixed;
		attribute difference = TensorCalculations::'-'(mixed, stress);
		attribute doubled = 2 * stress;
		attribute doubledRight = TensorScalarMult(stress, 2.0);
		attribute stretched = 3 [m] * stress;
		attribute stretchedRight = TensorScalarQuantityMult(stress, 3 [m]);

		attribute zero = isZeroTensorQuantity(stress);
		attribute isZero = isZeroTensorQuantity(TensorCalculations::'['((0, 0, 0, 0), stressRef));
		attribute isUnit = isUnitTensorQuantity(inertia);
		attribute notUnit = isUnitTensorQuantity(stress);

		attribute vectorZero = isZeroTensorQuantity(VectorFunctions::VectorOf((0.0, 0.0)) [m]);
		attribute vectorSum = TensorCalculations::'+'(VectorFunctions::VectorOf((1.0, 2.0)) [m], VectorFunctions::VectorOf((1.0, 2.0)) [km]);
		attribute scalarSum = TensorCalculations::'+'(1 [m], 2 [m]);
		attribute scalarBuilt = TensorCalculations::'['((5), m);
	}
}
