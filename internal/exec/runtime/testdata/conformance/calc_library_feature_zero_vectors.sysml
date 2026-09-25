// cartesianZeroVector is one feature of three vectors, the zero vectors of
// dimension 1, 2 and 3, each kept whole rather than flattened to six Reals.
package test {
	private import ScalarValues::*;
	private import VectorValues::*;

	calc def zeros {
		return : CartesianVectorValue[3] = VectorFunctions::cartesianZeroVector;
	}
}
