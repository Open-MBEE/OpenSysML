// The scalar-valued vector operations: the norm and the inner product of the
// vendored VectorFunctions, reached by their unqualified names.
package test {
	private import ScalarValues::*;
	private import VectorFunctions::*;

	calc def diagonal {
		return : Real = norm((3.0, 4.0)) + cartesianInner((1.0, 2.0), (3.0, 4.0));
	}
}
