// Indexing a tensor of order three is shape-checked: `#` takes exactly one
// index per dimension, so two or four indexes are a multiplicity violation, and
// each index must lie in 1..dimension, so 0 and dimension + 1 are out of range
// in whichever position they stand. `'['` refuses numbers that do not fill the
// three dimensions, and `+` refuses two tensors of order three whose shapes
// differ, as it does a tensor of order three against one of order two.
package test {
	public import ScalarValues::*;
	public import ISQ::*;
	public import SI::*;
	public import MeasurementReferences::*;
	public import Quantities::*;
	public import TensorCalculations::*;

	attribute cubeRef : TensorMeasurementReference {
		:>> dimensions = (2, 2, 2);
		:>> mRefs = (Pa, Pa, Pa, Pa, Pa, Pa, Pa, Pa);
	}
	attribute slabRef : TensorMeasurementReference {
		:>> dimensions = (2, 3, 2);
		:>> mRefs = (Pa, Pa, Pa, Pa, Pa, Pa, Pa, Pa, Pa, Pa, Pa, Pa);
	}
	attribute planeRef : TensorMeasurementReference {
		:>> dimensions = (2, 2);
		:>> mRefs = (Pa, Pa, Pa, Pa);
	}

	part def Block {
		attribute cube : TensorQuantityValue = TensorCalculations::'['((1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0), cubeRef);
		attribute slab : TensorQuantityValue = TensorCalculations::'['((1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0, 11.0, 12.0), slabRef);
		attribute plane : TensorQuantityValue = TensorCalculations::'['((1.0, 2.0, 3.0, 4.0), planeRef);

		attribute tooFewIndexes = cube#(1, 2);
		attribute oneIndex = cube#(1);
		attribute tooManyIndexes = cube#(1, 1, 1, 1);
		attribute firstIndexLow = cube#(0, 1, 1);
		attribute firstIndexHigh = cube#(3, 1, 1);
		attribute middleIndexHigh = cube#(1, 3, 1);
		attribute lastIndexLow = cube#(1, 1, 0);
		attribute lastIndexHigh = cube#(1, 1, 3);
		attribute slabMiddleHigh = slab#(1, 4, 1);
		attribute nonIntegerIndex = cube#(1, 1.5, 1);

		attribute short = TensorCalculations::'['((1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0), cubeRef);
		attribute long = TensorCalculations::'['((1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0), cubeRef);
		attribute shapes = cube + slab;
		attribute orders = cube + plane;
		attribute scaledShapes = 2 * cube - slab;
	}
}
