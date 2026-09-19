// ScalarQuantityValue specializes VectorQuantityValue, but it is a ScalarValue:
// a vector quantity of two axes is refused where a scalar quantity is declared.
package test {
	private import ScalarValues::*;
	private import VectorValues::*;
	private import VectorFunctions::*;
	private import Quantities::*;
	private import ISQ::*;
	private import SI::*;

	calc def flattened {
		return : LengthValue = VectorOf((1.0, 2.0)) [m];
	}
}
