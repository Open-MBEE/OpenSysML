// VectorCalculations::norm and inner over vector quantities are declared
// `return : Number[1]`: the magnitude in the unit the axes compose, which a
// Number feature therefore accepts.
package test {
	private import ScalarValues::*;
	private import VectorValues::*;
	private import Quantities::*;
	private import SI::*;

	attribute displacement : VectorQuantityValue = VectorFunctions::VectorOf((3.0, 4.0)) [m];

	calc def distance {
		attribute squared : Number = VectorCalculations::inner(displacement, displacement);
		return : Number = VectorCalculations::norm(displacement) + squared;
	}
}
