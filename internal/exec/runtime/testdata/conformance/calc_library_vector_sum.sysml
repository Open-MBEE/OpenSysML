// sum over a collection of vectors adds them as vectors; each is built with
// VectorOf, since a sequence literal of sequences flattens.
package test {
	private import ScalarValues::*;
	private import VectorValues::*;
	private import VectorFunctions::*;

	calc def total {
		return : CartesianThreeVectorValue = sum((VectorOf((1.0, 2.0, 3.0)), VectorOf((4.0, 5.0, 6.0))));
	}
}
