// A vector quantity is a vector with a unit on each axis: a scalar quantity
// times a vector carries the unit, and dividing by a quantity composes units as
// the scalar QuantityCalculations do.
package test {
	private import ScalarValues::*;
	private import VectorValues::*;
	private import Quantities::*;
	private import SI::*;

	attribute displacement : VectorQuantityValue = 2 [m] * VectorFunctions::VectorOf((1.0, 2.0, 3.0));

	calc def velocity {
		return : VectorQuantityValue = displacement / 4 [s];
	}
}
