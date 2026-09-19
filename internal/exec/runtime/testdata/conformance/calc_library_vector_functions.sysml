// A calc body computing with VectorFunctions: a tuple of numbers is accepted as
// a vector, and the library's Cartesian operations answer a vector value.
package test {
	private import ScalarValues::*;
	private import VectorValues::*;

	calc def scaledSum {
		in x : Real;
		return : CartesianVectorValue = VectorFunctions::cartesianVectorScalarMult(
			VectorFunctions::'cartesian+'((1.0, 2.0), (3.0, 4.0)), x);
	}
}
