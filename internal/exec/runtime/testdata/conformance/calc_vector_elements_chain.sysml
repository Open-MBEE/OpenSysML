// A vector's `elements` feature is the sequence of its components, so chaining
// into it from a vector result reads that sequence.
package test {
	private import ScalarValues::*;
	private import VectorFunctions::*;

	calc def probe {
		return : Real[*] = 'cartesian+'(
			CartesianThreeVectorOf((1.0, 2.0, 3.0)),
			CartesianThreeVectorOf((4.0, 5.0, 6.0))).elements;
	}
}
