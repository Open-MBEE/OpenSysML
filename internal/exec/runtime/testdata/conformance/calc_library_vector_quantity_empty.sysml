// A quantity's num is Number[1..*] and its unit lives on an axis, so a vector
// of no components is not a vector quantity in any unit: `[m]` over it is a
// multiplicity violation, not a unitless empty value.
package test {
	private import ScalarValues::*;
	private import VectorValues::*;
	private import VectorFunctions::*;
	private import Quantities::*;
	private import SI::*;

	calc def emptyQuantity {
		return : VectorQuantityValue = CartesianVectorOf(()) [m];
	}
}
