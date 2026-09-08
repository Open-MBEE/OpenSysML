// A TensorQuantityValue is of any order: `'['` pairs the numbers with a
// reference of three (or four) dimensions in row-major order, the last index
// varying fastest, and the tensor answers its dimensions, order, flattenedSize
// and num from that shape. `#` takes one Positive index per dimension; `+`, `-`
// and the scalar multiplications keep the shape, component by component.
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
		:>> mRefs = (m, m, m, m, m, m, m, m, m, m, m, m);
	}
	attribute hyperRef : TensorMeasurementReference {
		:>> dimensions = (2, 1, 2, 2);
		:>> mRefs = (s, s, s, s, s, s, s, s);
	}

	part def Block {
		attribute cube : TensorQuantityValue = TensorCalculations::'['((1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0), cubeRef);
		attribute slab : TensorQuantityValue = TensorCalculations::'['((1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12), slabRef);
		attribute hyper : TensorQuantityValue = TensorCalculations::'['((1, 2, 3, 4, 5, 6, 7, 8), hyperRef);

		attribute dims : Positive[*] ordered = cube.dimensions;
		attribute order = cube.order;
		attribute flattenedSize = cube.flattenedSize;
		attribute slabDims : Positive[*] ordered = slab.dimensions;
		attribute hyperOrder = hyper.order;
		attribute num : Number[*] ordered = cube.num;

		attribute origin = cube#(1, 1, 1);
		attribute lastVariesFastest = cube#(1, 1, 2);
		attribute middleIndex = cube#(1, 2, 1);
		attribute firstIndex = cube#(2, 1, 1);
		attribute last = cube#(2, 2, 2);
		attribute slabCell = slab#(2, 3, 1);
		attribute hyperCell = hyper#(2, 1, 1, 2);

		attribute sum = cube + cube;
		attribute difference = cube - cube;
		attribute doubled = 2 * cube;
		attribute stretched = 3 [m] * slab;

		attribute zero = isZeroTensorQuantity(cube);
		attribute isZero = isZeroTensorQuantity(cube - cube);
	}
}
