// Vector addition needs vectors of one dimension; the mismatch is reported with
// both dimensions.
package test {
	private import ScalarValues::*;
	private import VectorValues::*;
	private import VectorFunctions::*;

	calc def total {
		return : CartesianVectorValue = VectorOf((1.0, 2.0)) + VectorOf((1.0, 2.0, 3.0));
	}
}
